package transactions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
)

const sqlitePlainHeader = "SQLite format 3\x00"

// DBInspectSummary holds verified format, integrity, and schema details for a SQLite/SQLCipher file.
type DBInspectSummary struct {
	Path              string
	FileSizeBytes     int64
	IsPlaintextSQLite bool
	IsEncrypted       bool
	KeyValid          bool
	SchemaVersion     string
	IntegrityCheck    string
	ForeignKeyErrors  int
	TableColumns      map[string][]string
	TableCounts       map[string]int64
}

// CopyAttachedOptions controls table-level copy behavior from an attached SQLite database.
type CopyAttachedOptions struct {
	ReplaceExisting bool
	ExcludeTables   []string
	PriorityOrder   []string
}

// CopyAttachedResult summarizes rows copied and post-copy structural checks.
type CopyAttachedResult struct {
	RowsCopiedByTable map[string]int64
	IntegrityCheck    string
	ForeignKeyErrors  int
	Warnings          []string
}

// QuoteIdent safely wraps a SQLite identifier in double quotes.
func QuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func escapeSQLString(val string) string {
	return "'" + strings.ReplaceAll(val, "'", "''") + "'"
}

// DetectSQLitePlaintext inspects the first 16 bytes of a file for the standard SQLite header.
func DetectSQLitePlaintext(path string) (bool, int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, 0, fmt.Errorf("stat database file %s: %w", path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return false, info.Size(), fmt.Errorf("open database file %s: %w", path, err)
	}
	defer f.Close()

	buf := make([]byte, len(sqlitePlainHeader))
	n, err := f.Read(buf)
	if err != nil {
		return false, info.Size(), fmt.Errorf("read header %s: %w", path, err)
	}
	return n == len(sqlitePlainHeader) && string(buf) == sqlitePlainHeader, info.Size(), nil
}

// OpenSQLiteOrSQLCipherReadOnly opens an external SQLite/SQLCipher file for inspection without mutating it.
func OpenSQLiteOrSQLCipherReadOnly(path, key string) (*sql.DB, bool, error) {
	isPlain, _, err := DetectSQLitePlaintext(path)
	if err != nil {
		return nil, false, err
	}

	var dsn string
	if isPlain {
		dsn = fmt.Sprintf("%s?mode=ro&_foreign_keys=on", path)
	} else {
		if strings.TrimSpace(key) == "" {
			return nil, false, errors.New("database is encrypted (SQLCipher); encryption key or passphrase is required")
		}
		dsn = fmt.Sprintf(
			"%s?mode=ro&_pragma_key=%s&_pragma_cipher_page_size=4096&_foreign_keys=on",
			path,
			url.QueryEscape(key),
		)
	}

	extDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, isPlain, fmt.Errorf("open external db: %w", err)
	}
	extDB.SetMaxOpenConns(1)

	var count int
	if err := extDB.QueryRow(`SELECT count(*) FROM sqlite_master`).Scan(&count); err != nil {
		_ = extDB.Close()
		return nil, isPlain, fmt.Errorf("unlock/read external database failed (check key or file integrity): %w", err)
	}
	return extDB, isPlain, nil
}

