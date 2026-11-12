package recycle_bin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const DefaultPageSize = 50

type fkParentSpec struct {
	ParentTable string
	FKColumn    string
}

type fkChildSpec struct {
	ChildTable string
	FKColumn   string
}

type tableMeta struct {
	Name      string
	LabelExpr string
	Summary   string
	Parents   []fkParentSpec
	Children  []fkChildSpec
}

var canonicalTables = []string{
	"expenses",
	"expense_invoices",
	"boe_calculations",
	"boe_details",
	"invoice_line_items",
	"invoices",
	"shipments",
	"items",
	"suppliers",
	"service_providers",
	"expense_types",
}

var tableRegistry = map[string]tableMeta{
	"suppliers": {
		Name:      "suppliers",
		LabelExpr: `COALESCE(supplier_name, '') || COALESCE(' (' || short_name || ')', '')`,
		Summary:   `COALESCE(country, '') || ' · ' || COALESCE(email, '')`,
		Children: []fkChildSpec{
			{ChildTable: "shipments", FKColumn: "supplier_id"},
			{ChildTable: "items", FKColumn: "supplier_id"},
		},
	},
	"items": {
		Name:      "items",
		LabelExpr: `COALESCE(part_number, '') || ' - ' || COALESCE(item_description, '')`,
		Summary:   `'HSN ' || COALESCE(hsn_code, '') || ' · ' || COALESCE(currency, '')`,
		Parents: []fkParentSpec{
			{ParentTable: "suppliers", FKColumn: "supplier_id"},
		},
		Children: []fkChildSpec{
			{ChildTable: "invoice_line_items", FKColumn: "item_id"},
		},
	},
	"shipments": {
		Name:      "shipments",
		LabelExpr: `COALESCE(invoice_number, id) || ' (' || COALESCE(shipment_mode, 'N/A') || ')'`,
		Summary:   `COALESCE(invoice_date, '') || ' · ' || COALESCE(invoice_currency, '')`,
		Parents: []fkParentSpec{
			{ParentTable: "suppliers", FKColumn: "supplier_id"},
		},
		Children: []fkChildSpec{
			{ChildTable: "invoices", FKColumn: "shipment_id"},
			{ChildTable: "boe_calculations", FKColumn: "shipment_id"},
			{ChildTable: "expense_invoices", FKColumn: "shipment_id"},
			{ChildTable: "expenses", FKColumn: "shipment_id"},
		},
	},
	"invoices": {
		Name:      "invoices",
		LabelExpr: `id || ' (' || COALESCE(status, '') || ')'`,
		Summary:   `'Shipment ' || COALESCE(shipment_id, '')`,
		Parents: []fkParentSpec{
			{ParentTable: "shipments", FKColumn: "shipment_id"},
		},
		Children: []fkChildSpec{
			{ChildTable: "invoice_line_items", FKColumn: "invoice_id"},
		},
	},
	"invoice_line_items": {
		Name:      "invoice_line_items",
		LabelExpr: `id`,
		Summary:   `'Invoice ' || COALESCE(invoice_id, '') || ' · Item ' || COALESCE(item_id, '')`,
		Parents: []fkParentSpec{
			{ParentTable: "invoices", FKColumn: "invoice_id"},
			{ParentTable: "items", FKColumn: "item_id"},
		},
	},
	"boe_details": {
		Name:      "boe_details",
		LabelExpr: `COALESCE(be_number, id) || ' (' || COALESCE(be_date, '') || ')'`,
		Summary:   `COALESCE(location, '') || ' · Duty ' || printf('%.2f', COALESCE(duty_amount, 0))`,
		Children: []fkChildSpec{
			{ChildTable: "boe_calculations", FKColumn: "boe_id"},
		},
	},
	"boe_calculations": {
		Name:      "boe_calculations",
		LabelExpr: `COALESCE(invoice_number, id) || ' (' || COALESCE(status, '') || ')'`,
		Summary:   `COALESCE(supplier_name, '') || ' · Shipment ' || COALESCE(shipment_id, '')`,
		Parents: []fkParentSpec{
			{ParentTable: "shipments", FKColumn: "shipment_id"},
			{ParentTable: "boe_details", FKColumn: "boe_id"},
		},
	},
	"service_providers": {
		Name:      "service_providers",
		LabelExpr: `COALESCE(name, id)`,
		Summary:   `COALESCE(gstin, 'No GSTIN') || ' · ' || COALESCE(state, '')`,
		Children: []fkChildSpec{
			{ChildTable: "expense_invoices", FKColumn: "service_provider_id"},
			{ChildTable: "expenses", FKColumn: "service_provider_id"},
		},
	},
	"expense_types": {
		Name:      "expense_types",
		LabelExpr: `COALESCE(name, id)`,
		Summary:   `'Default IGST ' || COALESCE(CAST(default_igst_rate AS TEXT), '0') || '%'`,
		Children: []fkChildSpec{
			{ChildTable: "expenses", FKColumn: "expense_type_id"},
		},
	},
	"expense_invoices": {
		Name:      "expense_invoices",
		LabelExpr: `COALESCE(invoice_no, id) || ' (' || COALESCE(invoice_date, '') || ')'`,
		Summary:   `'Shipment ' || COALESCE(shipment_id, '') || ' · Provider ' || COALESCE(service_provider_id, '')`,
		Parents: []fkParentSpec{
			{ParentTable: "shipments", FKColumn: "shipment_id"},
			{ParentTable: "service_providers", FKColumn: "service_provider_id"},
		},
		Children: []fkChildSpec{
			{ChildTable: "expenses", FKColumn: "expense_invoice_id"},
			{ChildTable: "expense_attachments", FKColumn: "expense_invoice_id"},
		},
	},
	"expenses": {
		Name:      "expenses",
		LabelExpr: `COALESCE(invoice_no, id) || ' · ' || COALESCE(remarks, expense_type_id)`,
		Summary:   `'Amount ₹' || printf('%.2f', COALESCE(amount, 0)) || ' · Date ' || COALESCE(invoice_date, '')`,
		Parents: []fkParentSpec{
			{ParentTable: "expense_invoices", FKColumn: "expense_invoice_id"},
			{ParentTable: "shipments", FKColumn: "shipment_id"},
			{ParentTable: "expense_types", FKColumn: "expense_type_id"},
			{ParentTable: "service_providers", FKColumn: "service_provider_id"},
		},
		Children: []fkChildSpec{
			{ChildTable: "expense_attachments", FKColumn: "expense_id"},
		},
	},
}

