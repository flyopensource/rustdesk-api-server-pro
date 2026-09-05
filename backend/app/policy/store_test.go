package policy

import (
	"encoding/json"
	"fmt"
	"rustdesk-api-server-pro/app/model"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func TestResolveForDeviceLoadsPublishedHierarchy(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:policy-store-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(new(model.Device), new(model.DeviceGroup), new(model.DeviceGroupMember), new(model.ManagedDevicePolicy)); err != nil {
		t.Fatal(err)
	}
	device := model.Device{RustdeskId: "123", RootCommand: "auto"}
	if _, err = db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	group := model.DeviceGroup{Name: "test", Priority: 20, Enabled: true}
	if _, err = db.Insert(&group); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Insert(&model.DeviceGroupMember{GroupId: group.Id, DeviceId: device.Id}); err != nil {
		t.Fatal(err)
	}
	insertPolicy := func(scope string, id int, document Document) {
		encoded, marshalErr := json.Marshal(document)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if _, insertErr := db.Insert(&model.ManagedDevicePolicy{ScopeType: scope, ScopeId: id, Document: string(encoded), Revision: 1, Enabled: true}); insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	insertPolicy(model.DevicePolicyScopeGlobal, 0, Document{Unattended: Unattended{Enabled: boolPtr(true)}})
	insertPolicy(model.DevicePolicyScopeGroup, group.Id, Document{Unattended: Unattended{RootCommand: stringPtr("su")}})
	insertPolicy(model.DevicePolicyScopeDevice, device.Id, Document{Unattended: Unattended{Enabled: boolPtr(false)}})

	resolution, err := ResolveForDevice(db, &device)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Effective.UnattendedEnabled || resolution.Effective.RootCommand != "su" {
		t.Fatalf("unexpected effective policy: %+v", resolution.Effective)
	}
	wantLayers := []string{"global", fmt.Sprintf("group:%d", group.Id), "device"}
	if fmt.Sprint(resolution.Layers) != fmt.Sprint(wantLayers) {
		t.Fatalf("unexpected layers: %v", resolution.Layers)
	}
}

func TestResolveForDeviceFallsBackToLegacyFields(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:policy-legacy-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(new(model.Device), new(model.DeviceGroup), new(model.DeviceGroupMember), new(model.ManagedDevicePolicy)); err != nil {
		t.Fatal(err)
	}
	device := model.Device{UnattendedEnabled: true, RootCommand: "testsu", ProfileEnabled: true, ProfileIdServer: "legacy.example"}
	resolution, err := ResolveForDevice(db, &device)
	if err != nil {
		t.Fatal(err)
	}
	if !resolution.Effective.UnattendedEnabled || resolution.Effective.RootCommand != "testsu" || resolution.Effective.IDServer != "legacy.example" {
		t.Fatalf("legacy policy was not preserved: %+v", resolution.Effective)
	}
}
