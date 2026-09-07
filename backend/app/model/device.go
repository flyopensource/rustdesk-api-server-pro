package model

import "time"

type Device struct {
	Id                     int       `xorm:"'id' int notnull pk autoincr"`
	Cpu                    string    `xorm:"'cpu' varchar(255)"`
	Hostname               string    `xorm:"'hostname' varchar(255)"`
	RustdeskId             string    `xorm:"'rustdesk_id' varchar(255)"`
	Memory                 string    `xorm:"'memory' varchar(50)"`
	Os                     string    `xorm:"'os' varchar(255)"`
	Username               string    `xorm:"'username' varchar(255)"`
	Uuid                   string    `xorm:"'uuid' varchar(255)"`
	Version                string    `xorm:"'version' varchar(255)"`
	IsOnline               bool      `xorm:"'is_online' tinyint"`
	Disabled               bool      `xorm:"'disabled' tinyint notnull default 0"`
	Conns                  int       `xorm:"'conns' int"`
	StrategyGroupId        int       `xorm:"'strategy_group_id' int notnull default 0 index"`
	StrategyProfileId      int       `xorm:"'strategy_profile_id' int notnull default 0 index"`
	UnattendedEnabled      bool      `xorm:"'unattended_enabled' tinyint notnull default 0"`
	RootCommand            string    `xorm:"'root_command' varchar(255) notnull default 'auto'"`
	PolicyRevision         int64     `xorm:"'policy_revision' bigint notnull default 0"`
	AppliedRevision        int64     `xorm:"'applied_revision' bigint notnull default 0"`
	UnattendedStatus       string    `xorm:"'unattended_status' varchar(32)"`
	RootExecutor           string    `xorm:"'root_executor' varchar(255)"`
	RootAvailable          bool      `xorm:"'root_available' tinyint"`
	ScreenCaptureReady     bool      `xorm:"'screen_capture_ready' tinyint"`
	AccessibilityReady     bool      `xorm:"'accessibility_ready' tinyint"`
	ServiceRunning         bool      `xorm:"'service_running' tinyint"`
	UnattendedError        string    `xorm:"'unattended_error' varchar(255)"`
	UnattendedReportedAt   time.Time `xorm:"'unattended_reported_at' datetime"`
	ProfileAppliedRevision int64     `xorm:"'profile_applied_revision' bigint notnull default 0"`
	ProfileActiveSource    string    `xorm:"'profile_active_source' varchar(32)"`
	ProfileConnected       bool      `xorm:"'profile_connected' tinyint"`
	ProfileReportedAt      time.Time `xorm:"'profile_reported_at' datetime"`
	CreatedAt              time.Time `xorm:"'created_at' datetime created"`
	UpdatedAt              time.Time `xorm:"'updated_at' datetime updated"`
}

func (m *Device) TableName() string {
	return "device"
}
