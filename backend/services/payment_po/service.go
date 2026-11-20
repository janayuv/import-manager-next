package payment_po

import (
	"database/sql"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service provides Wails-bound APIs for Purchase Orders, staged milestones, payment vouchers,
// multi-invoice batch allocation, advance reconciliation, material tracking, and payment annexures.
type Service struct {
	db    *sql.DB
	repo  *repositories.PaymentPORepository
	sec   *security.Service
	audit *audit.Service
}

// NewService constructs a payment and PO Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{
		db:    db,
		repo:  repositories.NewPaymentPORepository(db),
		sec:   sec,
		audit: auditSvc,
	}
}

// CreatePurchaseOrder validates, creates a PO with line items and milestones, and logs an audit record.
func (s *Service) CreatePurchaseOrder(input models.PurchaseOrderInput) (models.PurchaseOrder, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.PurchaseOrder{}, err
	}

	po, err := s.repo.CreatePurchaseOrder(input)
	if err != nil {
		return models.PurchaseOrder{}, err
	}

	_ = s.audit.RecordTableAudit("purchase_orders", po.ID, "INSERT", sess.UserID, nil, po, nil)
	_ = s.audit.RecordUserActivity(sess.UserID, "purchase_order.create", "purchase_orders", po.ID, "SUCCESS", "INFO", map[string]any{
		"poNumber":    po.PONumber,
		"supplierId":  po.SupplierID,
		"totalAmount": po.TotalAmount,
		"currency":    po.Currency,
		"preset":      po.PaymentTermsPreset,
	})

	return po, nil
}

// GetPurchaseOrder retrieves a complete purchase order dossier.
func (s *Service) GetPurchaseOrder(id string) (models.PurchaseOrder, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.PurchaseOrder{}, err
	}
	return s.repo.GetPurchaseOrderByID(id)
}

// ListPurchaseOrders lists paginated purchase orders with search and filters.
func (s *Service) ListPurchaseOrders(req models.PurchaseOrderPageRequest) (models.PurchaseOrderPage, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.PurchaseOrderPage{}, err
	}
	return s.repo.ListPurchaseOrders(req)
}

// DeletePurchaseOrder soft deletes a PO if no payments exist.
func (s *Service) DeletePurchaseOrder(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}

	if err := s.repo.DeletePurchaseOrder(id, sess.Username); err != nil {
		return err
	}

	_ = s.audit.RecordTableAudit("purchase_orders", id, "DELETE", sess.UserID, nil, nil, map[string]any{"softDelete": true})
	_ = s.audit.RecordUserActivity(sess.UserID, "purchase_order.delete", "purchase_orders", id, "SUCCESS", "INFO", map[string]any{
		"poId": id,
	})

	return nil
}

// CreatePayment processes a new payment voucher, records allocations, and updates milestones.
func (s *Service) CreatePayment(input models.PaymentInput) (models.Payment, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Payment{}, err
	}

	pay, err := s.repo.CreatePayment(input, sess.Username)
	if err != nil {
		return models.Payment{}, err
	}

	_ = s.audit.RecordTableAudit("payments", pay.ID, "INSERT", sess.UserID, nil, pay, nil)
	_ = s.audit.RecordUserActivity(sess.UserID, "payment.create", "payments", pay.ID, "SUCCESS", "INFO", map[string]any{
		"voucherNumber": pay.VoucherNumber,
		"supplierId":    pay.SupplierID,
		"paymentType":   pay.PaymentType,
		"totalAmount":   pay.TotalAmount,
		"reference":     pay.ReferenceNumber,
	})

	return pay, nil
}

// GetPayment fetches a payment voucher with itemized allocations.
func (s *Service) GetPayment(id string) (models.Payment, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.Payment{}, err
	}
	return s.repo.GetPaymentByID(id)
}

// ListPayments returns paginated payment records.
func (s *Service) ListPayments(req models.PaymentPageRequest) (models.PaymentPage, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.PaymentPage{}, err
	}
	return s.repo.ListPayments(req)
}

// ListInvoicePaymentCandidates returns outstanding commercial invoices ready for fund distribution.
func (s *Service) ListInvoicePaymentCandidates(supplierID string) ([]models.InvoicePaymentCandidate, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.repo.ListInvoicePaymentCandidates(supplierID)
}

// GetAdvanceReconciliation compares advance payments against commercial invoices with variance metrics.
func (s *Service) GetAdvanceReconciliation(supplierID string) ([]models.AdvanceReconciliationSummary, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.repo.GetAdvanceReconciliation(supplierID)
}

// LinkInvoiceToPO establishes a cross-reference between an invoice and a purchase order.
func (s *Service) LinkInvoiceToPO(invoiceID, poID string) error {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return err
	}

	if err := s.repo.LinkInvoiceToPO(invoiceID, poID); err != nil {
		return err
	}

	_ = s.audit.RecordUserActivity(sess.UserID, "invoice.link_po", "invoices", invoiceID, "SUCCESS", "INFO", map[string]any{
		"invoiceId": invoiceID,
		"poId":      poID,
	})

	return nil
}

// GetMaterialTracking returns ordered vs advance-paid vs received quantity tracking for materials.
func (s *Service) GetMaterialTracking(supplierID, search string) ([]models.MaterialTrackingItem, models.MaterialTrackingSummary, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, models.MaterialTrackingSummary{}, err
	}
	return s.repo.GetMaterialTracking(supplierID, search)
}

// RecordMaterialReceipt logs inward material delivery against a PO line item.
func (s *Service) RecordMaterialReceipt(input models.MaterialReceiptInput) error {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return err
	}

	if err := s.repo.RecordMaterialReceipt(input); err != nil {
		return err
	}

	_ = s.audit.RecordUserActivity(sess.UserID, "material_receipt.create", "material_receipts", input.POLineItemID, "SUCCESS", "INFO", map[string]any{
		"poLineItemId": input.POLineItemID,
		"qtyReceived":  input.QuantityReceived,
		"receiptDate":  input.ReceiptDate,
	})

	return nil
}

// GeneratePaymentAnnexure compiles a formal payment annexure document.
func (s *Service) GeneratePaymentAnnexure(paymentID, companyName string) (models.PaymentAnnexure, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.PaymentAnnexure{}, err
	}
	return s.repo.GeneratePaymentAnnexure(paymentID, companyName)
}
