package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math"
	deviceform "rustdesk-api-server-pro/app/form/api"
	"rustdesk-api-server-pro/app/model"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
)

const desktopRegistrationVersion = 1

func DesktopTokenHash(secret string) string {
	digest := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(digest[:])
}

func ParseDesktopEnrollmentToken(value string) (string, string, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != "rud1" || len(parts[1]) < 16 || len(parts[2]) < 32 {
		return "", "", errors.New("invalid desktop enrollment token")
	}
	if _, err := base64.RawURLEncoding.DecodeString(parts[1]); err != nil {
		return "", "", errors.New("invalid desktop enrollment selector")
	}
	if secret, err := base64.RawURLEncoding.DecodeString(parts[2]); err != nil || len(secret) < 24 {
		return "", "", errors.New("invalid desktop enrollment secret")
	}
	return parts[1], parts[2], nil
}

func GenerateDesktopEnrollmentToken() (string, string, string, error) {
	selectorBytes := make([]byte, 12)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(selectorBytes); err != nil {
		return "", "", "", err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", "", err
	}
	selector := base64.RawURLEncoding.EncodeToString(selectorBytes)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	return "rud1." + selector + "." + secret, selector, DesktopTokenHash(secret), nil
}

func BuildDesktopRegistrationMessage(form *deviceform.DesktopDeviceRegistrationForm, signPublicKey, boxPublicKey []byte) []byte {
	message := append([]byte{}, []byte("RUD-DESKTOP-REGISTER\x00")...)
	for _, value := range []string{
		strconv.Itoa(form.Version), form.RequestId, form.RustdeskId, form.Uuid,
		form.Hostname, form.Os, form.Arch, form.ClientVersion,
	} {
		message = appendField(message, value)
	}
	message = append(message, signPublicKey...)
	message = append(message, 0)
	message = append(message, boxPublicKey...)
	message = append(message, 0)
	message = appendField(message, strconv.FormatInt(form.Timestamp, 10))
	tokenDigest := sha256.Sum256([]byte(form.Token))
	return append(message, tokenDigest[:]...)
}

func desktopPolicyVerifyKey(seedValue string) (string, error) {
	seed, err := base64.StdEncoding.DecodeString(seedValue)
	if err != nil || len(seed) != ed25519.SeedSize {
		return "", errors.New("provisioning signing key is not configured")
	}
	publicKey := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	return base64.StdEncoding.EncodeToString(publicKey), nil
}

func desktopRegistrationDigest(message, signature []byte) string {
	digestInput := make([]byte, 0, len(message)+len(signature))
	digestInput = append(digestInput, message...)
	digestInput = append(digestInput, signature...)
	digest := sha256.Sum256(digestInput)
	return hex.EncodeToString(digest[:])
}

