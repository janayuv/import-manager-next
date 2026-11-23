package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestAppError(t *testing.T) {
	origErr := errors.New("underlying sqlite disk I/O failure")
	appErr := NewAppError(ErrDatabaseFailure, "Database is busy", "disk locked at row 42", origErr)
	appErr.WithField("table", "invoices")

	if appErr.Code != ErrDatabaseFailure {
		t.Errorf("expected code %s, got %s", ErrDatabaseFailure, appErr.Code)
	}

	if !errors.Is(appErr, origErr) {
		t.Errorf("expected appErr to wrap origErr")
	}

	errStr := appErr.Error()
	if !strings.Contains(errStr, "DATABASE_FAILURE") || !strings.Contains(errStr, "Database is busy") {
		t.Errorf("error string missing expected parts: %s", errStr)
	}

	if appErr.Fields["table"] != "invoices" {
		t.Errorf("expected field table=invoices, got %v", appErr.Fields["table"])
	}
}

func TestCorrelationID(t *testing.T) {
	ctx := context.Background()

	// Default without explicit ID should generate UUID
	cid := GetCorrelationID(ctx)
	if cid == "" {
		t.Fatal("expected non-empty correlation ID")
	}

	// Explicit correlation ID
	expectedID := "req-12345-test"
	ctxWithID := WithCorrelationID(ctx, expectedID)
	actualID := GetCorrelationID(ctxWithID)
	if actualID != expectedID {
		t.Fatalf("expected correlation ID %s, got %s", expectedID, actualID)
	}
}

func TestRedactingLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, slog.LevelInfo)

	ctx := WithCorrelationID(context.Background(), "corr-test-999")
	logger.Info(ctx, "user authenticated",
		"username", "admin",
		"password", "SuperSecret123!",
		"sqlcipher_key", "0xABCD1234",
		"tenant_id", "T001",
	)

	logged := buf.String()
	if !strings.Contains(logged, "corr-test-999") {
		t.Errorf("log missing correlation_id: %s", logged)
	}

	if strings.Contains(logged, "SuperSecret123!") {
		t.Errorf("log leaked sensitive password: %s", logged)
	}

	if strings.Contains(logged, "0xABCD1234") {
		t.Errorf("log leaked sqlcipher_key: %s", logged)
	}

	if !strings.Contains(logged, "[REDACTED]") {
		t.Errorf("log missing [REDACTED] tag: %s", logged)
	}

	// Verify valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("logged output is not valid JSON: %v", err)
	}
}

func TestMetricsRegistry(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, slog.LevelInfo)
	reg := NewRegistry(logger)
	reg.SetSlowQueryThreshold(10 * time.Millisecond)

	// Record job metrics
	reg.RecordJobDuration("cleanup_job", 150*time.Millisecond, "SUCCESS")
	reg.RecordJobDuration("cleanup_job", 200*time.Millisecond, "FAILED")

	jobMetrics := reg.GetJobMetrics()
	metric, ok := jobMetrics["cleanup_job"]
	if !ok {
		t.Fatal("missing cleanup_job metric")
	}
	if metric.TotalRuns != 2 || metric.SuccessRuns != 1 || metric.FailedRuns != 1 {
		t.Errorf("unexpected job counts: %+v", metric)
	}

	// Record normal query
	ctx := context.Background()
	reg.RecordQueryTiming(ctx, "SELECT", "invoices", 5*time.Millisecond)

	// Record slow query (> 10ms)
	reg.RecordQueryTiming(ctx, "SELECT", "boe_entries", 25*time.Millisecond)

	qMetrics := reg.GetQueryMetrics()
	slowMetric := qMetrics["SELECT:boe_entries"]
	if slowMetric.SlowCount != 1 {
		t.Errorf("expected 1 slow count for boe_entries, got %d", slowMetric.SlowCount)
	}

	// Verify warning logged for slow query
	if !strings.Contains(buf.String(), "slow query detected") {
		t.Errorf("expected slow query warning log: %s", buf.String())
	}
}

func TestNoOpTracer(t *testing.T) {
	tracer := NewNoOpTracer()
	ctx, span := tracer.Start(context.Background(), "test_operation")
	span.SetAttribute("test_key", "test_val")
	span.RecordError(nil)
	span.End()

	if ctx == nil {
		t.Fatal("expected non-nil context from tracer")
	}
}
