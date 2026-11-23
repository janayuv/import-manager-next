package valuation

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/janayuv/import-manager-next/backend/pkg/finance/money"
)

const (
	StatutoryDeemedInsuranceRate = 1.125 // 1.125% under Rule 10(2) Customs Valuation Rules 2007
	DefaultOpenCoverMarineRate   = 0.015 // 0.015% open cover marine policy
	AirFreightCapPercentage      = 20.0  // Maximum 20% of FOB value for air transport
)

// roundTo rounds n using symmetric half-up semantics to specified decimal precision.
func roundTo(n float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round((n+math.Copysign(1e-9, n))*factor) / factor
}

// ComputeAssessableValue evaluates statutory customs valuation and per-item assessable values
// strictly under the Customs Valuation (Determination of Value of Imported Goods) Rules, 2007.
func ComputeAssessableValue(params ValuationParams) (ValuationResult, error) {
	if params.ExchangeRate < 0 || params.FreightCost < 0 || params.EXWCost < 0 ||
		params.OriginInlandFreight < 0 || params.ExportHandling < 0 ||
		params.LoadingCharges < 0 || params.PackingCharges < 0 ||
		params.OtherRule10Additions < 0 || params.ActualInsurance < 0 {
		return ValuationResult{}, errors.New("valuation costs, freight, and insurance must be non-negative")
	}
	if len(params.LineItems) == 0 {
		return ValuationResult{}, errors.New("at least one line item is required for customs valuation")
	}

	rule := GetIncotermRule(params.Incoterm)

	// 1. Origin & Rule 10(1) charges
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

	// 2. Freight determination
	freightCost := params.FreightCost
	if rule.FreightIncluded || params.FreightIncluded {
		freightCost = 0
	}

	// 3. Foreign invoice total
	totalInvoiceForex := 0.0
	for _, it := range params.LineItems {
		if it.Quantity < 0 || it.UnitPrice < 0 {
			return ValuationResult{}, fmt.Errorf("item %s has negative quantity or unit price", it.PartNumber)
		}
		totalInvoiceForex += it.Quantity * it.UnitPrice
	}
	totalInvoiceForex = roundTo(totalInvoiceForex, 2)

	// 4. Converted INR value & FOB value
	totalValueINR := roundTo(totalInvoiceForex*params.ExchangeRate, 2)
	totalFOB := roundTo(totalValueINR+totalOrigin, 2)

	// Air freight cap under Rule 10(2): air freight cannot exceed 20% of FOB
	airFreightCapped := false
	if params.IsAirTransport && totalFOB > 0 && freightCost > 0 {
		maxAirFreight := roundTo(totalFOB*(AirFreightCapPercentage/100.0), 2)
		if freightCost > maxAirFreight {
			freightCost = maxAirFreight
			airFreightCapped = true
		}
	}

	// 5. Insurance determination under Rule 10(2)
	insMode := strings.ToLower(strings.TrimSpace(params.InsuranceMode))
	var headerValuationRule ValuationRuleApplied

	if rule.InsuranceIncluded {
		insMode = "included"
		headerValuationRule = ValuationRuleIncluded
	} else if insMode == "" {
		if params.ActualInsurance > 0 {
			insMode = "actual"
			headerValuationRule = ValuationRuleActual
		} else if !params.InsuranceRate.IsZero() {
			insMode = "rate"
			headerValuationRule = ValuationRuleOpenCoverMarine
		} else {
			insMode = "deemed_1.125"
			headerValuationRule = ValuationRule10_2_FOBDeemed
		}
	} else {
		switch insMode {
		case "included":
			headerValuationRule = ValuationRuleIncluded
		case "actual":
			headerValuationRule = ValuationRuleActual
		case "open_cover", "rate":
			headerValuationRule = ValuationRuleOpenCoverMarine
		default:
			headerValuationRule = ValuationRule10_2_FOBDeemed
		}
	}

	res := ValuationResult{
		Items:                 make([]LineValuationResult, 0, len(params.LineItems)),
		TotalInvoiceForex:     totalInvoiceForex,
		TotalFreightINR:       freightCost,
		TotalOriginChargesINR: totalOrigin,
		TotalFOBValueINR:      totalFOB,
		ValuationRuleApplied:  headerValuationRule,
		AirFreightCapped:      airFreightCapped,
		IncotermRule:          rule,
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

		// Statutory Insurance computation under Rule 10(2)
		var itemInsurance float64
		lineRule := headerValuationRule

		switch insMode {
		case "included":
			itemInsurance = 0
		case "actual":
			if totalInvoiceForex > 0 {
				itemInsurance = roundTo((params.ActualInsurance/totalInvoiceForex)*itemForex, 1)
			}
		case "open_cover":
			rate := DefaultOpenCoverMarineRate
			if !params.InsuranceRate.IsZero() {
				rate = params.InsuranceRate.PercentageFloat()
			}
			itemInsurance = roundTo(itemFOB*(rate/100.0), 1)
		case "rate":
			rate := params.InsuranceRate.PercentageFloat()
			if rate <= 0 {
				rate = DefaultOpenCoverMarineRate
			}
			itemInsurance = roundTo(itemFOB*(rate/100.0), 1)
		case "deemed_1.125":
			fallthrough
		default:
			// Rule 10(2) Statutory Fallback: 1.125% of FOB
			if itemFOB > 0 {
				itemInsurance = roundTo(itemFOB*(StatutoryDeemedInsuranceRate/100.0), 1)
			} else if (itemInr + itemFreight) > 0 {
				// Fallback where FOB itself is un-ascertainable but FOB + Transport is ascertainable
				lineRule = ValuationRule10_2_FOBPlusTransport
				itemInsurance = roundTo((itemInr+itemFreight)*(StatutoryDeemedInsuranceRate/100.0), 1)
			}
		}

		// Assessable Value Formula: AV = Converted INR + Freight + Origin + Insurance
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
