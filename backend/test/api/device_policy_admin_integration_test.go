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

func TestAdminDeviceGroupsOwnCompletePolicies(t *testing.T) {
	db, err := xorm.NewEngine("sqlite", fmt.Sprintf("file:strategy-admin-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Sync(
		new(model.User), new(model.AuthToken), new(model.Device), new(model.DeviceGroup),
		new(model.ServerProfile), new(model.StrategyState),
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
	device := model.Device{RustdeskId: "123"}
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

	profileResult := adminJSON(t, application, http.MethodPost, "/admin/devices/server-profiles", map[string]any{
		"name": "Primary", "id_server": "id.example.com", "relay_server": "relay.example.com",
		"server_key": "public-key", "enabled": true,
	})
	profileData := profileResult["data"].(map[string]any)
	profileID := int(profileData["id"].(float64))
	if _, exists := profileData["password_set"]; exists {
		t.Fatalf("server profile still owns password state: %v", profileData)
	}

	groupResult := adminJSON(t, application, http.MethodPost, "/admin/devices/groups", map[string]any{
		"name": "Kiosks", "enabled": true, "is_default": true, "profile_id": profileID,
		"unattended_enabled": true, "permanent_password": "private-password", "root_command": "testsu",
	})
	groupID := int(groupResult["data"].(map[string]any)["id"].(float64))
	groupsResult := adminJSON(t, application, http.MethodGet, "/admin/devices/groups", nil)
	groupsJSON, _ := json.Marshal(groupsResult)
	if strings.Contains(string(groupsJSON), "private-password") {
		t.Fatalf("device group response exposed its password: %s", groupsJSON)
	}
	groupData := groupsResult["data"].(map[string]any)["groups"].([]any)[0].(map[string]any)
	if groupsResult["data"].(map[string]any)["default_group_id"] != float64(groupID) ||
		groupData["is_default"] != true || groupData["default_coverage_count"] != float64(1) ||
		groupData["password_set"] != true || groupData["unattended_enabled"] != true || groupData["root_command"] != "testsu" {
		t.Fatalf("device group did not expose safe policy state: %v", groupData)
	}
	secondGroupResult := adminJSON(t, application, http.MethodPost, "/admin/devices/groups", map[string]any{
		"name": "Fallback", "enabled": true, "is_default": true, "profile_id": profileID,
		"unattended_enabled": false, "root_command": "auto",
	})
	secondGroupID := int(secondGroupResult["data"].(map[string]any)["id"].(float64))
	groupsResult = adminJSON(t, application, http.MethodGet, "/admin/devices/groups", nil)
	groupsData := groupsResult["data"].(map[string]any)
	defaultCount := 0
	for _, item := range groupsData["groups"].([]any) {
		if item.(map[string]any)["is_default"] == true {
			defaultCount++
		}
	}
	if groupsData["default_group_id"] != float64(secondGroupID) || defaultCount != 1 {
		t.Fatalf("setting a new default group did not replace the previous default: %v", groupsData)
	}
	adminJSON(t, application, http.MethodPut, "/admin/devices/groups", map[string]any{
		"id": groupID, "name": "Kiosks", "enabled": true, "is_default": true, "profile_id": profileID,
		"unattended_enabled": true, "root_command": "testsu", "clear_password": false,
	})
	adminJSON(t, application, http.MethodDelete, fmt.Sprintf("/admin/devices/groups?id=%d", secondGroupID), nil)

	moved := adminJSON(t, application, http.MethodPut, "/admin/devices/group", map[string]any{
		"id": device.Id, "group_id": groupID,
	})
	unchanged := adminJSON(t, application, http.MethodPut, "/admin/devices/group", map[string]any{
		"id": device.Id, "group_id": groupID,
	})
	if moved["data"].(map[string]any)["revision"] != unchanged["data"].(map[string]any)["revision"] {
		t.Fatal("unchanged group assignment unexpectedly increased the strategy revision")
	}
	preview := adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/policy-preview?device_id=%d", device.Id), nil)
	effective := preview["data"].(map[string]any)
	if effective["group_name"] != "Kiosks" || effective["group_source"] != "assigned" ||
		effective["profile_name"] != "Primary" || effective["unattended_enabled"] != true ||
		effective["root_command"] != "testsu" || effective["password_set"] != true {
		t.Fatalf("unexpected assigned group policy: %v", effective)
	}

	adminJSON(t, application, http.MethodPut, "/admin/devices/group", map[string]any{"id": device.Id, "group_id": 0})
	preview = adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/policy-preview?device_id=%d", device.Id), nil)
	effective = preview["data"].(map[string]any)
	if effective["group_name"] != "Kiosks" || effective["group_source"] != "default" {
		t.Fatalf("unassigned device did not use the default group: %v", effective)
	}

	deviceList := adminJSON(t, application, http.MethodGet, "/admin/devices/list?current=1&size=10", nil)
	listed := deviceList["data"].(map[string]any)["records"].([]any)[0].(map[string]any)
	if listed["group_id"] != float64(0) || listed["effective_group_id"] != float64(groupID) ||
		listed["unattended_enabled"] != true || listed["profile_password_set"] != true {
		t.Fatalf("device list did not expose the effective default group policy: %v", listed)
	}

	adminJSON(t, application, http.MethodPut, "/admin/devices/groups", map[string]any{
		"id": groupID, "name": "Kiosks", "enabled": true, "is_default": true, "profile_id": profileID,
		"unattended_enabled": true, "root_command": "su", "clear_password": false,
	})
	groupsResult = adminJSON(t, application, http.MethodGet, "/admin/devices/groups", nil)
	groupData = groupsResult["data"].(map[string]any)["groups"].([]any)[0].(map[string]any)
	if groupData["password_set"] != true || groupData["root_command"] != "su" {
		t.Fatalf("editing non-password fields cleared or lost group policy: %v", groupData)
	}
	adminJSON(t, application, http.MethodPut, "/admin/devices/groups", map[string]any{
		"id": groupID, "name": "Kiosks", "enabled": true, "is_default": true, "profile_id": profileID,
		"unattended_enabled": true, "root_command": "su", "clear_password": true,
	})
	groupsResult = adminJSON(t, application, http.MethodGet, "/admin/devices/groups", nil)
	groupData = groupsResult["data"].(map[string]any)["groups"].([]any)[0].(map[string]any)
	if groupData["password_set"] != false || groupData["configuration_complete"] != false {
		t.Fatalf("cleared group password still appeared configured: %v", groupData)
	}

	adminJSON(t, application, http.MethodPut, "/admin/devices/group", map[string]any{"id": device.Id, "group_id": groupID})
	adminJSON(t, application, http.MethodPut, "/admin/devices/groups", map[string]any{
		"id": groupID, "name": "Kiosks", "enabled": false, "is_default": false, "profile_id": profileID,
		"unattended_enabled": true, "root_command": "su", "clear_password": false,
	})
	groupsResult = adminJSON(t, application, http.MethodGet, "/admin/devices/groups", nil)
	groupsData = groupsResult["data"].(map[string]any)
	if groupsData["default_group_id"] != float64(0) {
		t.Fatalf("disabling the default group did not clear the default relation: %v", groupsData)
	}
	preview = adminJSON(t, application, http.MethodGet, fmt.Sprintf("/admin/devices/policy-preview?device_id=%d", device.Id), nil)
	effective = preview["data"].(map[string]any)
	if effective["group_source"] != "none" || effective["group_warning"] != "assigned_group_disabled" {
		t.Fatalf("disabled assigned group unexpectedly fell back: %v", effective)
	}
	adminJSON(t, application, http.MethodPut, "/admin/devices/groups", map[string]any{
		"id": groupID, "name": "Kiosks", "enabled": true, "is_default": true, "profile_id": profileID,
		"unattended_enabled": true, "root_command": "su", "clear_password": false,
	})

	adminJSON(t, application, http.MethodDelete, fmt.Sprintf("/admin/devices/groups?id=%d", groupID), nil)
	groupsResult = adminJSON(t, application, http.MethodGet, "/admin/devices/groups", nil)
	groupsData = groupsResult["data"].(map[string]any)
	if groupsData["default_group_id"] != float64(0) || len(groupsData["groups"].([]any)) != 0 {
		t.Fatalf("deleting the default group left strategy state behind: %v", groupsData)
	}
	savedDevice := model.Device{}
	if _, err = db.ID(device.Id).Get(&savedDevice); err != nil || savedDevice.StrategyGroupId != 0 {
		t.Fatal("deleting a group did not clear device membership", err)
	}
}
