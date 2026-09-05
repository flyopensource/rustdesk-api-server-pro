package api

type HeartbeatForm struct {
	RustdeskId          string                   `json:"id"`
	Uuid                string                   `json:"uuid"`
	ModifiedAt          int64                    `json:"modified_at"`
	Ver                 int64                    `json:"ver"`
	Version             string                   `json:"version"`
	Conns               []int                    `json:"conns"`
	UnattendedStatus    *UnattendedStatusForm    `json:"unattended_status,omitempty"`
	ServerProfileStatus *ServerProfileStatusForm `json:"server_profile_status,omitempty"`
}

type ServerProfileStatusForm struct {
	PolicyRevision int64  `json:"policy_revision"`
	ActiveSource   string `json:"active_source"`
	Connected      bool   `json:"connected"`
}

type UnattendedStatusForm struct {
	PolicyRevision     int64  `json:"policy_revision"`
	Status             string `json:"status"`
	RootExecutor       string `json:"root_executor"`
	RootAvailable      bool   `json:"root_available"`
	ScreenCaptureReady bool   `json:"screen_capture_ready"`
	AccessibilityReady bool   `json:"accessibility_ready"`
	ServiceRunning     bool   `json:"service_running"`
	LastError          string `json:"last_error"`
}
