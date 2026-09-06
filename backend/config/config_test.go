package config

import "testing"

func TestApplyEnvironmentOverridesSignKey(t *testing.T) {
	t.Setenv("RUD_API_SIGN_KEY", "0123456789abcdef0123456789abcdef")
	cfg := GetDefaultServerConfig()

	applyEnvironment(cfg)

	if cfg.SignKey != "0123456789abcdef0123456789abcdef" {
		t.Fatal("environment sign key was not applied")
	}
	if err := cfg.ValidateForServer(); err != nil {
		t.Fatalf("valid sign key rejected: %v", err)
	}
}

func TestValidateForServerRejectsMissingOrShortSignKey(t *testing.T) {
	for _, signKey := range []string{"", "too-short"} {
		cfg := GetDefaultServerConfig()
		cfg.SignKey = signKey
		if err := cfg.ValidateForServer(); err == nil {
			t.Fatalf("sign key %q should be rejected", signKey)
		}
	}
}
