package admin

import (
	"errors"
	"rustdesk-api-server-pro/app/model"
	devicepolicy "rustdesk-api-server-pro/app/policy"
	"strings"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"xorm.io/xorm"
)

type serverProfileForm struct {
	Id                int     `json:"id"`
	Name              string  `json:"name"`
	IDServer          string  `json:"id_server"`
	RelayServer       string  `json:"relay_server"`
	ServerKey         string  `json:"server_key"`
	PermanentPassword *string `json:"permanent_password"`
	Enabled           bool    `json:"enabled"`
}

func registerPolicyRoutes(b mvc.BeforeActivation) {
	b.Handle("GET", "/devices/groups", "HandleGroups")
	b.Handle("POST", "/devices/groups", "HandleCreateGroup")
	b.Handle("PUT", "/devices/groups", "HandleUpdateGroup")
	b.Handle("DELETE", "/devices/groups", "HandleDeleteGroup")
	b.Handle("PUT", "/devices/groups/members", "HandleGroupMembers")
	b.Handle("GET", "/devices/server-profiles", "HandleServerProfiles")
	b.Handle("POST", "/devices/server-profiles", "HandleCreateServerProfile")
	b.Handle("PUT", "/devices/server-profiles", "HandleUpdateServerProfile")
	b.Handle("DELETE", "/devices/server-profiles", "HandleDeleteServerProfile")
	b.Handle("PUT", "/devices/server-profile-assignment", "HandleServerProfileAssignment")
	b.Handle("GET", "/devices/server-profile-preview", "HandleServerProfilePreview")
}

func (c *DevicesController) HandleServerProfiles() mvc.Result {
	profiles := make([]model.ServerProfile, 0)
	if err := c.Db.Asc("name", "id").Find(&profiles); err != nil {
		return c.Error(nil, err.Error())
	}
	result := make([]iris.Map, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, serverProfileResponse(profile))
	}
	globalProfileId, err := devicepolicy.AssignmentProfileID(c.Db, model.StrategyScopeGlobal, 0)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"profiles": result, "global_profile_id": globalProfileId}, "ok")
}