// Service manages centralized soft-delete browsing, parent-validated restore, dependency-gated purge, and retention cleanup.
type Service struct {
	db          *sql.DB
	security    *security.Service
	audit       *audit.Service
	restoreCtrl *connection.RestoreControl
}

// NewService constructs a Recycle Bin Service instance.
func NewService(db *sql.DB, sec *security.Service, aud *audit.Service, restoreCtrl *connection.RestoreControl) *Service {
	return &Service{
		db:          db,
		security:    sec,
		audit:       aud,
		restoreCtrl: restoreCtrl,
	}
}

func validateTable(name string) (tableMeta, error) {
	key := strings.TrimSpace(name)
	meta, ok := tableRegistry[key]
	if !ok {
		return tableMeta{}, fmt.Errorf("invalid recycle bin table %q (must be one of the 11 core allowlisted tables)", name)
	}
	return meta, nil
}

// GetSoftDeleteTables returns the fixed 11 allowlisted tables tracked in the Recycle Bin.
func (s *Service) GetSoftDeleteTables() ([]string, error) {
	if _, err := s.security.RequirePermission(security.PermRecycleBinView); err != nil {
		return nil, err
	}
	out := make([]string, len(canonicalTables))
	copy(out, canonicalTables)
	return out, nil
}

// GetRecycleBinDeletedCount returns the exact aggregate soft-deleted row count across all 11 tables.
func (s *Service) GetRecycleBinDeletedCount() (int64, error) {
	if _, err := s.security.RequirePermission(security.PermRecycleBinView); err != nil {
		return 0, err
	}
	var total int64
	for _, tbl := range canonicalTables {
		var c int64
		q := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NOT NULL`, transactions.QuoteIdent(tbl))
		if err := s.db.QueryRow(q).Scan(&c); err != nil {
			return 0, fmt.Errorf("count deleted rows in %s: %w", tbl, err)
		}
		total += c
	}
	return total, nil
}

// GetDeletedRecords lists soft-deleted rows (PAGE_SIZE=50 by default) with per-table counts and reconciliation checks.
func (s *Service) GetDeletedRecords(tableName, search string, page, pageSize int) (models.RecycleBinListResponse, error) {
	if _, err := s.security.RequirePermission(security.PermRecycleBinView); err != nil {
		return models.RecycleBinListResponse{}, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	perTable := make(map[string]int64, len(canonicalTables))
	var sumDeleted int64
	for _, tbl := range canonicalTables {
		var c int64
		if err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NOT NULL`, transactions.QuoteIdent(tbl))).Scan(&c); err != nil {
			return models.RecycleBinListResponse{}, err
		}
		perTable[tbl] = c
		sumDeleted += c
	}

	search = strings.TrimSpace(search)
	var targetTables []string
	if strings.TrimSpace(tableName) == "" || strings.EqualFold(tableName, "all") {
		targetTables = canonicalTables
	} else {
		if _, err := validateTable(tableName); err != nil {
			return models.RecycleBinListResponse{}, err
		}
		targetTables = []string{tableName}
	}

	unionParts := make([]string, 0, len(targetTables))
	args := make([]any, 0, len(targetTables)*3)
	for _, tbl := range targetTables {
		meta := tableRegistry[tbl]
		qTbl := transactions.QuoteIdent(tbl)
		part := fmt.Sprintf(
			`SELECT '%s' AS tbl, CAST(id AS TEXT) AS rec_id, (%s) AS label, (%s) AS summary, COALESCE(deleted_at, '') AS deleted_at, COALESCE(deleted_by, '') AS deleted_by
			 FROM %s
			 WHERE deleted_at IS NOT NULL`,
			tbl, meta.LabelExpr, meta.Summary, qTbl,
		)
		if search != "" {
			like := "%" + search + "%"
			part += fmt.Sprintf(` AND (CAST(id AS TEXT) LIKE ? OR (%s) LIKE ? OR (%s) LIKE ?)`, meta.LabelExpr, meta.Summary)
			args = append(args, like, like, like)
		}
		unionParts = append(unionParts, part)
	}

	fullSubquery := strings.Join(unionParts, " UNION ALL ")

	var filteredTotal int64
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM (%s)`, fullSubquery)
	if err := s.db.QueryRow(countSQL, args...).Scan(&filteredTotal); err != nil {
		return models.RecycleBinListResponse{}, fmt.Errorf("count filtered deleted records: %w", err)
	}

	offset := (page - 1) * pageSize
	pageSQL := fmt.Sprintf(`SELECT tbl, rec_id, label, summary, deleted_at, deleted_by FROM (%s) ORDER BY deleted_at DESC, tbl ASC LIMIT ? OFFSET ?`, fullSubquery)
	pageArgs := append(append([]any{}, args...), pageSize, offset)

	rows, err := s.db.Query(pageSQL, pageArgs...)
	if err != nil {
		return models.RecycleBinListResponse{}, fmt.Errorf("query deleted records: %w", err)
	}
	defer rows.Close()

	records := make([]models.RecycleBinRecord, 0)
	for rows.Next() {
		var r models.RecycleBinRecord
		if err := rows.Scan(&r.TableName, &r.RecordID, &r.Label, &r.Summary, &r.DeletedAt, &r.DeletedBy); err != nil {
			return models.RecycleBinListResponse{}, err
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return models.RecycleBinListResponse{}, err
	}

	return models.RecycleBinListResponse{
		TableName:        tableName,
		Search:           search,
		Page:             page,
		PageSize:         pageSize,
		Total:            filteredTotal,
		AggregateDeleted: sumDeleted,
		TableSumDeleted:  sumDeleted,
		CountMismatch:    false,
		PerTableCounts:   perTable,
		Rows:             records,
	}, nil
}

// RestoreDeletedRecords restores records in an allowlisted table after verifying every parent exists and is active.
func (s *Service) RestoreDeletedRecords(tableName string, recordIDs []string) (models.RecycleBinActionResult, error) {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermRecycleBinRestore)
	if err != nil {
		return models.RecycleBinActionResult{}, err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return models.RecycleBinActionResult{}, err
	}
	meta, err := validateTable(tableName)
	if err != nil {
		return models.RecycleBinActionResult{}, err
	}
	cleanIDs := cleanNonEmpty(recordIDs)
	if len(cleanIDs) == 0 {
		return models.RecycleBinActionResult{}, errors.New("at least one recordId is required")
	}

	missingMap := make(map[string]map[string]struct{})
	for _, id := range cleanIDs {
		for _, p := range meta.Parents {
			q := fmt.Sprintf(`SELECT COALESCE(CAST(%s AS TEXT), '') FROM %s WHERE CAST(id AS TEXT) = ?`,
				transactions.QuoteIdent(p.FKColumn), transactions.QuoteIdent(meta.Name))
			var parentID string
			if err := s.db.QueryRow(q, id).Scan(&parentID); err != nil || strings.TrimSpace(parentID) == "" {
				continue
			}
			var activeCount int
			pQ := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE CAST(id AS TEXT) = ? AND deleted_at IS NULL`,
				transactions.QuoteIdent(p.ParentTable))
			if err := s.db.QueryRow(pQ, parentID).Scan(&activeCount); err != nil || activeCount == 0 {
				if missingMap[p.ParentTable] == nil {
					missingMap[p.ParentTable] = make(map[string]struct{})
				}
				missingMap[p.ParentTable][parentID] = struct{}{}
			}
		}
	}

	if len(missingMap) > 0 {
		groups := make([]models.MissingParentGroup, 0, len(missingMap))
		for pTbl, idSet := range missingMap {
			ids := make([]string, 0, len(idSet))
			for pid := range idSet {
				ids = append(ids, pid)
			}
			slices.Sort(ids)
			groups = append(groups, models.MissingParentGroup{TableName: pTbl, ParentIDs: ids})
		}
		slices.SortFunc(groups, func(a, b models.MissingParentGroup) int {
			return strings.Compare(a.TableName, b.TableName)
		})
		return models.RecycleBinActionResult{
			Action:         "restore",
			TableName:      meta.Name,
			Blocked:        true,
			ErrorCode:      "missing_parent",
			Message:        "Cannot restore record(s) because referenced parent records are missing or deleted.",
			MissingParents: groups,
		}, nil
	}

	var restored int
	err = transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		q := fmt.Sprintf(`UPDATE %s SET deleted_at = NULL, deleted_by = NULL WHERE CAST(id AS TEXT) = ? AND deleted_at IS NOT NULL`, transactions.QuoteIdent(meta.Name))
		for _, id := range cleanIDs {
			res, err := tx.ExecContext(ctx, q, id)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			restored += int(n)
		}
		return nil
	})
	if err != nil {
		return models.RecycleBinActionResult{}, fmt.Errorf("restore deleted records: %w", err)
	}

	_ = s.audit.RecordTableAudit(meta.Name, strings.Join(cleanIDs, ","), "RESTORE", sess.UserID, nil, map[string]any{"restored": restored}, nil)
	return models.RecycleBinActionResult{
		Action:         "restore",
		TableName:      meta.Name,
		ProcessedCount: restored,
		SkippedCount:   len(cleanIDs) - restored,
		Message:        fmt.Sprintf("Restored %d record(s) from %s.", restored, meta.Name),
	}, nil
}

