package model

import "time"

const (
	DevicePolicyScopeGlobal = "global"
	DevicePolicyScopeGroup  = "group"
	DevicePolicyScopeDevice = "device"
)

type DeviceGroup struct {
	Id        int       `xorm:"'id' int notnull pk autoincr"`
	Name      string    `xorm:"'name' varchar(100) notnull unique"`
	Priority  int       `xorm:"'priority' int notnull default 0"`
	Enabled   bool      `xorm:"'enabled' tinyint notnull default 1"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *DeviceGroup) TableName() string {
	return "device_group"
}

type DeviceGroupMember struct {
	Id        int       `xorm:"'id' int notnull pk autoincr"`
	GroupId   int       `xorm:"'group_id' int notnull unique(group_device)"`
	DeviceId  int       `xorm:"'device_id' int notnull unique(group_device)"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
}

func (m *DeviceGroupMember) TableName() string {
	return "device_group_member"
}

type ManagedDevicePolicy struct {
	Id        int       `xorm:"'id' int notnull pk autoincr"`
	ScopeType string    `xorm:"'scope_type' varchar(16) notnull unique(scope)"`
	ScopeId   int       `xorm:"'scope_id' int notnull default 0 unique(scope)"`
	Document  string    `xorm:"'document' text notnull"`
	Revision  int64     `xorm:"'revision' bigint notnull default 0"`
	Enabled   bool      `xorm:"'enabled' tinyint notnull default 1"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *ManagedDevicePolicy) TableName() string {
	return "managed_device_policy"
}
