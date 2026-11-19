package expenses

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

type allocTargetItem struct {
	itemID      string
	partNumber  string
	basisValue  float64
	boeID       string
}

type allocExpenseItem struct {
	expenseID        string
	expenseInvoiceID string
	expenseTypeID    string
	category         string
	amountPaise      int64
}

// RecordExpenseAllocation computes statutory landed cost expense allocation across line items
// and inserts an immutable audit run into expense_allocation_ledger.
func (s *Service) RecordExpenseAllocation(ctx context.Context, shipmentID string, reason string) (string, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return "", err
	}

	shipID := strings.TrimSpace(shipmentID)
	if shipID == "" {
		return "", fmt.Errorf("shipmentID is required")
	}
	recalcReason := strings.TrimSpace(reason)
	if recalcReason == "" {
		recalcReason = "PERIODIC_RECALCULATION"
	}

	// 1. Fetch active expenses for this shipment
	expRows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.expense_invoice_id, e.expense_type_id, COALESCE(et.name, 'OTHER'),
		       CASE
		           WHEN COALESCE(e.amount_paise, 0) > 0 THEN e.amount_paise
		           ELSE CAST(ROUND(COALESCE(e.amount, 0.0) * 100) AS INTEGER)
		       END AS amount_paise
		FROM expenses e
		JOIN expense_invoices ei ON ei.id = e.expense_invoice_id AND ei.deleted_at IS NULL
		LEFT JOIN expense_types et ON et.id = e.expense_type_id
		WHERE ei.shipment_id = ? AND e.deleted_at IS NULL
		ORDER BY e.id ASC`, shipID)
	if err != nil {
		return "", fmt.Errorf("query shipment expenses for allocation: %w", err)
	}
	defer expRows.Close()

	var expenses []allocExpenseItem
	for expRows.Next() {
		var itm allocExpenseItem
		if err := expRows.Scan(&itm.expenseID, &itm.expenseInvoiceID, &itm.expenseTypeID, &itm.category, &itm.amountPaise); err != nil {
			return "", err
		}
		if itm.amountPaise > 0 {
			expenses = append(expenses, itm)
		}
	}
	if len(expenses) == 0 {
		return "", nil // No expenses to allocate
	}

	// 2. Fetch targets: prefer latest BOE calculation items, fallback to invoice line items
	var targets []allocTargetItem
	var totalBasis float64
	allocMethod := "ASSESSABLE_VALUE_PRO_RATA"

	boeRows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(bi.item_id, ''), bi.part_number, COALESCE(bi.assessable_value, 0.0), COALESCE(bc.boe_id, '')
		FROM boe_items bi
		JOIN boe_calculations bc ON bc.id = bi.boe_calculation_id
		WHERE bc.shipment_id = ? AND bc.deleted_at IS NULL
		  AND bc.id = (
		      SELECT b2.id FROM boe_calculations b2
		      WHERE b2.shipment_id = bc.shipment_id AND b2.deleted_at IS NULL
		      ORDER BY b2.created_at DESC, b2.id DESC LIMIT 1
		  )
		ORDER BY bi.id ASC`, shipID)
	if err == nil {
		defer boeRows.Close()
		for boeRows.Next() {
			var t allocTargetItem
			if err := boeRows.Scan(&t.itemID, &t.partNumber, &t.basisValue, &t.boeID); err == nil && t.basisValue > 0 {
				targets = append(targets, t)
				totalBasis += t.basisValue
			}
		}
	}

	// Fallback to invoice line items if no BOE items exist
	if len(targets) == 0 {
		allocMethod = "INVOICE_VALUE_PRO_RATA"
		invRows, err := s.db.QueryContext(ctx, `
			SELECT ili.item_id, COALESCE(i.part_number, ili.item_id), (ili.quantity * ili.unit_price) AS line_val
			FROM invoice_line_items ili
			JOIN invoices inv ON inv.id = ili.invoice_id AND inv.deleted_at IS NULL
			LEFT JOIN items i ON i.id = ili.item_id
			WHERE inv.shipment_id = ? AND ili.deleted_at IS NULL
			ORDER BY ili.id ASC`, shipID)
		if err != nil {
			return "", fmt.Errorf("query invoice line items for fallback allocation: %w", err)
		}
		defer invRows.Close()
		for invRows.Next() {
			var t allocTargetItem
			if err := invRows.Scan(&t.itemID, &t.partNumber, &t.basisValue); err == nil && t.basisValue > 0 {
				targets = append(targets, t)
				totalBasis += t.basisValue
			}
		}
	}

	if len(targets) == 0 || totalBasis <= 0 {
		return "", nil // No items to allocate to
	}

	// 3. Insert immutable allocation ledger entries
	runID := "RUN-" + strings.ToUpper(uuid.NewString()[:8])
	now := time.Now().UTC().Format(time.RFC3339)

	err = transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		stmt, prepErr := tx.PrepareContext(ctx, `
			INSERT INTO expense_allocation_ledger (
				run_id, shipment_id, boe_id, expense_invoice_id, expense_type_id, expense_category,
				item_id, part_number, base_expense_amount_paise, allocation_method,
				allocation_basis_value, allocation_percentage, allocated_amount_paise,
				recalculation_reason, calculated_at, calculated_by
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if prepErr != nil {
			return prepErr
		}
		defer stmt.Close()

		for _, exp := range expenses {
			var allocatedSum int64
			for i, tgt := range targets {
				pct := (tgt.basisValue / totalBasis) * 100.0
				var itemPaise int64
				if i == len(targets)-1 {
					// Rounding residual balancing
					itemPaise = exp.amountPaise - allocatedSum
				} else {
					itemPaise = int64(math.Round(float64(exp.amountPaise) * (tgt.basisValue / totalBasis)))
					allocatedSum += itemPaise
				}

				var boeIDArg any
				if strings.TrimSpace(tgt.boeID) != "" {
					boeIDArg = tgt.boeID
				}
				var itemIDArg any
				if strings.TrimSpace(tgt.itemID) != "" {
					itemIDArg = tgt.itemID
				}

				if _, execErr := stmt.ExecContext(ctx,
					runID, shipID, boeIDArg, exp.expenseInvoiceID, exp.expenseTypeID, exp.category,
					itemIDArg, tgt.partNumber, exp.amountPaise, allocMethod,
					tgt.basisValue, pct, itemPaise,
					recalcReason, now, sess.UserID,
				); execErr != nil {
					return fmt.Errorf("insert allocation ledger row: %w", execErr)
				}
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	_ = s.audit.RecordTableAudit("expense_allocation_ledger", runID, "CREATE", sess.UserID, nil, map[string]any{
		"runId":      runID,
		"shipmentId": shipID,
		"reason":     recalcReason,
		"targets":    len(targets),
		"expenses":   len(expenses),
	}, nil)

	return runID, nil
}

// ListAllocationHistory returns allocation records matching the filter criteria.
func (s *Service) ListAllocationHistory(filter models.AllocationLedgerFilter) ([]models.ExpenseAllocationRecord, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}

	q := `
		SELECT id, run_id, shipment_id, COALESCE(boe_id, ''), COALESCE(expense_invoice_id, ''),
		       COALESCE(expense_type_id, ''), expense_category, COALESCE(item_id, ''),
		       part_number, base_expense_amount_paise, allocation_method, allocation_basis_value,
		       allocation_percentage, allocated_amount_paise, recalculation_reason, calculated_at, calculated_by
		FROM expense_allocation_ledger
		WHERE 1=1`
	var args []any

	if st := strings.TrimSpace(filter.ShipmentID); st != "" {
		q += ` AND shipment_id = ?`
		args = append(args, st)
	}
	if st := strings.TrimSpace(filter.RunID); st != "" {
		q += ` AND run_id = ?`
		args = append(args, st)
	}
	if st := strings.TrimSpace(filter.PartNumber); st != "" {
		q += ` AND part_number LIKE ?`
		args = append(args, "%"+st+"%")
	}
	if filter.StartDate != "" {
		q += ` AND calculated_at >= ?`
		args = append(args, filter.StartDate)
	}
	if filter.EndDate != "" {
		q += ` AND calculated_at <= ?`
		args = append(args, filter.EndDate)
	}

	q += ` ORDER BY calculated_at DESC, id ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list allocation history: %w", err)
	}
	defer rows.Close()

	var out []models.ExpenseAllocationRecord
	for rows.Next() {
		var r models.ExpenseAllocationRecord
		if err := rows.Scan(
			&r.ID, &r.RunID, &r.ShipmentID, &r.BOEID, &r.ExpenseInvoiceID,
			&r.ExpenseTypeID, &r.ExpenseCategory, &r.ItemID,
			&r.PartNumber, &r.BaseExpenseAmountPaise, &r.AllocationMethod, &r.AllocationBasisValue,
			&r.AllocationPercentage, &r.AllocatedAmountPaise, &r.RecalculationReason, &r.CalculatedAt, &r.CalculatedBy,
		); err != nil {
			return nil, err
		}
		r.AllocatedAmountINR = float64(r.AllocatedAmountPaise) / 100.0
		out = append(out, r)
	}
	if out == nil {
		out = []models.ExpenseAllocationRecord{}
	}
	return out, rows.Err()
}

// ListAllocationRuns lists historical calculation runs for a shipment.
func (s *Service) ListAllocationRuns(shipmentID string) ([]models.AllocationRunSummary, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT run_id, shipment_id, SUM(allocated_amount_paise) / 100.0 AS total_inr,
		       COUNT(DISTINCT part_number) AS item_count, recalculation_reason, calculated_at, calculated_by
		FROM expense_allocation_ledger
		WHERE shipment_id = ?
		GROUP BY run_id, shipment_id, recalculation_reason, calculated_at, calculated_by
		ORDER BY calculated_at DESC`, strings.TrimSpace(shipmentID))
	if err != nil {
		return nil, fmt.Errorf("list allocation runs: %w", err)
	}
	defer rows.Close()

	var out []models.AllocationRunSummary
	for rows.Next() {
		var sum models.AllocationRunSummary
		if err := rows.Scan(&sum.RunID, &sum.ShipmentID, &sum.TotalAllocatedINR, &sum.ItemCount, &sum.RecalculationReason, &sum.CalculatedAt, &sum.CalculatedBy); err != nil {
			return nil, err
		}
		out = append(out, sum)
	}
	if out == nil {
		out = []models.AllocationRunSummary{}
	}
	return out, rows.Err()
}
