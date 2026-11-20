package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
)

// PaymentPORepository handles persistence and financial reconciliation for Purchase Orders,
// staged payment milestones, payment vouchers, allocations, and material receipts.
type PaymentPORepository struct {
	db *sql.DB
}

// NewPaymentPORepository constructs a PaymentPORepository bound to db.
func NewPaymentPORepository(db *sql.DB) *PaymentPORepository {
	return &PaymentPORepository{db: db}
}

// CreatePurchaseOrder saves a new PO, line items, and generates payment milestones in a transaction.
func (r *PaymentPORepository) CreatePurchaseOrder(input models.PurchaseOrderInput) (models.PurchaseOrder, error) {
	if strings.TrimSpace(input.PONumber) == "" {
		return models.PurchaseOrder{}, errors.New("PO number is required")
	}
	if strings.TrimSpace(input.SupplierID) == "" {
		return models.PurchaseOrder{}, errors.New("supplier is required")
	}
	if len(input.LineItems) == 0 {
		return models.PurchaseOrder{}, errors.New("at least one line item is required")
	}

	poID := uuid.New().String()
	if input.Currency == "" {
		input.Currency = "USD"
	}
	if input.ExchangeRate <= 0 {
		input.ExchangeRate = 1.0
	}
	if input.Incoterm == "" {
		input.Incoterm = "FOB"
	}
	if input.PaymentTermsPreset == "" {
		input.PaymentTermsPreset = "100_ADVANCE"
	}
	if input.Status == "" {
		input.Status = "CONFIRMED"
	}

	var totalAmount float64
	for _, li := range input.LineItems {
		totalAmount += math.Round((li.Quantity*li.UnitPrice)*100) / 100
	}

	now := time.Now().UTC().Format(time.RFC3339)

	err := transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		// Verify PO number uniqueness
		var exists int
		err := tx.QueryRow(`SELECT COUNT(*) FROM purchase_orders WHERE po_number = ? AND deleted_at IS NULL`, input.PONumber).Scan(&exists)
		if err != nil {
			return err
		}
		if exists > 0 {
			return fmt.Errorf("purchase order number %q already exists", input.PONumber)
		}

		// Insert PO Header
		_, err = tx.Exec(`
			INSERT INTO purchase_orders (
				id, po_number, po_date, supplier_id, currency, exchange_rate, incoterm,
				payment_terms_preset, payment_terms_desc, total_amount, status,
				expected_delivery_date, notes, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			poID, input.PONumber, input.PODate, input.SupplierID, input.Currency, input.ExchangeRate, input.Incoterm,
			input.PaymentTermsPreset, input.PaymentTermsDesc, totalAmount, input.Status,
			input.ExpectedDeliveryDate, input.Notes, now, now,
		)
		if err != nil {
			return fmt.Errorf("insert purchase order: %w", err)
		}

		// Insert Line Items
		for _, li := range input.LineItems {
			lineID := uuid.New().String()
			lt := math.Round((li.Quantity*li.UnitPrice)*100) / 100
			_, err = tx.Exec(`
				INSERT INTO po_line_items (
					id, po_id, item_id, part_number, item_description, quantity,
					unit, unit_price, line_total, hsn_code, received_quantity
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0.0)`,
				lineID, poID, nullableString(li.ItemID), li.PartNumber, li.ItemDescription,
				li.Quantity, li.Unit, li.UnitPrice, lt, nullableString(li.HSNCode),
			)
			if err != nil {
				return fmt.Errorf("insert po line item: %w", err)
			}
		}

		// Generate / Insert Milestones
		milestones := generateMilestones(poID, totalAmount, input.PaymentTermsPreset, input.Milestones)
		for _, ms := range milestones {
			msID := uuid.New().String()
			msAmount := math.Round((totalAmount*ms.Percentage/100.0)*100) / 100
			_, err = tx.Exec(`
				INSERT INTO po_payment_milestones (
					id, po_id, sequence, milestone_name, stage_type, percentage,
					amount, paid_amount, due_date, status, notes
				) VALUES (?, ?, ?, ?, ?, ?, ?, 0.0, ?, 'PENDING', ?)`,
				msID, poID, ms.Sequence, ms.MilestoneName, ms.StageType, ms.Percentage,
				msAmount, nullableString(ms.DueDate), nullableString(ms.Notes),
			)
			if err != nil {
				return fmt.Errorf("insert po milestone: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return models.PurchaseOrder{}, err
	}

	return r.GetPurchaseOrderByID(poID)
}

func generateMilestones(poID string, totalAmount float64, preset string, custom []models.POMilestoneInput) []models.POMilestoneInput {
	if preset == "CUSTOM" && len(custom) > 0 {
		return custom
	}

	switch preset {
	case "40_40_20":
		return []models.POMilestoneInput{
			{Sequence: 1, MilestoneName: "40% Advance on PO Confirmation", StageType: "ADVANCE", Percentage: 40.0, Notes: "Payable upon formal order confirmation"},
			{Sequence: 2, MilestoneName: "40% Against Bill of Lading (BL) / Dispatch", StageType: "ON_DISPATCH", Percentage: 40.0, Notes: "Payable on provision of shipping documents & BL copy"},
			{Sequence: 3, MilestoneName: "20% on Factory Delivery & Inspection", StageType: "ON_DELIVERY", Percentage: 20.0, Notes: "Payable after physical arrival and QC clearance"},
		}
	case "30_30_40":
		return []models.POMilestoneInput{
			{Sequence: 1, MilestoneName: "30% Advance on PO Confirmation", StageType: "ADVANCE", Percentage: 30.0, Notes: "Payable on order confirmation"},
			{Sequence: 2, MilestoneName: "30% Against Bill of Lading (BL) / Dispatch", StageType: "ON_DISPATCH", Percentage: 30.0, Notes: "Payable on export BL copy"},
			{Sequence: 3, MilestoneName: "40% on Factory Delivery & Inspection", StageType: "ON_DELIVERY", Percentage: 40.0, Notes: "Payable after final acceptance"},
		}
	case "50_50":
		return []models.POMilestoneInput{
			{Sequence: 1, MilestoneName: "50% Advance on PO Confirmation", StageType: "ADVANCE", Percentage: 50.0, Notes: "Payable upon PO sign-off"},
			{Sequence: 2, MilestoneName: "50% on Bill of Lading (BL) / Delivery", StageType: "ON_DISPATCH", Percentage: 50.0, Notes: "Payable on shipping document dispatch"},
		}
	case "100_ADVANCE":
		fallthrough
	default:
		return []models.POMilestoneInput{
			{Sequence: 1, MilestoneName: "100% Full Advance Payment", StageType: "ADVANCE", Percentage: 100.0, Notes: "100% full advance before production/dispatch"},
		}
	}
}

// GetPurchaseOrderByID fetches a complete PO dossier with computed advances, lines, and milestones.
func (r *PaymentPORepository) GetPurchaseOrderByID(id string) (models.PurchaseOrder, error) {
	var po models.PurchaseOrder
	query := `
		SELECT po.id, po.po_number, po.po_date, po.supplier_id, COALESCE(sup.supplier_name, ''),
		       po.currency, po.exchange_rate, po.incoterm, po.payment_terms_preset,
		       po.payment_terms_desc, po.total_amount, po.status, COALESCE(po.expected_delivery_date, ''),
		       COALESCE(po.notes, ''), po.created_at, po.updated_at
		FROM purchase_orders po
		LEFT JOIN suppliers sup ON sup.id = po.supplier_id
		WHERE po.id = ? AND po.deleted_at IS NULL`

	err := r.db.QueryRow(query, id).Scan(
		&po.ID, &po.PONumber, &po.PODate, &po.SupplierID, &po.SupplierName,
		&po.Currency, &po.ExchangeRate, &po.Incoterm, &po.PaymentTermsPreset,
		&po.PaymentTermsDesc, &po.TotalAmount, &po.Status, &po.ExpectedDeliveryDate,
		&po.Notes, &po.CreatedAt, &po.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.PurchaseOrder{}, fmt.Errorf("purchase order %s not found", id)
		}
		return models.PurchaseOrder{}, fmt.Errorf("get purchase order: %w", err)
	}

	// Fetch Milestones
	msRows, err := r.db.Query(`
		SELECT id, po_id, sequence, milestone_name, stage_type, percentage,
		       amount, paid_amount, (amount - paid_amount) AS balance,
		       COALESCE(due_date, ''), status, COALESCE(notes, '')
		FROM po_payment_milestones
		WHERE po_id = ?
		ORDER BY sequence ASC`, id)
	if err != nil {
		return models.PurchaseOrder{}, fmt.Errorf("get po milestones: %w", err)
	}
	defer msRows.Close()

	for msRows.Next() {
		var ms models.POPaymentMilestone
		if err := msRows.Scan(
			&ms.ID, &ms.POID, &ms.Sequence, &ms.MilestoneName, &ms.StageType, &ms.Percentage,
			&ms.Amount, &ms.PaidAmount, &ms.BalanceAmount, &ms.DueDate, &ms.Status, &ms.Notes,
		); err != nil {
			return models.PurchaseOrder{}, err
		}
		po.Milestones = append(po.Milestones, ms)

		po.TotalPaid += ms.PaidAmount
		if ms.StageType == "ADVANCE" {
			po.AdvanceRequired += ms.Amount
			po.AdvancePaid += ms.PaidAmount
		}
	}
	po.AdvanceBalance = math.Max(0, po.AdvanceRequired-po.AdvancePaid)
	po.TotalBalance = math.Max(0, po.TotalAmount-po.TotalPaid)

	// Fetch Line Items
	liRows, err := r.db.Query(`
		SELECT id, po_id, COALESCE(item_id, ''), part_number, item_description,
		       quantity, unit, unit_price, line_total, COALESCE(hsn_code, ''),
		       received_quantity
		FROM po_line_items
		WHERE po_id = ? AND deleted_at IS NULL
		ORDER BY part_number ASC`, id)
	if err != nil {
		return models.PurchaseOrder{}, fmt.Errorf("get po line items: %w", err)
	}
	defer liRows.Close()

	advRatio := 0.0
	if po.TotalAmount > 0 {
		advRatio = po.AdvancePaid / po.TotalAmount
	}

	for liRows.Next() {
		var li models.POLineItem
		if err := liRows.Scan(
			&li.ID, &li.POID, &li.ItemID, &li.PartNumber, &li.ItemDescription,
			&li.Quantity, &li.Unit, &li.UnitPrice, &li.LineTotal, &li.HSNCode,
			&li.ReceivedQuantity,
		); err != nil {
			return models.PurchaseOrder{}, err
		}

		li.PendingQuantity = math.Max(0, li.Quantity-li.ReceivedQuantity)
		li.AdvanceCoveredQuantity = math.Round((li.Quantity*advRatio)*100) / 100
		li.AdvancePendingQuantity = math.Max(0, li.AdvanceCoveredQuantity-li.ReceivedQuantity)

		if li.ReceivedQuantity >= li.Quantity {
			li.FulfillmentStatus = "FULLY_RECEIVED"
		} else if li.ReceivedQuantity > 0 {
			li.FulfillmentStatus = "PARTIALLY_RECEIVED"
		} else {
			li.FulfillmentStatus = "AWAITING_SHIPMENT"
		}

		po.LineItems = append(po.LineItems, li)
	}

	return po, nil
}

// ListPurchaseOrders lists paginated purchase orders with search and filters.
func (r *PaymentPORepository) ListPurchaseOrders(req models.PurchaseOrderPageRequest) (models.PurchaseOrderPage, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 25
	}

	where := []string{"po.deleted_at IS NULL"}
	var args []any

	if q := strings.TrimSpace(req.Search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(po.po_number LIKE ? OR sup.supplier_name LIKE ? OR po.notes LIKE ?)")
		args = append(args, like, like, like)
	}
	if s := strings.TrimSpace(req.SupplierID); s != "" {
		where = append(where, "po.supplier_id = ?")
		args = append(args, s)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		where = append(where, "po.status = ?")
		args = append(args, st)
	}

	whereClause := strings.Join(where, " AND ")

	var totalCount int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM purchase_orders po
		LEFT JOIN suppliers sup ON sup.id = po.supplier_id
		WHERE %s`, whereClause)
	if err := r.db.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
		return models.PurchaseOrderPage{}, fmt.Errorf("count purchase orders: %w", err)
	}

	offset := (req.Page - 1) * req.PageSize
	query := fmt.Sprintf(`
		SELECT po.id, po.po_number, po.po_date, po.supplier_id, COALESCE(sup.supplier_name, ''),
		       po.currency, po.exchange_rate, po.incoterm, po.payment_terms_preset,
		       po.payment_terms_desc, po.total_amount, po.status, COALESCE(po.expected_delivery_date, ''),
		       COALESCE(po.notes, ''), po.created_at, po.updated_at,
		       COALESCE((
		           SELECT SUM(pm.amount) FROM po_payment_milestones pm
		           WHERE pm.po_id = po.id AND pm.stage_type = 'ADVANCE'
		       ), 0.0) AS adv_required,
		       COALESCE((
		           SELECT SUM(pm.paid_amount) FROM po_payment_milestones pm
		           WHERE pm.po_id = po.id AND pm.stage_type = 'ADVANCE'
		       ), 0.0) AS adv_paid,
		       COALESCE((
		           SELECT SUM(pm.paid_amount) FROM po_payment_milestones pm
		           WHERE pm.po_id = po.id
		       ), 0.0) AS total_paid
		FROM purchase_orders po
		LEFT JOIN suppliers sup ON sup.id = po.supplier_id
		WHERE %s
		ORDER BY po.po_date DESC, po.po_number DESC
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, req.PageSize, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return models.PurchaseOrderPage{}, fmt.Errorf("list purchase orders: %w", err)
	}
	defer rows.Close()

	var items []models.PurchaseOrder
	for rows.Next() {
		var po models.PurchaseOrder
		if err := rows.Scan(
			&po.ID, &po.PONumber, &po.PODate, &po.SupplierID, &po.SupplierName,
			&po.Currency, &po.ExchangeRate, &po.Incoterm, &po.PaymentTermsPreset,
			&po.PaymentTermsDesc, &po.TotalAmount, &po.Status, &po.ExpectedDeliveryDate,
			&po.Notes, &po.CreatedAt, &po.UpdatedAt,
			&po.AdvanceRequired, &po.AdvancePaid, &po.TotalPaid,
		); err != nil {
			return models.PurchaseOrderPage{}, err
		}
		po.AdvanceBalance = math.Max(0, po.AdvanceRequired-po.AdvancePaid)
		po.TotalBalance = math.Max(0, po.TotalAmount-po.TotalPaid)
		items = append(items, po)
	}

	return models.PurchaseOrderPage{
		Items:      items,
		TotalCount: totalCount,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

// DeletePurchaseOrder soft deletes a PO if no payments have been processed against it.
func (r *PaymentPORepository) DeletePurchaseOrder(id, user string) error {
	var paymentCount int
	err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM payment_allocations pa
		JOIN payments p ON p.id = pa.payment_id
		WHERE pa.po_id = ? AND p.deleted_at IS NULL`, id).Scan(&paymentCount)
	if err != nil {
		return fmt.Errorf("check po payments: %w", err)
	}
	if paymentCount > 0 {
		return errors.New("cannot delete purchase order: active payments/advances exist against this order")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		UPDATE purchase_orders
		SET deleted_at = ?, deleted_by = ?
		WHERE id = ? AND deleted_at IS NULL`, now, user, id)
	if err != nil {
		return fmt.Errorf("delete purchase order: %w", err)
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return errors.New("purchase order not found or already deleted")
	}
	return nil
}

// CreatePayment creates a new payment voucher and records allocations to invoices or PO milestones.
func (r *PaymentPORepository) CreatePayment(input models.PaymentInput, user string) (models.Payment, error) {
	if strings.TrimSpace(input.SupplierID) == "" {
		return models.Payment{}, errors.New("supplier is required")
	}
	if len(input.Allocations) == 0 {
		return models.Payment{}, errors.New("at least one payment allocation is required")
	}
	if input.TotalAmount <= 0 {
		return models.Payment{}, errors.New("total payment amount must be greater than zero")
	}

	paymentID := uuid.New().String()
	voucherNum := fmt.Sprintf("PAY-%s-%04d", time.Now().Format("2006"), time.Now().Unix()%10000)
	now := time.Now().UTC().Format(time.RFC3339)

	if input.PaymentMode == "" {
		input.PaymentMode = "WIRE_TRANSFER"
	}
	if input.Currency == "" {
		input.Currency = "USD"
	}
	if input.ExchangeRate <= 0 {
		input.ExchangeRate = 1.0
	}

	// Fetch supplier banking info if beneficiary fields are blank
	if input.BeneficiaryName == "" {
		_ = r.db.QueryRow(`
			SELECT COALESCE(beneficiary_name, supplier_name), COALESCE(bank_name, ''),
			       COALESCE(account_no, ''), COALESCE(swift_code, ''), COALESCE(iban, '')
			FROM suppliers WHERE id = ?`, input.SupplierID).Scan(
			&input.BeneficiaryName, &input.BeneficiaryBank, &input.BeneficiaryAccount,
			&input.BeneficiarySWIFT, &input.BeneficiaryIBAN,
		)
	}

	err := transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		// Insert payment header
		_, err := tx.Exec(`
			INSERT INTO payments (
				id, voucher_number, payment_date, supplier_id, payment_type, payment_mode,
				currency, exchange_rate, total_amount, bank_name, bank_account_no, reference_number,
				beneficiary_name, beneficiary_bank, beneficiary_account, beneficiary_swift, beneficiary_iban,
				status, notes, created_by, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'PROCESSED', ?, ?, ?, ?)`,
			paymentID, voucherNum, input.PaymentDate, input.SupplierID, input.PaymentType, input.PaymentMode,
			input.Currency, input.ExchangeRate, input.TotalAmount, input.BankName, input.BankAccountNo, input.ReferenceNumber,
			input.BeneficiaryName, input.BeneficiaryBank, input.BeneficiaryAccount, input.BeneficiarySWIFT, input.BeneficiaryIBAN,
			input.Notes, user, now, now,
		)
		if err != nil {
			return fmt.Errorf("insert payment header: %w", err)
		}

		// Insert allocations & update milestones/invoices
		for _, alloc := range input.Allocations {
			allocID := uuid.New().String()
			netPaid := math.Round((alloc.GrossAmount-alloc.AdvanceAdjusted-alloc.Deductions)*100) / 100
			if netPaid < 0 {
				netPaid = 0
			}

			_, err = tx.Exec(`
				INSERT INTO payment_allocations (
					id, payment_id, po_id, po_milestone_id, invoice_id, shipment_id,
					gross_amount, advance_adjusted, deductions, net_paid_amount, notes
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				allocID, paymentID, nullableString(alloc.POID), nullableString(alloc.POMilestoneID),
				nullableString(alloc.InvoiceID), nullableString(alloc.ShipmentID),
				alloc.GrossAmount, alloc.AdvanceAdjusted, alloc.Deductions, netPaid, alloc.Notes,
			)
			if err != nil {
				return fmt.Errorf("insert payment allocation: %w", err)
			}

			// If linked to a PO payment milestone, update milestone paid amount
			if alloc.POMilestoneID != "" {
				_, err = tx.Exec(`
					UPDATE po_payment_milestones
					SET paid_amount = paid_amount + ?,
					    status = CASE WHEN paid_amount + ? >= amount THEN 'PAID' ELSE 'PARTIALLY_PAID' END
					WHERE id = ?`, alloc.GrossAmount, alloc.GrossAmount, alloc.POMilestoneID)
				if err != nil {
					return fmt.Errorf("update milestone status: %w", err)
				}
			}

			// If linked to a PO, update PO status
			if alloc.POID != "" {
				var poTotal, totalPaid float64
				_ = tx.QueryRow(`
					SELECT po.total_amount, COALESCE(SUM(pm.paid_amount), 0)
					FROM purchase_orders po
					LEFT JOIN po_payment_milestones pm ON pm.po_id = po.id
					WHERE po.id = ? GROUP BY po.id`, alloc.POID).Scan(&poTotal, &totalPaid)

				newStatus := "PARTIALLY_PAID"
				if totalPaid >= poTotal && poTotal > 0 {
					newStatus = "FULLY_PAID"
				}
				_, _ = tx.Exec(`UPDATE purchase_orders SET status = ?, updated_at = ? WHERE id = ?`, newStatus, now, alloc.POID)
			}
		}

		return nil
	})
	if err != nil {
		return models.Payment{}, err
	}

	return r.GetPaymentByID(paymentID)
}

