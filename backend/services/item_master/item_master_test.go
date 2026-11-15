package item_master_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/item_master"
	"github.com/janayuv/import-manager-next/backend/services/options"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/supplier"
)

func setupItemSvc(t *testing.T) (*item_master.Service, *supplier.Service, *options.Service) {
	t.Helper()
	tmp := t.TempDir()
	mgr, err := connection.OpenEncrypted(filepath.Join(tmp, "item_test.db"), "test-key-32-chars-minimum-123456")
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
	return item_master.NewService(db, secSvc, auditSvc, filepath.Join(tmp, "att")),
		supplier.NewService(db, secSvc, auditSvc),
		options.NewService(db, secSvc, auditSvc)
}

func TestItemMasterCRUDCSVAndOptions(t *testing.T) {
	itemSvc, supSvc, optSvc := setupItemSvc(t)

	opt, err := optSvc.UpsertOption("units", "PCS", "Pieces")
	if err != nil || opt.Label != "Pieces" {
		t.Fatalf("upsert option failed: %+v err=%v", opt, err)
	}
	allOpts, err := optSvc.ListAllOptions()
	if err != nil || len(allOpts) != 14 {
		t.Fatalf("expected 14 lookup option tables, got %d err=%v", len(allOpts), err)
	}

	sup, err := supSvc.Create(models.SupplierInput{
		SupplierName: "Delta Components",
		Country:      "USA",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	item, err := itemSvc.Create(models.ItemInput{
		PartNumber:      "PN-100",
		ItemDescription: "Hydraulic Valve",
		Unit:            "PCS",
		Currency:        "usd",
		UnitPrice:       42.5,
		HSNCode:         "84812000",
		SupplierID:      sup.ID,
		IsActive:        true,
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if !strings.HasPrefix(item.ID, "ITM-") || item.Currency != "USD" {
		t.Fatalf("unexpected created item: %+v", item)
	}

	// Duplicate partNumber rejected
	if _, err := itemSvc.Create(models.ItemInput{
		PartNumber:      "pn-100",
		ItemDescription: "Duplicate",
		Unit:            "PCS",
		Currency:        "USD",
		UnitPrice:       10,
	}); err == nil {
		t.Fatalf("expected duplicate partNumber error")
	}

	// CSV resolves supplierName -> supplierId automatically and warns on unknown supplierName
	csvData := strings.Join([]string{
		"partNumber,itemDescription,unit,currency,unitPrice,hsnCode,supplierName,isActive",
		"PN-200,Sensor Probe,PCS,USD,18.75,90318000,Delta Components,true",
		"PN-300,Brass Fitting,PCS,EUR,4.20,74122000,Unknown Vendor,false",
		"PN-200,Dup Inside Batch,PCS,USD,10,90318000,Delta Components,true",
	}, "\n")

	imp, err := itemSvc.ImportCSV(csvData)
	if err != nil {
		t.Fatalf("import csv: %v", err)
	}
	if imp.ImportedRows != 2 || imp.SkippedRows != 1 || len(imp.Warnings) != 1 {
		t.Fatalf("unexpected item import result: %+v", imp)
	}

	exported, err := itemSvc.ExportCSV()
	if err != nil {
		t.Fatalf("export csv: %v", err)
	}
	if !strings.Contains(exported, "PN-200") || !strings.Contains(exported, "PN-300") {
		t.Fatalf("exported CSV missing expected part numbers")
	}
}
