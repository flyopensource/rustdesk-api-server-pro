package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"rustdesk-api-server-pro/app/model"
	devicepolicy "rustdesk-api-server-pro/app/policy"
	"rustdesk-api-server-pro/config"
	"testing"

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
	device := model.Device{RustdeskId: "123", Uuid: "uuid", PolicyRevision: 42, UnattendedEnabled: false}
	effective := devicepolicy.Effective{UnattendedEnabled: true, RootCommand: "su", ProfileEnabled: true, IDServer: "group.example", Key: "secret"}
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
	if policy.Revision != 42 || !policy.Android.Unattended.Enabled || policy.Android.Unattended.RootCommand != "su" || policy.ServerProfile.IDServer != "group.example" || policy.ServerProfile.Key != "secret" {
		t.Fatalf("envelope did not use resolved policy: %+v", policy)
	}
}
