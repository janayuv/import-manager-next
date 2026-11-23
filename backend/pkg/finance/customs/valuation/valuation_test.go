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
	if !res.DuplicateAdditionsPrevented {
		t.Errorf("Expected DuplicateAdditionsPrevented to be true")
	}
}

func TestValuationRule10_2_ActualTransportAndInsurance(t *testing.T) {
	// Actual ascertainable freight and insurance
	// FOB: 100 * 100 = 10,000 USD * 80 = 800,000 INR
	// Actual Freight = 45,000 INR, Actual Insurance = 3,500 INR
	// Assessable Value = 800,000 + 45,000 + 3,500 = 848,500 INR
	params := ValuationParams{
		Incoterm:        "FOB",
		ExchangeRate:    80.0,
		FreightCost:     45000.0,
		FreightMode:     "actual",
		ActualInsurance: 3500.0,
		InsuranceMode:   "actual",
		LineItems: []ValuationLineInput{
			{PartNumber: "ITEM-ACTUAL", Quantity: 100, UnitPrice: 100.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if res.TotalFreightINR != 45000.0 {
		t.Errorf("Expected freight 45000.0, got %f", res.TotalFreightINR)
	}
	if res.TotalInsuranceINR != 3500.0 {
		t.Errorf("Expected insurance 3500.0, got %f", res.TotalInsuranceINR)
	}
	if math.Abs(res.TotalAssessableValue-848500.0) > 0.1 {
		t.Errorf("Expected assessable value 848500.0, got %f", res.TotalAssessableValue)
	}
	if res.ValuationRuleApplied != ValuationRuleActual {
		t.Errorf("Expected rule %s, got %s", ValuationRuleActual, res.ValuationRuleApplied)
	}
}

func TestValuationRule10_2_TransportFallback20PercentFOB(t *testing.T) {
	// Rule 10(2)(a) 1st proviso: Where cost of transport is not ascertainable,
	// such cost shall be 20% of FOB value.
	// FOB = 50 * 50 = 2,500 USD * 80 = 200,000 INR.
	// Transport fallback = 20% of 200,000 = 40,000 INR.
	// Insurance actual = 1,000 INR.
	// AV = 200,000 + 40,000 + 1,000 = 241,000 INR.
	params := ValuationParams{
		Incoterm:        "FOB",
		ExchangeRate:    80.0,
		FreightMode:     "unascertainable",
		ActualInsurance: 1000.0,
		InsuranceMode:   "actual",
		LineItems: []ValuationLineInput{
			{PartNumber: "FB-01", Quantity: 50, UnitPrice: 50.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if math.Abs(res.TotalFreightINR-40000.0) > 0.1 {
		t.Errorf("Expected transport fallback 40,000 INR (20%% of FOB), got %f", res.TotalFreightINR)
	}
	if res.ValuationRuleApplied != ValuationRule10_2_TransportFOBDeemed {
		t.Errorf("Expected rule %s, got %s", ValuationRule10_2_TransportFOBDeemed, res.ValuationRuleApplied)
	}
	if math.Abs(res.TotalAssessableValue-241000.0) > 0.1 {
		t.Errorf("Expected AV 241,000 INR, got %f", res.TotalAssessableValue)
	}
}

func TestValuationRule10_2_TransportFallback20PercentFOBPlusInsurance(t *testing.T) {
	// Rule 10(2)(a) 2nd proviso: Where FOB is not ascertainable, but the sum of
	// (FOB + Insurance) is ascertainable, transport shall be 20% of such sum.
	// Sum of (FOB + Insurance) = 150,000 INR.
	// Transport fallback = 20% of 150,000 = 30,000 INR.
	params := ValuationParams{
		Incoterm:                      "FOB",
		ExchangeRate:                  80.0,
		FOBUnascertainable:            true,
		FOBPlusInsuranceAscertainable: 150000.0,
		FreightMode:                   "unascertainable",
		ActualInsurance:               1500.0,
		InsuranceMode:                 "actual",
		LineItems: []ValuationLineInput{
			{PartNumber: "UNASC-01", Quantity: 10, UnitPrice: 10.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if math.Abs(res.TotalFreightINR-30000.0) > 0.1 {
		t.Errorf("Expected transport fallback 30,000 INR (20%% of FOB+Ins), got %f", res.TotalFreightINR)
	}
	if res.ValuationRuleApplied != ValuationRule10_2_TransportFOBPlusInsurance {
		t.Errorf("Expected rule %s, got %s", ValuationRule10_2_TransportFOBPlusInsurance, res.ValuationRuleApplied)
	}
}

func TestValuationRule10_2_InsuranceFallback1_125PercentFOB(t *testing.T) {
	// Rule 10(2)(b) 1st proviso: Where cost of insurance is not ascertainable,
	// such cost shall be 1.125% of FOB value.
	// FOB = 2,500 USD * 80 = 200,000 INR. Freight = 15,000 INR.
	// Insurance fallback = 1.125% of 200,000 = 2,250.0 INR.
	// AV = 200,000 + 15,000 + 2,250 = 217,250.0 INR.
	params := ValuationParams{
		Incoterm:      "FOB",
		ExchangeRate:  80.0,
		FreightCost:   15000.0,
		FreightMode:   "actual",
		InsuranceMode: "unascertainable",
		LineItems: []ValuationLineInput{
			{PartNumber: "INS-FOB", Quantity: 50, UnitPrice: 50.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if math.Abs(res.TotalInsuranceINR-2250.0) > 0.1 {
		t.Errorf("Expected insurance fallback 2,250.0 INR (1.125%% of FOB), got %f", res.TotalInsuranceINR)
	}
	if res.ValuationRuleApplied != ValuationRule10_2_FOBDeemed {
		t.Errorf("Expected rule %s, got %s", ValuationRule10_2_FOBDeemed, res.ValuationRuleApplied)
	}
	if math.Abs(res.TotalAssessableValue-217250.0) > 0.1 {
		t.Errorf("Expected AV 217,250.0 INR, got %f", res.TotalAssessableValue)
	}
}

func TestValuationRule10_2_InsuranceFallback1_125PercentFOBPlusTransport(t *testing.T) {
	// Rule 10(2)(b) 2nd proviso: Where FOB is not ascertainable, but the sum of
	// (FOB + Transport) is ascertainable, insurance shall be 1.125% of such sum.
	// Sum of (FOB + Transport) = 250,000 INR.
	// Insurance fallback = 1.125% of 250,000 = 2,812.5 INR.
	params := ValuationParams{
		Incoterm:                      "FOB",
		ExchangeRate:                  80.0,
		FOBUnascertainable:            true,
		FOBPlusTransportAscertainable: 250000.0,
		FreightCost:                   25000.0,
		FreightMode:                   "actual",
		InsuranceMode:                 "unascertainable",
		LineItems: []ValuationLineInput{
			{PartNumber: "INS-FOB-TRANS", Quantity: 10, UnitPrice: 10.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("ComputeAssessableValue failed: %v", err)
	}

	if math.Abs(res.TotalInsuranceINR-2812.5) > 0.1 {
		t.Errorf("Expected insurance fallback 2,812.5 INR (1.125%% of FOB+Trans), got %f", res.TotalInsuranceINR)
	}
	if res.ValuationRuleApplied != ValuationRule10_2_FOBPlusTransport {
		t.Errorf("Expected rule %s, got %s", ValuationRule10_2_FOBPlusTransport, res.ValuationRuleApplied)
	}
}

func TestValuationAirFreightCapAndSeaExemption(t *testing.T) {
	// Air freight capped at 20% of FOB under Rule 10(2)(a) 3rd proviso.
	// FOB = 100,000 INR. Actual freight = 35,000 INR.
	// 1. Air Mode: Must be capped at 20,000 INR.
	paramsAir := ValuationParams{
		Incoterm:        "FOB",
		ExchangeRate:    100.0,
		FreightCost:     35000.0,
		FreightMode:     "actual",
		IsAirTransport:  true,
		ActualInsurance: 500.0,
		InsuranceMode:   "actual",
		LineItems: []ValuationLineInput{
			{PartNumber: "AIR-01", Quantity: 10, UnitPrice: 100.0},
		},
	}
	resAir, err := ComputeAssessableValue(paramsAir)
	if err != nil {
		t.Fatalf("Air valuation failed: %v", err)
	}
	if !resAir.AirFreightCapped {
		t.Errorf("Expected air freight to be capped")
	}
	if resAir.TotalFreightINR != 20000.0 {
		t.Errorf("Expected air freight capped at 20,000 INR, got %f", resAir.TotalFreightINR)
	}
	if resAir.ValuationRuleApplied != ValuationRule10_2_AirFreightCapped {
		t.Errorf("Expected rule %s, got %s", ValuationRule10_2_AirFreightCapped, resAir.ValuationRuleApplied)
	}

	// 2. Sea / Ocean Mode: Actual freight exceeding 20% of FOB is NOT capped.
	paramsSea := paramsAir
	paramsSea.IsAirTransport = false
	resSea, err := ComputeAssessableValue(paramsSea)
	if err != nil {
		t.Fatalf("Sea valuation failed: %v", err)
	}
	if resSea.AirFreightCapped {
		t.Errorf("Sea freight must NOT be capped at 20%%")
	}
	if resSea.TotalFreightINR != 35000.0 {
		t.Errorf("Expected full actual sea freight 35,000 INR, got %f", resSea.TotalFreightINR)
	}

	// 3. Air Freight Cap when FOB is unascertainable but (FOB + Insurance) is ascertainable
	paramsAirUnasc := ValuationParams{
		Incoterm:                      "FOB",
		ExchangeRate:                  100.0,
		FreightCost:                   50000.0,
		FreightMode:                   "actual",
		IsAirTransport:                true,
		FOBUnascertainable:            true,
		FOBPlusInsuranceAscertainable: 180000.0,
		LineItems: []ValuationLineInput{
			{PartNumber: "AIR-UNASC", Quantity: 10, UnitPrice: 100.0},
		},
	}
	resAirUnasc, err := ComputeAssessableValue(paramsAirUnasc)
	if err != nil {
		t.Fatalf("Air unascertainable valuation failed: %v", err)
	}
	// Max air freight = 20% of 180,000 = 36,000 INR
	if !resAirUnasc.AirFreightCapped {
		t.Errorf("Expected air freight to be capped on FOB+Ins sum")
	}
	if resAirUnasc.TotalFreightINR != 36000.0 {
		t.Errorf("Expected air freight capped at 36,000 INR, got %f", resAirUnasc.TotalFreightINR)
	}
}

func TestValuationRule10_2_TransshipmentExclusions(t *testing.T) {
	// Rule 10(2) Explanation 2: Post-import transport and inland transshipment charges
	// shall NOT be included in the customs assessable value.
	// FOB = 100,000 INR, Freight = 10,000 INR, Insurance = 1,125 INR.
	// Inland Transshipment = 12,000 INR (e.g. CFS to ICD transport).
	// Assessable Value must be exactly 100,000 + 10,000 + 1,125 = 111,125 INR (excluding 12,000).
	params := ValuationParams{
		Incoterm:                "FOB",
		ExchangeRate:            100.0,
		FreightCost:             10000.0,
		FreightMode:             "actual",
		InsuranceMode:           "deemed_1.125",
		PostImportTransshipment: 12000.0,
		LineItems: []ValuationLineInput{
			{PartNumber: "TRANS-01", Quantity: 10, UnitPrice: 100.0},
		},
	}

	res, err := ComputeAssessableValue(params)
	if err != nil {
		t.Fatalf("Valuation failed: %v", err)
	}

	if res.ExcludedTransshipmentINR != 12000.0 {
		t.Errorf("Expected excluded transshipment 12,000 INR, got %f", res.ExcludedTransshipmentINR)
	}
	if math.Abs(res.TotalAssessableValue-111125.0) > 0.1 {
		t.Errorf("Expected AV 111,125.0 INR (excluding transshipment), got %f", res.TotalAssessableValue)
	}
}

func TestValuationRule10_2_PreventionOfDuplicateCosts(t *testing.T) {
	// For CIF, CIP, DAP, DPU, and DDP: Invoice already includes freight and insurance.
	// The engine must lock additions to 0 to prevent duplicate additions.
	// Also confirms that obsolete 1% landing charges are never added.
	incoterms := []string{"CIF", "CIP", "DAP", "DPU", "DDP"}
	for _, term := range incoterms {
		p := ValuationParams{
			Incoterm:            "CIF",
			ExchangeRate:        80.0,
			FreightCost:         25000.0, // user erroneously provides freight
			ActualInsurance:     2000.0,  // user erroneously provides insurance
			OriginInlandFreight: 5000.0,  // user provides origin handling
			LineItems: []ValuationLineInput{
				{PartNumber: "DUP-01", Quantity: 10, UnitPrice: 100.0}, // 1000 USD * 80 = 80,000 INR
			},
		}
		p.Incoterm = term

		r, err := ComputeAssessableValue(p)
		if err != nil {
			t.Fatalf("Valuation for %s failed: %v", term, err)
		}

		if r.TotalFreightINR != 0 {
			t.Errorf("[%s] Expected freight locked to 0, got %f", term, r.TotalFreightINR)
		}
		if r.TotalInsuranceINR != 0 {
			t.Errorf("[%s] Expected insurance locked to 0, got %f", term, r.TotalInsuranceINR)
		}
		if r.TotalOriginChargesINR != 0 {
			t.Errorf("[%s] Expected origin charges locked to 0, got %f", term, r.TotalOriginChargesINR)
		}
		if math.Abs(r.TotalAssessableValue-80000.0) > 0.1 {
			t.Errorf("[%s] Expected AV exactly 80,000.0 (no landing charges/duplicate additions), got %f", term, r.TotalAssessableValue)
		}
		if !r.DuplicateAdditionsPrevented {
			t.Errorf("[%s] Expected DuplicateAdditionsPrevented = true", term)
		}
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
