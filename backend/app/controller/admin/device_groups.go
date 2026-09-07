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

type deviceGroupForm struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	ProfileId int    `json:"profile_id"`
}

func registerPolicyRoutes(b mvc.BeforeActivation) {
	b.Handle("GET", "/devices/groups", "HandleGroups")
	b.Handle("POST", "/devices/groups", "HandleCreateGroup")
	b.Handle("PUT", "/devices/groups", "HandleUpdateGroup")
	b.Handle("DELETE", "/devices/groups", "HandleDeleteGroup")
	b.Handle("PUT", "/devices/group", "HandleDeviceGroup")
	b.Handle("GET", "/devices/server-profiles", "HandleServerProfiles")
	b.Handle("POST", "/devices/server-profiles", "HandleCreateServerProfile")
	b.Handle("PUT", "/devices/server-profiles", "HandleUpdateServerProfile")
	b.Handle("DELETE", "/devices/server-profiles", "HandleDeleteServerProfile")
	b.Handle("PUT", "/devices/server-profile", "HandleDeviceServerProfile")
	b.Handle("PUT", "/devices/global-server-profile", "HandleGlobalServerProfile")
	b.Handle("GET", "/devices/server-profile-preview", "HandleServerProfilePreview")
}

func (c *DevicesController) HandleServerProfiles() mvc.Result {
	profiles := make([]model.ServerProfile, 0)
	if err := c.Db.Asc("name", "id").Find(&profiles); err != nil {
		return c.Error(nil, err.Error())
	}
	state, err := devicepolicy.CurrentState(c.Db)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	result := make([]iris.Map, 0, len(profiles))
	for _, profile := range profiles {
		groupCount, countErr := c.Db.Where("profile_id = ?", profile.Id).Count(new(model.DeviceGroup))
		if countErr != nil {
			return c.Error(nil, countErr.Error())
		}
		deviceCount, countErr := c.Db.Where("strategy_profile_id = ?", profile.Id).Count(new(model.Device))
		if countErr != nil {
			return c.Error(nil, countErr.Error())
		}
		item := serverProfileResponse(profile)
		item["is_global_default"] = state.GlobalProfileId == profile.Id
		item["group_count"] = groupCount
		item["device_count"] = deviceCount
		result = append(result, item)
	}
	return c.Success(iris.Map{"profiles": result, "global_profile_id": state.GlobalProfileId}, "ok")
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
	inUse, err := serverProfileInUse(c.Db, id)
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

func (c *DevicesController) HandleDeviceServerProfile() mvc.Result {
	var form struct {
		Id        int `json:"id"`
		ProfileId int `json:"profile_id"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || form.ProfileId < 0 {
		return c.Error(nil, "InvalidServerProfile")
	}
	if err := validateSelectableProfile(c.Db, form.ProfileId); err != nil {
		return c.Error(nil, err.Error())
	}
	device := model.Device{}
	has, err := c.Db.ID(form.Id).Get(&device)
	if err != nil || !has {
		return c.Error(nil, "DeviceNotFound")
	}
	if device.StrategyProfileId == form.ProfileId {
		effective, resolveErr := devicepolicy.ResolveForDevice(c.Db, &device)
		if resolveErr != nil {
			return c.Error(nil, resolveErr.Error())
		}
		return c.Success(effectivePolicyResponse(effective), "ok")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = session.ID(device.Id).Cols("strategy_profile_id").Update(&model.Device{StrategyProfileId: form.ProfileId}); err == nil {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	device.StrategyProfileId = form.ProfileId
	effective, err := devicepolicy.ResolveForDevice(c.Db, &device)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(effectivePolicyResponse(effective), "ok")
}

func (c *DevicesController) HandleGlobalServerProfile() mvc.Result {
	var form struct {
		ProfileId int `json:"profile_id"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.ProfileId < 0 {
		return c.Error(nil, "InvalidServerProfile")
	}
	if err := validateSelectableProfile(c.Db, form.ProfileId); err != nil {
		return c.Error(nil, err.Error())
	}
	state, err := devicepolicy.CurrentState(c.Db)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if state.GlobalProfileId == form.ProfileId {
		return c.Success(iris.Map{"revision": state.Revision}, "ok")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = session.ID(state.Id).Cols("global_profile_id").Update(&model.StrategyState{GlobalProfileId: form.ProfileId}); err == nil {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
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
		memberCount, err := c.Db.Where("strategy_group_id = ?", group.Id).Count(new(model.Device))
		if err != nil {
			return c.Error(nil, err.Error())
		}
		result = append(result, iris.Map{
			"id": group.Id, "name": group.Name, "enabled": group.Enabled,
			"member_count": memberCount, "profile_id": group.ProfileId,
		})
	}
	return c.Success(result, "ok")
}

func (c *DevicesController) HandleCreateGroup() mvc.Result {
	form := deviceGroupForm{}
	if c.Ctx.ReadJSON(&form) != nil || validateDeviceGroupForm(&form) != nil {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	if err := validateSelectableProfile(c.Db, form.ProfileId); err != nil {
		return c.Error(nil, err.Error())
	}
	group := model.DeviceGroup{
		Name: strings.TrimSpace(form.Name), Enabled: form.Enabled, ProfileId: form.ProfileId,
	}
	if _, err := c.Db.Insert(&group); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"id": group.Id}, "ok")
}

func (c *DevicesController) HandleUpdateGroup() mvc.Result {
	form := deviceGroupForm{}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || validateDeviceGroupForm(&form) != nil {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	if err := validateSelectableProfile(c.Db, form.ProfileId); err != nil {
		return c.Error(nil, err.Error())
	}
	group := model.DeviceGroup{}
	has, err := c.Db.ID(form.Id).Get(&group)
	if err != nil || !has {
		return c.Error(nil, "DeviceGroupNotFound")
	}
	name := strings.TrimSpace(form.Name)
	if group.Name == name && group.Enabled == form.Enabled && group.ProfileId == form.ProfileId {
		return c.Success(nil, "ok")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	updated := model.DeviceGroup{Name: name, Enabled: form.Enabled, ProfileId: form.ProfileId}
	if _, err = session.ID(form.Id).Cols("name", "enabled", "profile_id").Update(&updated); err == nil {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
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
	if has, err := c.Db.ID(id).Exist(new(model.DeviceGroup)); err != nil || !has {
		return c.Error(nil, "DeviceGroupNotFound")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err := session.Where("strategy_group_id = ?", id).Cols("strategy_group_id").Update(&model.Device{StrategyGroupId: 0}); err != nil {
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

func (c *DevicesController) HandleDeviceGroup() mvc.Result {
	var form struct {
		Id      int `json:"id"`
		GroupId int `json:"group_id"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || form.GroupId < 0 {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	if form.GroupId > 0 {
		group := model.DeviceGroup{}
		has, err := c.Db.ID(form.GroupId).Where("enabled = ?", true).Get(&group)
		if err != nil || !has {
			return c.Error(nil, "DeviceGroupNotFound")
		}
	}
	device := model.Device{}
	has, err := c.Db.ID(form.Id).Get(&device)
	if err != nil || !has {
		return c.Error(nil, "DeviceNotFound")
	}
	if device.StrategyGroupId == form.GroupId {
		effective, resolveErr := devicepolicy.ResolveForDevice(c.Db, &device)
		if resolveErr != nil {
			return c.Error(nil, resolveErr.Error())
		}
		return c.Success(effectivePolicyResponse(effective), "ok")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = session.ID(device.Id).Cols("strategy_group_id").Update(&model.Device{StrategyGroupId: form.GroupId}); err == nil {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	device.StrategyGroupId = form.GroupId
	effective, err := devicepolicy.ResolveForDevice(c.Db, &device)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(effectivePolicyResponse(effective), "ok")
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

func validateDeviceGroupForm(form *deviceGroupForm) error {
	form.Name = strings.TrimSpace(form.Name)
	if form.Name == "" || len(form.Name) > 100 || form.ProfileId < 0 {
		return errors.New("invalid device group")
	}
	return nil
}

func validateSelectableProfile(db *xorm.Engine, id int) error {
	if id == 0 {
		return nil
	}
	has, err := db.ID(id).Where("enabled = ?", true).Exist(new(model.ServerProfile))
	if err != nil {
		return err
	}
	if !has {
		return errors.New("ServerProfileNotFound")
	}
	return nil
}

func serverProfileInUse(db *xorm.Engine, id int) (bool, error) {
	for _, reference := range []struct {
		column string
		model  any
	}{
		{"global_profile_id", new(model.StrategyState)},
		{"profile_id", new(model.DeviceGroup)},
		{"strategy_profile_id", new(model.Device)},
	} {
		has, err := db.Where(reference.column+" = ?", id).Exist(reference.model)
		if err != nil || has {
			return has, err
		}
	}
	return false, nil
}

func serverProfileResponse(profile model.ServerProfile) iris.Map {
	return iris.Map{
		"id": profile.Id, "name": profile.Name, "id_server": profile.IdServer,
		"relay_server": profile.RelayServer, "server_key": profile.ServerKey,
		"password_set": profile.PasswordCiphertext != "", "enabled": profile.Enabled,
		"is_global_default": false, "group_count": 0, "device_count": 0,
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
