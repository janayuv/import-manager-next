package observability

import (
	"context"

	"github.com/google/uuid"
)

type correlationIDKeyType struct{}

var correlationIDKey = correlationIDKeyType{}

// WithCorrelationID attaches a correlation ID to the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.New().String()
	}
	return context.WithValue(ctx, correlationIDKey, id)
}

// GetCorrelationID extracts the correlation ID from context or generates a new one.
func GetCorrelationID(ctx context.Context) string {
	if ctx == nil {
		return uuid.New().String()
	}
	if val, ok := ctx.Value(correlationIDKey).(string); ok && val != "" {
		return val
	}
	return uuid.New().String()
}
