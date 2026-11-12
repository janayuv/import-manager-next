package invoice_wizard_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/invoice"
	"github.com/janayuv/import-manager-next/backend/services/invoice_wizard"
	"github.com/janayuv/import-manager-next/backend/services/item_master"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/shipment"
	"github.com/janayuv/import-manager-next/backend/services/supplier"
)

func TestInvoiceWizardDraftFinalizePasteAndTransition(t *testing.T) {
	tmp := t.TempDir()
	mgr, err := connection.OpenEncrypted(filepath.Join(tmp, "wiz_test.db"), "test-key-32-chars-minimum-123456")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = mgr.Close() }()
	db := mgr.DB()

	if _, err := migrations.Run(context.Background(), db, "1.0.0", nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	auditSvc := audit.NewService(db)
	secSvc := security.NewService(db, false, auditSvc)
	if _, err := secSvc.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("setup admin: %v", err)
	}

	supSvc := supplier.NewService(db, secSvc, auditSvc)
	sup, _ := supSvc.Create(models.SupplierInput{SupplierName: "Orion GmbH", Country: "Germany", IsActive: true})

	itemSvc := item_master.NewService(db, secSvc, auditSvc, tmp)
	it, _ := itemSvc.Create(models.ItemInput{
		PartNumber: "OR-100", ItemDescription: "Stepper Motor", Unit: "PCS", Currency: "USD",
		UnitPrice: 100.00, HSNCode: "85011090", SupplierID: sup.ID, IsActive: true, BCD: "7.5", SWS: "10", IGST: "18",
	})

	shipSvc := shipment.NewService(db, secSvc, auditSvc)
	sh, _ := shipSvc.Create(models.ShipmentInput{
		SupplierID: sup.ID, InvoiceNumber: "CI-900", InvoiceDate: "2026-11-10",
		GoodsCategory: "Motors", InvoiceValue: 500.00, InvoiceCurrency: "USD", Incoterm: "CIF",
	})

	wiz := invoice_wizard.NewService(db, secSvc, auditSvc)
	invSvc := invoice.NewService(db, secSvc, auditSvc)

	// Multi-line paste format A (tab delimited with diverged price 105 vs master 100)
	parsed, err := wiz.ParseMultiLinePaste("OR-100\t4\t105.00\nUNKNOWN-PN\t2\t50")
	if err != nil || len(parsed) != 2 {
		t.Fatalf("paste parse failed: %+v err=%v", parsed, err)
	}
	if !parsed[0].Matched || !parsed[0].PriceDiverged || parsed[1].Matched {
		t.Fatalf("unexpected paste match/divergence flags: %+v", parsed)
	}

	// Finalize MUST fail when sum (4 * 100 = 400) != shipment.invoice_value (500)
	if _, err := wiz.SaveInvoice(models.SaveInvoiceRequest{
		ShipmentID: sh.ID, Finalize: true, LineTotalDecimals: 2, InvoiceTotalDecimals: 2,
		Lines: []models.InvoiceLineInput{{ItemID: it.ID, Quantity: 4, UnitPrice: 100}},
	}); err == nil {
		t.Fatalf("expected finalize mismatch error")
	}

	// Draft bypasses total mismatch and transitions shipment -> in-transit
	draft, err := wiz.SaveInvoice(models.SaveInvoiceRequest{
		ShipmentID: sh.ID, Finalize: false, LineTotalDecimals: 2, InvoiceTotalDecimals: 2,
		Lines: []models.InvoiceLineInput{{ItemID: it.ID, Quantity: 4, UnitPrice: 100}},
	})
	if err != nil {
		t.Fatalf("draft save failed: %v", err)
	}
	if draft.Invoice.Status != "Draft" || draft.Invoice.ComputedTotal != 400.00 {
		t.Fatalf("unexpected draft: %+v", draft)
	}
	shMid, _ := shipSvc.GetByID(sh.ID)
	if shMid.Status != "in-transit" {
		t.Fatalf("expected shipment auto-transitioned to in-transit, got %s", shMid.Status)
	}

	// Finalize with exact match (5 * 100 = 500) + freeze shipment succeeds
	final, err := wiz.SaveInvoice(models.SaveInvoiceRequest{
		InvoiceID: draft.Invoice.ID, ShipmentID: sh.ID, Finalize: true, FreezeShipment: true,
		LineTotalDecimals: 2, InvoiceTotalDecimals: 2,
		Lines: []models.InvoiceLineInput{{ItemID: it.ID, Quantity: 5, UnitPrice: 100, DutyPercent: 7.5, SWSPercent: 10, IGSTPercent: 18}},
	})
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}
	if final.Invoice.Status != "Finalized" || !final.Invoice.ShipmentFrozen {
		t.Fatalf("expected Finalized + Frozen, got %+v", final.Invoice)
	}
	// Delete blocked on frozen shipment
	if err := invSvc.Delete(final.Invoice.ID); err == nil {
		t.Fatalf("expected delete blocked on frozen shipment")
	}
}
