package api_test

import (
	"bytes"
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

func TestAdminHierarchicalPolicyAndPreview(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:policy-admin-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(new(model.User), new(model.AuthToken), new(model.Device), new(model.DeviceGroup), new(model.DeviceGroupMember), new(model.ManagedDevicePolicy)); err != nil {
		t.Fatal(err)
	}
	admin := model.User{Username: "admin", Status: 1, IsAdmin: true}
	if _, err = db.Insert(&admin); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO auth_token(user_id, token, expired, is_admin, status) VALUES(?, ?, ?, 1, 1)", admin.Id, "test-admin-token", time.Now().Add(time.Hour).Format(config.TimeFormat)); err != nil {
		t.Fatal(err)
	}
	var token model.AuthToken
	if found, findErr := db.Where("token = ? and expired > ? and status = 1 and is_admin = 1", "test-admin-token", time.Now().Format(config.TimeFormat)).Get(&token); findErr != nil || !found {
		t.Fatalf("admin token fixture is invalid: found=%v err=%v", found, findErr)
	}
	device := model.Device{RustdeskId: "123", RootCommand: "auto"}
	if _, err = db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	cfg := config.GetDefaultServerConfig()
	application := iris.New()
	application.RegisterDependency(db, cfg)
	appserver.SetRoute(application)
	if err = application.Build(); err != nil {
		t.Fatal(err)
	}

	created := adminJSON(t, application, http.MethodPost, "/admin/devices/groups", map[string]any{"name": "kiosks", "priority": 10, "enabled": true})
	groupID := int(created["data"].(map[string]any)["id"].(float64))
	adminJSON(t, application, http.MethodPut, "/admin/devices/groups/members", map[string]any{"group_id": groupID, "device_ids": []int{device.Id}})
	adminJSON(t, application, http.MethodPut, "/admin/devices/policy", map[string]any{"scope_type": "global", "scope_id": 0, "enabled": true, "document": map[string]any{"unattended": map[string]any{"enabled": true}, "server_profile": map[string]any{}}})
	adminJSON(t, application, http.MethodPut, "/admin/devices/policy", map[string]any{"scope_type": "group", "scope_id": groupID, "enabled": true, "document": map[string]any{"unattended": map[string]any{"root_command": "su"}, "server_profile": map[string]any{}}})
	adminJSON(t, application, http.MethodPut, "/admin/devices/policy", map[string]any{"scope_type": "device", "scope_id": device.Id, "enabled": true, "document": map[string]any{"unattended": map[string]any{"enabled": false}, "server_profile": map[string]any{"key": "private-key", "permanent_password": "private-password"}}})
	stored := adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/policy?scope_type=device&scope_id=%d", device.Id), nil)
	storedJSON, _ := json.Marshal(stored)
	if strings.Contains(string(storedJSON), "private-key") || strings.Contains(string(storedJSON), "private-password") {
		t.Fatalf("policy response exposed a secret: %s", storedJSON)
	}
	preview := adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/policy/preview?device_id=%d", device.Id), nil)
	data := preview["data"].(map[string]any)
	effective := data["effective"].(map[string]any)
	if effective["unattended_enabled"] != false || effective["root_command"] != "su" {
		t.Fatalf("unexpected preview: %v", data)
	}
	if effective["key_set"] != true || effective["permanent_password_set"] != true {
		t.Fatalf("preview lost secret presence flags: %v", effective)
	}
	layers := data["layers"].([]any)
	if len(layers) != 3 || layers[0] != "global" || layers[2] != "device" {
		t.Fatalf("unexpected layers: %v", layers)
	}
	loaded := model.Device{}
	if _, err = db.ID(device.Id).Get(&loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.PolicyRevision != 4 {
		t.Fatalf("policy revision = %d, want 4", loaded.PolicyRevision)
	}
}
