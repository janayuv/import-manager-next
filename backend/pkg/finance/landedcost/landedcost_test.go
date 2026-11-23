package landedcost

import (
	"math"
	"testing"
)

func TestLandedCostRecoverableIGSTExcluded(t *testing.T) {
	// Item: Qty = 100, AV = 50,000 INR, BCD = 3,750 INR, SWS = 375 INR, IGST = 9,742.50 INR.
	// Eligible for ITC under Section 16(1) => IGST is excluded.
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
	if line.NonRecoverableIGST != 0.0 {
		t.Errorf("Expected NonRecoverableIGST to be 0, got %f", line.NonRecoverableIGST)
	}
	if math.Abs(line.TotalInventoryCostINR-55125.0) > 0.01 {
		t.Errorf("Expected total cost 55125.0, got %f", line.TotalInventoryCostINR)
	}
	if math.Abs(line.LDCPerQuantity-551.25) > 0.01 {
		t.Errorf("Expected LDC per qty 551.25, got %f", line.LDCPerQuantity)
	}
}

func TestLandedCostCapitalGoodsWithoutTaxDepreciationEligible(t *testing.T) {
	// Statutory Invariant: Capitalization alone does NOT automatically block ITC!
	// Importer capitalizes Capital Goods (Plant & Machinery) on balance sheet under Section 16(1),
	// but claims depreciation under Income Tax Act Section 32 ONLY on the base asset cost (excluding GST).
	// Under Section 16(3), ITC is fully ALLOWED and must NOT be capitalized into inventory cost.
	// Qty = 1, AV = 2,000,000 INR, BCD = 150,000 INR, SWS = 15,000 INR, IGST = 389,700 INR.
	// Landed Cost = 2,000,000 + 150,000 + 15,000 = 2,165,000 INR (IGST completely excluded).
	items := []ItemLandedCostInput{
		{
			PartNumber:      "CAP-PLANT-01",
			Quantity:        1,
			BasisValue:      2000000.0,
			AssessableValue: 2000000.0,
			BCDAmount:       150000.0,
			SWSAmount:       15000.0,
			IGSTAmount:      389700.0,
			ITCEligibility:  ITCCapitalGoodsEligible,
		},
	}

	res, err := ComputeLandedCost(items, nil, BasisAssessableValue)
	if err != nil {
		t.Fatalf("ComputeLandedCost failed: %v", err)
	}

	line := res.Items[0]
	if line.IGSTIncludedInCost {
		t.Errorf("Capitalization alone must NOT block ITC; expected IGST excluded from landed cost")
	}
	if line.NonRecoverableIGST != 0.0 {
		t.Errorf("Expected NonRecoverableIGST 0, got %f", line.NonRecoverableIGST)
	}
	if math.Abs(line.TotalInventoryCostINR-2165000.0) > 0.01 {
		t.Errorf("Expected total inventory cost 2,165,000.0, got %f", line.TotalInventoryCostINR)
	}
}

func TestLandedCostCapitalGoodsWithTaxDepreciationBlocked(t *testing.T) {
	// CGST Act Section 16(3): Where depreciation is claimed on the tax component of capital goods,
	// ITC is disallowed. IGST must be capitalized into landed cost.
	// Qty = 1, AV = 2,000,000 INR, BCD = 150,000 INR, SWS = 15,000 INR, IGST = 389,700 INR.
	// Landed Cost = 2,000,000 + 150,000 + 15,000 + 389,700 = 2,554,700 INR.
	items := []ItemLandedCostInput{
		{
			PartNumber:      "CAP-PLANT-TAX-DEP",
			Quantity:        1,
			BasisValue:      2000000.0,
			AssessableValue: 2000000.0,
			BCDAmount:       150000.0,
			SWSAmount:       15000.0,
			IGSTAmount:      389700.0,
			ITCEligibility:  ITCCapitalGoodsTaxDepreciationClaimed,
		},
	}

	res, err := ComputeLandedCost(items, nil, BasisAssessableValue)
	if err != nil {
		t.Fatalf("ComputeLandedCost failed: %v", err)
	}

	line := res.Items[0]
	if !line.IGSTIncludedInCost {
		t.Errorf("Expected IGST to be included in landed cost under Section 16(3)")
	}
	if line.NonRecoverableIGST != 389700.0 {
		t.Errorf("Expected NonRecoverableIGST 389,700.0, got %f", line.NonRecoverableIGST)
	}
	if math.Abs(line.TotalInventoryCostINR-2554700.0) > 0.01 {
		t.Errorf("Expected total inventory cost 2,554,700.0, got %f", line.TotalInventoryCostINR)
	}
}

func TestLandedCostPartiallyEligible(t *testing.T) {
	// Partial ITC eligibility under CGST Act Section 17(1) / Section 17(2) & Rule 42.
	// Goods used 60% for taxable business supplies and 40% for exempt supplies.
	// Ineligible Ratio = 0.40.
	// AV = 100,000 INR, BCD = 10,000 INR, SWS = 1,000 INR, IGST = 19,980 INR.
	// Ineligible / Non-Recoverable IGST = 19,980 * 0.40 = 7,992.00 INR.
	// Landed Cost = 100,000 + 10,000 + 1,000 + 7,992 = 118,992.00 INR.
	items := []ItemLandedCostInput{
		{
			PartNumber:      "PARTIAL-01",
			Quantity:        10,
			BasisValue:      100000.0,
			AssessableValue: 100000.0,
			BCDAmount:       10000.0,
			SWSAmount:       1000.0,
			IGSTAmount:      19980.0,
			ITCEligibility:  ITCPartiallyEligible,
			IneligibleRatio: 0.40,
		},
	}

	res, err := ComputeLandedCost(items, nil, BasisAssessableValue)
	if err != nil {
		t.Fatalf("ComputeLandedCost failed: %v", err)
	}

	line := res.Items[0]
	if !line.IGSTIncludedInCost {
		t.Errorf("Expected IGST partial inclusion flag to be true")
	}
	if math.Abs(line.NonRecoverableIGST-7992.0) > 0.01 {
		t.Errorf("Expected non-recoverable IGST 7,992.0, got %f", line.NonRecoverableIGST)
	}
	if math.Abs(line.TotalInventoryCostINR-118992.0) > 0.01 {
		t.Errorf("Expected total inventory cost 118,992.0, got %f", line.TotalInventoryCostINR)
	}
	if math.Abs(line.LDCPerQuantity-11899.20) > 0.01 {
		t.Errorf("Expected LDC per qty 11,899.20, got %f", line.LDCPerQuantity)
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

func TestAllStatutoryITCSections(t *testing.T) {
	tests := []struct {
		name        string
		eligibility ITCEligibility
		expected    bool
	}{
		{"Section 16(1) Creditable Inputs", ITCEligible, false},
		{"Section 16(1) Capital Goods Capitalized Without Tax Depreciation", ITCCapitalGoodsEligible, false},
		{"Section 16(3) Capital Goods Tax Depreciation Claimed", ITCCapitalGoodsTaxDepreciationClaimed, true},
		{"Section 16(3) Legacy ITCCapitalGoods Alias", ITCCapitalGoods, true},
		{"Section 17(1) Non-Business Use", ITCIneligible17_1, true},
		{"Section 17(2) Exempt Outward Supplies", ITCIneligible17_2, true},
		{"Section 17(5) Blocked Credit", ITCIneligible17_5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIncludeInLandedCost(tt.eligibility)
			if got != tt.expected {
				t.Errorf("%s: ShouldIncludeInLandedCost(%v) = %v, expected %v", tt.name, tt.eligibility, got, tt.expected)
			}
		})
	}
}
