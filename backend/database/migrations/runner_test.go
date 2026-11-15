package migrations_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
)

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestEmbeddedMigrationsExecuteCleanlyOnSQLCipher(t *testing.T) {
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "erp.db"), "strong-key-001")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer mgr.Close()

	ctx := context.Background()
	res, err := migrations.Run(ctx, mgr.DB(), "1.0.0", nil)
	if err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}
	if res.AppliedCount != 8 || res.CurrentVersion != 8 {
		t.Fatalf("expected 8 migrations applied to v8, got %+v", res)
	}

	// Re-run should be a no-op.
	res2, err := migrations.Run(ctx, mgr.DB(), "1.0.0", nil)
	if err != nil {
		t.Fatalf("second migration run failed: %v", err)
	}
	if res2.AppliedCount != 0 || res2.CurrentVersion != 8 {
		t.Fatalf("expected 0 pending migrations on second run, got %+v", res2)
	}

	var supCount int
	if err := mgr.DB().QueryRow(`SELECT COUNT(*) FROM suppliers`).Scan(&supCount); err != nil || supCount != 11 {
		t.Fatalf("expected 11 seeded suppliers, got %d (err=%v)", supCount, err)
	}

	applied, err := migrations.ListApplied(ctx, mgr.DB())
	if err != nil || len(applied) != 8 {
		t.Fatalf("expected 8 applied records, got %d (err=%v)", len(applied), err)
	}
	for _, r := range applied {
		if r.Status != "APPLIED" || r.Checksum == "" || r.AppVersion != "1.0.0" {
			t.Fatalf("unexpected migration record: %+v", r)
		}
	}
}

func TestMigrationGuardsChecksumDowngradeFailureAndPreBackup(t *testing.T) {
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "guards.db"), "strong-key-002")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer mgr.Close()
	ctx := context.Background()

	v1SQL := "CREATE TABLE t1 (id INTEGER PRIMARY KEY);"
	v1 := migrations.Migration{Version: 1, Name: "0001_init.sql", SQL: v1SQL, Checksum: sha(v1SQL)}
	if _, err := migrations.RunWithMigrations(ctx, mgr.DB(), "1.0.0", []migrations.Migration{v1}, nil); err != nil {
		t.Fatalf("apply v1: %v", err)
	}

	// Upgrade triggers pre-migration backup hook.
	v2SQL := "CREATE TABLE t2 (id INTEGER PRIMARY KEY);"
	v2 := migrations.Migration{Version: 2, Name: "0002_next.sql", SQL: v2SQL, Checksum: sha(v2SQL)}
	hookCalled := false
	resUp, err := migrations.RunWithMigrations(ctx, mgr.DB(), "1.1.0", []migrations.Migration{v1, v2}, func(from, to int) (string, error) {
		hookCalled = true
		if from != 1 || to != 2 {
			t.Fatalf("unexpected hook versions %d -> %d", from, to)
		}
		return "backup-v1-to-v2.db", nil
	})
	if err != nil || !hookCalled || resUp.PreBackupPath != "backup-v1-to-v2.db" {
		t.Fatalf("upgrade backup hook failed: hookCalled=%v res=%+v err=%v", hookCalled, resUp, err)
	}

	// Checksum tamper detection.
	v2Tampered := migrations.Migration{Version: 2, Name: "0002_next.sql", SQL: "SELECT 1;", Checksum: sha("SELECT 1;")}
	if _, err := migrations.RunWithMigrations(ctx, mgr.DB(), "1.1.0", []migrations.Migration{v1, v2Tampered}, nil); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("expected SHA-256 mismatch error, got %v", err)
	}

	// Downgrade protection.
	if _, err := migrations.RunWithMigrations(ctx, mgr.DB(), "0.9.0", []migrations.Migration{v1}, nil); err == nil || !strings.Contains(err.Error(), "downgrade refused") {
		t.Fatalf("expected downgrade refused error, got %v", err)
	}

	// Failed migration rolls back SQL and blocks subsequent runs.
	badSQL := "CREATE TABLE t3 (id INTEGER PRIMARY KEY); INVALID SQL SYNTAX;"
	v3Bad := migrations.Migration{Version: 3, Name: "0003_bad.sql", SQL: badSQL, Checksum: sha(badSQL)}
	if _, err := migrations.RunWithMigrations(ctx, mgr.DB(), "1.2.0", []migrations.Migration{v1, v2, v3Bad}, nil); err == nil {
		t.Fatalf("expected failing migration to return error")
	}
	var t3Exists int
	_ = mgr.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='t3'`).Scan(&t3Exists)
	if t3Exists != 0 {
		t.Fatalf("expected t3 creation to roll back")
	}
	if _, err := migrations.RunWithMigrations(ctx, mgr.DB(), "1.2.0", []migrations.Migration{v1, v2, v3Bad}, nil); err == nil || !strings.Contains(err.Error(), "status=FAILED") {
		t.Fatalf("expected blocked FAILED status error, got %v", err)
	}
}

func TestMigrationLineEndingChecksumEquivalence(t *testing.T) {
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "line_endings.db"), "strong-key-003")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer mgr.Close()
	ctx := context.Background()

	// Apply migration with CRLF
	crlfSQL := "CREATE TABLE t_crlf (\r\n  id INTEGER PRIMARY KEY,\r\n  val TEXT\r\n);"
	m1CRLF := migrations.Migration{
		Version:  1,
		Name:     "0001_crlf.sql",
		SQL:      crlfSQL,
		Checksum: sha(crlfSQL),
	}
	if _, err := migrations.RunWithMigrations(ctx, mgr.DB(), "1.0.0", []migrations.Migration{m1CRLF}, nil); err != nil {
		t.Fatalf("apply v1 CRLF: %v", err)
	}

	// Verify using LF version of the same SQL succeeds without SHA-256 mismatch
	lfSQL := "CREATE TABLE t_crlf (\n  id INTEGER PRIMARY KEY,\n  val TEXT\n);"
	m1LF := migrations.Migration{
		Version:  1,
		Name:     "0001_crlf.sql",
		SQL:      lfSQL,
		Checksum: sha(lfSQL),
	}
	if _, err := migrations.RunWithMigrations(ctx, mgr.DB(), "1.0.0", []migrations.Migration{m1LF}, nil); err != nil {
		t.Fatalf("expected LF version to match CRLF applied record, got error: %v", err)
	}
}
