package integrity

import (
	"database/sql"
	"errors"
	"fmt"
)

// Report summarizes SQLite integrity, foreign-key consistency, and schema object counts.
type Report struct {
	IntegrityOK       bool     `json:"integrityOk"`
	IntegrityMessages []string `json:"integrityMessages"`
	ForeignKeyErrors  int      `json:"foreignKeyErrors"`
	TableCount        int      `json:"tableCount"`
	ViewCount         int      `json:"viewCount"`
	RecoveredWrites   int      `json:"recoveredWrites"`
}

// Verify runs PRAGMA integrity_check, PRAGMA foreign_key_check, and object count inspection.
func Verify(db *sql.DB) (Report, error) {
	if db == nil {
		return Report{}, errors.New("database handle is nil")
	}

	var rep Report

	rows, err := db.Query("PRAGMA integrity_check;")
	if err != nil {
		return rep, fmt.Errorf("run PRAGMA integrity_check: %w", err)
	}
	for rows.Next() {
		var msg string
		if scanErr := rows.Scan(&msg); scanErr != nil {
			_ = rows.Close()
			return rep, fmt.Errorf("scan integrity_check: %w", scanErr)
		}
		rep.IntegrityMessages = append(rep.IntegrityMessages, msg)
	}
	_ = rows.Close()
	rep.IntegrityOK = len(rep.IntegrityMessages) == 1 && rep.IntegrityMessages[0] == "ok"

	fkRows, err := db.Query("PRAGMA foreign_key_check;")
	if err != nil {
		return rep, fmt.Errorf("run PRAGMA foreign_key_check: %w", err)
	}
	for fkRows.Next() {
		var tbl string
		var rowid int64
		var parent string
		var fkid int
		if scanErr := fkRows.Scan(&tbl, &rowid, &parent, &fkid); scanErr == nil {
			rep.ForeignKeyErrors++
		}
	}
	_ = fkRows.Close()

	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&rep.TableCount); err != nil {
		return rep, fmt.Errorf("count tables: %w", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'view'`).Scan(&rep.ViewCount); err != nil {
		return rep, fmt.Errorf("count views: %w", err)
	}

	return rep, nil
}

// MarkInterruptedWritesFailed marks stale 'pending' write-recovery journal entries cleanly on startup.
func MarkInterruptedWritesFailed(db *sql.DB) (int, error) {
	if db == nil {
		return 0, errors.New("database handle is nil")
	}
	total := 0

	if hasTable(db, "write_recovery_journal") {
		res, err := db.Exec(`
			UPDATE write_recovery_journal
			SET status = 'INTERRUPTED',
			    error_message = COALESCE(error_message, 'Application restarted before write completion'),
			    finished_at = strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')
			WHERE UPPER(status) = 'PENDING'`)
		if err != nil {
			return total, fmt.Errorf("recover write_recovery_journal: %w", err)
		}
		if n, err := res.RowsAffected(); err == nil {
			total += int(n)
		}
	}

	if hasTable(db, "boe_write_recovery") {
		res, err := db.Exec(`
			UPDATE boe_write_recovery
			SET status = 'interrupted'
			WHERE LOWER(status) = 'pending'`)
		if err != nil {
			return total, fmt.Errorf("recover boe_write_recovery: %w", err)
		}
		if n, err := res.RowsAffected(); err == nil {
			total += int(n)
		}
	}

	return total, nil
}

func hasTable(db *sql.DB, name string) bool {
	var cnt int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&cnt)
	return err == nil && cnt > 0
}
