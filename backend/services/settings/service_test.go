package settings

import (
	"testing"
)

func TestDefaultProfileIncludesCBICRatesAndReconciledBOE(t *testing.T) {
	p := DefaultProfile()

	expectedModules := []string{
		"shipment",
		"invoice",
		"boe",
		"boeSummary",
		"supplier",
		"itemMaster",
		"expenses",
		"cbicRates",
		"reconciledBoe",
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

	enforceInvariants(&p)

	if _, ok := p.Modules["cbicRates"]; !ok {
		t.Error("enforceInvariants failed to populate missing cbicRates")
	}
	if _, ok := p.Modules["reconciledBoe"]; !ok {
		t.Error("enforceInvariants failed to populate missing reconciledBoe")
	}
}
