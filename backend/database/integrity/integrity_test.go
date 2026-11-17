package integrity_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/crypto"
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

func TestInspectAllForeignKeys(t *testing.T) {
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "fk_inspect.db"), "secret-key-xyz")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer mgr.Close()

	if _, err := migrations.Run(context.Background(), mgr.DB(), "1.0.0", nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	rows, err := mgr.DB().Query(`
		SELECT m.name as child_table, fk."from" as from_col, fk."table" as parent_table, fk."to" as to_col, fk."on_delete", fk."on_update"
		FROM sqlite_master m
		JOIN pragma_foreign_key_list(m.name) fk
		WHERE m.type = 'table'
		ORDER BY m.name, fk.id;
	`)
	if err != nil {
		t.Fatalf("query fk list: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var childTable, fromCol, parentTable, toCol, onDelete, onUpdate string
		if err := rows.Scan(&childTable, &fromCol, &parentTable, &toCol, &onDelete, &onUpdate); err != nil {
			t.Fatalf("scan fk: %v", err)
		}
		t.Logf("FK: %s(%s) -> %s(%s) [ON DELETE: %s, ON UPDATE: %s]", childTable, fromCol, parentTable, toCol, onDelete, onUpdate)
	}
}

func TestLiveUserDatabaseIntegrityAndForeignKeys(t *testing.T) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		t.Skip("APPDATA not set")
	}
	dbPath := filepath.Join(appData, "ImportManager", "import_manager.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skip("Live database does not exist at " + dbPath)
	}

	keyStore := crypto.NewWinCredKeyStore()
	dbKey, err := keyStore.GetOrCreateDatabaseKey()
	if err != nil {
		t.Fatalf("resolve db key: %v", err)
	}

	mgr, err := connection.OpenEncrypted(dbPath, dbKey)
	if err != nil {
		t.Fatalf("open live encrypted db: %v", err)
	}
	defer mgr.Close()

	// 1. Run PRAGMA integrity_check
	rep, err := integrity.Verify(mgr.DB())
	if err != nil {
		t.Fatalf("integrity verify failed: %v", err)
	}
	t.Logf("LIVE DB IntegrityOK: %v", rep.IntegrityOK)
	t.Logf("LIVE DB IntegrityMessages: %v", rep.IntegrityMessages)
	t.Logf("LIVE DB ForeignKeyErrors: %d", rep.ForeignKeyErrors)
	t.Logf("LIVE DB TableCount: %d, ViewCount: %d", rep.TableCount, rep.ViewCount)

	// 2. Run detailed PRAGMA foreign_key_check
	fkRows, err := mgr.DB().Query("PRAGMA foreign_key_check;")
	if err != nil {
		t.Fatalf("run PRAGMA foreign_key_check: %v", err)
	}
	defer fkRows.Close()

	violationCount := 0
	for fkRows.Next() {
		var tbl string
		var rowid int64
		var parent string
		var fkid int
		if err := fkRows.Scan(&tbl, &rowid, &parent, &fkid); err != nil {
			t.Fatalf("scan foreign_key_check: %v", err)
		}
		violationCount++
		t.Logf("VIOLATION #%d: table=%s, rowid=%d, references=%s, fkid=%d", violationCount, tbl, rowid, parent, fkid)
	}
	t.Logf("Total foreign key violations found in live database: %d", violationCount)
}
