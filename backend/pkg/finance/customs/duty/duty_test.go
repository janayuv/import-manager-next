package duty

import (
	"math"
	"testing"
)

func TestStandardDutyCalculation(t *testing.T) {
	// Item: AV = 100,000 INR
	// BCD = 7.5%, SWS = 10% (of BCD), IGST = 18% (of AV + BCD + SWS)
	// BCD = 7,500.0 INR
	// SWS = 7,500 * 0.10 = 750.0 INR
	// IGST Base = 100,000 + 7,500 + 750 = 108,250.0 INR => IGST = 108,250 * 0.18 = 19,485.0 INR
	// Total Duty = 7,500 + 750 + 19,485 = 27,735.0 INR
	params := DutyCalcParams{
		LineItems: []DutyItemInput{
			{
				PartNumber:        "STD-01",
				AssessableValue:   100000.0,
				CalculationMethod: "Standard",
				ActualBCDRate:     7.5,
				ActualSWSRate:     10.0,
				ActualIGSTRate:    18.0,
				BOEBCDRate:        7.5,
				BOESWSRate:        10.0,
				BOEIGSTRate:       18.0,
			},
		},
		RoDTEPScripBalance: 0,
		Interest:           0,
		RoundOffAdjustment: 0,
	}

	res, err := ComputeDuties(params)
	if err != nil {
		t.Fatalf("ComputeDuties failed: %v", err)
	}

	if len(res.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]

	if math.Abs(item.GrossBCD-7500.0) > 0.1 {
		t.Errorf("Expected Gross BCD 7500.0, got %f", item.GrossBCD)
	}
	if math.Abs(item.SWSValue-750.0) > 0.1 {
		t.Errorf("Expected SWS 750.0, got %f", item.SWSValue)
	}
	if math.Abs(item.IGSTValue-19485.0) > 0.1 {
		t.Errorf("Expected IGST 19485.0, got %f", item.IGSTValue)
	}
	if math.Abs(res.CustomsDutyTotal-27735.0) > 0.1 {
		t.Errorf("Expected CustomsDutyTotal 27735.0, got %f", res.CustomsDutyTotal)
	}
	if math.Abs(res.CashCustomsDutyTotal-27735.0) > 0.1 {
		t.Errorf("Expected CashCustomsDutyTotal 27735.0, got %f", res.CashCustomsDutyTotal)
	}
}

func TestRoDTEPScripDebiting(t *testing.T) {
	// Item: AV = 100,000 INR
	// Actual BCD = 7.5%, SWS = 10%, IGST = 18%
	// Scrip balance = 5,000 INR
	// Gross BCD = 7,500 INR
	// Scrip utilized = 5,000 INR
	// Cash BCD = 2,500 INR
	// SWS = 10% on Gross BCD (7,500) = 750 INR (payable in cash)
	// IGST = 18% on (100,000 + 7,500 + 750) = 19,485 INR (payable in cash)
	// Gross Duty Total = 7,500 + 750 + 19,485 = 27,735 INR
	// Cash Duty Total = 2,500 + 750 + 19,485 = 22,735 INR
	params := DutyCalcParams{
		LineItems: []DutyItemInput{
			{
				PartNumber:        "ROD-01",
				AssessableValue:   100000.0,
				CalculationMethod: "Rodtep",
				ActualBCDRate:     7.5,
				ActualSWSRate:     10.0,
				ActualIGSTRate:    18.0,
				BOEBCDRate:        0, // In RoDTEP, 0 signals scrip settlement against statutory BCD
				BOESWSRate:        10.0,
				BOEIGSTRate:       18.0,
			},
		},
		RoDTEPScripBalance: 5000.0,
		Interest:           0,
		RoundOffAdjustment: 0,
	}

	res, err := ComputeDuties(params)
	if err != nil {
		t.Fatalf("ComputeDuties failed: %v", err)
	}

	item := res.Items[0]
	if math.Abs(item.GrossBCD-7500.0) > 0.1 {
		t.Errorf("Expected Gross BCD 7500.0, got %f", item.GrossBCD)
	}
	if math.Abs(item.RoDTEPUtilized-5000.0) > 0.1 {
		t.Errorf("Expected RoDTEP Utilized 5000.0, got %f", item.RoDTEPUtilized)
	}
	if math.Abs(item.BCDCashPayable-2500.0) > 0.1 {
		t.Errorf("Expected Cash BCD 2500.0, got %f", item.BCDCashPayable)
	}
	if math.Abs(item.SWSValue-750.0) > 0.1 {
		t.Errorf("Expected SWS 750.0 (assessed on Gross BCD), got %f", item.SWSValue)
	}
	if math.Abs(res.RoDTEPScripUtilized-5000.0) > 0.1 {
		t.Errorf("Expected Scrip Utilized 5000, got %f", res.RoDTEPScripUtilized)
	}
	if math.Abs(res.CustomsDutyTotal-27735.0) > 0.1 {
		t.Errorf("Expected CustomsDutyTotal 27735.0, got %f", res.CustomsDutyTotal)
	}
	if math.Abs(res.CashCustomsDutyTotal-22735.0) > 0.1 {
		t.Errorf("Expected CashCustomsDutyTotal 22735.0, got %f", res.CashCustomsDutyTotal)
	}
}

func TestBlockingBCDOverride(t *testing.T) {
	// Discrepancy where BOE BCD rate exceeds statutory actual BCD rate
	params := DutyCalcParams{
		LineItems: []DutyItemInput{
			{
				PartNumber:        "DISC-01",
				AssessableValue:   50000.0,
				CalculationMethod: "Standard",
				ActualBCDRate:     5.0,
				BOEBCDRate:        10.0, // BOE > Actual -> blocking discrepancy
				BOESWSRate:        10.0,
				BOEIGSTRate:       18.0,
			},
		},
	}

	res, err := ComputeDuties(params)
	if err != nil {
		t.Fatalf("ComputeDuties failed: %v", err)
	}

	if !res.HasBlockingBCDOverride {
		t.Errorf("Expected HasBlockingBCDOverride to be true")
	}
	if !res.Items[0].BCDDiscrepancy {
		t.Errorf("Expected line BCDDiscrepancy to be true")
	}
}
