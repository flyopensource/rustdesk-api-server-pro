package admin

import (
	controllerapi "rustdesk-api-server-pro/app/controller/api"
	"rustdesk-api-server-pro/app/model"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
)

func registerDesktopEnrollmentRoutes(b mvc.BeforeActivation) {
	b.Handle("GET", "/devices/desktop-enrollment-tokens", "HandleDesktopEnrollmentTokens")
	b.Handle("POST", "/devices/desktop-enrollment-tokens", "HandleCreateDesktopEnrollmentToken")
	b.Handle("DELETE", "/devices/desktop-enrollment-tokens", "HandleRevokeDesktopEnrollmentToken")
}

func (c *DevicesController) HandleDesktopEnrollmentTokens() mvc.Result {
	tokens := make([]model.DesktopEnrollmentToken, 0)
	if err := c.Db.Desc("id").Limit(200).Find(&tokens); err != nil {
		return c.Error(nil, err.Error())
	}
	now := time.Now()
	result := make([]iris.Map, 0, len(tokens))
	for _, token := range tokens {
		status := "unused"
		if token.ConsumedDeviceId > 0 {
			status = "consumed"
		} else if !token.RevokedAt.IsZero() {
			status = "revoked"
		} else if !token.ExpiresAt.IsZero() && !now.Before(token.ExpiresAt) {
			status = "expired"
		}
		result = append(result, iris.Map{
			"id": token.Id, "selector": token.Selector, "group_id": token.GroupId,
			"note": token.Note, "status": status, "expires_at": token.ExpiresAt,
			"consumed_at": token.ConsumedAt, "consumed_device_id": token.ConsumedDeviceId,
			"created_at": token.CreatedAt,
		})
	}
	return c.Success(iris.Map{"tokens": result}, "ok")
}

func (c *DevicesController) HandleCreateDesktopEnrollmentToken() mvc.Result {
	var form struct {
		ExpiresAt int64  `json:"expires_at"`
		GroupId   int    `json:"group_id"`
		Note      string `json:"note"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.ExpiresAt <= time.Now().Unix() || len(strings.TrimSpace(form.Note)) > 255 || form.GroupId < 0 {
		return c.Error(nil, "InvalidDesktopEnrollmentToken")
	}
	if form.GroupId > 0 {
		group := model.DeviceGroup{}
		if found, err := c.Db.ID(form.GroupId).Where("enabled = ?", true).Get(&group); err != nil || !found {
			return c.Error(nil, "DeviceGroupNotFound")
		}
	}
	plain, selector, secretHash, err := controllerapi.GenerateDesktopEnrollmentToken()
	if err != nil {
		return c.Error(nil, err.Error())
	}
	token := model.DesktopEnrollmentToken{
		Selector: selector, SecretHash: secretHash, GroupId: form.GroupId,
		Note: strings.TrimSpace(form.Note), ExpiresAt: time.Unix(form.ExpiresAt, 0), CreatedBy: c.GetUser().Id,
	}
	if _, err = c.Db.Insert(&token); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{
		"id": token.Id, "token": plain, "group_id": token.GroupId, "expires_at": token.ExpiresAt,
	}, "ok")
}

func (c *DevicesController) HandleRevokeDesktopEnrollmentToken() mvc.Result {
	id := c.Ctx.URLParamIntDefault("id", 0)
	if id <= 0 {
		return c.Error(nil, "InvalidDesktopEnrollmentToken")
	}
	token := model.DesktopEnrollmentToken{}
	found, err := c.Db.ID(id).Get(&token)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if !found || token.ConsumedDeviceId != 0 || !token.RevokedAt.IsZero() {
		return c.Error(nil, "DesktopEnrollmentTokenNotRevocable")
	}
	token.RevokedAt = time.Now()
	updated, err := c.Db.ID(id).Where("consumed_device_id = ?", 0).Cols("revoked_at").Update(&token)
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if updated != 1 {
		return c.Error(nil, "DesktopEnrollmentTokenNotRevocable")
	}
	return c.Success(nil, "ok")
}
