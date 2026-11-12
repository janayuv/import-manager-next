package database_management

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	appcrypto "github.com/janayuv/import-manager-next/backend/crypto"
	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/backup"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const (
	bulkChunkSize       = 500
	bulkUndoWindow      = 5 * time.Minute
	highImpactThreshold = 5000
	hardDeletePinKey    = "db_hard_delete_pin_hash"
)

type browseTableSpec struct {
	PrimaryKey string
	HasSoftDel bool
	Editable   bool
}

var browseAllowlist = map[string]browseTableSpec{
	"suppliers":           {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"items":               {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"shipments":           {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"invoices":            {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"invoice_line_items":  {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"boe_details":         {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"boe_calculations":    {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"service_providers":   {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"expense_types":       {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"expense_invoices":    {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"expenses":            {PrimaryKey: "id", HasSoftDel: true, Editable: true},
	"expense_attachments": {PrimaryKey: "id", HasSoftDel: false, Editable: false},
	"users":               {PrimaryKey: "id", HasSoftDel: false, Editable: false},
	"roles":               {PrimaryKey: "id", HasSoftDel: false, Editable: false},
	"backups":             {PrimaryKey: "id", HasSoftDel: false, Editable: false},
	"backup_schedules":    {PrimaryKey: "id", HasSoftDel: false, Editable: true},
	"app_settings":        {PrimaryKey: "key", HasSoftDel: false, Editable: true},
	"audit_log":           {PrimaryKey: "id", HasSoftDel: false, Editable: false},
}

var coreEntityOrder = []string{
	"suppliers", "items", "shipments", "invoices", "invoice_line_items",
	"boe_details", "boe_calculations", "service_providers", "expense_types",
	"expense_invoices", "expenses",
}

var protectedEditColumns = []string{
	"id", "key", "password_hash", "deleted_at", "deleted_by", "created_at",
}

type childDependencySpec struct {
	Table string
	Col   string
}

var tableChildren = map[string][]childDependencySpec{
	"suppliers":         {{Table: "shipments", Col: "supplier_id"}, {Table: "items", Col: "supplier_id"}},
	"items":             {{Table: "invoice_line_items", Col: "item_id"}},
	"shipments":         {{Table: "invoices", Col: "shipment_id"}, {Table: "boe_calculations", Col: "shipment_id"}, {Table: "expense_invoices", Col: "shipment_id"}, {Table: "expenses", Col: "shipment_id"}},
	"invoices":          {{Table: "invoice_line_items", Col: "invoice_id"}},
	"boe_details":       {{Table: "boe_calculations", Col: "boe_id"}},
	"service_providers": {{Table: "expense_invoices", Col: "service_provider_id"}, {Table: "expenses", Col: "service_provider_id"}},
	"expense_types":     {{Table: "expenses", Col: "expense_type_id"}},
	"expense_invoices":  {{Table: "expenses", Col: "expense_invoice_id"}, {Table: "expense_attachments", Col: "expense_invoice_id"}},
	"expenses":          {{Table: "expense_attachments", Col: "expense_id"}},
}

type undoBatch struct {
	TableName string
	RecordIDs []string
	ExpiresAt time.Time
	ActorID   string
}

// Service provides live database administration, table browsing/editing, chunked bulk delete with 5-minute undo,
// PIN-verified hard delete, maintenance tools, and non-destructive legacy DB migration.
type Service struct {
	db          *sql.DB
	dbMgr       *connection.Manager
	security    *security.Service
	audit       *audit.Service
	restoreCtrl *connection.RestoreControl
	backupSvc   *backup.Service
	dbPath      string
	dbKey       string
	tempDir     string

	mu          sync.Mutex
	undoBatches map[string]undoBatch
}

// NewService constructs a Database Management Service.
func NewService(
	db *sql.DB,
	dbMgr *connection.Manager,
	sec *security.Service,
	aud *audit.Service,
	restoreCtrl *connection.RestoreControl,
	backupSvc *backup.Service,
	dbPath string,
	dbKey string,
	tempDir string,
) *Service {
	return &Service{
		db:          db,
		dbMgr:       dbMgr,
		security:    sec,
		audit:       aud,
		restoreCtrl: restoreCtrl,
		backupSvc:   backupSvc,
		dbPath:      dbPath,
		dbKey:       dbKey,
		tempDir:     tempDir,
		undoBatches: make(map[string]undoBatch),
	}
}

// ListBrowseableTables returns the allowlisted table names exposed in the Database Console.
func (s *Service) ListBrowseableTables() ([]string, error) {
	if _, err := s.security.RequirePermission(security.PermAdminSystemTools); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(browseAllowlist))
	for k := range browseAllowlist {
		names = append(names, k)
	}
	slices.Sort(names)
	return names, nil
}

// GetOverviewStats returns live SQLCipher, WAL, size, schema version, and per-table active/deleted counts.
func (s *Service) GetOverviewStats() (models.DatabaseOverviewStats, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAdminSystemTools, security.PermBackupCreate, security.PermAdminSystemHealth); err != nil {
		return models.DatabaseOverviewStats{}, err
	}

	health, err := s.dbMgr.Health()
	if err != nil {
		return models.DatabaseOverviewStats{}, err
	}

	var pageCount, pageSize int64
	_ = s.db.QueryRow(`PRAGMA page_count;`).Scan(&pageCount)
	_ = s.db.QueryRow(`PRAGMA page_size;`).Scan(&pageSize)

	var fileSize int64
	if info, err := os.Stat(s.dbPath); err == nil {
		fileSize = info.Size()
	}

	var schemaVersion int
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM next_schema_migrations WHERE status = 'APPLIED'`).Scan(&schemaVersion)

	tableCounts := make(map[string]int64, len(browseAllowlist))
	deletedCounts := make(map[string]int64, len(coreEntityOrder))

	for tbl, spec := range browseAllowlist {
		var c int64
		if spec.HasSoftDel {
			_ = s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL`, transactions.QuoteIdent(tbl))).Scan(&c)
			var delC int64
			_ = s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NOT NULL`, transactions.QuoteIdent(tbl))).Scan(&delC)
			deletedCounts[tbl] = delC
		} else {
			_ = s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, transactions.QuoteIdent(tbl))).Scan(&c)
		}
		tableCounts[tbl] = c
	}

	return models.DatabaseOverviewStats{
		DatabasePath:   s.dbPath,
		CipherVersion:  health.CipherVersion,
		JournalMode:    health.JournalMode,
		ForeignKeysOn:  health.ForeignKeysOn,
		IntegrityCheck: health.IntegrityCheck,
		PageCount:      pageCount,
		PageSize:       pageSize,
		FileSizeBytes:  fileSize,
		SchemaVersion:  schemaVersion,
		RestoreLocked:  s.restoreCtrl.IsRestoring(),
		TableCounts:    tableCounts,
		DeletedCounts:  deletedCounts,
	}, nil
}

// BrowseTable reads paginated rows from a validated allowlisted table.
func (s *Service) BrowseTable(req models.BrowseTableRequest) (models.BrowseTableResponse, error) {
	ctx := context.Background()
	if _, err := s.security.RequirePermission(security.PermAdminSystemTools); err != nil {
		return models.BrowseTableResponse{}, err
	}
	spec, ok := browseAllowlist[strings.TrimSpace(req.TableName)]
	if !ok {
		return models.BrowseTableResponse{}, fmt.Errorf("table %q is not allowlisted for browsing", req.TableName)
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 25
	}

	cols, err := s.listTableColumns(ctx, req.TableName)
	if err != nil {
		return models.BrowseTableResponse{}, err
	}
	if len(cols) == 0 {
		return models.BrowseTableResponse{}, fmt.Errorf("table %q has no inspectable columns", req.TableName)
	}

	whereParts := make([]string, 0, 2)
	args := make([]any, 0, len(cols)+2)
	if spec.HasSoftDel && !req.IncludeDeleted {
		whereParts = append(whereParts, `deleted_at IS NULL`)
	}
	search := strings.TrimSpace(req.Search)
	if search != "" {
		likeParts := make([]string, 0, len(cols))
		likeArg := "%" + search + "%"
		for _, c := range cols {
			likeParts = append(likeParts, fmt.Sprintf(`CAST(%s AS TEXT) LIKE ?`, transactions.QuoteIdent(c)))
			args = append(args, likeArg)
		}
		whereParts = append(whereParts, "("+strings.Join(likeParts, " OR ")+")")
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = " WHERE " + strings.Join(whereParts, " AND ")
	}

	qTbl := transactions.QuoteIdent(req.TableName)
	var total int64
	if err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, qTbl, whereClause), args...).Scan(&total); err != nil {
		return models.BrowseTableResponse{}, fmt.Errorf("count rows in %s: %w", req.TableName, err)
	}

	selectCols := make([]string, 0, len(cols))
	for _, c := range cols {
		selectCols = append(selectCols, transactions.QuoteIdent(c))
	}
	offset := (req.Page - 1) * req.PageSize
	querySQL := fmt.Sprintf(`SELECT %s FROM %s%s ORDER BY %s DESC LIMIT ? OFFSET ?`,
		strings.Join(selectCols, ", "), qTbl, whereClause, transactions.QuoteIdent(spec.PrimaryKey))
	queryArgs := append(append([]any{}, args...), req.PageSize, offset)

	rows, err := s.db.Query(querySQL, queryArgs...)
	if err != nil {
		return models.BrowseTableResponse{}, fmt.Errorf("query rows from %s: %w", req.TableName, err)
	}
	defer rows.Close()

	resultRows := make([]map[string]any, 0)
	for rows.Next() {
		dest := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cols {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return models.BrowseTableResponse{}, err
		}
		rowMap := make(map[string]any, len(cols))
		for i, colName := range cols {
			switch v := dest[i].(type) {
			case []byte:
				rowMap[colName] = string(v)
			default:
				rowMap[colName] = v
			}
		}
		resultRows = append(resultRows, rowMap)
	}
	if err := rows.Err(); err != nil {
		return models.BrowseTableResponse{}, err
	}

	return models.BrowseTableResponse{
		TableName:  req.TableName,
		PrimaryKey: spec.PrimaryKey,
		Editable:   spec.Editable,
		Columns:    cols,
		Page:       req.Page,
		PageSize:   req.PageSize,
		Total:      total,
		Rows:       resultRows,
	}, nil
}

// UpdateTableCell updates a single safe cell in an editable allowlisted table with audit logging.
func (s *Service) UpdateTableCell(req models.UpdateTableCellRequest) error {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermAdminSystemTools)
	if err != nil {
		return err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return err
	}
	spec, ok := browseAllowlist[strings.TrimSpace(req.TableName)]
	if !ok || !spec.Editable {
		return fmt.Errorf("table %q does not allow inline console edits", req.TableName)
	}
	col := strings.TrimSpace(req.Column)
	if slices.Contains(protectedEditColumns, strings.ToLower(col)) {
		return fmt.Errorf("column %q is protected and cannot be inline edited", col)
	}
	cols, err := s.listTableColumns(ctx, req.TableName)
	if err != nil {
		return err
	}
	if !slices.Contains(cols, col) {
		return fmt.Errorf("column %q does not exist in %s", col, req.TableName)
	}

	var beforeVal sql.NullString
	_ = s.db.QueryRow(
		fmt.Sprintf(`SELECT CAST(%s AS TEXT) FROM %s WHERE CAST(%s AS TEXT) = ?`,
			transactions.QuoteIdent(col), transactions.QuoteIdent(req.TableName), transactions.QuoteIdent(spec.PrimaryKey)),
		req.RowID,
	).Scan(&beforeVal)

	var newVal any = req.Value
	if req.IsNull {
		newVal = nil
	}
	res, err := s.db.Exec(
		fmt.Sprintf(`UPDATE %s SET %s = ? WHERE CAST(%s AS TEXT) = ?`,
			transactions.QuoteIdent(req.TableName), transactions.QuoteIdent(col), transactions.QuoteIdent(spec.PrimaryKey)),
		newVal, req.RowID,
	)
	if err != nil {
		return fmt.Errorf("update cell %s.%s: %w", req.TableName, col, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("row %q not found in %s", req.RowID, req.TableName)
	}

	_ = s.audit.RecordTableAudit(req.TableName, req.RowID, "CELL_UPDATE", sess.UserID,
		map[string]any{"column": col, "value": beforeVal.String},
		map[string]any{"column": col, "value": newVal},
		nil,
	)
	return nil
}

// SetHardDeletePin sets or rotates the dedicated Argon2id hard-delete PIN after verifying current admin password.
func (s *Service) SetHardDeletePin(currentPassword, newPin string) error {
	sess, err := s.security.RequirePermission(security.PermAdminSystemTools)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(newPin)) < 4 {
		return errors.New("hard-delete PIN must be at least 4 characters")
	}

	var pwHash string
	_ = s.db.QueryRow(`SELECT value FROM app_settings WHERE key = 'auth.admin_password_hash'`).Scan(&pwHash)
	if strings.TrimSpace(pwHash) == "" {
		_ = s.db.QueryRow(`SELECT COALESCE(password_hash, '') FROM users WHERE id = ?`, sess.UserID).Scan(&pwHash)
	}
	ok, _, err := appcrypto.VerifyPassword(currentPassword, pwHash)
	if err != nil || !ok {
		return errors.New("current password verification failed")
	}

	pinHash, err := appcrypto.HashPassword(newPin)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		hardDeletePinKey, pinHash,
	); err != nil {
		return fmt.Errorf("save hard delete PIN: %w", err)
	}
	_ = s.audit.RecordTableAudit("app_settings", hardDeletePinKey, "UPDATE", sess.UserID, nil, map[string]string{"pinConfigured": "true"}, nil)
	return nil
}

func (s *Service) verifyHardDeletePin(ctx context.Context, userID, suppliedPin string) error {
	if strings.TrimSpace(suppliedPin) == "" {
		return errors.New("security PIN is required for permanent hard delete")
	}
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, hardDeletePinKey).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) || strings.TrimSpace(hash) == "" {
		_ = s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'auth.admin_password_hash'`).Scan(&hash)
		if strings.TrimSpace(hash) == "" {
			_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(password_hash, '') FROM users WHERE id = ?`, userID).Scan(&hash)
		}
	}
	if strings.TrimSpace(hash) == "" {
		return errors.New("security PIN verification unavailable")
	}
	ok, _, verifyErr := appcrypto.VerifyPassword(suppliedPin, hash)
	if verifyErr != nil || !ok {
		return errors.New("invalid security PIN")
	}
	return nil
}

// BulkDelete deletes selected rows in batches of 500:
// - Soft delete returns a 5-minute UndoToken.
// - Hard delete requires PIN verification and 'DELETE' confirmation when total referencing rows > 5000.
func (s *Service) BulkDelete(req models.BulkDeleteRequest) (models.BulkDeleteResult, error) {
	ctx := context.Background()
	perm := security.PermDataDelete
	if req.HardDelete {
		perm = security.PermAdminSystemTools
	}
	sess, err := s.security.RequirePermission(perm)
	if err != nil {
		return models.BulkDeleteResult{}, err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return models.BulkDeleteResult{}, err
	}

	spec, ok := browseAllowlist[strings.TrimSpace(req.TableName)]
	if !ok || !spec.HasSoftDel {
		return models.BulkDeleteResult{}, fmt.Errorf("table %q does not support bulk delete", req.TableName)
	}

	ids := dedupeIDs(req.RecordIDs)
	if len(ids) == 0 {
		return models.BulkDeleteResult{}, errors.New("at least one recordId is required")
	}

	deps, totalDeps, err := s.computeDependencies(ctx, req.TableName, ids)
	if err != nil {
		return models.BulkDeleteResult{}, err
	}

	mode := "soft"
	if req.HardDelete {
		mode = "hard"
		if err := s.verifyHardDeletePin(ctx, sess.UserID, req.Pin); err != nil {
			return models.BulkDeleteResult{}, err
		}
		if totalDeps > highImpactThreshold && strings.TrimSpace(req.HighImpactConfirm) != "DELETE" {
			return models.BulkDeleteResult{}, fmt.Errorf(
				"high-impact hard delete affects %d dependent rows (>%d); type DELETE in confirmation to proceed",
				totalDeps, highImpactThreshold,
			)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	processed := 0
	chunks := 0
	touchedIDs := make([]string, 0, len(ids))

	for i := 0; i < len(ids); i += bulkChunkSize {
		end := min(i+bulkChunkSize, len(ids))
		batch := ids[i:end]
		chunks++

		err := transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
			for _, id := range batch {
				var res sql.Result
				if req.HardDelete {
					res, err = tx.ExecContext(ctx,
						fmt.Sprintf(`DELETE FROM %s WHERE CAST(id AS TEXT) = ?`, transactions.QuoteIdent(req.TableName)),
						id,
					)
				} else {
					res, err = tx.ExecContext(ctx,
						fmt.Sprintf(`UPDATE %s SET deleted_at = ?, deleted_by = ? WHERE CAST(id AS TEXT) = ? AND deleted_at IS NULL`,
							transactions.QuoteIdent(req.TableName)),
						now, sess.UserID, id,
					)
				}
				if err != nil {
					return err
				}
				n, _ := res.RowsAffected()
				if n > 0 {
					processed += int(n)
					touchedIDs = append(touchedIDs, id)
				}
			}
			return nil
		})
		if err != nil {
			return models.BulkDeleteResult{}, fmt.Errorf("bulk %s delete chunk %d failed: %w", mode, chunks, err)
		}
	}

	out := models.BulkDeleteResult{
		TableName:         req.TableName,
		Mode:              mode,
		RequestedCount:    len(ids),
		ProcessedCount:    processed,
		ChunkCount:        chunks,
		TotalDependencies: totalDeps,
		Dependencies:      deps,
	}

	if !req.HardDelete && len(touchedIDs) > 0 {
		tokBytes := make([]byte, 12)
		_, _ = rand.Read(tokBytes)
		undoTok := hex.EncodeToString(tokBytes)
		exp := time.Now().UTC().Add(bulkUndoWindow)

		s.mu.Lock()
		s.undoBatches[undoTok] = undoBatch{
			TableName: req.TableName,
			RecordIDs: touchedIDs,
			ExpiresAt: exp,
			ActorID:   sess.UserID,
		}
		s.mu.Unlock()

		out.UndoToken = undoTok
		out.UndoExpiresAt = exp.Format(time.RFC3339)
	}

	_ = s.audit.RecordTableAudit(req.TableName, fmt.Sprintf("bulk_%d", len(ids)), "BULK_"+strings.ToUpper(mode)+"_DELETE", sess.UserID, nil, out, nil)
	return out, nil
}

// UndoBulkSoftDelete reverses a bulk soft delete batch within its 5-minute undo window.
func (s *Service) UndoBulkSoftDelete(undoToken string) (int, error) {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermDataDelete)
	if err != nil {
		return 0, err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	batch, ok := s.undoBatches[undoToken]
	if ok {
		delete(s.undoBatches, undoToken)
	}
	s.mu.Unlock()

	if !ok {
		return 0, errors.New("undo token not found or already used")
	}
	if time.Now().UTC().After(batch.ExpiresAt) {
		return 0, errors.New("5-minute undo window has expired; recover records from Recycle Bin")
	}

	restored := 0
	err = transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		q := fmt.Sprintf(`UPDATE %s SET deleted_at = NULL, deleted_by = NULL WHERE CAST(id AS TEXT) = ? AND deleted_at IS NOT NULL`,
			transactions.QuoteIdent(batch.TableName))
		for _, id := range batch.RecordIDs {
			res, execErr := tx.ExecContext(ctx, q, id)
			if execErr != nil {
				return execErr
			}
			n, _ := res.RowsAffected()
			restored += int(n)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("undo bulk soft delete: %w", err)
	}

	_ = s.audit.RecordTableAudit(batch.TableName, undoToken, "BULK_UNDO", sess.UserID, nil, map[string]int{"restored": restored}, nil)
	return restored, nil
}

// RunIntegrityAndVacuum runs integrity_check, foreign_key_check, WAL checkpoint, and VACUUM.
func (s *Service) RunIntegrityAndVacuum() (string, error) {
	sess, err := s.security.RequirePermission(security.PermAdminSystemTools)
	if err != nil {
		return "", err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return "", err
	}

	var integrity string
	if err := s.db.QueryRow(`PRAGMA integrity_check;`).Scan(&integrity); err != nil {
		return "", err
	}
	fkRows, err := s.db.Query(`PRAGMA foreign_key_check;`)
	if err != nil {
		return "", err
	}
	fkErrors := 0
	for fkRows.Next() {
		fkErrors++
	}
	_ = fkRows.Close()

	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`)
	if _, err := s.db.Exec(`VACUUM;`); err != nil {
		return "", fmt.Errorf("vacuum failed: %w", err)
	}

	msg := fmt.Sprintf("integrity_check=%s, foreign_key_errors=%d, VACUUM completed", integrity, fkErrors)
	_ = s.audit.RecordTableAudit("database", "maintenance", "VACUUM", sess.UserID, nil, map[string]string{"result": msg}, nil)
	return msg, nil
}

func (s *Service) computeDependencies(ctx context.Context, tableName string, ids []string) ([]models.DependencyCount, int64, error) {
	specs := tableChildren[tableName]
	if len(specs) == 0 || len(ids) == 0 {
		return nil, 0, nil
	}
	var out []models.DependencyCount
	var grandTotal int64

	counts := make(map[string]int64, len(specs))
	for i := 0; i < len(ids); i += bulkChunkSize {
		end := min(i+bulkChunkSize, len(ids))
		batch := ids[i:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		args := make([]any, 0, len(batch))
		for _, id := range batch {
			args = append(args, id)
		}
		for _, sp := range specs {
			var c int64
			q := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE CAST(%s AS TEXT) IN (%s)`,
				transactions.QuoteIdent(sp.Table), transactions.QuoteIdent(sp.Col), placeholders)
			if err := s.db.QueryRowContext(ctx, q, args...).Scan(&c); err != nil {
				return nil, 0, err
			}
			counts[sp.Table+"."+sp.Col] += c
			grandTotal += c
		}
	}

	for _, sp := range specs {
		c := counts[sp.Table+"."+sp.Col]
		if c > 0 {
			out = append(out, models.DependencyCount{
				ChildTable:  sp.Table,
				ForeignKey:  sp.Col,
				ActiveCount: c,
				TotalCount:  c,
			})
		}
	}
	return out, grandTotal, nil
}

func (s *Service) listTableColumns(ctx context.Context, table string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s);`, transactions.QuoteIdent(table)))
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

func dedupeIDs(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
