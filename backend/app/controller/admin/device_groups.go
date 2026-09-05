package admin

import (
	"encoding/json"
	"errors"
	"net/url"
	"rustdesk-api-server-pro/app/model"
	devicepolicy "rustdesk-api-server-pro/app/policy"
	"strings"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"xorm.io/xorm"
)

func registerPolicyRoutes(b mvc.BeforeActivation) {
	b.Handle("GET", "/devices/groups", "HandleGroups")
	b.Handle("POST", "/devices/groups", "HandleCreateGroup")
	b.Handle("PUT", "/devices/groups", "HandleUpdateGroup")
	b.Handle("DELETE", "/devices/groups", "HandleDeleteGroup")
	b.Handle("PUT", "/devices/groups/members", "HandleGroupMembers")
	b.Handle("GET", "/devices/policy", "HandleGetPolicy")
	b.Handle("PUT", "/devices/policy", "HandlePutPolicy")
	b.Handle("DELETE", "/devices/policy", "HandleDeletePolicy")
	b.Handle("GET", "/devices/policy/preview", "HandlePolicyPreview")
}

func (c *DevicesController) HandleGroups() mvc.Result {
	groups := make([]model.DeviceGroup, 0)
	if err := c.Db.Asc("priority", "id").Find(&groups); err != nil {
		return c.Error(nil, err.Error())
	}
	result := make([]iris.Map, 0, len(groups))
	for _, group := range groups {
		members := make([]model.DeviceGroupMember, 0)
		if err := c.Db.Where("group_id = ?", group.Id).Asc("device_id").Find(&members); err != nil {
			return c.Error(nil, err.Error())
		}
		memberIDs := make([]int, 0, len(members))
		for _, member := range members {
			memberIDs = append(memberIDs, member.DeviceId)
		}
		result = append(result, iris.Map{"id": group.Id, "name": group.Name, "priority": group.Priority, "enabled": group.Enabled, "member_count": len(memberIDs), "device_ids": memberIDs})
	}
	return c.Success(result, "ok")
}

func (c *DevicesController) HandleCreateGroup() mvc.Result {
	var form struct {
		Name     string `json:"name"`
		Priority int    `json:"priority"`
		Enabled  bool   `json:"enabled"`
	}
	if c.Ctx.ReadJSON(&form) != nil || strings.TrimSpace(form.Name) == "" || len(strings.TrimSpace(form.Name)) > 100 {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	group := model.DeviceGroup{Name: strings.TrimSpace(form.Name), Priority: form.Priority, Enabled: form.Enabled}
	if _, err := c.Db.Insert(&group); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"id": group.Id}, "ok")
}