func (c *DevicesController) HandleCreateServerProfile() mvc.Result {
	form := serverProfileForm{}
	if c.Ctx.ReadJSON(&form) != nil || validateServerProfileForm(&form) != nil {
		return c.Error(nil, "InvalidServerProfile")
	}
	password := ""
	if form.PermanentPassword != nil {
		password = *form.PermanentPassword
	}
	ciphertext, err := devicepolicy.EncryptPassword(password, c.Cfg.ProvisioningSecretKey)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	profile := model.ServerProfile{
		Name: strings.TrimSpace(form.Name), IdServer: strings.TrimSpace(form.IDServer),
		RelayServer: strings.TrimSpace(form.RelayServer), ServerKey: strings.TrimSpace(form.ServerKey),
		PasswordCiphertext: ciphertext, Enabled: form.Enabled,
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = session.Insert(&profile); err == nil {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(serverProfileResponse(profile), "ok")
}

func (c *DevicesController) HandleUpdateServerProfile() mvc.Result {
	form := serverProfileForm{}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || validateServerProfileForm(&form) != nil {
		return c.Error(nil, "InvalidServerProfile")
	}
	profile := model.ServerProfile{}
	has, err := c.Db.ID(form.Id).Get(&profile)
	if err != nil || !has {
		return c.Error(nil, "ServerProfileNotFound")
	}
	profile.Name = strings.TrimSpace(form.Name)
	profile.IdServer = strings.TrimSpace(form.IDServer)
	profile.RelayServer = strings.TrimSpace(form.RelayServer)
	profile.ServerKey = strings.TrimSpace(form.ServerKey)
	profile.Enabled = form.Enabled
	if form.PermanentPassword != nil {
		profile.PasswordCiphertext, err = devicepolicy.EncryptPassword(*form.PermanentPassword, c.Cfg.ProvisioningSecretKey)
		if err != nil {
			return c.Error(nil, err.Error())
		}
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = session.ID(profile.Id).Cols("name", "id_server", "relay_server", "server_key", "password_ciphertext", "enabled").Update(&profile); err == nil {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(serverProfileResponse(profile), "ok")
}

func (c *DevicesController) HandleDeleteServerProfile() mvc.Result {
	id := c.Ctx.URLParamIntDefault("id", 0)
	if id <= 0 {
		return c.Error(nil, "InvalidServerProfile")
	}
	inUse, err := c.Db.Where("profile_id = ?", id).Exist(new(model.ServerProfileAssignment))
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if inUse {
		return c.Error(nil, "ServerProfileInUse")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	deleted, err := session.ID(id).Delete(new(model.ServerProfile))
	if err == nil && deleted > 0 {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if deleted == 0 {
		session.Rollback()
		return c.Error(nil, "ServerProfileNotFound")
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}

func (c *DevicesController) HandleServerProfileAssignment() mvc.Result {
	var form struct {
		ScopeType string `json:"scope_type"`
		ScopeId   int    `json:"scope_id"`
		ProfileId int    `json:"profile_id"`
	}
	if c.Ctx.ReadJSON(&form) != nil || devicepolicy.ValidateScope(form.ScopeType, form.ScopeId) != nil || form.ProfileId < 0 {
		return c.Error(nil, "InvalidStrategyAssignment")
	}
	if err := validateStrategyScopeTarget(c.Db, form.ScopeType, form.ScopeId); err != nil {
		return c.Error(nil, err.Error())
	}
	if form.ProfileId > 0 {
		has, err := c.Db.ID(form.ProfileId).Where("enabled = ?", true).Exist(new(model.ServerProfile))
		if err != nil || !has {
			return c.Error(nil, "ServerProfileNotFound")
		}
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err := session.Where("scope_type = ? AND scope_id = ?", form.ScopeType, form.ScopeId).Delete(new(model.ServerProfileAssignment)); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if form.ProfileId > 0 {
		assignment := model.ServerProfileAssignment{ScopeType: form.ScopeType, ScopeId: form.ScopeId, ProfileId: form.ProfileId}
		if _, err := session.Insert(&assignment); err != nil {
			session.Rollback()
			return c.Error(nil, err.Error())
		}
	}
	revision, err := devicepolicy.NextRevision(session)
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"revision": revision}, "ok")
}

func (c *DevicesController) HandleServerProfilePreview() mvc.Result {
	id := c.Ctx.URLParamIntDefault("device_id", 0)
	device := model.Device{}
	has, err := c.Db.ID(id).Get(&device)
	if err != nil || !has {
		return c.Error(nil, "DeviceNotFound")
	}
	effective, err := devicepolicy.ResolveForDevice(c.Db, &device)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(effectivePolicyResponse(effective), "ok")
}

func (c *DevicesController) HandleGroups() mvc.Result {
	groups := make([]model.DeviceGroup, 0)
	if err := c.Db.Asc("name", "id").Find(&groups); err != nil {
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
		profileId, err := devicepolicy.AssignmentProfileID(c.Db, model.StrategyScopeGroup, group.Id)
		if err != nil {
			return c.Error(nil, err.Error())
		}
		result = append(result, iris.Map{
			"id": group.Id, "name": group.Name, "enabled": group.Enabled,
			"member_count": len(memberIDs), "device_ids": memberIDs, "profile_id": profileId,
		})
	}
	return c.Success(result, "ok")
}

func (c *DevicesController) HandleCreateGroup() mvc.Result {
	var form struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if c.Ctx.ReadJSON(&form) != nil || strings.TrimSpace(form.Name) == "" || len(strings.TrimSpace(form.Name)) > 100 {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	group := model.DeviceGroup{Name: strings.TrimSpace(form.Name), Enabled: form.Enabled}
	if _, err := c.Db.Insert(&group); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"id": group.Id}, "ok")
}

func (c *DevicesController) HandleUpdateGroup() mvc.Result {
	var form struct {
		Id      int    `json:"id"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || strings.TrimSpace(form.Name) == "" || len(strings.TrimSpace(form.Name)) > 100 {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	updated, err := session.ID(form.Id).Cols("name", "enabled").Update(&model.DeviceGroup{Name: strings.TrimSpace(form.Name), Enabled: form.Enabled})
	if err == nil && updated > 0 {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if updated == 0 {
		session.Rollback()
		return c.Error(nil, "DeviceGroupNotFound")
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}

func (c *DevicesController) HandleDeleteGroup() mvc.Result {
	id := c.Ctx.URLParamIntDefault("id", 0)
	if id <= 0 {
		return c.Error(nil, "InvalidDeviceGroup")
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
	if _, err := session.Where("scope_type = ? AND scope_id = ?", model.StrategyScopeGroup, id).Delete(new(model.ServerProfileAssignment)); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	deleted, err := session.ID(id).Delete(new(model.DeviceGroup))
	if err == nil && deleted > 0 {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if deleted == 0 {
		session.Rollback()
		return c.Error(nil, "DeviceGroupNotFound")
	}
	if err = session.Commit(); err != nil {
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
	if has, err := c.Db.ID(form.GroupId).Exist(new(model.DeviceGroup)); err != nil || !has {
		return c.Error(nil, "DeviceGroupNotFound")
	}
	unique := make(map[int]struct{}, len(form.DeviceIds))
	for _, id := range form.DeviceIds {
		if id <= 0 {
			return c.Error(nil, "InvalidGroupMembers")
		}
		unique[id] = struct{}{}
	}
	ids := make([]int, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		count, err := c.Db.In("id", ids).Count(new(model.Device))
		if err != nil || int(count) != len(ids) {
			return c.Error(nil, "DeviceNotFound")
		}
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
	if len(ids) > 0 {
		if _, err := session.In("device_id", ids).Delete(new(model.DeviceGroupMember)); err != nil {
			session.Rollback()
			return c.Error(nil, err.Error())
		}
		for _, id := range ids {
			if _, err := session.Insert(&model.DeviceGroupMember{GroupId: form.GroupId, DeviceId: id}); err != nil {
				session.Rollback()
				return c.Error(nil, err.Error())
			}
		}
	}
	if _, err := devicepolicy.NextRevision(session); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err := session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}

func validateServerProfileForm(form *serverProfileForm) error {
	form.Name = strings.TrimSpace(form.Name)
	form.IDServer = strings.TrimSpace(form.IDServer)
	form.RelayServer = strings.TrimSpace(form.RelayServer)
	form.ServerKey = strings.TrimSpace(form.ServerKey)
	if form.Name == "" || len(form.Name) > 100 || form.IDServer == "" {
		return errors.New("invalid server profile")
	}
	for _, value := range []string{form.IDServer, form.RelayServer, form.ServerKey} {
		if len(value) > 255 || strings.ContainsAny(value, "\r\n\t ") {
			return errors.New("invalid server profile")
		}
	}
	if form.PermanentPassword != nil && len(*form.PermanentPassword) > 255 {
		return errors.New("invalid server profile")
	}
	return nil
}

func validateStrategyScopeTarget(db *xorm.Engine, scope string, id int) error {
	if scope == model.StrategyScopeGlobal {
		return nil
	}
	target := any(new(model.Device))
	message := "DeviceNotFound"
	if scope == model.StrategyScopeGroup {
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

func serverProfileResponse(profile model.ServerProfile) iris.Map {
	return iris.Map{
		"id": profile.Id, "name": profile.Name, "id_server": profile.IdServer,
		"relay_server": profile.RelayServer, "server_key": profile.ServerKey,
		"password_set": profile.PasswordCiphertext != "", "enabled": profile.Enabled,
	}
}

func effectivePolicyResponse(effective devicepolicy.Effective) iris.Map {
	return iris.Map{
		"revision":           effective.Revision,
		"unattended_enabled": effective.UnattendedEnabled,
		"root_command":       effective.RootCommand,
		"profile_enabled":    effective.ProfileEnabled,
		"profile_id":         effective.Profile.ID,
		"profile_name":       effective.Profile.Name,
		"profile_source":     effective.ProfileSource,
		"id_server":          effective.Profile.IDServer,
		"relay_server":       effective.Profile.RelayServer,
		"server_key":         effective.Profile.ServerKey,
		"password_set":       effective.Profile.PasswordCiphertext != "",
	}
}
