package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	appserver "rustdesk-api-server-pro/app"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/config"
	"strings"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func adminJSON(t *testing.T, app http.Handler, method, path string, body any) map[string]any {
	t.Helper()
	var input *bytes.Reader
	if body == nil {
		input = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		input = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, input)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "test-admin-token")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s %s status=%d body=%s", method, path, response.Code, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["code"] != float64(200) {
		t.Fatalf("%s %s failed: %s", method, path, response.Body.String())
	}
	return result
}

func TestAdminServerProfilesAssignmentsAndPreview(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:strategy-admin-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(
		new(model.User), new(model.AuthToken), new(model.Device), new(model.DeviceGroup),
		new(model.DeviceCredential), new(model.ServerProfile), new(model.StrategyState),
	); err != nil {
		t.Fatal(err)
	}
	admin := model.User{Username: "admin", Status: 1, IsAdmin: true}
	if _, err = db.Insert(&admin); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO auth_token(user_id, token, expired, is_admin, status) VALUES(?, ?, ?, 1, 1)", admin.Id, "test-admin-token", time.Now().Add(time.Hour).Format(config.TimeFormat)); err != nil {
		t.Fatal(err)
	}
	device := model.Device{RustdeskId: "123", RootCommand: "auto"}
	if _, err = db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	cfg := config.GetDefaultServerConfig()
	cfg.ProvisioningSecretKey = base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	application := iris.New()
	application.RegisterDependency(db, cfg)
	appserver.SetRoute(application)
	if err = application.Build(); err != nil {
		t.Fatal(err)
	}

	created := adminJSON(t, application, http.MethodPost, "/admin/devices/server-profiles", map[string]any{
		"name": "Primary", "id_server": "id.example.com", "relay_server": "relay.example.com",
		"server_key": "public-key", "permanent_password": "private-password", "enabled": true,
	})
	createdData := created["data"].(map[string]any)
	profileID := int(createdData["id"].(float64))
	createdJSON, _ := json.Marshal(created)
	if strings.Contains(string(createdJSON), "private-password") || createdData["password_set"] != true {
		t.Fatalf("profile response exposed or lost password state: %s", createdJSON)
	}
	groupResult := adminJSON(t, application, http.MethodPost, "/admin/devices/groups", map[string]any{
		"name": "Kiosks", "enabled": true, "profile_id": profileID,
	})
	groupID := int(groupResult["data"].(map[string]any)["id"].(float64))
	moved := adminJSON(t, application, http.MethodPut, "/admin/devices/group", map[string]any{
		"id": device.Id, "group_id": groupID,
	})
	unchanged := adminJSON(t, application, http.MethodPut, "/admin/devices/group", map[string]any{
		"id": device.Id, "group_id": groupID,
	})
	if moved["data"].(map[string]any)["revision"] != unchanged["data"].(map[string]any)["revision"] {
		t.Fatal("unchanged group assignment unexpectedly increased the strategy revision")
	}
	deviceList := adminJSON(t, application, http.MethodGet, "/admin/devices/list?current=1&size=10", nil)
	records := deviceList["data"].(map[string]any)["records"].([]any)
	listedDevice := records[0].(map[string]any)
	if listedDevice["group_id"] != float64(groupID) || listedDevice["group_name"] != "Kiosks" {
		t.Fatalf("device list did not expose the selected group: %v", listedDevice)
	}
	preview := adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/server-profile-preview?device_id=%d", device.Id), nil)
	effective := preview["data"].(map[string]any)
	if effective["profile_name"] != "Primary" || effective["profile_source"] != "group:Kiosks" || effective["root_command"] != "auto" {
		t.Fatalf("unexpected effective strategy: %v", effective)
	}
	adminJSON(t, application, http.MethodPut, "/admin/devices/server-profile", map[string]any{
		"id": device.Id, "profile_id": profileID,
	})
	adminJSON(t, application, http.MethodPut, "/admin/devices/group", map[string]any{
		"id": device.Id, "group_id": 0,
	})
	preview = adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/server-profile-preview?device_id=%d", device.Id), nil)
	effective = preview["data"].(map[string]any)
	if effective["profile_source"] != "device" {
		t.Fatalf("moving groups unexpectedly cleared the device profile: %v", effective)
	}
	adminJSON(t, application, http.MethodPut, "/admin/devices/server-profile", map[string]any{
		"id": device.Id, "profile_id": 0,
	})
	adminJSON(t, application, http.MethodPut, "/admin/devices/global-server-profile", map[string]any{
		"profile_id": profileID,
	})
	preview = adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/server-profile-preview?device_id=%d", device.Id), nil)
	effective = preview["data"].(map[string]any)
	if effective["profile_source"] != "global" {
		t.Fatalf("device did not fall back to the global profile: %v", effective)
	}
	adminJSON(t, application, http.MethodPut, "/admin/devices/unattended", map[string]any{
		"id": device.Id, "enabled": true, "root_command": "/system/xbin/su",
	})
	preview = adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/server-profile-preview?device_id=%d", device.Id), nil)
	effective = preview["data"].(map[string]any)
	if effective["unattended_enabled"] != true || effective["root_command"] != "/system/xbin/su" {
		t.Fatalf("unexpected unattended strategy: %v", effective)
	}
}
