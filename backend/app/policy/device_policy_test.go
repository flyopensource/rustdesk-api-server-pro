package policy

import "testing"

func boolPtr(value bool) *bool       { return &value }
func stringPtr(value string) *string { return &value }

func TestResolvePolicyHierarchy(t *testing.T) {
	global := Document{
		Unattended:    Unattended{Enabled: boolPtr(true), RootCommand: stringPtr("auto")},
		ServerProfile: ServerProfile{Enabled: boolPtr(true), IDServer: stringPtr("global.example")},
	}
	groups := []GroupDocument{
		{GroupID: 20, Priority: 10, Document: Document{Unattended: Unattended{RootCommand: stringPtr("su")}, ServerProfile: ServerProfile{IDServer: stringPtr("later-group.example")}}},
		{GroupID: 10, Priority: 10, Document: Document{ServerProfile: ServerProfile{IDServer: stringPtr("group.example")}}},
	}
	device := Document{
		Unattended:    Unattended{Enabled: boolPtr(false)},
		ServerProfile: ServerProfile{RelayServer: stringPtr("relay.example"), Key: stringPtr("")},
	}

	effective, err := Resolve(&global, groups, &device)
	if err != nil {
		t.Fatal(err)
	}
	if effective.UnattendedEnabled || effective.RootCommand != "su" {
		t.Fatalf("unexpected unattended policy: %+v", effective)
	}
	if effective.IDServer != "later-group.example" || effective.RelayServer != "relay.example" || effective.Key != "" {
		t.Fatalf("unexpected server profile: %+v", effective)
	}
}

func TestResolveDistinguishesInheritanceFromExplicitDisable(t *testing.T) {
	global := Document{Unattended: Unattended{Enabled: boolPtr(true)}}
	device := Document{Unattended: Unattended{Enabled: boolPtr(false)}}

	inherited, err := Resolve(&global, nil, nil)
	if err != nil || !inherited.UnattendedEnabled {
		t.Fatalf("expected inherited enable: %+v, %v", inherited, err)
	}
	disabled, err := Resolve(&global, nil, &device)
	if err != nil || disabled.UnattendedEnabled {
		t.Fatalf("expected explicit disable: %+v, %v", disabled, err)
	}
}

func TestResolveRejectsIncompleteEnabledProfile(t *testing.T) {
	_, err := Resolve(&Document{ServerProfile: ServerProfile{Enabled: boolPtr(true)}}, nil, nil)
	if err == nil {
		t.Fatal("expected incomplete enabled profile to be rejected")
	}
}
