package connection

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
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
