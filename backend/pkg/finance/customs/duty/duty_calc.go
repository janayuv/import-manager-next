package duty

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/janayuv/import-manager-next/backend/pkg/finance/money"
)

// CalculationMethod represents statutory tariff assessment scheme.
type CalculationMethod string

const (
	MethodStandard CalculationMethod = "Standard"
	MethodCEPA     CalculationMethod = "CEPA"
	MethodRoDTEP   CalculationMethod = "Rodtep"
)

// NormalizeMethod canonicalizes customs duty calculation method names.
func NormalizeMethod(raw string) (CalculationMethod, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "standard":
		return MethodStandard, nil
	case "cepa":
		return MethodCEPA, nil
	case "rodtep":
		return MethodRoDTEP, nil
	default:
		return "", fmt.Errorf("unsupported customs calculation method %q (expected Standard, CEPA, or Rodtep)", raw)
	}
}

// DutyItemInput represents line inputs for duty calculation.
type DutyItemInput struct {
	ItemID            string  `json:"itemId"`
	PartNumber        string  `json:"partNumber"`
	Description       string  `json:"description"`
	HSNCode           string  `json:"hsnCode"`
	Unit              string  `json:"unit"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unitPrice"`
	AssessableValue   float64 `json:"assessableValue"`   // INR
	CalculationMethod string  `json:"calculationMethod"` // Standard | CEPA | Rodtep
	ActualBCDRate     float64 `json:"actualBcdRate"`     // %
	ActualSWSRate     float64 `json:"actualSwsRate"`     // %
	ActualIGSTRate    float64 `json:"actualIgstRate"`    // %
	BOEBCDRate        float64 `json:"boeBcdRate"`        // %
	BOESWSRate        float64 `json:"boeSwsRate"`        // %
	BOEIGSTRate       float64 `json:"boeIgstRate"`       // %
}

// DutyCalcParams represents parameters for customs duty computation.
type DutyCalcParams struct {
	LineItems          []DutyItemInput `json:"lineItems"`
	RoDTEPScripBalance float64         `json:"rodtepScripBalance"` // INR
	Interest           float64         `json:"interest"`           // INR
	RoundOffAdjustment float64         `json:"roundOffAdjustment"` // ±50 INR
}

// LineDutyResult captures statutory duty breakdown for an item.
type LineDutyResult struct {
	ItemID            string  `json:"itemId"`
	PartNumber        string  `json:"partNumber"`
	Description       string  `json:"description"`
	HSNCode           string  `json:"hsnCode"`
	Unit              string  `json:"unit"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unitPrice"`
	AssessableValue   float64 `json:"assessableValue"`
	CalculationMethod string  `json:"calculationMethod"`
	ActualBCDRate     float64 `json:"actualBcdRate"`
	ActualSWSRate     float64 `json:"actualSwsRate"`
	ActualIGSTRate    float64 `json:"actualIgstRate"`
	BOEBCDRate        float64 `json:"boeBcdRate"`
	BOESWSRate        float64 `json:"boeSwsRate"`
	BOEIGSTRate       float64 `json:"boeIgstRate"`

	GrossBCD          float64 `json:"grossBcd"`          // INR (1 decimal)
	RoDTEPUtilized    float64 `json:"rodtepUtilized"`    // INR (1 decimal)
	BCDCashPayable    float64 `json:"bcdCashPayable"`    // INR (1 decimal)
	SWSValue          float64 `json:"swsValue"`          // INR (1 decimal)
	IGSTValue         float64 `json:"igstValue"`         // INR (1 decimal)
	TotalLineDuty     float64 `json:"totalLineDuty"`     // Gross BCD + SWS + IGST
	TotalLineCashDuty float64 `json:"totalLineCashDuty"` // Cash BCD + SWS + IGST
	BCDDiscrepancy    bool    `json:"bcdDiscrepancy"`    // true if boeBcdRate > actualBcdRate
}

