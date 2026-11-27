package boe_entry

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/pkg/finance/customs/duty"
	"github.com/janayuv/import-manager-next/backend/pkg/finance/customs/valuation"
	"github.com/janayuv/import-manager-next/backend/pkg/finance/money"
)

const (
	MaxBOEPayloadBytes     = 1_048_576
	WarnBOEPayloadBytes    = 786_432
	DefaultOpenCoverRate   = 0.015 // Default 0.015% Open Cover Marine Policy
	StatutoryDeemedInsRate = 1.125 // 1.125% of FOB under Rule 10(2) Customs Valuation Rules 2007
)

// roundTo rounds n using pkg/finance/money integer-paise helpers for 0/1/2 decimals.
func roundTo(n float64, decimals int) float64 {
	switch decimals {
	case 0:
		return money.RoundWholeRupeeINR(n)
	case 1:
		return money.RoundCustomsOneDecimalINR(n)
	case 2:
		return money.RoundINR(n)
	default:
		factor := math.Pow(10, float64(decimals))
		return math.Round((n+math.Copysign(1e-9, n))*factor) / factor
	}
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

// IncotermRules defines the statutory cost inclusion treatment for all 11 Incoterms (2020).
var IncotermRules = map[string]models.IncotermRule{
	"EXW": {
		Code:                  "EXW",
		Name:                  "Ex Works",
		FreightIncluded:       false,
		InsuranceIncluded:     false,
		OriginChargesExpected: true,
		Description:           "Origin inland transport, export handling, international freight and insurance must be added if not included.",
	},
	"FCA": {
		Code:                  "FCA",
		Name:                  "Free Carrier",
		FreightIncluded:       false,
		InsuranceIncluded:     false,
		OriginChargesExpected: true,
		Description:           "Post-handover freight and insurance must be added; check origin terminal charges.",
	},
	"FAS": {
		Code:                  "FAS",
		Name:                  "Free Alongside Ship",
		FreightIncluded:       false,
		InsuranceIncluded:     false,
		OriginChargesExpected: true,
		Description:           "Sea freight and insurance must be added; check origin loading expenses.",
	},
	"FOB": {
		Code:                  "FOB",
		Name:                  "Free On Board",
		FreightIncluded:       false,
		InsuranceIncluded:     false,
		OriginChargesExpected: false,
		Description:           "Main international freight and insurance must be added; goods placed on board at origin port.",
	},
	"CFR": {
		Code:                  "CFR",
		Name:                  "Cost and Freight",
		FreightIncluded:       true,
		InsuranceIncluded:     false,
		OriginChargesExpected: false,
		Description:           "Freight is already included in invoice value; insurance must be added (actual or statutory 1.125%).",
	},
	"CIF": {
		Code:                  "CIF",
		Name:                  "Cost, Insurance and Freight",
		FreightIncluded:       true,
		InsuranceIncluded:     true,
		OriginChargesExpected: false,
		Description:           "Freight and insurance are already included in invoice value; duplicate additions locked to 0.",
	},
	"CPT": {
		Code:                  "CPT",
		Name:                  "Carriage Paid To",
		FreightIncluded:       true,
		InsuranceIncluded:     false,
		OriginChargesExpected: false,
		Description:           "Carriage is included up to named destination; insurance must be added (actual or statutory 1.125%).",
	},
	"CIP": {
		Code:                  "CIP",
		Name:                  "Carriage and Insurance Paid To",
		FreightIncluded:       true,
		InsuranceIncluded:     true,
		OriginChargesExpected: false,
		Description:           "Carriage and insurance are already included in invoice value; duplicate additions locked to 0.",
	},
	"DAP": {
		Code:                  "DAP",
		Name:                  "Delivered at Place",
		FreightIncluded:       true,
		InsuranceIncluded:     true,
		OriginChargesExpected: false,
		Description:           "Carriage to named destination included; seller bears all transit risks (insurance included).",
	},
	"DPU": {
		Code:                  "DPU",
		Name:                  "Delivered at Place Unloaded",
		FreightIncluded:       true,
		InsuranceIncluded:     true,
		OriginChargesExpected: false,
		Description:           "Carriage and unloading at destination included; seller bears transit risks (insurance included).",
	},
	"DDP": {
		Code:                  "DDP",
		Name:                  "Delivered Duty Paid",
		FreightIncluded:       true,
		InsuranceIncluded:     true,
		OriginChargesExpected: false,
		Description:           "Carriage to destination included. Note: Indian import duties and taxes must be excluded from assessable value.",
	},
}

// GetIncotermRule returns statutory inclusion rules for any of the 11 Incoterms.
// Delegates to the pure finance valuation package for verified Incoterms 2020 definitions.
func GetIncotermRule(incoterm string) models.IncotermRule {
	rule := valuation.GetIncotermRule(incoterm)
	return models.IncotermRule{
		Code:                  rule.Code,
		Name:                  rule.Name,
		FreightIncluded:       rule.FreightIncluded,
		InsuranceIncluded:     rule.InsuranceIncluded,
		OriginChargesExpected: rule.OriginChargesExpected,
		Description:           rule.Description,
	}
}

// CalculateDuties computes exact per-item customs valuation & Standard/CEPA/RoDTEP duties,
// enforcing statutory 11-Incoterm rules, Rule 10(2) insurance, Rule 10(1) origin charges,
// RoDTEP electronic scrip debiting, and ICEGATE line-level single digit / whole-integer totals.
func CalculateDuties(params models.BOECalcParams) (models.BOECalculationResult, error) {
	if params.ExchangeRate < 0 || params.FreightCost < 0 || params.EXWCost < 0 ||
		params.OriginInlandFreight < 0 || params.ExportHandling < 0 ||
		params.LoadingCharges < 0 || params.PackingCharges < 0 ||
		params.OtherRule10Additions < 0 || params.ActualInsurance < 0 ||
		params.InsuranceRate < 0 || params.RoDTEPScripBalance < 0 || params.Interest < 0 {
		return models.BOECalculationResult{}, errors.New("valuation parameters, costs, scrip balance, and interest must be non-negative")
	}
	if len(params.LineItems) == 0 {
		return models.BOECalculationResult{}, errors.New("at least one line item is required for BOE calculation")
	}
	if math.Abs(params.RoundOffAdjustment) > 50 {
		return models.BOECalculationResult{}, fmt.Errorf("round-off adjustment cannot exceed ±50 INR (got %.2f)", params.RoundOffAdjustment)
	}

	rule := GetIncotermRule(params.Incoterm)

	// Origin & Rule 10 charges treatment
	// For CIF/CIP/DAP/DPU/DDP, origin freight and handling are already included in the invoice price.
	// Only distinct Rule 10 adjustments (assists, royalties, license fees) are added to CIF/CIP.
	// For EXW/FCA/FAS/FOB/CFR/CPT, origin charges not included in the transaction price are added.
	var totalOrigin float64
	if rule.Code == "CIF" || rule.Code == "CIP" || rule.Code == "DAP" || rule.Code == "DPU" || rule.Code == "DDP" {
		totalOrigin = params.OtherRule10Additions
	} else {
		totalOrigin = params.OriginInlandFreight + params.ExportHandling + params.LoadingCharges +
			params.PackingCharges + params.OtherRule10Additions
		if totalOrigin == 0 && params.EXWCost > 0 {
			totalOrigin = params.EXWCost
		}
	}

	// Freight treatment
	freightCost := params.FreightCost
	if rule.FreightIncluded {
		freightCost = 0
	}

	// Determine Insurance Mode
	insMode := strings.ToLower(strings.TrimSpace(params.InsuranceMode))
	if rule.InsuranceIncluded {
		insMode = "included"
	} else if insMode == "" {
		if params.ActualInsurance > 0 {
			insMode = "actual"
		} else if params.InsuranceRate > 0 {
			insMode = "rate"
		} else {
			insMode = "deemed_1.125"
		}
	}

	totalInvoiceValue := 0.0
	for _, it := range params.LineItems {
		if it.Quantity < 0 || it.UnitPrice < 0 {
			return models.BOECalculationResult{}, fmt.Errorf("item %s has negative quantity or unitPrice", it.PartNumber)
		}
		totalInvoiceValue += it.Quantity * it.UnitPrice
	}

	res := models.BOECalculationResult{
		Items:                make([]models.BOECalcLineResult, 0, len(params.LineItems)),
		TotalInvoiceValue:   roundTo(totalInvoiceValue, 2),
		Interest:             roundTo(params.Interest, 0),
		CIFZeroLocked:        rule.FreightIncluded && rule.InsuranceIncluded,
		IncotermRule:         rule,
		TotalOriginCharges:   roundTo(totalOrigin, 2),
	}

	type tempLineData struct {
		item          models.BOECalculationItemInput
		method        string
		itemForex     float64
		itemInr       float64
		itemFreight   float64
		itemOrigin    float64
		itemInsurance float64
		assessable    float64
	}

	tempLines := make([]tempLineData, 0, len(params.LineItems))

	for _, it := range params.LineItems {
		method, err := NormalizeMethod(it.CalculationMethod)
		if err != nil {
			return models.BOECalculationResult{}, err
		}

		itemForex := roundTo(it.Quantity*it.UnitPrice, 2)
		itemInr := roundTo(itemForex*params.ExchangeRate, 2)

		var itemFreight, itemOrigin float64
		if totalInvoiceValue > 0 {
			itemFreight = roundTo((freightCost/totalInvoiceValue)*itemForex, 2)
			itemOrigin = roundTo((totalOrigin/totalInvoiceValue)*itemForex, 2)
		}

		// Statutory Insurance computation under Rule 10(2)
		var itemInsurance float64
		switch insMode {
		case "included":
			itemInsurance = 0
		case "actual":
			if totalInvoiceValue > 0 {
				itemInsurance = roundTo((params.ActualInsurance/totalInvoiceValue)*itemForex, 1)
			}
		case "open_cover", "0.015":
			rate := params.InsuranceRate
			if rate <= 0 {
				rate = DefaultOpenCoverRate
			}
			itemInsurance = roundTo((itemInr+itemOrigin)*(rate/100.0), 1)
		case "rate":
			// Percentage rate provided (e.g. open cover 0.015%, customized user-added rate, etc.)
			rate := params.InsuranceRate
			if rate <= 0 {
				rate = DefaultOpenCoverRate
			}
			itemInsurance = roundTo((itemInr+itemOrigin)*(rate/100.0), 1)
		case "deemed_1.125":
			fallthrough
		default:
			// Statutory Deemed Insurance: 1.125% of FOB Value (Item INR + Origin Charges)
			itemFOB := itemInr + itemOrigin
			itemInsurance = roundTo(itemFOB*(StatutoryDeemedInsRate/100.0), 1)
		}

		// Assessable Value Formula: AV = Converted INR + Freight + Insurance + Origin/Rule 10 Charges
		assessable := roundTo(itemInr+itemFreight+itemOrigin+itemInsurance, 1)

		res.TotalValueINR = roundTo(res.TotalValueINR+itemInr, 2)
		res.TotalFreight = roundTo(res.TotalFreight+itemFreight, 2)
		res.TotalEXW = roundTo(res.TotalEXW+itemOrigin, 2)
		res.TotalInsurance = roundTo(res.TotalInsurance+itemInsurance, 1)
		res.TotalAssessableValue = roundTo(res.TotalAssessableValue+assessable, 1)

		tempLines = append(tempLines, tempLineData{
			item:          it,
			method:        method,
			itemForex:     itemForex,
			itemInr:       itemInr,
			itemFreight:   itemFreight,
			itemOrigin:    itemOrigin,
			itemInsurance: itemInsurance,
			assessable:    assessable,
		})
	}

	// Delegate statutory duty calculation and RoDTEP scrip apportionment to pkg/finance/customs/duty.
	dutyInputs := make([]duty.DutyItemInput, 0, len(tempLines))
	for _, tmp := range tempLines {
		dutyInputs = append(dutyInputs, duty.DutyItemInput{
			ItemID:            tmp.item.ItemID,
			PartNumber:        tmp.item.PartNumber,
			Description:       tmp.item.Description,
			HSNCode:           tmp.item.HSNCode,
			Unit:              tmp.item.Unit,
			Quantity:          tmp.item.Quantity,
			UnitPrice:         tmp.item.UnitPrice,
			AssessableValue:   tmp.assessable,
			CalculationMethod: tmp.method,
			ActualBCDRate:     tmp.item.ActualBCDRate,
			ActualSWSRate:     tmp.item.ActualSWSRate,
			ActualIGSTRate:    tmp.item.ActualIGSTRate,
			BOEBCDRate:        tmp.item.BOEBCDRate,
			BOESWSRate:        tmp.item.BOESWSRate,
			BOEIGSTRate:       tmp.item.BOEIGSTRate,
		})
	}

	dutyRes, err := duty.ComputeDuties(duty.DutyCalcParams{
		LineItems:          dutyInputs,
		RoDTEPScripBalance: params.RoDTEPScripBalance,
		Interest:           params.Interest,
		RoundOffAdjustment: params.RoundOffAdjustment,
	})
	if err != nil {
		return models.BOECalculationResult{}, err
	}
	res.HasBlockingBCDOverride = dutyRes.HasBlockingBCDOverride

	for i, tmp := range tempLines {
		dl := dutyRes.Items[i]
		res.Items = append(res.Items, models.BOECalcLineResult{
			ItemID:            tmp.item.ItemID,
			PartNumber:        tmp.item.PartNumber,
			Description:       tmp.item.Description,
			HSNCode:           tmp.item.HSNCode,
			Unit:              tmp.item.Unit,
			Quantity:          tmp.item.Quantity,
			UnitPrice:         tmp.item.UnitPrice,
			ItemForexValue:    tmp.itemForex,
			ItemValueINR:      tmp.itemInr,
			ItemFreight:       tmp.itemFreight,
			ItemEXW:           tmp.itemOrigin,
			ItemInsurance:     tmp.itemInsurance,
			ItemOriginCharges: tmp.itemOrigin,
			AssessableValue:   tmp.assessable,
			CalculationMethod: dl.CalculationMethod,
			ActualBCDRate:     tmp.item.ActualBCDRate,
			ActualSWSRate:     tmp.item.ActualSWSRate,
			ActualIGSTRate:    tmp.item.ActualIGSTRate,
			BOEBCDRate:        tmp.item.BOEBCDRate,
			BOESWSRate:        tmp.item.BOESWSRate,
			BOEIGSTRate:       tmp.item.BOEIGSTRate,
			GrossBCD:          dl.GrossBCD,
			RoDTEPUtilized:    dl.RoDTEPUtilized,
			BCDCashPayable:    dl.BCDCashPayable,
			BCDValue:          dl.BCDCashPayable,
			SWSValue:          dl.SWSValue,
			IGSTValue:         dl.IGSTValue,
			TotalLineDuty:     dl.TotalLineDuty,
			TotalLineCashDuty: dl.TotalLineCashDuty,
			BCDDiscrepancy:    dl.BCDDiscrepancy,
		})
	}

	// Header Totals from pkg/finance/customs/duty
	res.RoundOffAdjustment = dutyRes.RoundOffAdjustment
	res.GrossBCDTotal = dutyRes.GrossBCDTotal
	res.RoDTEPScripUtilized = dutyRes.RoDTEPScripUtilized
	res.BCDCashTotal = dutyRes.BCDCashTotal
	res.BCDTotal = dutyRes.BCDTotal
	res.SWSTotal = dutyRes.SWSTotal
	res.IGSTTotal = dutyRes.IGSTTotal
	res.Interest = dutyRes.Interest
	res.CustomsDutyTotal = dutyRes.CustomsDutyTotal
	res.CashCustomsDutyTotal = dutyRes.CashCustomsDutyTotal

	if blob, err := json.Marshal(res); err == nil {
		res.PayloadBytes = len(blob)
		res.PayloadWarning = len(blob) >= WarnBOEPayloadBytes
	}
	return res, nil
}
