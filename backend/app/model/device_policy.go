package model

import "time"

const (
	StrategyScopeGlobal = "global"
	StrategyScopeGroup  = "group"
	StrategyScopeDevice = "device"
)

type DeviceGroup struct {
	Id        int       `xorm:"'id' int notnull pk autoincr"`
	Name      string    `xorm:"'name' varchar(100) notnull unique"`
	Enabled   bool      `xorm:"'enabled' tinyint notnull default 1"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *DeviceGroup) TableName() string {
	return "strategy_device_group"
}

type DeviceGroupMember struct {
	Id        int       `xorm:"'id' int notnull pk autoincr"`
	GroupId   int       `xorm:"'group_id' int notnull index"`
	DeviceId  int       `xorm:"'device_id' int notnull unique"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
}

func (m *DeviceGroupMember) TableName() string {
	return "strategy_device_group_member"
}

type ServerProfile struct {
	Id                 int       `xorm:"'id' int notnull pk autoincr"`
	Name               string    `xorm:"'name' varchar(100) notnull unique"`
	IdServer           string    `xorm:"'id_server' varchar(255) notnull"`
	RelayServer        string    `xorm:"'relay_server' varchar(255)"`
	ServerKey          string    `xorm:"'server_key' varchar(255)"`
	PasswordCiphertext string    `xorm:"'password_ciphertext' text"`
	Enabled            bool      `xorm:"'enabled' tinyint notnull default 1"`
	CreatedAt          time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt          time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *ServerProfile) TableName() string {
	return "strategy_server_profile"
}

type ServerProfileAssignment struct {
	Id        int       `xorm:"'id' int notnull pk autoincr"`
	ScopeType string    `xorm:"'scope_type' varchar(16) notnull unique(scope)"`
	ScopeId   int       `xorm:"'scope_id' int notnull default 0 unique(scope)"`
	ProfileId int       `xorm:"'profile_id' int notnull index"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *ServerProfileAssignment) TableName() string {
	return "strategy_server_profile_assignment"
}

type StrategyState struct {
	Id        int       `xorm:"'id' int notnull pk"`
	Revision  int64     `xorm:"'revision' bigint notnull"`
	UpdatedAt time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *StrategyState) TableName() string {
	return "strategy_state"
}
