package expenses_test

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/expenses"
)

func setupExpenseEnv(t *testing.T) *bootstrap.Runtime {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "expenses-test-secret-0123456789abcdef")
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

func TestPaiseCalculatorPrecisionAndMerge(t *testing.T) {
	line, err := expenses.CalcLinePaise(models.ExpenseLineInput{
		ExpenseTypeID: "ET-1",
		AmountPaise:   1500033, // ₹15,000.33
		CGSTRateBP:    900,     // 9%
		SGSTRateBP:    900,     // 9%
		IGSTRateBP:    0,
		TDSRateBP:     200, // 2%
	})
	if err != nil {
		t.Fatalf("CalcLinePaise failed: %v", err)
	}
	if line.CGSTPaise != (1500033*900)/10000 || line.SGSTPaise != (1500033*900)/10000 || line.TDSPaise != (1500033*200)/10000 {
		t.Fatalf("unexpected integer paise math: %+v", line)
	}
	if line.TotalPaise != line.AmountPaise+line.CGSTPaise+line.SGSTPaise+line.IGSTPaise {
		t.Fatalf("totalPaise mismatch: %+v", line)
	}
	if line.NetPaise != line.TotalPaise-line.TDSPaise {
		t.Fatalf("netPaise mismatch: %+v", line)
	}

	merged, err := expenses.MergeDuplicateLines([]models.ExpenseLineInput{
		{ExpenseTypeID: "ET-1", AmountPaise: 50000, CGSTRateBP: 900, SGSTRateBP: 900, TDSRateBP: 200, Remarks: "Part A"},
		{ExpenseTypeID: "ET-1", AmountPaise: 70000, CGSTRateBP: 900, SGSTRateBP: 900, TDSRateBP: 200, Remarks: "Part B"},
	})
	if err != nil {
		t.Fatalf("MergeDuplicateLines failed: %v", err)
	}
	if len(merged) != 1 || merged[0].AmountPaise != 120000 {
		t.Fatalf("expected merged line 120000 paise, got %+v", merged)
	}
}

