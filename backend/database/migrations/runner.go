package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed sql/*.sql
var embeddedMigrations embed.FS

// Migration represents a parsed versioned SQL migration script.
type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

// AppliedRecord mirrors a row stored in next_schema_migrations.
type AppliedRecord struct {
	Version    int    `json:"version"`
	Name       string `json:"name"`
	Checksum   string `json:"checksum"`
	AppVersion string `json:"appVersion"`
	ExecutedAt string `json:"executedAt"`
	DurationMs int64  `json:"durationMs"`
	Status     string `json:"status"`
}

// RunResult summarizes a migration pass at startup.
type RunResult struct {
	PreviousVersion int    `json:"previousVersion"`
	CurrentVersion  int    `json:"currentVersion"`
	AppliedCount    int    `json:"appliedCount"`
	PreBackupPath   string `json:"preBackupPath,omitempty"`
}

// PreMigrationBackupHook creates a consistent pre-upgrade backup before pending migrations execute.
type PreMigrationBackupHook func(fromVersion, toVersion int) (string, error)

const createTrackingTableSQL = `
CREATE TABLE IF NOT EXISTS next_schema_migrations (
	version         INTEGER PRIMARY KEY,
	name            TEXT NOT NULL,
	checksum_sha256 TEXT NOT NULL,
	app_version     TEXT NOT NULL,
	executed_at     TEXT NOT NULL,
	duration_ms     INTEGER NOT NULL,
	status          TEXT NOT NULL CHECK (status IN ('APPLIED', 'FAILED'))
);`

// LoadEmbedded reads and checksums all embedded SQL migration scripts ordered by version.
func LoadEmbedded() ([]Migration, error) {
	return loadFromFS(embeddedMigrations, "sql")
}

func loadFromFS(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory %s: %w", dir, err)
	}

	var out []Migration
	seenVersions := make(map[int]string)

	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(ent.Name(), "_", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid migration filename %q (expected NNNN_name.sql)", ent.Name())
		}
		ver, convErr := strconv.Atoi(parts[0])
		if convErr != nil || ver <= 0 {
			return nil, fmt.Errorf("invalid migration version prefix in %q", ent.Name())
		}
		if prev, dup := seenVersions[ver]; dup {
			return nil, fmt.Errorf("duplicate migration version %d (%s and %s)", ver, prev, ent.Name())
		}
		seenVersions[ver] = ent.Name()

		raw, readErr := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(dir, ent.Name())))
		if readErr != nil {
			return nil, fmt.Errorf("read migration %s: %w", ent.Name(), readErr)
		}

		sum := sha256.Sum256(raw)
		out = append(out, Migration{
			Version:  ver,
			Name:     ent.Name(),
			SQL:      string(raw),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Version < out[j].Version
	})
	return out, nil
}

// Run validates existing migration checksums/downgrade safety and applies pending migrations transactionally.
func Run(ctx context.Context, db *sql.DB, appVersion string, backupHook PreMigrationBackupHook) (RunResult, error) {
	migs, err := LoadEmbedded()
	if err != nil {
		return RunResult{}, err
	}
	return RunWithMigrations(ctx, db, appVersion, migs, backupHook)
}

// RunWithMigrations executes a provided ordered slice of migrations against db.
func RunWithMigrations(ctx context.Context, db *sql.DB, appVersion string, migs []Migration, backupHook PreMigrationBackupHook) (RunResult, error) {
	if db == nil {
		return RunResult{}, errors.New("database handle is nil")
	}
	if _, err := db.ExecContext(ctx, createTrackingTableSQL); err != nil {
		return RunResult{}, fmt.Errorf("ensure next_schema_migrations table: %w", err)
	}

	appliedRows, err := listAppliedRecords(ctx, db)
	if err != nil {
		return RunResult{}, err
	}

	appliedMap := make(map[int]AppliedRecord, len(appliedRows))
	maxAppliedVersion := 0
	for _, rec := range appliedRows {
		if rec.Status == "FAILED" {
			return RunResult{}, fmt.Errorf("prior migration v%d (%s) recorded status=FAILED; manual inspection or restore required", rec.Version, rec.Name)
		}
		appliedMap[rec.Version] = rec
		if rec.Version > maxAppliedVersion {
			maxAppliedVersion = rec.Version
		}
	}

	maxAvailableVersion := 0
	migByVer := make(map[int]Migration, len(migs))
	for _, m := range migs {
		migByVer[m.Version] = m
		if m.Version > maxAvailableVersion {
			maxAvailableVersion = m.Version
		}
	}

	// Downgrade protection: refuse opening a database migrated by a newer application version.
	if maxAppliedVersion > maxAvailableVersion {
		return RunResult{}, fmt.Errorf("database schema version v%d is newer than supported v%d (downgrade refused)", maxAppliedVersion, maxAvailableVersion)
	}

	// Verify checksums of every previously applied migration.
	var pending []Migration
	for _, m := range migs {
		if rec, ok := appliedMap[m.Version]; ok {
			if !matchesChecksum(m, rec.Checksum) {
				return RunResult{}, fmt.Errorf("migration v%d (%s) SHA-256 mismatch: applied=%s current=%s", m.Version, m.Name, rec.Checksum, m.Checksum)
			}
		} else {
			pending = append(pending, m)
		}
	}

	res := RunResult{
		PreviousVersion: maxAppliedVersion,
		CurrentVersion:  maxAppliedVersion,
	}

	if len(pending) == 0 {
		return res, nil
	}

	// Automatic pre-migration backup on upgrades to an existing schema.
	if maxAppliedVersion > 0 && backupHook != nil {
		targetVer := pending[len(pending)-1].Version
		backupPath, bErr := backupHook(maxAppliedVersion, targetVer)
		if bErr != nil {
			return res, fmt.Errorf("pre-migration safety backup failed: %w", bErr)
		}
		res.PreBackupPath = backupPath
	}

	conn, connErr := db.Conn(ctx)
	if connErr != nil {
		return res, fmt.Errorf("acquire migration connection: %w", connErr)
	}
	defer conn.Close()

	for _, m := range pending {
		start := time.Now()

		// SQLite foreign keys cannot be toggled inside a multi-statement transaction.
		// Disable foreign keys on this dedicated connection prior to BeginTx so table rebuilding works.
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF;"); err != nil {
			return res, fmt.Errorf("disable foreign keys for migration: %w", err)
		}

		tx, beginErr := conn.BeginTx(ctx, nil)
		if beginErr != nil {
			_, _ = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON;")
			return res, fmt.Errorf("begin migration transaction: %w", beginErr)
		}

		var durMs int64
		runErr := func() error {
			if _, execErr := tx.ExecContext(ctx, m.SQL); execErr != nil {
				return execErr
			}
			durMs = time.Since(start).Milliseconds()
			_, recErr := tx.ExecContext(ctx, `
				INSERT INTO next_schema_migrations (
					version, name, checksum_sha256, app_version, executed_at, duration_ms, status
				) VALUES (?, ?, ?, ?, ?, ?, 'APPLIED')`,
				m.Version,
				m.Name,
				m.Checksum,
				appVersion,
				time.Now().UTC().Format(time.RFC3339),
				durMs,
			)
			return recErr
		}()

		if runErr != nil {
			_ = tx.Rollback()
			durMs = time.Since(start).Milliseconds()
			// Record failed migration state explicitly outside the rolled-back tx.
			_, _ = conn.ExecContext(ctx, `
				INSERT OR REPLACE INTO next_schema_migrations (
					version, name, checksum_sha256, app_version, executed_at, duration_ms, status
				) VALUES (?, ?, ?, ?, ?, ?, 'FAILED')`,
				m.Version,
				m.Name,
				m.Checksum,
				appVersion,
				time.Now().UTC().Format(time.RFC3339),
				durMs,
			)
			_, _ = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON;")
			return res, fmt.Errorf("apply migration v%d (%s): %w", m.Version, m.Name, runErr)
		}

		if commitErr := tx.Commit(); commitErr != nil {
			_, _ = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON;")
			return res, fmt.Errorf("commit migration v%d (%s): %w", m.Version, m.Name, commitErr)
		}

		_, _ = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON;")

		// Verify foreign key integrity after re-enabling
		fkRows, fkErr := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
		if fkErr == nil {
			if fkRows.Next() {
				var table, parent string
				var rowid, fkid int
				_ = fkRows.Scan(&table, &rowid, &parent, &fkid)
				_ = fkRows.Close()
				return res, fmt.Errorf("migration v%d (%s) foreign key check failed: table %s rowid %d references %s", m.Version, m.Name, table, rowid, parent)
			}
			_ = fkRows.Close()
		}

		res.AppliedCount++
		res.CurrentVersion = m.Version
	}

	return res, nil
}

// ListApplied returns the full ordered history stored in next_schema_migrations.
func ListApplied(ctx context.Context, db *sql.DB) ([]AppliedRecord, error) {
	if db == nil {
		return nil, errors.New("database handle is nil")
	}
	if _, err := db.ExecContext(ctx, createTrackingTableSQL); err != nil {
		return nil, err
	}
	return listAppliedRecords(ctx, db)
}

func listAppliedRecords(ctx context.Context, db *sql.DB) ([]AppliedRecord, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT version, name, checksum_sha256, app_version, executed_at, duration_ms, status
		FROM next_schema_migrations
		ORDER BY version ASC`)
	if err != nil {
		return nil, fmt.Errorf("query next_schema_migrations: %w", err)
	}
	defer rows.Close()

	var out []AppliedRecord
	for rows.Next() {
		var r AppliedRecord
		if scanErr := rows.Scan(&r.Version, &r.Name, &r.Checksum, &r.AppVersion, &r.ExecutedAt, &r.DurationMs, &r.Status); scanErr != nil {
			return nil, fmt.Errorf("scan migration row: %w", scanErr)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// matchesChecksum checks if appliedChecksum matches m.Checksum directly,
// or if it matches after normalizing line endings (LF vs CRLF).
func matchesChecksum(m Migration, appliedChecksum string) bool {
	if m.Checksum == appliedChecksum {
		return true
	}
	// Normalize to LF
	lfSQL := strings.ReplaceAll(m.SQL, "\r\n", "\n")
	sumLF := sha256.Sum256([]byte(lfSQL))
	if hex.EncodeToString(sumLF[:]) == appliedChecksum {
		return true
	}
	// Normalize to CRLF
	crlfSQL := strings.ReplaceAll(lfSQL, "\n", "\r\n")
	sumCRLF := sha256.Sum256([]byte(crlfSQL))
	if hex.EncodeToString(sumCRLF[:]) == appliedChecksum {
		return true
	}
	return false
}
