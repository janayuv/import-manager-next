package money

import (
	"fmt"
	"math"
)

// RateScale is parts-per-billion (1e9), where 100% = 1,000,000,000.
// This allows exact representation of fractional statutory rates:
// - 0.015% (Open Cover Marine Insurance) = 150,000 PPB
// - 1.125% (Rule 10(2) Deemed Insurance) = 11,250,000 PPB
// - 18.00% (Standard GST) = 180,000,000 PPB
const RateScale int64 = 1_000_000_000

// DecimalRate represents a high-precision statutory percentage rate.
type DecimalRate struct {
	PPB int64 `json:"ppb"` // Parts-per-billion (100% = 1,000,000,000)
}

// RateFromPercentageFloat creates a DecimalRate from a percentage float (e.g. 18.0 for 18%, 1.125 for 1.125%).
func RateFromPercentageFloat(pct float64) DecimalRate {
	// 100% = 1e9 PPB => 1% = 1e7 PPB
	ppb := int64(math.Round(pct * 10_000_000))
	return DecimalRate{PPB: ppb}
}

// RateFromBasisPoints creates a DecimalRate from integer basis points (10000 bp = 100%).
func RateFromBasisPoints(bp int64) DecimalRate {
	// 10,000 BP = 1e9 PPB => 1 BP = 100,000 PPB
	return DecimalRate{PPB: bp * 100_000}
}

// RateFromFraction creates a DecimalRate from numerator and denominator (e.g. 1125 / 100000 = 1.125%).
func RateFromFraction(num, den int64) (DecimalRate, error) {
	if den <= 0 {
		return DecimalRate{}, fmt.Errorf("denominator must be positive (got %d)", den)
	}
	// (num * RateScale) / den
	ppb := (num * RateScale) / den
	return DecimalRate{PPB: ppb}, nil
}

// RateFromPPB creates a DecimalRate directly from parts-per-billion.
func RateFromPPB(ppb int64) DecimalRate {
	return DecimalRate{PPB: ppb}
}

// PercentageFloat returns the rate as a percentage float for display/export (e.g. 18.0).
func (r DecimalRate) PercentageFloat() float64 {
	return float64(r.PPB) / 10_000_000.0
}

// BasisPoints returns the rate in integer basis points (rounded).
func (r DecimalRate) BasisPoints() int64 {
	return (r.PPB + 50_000) / 100_000
}

// IsZero returns true if the rate is 0.
func (r DecimalRate) IsZero() bool {
	return r.PPB == 0
}

// String formats the rate with trailing zeros trimmed (e.g. "18%", "1.125%", "0.015%").
func (r DecimalRate) String() string {
	pct := r.PercentageFloat()
	return fmt.Sprintf("%.4f%%", pct)
}
