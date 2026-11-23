package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Logger wraps structured slog logging directed to both stderr and the AppData log file.
type Logger struct {
	*slog.Logger
	mu   sync.Mutex
	file *os.File
	path string
}

type resilientLogWriter struct {
	file   io.Writer
	stderr io.Writer
}

func (w resilientLogWriter) Write(p []byte) (int, error) {
	if w.stderr != nil {
		// Windows GUI binaries (-H windowsgui) may lack a valid stderr handle; ignore stderr errors.
		_, _ = w.stderr.Write(p)
	}
	if w.file != nil {
		return w.file.Write(p)
	}
	return len(p), nil
}

// New opens or appends to the daily/application log file under logsDir with restricted permissions.
func New(logsDir string) (*Logger, error) {
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		return nil, fmt.Errorf("ensure logs dir: %w", err)
	}
	logPath := filepath.Join(logsDir, "import-manager.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", logPath, err)
	}

	mw := resilientLogWriter{file: f, stderr: os.Stderr}
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			lowerKey := strings.ToLower(a.Key)
			for _, sens := range []string{"password", "secret", "token", "auth_token", "sqlcipher_key", "db_key", "encryption_key", "client_secret", "api_key", "credential"} {
				if strings.Contains(lowerKey, sens) {
					return slog.Attr{Key: a.Key, Value: slog.StringValue("[REDACTED]")}
				}
			}
			return a
		},
	}
	handler := slog.NewJSONHandler(mw, opts)
	return &Logger{
		Logger: slog.New(handler),
		file:   f,
		path:   logPath,
	}, nil
}

// Path returns the active log file path.
func (l *Logger) Path() string {
	return l.path
}

// Close flushes and closes the underlying log file handle.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}
