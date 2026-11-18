package transactions_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
)

func TestCopyAttachedTablesWithDeleteProtectionTriggers(t *testing.T) {
	ctx := context.Background()

	// 1. Setup Source DB (Backup) with data
	srcDir := t.TempDir()
	srcDBPath := filepath.Join(srcDir, "source.db")
	srcMgr, err := connection.OpenEncrypted(srcDBPath, "src-key-123456789012345678901234")
	if err != nil {
		t.Fatalf("open src db: %v", err)
	}
	defer srcMgr.Close()

	if _, err := migrations.Run(ctx, srcMgr.DB(), "1.0.0", nil); err != nil {
		t.Fatalf("run src migrations: %v", err)
	}

	// Seed source data
	_, err = srcMgr.DB().Exec(`
		INSERT INTO suppliers (id, supplier_name, country, email, is_active) VALUES ('SUP-SRC-1', 'Source Supplier', 'DE', 'src@test.de', 1);
		INSERT INTO items (id, part_number, item_description, unit, currency, unit_price, hsn_code, supplier_id, is_active)
			VALUES ('ITM-SRC-1', 'PART-SRC', 'Source Part', 'PCS', 'EUR', 100.0, '84099990', 'SUP-SRC-1', 1);
		INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, status)
			VALUES ('SHP-SRC-1', 'SUP-SRC-1', 'INV-SRC-1', '2026-03-01', 'PARTS', 50000, 'EUR', 'FOB', 'docs-rcvd');
		INSERT INTO invoices (id, shipment_id, status)
			VALUES ('INV-SRC-1', 'SHP-SRC-1', 'Draft');
		INSERT INTO invoice_line_items (id, invoice_id, item_id, quantity, unit_price)
			VALUES ('INV-LN-SRC', 'INV-SRC-1', 'ITM-SRC-1', 500, 100.0);
		INSERT INTO boe_calculations (id, shipment_id, boe_id, supplier_name, invoice_number, status, form_values_json, item_inputs_json, calculation_result_json)
			VALUES ('BOE-SRC-1', 'SHP-SRC-1', 'BOE-DET-SRC', 'Source Supplier', 'INV-SRC-1', 'Computed', '{}', '{}', '{}');
		INSERT INTO boe_items (id, boe_calculation_id, item_id, assessable_value, bcd_rate, sws_rate, igst_rate, total)
			VALUES ('BOE-ITM-SRC', 'BOE-SRC-1', 'ITM-SRC-1', 50000, 7.5, 10.0, 18.0, 5000);
	`)
	if err != nil {
		t.Fatalf("seed src data: %v", err)
	}

	// 2. Setup Destination DB (Live DB) with EXISTING DIFFERENT data and active delete triggers!
	dstDir := t.TempDir()
	dstDBPath := filepath.Join(dstDir, "dest.db")
	dstMgr, err := connection.OpenEncrypted(dstDBPath, "dst-key-123456789012345678901234")
	if err != nil {
		t.Fatalf("open dst db: %v", err)
	}
	defer dstMgr.Close()

	if _, err := migrations.Run(ctx, dstMgr.DB(), "1.0.0", nil); err != nil {
		t.Fatalf("run dst migrations: %v", err)
	}

	// Seed destination data that references each other (would trigger delete restrictions if cleared out of order)
	_, err = dstMgr.DB().Exec(`
		INSERT INTO suppliers (id, supplier_name, country, email, is_active) VALUES ('SUP-DST-1', 'Dest Supplier', 'JP', 'dst@test.jp', 1);
		INSERT INTO items (id, part_number, item_description, unit, currency, unit_price, hsn_code, supplier_id, is_active)
			VALUES ('ITM-DST-1', 'PART-DST', 'Dest Part', 'PCS', 'JPY', 200.0, '84099990', 'SUP-DST-1', 1);
		INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, status)
			VALUES ('SHP-DST-1', 'SUP-DST-1', 'INV-DST-1', '2026-03-01', 'PARTS', 10000, 'JPY', 'FOB', 'docs-rcvd');
		INSERT INTO invoices (id, shipment_id, status)
			VALUES ('INV-DST-1', 'SHP-DST-1', 'Draft');
		INSERT INTO invoice_line_items (id, invoice_id, item_id, quantity, unit_price)
			VALUES ('INV-LN-DST', 'INV-DST-1', 'ITM-DST-1', 200, 200.0);
		INSERT INTO boe_calculations (id, shipment_id, boe_id, supplier_name, invoice_number, status, form_values_json, item_inputs_json, calculation_result_json)
			VALUES ('BOE-DST-1', 'SHP-DST-1', 'BOE-DET-DST', 'Dest Supplier', 'INV-DST-1', 'Computed', '{}', '{}', '{}');
		INSERT INTO boe_items (id, boe_calculation_id, item_id, assessable_value, bcd_rate, sws_rate, igst_rate, total)
			VALUES ('BOE-ITM-DST', 'BOE-DST-1', 'ITM-DST-1', 20000, 7.5, 10.0, 18.0, 2000);
	`)
	if err != nil {
		t.Fatalf("seed dst data: %v", err)
	}

	// 3. Execute CopyAttachedTables with ReplaceExisting = true
	res, err := transactions.CopyAttachedTables(ctx, dstMgr.DB(), srcDBPath, "src-key-123456789012345678901234", false, transactions.CopyAttachedOptions{
		ReplaceExisting: true,
		ExcludeTables:   []string{"next_schema_migrations", "schema_migrations"},
	})
	if err != nil {
		t.Fatalf("CopyAttachedTables failed: %v", err)
	}

	if res.ForeignKeyErrors > 0 {
		t.Fatalf("expected 0 foreign key errors, got %d", res.ForeignKeyErrors)
	}
	if res.IntegrityCheck != "ok" {
		t.Fatalf("expected integrity check ok, got %s", res.IntegrityCheck)
	}

	// 4. Verify the destination has the restored source data
	var srcSupCount, dstSupCount int
	_ = dstMgr.DB().QueryRow("SELECT COUNT(*) FROM suppliers WHERE id = 'SUP-SRC-1'").Scan(&srcSupCount)
	_ = dstMgr.DB().QueryRow("SELECT COUNT(*) FROM suppliers WHERE id = 'SUP-DST-1'").Scan(&dstSupCount)
	if srcSupCount != 1 || dstSupCount != 0 {
		t.Fatalf("expected 1 src supplier and 0 dst supplier, got src=%d dst=%d", srcSupCount, dstSupCount)
	}

	var srcBoeCount, dstBoeCount int
	_ = dstMgr.DB().QueryRow("SELECT COUNT(*) FROM boe_calculations WHERE id = 'BOE-SRC-1'").Scan(&srcBoeCount)
	_ = dstMgr.DB().QueryRow("SELECT COUNT(*) FROM boe_calculations WHERE id = 'BOE-DST-1'").Scan(&dstBoeCount)
	if srcBoeCount != 1 || dstBoeCount != 0 {
		t.Fatalf("expected 1 src boe and 0 dst boe, got src=%d dst=%d", srcBoeCount, dstBoeCount)
	}
}
