package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
)

// InvoiceRepository manages persistence for commercial invoices and invoice_line_items.
type InvoiceRepository struct {
	db *sql.DB
}

// NewInvoiceRepository constructs an InvoiceRepository bound to db.
func NewInvoiceRepository(db *sql.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

// ListAll returns commercial invoices ordered newest-first with computed totals and shipment metadata.
func (r *InvoiceRepository) ListAll(search, status string) ([]models.InvoiceSummary, error) {
	where := []string{"inv.deleted_at IS NULL", "s.deleted_at IS NULL"}
	var args []any

	if q := strings.TrimSpace(search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(inv.id LIKE ? OR s.invoice_number LIKE ? OR COALESCE(sup.supplier_name,'') LIKE ? OR inv.shipment_id LIKE ?)")
		args = append(args, like, like, like, like)
	}
	if st := strings.TrimSpace(status); st != "" {
		where = append(where, "LOWER(inv.status) = LOWER(?)")
		args = append(args, st)
	}

	query := fmt.Sprintf(`
		SELECT inv.id, inv.shipment_id, s.supplier_id, COALESCE(sup.supplier_name,''),
		       s.invoice_number, s.invoice_date, s.invoice_currency, s.invoice_value,
		       COALESCE((
		           SELECT SUM(ROUND(ili.quantity * ili.unit_price, inv.line_total_decimals))
		           FROM invoice_line_items ili
		           WHERE ili.invoice_id = inv.id AND ili.deleted_at IS NULL
		       ), 0),
		       COALESCE((
		           SELECT COUNT(*)
		           FROM invoice_line_items ili
		           WHERE ili.invoice_id = inv.id AND ili.deleted_at IS NULL
		       ), 0),
		       inv.status, inv.line_total_decimals, inv.invoice_total_decimals, s.is_frozen
		FROM invoices inv
		JOIN shipments s ON s.id = inv.shipment_id
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		WHERE %s
		ORDER BY s.invoice_date DESC, inv.id DESC`, strings.Join(where, " AND "))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	var out []models.InvoiceSummary
	for rows.Next() {
		var sum models.InvoiceSummary
		if err := rows.Scan(
			&sum.ID, &sum.ShipmentID, &sum.SupplierID, &sum.SupplierName,
			&sum.InvoiceNumber, &sum.InvoiceDate, &sum.Currency, &sum.ShipmentInvoiceValue,
			&sum.ComputedTotal, &sum.LineCount, &sum.Status,
			&sum.LineTotalDecimals, &sum.InvoiceTotalDecimals, &sum.ShipmentFrozen,
		); err != nil {
			return nil, err
		}
		sum.ComputedTotal = RoundDecimals(sum.ComputedTotal, sum.InvoiceTotalDecimals)
		out = append(out, sum)
	}
	return out, rows.Err()
}

// ListByShipment fetches invoices attached to a specific shipment.
func (r *InvoiceRepository) ListByShipment(shipmentID string) ([]models.InvoiceSummary, error) {
	all, err := r.ListAll("", "")
	if err != nil {
		return nil, err
	}
	var filtered []models.InvoiceSummary
	for _, inv := range all {
		if inv.ShipmentID == strings.TrimSpace(shipmentID) {
			filtered = append(filtered, inv)
		}
	}
	return filtered, nil
}

// GetDetail fetches a single commercial invoice header plus enriched line items.
func (r *InvoiceRepository) GetDetail(invoiceID string) (models.InvoiceDetail, error) {
	var sum models.InvoiceSummary
	err := r.db.QueryRow(`
		SELECT inv.id, inv.shipment_id, s.supplier_id, COALESCE(sup.supplier_name,''),
		       s.invoice_number, s.invoice_date, s.invoice_currency, s.invoice_value,
		       0, 0,
		       inv.status, inv.line_total_decimals, inv.invoice_total_decimals, s.is_frozen
		FROM invoices inv
		JOIN shipments s ON s.id = inv.shipment_id
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		WHERE inv.id = ? AND inv.deleted_at IS NULL`, strings.TrimSpace(invoiceID)).Scan(
		&sum.ID, &sum.ShipmentID, &sum.SupplierID, &sum.SupplierName,
		&sum.InvoiceNumber, &sum.InvoiceDate, &sum.Currency, &sum.ShipmentInvoiceValue,
		&sum.ComputedTotal, &sum.LineCount, &sum.Status,
		&sum.LineTotalDecimals, &sum.InvoiceTotalDecimals, &sum.ShipmentFrozen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.InvoiceDetail{}, fmt.Errorf("invoice %q not found", invoiceID)
	}
	if err != nil {
		return models.InvoiceDetail{}, err
	}

	rows, err := r.db.Query(`
		SELECT ili.id, ili.invoice_id, ili.item_id,
		       COALESCE(i.part_number,''), COALESCE(i.item_description,''), COALESCE(i.unit,''), COALESCE(i.hsn_code,''),
		       ili.quantity, ili.unit_price, COALESCE(i.unit_price, 0),
		       COALESCE(ili.duty_percent, 0), COALESCE(ili.sws_percent, 0), COALESCE(ili.igst_percent, 0)
		FROM invoice_line_items ili
		LEFT JOIN items i ON i.id = ili.item_id
		WHERE ili.invoice_id = ? AND ili.deleted_at IS NULL
		ORDER BY ili.id ASC`, sum.ID)
	if err != nil {
		return models.InvoiceDetail{}, fmt.Errorf("query invoice lines: %w", err)
	}
	defer rows.Close()

	var lines []models.InvoiceLineItem
	var total float64
	for rows.Next() {
		var ln models.InvoiceLineItem
		if err := rows.Scan(
			&ln.ID, &ln.InvoiceID, &ln.ItemID,
			&ln.PartNumber, &ln.ItemDescription, &ln.Unit, &ln.HSNCode,
			&ln.Quantity, &ln.UnitPrice, &ln.MasterUnitPrice,
			&ln.DutyPercent, &ln.SWSPercent, &ln.IGSTPercent,
		); err != nil {
			return models.InvoiceDetail{}, err
		}
		ln.LineTotal = RoundDecimals(ln.Quantity*ln.UnitPrice, sum.LineTotalDecimals)
		ln.PriceDiverged = ln.MasterUnitPrice > 0 && math.Abs(ln.UnitPrice-ln.MasterUnitPrice) > 0.001
		total += ln.LineTotal
		lines = append(lines, ln)
	}
	if err := rows.Err(); err != nil {
		return models.InvoiceDetail{}, err
	}

	sum.LineCount = len(lines)
	sum.ComputedTotal = RoundDecimals(total, sum.InvoiceTotalDecimals)
	return models.InvoiceDetail{Invoice: sum, Lines: lines}, nil
}

// NextInvoiceID computes the next sequential INV-XXX identifier.
func (r *InvoiceRepository) NextInvoiceID() (string, error) {
	rows, err := r.db.Query(`SELECT id FROM invoices WHERE id LIKE 'INV-%'`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	maxNum := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "INV-")); err == nil && n > maxNum {
			maxNum = n
		}
	}
	return fmt.Sprintf("INV-%03d", maxNum+1), rows.Err()
}

// SoftDelete marks an invoice and its line items as deleted within a single atomic transaction.
func (r *InvoiceRepository) SoftDelete(id, userID string) error {
	cleanID := strings.TrimSpace(id)
	now := time.Now().UTC().Format(time.RFC3339)
	return transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE invoices SET deleted_at = ?, deleted_by = ? WHERE id = ? AND deleted_at IS NULL`, now, userID, cleanID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("invoice %q not found", cleanID)
		}
		_, err = tx.Exec(`UPDATE invoice_line_items SET deleted_at = ?, deleted_by = ? WHERE invoice_id = ? AND deleted_at IS NULL`, now, userID, cleanID)
		return err
	})
}

// RoundDecimals rounds float values cleanly using standard half-up precision at 0 or 2 decimals.
func RoundDecimals(val float64, decimals int) float64 {
	if decimals <= 0 {
		return math.Round(val)
	}
	factor := math.Pow(10, float64(decimals))
	return math.Round(val*factor) / factor
}
