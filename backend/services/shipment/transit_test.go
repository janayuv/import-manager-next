package shipment_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/shipment"
	"github.com/janayuv/import-manager-next/backend/services/supplier"
)

func TestComputeTransitStatusAndDaysRules(t *testing.T) {
	today := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	// Case 1: In Transit (Future ETA, no ATA)
	status, days := shipment.ComputeTransitStatusAndDays("2026-11-10", "2026-11-25", "", "in-transit", today)
	if status != "In Transit" {
		t.Errorf("expected 'In Transit', got %q", status)
	}
	if days == nil || *days != 15 {
		t.Errorf("expected 15 transit days, got %v", days)
	}

	// Case 2: Delayed (Past ETA, no ATA)
	status, days = shipment.ComputeTransitStatusAndDays("2026-11-01", "2026-11-15", "", "in-transit", today)
	if status != "Delayed" {
		t.Errorf("expected 'Delayed', got %q", status)
	}
	if days == nil || *days != 14 {
		t.Errorf("expected 14 transit days, got %v", days)
	}

	// Case 3: Arrived (ATA / dateOfDelivery entered)
	status, days = shipment.ComputeTransitStatusAndDays("2026-11-01", "2026-11-15", "2026-11-16", "docs-rcvd", today)
	if status != "Arrived" {
		t.Errorf("expected 'Arrived', got %q", status)
	}
	if days == nil || *days != 14 {
		t.Errorf("expected 14 transit days, got %v", days)
	}

	// Case 4: Pending (Missing ETA)
	status, days = shipment.ComputeTransitStatusAndDays("2026-11-10", "", "", "docs-rcvd", today)
	if status != "Pending" {
		t.Errorf("expected 'Pending', got %q", status)
	}
	if days != nil {
		t.Errorf("expected nil transit days for missing ETA, got %v", *days)
	}

	// Case 5: Delivered status automatically marks Arrived
	status, _ = shipment.ComputeTransitStatusAndDays("2026-11-10", "2026-11-25", "", "delivered", today)
	if status != "Arrived" {
		t.Errorf("expected 'Arrived' for delivered status, got %q", status)
	}
}