// PermanentlyDeleteRecords checks FK dependencies across child tables and permanently deletes safe records.
func (s *Service) PermanentlyDeleteRecords(tableName string, recordIDs []string) (models.RecycleBinActionResult, error) {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermRecycleBinPurge)
	if err != nil {
		return models.RecycleBinActionResult{}, err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return models.RecycleBinActionResult{}, err
	}
	meta, err := validateTable(tableName)
	if err != nil {
		return models.RecycleBinActionResult{}, err
	}
	cleanIDs := cleanNonEmpty(recordIDs)
	if len(cleanIDs) == 0 {
		return models.RecycleBinActionResult{}, errors.New("at least one recordId is required")
	}

	deps, err := s.scanDependencies(ctx, meta, cleanIDs)
	if err != nil {
		return models.RecycleBinActionResult{}, err
	}
	if len(deps) > 0 {
		parts := make([]string, 0, len(deps))
		for _, d := range deps {
			parts = append(parts, fmt.Sprintf("%s.%s (%d)", d.ChildTable, d.ForeignKey, d.TotalCount))
		}
		return models.RecycleBinActionResult{
			Action:       "purge",
			TableName:    meta.Name,
			Blocked:      true,
			ErrorCode:    "DEPENDENCY",
			Message:      "DEPENDENCY: cannot permanently delete because referencing rows exist in " + strings.Join(parts, ", "),
			Dependencies: deps,
		}, nil
	}

	var purged int
	err = transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		q := fmt.Sprintf(`DELETE FROM %s WHERE CAST(id AS TEXT) = ? AND deleted_at IS NOT NULL`, transactions.QuoteIdent(meta.Name))
		for _, id := range cleanIDs {
			res, err := tx.ExecContext(ctx, q, id)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			purged += int(n)
		}
		return nil
	})
	if err != nil {
		return models.RecycleBinActionResult{}, fmt.Errorf("permanently delete records: %w", err)
	}

	_ = s.audit.RecordTableAudit(meta.Name, strings.Join(cleanIDs, ","), "PURGE", sess.UserID, nil, map[string]any{"purged": purged}, nil)
	return models.RecycleBinActionResult{
		Action:         "purge",
		TableName:      meta.Name,
		ProcessedCount: purged,
		SkippedCount:   len(cleanIDs) - purged,
		Message:        fmt.Sprintf("Permanently deleted %d record(s) from %s.", purged, meta.Name),
	}, nil
}

