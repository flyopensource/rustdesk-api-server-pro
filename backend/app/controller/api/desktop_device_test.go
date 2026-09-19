package api

import (
	"crypto/sha256"
	"encoding/hex"
	deviceform "rustdesk-api-server-pro/app/form/api"
	"testing"
)

func TestDesktopRegistrationMessageMatchesRustVector(t *testing.T) {
	form := deviceform.DesktopDeviceRegistrationForm{
		Version: 1, RequestId: "request", Token: "rud1.selector.secret",
		RustdeskId: "123", Uuid: "uuid", Hostname: "host", Os: "linux",
		Arch: "x86_64", ClientVersion: "1.4.6", Timestamp: 1700000000,
	}
	signPublicKey := make([]byte, 32)
	boxPublicKey := make([]byte, 32)
	for index := range signPublicKey {
		signPublicKey[index] = 1
		boxPublicKey[index] = 2
	}
	digest := sha256.Sum256(BuildDesktopRegistrationMessage(&form, signPublicKey, boxPublicKey))
	if actual := hex.EncodeToString(digest[:]); actual != "d65d539941331f2eededeb5c0daf5da1ce3f0ab2bb7ebb4fc8573dbaf0946728" {
		t.Fatalf("desktop registration vector mismatch: %s", actual)
	}
}

func TestDesktopEnrollmentTokenRoundTrip(t *testing.T) {
	plain, selector, secretHash, err := GenerateDesktopEnrollmentToken()
	if err != nil {
		t.Fatal(err)
	}
	parsedSelector, secret, err := ParseDesktopEnrollmentToken(plain)
	if err != nil {
		t.Fatal(err)
	}
	if parsedSelector != selector || DesktopTokenHash(secret) != secretHash {
		t.Fatal("desktop enrollment token did not round trip")
	}
}
