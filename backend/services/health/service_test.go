package health

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mutecomm/go-sqlcipher/v4"
	"github.com/janayuv/import-manager-next/backend/windows"
)

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"invalid", ""},
		{"a@example.com", "a***@example.com"},
		{"ab@example.com", "a***@example.com"},
		{"abc@example.com", "a***c@example.com"},
		{"johndoe@example.com", "j***e@example.com"},
	}

	for _, tt := range tests {
		actual := maskEmail(tt.input)
		if actual != tt.expected {
			t.Errorf("maskEmail(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	// Create required schema tables for health check
	ddl := `
		CREATE TABLE next_schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL,
			checksum TEXT NOT NULL,
			status TEXT NOT NULL
		);
		INSERT INTO next_schema_migrations (version, name, applied_at, checksum, status)
		VALUES (19, 'test_migration', '2026-11-23 00:00:00', 'hash', 'APPLIED');

		CREATE TABLE workflow_job_execution_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			completed_at TEXT,
			duration_ms INTEGER,
			error_message TEXT
		);

		CREATE TABLE audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			action TEXT NOT NULL,
			outcome TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("failed to execute DDL: %v", err)
	}
	return db
}

func TestHealthService_GetHealthSummary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tempDir := t.TempDir()
	paths := windows.AppPaths{
		DataDir:    tempDir,
		BackupsDir: tempDir,
	}

	svc := NewService(db, nil, filepath.Join(tempDir, "test.db"), nil, nil, nil, nil, nil, paths)

	ctx := context.Background()
	summary, err := svc.GetHealthSummary(ctx)
	if err != nil {
		t.Fatalf("GetHealthSummary failed: %v", err)
	}

	if summary.SchemaVersion != 19 {
		t.Errorf("expected SchemaVersion 19, got %d", summary.SchemaVersion)
	}
	if summary.DatabaseIntegrity != "OK" {
		t.Errorf("expected DatabaseIntegrity OK, got %s", summary.DatabaseIntegrity)
	}
	if summary.DatabaseDiskTotalBytes == 0 {
		t.Logf("DatabaseDiskTotalBytes is 0 (may depend on OS platform check)")
	}
}

func TestHealthService_RunManualIntegrityCheck(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	svc := NewService(db, nil, "", nil, nil, nil, nil, nil, windows.AppPaths{})
	ctx := context.Background()

	result, err := svc.RunManualIntegrityCheck(ctx)
	if err != nil {
		t.Fatalf("RunManualIntegrityCheck failed: %v", err)
	}
	if result != "ok" && result != "OK" {
		t.Errorf("expected 'ok', got %q", result)
	}

	// Verify that subsequent summary reports the integrity check
	summary, err := svc.GetHealthSummary(ctx)
	if err != nil {
		t.Fatalf("GetHealthSummary failed: %v", err)
	}
	if summary.LastIntegrityCheckAt == "" {
		t.Errorf("expected LastIntegrityCheckAt to be populated")
	}
}

func TestHealthService_RunIdempotentJobRecovery(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert stuck job execution
	_, err := db.Exec(`
		INSERT INTO workflow_job_execution_log (job_id, status, started_at)
		VALUES ('test_job_1', 'RUNNING', '2026-11-23 00:00:00')
	`)
	if err != nil {
		t.Fatalf("failed to insert running job log: %v", err)
	}

	svc := NewService(db, nil, "", nil, nil, nil, nil, nil, windows.AppPaths{})
	ctx := context.Background()

	if err := svc.RunIdempotentJobRecovery(ctx, "test_job_1"); err != nil {
		t.Fatalf("RunIdempotentJobRecovery failed: %v", err)
	}

	var status string
	err = db.QueryRow("SELECT status FROM workflow_job_execution_log WHERE job_id = 'test_job_1'").Scan(&status)
	if err != nil {
		t.Fatalf("failed to query job status: %v", err)
	}
	if status != "TIMEOUT" {
		t.Errorf("expected status 'TIMEOUT', got %q", status)
	}
}
