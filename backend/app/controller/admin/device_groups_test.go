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

func TestValidateDeviceGroupForm(t *testing.T) {
	valid := deviceGroupForm{Name: "Kiosks", ProfileId: 1, RootCommand: "auto"}
	if err := validateDeviceGroupForm(&valid); err != nil {
		t.Fatal(err)
	}
	defaultRoot := deviceGroupForm{Name: "Default", ProfileId: 1}
	if err := validateDeviceGroupForm(&defaultRoot); err != nil || defaultRoot.RootCommand != "auto" {
		t.Fatal("empty Root executor was not normalized to auto", err)
	}
	for _, invalid := range []deviceGroupForm{
		{Name: "No profile", ProfileId: 0, RootCommand: "auto"},
		{Name: "Bad Root", ProfileId: 1, RootCommand: "su -c"},
		{Name: "Disabled default", Enabled: false, IsDefault: true, ProfileId: 1, RootCommand: "auto"},
	} {
		if err := validateDeviceGroupForm(&invalid); err == nil {
			t.Fatalf("invalid device group accepted: %+v", invalid)
		}
	}
}
