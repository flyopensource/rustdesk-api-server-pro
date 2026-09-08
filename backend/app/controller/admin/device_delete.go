package admin

import (
	"encoding/json"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/app/service"
)

func (c *DevicesController) HandleDeviceDelete() mvc.Result {
	var form struct {
		Id         int    `json:"id"`
		RustdeskId string `json:"rustdesk_id"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || form.RustdeskId == "" {
		return c.Error(nil, "InvalidDeviceConfirmation")
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
	if device.RustdeskId != form.RustdeskId {
		return c.Error(nil, "DeviceConfirmationMismatch")
	}
	if !device.Disabled || device.IsOnline {
		return c.Error(nil, "DeviceMustBeDisabledAndOffline")
	}
	// Conditional deletion rejects a concurrent enable/heartbeat before any cleanup.
	affected, err := s.ID(device.Id).Where("disabled = ? AND is_online = ?", true, false).Delete(new(model.Device))
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if affected != 1 {
		return c.Error(nil, "DeviceStateChanged")
	}
	if _, err = s.Where("device_id = ?", device.Id).Delete(new(model.DeviceCredential)); err != nil {
		return c.Error(nil, err.Error())
	}
	peers := []model.Peer{}
	if err = s.Where("managed_device_id = ?", device.Id).Find(&peers); err != nil {
		return c.Error(nil, err.Error())
	}
	for _, peer := range peers {
		if err = service.ReleaseManagedPeer(s, &peer); err != nil {
			return c.Error(nil, err.Error())
		}
	}
	detail, err := json.Marshal(iris.Map{"alias": device.Alias, "group_id": device.StrategyGroupId})
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = s.Insert(&model.DeviceOperation{DeviceId: device.Id, RustdeskId: device.RustdeskId, ActorId: c.GetUser().Id, Action: "delete", Detail: string(detail)}); err != nil {
		return c.Error(nil, err.Error())
	}
	if err = s.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(nil, "ok")
}
