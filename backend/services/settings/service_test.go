package settings

import (
	"testing"
)

func TestDefaultProfileIncludesCBICRatesAndReconciledBOE(t *testing.T) {
	p := DefaultProfile()

	expectedModules := []string{
		"shipment",
		"transitTracker",
		"invoice",
		"purchaseOrders",
		"payments",
		"advanceRecon",
		"boe",
		"boeSummary",
		"supplier",
		"itemMaster",
		"expenses",
		"cbicRates",
		"reconciledBoe",
		"landedCost",
		"partPeriodAnalysis",
	}

	for _, modKey := range expectedModules {
		mod, ok := p.Modules[modKey]
		if !ok {
			t.Fatalf("expected module %q in DefaultProfile, but was missing", modKey)
		}
		if mod.ItemsPerPage != 10 {
			t.Errorf("expected module %q itemsPerPage=10, got %d", modKey, mod.ItemsPerPage)
		}
		if mod.DecimalPrecision != 2 {
			t.Errorf("expected module %q decimalPrecision=2, got %d", modKey, mod.DecimalPrecision)
		}
	}
}

func TestEnforceInvariantsPopulatesMissingCBICRatesAndReconciledBOE(t *testing.T) {
	p := DefaultProfile()
	delete(p.Modules, "cbicRates")
	delete(p.Modules, "reconciledBoe")
	delete(p.Modules, "transitTracker")
	delete(p.Modules, "purchaseOrders")
	delete(p.Modules, "payments")
	delete(p.Modules, "advanceRecon")
	delete(p.Modules, "landedCost")
	delete(p.Modules, "partPeriodAnalysis")

	enforceInvariants(&p)

	for _, key := range []string{"cbicRates", "reconciledBoe", "transitTracker", "purchaseOrders", "payments", "advanceRecon", "landedCost", "partPeriodAnalysis"} {
		if _, ok := p.Modules[key]; !ok {
			t.Errorf("enforceInvariants failed to populate missing %s", key)
		}
	}
}
