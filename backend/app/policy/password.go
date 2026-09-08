package policy

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/nacl/secretbox"
)

func EncryptPassword(value, encodedKey string) (string, error) {
	if value == "" {
		return "", nil
	}
	key, err := decodeSecretboxKey(encodedKey)
	if err != nil {
		return "", err
	}
	var nonce [24]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return "", err
	}
	result := append([]byte{}, nonce[:]...)
	result = secretbox.Seal(result, []byte(value), &nonce, &key)
	return base64.StdEncoding.EncodeToString(result), nil
}

func DecryptPassword(value, encodedKey string) (string, error) {
	if value == "" {
		return "", nil
	}
	key, err := decodeSecretboxKey(encodedKey)
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) < 24+secretbox.Overhead {
		return "", errors.New("invalid stored device group password")
	}
	var nonce [24]byte
	copy(nonce[:], decoded[:24])
	plaintext, ok := secretbox.Open(nil, decoded[24:], &nonce, &key)
	if !ok {
		return "", errors.New("failed to decrypt stored device group password")
	}
	return string(plaintext), nil
}

func decodeSecretboxKey(value string) ([32]byte, error) {
	var key [32]byte
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != len(key) {
		return key, errors.New("provisioning encryption key is not configured")
	}
	copy(key[:], decoded)
	return key, nil
}
