package api_test

import (
	"bytes"
	"net/http"
	"rustdesk-api-server-pro/app/model"
	"testing"
)

func TestDeviceDelete(t *testing.T) {
	for _, scenario := range []string{"active", "online", "confirmation", "unauthorized", "missing", "audit-failure", "success"} {
		t.Run(scenario, func(t *testing.T) {
			db, app := managementApp(t)
			group := model.DeviceGroup{Name: "keep-group"}
			profile := model.ServerProfile{Name: "keep-profile"}
			if _, err := db.Insert(&group, &profile); err != nil {
				t.Fatal(err)
			}
			device := model.Device{RustdeskId: "delete-test", Disabled: true, Alias: "店铺", StrategyGroupId: group.Id, StrategyProfileId: profile.Id}
			if scenario == "active" {
				device.Disabled = false
			}
			if scenario == "online" {
				device.IsOnline = true
			}
			if _, err := db.Insert(&device); err != nil {
				t.Fatal(err)
			}
			credential := model.DeviceCredential{DeviceId: device.Id, Enabled: false, PublicKey: "test-key"}
			created := model.Peer{UserId: 1, AbId: 1, RustdeskId: device.RustdeskId, ManagedDeviceId: device.Id, ManagedCreated: true}
			personal := model.Peer{UserId: 2, AbId: 2, RustdeskId: device.RustdeskId, ManagedDeviceId: device.Id, Password: "keep-password"}
			history := model.DeviceOperation{DeviceId: device.Id, RustdeskId: device.RustdeskId, ActorId: 1, Action: "disable"}
			if _, err := db.Insert(&credential, &created, &personal, &history); err != nil {
				t.Fatal(err)
			}
			id, confirmation, token := device.Id, device.RustdeskId, "test-admin-token"
			if scenario == "confirmation" {
				confirmation = "wrong"
			}
			if scenario == "missing" {
				id = 9999
			}
			if scenario == "unauthorized" {
				token = ""
			}
			if scenario == "audit-failure" {
				if _, err := db.Exec("DROP TABLE device_operation"); err != nil {
					t.Fatal(err)
				}
			}
			body := map[string]any{"id": id, "rustdesk_id": confirmation}
			res := managementRequest(t, app, http.MethodDelete, "/admin/devices/record", token, body)
			success := bytes.Contains(res.Body.Bytes(), []byte(`"code":200`))
			if success != (scenario == "success") {
				t.Fatalf("unexpected delete result: %s", res.Body.String())
			}
			exists, err := db.ID(device.Id).Exist(new(model.Device))
			if err != nil || exists == success {
				t.Fatal("device state wrong", err)
			}
			exists, err = db.ID(credential.Id).Exist(new(model.DeviceCredential))
			if err != nil || exists == success {
				t.Fatal("credential cleanup not atomic", err)
			}
			exists, err = db.ID(created.Id).Exist(new(model.Peer))
			if err != nil || exists == success {
				t.Fatal("managed peer cleanup not atomic", err)
			}
			saved := model.Peer{}
			if has, err := db.ID(personal.Id).Get(&saved); err != nil || !has || saved.Password != "keep-password" {
				t.Fatal("personal peer lost")
			}
			if success && saved.ManagedDeviceId != 0 || !success && saved.ManagedDeviceId != device.Id {
				t.Fatal("personal association cleanup not atomic")
			}
			if has, err := db.ID(group.Id).Exist(new(model.DeviceGroup)); err != nil || !has {
				t.Fatal("shared group removed")
			}
			if has, err := db.ID(profile.Id).Exist(new(model.ServerProfile)); err != nil || !has {
				t.Fatal("shared profile removed")
			}
			if success {
				count, err := db.Where("rustdesk_id = ?", device.RustdeskId).Count(new(model.DeviceOperation))
				if err != nil || count != 2 {
					t.Fatal("history missing", count, err)
				}
				res = managementRequest(t, app, http.MethodDelete, "/admin/devices/record", token, body)
				if !bytes.Contains(res.Body.Bytes(), []byte("DeviceNotFound")) {
					t.Fatal("repeat delete response", res.Body.String())
				}
			}
		})
	}
}
