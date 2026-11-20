package payment_po_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/payment_po"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func setupTestService(t *testing.T) (*payment_po.Service, *connection.Manager) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_erp.db")
	mgr, err := connection.OpenEncrypted(dbPath, "test-key-po-01")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	_, err = migrations.Run(context.Background(), mgr.DB(), "1.0.0", nil)
	if err != nil {
		mgr.Close()
		t.Fatalf("run migrations: %v", err)
	}

	auditSvc := audit.NewService(mgr.DB())
	secSvc := security.NewService(mgr.DB(), false, auditSvc)
	if _, err := secSvc.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		mgr.Close()
		t.Fatalf("admin setup: %v", err)
	}

	svc := payment_po.NewService(mgr.DB(), secSvc, auditSvc)
	return svc, mgr
}

func TestPurchaseOrderCreationAndStagedMilestones(t *testing.T) {
	svc, mgr := setupTestService(t)
	defer mgr.Close()

	var supID string
	err := mgr.DB().QueryRow(`SELECT id FROM suppliers LIMIT 1`).Scan(&supID)
	if err != nil {
		t.Fatalf("query supplier: %v", err)
	}

	// 1. Create a PO with 40-40-20 terms
	poInput := models.PurchaseOrderInput{
		PONumber:           "PO-2026-TEST-001",
		PODate:             "2026-11-20",
		SupplierID:         supID,
		Currency:           "USD",
		ExchangeRate:       85.50,
		Incoterm:           "FOB",
		PaymentTermsPreset: "40_40_20",
		PaymentTermsDesc:   "40% Advance, 40% on BL, 20% on Delivery",
		LineItems: []models.POLineItemInput{
			{
				PartNumber:      "COMP-X100",
				ItemDescription: "Industrial Microcontroller Unit",
				Quantity:        100,
				Unit:            "NOS",
				UnitPrice:       50.0,
			},
		},
	}

	po, err := svc.CreatePurchaseOrder(poInput)
	if err != nil {
		t.Fatalf("create purchase order: %v", err)
	}

	if po.TotalAmount != 5000.0 {
		t.Fatalf("expected total amount 5000.0, got %.2f", po.TotalAmount)
	}
	if len(po.Milestones) != 3 {
		t.Fatalf("expected 3 milestones for 40_40_20, got %d", len(po.Milestones))
	}
	if po.Milestones[0].Percentage != 40.0 || po.Milestones[0].Amount != 2000.0 {
		t.Fatalf("expected milestone 1 to be 40%% ($2000), got %.1f%% ($%.2f)", po.Milestones[0].Percentage, po.Milestones[0].Amount)
	}
	if po.Milestones[1].Percentage != 40.0 || po.Milestones[1].Amount != 2000.0 {
		t.Fatalf("expected milestone 2 to be 40%% ($2000), got %.1f%% ($%.2f)", po.Milestones[1].Percentage, po.Milestones[1].Amount)
	}
	if po.Milestones[2].Percentage != 20.0 || po.Milestones[2].Amount != 1000.0 {
		t.Fatalf("expected milestone 3 to be 20%% ($1000), got %.1f%% ($%.2f)", po.Milestones[2].Percentage, po.Milestones[2].Amount)
	}

	// 2. Process Advance Payment for Milestone 1 ($2,000)
	ms1 := po.Milestones[0]
	payInput := models.PaymentInput{
		PaymentDate:     "2026-11-20",
		SupplierID:      po.SupplierID,
		PaymentType:     "PO_ADVANCE",
		PaymentMode:     "WIRE_TRANSFER",
		Currency:        "USD",
		ExchangeRate:    85.50,
		TotalAmount:     2000.0,
		BankName:        "Citibank NA",
		BankAccountNo:   "ACC-987654321",
		ReferenceNumber: "UTR-CITI-20261120-001",
		Allocations: []models.PaymentAllocationInput{
			{
				POID:          po.ID,
				POMilestoneID: ms1.ID,
				GrossAmount:   2000.0,
				Notes:         "40% Advance on order confirmation",
			},
		},
	}

	pay, err := svc.CreatePayment(payInput)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if pay.TotalAmount != 2000.0 {
		t.Fatalf("expected payment total 2000.0, got %.2f", pay.TotalAmount)
	}

	// 3. Verify PO updated balances
	poUpdated, err := svc.GetPurchaseOrder(po.ID)
	if err != nil {
		t.Fatalf("get po: %v", err)
	}
	if poUpdated.AdvancePaid != 2000.0 {
		t.Fatalf("expected advance paid 2000.0, got %.2f", poUpdated.AdvancePaid)
	}
	if poUpdated.AdvanceBalance != 0.0 {
		t.Fatalf("expected advance balance 0.0, got %.2f", poUpdated.AdvanceBalance)
	}
	if poUpdated.TotalBalance != 3000.0 {
		t.Fatalf("expected total balance 3000.0, got %.2f", poUpdated.TotalBalance)
	}

	// 4. Test Material Tracking:
	// Total ordered: 100 nos. Advance % = 40% (2000/5000).
	// Advance-covered quantity = 40 nos.
	matItems, summary, err := svc.GetMaterialTracking(po.SupplierID, "")
	if err != nil {
		t.Fatalf("get material tracking: %v", err)
	}
	if len(matItems) == 0 {
		t.Fatalf("expected tracked material items")
	}
	item := matItems[0]
	if item.OrderedQuantity != 100.0 {
		t.Fatalf("expected ordered qty 100, got %.2f", item.OrderedQuantity)
	}
	if item.AdvancePaidQuantity != 40.0 {
		t.Fatalf("expected advance-paid qty 40.0, got %.2f", item.AdvancePaidQuantity)
	}
	if item.AdvanceLockedCapital != 2000.0 { // 40 nos * $50
		t.Fatalf("expected locked capital $2000.0, got %.2f", item.AdvanceLockedCapital)
	}
	if summary.TotalAdvanceCapitalLocked != 2000.0 {
		t.Fatalf("expected total locked capital $2000.0, got %.2f", summary.TotalAdvanceCapitalLocked)
	}

	// 5. Record partial receipt of 30 nos
	err = svc.RecordMaterialReceipt(models.MaterialReceiptInput{
		POLineItemID:     item.POLineItemID,
		ReceiptDate:      "2026-11-22",
		QuantityReceived: 30.0,
		Remarks:          "Batch 1 shipment received (30 units)",
	})
	if err != nil {
		t.Fatalf("record material receipt: %v", err)
	}

	matItemsAfter, _, err := svc.GetMaterialTracking(po.SupplierID, "")
	if err != nil {
		t.Fatalf("get material tracking after receipt: %v", err)
	}
	itemAfter := matItemsAfter[0]
	if itemAfter.ReceivedQuantity != 30.0 {
		t.Fatalf("expected received qty 30.0, got %.2f", itemAfter.ReceivedQuantity)
	}
	if itemAfter.PendingQuantity != 70.0 {
		t.Fatalf("expected pending qty 70.0, got %.2f", itemAfter.PendingQuantity)
	}
	if itemAfter.AdvancePaidPendingQuantity != 10.0 { // 40 paid - 30 received = 10 paid still not received
		t.Fatalf("expected advance-paid pending qty 10.0, got %.2f", itemAfter.AdvancePaidPendingQuantity)
	}
	if itemAfter.AdvanceLockedCapital != 500.0 { // 10 nos * $50 = $500
		t.Fatalf("expected locked capital $500.0, got %.2f", itemAfter.AdvanceLockedCapital)
	}

	// 6. Test Advance vs Invoice Reconciliation
	recon, err := svc.GetAdvanceReconciliation(po.SupplierID)
	if err != nil {
		t.Fatalf("get advance reconciliation: %v", err)
	}
	if len(recon) == 0 {
		t.Fatalf("expected reconciliation records")
	}
	r0 := recon[0]
	if r0.AdvancePaidAmount != 2000.0 {
		t.Fatalf("expected advance paid $2000, got %.2f", r0.AdvancePaidAmount)
	}
	if r0.VarianceType != "AWAITING_INVOICE" {
		t.Fatalf("expected awaiting invoice variance, got %s", r0.VarianceType)
	}

	// 7. Test Payment Annexure Generation
	annexure, err := svc.GeneratePaymentAnnexure(pay.ID, "Acme Imports Corp")
	if err != nil {
		t.Fatalf("generate payment annexure: %v", err)
	}
	if annexure.TotalNetAmount != 2000.0 {
		t.Fatalf("expected annexure net amount 2000.0, got %.2f", annexure.TotalNetAmount)
	}
	if len(annexure.Items) != 1 {
		t.Fatalf("expected 1 annexure item, got %d", len(annexure.Items))
	}
	if annexure.RemitterBank != "Citibank NA" {
		t.Fatalf("expected remitter bank Citibank NA, got %s", annexure.RemitterBank)
	}
	if annexure.NetAmountInWords == "" {
		t.Fatalf("expected net amount in words to be populated")
	}
}


