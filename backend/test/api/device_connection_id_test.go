package api_test

import (
	"bytes"
	"net/http"
	"rustdesk-api-server-pro/app/model"
	"sync"
	"testing"
)

func connectionIDRequest(t *testing.T, app http.Handler, deviceID int, connectionID string) bool {
	t.Helper()
	res := managementRequest(t, app, http.MethodPut, "/admin/devices/connection-id", "test-admin-token", map[string]any{
		"id": deviceID, "connection_id": connectionID,
	})
	return bytes.Contains(res.Body.Bytes(), []byte(`"code":200`))
}

func TestManagedConnectionIDAssignment(t *testing.T) {
	db, app := managementApp(t)
	device := model.Device{RustdeskId: "123456789", Uuid: "managed-uuid"}
	if _, err := db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	credential := model.DeviceCredential{DeviceId: device.Id, PublicKey: "managed-key", Enabled: true}
	if _, err := db.Insert(&credential); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"short", "1shop23", "shop_23", "shop23.example", "shop-id-that-is-too-long"} {
		if connectionIDRequest(t, app, device.Id, invalid) {
			t.Fatalf("invalid connection id accepted: %q", invalid)
		}
	}
	if !connectionIDRequest(t, app, device.Id, "  Shop23-A01  ") {
		t.Fatal("valid connection id rejected")
	}
	saved := model.Device{}
	if _, err := db.ID(device.Id).Get(&saved); err != nil {
		t.Fatal(err)
	}
	if saved.RequestedRustdeskId != "shop23-a01" || saved.ConnectionIdStatus != model.ConnectionIdPending || saved.ConnectionIdRevision <= 0 || saved.ConnectionIdRequestedAt.IsZero() {
		t.Fatalf("request not persisted: %#v", saved)
	}
	if !connectionIDRequest(t, app, device.Id, "shop23-a01") {
		t.Fatal("idempotent request rejected")
	}
	if count, err := db.Where("action = ?", "connection_id_request").Count(new(model.DeviceOperation)); err != nil || count != 1 {
		t.Fatalf("idempotent request duplicated audit: count=%d err=%v", count, err)
	}
	list := adminJSON(t, app, http.MethodGet, "/admin/devices/list", nil)
	record := list["data"].(map[string]any)["records"].([]any)[0].(map[string]any)
	if record["managed"] != true || record["connection_id_status"] != model.ConnectionIdPending || record["requested_rustdesk_id"] != "shop23-a01" {
		t.Fatalf("connection id state missing from list: %#v", record)
	}
}

func TestConcurrentConnectionIDReservation(t *testing.T) {
	db, app := managementApp(t)
	devices := []model.Device{{RustdeskId: "123456781"}, {RustdeskId: "123456782"}}
	for i := range devices {
		if _, err := db.Insert(&devices[i]); err != nil {
			t.Fatal(err)
		}
	}
	for i := range devices {
		credential := model.DeviceCredential{DeviceId: devices[i].Id, PublicKey: devices[i].RustdeskId, Enabled: true}
		if _, err := db.Insert(&credential); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	results := make(chan bool, len(devices))
	var wg sync.WaitGroup
	for i := range devices {
		wg.Add(1)
		go func(deviceID int) {
			defer wg.Done()
			<-start
			results <- connectionIDRequest(t, app, deviceID, "shared-a01")
		}(devices[i].Id)
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for success := range results {
		if success {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one reservation, got %d", successes)
	}
}
