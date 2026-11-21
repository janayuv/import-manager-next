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

	// Unfreeze and set DateOfDelivery -> auto-transitions to delivered and prevents ready-dly reversion
	if _, err := svc.Unfreeze(sh.ID); err != nil {
		t.Fatalf("unfreeze: %v", err)
	}
	deliveredShp, err := svc.Update(sh.ID, models.ShipmentInput{
		SupplierID:     sup.ID,
		InvoiceNumber:  "INV-2026-10",
		InvoiceDate:    "2023-07-17",
		InvoiceValue:   2500.50,
		InvoiceCurrency: "USD",
		Incoterm:       "CIF",
		Status:         "ready-dly", // submitted as ready-dly with delivery date
		DateOfDelivery: "2026-10-25",
	})
	if err != nil {
		t.Fatalf("update delivery date: %v", err)
	}
	if deliveredShp.Status != "delivered" || deliveredShp.DateOfDelivery != "2026-10-25" {
		t.Fatalf("expected delivered status and normalized date, got status=%q date=%q", deliveredShp.Status, deliveredShp.DateOfDelivery)
	}

	// Ready-dly sweep must not revert delivered shipment
	n2, err := svc.CheckReadyForDelivery("2026-10-30")
	if err != nil || n2 != 0 {
		t.Fatalf("expected 0 ready-dly updates on delivered shipment, got %d err=%v", n2, err)
	}
}