// GetPaymentByID fetches a complete payment voucher with its allocations.
func (r *PaymentPORepository) GetPaymentByID(id string) (models.Payment, error) {
	var p models.Payment
	query := `
		SELECT p.id, p.voucher_number, p.payment_date, p.supplier_id, COALESCE(sup.supplier_name, ''),
		       p.payment_type, p.payment_mode, p.currency, p.exchange_rate, p.total_amount,
		       p.bank_name, p.bank_account_no, p.reference_number, p.beneficiary_name,
		       p.beneficiary_bank, p.beneficiary_account, p.beneficiary_swift, p.beneficiary_iban,
		       p.status, COALESCE(p.notes, ''), COALESCE(p.created_by, ''), p.created_at, p.updated_at
		FROM payments p
		LEFT JOIN suppliers sup ON sup.id = p.supplier_id
		WHERE p.id = ? AND p.deleted_at IS NULL`

	err := r.db.QueryRow(query, id).Scan(
		&p.ID, &p.VoucherNumber, &p.PaymentDate, &p.SupplierID, &p.SupplierName,
		&p.PaymentType, &p.PaymentMode, &p.Currency, &p.ExchangeRate, &p.TotalAmount,
		&p.BankName, &p.BankAccountNo, &p.ReferenceNumber, &p.BeneficiaryName,
		&p.BeneficiaryBank, &p.BeneficiaryAccount, &p.BeneficiarySWIFT, &p.BeneficiaryIBAN,
		&p.Status, &p.Notes, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return models.Payment{}, fmt.Errorf("get payment by id: %w", err)
	}

	allocRows, err := r.db.Query(`
		SELECT pa.id, pa.payment_id, COALESCE(pa.po_id, ''), COALESCE(po.po_number, ''),
		       COALESCE(pa.po_milestone_id, ''), COALESCE(pm.milestone_name, ''),
		       COALESCE(pa.invoice_id, ''), COALESCE(s.invoice_number, ''),
		       COALESCE(pa.shipment_id, ''), pa.gross_amount, pa.advance_adjusted,
		       pa.deductions, pa.net_paid_amount, COALESCE(pa.notes, '')
		FROM payment_allocations pa
		LEFT JOIN purchase_orders po ON po.id = pa.po_id
		LEFT JOIN po_payment_milestones pm ON pm.id = pa.po_milestone_id
		LEFT JOIN invoices inv ON inv.id = pa.invoice_id
		LEFT JOIN shipments s ON s.id = inv.shipment_id OR s.id = pa.shipment_id
		WHERE pa.payment_id = ?`, id)
	if err != nil {
		return models.Payment{}, fmt.Errorf("get payment allocations: %w", err)
	}
	defer allocRows.Close()

	for allocRows.Next() {
		var a models.PaymentAllocation
		if err := allocRows.Scan(
			&a.ID, &a.PaymentID, &a.POID, &a.PONumber, &a.POMilestoneID, &a.MilestoneName,
			&a.InvoiceID, &a.InvoiceNumber, &a.ShipmentID, &a.GrossAmount, &a.AdvanceAdjusted,
			&a.Deductions, &a.NetPaidAmount, &a.Notes,
		); err != nil {
			return models.Payment{}, err
		}
		p.Allocations = append(p.Allocations, a)
	}

	return p, nil
}