// PurgeExpiredRecords purges soft-deleted rows older than retentionDays (reading app_settings when <=0).
func (s *Service) PurgeExpiredRecords(retentionDays int) (models.RecycleBinActionResult, error) {
	ctx := context.Background()
	sess, err := s.security.RequirePermission(security.PermRecycleBinPurge)
	if err != nil {
		return models.RecycleBinActionResult{}, err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return models.RecycleBinActionResult{}, err
	}
	if retentionDays <= 0 {
		retentionDays = 30
		var raw string
		if err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = 'recycle_retention_days'`).Scan(&raw); err == nil {
			if parsed, pErr := strconv.Atoi(strings.TrimSpace(raw)); pErr == nil && parsed > 0 {
				retentionDays = parsed
			}
		}
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	totalPurged := 0

	for _, tbl := range canonicalTables {
		meta := tableRegistry[tbl]
		rows, err := s.db.Query(
			fmt.Sprintf(`SELECT CAST(id AS TEXT) FROM %s WHERE deleted_at IS NOT NULL AND deleted_at < ?`, transactions.QuoteIdent(tbl)),
			cutoff,
		)
		if err != nil {
			return models.RecycleBinActionResult{}, err
		}
		var candidates []string
		for rows.Next() {
			var id string
			if scanErr := rows.Scan(&id); scanErr == nil {
				candidates = append(candidates, id)
			}
		}
		_ = rows.Close()

		for _, id := range candidates {
			deps, err := s.scanDependencies(ctx, meta, []string{id})
			if err != nil || len(deps) > 0 {
				continue
			}
			res, err := s.db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE CAST(id AS TEXT) = ? AND deleted_at IS NOT NULL`, transactions.QuoteIdent(tbl)), id)
			if err == nil {
				n, _ := res.RowsAffected()
				totalPurged += int(n)
			}
		}
	}

	_ = s.audit.RecordTableAudit("recycle_bin", "retention", "PURGE_EXPIRED", sess.UserID, nil, map[string]any{"retentionDays": retentionDays, "purged": totalPurged}, nil)
	return models.RecycleBinActionResult{
		Action:         "purge_expired",
		ProcessedCount: totalPurged,
		Message:        fmt.Sprintf("Purged %d expired soft-deleted record(s) older than %d days.", totalPurged, retentionDays),
	}, nil
}

