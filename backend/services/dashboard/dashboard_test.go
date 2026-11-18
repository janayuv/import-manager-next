package dashboard_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/models"
)

func setupDashEnv(t *testing.T) *bootstrap.Runtime {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "dash-test-secret-0123456789abcdef")
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

func TestRealDashboardKPIsFYFilteringCacheAndSettings(t *testing.T) {
	rt := setupDashEnv(t)

	// 1. Fresh DB has 0 shipments, 0 import value, 0 duty (no fabricated metrics).
	emptyOv, err := rt.Dashboard.GetOverview(models.DashboardFilter{FinancialYear: "all"})
	if err != nil {
		t.Fatalf("GetOverview empty failed: %v", err)
	}
	if emptyOv.KPIs.TotalShipments != 0 || emptyOv.KPIs.TotalImportValueINR != 0 || emptyOv.KPIs.TotalCustomsDutyINR != 0 {
		t.Fatalf("expected zero operational counts on empty DB, got %+v", emptyOv.KPIs)
	}

	// Second call within 180s hits dashboard_metrics_cache
	cachedOv, err := rt.Dashboard.GetOverview(models.DashboardFilter{FinancialYear: "all"})
	if err != nil {
		t.Fatalf("cached GetOverview failed: %v", err)
	}
	if !cachedOv.CacheHit {
		t.Fatalf("expected CacheHit=true within 180s TTL")
	}

	// 2. Seed two shipments in separate FYs: FY 2025-26 (May 2025) vs FY 2026-27 (Apr 2026)
	sup, _ := rt.Suppliers.Create(models.SupplierInput{SupplierName: "Festo AG", Country: "Germany", IsActive: true})
	item, _ := rt.Items.Create(models.ItemInput{
		PartNumber: "FES-1", ItemDescription: "Cylinder", Unit: "PCS", Currency: "EUR",
		UnitPrice: 100, HSNCode: "8412", SupplierID: sup.ID, IsActive: true, BCD: "7.5", SWS: "10", IGST: "18",
	})
	sh25, _ := rt.Shipments.Create(models.ShipmentInput{
		SupplierID: sup.ID, InvoiceNumber: "INV-25", InvoiceDate: "2025-05-10",
		GoodsCategory: "Machinery", InvoiceValue: 100, InvoiceCurrency: "EUR", Incoterm: "CIF",
		Status: "delivered",
	})
	sh26, _ := rt.Shipments.Create(models.ShipmentInput{
		SupplierID: sup.ID, InvoiceNumber: "INV-26", InvoiceDate: "2026-04-15",
		GoodsCategory: "Machinery", InvoiceValue: 200, InvoiceCurrency: "EUR", Incoterm: "CIF",
		ETA: "2025-01-01", Status: "in-transit",
	})
	_, _ = rt.InvoiceWizard.SaveInvoice(models.SaveInvoiceRequest{
		ShipmentID: sh26.ID, Finalize: true, LineTotalDecimals: 2, InvoiceTotalDecimals: 2,
		Lines: []models.InvoiceLineInput{{ItemID: item.ID, Quantity: 2, UnitPrice: 100, DutyPercent: 7.5, SWSPercent: 10, IGSTPercent: 18}},
	})
	ctx26, _ := rt.BOEEntry.LoadShipmentContext(sh26.ID)
	ctx26.Params.ExchangeRate = 90
	_, _ = rt.BOEEntry.SaveCalculation(ctx26.Params)

	sp, _ := rt.Expenses.CreateServiceProvider(models.ServiceProviderInput{Name: "UPS India"})
	et, _ := rt.Expenses.CreateExpenseType(models.ExpenseTypeInput{Name: "Freight", IsActive: true})
	_, _ = rt.Expenses.CreateExpenseInvoice(models.ExpenseInvoiceRequest{
		ShipmentID: sh26.ID, ServiceProviderID: sp.ID, InvoiceNo: "UPS-26", InvoiceDate: "2026-04-20",
		Lines: []models.ExpenseLineInput{{ExpenseTypeID: et.ID, AmountPaise: 500000}}, // ₹5,000
	})
	_ = sh25

	if err := rt.Dashboard.InvalidateCache(); err != nil {
		t.Fatalf("InvalidateCache failed: %v", err)
	}

	ov26, err := rt.Dashboard.GetOverview(models.DashboardFilter{FinancialYear: "2026-27"})
	if err != nil {
		t.Fatalf("GetOverview 2026-27 failed: %v", err)
	}
	if ov26.KPIs.TotalShipments != 1 || ov26.KPIs.PendingShipments != 1 || ov26.KPIs.OverdueETACount != 1 {
		t.Fatalf("unexpected FY 2026-27 shipment counts: %+v", ov26.KPIs)
	}
	if math.Abs(ov26.KPIs.TotalImportValueINR-18000.0) > 0.01 || math.Abs(ov26.KPIs.TotalBasicExpensesINR-5000.0) > 0.01 {
		t.Fatalf("unexpected FY 2026-27 financials: %+v", ov26.KPIs)
	}

	ovAll, err := rt.Dashboard.GetOverview(models.DashboardFilter{FinancialYear: "all", ForceRefresh: true})
	if err != nil {
		t.Fatalf("GetOverview all failed: %v", err)
	}
	if ovAll.KPIs.TotalShipments != 2 || ovAll.KPIs.CompletedShipments != 1 {
		t.Fatalf("expected 2 total / 1 completed under FY=all, got %+v", ovAll.KPIs)
	}
	if len(ovAll.RecentActivity) > 5 {
		t.Fatalf("expected RecentActivity to be at most 5 items, got %d", len(ovAll.RecentActivity))
	}

	snapDate, err := rt.Dashboard.RebuildDailySnapshot()
	if err != nil || snapDate == "" {
		t.Fatalf("RebuildDailySnapshot failed: date=%q err=%v", snapDate, err)
	}

	// 3. Settings invariants & persistence
	prof, err := rt.Settings.GetProfile()
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	prof.Theme = "steel"
	prof.Mode = "dark"
	prof.LineTotalDecimals = 0
	prof.Modules["shipment"].FieldCases["shipmentType"] = "lowercase" // must be forced back to uppercase

	saved, err := rt.Settings.SaveProfile(prof)
	if err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}
	if saved.Theme != "steel" || saved.LineTotalDecimals != 0 || saved.Modules["shipment"].FieldCases["shipmentType"] != "uppercase" {
		t.Fatalf("settings invariant failed: %+v", saved)
	}

	bad := prof
	bad.LineTotalDecimals = 3
	if _, err := rt.Settings.SaveProfile(bad); err == nil {
		t.Fatalf("expected validation error for lineTotalDecimals=3")
	}
}