// InspectExternalDB opens an external SQLite/SQLCipher database read-only and collects schema & count metrics.
func InspectExternalDB(path, key string) (DBInspectSummary, error) {
	isPlain, size, err := DetectSQLitePlaintext(path)
	if err != nil {
		return DBInspectSummary{}, err
	}

	out := DBInspectSummary{
		Path:              path,
		FileSizeBytes:     size,
		IsPlaintextSQLite: isPlain,
		IsEncrypted:       !isPlain,
		TableColumns:      make(map[string][]string),
		TableCounts:       make(map[string]int64),
	}

	extDB, _, err := OpenSQLiteOrSQLCipherReadOnly(path, key)
	if err != nil {
		return out, nil // returns KeyValid=false with format detection populated
	}
	defer extDB.Close()
	out.KeyValid = true

	var integrity string
	if err := extDB.QueryRow(`PRAGMA integrity_check;`).Scan(&integrity); err == nil {
		out.IntegrityCheck = integrity
	}

	fkRows, err := extDB.Query(`PRAGMA foreign_key_check;`)
	if err == nil {
		for fkRows.Next() {
			out.ForeignKeyErrors++
		}
		_ = fkRows.Close()
	}

	// Query schema migration version if vnext or legacy/refinery table exists.
	for _, query := range []string{
		`SELECT COALESCE(CAST(MAX(version) AS TEXT), '') FROM next_schema_migrations WHERE status = 'APPLIED'`,
		`SELECT COALESCE(CAST(MAX(version) AS TEXT), '') FROM schema_migrations WHERE status IN ('APPLIED','SUCCESS')`,
		`SELECT COALESCE(CAST(MAX(version) AS TEXT), '') FROM refinery_schema_history`,
		`SELECT COALESCE(CAST(MAX(version) AS TEXT), '') FROM schema_migrations`,
	} {
		var ver string
		if err := extDB.QueryRow(query).Scan(&ver); err == nil && strings.TrimSpace(ver) != "" {
			out.SchemaVersion = ver
			break
		}
	}

	tblRows, err := extDB.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return out, fmt.Errorf("list external tables: %w", err)
	}
	var tables []string
	for tblRows.Next() {
		var tbl string
		if err := tblRows.Scan(&tbl); err == nil {
			tables = append(tables, tbl)
		}
	}
	_ = tblRows.Close()

	for _, tbl := range tables {
		cols, err := listColumns(extDB, "main", tbl)
		if err != nil {
			continue
		}
		out.TableColumns[tbl] = cols
		var cnt int64
		if err := extDB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, QuoteIdent(tbl))).Scan(&cnt); err == nil {
			out.TableCounts[tbl] = cnt
		}
	}

	return out, nil
}

