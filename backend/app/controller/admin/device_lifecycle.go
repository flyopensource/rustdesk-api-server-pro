package admin

import (
	"github.com/kataras/iris/v12/mvc"
	"rustdesk-api-server-pro/app/model"
)

func (c *DevicesController) HandleDeviceEnabled() mvc.Result {
	var form struct {
		Id      int   `json:"id"`
		Enabled *bool `json:"enabled"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || form.Enabled == nil {
		return c.Error(nil, "InvalidDeviceState")
	}
	s := c.Db.NewSession()
	defer s.Close()
	if err := s.Begin(); err != nil {
		return c.Error(nil, err.Error())
	}
	defer s.Rollback()
	device := model.Device{}
	if has, err := s.ID(form.Id).Get(&device); err != nil || !has {
		return c.Error(nil, "DeviceNotFound")
	}
	if _, err := s.Table(new(model.Device)).ID(device.Id).Update(map[string]interface{}{"disabled": !*form.Enabled}); err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err := s.Where("device_id = ?", device.Id).Cols("enabled").Update(&model.DeviceCredential{Enabled: *form.Enabled}); err != nil {
		return c.Error(nil, err.Error())
	}
	if device.Disabled == *form.Enabled {
		action := "disable"
		if *form.Enabled {
			action = "enable"
		}
		if _, err := s.Insert(&model.DeviceOperation{DeviceId: device.Id, RustdeskId: device.RustdeskId, ActorId: c.GetUser().Id, Action: action}); err != nil {
			return c.Error(nil, err.Error())
		}
	}
	if err := s.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}
