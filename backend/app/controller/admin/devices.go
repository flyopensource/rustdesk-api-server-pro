package admin

import (
	"rustdesk-api-server-pro/app/model"
	devicepolicy "rustdesk-api-server-pro/app/policy"
	"rustdesk-api-server-pro/config"
	"rustdesk-api-server-pro/db"
	"strings"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"xorm.io/xorm"
)

type DevicesController struct {
	basicController
	Cfg *config.ServerConfig
}

func (c *DevicesController) BeforeActivation(b mvc.BeforeActivation) {
	b.Handle("GET", "/devices/list", "HandleList")
	b.Handle("PUT", "/devices/unattended", "HandleUnattended")
	registerPolicyRoutes(b)
}

func (c *DevicesController) HandleUnattended() mvc.Result {
	var form struct {
		Id          int    `json:"id"`
		Enabled     bool   `json:"enabled"`
		RootCommand string `json:"root_command"`
	}
	if err := c.Ctx.ReadJSON(&form); err != nil || form.Id <= 0 {
		return c.Error(nil, "DataError")
	}
	form.RootCommand = strings.TrimSpace(form.RootCommand)
	if !devicepolicy.ValidRootCommand(form.RootCommand) {
		return c.Error(nil, "InvalidRootCommand")
	}
	device := model.Device{}
	has, err := c.Db.ID(form.Id).Get(&device)
	if err != nil || !has {
		return c.Error(nil, "DeviceNotFound")
	}
	session := c.Db.NewSession()
	defer session.Close()
	if err = session.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = session.Table(new(model.Device)).ID(form.Id).Update(map[string]interface{}{
		"unattended_enabled": form.Enabled,
		"root_command":       form.RootCommand,
	}); err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	revision, err := devicepolicy.NextRevision(session)
	if err != nil {
		session.Rollback()
		return c.Error(nil, err.Error())
	}
	if err = session.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"policy_revision": revision}, "ok")
}

func (c *DevicesController) HandleList() mvc.Result {
	currentPage := c.Ctx.URLParamIntDefault("current", 1)
	pageSize := c.Ctx.URLParamIntDefault("size", 10)
	hostname := c.Ctx.URLParamDefault("hostname", "")
	username := c.Ctx.URLParamDefault("username", "")
	rustdesk_id := c.Ctx.URLParamDefault("rustdesk_id", "")
	query := func() *xorm.Session {
		q := c.Db.Table(&model.Device{})

		if hostname != "" {
			q.Where("hostname LIKE ?", "%"+hostname+"%")
		}
		if username != "" {
			q.Where("username LIKE ?", "%"+username+"%")
		}
		if rustdesk_id != "" {
			q.Where("rustdesk_id LIKE ?", "%"+rustdesk_id+"%")
		}
		q.Asc("username")
		return q
	}

	pagination := db.NewPagination(currentPage, pageSize)
	deviceList := make([]model.Device, 0)

	err := pagination.Paginate(query, &model.Device{}, &deviceList)
	if err != nil {
		return c.Error(nil, err.Error())
	}

	list := make([]iris.Map, 0)
	for _, a := range deviceList {
		effective, resolveErr := devicepolicy.ResolveForDevice(c.Db, &a)
		if resolveErr != nil {
			return c.Error(nil, resolveErr.Error())
		}
		groupName := ""
		groupEnabled := false
		if a.StrategyGroupId > 0 {
			group := model.DeviceGroup{}
			if _, groupErr := c.Db.ID(a.StrategyGroupId).Get(&group); groupErr != nil {
				return c.Error(nil, groupErr.Error())
			}
			groupName, groupEnabled = group.Name, group.Enabled
		}
		list = append(list, iris.Map{
			"id":                       a.Id,
			"rustdesk_id":              a.RustdeskId,
			"hostname":                 a.Hostname,
			"username":                 a.Username,
			"uuid":                     a.Uuid,
			"version":                  a.Version,
			"os":                       a.Os,
			"memory":                   a.Memory,
			"created_at":               a.CreatedAt.Format(config.TimeFormat),
			"is_online":                a.IsOnline,
			"unattended_enabled":       a.UnattendedEnabled,
			"root_command":             a.RootCommand,
			"policy_revision":          effective.Revision,
			"applied_revision":         a.AppliedRevision,
			"unattended_status":        a.UnattendedStatus,
			"root_executor":            a.RootExecutor,
			"root_available":           a.RootAvailable,
			"screen_capture_ready":     a.ScreenCaptureReady,
			"accessibility_ready":      a.AccessibilityReady,
			"service_running":          a.ServiceRunning,
			"unattended_error":         a.UnattendedError,
			"unattended_reported_at":   a.UnattendedReportedAt.Format(config.TimeFormat),
			"group_id":                 a.StrategyGroupId,
			"group_name":               groupName,
			"group_enabled":            groupEnabled,
			"profile_assignment_id":    a.StrategyProfileId,
			"profile_enabled":          effective.ProfileEnabled,
			"profile_id":               effective.Profile.ID,
			"profile_name":             effective.Profile.Name,
			"profile_source":           effective.ProfileSource,
			"profile_id_server":        effective.Profile.IDServer,
			"profile_relay_server":     effective.Profile.RelayServer,
			"profile_key_set":          effective.Profile.ServerKey != "",
			"profile_password_set":     effective.Profile.PasswordCiphertext != "",
			"profile_applied_revision": a.ProfileAppliedRevision,
			"profile_active_source":    a.ProfileActiveSource,
			"profile_connected":        a.ProfileConnected,
			"profile_reported_at":      a.ProfileReportedAt.Format(config.TimeFormat),
		})
	}
	return c.Success(iris.Map{
		"total":   pagination.TotalCount,
		"records": list,
		"current": currentPage,
		"size":    pageSize,
	}, "ok")
}
