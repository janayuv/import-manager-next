package supplier_test

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
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/supplier"
)

func setupSupplierSvc(t *testing.T) (*supplier.Service, *security.Service, *sql.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "sup_test.db")
	mgr, err := connection.OpenEncrypted(dbPath, "test-key-32-chars-minimum-123456")
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
	return supplier.NewService(db, secSvc, auditSvc), secSvc, db
}

func TestSupplierCRUDDuplicateAndCSV(t *testing.T) {
	svc, _, _ := setupSupplierSvc(t)

	created, err := svc.Create(models.SupplierInput{
		SupplierName: "Nordic Precision GmbH",
		ShortName:    "NORDIC",
		Country:      "Germany",
		Email:        "orders@nordic.example",
		Phone:        "+49-30-1234",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	if !strings.HasPrefix(created.ID, "Sup-") {
		t.Fatalf("expected Sup-XXX id, got %q", created.ID)
	}

	// Duplicate block (case-insensitive)
	if _, err := svc.Create(models.SupplierInput{
		SupplierName: "  nordic precision gmbh ",
		Country:      "Germany",
		IsActive:     true,
	}); err == nil {
		t.Fatalf("expected duplicate supplier error")
	}

	// RFC 4180 CSV with quoted commas inside address/name + intra-batch duplicate
	csvPayload := strings.Join([]string{
		"supplierName,shortName,country,email,phone,bankAddress,isActive",
		`"Global Parts, Co.",GPC,Japan,gp@example.com,+81-3-1111,"Tokyo, Chiyoda-ku, 100-0001",true`,
		`"Global Parts, Co.",GPC2,Japan,gp2@example.com,+81-3-2222,"Osaka",true`,
		`"Missing Country",MC,,bad@example.com,,,true`,
	}, "\n")

	res, err := svc.ImportCSV(csvPayload)
	if err != nil {
		t.Fatalf("import csv: %v", err)
	}
	if res.ImportedRows != 1 || res.SkippedRows != 2 {
		t.Fatalf("unexpected csv result: %+v", res)
	}

	page, err := svc.List(models.SupplierPageRequest{Search: "Global Parts", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list suppliers: %v", err)
	}
	if page.TotalCount != 1 || page.Items[0].BankAddress != "Tokyo, Chiyoda-ku, 100-0001" {
		t.Fatalf("unexpected listed supplier: %+v", page)
	}

	// Soft delete excludes from default list
	if err := svc.Delete(page.Items[0].ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	pageAfter, err := svc.List(models.SupplierPageRequest{Search: "Global Parts", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if pageAfter.TotalCount != 0 {
		t.Fatalf("expected deleted supplier hidden from list, got %d", pageAfter.TotalCount)
	}
}

func TestSupplierGetHistoryJoinsShipmentsForInvoices(t *testing.T) {
	svc, _, db := setupSupplierSvc(t)

	sup, err := svc.Create(models.SupplierInput{
		SupplierName: "History Test Vendor GmbH",
		ShortName:    "HTV",
		Country:      "Germany",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, status)
		VALUES ('SHP-HIST-1', ?, 'INV-HIST-01', '2026-11-10', 'Filters', 12500.50, 'USD', 'CIF', 'in-transit')`,
		sup.ID,
	); err != nil {
		t.Fatalf("insert shipment: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO invoices (id, shipment_id, status)
		VALUES ('INV-HIST-1', 'SHP-HIST-1', 'finalized')`,
	); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}

	hist, err := svc.GetHistory(sup.ID)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if hist.ShipmentCount != 1 || hist.InvoiceCount != 1 || hist.TotalValue != 12500.50 {
		t.Fatalf("unexpected supplier history summary: %+v", hist)
	}
	if len(hist.RecentRecords) != 1 || hist.RecentRecords[0].ShipmentID != "SHP-HIST-1" {
		t.Fatalf("unexpected recent records: %+v", hist.RecentRecords)
	}
}
