package observability

import (
	"context"
)

// Span represents a single span of trace execution.
type Span interface {
	End()
	SetAttribute(key string, value any)
	RecordError(err error)
}

// Tracer defines the tracing contract for application execution spans.
type Tracer interface {
	Start(ctx context.Context, spanName string) (context.Context, Span)
}

// NoOpSpan is an empty, zero-allocation span implementation.
type NoOpSpan struct{}

func (s *NoOpSpan) End()                         {}
func (s *NoOpSpan) SetAttribute(key string, val any) {}
func (s *NoOpSpan) RecordError(err error)       {}

// NoOpTracer is the default local tracer requiring zero network egress and zero memory allocation.
type NoOpTracer struct{}

// NewNoOpTracer constructs a default zero-overhead tracer.
func NewNoOpTracer() *NoOpTracer {
	return &NoOpTracer{}
}

// Start returns the context and a no-op span.
func (t *NoOpTracer) Start(ctx context.Context, spanName string) (context.Context, Span) {
	return ctx, &NoOpSpan{}
}
