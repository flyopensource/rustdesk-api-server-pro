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
	Id          int    `json:"id"`
	Name        string `json:"name"`
	IDServer    string `json:"id_server"`
	RelayServer string `json:"relay_server"`
	ServerKey   string `json:"server_key"`
	Enabled     bool   `json:"enabled"`
}

type deviceGroupForm struct {
	Id                int     `json:"id"`
	Name              string  `json:"name"`
	Enabled           bool    `json:"enabled"`
	IsDefault         bool    `json:"is_default"`
	ProfileId         int     `json:"profile_id"`
	UnattendedEnabled bool    `json:"unattended_enabled"`
	PermanentPassword *string `json:"permanent_password"`
	ClearPassword     bool    `json:"clear_password"`
	RootCommand       string  `json:"root_command"`
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
	b.Handle("GET", "/devices/policy-preview", "HandlePolicyPreview")
}

func (c *DevicesController) HandleServerProfiles() mvc.Result {
	profiles := make([]model.ServerProfile, 0)
	if err := c.Db.Asc("name", "id").Find(&profiles); err != nil {
		return c.Error(nil, err.Error())
	}
	result := make([]iris.Map, 0, len(profiles))
	for _, profile := range profiles {
		groupCount, err := c.Db.Where("profile_id = ?", profile.Id).Count(new(model.DeviceGroup))
		if err != nil {
			return c.Error(nil, err.Error())
		}
		item := serverProfileResponse(profile)
		item["group_count"] = groupCount
		result = append(result, item)
	}
	return c.Success(iris.Map{"profiles": result}, "ok")
}

