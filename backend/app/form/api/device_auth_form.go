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
