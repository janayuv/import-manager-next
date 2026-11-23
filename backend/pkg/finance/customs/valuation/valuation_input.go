package valuation

import (
	"github.com/janayuv/import-manager-next/backend/pkg/finance/money"
)

// ValuationRuleApplied indicates the specific customs valuation basis used under Rule 10.
type ValuationRuleApplied string

const (
	ValuationRuleActual                         ValuationRuleApplied = "ACTUAL_INSURANCE"                       // Actual ascertainable policy
	ValuationRule10_2_FOBDeemed                 ValuationRuleApplied = "RULE_10_2_FOB_DEEMED"                  // 1.125% of FOB
	ValuationRule10_2_FOBPlusTransport          ValuationRuleApplied = "RULE_10_2_FOB_PLUS_TRANSPORT_DEEMED"   // 1.125% of (FOB + Transport)
	ValuationRule10_2_TransportFOBDeemed         ValuationRuleApplied = "RULE_10_2_TRANSPORT_FOB_DEEMED"       // 20% of FOB
	ValuationRule10_2_TransportFOBPlusInsurance ValuationRuleApplied = "RULE_10_2_TRANSPORT_FOB_PLUS_INSURANCE" // 20% of (FOB + Insurance)
	ValuationRule10_2_AirFreightCapped          ValuationRuleApplied = "RULE_10_2_AIR_FREIGHT_CAPPED"          // Air freight capped at 20% of FOB / (FOB + Insurance)
	ValuationRuleIncluded                       ValuationRuleApplied = "INCLUDED_IN_TRANSACTION_VALUE"         // CIF / CIP / DAP included
	ValuationRuleOpenCoverMarine                ValuationRuleApplied = "OPEN_COVER_MARINE_RATE"                // Contractual open cover rate (e.g. 0.015%)
)

// ValuationLineInput represents one invoice line for valuation.
type ValuationLineInput struct {
	ItemID            string  `json:"itemId"`
	PartNumber        string  `json:"partNumber"`
	Description       string  `json:"description"`
	HSNCode           string  `json:"hsnCode"`
	Unit              string  `json:"unit"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unitPrice"`         // in foreign currency
	ExFactoryPrice    float64 `json:"exFactoryPrice"`    // optional breakdown if ex-factory differs
	CalculationMethod string  `json:"calculationMethod"` // Standard | CEPA | Rodtep
}

// ValuationParams represents header valuation and shipping parameters.
type ValuationParams struct {
	Incoterm               string            `json:"incoterm"`
	ExchangeRate           float64           `json:"exchangeRate"` // CBIC notified exchange rate (INR per foreign unit)
	CBICNotificationDate   string            `json:"cbicNotificationDate"`
	IsAirTransport         bool              `json:"isAirTransport"` // triggers 20% FOB cap on air freight under Rule 10(2)

	// Freight inputs
	FreightCost            float64           `json:"freightCost"`            // INR
	FreightIncluded        bool              `json:"freightIncluded"`        // true if invoice already includes freight
	FreightMode            string            `json:"freightMode"`            // "actual" | "unascertainable" | "deemed_20" | "included"

	// Insurance inputs
	ActualInsurance        float64           `json:"actualInsurance"`        // INR
	InsuranceRate          money.DecimalRate `json:"insuranceRate"`          // optional percentage rate (e.g. 0.015%)
	InsuranceMode          string            `json:"insuranceMode"`          // "included" | "actual" | "rate" | "deemed_1.125" | "open_cover" | "unascertainable"

	// Rule 10(2) Fallbacks when FOB is unascertainable
	FOBUnascertainable            bool    `json:"fobUnascertainable"`            // triggers proviso (ii) for transport or insurance
	FOBPlusInsuranceAscertainable float64 `json:"fobPlusInsuranceAscertainable"` // sum of (FOB + Insurance) for 20% transport fallback
	FOBPlusTransportAscertainable float64 `json:"fobPlusTransportAscertainable"` // sum of (FOB + Transport) for 1.125% insurance fallback

	// Rule 10(2) Explanation 2 Transshipment Exclusions
	PostImportTransshipment float64 `json:"postImportTransshipment"` // transport/transshipment incurred after arrival at place of importation (excluded from AV)

	// Rule 10(1) Origin Charges
	OriginInlandFreight    float64           `json:"originInlandFreight"`    // INR
	ExportHandling         float64           `json:"exportHandling"`         // INR
	LoadingCharges         float64           `json:"loadingCharges"`         // INR
	PackingCharges         float64           `json:"packingCharges"`         // INR
	OtherRule10Additions   float64           `json:"otherRule10Additions"`   // INR (assists, royalties, license fees)
	EXWCost                float64           `json:"exwCost"`                // Legacy EXW field (fallback if origin breakdown omitted)

	LineItems              []ValuationLineInput `json:"lineItems"`
}

// LineValuationResult represents the statutory assessable value breakdown for a single item.
type LineValuationResult struct {
	ItemID             string               `json:"itemId"`
	PartNumber         string               `json:"partNumber"`
	Quantity           float64              `json:"quantity"`
	UnitPrice          float64              `json:"unitPrice"`
	ForexValue         float64              `json:"forexValue"`         // foreign currency (2 decimals)
	ValueINR           float64              `json:"valueInr"`           // Converted INR (2 decimals)
	AllocatedFreight   float64              `json:"allocatedFreight"`   // INR (2 decimals)
	AllocatedOrigin    float64              `json:"allocatedOrigin"`    // INR (2 decimals)
	FOBValue           float64              `json:"fobValue"`           // INR (ValueINR + AllocatedOrigin)
	AllocatedInsurance float64              `json:"allocatedInsurance"` // INR (1 decimal per ICEGATE)
	AssessableValue    float64              `json:"assessableValue"`    // INR (1 decimal per ICEGATE)
	RuleApplied        ValuationRuleApplied `json:"ruleApplied"`
}

// ValuationResult represents the header aggregate valuation result.
type ValuationResult struct {
	Items                       []LineValuationResult `json:"items"`
	TotalInvoiceForex           float64               `json:"totalInvoiceForex"`
	TotalValueINR               float64               `json:"totalValueInr"`
	TotalFreightINR             float64               `json:"totalFreightInr"`
	TotalOriginChargesINR       float64               `json:"totalOriginChargesInr"`
	TotalFOBValueINR            float64               `json:"totalFobValueInr"`
	TotalInsuranceINR           float64               `json:"totalInsuranceInr"`
	TotalAssessableValue        float64               `json:"totalAssessableValue"` // ICEGATE header total
	ValuationRuleApplied        ValuationRuleApplied  `json:"valuationRuleApplied"`
	AirFreightCapped            bool                  `json:"airFreightCapped"`
	ExcludedTransshipmentINR    float64               `json:"excludedTransshipmentInr"` // Rule 10(2) Explanation 2 excluded transshipment charges
	DuplicateAdditionsPrevented bool                  `json:"duplicateAdditionsPrevented"`
	IncotermRule                IncotermRule          `json:"incotermRule"`
}
