package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/danieljoos/wincred"
)

const (
	CredentialServiceName         = "ImportManager"
	dbKeyAccountName              = "ImportManager_DatabaseKey_v1"
	BackupPassphraseAccount       = "ImportManager_BackupPassphrase_v1"
	GoogleDriveTokensAccount      = "ImportManager_GoogleDriveTokens_v1"
	GoogleDriveOAuthClientAccount = "ImportManager_GoogleDriveOAuthClient_v1"
)

// ErrSecretNotFound is returned when a named secret is not present in the credential store.
var ErrSecretNotFound = errors.New("secret not found")

// KeyStore abstracts retrieval and persistence of the local database encryption key.
type KeyStore interface {
	GetOrCreateDatabaseKey() (string, error)
}

// SecretStore abstracts named secret persistence (Windows Credential Manager in production,
// or an isolated JSON keyring file during tests when IMPORT_MANAGER_KEYRING_FILE is set).
type SecretStore interface {
	GetSecret(account string) (string, error)
	SetSecret(account, secret string) error
	DeleteSecret(account string) error
}

// WinCredKeyStore uses Windows Credential Manager to protect keys and OAuth tokens.
type WinCredKeyStore struct {
	Service string
	Account string
	mu      sync.Mutex
}

// NewWinCredKeyStore returns the default Windows Credential Manager store.
func NewWinCredKeyStore() *WinCredKeyStore {
	return &WinCredKeyStore{
		Service: CredentialServiceName,
		Account: dbKeyAccountName,
	}
}

func targetName(service, account string) string {
	return service + "/" + account
}

// GetSecret reads a named secret from the credential store.
func (s *WinCredKeyStore) GetSecret(account string) (string, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return "", errors.New("credential account name is required")
	}
	if keyringFile := strings.TrimSpace(os.Getenv("IMPORT_MANAGER_KEYRING_FILE")); keyringFile != "" {
		return s.getSecretFile(keyringFile, account)
	}
	target := targetName(s.Service, account)
	cred, err := wincred.GetGenericCredential(target)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return "", ErrSecretNotFound
		}
		return "", fmt.Errorf("read credential %s: %w", target, err)
	}
	val := strings.TrimSpace(string(cred.CredentialBlob))
	if val == "" {
		return "", ErrSecretNotFound
	}
	return val, nil
}

// SetSecret writes a named secret into the credential store.
func (s *WinCredKeyStore) SetSecret(account, secret string) error {
	account = strings.TrimSpace(account)
	if account == "" {
		return errors.New("credential account name is required")
	}
	if secret == "" {
		return errors.New("secret value cannot be empty")
	}
	if keyringFile := strings.TrimSpace(os.Getenv("IMPORT_MANAGER_KEYRING_FILE")); keyringFile != "" {
		return s.setSecretFile(keyringFile, account, secret)
	}
	target := targetName(s.Service, account)
	gen := wincred.NewGenericCredential(target)
	gen.UserName = account
	gen.CredentialBlob = []byte(secret)
	gen.Persist = wincred.PersistLocalMachine
	if err := gen.Write(); err != nil {
		return fmt.Errorf("store credential %s: %w", target, err)
	}
	return nil
}

// DeleteSecret removes a named secret from the credential store (idempotent if absent).
func (s *WinCredKeyStore) DeleteSecret(account string) error {
	account = strings.TrimSpace(account)
	if account == "" {
		return errors.New("credential account name is required")
	}
	if keyringFile := strings.TrimSpace(os.Getenv("IMPORT_MANAGER_KEYRING_FILE")); keyringFile != "" {
		return s.deleteSecretFile(keyringFile, account)
	}
	target := targetName(s.Service, account)
	cred, err := wincred.GetGenericCredential(target)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return nil
		}
		return fmt.Errorf("lookup credential %s: %w", target, err)
	}
	if err := cred.Delete(); err != nil && !errors.Is(err, wincred.ErrElementNotFound) {
		return fmt.Errorf("delete credential %s: %w", target, err)
	}
	return nil
}

func (s *WinCredKeyStore) getSecretFile(path, account string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := readKeyringMap(path)
	if err != nil {
		return "", err
	}
	val, ok := m[account]
	if !ok || val == "" {
		return "", ErrSecretNotFound
	}
	return val, nil
}

func (s *WinCredKeyStore) setSecretFile(path, account, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := readKeyringMap(path)
	if err != nil {
		return err
	}
	m[account] = secret
	return writeKeyringMap(path, m)
}

func (s *WinCredKeyStore) deleteSecretFile(path, account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := readKeyringMap(path)
	if err != nil {
		return err
	}
	delete(m, account)
	return writeKeyringMap(path, m)
}

func readKeyringMap(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read test keyring file: %w", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse test keyring file: %w", err)
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

func writeKeyringMap(path string, m map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create test keyring dir: %w", err)
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode test keyring file: %w", err)
	}
	return os.WriteFile(path, raw, 0o600)
}

// GetOrCreateDatabaseKey reads the persisted 256-bit hex key from Windows Credential Manager,
// creating and storing a new cryptographically random key on first run.
func (s *WinCredKeyStore) GetOrCreateDatabaseKey() (string, error) {
	if envKey := strings.TrimSpace(os.Getenv("IMPORT_MANAGER_DB_KEY")); envKey != "" {
		return envKey, nil
	}

	if existing, err := s.GetSecret(s.Account); err == nil && existing != "" {
		return existing, nil
	} else if err != nil && !errors.Is(err, ErrSecretNotFound) {
		return "", err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random database key: %w", err)
	}
	newKey := hex.EncodeToString(raw)

	if err := s.SetSecret(s.Account, newKey); err != nil {
		return "", fmt.Errorf("store database key: %w", err)
	}
	return newKey, nil
}
