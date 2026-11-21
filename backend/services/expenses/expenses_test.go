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

	// Manual GST calculation for values with partial GST eligibility
	manualLine, err := expenses.CalcLinePaise(models.ExpenseLineInput{
		ExpenseTypeID: "ET-CHA",
		AmountPaise:   5000000, // ₹50,000 total bill
		IsManualGST:   true,
		CGSTPaise:     90000, // ₹900 (e.g. 9% only on eligible ₹10,000 portion)
		SGSTPaise:     90000, // ₹900
		IGSTPaise:     0,
		TDSRateBP:     200, // 2% TDS on ₹50,000 = ₹1,000 (100000 paise)
	})
	if err != nil {
		t.Fatalf("CalcLinePaise with manual GST failed: %v", err)
	}
	if manualLine.CGSTPaise != 90000 || manualLine.SGSTPaise != 90000 || manualLine.IGSTPaise != 0 {
		t.Fatalf("unexpected manual GST paise: %+v", manualLine)
	}
	if manualLine.TotalPaise != 5000000+180000 {
		t.Fatalf("expected total 5180000, got %d", manualLine.TotalPaise)
	}
	if manualLine.NetPaise != 5180000-100000 {
		t.Fatalf("expected net 5080000, got %d", manualLine.NetPaise)
	}

	// Merging manual GST lines sums manual paise
	mergedManual, err := expenses.MergeDuplicateLines([]models.ExpenseLineInput{
		{ExpenseTypeID: "ET-CHA", AmountPaise: 3000000, IsManualGST: true, CGSTPaise: 45000, SGSTPaise: 45000, TDSRateBP: 200, Remarks: "Duty Reimbursement"},
		{ExpenseTypeID: "ET-CHA", AmountPaise: 2000000, IsManualGST: true, CGSTPaise: 45000, SGSTPaise: 45000, TDSRateBP: 200, Remarks: "Agency Fee"},
	})
	if err != nil {
		t.Fatalf("MergeDuplicateLines manual failed: %v", err)
	}
	if len(mergedManual) != 1 || mergedManual[0].AmountPaise != 5000000 || mergedManual[0].CGSTPaise != 90000 || mergedManual[0].SGSTPaise != 90000 {
		t.Fatalf("expected combined manual amounts, got %+v", mergedManual[0])
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

	// 3b. Verify listing expense invoices with lines does not hang or deadlock
	list, err := rt.Expenses.ListExpenseInvoices("", "")
	if err != nil {
		t.Fatalf("ListExpenseInvoices failed: %v", err)
	}
	if len(list) != 1 || len(list[0].Lines) != 1 {
		t.Fatalf("unexpected list: %+v", list)
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

	listAfterUpdate, err := rt.Expenses.ListExpenseInvoices(sh.ID, "DHL-9001")
	if err != nil {
		t.Fatalf("ListExpenseInvoices with filter failed: %v", err)
	}
	if len(listAfterUpdate) != 1 || len(listAfterUpdate[0].Lines) != 2 {
		t.Fatalf("expected 2 lines after update, got %+v", listAfterUpdate)
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
		expectedLDC := (row.MaterialValue + row.BCDAmount + row.SWSAmount + row.BoeInterest + row.ExpensesTotal) / row.Qty
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

	// 6b. Verify BOE Interest proportional allocation into Landed Cost
	ctx2, err := rt.BOEEntry.LoadShipmentContext(sh.ID)
	if err != nil {
		t.Fatalf("LoadShipmentContext 2 failed: %v", err)
	}
	ctx2.Params.Interest = 300.0 // AV ratio 2:1 (18000 & 9000) -> 200.00 & 100.00
	if _, err := rt.BOEEntry.SaveCalculation(ctx2.Params); err != nil {
		t.Fatalf("SaveCalculation with Interest failed: %v", err)
	}
	lcWithInterest, err := rt.Reports.GetLandedCostReport(models.LandedCostFilter{})
	if err != nil {
		t.Fatalf("GetLandedCostReport with interest failed: %v", err)
	}
	if math.Abs(lcWithInterest.Summary.TotalBoeInterest-300.0) > 0.02 {
		t.Fatalf("expected totalBoeInterest 300.00, got %.2f", lcWithInterest.Summary.TotalBoeInterest)
	}
	for _, row := range lcWithInterest.Rows {
		expectedInterest := 300.0 * (row.AssessableValue / 27000.0)
		if math.Abs(row.BoeInterest-expectedInterest) > 0.02 {
			t.Fatalf("expected BoeInterest %.2f for %s, got %.2f", expectedInterest, row.PartNo, row.BoeInterest)
		}
		expectedLDC := (row.MaterialValue + row.BCDAmount + row.SWSAmount + row.BoeInterest + row.ExpensesTotal) / row.Qty
		if math.Abs(row.LDCPerQty-expectedLDC) > 0.01 {
			t.Fatalf("LDCPerQty with interest mismatch for %s: got %.4f want %.4f", row.PartNo, row.LDCPerQty, expectedLDC)
		}
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

func TestManualGSTCalculationAndPersistence(t *testing.T) {
	rt := setupExpenseEnv(t)

	sup, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Manual GST Supplier",
		ShortName:    "MGS",
		Country:      "India",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	sh, err := rt.Shipments.Create(models.ShipmentInput{
		SupplierID:      sup.ID,
		InvoiceNumber:   "INV-MGST-1",
		InvoiceDate:     "2026-04-15",
		GoodsCategory:   "Machinery",
		InvoiceValue:    100,
		InvoiceCurrency: "INR",
		Incoterm:        "FOB",
		ShipmentMode:    "Air",
		ShipmentType:    "Dutiable",
	})
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	sp, err := rt.Expenses.CreateServiceProvider(models.ServiceProviderInput{
		Name:  "Express Clearing Ltd",
		GSTIN: "33AABCE1234F1Z5",
		State: "Tamil Nadu",
	})
	if err != nil {
		t.Fatalf("CreateServiceProvider: %v", err)
	}

	etClear, err := rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{
		Name:              "Clearing Charges",
		DefaultCGSTRateBP: 900,
		DefaultSGSTRateBP: 900,
		DefaultTDSRateBP:  200,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("CreateExpenseType: %v", err)
	}

	etTrans, err := rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{
		Name:              "Transportation",
		DefaultCGSTRateBP: 900,
		DefaultSGSTRateBP: 900,
		DefaultTDSRateBP:  200,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("CreateExpenseType 2: %v", err)
	}

	// Create multi-line invoice where Line 1 has Manual GST (partial eligibility) and Line 2 has Automatic GST
	req := models.ExpenseInvoiceRequest{
		ShipmentID:        sh.ID,
		ServiceProviderID: sp.ID,
		InvoiceNo:         "EXP-MANUAL-001",
		InvoiceDate:       "2026-04-20",
		Currency:          "INR",
		IdempotencyKey:    "idem-manual-001",
		Lines: []models.ExpenseLineInput{
			{
				ExpenseTypeID: etClear.ID,
				AmountPaise:   5000000, // ₹50,000 total bill (only ₹10,000 eligible for GST)
				IsManualGST:   true,
				CGSTPaise:     90000, // ₹900 manual CGST (9% on 10,000)
				SGSTPaise:     90000, // ₹900 manual SGST (9% on 10,000)
				IGSTPaise:     0,
				TDSRateBP:     200, // 2% TDS on ₹50,000 = ₹1,000
				Remarks:       "Partial GST eligible agency fee",
			},
			{
				ExpenseTypeID: etTrans.ID,
				AmountPaise:   1000000, // ₹10,000 basic (automatic 9% + 9% = 900 + 900)
				IsManualGST:   false,
				CGSTRateBP:    900,
				SGSTRateBP:    900,
				TDSRateBP:     200,
				Remarks:       "Full GST eligible transportation",
			},
		},
	}

	inv, err := rt.Expenses.CreateExpenseInvoice(req)
	if err != nil {
		t.Fatalf("CreateExpenseInvoice failed: %v", err)
	}

	// Verify line 1: Base 50000, CGST 900, SGST 900, Total 51800, Net 50800
	// Line 2: Base 10000, CGST 900, SGST 900, Total 11800, Net 11600
	// Invoice totals: Base 60000, Total CGST 1800, Total SGST 1800, Total 63600, Net 62400
	if inv.BaseAmountPaise != 6000000 {
		t.Fatalf("expected base 6000000, got %d", inv.BaseAmountPaise)
	}
	if inv.TotalCGSTPaise != 180000 || inv.TotalSGSTPaise != 180000 {
		t.Fatalf("expected CGST 180000 & SGST 180000, got CGST=%d SGST=%d", inv.TotalCGSTPaise, inv.TotalSGSTPaise)
	}
	if inv.TotalAmountPaise != 6360000 || inv.NetAmountPaise != 6240000 {
		t.Fatalf("expected total 6360000 & net 6240000, got total=%d net=%d", inv.TotalAmountPaise, inv.NetAmountPaise)
	}

	fetched, err := rt.Expenses.GetExpenseInvoice(inv.ID)
	if err != nil {
		t.Fatalf("GetExpenseInvoice failed: %v", err)
	}
	if len(fetched.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(fetched.Lines))
	}
	if !fetched.Lines[0].IsManualGST || fetched.Lines[0].CGSTPaise != 90000 {
		t.Fatalf("expected Line 0 to have IsManualGST=true and CGSTPaise=90000, got %+v", fetched.Lines[0])
	}
	if fetched.Lines[1].IsManualGST || fetched.Lines[1].CGSTPaise != 90000 {
		t.Fatalf("expected Line 1 to have IsManualGST=false and CGSTPaise=90000, got %+v", fetched.Lines[1])
	}
}

func TestRecreateSoftDeletedExpenseInvoiceAndRoundOff(t *testing.T) {
	rt := setupExpenseEnv(t)

	sup, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Global Carrier",
		ShortName:    "GC",
		Country:      "India",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	sh, err := rt.Shipments.Create(models.ShipmentInput{
		SupplierID:      sup.ID,
		InvoiceNumber:   "INV-GC-1",
		InvoiceDate:     "2026-10-14",
		GoodsCategory:   "LCL Cargo",
		InvoiceValue:    5000,
		InvoiceCurrency: "INR",
		Incoterm:        "FOB",
		ShipmentMode:    "Sea",
		ShipmentType:    "Dutiable",
	})
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	sp, err := rt.Expenses.CreateServiceProvider(models.ServiceProviderInput{
		Name:  "MANTHRA SHIPPING",
		GSTIN: "33AABCM3770F1Z1",
		State: "Tamil Nadu",
	})
	if err != nil {
		t.Fatalf("CreateServiceProvider: %v", err)
	}

	etTrans, err := rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{
		Name:              "Transport Charges-LCL",
		DefaultCGSTRateBP: 900,
		DefaultSGSTRateBP: 900,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("CreateExpenseType transport: %v", err)
	}

	etClear, err := rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{
		Name:              "Clearing & Forwarding LCL",
		DefaultCGSTRateBP: 900,
		DefaultSGSTRateBP: 900,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("CreateExpenseType clearing: %v", err)
	}

	// 1. Create invoice with 1 line (Transport)
	initialReq := models.ExpenseInvoiceRequest{
		ShipmentID:        sh.ID,
		ServiceProviderID: sp.ID,
		InvoiceNo:         "377",
		InvoiceDate:       "2026-10-14",
		Currency:          "INR",
		Lines: []models.ExpenseLineInput{
			{
				ExpenseTypeID: etTrans.ID,
				AmountPaise:   625000, // ₹6,250
				CGSTRateBP:    900,
				SGSTRateBP:    900,
				RoundOff:      true,
			},
		},
	}
	inv1, err := rt.Expenses.CreateExpenseInvoice(initialReq)
	if err != nil {
		t.Fatalf("Create initial invoice failed: %v", err)
	}

	// 2. Soft-delete the invoice (just like the user did)
	if err := rt.Expenses.DeleteExpenseInvoice(inv1.ID); err != nil {
		t.Fatalf("DeleteExpenseInvoice failed: %v", err)
	}

	// 3. Re-create invoice 377 with multiple services (Transport + Clearing) and RoundOff
	// Transport ₹6,250: 9% = 562.50 -> rounded to 563 (56300 paise)
	// Clearing ₹3,399: 9% = 305.91 -> rounded to 306 (30600 paise)
	// Base = 9649 (964900 paise)
	// Total CGST = 563 + 306 = 869 (86900 paise)
	// Total SGST = 563 + 306 = 869 (86900 paise)
	// Total = 9649 + 869 + 869 = 11387 (1138700 paise)
	multiReq := models.ExpenseInvoiceRequest{
		ShipmentID:        sh.ID,
		ServiceProviderID: sp.ID,
		InvoiceNo:         "377",
		InvoiceDate:       "2026-10-14",
		Currency:          "INR",
		RoundOffPaise:     -100, // Adjust by -₹1 to match paper invoice of ₹11,386
		Lines: []models.ExpenseLineInput{
			{
				ExpenseTypeID: etTrans.ID,
				AmountPaise:   625000,
				CGSTRateBP:    900,
				SGSTRateBP:    900,
				RoundOff:      true,
				Remarks:       "Transport portion",
			},
			{
				ExpenseTypeID: etClear.ID,
				AmountPaise:   339900,
				CGSTRateBP:    900,
				SGSTRateBP:    900,
				RoundOff:      true,
				Remarks:       "Customs clearance portion",
			},
		},
	}
	inv2, err := rt.Expenses.CreateExpenseInvoice(multiReq)
	if err != nil {
		t.Fatalf("CreateExpenseInvoice on soft-deleted invoice_no failed: %v", err)
	}

	if inv2.InvoiceNo != "377" {
		t.Fatalf("expected invoice 377, got %s", inv2.InvoiceNo)
	}
	if inv2.BaseAmountPaise != 964900 {
		t.Fatalf("expected base 964900, got %d", inv2.BaseAmountPaise)
	}
	// Verify integer rupee paise (no digits / fractions)
	if inv2.TotalCGSTPaise%100 != 0 || inv2.TotalSGSTPaise%100 != 0 {
		t.Fatalf("expected integer rupee taxes without fractional paise, got CGST=%d SGST=%d", inv2.TotalCGSTPaise, inv2.TotalSGSTPaise)
	}
	if inv2.TotalCGSTPaise != 86900 || inv2.TotalSGSTPaise != 86900 {
		t.Fatalf("expected rounded CGST=86900 SGST=86900, got CGST=%d SGST=%d", inv2.TotalCGSTPaise, inv2.TotalSGSTPaise)
	}
	if inv2.RoundOffPaise != -100 {
		t.Fatalf("expected roundOffPaise=-100, got %d", inv2.RoundOffPaise)
	}
	if inv2.NetAmountPaise != 1138600 {
		t.Fatalf("expected net 1138600, got %d", inv2.NetAmountPaise)
	}

	// Verify GetExpenseInvoice returns the roundoff
	fetched, err := rt.Expenses.GetExpenseInvoice(inv2.ID)
	if err != nil {
		t.Fatalf("GetExpenseInvoice failed: %v", err)
	}
	if fetched.RoundOffPaise != -100 || fetched.NetAmountPaise != 1138600 {
		t.Fatalf("fetched roundoff mismatch: %+v", fetched)
	}
}

func TestBulkCSVImportMultipleShipments(t *testing.T) {
	rt := setupExpenseEnv(t)

	sup, err := rt.Suppliers.Create(models.SupplierInput{
		SupplierName: "Global Auto Parts",
		ShortName:    "GAP",
		Country:      "Japan",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	shA, err := rt.Shipments.Create(models.ShipmentInput{
		SupplierID:      sup.ID,
		InvoiceNumber:   "SHP-MULTI-A",
		InvoiceDate:     "2026-05-01",
		GoodsCategory:   "Auto Components",
		InvoiceValue:    50000,
		InvoiceCurrency: "USD",
		Incoterm:        "FOB",
	})
	if err != nil {
		t.Fatalf("create shipment A: %v", err)
	}

	shB, err := rt.Shipments.Create(models.ShipmentInput{
		SupplierID:      sup.ID,
		InvoiceNumber:   "SHP-MULTI-B",
		InvoiceDate:     "2026-05-02",
		GoodsCategory:   "Auto Components",
		InvoiceValue:    75000,
		InvoiceCurrency: "USD",
		Incoterm:        "CIF",
	})
	if err != nil {
		t.Fatalf("create shipment B: %v", err)
	}

	shFrozen, err := rt.Shipments.Create(models.ShipmentInput{
		SupplierID:      sup.ID,
		InvoiceNumber:   "SHP-MULTI-FROZEN",
		InvoiceDate:     "2026-05-03",
		GoodsCategory:   "Auto Components",
		InvoiceValue:    20000,
		InvoiceCurrency: "USD",
		Incoterm:        "FOB",
	})
	if err != nil {
		t.Fatalf("create shipment frozen: %v", err)
	}
	if _, err := rt.Shipments.Freeze(shFrozen.ID); err != nil {
		t.Fatalf("freeze shipment: %v", err)
	}

	_, err = rt.Expenses.CreateServiceProvider(models.ServiceProviderInput{
		Name: "Speedy Logistics Hub",
	})
	if err != nil {
		t.Fatalf("create service provider: %v", err)
	}

	_, err = rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{
		Name:              "Customs Clearance Multi",
		DefaultCGSTRateBP: 900,
		DefaultSGSTRateBP: 900,
		DefaultIGSTRateBP: 0,
		DefaultTDSRateBP:  200,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("create expense type 1: %v", err)
	}

	_, err = rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{
		Name:              "Freight Ocean Multi",
		DefaultCGSTRateBP: 0,
		DefaultSGSTRateBP: 0,
		DefaultIGSTRateBP: 1800,
		DefaultTDSRateBP:  200,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("create expense type 2: %v", err)
	}

	// Multi-shipment CSV containing:
	// - 2 lines for SHP-MULTI-A under the same invoice EXP-A100 (atomic multi-line invoice)
	// - 1 line for SHP-MULTI-B under invoice EXP-B200
	// - 1 line for SHP-MULTI-FROZEN (should be rejected/skipped with frozen message)
	// - 1 line for an unknown shipment SHP-UNKNOWN (should be rejected/skipped with unknown message)
	multiCSV := "Shipment Invoice No,Expense Type,Service Provider,Invoice No,Invoice Date,Amount,CGST Amount,SGST Amount,IGST Amount,TDS Amount,Total Amount,Remarks\n" +
		"SHP-MULTI-A,Customs Clearance Multi,Speedy Logistics Hub,EXP-A100,2026-05-10,10000.00,900.00,900.00,0,200.00,11800.00,Line 1 on Shipment A\n" +
		"SHP-MULTI-A,Freight Ocean Multi,Speedy Logistics Hub,EXP-A100,2026-05-10,5000.00,0,0,900.00,100.00,5900.00,Line 2 on Shipment A\n" +
		"SHP-MULTI-B,Customs Clearance Multi,Speedy Logistics Hub,EXP-B200,2026-05-11,8000.00,720.00,720.00,0,160.00,9440.00,Line 1 on Shipment B\n" +
		"SHP-MULTI-FROZEN,Customs Clearance Multi,Speedy Logistics Hub,EXP-F300,2026-05-12,3000.00,270.00,270.00,0,60.00,3540.00,Should fail frozen\n" +
		"SHP-UNKNOWN,Customs Clearance Multi,Speedy Logistics Hub,EXP-U400,2026-05-12,4000.00,360.00,360.00,0,80.00,4720.00,Should fail unknown\n"

	// Notice: shipmentID argument is passed empty "", so all rows rely on the "Shipment Invoice No" column
	res, err := rt.Expenses.ImportBulkCSV("", multiCSV)
	if err != nil {
		t.Fatalf("ImportBulkCSV failed: %v", err)
	}

	if res.TotalRows != 5 {
		t.Errorf("expected 5 total rows, got %d", res.TotalRows)
	}
	if res.ImportedRows != 3 {
		t.Errorf("expected 3 imported rows, got %d", res.ImportedRows)
	}
	if res.SkippedRows != 2 {
		t.Errorf("expected 2 skipped rows, got %d", res.SkippedRows)
	}
	if len(res.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d: %+v", len(res.Errors), res.Errors)
	}

	// Verify Shipment A has the 2-line invoice
	invoicesA, err := rt.Expenses.ListExpenseInvoices(shA.ID, "")
	if err != nil {
		t.Fatalf("ListExpenseInvoices for shA failed: %v", err)
	}
	if len(invoicesA) != 1 {
		t.Fatalf("expected 1 invoice on shA, got %d", len(invoicesA))
	}
	if invoicesA[0].InvoiceNo != "EXP-A100" {
		t.Errorf("expected invoice EXP-A100 on shA, got %s", invoicesA[0].InvoiceNo)
	}
	if invoicesA[0].BaseAmountPaise != 1500000 { // ₹15,000.00 = 1500000 paise
		t.Errorf("expected 1500000 paise base on shA, got %d", invoicesA[0].BaseAmountPaise)
	}
	if len(invoicesA[0].Lines) != 2 {
		t.Errorf("expected 2 lines on shA invoice, got %d", len(invoicesA[0].Lines))
	}

	// Verify Shipment B has its invoice
	invoicesB, err := rt.Expenses.ListExpenseInvoices(shB.ID, "")
	if err != nil {
		t.Fatalf("ListExpenseInvoices for shB failed: %v", err)
	}
	if len(invoicesB) != 1 {
		t.Fatalf("expected 1 invoice on shB, got %d", len(invoicesB))
	}
	if invoicesB[0].InvoiceNo != "EXP-B200" {
		t.Errorf("expected invoice EXP-B200 on shB, got %s", invoicesB[0].InvoiceNo)
	}
	if invoicesB[0].BaseAmountPaise != 800000 {
		t.Errorf("expected 800000 paise base on shB, got %d", invoicesB[0].BaseAmountPaise)
	}

	// Verify Shipment Frozen has NO invoices
	invoicesF, err := rt.Expenses.ListExpenseInvoices(shFrozen.ID, "")
	if err != nil {
		t.Fatalf("ListExpenseInvoices for shFrozen failed: %v", err)
	}
	if len(invoicesF) != 0 {
		t.Errorf("expected 0 invoices on frozen shipment, got %d", len(invoicesF))
	}
}
