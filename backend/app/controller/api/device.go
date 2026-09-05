package api

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"rustdesk-api-server-pro/app/form/api"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/config"
	versionhelper "rustdesk-api-server-pro/helper/version"
	"strconv"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
)

const deviceClockSkewSeconds int64 = 300

type DeviceController struct {
	basicController
	ServerConfig *config.ServerConfig
}

func appendField(message []byte, value string) []byte {
	message = append(message, value...)
	return append(message, 0)
}

func BuildRegistrationMessage(id, uuid string, publicKey []byte, timestamp int64) []byte {
	message := append([]byte{}, []byte("RUD-DEVICE-REGISTER\x00")...)
	message = appendField(message, id)
	message = appendField(message, uuid)
	message = append(message, publicKey...)
	message = append(message, 0)
	return appendField(message, strconv.FormatInt(timestamp, 10))
}

func BuildDeviceRequestMessage(deviceID, sequence int64, payload []byte) []byte {
	message := append([]byte{}, []byte("RUD-DEVICE-REQUEST\x00")...)
	message = appendField(message, strconv.FormatInt(deviceID, 10))
	message = appendField(message, strconv.FormatInt(sequence, 10))
	digest := sha256.Sum256(payload)
	return append(message, digest[:]...)
}

func VerifyRegistrationProof(message, proof, enrollmentKey []byte) bool {
	mac := hmac.New(sha512.New, enrollmentKey)
	_, _ = mac.Write(message)
	return hmac.Equal(proof, mac.Sum(nil)[:sha512.Size256])
}

func decodeBase64(value string, expectedSize int) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(decoded) != expectedSize {
		return nil, errors.New("invalid encoded value length")
	}
	return decoded, nil
}

func responseError(code int, message string) mvc.Result {
	return mvc.Response{Code: code, Object: iris.Map{"error": message}}
}

func (c *DeviceController) PostRegister() mvc.Result {
	var form api.DeviceRegistrationForm
	if err := c.Ctx.ReadJSON(&form); err != nil {
		return responseError(iris.StatusBadRequest, "invalid registration request")
	}
	if form.RustdeskId == "" || form.Uuid == "" {
		return responseError(iris.StatusBadRequest, "missing device identity")
	}
	serverTime := time.Now().Unix()
	if form.Timestamp <= 0 || math.Abs(float64(serverTime-form.Timestamp)) > float64(deviceClockSkewSeconds) {
		return mvc.Response{Code: iris.StatusUnauthorized, Object: iris.Map{"accepted": false, "server_time": serverTime}}
	}
	publicKey, err := decodeBase64(form.PublicKey, ed25519.PublicKeySize)
	if err != nil {
		return responseError(iris.StatusBadRequest, "invalid device public key")
	}
	signature, err := decodeBase64(form.Signature, ed25519.SignatureSize)
	if err != nil {
		return responseError(iris.StatusBadRequest, "invalid device signature")
	}
	proof, err := decodeBase64(form.EnrollmentProof, sha512.Size256)
	if err != nil {
		return responseError(iris.StatusBadRequest, "invalid enrollment proof")
	}
	if c.ServerConfig == nil {
		return responseError(iris.StatusServiceUnavailable, "device enrollment is not configured")
	}
	enrollmentKeyRaw, err := base64.StdEncoding.DecodeString(c.ServerConfig.DeviceEnrollmentKey)
	if err != nil || len(enrollmentKeyRaw) != 32 {
		return responseError(iris.StatusServiceUnavailable, "device enrollment is not configured")
	}
	message := BuildRegistrationMessage(form.RustdeskId, form.Uuid, publicKey, form.Timestamp)
	if !VerifyRegistrationProof(message, proof, enrollmentKeyRaw) || !ed25519.Verify(publicKey, message, signature) {
		return responseError(iris.StatusUnauthorized, "device registration rejected")
	}

	device := model.Device{}
	hasDevice, err := c.Db.Where("rustdesk_id = ?", form.RustdeskId).Get(&device)
	if err != nil {
		return responseError(iris.StatusInternalServerError, "failed to resolve device")
	}
	if !hasDevice {
		device.RustdeskId = form.RustdeskId
		device.Uuid = form.Uuid
		device.IsOnline = true
		if _, err = c.Db.Insert(&device); err != nil {
			return responseError(iris.StatusInternalServerError, "failed to create device")
		}
	}

	credential := model.DeviceCredential{}
	hasCredential, err := c.Db.Where("device_id = ?", device.Id).Get(&credential)
	if err != nil {
		return responseError(iris.StatusInternalServerError, "failed to resolve device credential")
	}
	if hasCredential && !credential.Enabled {
		return responseError(iris.StatusForbidden, "device credential disabled")
	}
	if device.Uuid != "" && device.Uuid != form.Uuid {
		if hasCredential {
			return responseError(iris.StatusConflict, "device identity conflict")
		}
		device.Uuid = form.Uuid
		if _, err = c.Db.ID(device.Id).Cols("uuid").Update(&device); err != nil {
			return responseError(iris.StatusInternalServerError, "failed to claim device")
		}
	}
	if !hasCredential {
		credential.DeviceId = device.Id
		credential.PublicKey = form.PublicKey
		credential.Enabled = true
		if _, err = c.Db.Insert(&credential); err != nil {
			return responseError(iris.StatusInternalServerError, "failed to create device credential")
		}
	} else if credential.PublicKey != form.PublicKey {
		credential.PublicKey = form.PublicKey
		credential.LastSeq = 0
		if _, err = c.Db.ID(credential.Id).Cols("public_key", "last_seq").Update(&credential); err != nil {
			return responseError(iris.StatusInternalServerError, "failed to rotate device credential")
		}
	}

	return mvc.Response{Object: iris.Map{
		"accepted":      true,
		"device_id":     device.Id,
		"last_sequence": credential.LastSeq,
	}}
}

