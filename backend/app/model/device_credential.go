package model

import "time"

type DeviceCredential struct {
	Id        int       `xorm:"'id' int notnull pk autoincr"`
	DeviceId  int       `xorm:"'device_id' int notnull unique"`
	PublicKey string    `xorm:"'public_key' varchar(64) notnull unique"`
	Enabled   bool      `xorm:"'enabled' tinyint notnull default 1"`
	LastSeq   int64     `xorm:"'last_seq' bigint notnull default 0"`
	CreatedAt time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *DeviceCredential) TableName() string {
	return "device_credential"
}
