package logging

import (
	"os"
	"strings"
	"testing"
)

func TestLoggerRedactsSensitiveData(t *testing.T) {
	tempDir := t.TempDir()
	logger, err := New(tempDir)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	logger.Info("user authentication attempt",
		"username", "admin",
		"password", "PlainPassword123!",
		"sqlcipher_key", "secretKey0x999",
		"auth_token", "jwt.token.abc",
	)

	_ = logger.Close()

	content, err := os.ReadFile(logger.Path())
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logStr := string(content)
	if strings.Contains(logStr, "PlainPassword123!") {
		t.Errorf("log leaked sensitive password: %s", logStr)
	}
	if strings.Contains(logStr, "secretKey0x999") {
		t.Errorf("log leaked sensitive sqlcipher_key: %s", logStr)
	}
	if strings.Contains(logStr, "jwt.token.abc") {
		t.Errorf("log leaked sensitive auth_token: %s", logStr)
	}
	if !strings.Contains(logStr, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in log output: %s", logStr)
	}
}
