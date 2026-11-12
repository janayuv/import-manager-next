package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	BackupMagicV2      = "IMBK2"
	BackupMagicV1      = "IMBK1"
	backupSaltLen      = 16
	backupNonceLen     = 12
	backupKeyLen       = 32
	pbkdf2ItersV2      = 600000
	pbkdf2ItersV1      = 100000
)

// EncryptBackupIMBK2 encrypts raw backup bytes using the exact IMBK2 wire format:
// 5B "IMBK2" + 16B salt + 12B nonce + AES-256-GCM ciphertext (PBKDF2-HMAC-SHA256, 600k iters).
func EncryptBackupIMBK2(plaintext []byte, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, errors.New("backup passphrase is required")
	}
	salt := make([]byte, backupSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate backup salt: %w", err)
	}
	nonce := make([]byte, backupNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate backup nonce: %w", err)
	}

	key := pbkdf2.Key([]byte(passphrase), salt, pbkdf2ItersV2, backupKeyLen, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init aes-gcm: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(BackupMagicV2)+backupSaltLen+backupNonceLen+len(ciphertext))
	out = append(out, []byte(BackupMagicV2)...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// DecryptBackup decrypts IMBK2 (600k iters) and legacy IMBK1 (100k iters) backup payloads.
func DecryptBackup(payload []byte, passphrase string) (plaintext []byte, formatVersion string, err error) {
	if passphrase == "" {
		return nil, "", errors.New("backup passphrase is required")
	}
	headerLen := len(BackupMagicV2)
	minLen := headerLen + backupSaltLen + backupNonceLen + 16
	if len(payload) < minLen {
		return nil, "", errors.New("backup payload truncated or invalid")
	}

	magic := string(payload[:headerLen])
	var iters int
	switch magic {
	case BackupMagicV2:
		iters = pbkdf2ItersV2
	case BackupMagicV1:
		iters = pbkdf2ItersV1
	default:
		return nil, "", fmt.Errorf("unsupported backup header %q (expected IMBK2 or IMBK1)", magic)
	}

	salt := payload[headerLen : headerLen+backupSaltLen]
	nonce := payload[headerLen+backupSaltLen : headerLen+backupSaltLen+backupNonceLen]
	ciphertext := payload[headerLen+backupSaltLen+backupNonceLen:]

	key := pbkdf2.Key([]byte(passphrase), salt, iters, backupKeyLen, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, magic, fmt.Errorf("init aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, magic, fmt.Errorf("init aes-gcm: %w", err)
	}

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, magic, errors.New("decryption failed: invalid passphrase or corrupted backup")
	}
	return plain, magic, nil
}

// SHA256Hex computes lowercase hex SHA-256 over data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// WriteSidecarSHA256 writes a "<path>.sha256" checksum sidecar file.
func WriteSidecarSHA256(targetPath string, data []byte) (string, error) {
	sidecar := targetPath + ".sha256"
	hashHex := SHA256Hex(data)
	if err := os.WriteFile(sidecar, []byte(hashHex+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write sha256 sidecar: %w", err)
	}
	return hashHex, nil
}

// VerifyFileSidecarSHA256 reads the file and validates its "<path>.sha256" sidecar checksum if present.
func VerifyFileSidecarSHA256(targetPath string) (string, error) {
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("read backup file: %w", err)
	}
	actual := SHA256Hex(data)
	sidecarRaw, err := os.ReadFile(targetPath + ".sha256")
	if err != nil {
		return actual, fmt.Errorf("read sha256 sidecar: %w", err)
	}
	var expected string
	_, _ = fmt.Sscanf(string(sidecarRaw), "%64s", &expected)
	if expected != actual {
		return actual, fmt.Errorf("backup checksum mismatch: expected=%s actual=%s", expected, actual)
	}
	return actual, nil
}
