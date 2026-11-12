package database_management_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mutecomm/go-sqlcipher/v4"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/models"
)

func setupDBAdminEnv(t *testing.T) *bootstrap.Runtime {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "dbadmin-test-key-0123456789abcdef01")
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

func sha256FileHex(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file %s: %v", p, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func createLegacyFixtureDB(t *testing.T, path, sqlCipherKey string) {
	t.Helper()
	var db *sql.DB
	if sqlCipherKey == "" {
		var err error
		db, err = sql.Open("sqlite3", path+"?_foreign_keys=on")
		if err != nil {
			t.Fatalf("open plaintext legacy db: %v", err)
		}
		defer db.Close()
	} else {
		mgr, err := connection.OpenEncrypted(path, sqlCipherKey)
		if err != nil {
			t.Fatalf("open encrypted legacy db: %v", err)
		}
		defer mgr.Close()
		db = mgr.DB()
	}

	ddl := []string{
		`CREATE TABLE refinery_schema_history (version INTEGER PRIMARY KEY, name TEXT);`,
		`INSERT INTO refinery_schema_history VALUES (80, 'V80__final_legacy');`,
		`CREATE TABLE suppliers (id TEXT PRIMARY KEY, supplier_name TEXT NOT NULL, short_name TEXT, country TEXT NOT NULL, email TEXT NOT NULL, is_active BOOLEAN NOT NULL);`,
		`CREATE TABLE shipments (id TEXT PRIMARY KEY, supplier_id TEXT NOT NULL, invoice_number TEXT NOT NULL, invoice_date TEXT NOT NULL, goods_category TEXT NOT NULL, invoice_value REAL NOT NULL, invoice_currency TEXT NOT NULL, incoterm TEXT NOT NULL);`,
		`CREATE TABLE items (id TEXT PRIMARY KEY, part_number TEXT NOT NULL UNIQUE, item_description TEXT NOT NULL, unit TEXT NOT NULL, currency TEXT NOT NULL, unit_price REAL NOT NULL, hsn_code TEXT NOT NULL, supplier_id TEXT, is_active BOOLEAN NOT NULL);`,
		`CREATE TABLE invoices (id TEXT PRIMARY KEY, shipment_id TEXT NOT NULL, status TEXT NOT NULL);`,
		`CREATE TABLE invoice_line_items (id TEXT PRIMARY KEY, invoice_id TEXT NOT NULL, item_id TEXT NOT NULL, quantity REAL NOT NULL, unit_price REAL NOT NULL);`,
		`CREATE TABLE boe_details (id TEXT PRIMARY KEY, be_number TEXT NOT NULL, be_date TEXT NOT NULL, location TEXT NOT NULL, total_assessment_value REAL NOT NULL, duty_amount REAL NOT NULL);`,
		`CREATE TABLE service_providers (id TEXT PRIMARY KEY, name TEXT NOT NULL);`,
		`CREATE TABLE expense_types (id TEXT PRIMARY KEY, name TEXT NOT NULL);`,
		`CREATE TABLE expense_invoices (id TEXT PRIMARY KEY, shipment_id TEXT NOT NULL, service_provider_id TEXT NOT NULL, invoice_no TEXT NOT NULL, invoice_date TEXT NOT NULL, total_amount REAL NOT NULL);`,
		`CREATE TABLE expenses (id TEXT PRIMARY KEY, expense_invoice_id TEXT NOT NULL, shipment_id TEXT NOT NULL, service_provider_id TEXT NOT NULL, invoice_no TEXT NOT NULL, invoice_date TEXT NOT NULL, expense_type_id TEXT NOT NULL, amount REAL NOT NULL, amount_paise INTEGER NOT NULL, total_amount_paise INTEGER NOT NULL);`,
	}
	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("legacy ddl failed (%s): %v", stmt, err)
		}
	}

	inserts := []string{
		`INSERT INTO suppliers (id, supplier_name, short_name, country, email, is_active) VALUES ('leg-sup-1', 'Legacy Tokyo Precision', 'LTP', 'Japan', 'ltp@legacy.jp', 1);`,
		`INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm) VALUES ('leg-shp-1', 'leg-sup-1', 'LEG-INV-01', '2025-11-15', 'Machinery', 25000.50, 'USD', 'CIF');`,
		`INSERT INTO items (id, part_number, item_description, unit, currency, unit_price, hsn_code, supplier_id, is_active) VALUES ('leg-itm-1', 'LEG-PN-01', 'Precision Spindle', 'PCS', 'USD', 2500.05, '84669390', 'leg-sup-1', 1);`,
		`INSERT INTO invoices (id, shipment_id, status) VALUES ('leg-inv-1', 'leg-shp-1', 'finalized');`,
		`INSERT INTO invoice_line_items (id, invoice_id, item_id, quantity, unit_price) VALUES ('leg-ili-1', 'leg-inv-1', 'leg-itm-1', 10, 2500.05);`,
		`INSERT INTO boe_details (id, be_number, be_date, location, total_assessment_value, duty_amount) VALUES ('leg-boe-1', '8877661', '2025-11-20', 'INMAA4', 2100000, 315000.00);`,
		`INSERT INTO service_providers (id, name) VALUES ('leg-sp-1', 'Legacy CHA Logistics');`,
		`INSERT INTO expense_types (id, name) VALUES ('leg-et-1', 'Legacy Customs Clearance');`,
		`INSERT INTO expense_invoices (id, shipment_id, service_provider_id, invoice_no, invoice_date, total_amount) VALUES ('leg-ei-1', 'leg-shp-1', 'leg-sp-1', 'CHA-101', '2025-11-22', 11800.00);`,
		`INSERT INTO expenses (id, expense_invoice_id, shipment_id, service_provider_id, invoice_no, invoice_date, expense_type_id, amount, amount_paise, total_amount_paise) VALUES ('leg-exp-1', 'leg-ei-1', 'leg-shp-1', 'leg-sp-1', 'CHA-101', '2025-11-22', 'leg-et-1', 10000.00, 1000000, 1180000);`,
	}
	for _, stmt := range inserts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("legacy insert failed (%s): %v", stmt, err)
		}
	}
}