// ListPayments returns paginated payment records.
func (r *PaymentPORepository) ListPayments(req models.PaymentPageRequest) (models.PaymentPage, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 25
	}

	where := []string{"p.deleted_at IS NULL"}
	var args []any

	if q := strings.TrimSpace(req.Search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(p.voucher_number LIKE ? OR p.reference_number LIKE ? OR sup.supplier_name LIKE ? OR p.notes LIKE ?)")
		args = append(args, like, like, like, like)
	}
	if s := strings.TrimSpace(req.SupplierID); s != "" {
		where = append(where, "p.supplier_id = ?")
		args = append(args, s)
	}
	if t := strings.TrimSpace(req.PaymentType); t != "" {
		where = append(where, "p.payment_type = ?")
		args = append(args, t)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		where = append(where, "p.status = ?")
		args = append(args, st)
	}

	whereClause := strings.Join(where, " AND ")

	var totalCount int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM payments p
		LEFT JOIN suppliers sup ON sup.id = p.supplier_id
		WHERE %s`, whereClause)
	if err := r.db.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
		return models.PaymentPage{}, fmt.Errorf("count payments: %w", err)
	}

	offset := (req.Page - 1) * req.PageSize
	query := fmt.Sprintf(`
		SELECT p.id, p.voucher_number, p.payment_date, p.supplier_id, COALESCE(sup.supplier_name, ''),
		       p.payment_type, p.payment_mode, p.currency, p.exchange_rate, p.total_amount,
		       p.bank_name, p.bank_account_no, p.reference_number, p.beneficiary_name,
		       p.beneficiary_bank, p.beneficiary_account, p.beneficiary_swift, p.beneficiary_iban,
		       p.status, COALESCE(p.notes, ''), COALESCE(p.created_by, ''), p.created_at, p.updated_at
		FROM payments p
		LEFT JOIN suppliers sup ON sup.id = p.supplier_id
		WHERE %s
		ORDER BY p.payment_date DESC, p.voucher_number DESC
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, req.PageSize, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return models.PaymentPage{}, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	var items []models.Payment
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(
			&p.ID, &p.VoucherNumber, &p.PaymentDate, &p.SupplierID, &p.SupplierName,
			&p.PaymentType, &p.PaymentMode, &p.Currency, &p.ExchangeRate, &p.TotalAmount,
			&p.BankName, &p.BankAccountNo, &p.ReferenceNumber, &p.BeneficiaryName,
			&p.BeneficiaryBank, &p.BeneficiaryAccount, &p.BeneficiarySWIFT, &p.BeneficiaryIBAN,
			&p.Status, &p.Notes, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return models.PaymentPage{}, err
		}
		items = append(items, p)
	}

	return models.PaymentPage{
		Items:      items,
		TotalCount: totalCount,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

// ListInvoicePaymentCandidates returns outstanding commercial invoices ready for payment allocation.
func (r *PaymentPORepository) ListInvoicePaymentCandidates(supplierID string) ([]models.InvoicePaymentCandidate, error) {
	where := []string{"inv.deleted_at IS NULL", "s.deleted_at IS NULL"}
	var args []any

	if s := strings.TrimSpace(supplierID); s != "" {
		where = append(where, "s.supplier_id = ?")
		args = append(args, s)
	}

	whereClause := strings.Join(where, " AND ")
	query := fmt.Sprintf(`
		SELECT inv.id, s.id, s.supplier_id, COALESCE(sup.supplier_name, ''),
		       s.invoice_number, s.invoice_date, s.invoice_currency,
		       COALESCE((
		           SELECT SUM(ROUND(ili.quantity * ili.unit_price, inv.line_total_decimals))
		           FROM invoice_line_items ili
		           WHERE ili.invoice_id = inv.id AND ili.deleted_at IS NULL
		       ), s.invoice_value) AS gross_amount,
		       COALESCE((
		           SELECT SUM(pa.gross_amount)
		           FROM payment_allocations pa
		           JOIN payments p ON p.id = pa.payment_id
		           WHERE pa.invoice_id = inv.id AND p.deleted_at IS NULL
		       ), 0.0) AS already_paid,
		       COALESCE((
		           SELECT SUM(pa.advance_adjusted)
		           FROM payment_allocations pa
		           JOIN payments p ON p.id = pa.payment_id
		           WHERE pa.invoice_id = inv.id AND p.deleted_at IS NULL
		       ), 0.0) AS advance_adjusted,
		       COALESCE(po.po_number, ''),
		       COALESCE((
		           SELECT SUM(pm.paid_amount)
		           FROM po_payment_milestones pm
		           WHERE pm.po_id = s.po_id AND pm.stage_type = 'ADVANCE'
		       ), 0.0) AS available_advance
		FROM invoices inv
		JOIN shipments s ON s.id = inv.shipment_id
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		LEFT JOIN purchase_orders po ON po.id = s.po_id
		WHERE %s
		ORDER BY s.invoice_date ASC`, whereClause)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list invoice candidates: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	var candidates []models.InvoicePaymentCandidate

	for rows.Next() {
		var c models.InvoicePaymentCandidate
		if err := rows.Scan(
			&c.InvoiceID, &c.ShipmentID, &c.SupplierID, &c.SupplierName,
			&c.InvoiceNumber, &c.InvoiceDate, &c.Currency, &c.GrossAmount,
			&c.AlreadyPaid, &c.AdvanceAdjusted, &c.LinkedPONumber, &c.AvailablePOAdvance,
		); err != nil {
			return nil, err
		}

		c.PendingBalance = math.Max(0, c.GrossAmount-c.AlreadyPaid)
		if c.PendingBalance <= 0.01 {
			continue // Already fully paid
		}

		// Calculate payment term & due date (default Net 30 days from invoice date)
		c.PaymentTerm = "Net 30 Days"
		if invTime, pErr := time.Parse("2006-01-02", c.InvoiceDate); pErr == nil {
			due := invTime.AddDate(0, 0, 30)
			c.DueDate = due.Format("2006-01-02")
			diff := int(now.Sub(due).Hours() / 24)
			if diff > 0 {
				c.DaysOverdue = diff
				c.IsOverdue = true
			}
		}

		candidates = append(candidates, c)
	}

	return candidates, nil
}

// GetAdvanceReconciliation calculates variance and matching between PO advance payments and commercial invoices.
func (r *PaymentPORepository) GetAdvanceReconciliation(supplierID string) ([]models.AdvanceReconciliationSummary, error) {
	where := []string{"po.deleted_at IS NULL"}
	var args []any

	if s := strings.TrimSpace(supplierID); s != "" {
		where = append(where, "po.supplier_id = ?")
		args = append(args, s)
	}

	whereClause := strings.Join(where, " AND ")
	query := fmt.Sprintf(`
		SELECT po.id, po.po_number, po.po_date, po.supplier_id, COALESCE(sup.supplier_name, ''),
		       po.currency, po.total_amount,
		       COALESCE((
		           SELECT SUM(pm.amount) FROM po_payment_milestones pm
		           WHERE pm.po_id = po.id AND pm.stage_type = 'ADVANCE'
		       ), 0.0) AS adv_required,
		       COALESCE((
		           SELECT SUM(pm.paid_amount) FROM po_payment_milestones pm
		           WHERE pm.po_id = po.id AND pm.stage_type = 'ADVANCE'
		       ), 0.0) AS adv_paid
		FROM purchase_orders po
		LEFT JOIN suppliers sup ON sup.id = po.supplier_id
		WHERE %s
		ORDER BY po.po_date DESC`, whereClause)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("reconciliation query: %w", err)
	}
	defer rows.Close()

	var summaries []models.AdvanceReconciliationSummary

	for rows.Next() {
		var s models.AdvanceReconciliationSummary
		if err := rows.Scan(
			&s.POID, &s.PONumber, &s.PODate, &s.SupplierID, &s.SupplierName,
			&s.Currency, &s.POTotalAmount, &s.AdvanceRequired, &s.AdvancePaidAmount,
		); err != nil {
			return nil, err
		}
		s.AdvanceBalance = math.Max(0, s.AdvanceRequired-s.AdvancePaidAmount)
		summaries = append(summaries, s)
	}
	rows.Close()

	// Fetch linked invoices sequentially for each PO after closing the outer query
	for i := range summaries {
		s := &summaries[i]
		invRows, iErr := r.db.Query(`
			SELECT inv.id, s.id, s.invoice_number, s.invoice_date, s.invoice_value, s.invoice_currency,
			       COALESCE((
			           SELECT SUM(pa.advance_adjusted)
			           FROM payment_allocations pa
			           WHERE pa.invoice_id = inv.id
			       ), 0.0) AS adv_alloc,
			       COALESCE((
			           SELECT SUM(pa.gross_amount)
			           FROM payment_allocations pa
			           WHERE pa.invoice_id = inv.id
			       ), 0.0) AS direct_paid
			FROM invoices inv
			JOIN shipments s ON s.id = inv.shipment_id
			WHERE (s.po_id = ? OR inv.po_id = ?) AND inv.deleted_at IS NULL AND s.deleted_at IS NULL
			ORDER BY s.invoice_date ASC`, s.POID, s.POID)
		if iErr == nil {
			for invRows.Next() {
				var item models.ReconciledInvoiceItem
				if err := invRows.Scan(
					&item.InvoiceID, &item.ShipmentID, &item.InvoiceNumber, &item.InvoiceDate,
					&item.InvoiceValue, &item.Currency, &item.AdvanceAllocated, &item.DirectPaid,
				); err == nil {
					item.BalanceRemaining = math.Max(0, item.InvoiceValue-item.DirectPaid-item.AdvanceAllocated)
					if item.BalanceRemaining <= 0.01 {
						item.Status = "SETTLED"
					} else {
						item.Status = "PENDING_BALANCE"
					}
					s.Invoices = append(s.Invoices, item)
					s.TotalInvoicedAmount += item.InvoiceValue
					s.AdvanceAdjustedAmount += item.AdvanceAllocated
				}
			}
			invRows.Close()
		}

		s.UnadjustedAdvance = math.Max(0, s.AdvancePaidAmount-s.AdvanceAdjustedAmount)

		// Calculate variance: AdvancePaid - Invoiced
		s.VarianceAmount = math.Round((s.AdvancePaidAmount-s.TotalInvoicedAmount)*100) / 100

		if len(s.Invoices) == 0 {
			s.VarianceType = "AWAITING_INVOICE"
			s.ReconciliationStatus = "PENDING_INVOICE"
		} else if math.Abs(s.VarianceAmount) <= 0.01 {
			s.VarianceType = "MATCHED"
			s.ReconciliationStatus = "FULLY_RECONCILED"
		} else if s.VarianceAmount > 0 {
			// Advance Paid > Invoice Amount
			s.VarianceType = "UNDER_INVOICED"
			s.ReconciliationStatus = "DISCREPANCY"
		} else {
			// Invoice Amount > Advance Paid
			s.VarianceType = "OVER_INVOICED"
			s.ReconciliationStatus = "PARTIALLY_RECONCILED"
		}
	}

	return summaries, nil
}