func (c *DevicesController) HandleUpdateGroup() mvc.Result {
	var form struct {
		Id       int    `json:"id"`
		Name     string `json:"name"`
		Priority int    `json:"priority"`
		Enabled  bool   `json:"enabled"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || strings.TrimSpace(form.Name) == "" || len(strings.TrimSpace(form.Name)) > 100 {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	updated, err := c.Db.ID(form.Id).Cols("name", "priority", "enabled").Update(&model.DeviceGroup{Name: strings.TrimSpace(form.Name), Priority: form.Priority, Enabled: form.Enabled})
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if updated == 0 {
		return c.Error(nil, "DeviceGroupNotFound")
	}
	if err := bumpGroupDevices(c.Db, form.Id); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}

func (c *DevicesController) HandleDeleteGroup() mvc.Result {
	id := c.Ctx.URLParamIntDefault("id", 0)
	if id <= 0 {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	members := make([]model.DeviceGroupMember, 0)
	if err := c.Db.Where("group_id = ?", id).Find(&members); err != nil {
		return c.Error(nil, err.Error())
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err := session.Where("group_id = ?", id).Delete(new(model.DeviceGroupMember)); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if _, err := session.Where("scope_type = ? AND scope_id = ?", model.DevicePolicyScopeGroup, id).Delete(new(model.ManagedDevicePolicy)); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if _, err := session.ID(id).Delete(new(model.DeviceGroup)); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	for _, member := range members {
		if err := bumpDeviceSession(session, member.DeviceId); err != nil {
			session.Rollback()
			return c.Error(nil, err.Error())
		}
	}
	if err := session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}

func (c *DevicesController) HandleGroupMembers() mvc.Result {
	var form struct {
		GroupId   int   `json:"group_id"`
		DeviceIds []int `json:"device_ids"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.GroupId <= 0 {
		return c.Error(nil, "InvalidGroupMembers")
	}
	unique := make(map[int]struct{}, len(form.DeviceIds))
	for _, id := range form.DeviceIds {
		if id <= 0 {
			return c.Error(nil, "InvalidGroupMembers")
		}
		unique[id] = struct{}{}
	}
	groupExists, err := c.Db.ID(form.GroupId).Exist(new(model.DeviceGroup))
	if err != nil || !groupExists {
		return c.Error(nil, "DeviceGroupNotFound")
	}
	if len(unique) > 0 {
		ids := make([]int, 0, len(unique))
		for id := range unique {
			ids = append(ids, id)
		}
		count, countErr := c.Db.In("id", ids).Count(new(model.Device))
		if countErr != nil || int(count) != len(ids) {
			return c.Error(nil, "DeviceNotFound")
		}
		groupPolicyExists, policyErr := c.Db.Where("scope_type = ? AND scope_id = ? AND enabled = ?", model.DevicePolicyScopeGroup, form.GroupId, true).Exist(new(model.ManagedDevicePolicy))
		if policyErr != nil {
			return c.Error(nil, policyErr.Error())
		}
		if groupPolicyExists {
			for _, id := range ids {
				if err := materializeLegacyDevicePolicy(c.Db, id); err != nil {
					return c.Error(nil, err.Error())
				}
			}
		}
	}
	old := make([]model.DeviceGroupMember, 0)
	if err = c.Db.Where("group_id = ?", form.GroupId).Find(&old); err != nil {
		return c.Error(nil, err.Error())
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err := session.Where("group_id = ?", form.GroupId).Delete(new(model.DeviceGroupMember)); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	for id := range unique {
		if _, err := session.Insert(&model.DeviceGroupMember{GroupId: form.GroupId, DeviceId: id}); err != nil {
			session.Rollback()
			return c.Error(nil, err.Error())
		}
	}
	affected := unique
	for _, member := range old {
		affected[member.DeviceId] = struct{}{}
	}
	for id := range affected {
		if err := bumpDeviceSession(session, id); err != nil {
			session.Rollback()
			return c.Error(nil, err.Error())
		}
	}
	if err := session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}

func (c *DevicesController) HandleGetPolicy() mvc.Result {
	scope, id, ok := policyScope(c.Ctx.URLParam("scope_type"), c.Ctx.URLParamIntDefault("scope_id", 0))
	if !ok {
		return c.Error(nil, "InvalidPolicyScope")
	}
	item := model.ManagedDevicePolicy{}
	has, err := c.Db.Where("scope_type = ? AND scope_id = ?", scope, id).Get(&item)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if !has {
		return c.Success(nil, "ok")
	}
	var document devicepolicy.Document
	if json.Unmarshal([]byte(item.Document), &document) != nil {
		return c.Error(nil, "InvalidStoredPolicy")
	}
	return c.Success(iris.Map{"scope_type": scope, "scope_id": id, "revision": item.Revision, "enabled": item.Enabled, "document": redactPolicy(document)}, "ok")
}

func (c *DevicesController) HandlePutPolicy() mvc.Result {
	var form struct {
		ScopeType string                `json:"scope_type"`
		ScopeId   int                   `json:"scope_id"`
		Enabled   bool                  `json:"enabled"`
		Document  devicepolicy.Document `json:"document"`
	}
	if c.Ctx.ReadJSON(&form) != nil {
		return c.Error(nil, "InvalidPolicy")
	}
	scope, id, ok := policyScope(form.ScopeType, form.ScopeId)
	if !ok || validatePolicyDocument(&form.Document) != nil {
		return c.Error(nil, "InvalidPolicy")
	}
	if err := validatePolicyScopeTarget(c.Db, scope, id); err != nil {
		return c.Error(nil, err.Error())
	}
	encoded, _ := json.Marshal(form.Document)
	item := model.ManagedDevicePolicy{}
	has, err := c.Db.Where("scope_type = ? AND scope_id = ?", scope, id).Get(&item)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if !has && (scope == model.DevicePolicyScopeGlobal || scope == model.DevicePolicyScopeGroup) {
		if err = materializeLegacyScope(c.Db, scope, id); err != nil {
			return c.Error(nil, err.Error())
		}
	}
	if has {
		var previous devicepolicy.Document
		if json.Unmarshal([]byte(item.Document), &previous) != nil {
			return c.Error(nil, "InvalidStoredPolicy")
		}
		if form.Document.ServerProfile.Key == nil {
			form.Document.ServerProfile.Key = previous.ServerProfile.Key
		}
		if form.Document.ServerProfile.PermanentPassword == nil {
			form.Document.ServerProfile.PermanentPassword = previous.ServerProfile.PermanentPassword
		}
		encoded, _ = json.Marshal(form.Document)
	}
	if has {
		item.Document = string(encoded)
		item.Enabled = form.Enabled
		item.Revision++
		_, err = c.Db.ID(item.Id).Cols("document", "enabled", "revision").Update(&item)
	} else {
		item = model.ManagedDevicePolicy{ScopeType: scope, ScopeId: id, Document: string(encoded), Revision: 1, Enabled: form.Enabled}
		_, err = c.Db.Insert(&item)
	}
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if err = bumpScopeDevices(c.Db, scope, id); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"revision": item.Revision}, "ok")
}

