package model

import "time"

type DeviceGroup struct {
	Id        int       `xorm:"'id' int notnull pk autoincr"`
	Name      string    `xorm:"'name' varchar(100) notnull unique"`
	Enabled   bool      `xorm:"'enabled' tinyint notnull default 1"`
	ProfileId int       `xorm:"'profile_id' int notnull default 0 index"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *DeviceGroup) TableName() string {
	return "strategy_device_group"
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

type StrategyState struct {
	Id              int       `xorm:"'id' int notnull pk"`
	GlobalProfileId int       `xorm:"'global_profile_id' int notnull default 0 index"`
	Revision        int64     `xorm:"'revision' bigint notnull"`
	UpdatedAt       time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *StrategyState) TableName() string {
	return "strategy_state"
}
