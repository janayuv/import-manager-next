package boe_summary_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/boe_summary"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func setupBOESummarySvc(t *testing.T) (*boe_summary.Service, *sql.DB, *security.Service) {
	t.Helper()
	tmp := t.TempDir()
	mgr, err := connection.OpenEncrypted(filepath.Join(tmp, "boe_summary_test.db"), "test-key-32-chars-minimum-123456")
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
	return boe_summary.NewService(db, secSvc, auditSvc), db, secSvc
}

func TestBOESummaryListGetUpdateAndDelete(t *testing.T) {
	svc, db, _ := setupBOESummarySvc(t)

	// Seed supplier, shipment, and boe_details
	_, err := db.Exec(`INSERT INTO suppliers (id, supplier_name, country, email, is_active) VALUES ('SUP-1', 'Tokyo Electron', 'Japan', 'tokyo@example.com', 1)`)
	if err != nil {
		t.Fatalf("insert supplier: %v", err)
	}
	_, err = db.Exec(`INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, shipment_mode, bl_awb_number, status, is_frozen)
		VALUES ('SHP-100', 'SUP-1', 'INV-2026-001', '2026-09-20', 'Machinery', 25000, 'USD', 'CIF', 'Sea', 'BL-100', 'customs-clearance', 0)`)
	if err != nil {
		t.Fatalf("insert shipment: %v", err)
	}
	_, err = db.Exec(`INSERT INTO boe_details (id, be_number, be_date, location, total_assessment_value, duty_amount, duty_paid, challan_number)
		VALUES ('BOE-DET-100', '8877665', '2026-09-22', 'INMAA1', 1000000, 150000, 150000, 'CH-998877')`)
	if err != nil {
		t.Fatalf("insert boe_details: %v", err)
	}

	formParams := models.BOECalcParams{
		ID:            "CALC-REC-1",
		ShipmentID:    "SHP-100",
		BOEID:         "BOE-DET-100",
		SupplierName:  "Tokyo Electron",
		InvoiceNumber: "INV-2026-001",
		Status:        "Reconciled",
		ExchangeRate:  85.0,
	}
	formJSON, _ := json.Marshal(formParams)

	calcResult := models.BOECalculationResult{
		TotalAssessableValue: 1000000,
		GrossBCDTotal:        75000,
		SWSTotal:             7500,
		IGSTTotal:            67500,
		CustomsDutyTotal:     150000,
		CashCustomsDutyTotal: 150000,
	}
	resJSON, _ := json.Marshal(calcResult)

	_, err = db.Exec(`INSERT INTO boe_calculations (id, shipment_id, boe_id, supplier_name, invoice_number, status, form_values_json, item_inputs_json, calculation_result_json)
		VALUES ('CALC-REC-1', 'SHP-100', 'BOE-DET-100', 'Tokyo Electron', 'INV-2026-001', 'Reconciled', ?, '[]', ?)`,
		string(formJSON), string(resJSON))
	if err != nil {
		t.Fatalf("insert calculation: %v", err)
	}

	// 1. List calculations (no filters)
	list, err := svc.List("", "")
	if err != nil {
		t.Fatalf("list calculations: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 calculation, got %d", len(list))
	}
	if list[0].BENumber != "8877665" || list[0].Status != "Reconciled" {
		t.Fatalf("unexpected summary item: %+v", list[0])
	}

	// 2. Filter by status
	listReconciled, err := svc.List("", "Reconciled")
	if err != nil || len(listReconciled) != 1 {
		t.Fatalf("expected 1 reconciled calculation, got %d (err: %v)", len(listReconciled), err)
	}
	listClosed, err := svc.List("", "Closed")
	if err != nil || len(listClosed) != 0 {
		t.Fatalf("expected 0 closed calculations, got %d", len(listClosed))
	}

	// 3. GetByID
	detail, err := svc.GetByID("CALC-REC-1")
	if err != nil {
		t.Fatalf("get detail by id: %v", err)
	}
	if detail.ID != "CALC-REC-1" || detail.BENumber != "8877665" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if !detail.Reconciliation.IsReconciled || detail.Reconciliation.DutyDifference != 0 {
		t.Fatalf("expected 0 duty difference and isReconciled=true, got %+v", detail.Reconciliation)
	}

	// 4. UpdateStatus
	if err := svc.UpdateStatus("CALC-REC-1", "Investigation"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	detailAfter, err := svc.GetByID("CALC-REC-1")
	if err != nil || detailAfter.Status != "Investigation" {
		t.Fatalf("expected status Investigation, got %s", detailAfter.Status)
	}

	// 5. Delete
	if err := svc.Delete("CALC-REC-1"); err != nil {
		t.Fatalf("delete calculation: %v", err)
	}

	// Verify excluded from list
	afterDeleteList, err := svc.List("", "")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(afterDeleteList) != 0 {
		t.Fatalf("expected 0 calculations after delete, got %d", len(afterDeleteList))
	}

	// Verify GetByID returns not found
	_, err = svc.GetByID("CALC-REC-1")
	if err == nil {
		t.Fatalf("expected error getting soft-deleted calculation, got nil")
	}

	// Verify shipment reverted to docu-received
	var shipStatus string
	_ = db.QueryRow(`SELECT status FROM shipments WHERE id = 'SHP-100'`).Scan(&shipStatus)
	if shipStatus != "docu-received" {
		t.Fatalf("expected shipment status docu-received, got %s", shipStatus)
	}
}
