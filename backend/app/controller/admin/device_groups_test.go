package admin

import "testing"

func TestValidateServerProfileForm(t *testing.T) {
	valid := serverProfileForm{Name: "Primary", IDServer: "id.example.com", RelayServer: "relay.example.com", Enabled: true}
	if err := validateServerProfileForm(&valid); err != nil {
		t.Fatal(err)
	}
	invalid := serverProfileForm{Name: "Invalid", IDServer: "id.example.com extra", Enabled: true}
	if err := validateServerProfileForm(&invalid); err == nil {
		t.Fatal("expected whitespace in ID Server to be rejected")
	}
}