// LinkInvoiceToPO connects an invoice and its shipment to a purchase order.
func (r *PaymentPORepository) LinkInvoiceToPO(invoiceID, poID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		var shipmentID string
		err := tx.QueryRow(`SELECT shipment_id FROM invoices WHERE id = ?`, invoiceID).Scan(&shipmentID)
		if err != nil {
			return fmt.Errorf("find invoice: %w", err)
		}

		_, err = tx.Exec(`UPDATE invoices SET po_id = ? WHERE id = ?`, nullableString(poID), invoiceID)
		if err != nil {
			return fmt.Errorf("update invoice po_id: %w", err)
		}

		_, err = tx.Exec(`UPDATE shipments SET po_id = ? WHERE id = ?`, nullableString(poID), shipmentID)
		if err != nil {
			return fmt.Errorf("update shipment po_id: %w", err)
		}

		// Also check if any line items match
		_, _ = tx.Exec(`
			UPDATE invoice_line_items
			SET po_line_item_id = (
				SELECT pli.id FROM po_line_items pli
				WHERE pli.po_id = ? AND (pli.item_id = invoice_line_items.item_id OR pli.part_number = (
					SELECT part_number FROM items WHERE items.id = invoice_line_items.item_id
				))
				LIMIT 1
			)
			WHERE invoice_id = ? AND po_line_item_id IS NULL`, poID, invoiceID)

		_ = now
		return nil
	})
}

