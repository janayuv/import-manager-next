package landedcost

// ITCEligibility represents the Input Tax Credit treatment under GST law.
type ITCEligibility string

const (
	// ITCEligible: Creditable input tax credit under GST Section 16(1) (excluded from Landed Cost).
	ITCEligible ITCEligibility = "ITC_ELIGIBLE"

	// ITCCapitalGoodsEligible: Capital goods capitalized in balance sheet where depreciation is NOT claimed
	// on the tax component under Section 16(3) CGST Act. Capitalization alone does not block ITC;
	// ITC is 100% eligible (excluded from Landed Cost).
	ITCCapitalGoodsEligible ITCEligibility = "ITC_CAPITAL_GOODS_ELIGIBLE"

	// ITCCapitalGoods: Capital goods where depreciation claimed under Income Tax Act Section 32 read with
	// Section 16(3) CGST Act (ITC blocked, included in Landed Cost).
	ITCCapitalGoods ITCEligibility = "ITC_CAPITAL_GOODS"

	// ITCCapitalGoodsTaxDepreciationClaimed: Explicit alias for Section 16(3) blocked credit on capital goods.
	ITCCapitalGoodsTaxDepreciationClaimed ITCEligibility = "ITC_CAPITAL_GOODS_TAX_DEPRECIATION"

	// ITCIneligible17_1: Non-business end use under Section 17(1) CGST Act (included in Landed Cost).
	ITCIneligible17_1 ITCEligibility = "ITC_INELIGIBLE_17_1"

	// ITCIneligible17_2: Exempt supplies or non-taxable outward supplies under Section 17(2) CGST Act (included in Landed Cost).
	ITCIneligible17_2 ITCEligibility = "ITC_INELIGIBLE_17_2"

	// ITCIneligible17_5: Blocked credit under CGST Act Section 17(5) (included in Landed Cost).
	ITCIneligible17_5 ITCEligibility = "ITC_INELIGIBLE_17_5"

	// ITCPartiallyEligible: Partial ITC eligibility under Section 17(1) / Section 17(2) read with Rules 42 & 43 CGST Rules.
	// Only the non-recoverable portion is capitalized into Landed Cost.
	ITCPartiallyEligible ITCEligibility = "ITC_PARTIALLY_ELIGIBLE"
)

// ShouldIncludeInLandedCost returns true if IGST is completely non-recoverable and must be capitalized into inventory cost.
func ShouldIncludeInLandedCost(eligibility ITCEligibility) bool {
	switch eligibility {
	case ITCIneligible17_1, ITCIneligible17_2, ITCIneligible17_5:
		return true
	case ITCCapitalGoods, ITCCapitalGoodsTaxDepreciationClaimed:
		// When depreciation is claimed on the tax portion under Section 16(3), it cannot be claimed as ITC
		return true
	case ITCEligible, ITCCapitalGoodsEligible:
		// Capitalization alone does NOT block ITC! If no tax depreciation is claimed, ITC is fully eligible.
		return false
	case ITCPartiallyEligible:
		// Partial cases handle non-recoverable amounts proportionately
		return false
	default:
		return false
	}
}
