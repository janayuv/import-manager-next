package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// sensitiveKeys defines case-insensitive attribute names whose values must be masked.
var sensitiveKeys = map[string]bool{
	"password":       true,
	"secret":         true,
	"token":          true,
	"auth_token":     true,
	"sqlcipher_key":  true,
	"db_key":         true,
	"encryption_key": true,
	"client_secret":  true,
	"api_key":        true,
	"credential":     true,
}

// RedactValue masks strings if key is sensitive.
func RedactValue(key string, val slog.Value) slog.Value {
	lowerKey := strings.ToLower(key)
	for sens := range sensitiveKeys {
		if strings.Contains(lowerKey, sens) {
			return slog.StringValue("[REDACTED]")
		}
	}
	return val
}

// RedactingHandler wraps an slog.Handler to sanitize sensitive keys and inject correlation IDs.
type RedactingHandler struct {
	inner slog.Handler
}

// NewRedactingHandler constructs a handler that sanitizes logs and formats as structured JSON.
func NewRedactingHandler(w io.Writer, level slog.Level) *RedactingHandler {
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return slog.Attr{
				Key:   a.Key,
				Value: RedactValue(a.Key, a.Value),
			}
		},
	}
	return &RedactingHandler{
		inner: slog.NewJSONHandler(w, opts),
	}
}

// Enabled implements slog.Handler.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle implements slog.Handler and adds correlation_id if present.
func (h *RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	cid := GetCorrelationID(ctx)
	record.Add(slog.String("correlation_id", cid))
	return h.inner.Handle(ctx, record)
}

// WithAttrs implements slog.Handler.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &RedactingHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{inner: h.inner.WithGroup(name)}
}

// Logger provides convenient context-aware logging methods.
type Logger struct {
	slogLogger *slog.Logger
}

// NewLogger creates a new structured JSON Logger writing to the given writer.
func NewLogger(w io.Writer, level slog.Level) *Logger {
	if w == nil {
		w = os.Stdout
	}
	handler := NewRedactingHandler(w, level)
	return &Logger{
		slogLogger: slog.New(handler),
	}
}

// Slog returns the underlying standard slog.Logger.
func (l *Logger) Slog() *slog.Logger {
	return l.slogLogger
}

// Info logs at INFO level with context and correlation ID.
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	l.slogLogger.InfoContext(ctx, msg, args...)
}

// Warn logs at WARN level with context and correlation ID.
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	l.slogLogger.WarnContext(ctx, msg, args...)
}

// Error logs at ERROR level with context and correlation ID.
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	l.slogLogger.ErrorContext(ctx, msg, args...)
}

// Debug logs at DEBUG level with context and correlation ID.
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
	l.slogLogger.DebugContext(ctx, msg, args...)
}
