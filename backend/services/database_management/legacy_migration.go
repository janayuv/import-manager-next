package database_management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

var legacyPriorityOrder = []string{
	"roles",
	"role_permissions",
	"users",
	"app_settings",
	"backup_schedules",
	"suppliers",
	"items",
	"service_providers",
	"expense_types",
	"shipments",
	"invoices",
	"invoice_line_items",
	"boe_details",
	"boe_calculations",
	"expense_invoices",
	"expenses",
	"expense_attachments",
	"workflow_exceptions",
	"workflow_incidents",
	"audit_log",
}

// InspectLegacyDatabase reads a legacy Import Manager .db file non-destructively (mode=ro)
// to detect plaintext vs SQLCipher encryption, validate the supplied key, inspect schema & rows,
// and compute baseline financial totals.
func (s *Service) InspectLegacyDatabase(req models.LegacyInspectRequest) (models.LegacyInspectResult, error) {
	ctx := context.Background()
	if _, err := s.security.RequireAnyPermission(security.PermAdminSystemTools, security.PermBackupRestore); err != nil {
		return models.LegacyInspectResult{}, err
	}
	if strings.TrimSpace(req.SourcePath) == "" {
		return models.LegacyInspectResult{}, errors.New("legacy database sourcePath is required")
	}

	extSummary, err := transactions.InspectExternalDB(req.SourcePath, req.SourceKey)
	if err != nil {
		return models.LegacyInspectResult{}, err
	}

	result := models.LegacyInspectResult{
		SourcePath:        req.SourcePath,
		FileSizeBytes:     extSummary.FileSizeBytes,
		IsPlaintextSQLite: extSummary.IsPlaintextSQLite,
		IsEncrypted:       extSummary.IsEncrypted,
		KeyValid:          extSummary.KeyValid,
		MigrationVersion:  extSummary.SchemaVersion,
	}

	if !extSummary.KeyValid {
		if extSummary.IsEncrypted {
			result.Warnings = append(result.Warnings, "Database appears SQLCipher encrypted; provide a valid legacy database key to inspect tables.")
		} else {
			result.Warnings = append(result.Warnings, "Database could not be opened cleanly; check file permissions or SQLite corruption.")
		}
		return result, nil
	}

	inspected := make([]models.LegacyTableInspect, 0, len(legacyPriorityOrder))
	for _, tbl := range legacyPriorityOrder {
		dstCols, _ := s.listTableColumns(ctx, tbl)
		srcCols, srcExists := extSummary.TableColumns[tbl]
		matched, missing := diffColumns(dstCols, srcCols)

		var targetCount int64
		_ = s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, transactions.QuoteIdent(tbl))).Scan(&targetCount)

		inspected = append(inspected, models.LegacyTableInspect{
			TableName:    tbl,
			SourceExists: srcExists,
			SourceRows:   extSummary.TableCounts[tbl],
			TargetRows:   targetCount,
			MatchedCols:  matched,
			MissingInSrc: missing,
		})
	}
	result.Tables = inspected

	extDB, _, err := transactions.OpenSQLiteOrSQLCipherReadOnly(req.SourcePath, req.SourceKey)
	if err == nil {
		defer extDB.Close()
		result.SourceFinancials = computeFinancialSummary(ctx, extDB, extSummary.TableColumns)
	}

	return result, nil
}

