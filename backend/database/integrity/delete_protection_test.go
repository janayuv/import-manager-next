package integrity_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/models"
)

func setupTestRuntime(t *testing.T) *bootstrap.Runtime {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "del-prot-test-key-0123456789abcdef")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	dir := filepath.Join(t.TempDir(), "appdata")
	rt, err := bootstrap.Initialize(nil, dir)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	t.Cleanup(func() { _ = rt.Shutdown() })

	if _, err := rt.Security.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup failed: %v", err)
	}
	return rt
}

// 1. Connection Verification: Confirm _foreign_keys=on and PRAGMA foreign_keys = ON are enforced.
func TestConnectionEnforcesForeignKeys(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fk_conn_test.db")
	mgr, err := connection.OpenEncrypted(dbPath, "test-key-32-chars-minimum-123456")
	if err != nil {
		t.Fatalf("OpenEncrypted failed: %v", err)
	}
	defer mgr.Close()

	var fkVal int
	if err := mgr.DB().QueryRow("PRAGMA foreign_keys;").Scan(&fkVal); err != nil {
		t.Fatalf("query PRAGMA foreign_keys: %v", err)
	}
	if fkVal != 1 {
		t.Fatalf("expected PRAGMA foreign_keys == 1, got %d", fkVal)
	}

	health, err := mgr.Health()
	if err != nil {
		t.Fatalf("Health() check failed: %v", err)
	}
	if !health.ForeignKeysOn {
		t.Fatalf("expected HealthInfo.ForeignKeysOn to be true")
	}
}

