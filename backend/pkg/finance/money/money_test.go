package money

import (
	"math"
	"testing"
)

func TestCurrencyScaleFactors(t *testing.T) {
	tests := []struct {
		code     string
		decimals int
		scale    int64
	}{
		{"INR", 2, 100},
		{"USD", 2, 100},
		{"EUR", 2, 100},
		{"JPY", 0, 1},
		{"KRW", 0, 1},
		{"BHD", 3, 1000},
		{"KWD", 3, 1000},
	}

	for _, tc := range tests {
		c := GetCurrency(tc.code)
		if c.MinorUnitDecimals != tc.decimals {
			t.Errorf("Currency %s expected %d decimals, got %d", tc.code, tc.decimals, c.MinorUnitDecimals)
		}
		if c.ScaleFactor != tc.scale {
			t.Errorf("Currency %s expected scale %d, got %d", tc.code, tc.scale, c.ScaleFactor)
		}
	}
}

func TestDecimalRatePrecision(t *testing.T) {
	// 0.015% Open Cover Marine Insurance
	rOpenCover := RateFromPercentageFloat(0.015)
	if rOpenCover.PPB != 150_000 {
		t.Errorf("Expected 150,000 PPB for 0.015%%, got %d", rOpenCover.PPB)
	}
	if math.Abs(rOpenCover.PercentageFloat()-0.015) > 1e-6 {
		t.Errorf("Expected 0.015%% back, got %f", rOpenCover.PercentageFloat())
	}

	// 1.125% Rule 10(2) Deemed Insurance
	rDeemed := RateFromPercentageFloat(1.125)
	if rDeemed.PPB != 11_250_000 {
		t.Errorf("Expected 11,250,000 PPB for 1.125%%, got %d", rDeemed.PPB)
	}

	// 18.00% Standard GST
	rGST := RateFromPercentageFloat(18.0)
	if rGST.PPB != 180_000_000 {
		t.Errorf("Expected 180,000,000 PPB for 18%%, got %d", rGST.PPB)
	}
	if rGST.BasisPoints() != 1800 {
		t.Errorf("Expected 1800 bp, got %d", rGST.BasisPoints())
	}
}

func TestSymmetricRoundingPolicies(t *testing.T) {
	// Nearest Rupee: symmetric around 0
	testsRupee := []struct {
		input    int64
		expected int64
	}{
		{0, 0},
		{49, 0},
		{50, 100},
		{140, 100},
		{150, 200},
		{160, 200},
		{-49, 0},
		{-50, -100},
		{-140, -100},
		{-150, -200},
		{-160, -200},
	}
	for _, tc := range testsRupee {
		got := RoundNearestRupee(tc.input)
		if got != tc.expected {
			t.Errorf("RoundNearestRupee(%d) expected %d, got %d", tc.input, tc.expected, got)
		}
	}

	// Customs One-Decimal (10 paise = 0.10 INR)
	testsCustoms := []struct {
		input    int64
		expected int64
	}{
		{124, 120},
		{125, 130},
		{129, 130},
		{-124, -120},
		{-125, -130},
	}
	for _, tc := range testsCustoms {
		got := RoundCustomsOneDecimal(tc.input)
		if got != tc.expected {
			t.Errorf("RoundCustomsOneDecimal(%d) expected %d, got %d", tc.input, tc.expected, got)
		}
	}
}

func TestCheckedArithmeticOverflow(t *testing.T) {
	_, err := SafeAdd(math.MaxInt64, 1)
	if err != ErrOverflow {
		t.Errorf("Expected ErrOverflow on MaxInt64 + 1, got %v", err)
	}

	_, err = SafeSub(math.MinInt64, 1)
	if err != ErrOverflow {
		t.Errorf("Expected ErrOverflow on MinInt64 - 1, got %v", err)
	}

	// Large amount multiplication should succeed without overflow due to big.Int scaling
	hugeAmount := int64(500_000_000_00) // 500 million INR in paise
	rate18 := RateFromPercentageFloat(18.0)
	tax, err := ApplyRate(hugeAmount, rate18)
	if err != nil {
		t.Fatalf("ApplyRate failed on large amount: %v", err)
	}
	expected := int64(90_000_000_00) // 90 million INR in paise
	if tax != expected {
		t.Errorf("Expected %d, got %d", expected, tax)
	}
}