// CopyAttachedTables copies intersecting table columns from an external database file into main DB
// on a single pinned SQLite connection, verifying integrity and foreign keys post-commit.
func CopyAttachedTables(ctx context.Context, db *sql.DB, attachedPath, attachedKey string, isPlain bool, opts CopyAttachedOptions) (CopyAttachedResult, error) {
	result := CopyAttachedResult{
		RowsCopiedByTable: make(map[string]int64),
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return result, fmt.Errorf("pin sqlite connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF;`); err != nil {
		return result, fmt.Errorf("disable foreign keys for copy: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `PRAGMA foreign_keys = ON;`)
	}()

	const alias = "import_src"
	keyClause := ""
	if isPlain {
		keyClause = ` KEY ''`
	} else if attachedKey != "" {
		keyClause = fmt.Sprintf(` KEY %s`, escapeSQLString(attachedKey))
	}

	attachSQL := fmt.Sprintf(`ATTACH DATABASE %s AS %s%s;`, escapeSQLString(attachedPath), QuoteIdent(alias), keyClause)
	if _, err := conn.ExecContext(ctx, attachSQL); err != nil {
		return result, fmt.Errorf("attach source database: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), fmt.Sprintf(`DETACH DATABASE %s;`, QuoteIdent(alias)))
	}()

	srcTables, err := listTablesConn(ctx, conn, alias)
	if err != nil {
		return result, err
	}
	dstTables, err := listTablesConn(ctx, conn, "main")
	if err != nil {
		return result, err
	}

	dstSet := make(map[string]struct{}, len(dstTables))
	for _, t := range dstTables {
		dstSet[t] = struct{}{}
	}
	srcSet := make(map[string]struct{}, len(srcTables))
	for _, t := range srcTables {
		srcSet[t] = struct{}{}
	}

	orderedTables := make([]string, 0, len(dstTables))
	seen := make(map[string]struct{})
	for _, p := range opts.PriorityOrder {
		if _, ok := dstSet[p]; ok {
			if _, okSrc := srcSet[p]; okSrc && !slices.Contains(opts.ExcludeTables, p) {
				orderedTables = append(orderedTables, p)
				seen[p] = struct{}{}
			}
		}
	}
	for _, t := range srcTables {
		if _, ok := seen[t]; ok {
			continue
		}
		if _, okDst := dstSet[t]; okDst && !slices.Contains(opts.ExcludeTables, t) {
			orderedTables = append(orderedTables, t)
			seen[t] = struct{}{}
		}
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin attached copy tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	for _, tbl := range orderedTables {
		srcCols, err := listColumnsTx(ctx, tx, alias, tbl)
		if err != nil {
			return result, err
		}
		dstCols, err := listColumnsTx(ctx, tx, "main", tbl)
		if err != nil {
			return result, err
		}
		commonCols := intersectStrings(dstCols, srcCols)
		if len(commonCols) == 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("table %s had no matching columns", tbl))
			continue
		}

		quotedCols := make([]string, 0, len(commonCols))
		for _, c := range commonCols {
			quotedCols = append(quotedCols, QuoteIdent(c))
		}
		colList := strings.Join(quotedCols, ", ")

		if opts.ReplaceExisting {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM main.%s;`, QuoteIdent(tbl))); err != nil {
				return result, fmt.Errorf("clear destination table %s: %w", tbl, err)
			}
		}

		insertVerb := "INSERT OR REPLACE"
		if tbl == "roles" || tbl == "role_permissions" {
			insertVerb = "INSERT OR IGNORE"
		}
		insertSQL := fmt.Sprintf(
			`%s INTO main.%s (%s) SELECT %s FROM %s.%s;`,
			insertVerb, QuoteIdent(tbl), colList, colList, QuoteIdent(alias), QuoteIdent(tbl),
		)
		res, err := tx.ExecContext(ctx, insertSQL)
		if err != nil {
			return result, fmt.Errorf("copy table %s: %w", tbl, err)
		}
		affected, _ := res.RowsAffected()
		result.RowsCopiedByTable[tbl] = affected
	}

	// Preserve sqlite_sequence autoincrement high-water marks when present in both DBs.
	var seqExists int
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.sqlite_master WHERE type='table' AND name='sqlite_sequence'`, QuoteIdent(alias))).Scan(&seqExists); err == nil && seqExists > 0 {
		var dstSeqExists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name='sqlite_sequence'`).Scan(&dstSeqExists); err == nil && dstSeqExists > 0 {
			if opts.ReplaceExisting {
				_, _ = tx.ExecContext(ctx, `DELETE FROM main.sqlite_sequence;`)
			}
			seqSQL := fmt.Sprintf(
				`INSERT OR REPLACE INTO main.sqlite_sequence (name, seq) SELECT name, seq FROM %s.sqlite_sequence;`,
				QuoteIdent(alias),
			)
			_, _ = tx.ExecContext(ctx, seqSQL)
		}
	}

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit attached copy tx: %w", err)
	}
	tx = nil

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return result, fmt.Errorf("re-enable foreign keys: %w", err)
	}

	fkRows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_check;`)
	if err != nil {
		return result, fmt.Errorf("run foreign_key_check: %w", err)
	}
	for fkRows.Next() {
		result.ForeignKeyErrors++
	}
	_ = fkRows.Close()

	if err := conn.QueryRowContext(ctx, `PRAGMA integrity_check;`).Scan(&result.IntegrityCheck); err != nil {
		return result, fmt.Errorf("run integrity_check: %w", err)
	}

	return result, nil
}

func listTablesConn(ctx context.Context, conn *sql.Conn, schema string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(
		`SELECT name FROM %s.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%%' ORDER BY name`,
		QuoteIdent(schema),
	))
	if err != nil {
		return nil, fmt.Errorf("list tables in %s: %w", schema, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func listColumns(db *sql.DB, schema, table string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA %s.table_info(%s);`, QuoteIdent(schema), QuoteIdent(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

func listColumnsTx(ctx context.Context, tx *sql.Tx, schema, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`PRAGMA %s.table_info(%s);`, QuoteIdent(schema), QuoteIdent(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

func intersectStrings(dst, src []string) []string {
	srcSet := make(map[string]struct{}, len(src))
	for _, s := range src {
		srcSet[s] = struct{}{}
	}
	out := make([]string, 0, len(dst))
	for _, d := range dst {
		if _, ok := srcSet[d]; ok {
			out = append(out, d)
		}
	}
	return out
}
