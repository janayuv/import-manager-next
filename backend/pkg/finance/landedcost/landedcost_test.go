package landedcost

import (
	"math"
	"testing"
)

func TestLandedCostRecoverableIGSTExcluded(t *testing.T) {
	// Item: Qty = 100, AV = 50,000 INR, BCD = 3,750 INR, SWS = 375 INR, IGST = 9,742.50 INR.
	// Eligible for ITC => IGST is excluded.
	// Common Expense = 1,000 INR = 100,000 paise.
	// Total Inventory Cost = 50,000 + 3,750 + 375 + 1,000 = 55,125 INR.
	// LDC per qty = 55,125 / 100 = 551.25 INR.
	items := []ItemLandedCostInput{
		{
			PartNumber:      "P-01",
			Quantity:        100,
			BasisValue:      50000.0,
			AssessableValue: 50000.0,
			BCDAmount:       3750.0,
			SWSAmount:       375.0,
			IGSTAmount:      9742.50,
			ITCEligibility:  ITCEligible,
		},
	}
	expenses := []ExpenseItemInput{
		{ExpenseID: "EXP-1", AmountPaise: 100000},
	}

	res, err := ComputeLandedCost(items, expenses, BasisAssessableValue)
	if err != nil {
		t.Fatalf("ComputeLandedCost failed: %v", err)
	}

	line := res.Items[0]
	if line.IGSTIncludedInCost {
		t.Errorf("Expected IGST to be excluded from landed cost for ITCEligible")
	}
	if math.Abs(line.TotalInventoryCostINR-55125.0) > 0.01 {
		t.Errorf("Expected total cost 55125.0, got %f", line.TotalInventoryCostINR)
	}
	if math.Abs(line.LDCPerQuantity-551.25) > 0.01 {
		t.Errorf("Expected LDC per qty 551.25, got %f", line.LDCPerQuantity)
	}
}

func TestLandedCostNonRecoverableIGSTIncluded(t *testing.T) {
	// Item: Qty = 10, AV = 10,000 INR, BCD = 1,000 INR, SWS = 100 INR, IGST = 1,998 INR.
	// Blocked under Section 17(5) => IGST must be capitalized into inventory cost!
	// Total Inventory Cost = 10,000 + 1,000 + 100 + 1,998 = 13,098 INR.
	// LDC per qty = 13,098 / 10 = 1,309.80 INR.
	items := []ItemLandedCostInput{
		{
			PartNumber:      "VEH-01",
			Quantity:        10,
			BasisValue:      10000.0,
			AssessableValue: 10000.0,
			BCDAmount:       1000.0,
			SWSAmount:       100.0,
			IGSTAmount:      1998.0,
			ITCEligibility:  ITCIneligible17_5,
		},
	}

	res, err := ComputeLandedCost(items, nil, BasisAssessableValue)
	if err != nil {
		t.Fatalf("ComputeLandedCost failed: %v", err)
	}

	line := res.Items[0]
	if !line.IGSTIncludedInCost {
		t.Errorf("Expected IGST to be included in landed cost for ITCIneligible17_5")
	}
	if math.Abs(line.TotalInventoryCostINR-13098.0) > 0.01 {
		t.Errorf("Expected total cost 13098.0, got %f", line.TotalInventoryCostINR)
	}
	if math.Abs(line.LDCPerQuantity-1309.80) > 0.01 {
		t.Errorf("Expected LDC per qty 1309.80, got %f", line.LDCPerQuantity)
	}
}

func TestResidualPaiseBalancingExactParity(t *testing.T) {
	// 3 items with fractional split: expense = 100,001 paise (1000.01 INR).
	// Items basis: 33.33%, 33.33%, 33.34%.
	// Terminal item must absorb the 1 paise remainder so sum(allocated) == 100,001 paise exactly!
	items := []ItemLandedCostInput{
		{PartNumber: "ITEM-1", Quantity: 1, BasisValue: 100.0, AssessableValue: 100.0, ITCEligibility: ITCEligible},
		{PartNumber: "ITEM-2", Quantity: 1, BasisValue: 100.0, AssessableValue: 100.0, ITCEligibility: ITCEligible},
		{PartNumber: "ITEM-3", Quantity: 1, BasisValue: 100.0, AssessableValue: 100.0, ITCEligibility: ITCEligible},
	}
	expenses := []ExpenseItemInput{
		{ExpenseID: "EXP-FRAC", AmountPaise: 100001},
	}

	res, err := ComputeLandedCost(items, expenses, BasisAssessableValue)
	if err != nil {
		t.Fatalf("ComputeLandedCost failed: %v", err)
	}

	var sumPaise int64
	for _, it := range res.Items {
		sumPaise += it.AllocatedExpensePaise
	}

	if sumPaise != 100001 {
		t.Errorf("Residual paise balancing failed: expected 100001 paise, got %d", sumPaise)
	}
	if !res.ResidualPaiseBalanced {
		t.Errorf("Expected ResidualPaiseBalanced flag to be true")
	}
}