func TestDatabaseConsoleBrowseEditBulkUndoAndPinHardDelete(t *testing.T) {
	rt := setupDBAdminEnv(t)

	s1, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Console Supplier A",
		Country:      "India",
		Email:        "a@console.in",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	s2, _ := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Console Supplier B",
		Country:      "India",
		Email:        "b@console.in",
		IsActive:     true,
	})

	stats, err := rt.DatabaseManagement.GetOverviewStats()
	if err != nil {
		t.Fatalf("GetOverviewStats failed: %v", err)
	}
	if !strings.Contains(stats.IntegrityCheck, "ok") || !stats.ForeignKeysOn || stats.SchemaVersion < 5 || stats.TableCounts["suppliers"] < 2 {
		t.Fatalf("unexpected overview stats: %+v", stats)
	}

	browse, err := rt.DatabaseManagement.BrowseTable(models.BrowseTableRequest{
		TableName: "suppliers",
		Search:    "Console",
		Page:      1,
		PageSize:  25,
	})
	if err != nil || browse.Total != 2 {
		t.Fatalf("BrowseTable failed: total=%d err=%v", browse.Total, err)
	}

	if err := rt.DatabaseManagement.UpdateTableCell(models.UpdateTableCellRequest{
		TableName: "suppliers",
		RowID:     s1.ID,
		Column:    "short_name",
		Value:     "CSA",
	}); err != nil {
		t.Fatalf("UpdateTableCell failed: %v", err)
	}
	if err := rt.DatabaseManagement.UpdateTableCell(models.UpdateTableCellRequest{
		TableName: "users",
		RowID:     "any",
		Column:    "password_hash",
		Value:     "tampered",
	}); err == nil {
		t.Fatal("expected protected column update to be rejected")
	}

	softRes, err := rt.DatabaseManagement.BulkDelete(models.BulkDeleteRequest{
		TableName: "suppliers",
		RecordIDs: []string{s1.ID, s2.ID},
	})
	if err != nil || softRes.ProcessedCount != 2 || softRes.UndoToken == "" {
		t.Fatalf("BulkDelete soft failed: %+v err=%v", softRes, err)
	}
	restored, err := rt.DatabaseManagement.UndoBulkSoftDelete(softRes.UndoToken)
	if err != nil || restored != 2 {
		t.Fatalf("UndoBulkSoftDelete expected 2 restored, got %d err=%v", restored, err)
	}

	if err := rt.DatabaseManagement.SetHardDeletePin("StrongPass1!", "9876"); err != nil {
		t.Fatalf("SetHardDeletePin failed: %v", err)
	}
	if _, err := rt.DatabaseManagement.BulkDelete(models.BulkDeleteRequest{
		TableName:  "suppliers",
		RecordIDs:  []string{s2.ID},
		HardDelete: true,
		Pin:        "0000",
	}); err == nil {
		t.Fatal("expected invalid PIN rejection")
	}
	hardRes, err := rt.DatabaseManagement.BulkDelete(models.BulkDeleteRequest{
		TableName:  "suppliers",
		RecordIDs:  []string{s2.ID},
		HardDelete: true,
		Pin:        "9876",
	})
	if err != nil || hardRes.ProcessedCount != 1 {
		t.Fatalf("PIN-verified hard delete failed: %+v err=%v", hardRes, err)
	}
}

