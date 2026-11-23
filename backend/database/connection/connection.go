package connection

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

// HealthInfo reports verified SQLCipher runtime configuration for diagnostics.
type HealthInfo struct {
	CipherVersion  string `json:"cipherVersion"`
	JournalMode    string `json:"journalMode"`
	ForeignKeysOn  bool   `json:"foreignKeysOn"`
	IntegrityCheck string `json:"integrityCheck"`
	Encrypted      bool   `json:"encrypted"`
}

// Manager owns the SQLCipher-backed database/sql handle and connection lifecycle.
type Manager struct {
	mu     sync.RWMutex
	db     *sql.DB
	dbPath string
}

// OpenEncrypted opens (or creates) an encrypted SQLite database via SQLCipher,
// verifies cipher activation, enables foreign keys, and configures WAL mode.
func OpenEncrypted(dbPath, hexOrPassKey string) (*Manager, error) {
	if dbPath == "" {
		return nil, errors.New("database path is required")
	}
	if hexOrPassKey == "" {
		return nil, errors.New("database encryption key is required")
	}

	dsn := fmt.Sprintf(
		"%s?_pragma_key=%s&_pragma_cipher_page_size=4096&_busy_timeout=5000&_foreign_keys=on",
		dbPath,
		url.QueryEscape(hexOrPassKey),
	)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlcipher db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlcipher db: %w", err)
	}

	var cipherVer string
	if err := db.QueryRow("PRAGMA cipher_version;").Scan(&cipherVer); err != nil || cipherVer == "" {
		_ = db.Close()
		return nil, fmt.Errorf("sqlcipher not active (PRAGMA cipher_version failed): %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode = WAL;").Scan(&journalMode); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Optimize memory consumption & page cache bounds for desktop efficiency:
	// - synchronous = NORMAL: In WAL mode, completely safe against crashes and avoids fsync stalls on every commit
	// - journal_size_limit = 67108864: Caps WAL file growth to 64 MiB, reclaiming disk space on checkpoint
	// - cache_size = -16000: caps in-memory cache to 16 MiB per connection
	// - mmap_size = 67108864: enables up to 64 MiB memory-mapped I/O for high read throughput without memory bloat
	// - temp_store = MEMORY: caches temporary indexes and aggregates in RAM without disk churn
	optimizations := []string{
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA journal_size_limit = 67108864;",
		"PRAGMA cache_size = -16000;",
		"PRAGMA mmap_size = 67108864;",
		"PRAGMA temp_store = MEMORY;",
	}
	for _, pragma := range optimizations {
		if _, err := db.Exec(pragma); err != nil {
			// Non-fatal if specific pragma unsupported in driver build
			continue
		}
	}

	return &Manager{db: db, dbPath: dbPath}, nil
}

// DB returns the underlying *sql.DB handle.
func (m *Manager) DB() *sql.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db
}

// Health inspects cipher version, journal mode, foreign keys state, and quick integrity.
func (m *Manager) Health() (HealthInfo, error) {
	m.mu.RLock()
	db := m.db
	m.mu.RUnlock()
	if db == nil {
		return HealthInfo{}, errors.New("database connection is closed")
	}

	var info HealthInfo
	if err := db.QueryRow("PRAGMA cipher_version;").Scan(&info.CipherVersion); err != nil {
		return HealthInfo{}, fmt.Errorf("query cipher_version: %w", err)
	}
	info.Encrypted = info.CipherVersion != ""

	if err := db.QueryRow("PRAGMA journal_mode;").Scan(&info.JournalMode); err != nil {
		return HealthInfo{}, fmt.Errorf("query journal_mode: %w", err)
	}

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys;").Scan(&fk); err != nil {
		return HealthInfo{}, fmt.Errorf("query foreign_keys: %w", err)
	}
	info.ForeignKeysOn = fk == 1

	if err := db.QueryRow("PRAGMA integrity_check;").Scan(&info.IntegrityCheck); err != nil {
		return HealthInfo{}, fmt.Errorf("query integrity_check: %w", err)
	}

	return info, nil
}

// ShrinkMemory executes SQLite's PRAGMA shrink_memory to release cached pages back to the process heap
// and runs PRAGMA optimize to refresh query planner statistics.
func (m *Manager) ShrinkMemory() error {
	m.mu.RLock()
	db := m.db
	m.mu.RUnlock()
	if db == nil {
		return errors.New("database connection is closed")
	}
	_, _ = db.Exec("PRAGMA optimize;")
	_, err := db.Exec("PRAGMA shrink_memory;")
	return err
}

// CheckpointWAL flushes uncommitted WAL journal frames to main database storage and checkpoints passively,
// allowing SQLite to trim memory-mapped WAL index buffers.
func (m *Manager) CheckpointWAL() error {
	m.mu.RLock()
	db := m.db
	m.mu.RUnlock()
	if db == nil {
		return errors.New("database connection is closed")
	}
	_, err := db.Exec("PRAGMA wal_checkpoint(PASSIVE);")
	return err
}

// Close closes the database handle cleanly.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db == nil {
		return nil
	}
	err := m.db.Close()
	m.db = nil
	return err
}

// StandardSQLiteMagic is the canonical 16-byte header for unencrypted SQLite databases.
var StandardSQLiteMagic = []byte("SQLite format 3\x00")

// VerifyDiskHeaderEncrypted checks the raw 16-byte file header on disk.
// Standard unencrypted SQLite databases start with "SQLite format 3\x00".
// An encrypted SQLCipher database will have high-entropy random ciphertext in the first 16 bytes.
// Returns true if the file exists and is encrypted (does NOT match standard SQLite header).
func VerifyDiskHeaderEncrypted(dbPath string) (bool, error) {
	f, err := os.Open(dbPath)
	if err != nil {
		return false, fmt.Errorf("open db file for header check: %w", err)
	}
	defer f.Close()

	header := make([]byte, 16)
	n, err := f.Read(header)
	if err != nil {
		return false, fmt.Errorf("read db header: %w", err)
	}
	if n < 16 {
		return false, errors.New("file too short to be a valid SQLite/SQLCipher database")
	}

	if bytes.Equal(header, StandardSQLiteMagic) {
		return false, nil // Unencrypted plain SQLite!
	}
	return true, nil // Encrypted SQLCipher!
}

// VerifyForeignKeysActive checks PRAGMA foreign_keys directly on the active connection.
func VerifyForeignKeysActive(db *sql.DB) (bool, error) {
	if db == nil {
		return false, errors.New("nil database handle")
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys;").Scan(&fk); err != nil {
		return false, fmt.Errorf("query foreign_keys: %w", err)
	}
	return fk == 1, nil
}