func (c *DeviceController) PostRegisterDesktop() mvc.Result {
	var form deviceform.DesktopDeviceRegistrationForm
	if err := c.Ctx.ReadJSON(&form); err != nil {
		return responseError(iris.StatusBadRequest, "invalid_registration_request")
	}
	if form.Version != desktopRegistrationVersion || form.RequestId == "" || len(form.RequestId) > 128 ||
		form.RustdeskId == "" || form.Uuid == "" || len(form.Hostname) > 255 || len(form.Os) > 255 ||
		len(form.Arch) > 32 || len(form.ClientVersion) > 255 {
		return responseError(iris.StatusBadRequest, "invalid_registration_request")
	}
	serverTime := time.Now().Unix()
	clockValid := form.Timestamp > 0 && math.Abs(float64(serverTime-form.Timestamp)) <= float64(deviceClockSkewSeconds)
	selector, secret, err := ParseDesktopEnrollmentToken(form.Token)
	if err != nil {
		return responseError(iris.StatusUnauthorized, "token_invalid")
	}
	signPublicKey, err := decodeBase64(form.SignPublicKey, ed25519.PublicKeySize)
	if err != nil {
		return responseError(iris.StatusBadRequest, "invalid_device_public_key")
	}
	boxPublicKey, err := decodeBase64(form.BoxPublicKey, 32)
	if err != nil {
		return responseError(iris.StatusBadRequest, "invalid_device_box_key")
	}
	signature, err := decodeBase64(form.Signature, ed25519.SignatureSize)
	if err != nil {
		return responseError(iris.StatusBadRequest, "invalid_device_signature")
	}
	message := BuildDesktopRegistrationMessage(&form, signPublicKey, boxPublicKey)
	if !ed25519.Verify(signPublicKey, message, signature) {
		return responseError(iris.StatusUnauthorized, "device_signature_rejected")
	}
	if c.ServerConfig == nil {
		return responseError(iris.StatusServiceUnavailable, "device_enrollment_not_configured")
	}
	verifyPublicKey, err := desktopPolicyVerifyKey(c.ServerConfig.ProvisioningSignSeed)
	if err != nil {
		return responseError(iris.StatusServiceUnavailable, err.Error())
	}
	keyID := c.ServerConfig.ProvisioningKeyId
	if keyID == "" {
		keyID = "android-v1"
	}
	requestDigest := desktopRegistrationDigest(message, signature)

	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return responseError(iris.StatusInternalServerError, "registration_transaction_failed")
	}
	defer session.Rollback()
	token := model.DesktopEnrollmentToken{}
	found, err := session.Where("selector = ?", selector).Get(&token)
	if err != nil || !found || subtle.ConstantTimeCompare([]byte(token.SecretHash), []byte(DesktopTokenHash(secret))) != 1 {
		return responseError(iris.StatusUnauthorized, "token_invalid")
	}
	if token.ConsumedDeviceId > 0 {
		if token.RequestId != form.RequestId || token.RequestDigest != requestDigest {
			return responseError(iris.StatusConflict, "token_consumed")
		}
		credential := model.DeviceCredential{}
		if hasCredential, loadErr := session.Where("device_id = ?", token.ConsumedDeviceId).Get(&credential); loadErr != nil || !hasCredential {
			return responseError(iris.StatusConflict, "registration_result_missing")
		}
		return mvc.Response{Object: iris.Map{
			"accepted": true, "device_id": token.ConsumedDeviceId, "last_sequence": credential.LastSeq,
			"policy_verify_key_id": keyID, "policy_verify_public_key": verifyPublicKey, "server_time": serverTime,
		}}
	}
	if !token.RevokedAt.IsZero() {
		return responseError(iris.StatusUnauthorized, "token_revoked")
	}
	if !token.ExpiresAt.IsZero() && !time.Now().Before(token.ExpiresAt) {
		return responseError(iris.StatusUnauthorized, "token_expired")
	}
	if !clockValid {
		return mvc.Response{Code: iris.StatusUnauthorized, Object: iris.Map{"accepted": false, "error": "clock_skew", "server_time": serverTime}}
	}

	device := model.Device{}
	hasDevice, err := session.Where("rustdesk_id = ?", form.RustdeskId).Get(&device)
	if err != nil {
		return responseError(iris.StatusInternalServerError, "failed_to_resolve_device")
	}
	if hasDevice && device.Disabled {
		return responseError(iris.StatusForbidden, "credential_disabled")
	}
	if hasDevice && device.Uuid != "" && device.Uuid != form.Uuid {
		return responseError(iris.StatusConflict, "device_identity_conflict")
	}
	if hasDevice {
		credential := model.DeviceCredential{}
		if hasCredential, loadErr := session.Where("device_id = ?", device.Id).Get(&credential); loadErr != nil {
			return responseError(iris.StatusInternalServerError, "failed_to_resolve_device_credential")
		} else if hasCredential {
			if !credential.Enabled {
				return responseError(iris.StatusForbidden, "credential_disabled")
			}
			return responseError(iris.StatusConflict, "device_identity_conflict")
		}
		device.Uuid = form.Uuid
		device.Hostname = form.Hostname
		device.Os = form.Os
		device.Platform = "desktop"
		device.Arch = form.Arch
		device.Version = form.ClientVersion
		if token.GroupId > 0 {
			device.StrategyGroupId = token.GroupId
		}
		if _, err = session.ID(device.Id).Cols("uuid", "hostname", "os", "platform", "arch", "version", "strategy_group_id").Update(&device); err != nil {
			return responseError(iris.StatusInternalServerError, "failed_to_update_device")
		}
	} else {
		device = model.Device{
			RustdeskId: form.RustdeskId, Uuid: form.Uuid, Hostname: form.Hostname, Os: form.Os,
			Platform: "desktop", Arch: form.Arch, Version: form.ClientVersion,
			StrategyGroupId: token.GroupId, IsOnline: true,
		}
		if _, err = session.Insert(&device); err != nil {
			return responseError(iris.StatusInternalServerError, "failed_to_create_device")
		}
	}
	credential := model.DeviceCredential{
		DeviceId: device.Id, PublicKey: form.SignPublicKey, BoxPublicKey: form.BoxPublicKey,
		RegistrationType: "desktop_token", Enabled: true,
	}
	if _, err = session.Insert(&credential); err != nil {
		return responseError(iris.StatusConflict, "device_identity_conflict")
	}
	now := time.Now()
	updated, err := session.ID(token.Id).
		Where("consumed_device_id = ?", 0).
		Cols("consumed_at", "consumed_device_id", "request_id", "request_digest").
		Update(&model.DesktopEnrollmentToken{
			ConsumedAt: now, ConsumedDeviceId: device.Id, RequestId: form.RequestId, RequestDigest: requestDigest,
		})
	if err != nil || updated != 1 {
		return responseError(iris.StatusConflict, "token_consumed")
	}
	if err = session.Commit(); err != nil {
		return responseError(iris.StatusInternalServerError, "registration_commit_failed")
	}
	return mvc.Response{Object: iris.Map{
		"accepted": true, "device_id": device.Id, "last_sequence": int64(0),
		"policy_verify_key_id": keyID, "policy_verify_public_key": verifyPublicKey, "server_time": serverTime,
	}}
}