func (c *DevicesController) HandleCreateServerProfile() mvc.Result {
	form := serverProfileForm{}
	if c.Ctx.ReadJSON(&form) != nil || validateServerProfileForm(&form) != nil {
		return c.Error(nil, "InvalidServerProfile")
	}
	profile := model.ServerProfile{
		Name: strings.TrimSpace(form.Name), IdServer: strings.TrimSpace(form.IDServer),
		RelayServer: strings.TrimSpace(form.RelayServer), ServerKey: strings.TrimSpace(form.ServerKey),
		Enabled: form.Enabled,
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err := session.Insert(&profile); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if _, err := devicepolicy.NextRevision(session); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err := session.Commit(); err != nil {
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
	updated := model.ServerProfile{
		Name: strings.TrimSpace(form.Name), IdServer: strings.TrimSpace(form.IDServer),
		RelayServer: strings.TrimSpace(form.RelayServer), ServerKey: strings.TrimSpace(form.ServerKey),
		Enabled: form.Enabled,
	}
	if profile.Name == updated.Name && profile.IdServer == updated.IdServer &&
		profile.RelayServer == updated.RelayServer && profile.ServerKey == updated.ServerKey &&
		profile.Enabled == updated.Enabled {
		return c.Success(serverProfileResponse(profile), "ok")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = session.ID(profile.Id).Cols("name", "id_server", "relay_server", "server_key", "enabled").Update(&updated); err == nil {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	updated.Id = profile.Id
	return c.Success(serverProfileResponse(updated), "ok")
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

func (c *DevicesController) HandlePolicyPreview() mvc.Result {
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
	state, err := devicepolicy.CurrentState(c.Db)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	unassignedCount, err := c.Db.Where("strategy_group_id = ?", 0).Count(new(model.Device))
	if err != nil {
		return c.Error(nil, err.Error())
	}
	result := make([]iris.Map, 0, len(groups))
	for _, group := range groups {
		item, responseErr := c.deviceGroupResponse(group, state.DefaultGroupId == group.Id, unassignedCount)
		if responseErr != nil {
			return c.Error(nil, responseErr.Error())
		}
		result = append(result, item)
	}
	return c.Success(iris.Map{
		"groups": result, "default_group_id": state.DefaultGroupId, "unassigned_device_count": unassignedCount,
	}, "ok")
}

func (c *DevicesController) HandleCreateGroup() mvc.Result {
	form := deviceGroupForm{}
	if c.Ctx.ReadJSON(&form) != nil || validateDeviceGroupForm(&form) != nil {
		return c.Error(nil, "InvalidDeviceGroup")
	}
	if err := validateSelectableProfile(c.Db, form.ProfileId); err != nil {
		return c.Error(nil, err.Error())
	}
	password := ""
	if form.PermanentPassword != nil {
		password = *form.PermanentPassword
	}
	ciphertext, err := devicepolicy.EncryptPassword(password, c.Cfg.ProvisioningSecretKey)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	group := model.DeviceGroup{
		Name: strings.TrimSpace(form.Name), Enabled: form.Enabled, ProfileId: form.ProfileId,
		UnattendedEnabled: form.UnattendedEnabled, PasswordCiphertext: ciphertext,
		RootCommand: form.RootCommand,
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = session.Insert(&group); err == nil {
		_, err = updateGroupDefault(session, group.Id, form.IsDefault)
	}
	if err == nil {
		_, err = devicepolicy.NextRevision(session)
	}
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
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
	ciphertext := group.PasswordCiphertext
	if form.ClearPassword {
		ciphertext = ""
	} else if form.PermanentPassword != nil && *form.PermanentPassword != "" {
		ciphertext, err = devicepolicy.EncryptPassword(*form.PermanentPassword, c.Cfg.ProvisioningSecretKey)
		if err != nil {
			return c.Error(nil, err.Error())
		}
	}
	updated := model.DeviceGroup{
		Name: strings.TrimSpace(form.Name), Enabled: form.Enabled, ProfileId: form.ProfileId,
		UnattendedEnabled: form.UnattendedEnabled, PasswordCiphertext: ciphertext,
		RootCommand: form.RootCommand,
	}
	groupChanged := group.Name != updated.Name || group.Enabled != updated.Enabled ||
		group.ProfileId != updated.ProfileId || group.UnattendedEnabled != updated.UnattendedEnabled ||
		group.PasswordCiphertext != updated.PasswordCiphertext || group.RootCommand != updated.RootCommand
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if groupChanged {
		_, err = session.ID(form.Id).Cols(
			"name", "enabled", "profile_id", "unattended_enabled", "password_ciphertext", "root_command",
		).Update(&updated)
	}
	defaultChanged := false
	if err == nil {
		defaultChanged, err = updateGroupDefault(session, form.Id, form.IsDefault)
	}
	if err == nil && (groupChanged || defaultChanged) {
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

func updateGroupDefault(session *xorm.Session, groupID int, isDefault bool) (bool, error) {
	state := model.StrategyState{Id: 1}
	has, err := session.ID(state.Id).Get(&state)
	if err != nil {
		return false, err
	}
	if !has {
		if !isDefault {
			return false, nil
		}
		state.DefaultGroupId = groupID
		_, err = session.Insert(&state)
		return err == nil, err
	}
	desired := state.DefaultGroupId
	if isDefault {
		desired = groupID
	} else if desired == groupID {
		desired = 0
	}
	if desired == state.DefaultGroupId {
		return false, nil
	}
	_, err = session.ID(state.Id).Cols("default_group_id").Update(&model.StrategyState{DefaultGroupId: desired})
	return err == nil, err
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
	if _, err := session.ID(1).Where("default_group_id = ?", id).Cols("default_group_id").Update(&model.StrategyState{DefaultGroupId: 0}); err != nil {
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
	if err := validateSelectableGroup(c.Db, form.GroupId); err != nil {
		return c.Error(nil, err.Error())
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
	return nil
}

func validateDeviceGroupForm(form *deviceGroupForm) error {
	form.Name = strings.TrimSpace(form.Name)
	form.RootCommand = strings.TrimSpace(form.RootCommand)
	if form.RootCommand == "" {
		form.RootCommand = "auto"
	}
	if form.Name == "" || len(form.Name) > 100 || form.ProfileId <= 0 || form.IsDefault && !form.Enabled ||
		!devicepolicy.ValidRootCommand(form.RootCommand) ||
		(form.PermanentPassword != nil && len(*form.PermanentPassword) > 255) ||
		(form.ClearPassword && form.PermanentPassword != nil && *form.PermanentPassword != "") {
		return errors.New("invalid device group")
	}
	return nil
}

func validateSelectableProfile(db *xorm.Engine, id int) error {
	if id <= 0 {
		return errors.New("ServerProfileNotFound")
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

func validateSelectableGroup(db *xorm.Engine, id int) error {
	if id == 0 {
		return nil
	}
	group := model.DeviceGroup{}
	has, err := db.ID(id).Where("enabled = ?", true).Get(&group)
	if err != nil {
		return err
	}
	if !has {
		return errors.New("DeviceGroupNotFound")
	}
	return validateSelectableProfile(db, group.ProfileId)
}

func serverProfileInUse(db *xorm.Engine, id int) (bool, error) {
	return db.Where("profile_id = ?", id).Exist(new(model.DeviceGroup))
}

func serverProfileResponse(profile model.ServerProfile) iris.Map {
	return iris.Map{
		"id": profile.Id, "name": profile.Name, "id_server": profile.IdServer,
		"relay_server": profile.RelayServer, "server_key": profile.ServerKey,
		"enabled": profile.Enabled, "group_count": 0,
	}
}

func (c *DevicesController) deviceGroupResponse(group model.DeviceGroup, isDefault bool, unassignedCount int64) (iris.Map, error) {
	memberCount, err := c.Db.Where("strategy_group_id = ?", group.Id).Count(new(model.Device))
	if err != nil {
		return nil, err
	}
	profile := model.ServerProfile{}
	profileFound, err := c.Db.ID(group.ProfileId).Get(&profile)
	if err != nil {
		return nil, err
	}
	passwordSet := group.PasswordCiphertext != ""
	return iris.Map{
		"id": group.Id, "name": group.Name, "enabled": group.Enabled,
		"member_count": memberCount, "default_coverage_count": unassignedCount, "profile_id": group.ProfileId,
		"profile_name": profile.Name, "profile_enabled": profileFound && profile.Enabled,
		"unattended_enabled": group.UnattendedEnabled, "password_set": passwordSet,
		"root_command": group.RootCommand, "is_default": isDefault,
		"configuration_complete": profileFound && profile.Enabled && (!group.UnattendedEnabled || passwordSet),
	}, nil
}

func effectivePolicyResponse(effective devicepolicy.Effective) iris.Map {
	return iris.Map{
		"revision":           effective.Revision,
		"group_id":           effective.GroupID,
		"group_name":         effective.GroupName,
		"group_source":       effective.GroupSource,
		"group_warning":      effective.GroupWarning,
		"unattended_enabled": effective.UnattendedEnabled,
		"root_command":       effective.RootCommand,
		"profile_enabled":    effective.ProfileEnabled,
		"profile_id":         effective.Profile.ID,
		"profile_name":       effective.Profile.Name,
		"profile_source":     effective.ProfileSource,
		"id_server":          effective.Profile.IDServer,
		"relay_server":       effective.Profile.RelayServer,
		"server_key":         effective.Profile.ServerKey,
		"password_set":       effective.PasswordCiphertext != "",
	}
}
