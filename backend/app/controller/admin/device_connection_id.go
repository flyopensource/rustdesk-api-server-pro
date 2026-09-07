package admin

import (
	"encoding/json"
	"regexp"
	"rustdesk-api-server-pro/app/model"
	devicepolicy "rustdesk-api-server-pro/app/policy"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
)

var managedConnectionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{5,15}$`)

func normalizeManagedConnectionID(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	return value, managedConnectionIDPattern.MatchString(value)
}

func (c *DevicesController) HandleDeviceConnectionId() mvc.Result {
	var form struct {
		Id           int    `json:"id"`
		ConnectionId string `json:"connection_id"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 {
		return c.Error(nil, "InvalidConnectionId")
	}
	connectionID, valid := normalizeManagedConnectionID(form.ConnectionId)
	if !valid {
		return c.Error(nil, "InvalidConnectionId")
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
	if device.Disabled {
		return c.Error(nil, "DeviceDisabled")
	}
	managed, err := s.Where("device_id = ? AND enabled = ?", device.Id, true).Exist(new(model.DeviceCredential))
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if !managed {
		return c.Error(nil, "DeviceNotManaged")
	}
	if device.ConnectionIdStatus == model.ConnectionIdApplied {
		return c.Error(nil, "ConnectionIdAlreadyApplied")
	}
	if connectionID == device.RustdeskId {
		return c.Error(nil, "ConnectionIdUnchanged")
	}
	conflict, err := s.Where("id <> ? AND (rustdesk_id = ? OR requested_rustdesk_id = ?)", device.Id, connectionID, connectionID).Exist(new(model.Device))
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if conflict {
		return c.Error(nil, "ConnectionIdTaken")
	}
	managedPeers := make([]model.Peer, 0)
	if err = s.Where("managed_device_id = ?", device.Id).Find(&managedPeers); err != nil {
		return c.Error(nil, err.Error())
	}
	for _, peer := range managedPeers {
		conflict, err = s.Where("id <> ? AND user_id = ? AND ab_id = ? AND rustdesk_id = ?", peer.Id, peer.UserId, peer.AbId, connectionID).Exist(new(model.Peer))
		if err != nil {
			return c.Error(nil, err.Error())
		}
		if conflict {
			return c.Error(nil, "ConnectionIdAddressBookConflict")
		}
	}
	if device.ConnectionIdStatus == model.ConnectionIdPending && device.RequestedRustdeskId == connectionID {
		return c.Success(iris.Map{"connection_id": connectionID, "status": model.ConnectionIdPending, "policy_revision": device.ConnectionIdRevision}, "ok")
	}
	now := time.Now()
	revision, err := devicepolicy.NextRevision(s)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = s.Table(new(model.Device)).ID(device.Id).Update(map[string]interface{}{
		"requested_rustdesk_id":      connectionID,
		"connection_id_status":       model.ConnectionIdPending,
		"connection_id_revision":     revision,
		"connection_id_error":        "",
		"connection_id_requested_at": now,
	}); err != nil {
		return c.Error(nil, err.Error())
	}
	detail, err := json.Marshal(iris.Map{"old_id": device.RustdeskId, "requested_id": connectionID})
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = s.Insert(&model.DeviceOperation{DeviceId: device.Id, RustdeskId: device.RustdeskId, ActorId: c.GetUser().Id, Action: "connection_id_request", Detail: string(detail)}); err != nil {
		return c.Error(nil, err.Error())
	}
	if err = s.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"connection_id": connectionID, "status": model.ConnectionIdPending, "policy_revision": revision}, "ok")
}
