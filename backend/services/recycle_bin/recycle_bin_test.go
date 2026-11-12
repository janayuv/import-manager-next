package recycle_bin_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/models"
)

func setupRecycleEnv(t *testing.T) *bootstrap.Runtime {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "recycle-test-key-0123456789abcdef01")
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

func TestRecycleBinMissingParentAndDependencyPurgeGuards(t *testing.T) {
	rt := setupRecycleEnv(t)

	tables, err := rt.RecycleBin.GetSoftDeleteTables()
	if err != nil || len(tables) != 11 {
		t.Fatalf("expected 11 allowlisted tables, got %d err=%v", len(tables), err)
	}

	sup, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Recycle Vendor",
		Country:      "USA",
		Email:        "vendor@recycle.com",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	shp, err := rt.Shipments.Create(models.ShipmentInput{
		SupplierID:      sup.ID,
		InvoiceNumber:   "INV-REC-1",
		InvoiceDate:     "2026-10-01",
		GoodsCategory:   "Electronics",
		InvoiceValue:    5000,
		InvoiceCurrency: "USD",
		Incoterm:        "FOB",
		ShipmentMode:    "AIR",
		ShipmentType:    "STANDARD",
	})
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	if _, err := rt.DB.DB().Exec(`UPDATE shipments SET deleted_at = '2020-01-01T00:00:00Z', deleted_by = 'admin' WHERE id = ?`, shp.ID); err != nil {
		t.Fatalf("soft-delete shipment: %v", err)
	}
	if _, err := rt.DB.DB().Exec(`UPDATE suppliers SET deleted_at = '2020-01-01T00:00:00Z', deleted_by = 'admin' WHERE id = ?`, sup.ID); err != nil {
		t.Fatalf("soft-delete supplier: %v", err)
	}

	cnt, err := rt.RecycleBin.GetRecycleBinDeletedCount()
	if err != nil || cnt != 2 {
		t.Fatalf("expected 2 deleted records, got %d err=%v", cnt, err)
	}

	purgeBlocked, err := rt.RecycleBin.PermanentlyDeleteRecords("suppliers", []string{sup.ID})
	if err != nil {
		t.Fatalf("PermanentlyDeleteRecords returned unexpected error: %v", err)
	}
	if !purgeBlocked.Blocked || purgeBlocked.ErrorCode != "DEPENDENCY" || !strings.HasPrefix(purgeBlocked.Message, "DEPENDENCY:") {
		t.Fatalf("expected DEPENDENCY blocker, got %+v", purgeBlocked)
	}

	restoreBlocked, err := rt.RecycleBin.RestoreDeletedRecords("shipments", []string{shp.ID})
	if err != nil {
		t.Fatalf("RestoreDeletedRecords failed: %v", err)
	}
	if !restoreBlocked.Blocked || restoreBlocked.ErrorCode != "missing_parent" || len(restoreBlocked.MissingParents) != 1 || restoreBlocked.MissingParents[0].TableName != "suppliers" {
		t.Fatalf("expected missing_parent blocker for suppliers, got %+v", restoreBlocked)
	}

	supRestore, err := rt.RecycleBin.RestoreDeletedRecords("suppliers", []string{sup.ID})
	if err != nil || supRestore.Blocked || supRestore.ProcessedCount != 1 {
		t.Fatalf("supplier restore failed: %+v err=%v", supRestore, err)
	}
	shpRestore, err := rt.RecycleBin.RestoreDeletedRecords("shipments", []string{shp.ID})
	if err != nil || shpRestore.Blocked || shpRestore.ProcessedCount != 1 {
		t.Fatalf("shipment restore failed after parent restored: %+v err=%v", shpRestore, err)
	}

	if _, err := rt.RecycleBin.GetDeletedRecords("users; DROP TABLE suppliers;--", "", 1, 50); err == nil {
		t.Fatal("expected invalid table rejection")
	}
}
