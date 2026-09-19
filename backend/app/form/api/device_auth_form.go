package api

type DeviceRegistrationForm struct {
	RustdeskId      string `json:"id"`
	Uuid            string `json:"uuid"`
	PublicKey       string `json:"public_key"`
	Timestamp       int64  `json:"timestamp"`
	Signature       string `json:"signature"`
	EnrollmentProof string `json:"enrollment_proof"`
}

type SignedDeviceRequestForm struct {
	DeviceId  int64  `json:"device_id"`
	Sequence  int64  `json:"sequence"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type DesktopDeviceRegistrationForm struct {
	Version       int    `json:"version"`
	RequestId     string `json:"request_id"`
	Token         string `json:"token"`
	RustdeskId    string `json:"id"`
	Uuid          string `json:"uuid"`
	Hostname      string `json:"hostname"`
	Os            string `json:"os"`
	Arch          string `json:"arch"`
	ClientVersion string `json:"client_version"`
	SignPublicKey string `json:"sign_public_key"`
	BoxPublicKey  string `json:"box_public_key"`
	Timestamp     int64  `json:"timestamp"`
	Signature     string `json:"signature"`
}