func TestLegacyMigrationPlaintextAndEncryptedNonDestructiveParity(t *testing.T) {
	rt := setupDBAdminEnv(t)

	legacyDir := t.TempDir()
	plainLegacyPath := filepath.Join(legacyDir, "legacy_plain.db")
	createLegacyFixtureDB(t, plainLegacyPath, "")
	origHash := sha256FileHex(t, plainLegacyPath)

	inspectRes, err := rt.DatabaseManagement.InspectLegacyDatabase(models.LegacyInspectRequest{
		SourcePath: plainLegacyPath,
	})
	if err != nil {
		t.Fatalf("InspectLegacyDatabase failed: %v", err)
	}
	if !inspectRes.IsPlaintextSQLite || inspectRes.IsEncrypted || !inspectRes.KeyValid || inspectRes.MigrationVersion != "80" {
		t.Fatalf("unexpected inspect metadata: %+v", inspectRes)
	}
	if inspectRes.SourceFinancials.ExpenseTotalPaise != 1180000 || inspectRes.SourceFinancials.BOETotalDutyPaise != 31500000 {
		t.Fatalf("unexpected source financials: %+v", inspectRes.SourceFinancials)
	}

	beforeStats, _ := rt.DatabaseManagement.GetOverviewStats()

	dryRep, err := rt.DatabaseManagement.RunLegacyMigration(models.LegacyMigrationRequest{
		SourcePath: plainLegacyPath,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("DryRun legacy migration failed: %v", err)
	}
	if !dryRep.DryRun || !dryRep.RowParityOK || !dryRep.FinancialParityOK || !dryRep.ForeignKeysOK {
		t.Fatalf("dry-run parity failed: %+v", dryRep)
	}
	liveStats, _ := rt.DatabaseManagement.GetOverviewStats()
	if liveStats.TableCounts["suppliers"] != beforeStats.TableCounts["suppliers"] {
		t.Fatalf("dry-run unexpectedly mutated live DB suppliers before=%d after=%d", beforeStats.TableCounts["suppliers"], liveStats.TableCounts["suppliers"])
	}

	commitRep, err := rt.DatabaseManagement.RunLegacyMigration(models.LegacyMigrationRequest{
		SourcePath: plainLegacyPath,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Commit legacy migration failed: %v", err)
	}
	if commitRep.DryRun || commitRep.SafetyBackupPath == "" || !commitRep.RowParityOK || !commitRep.FinancialParityOK || !commitRep.ForeignKeysOK {
		t.Fatalf("commit migration parity failed: %+v", commitRep)
	}
	if afterHash := sha256FileHex(t, plainLegacyPath); afterHash != origHash {
		t.Fatalf("original legacy DB checksum altered! before=%s after=%s", origHash, afterHash)
	}

	encLegacyPath := filepath.Join(legacyDir, "legacy_enc.db")
	createLegacyFixtureDB(t, encLegacyPath, "LegacySecret#42")

	wrongKeyRes, err := rt.DatabaseManagement.InspectLegacyDatabase(models.LegacyInspectRequest{
		SourcePath: encLegacyPath,
		SourceKey:  "WrongKey",
	})
	if err != nil {
		t.Fatalf("InspectLegacyDatabase encrypted wrong-key unexpected err: %v", err)
	}
	if wrongKeyRes.IsPlaintextSQLite || !wrongKeyRes.IsEncrypted || wrongKeyRes.KeyValid {
		t.Fatalf("expected encrypted + invalid key result, got %+v", wrongKeyRes)
	}

	rightKeyRes, err := rt.DatabaseManagement.InspectLegacyDatabase(models.LegacyInspectRequest{
		SourcePath: encLegacyPath,
		SourceKey:  "LegacySecret#42",
	})
	if err != nil || !rightKeyRes.KeyValid {
		t.Fatalf("expected encrypted right-key valid inspection, got %+v err=%v", rightKeyRes, err)
	}
}