func TestMultiInvoicePaymentAndAdvanceReconciliation(t *testing.T) {
	svc, mgr := setupTestService(t)
	defer mgr.Close()

	var supID string
	err := mgr.DB().QueryRow(`SELECT id FROM suppliers LIMIT 1`).Scan(&supID)
	if err != nil {
		t.Fatalf("query supplier: %v", err)
	}

	// 1. Create a PO with 100% advance
	po, err := svc.CreatePurchaseOrder(models.PurchaseOrderInput{
		PONumber:           "PO-100-ADV-001",
		PODate:             "2026-11-20",
		SupplierID:         supID,
		Currency:           "USD",
		PaymentTermsPreset: "100_ADVANCE",
		LineItems: []models.POLineItemInput{
			{
				PartNumber:      "P-100",
				ItemDescription: "Standard Parts",
				Quantity:        200,
				Unit:            "NOS",
				UnitPrice:       25.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("create 100%% advance po: %v", err)
	}

	// 2. Pay 100% advance ($5000)
	pay, err := svc.CreatePayment(models.PaymentInput{
		PaymentDate:     "2026-11-20",
		SupplierID:      supID,
		PaymentType:     "PO_ADVANCE",
		PaymentMode:     "TT",
		Currency:        "USD",
		TotalAmount:     5000.0,
		BankName:        "HSBC",
		ReferenceNumber: "HSBC-ADV-001",
		Allocations: []models.PaymentAllocationInput{
			{
				POID:          po.ID,
				POMilestoneID: po.Milestones[0].ID,
				GrossAmount:   5000.0,
				Notes:         "Full 100% advance",
			},
		},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if pay.TotalAmount != 5000.0 {
		t.Fatalf("expected pay total 5000, got %.2f", pay.TotalAmount)
	}

	// 3. Insert a shipment and invoice for this supplier
	shipmentID := "ship-test-recon-01"
	invoiceID := "inv-test-recon-01"
	_, err = mgr.DB().Exec(`
		INSERT INTO shipments (
			id, supplier_id, invoice_number, invoice_date, goods_category,
			invoice_value, invoice_currency, incoterm, po_id, is_frozen
		) VALUES (?, ?, 'INV-COMM-4800', '2026-11-25', 'General', 4800.0, 'USD', 'FOB', ?, 0)`,
		shipmentID, supID, po.ID)
	if err != nil {
		t.Fatalf("insert test shipment: %v", err)
	}

	_, err = mgr.DB().Exec(`
		INSERT INTO invoices (
			id, shipment_id, status, po_id, line_total_decimals, invoice_total_decimals
		) VALUES (?, ?, 'FINALIZED', ?, 2, 2)`,
		invoiceID, shipmentID, po.ID)
	if err != nil {
		t.Fatalf("insert test invoice: %v", err)
	}

	// 4. Test Advance Reconciliation:
	// Advance Paid = $5,000, Invoice Amount = $4,800.
	// Expected Variance: +$200 (Under-invoiced / Excess Advance of $200 held by supplier)
	recon, err := svc.GetAdvanceReconciliation(supID)
	if err != nil {
		t.Fatalf("get advance reconciliation: %v", err)
	}
	if len(recon) == 0 {
		t.Fatalf("expected reconciliation entries")
	}

	var found *models.AdvanceReconciliationSummary
	for i := range recon {
		if recon[i].POID == po.ID {
			found = &recon[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected reconciliation entry for po %s", po.ID)
	}

	if found.AdvancePaidAmount != 5000.0 {
		t.Fatalf("expected advance paid 5000, got %.2f", found.AdvancePaidAmount)
	}
	if found.TotalInvoicedAmount != 4800.0 {
		t.Fatalf("expected invoiced amount 4800, got %.2f", found.TotalInvoicedAmount)
	}
	if found.VarianceAmount != 200.0 {
		t.Fatalf("expected variance +200.0, got %.2f", found.VarianceAmount)
	}
	if found.VarianceType != "UNDER_INVOICED" {
		t.Fatalf("expected variance type UNDER_INVOICED, got %s", found.VarianceType)
	}
}