func TestShipmentDuplicateRejectionAndCSVIntraBatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shp_dup_test.db")
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
	supA, err := supSvc.Create(models.SupplierInput{SupplierName: "CNF Co LTD", Country: "South Korea", IsActive: true})
	if err != nil {
		t.Fatalf("create supplier A: %v", err)
	}
	supB, err := supSvc.Create(models.SupplierInput{SupplierName: "Hanwha Tech", Country: "South Korea", IsActive: true})
	if err != nil {
		t.Fatalf("create supplier B: %v", err)
	}

	svc := shipment.NewService(db, secSvc, auditSvc)

	// 1. Create original shipment
	shp1, err := svc.Create(models.ShipmentInput{
		SupplierID:      supA.ID,
		InvoiceNumber:   "CNF-2608-06",
		InvoiceDate:     "2026-08-25",
		GoodsCategory:   "Machinery",
		InvoiceValue:    57819456,
		InvoiceCurrency: "KRW",
		Incoterm:        "EXW",
	})
	if err != nil {
		t.Fatalf("create initial shipment: %v", err)
	}
	if shp1.ID == "" {
		t.Fatalf("expected non-empty shipment ID")
	}

	// 2. Exact duplicate invoice number for the same supplier must be rejected
	_, err = svc.Create(models.ShipmentInput{
		SupplierID:      supA.ID,
		InvoiceNumber:   "CNF-2608-06",
		InvoiceDate:     "2026-08-25",
		GoodsCategory:   "Machinery",
		InvoiceValue:    57819456,
		InvoiceCurrency: "KRW",
		Incoterm:        "EXW",
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate shipment detected") {
		t.Fatalf("expected duplicate shipment rejection for same supplier and invoice, got err=%v", err)
	}

	// 3. Case-insensitive & trimmed duplicate must also be rejected
	_, err = svc.Create(models.ShipmentInput{
		SupplierID:      supA.ID,
		InvoiceNumber:   "  cnf-2608-06  ",
		InvoiceDate:     "2026-08-25",
		GoodsCategory:   "Machinery",
		InvoiceValue:    57819456,
		InvoiceCurrency: "KRW",
		Incoterm:        "EXW",
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate shipment detected") {
		t.Fatalf("expected case-insensitive duplicate shipment rejection, got err=%v", err)
	}

	// 4. Same invoice number for a DIFFERENT supplier is allowed
	shpOtherSup, err := svc.Create(models.ShipmentInput{
		SupplierID:      supB.ID,
		InvoiceNumber:   "CNF-2608-06",
		InvoiceDate:     "2026-08-25",
		GoodsCategory:   "Machinery",
		InvoiceValue:    12345,
		InvoiceCurrency: "KRW",
		Incoterm:        "EXW",
	})
	if err != nil {
		t.Fatalf("expected different supplier with same invoice number to succeed, got %v", err)
	}
	if shpOtherSup.ID == "" {
		t.Fatalf("expected valid shpOtherSup ID")
	}

	// 5. Updating shp1 while keeping its own invoice number must succeed
	if _, err := svc.Update(shp1.ID, models.ShipmentInput{
		SupplierID:      supA.ID,
		InvoiceNumber:   "CNF-2608-06",
		InvoiceDate:     "2026-08-25",
		GoodsCategory:   "Machinery Updated",
		InvoiceValue:    57819456,
		InvoiceCurrency: "KRW",
		Incoterm:        "EXW",
	}); err != nil {
		t.Fatalf("updating self without changing invoice number failed: %v", err)
	}

	// 6. Creating another shipment for supA and updating it to shp1's invoice must be rejected
	shp2, err := svc.Create(models.ShipmentInput{
		SupplierID:      supA.ID,
		InvoiceNumber:   "CNF-2608-07",
		InvoiceDate:     "2026-08-26",
		GoodsCategory:   "Spare Parts",
		InvoiceValue:    1000,
		InvoiceCurrency: "KRW",
		Incoterm:        "EXW",
	})
	if err != nil {
		t.Fatalf("create shp2: %v", err)
	}
	if _, err := svc.Update(shp2.ID, models.ShipmentInput{
		SupplierID:      supA.ID,
		InvoiceNumber:   "CNF-2608-06",
		InvoiceDate:     "2026-08-26",
		GoodsCategory:   "Spare Parts",
		InvoiceValue:    1000,
		InvoiceCurrency: "KRW",
		Incoterm:        "EXW",
	}); err == nil || !strings.Contains(err.Error(), "duplicate shipment detected") {
		t.Fatalf("expected update duplicate rejection, got err=%v", err)
	}

	// 7. CSV Import: test both intra-batch duplicates and database duplicates
	csvContent := strings.Join([]string{
		"supplierName,invoiceNumber,invoiceDate,invoiceValue,invoiceCurrency,incoterm",
		"CNF Co LTD,CNF-2608-06,2026-08-25,5000,KRW,EXW", // duplicate of existing DB shp1
		"CNF Co LTD,CSV-NEW-01,2026-08-25,5000,KRW,EXW",  // valid new row
		"CNF Co LTD,CSV-NEW-01,2026-08-25,5000,KRW,EXW",  // intra-batch duplicate of preceding row
		"CNF Co LTD,CSV-NEW-02,2026-08-25,6000,KRW,EXW",  // valid new row
	}, "\n")

	impRes, err := svc.ImportCSV("test.csv", csvContent)
	if err != nil {
		t.Fatalf("ImportCSV returned error: %v", err)
	}
	if impRes.TotalRows != 4 {
		t.Fatalf("expected 4 total rows, got %d", impRes.TotalRows)
	}
	if impRes.ImportedRows != 2 {
		t.Fatalf("expected 2 imported rows, got %d", impRes.ImportedRows)
	}
	if impRes.SkippedRows != 2 {
		t.Fatalf("expected 2 skipped rows, got %d", impRes.SkippedRows)
	}
	if len(impRes.Errors) != 2 {
		t.Fatalf("expected 2 error entries, got %d: %+v", len(impRes.Errors), impRes.Errors)
	}
	// Row 2 was duplicate of DB
	if !strings.Contains(impRes.Errors[0].Message, "already exists") {
		t.Errorf("expected DB duplicate error for row 2, got: %s", impRes.Errors[0].Message)
	}
	// Row 4 was intra-batch duplicate
	if !strings.Contains(impRes.Errors[1].Message, "duplicate invoice") || !strings.Contains(impRes.Errors[1].Message, "first seen at row 3") {
		t.Errorf("expected intra-batch duplicate error for row 4, got: %s", impRes.Errors[1].Message)
	}
}
