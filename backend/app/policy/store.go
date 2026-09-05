package policy

import (
	"encoding/json"
	"fmt"
	"rustdesk-api-server-pro/app/model"
	"sort"

	"xorm.io/xorm"
)

type Resolution struct {
	Effective Effective `json:"effective"`
	Layers    []string  `json:"layers"`
}

func ResolveForDevice(db *xorm.Engine, device *model.Device) (Resolution, error) {
	policies := make([]model.ManagedDevicePolicy, 0)
	if err := db.Where("enabled = ?", true).Find(&policies); err != nil {
		return Resolution{}, err
	}
	byScope := make(map[string]model.ManagedDevicePolicy)
	for _, item := range policies {
		byScope[fmt.Sprintf("%s:%d", item.ScopeType, item.ScopeId)] = item
	}
	var global *Document
	layers := make([]string, 0, 3)
	if item, ok := byScope[fmt.Sprintf("%s:0", model.DevicePolicyScopeGlobal)]; ok {
		document, err := decodeDocument(item.Document)
		if err != nil {
			return Resolution{}, err
		}
		global = &document
		layers = append(layers, "global")
	}
	type membership struct {
		GroupId  int `xorm:"group_id"`
		Priority int `xorm:"priority"`
	}
	memberships := make([]membership, 0)
	if err := db.Table(new(model.DeviceGroupMember)).Alias("m").
		Join("INNER", []string{new(model.DeviceGroup).TableName(), "g"}, "g.id = m.group_id AND g.enabled = 1").
		Where("m.device_id = ?", device.Id).Cols("m.group_id", "g.priority").Find(&memberships); err != nil {
		return Resolution{}, err
	}
	groups := make([]GroupDocument, 0, len(memberships))
	for _, member := range memberships {
		if item, ok := byScope[fmt.Sprintf("%s:%d", model.DevicePolicyScopeGroup, member.GroupId)]; ok {
			document, err := decodeDocument(item.Document)
			if err != nil {
				return Resolution{}, err
			}
			groups = append(groups, GroupDocument{GroupID: member.GroupId, Priority: member.Priority, Document: document})
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Priority == groups[j].Priority {
			return groups[i].GroupID < groups[j].GroupID
		}
		return groups[i].Priority < groups[j].Priority
	})
	var deviceDocument *Document
	if item, ok := byScope[fmt.Sprintf("%s:%d", model.DevicePolicyScopeDevice, device.Id)]; ok {
		document, err := decodeDocument(item.Document)
		if err != nil {
			return Resolution{}, err
		}
		deviceDocument = &document
	}
	if global == nil && len(groups) == 0 && deviceDocument == nil {
		return Resolution{Effective: LegacyEffective(device), Layers: []string{"legacy-device"}}, nil
	}
	for _, group := range groups {
		layers = append(layers, fmt.Sprintf("group:%d", group.GroupID))
	}
	if deviceDocument != nil {
		layers = append(layers, "device")
	}
	effective, err := Resolve(global, groups, deviceDocument)
	return Resolution{Effective: effective, Layers: layers}, err
}

func LegacyEffective(device *model.Device) Effective {
	return Effective{
		UnattendedEnabled: device.UnattendedEnabled, RootCommand: device.RootCommand,
		ProfileEnabled: device.ProfileEnabled, IDServer: device.ProfileIdServer,
		RelayServer: device.ProfileRelayServer, APIServer: device.ProfileApiServer,
		Key: device.ProfileKey, PermanentPassword: device.ProfilePassword,
	}
}

func decodeDocument(value string) (Document, error) {
	var document Document
	if err := json.Unmarshal([]byte(value), &document); err != nil {
		return Document{}, err
	}
	return document, nil
}