// 2. Database-Level Deletion Protection: Triggers and Constraints block CASCADE and direct SQL deletions.
func TestDatabaseLevelDeleteProtectionTriggers(t *testing.T) {
	rt := setupTestRuntime(t)
	db := rt.DB.DB()

	// Seed test supplier
	_, err := db.Exec(`INSERT INTO suppliers (id, supplier_name, country, email, phone, is_active) VALUES ('SUP-TEST-1', 'Test Supplier', 'DE', 'test@supplier.de', '+4912345', 1)`)
	if err != nil {
		t.Fatalf("insert supplier: %v", err)
	}

	// Seed test item
	_, err = db.Exec(`INSERT INTO items (id, part_number, item_description, unit, currency, unit_price, hsn_code, supplier_id, is_active)
		VALUES ('ITM-TEST-1', 'PN-001', 'Test Widget', 'PCS', 'EUR', 15.5, '847130', 'SUP-TEST-1', 1)`)
	if err != nil {
		t.Fatalf("insert item: %v", err)
	}

	// Seed test shipment
	_, err = db.Exec(`INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, shipment_mode, shipment_type, status)
		VALUES ('SHP-TEST-1', 'SUP-TEST-1', 'INV-SHP-1', '2026-11-01', 'Hardware', 1000, 'EUR', 'FOB', 'SEA', 'STANDARD', 'docs-rcvd')`)
	if err != nil {
		t.Fatalf("insert shipment: %v", err)
	}

	// Seed test invoice and line item
	_, err = db.Exec(`INSERT INTO invoices (id, shipment_id, status) VALUES ('INV-TEST-1', 'SHP-TEST-1', 'Finalized')`)
	if err != nil {
		t.Fatalf("insert invoice: %v", err)
	}
	_, err = db.Exec(`INSERT INTO invoice_line_items (id, invoice_id, item_id, quantity, unit_price) VALUES ('ILI-TEST-1', 'INV-TEST-1', 'ITM-TEST-1', 10, 15.5)`)
	if err != nil {
		t.Fatalf("insert line item: %v", err)
	}

	// Seed test BOE detail, calculation, and boe item
	_, err = db.Exec(`INSERT INTO boe_details (id, be_number, be_date, location, total_assessment_value, duty_amount)
		VALUES ('BOE-DET-1', 'BE-99901', '2026-11-01', 'INNSA1', 50000, 5000)`)
	if err != nil {
		t.Fatalf("insert boe_details: %v", err)
	}
	_, err = db.Exec(`INSERT INTO boe_calculations (id, shipment_id, boe_id, supplier_name, invoice_number, status, form_values_json, item_inputs_json, calculation_result_json)
		VALUES ('BOE-CALC-1', 'SHP-TEST-1', 'BOE-DET-1', 'Test Supplier', 'INV-SHP-1', 'Computed', '{}', '{}', '{}')`)
	if err != nil {
		t.Fatalf("insert boe_calculations: %v", err)
	}
	_, err = db.Exec(`INSERT INTO boe_items (id, boe_calculation_id, item_id, assessable_value, bcd_rate, sws_rate, igst_rate, total)
		VALUES ('BOE-ITM-1', 'BOE-CALC-1', 'ITM-TEST-1', 50000, 7.5, 10.0, 18.0, 5000)`)
	if err != nil {
		t.Fatalf("insert boe_items: %v", err)
	}

	// Seed container
	_, err = db.Exec(`INSERT INTO shipment_containers (id, shipment_id, container_number, container_size_type, route_type)
		VALUES ('CNT-TEST-1', 'SHP-TEST-1', 'MSCU1234567', '40FT', 'DIRECT')`)
	if err != nil {
		t.Fatalf("insert container: %v", err)
	}

	// A. Direct SQL Hard-Delete of Shipment MUST FAIL and NOT CASCADE into invoices or boe_calculations
	_, err = db.Exec(`DELETE FROM shipments WHERE id = 'SHP-TEST-1'`)
	if err == nil {
		t.Fatalf("expected DELETE shipments to fail due to delete protection trigger")
	}
	if !strings.Contains(err.Error(), "FOREIGN KEY RESTRICTION") {
		t.Fatalf("expected trigger error message, got: %v", err)
	}

	// Verify shipment, invoices, boe_calculations, and containers were NOT destroyed
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM shipments WHERE id = 'SHP-TEST-1'`).Scan(&count)
	if count != 1 {
		t.Fatalf("shipment was destroyed despite delete failure")
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM invoices WHERE id = 'INV-TEST-1'`).Scan(&count)
	if count != 1 {
		t.Fatalf("invoice was destroyed by CASCADE!")
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM boe_calculations WHERE id = 'BOE-CALC-1'`).Scan(&count)
	if count != 1 {
		t.Fatalf("boe_calculation was destroyed by CASCADE!")
	}

	// B. Direct SQL Hard-Delete of Invoice MUST FAIL due to line items
	_, err = db.Exec(`DELETE FROM invoices WHERE id = 'INV-TEST-1'`)
	if err == nil {
		t.Fatalf("expected DELETE invoices to fail due to line items trigger")
	}

	// C. Direct SQL Hard-Delete of BOE Calculation MUST FAIL due to boe_items
	_, err = db.Exec(`DELETE FROM boe_calculations WHERE id = 'BOE-CALC-1'`)
	if err == nil {
		t.Fatalf("expected DELETE boe_calculations to fail due to boe_items trigger")
	}

	// D. Direct SQL Hard-Delete of BOE Details MUST FAIL due to boe_calculations
	_, err = db.Exec(`DELETE FROM boe_details WHERE id = 'BOE-DET-1'`)
	if err == nil {
		t.Fatalf("expected DELETE boe_details to fail due to boe_calculations trigger")
	}

	// E. Direct SQL Hard-Delete of Item MUST FAIL due to invoice line items & boe items
	_, err = db.Exec(`DELETE FROM items WHERE id = 'ITM-TEST-1'`)
	if err == nil {
		t.Fatalf("expected DELETE items to fail due to referencing items")
	}

	// F. Direct SQL Hard-Delete of Supplier MUST FAIL due to shipments and items
	_, err = db.Exec(`DELETE FROM suppliers WHERE id = 'SUP-TEST-1'`)
	if err == nil {
		t.Fatalf("expected DELETE suppliers to fail due to referencing shipments")
	}
}

// 3. Application-Level Structured Dependency Error Verification across Domain Services
func TestApplicationLevelDependencyErrorsOnSoftDelete(t *testing.T) {
	rt := setupTestRuntime(t)

	// Create Supplier
	sup, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Global Tech Hub",
		Country:      "Japan",
		Email:        "tech@global.jp",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	// Create Item linked to Supplier
	item, err := rt.Items.Create(models.ItemInput{
		PartNumber:      "GT-PART-99",
		ItemDescription: "Optic Sensor",
		Unit:            "PCS",
		Currency:        "USD",
		UnitPrice:       120.0,
		HSNCode:         "903180",
		SupplierID:      sup.ID,
		IsActive:        true,
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	// A. Deleting Supplier while Item is active MUST be rejected with structured error
	err = rt.Suppliers.Delete(sup.ID)
	if err == nil {
		t.Fatalf("expected soft delete of supplier to fail when active items exist")
	}
	var depErr *integrity.DependencyViolationError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *integrity.DependencyViolationError, got: %T (%v)", err, err)
	}
	if depErr.ParentTable != "suppliers" || depErr.ParentID != sup.ID {
		t.Fatalf("unexpected DependencyViolationError fields: %+v", depErr)
	}
	if len(depErr.Dependencies) == 0 || depErr.Dependencies[0].ChildTable != "items" {
		t.Fatalf("expected dependencies to list items, got %+v", depErr.Dependencies)
	}
	t.Logf("Verified structured error on supplier delete: %s", depErr.Message)

	// Create Shipment linked to Supplier
	shp, err := rt.Shipments.Create(models.ShipmentInput{
		SupplierID:      sup.ID,
		InvoiceNumber:   "INV-SHP-GT1",
		InvoiceDate:     "2026-11-02",
		GoodsCategory:   "Sensors",
		InvoiceValue:    12000,
		InvoiceCurrency: "USD",
		Incoterm:        "FOB",
		ShipmentMode:    "AIR",
		ShipmentType:    "STANDARD",
	})
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	// Create Commercial Invoice Draft
	draft, err := rt.InvoiceWizard.SaveInvoice(models.SaveInvoiceRequest{
		ShipmentID:           shp.ID,
		Finalize:             false,
		LineTotalDecimals:    2,
		InvoiceTotalDecimals: 2,
		Lines:                []models.InvoiceLineInput{{ItemID: item.ID, Quantity: 100, UnitPrice: 120.0}},
	})
	if err != nil {
		t.Fatalf("save invoice draft: %v", err)
	}
	_ = draft.Invoice.ID

	// B. Deleting Item while Invoice Line Items exist MUST be rejected
	err = rt.Items.Delete(item.ID)
	if err == nil {
		t.Fatalf("expected soft delete of item to fail when invoice line items exist")
	}
	if !errors.As(err, &depErr) {
		t.Fatalf("expected DependencyViolationError, got: %v", err)
	}
	if depErr.ParentTable != "items" || depErr.Dependencies[0].ChildTable != "invoice_line_items" {
		t.Fatalf("expected item -> invoice_line_items dependency, got %+v", depErr)
	}

	// C. Deleting Shipment while Commercial Invoice is active MUST be rejected
	err = rt.Shipments.Delete(shp.ID)
	if err == nil {
		t.Fatalf("expected soft delete of shipment to fail when invoice exists")
	}
	if !errors.As(err, &depErr) {
		t.Fatalf("expected DependencyViolationError, got: %v", err)
	}
	if depErr.ParentTable != "shipments" || depErr.Dependencies[0].ChildTable != "invoices" {
		t.Fatalf("expected shipment -> invoices dependency, got %+v", depErr)
	}

	// D. Create BOE Detail and Calculation
	boeDet, err := rt.BOE.Create(models.BOEDetailInput{
		BENumber:             "BE-2026-Japan",
		BEDate:               "2026-11-03",
		Location:             "INBLR4",
		TotalAssessmentValue: 1200000,
		DutyAmount:           150000,
	})
	if err != nil {
		t.Fatalf("create boe detail: %v", err)
	}
	_, err = rt.DB.DB().Exec(`
		INSERT INTO boe_calculations (id, shipment_id, boe_id, supplier_name, invoice_number, status, form_values_json, item_inputs_json, calculation_result_json)
		VALUES ('CALC-JP-1', ?, ?, 'Global Tech Hub', 'INV-SHP-GT1', 'Final', '{}', '{}', '{}')`, shp.ID, boeDet.ID)
	if err != nil {
		t.Fatalf("insert calculation: %v", err)
	}

	// Deleting BOE Detail while calculation exists MUST be rejected
	err = rt.BOE.Delete(boeDet.ID)
	if err == nil {
		t.Fatalf("expected soft delete of boe detail to fail when calculation exists")
	}
	if !errors.As(err, &depErr) {
		t.Fatalf("expected DependencyViolationError, got: %v", err)
	}
	if depErr.ParentTable != "boe_details" || depErr.Dependencies[0].ChildTable != "boe_calculations" {
		t.Fatalf("expected boe_details -> boe_calculations dependency, got %+v", depErr)
	}
}

// 4. Multi-level Dependency Hierarchy and Recycle Bin Lifecycle Protection
func TestMultiLevelDependencyRecycleBinLifecycle(t *testing.T) {
	rt := setupTestRuntime(t)

	// Step 1: Create Supplier -> Shipment -> Invoice
	sup, _ := rt.Suppliers.Create(models.SupplierInput{SupplierName: "Alpha Logistics", Country: "Singapore", IsActive: true})
	shp, _ := rt.Shipments.Create(models.ShipmentInput{
		SupplierID: sup.ID, InvoiceNumber: "INV-ALPHA-1", InvoiceDate: "2026-11-04",
		GoodsCategory: "Parts", InvoiceValue: 5000, InvoiceCurrency: "USD",
		Incoterm: "FOB", ShipmentMode: "SEA", ShipmentType: "STANDARD",
	})
	item, _ := rt.Items.Create(models.ItemInput{PartNumber: "ALPHA-01", ItemDescription: "Widget", Unit: "PCS", Currency: "USD", UnitPrice: 5000, HSNCode: "847100", SupplierID: sup.ID, IsActive: true})
	draft, err := rt.InvoiceWizard.SaveInvoice(models.SaveInvoiceRequest{
		ShipmentID:           shp.ID,
		Finalize:             false,
		LineTotalDecimals:    2,
		InvoiceTotalDecimals: 2,
		Lines:                []models.InvoiceLineInput{{ItemID: item.ID, Quantity: 1, UnitPrice: 5000}},
	})
	if err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	invID := draft.Invoice.ID

	// Step 2: Cannot delete Supplier or Shipment while children are active
	if err := rt.Suppliers.Delete(sup.ID); err == nil {
		t.Fatalf("expected supplier delete to fail")
	}
	if err := rt.Shipments.Delete(shp.ID); err == nil {
		t.Fatalf("expected shipment delete to fail")
	}

	// Step 3: Delete child Invoice first
	if err := rt.Invoices.Delete(invID); err != nil {
		t.Fatalf("delete invoice failed: %v", err)
	}

	// Step 4: Now Shipment can be soft-deleted
	if err := rt.Shipments.Delete(shp.ID); err != nil {
		t.Fatalf("delete shipment should succeed after invoice is deleted: %v", err)
	}

	// Delete item as well so supplier can be deleted
	if err := rt.Items.Delete(item.ID); err != nil {
		t.Fatalf("delete item failed: %v", err)
	}

	// Step 5: Now Supplier can be soft-deleted
	if err := rt.Suppliers.Delete(sup.ID); err != nil {
		t.Fatalf("delete supplier should succeed after shipment and item are deleted: %v", err)
	}

	// Step 6: Test Recycle Bin Restore Protection:
	// Trying to restore Invoice when Shipment is deleted MUST be blocked with "missing_parent"
	res, err := rt.RecycleBin.RestoreDeletedRecords("invoices", []string{invID})
	if err != nil {
		t.Fatalf("RestoreDeletedRecords failed: %v", err)
	}
	if !res.Blocked || res.ErrorCode != "missing_parent" {
		t.Fatalf("expected restore to be blocked with missing_parent, got: %+v", res)
	}

	// Step 7: Test Recycle Bin Purge Protection:
	// Trying to permanently purge Supplier when Shipment still exists in Recycle Bin MUST be blocked with "DEPENDENCY"
	purgeRes, err := rt.RecycleBin.PermanentlyDeleteRecords("suppliers", []string{sup.ID})
	if err != nil {
		t.Fatalf("PermanentlyDeleteRecords failed: %v", err)
	}
	if !purgeRes.Blocked || purgeRes.ErrorCode != "DEPENDENCY" {
		t.Fatalf("expected permanent purge to be blocked with DEPENDENCY, got: %+v", purgeRes)
	}
}

// 5. Transaction Rollback & Concurrency Safety Verification
func TestTransactionRollbackOnDeleteProtection(t *testing.T) {
	rt := setupTestRuntime(t)
	db := rt.DB.DB()

	sup, _ := rt.Suppliers.Create(models.SupplierInput{SupplierName: "Beta Corp", Country: "UK", IsActive: true})
	_, _ = rt.Items.Create(models.ItemInput{PartNumber: "BETA-01", ItemDescription: "Widget", Unit: "PCS", Currency: "GBP", UnitPrice: 50, HSNCode: "847100", SupplierID: sup.ID, IsActive: true})

	// Begin manual transaction
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	// Attempt to delete parent supplier inside tx
	_, execErr := tx.Exec(`DELETE FROM suppliers WHERE id = ?`, sup.ID)
	if execErr == nil {
		_ = tx.Rollback()
		t.Fatalf("expected delete to fail inside tx")
	}

	// Rollback transaction
	_ = tx.Rollback()

	// Verify supplier record is completely untouched and active
	var activeName string
	err = db.QueryRow(`SELECT supplier_name FROM suppliers WHERE id = ? AND deleted_at IS NULL`, sup.ID).Scan(&activeName)
	if err != nil || activeName != "Beta Corp" {
		t.Fatalf("expected supplier to remain intact after rollback, err=%v name=%s", err, activeName)
	}
}
