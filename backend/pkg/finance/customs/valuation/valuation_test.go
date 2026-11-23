package valuation

import (
	"math"
	"testing"

	"github.com/janayuv/import-manager-next/backend/pkg/finance/money"
)

func TestValuationCIFZeroLocked(t *testing.T) {
	params := ValuationParams{
		Incoterm:     "CIF",
		ExchangeRate: 84.0,
		FreightCost:  50000.0, // should be forced to 0
		LineItems: []ValuationLineInput{
			{PartNumber: "P-001", Quantity: 100, UnitPrice: 10.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if res.TotalFreightINR != 0 {
		t.Errorf("Expected CIF freight to be locked to 0, got %f", res.TotalFreightINR)
	}
	if res.TotalInsuranceINR != 0 {
		t.Errorf("Expected CIF insurance to be locked to 0, got %f", res.TotalInsuranceINR)
	}
	// Forex = 100 * 10 = 1000 USD * 84 = 84000 INR
	if res.TotalAssessableValue != 84000.0 {
		t.Errorf("Expected Assessable Value 84000.0, got %f", res.TotalAssessableValue)
	}
	if res.ValuationRuleApplied != ValuationRuleIncluded {
		t.Errorf("Expected rule %s, got %s", ValuationRuleIncluded, res.ValuationRuleApplied)
	}
}

func TestValuationFOBRule10_2DeemedInsurance(t *testing.T) {
	// FOB: Invoice = 1000 USD * 80 = 80,000 INR. Freight = 10,000 INR.
	// No insurance provided => Rule 10(2) Deemed 1.125% on FOB: 80,000 * 0.01125 = 900.0 INR.
	// AV = 80,000 + 10,000 + 900 = 90,900.0 INR.
	params := ValuationParams{
		Incoterm:      "FOB",
		ExchangeRate:  80.0,
		FreightCost:   10000.0,
		InsuranceMode: "deemed_1.125",
		LineItems: []ValuationLineInput{
			{PartNumber: "ITEM-A", Quantity: 50, UnitPrice: 20.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if math.Abs(res.TotalInsuranceINR-900.0) > 0.1 {
		t.Errorf("Expected Deemed Insurance 900.0 INR, got %f", res.TotalInsuranceINR)
	}
	if math.Abs(res.TotalAssessableValue-90900.0) > 0.1 {
		t.Errorf("Expected AV 90900.0 INR, got %f", res.TotalAssessableValue)
	}
	if res.ValuationRuleApplied != ValuationRule10_2_FOBDeemed {
		t.Errorf("Expected rule %s, got %s", ValuationRule10_2_FOBDeemed, res.ValuationRuleApplied)
	}
}

func TestValuationAirFreightCap(t *testing.T) {
	// FOB = 100,000 INR. Air transport cap = 20% = 20,000 INR.
	// If user enters FreightCost = 35,000 INR, it must be capped at 20,000 INR.
	params := ValuationParams{
		Incoterm:       "FOB",
		ExchangeRate:   100.0,
		FreightCost:    35000.0,
		IsAirTransport: true,
		InsuranceMode:  "included",
		LineItems: []ValuationLineInput{
			{PartNumber: "AIR-01", Quantity: 10, UnitPrice: 100.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if !res.AirFreightCapped {
		t.Errorf("Expected airFreightCapped to be true")
	}
	if res.TotalFreightINR != 20000.0 {
		t.Errorf("Expected capped freight 20,000 INR, got %f", res.TotalFreightINR)
	}
}

func TestValuationOpenCoverMarineRate(t *testing.T) {
	// Rate = 0.015%. FOB = 1,000,000 INR.
	// Insurance = 1,000,000 * 0.00015 = 150.0 INR.
	params := ValuationParams{
		Incoterm:      "FOB",
		ExchangeRate:  100.0,
		FreightCost:   0,
		InsuranceRate: money.RateFromPercentageFloat(0.015),
		InsuranceMode: "rate",
		LineItems: []ValuationLineInput{
			{PartNumber: "MAR-01", Quantity: 1000, UnitPrice: 10.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if math.Abs(res.TotalInsuranceINR-150.0) > 0.1 {
		t.Errorf("Expected open cover insurance 150.0 INR, got %f", res.TotalInsuranceINR)
	}
}
