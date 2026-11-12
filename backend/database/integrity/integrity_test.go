package integrity_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
)

func TestVerifyAndRecoverInterruptedWritesOnEncryptedDB(t *testing.T) {
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "integ.db"), "secret-key-xyz")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer mgr.Close()

	if _, err := migrations.Run(context.Background(), mgr.DB(), "1.0.0", nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := mgr.DB().Exec(`
		INSERT INTO write_recovery_journal (id, module, operation, entity_id, status)
		VALUES ('wr-1', 'boe', 'save', 'boe-1', 'PENDING');
		INSERT INTO boe_write_recovery (id, boe_calculation_id, operation, status)
		VALUES ('bwr-1', 'calc-1', 'save', 'pending');
	`); err != nil {
		t.Fatalf("seed pending writes: %v", err)
	}

	recovered, err := integrity.MarkInterruptedWritesFailed(mgr.DB())
	if err != nil {
		t.Fatalf("MarkInterruptedWritesFailed: %v", err)
	}
	if recovered != 2 {
		t.Fatalf("expected 2 recovered rows, got %d", recovered)
	}

	rep, err := integrity.Verify(mgr.DB())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !rep.IntegrityOK || rep.ForeignKeyErrors != 0 {
		t.Fatalf("unexpected integrity report: %+v", rep)
	}
	if rep.TableCount < 155 || rep.ViewCount != 1 {
		t.Fatalf("expected >=155 tables and 1 view, got tables=%d views=%d", rep.TableCount, rep.ViewCount)
	}
}
