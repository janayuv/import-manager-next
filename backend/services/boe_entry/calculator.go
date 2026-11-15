package boe_entry

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
)

const (
	MaxBOEPayloadBytes  = 1_048_576
	WarnBOEPayloadBytes = 786_432
)

// roundTo rounds n using half-up semantics matching JS Math.round(n * 10^d) / 10^d.
func roundTo(n float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round((n+math.Copysign(1e-9, n))*factor) / factor
}

// NormalizeMethod canonicalizes customs duty calculation method names.
func NormalizeMethod(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "standard":
		return "Standard", nil
	case "cepa":
		return "CEPA", nil
	case "rodtep":
		return "Rodtep", nil
	default:
		return "", fmt.Errorf("unsupported calculation method %q (expected Standard, CEPA, or Rodtep)", raw)
	}
}

// CalculateDuties computes exact per-item customs valuation & Standard/CEPA/Rodtep duties,
// enforcing Incoterm CIF zero-lock, exact rounding rules, and BCD save blocker flags.
func CalculateDuties(params models.BOECalcParams) (models.BOECalculationResult, error) {
	if params.ExchangeRate < 0 || params.FreightCost < 0 || params.EXWCost < 0 || params.InsuranceRate < 0 || params.Interest < 0 {
		return models.BOECalculationResult{}, errors.New("exchangeRate, freightCost, exwCost, insuranceRate, and interest must be non-negative")
	}
	if len(params.LineItems) == 0 {
		return models.BOECalculationResult{}, errors.New("at least one line item is required for BOE calculation")
	}

	freightCost := params.FreightCost
	exwCost := params.EXWCost
	insuranceRate := params.InsuranceRate
	cifLocked := strings.EqualFold(strings.TrimSpace(params.Incoterm), "CIF")
	if cifLocked {
		freightCost = 0
		exwCost = 0
		insuranceRate = 0
	}

	totalInvoiceValue := 0.0
	for _, it := range params.LineItems {
		if it.Quantity < 0 || it.UnitPrice < 0 {
			return models.BOECalculationResult{}, fmt.Errorf("item %s has negative quantity or unitPrice", it.PartNumber)
		}
		totalInvoiceValue += it.Quantity * it.UnitPrice
	}

	res := models.BOECalculationResult{
		Items:             make([]models.BOECalcLineResult, 0, len(params.LineItems)),
		TotalInvoiceValue: roundTo(totalInvoiceValue, 2),
		Interest:          roundTo(params.Interest, 2),
		CIFZeroLocked:     cifLocked,
	}

	var bcdSum, swsSum, igstSum float64
	for _, it := range params.LineItems {
		method, err := NormalizeMethod(it.CalculationMethod)
		if err != nil {
			return models.BOECalculationResult{}, err
		}

		itemForex := roundTo(it.Quantity*it.UnitPrice, 2)
		itemInr := roundTo(itemForex*params.ExchangeRate, 2)
		var itemFreight, itemExw float64
		if totalInvoiceValue > 0 {
			itemFreight = roundTo((freightCost/totalInvoiceValue)*itemForex, 2)
			itemExw = roundTo((exwCost/totalInvoiceValue)*itemForex, 2)
		}
		itemInsurance := roundTo((itemInr+itemExw)*(insuranceRate/100.0), 1)
		assessable := roundTo(itemInr+itemFreight+itemExw+itemInsurance, 1)

		rBoeBCD := it.BOEBCDRate / 100.0
		rBoeSWS := it.BOESWSRate / 100.0
		rBoeIGST := it.BOEIGSTRate / 100.0
		rActBCD := it.ActualBCDRate / 100.0

		var bcdVal, swsVal, igstVal float64
		switch method {
		case "Rodtep":
			bcdVal = roundTo(assessable*rBoeBCD, 1)
			actualBCD := roundTo(assessable*rActBCD, 1)
			swsVal = roundTo(actualBCD*rBoeSWS, 1)
			igstVal = roundTo((assessable+actualBCD+swsVal)*rBoeIGST, 1)
		default: // Standard & CEPA
			bcdVal = roundTo(assessable*rBoeBCD, 1)
			swsVal = roundTo(bcdVal*rBoeSWS, 1)
			igstVal = roundTo((assessable+bcdVal+swsVal)*rBoeIGST, 1)
		}

		lineDutyTotal := roundTo(bcdVal+swsVal+igstVal, 1)

		discrepancy := it.BOEBCDRate > 0 && it.BOEBCDRate > it.ActualBCDRate
		if discrepancy {
			res.HasBlockingBCDOverride = true
		}

		res.TotalValueINR = roundTo(res.TotalValueINR+itemInr, 2)
		res.TotalFreight = roundTo(res.TotalFreight+itemFreight, 2)
		res.TotalEXW = roundTo(res.TotalEXW+itemExw, 2)
		res.TotalInsurance = roundTo(res.TotalInsurance+itemInsurance, 1)
		res.TotalAssessableValue = roundTo(res.TotalAssessableValue+assessable, 1)

		bcdSum = roundTo(bcdSum+bcdVal, 1)
		swsSum = roundTo(swsSum+swsVal, 1)
		igstSum = roundTo(igstSum+igstVal, 1)

		res.Items = append(res.Items, BOECalcLineResultFromInput(it, method, itemForex, itemInr, itemFreight, itemExw, itemInsurance, assessable, bcdVal, swsVal, igstVal, lineDutyTotal, discrepancy))
	}

	res.BCDTotal = roundTo(bcdSum, 0)
	res.SWSTotal = roundTo(swsSum, 0)
	res.IGSTTotal = roundTo(igstSum, 0)
	res.Interest = roundTo(params.Interest, 0)
	res.CustomsDutyTotal = roundTo(res.BCDTotal+res.SWSTotal+res.IGSTTotal+res.Interest, 0)

	if blob, err := json.Marshal(res); err == nil {
		res.PayloadBytes = len(blob)
		res.PayloadWarning = len(blob) >= WarnBOEPayloadBytes
	}
	return res, nil
}

func BOECalcLineResultFromInput(
	in models.BOECalculationItemInput,
	method string,
	forex, inr, freight, exw, insurance, assessable, bcd, sws, igst, total float64,
	discrepancy bool,
) models.BOECalcLineResult {
	return models.BOECalcLineResult{
		ItemID:            in.ItemID,
		PartNumber:        in.PartNumber,
		Description:       in.Description,
		HSNCode:           in.HSNCode,
		Unit:              in.Unit,
		Quantity:          in.Quantity,
		UnitPrice:         in.UnitPrice,
		ItemForexValue:    forex,
		ItemValueINR:      inr,
		ItemFreight:       freight,
		ItemEXW:           exw,
		ItemInsurance:     insurance,
		AssessableValue:   assessable,
		CalculationMethod: method,
		ActualBCDRate:     in.ActualBCDRate,
		ActualSWSRate:     in.ActualSWSRate,
		ActualIGSTRate:    in.ActualIGSTRate,
		BOEBCDRate:        in.BOEBCDRate,
		BOESWSRate:        in.BOESWSRate,
		BOEIGSTRate:       in.BOEIGSTRate,
		BCDValue:          bcd,
		SWSValue:          sws,
		IGSTValue:         igst,
		TotalLineDuty:     total,
		BCDDiscrepancy:    discrepancy,
	}
}
