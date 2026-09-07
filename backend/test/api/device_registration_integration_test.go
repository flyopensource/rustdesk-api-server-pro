package api_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	appserver "rustdesk-api-server-pro/app"
	controller "rustdesk-api-server-pro/app/controller/api"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/config"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func registrationProof(message, key []byte) []byte {
	mac := hmac.New(sha512.New, key)
	_, _ = mac.Write(message)
	return mac.Sum(nil)[:sha512.Size256]
}

func postJSON(t *testing.T, application http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	application.ServeHTTP(response, request)
	return response
}

func TestDeviceRegistrationAndSignedHeartbeat(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:device-auth-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	if err = engine.Sync(
		new(model.Device), new(model.DeviceCredential), new(model.DeviceGroup),
		new(model.ServerProfile), new(model.StrategyState),
	); err != nil {
		t.Fatal(err)
	}
	enrollmentKey := []byte("01234567890123456789012345678901")
	serverConfig := config.GetDefaultServerConfig()
	serverConfig.DeviceEnrollmentKey = base64.StdEncoding.EncodeToString(enrollmentKey)
	serverConfig.ProvisioningSignSeed = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SeedSize))
	serverConfig.ProvisioningSecretKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	application := iris.New()
	application.RegisterDependency(engine, serverConfig)
	appserver.SetRoute(application)
	if err = application.Build(); err != nil {
		t.Fatal(err)
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	timestamp := time.Now().Unix()
	registrationMessage := controller.BuildRegistrationMessage("123456789", "device-uuid", publicKey, timestamp)
	registration := map[string]any{
		"id":               "123456789",
		"uuid":             "device-uuid",
		"public_key":       base64.StdEncoding.EncodeToString(publicKey),
		"timestamp":        timestamp,
		"signature":        base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, registrationMessage)),
		"enrollment_proof": base64.StdEncoding.EncodeToString(registrationProof(registrationMessage, enrollmentKey)),
	}
	registrationResponse := postJSON(t, application, "/api/device/register", registration)
	if registrationResponse.Code != iris.StatusOK {
		t.Fatalf("registration status = %d, body = %s", registrationResponse.Code, registrationResponse.Body.String())
	}
	var registrationResult struct {
		Accepted bool  `json:"accepted"`
		DeviceID int64 `json:"device_id"`
	}
	if err = json.Unmarshal(registrationResponse.Body.Bytes(), &registrationResult); err != nil {
		t.Fatal(err)
	}
	if !registrationResult.Accepted || registrationResult.DeviceID <= 0 {
		t.Fatalf("unexpected registration response: %s", registrationResponse.Body.String())
	}
	deviceID := registrationResult.DeviceID
	savedDevice := model.Device{}
	if found, findErr := engine.ID(deviceID).Get(&savedDevice); findErr != nil || !found {
		t.Fatalf("failed to load device: found=%v err=%v", found, findErr)
	}
	if savedDevice.ConnectionIdStatus != model.ConnectionIdUnassigned {
		t.Fatalf("unexpected initial connection id status: %q", savedDevice.ConnectionIdStatus)
	}

	changedIDMessage := controller.BuildRegistrationMessage("changed-device", "device-uuid", publicKey, timestamp)
	changedIDRegistration := map[string]any{
		"id":               "changed-device",
		"uuid":             "device-uuid",
		"public_key":       base64.StdEncoding.EncodeToString(publicKey),
		"timestamp":        timestamp,
		"signature":        base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, changedIDMessage)),
		"enrollment_proof": base64.StdEncoding.EncodeToString(registrationProof(changedIDMessage, enrollmentKey)),
	}
	if response := postJSON(t, application, "/api/device/register", changedIDRegistration); response.Code != iris.StatusConflict {
		t.Fatalf("unexpected changed id registration status = %d, body = %s", response.Code, response.Body.String())
	}
	if count, countErr := engine.Count(new(model.Device)); countErr != nil || count != 1 {
		t.Fatalf("changed id created duplicate device: count=%d err=%v", count, countErr)
	}

	payload, err := json.Marshal(map[string]any{
		"id":          "123456789",
		"uuid":        "device-uuid",
		"version":     "1.5.0",
		"modified_at": 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	requestMessage := controller.BuildDeviceRequestMessage(deviceID, 1, payload)
	heartbeat := map[string]any{
		"device_id": deviceID,
		"sequence":  1,
		"payload":   base64.StdEncoding.EncodeToString(payload),
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, requestMessage)),
	}
	if response := postJSON(t, application, "/api/device/heartbeat", heartbeat); response.Code != iris.StatusOK {
		t.Fatalf("heartbeat status = %d, body = %s", response.Code, response.Body.String())
	}
	if response := postJSON(t, application, "/api/device/heartbeat", heartbeat); response.Code != iris.StatusConflict {
		t.Fatalf("replay status = %d, body = %s", response.Code, response.Body.String())
	}
	credential := model.DeviceCredential{}
	if found, findErr := engine.Where("device_id = ?", deviceID).Get(&credential); findErr != nil || !found {
		t.Fatalf("failed to load credential: found=%v err=%v", found, findErr)
	}
	credential.Enabled = false
	if _, err = engine.ID(credential.Id).Cols("enabled").Update(&credential); err != nil {
		t.Fatal(err)
	}
	if response := postJSON(t, application, "/api/device/register", registration); response.Code != iris.StatusForbidden {
		t.Fatalf("disabled registration status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, path := range []string{"/api/device/heartbeat", "/api/device/sysinfo"} {
		if response := postJSON(t, application, path, heartbeat); response.Code != iris.StatusUnauthorized {
			t.Fatalf("disabled credential accepted by %s: %s", path, response.Body.String())
		}
	}
	credential.Enabled = true
	if _, err = engine.ID(credential.Id).Cols("enabled").Update(&credential); err != nil {
		t.Fatal(err)
	}
	heartbeat["sequence"] = 2
	heartbeat["signature"] = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, controller.BuildDeviceRequestMessage(deviceID, 2, payload)))
	if response := postJSON(t, application, "/api/device/heartbeat", heartbeat); response.Code != iris.StatusOK {
		t.Fatalf("enabled credential rejected: %s", response.Body.String())
	}
	for i, ready := range []bool{true, false} {
		payload, err = json.Marshal(map[string]any{"id": "123456789", "uuid": "device-uuid", "unattended_status": map[string]any{"all_files_access_ready": ready, "status": "partial"}})
		if err != nil {
			t.Fatal(err)
		}
		sequence := int64(i + 3)
		statusRequest := map[string]any{"device_id": deviceID, "sequence": sequence, "payload": base64.StdEncoding.EncodeToString(payload), "signature": base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, controller.BuildDeviceRequestMessage(deviceID, sequence, payload)))}
		if response := postJSON(t, application, "/api/device/heartbeat", statusRequest); response.Code != iris.StatusOK {
			t.Fatalf("status rejected: %s", response.Body.String())
		}
		saved := model.Device{}
		if _, err = engine.ID(deviceID).Get(&saved); err != nil || saved.AllFilesAccessReady != ready {
			t.Fatalf("permission status not persisted: %v", err)
		}
	}
}