func TestTransitShipmentReportAndCSVExport(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "transit_test.db")
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
	sup, err := supSvc.Create(models.SupplierInput{SupplierName: "Global Logistics Ltd", Country: "Singapore", IsActive: true})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	svc := shipment.NewService(db, secSvc, auditSvc)

	// Create 4 test shipments covering each status
	// 1: In Transit (future ETA)
	_, err = svc.Create(models.ShipmentInput{
		SupplierID:    sup.ID,
		InvoiceNumber: "INV-TRANSIT-01",
		InvoiceDate:   "2026-11-10",
		GoodsCategory: "RAW",
		InvoiceValue:  10000,
		InvoiceCurrency: "USD",
		Incoterm:      "FOB",
		VesselName:    "EVER GIVEN V.102",
		ETD:           "2026-11-10",
		ETA:           "2026-12-05", // 25 days transit
		Status:        "in-transit",
	})
	if err != nil {
		t.Fatalf("create shipment 1: %v", err)
	}

	// 2: Delayed (past ETA)
	_, err = svc.Create(models.ShipmentInput{
		SupplierID:    sup.ID,
		InvoiceNumber: "INV-DELAYED-02",
		InvoiceDate:   "2026-10-01",
		GoodsCategory: "CAPITAL",
		InvoiceValue:  25000,
		InvoiceCurrency: "USD",
		Incoterm:      "CIF",
		VesselName:    "MAERSK MC-KINNEY",
		ETD:           "2026-10-05",
		ETA:           "2026-10-20", // 15 days transit, past date
		Status:        "in-transit",
	})
	if err != nil {
		t.Fatalf("create shipment 2: %v", err)
	}

	// 3: Arrived (dateOfDelivery populated)
	_, err = svc.Create(models.ShipmentInput{
		SupplierID:     sup.ID,
		InvoiceNumber:  "INV-ARRIVED-03",
		InvoiceDate:    "2026-10-15",
		GoodsCategory:  "COMPONENTS",
		InvoiceValue:   15000,
		InvoiceCurrency: "EUR",
		Incoterm:       "DAP",
		VesselName:     "CMA CGM MARCO POLO",
		ETD:            "2026-10-16",
		ETA:            "2026-11-05", // 20 days transit
		DateOfDelivery: "2026-11-06",
		Status:         "delivered",
	})
	if err != nil {
		t.Fatalf("create shipment 3: %v", err)
	}

	// 4: Pending (no ETA)
	_, err = svc.Create(models.ShipmentInput{
		SupplierID:    sup.ID,
		InvoiceNumber: "INV-PENDING-04",
		InvoiceDate:   "2026-11-15",
		GoodsCategory: "RAW",
		InvoiceValue:  5000,
		InvoiceCurrency: "USD",
		Incoterm:      "FOB",
		Status:        "docs-rcvd",
	})
	if err != nil {
		t.Fatalf("create shipment 4: %v", err)
	}

	// Query Transit Shipment Report
	report, err := svc.GetTransitShipmentReport(models.TransitShipmentFilter{})
	if err != nil {
		t.Fatalf("get transit report: %v", err)
	}

	if report.Summary.TotalShipments != 4 {
		t.Errorf("expected 4 total shipments, got %d", report.Summary.TotalShipments)
	}
	if report.Summary.InTransit != 1 {
		t.Errorf("expected 1 In Transit, got %d", report.Summary.InTransit)
	}
	if report.Summary.Delayed != 1 {
		t.Errorf("expected 1 Delayed, got %d", report.Summary.Delayed)
	}
	if report.Summary.Arrived != 1 {
		t.Errorf("expected 1 Arrived, got %d", report.Summary.Arrived)
	}
	if report.Summary.Pending != 1 {
		t.Errorf("expected 1 Pending, got %d", report.Summary.Pending)
	}

	// Average planned transit: (25 + 15 + 20) / 3 = 60 / 3 = 20.0 days
	if report.Summary.AveragePlannedTransit != 20.0 {
		t.Errorf("expected AveragePlannedTransit 20.0, got %.1f", report.Summary.AveragePlannedTransit)
	}

	if len(report.Items) != 4 {
		t.Errorf("expected 4 items, got %d", len(report.Items))
	}

	// Test status filter
	filteredReport, err := svc.GetTransitShipmentReport(models.TransitShipmentFilter{Status: "in-transit"})
	if err != nil {
		t.Fatalf("get filtered transit report: %v", err)
	}
	if len(filteredReport.Items) != 1 || filteredReport.Items[0].InvoiceNumber != "INV-TRANSIT-01" {
		t.Errorf("expected 1 in-transit item INV-TRANSIT-01, got %+v", filteredReport.Items)
	}

	// Test CSV Export
	csvContent, err := svc.ExportTransitShipmentsCSV(models.TransitShipmentFilter{})
	if err != nil {
		t.Fatalf("export csv: %v", err)
	}

	// Verify template components
	if !strings.HasPrefix(csvContent, "\xEF\xBB\xBF") {
		t.Errorf("expected UTF-8 BOM prefix")
	}
	if !strings.Contains(csvContent, "IMPORT SHIPMENT TRANSIT DETAILS") {
		t.Errorf("missing title 'IMPORT SHIPMENT TRANSIT DETAILS'")
	}
	if !strings.Contains(csvContent, "S.No.,Shipper / Supplier,Invoice No.,Invoice Date,Vessel / Voyage,ETD,ETA,Transit Period (Days),Shipment Status,Remarks") {
		t.Errorf("missing table column headers")
	}
	if !strings.Contains(csvContent, "INV-TRANSIT-01") {
		t.Errorf("missing shipment invoice number INV-TRANSIT-01")
	}
	if !strings.Contains(csvContent, "IMPORT SHIPMENT OVERVIEW") {
		t.Errorf("missing 'IMPORT SHIPMENT OVERVIEW'")
	}
	if !strings.Contains(csvContent, "Shipment Status Breakdown") {
		t.Errorf("missing 'Shipment Status Breakdown'")
	}
	if !strings.Contains(csvContent, "HOW TO USE THIS TEMPLATE") {
		t.Errorf("missing 'HOW TO USE THIS TEMPLATE'")
	}
	if !strings.Contains(csvContent, "Note,Transit period is calendar days (not working days).") {
		t.Errorf("missing template note")
	}
}
