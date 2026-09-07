package api

import (
	"fmt"
	apiForm "rustdesk-api-server-pro/app/form/api"
	"rustdesk-api-server-pro/app/model"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func TestReconcileConnectionID(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:connection-id-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(new(model.Device), new(model.Peer), new(model.DeviceOperation)); err != nil {
		t.Fatal(err)
	}
	device := model.Device{RustdeskId: "123456789", RequestedRustdeskId: "shop23-a01", ConnectionIdStatus: model.ConnectionIdPending, ConnectionIdRevision: 42, Uuid: "device-uuid"}
	if _, err = db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	peer := model.Peer{UserId: 1, AbId: 1, RustdeskId: device.RustdeskId, Alias: "上海店", ManagedDeviceId: device.Id}
	if _, err = db.Insert(&peer); err != nil {
		t.Fatal(err)
	}
	report := apiForm.ConnectionIdStatusForm{RequestedId: device.RequestedRustdeskId, ActiveId: device.RequestedRustdeskId, Status: model.ConnectionIdApplied, Revision: device.ConnectionIdRevision}
	if err = reconcileConnectionID(db, &device, &report, device.RequestedRustdeskId); err != nil {
		t.Fatal(err)
	}
	if device.RustdeskId != "shop23-a01" || device.ConnectionIdStatus != model.ConnectionIdApplied {
		t.Fatalf("device not reconciled: %#v", device)
	}
	savedPeer := model.Peer{}
	if _, err = db.ID(peer.Id).Get(&savedPeer); err != nil || savedPeer.RustdeskId != device.RustdeskId || savedPeer.Alias != peer.Alias {
		t.Fatalf("managed peer not reconciled: %#v err=%v", savedPeer, err)
	}
	if count, countErr := db.Where("action = ?", "connection_id_applied").Count(new(model.DeviceOperation)); countErr != nil || count != 1 {
		t.Fatalf("applied transition not audited: count=%d err=%v", count, countErr)
	}
	if err = reconcileConnectionID(db, &device, &report, device.RustdeskId); err != nil {
		t.Fatalf("idempotent applied report rejected: %v", err)
	}
	if count, countErr := db.Where("action = ?", "connection_id_applied").Count(new(model.DeviceOperation)); countErr != nil || count != 1 {
		t.Fatalf("idempotent report duplicated audit: count=%d err=%v", count, countErr)
	}
}

func TestReconcileConnectionIDFailureKeepsActiveID(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:connection-id-failure-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(new(model.Device), new(model.DeviceOperation)); err != nil {
		t.Fatal(err)
	}
	device := model.Device{RustdeskId: "123456789", RequestedRustdeskId: "shop23-a01", ConnectionIdStatus: model.ConnectionIdPending, ConnectionIdRevision: 42}
	if _, err = db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	report := apiForm.ConnectionIdStatusForm{RequestedId: device.RequestedRustdeskId, ActiveId: device.RustdeskId, Status: model.ConnectionIdFailed, Revision: device.ConnectionIdRevision, LastError: "id_taken"}
	if err = reconcileConnectionID(db, &device, &report, device.RustdeskId); err != nil {
		t.Fatal(err)
	}
	if device.RustdeskId != "123456789" || device.ConnectionIdStatus != model.ConnectionIdFailed || device.ConnectionIdError != "id_taken" {
		t.Fatalf("failure state corrupted active id: %#v", device)
	}
	if err = reconcileConnectionID(db, &device, &report, device.RustdeskId); err != nil {
		t.Fatalf("idempotent failed report rejected: %v", err)
	}
	device.ConnectionIdStatus = model.ConnectionIdPending
	device.ConnectionIdRevision++
	if err = reconcileConnectionID(db, &device, &report, device.RustdeskId); err != nil {
		t.Fatalf("stale failure report rejected: %v", err)
	}
}