func (s *Service) scanDependencies(ctx context.Context, meta tableMeta, ids []string) ([]models.DependencyCount, error) {
	if len(meta.Children) == 0 || len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	var out []models.DependencyCount
	for _, child := range meta.Children {
		var hasDeleted bool
		_ = s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info(%s) WHERE name = 'deleted_at'`, transactions.QuoteIdent(child.ChildTable))).Scan(&hasDeleted)

		var totalCnt, activeCnt int64
		totalSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE CAST(%s AS TEXT) IN (%s)`,
			transactions.QuoteIdent(child.ChildTable), transactions.QuoteIdent(child.FKColumn), placeholders)
		if err := s.db.QueryRowContext(ctx, totalSQL, args...).Scan(&totalCnt); err != nil {
			return nil, fmt.Errorf("scan dependencies on %s: %w", child.ChildTable, err)
		}
		if hasDeleted {
			activeSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL AND CAST(%s AS TEXT) IN (%s)`,
				transactions.QuoteIdent(child.ChildTable), transactions.QuoteIdent(child.FKColumn), placeholders)
			_ = s.db.QueryRowContext(ctx, activeSQL, args...).Scan(&activeCnt)
		} else {
			activeCnt = totalCnt
		}

		if totalCnt > 0 {
			out = append(out, models.DependencyCount{
				ChildTable:  child.ChildTable,
				ForeignKey:  child.FKColumn,
				ActiveCount: activeCnt,
				TotalCount:  totalCnt,
			})
		}
	}
	return out, nil
}

func cleanNonEmpty(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
