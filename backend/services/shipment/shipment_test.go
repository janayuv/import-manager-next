package shipment_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/shipment"
	"github.com/janayuv/import-manager-next/backend/services/supplier"
)

func TestShipmentLifecycleFreezeCSVAndReadyDly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shp_test.db")
	mgr, err := connection.OpenEncrypted(dbPath, "test-key-32-chars-minimum-123456")
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
		t.Fatalf("admin setup: %v", err)
	}

	supSvc := supplier.NewService(db, secSvc, auditSvc)
	sup, err := supSvc.Create(models.SupplierInput{SupplierName: "Apex Valves", Country: "Japan", IsActive: true})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	svc := shipment.NewService(db, secSvc, auditSvc)

	// Tab-delimited CSV with Excel serial date 45124 (= 2023-07-17)
	tsv := strings.Join([]string{
		"supplierName\tinvoiceNumber\tinvoiceDate\tinvoiceValue\tinvoiceCurrency\tincoterm",
		"Apex Valves\tINV-2026-10\t45124\t2500.50\tUSD\tCIF",
		"Apex Valves\tMISSING_VAL\t2026-11-01\t\tUSD\tCIF",
	}, "\n")
	imp, err := svc.ImportCSV("shipments.tsv", tsv)
	if err != nil {
		t.Fatalf("import csv: %v", err)
	}
	if imp.ImportedRows != 1 || imp.SkippedRows != 1 {
		t.Fatalf("unexpected import result: %+v", imp)
	}

	page, err := svc.List(models.ShipmentPageRequest{Search: "INV-2026-10", Page: 1, PageSize: 10})
	if err != nil || page.TotalCount != 1 {
		t.Fatalf("list shipments: %+v err=%v", page, err)
	}
	sh := page.Items[0]
	if sh.InvoiceDate != "2023-07-17" || sh.Status != "docs-rcvd" {
		t.Fatalf("unexpected normalized shipment: %+v", sh)
	}

	// Insert a BOE detail + boe_calculations link dated 2026-10-01 and test 7-day ready-dly rule on 2026-10-10
	if _, err := db.Exec(`
		INSERT INTO boe_details (id, be_number, be_date, location, total_assessment_value, duty_amount)
		VALUES ('BOE-1', 'BE-777', '2026-10-01', 'INMAA4', 200000, 35000)`,
	); err != nil {
		t.Fatalf("insert boe_details: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO boe_calculations (id, shipment_id, boe_id, supplier_name, invoice_number, status, form_values_json, item_inputs_json, calculation_result_json)
		VALUES ('BOEC-1', ?, 'BOE-1', 'Apex Valves', 'INV-2026-10', 'Reconciled', '{}', '[]', '{}')`,
		sh.ID,
	); err != nil {
		t.Fatalf("insert boe_calculations: %v", err)
	}
	n, err := svc.CheckReadyForDelivery("2026-10-10")
	if err != nil || n != 1 {
		t.Fatalf("expected 1 ready-dly update, got %d err=%v", n, err)
	}
	updated, err := svc.GetByID(sh.ID)
	if err != nil || updated.Status != "ready-dly" {
		t.Fatalf("expected ready-dly status, got %+v err=%v", updated, err)
	}

	// Freeze blocks edits and delete
	if _, err := svc.Freeze(sh.ID); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := svc.Update(sh.ID, models.ShipmentInput{
		SupplierID: sup.ID, InvoiceNumber: "CHANGED", InvoiceDate: "2026-11-01", InvoiceValue: 100,
	}); err == nil {
		t.Fatalf("expected frozen edit rejection")
	}
	if err := svc.Delete(sh.ID); err == nil {
		t.Fatalf("expected frozen delete rejection")
	}
}
