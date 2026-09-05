package admin

import (
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/config"
	"rustdesk-api-server-pro/db"

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
			"id":                     a.Id,
			"rustdesk_id":            a.RustdeskId,
			"hostname":               a.Hostname,
			"username":               a.Username,
			"uuid":                   a.Uuid,
			"version":                a.Version,
			"os":                     a.Os,
			"memory":                 a.Memory,
			"created_at":             a.CreatedAt.Format(config.TimeFormat),
			"is_online":              a.IsOnline,
			"unattended_enabled":     a.UnattendedEnabled,
			"root_command":           a.RootCommand,
			"policy_revision":        a.PolicyRevision,
			"applied_revision":       a.AppliedRevision,
			"unattended_status":      a.UnattendedStatus,
			"root_executor":          a.RootExecutor,
			"root_available":         a.RootAvailable,
			"screen_capture_ready":   a.ScreenCaptureReady,
			"accessibility_ready":    a.AccessibilityReady,
			"service_running":        a.ServiceRunning,
			"unattended_error":       a.UnattendedError,
			"unattended_reported_at": a.UnattendedReportedAt.Format(config.TimeFormat),
		})
	}
	return c.Success(iris.Map{
		"total":   pagination.TotalCount,
		"records": list,
		"current": currentPage,
		"size":    pageSize,
	}, "ok")
}
