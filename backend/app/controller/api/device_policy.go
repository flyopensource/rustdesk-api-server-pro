package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"rustdesk-api-server-pro/app/model"
	devicepolicy "rustdesk-api-server-pro/app/policy"
	"rustdesk-api-server-pro/config"
	"time"

	"golang.org/x/crypto/nacl/secretbox"
)

type unattendedPolicy struct {
	Version   int   `json:"version"`
	Revision  int64 `json:"revision"`
	IssuedAt  int64 `json:"issued_at"`
	ExpiresAt int64 `json:"expires_at"`
	Target    struct {
		RustdeskID string `json:"rustdesk_id"`
		UUID       string `json:"uuid"`
	} `json:"target"`
	Android struct {
		Unattended struct {
			Enabled     bool   `json:"enabled"`
			RootCommand string `json:"root_command"`
		} `json:"unattended"`
	} `json:"android"`
	ServerProfile struct {
		Enabled           bool   `json:"enabled"`
		IDServer          string `json:"id_server"`
		RelayServer       string `json:"relay_server"`
		Key               string `json:"key"`
		PermanentPassword string `json:"permanent_password"`
	} `json:"server_profile"`
}

type policyEnvelope struct {
	Version    int    `json:"version"`
	Purpose    string `json:"purpose"`
	KeyID      string `json:"key_id"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	Signature  string `json:"signature"`
}

func buildPolicyEnvelope(device *model.Device, cfg *config.ServerConfig, effective devicepolicy.Effective) (string, error) {
	seed, err := base64.StdEncoding.DecodeString(cfg.ProvisioningSignSeed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return "", errors.New("provisioning signing key is not configured")
	}
	secret, err := base64.StdEncoding.DecodeString(cfg.ProvisioningSecretKey)
	if err != nil || len(secret) != 32 {
		return "", errors.New("provisioning encryption key is not configured")
	}
	now := time.Now().Unix()
	policy := unattendedPolicy{Version: 1, Revision: effective.Revision, IssuedAt: now, ExpiresAt: now + 7*24*60*60}
	policy.Target.RustdeskID = device.RustdeskId
	policy.Target.UUID = device.Uuid
	policy.Android.Unattended.Enabled = effective.UnattendedEnabled
	policy.Android.Unattended.RootCommand = effective.RootCommand
	policy.ServerProfile.Enabled = effective.ProfileEnabled
	policy.ServerProfile.IDServer = effective.Profile.IDServer
	policy.ServerProfile.RelayServer = effective.Profile.RelayServer
	policy.ServerProfile.Key = effective.Profile.ServerKey
	policy.ServerProfile.PermanentPassword, err = devicepolicy.DecryptPassword(effective.Profile.PasswordCiphertext, cfg.ProvisioningSecretKey)
	if err != nil {
		return "", err
	}
	plaintext, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	var nonce [24]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return "", err
	}
	var secretKey [32]byte
	copy(secretKey[:], secret)
	ciphertext := secretbox.Seal(nil, plaintext, &nonce, &secretKey)
	keyID := cfg.ProvisioningKeyId
	if keyID == "" {
		keyID = "android-v1"
	}
	message := append([]byte("RUD1"), make([]byte, 4)...)
	binary.BigEndian.PutUint32(message[4:8], 1)
	message = append(message, []byte("policy")...)
	message = append(message, 0)
	message = append(message, []byte(keyID)...)
	message = append(message, 0)
	message = append(message, nonce[:]...)
	message = append(message, ciphertext...)
	signature := ed25519.Sign(ed25519.NewKeyFromSeed(seed), message)
	envelope, err := json.Marshal(policyEnvelope{
		Version: 1, Purpose: "policy", KeyID: keyID,
		Nonce:      base64.StdEncoding.EncodeToString(nonce[:]),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Signature:  base64.StdEncoding.EncodeToString(signature),
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(envelope), nil
}
