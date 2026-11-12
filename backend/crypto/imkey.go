package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	imkeyMagic      = "IMKEY1"
	imkeySaltLen    = 16
	imkeyNonceLen   = 12
	imkeyIterations = 600_000
)

// IMKeyPayload represents the key material serialized inside an encrypted .imkey file.
type IMKeyPayload struct {
	Version          int    `json:"version"`
	ExportedAt       string `json:"exportedAt"`
	BackupPassphrase string `json:"backupPassphrase,omitempty"`
	DatabaseKeyHint  string `json:"databaseKeyHint,omitempty"`
}

// EncryptIMKey encrypts key material with a user-supplied transport passphrase into IMKEY1 binary format.
func EncryptIMKey(transportPassphrase string, payload IMKeyPayload) ([]byte, error) {
	if strings.TrimSpace(transportPassphrase) == "" {
		return nil, errors.New("transport passphrase is required for .imkey export")
	}
	if strings.TrimSpace(payload.BackupPassphrase) == "" {
		return nil, errors.New("backup passphrase cannot be empty in .imkey export")
	}
	if payload.Version == 0 {
		payload.Version = 1
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal imkey payload: %w", err)
	}

	salt := make([]byte, imkeySaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate imkey salt: %w", err)
	}
	nonce := make([]byte, imkeyNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate imkey nonce: %w", err)
	}

	key := pbkdf2.Key([]byte(transportPassphrase), salt, imkeyIterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create imkey cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create imkey gcm: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, raw, []byte(imkeyMagic))
	out := make([]byte, 0, len(imkeyMagic)+imkeySaltLen+imkeyNonceLen+len(ciphertext))
	out = append(out, []byte(imkeyMagic)...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// DecryptIMKey decrypts and parses an IMKEY1 binary payload.
func DecryptIMKey(transportPassphrase string, data []byte) (IMKeyPayload, error) {
	if strings.TrimSpace(transportPassphrase) == "" {
		return IMKeyPayload{}, errors.New("transport passphrase is required for .imkey import")
	}
	headerLen := len(imkeyMagic) + imkeySaltLen + imkeyNonceLen
	if len(data) <= headerLen {
		return IMKeyPayload{}, errors.New("invalid .imkey file: payload too short")
	}
	if string(data[:len(imkeyMagic)]) != imkeyMagic {
		return IMKeyPayload{}, errors.New("invalid .imkey header (expected IMKEY1)")
	}

	salt := data[len(imkeyMagic) : len(imkeyMagic)+imkeySaltLen]
	nonce := data[len(imkeyMagic)+imkeySaltLen : headerLen]
	ciphertext := data[headerLen:]

	key := pbkdf2.Key([]byte(transportPassphrase), salt, imkeyIterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return IMKeyPayload{}, fmt.Errorf("create imkey cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return IMKeyPayload{}, fmt.Errorf("create imkey gcm: %w", err)
	}

	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(imkeyMagic))
	if err != nil {
		return IMKeyPayload{}, errors.New("failed to decrypt .imkey file (invalid passphrase or corrupted key file)")
	}

	var payload IMKeyPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return IMKeyPayload{}, fmt.Errorf("parse decrypted .imkey payload: %w", err)
	}
	if strings.TrimSpace(payload.BackupPassphrase) == "" {
		return IMKeyPayload{}, errors.New(".imkey file did not contain a valid backup passphrase")
	}
	return payload, nil
}
