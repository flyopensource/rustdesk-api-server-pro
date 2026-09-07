package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/config"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManagedDeviceAlias(t *testing.T) {
	db, app := managementApp(t)
	device := model.Device{RustdeskId: "123456789", Uuid: "alias-test-uuid", Hostname: "board"}
	if _, err := db.Insert(&device); err != nil {
		t.Fatal(err)
	}
	users := []model.User{{Username: "alice", Status: 1}, {Username: "bob", Status: 1}}
	books := []model.AddressBook{}
	for i := range users {
		if _, err := db.Insert(&users[i]); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO auth_token(user_id, token, expired, is_admin, status) VALUES(?, ?, ?, 0, 1)", users[i].Id, users[i].Username, time.Now().Add(time.Hour).Format(config.TimeFormat)); err != nil {
			t.Fatal(err)
		}
		book := model.AddressBook{UserId: users[i].Id, Guid: users[i].Username, Name: model.PersonalAddressBookName, Rule: 3}
		if _, err := db.Insert(&book); err != nil {
			t.Fatal(err)
		}
		books = append(books, book)
	}
	personal := model.Peer{UserId: users[0].Id, AbId: books[0].Id, RustdeskId: device.RustdeskId, Alias: "personal", Password: "test-password", Tags: `["keep"]`}
	unrelated := model.Peer{UserId: users[1].Id, AbId: books[1].Id, RustdeskId: "unrelated", Alias: "untouched", Tags: "[]"}
	if _, err := db.Insert(&personal, &unrelated); err != nil {
		t.Fatal(err)
	}
	path := "/admin/devices/alias"
	form := func(alias string, targets ...int) map[string]any {
		if targets == nil {
			targets = []int{}
		}
		return map[string]any{"id": device.Id, "alias": alias, "address_book_ids": targets}
	}
	for _, token := range []string{"", "Bearer alice"} {
		res := managementRequest(t, app, http.MethodPut, path, token, form("bad", books[0].Id))
		if bytes.Contains(res.Body.Bytes(), []byte(`"code":200`)) {
			t.Fatal("non-admin changed alias")
		}
	}
	for _, alias := range []string{"bad\nname", strings.Repeat("中", 129)} {
		res := managementRequest(t, app, http.MethodPut, path, "test-admin-token", form(alias, books[0].Id))
		if bytes.Contains(res.Body.Bytes(), []byte(`"code":200`)) {
			t.Fatal("invalid alias accepted")
		}
	}
	for i := 0; i < 2; i++ {
		adminJSON(t, app, http.MethodPut, path, form("  上海店一号机  ", books[0].Id))
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for _, alias := range []string{"parallel-A", "parallel-B"} {
		wg.Add(1)
		go func(alias string) {
			defer wg.Done()
			res := managementRequest(t, app, http.MethodPut, path, "test-admin-token", form(alias, books[0].Id))
			results <- bytes.Contains(res.Body.Bytes(), []byte(`"code":200`))
		}(alias)
	}
	wg.Wait()
	close(results)
	for success := range results {
		if !success {
			t.Fatal("concurrent alias request failed")
		}
	}
	concurrentDevice, concurrentPeer := model.Device{}, model.Peer{}
	if _, err := db.ID(device.Id).Get(&concurrentDevice); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ID(personal.Id).Get(&concurrentPeer); err != nil {
		t.Fatal(err)
	}
	if concurrentDevice.Alias != concurrentPeer.Alias {
		t.Fatal("concurrent publication lost atomicity")
	}
	adminJSON(t, app, http.MethodPut, path, form("上海店一号机", books[0].Id))
	readPeer := func(id int) model.Peer {
		t.Helper()
		peer := model.Peer{}
		if has, err := db.ID(id).Get(&peer); err != nil || !has {
			t.Fatalf("peer missing: %v", err)
		}
		return peer
	}
	p := readPeer(personal.Id)
	if p.Alias != "上海店一号机" || p.ManagedDeviceId != device.Id || p.ManagedCreated || p.Password != "test-password" || p.Tags != `["keep"]` {
		t.Fatalf("personal fields lost: %#v", p)
	}
	count, err := db.Count(new(model.Peer))
	if err != nil || count != 2 {
		t.Fatal("duplicate peers", count, err)
	}
	list := adminJSON(t, app, http.MethodGet, "/admin/devices/list?alias="+url.QueryEscape("上海店"), nil)
	if list["data"].(map[string]any)["total"] != float64(1) {
		t.Fatal("alias search failed")
	}
	postJSON(t, app, "/api/sysinfo", map[string]any{"id": device.RustdeskId, "uuid": device.Uuid, "hostname": "new-host", "alias": "reported-alias"})
	reported := model.Device{}
	if _, err := db.ID(device.Id).Get(&reported); err != nil || reported.Alias != "上海店一号机" || reported.Hostname != "new-host" {
		t.Fatal("device report changed managed alias")
	}

	for _, route := range []string{"/api/ab", "/api/ab/peers?ab=alice"} {
		method := http.MethodGet
		if strings.Contains(route, "peers") {
			method = http.MethodPost
		}
		res := managementRequest(t, app, method, route, "Bearer alice", nil)
		if res.Code != 200 || !strings.Contains(res.Body.String(), "上海店一号机") {
			t.Fatalf("alias not readable: %s %s", route, res.Body.String())
		}
		res = managementRequest(t, app, method, route, "Bearer bob", nil)
		if strings.Contains(res.Body.String(), "上海店一号机") {
			t.Fatal("alias leaked to unselected user")
		}
	}
	res := managementRequest(t, app, http.MethodPut, "/api/ab/peer/update/alice", "Bearer alice", map[string]any{"id": device.RustdeskId, "alias": "stale", "password": "new-password", "tags": []string{"updated"}})
	if res.Code != 200 || strings.Contains(res.Body.String(), "error") {
		t.Fatal(res.Body.String())
	}
	p = readPeer(personal.Id)
	if p.Alias != "上海店一号机" || p.Password != "new-password" || p.Tags != `["updated"]` {
		t.Fatal("managed alias overwrite or personal field update failed", p)
	}
	res = managementRequest(t, app, http.MethodDelete, "/api/ab/peer/alice", "Bearer alice", []string{device.RustdeskId})
	if res.Code != http.StatusConflict {
		t.Fatal("managed delete allowed", res.Body.String())
	}
	res = managementRequest(t, app, http.MethodPost, "/api/ab/peer/add/alice", "Bearer alice", map[string]any{"id": device.RustdeskId, "alias": "old"})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	legacy, err := json.Marshal(map[string]any{"tags": []string{}, "tag_colors": "{}", "peers": []map[string]any{{"id": device.RustdeskId, "alias": "stale", "tags": []string{}}}})
	if err != nil {
		t.Fatal(err)
	}
	res = managementRequest(t, app, http.MethodPost, "/api/ab", "Bearer alice", map[string]any{"data": string(legacy)})
	if res.Code != 200 || strings.Contains(res.Body.String(), "error") {
		t.Fatal(res.Body.String())
	}
	if p = readPeer(personal.Id); p.Alias != "上海店一号机" || p.Password != "new-password" {
		t.Fatal("whole-book write destroyed managed peer")
	}
	res = managementRequest(t, app, http.MethodPut, "/api/ab/peer/update/alice", "Bearer bob", map[string]any{"id": device.RustdeskId, "alias": "stolen"})
	if res.Code != 404 {
		t.Fatal("foreign book mutation accepted")
	}

	adminJSON(t, app, http.MethodPut, path, form("second", books[0].Id, books[1].Id, books[1].Id))
	count, err = db.Where("managed_device_id = ?", device.Id).Count(new(model.Peer))
	if err != nil || count != 2 {
		t.Fatal("publish not idempotent", count, err)
	}
	adminJSON(t, app, http.MethodPut, path, form(""))
	if p = readPeer(personal.Id); p.ManagedDeviceId != 0 || p.Password != "new-password" {
		t.Fatal("unpublishing destroyed personal peer")
	}
	if p = readPeer(unrelated.Id); p.Alias != "untouched" {
		t.Fatal("unrelated peer overwritten")
	}
	count, err = db.Count(new(model.Peer))
	if err != nil || count != 2 {
		t.Fatal("managed-created peer not removed")
	}
	res = managementRequest(t, app, http.MethodPut, path, "test-admin-token", form("rollback", books[0].Id, 99999))
	if bytes.Contains(res.Body.Bytes(), []byte(`"code":200`)) {
		t.Fatal("invalid target accepted")
	}
	saved := model.Device{}
	if _, err = db.ID(device.Id).Get(&saved); err != nil || saved.Alias != "" {
		t.Fatal("target failure not rolled back")
	}
	if _, err = db.Exec("DROP TABLE device_operation"); err != nil {
		t.Fatal(err)
	}
	res = managementRequest(t, app, http.MethodPut, path, "test-admin-token", form("audit-failure", books[0].Id))
	if bytes.Contains(res.Body.Bytes(), []byte(`"code":200`)) {
		t.Fatal("audit failure accepted")
	}
	if p = readPeer(personal.Id); p.ManagedDeviceId != 0 {
		t.Fatal("audit failure not rolled back")
	}
}
