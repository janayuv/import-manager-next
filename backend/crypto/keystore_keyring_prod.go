//go:build production

package crypto

import "errors"

func getTestKeyringFile() string {
	return ""
}

func (s *WinCredKeyStore) getSecretFile(_, _ string) (string, error) {
	return "", errors.New("test keyring file disabled in production builds")
}

func (s *WinCredKeyStore) setSecretFile(_, _, _ string) error {
	return errors.New("test keyring file disabled in production builds")
}

func (s *WinCredKeyStore) deleteSecretFile(_, _ string) error {
	return errors.New("test keyring file disabled in production builds")
}
