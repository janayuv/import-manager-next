//go:build !production

package crypto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func getTestKeyringFile() string {
	// Only allow the plaintext JSON keyring fallback inside `go test` runs in non-production builds.
	if !testing.Testing() {
		return ""
	}
	return strings.TrimSpace(os.Getenv("IMPORT_MANAGER_KEYRING_FILE"))
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
