package admin

import (
	"net/url"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/config"
	"rustdesk-api-server-pro/db"
	"strings"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"xorm.io/xorm"
)

type DevicesController struct {
	basicController
}

func (c *DevicesController) BeforeActivation(b mvc.BeforeActivation) {
	b.Handle("GET", "/devices/list", "HandleList")
	b.Handle("PUT", "/devices/unattended", "HandleUnattended")
	b.Handle("PUT", "/devices/profile", "HandleProfile")
	registerPolicyRoutes(b)
}

func (c *DevicesController) HandleProfile() mvc.Result {
	var form struct {
		Id                int     `json:"id"`
		Enabled           bool    `json:"enabled"`
		IDServer          string  `json:"id_server"`
		RelayServer       string  `json:"relay_server"`
		APIServer         string  `json:"api_server"`
		Key               *string `json:"key"`
		PermanentPassword *string `json:"permanent_password"`
	}
	if err := c.Ctx.ReadJSON(&form); err != nil || form.Id <= 0 {
		return c.Error(nil, "DataError")
	}
	form.IDServer = strings.TrimSpace(form.IDServer)
	form.RelayServer = strings.TrimSpace(form.RelayServer)
	form.APIServer = strings.TrimSpace(form.APIServer)
	if len(form.IDServer) > 255 || len(form.RelayServer) > 255 || len(form.APIServer) > 255 ||
		strings.ContainsAny(form.IDServer+form.RelayServer, "\r\n\t ") {
		return c.Error(nil, "InvalidServerProfile")
	}
	if form.Key != nil && len(*form.Key) > 255 || form.PermanentPassword != nil && len(*form.PermanentPassword) > 255 {
		return c.Error(nil, "InvalidServerProfile")
	}
	if form.APIServer != "" {
		parsed, err := url.ParseRequestURI(form.APIServer)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
			return c.Error(nil, "InvalidAPIServer")
		}
	}
	device := model.Device{}
	has, err := c.Db.ID(form.Id).Get(&device)
	if err != nil || !has {
		return c.Error(nil, "DeviceNotFound")
	}
	if form.Enabled && form.IDServer == "" {
		return c.Error(nil, "IDServerRequired")
	}
	device.PolicyRevision++
	updates := map[string]interface{}{
		"profile_enabled": form.Enabled, "profile_id_server": form.IDServer,
		"profile_relay_server": form.RelayServer, "profile_api_server": form.APIServer,
		"policy_revision": device.PolicyRevision,
	}
	if form.Key != nil {
		updates["profile_key"] = *form.Key
	}
	if form.PermanentPassword != nil {
		updates["profile_password"] = *form.PermanentPassword
	}
	_, err = c.Db.Table(new(model.Device)).ID(form.Id).Update(updates)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	device.ProfileEnabled = form.Enabled
	device.ProfileIdServer = form.IDServer
	device.ProfileRelayServer = form.RelayServer
	device.ProfileApiServer = form.APIServer
	if form.Key != nil {
		device.ProfileKey = *form.Key
	}
	if form.PermanentPassword != nil {
		device.ProfilePassword = *form.PermanentPassword
	}
	if err = upsertDeviceSnapshot(c.Db, &device, true); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"policy_revision": device.PolicyRevision}, "ok")
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
	if form.RootCommand == "" {
		form.RootCommand = "auto"
	}
	if form.RootCommand != "auto" && form.RootCommand != "su" && form.RootCommand != "testsu" && form.RootCommand != "disabled" {
		return c.Error(nil, "InvalidRootCommand")
	}
	device := model.Device{}
	has, err := c.Db.ID(form.Id).Get(&device)
	if err != nil || !has {
		return c.Error(nil, "DeviceNotFound")
	}
	device.UnattendedEnabled = form.Enabled
	device.RootCommand = form.RootCommand
	device.PolicyRevision++
	_, err = c.Db.Table(new(model.Device)).ID(form.Id).Update(map[string]interface{}{
		"unattended_enabled": form.Enabled,
		"root_command":       form.RootCommand,
		"policy_revision":    device.PolicyRevision,
	})
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if err = upsertDeviceSnapshot(c.Db, &device, true); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"policy_revision": device.PolicyRevision}, "ok")
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

	err := pagination.Paginate(query, &model.Audit{}, &deviceList)
	if err != nil {
		return c.Error(nil, err.Error())
	}

	list := make([]iris.Map, 0)
	for _, a := range deviceList {
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
			"policy_revision":          a.PolicyRevision,
			"applied_revision":         a.AppliedRevision,
			"unattended_status":        a.UnattendedStatus,
			"root_executor":            a.RootExecutor,
			"root_available":           a.RootAvailable,
			"screen_capture_ready":     a.ScreenCaptureReady,
			"accessibility_ready":      a.AccessibilityReady,
			"service_running":          a.ServiceRunning,
			"unattended_error":         a.UnattendedError,
			"unattended_reported_at":   a.UnattendedReportedAt.Format(config.TimeFormat),
			"profile_enabled":          a.ProfileEnabled,
			"profile_id_server":        a.ProfileIdServer,
			"profile_relay_server":     a.ProfileRelayServer,
			"profile_api_server":       a.ProfileApiServer,
			"profile_key_set":          a.ProfileKey != "",
			"profile_password_set":     a.ProfilePassword != "",
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
