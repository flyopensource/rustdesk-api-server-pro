package admin

import (
	"encoding/json"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"rustdesk-api-server-pro/app/model"
	"rustdesk-api-server-pro/app/service"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (c *DevicesController) HandleAliasTargets() mvc.Result {
	id := c.Ctx.URLParamIntDefault("id", 0)
	device := model.Device{}
	if has, err := c.Db.ID(id).Get(&device); err != nil || !has || id <= 0 {
		return c.Error(nil, "DeviceNotFound")
	}
	books := []model.AddressBook{}
	if err := c.Db.Where("shared = ?", false).Asc("user_id", "id").Find(&books); err != nil {
		return c.Error(nil, err.Error())
	}
	links := []model.Peer{}
	if err := c.Db.Where("managed_device_id = ?", id).Find(&links); err != nil {
		return c.Error(nil, err.Error())
	}
	selected := make([]int, 0, len(links))
	for _, peer := range links {
		selected = append(selected, peer.AbId)
	}
	targets := []iris.Map{}
	for _, book := range books {
		user := model.User{}
		has, err := c.Db.ID(book.UserId).Get(&user)
		if err != nil {
			return c.Error(nil, err.Error())
		}
		if !has {
			continue
		}
		targets = append(targets, iris.Map{"id": book.Id, "user_id": user.Id, "username": user.Username, "name": book.Name, "enabled": user.Status == 1})
	}
	return c.Success(iris.Map{"alias": device.Alias, "address_book_ids": selected, "targets": targets}, "ok")
}

func (c *DevicesController) HandleDeviceAlias() mvc.Result {
	var form struct {
		Id             int     `json:"id"`
		Alias          *string `json:"alias"`
		AddressBookIds *[]int  `json:"address_book_ids"`
	}
	if c.Ctx.ReadJSON(&form) != nil || form.Id <= 0 || form.Alias == nil || form.AddressBookIds == nil {
		return c.Error(nil, "InvalidDeviceAlias")
	}
	if !utf8.ValidString(*form.Alias) || strings.IndexFunc(*form.Alias, unicode.IsControl) >= 0 {
		return c.Error(nil, "InvalidDeviceAlias")
	}
	alias := strings.TrimSpace(*form.Alias)
	if utf8.RuneCountInString(alias) > 128 || len(*form.AddressBookIds) > 200 {
		return c.Error(nil, "InvalidDeviceAlias")
	}
	targets := map[int]bool{}
	for _, id := range *form.AddressBookIds {
		if id <= 0 {
			return c.Error(nil, "InvalidAddressBook")
		}
		targets[id] = true
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
	if _, err := s.ID(device.Id).Cols("alias").Update(&model.Device{Alias: alias}); err != nil {
		return c.Error(nil, err.Error())
	}
	links := []model.Peer{}
	if err := s.Where("managed_device_id = ?", device.Id).Find(&links); err != nil {
		return c.Error(nil, err.Error())
	}
	for _, peer := range links {
		if !targets[peer.AbId] {
			if err := service.ReleaseManagedPeer(s, &peer); err != nil {
				return c.Error(nil, err.Error())
			}
		}
	}
	for id := range targets {
		book := model.AddressBook{}
		has, err := s.ID(id).Get(&book)
		if err != nil || !has || book.Shared {
			return c.Error(nil, "InvalidAddressBook")
		}
		user := model.User{}
		has, err = s.ID(book.UserId).Get(&user)
		if err != nil || !has || user.Status != 1 {
			return c.Error(nil, "InvalidAddressBookOwner")
		}
		peer := model.Peer{}
		has, err = s.Where("ab_id = ? AND user_id = ? AND rustdesk_id = ?", id, user.Id, device.RustdeskId).Get(&peer)
		if err != nil {
			return c.Error(nil, err.Error())
		}
		if has {
			if peer.ManagedDeviceId != 0 && peer.ManagedDeviceId != device.Id {
				return c.Error(nil, "PeerAlreadyManaged")
			}
			if _, err = s.ID(peer.Id).Cols("alias", "managed_device_id").Update(&model.Peer{Alias: alias, ManagedDeviceId: device.Id}); err != nil {
				return c.Error(nil, err.Error())
			}
		} else {
			count, err := s.Where("ab_id = ? AND user_id = ?", id, user.Id).Count(new(model.Peer))
			if err != nil {
				return c.Error(nil, err.Error())
			}
			if book.MaxPeer > 0 && count >= int64(book.MaxPeer) {
				return c.Error(nil, "exceed_max_devices")
			}
			peer = model.Peer{UserId: user.Id, AbId: id, RustdeskId: device.RustdeskId, Alias: alias, Hostname: device.Hostname, Username: device.Username, Platform: device.Os, Tags: "[]", ManagedDeviceId: device.Id, ManagedCreated: true}
			if _, err = s.Insert(&peer); err != nil {
				return c.Error(nil, err.Error())
			}
		}
	}
	detail, err := json.Marshal(iris.Map{"alias": alias, "address_book_ids": *form.AddressBookIds})
	if err != nil {
		return c.Error(nil, err.Error())
	}
	if _, err = s.Insert(&model.DeviceOperation{DeviceId: device.Id, RustdeskId: device.RustdeskId, ActorId: c.GetUser().Id, Action: "alias", Detail: string(detail)}); err != nil {
		return c.Error(nil, err.Error())
	}
	if err = s.Commit(); err != nil {
		return c.Error(nil, err.Error())
	}
	return c.Success(iris.Map{"alias": alias, "synced_address_books": len(targets)}, "ok")
}
