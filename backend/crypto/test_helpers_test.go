package crypto_test

import "os"

func writeDummy(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
