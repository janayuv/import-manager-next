package connection_test

import (
	"bytes"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
)

func TestOpenEncryptedEnforcesSQLCipherWALAndFK(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "encrypted.db")
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	mgr, err := connection.OpenEncrypted(dbPath, key)
	if err != nil {
		t.Fatalf("OpenEncrypted failed: %v", err)
	}

	health, err := mgr.Health()
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if !health.Encrypted || health.CipherVersion == "" {
		t.Fatalf("expected active SQLCipher encryption, got %+v", health)
	}
	if !health.ForeignKeysOn {
		t.Fatalf("expected foreign_keys=ON")
	}
	if health.JournalMode != "wal" {
		t.Fatalf("expected journal_mode=wal, got %q", health.JournalMode)
	}
	if health.IntegrityCheck != "ok" {
		t.Fatalf("expected integrity_check=ok, got %q", health.IntegrityCheck)
	}

	if _, err := mgr.DB().Exec(`CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT NOT NULL); INSERT INTO test_items (name) VALUES ('steel-coil');`); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read db file failed: %v", err)
	}
	if len(raw) < 16 || bytes.HasPrefix(raw, []byte("SQLite format 3\x00")) {
		t.Fatalf("database header is not encrypted!")
	}

	if _, err := connection.OpenEncrypted(dbPath, "wrong-key"); err == nil {
		t.Fatalf("expected OpenEncrypted with wrong key to fail")
	}
}

func TestDriverCanOpenPlaintextLegacyDatabase(t *testing.T) {
	plainPath := filepath.Join(t.TempDir(), "legacy_plain.db")
	dsn := plainPath + "?_foreign_keys=on"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open plaintext failed: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE suppliers (id TEXT PRIMARY KEY, name TEXT); INSERT INTO suppliers VALUES ('Sup-001', 'Korea Co');`); err != nil {
		t.Fatalf("seed plaintext failed: %v", err)
	}
	_ = db.Close()

	raw, _ := os.ReadFile(plainPath)
	if !bytes.HasPrefix(raw, []byte("SQLite format 3\x00")) {
		t.Fatalf("expected plaintext SQLite magic header when no pragma key passed")
	}

	dbRead, err := sql.Open("sqlite3", plainPath+"?mode=ro&_query_only=on&_key="+url.QueryEscape(""))
	if err != nil {
		t.Fatalf("reopen plaintext failed: %v", err)
	}
	defer dbRead.Close()
	var cnt int
	if err := dbRead.QueryRow(`SELECT COUNT(*) FROM suppliers`).Scan(&cnt); err != nil || cnt != 1 {
		t.Fatalf("expected 1 supplier from legacy plaintext DB, got cnt=%d err=%v", cnt, err)
	}
}

func TestShrinkMemoryAndCheckpointWAL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shrink_test.db")
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	mgr, err := connection.OpenEncrypted(dbPath, key)
	if err != nil {
		t.Fatalf("OpenEncrypted failed: %v", err)
	}
	defer mgr.Close()

	if _, err := mgr.DB().Exec(`CREATE TABLE cache_test (id INTEGER PRIMARY KEY, payload TEXT);`); err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	for i := 0; i < 50; i++ {
		if _, err := mgr.DB().Exec(`INSERT INTO cache_test (payload) VALUES (hex(randomblob(1024)));`); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	if err := mgr.CheckpointWAL(); err != nil {
		t.Fatalf("CheckpointWAL failed: %v", err)
	}
	if err := mgr.ShrinkMemory(); err != nil {
		t.Fatalf("ShrinkMemory failed: %v", err)
	}
}
