package money

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
// Correctly handles negative adjustments symmetrically:
// e.g. -140 paise -> -100 paise; -160 paise -> -200 paise; +140 -> +100; +160 -> +200.
func RoundNearestRupee(paise int64) int64 {
	if paise == 0 {
		return 0
	}
	sign := int64(1)
	val := paise
	if val < 0 {
		sign = -1
		val = -val
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
	sign := int64(1)
	val := paise
	if val < 0 {
		sign = -1
		val = -val
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
