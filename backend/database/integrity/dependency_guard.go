package integrity

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
)

// ChildRelation defines a dependent child table and referencing foreign key column.
type ChildRelation struct {
	ChildTable string
	FKColumn   string
}

// ParentChildRegistry defines the canonical child dependencies for all core ERP business entities.
var ParentChildRegistry = map[string][]ChildRelation{
	"suppliers": {
		{ChildTable: "shipments", FKColumn: "supplier_id"},
		{ChildTable: "items", FKColumn: "supplier_id"},
	},
	"items": {
		{ChildTable: "invoice_line_items", FKColumn: "item_id"},
		{ChildTable: "boe_items", FKColumn: "item_id"},
	},
	"shipments": {
		{ChildTable: "invoices", FKColumn: "shipment_id"},
		{ChildTable: "boe_calculations", FKColumn: "shipment_id"},
		{ChildTable: "shipment_containers", FKColumn: "shipment_id"},
		{ChildTable: "expense_invoices", FKColumn: "shipment_id"},
		{ChildTable: "expenses", FKColumn: "shipment_id"},
	},
	"invoices": {
		{ChildTable: "invoice_line_items", FKColumn: "invoice_id"},
	},
	"boe_details": {
		{ChildTable: "boe_calculations", FKColumn: "boe_id"},
	},
	"boe_calculations": {
		{ChildTable: "boe_items", FKColumn: "boe_calculation_id"},
		{ChildTable: "boe_attachments", FKColumn: "boe_calculation_id"},
	},
	"service_providers": {
		{ChildTable: "expense_invoices", FKColumn: "service_provider_id"},
		{ChildTable: "expenses", FKColumn: "service_provider_id"},
	},
	"expense_types": {
		{ChildTable: "expenses", FKColumn: "expense_type_id"},
	},
	"expense_invoices": {
		{ChildTable: "expenses", FKColumn: "expense_invoice_id"},
		{ChildTable: "expense_attachments", FKColumn: "expense_invoice_id"},
	},
	"expenses": {
		{ChildTable: "expense_attachments", FKColumn: "expense_id"},
	},
}

// DependencyViolationError is returned when a deletion is rejected due to existing child dependencies.
type DependencyViolationError struct {
	ParentTable  string                   `json:"parentTable"`
	ParentID     string                   `json:"parentId"`
	Dependencies []models.DependencyCount `json:"dependencies"`
	Message      string                   `json:"message"`
}

func (e *DependencyViolationError) Error() string {
	return e.Message
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// CheckDependencies inspects whether a single parent record has dependent child records.
// When onlyActive is true, only non-soft-deleted child records are counted (used for soft-delete guards).
// When onlyActive is false, all referencing records are counted (used for permanent purge guards).
func CheckDependencies(ctx context.Context, db *sql.DB, parentTable, parentID string, onlyActive bool) (*DependencyViolationError, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	return checkDependencies(ctx, db, parentTable, parentID, onlyActive)
}

// CheckDependenciesTx inspects dependent child records using an active SQL transaction.
// Use this inside transactions.WithinTx to avoid single-connection SQLite lock contention.
func CheckDependenciesTx(ctx context.Context, tx *sql.Tx, parentTable, parentID string, onlyActive bool) (*DependencyViolationError, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction handle is nil")
	}
	return checkDependencies(ctx, tx, parentTable, parentID, onlyActive)
}

