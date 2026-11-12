package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const (
	argonMemoryKiB = 65536 // 64 MiB
	argonTime      = 3
	argonThreads   = 4
	argonKeyLen    = 32
	argonSaltLen   = 16
	MinPasswordLen = 8
)

var ErrPasswordPolicy = errors.New("password policy violation")

// ValidatePasswordComplexity enforces the legacy Import Manager password rules:
// >= 8 chars, at least 1 uppercase, 1 lowercase, 1 digit, 1 symbol, and no whitespace.
func ValidatePasswordComplexity(password string) error {
	if len(password) < MinPasswordLen {
		return fmt.Errorf("%w: must be at least %d characters", ErrPasswordPolicy, MinPasswordLen)
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsSpace(r):
			return fmt.Errorf("%w: whitespace is not allowed", ErrPasswordPolicy)
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	if !hasUpper {
		return fmt.Errorf("%w: must include an uppercase letter", ErrPasswordPolicy)
	}
	if !hasLower {
		return fmt.Errorf("%w: must include a lowercase letter", ErrPasswordPolicy)
	}
	if !hasDigit {
		return fmt.Errorf("%w: must include a digit", ErrPasswordPolicy)
	}
	if !hasSymbol {
		return fmt.Errorf("%w: must include a special character", ErrPasswordPolicy)
	}
	return nil
}

// HashPassword produces an Argon2id PHC-encoded password hash string.
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", errors.New("password cannot be empty")
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(plain), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemoryKiB,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword verifies a plaintext password against an Argon2id PHC string or legacy bcrypt hash.
// Returns needsRehash=true when a valid legacy hash should be upgraded to current Argon2id parameters.
func VerifyPassword(plain, encoded string) (match bool, needsRehash bool, err error) {
	if plain == "" || encoded == "" {
		return false, false, nil
	}
	if strings.HasPrefix(encoded, "$2a$") || strings.HasPrefix(encoded, "$2b$") || strings.HasPrefix(encoded, "$2y$") {
		err = bcrypt.CompareHashAndPassword([]byte(encoded), []byte(plain))
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, false, nil
		}
		if err != nil {
			return false, false, fmt.Errorf("verify bcrypt hash: %w", err)
		}
		return true, true, nil
	}

	if !strings.HasPrefix(encoded, "$argon2id$") {
		return false, false, fmt.Errorf("unsupported password hash algorithm")
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, false, fmt.Errorf("invalid argon2id hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, false, fmt.Errorf("unsupported argon2id version")
	}

	var mem uint32
	var iter uint32
	var par uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iter, &par); err != nil {
		return false, false, fmt.Errorf("invalid argon2id parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, false, fmt.Errorf("decode argon2id salt: %w", err)
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, false, fmt.Errorf("decode argon2id hash: %w", err)
	}

	actualHash := argon2.IDKey([]byte(plain), salt, iter, mem, par, uint32(len(expectedHash)))
	if subtle.ConstantTimeCompare(actualHash, expectedHash) != 1 {
		return false, false, nil
	}
	rehash := mem < argonMemoryKiB || iter < argonTime || uint32(len(expectedHash)) < argonKeyLen
	return true, rehash, nil
}
