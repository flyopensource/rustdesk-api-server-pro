package policy

import "unicode"

type ServerProfile struct {
	ID                 int
	Name               string
	IDServer           string
	RelayServer        string
	ServerKey          string
	PasswordCiphertext string
}

type Effective struct {
	Revision          int64
	UnattendedEnabled bool
	RootCommand       string
	ProfileEnabled    bool
	ProfileSource     string
	Profile           ServerProfile
}

func ValidRootCommand(value string) bool {
	if value == "auto" {
		return true
	}
	if value == "" || value == "disabled" || len(value) > 255 {
		return false
	}
	for _, char := range value {
		if unicode.IsSpace(char) || unicode.IsControl(char) {
			return false
		}
	}
	return true
}