func (c *DeviceController) authenticateRequest(form *api.SignedDeviceRequestForm) ([]byte, *model.Device, mvc.Result) {
	if form.DeviceId <= 0 || form.Sequence <= 0 {
		return nil, nil, responseError(iris.StatusBadRequest, "invalid device request")
	}
	payload, err := base64.StdEncoding.DecodeString(form.Payload)
	if err != nil || len(payload) == 0 || len(payload) > 256*1024 {
		return nil, nil, responseError(iris.StatusBadRequest, "invalid device payload")
	}
	signature, err := decodeBase64(form.Signature, ed25519.SignatureSize)
	if err != nil {
		return nil, nil, responseError(iris.StatusBadRequest, "invalid device signature")
	}
	credential := model.DeviceCredential{}
	has, err := c.Db.Where("device_id = ? AND enabled = ?", form.DeviceId, true).Get(&credential)
	if err != nil || !has {
		return nil, nil, responseError(iris.StatusUnauthorized, "unknown device credential")
	}
	publicKey, err := decodeBase64(credential.PublicKey, ed25519.PublicKeySize)
	if err != nil || !ed25519.Verify(publicKey, BuildDeviceRequestMessage(form.DeviceId, form.Sequence, payload), signature) {
		return nil, nil, responseError(iris.StatusUnauthorized, "device signature rejected")
	}
	affected, err := c.Db.Where("device_id = ? AND last_seq < ?", form.DeviceId, form.Sequence).
		Cols("last_seq").Update(&model.DeviceCredential{LastSeq: form.Sequence})
	if err != nil {
		return nil, nil, responseError(iris.StatusInternalServerError, "failed to update device sequence")
	}
	if affected != 1 {
		return nil, nil, responseError(iris.StatusConflict, "device request replayed")
	}
	device := model.Device{}
	has, err = c.Db.ID(form.DeviceId).Get(&device)
	if err != nil || !has {
		return nil, nil, responseError(iris.StatusUnauthorized, "registered device not found")
	}
	return payload, &device, nil
}

func (c *DeviceController) PostHeartbeat() mvc.Result {
	var request api.SignedDeviceRequestForm
	if err := c.Ctx.ReadJSON(&request); err != nil {
		return responseError(iris.StatusBadRequest, "invalid signed request")
	}
	payload, device, failure := c.authenticateRequest(&request)
	if failure != nil {
		return failure
	}
	var form api.HeartbeatForm
	if err := json.Unmarshal(payload, &form); err != nil {
		return responseError(iris.StatusBadRequest, "invalid heartbeat payload")
	}
	if ResolveHeartbeatRustdeskID(form.RustdeskId, form.Uuid) != device.RustdeskId || (device.Uuid != "" && form.Uuid != device.Uuid) {
		return responseError(iris.StatusConflict, "heartbeat identity mismatch")
	}
	if _, err := c.Db.ID(device.Id).Cols("is_online", "conns").Update(&model.Device{
		IsOnline: true,
		Conns:    len(form.Conns),
	}); err != nil {
		return responseError(iris.StatusInternalServerError, "failed to update heartbeat")
	}
	return mvc.Response{Object: iris.Map{
		"modified_at": time.Now().Unix(),
		"strategy":    versionhelper.ResolveCapability(NormalizeReportedVersion(form.Version, form.Ver)),
	}}
}

func (c *DeviceController) PostSysinfo() mvc.Result {
	var request api.SignedDeviceRequestForm
	if err := c.Ctx.ReadJSON(&request); err != nil {
		return responseError(iris.StatusBadRequest, "invalid signed request")
	}
	payload, device, failure := c.authenticateRequest(&request)
	if failure != nil {
		return failure
	}
	var form api.DeviceForm
	if err := json.Unmarshal(payload, &form); err != nil {
		return responseError(iris.StatusBadRequest, "invalid sysinfo payload")
	}
	if ResolveHeartbeatRustdeskID(form.RustdeskId, form.Uuid) != device.RustdeskId || (device.Uuid != "" && form.Uuid != device.Uuid) {
		return responseError(iris.StatusConflict, "sysinfo identity mismatch")
	}
	device.Cpu = form.Cpu
	device.Hostname = form.Hostname
	device.Memory = form.Memory
	device.Os = form.Os
	device.Username = form.Username
	device.Version = NormalizeReportedVersion(form.Version, form.Ver)
	if _, err := c.Db.ID(device.Id).Update(device); err != nil {
		return responseError(iris.StatusInternalServerError, "failed to update sysinfo")
	}
	return mvc.Response{Text: "SYSINFO_UPDATED"}
}
