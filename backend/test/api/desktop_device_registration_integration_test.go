package api_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	appserver "rustdesk-api-server-pro/app"
	"rustdesk-api-server-pro/app/controller/api"
	deviceform "rustdesk-api-server-pro/app/form/api"
	"rustdesk-api-server-pro/app/model"
	devicepolicy "rustdesk-api-server-pro/app/policy"
	"rustdesk-api-server-pro/config"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	"golang.org/x/crypto/nacl/box"
	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func TestDesktopRegistrationAndEncryptedFixedPasswordPolicy(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:desktop-auth-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(
		new(model.Device), new(model.DeviceCredential), new(model.DesktopEnrollmentToken),
		new(model.DeviceGroup), new(model.ServerProfile), new(model.StrategyState),
	); err != nil {
		t.Fatal(err)
	}
	cfg := config.GetDefaultServerConfig()
	seed := make([]byte, ed25519.SeedSize)
	if _, err = rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	cfg.ProvisioningSignSeed = base64.StdEncoding.EncodeToString(seed)
	cfg.ProvisioningSecretKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	cfg.ProvisioningKeyId = "desktop-integration"
	profile := model.ServerProfile{Name: "desktop", IdServer: "id.example.com", Enabled: true}
	if _, err = db.Insert(&profile); err != nil {
		t.Fatal(err)
	}
	encryptedPassword, err := devicepolicy.EncryptPassword("managed-password", cfg.ProvisioningSecretKey)
	if err != nil {
		t.Fatal(err)
	}
	group := model.DeviceGroup{
		Name: "desktop", Enabled: true, ProfileId: profile.Id, UnattendedEnabled: true,
		PasswordCiphertext: encryptedPassword, RootCommand: "/system/xbin/su",
	}
	if _, err = db.Insert(&group); err != nil {
		t.Fatal(err)
	}
	state := model.StrategyState{Id: 1, DefaultGroupId: group.Id, Revision: 41}
	if _, err = db.Insert(&state); err != nil {
		t.Fatal(err)
	}
	plainToken, selector, tokenHash, err := api.GenerateDesktopEnrollmentToken()
	if err != nil {
		t.Fatal(err)
	}
	token := model.DesktopEnrollmentToken{
		Selector: selector, SecretHash: tokenHash, GroupId: group.Id, ExpiresAt: time.Now().Add(time.Hour),
	}
	if _, err = db.Insert(&token); err != nil {
		t.Fatal(err)
	}
	application := iris.New()
	application.RegisterDependency(db, cfg)
	appserver.SetRoute(application)
	if err = application.Build(); err != nil {
		t.Fatal(err)
	}

	signPublicKey, signPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	boxPublicKey, boxSecretKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	registration := deviceform.DesktopDeviceRegistrationForm{
		Version: 1, RequestId: "request-1", Token: plainToken, RustdeskId: "987654321",
		Uuid: "desktop-uuid", Hostname: "desktop-host", Os: "linux", Arch: "x86_64",
		ClientVersion: "1.4.6", SignPublicKey: base64.StdEncoding.EncodeToString(signPublicKey),
		BoxPublicKey: base64.StdEncoding.EncodeToString(boxPublicKey[:]), Timestamp: time.Now().Unix(),
	}
	registration.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(
		signPrivateKey, api.BuildDesktopRegistrationMessage(&registration, signPublicKey, boxPublicKey[:]),
	))
	registrationResponse := postJSON(t, application, "/api/device/register/desktop", registration)
	if registrationResponse.Code != iris.StatusOK {
		t.Fatalf("registration status=%d body=%s", registrationResponse.Code, registrationResponse.Body.String())
	}
	var registered struct {
		Accepted              bool   `json:"accepted"`
		DeviceID              int    `json:"device_id"`
		PolicyVerifyPublicKey string `json:"policy_verify_public_key"`
	}
	if err = json.Unmarshal(registrationResponse.Body.Bytes(), &registered); err != nil {
		t.Fatal(err)
	}
	if !registered.Accepted || registered.DeviceID <= 0 {
		t.Fatalf("unexpected registration response: %s", registrationResponse.Body.String())
	}
	if _, err = db.ID(token.Id).Cols("expires_at").Update(&model.DesktopEnrollmentToken{ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if retry := postJSON(t, application, "/api/device/register/desktop", registration); retry.Code != iris.StatusOK {
		t.Fatalf("idempotent registration after token expiry failed: %s", retry.Body.String())
	}
	registration.RequestId = "request-2"
	registration.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(
		signPrivateKey, api.BuildDesktopRegistrationMessage(&registration, signPublicKey, boxPublicKey[:]),
	))
	if replay := postJSON(t, application, "/api/device/register/desktop", registration); replay.Code != iris.StatusConflict {
		t.Fatalf("consumed token accepted a different request: status=%d body=%s", replay.Code, replay.Body.String())
	}

	payload, err := json.Marshal(map[string]any{
		"id": "987654321", "uuid": "desktop-uuid", "version": "1.4.6", "modified_at": 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	heartbeat := map[string]any{
		"device_id": registered.DeviceID, "sequence": 1,
		"payload": base64.StdEncoding.EncodeToString(payload),
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(
			signPrivateKey, api.BuildDeviceRequestMessage(int64(registered.DeviceID), 1, payload),
		)),
	}
	heartbeatResponse := postJSON(t, application, "/api/device/heartbeat", heartbeat)
	if heartbeatResponse.Code != iris.StatusOK {
		t.Fatalf("heartbeat status=%d body=%s", heartbeatResponse.Code, heartbeatResponse.Body.String())
	}
	var response struct {
		ModifiedAt int64 `json:"modified_at"`
		Strategy   struct {
			Extra map[string]string `json:"extra"`
		} `json:"strategy"`
	}
	if err = json.Unmarshal(heartbeatResponse.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	encodedEnvelope := response.Strategy.Extra["desktop_provisioning"]
	if encodedEnvelope == "" || response.Strategy.Extra["android_provisioning"] != "" {
		t.Fatalf("unexpected strategy envelope: %s", heartbeatResponse.Body.String())
	}
	policy := decryptDesktopPolicyForTest(t, encodedEnvelope, registered.PolicyVerifyPublicKey, boxPublicKey, boxSecretKey)
	if policy.PasswordAction != "set" || policy.PermanentPassword != "managed-password" || policy.HasRootCommand {
		t.Fatalf("unexpected desktop policy: %+v", policy)
	}

	statusPayload, err := json.Marshal(map[string]any{
		"id": "987654321", "uuid": "desktop-uuid", "version": "1.4.6", "modified_at": response.ModifiedAt,
		"password_status": map[string]any{
			"applied_revision": response.ModifiedAt, "status": "success", "permanent_password_set": true, "last_error": "",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	heartbeat["sequence"] = 2
	heartbeat["payload"] = base64.StdEncoding.EncodeToString(statusPayload)
	heartbeat["signature"] = base64.StdEncoding.EncodeToString(ed25519.Sign(
		signPrivateKey, api.BuildDeviceRequestMessage(int64(registered.DeviceID), 2, statusPayload),
	))
	if statusResponse := postJSON(t, application, "/api/device/heartbeat", heartbeat); statusResponse.Code != iris.StatusOK {
		t.Fatalf("password status rejected: %s", statusResponse.Body.String())
	}
	saved := model.Device{}
	if found, loadErr := db.ID(registered.DeviceID).Get(&saved); loadErr != nil || !found {
		t.Fatalf("failed to load registered desktop: found=%v err=%v", found, loadErr)
	}
	if saved.Platform != "desktop" || saved.PasswordApplyStatus != "success" || !saved.PermanentPasswordSet || saved.PasswordAppliedRevision != 41 {
		t.Fatalf("desktop password status was not stored: %+v", saved)
	}

	if _, err = db.ID(group.Id).Cols("password_ciphertext").Update(&model.DeviceGroup{}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ID(state.Id).Cols("revision").Update(&model.StrategyState{Revision: 42}); err != nil {
		t.Fatal(err)
	}
	clearRequest, err := json.Marshal(map[string]any{
		"id": "987654321", "uuid": "desktop-uuid", "version": "1.4.6", "modified_at": 41,
	})
	if err != nil {
		t.Fatal(err)
	}
	heartbeat["sequence"] = 3
	heartbeat["payload"] = base64.StdEncoding.EncodeToString(clearRequest)
	heartbeat["signature"] = base64.StdEncoding.EncodeToString(ed25519.Sign(
		signPrivateKey, api.BuildDeviceRequestMessage(int64(registered.DeviceID), 3, clearRequest),
	))
	clearResponse := postJSON(t, application, "/api/device/heartbeat", heartbeat)
	if clearResponse.Code != iris.StatusOK {
		t.Fatalf("clear policy heartbeat failed: %s", clearResponse.Body.String())
	}
	if err = json.Unmarshal(clearResponse.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	clearPolicy := decryptDesktopPolicyForTest(
		t, response.Strategy.Extra["desktop_provisioning"], registered.PolicyVerifyPublicKey, boxPublicKey, boxSecretKey,
	)
	if clearPolicy.PasswordAction != "clear" || clearPolicy.PermanentPassword != "" || clearPolicy.HasRootCommand {
		t.Fatalf("unexpected clear policy: %+v", clearPolicy)
	}
	clearedStatus, err := json.Marshal(map[string]any{
		"id": "987654321", "uuid": "desktop-uuid", "version": "1.4.6", "modified_at": int64(42),
		"password_status": map[string]any{
			"applied_revision": int64(42), "status": "cleared", "permanent_password_set": false, "last_error": "",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	heartbeat["sequence"] = 4
	heartbeat["payload"] = base64.StdEncoding.EncodeToString(clearedStatus)
	heartbeat["signature"] = base64.StdEncoding.EncodeToString(ed25519.Sign(
		signPrivateKey, api.BuildDeviceRequestMessage(int64(registered.DeviceID), 4, clearedStatus),
	))
	if clearStatusResponse := postJSON(t, application, "/api/device/heartbeat", heartbeat); clearStatusResponse.Code != iris.StatusOK {
		t.Fatalf("cleared password status rejected: %s", clearStatusResponse.Body.String())
	}
	saved = model.Device{}
	if found, loadErr := db.ID(registered.DeviceID).Get(&saved); loadErr != nil || !found {
		t.Fatalf("failed to reload registered desktop: found=%v err=%v", found, loadErr)
	}
	if saved.PasswordApplyStatus != "cleared" || saved.PermanentPasswordSet || saved.PasswordAppliedRevision != 42 {
		t.Fatalf("desktop cleared password status was not stored: %+v", saved)
	}
}

type desktopPolicyForTest struct {
	PasswordAction    string
	PermanentPassword string
	HasRootCommand    bool
}

func decryptDesktopPolicyForTest(t *testing.T, encoded, verifyKey string, boxPublicKey, boxSecretKey *[32]byte) desktopPolicyForTest {
	t.Helper()
	envelopeJSON, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Version    int    `json:"version"`
		Purpose    string `json:"purpose"`
		KeyID      string `json:"key_id"`
		DeviceID   int    `json:"device_id"`
		Revision   int64  `json:"revision"`
		Ciphertext string `json:"ciphertext"`
		Signature  string `json:"signature"`
	}
	if err = json.Unmarshal(envelopeJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := base64.StdEncoding.DecodeString(verifyKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		t.Fatal("invalid registration policy verification key")
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil || !ed25519.Verify(publicKey, api.BuildDesktopPolicySignatureMessage(
		envelope.Version, envelope.Purpose, envelope.KeyID, envelope.DeviceID, envelope.Revision, ciphertext,
	), signature) {
		t.Fatal("desktop policy signature was rejected")
	}
	plaintext, ok := box.OpenAnonymous(nil, ciphertext, boxPublicKey, boxSecretKey)
	if !ok {
		t.Fatal("desktop policy decryption failed")
	}
	var policy struct {
		Desktop struct {
			Unattended struct {
				PasswordAction    string `json:"password_action"`
				PermanentPassword string `json:"permanent_password"`
			} `json:"unattended"`
		} `json:"desktop"`
	}
	if err = json.Unmarshal(plaintext, &policy); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err = json.Unmarshal(plaintext, &raw); err != nil {
		t.Fatal(err)
	}
	_, rootAtTop := raw["root_command"]
	return desktopPolicyForTest{
		PasswordAction:    policy.Desktop.Unattended.PasswordAction,
		PermanentPassword: policy.Desktop.Unattended.PermanentPassword,
		HasRootCommand:    rootAtTop || bytes.Contains(plaintext, []byte("root_command")),
	}
}