func (c *DevicesController) HandleDeletePolicy() mvc.Result {
	scope, id, ok := policyScope(c.Ctx.URLParam("scope_type"), c.Ctx.URLParamIntDefault("scope_id", 0))
	if !ok {
		return c.Error(nil, "InvalidPolicyScope")
	}
	if _, err := c.Db.Where("scope_type = ? AND scope_id = ?", scope, id).Delete(new(model.ManagedDevicePolicy)); err != nil {
		return c.Error(nil, err.Error())
	}
	if err := bumpScopeDevices(c.Db, scope, id); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}

func (c *DevicesController) HandlePolicyPreview() mvc.Result {
	id := c.Ctx.URLParamIntDefault("device_id", 0)
	device := model.Device{}
	has, err := c.Db.ID(id).Get(&device)
	if err != nil || !has {
		return c.Error(nil, "DeviceNotFound")
	}
	resolution, err := devicepolicy.ResolveForDevice(c.Db, &device)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	effective := resolution.Effective
	return c.Success(iris.Map{"device_id": id, "layers": resolution.Layers, "effective": iris.Map{
		"unattended_enabled": effective.UnattendedEnabled, "root_command": effective.RootCommand,
		"profile_enabled": effective.ProfileEnabled, "id_server": effective.IDServer, "relay_server": effective.RelayServer,
		"api_server": effective.APIServer, "key_set": effective.Key != "", "permanent_password_set": effective.PermanentPassword != "",
	}}, "ok")
}

func policyScope(scope string, id int) (string, int, bool) {
	if scope == model.DevicePolicyScopeGlobal {
		return scope, 0, id == 0
	}
	return scope, id, id > 0 && (scope == model.DevicePolicyScopeGroup || scope == model.DevicePolicyScopeDevice)
}

func validatePolicyScopeTarget(db *xorm.Engine, scope string, id int) error {
	if scope == model.DevicePolicyScopeGlobal {
		return nil
	}
	var target interface{} = new(model.Device)
	message := "DeviceNotFound"
	if scope == model.DevicePolicyScopeGroup {
		target = new(model.DeviceGroup)
		message = "DeviceGroupNotFound"
	}
	has, err := db.ID(id).Exist(target)
	if err != nil {
		return err
	}
	if !has {
		return errors.New(message)
	}
	return nil
}

func validatePolicyDocument(document *devicepolicy.Document) error {
	if value := document.Unattended.RootCommand; value != nil && *value != "auto" && *value != "su" && *value != "testsu" && *value != "disabled" {
		return iris.ErrNotFound
	}
	profile := document.ServerProfile
	for _, value := range []*string{profile.APIServer, profile.Key, profile.PermanentPassword} {
		if value != nil && len(*value) > 255 {
			return iris.ErrNotFound
		}
	}
	for _, value := range []*string{profile.IDServer, profile.RelayServer} {
		if value != nil && (len(*value) > 255 || strings.TrimSpace(*value) != *value || strings.ContainsAny(*value, "\r\n\t ")) {
			return iris.ErrNotFound
		}
	}
	if profile.APIServer != nil && *profile.APIServer != "" {
		parsed, err := url.ParseRequestURI(*profile.APIServer)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
			return iris.ErrNotFound
		}
	}
	return nil
}

