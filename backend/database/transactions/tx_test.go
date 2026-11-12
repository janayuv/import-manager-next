package transactions_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
)

func TestWithinTxCommitsAndRollsBack(t *testing.T) {
	mgr, err := connection.OpenEncrypted(filepath.Join(t.TempDir(), "tx.db"), "test-key-123")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer mgr.Close()

	if _, err := mgr.DB().Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	ctx := context.Background()
	if err := transactions.WithinTx(ctx, mgr.DB(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO kv(k, v) VALUES ('a', '1')`)
		return err
	}); err != nil {
		t.Fatalf("commit tx failed: %v", err)
	}

	errBoom := errors.New("boom")
	if err := transactions.WithinTx(ctx, mgr.DB(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO kv(k, v) VALUES ('b', '2')`); err != nil {
			return err
		}
		return errBoom
	}); !errors.Is(err, errBoom) {
		t.Fatalf("expected boom error, got %v", err)
	}

	var count int
	if err := mgr.DB().QueryRow(`SELECT COUNT(*) FROM kv`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected 1 row after rollback, got %d (err=%v)", count, err)
	}
}
