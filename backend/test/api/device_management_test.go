package api_test

import (
	"bytes"
	"encoding/json"
	"github.com/kataras/iris/v12"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	appserver "rustdesk-api-server-pro/app"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/config"
	"testing"
	"time"
	"xorm.io/xorm"
)

func managementApp(t *testing.T) (*xorm.Engine, *iris.Application) {
	t.Helper()
	db, err := xorm.NewEngine("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.Sync(new(model.User), new(model.AuthToken), new(model.Device), new(model.DeviceCredential), new(model.DeviceOperation), new(model.DeviceGroup), new(model.ServerProfile), new(model.StrategyState)); err != nil {
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
	device := model.Device{RustdeskId: "test-device", Uuid: "test-uuid", StrategyGroupId: 42, UnattendedEnabled: true}
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
	if !saved.Disabled || saved.StrategyGroupId != 42 || !saved.UnattendedEnabled {
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
