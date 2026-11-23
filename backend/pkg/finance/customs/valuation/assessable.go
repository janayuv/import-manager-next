package valuation

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/janayuv/import-manager-next/backend/pkg/finance/money"
)

const (
	StatutoryDeemedInsuranceRate = 1.125 // 1.125% under Rule 10(2)(b) Customs Valuation Rules 2007
	DefaultOpenCoverMarineRate   = 0.015 // 0.015% open cover marine policy
	AirFreightCapPercentage      = 20.0  // Maximum 20% of FOB value for air transport under Rule 10(2)(a)
	StatutoryDeemedFreightRate   = 20.0  // 20% fallback under Rule 10(2)(a) provisos (i) & (ii)
)

// roundTo rounds n using symmetric half-up semantics to specified decimal precision.
func roundTo(n float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round((n+math.Copysign(1e-9, n))*factor) / factor
}

// ComputeAssessableValue evaluates statutory customs valuation and per-item assessable values
// strictly under the amended Customs Valuation (Determination of Value of Imported Goods) Rules, 2007 (Rule 10).
func ComputeAssessableValue(params ValuationParams) (ValuationResult, error) {
	if params.ExchangeRate < 0 || params.FreightCost < 0 || params.EXWCost < 0 ||
		params.OriginInlandFreight < 0 || params.ExportHandling < 0 ||
		params.LoadingCharges < 0 || params.PackingCharges < 0 ||
		params.OtherRule10Additions < 0 || params.ActualInsurance < 0 ||
		params.PostImportTransshipment < 0 || params.FOBPlusInsuranceAscertainable < 0 ||
		params.FOBPlusTransportAscertainable < 0 {
		return ValuationResult{}, errors.New("valuation costs, freight, insurance, and transshipment must be non-negative")
	}
	if len(params.LineItems) == 0 {
		return ValuationResult{}, errors.New("at least one line item is required for customs valuation")
	}

	rule := GetIncotermRule(params.Incoterm)

	// 1. Origin & Rule 10(1) charges
	// For CIF/CIP/DAP/DPU/DDP, origin freight and handling are already in the invoice price.
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

	// 2. Foreign invoice total
	totalInvoiceForex := 0.0
	for _, it := range params.LineItems {
		if it.Quantity < 0 || it.UnitPrice < 0 {
			return ValuationResult{}, fmt.Errorf("item %s has negative quantity or unit price", it.PartNumber)
		}
		totalInvoiceForex += it.Quantity * it.UnitPrice
	}
	totalInvoiceForex = roundTo(totalInvoiceForex, 2)

	// 3. Converted INR value & FOB value
	totalValueINR := roundTo(totalInvoiceForex*params.ExchangeRate, 2)
	totalFOB := roundTo(totalValueINR+totalOrigin, 2)
	if params.FOBUnascertainable {
		totalFOB = 0
	}

	// 4. Freight determination under Rule 10(2)(a)
	freightMode := strings.ToLower(strings.TrimSpace(params.FreightMode))
	var freightCost float64
	var freightRule ValuationRuleApplied
	duplicateFreightPrevented := false

	if rule.FreightIncluded || params.FreightIncluded || freightMode == "included" {
		freightCost = 0
		freightRule = ValuationRuleIncluded
		duplicateFreightPrevented = true
	} else if freightMode == "unascertainable" || freightMode == "deemed_20" || (params.FreightCost == 0 && freightMode != "actual") {
		// Fallbacks under Rule 10(2)(a)
		if params.FOBUnascertainable && params.FOBPlusInsuranceAscertainable > 0 {
			// Rule 10(2)(a) 2nd Proviso: FOB unascertainable, FOB + Insurance ascertainable -> 20% of (FOB + Insurance)
			freightCost = roundTo(params.FOBPlusInsuranceAscertainable*(StatutoryDeemedFreightRate/100.0), 2)
			freightRule = ValuationRule10_2_TransportFOBPlusInsurance
		} else if totalFOB > 0 {
			// Rule 10(2)(a) 1st Proviso: Transport unascertainable -> 20% of FOB
			freightCost = roundTo(totalFOB*(StatutoryDeemedFreightRate/100.0), 2)
			freightRule = ValuationRule10_2_TransportFOBDeemed
		} else {
			freightCost = 0
			freightRule = ValuationRuleActual
		}
	} else {
		// Actual ascertainable freight
		freightCost = params.FreightCost
		freightRule = ValuationRuleActual
	}

	// Air freight cap under Rule 10(2)(a) 3rd and 4th provisos:
	// Air freight shall not exceed 20% of FOB (or 20% of FOB + Insurance if FOB unascertainable)
	airFreightCapped := false
	if params.IsAirTransport && freightCost > 0 {
		var maxAirFreight float64
		if params.FOBUnascertainable && params.FOBPlusInsuranceAscertainable > 0 {
			maxAirFreight = roundTo(params.FOBPlusInsuranceAscertainable*(AirFreightCapPercentage/100.0), 2)
		} else if totalFOB > 0 {
			maxAirFreight = roundTo(totalFOB*(AirFreightCapPercentage/100.0), 2)
		}
		if maxAirFreight > 0 && freightCost > maxAirFreight {
			freightCost = maxAirFreight
			airFreightCapped = true
			freightRule = ValuationRule10_2_AirFreightCapped
		}
	}

	// 5. Insurance determination under Rule 10(2)(b)
	insMode := strings.ToLower(strings.TrimSpace(params.InsuranceMode))
	var insuranceCost float64
	var insuranceRule ValuationRuleApplied
	duplicateInsurancePrevented := false

	if rule.InsuranceIncluded || insMode == "included" {
		insuranceCost = 0
		insuranceRule = ValuationRuleIncluded
		duplicateInsurancePrevented = true
	} else if insMode == "actual" || (insMode == "" && params.ActualInsurance > 0) {
		insuranceCost = roundTo(params.ActualInsurance, 1)
		insuranceRule = ValuationRuleActual
	} else if insMode == "open_cover" || insMode == "rate" {
		rate := DefaultOpenCoverMarineRate
		if !params.InsuranceRate.IsZero() {
			rate = params.InsuranceRate.PercentageFloat()
		}
		insuranceCost = roundTo(totalFOB*(rate/100.0), 1)
		insuranceRule = ValuationRuleOpenCoverMarine
	} else {
		// Rule 10(2)(b) Statutory Fallbacks
		if params.FOBUnascertainable && params.FOBPlusTransportAscertainable > 0 {
			// Rule 10(2)(b) 2nd Proviso: FOB unascertainable, FOB + Transport ascertainable -> 1.125% of (FOB + Transport)
			insuranceCost = roundTo(params.FOBPlusTransportAscertainable*(StatutoryDeemedInsuranceRate/100.0), 1)
			insuranceRule = ValuationRule10_2_FOBPlusTransport
		} else if totalFOB > 0 {
			// Rule 10(2)(b) 1st Proviso: Insurance unascertainable -> 1.125% of FOB
			insuranceCost = roundTo(totalFOB*(StatutoryDeemedInsuranceRate/100.0), 1)
			insuranceRule = ValuationRule10_2_FOBDeemed
		} else if (totalValueINR + freightCost) > 0 {
			// Fallback where FOB itself is un-ascertainable but sum of Value + Transport is ascertainable
			insuranceCost = roundTo((totalValueINR+freightCost)*(StatutoryDeemedInsuranceRate/100.0), 1)
			insuranceRule = ValuationRule10_2_FOBPlusTransport
		} else {
			insuranceCost = 0
			insuranceRule = ValuationRule10_2_FOBDeemed
		}
	}

	// Select primary header valuation rule applied
	headerValuationRule := insuranceRule
	if airFreightCapped {
		headerValuationRule = ValuationRule10_2_AirFreightCapped
	} else if freightRule == ValuationRule10_2_TransportFOBPlusInsurance {
		headerValuationRule = ValuationRule10_2_TransportFOBPlusInsurance
	} else if freightRule == ValuationRule10_2_TransportFOBDeemed {
		headerValuationRule = ValuationRule10_2_TransportFOBDeemed
	} else if insuranceRule == ValuationRule10_2_FOBPlusTransport {
		headerValuationRule = ValuationRule10_2_FOBPlusTransport
	} else if insuranceRule == ValuationRule10_2_FOBDeemed {
		headerValuationRule = ValuationRule10_2_FOBDeemed
	} else if freightRule == ValuationRuleIncluded && insuranceRule == ValuationRuleIncluded {
		headerValuationRule = ValuationRuleIncluded
	}

	// 6. Transshipment Exclusions under Rule 10(2) Explanation 2
	// Post-arrival transport or transshipment charges incurred in India/after port of arrival
	// are excluded from the customs assessable value.
	excludedTransshipment := roundTo(params.PostImportTransshipment, 2)

	res := ValuationResult{
		Items:                       make([]LineValuationResult, 0, len(params.LineItems)),
		TotalInvoiceForex:           totalInvoiceForex,
		TotalFreightINR:             freightCost,
		TotalOriginChargesINR:       totalOrigin,
		TotalFOBValueINR:            totalFOB,
		TotalInsuranceINR:           insuranceCost,
		ValuationRuleApplied:        headerValuationRule,
		AirFreightCapped:            airFreightCapped,
		ExcludedTransshipmentINR:    excludedTransshipment,
		DuplicateAdditionsPrevented: duplicateFreightPrevented || duplicateInsurancePrevented,
		IncotermRule:                rule,
	}

	var sumValueINR, sumFreight, sumOrigin, sumInsurance, sumAssessable float64

	for _, it := range params.LineItems {
		itemForex := roundTo(it.Quantity*it.UnitPrice, 2)
		itemInr := roundTo(itemForex*params.ExchangeRate, 2)

		var itemFreight, itemOrigin float64
		if totalInvoiceForex > 0 {
			itemFreight = roundTo((freightCost/totalInvoiceForex)*itemForex, 2)
			itemOrigin = roundTo((totalOrigin/totalInvoiceForex)*itemForex, 2)
		}

		itemFOB := itemInr + itemOrigin

		// Statutory Insurance allocation per line
		var itemInsurance float64
		lineRule := headerValuationRule

		switch insuranceRule {
		case ValuationRuleIncluded:
			itemInsurance = 0
		case ValuationRuleActual:
			if totalInvoiceForex > 0 {
				itemInsurance = roundTo((insuranceCost/totalInvoiceForex)*itemForex, 1)
			}
		case ValuationRuleOpenCoverMarine:
			rate := DefaultOpenCoverMarineRate
			if !params.InsuranceRate.IsZero() {
				rate = params.InsuranceRate.PercentageFloat()
			}
			itemInsurance = roundTo(itemFOB*(rate/100.0), 1)
		case ValuationRule10_2_FOBPlusTransport:
			if (totalValueINR + freightCost) > 0 {
				itemInsurance = roundTo(((itemInr+itemFreight)/(totalValueINR+freightCost))*insuranceCost, 1)
			} else if totalInvoiceForex > 0 {
				itemInsurance = roundTo((insuranceCost/totalInvoiceForex)*itemForex, 1)
			}
		case ValuationRule10_2_FOBDeemed:
			fallthrough
		default:
			if itemFOB > 0 {
				itemInsurance = roundTo(itemFOB*(StatutoryDeemedInsuranceRate/100.0), 1)
			} else if totalInvoiceForex > 0 {
				itemInsurance = roundTo((insuranceCost/totalInvoiceForex)*itemForex, 1)
			}
		}

		// Assessable Value Formula: AV = Converted INR + Freight + Origin + Insurance
		// (Post-import transshipment charges are excluded per Explanation 2).
		// (No 1% landing charge added, preventing obsolete/duplicate additions).
		assessable := roundTo(itemInr+itemFreight+itemOrigin+itemInsurance, 1)

		sumValueINR = roundTo(sumValueINR+itemInr, 2)
		sumFreight = roundTo(sumFreight+itemFreight, 2)
		sumOrigin = roundTo(sumOrigin+itemOrigin, 2)
		sumInsurance = roundTo(sumInsurance+itemInsurance, 1)
		sumAssessable = roundTo(sumAssessable+assessable, 1)

		res.Items = append(res.Items, LineValuationResult{
			ItemID:             it.ItemID,
			PartNumber:         it.PartNumber,
			Quantity:           it.Quantity,
			UnitPrice:          it.UnitPrice,
			ForexValue:         itemForex,
			ValueINR:           itemInr,
			AllocatedFreight:   itemFreight,
			AllocatedOrigin:    itemOrigin,
			FOBValue:           itemFOB,
			AllocatedInsurance: itemInsurance,
			AssessableValue:    assessable,
			RuleApplied:        lineRule,
		})
	}

	res.TotalValueINR = sumValueINR
	res.TotalFreightINR = sumFreight
	res.TotalOriginChargesINR = sumOrigin
	res.TotalInsuranceINR = sumInsurance
	res.TotalAssessableValue = sumAssessable

	_ = money.PolicyCustomsOneDecimal // compiler check
	return res, nil
}
