package api_test

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	controller "rustdesk-api-server-pro/app/controller/api"
	"testing"
)

func TestRegistrationProofAndSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	message := controller.BuildRegistrationMessage("123456789", "device-uuid", publicKey, 1788451200)
	signature := ed25519.Sign(privateKey, message)
	if !ed25519.Verify(publicKey, message, signature) {
		t.Fatal("device signature should verify")
	}

	enrollmentKey := []byte("01234567890123456789012345678901")
	mac := hmac.New(sha512.New, enrollmentKey)
	_, _ = mac.Write(message)
	proof := mac.Sum(nil)[:sha512.Size256]
	if !controller.VerifyRegistrationProof(message, proof, enrollmentKey) {
		t.Fatal("enrollment proof should verify")
	}
	message[0] ^= 1
	if controller.VerifyRegistrationProof(message, proof, enrollmentKey) {
		t.Fatal("modified registration message must be rejected")
	}
}

func TestDeviceRequestMessageBindsPayloadAndSequence(t *testing.T) {
	payload := []byte(`{"id":"123456789","uuid":"device-uuid"}`)
	message := controller.BuildDeviceRequestMessage(7, 1, payload)
	if string(message) == string(controller.BuildDeviceRequestMessage(7, 2, payload)) {
		t.Fatal("sequence must be signed")
	}
	if string(message) == string(controller.BuildDeviceRequestMessage(7, 1, append(payload, ' '))) {
		t.Fatal("payload must be signed")
	}
}