func TestExpensesIdempotencyOptimisticLockReportsAndLandedCost(t *testing.T) {
	rt := setupExpenseEnv(t)

	// 1. Seed supplier, 2 items, shipment, finalized invoice, and BOE calc
	sup, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Bosch GmbH",
		ShortName:    "BOSCH",
		Country:      "Germany",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	item1, err := rt.Items.Create(models.ItemInput{
		PartNumber:      "PN-100",
		ItemDescription: "High Pressure Pump",
		HSNCode:         "84133010",
		Unit:            "PCS",
		UnitPrice:       100,
		Currency:        "EUR",
		SupplierID:      sup.ID,
		IsActive:        true,
		BCD:             "7.5",
		SWS:             "10",
		IGST:            "18",
	})
	if err != nil {
		t.Fatalf("create item1: %v", err)
	}
	item2, err := rt.Items.Create(models.ItemInput{
		PartNumber:      "PN-200",
		ItemDescription: "Control Valve",
		HSNCode:         "84818090",
		Unit:            "PCS",
		UnitPrice:       50,
		Currency:        "EUR",
		SupplierID:      sup.ID,
		IsActive:        true,
		BCD:             "7.5",
		SWS:             "10",
		IGST:            "18",
	})
	if err != nil {
		t.Fatalf("create item2: %v", err)
	}

	sh, err := rt.Shipments.Create(models.ShipmentInput{
		SupplierID:      sup.ID,
		InvoiceNumber:   "INV-LDC-1",
		InvoiceDate:     "2026-04-15",
		GoodsCategory:   "Machinery",
		InvoiceValue:    300,
		InvoiceCurrency: "EUR",
		Incoterm:        "CIF",
		ShipmentMode:    "Air",
		ShipmentType:    "Dutiable",
	})
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	if _, err := rt.InvoiceWizard.SaveInvoice(models.SaveInvoiceRequest{
		ShipmentID:           sh.ID,
		Finalize:             true,
		LineTotalDecimals:    2,
		InvoiceTotalDecimals: 2,
		Lines: []models.InvoiceLineInput{
			{ItemID: item1.ID, Quantity: 2, UnitPrice: 100, DutyPercent: 7.5, SWSPercent: 10, IGSTPercent: 18}, // value 200
			{ItemID: item2.ID, Quantity: 2, UnitPrice: 50, DutyPercent: 7.5, SWSPercent: 10, IGSTPercent: 18},  // value 100
		},
	}); err != nil {
		t.Fatalf("SaveInvoice failed: %v", err)
	}

	ctx, err := rt.BOEEntry.LoadShipmentContext(sh.ID)
	if err != nil {
		t.Fatalf("LoadShipmentContext failed: %v", err)
	}
	ctx.Params.ExchangeRate = 90.0 // AV: item1 = 18000, item2 = 9000, total AV = 27000
	if _, err := rt.BOEEntry.SaveCalculation(ctx.Params); err != nil {
		t.Fatalf("SaveCalculation failed: %v", err)
	}

	// 2. Service Provider & Expense Types
	sp, err := rt.Expenses.CreateServiceProvider(models.ServiceProviderInput{
		Name:  "DHL Express India",
		GSTIN: "29AAACD1234F1Z5",
		State: "Karnataka",
	})
	if err != nil {
		t.Fatalf("CreateServiceProvider: %v", err)
	}
	etClear, err := rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{
		Name:              "Customs Clearance",
		DefaultCGSTRateBP: 900,
		DefaultSGSTRateBP: 900,
		DefaultTDSRateBP:  200,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("CreateExpenseType: %v", err)
	}
	etFreight, err := rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{
		Name:              "Local Cartage",
		DefaultIGSTRateBP: 1800,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("CreateExpenseType 2: %v", err)
	}

	// 3. Idempotent CreateExpenseInvoice with MergeDuplicates
	req := models.ExpenseInvoiceRequest{
		ShipmentID:        sh.ID,
		ServiceProviderID: sp.ID,
		InvoiceNo:         "DHL-9001",
		InvoiceDate:       "2026-04-18",
		Currency:          "INR",
		IdempotencyKey:    "idem-key-dhl-9001",
		MergeDuplicates:   true,
		Lines: []models.ExpenseLineInput{
			{ExpenseTypeID: etClear.ID, AmountPaise: 600000, CGSTRateBP: 900, SGSTRateBP: 900, TDSRateBP: 200, Remarks: "Fee 1"},
			{ExpenseTypeID: etClear.ID, AmountPaise: 300000, CGSTRateBP: 900, SGSTRateBP: 900, TDSRateBP: 200, Remarks: "Fee 2"},
		},
	}
	first, err := rt.Expenses.CreateExpenseInvoice(req)
	if err != nil {
		t.Fatalf("CreateExpenseInvoice failed: %v", err)
	}
	second, err := rt.Expenses.CreateExpenseInvoice(req)
	if err != nil {
		t.Fatalf("idempotent retry failed: %v", err)
	}
	if first.ID != second.ID || first.BaseAmountPaise != 900000 {
		t.Fatalf("idempotency or merge failed: first=%+v second=%+v", first, second)
	}

	// 4. Optimistic locking + UpdateExpenseInvoice
	stale := req
	stale.ID = first.ID
	stale.Version = 99
	if _, err := rt.Expenses.UpdateExpenseInvoice(stale); err == nil {
		t.Fatalf("expected optimistic lock conflict error")
	}
	updateReq := models.ExpenseInvoiceRequest{
		ID:                first.ID,
		ShipmentID:        sh.ID,
		ServiceProviderID: sp.ID,
		InvoiceNo:         "DHL-9001",
		InvoiceDate:       "2026-04-18",
		Currency:          "INR",
		Version:           first.Version,
		Lines: []models.ExpenseLineInput{
			{ExpenseTypeID: etClear.ID, AmountPaise: 600000, CGSTRateBP: 900, SGSTRateBP: 900, TDSRateBP: 200},
			{ExpenseTypeID: etFreight.ID, AmountPaise: 300000, IGSTRateBP: 1800},
		},
	}
	updated, err := rt.Expenses.UpdateExpenseInvoice(updateReq)
	if err != nil {
		t.Fatalf("UpdateExpenseInvoice failed: %v", err)
	}
	if updated.Version != 2 || updated.BaseAmountPaise != 900000 {
		t.Fatalf("unexpected updated version/base: %+v", updated)
	}

	// 5. All 5 Expense Report views
	for _, view := range []string{"detailed", "summary-by-type", "summary-by-provider", "summary-by-shipment", "summary-by-month"} {
		rep, err := rt.ExpenseReports.GetExpenseReport(models.ExpenseReportFilter{ViewMode: view})
		if err != nil {
			t.Fatalf("GetExpenseReport %s failed: %v", view, err)
		}
		if rep.TotalBasePaise != 900000 || rep.TotalLines != 2 {
			t.Fatalf("view %s unexpected totals: %+v", view, rep)
		}
		if view != "detailed" && len(rep.GroupedRows) == 0 {
			t.Fatalf("view %s expected grouped rows", view)
		}
	}

	// 6. Landed Cost Report proportional allocation (total basic expenses = 9000.00 INR; AV ratio 2:1 -> 6000 & 3000; excludes IGST!)
	lc, err := rt.Reports.GetLandedCostReport(models.LandedCostFilter{})
	if err != nil {
		t.Fatalf("GetLandedCostReport failed: %v", err)
	}
	if len(lc.Rows) != 2 {
		t.Fatalf("expected 2 landed cost rows, got %d (%+v)", len(lc.Rows), lc.Rows)
	}
	if math.Abs(lc.Summary.TotalBasicExpenses-9000.0) > 0.02 {
		t.Fatalf("expected proportional allocated basic expenses sum to 9000.00, got %.4f", lc.Summary.TotalBasicExpenses)
	}
	for _, row := range lc.Rows {
		expectedLDC := (row.AssessableValue + row.BCDAmount + row.SWSAmount + row.ExpensesTotal) / row.Qty
		if math.Abs(row.LDCPerQty-expectedLDC) > 0.01 {
			t.Fatalf("LDCPerQty for %s includes unexpected IGST or math mismatch: got %.4f want %.4f (igst=%.2f)", row.PartNo, row.LDCPerQty, expectedLDC, row.IGSTAmount)
		}
	}

	csvText, err := rt.Reports.ExportLandedCostCSV(models.LandedCostFilter{})
	if err != nil {
		t.Fatalf("ExportLandedCostCSV failed: %v", err)
	}
	if !strings.Contains(csvText, "Expenses (Basic),LDC per qty") || !strings.Contains(csvText, "PN-100") {
		t.Fatalf("unexpected 14-column CSV output:\n%s", csvText)
	}

	// 7. Bulk CSV import validation & frozen shipment protection
	bulkCSV := "Expense Type,Service Provider,Invoice No,Invoice Date,Amount,CGST Amount,SGST Amount,IGST Amount,TDS Amount,Total Amount,Remarks\n" +
		"Customs Clearance,DHL Express India,DHL-9002,2026-04-20,5000.00,450.00,450.00,0,100.00,5900.00,Bulk fee\n" +
		"Unknown Head,DHL Express India,DHL-9003,2026-04-20,1000,0,0,0,0,1000,Bad row\n"
	bulkRes, err := rt.Expenses.ImportBulkCSV(sh.ID, bulkCSV)
	if err != nil {
		t.Fatalf("ImportBulkCSV failed: %v", err)
	}
	if bulkRes.ImportedRows != 1 || bulkRes.SkippedRows != 1 {
		t.Fatalf("expected 1 imported & 1 skipped, got %+v", bulkRes)
	}

	if _, err := rt.Shipments.Freeze(sh.ID); err != nil {
		t.Fatalf("freeze shipment failed: %v", err)
	}
	if _, err := rt.Expenses.CreateExpenseInvoice(models.ExpenseInvoiceRequest{
		ShipmentID:        sh.ID,
		ServiceProviderID: sp.ID,
		InvoiceNo:         "DHL-FROZEN",
		InvoiceDate:       "2026-04-21",
		Lines:             []models.ExpenseLineInput{{ExpenseTypeID: etFreight.ID, AmountPaise: 10000}},
	}); err == nil {
		t.Fatalf("expected frozen shipment to reject expense creation")
	}
}
