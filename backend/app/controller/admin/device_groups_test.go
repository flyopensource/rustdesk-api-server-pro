package admin

import (
	"fmt"
	"rustdesk-api-server-pro/app/model"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func TestBumpScopeDevices(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:policy-bump-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(new(model.Device), new(model.DeviceGroupMember)); err != nil {
		t.Fatal(err)
	}
	devices := []model.Device{{RustdeskId: "1"}, {RustdeskId: "2"}}
	for i := range devices {
		if _, err = db.Insert(&devices[i]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Insert(&model.DeviceGroupMember{GroupId: 9, DeviceId: devices[0].Id}); err != nil {
		t.Fatal(err)
	}
	if err = bumpScopeDevices(db, model.DevicePolicyScopeGroup, 9); err != nil {
		t.Fatal(err)
	}
	if err = bumpScopeDevices(db, model.DevicePolicyScopeDevice, devices[1].Id); err != nil {
		t.Fatal(err)
	}
	for _, device := range devices {
		loaded := model.Device{}
		if found, loadErr := db.ID(device.Id).Get(&loaded); loadErr != nil || !found {
			t.Fatalf("load device: %v", loadErr)
		}
		if loaded.PolicyRevision != 1 {
			t.Fatalf("device %d revision = %d", device.Id, loaded.PolicyRevision)
		}
	}
}
