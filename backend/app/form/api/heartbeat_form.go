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
	PasswordStatus      *PasswordStatusForm      `json:"password_status,omitempty"`
}

type PasswordStatusForm struct {
	AppliedRevision      int64  `json:"applied_revision"`
	Status               string `json:"status"`
	PermanentPasswordSet bool   `json:"permanent_password_set"`
	LastError            string `json:"last_error"`
}

type ServerProfileStatusForm struct {
	PolicyRevision   int64  `json:"policy_revision"`
	ReceivedRevision int64  `json:"received_revision"`
	AppliedRevision  int64  `json:"applied_revision"`
	FailedRevision   int64  `json:"failed_revision"`
	ApplyStatus      string `json:"apply_status"`
	ActiveSource     string `json:"active_source"`
	Connected        bool   `json:"connected"`
	Fingerprint      string `json:"fingerprint"`
	LastError        string `json:"last_error"`
}

type UnattendedStatusForm struct {
	PolicyRevision      int64  `json:"policy_revision"`
	Status              string `json:"status"`
	RootExecutor        string `json:"root_executor"`
	RootAvailable       bool   `json:"root_available"`
	ScreenCaptureReady  bool   `json:"screen_capture_ready"`
	AccessibilityReady  bool   `json:"accessibility_ready"`
	AllFilesAccessReady bool   `json:"all_files_access_ready"`
	ServiceRunning      bool   `json:"service_running"`
	LastError           string `json:"last_error"`
}