// GetMaterialTracking returns line-item advance-paid vs received quantity tracking.
func (r *PaymentPORepository) GetMaterialTracking(supplierID, search string) ([]models.MaterialTrackingItem, models.MaterialTrackingSummary, error) {
	where := []string{"po.deleted_at IS NULL", "pli.deleted_at IS NULL"}
	var args []any

	if s := strings.TrimSpace(supplierID); s != "" {
		where = append(where, "po.supplier_id = ?")
		args = append(args, s)
	}
	if q := strings.TrimSpace(search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(pli.part_number LIKE ? OR pli.item_description LIKE ? OR po.po_number LIKE ?)")
		args = append(args, like, like, like)
	}

	whereClause := strings.Join(where, " AND ")
	query := fmt.Sprintf(`
		SELECT pli.id, po.id, po.po_number, po.po_date, po.supplier_id, COALESCE(sup.supplier_name, ''),
		       po.currency, pli.part_number, pli.item_description, pli.unit, pli.unit_price,
		       pli.quantity, pli.line_total, pli.received_quantity,
		       COALESCE(po.expected_delivery_date, ''),
		       po.total_amount,
		       COALESCE((
		           SELECT SUM(pm.paid_amount) FROM po_payment_milestones pm
		           WHERE pm.po_id = po.id AND pm.stage_type = 'ADVANCE'
		       ), 0.0) AS adv_paid
		FROM po_line_items pli
		JOIN purchase_orders po ON po.id = pli.po_id
		LEFT JOIN suppliers sup ON sup.id = po.supplier_id
		WHERE %s
		ORDER BY po.po_date DESC, pli.part_number ASC`, whereClause)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, models.MaterialTrackingSummary{}, fmt.Errorf("material tracking query: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	var items []models.MaterialTrackingItem
	var summary models.MaterialTrackingSummary
	poMap := make(map[string]bool)

	for rows.Next() {
		var (
			pliID, poID, poNum, poDate, supID, supName, curr, partNum, desc, unit, expDate string
			uPrice, qty, lTotal, recQty, poTotal, advPaid                                  float64
		)
		if err := rows.Scan(
			&pliID, &poID, &poNum, &poDate, &supID, &supName, &curr,
			&partNum, &desc, &unit, &uPrice, &qty, &lTotal, &recQty,
			&expDate, &poTotal, &advPaid,
		); err != nil {
			return nil, summary, err
		}

		poMap[poID] = true
		summary.TotalItemsTracked++

		advPct := 0.0
		if poTotal > 0 {
			advPct = math.Min(100.0, (advPaid/poTotal)*100.0)
		}

		// Proportionate advance-paid quantity
		advPaidQty := math.Round((qty*(advPct/100.0))*100) / 100
		pendingQty := math.Max(0, qty-recQty)
		advPendingQty := math.Max(0, advPaidQty-recQty)
		advLockedCapital := math.Round((advPendingQty*uPrice)*100) / 100

		summary.TotalPendingQuantity += pendingQty
		summary.TotalAdvancePendingQuantity += advPendingQty
		summary.TotalAdvanceCapitalLocked += advLockedCapital

		fulfillment := "AWAITING_SHIPMENT"
		if recQty >= qty {
			fulfillment = "FULLY_RECEIVED"
		} else if recQty > 0 {
			fulfillment = "PARTIALLY_RECEIVED"
		} else if recQty > qty {
			fulfillment = "OVER_SUPPLIED"
		}

		daysDelayed := 0
		isDelayed := false
		if expDate != "" {
			if t, pErr := time.Parse("2006-01-02", expDate); pErr == nil {
				if now.After(t) && pendingQty > 0 {
					daysDelayed = int(now.Sub(t).Hours() / 24)
					isDelayed = true
					summary.DelayedItemsCount++
				}
			}
		}

		item := models.MaterialTrackingItem{
			POLineItemID:               pliID,
			POID:                       poID,
			PONumber:                   poNum,
			PODate:                     poDate,
			SupplierID:                 supID,
			SupplierName:               supName,
			Currency:                   curr,
			PartNumber:                 partNum,
			ItemDescription:            desc,
			Unit:                       unit,
			UnitPrice:                  uPrice,
			OrderedQuantity:            qty,
			OrderedAmount:              lTotal,
			AdvancePaidPercentage:      math.Round(advPct*10) / 10,
			AdvancePaidQuantity:        advPaidQty,
			ReceivedQuantity:           recQty,
			PendingQuantity:            pendingQty,
			AdvancePaidPendingQuantity: advPendingQty,
			AdvanceLockedCapital:       advLockedCapital,
			FulfillmentStatus:          fulfillment,
			ExpectedDeliveryDate:       expDate,
			DaysDelayed:                daysDelayed,
			IsDelayed:                  isDelayed,
		}

		items = append(items, item)
	}

	summary.TotalPOTracked = len(poMap)
	return items, summary, nil
}

// RecordMaterialReceipt logs inward material against a PO line item and increments received quantity.
func (r *PaymentPORepository) RecordMaterialReceipt(input models.MaterialReceiptInput) error {
	if strings.TrimSpace(input.POLineItemID) == "" {
		return errors.New("PO line item is required")
	}
	if input.QuantityReceived <= 0 {
		return errors.New("received quantity must be greater than zero")
	}
	if input.ReceiptDate == "" {
		input.ReceiptDate = time.Now().UTC().Format("2006-01-02")
	}

	receiptID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	return transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO material_receipts (
				id, po_line_item_id, invoice_line_item_id, shipment_id,
				receipt_date, quantity_received, remarks, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			receiptID, input.POLineItemID, nullableString(input.InvoiceLineItemID),
			nullableString(input.ShipmentID), input.ReceiptDate, input.QuantityReceived,
			input.Remarks, now,
		)
		if err != nil {
			return fmt.Errorf("insert material receipt: %w", err)
		}

		_, err = tx.Exec(`
			UPDATE po_line_items
			SET received_quantity = received_quantity + ?
			WHERE id = ?`, input.QuantityReceived, input.POLineItemID)
		if err != nil {
			return fmt.Errorf("update po line item received quantity: %w", err)
		}

		return nil
	})
}