// RunLegacyMigration executes a dry-run parity test on an isolated staging DB or performs the
// live migration under global restore_control lock after creating a mandatory safety backup.
// The original legacy database file is NEVER overwritten.
func (s *Service) RunLegacyMigration(req models.LegacyMigrationRequest) (models.LegacyMigrationReport, error) {
	ctx := context.Background()
	sess, err := s.security.RequireAnyPermission(security.PermAdminSystemTools, security.PermBackupRestore)
	if err != nil {
		return models.LegacyMigrationReport{}, err
	}
	if strings.TrimSpace(req.SourcePath) == "" {
		return models.LegacyMigrationReport{}, errors.New("legacy database sourcePath is required")
	}

	extSummary, err := transactions.InspectExternalDB(req.SourcePath, req.SourceKey)
	if err != nil {
		return models.LegacyMigrationReport{}, err
	}
	if !extSummary.KeyValid {
		return models.LegacyMigrationReport{}, errors.New("cannot unlock legacy database; verify encryption key or file integrity")
	}

	extDB, isPlain, err := transactions.OpenSQLiteOrSQLCipherReadOnly(req.SourcePath, req.SourceKey)
	if err != nil {
		return models.LegacyMigrationReport{}, err
	}
	srcFin := computeFinancialSummary(ctx, extDB, extSummary.TableColumns)
	_ = extDB.Close()

	if strings.ToLower(extSummary.IntegrityCheck) != "ok" {
		return models.LegacyMigrationReport{}, fmt.Errorf("legacy source database integrity_check failed: %s", extSummary.IntegrityCheck)
	}

	var targetDB *sql.DB
	var safetyPath string

	if req.DryRun {
		if err := os.MkdirAll(s.tempDir, 0o700); err != nil {
			return models.LegacyMigrationReport{}, err
		}
		stagePath := filepath.Join(s.tempDir, fmt.Sprintf("legacy_dryrun_%d.db", time.Now().UnixNano()))
		defer os.Remove(stagePath)

		_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`)
		escaped := strings.ReplaceAll(stagePath, "'", "''")
		if _, err := s.db.Exec(fmt.Sprintf(`VACUUM INTO '%s';`, escaped)); err != nil {
			return models.LegacyMigrationReport{}, fmt.Errorf("prepare dry-run staging copy: %w", err)
		}

		stagePlain, _, err := transactions.DetectSQLitePlaintext(stagePath)
		if err != nil {
			return models.LegacyMigrationReport{}, err
		}
		if stagePlain {
			targetDB, err = sql.Open("sqlite3", stagePath+"?_foreign_keys=on")
		} else {
			mgr, openErr := connection.OpenEncrypted(stagePath, s.dbKey)
			if openErr != nil {
				return models.LegacyMigrationReport{}, fmt.Errorf("open encrypted dry-run stage: %w", openErr)
			}
			defer mgr.Close()
			targetDB = mgr.DB()
		}
		if err != nil {
			return models.LegacyMigrationReport{}, err
		}
		if stagePlain {
			targetDB.SetMaxOpenConns(1)
			defer targetDB.Close()
		}
	} else {
		if err := s.restoreCtrl.RequireIdle(); err != nil {
			return models.LegacyMigrationReport{}, err
		}
		s.restoreCtrl.SetRestoring(true, "Migrating legacy database "+filepath.Base(req.SourcePath))
		defer s.restoreCtrl.SetRestoring(false, "")

		if s.backupSvc != nil {
			safety, err := s.backupSvc.CreateSafetySnapshot(ctx, sess.UserID, "", "Pre-migration safety backup before importing "+filepath.Base(req.SourcePath))
			if err != nil {
				return models.LegacyMigrationReport{}, fmt.Errorf("create pre-migration safety backup: %w", err)
			}
			safetyPath = safety.Path
		}
		targetDB = s.db
	}

	copyRes, err := transactions.CopyAttachedTables(ctx, targetDB, req.SourcePath, req.SourceKey, isPlain, transactions.CopyAttachedOptions{
		ReplaceExisting: false,
		ExcludeTables:   []string{"next_schema_migrations", "schema_migrations", "refinery_schema_history", "migration_history", "backups"},
		PriorityOrder:   legacyPriorityOrder,
	})
	if err != nil {
		return models.LegacyMigrationReport{}, fmt.Errorf("copy legacy tables: %w", err)
	}

	targetCols := collectTableColumnsMap(ctx, targetDB, legacyPriorityOrder)
	dstFin := computeFinancialSummary(ctx, targetDB, targetCols)

	rowParityOK := true
	warnings := append([]string{}, copyRes.Warnings...)
	for _, coreTbl := range coreEntityOrder {
		srcCount := extSummary.TableCounts[coreTbl]
		if srcCount > 0 {
			var dstCount int64
			_ = targetDB.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, transactions.QuoteIdent(coreTbl))).Scan(&dstCount)
			if dstCount < srcCount {
				rowParityOK = false
				warnings = append(warnings, fmt.Sprintf("table %s: target rows (%d) < source rows (%d)", coreTbl, dstCount, srcCount))
			}
		}
	}

	financialParityOK := approxEqual(srcFin.InvoiceValueSum, dstFin.InvoiceValueSum) &&
		dstFin.ExpenseAmountPaise >= srcFin.ExpenseAmountPaise &&
		dstFin.ExpenseTotalPaise >= srcFin.ExpenseTotalPaise

	report := models.LegacyMigrationReport{
		DryRun:            req.DryRun,
		SourcePath:        req.SourcePath,
		SafetyBackupPath:  safetyPath,
		IntegrityCheck:    copyRes.IntegrityCheck,
		ForeignKeysOK:     copyRes.ForeignKeyErrors == 0,
		ForeignKeyErrors:  copyRes.ForeignKeyErrors,
		RowsCopiedByTable: copyRes.RowsCopiedByTable,
		RowParityOK:       rowParityOK,
		SourceFinancials:  srcFin,
		TargetFinancials:  dstFin,
		FinancialParityOK: financialParityOK,
		Warnings:          warnings,
		CompletedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	if !req.DryRun {
		_ = s.audit.RecordTableAudit("legacy_migration", filepath.Base(req.SourcePath), "MIGRATE", sess.UserID, nil, report, nil)
	}
	return report, nil
}

func computeFinancialSummary(ctx context.Context, db *sql.DB, tableCols map[string][]string) models.LegacyFinancialSummary {
	var out models.LegacyFinancialSummary
	if cols := tableCols["shipments"]; len(cols) > 0 && hasCol(cols, "invoice_value") {
		_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(invoice_value), 0) FROM shipments`).Scan(&out.InvoiceValueSum)
	}
	if cols := tableCols["boe_details"]; len(cols) > 0 && hasCol(cols, "duty_amount") {
		var dutyRupees float64
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(duty_amount), 0) FROM boe_details`).Scan(&dutyRupees); err == nil {
			out.BOETotalDutyPaise = int64(math.Round(dutyRupees * 100))
		}
	}
	if cols := tableCols["expenses"]; len(cols) > 0 {
		if hasCol(cols, "amount_paise") {
			_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_paise), 0) FROM expenses`).Scan(&out.ExpenseAmountPaise)
		} else if hasCol(cols, "amount") {
			var amt float64
			if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM expenses`).Scan(&amt); err == nil {
				out.ExpenseAmountPaise = int64(math.Round(amt * 100))
			}
		}

		if hasCol(cols, "total_amount_paise") {
			_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_amount_paise), 0) FROM expenses`).Scan(&out.ExpenseTotalPaise)
		} else if hasCol(cols, "total_amount") {
			var tot float64
			if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_amount), 0) FROM expenses`).Scan(&tot); err == nil {
				out.ExpenseTotalPaise = int64(math.Round(tot * 100))
			}
		} else if hasCol(cols, "amount") {
			var tot float64
			if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount + COALESCE(cgst_amount,0) + COALESCE(sgst_amount,0) + COALESCE(igst_amount,0)), 0) FROM expenses`).Scan(&tot); err == nil {
				out.ExpenseTotalPaise = int64(math.Round(tot * 100))
			}
		}
	}
	return out
}

func collectTableColumnsMap(ctx context.Context, db *sql.DB, tables []string) map[string][]string {
	out := make(map[string][]string, len(tables))
	for _, tbl := range tables {
		rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s);`, transactions.QuoteIdent(tbl)))
		if err != nil {
			continue
		}
		var cols []string
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
				cols = append(cols, name)
			}
		}
		_ = rows.Close()
		out[tbl] = cols
	}
	return out
}

func diffColumns(dst, src []string) (matched []string, missing []string) {
	srcSet := make(map[string]struct{}, len(src))
	for _, s := range src {
		srcSet[s] = struct{}{}
	}
	for _, d := range dst {
		if _, ok := srcSet[d]; ok {
			matched = append(matched, d)
		} else {
			missing = append(missing, d)
		}
	}
	return matched, missing
}

func hasCol(cols []string, name string) bool {
	for _, c := range cols {
		if strings.EqualFold(c, name) {
			return true
		}
	}
	return false
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) <= 0.01 || b >= a-0.01
}