// DutyCalculationResult aggregates all lines and grand duty totals.
type DutyCalculationResult struct {
	Items                  []LineDutyResult `json:"items"`
	GrossBCDTotal          float64          `json:"grossBcdTotal"`          // whole INR
	RoDTEPScripUtilized    float64          `json:"rodtepScripUtilized"`    // whole INR
	BCDCashTotal           float64          `json:"bcdCashTotal"`           // whole INR
	BCDTotal               float64          `json:"bcdTotal"`               // alias for GrossBCDTotal
	SWSTotal               float64          `json:"swsTotal"`               // whole INR
	IGSTTotal              float64          `json:"igstTotal"`              // whole INR
	Interest               float64          `json:"interest"`               // whole INR
	RoundOffAdjustment     float64          `json:"roundOffAdjustment"`     // whole INR
	CustomsDutyTotal       float64          `json:"customsDutyTotal"`       // Gross Duty Total
	CashCustomsDutyTotal   float64          `json:"cashCustomsDutyTotal"`   // Cash Payable Total
	HasBlockingBCDOverride bool             `json:"hasBlockingBcdOverride"` // true if any line has boeBcdRate > actualBcdRate
}

// round1 rounds float to 1 decimal place matching ICEGATE line duty precision.
func round1(val float64) float64 {
	return math.Round((val+math.Copysign(1e-9, val))*10.0) / 10.0
}

// roundWhole rounds float to whole integer matching ICEGATE header total precision.
func roundWhole(val float64) float64 {
	return math.Round(val + math.Copysign(1e-9, val))
}