// GeneratePaymentAnnexure produces a structured formal Payment Annexure for banking or audit.
func (r *PaymentPORepository) GeneratePaymentAnnexure(paymentID, companyName string) (models.PaymentAnnexure, error) {
	payment, err := r.GetPaymentByID(paymentID)
	if err != nil {
		return models.PaymentAnnexure{}, err
	}

	if companyName == "" {
		companyName = "Global Import Logistics Ltd."
	}

	// Fetch supplier details
	var supAddr, supCountry string
	_ = r.db.QueryRow(`
		SELECT COALESCE(bank_address, ''), COALESCE(country, '')
		FROM suppliers WHERE id = ?`, payment.SupplierID).Scan(&supAddr, &supCountry)

	annexureNum := fmt.Sprintf("ANNX-%s", payment.VoucherNumber)
	now := time.Now().Format("2006-01-02")

	var annexureItems []models.PaymentAnnexureItem
	var totalGross, totalAdvAdj, totalDeduct, totalNet float64

	for idx, alloc := range payment.Allocations {
		docType := "Commercial Invoice"
		docNum := alloc.InvoiceNumber
		term := "Net Terms"

		if alloc.POMilestoneID != "" || alloc.POID != "" {
			docType = "PO Advance"
			docNum = alloc.PONumber
			term = alloc.MilestoneName
		}

		item := models.PaymentAnnexureItem{
			SlNo:                 idx + 1,
			DocumentType:         docType,
			DocumentNumber:       docNum,
			DocumentDate:         payment.PaymentDate,
			GrossAmount:          alloc.GrossAmount,
			AdvanceAdjusted:      alloc.AdvanceAdjusted,
			Deductions:           alloc.Deductions,
			NetAmount:            alloc.NetPaidAmount,
			PaymentTermMilestone: term,
			Remarks:              alloc.Notes,
		}

		totalGross += alloc.GrossAmount
		totalAdvAdj += alloc.AdvanceAdjusted
		totalDeduct += alloc.Deductions
		totalNet += alloc.NetPaidAmount

		annexureItems = append(annexureItems, item)
	}

	return models.PaymentAnnexure{
		AnnexureNumber:       annexureNum,
		PaymentVoucherNumber: payment.VoucherNumber,
		GeneratedDate:        now,
		PaymentDate:          payment.PaymentDate,
		PaymentMode:          payment.PaymentMode,
		RemitterCompany:      companyName,
		RemitterBank:         payment.BankName,
		RemitterAccount:      payment.BankAccountNo,
		ReferenceNumber:      payment.ReferenceNumber,
		SupplierID:           payment.SupplierID,
		SupplierName:         payment.SupplierName,
		SupplierAddress:      supAddr,
		SupplierCountry:      supCountry,
		BeneficiaryName:      payment.BeneficiaryName,
		BeneficiaryBank:      payment.BeneficiaryBank,
		BeneficiaryAccount:   payment.BeneficiaryAccount,
		BeneficiarySWIFT:     payment.BeneficiarySWIFT,
		BeneficiaryIBAN:      payment.BeneficiaryIBAN,
		Currency:             payment.Currency,
		ExchangeRate:         payment.ExchangeRate,
		Items:                annexureItems,
		TotalGrossAmount:     math.Round(totalGross*100) / 100,
		TotalAdvanceAdjusted: math.Round(totalAdvAdj*100) / 100,
		TotalDeductions:      math.Round(totalDeduct*100) / 100,
		TotalNetAmount:       math.Round(totalNet*100) / 100,
		NetAmountInWords:     formatAmountInWords(totalNet, payment.Currency),
		Notes:                payment.Notes,
		PreparedBy:           payment.CreatedBy,
		CheckedBy:            "Finance Reviewer",
		ApprovedBy:           "Authorized Signatory",
	}, nil
}

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// formatAmountInWords formats a currency amount into English words.
func formatAmountInWords(amount float64, currency string) string {
	intPart := int64(math.Floor(amount))
	decPart := int64(math.Round((amount - math.Floor(amount)) * 100))

	words := numberToWords(intPart)
	if currency == "" {
		currency = "USD"
	}

	res := fmt.Sprintf("%s %s", currency, words)
	if decPart > 0 {
		res += fmt.Sprintf(" and %d/100 Cents", decPart)
	}
	return res + " Only"
}

func numberToWords(n int64) string {
	if n == 0 {
		return "Zero"
	}

	units := []string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine",
		"Ten", "Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen", "Nineteen"}
	tens := []string{"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety"}

	var convertChunk func(num int64) string
	convertChunk = func(num int64) string {
		var out string
		if num >= 100 {
			out += units[num/100] + " Hundred "
			num %= 100
		}
		if num >= 20 {
			out += tens[num/10] + " "
			num %= 10
		}
		if num > 0 {
			out += units[num] + " "
		}
		return strings.TrimSpace(out)
	}

	scales := []string{"", "Thousand", "Million", "Billion"}
	var parts []string
	scaleIdx := 0

	for n > 0 {
		chunk := n % 1000
		if chunk > 0 {
			chunkStr := convertChunk(chunk)
			if scales[scaleIdx] != "" {
				chunkStr += " " + scales[scaleIdx]
			}
			parts = append([]string{chunkStr}, parts...)
		}
		n /= 1000
		scaleIdx++
	}

	return strings.Join(parts, " ")
}
