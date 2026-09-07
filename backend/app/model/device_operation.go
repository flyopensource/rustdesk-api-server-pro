package model

import "time"

type DeviceOperation struct {
	Id         int    `xorm:"pk autoincr"`
	DeviceId   int    `xorm:"index"`
	RustdeskId string `xorm:"varchar(255)"`
	ActorId    int
	Action     string    `xorm:"varchar(32)"`
	Detail     string    `xorm:"text"`
	CreatedAt  time.Time `xorm:"created"`
}
