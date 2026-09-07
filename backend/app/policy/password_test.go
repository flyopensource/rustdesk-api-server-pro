package policy

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestProfilePasswordEncryptionRoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	encrypted, err := EncryptPassword("secret-password", key)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "secret-password" || strings.Contains(encrypted, "secret-password") {
		t.Fatal("password was not encrypted")
	}
	decrypted, err := DecryptPassword(encrypted, key)
	if err != nil || decrypted != "secret-password" {
		t.Fatalf("password round trip failed: %q, %v", decrypted, err)
	}
}