func checkDependencies(ctx context.Context, db queryRower, parentTable, parentID string, onlyActive bool) (*DependencyViolationError, error) {
	cleanID := strings.TrimSpace(parentID)
	if cleanID == "" {
		return nil, fmt.Errorf("parent ID cannot be empty")
	}

	children, ok := ParentChildRegistry[parentTable]
	if !ok || len(children) == 0 {
		return nil, nil
	}

	var deps []models.DependencyCount
	var summaryParts []string

	for _, rel := range children {
		var hasDeletedCol bool
		_ = db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info(%s) WHERE name = 'deleted_at'`,
				transactions.QuoteIdent(rel.ChildTable)),
		).Scan(&hasDeletedCol)

		var totalCnt, activeCnt int64
		totalSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE CAST(%s AS TEXT) = ?`,
			transactions.QuoteIdent(rel.ChildTable), transactions.QuoteIdent(rel.FKColumn))
		if err := db.QueryRowContext(ctx, totalSQL, cleanID).Scan(&totalCnt); err != nil {
			return nil, fmt.Errorf("query dependencies on %s: %w", rel.ChildTable, err)
		}

		if hasDeletedCol {
			activeSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL AND CAST(%s AS TEXT) = ?`,
				transactions.QuoteIdent(rel.ChildTable), transactions.QuoteIdent(rel.FKColumn))
			_ = db.QueryRowContext(ctx, activeSQL, cleanID).Scan(&activeCnt)
		} else {
			activeCnt = totalCnt
		}

		checkCnt := totalCnt
		if onlyActive {
			checkCnt = activeCnt
		}

		if checkCnt > 0 {
			deps = append(deps, models.DependencyCount{
				ChildTable:  rel.ChildTable,
				ForeignKey:  rel.FKColumn,
				ActiveCount: activeCnt,
				TotalCount:  totalCnt,
			})
			summaryParts = append(summaryParts, fmt.Sprintf("%s.%s (%d)", rel.ChildTable, rel.FKColumn, checkCnt))
		}
	}

	if len(deps) > 0 {
		statusLabel := "dependent"
		if onlyActive {
			statusLabel = "active"
		}
		msg := fmt.Sprintf("DEPENDENCY: cannot delete %s %s because %s child records exist in %s",
			parentTable, cleanID, statusLabel, strings.Join(summaryParts, ", "))
		return &DependencyViolationError{
			ParentTable:  parentTable,
			ParentID:     cleanID,
			Dependencies: deps,
			Message:      msg,
		}, nil
	}

	return nil, nil
}

// CheckBatchDependencies scans dependencies across an array of candidate parent IDs in chunks.
func CheckBatchDependencies(ctx context.Context, db *sql.DB, parentTable string, parentIDs []string, onlyActive bool) ([]models.DependencyCount, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	cleanIDs := make([]string, 0, len(parentIDs))
	seen := make(map[string]struct{}, len(parentIDs))
	for _, id := range parentIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			if _, exists := seen[trimmed]; !exists {
				seen[trimmed] = struct{}{}
				cleanIDs = append(cleanIDs, trimmed)
			}
		}
	}
	if len(cleanIDs) == 0 {
		return nil, 0, nil
	}

	children, ok := ParentChildRegistry[parentTable]
	if !ok || len(children) == 0 {
		return nil, 0, nil
	}

	const chunkSize = 500
	var out []models.DependencyCount
	var grandTotal int64
	summaryMap := make(map[string]*models.DependencyCount)

	for i := 0; i < len(cleanIDs); i += chunkSize {
		end := min(i+chunkSize, len(cleanIDs))
		batch := cleanIDs[i:end]

		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		args := make([]any, 0, len(batch))
		for _, id := range batch {
			args = append(args, id)
		}

		for _, rel := range children {
			key := rel.ChildTable + "." + rel.FKColumn
			if _, exists := summaryMap[key]; !exists {
				summaryMap[key] = &models.DependencyCount{
					ChildTable: rel.ChildTable,
					ForeignKey: rel.FKColumn,
				}
			}

			var hasDeletedCol bool
			_ = db.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info(%s) WHERE name = 'deleted_at'`,
					transactions.QuoteIdent(rel.ChildTable)),
			).Scan(&hasDeletedCol)

			var totalCnt, activeCnt int64
			totalSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE CAST(%s AS TEXT) IN (%s)`,
				transactions.QuoteIdent(rel.ChildTable), transactions.QuoteIdent(rel.FKColumn), placeholders)
			if err := db.QueryRowContext(ctx, totalSQL, args...).Scan(&totalCnt); err != nil {
				return nil, 0, fmt.Errorf("scan dependencies on %s: %w", rel.ChildTable, err)
			}

			if hasDeletedCol {
				activeSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL AND CAST(%s AS TEXT) IN (%s)`,
					transactions.QuoteIdent(rel.ChildTable), transactions.QuoteIdent(rel.FKColumn), placeholders)
				_ = db.QueryRowContext(ctx, activeSQL, args...).Scan(&activeCnt)
			} else {
				activeCnt = totalCnt
			}

			summaryMap[key].TotalCount += totalCnt
			summaryMap[key].ActiveCount += activeCnt
		}
	}

	for _, rel := range children {
		key := rel.ChildTable + "." + rel.FKColumn
		stat := summaryMap[key]
		checkCnt := stat.TotalCount
		if onlyActive {
			checkCnt = stat.ActiveCount
		}
		if checkCnt > 0 {
			out = append(out, *stat)
			grandTotal += checkCnt
		}
	}

	return out, grandTotal, nil
}
