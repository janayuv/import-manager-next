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
		{math.MinInt64, -9223372036854775800},
		{math.MaxInt64, 9223372036854775800},
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
		{math.MinInt64, -9223372036854775800},
		{math.MaxInt64, 9223372036854775800},
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

func TestINRHelpersAndProration(t *testing.T) {
	if got := RoundINR(123.456); got != 123.46 {
		t.Fatalf("RoundINR(123.456) = %.2f, want 123.46", got)
	}
	if got := SumINR(10.10, 20.20, 30.30); got != 60.60 {
		t.Fatalf("SumINR = %.2f, want 60.60", got)
	}
	if got := PerUnitINR(1000.0, 3.0); got != 333.33 {
		t.Fatalf("PerUnitINR(1000, 3) = %.2f, want 333.33", got)
	}
	if got := PerUnitINR(1000.0, 0); got != 0 {
		t.Fatalf("PerUnitINR with zero qty = %.2f, want 0", got)
	}
	paise, ratio := ProratePaise(100000, 250.0, 1000.0)
	if paise != 25000 || ratio != 0.25 {
		t.Fatalf("ProratePaise = (%d, %.4f), want (25000, 0.2500)", paise, ratio)
	}
	if got := ProrateINR(2500.0, 500.0, 1000.0); got != 1250.0 {
		t.Fatalf("ProrateINR = %.2f, want 1250.00", got)
	}
}
