package valuation

import (
	"strings"
)

// IncotermRule captures statutory cost inclusion treatment under Incoterms 2020.
type IncotermRule struct {
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	FreightIncluded       bool   `json:"freightIncluded"`
	InsuranceIncluded     bool   `json:"insuranceIncluded"`
	OriginChargesExpected bool   `json:"originChargesExpected"`
	Description           string `json:"description"`
}

// IncotermRules maps the 11 canonical Incoterms (2020).
var IncotermRules = map[string]IncotermRule{
	"EXW": {
		Code:                  "EXW",
		Name:                  "Ex Works",
		FreightIncluded:       false,
		InsuranceIncluded:     false,
		OriginChargesExpected: true,
		Description:           "Origin inland transport, export handling, international freight and insurance must be added.",
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
// Defaults to FOB if unspecified or unrecognized.
func GetIncotermRule(incoterm string) IncotermRule {
	code := strings.ToUpper(strings.TrimSpace(incoterm))
	if rule, exists := IncotermRules[code]; exists {
		return rule
	}
	return IncotermRule{
		Code:                  code,
		Name:                  code,
		FreightIncluded:       false,
		InsuranceIncluded:     false,
		OriginChargesExpected: false,
		Description:           "Standard Incoterm valuation rules apply.",
	}
}
