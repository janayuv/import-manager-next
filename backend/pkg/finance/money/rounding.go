package money

import (
	"math"
)

// RoundingPolicy identifies explicit calculation rounding modes.
type RoundingPolicy string

const (
	PolicyNearestRupee        RoundingPolicy = "NEAREST_RUPEE"         // Nearest 100 paise (symmetric)
	PolicyNearestPaise        RoundingPolicy = "NEAREST_PAISE"         // Nearest 1 paise (symmetric half-up)
	PolicyCustomsOneDecimal   RoundingPolicy = "CUSTOMS_ONE_DECIMAL"   // Nearest 10 paise (0.10 INR for ICEGATE line duty)
	PolicyStatutoryWholeRupee RoundingPolicy = "STATUTORY_WHOLE_RUPEE" // Whole Rupee (100 paise for ICEGATE headers)
	PolicyTruncate            RoundingPolicy = "TRUNCATE"              // Floor toward zero
)

// RoundNearestRupee rounds paise to the nearest whole Rupee (multiple of 100 paise).
// Correctly handles negative adjustments symmetrically and guards against integer overflow boundaries:
// e.g. -140 paise -> -100 paise; -160 paise -> -200 paise; +140 -> +100; +160 -> +200.
func RoundNearestRupee(paise int64) int64 {
	if paise == 0 {
		return 0
	}
	if paise == math.MinInt64 {
		return -9223372036854775800
	}
	sign := int64(1)
	val := paise
	if val < 0 {
		sign = -1
		val = -val
	}
	if val > math.MaxInt64-50 {
		val = math.MaxInt64 - 50
	}
	rounded := ((val + 50) / 100) * 100
	return sign * rounded
}

// RoundCustomsOneDecimal rounds paise to the nearest 10 paise (matching ICEGATE 0.1 INR precision).
// e.g. 125 paise -> 130 paise (1.30 INR); 124 paise -> 120 paise (1.20 INR).
func RoundCustomsOneDecimal(paise int64) int64 {
	if paise == 0 {
		return 0
	}
	if paise == math.MinInt64 {
		return -9223372036854775800
	}
	sign := int64(1)
	val := paise
	if val < 0 {
		sign = -1
		val = -val
	}
	if val > math.MaxInt64-5 {
		val = math.MaxInt64 - 5
	}
	rounded := ((val + 5) / 10) * 10
	return sign * rounded
}

// RoundStatutoryWholeRupee rounds paise to the nearest whole Rupee (100 paise) for ICEGATE grand totals.
func RoundStatutoryWholeRupee(paise int64) int64 {
	return RoundNearestRupee(paise)
}

// RoundTruncate truncates minor units to the nearest integer multiple of unitStep.
func RoundTruncate(amount int64, unitStep int64) int64 {
	if unitStep <= 1 {
		return amount
	}
	return (amount / unitStep) * unitStep
}

// RoundFloatToRupeePaise converts a float64 INR amount to integer paise using symmetric rounding.
func RoundFloatToRupeePaise(amountINR float64) int64 {
	if amountINR == 0 {
		return 0
	}
	sign := 1.0
	if amountINR < 0 {
		sign = -1.0
		amountINR = -amountINR
	}
	return int64(sign * (amountINR*100.0 + 0.5))
}

// RoundINR rounds a float64 INR amount to 2 decimal places using integer paise arithmetic.
func RoundINR(amountINR float64) float64 {
	return NewINR(RoundFloatToRupeePaise(amountINR)).MajorUnitsFloat()
}

// RoundWholeRupeeINR rounds a float64 INR amount to the nearest whole Rupee via integer paise.
func RoundWholeRupeeINR(amountINR float64) float64 {
	return NewINR(RoundNearestRupee(RoundFloatToRupeePaise(amountINR))).MajorUnitsFloat()
}

// RoundCustomsOneDecimalINR rounds a float64 INR amount to 0.10 INR (10 paise) via integer paise.
func RoundCustomsOneDecimalINR(amountINR float64) float64 {
	// Multiply by 1000 first to preserve the hundredths digit before rounding to 10 paise.
	if amountINR == 0 {
		return 0
	}
	sign := 1.0
	val := amountINR
	if val < 0 {
		sign = -1.0
		val = -val
	}
	tenths := int64(sign * math.Floor(val*10.0+0.5+1e-9))
	return float64(tenths) / 10.0
}

// SumINR adds INR amounts in integer paise to prevent floating-point drift.
func SumINR(amounts ...float64) float64 {
	var totalPaise int64
	for _, amt := range amounts {
		totalPaise += RoundFloatToRupeePaise(amt)
	}
	return NewINR(totalPaise).MajorUnitsFloat()
}

// PerUnitINR divides totalINR by qty and rounds the result via integer paise.
func PerUnitINR(totalINR, qty float64) float64 {
	if qty <= 0 {
		return 0
	}
	return RoundINR(totalINR / qty)
}

// ProratePaise apportions totalPaise by partBasis/totalBasis (clamped to [0, 1]),
// returning the integer paise share and the 4-decimal allocation ratio.
func ProratePaise(totalPaise int64, partBasis, totalBasis float64) (int64, float64) {
	ratio := 1.0
	if totalBasis > 0 && partBasis > 0 {
		ratio = partBasis / totalBasis
	}
	if ratio > 1.0 {
		ratio = 1.0
	} else if ratio < 0.0 {
		ratio = 0.0
	}
	roundedRatio := math.Round(ratio*10000.0) / 10000.0
	allocated := int64(math.Round(float64(totalPaise) * ratio))
	return allocated, roundedRatio
}

// ProrateINR apportions an INR amount by partBasis/totalBasis in integer paise.
func ProrateINR(totalINR, partBasis, totalBasis float64) float64 {
	if totalINR == 0 || totalBasis <= 0 || partBasis <= 0 {
		return 0
	}
	allocatedPaise, _ := ProratePaise(RoundFloatToRupeePaise(totalINR), partBasis, totalBasis)
	return NewINR(allocatedPaise).MajorUnitsFloat()
}
