package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kataras/iris/v12"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	appserver "rustdesk-api-server-pro/app"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/config"
	database "rustdesk-api-server-pro/db"
	"strings"
	"sync"
	"testing"
	"time"
	"xorm.io/xorm"
)

func managementApp(t *testing.T) (*xorm.Engine, *iris.Application) {
	t.Helper()
	db, err := database.NewEngine(&config.DbConfig{Driver: "sqlite", Dsn: filepath.Join(t.TempDir(), "test.db"), TimeZone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.Sync(new(model.User), new(model.AuthToken), new(model.Device), new(model.DeviceCredential), new(model.DeviceOperation), new(model.DeviceGroup), new(model.ServerProfile), new(model.StrategyState), new(model.DesktopEnrollmentToken)); err != nil {
		t.Fatal(err)
	}
	if err = db.Sync(new(model.Peer), new(model.AddressBook), new(model.Tags)); err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "admin", Status: 1, IsAdmin: true}
	if _, err = db.Insert(&user); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO auth_token(user_id, token, expired, is_admin, status) VALUES(?, ?, ?, 1, 1)", user.Id, "test-admin-token", time.Now().Add(time.Hour).Format(config.TimeFormat)); err != nil {
		t.Fatal(err)
	}
	app := iris.New()
	app.RegisterDependency(db, config.GetDefaultServerConfig())
	appserver.SetRoute(app)
	if err = app.Build(); err != nil {
		t.Fatal(err)
	}
	return db, app
}

func TestDesktopEnrollmentTokenManagementAndDeviceStatus(t *testing.T) {
	db, app := managementApp(t)
	device := model.Device{
		RustdeskId: "desktop-managed", Platform: "desktop", Arch: "x86_64",
		PasswordAppliedRevision: 17, PasswordApplyStatus: "success", PermanentPasswordSet: true,
		PasswordReportedAt: time.Now(),
	}
	if _, err := db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	credential := model.DeviceCredential{
		DeviceId: device.Id, PublicKey: "must-not-be-returned", RegistrationType: "desktop_token", Enabled: true,
	}
	if _, err := db.Insert(&credential); err != nil {
		t.Fatal(err)
	}

	created := adminJSON(t, app, http.MethodPost, "/admin/devices/desktop-enrollment-tokens", map[string]any{
		"expires_at": time.Now().Add(time.Hour).Unix(), "group_id": 0, "note": "desktop rollout",
	})
	createdData := created["data"].(map[string]any)
	plainToken, ok := createdData["token"].(string)
	if !ok || plainToken == "" {
		t.Fatalf("create did not return one-time plaintext token: %v", createdData)
	}
	tokenID := int(createdData["id"].(float64))

	listedTokens := adminJSON(t, app, http.MethodGet, "/admin/devices/desktop-enrollment-tokens", nil)
	listedTokenJSON, err := json.Marshal(listedTokens)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(listedTokenJSON), plainToken) || strings.Contains(string(listedTokenJSON), "secret_hash") {
		t.Fatalf("token list exposed secret material: %s", listedTokenJSON)
	}
	token := listedTokens["data"].(map[string]any)["tokens"].([]any)[0].(map[string]any)
	if token["status"] != "unused" || token["note"] != "desktop rollout" {
		t.Fatalf("unexpected enrollment token state: %v", token)
	}
	adminJSON(t, app, http.MethodDelete, fmt.Sprintf("/admin/devices/desktop-enrollment-tokens?id=%d", tokenID), nil)
	listedTokens = adminJSON(t, app, http.MethodGet, "/admin/devices/desktop-enrollment-tokens", nil)
	token = listedTokens["data"].(map[string]any)["tokens"].([]any)[0].(map[string]any)
	if token["status"] != "revoked" {
		t.Fatalf("token was not revoked: %v", token)
	}

	listedDevices := adminJSON(t, app, http.MethodGet, "/admin/devices/list?current=1&size=10", nil)
	listedDevice := listedDevices["data"].(map[string]any)["records"].([]any)[0].(map[string]any)
	if listedDevice["registration_type"] != "desktop_token" || listedDevice["platform"] != "desktop" ||
		listedDevice["arch"] != "x86_64" || listedDevice["password_apply_status"] != "success" ||
		listedDevice["permanent_password_set"] != true || listedDevice["password_applied_revision"] != float64(17) {
		t.Fatalf("device list did not expose desktop password status: %v", listedDevice)
	}
	listedDeviceJSON, err := json.Marshal(listedDevice)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(listedDeviceJSON), credential.PublicKey) {
		t.Fatalf("device list exposed credential key: %s", listedDeviceJSON)
	}
}

