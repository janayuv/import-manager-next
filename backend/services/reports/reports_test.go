package reports_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/reports"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func setupReportsTest(t *testing.T) (*reports.Service, *sql.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "reports_test.db")
	mgr, err := connection.OpenEncrypted(dbPath, "test-reports-key-32-chars-long!")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	db := mgr.DB()

	if _, err := migrations.Run(context.Background(), db, "1.0.0", nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	auditSvc := audit.NewService(db)
	secSvc := security.NewService(db, false, auditSvc)
	if _, err := secSvc.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup: %v", err)
	}

	return reports.NewService(db, secSvc), db
}

func TestPartPeriodAnalysisEndToEnd(t *testing.T) {
	svc, db := setupReportsTest(t)

	// 1. Seed Supplier
	_, err := db.Exec(`INSERT INTO suppliers (id, supplier_name, country, email, is_active) VALUES ('SUP-001', 'Acme Bearings Corp', 'Japan', 'sales@acme-bearings.jp', 1)`)
	if err != nil {
		t.Fatalf("insert supplier: %v", err)
	}

	// 2. Seed Item
	_, err = db.Exec(`
		INSERT INTO items (id, part_number, item_description, unit, currency, unit_price, hsn_code, supplier_id, is_active, bcd, sws, igst)
		VALUES ('ITM-001', 'BEAR-6204', 'Deep Groove Ball Bearing 6204', 'PCS', 'USD', 12.50, '84821010', 'SUP-001', 1, '7.5', '10', '18')
	`)
	if err != nil {
		t.Fatalf("insert item: %v", err)
	}

	// 3. Seed Shipments & Invoices in period
	// Shipment 1: 2026-05-10
	_, err = db.Exec(`
		INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, shipment_mode, bl_awb_number, status, is_frozen)
		VALUES ('SHP-001', 'SUP-001', 'INV-2026-01', '2026-05-10', 'Capital Goods', 12500.0, 'USD', 'FOB', 'Sea', 'BL-99881', 'delivered', 0)
	`)
	if err != nil {
		t.Fatalf("insert shipment 1: %v", err)
	}
	_, err = db.Exec(`INSERT INTO invoices (id, shipment_id, status) VALUES ('INV-001', 'SHP-001', 'finalized')`)
	if err != nil {
		t.Fatalf("insert invoice 1: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO invoice_line_items (id, invoice_id, item_id, quantity, unit_price, duty_percent, sws_percent, igst_percent)
		VALUES ('LIN-001', 'INV-001', 'ITM-001', 1000.0, 12.50, 7.5, 10.0, 18.0)
	`)
	if err != nil {
		t.Fatalf("insert line item 1: %v", err)
	}

	// Shipment 2: 2026-08-15
	_, err = db.Exec(`
		INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, shipment_mode, bl_awb_number, status, is_frozen)
		VALUES ('SHP-002', 'SUP-001', 'INV-2026-02', '2026-08-15', 'Capital Goods', 6500.0, 'USD', 'CIF', 'Air', 'AWB-11223', 'customs-clearance', 0)
	`)
	if err != nil {
		t.Fatalf("insert shipment 2: %v", err)
	}
	_, err = db.Exec(`INSERT INTO invoices (id, shipment_id, status) VALUES ('INV-002', 'SHP-002', 'finalized')`)
	if err != nil {
		t.Fatalf("insert invoice 2: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO invoice_line_items (id, invoice_id, item_id, quantity, unit_price, duty_percent, sws_percent, igst_percent)
		VALUES ('LIN-002', 'INV-002', 'ITM-001', 500.0, 13.00, 7.5, 10.0, 18.0)
	`)
	if err != nil {
		t.Fatalf("insert line item 2: %v", err)
	}

	// 4. Seed BOE Calculations for Shipment 1
	_, err = db.Exec(`
		INSERT INTO boe_details (id, be_number, be_date, location, total_assessment_value, duty_amount, duty_paid, challan_number, payment_date)
		VALUES ('BOE-001', 'BE-887766', '2026-05-15', 'INNSA1', 1050000.0, 195000.0, 195000.0, 'CH-001', '2026-05-16')
	`)
	if err != nil {
		t.Fatalf("insert boe_detail 1: %v", err)
	}

	calcJSON1 := `{
		"interest": 2500.0,
		"items": [
			{
				"partNumber": "BEAR-6204",
				"description": "Deep Groove Ball Bearing 6204",
				"quantity": 1000.0,
				"unit": "PCS",
				"unitPrice": 12.50,
				"itemForexValue": 12500.0,
				"itemValueInr": 1037500.0,
				"assessableValue": 1050000.0,
				"actualBcdRate": 7.5,
				"bcdValue": 78750.0,
				"actualSwsRate": 10.0,
				"swsValue": 7875.0,
				"actualIgstRate": 18.0,
				"igstValue": 204592.50
			}
		]
	}`
	_, err = db.Exec(`
		INSERT INTO boe_calculations (id, shipment_id, boe_id, supplier_name, invoice_number, status, form_values_json, item_inputs_json, calculation_result_json)
		VALUES ('CALC-001', 'SHP-001', 'BOE-001', 'Acme Bearings Corp', 'INV-2026-01', 'Reconciled', '{}', '[]', ?)
	`, calcJSON1)
	if err != nil {
		t.Fatalf("insert boe_calc 1: %v", err)
	}

	// 5. Seed Expenses for Shipment 1
	_, err = db.Exec(`INSERT INTO service_providers (id, name, state) VALUES ('SP-001', 'FastLogistics Customs CHA', 'Maharashtra')`)
	if err != nil {
		t.Fatalf("insert sp 1: %v", err)
	}
	_, err = db.Exec(`INSERT INTO expense_types (id, name) VALUES ('EXP-FRT', 'Ocean Freight & Port')`)
	if err != nil {
		t.Fatalf("insert exp type 1: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO expense_invoices (id, shipment_id, service_provider_id, invoice_no, invoice_date, total_amount, total_amount_paise)
		VALUES ('EI-001', 'SHP-001', 'SP-001', 'EXP-INV-101', '2026-05-18', 45000.0, 4500000)
	`)
	if err != nil {
		t.Fatalf("insert expense invoice: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO expenses (id, expense_invoice_id, shipment_id, service_provider_id, expense_type_id, invoice_no, invoice_date, amount, amount_paise, total_amount_paise)
		VALUES ('EXP-ROW-1', 'EI-001', 'SHP-001', 'SP-001', 'EXP-FRT', 'EXP-INV-101', '2026-05-18', 40000.0, 4000000, 4500000)
	`)
	if err != nil {
		t.Fatalf("insert expense: %v", err)
	}

	// Test A: SearchParts
	lookups, err := svc.SearchParts("BEAR", 10)
	if err != nil {
		t.Fatalf("SearchParts error: %v", err)
	}
	if len(lookups) != 1 {
		t.Fatalf("expected 1 lookup result, got %d", len(lookups))
	}
	if lookups[0].PartNumber != "BEAR-6204" {
		t.Errorf("expected BEAR-6204, got %s", lookups[0].PartNumber)
	}
	if lookups[0].ShipmentCount != 2 {
		t.Errorf("expected 2 shipments for BEAR-6204, got %d", lookups[0].ShipmentCount)
	}
	if lookups[0].TotalQuantity != 1500.0 {
		t.Errorf("expected 1500 total qty, got %.2f", lookups[0].TotalQuantity)
	}

	// Test B: GetPartPeriodAnalysis for full range
	res, err := svc.GetPartPeriodAnalysis(models.PartPeriodAnalysisRequest{
		PartNumber: "BEAR-6204",
		StartDate:  "2026-01-01",
		EndDate:    "2026-12-31",
		DateBasis:  "invoice_date",
	})
	if err != nil {
		t.Fatalf("GetPartPeriodAnalysis error: %v", err)
	}

	if res.TargetItem.PartNumber != "BEAR-6204" {
		t.Errorf("target item mismatch: got %s", res.TargetItem.PartNumber)
	}
	if len(res.Shipments) != 2 {
		t.Errorf("expected 2 shipments, got %d", len(res.Shipments))
	}
	if len(res.BOELines) != 1 {
		t.Errorf("expected 1 BOE line, got %d", len(res.BOELines))
	}
	if len(res.Expenses) != 1 {
		t.Errorf("expected 1 expense row, got %d", len(res.Expenses))
	}
	if res.Summary.TotalQuantity != 1500.0 {
		t.Errorf("expected 1500.0 total qty, got %.2f", res.Summary.TotalQuantity)
	}
	if res.Summary.TotalShipments != 2 {
		t.Errorf("expected 2 total shipments, got %d", res.Summary.TotalShipments)
	}
	if res.Summary.TotalBOEs != 1 {
		t.Errorf("expected 1 BOE, got %d", res.Summary.TotalBOEs)
	}
	if res.Summary.TotalAssessableValueINR != 1050000.0 {
		t.Errorf("expected 1050000 assessable, got %.2f", res.Summary.TotalAssessableValueINR)
	}
	if res.Summary.TotalBOEInterest != 2500.0 {
		t.Errorf("expected 2500.0 BOE interest, got %.2f", res.Summary.TotalBOEInterest)
	}
	if res.Summary.TotalAllocatedExpensesINR != 40000.0 {
		t.Errorf("expected 40000.0 allocated expenses, got %.2f", res.Summary.TotalAllocatedExpensesINR)
	}

	// Test C: Narrow period filtering
	resNarrow, err := svc.GetPartPeriodAnalysis(models.PartPeriodAnalysisRequest{
		PartNumber: "BEAR-6204",
		StartDate:  "2026-08-01",
		EndDate:    "2026-08-31",
		DateBasis:  "invoice_date",
	})
	if err != nil {
		t.Fatalf("narrow filter error: %v", err)
	}
	if len(resNarrow.Shipments) != 1 {
		t.Errorf("expected 1 shipment in narrow period, got %d", len(resNarrow.Shipments))
	}
	if resNarrow.Shipments[0].ShipmentID != "SHP-002" {
		t.Errorf("expected SHP-002 in narrow period, got %s", resNarrow.Shipments[0].ShipmentID)
	}

	// Test D: ExportPartPeriodAnalysisCSV
	csvContent, err := svc.ExportPartPeriodAnalysisCSV(models.PartPeriodAnalysisRequest{
		PartNumber: "BEAR-6204",
		StartDate:  "2026-01-01",
		EndDate:    "2026-12-31",
	})
	if err != nil {
		t.Fatalf("ExportPartPeriodAnalysisCSV error: %v", err)
	}
	if !strings.Contains(csvContent, "BEAR-6204") {
		t.Errorf("CSV missing BEAR-6204")
	}
	if !strings.Contains(csvContent, "RELATED SHIPMENTS & INVOICES") {
		t.Errorf("CSV missing shipments section")
	}
	if !strings.Contains(csvContent, "BILL OF ENTRY — CUSTOMS VALUATION & DUTY") {
		t.Errorf("CSV missing BOE section")
	}
	if !strings.Contains(csvContent, "ALLOCATED SHIPMENT EXPENSES") {
		t.Errorf("CSV missing expenses section")
	}
	if !strings.Contains(csvContent, "REPORT METADATA & FIELD INVENTORY") {
		t.Errorf("CSV missing metadata section")
	}

	// Test E: Search by Description
	resDesc, err := svc.GetPartPeriodAnalysis(models.PartPeriodAnalysisRequest{
		Search: "Ball Bearing",
	})
	if err != nil {
		t.Fatalf("search by description error: %v", err)
	}
	if resDesc.TargetItem.PartNumber != "BEAR-6204" {
		t.Errorf("expected BEAR-6204 from description search, got %s", resDesc.TargetItem.PartNumber)
	}

	// Test F: Invalid date order
	_, err = svc.GetPartPeriodAnalysis(models.PartPeriodAnalysisRequest{
		StartDate: "2026-12-01",
		EndDate:   "2026-01-01",
	})
	if err == nil {
		t.Errorf("expected error for startDate > endDate, got nil")
	}

	// Test G: Date basis BE Date
	resBEDate, err := svc.GetPartPeriodAnalysis(models.PartPeriodAnalysisRequest{
		PartNumber: "BEAR-6204",
		StartDate:  "2026-05-01",
		EndDate:    "2026-05-31",
		DateBasis:  "be_date",
	})
	if err != nil {
		t.Fatalf("be_date filter error: %v", err)
	}
	if len(resBEDate.Shipments) != 1 {
		t.Errorf("expected 1 shipment with BE in May 2026, got %d", len(resBEDate.Shipments))
	}
}
