package reconciliation

import (
	"math"
	"testing"
)

func TestReconcileBOEDutyExactAndTolerance(t *testing.T) {
	// Exact match
	res1 := ReconcileBOEDuty(25450.0, 25450.0, 2.0)
	if !res1.IsMatched || res1.Status != StatusReconciled {
		t.Errorf("Expected Reconciled for exact match, got %s", res1.Status)
	}

	// Within tolerance (diff = 1.50 INR <= 2.0 INR)
	res2 := ReconcileBOEDuty(25451.50, 25450.0, 2.0)
	if !res2.IsMatched || res2.Status != StatusReconciled {
		t.Errorf("Expected Reconciled within tolerance, got %s", res2.Status)
	}

	// Discrepancy beyond tolerance (diff = 10.0 INR > 2.0 INR)
	res3 := ReconcileBOEDuty(25460.0, 25450.0, 2.0)
	if res3.IsMatched || res3.Status != StatusDiscrepancyFound {
		t.Errorf("Expected Discrepancy Found, got %s", res3.Status)
	}

	// Awaiting data
	res4 := ReconcileBOEDuty(0, 25450.0, 2.0)
	if res4.Status != StatusAwaitingBOEData {
		t.Errorf("Expected Awaiting BOE Data, got %s", res4.Status)
	}
}

func TestSettleAdvanceAgainstInvoice(t *testing.T) {
	invoiceVal := 10000.0 // USD
	advances := []POAdvanceAllocation{
		{PaymentID: "PAY-1", AllocatedAmount: 4000.0, ExchangeRate: 1.0},
		{PaymentID: "PAY-2", AllocatedAmount: 7000.0, ExchangeRate: 1.0}, // will only use 6000
	}

	res, err := SettleAdvanceAgainstInvoice(invoiceVal, advances)
	if err != nil {
		t.Fatalf("SettleAdvanceAgainstInvoice failed: %v", err)
	}

	if !res.IsFullySettled {
		t.Errorf("Expected invoice to be fully settled")
	}
	if math.Abs(res.TotalAllocated-10000.0) > 0.01 {
		t.Errorf("Expected 10000.0 allocated, got %f", res.TotalAllocated)
	}
	if res.RemainingUnsettled != 0 {
		t.Errorf("Expected remaining 0, got %f", res.RemainingUnsettled)
	}
	if len(res.Allocations) != 2 {
		t.Fatalf("Expected 2 allocations, got %d", len(res.Allocations))
	}
	if res.Allocations[1].AllocatedAmount != 6000.0 {
		t.Errorf("Expected second allocation 6000.0, got %f", res.Allocations[1].AllocatedAmount)
	}
}
