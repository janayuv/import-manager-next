package landedcost

// ITCEligibility represents the Input Tax Credit treatment under GST law.
type ITCEligibility string

const (
	// ITCEligible: Creditable input tax credit under GST Section 16 (excluded from Landed Cost).
	ITCEligible ITCEligibility = "ITC_ELIGIBLE"

	// ITCIneligible17_5: Blocked credit under CGST Act Section 17(5) or exempt end-uses (included in Landed Cost).
	ITCIneligible17_5 ITCEligibility = "ITC_INELIGIBLE_17_5"

	// ITCCapitalGoods: Capital goods evaluated based on Income Tax Act Section 32 depreciation vs GST credit.
	ITCCapitalGoods ITCEligibility = "ITC_CAPITAL_GOODS"
)

// ShouldIncludeInLandedCost returns true if IGST is non-recoverable and must be capitalized into inventory cost.
func ShouldIncludeInLandedCost(eligibility ITCEligibility) bool {
	switch eligibility {
	case ITCIneligible17_5:
		return true
	case ITCCapitalGoods:
		// When depreciation is claimed on the tax portion, it cannot be claimed as ITC
		return true
	case ITCEligible:
		fallthrough
	default:
		return false
	}
}