func TestConcurrentDeviceStateIsAtomic(t *testing.T) {
	db, app := managementApp(t)
	device := model.Device{RustdeskId: "concurrent-device"}
	if _, err := db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	credential := model.DeviceCredential{DeviceId: device.Id, Enabled: true, PublicKey: "test-key"}
	if _, err := db.Insert(&credential); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(enabled bool) {
			defer wg.Done()
			<-start
			res := managementRequest(t, app, http.MethodPut, "/admin/devices/enabled", "test-admin-token", map[string]any{"id": device.Id, "enabled": enabled})
			results <- bytes.Contains(res.Body.Bytes(), []byte(`"code":200`))
		}(i%2 == 0)
	}
	close(start)
	wg.Wait()
	close(results)
	for success := range results {
		if !success {
			t.Fatal("concurrent state request failed")
		}
	}
	saved := model.Device{}
	cred := model.DeviceCredential{}
	if _, err := db.ID(device.Id).Get(&saved); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ID(credential.Id).Get(&cred); err != nil {
		t.Fatal(err)
	}
	if saved.Disabled == cred.Enabled {
		t.Fatal("concurrent lifecycle operation left inconsistent credential state")
	}
}

func managementRequest(t *testing.T, app http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	return res
}

func TestDeviceLifecycle(t *testing.T) {
	db, app := managementApp(t)
	device := model.Device{RustdeskId: "test-device", Uuid: "test-uuid", StrategyGroupId: 42}
	if _, err := db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	credential := model.DeviceCredential{DeviceId: device.Id, PublicKey: "test-key", Enabled: true, LastSeq: 10}
	if _, err := db.Insert(&credential); err != nil {
		t.Fatal(err)
	}
	path := "/admin/devices/enabled"
	body := map[string]any{"id": device.Id, "enabled": false}
	res := managementRequest(t, app, http.MethodPut, path, "", body)
	if res.Code == http.StatusOK && bytes.Contains(res.Body.Bytes(), []byte(`"code":200`)) {
		t.Fatal("unauthorized mutation accepted")
	}
	for i := 0; i < 2; i++ {
		adminJSON(t, app, http.MethodPut, path, body)
	}
	saved := model.Device{}
	if _, err := db.ID(device.Id).Get(&saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Disabled || saved.StrategyGroupId != 42 {
		t.Fatal("state or policy lost")
	}
	cred := model.DeviceCredential{}
	if _, err := db.ID(credential.Id).Get(&cred); err != nil {
		t.Fatal(err)
	}
	if cred.Enabled || cred.LastSeq != 10 {
		t.Fatal("credential not disabled or sequence reset")
	}
	for _, route := range []string{"/api/heartbeat", "/api/sysinfo"} {
		res := postJSON(t, app, route, map[string]any{"id": device.RustdeskId, "uuid": device.Uuid})
		if res.Code != http.StatusForbidden {
			t.Fatalf("disabled device accepted by %s: %s", route, res.Body.String())
		}
	}
	list := adminJSON(t, app, http.MethodGet, "/admin/devices/list?state=disabled", nil)
	if list["data"].(map[string]any)["total"] != float64(1) {
		t.Fatal("state filter failed")
	}
	count, err := db.Count(new(model.DeviceOperation))
	if err != nil || count != 1 {
		t.Fatalf("idempotent audit: %d %v", count, err)
	}
	adminJSON(t, app, http.MethodPut, path, map[string]any{"id": device.Id, "enabled": true})
	cred = model.DeviceCredential{}
	if _, err = db.ID(credential.Id).Get(&cred); err != nil || !cred.Enabled || cred.LastSeq != 10 {
		t.Fatal("enable failed")
	}
	// Audit failure must roll back the entire state change.
	if _, err = db.Exec("DROP TABLE device_operation"); err != nil {
		t.Fatal(err)
	}
	res = managementRequest(t, app, http.MethodPut, path, "test-admin-token", body)
	if bytes.Contains(res.Body.Bytes(), []byte(`"code":200`)) {
		t.Fatal("audit failure accepted")
	}
	saved = model.Device{}
	if _, err = db.ID(device.Id).Get(&saved); err != nil || saved.Disabled {
		t.Fatal("failed operation not rolled back")
	}
}
