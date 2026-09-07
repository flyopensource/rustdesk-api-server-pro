package policy

import (
	"fmt"
	"rustdesk-api-server-pro/app/model"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func newPolicyTestDB(t *testing.T) *xorm.Engine {
	t.Helper()
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:strategy-store-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Sync(new(model.Device), new(model.DeviceGroup), new(model.ServerProfile), new(model.StrategyState)); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestResolveForDeviceUsesDeviceGroupGlobalPrecedence(t *testing.T) {
	db := newPolicyTestDB(t)
	profiles := []model.ServerProfile{
		{Name: "global", IdServer: "global.example", Enabled: true},
		{Name: "group", IdServer: "group.example", Enabled: true},
		{Name: "device", IdServer: "device.example", Enabled: true},
	}
	for i := range profiles {
		if _, err := db.Insert(&profiles[i]); err != nil {
			t.Fatal(err)
		}
	}
	group := model.DeviceGroup{Name: "kiosks", Enabled: true, ProfileId: profiles[1].Id}
	if _, err := db.Insert(&group); err != nil {
		t.Fatal(err)
	}
	device := model.Device{
		RustdeskId: "123", UnattendedEnabled: true, RootCommand: "/system/xbin/su",
		StrategyGroupId: group.Id, StrategyProfileId: profiles[2].Id,
	}
	if _, err := db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Insert(&model.StrategyState{Id: 1, Revision: 1, GlobalProfileId: profiles[0].Id}); err != nil {
		t.Fatal(err)
	}
	effective, err := ResolveForDevice(db, &device)
	if err != nil {
		t.Fatal(err)
	}
	if !effective.UnattendedEnabled || effective.RootCommand != "/system/xbin/su" {
		t.Fatalf("unexpected per-device unattended settings: %+v", effective)
	}
	if effective.Profile.ID != profiles[2].Id || effective.ProfileSource != "device" {
		t.Fatalf("device profile did not win: %+v", effective)
	}
	device.StrategyProfileId = 0
	if _, err = db.ID(device.Id).Cols("strategy_profile_id").Update(&device); err != nil {
		t.Fatal(err)
	}
	effective, err = ResolveForDevice(db, &device)
	if err != nil || effective.Profile.ID != profiles[1].Id || effective.ProfileSource != "group:kiosks" {
		t.Fatalf("group profile did not win: %+v, %v", effective, err)
	}
	group.ProfileId = 0
	if _, err = db.ID(group.Id).Cols("profile_id").Update(&group); err != nil {
		t.Fatal(err)
	}
	effective, err = ResolveForDevice(db, &device)
	if err != nil || effective.Profile.ID != profiles[0].Id || effective.ProfileSource != "global" {
		t.Fatalf("global profile did not win: %+v, %v", effective, err)
	}
}

func TestStrategyRevisionIsMonotonic(t *testing.T) {
	db := newPolicyTestDB(t)
	first, err := CurrentRevision(db)
	if err != nil {
		t.Fatal(err)
	}
	session := db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		t.Fatal(err)
	}
	second, err := NextRevision(session)
	if err != nil {
		t.Fatal(err)
	}
	if err = session.Commit(); err != nil {
		t.Fatal(err)
	}
	if second <= first {
		t.Fatalf("revision did not increase: %d -> %d", first, second)
	}
}

func TestResolveForDeviceSkipsDisabledReferences(t *testing.T) {
	db := newPolicyTestDB(t)
	global := model.ServerProfile{Name: "global", IdServer: "global.example", Enabled: true}
	disabled := model.ServerProfile{Name: "disabled", IdServer: "disabled.example", Enabled: false}
	if _, err := db.Insert(&global, &disabled); err != nil {
		t.Fatal(err)
	}
	group := model.DeviceGroup{Name: "disabled-group", Enabled: false, ProfileId: global.Id}
	if _, err := db.Insert(&group); err != nil {
		t.Fatal(err)
	}
	device := model.Device{
		RustdeskId: "123", RootCommand: "auto", StrategyGroupId: group.Id, StrategyProfileId: disabled.Id,
	}
	if _, err := db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Insert(&model.StrategyState{Id: 1, Revision: 1, GlobalProfileId: global.Id}); err != nil {
		t.Fatal(err)
	}
	effective, err := ResolveForDevice(db, &device)
	if err != nil || effective.Profile.ID != global.Id || effective.ProfileSource != "global" {
		t.Fatalf("disabled references did not fall back to global: %+v, %v", effective, err)
	}
}