// ComputeDuties calculates statutory customs duties, RoDTEP electronic scrip debiting,
// and ICEGATE line/header integer roundings.
func ComputeDuties(params DutyCalcParams) (DutyCalculationResult, error) {
	if params.RoDTEPScripBalance < 0 || params.Interest < 0 {
		return DutyCalculationResult{}, errors.New("scrip balance and interest cannot be negative")
	}
	if len(params.LineItems) == 0 {
		return DutyCalculationResult{}, errors.New("at least one line item is required")
	}
	if math.Abs(params.RoundOffAdjustment) > 50 {
		return DutyCalculationResult{}, fmt.Errorf("round-off adjustment cannot exceed ±50 INR (got %.2f)", params.RoundOffAdjustment)
	}

	res := DutyCalculationResult{
		Items: make([]LineDutyResult, 0, len(params.LineItems)),
	}

	type tempLine struct {
		item        DutyItemInput
		method      CalculationMethod
		grossBCD    float64
		swsVal      float64
		igstVal     float64
		discrepancy bool
	}

	temp := make([]tempLine, 0, len(params.LineItems))
	var grossBCDSum, swsSum, igstSum float64
	var rodtepEligibleGrossBCDSum float64

	for _, it := range params.LineItems {
		method, err := NormalizeMethod(it.CalculationMethod)
		if err != nil {
			return DutyCalculationResult{}, err
		}

		bcdRate := it.BOEBCDRate
		if method == MethodRoDTEP && bcdRate == 0 {
			// In RoDTEP, statutory tariff rate applies; scrip debits liability afterwards
			bcdRate = it.ActualBCDRate
		}

		rBCD := bcdRate / 100.0
		rSWS := it.BOESWSRate / 100.0
		rIGST := it.BOEIGSTRate / 100.0

		// Statutory gross duty liability
		grossBCD := round1(it.AssessableValue * rBCD)
		swsVal := round1(grossBCD * rSWS)
		igstVal := round1((it.AssessableValue + grossBCD + swsVal) * rIGST)

		discrepancy := it.BOEBCDRate > 0 && it.BOEBCDRate > it.ActualBCDRate
		if discrepancy {
			res.HasBlockingBCDOverride = true
		}

		grossBCDSum = round1(grossBCDSum + grossBCD)
		swsSum = round1(swsSum + swsVal)
		igstSum = round1(igstSum + igstVal)

		if method == MethodRoDTEP {
			rodtepEligibleGrossBCDSum = round1(rodtepEligibleGrossBCDSum + grossBCD)
		}

		temp = append(temp, tempLine{
			item:        it,
			method:      method,
			grossBCD:    grossBCD,
			swsVal:      swsVal,
			igstVal:     igstVal,
			discrepancy: discrepancy,
		})
	}

	// RoDTEP scrip debiting against BCD liability
	availScrip := params.RoDTEPScripBalance
	if availScrip == 0 && rodtepEligibleGrossBCDSum > 0 {
		availScrip = rodtepEligibleGrossBCDSum
	}
	totalRoDTEPUtilized := math.Min(availScrip, grossBCDSum)

	// Apportion scrip utilization across lines
	for _, tmp := range temp {
		var lineScrip float64
		if grossBCDSum > 0 && totalRoDTEPUtilized > 0 {
			if rodtepEligibleGrossBCDSum > 0 && tmp.method == MethodRoDTEP {
				lineScrip = round1(math.Min(tmp.grossBCD, totalRoDTEPUtilized*(tmp.grossBCD/rodtepEligibleGrossBCDSum)))
			} else if rodtepEligibleGrossBCDSum == 0 {
				lineScrip = round1(totalRoDTEPUtilized * (tmp.grossBCD / grossBCDSum))
			}
		}

		lineBCDCash := round1(tmp.grossBCD - lineScrip)
		lineCashDuty := round1(lineBCDCash + tmp.swsVal + tmp.igstVal)
		lineTotalDuty := round1(tmp.grossBCD + tmp.swsVal + tmp.igstVal)

		res.Items = append(res.Items, LineDutyResult{
			ItemID:            tmp.item.ItemID,
			PartNumber:        tmp.item.PartNumber,
			Description:       tmp.item.Description,
			HSNCode:           tmp.item.HSNCode,
			Unit:              tmp.item.Unit,
			Quantity:          tmp.item.Quantity,
			UnitPrice:         tmp.item.UnitPrice,
			AssessableValue:   tmp.item.AssessableValue,
			CalculationMethod: string(tmp.method),
			ActualBCDRate:     tmp.item.ActualBCDRate,
			ActualSWSRate:     tmp.item.ActualSWSRate,
			ActualIGSTRate:    tmp.item.ActualIGSTRate,
			BOEBCDRate:        tmp.item.BOEBCDRate,
			BOESWSRate:        tmp.item.BOESWSRate,
			BOEIGSTRate:       tmp.item.BOEIGSTRate,
			GrossBCD:          tmp.grossBCD,
			RoDTEPUtilized:    lineScrip,
			BCDCashPayable:    lineBCDCash,
			SWSValue:          tmp.swsVal,
			IGSTValue:         tmp.igstVal,
			TotalLineDuty:     lineTotalDuty,
			TotalLineCashDuty: lineCashDuty,
			BCDDiscrepancy:    tmp.discrepancy,
		})
	}

	// Header aggregates
	roundOffPaise := money.RoundNearestRupee(int64(params.RoundOffAdjustment * 100.0))
	roundOff := float64(roundOffPaise) / 100.0

	res.RoundOffAdjustment = roundWhole(roundOff)
	res.GrossBCDTotal = roundWhole(grossBCDSum)
	res.RoDTEPScripUtilized = roundWhole(totalRoDTEPUtilized)
	res.BCDCashTotal = roundWhole(grossBCDSum - totalRoDTEPUtilized)
	res.BCDTotal = res.GrossBCDTotal
	res.SWSTotal = roundWhole(swsSum)
	res.IGSTTotal = roundWhole(igstSum)
	res.Interest = roundWhole(params.Interest)
	res.CustomsDutyTotal = math.Max(0, roundWhole(res.GrossBCDTotal+res.SWSTotal+res.IGSTTotal+res.Interest+res.RoundOffAdjustment))
	res.CashCustomsDutyTotal = math.Max(0, roundWhole(res.BCDCashTotal+res.SWSTotal+res.IGSTTotal+res.Interest+res.RoundOffAdjustment))

	return res, nil
}
