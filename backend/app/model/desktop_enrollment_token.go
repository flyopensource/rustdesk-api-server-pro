package model

import "time"

type DesktopEnrollmentToken struct {
	Id               int       `xorm:"'id' int notnull pk autoincr"`
	Selector         string    `xorm:"'selector' varchar(32) notnull unique"`
	SecretHash       string    `xorm:"'secret_hash' varchar(64) notnull"`
	GroupId          int       `xorm:"'group_id' int notnull default 0 index"`
	Note             string    `xorm:"'note' varchar(255)"`
	ExpiresAt        time.Time `xorm:"'expires_at' datetime index"`
	RevokedAt        time.Time `xorm:"'revoked_at' datetime"`
	ConsumedAt       time.Time `xorm:"'consumed_at' datetime"`
	ConsumedDeviceId int       `xorm:"'consumed_device_id' int notnull default 0"`
	RequestId        string    `xorm:"'request_id' varchar(128)"`
	RequestDigest    string    `xorm:"'request_digest' varchar(64)"`
	CreatedBy        int       `xorm:"'created_by' int notnull default 0"`
	CreatedAt        time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt        time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *DesktopEnrollmentToken) TableName() string {
	return "desktop_enrollment_token"
}
