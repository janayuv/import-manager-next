package bootstrap_test

import (
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/app/lifecycle"
)

func TestFullBootstrapAndShutdownWithSQLCipher(t *testing.T) {
	t.Setenv("IMPORT_MANAGER_DB_KEY", "bootstrap-test-secret-0123456789abcdef")
	dataDir := filepath.Join(t.TempDir(), "appdata")

	rt, err := bootstrap.Initialize(nil, dataDir)
	if err != nil {
		t.Fatalf("bootstrap.Initialize failed: %v", err)
	}
	coord := lifecycle.NewCoordinator(rt)
	defer coord.Close()

	if rt.MigrationResult.CurrentVersion != 19 || rt.MigrationResult.AppliedCount != 19 {
		t.Fatalf("unexpected migration result: %+v", rt.MigrationResult)
	}
	if !rt.Integrity.IntegrityOK || rt.Integrity.ForeignKeyErrors != 0 {
		t.Fatalf("unexpected integrity report: %+v", rt.Integrity)
	}
	if rt.Integrity.TableCount < 155 || rt.Integrity.ViewCount != 1 {
		t.Fatalf("unexpected schema counts: tables=%d views=%d", rt.Integrity.TableCount, rt.Integrity.ViewCount)
	}
	if !rt.Health.Encrypted || rt.Health.JournalMode != "wal" || !rt.Health.ForeignKeysOn {
		t.Fatalf("unexpected DB health: %+v", rt.Health)
	}

	if err := coord.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	// Second close must be idempotent.
	if err := coord.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}
