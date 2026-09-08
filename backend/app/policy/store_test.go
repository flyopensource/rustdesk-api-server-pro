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

func TestResolveForDeviceUsesAssignedGroupThenDefaultGroup(t *testing.T) {
	db := newPolicyTestDB(t)
	profiles := []model.ServerProfile{
		{Name: "default", IdServer: "default.example", Enabled: true},
		{Name: "assigned", IdServer: "assigned.example", Enabled: true},
	}
	for i := range profiles {
		if _, err := db.Insert(&profiles[i]); err != nil {
			t.Fatal(err)
		}
	}
	groups := []model.DeviceGroup{
		{Name: "Default", Enabled: true, ProfileId: profiles[0].Id, RootCommand: "auto"},
		{Name: "Kiosks", Enabled: true, ProfileId: profiles[1].Id, UnattendedEnabled: true, PasswordCiphertext: "encrypted", RootCommand: "testsu"},
	}
	for i := range groups {
		if _, err := db.Insert(&groups[i]); err != nil {
			t.Fatal(err)
		}
	}
	device := model.Device{RustdeskId: "123", StrategyGroupId: groups[1].Id}
	if _, err := db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Insert(&model.StrategyState{Id: 1, Revision: 1, DefaultGroupId: groups[0].Id}); err != nil {
		t.Fatal(err)
	}

	effective, err := ResolveForDevice(db, &device)
	if err != nil {
		t.Fatal(err)
	}
	if effective.GroupID != groups[1].Id || effective.GroupSource != "assigned" ||
		effective.Profile.ID != profiles[1].Id || !effective.UnattendedEnabled ||
		effective.RootCommand != "testsu" || effective.PasswordCiphertext != "encrypted" {
		t.Fatalf("assigned group was not resolved atomically: %+v", effective)
	}

	device.StrategyGroupId = 0
	effective, err = ResolveForDevice(db, &device)
	if err != nil {
		t.Fatal(err)
	}
	if effective.GroupID != groups[0].Id || effective.GroupSource != "default" ||
		effective.Profile.ID != profiles[0].Id || effective.UnattendedEnabled ||
		effective.RootCommand != "auto" || effective.PasswordCiphertext != "" {
		t.Fatalf("default group was not resolved atomically: %+v", effective)
	}
}

func TestResolveForDeviceDoesNotFallBackFromDisabledAssignedGroup(t *testing.T) {
	db := newPolicyTestDB(t)
	profile := model.ServerProfile{Name: "default", IdServer: "default.example", Enabled: true}
	if _, err := db.Insert(&profile); err != nil {
		t.Fatal(err)
	}
	groups := []model.DeviceGroup{
		{Name: "Default", Enabled: true, ProfileId: profile.Id, RootCommand: "auto"},
		{Name: "Disabled", Enabled: false, ProfileId: profile.Id, RootCommand: "auto"},
	}
	for i := range groups {
		if _, err := db.Insert(&groups[i]); err != nil {
			t.Fatal(err)
		}
	}
	device := model.Device{RustdeskId: "123", StrategyGroupId: groups[1].Id}
	if _, err := db.Insert(&model.StrategyState{Id: 1, Revision: 1, DefaultGroupId: groups[0].Id}); err != nil {
		t.Fatal(err)
	}
	effective, err := ResolveForDevice(db, &device)
	if err != nil {
		t.Fatal(err)
	}
	if effective.GroupID != 0 || effective.GroupSource != "none" ||
		effective.GroupWarning != "assigned_group_disabled" {
		t.Fatalf("disabled assigned group unexpectedly used the default group: %+v", effective)
	}
}

func TestResolveForDeviceReturnsNoPolicyForUnavailableDefaultProfile(t *testing.T) {
	db := newPolicyTestDB(t)
	profile := model.ServerProfile{Name: "disabled", IdServer: "disabled.example", Enabled: false}
	if _, err := db.Insert(&profile); err != nil {
		t.Fatal(err)
	}
	group := model.DeviceGroup{Name: "Default", Enabled: true, ProfileId: profile.Id, UnattendedEnabled: true, RootCommand: "su"}
	if _, err := db.Insert(&group); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Insert(&model.StrategyState{Id: 1, Revision: 1, DefaultGroupId: group.Id}); err != nil {
		t.Fatal(err)
	}
	effective, err := ResolveForDevice(db, &model.Device{RustdeskId: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if effective.ProfileEnabled || effective.UnattendedEnabled || effective.GroupSource != "none" ||
		effective.GroupWarning != "default_profile_unavailable" {
		t.Fatalf("unavailable default profile unexpectedly produced a policy: %+v", effective)
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