func redactPolicy(document devicepolicy.Document) iris.Map {
	encoded, _ := json.Marshal(document)
	var result map[string]interface{}
	_ = json.Unmarshal(encoded, &result)
	profile, _ := result["server_profile"].(map[string]interface{})
	if profile != nil {
		key, keyExists := profile["key"]
		password, passwordExists := profile["permanent_password"]
		delete(profile, "key")
		delete(profile, "permanent_password")
		profile["key_set"] = keyExists && key != ""
		profile["permanent_password_set"] = passwordExists && password != ""
	}
	return result
}

func bumpDevice(db *xorm.Engine, id int) error {
	_, err := db.Table(new(model.Device)).ID(id).Incr("policy_revision").Update(map[string]interface{}{})
	return err
}
func bumpDeviceSession(db *xorm.Session, id int) error {
	_, err := db.Table(new(model.Device)).ID(id).Incr("policy_revision").Update(map[string]interface{}{})
	return err
}
func bumpGroupDevices(db *xorm.Engine, groupId int) error {
	members := make([]model.DeviceGroupMember, 0)
	if err := db.Where("group_id = ?", groupId).Find(&members); err != nil {
		return err
	}
	for _, member := range members {
		if err := bumpDevice(db, member.DeviceId); err != nil {
			return err
		}
	}
	return nil
}
func bumpScopeDevices(db *xorm.Engine, scope string, id int) error {
	if scope == model.DevicePolicyScopeDevice {
		return bumpDevice(db, id)
	}
	if scope == model.DevicePolicyScopeGroup {
		return bumpGroupDevices(db, id)
	}
	_, err := db.Table(new(model.Device)).Incr("policy_revision").Update(map[string]interface{}{})
	return err
}

func materializeLegacyScope(db *xorm.Engine, scope string, id int) error {
	devices := make([]model.Device, 0)
	query := db.Table(new(model.Device))
	if scope == model.DevicePolicyScopeGroup {
		query = query.Join("INNER", []string{new(model.DeviceGroupMember).TableName(), "m"}, "m.device_id = device.id").Where("m.group_id = ?", id)
	}
	if err := query.Find(&devices); err != nil {
		return err
	}
	for _, device := range devices {
		if err := materializeLegacyDevicePolicy(db, device.Id); err != nil {
			return err
		}
	}
	return nil
}

func materializeLegacyDevicePolicy(db *xorm.Engine, deviceID int) error {
	exists, err := db.Where("scope_type = ? AND scope_id = ?", model.DevicePolicyScopeDevice, deviceID).Exist(new(model.ManagedDevicePolicy))
	if err != nil || exists {
		return err
	}
	device := model.Device{}
	has, err := db.ID(deviceID).Get(&device)
	if err != nil || !has {
		return err
	}
	if !device.UnattendedEnabled && (device.RootCommand == "" || device.RootCommand == "auto") && !device.ProfileEnabled && device.ProfileIdServer == "" && device.ProfileRelayServer == "" && device.ProfileApiServer == "" && device.ProfileKey == "" && device.ProfilePassword == "" {
		return nil
	}
	return upsertDeviceSnapshot(db, &device, false)
}

func upsertDeviceSnapshot(db *xorm.Engine, device *model.Device, replace bool) error {
	unattendedEnabled, rootCommand := device.UnattendedEnabled, device.RootCommand
	if rootCommand == "" {
		rootCommand = "auto"
	}
	profileEnabled := device.ProfileEnabled
	idServer, relayServer, apiServer := device.ProfileIdServer, device.ProfileRelayServer, device.ProfileApiServer
	key, password := device.ProfileKey, device.ProfilePassword
	document := devicepolicy.Document{
		Unattended:    devicepolicy.Unattended{Enabled: &unattendedEnabled, RootCommand: &rootCommand},
		ServerProfile: devicepolicy.ServerProfile{Enabled: &profileEnabled, IDServer: &idServer, RelayServer: &relayServer, APIServer: &apiServer, Key: &key, PermanentPassword: &password},
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}
	item := model.ManagedDevicePolicy{}
	has, err := db.Where("scope_type = ? AND scope_id = ?", model.DevicePolicyScopeDevice, device.Id).Get(&item)
	if err != nil {
		return err
	}
	if has {
		if !replace {
			return nil
		}
		item.Document = string(encoded)
		item.Enabled = true
		item.Revision++
		_, err = db.ID(item.Id).Cols("document", "enabled", "revision").Update(&item)
		return err
	}
	_, err = db.Insert(&model.ManagedDevicePolicy{ScopeType: model.DevicePolicyScopeDevice, ScopeId: device.Id, Document: string(encoded), Revision: 1, Enabled: true})
	return err
}
