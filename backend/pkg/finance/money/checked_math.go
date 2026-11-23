package money

import (
	"errors"
	"math"
	"math/big"
)

var (
	ErrOverflow = errors.New("integer arithmetic overflow")
)

// SafeAdd returns a + b or ErrOverflow if int64 limits are exceeded.
func SafeAdd(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, ErrOverflow
	}
	return a + b, nil
}

// SafeSub returns a - b or ErrOverflow if int64 limits are exceeded.
func SafeSub(a, b int64) (int64, error) {
	if (b < 0 && a > math.MaxInt64+b) || (b > 0 && a < math.MinInt64+b) {
		return 0, ErrOverflow
	}
	return a - b, nil
}

// MulDiv computes (amount * rate.PPB) / RateScale using big.Int for 128-bit safety.
// Does NOT perform rounding; returns truncated quotient and remainder.
func MulDiv(amount int64, rate DecimalRate) (quotient int64, remainder int64, err error) {
	if rate.PPB == 0 || amount == 0 {
		return 0, 0, nil
	}

	a := big.NewInt(amount)
	b := big.NewInt(rate.PPB)
	prod := new(big.Int).Mul(a, b)

	scale := big.NewInt(RateScale)
	q := new(big.Int)
	rem := new(big.Int)
	q.QuoRem(prod, scale, rem)

	if !q.IsInt64() {
		return 0, 0, ErrOverflow
	}

	return q.Int64(), rem.Int64(), nil
}

// ApplyRate computes amount * rate with explicit Half-Up rounding to nearest minor unit.
func ApplyRate(amount int64, rate DecimalRate) (int64, error) {
	if rate.PPB == 0 || amount == 0 {
		return 0, nil
	}

	a := big.NewInt(amount)
	b := big.NewInt(rate.PPB)
	prod := new(big.Int).Mul(a, b)

	scale := big.NewInt(RateScale)
	halfScale := big.NewInt(RateScale / 2)

	// Half-away-from-zero rounding:
	// If prod >= 0: (prod + scale/2) / scale
	// If prod < 0:  (prod - scale/2) / scale
	if prod.Sign() >= 0 {
		prod.Add(prod, halfScale)
	} else {
		prod.Sub(prod, halfScale)
	}

	res := new(big.Int).Quo(prod, scale)
	if !res.IsInt64() {
		return 0, ErrOverflow
	}
	return res.Int64(), nil
}
