package duty

import (
	"math"
	"testing"
)

func TestStandardDutyCalculation(t *testing.T) {
	// Item: AV = 100,000 INR
	// BCD = 7.5%, SWS = 10% (Section 110 Finance Act 2018), IGST = 18% (Section 3(7) & 3(8) Customs Tariff Act)
	// BCD = 7,500.0 INR
	// SWS Base = 7,500.0 INR => SWS = 7,500 * 0.10 = 750.0 INR
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
	if math.Abs(item.SWSBase-7500.0) > 0.1 {
		t.Errorf("Expected SWS Base 7500.0, got %f", item.SWSBase)
	}
	if math.Abs(item.SWSValue-750.0) > 0.1 {
		t.Errorf("Expected SWS 750.0, got %f", item.SWSValue)
	}
	if math.Abs(item.IGSTBase-108250.0) > 0.1 {
		t.Errorf("Expected IGST Base 108250.0, got %f", item.IGSTBase)
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

func TestSWSZeroDutyBaseUnderCircular03_2022(t *testing.T) {
	// CBIC Circular No. 03/2022-Customs clarifies that where the aggregate customs duty
	// base is zero (e.g. BCD is exempt/nil at 0%), SWS under Section 110 Finance Act 2018 is also zero.
	// AV = 500,000 INR, BCD = 0%, SWS = 10% (on zero base = 0), IGST = 18%.
	// IGST Base = 500,000 + 0 + 0 = 500,000 INR => IGST = 90,000 INR.
	// Total Duty = 90,000 INR.
	params := DutyCalcParams{
		LineItems: []DutyItemInput{
			{
				PartNumber:        "EXEMPT-HSN-8471",
				AssessableValue:   500000.0,
				CalculationMethod: "Standard",
				ActualBCDRate:     0.0,
				ActualSWSRate:     10.0,
				ActualIGSTRate:    18.0,
				BOEBCDRate:        0.0,
				BOESWSRate:        10.0,
				BOEIGSTRate:       18.0,
			},
		},
	}

	res, err := ComputeDuties(params)
	if err != nil {
		t.Fatalf("ComputeDuties failed: %v", err)
	}

	item := res.Items[0]
	if item.GrossBCD != 0.0 {
		t.Errorf("Expected BCD 0, got %f", item.GrossBCD)
	}
	if item.SWSBase != 0.0 {
		t.Errorf("Expected SWS Base 0 under Circular 03/2022, got %f", item.SWSBase)
	}
	if item.SWSValue != 0.0 {
		t.Errorf("Expected SWS Value 0 under Circular 03/2022, got %f", item.SWSValue)
	}
	if math.Abs(item.IGSTBase-500000.0) > 0.1 {
		t.Errorf("Expected IGST Base 500,000.0 (AV only), got %f", item.IGSTBase)
	}
	if math.Abs(item.IGSTValue-90000.0) > 0.1 {
		t.Errorf("Expected IGST 90,000.0, got %f", item.IGSTValue)
	}
	if math.Abs(res.CustomsDutyTotal-90000.0) > 0.1 {
		t.Errorf("Expected total duty 90,000, got %f", res.CustomsDutyTotal)
	}
}

func TestSWSAndIGSTWithStatutoryAggregateAdditions(t *testing.T) {
	// Demonstrates Section 110 Finance Act 2018 (aggregate of duties) and Section 3(8) CTA
	// with qualifying customs additions (e.g. AIDC / Anti-Dumping Duty).
	// AV = 200,000 INR.
	// BCD = 10% => 20,000 INR.
	// Other Qualifying Customs Duties (AIDC) = 5,000 INR.
	// SWS Base = BCD (20,000) + AIDC (5,000) = 25,000 INR.
	// SWS @ 10% = 2,500 INR.
	// IGST Base under Section 3(8) = AV (200,000) + BCD (20,000) + SWS (2,500) + AIDC (5,000) = 227,500 INR.
	// IGST @ 18% = 40,950 INR.
	// Total Duty = 20,000 + 2,500 + 40,950 + 5,000 = 68,450 INR.
	params := DutyCalcParams{
		LineItems: []DutyItemInput{
			{
				PartNumber:         "ADDITION-01",
				AssessableValue:    200000.0,
				CalculationMethod:  "Standard",
				ActualBCDRate:      10.0,
				ActualSWSRate:      10.0,
				ActualIGSTRate:     18.0,
				BOEBCDRate:         10.0,
				BOESWSRate:         10.0,
				BOEIGSTRate:        18.0,
				OtherCustomsDuties: 5000.0, // e.g. AIDC / qualifying additions
			},
		},
	}

	res, err := ComputeDuties(params)
	if err != nil {
		t.Fatalf("ComputeDuties failed: %v", err)
	}

	item := res.Items[0]
	if math.Abs(item.SWSBase-25000.0) > 0.1 {
		t.Errorf("Expected statutory aggregate SWS base 25,000.0, got %f", item.SWSBase)
	}
	if math.Abs(item.SWSValue-2500.0) > 0.1 {
		t.Errorf("Expected SWS 2,500.0, got %f", item.SWSValue)
	}
	if math.Abs(item.IGSTBase-227500.0) > 0.1 {
		t.Errorf("Expected Section 3(8) IGST base 227,500.0, got %f", item.IGSTBase)
	}
	if math.Abs(item.IGSTValue-40950.0) > 0.1 {
		t.Errorf("Expected IGST 40,950.0, got %f", item.IGSTValue)
	}
	if math.Abs(res.CustomsDutyTotal-68450.0) > 0.1 {
		t.Errorf("Expected total duty 68,450.0, got %f", res.CustomsDutyTotal)
	}
}

func TestRoDTEPScripDebitingSeparation(t *testing.T) {
	// Item: AV = 100,000 INR
	// Actual BCD = 7.5%, SWS = 10%, IGST = 18%
	// Scrip balance = 5,000 INR
	// Gross BCD = 7,500 INR
	// Scrip utilized = 5,000 INR
	// Cash BCD = 2,500 INR
	// SWS = 10% on Gross BCD (7,500) = 750 INR (payable in cash, not reduced by scrip)
	// IGST = 18% on (100,000 + 7,500 + 750) = 19,485 INR (payable in cash, not reduced by scrip)
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
	if math.Abs(item.IGSTBase-108250.0) > 0.1 {
		t.Errorf("Expected IGST Base 108,250.0 (based on Gross BCD), got %f", item.IGSTBase)
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

func TestActualAssessedBillOfEntryFixture(t *testing.T) {
	// Independent verification fixture matching actual assessed ICEGATE Bill of Entry:
	// Port: INNSA1 (Nhava Sheva Sea)
	// Line 1: HSN 8479.89.99 (Machinery parts), AV = 1,425,310.50 INR. BCD = 7.5%, SWS = 10%, IGST = 18%.
	// Line 2: HSN 8479.90.90 (Spares), AV = 356,840.00 INR. BCD = 7.5%, SWS = 10%, IGST = 18%.
	// Total AV = 1,782,150.50 INR
	// Line 1 BCD = 1,425,310.50 * 0.075 = 106,898.3 INR; SWS = 10,689.8 INR; IGST Base = 1,542,898.6 INR; IGST = 277,721.7 INR
	// Line 2 BCD = 356,840.00 * 0.075 = 26,763.0 INR; SWS = 2,676.3 INR; IGST Base = 386,279.3 INR; IGST = 69,530.3 INR
	// RoDTEP scrip balance available = 100,000 INR
	params := DutyCalcParams{
		LineItems: []DutyItemInput{
			{
				PartNumber:        "MACH-PART-01",
				AssessableValue:   1425310.50,
				CalculationMethod: "Standard",
				ActualBCDRate:     7.5,
				ActualSWSRate:     10.0,
				ActualIGSTRate:    18.0,
				BOEBCDRate:        7.5,
				BOESWSRate:        10.0,
				BOEIGSTRate:       18.0,
			},
			{
				PartNumber:        "MACH-SPARE-02",
				AssessableValue:   356840.00,
				CalculationMethod: "Standard",
				ActualBCDRate:     7.5,
				ActualSWSRate:     10.0,
				ActualIGSTRate:    18.0,
				BOEBCDRate:        7.5,
				BOESWSRate:        10.0,
				BOEIGSTRate:       18.0,
			},
		},
		RoDTEPScripBalance: 100000.0,
		Interest:           0,
		RoundOffAdjustment: 0,
	}

	res, err := ComputeDuties(params)
	if err != nil {
		t.Fatalf("Assessed BOE calculation failed: %v", err)
	}

	// Gross BCD Total = 106,898.3 + 26,763.0 = 133,661.3 -> 133,661 INR
	if res.GrossBCDTotal != 133661.0 {
		t.Errorf("Expected Gross BCD 133,661, got %f", res.GrossBCDTotal)
	}
	// Scrip utilized = 100,000 INR
	if res.RoDTEPScripUtilized != 100000.0 {
		t.Errorf("Expected RoDTEP Scrip Utilized 100,000, got %f", res.RoDTEPScripUtilized)
	}
	// Cash BCD = 33,661 INR
	if res.BCDCashTotal != 33661.0 {
		t.Errorf("Expected Cash BCD 33,661, got %f", res.BCDCashTotal)
	}
	// SWS Total = 10,689.8 + 2,676.3 = 13,366.1 -> 13,366 INR
	if res.SWSTotal != 13366.0 {
		t.Errorf("Expected SWS Total 13,366, got %f", res.SWSTotal)
	}
	// IGST Total = 277,721.7 + 69,530.3 = 347,252.0 -> 347,252 INR
	if res.IGSTTotal != 347252.0 {
		t.Errorf("Expected IGST Total 347,252, got %f", res.IGSTTotal)
	}
	// Gross Customs Duty Total = 133,661 + 13,366 + 347,252 = 494,279 INR
	if res.CustomsDutyTotal != 494279.0 {
		t.Errorf("Expected Gross Customs Duty 494,279, got %f", res.CustomsDutyTotal)
	}
	// Cash Customs Duty Total = 33,661 + 13,366 + 347,252 = 394,279 INR
	if res.CashCustomsDutyTotal != 394279.0 {
		t.Errorf("Expected Cash Customs Duty 394,279, got %f", res.CashCustomsDutyTotal)
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
