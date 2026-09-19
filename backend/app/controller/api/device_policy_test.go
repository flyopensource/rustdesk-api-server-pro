package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"rustdesk-api-server-pro/app/model"
	devicepolicy "rustdesk-api-server-pro/app/policy"
	"rustdesk-api-server-pro/config"
	"testing"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

func TestBuildPolicyEnvelopeUsesResolvedPolicy(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	secret := [32]byte{}
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	for i := range secret {
		secret[i] = byte(i + 2)
	}
	cfg := config.GetDefaultServerConfig()
	cfg.ProvisioningSignSeed = base64.StdEncoding.EncodeToString(seed)
	cfg.ProvisioningSecretKey = base64.StdEncoding.EncodeToString(secret[:])
	cfg.ProvisioningKeyId = "test"
	device := model.Device{RustdeskId: "123", Uuid: "uuid"}
	password, err := devicepolicy.EncryptPassword("private-password", cfg.ProvisioningSecretKey)
	if err != nil {
		t.Fatal(err)
	}
	effective := devicepolicy.Effective{
		Revision: 42, UnattendedEnabled: true, RootCommand: "/system/xbin/su", ProfileEnabled: true,
		PasswordCiphertext: password,
		Profile:            devicepolicy.ServerProfile{IDServer: "group.example", ServerKey: "server-key"},
	}
	encoded, err := buildPolicyEnvelope(&device, cfg, effective)
	if err != nil {
		t.Fatal(err)
	}
	envelopeJSON, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var envelope policyEnvelope
	if err = json.Unmarshal(envelopeJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	nonceRaw, _ := base64.StdEncoding.DecodeString(envelope.Nonce)
	ciphertext, _ := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	var nonce [24]byte
	copy(nonce[:], nonceRaw)
	plaintext, ok := secretbox.Open(nil, ciphertext, &nonce, &secret)
	if !ok {
		t.Fatal("failed to decrypt policy")
	}
	var policy unattendedPolicy
	if err = json.Unmarshal(plaintext, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Revision != 42 || !policy.Android.Unattended.Enabled || policy.Android.Unattended.RootCommand != "/system/xbin/su" || policy.ServerProfile.IDServer != "group.example" || policy.ServerProfile.Key != "server-key" || policy.ServerProfile.PermanentPassword != "private-password" {
		t.Fatalf("envelope did not use resolved policy: %+v", policy)
	}

	effective.UnattendedEnabled = false
	encoded, err = buildPolicyEnvelope(&device, cfg, effective)
	if err != nil {
		t.Fatal(err)
	}
	envelopeJSON, _ = base64.StdEncoding.DecodeString(encoded)
	if err = json.Unmarshal(envelopeJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	nonceRaw, _ = base64.StdEncoding.DecodeString(envelope.Nonce)
	ciphertext, _ = base64.StdEncoding.DecodeString(envelope.Ciphertext)
	copy(nonce[:], nonceRaw)
	plaintext, ok = secretbox.Open(nil, ciphertext, &nonce, &secret)
	if !ok {
		t.Fatal("failed to decrypt disabled policy")
	}
	if err = json.Unmarshal(plaintext, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.ServerProfile.PermanentPassword != "" {
		t.Fatal("disabled unattended policy still delivered the device group password")
	}
}

func TestBuildDesktopPolicyEnvelopeEncryptsOnlyDesktopPolicy(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 9)
	}
	boxPublicKey, boxSecretKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.GetDefaultServerConfig()
	cfg.ProvisioningSignSeed = base64.StdEncoding.EncodeToString(seed)
	cfg.ProvisioningSecretKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	cfg.ProvisioningKeyId = "desktop-test"
	password, err := devicepolicy.EncryptPassword("desktop-password", cfg.ProvisioningSecretKey)
	if err != nil {
		t.Fatal(err)
	}
	device := model.Device{Id: 17, RustdeskId: "123456789", Uuid: "desktop-uuid"}
	credential := model.DeviceCredential{BoxPublicKey: base64.StdEncoding.EncodeToString(boxPublicKey[:])}
	effective := devicepolicy.Effective{
		Revision: 88, GroupID: 3, UnattendedEnabled: true, PasswordCiphertext: password,
		ProfileEnabled: true,
		Profile: devicepolicy.ServerProfile{
			IDServer: "id.example.com", RelayServer: "relay.example.com", ServerKey: "server-key",
		},
	}
	encoded, err := buildDesktopPolicyEnvelope(&device, &credential, cfg, effective)
	if err != nil {
		t.Fatal(err)
	}
	envelopeJSON, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var envelope desktopPolicyEnvelope
	if err = json.Unmarshal(envelopeJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	if !ed25519.Verify(publicKey, BuildDesktopPolicySignatureMessage(
		envelope.Version, envelope.Purpose, envelope.KeyID, envelope.DeviceID, envelope.Revision, ciphertext,
	), mustDecodeBase64(t, envelope.Signature)) {
		t.Fatal("desktop policy signature is invalid")
	}
	plaintext, ok := box.OpenAnonymous(nil, ciphertext, boxPublicKey, boxSecretKey)
	if !ok {
		t.Fatal("failed to decrypt desktop policy")
	}
	var policy desktopPolicy
	if err = json.Unmarshal(plaintext, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Revision != 88 || policy.Target.DeviceID != 17 ||
		policy.Desktop.Unattended.PasswordAction != "set" ||
		policy.Desktop.Unattended.PermanentPassword != "desktop-password" ||
		!policy.Desktop.ServerProfile.Enabled ||
		policy.Desktop.ServerProfile.IDServer != "id.example.com" ||
		policy.Desktop.ServerProfile.RelayServer != "relay.example.com" ||
		policy.Desktop.ServerProfile.Key != "server-key" {
		t.Fatalf("unexpected desktop policy: %+v", policy)
	}
	if string(plaintext) == "" || json.Valid(plaintext) == false {
		t.Fatal("desktop policy plaintext is not valid JSON")
	}
	if bytes.Contains(plaintext, []byte("root_command")) {
		t.Fatal("desktop policy leaked Android root configuration")
	}

	effective.ProfileEnabled = false
	encoded, err = buildDesktopPolicyEnvelope(&device, &credential, cfg, effective)
	if err != nil {
		t.Fatal(err)
	}
	envelopeJSON, _ = base64.StdEncoding.DecodeString(encoded)
	if err = json.Unmarshal(envelopeJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	ciphertext, _ = base64.StdEncoding.DecodeString(envelope.Ciphertext)
	plaintext, ok = box.OpenAnonymous(nil, ciphertext, boxPublicKey, boxSecretKey)
	if !ok || json.Unmarshal(plaintext, &policy) != nil {
		t.Fatal("failed to decode disabled desktop profile policy")
	}
	if policy.Desktop.ServerProfile.Enabled || policy.Desktop.ServerProfile.IDServer != "" ||
		policy.Desktop.ServerProfile.RelayServer != "" || policy.Desktop.ServerProfile.Key != "" {
		t.Fatalf("disabled desktop profile leaked server fields: %+v", policy.Desktop.ServerProfile)
	}
}

func mustDecodeBase64(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
