package money

import (
	"fmt"
	"strings"
)

// Currency represents an ISO 4217 currency with its minor-unit precision.
type Currency struct {
	Code              string `json:"code"`
	MinorUnitDecimals int    `json:"minorUnitDecimals"` // e.g. 2 for INR/USD, 0 for JPY, 3 for BHD
	ScaleFactor       int64  `json:"scaleFactor"`       // 10^MinorUnitDecimals (e.g. 100 for INR, 1 for JPY)
	Symbol            string `json:"symbol"`
}

var supportedCurrencies = map[string]Currency{
	"INR": {Code: "INR", MinorUnitDecimals: 2, ScaleFactor: 100, Symbol: "₹"},
	"USD": {Code: "USD", MinorUnitDecimals: 2, ScaleFactor: 100, Symbol: "$"},
	"EUR": {Code: "EUR", MinorUnitDecimals: 2, ScaleFactor: 100, Symbol: "€"},
	"GBP": {Code: "GBP", MinorUnitDecimals: 2, ScaleFactor: 100, Symbol: "£"},
	"JPY": {Code: "JPY", MinorUnitDecimals: 0, ScaleFactor: 1, Symbol: "¥"},
	"KRW": {Code: "KRW", MinorUnitDecimals: 0, ScaleFactor: 1, Symbol: "₩"},
	"CNY": {Code: "CNY", MinorUnitDecimals: 2, ScaleFactor: 100, Symbol: "¥"},
	"AED": {Code: "AED", MinorUnitDecimals: 2, ScaleFactor: 100, Symbol: "AED"},
	"SGD": {Code: "SGD", MinorUnitDecimals: 2, ScaleFactor: 100, Symbol: "S$"},
	"BHD": {Code: "BHD", MinorUnitDecimals: 3, ScaleFactor: 1000, Symbol: "BD"},
	"KWD": {Code: "KWD", MinorUnitDecimals: 3, ScaleFactor: 1000, Symbol: "KD"},
}

// GetCurrency returns currency definition, defaulting to 2 decimals if unrecognized.
func GetCurrency(code string) Currency {
	clean := strings.ToUpper(strings.TrimSpace(code))
	if c, ok := supportedCurrencies[clean]; ok {
		return c
	}
	return Currency{Code: clean, MinorUnitDecimals: 2, ScaleFactor: 100, Symbol: clean}
}

// Amount represents an exact monetary quantity in integer minor units (e.g., paise, cents).
type Amount struct {
	MinorUnits int64    `json:"minorUnits"`
	Currency   Currency `json:"currency"`
}

// NewINR constructs an Amount in Indian Rupees with integer paise.
func NewINR(paise int64) Amount {
	return Amount{
		MinorUnits: paise,
		Currency:   GetCurrency("INR"),
	}
}

// NewAmount constructs an Amount for any supported currency.
func NewAmount(minorUnits int64, currencyCode string) Amount {
	return Amount{
		MinorUnits: minorUnits,
		Currency:   GetCurrency(currencyCode),
	}
}

// MajorUnitsFloat returns the amount as a floating-point number for display only.
func (a Amount) MajorUnitsFloat() float64 {
	if a.Currency.ScaleFactor == 0 {
		return float64(a.MinorUnits)
	}
	return float64(a.MinorUnits) / float64(a.Currency.ScaleFactor)
}

// String formats the amount for human-readable display.
func (a Amount) String() string {
	format := fmt.Sprintf("%%s %%.%df", a.Currency.MinorUnitDecimals)
	return fmt.Sprintf(format, a.Currency.Code, a.MajorUnitsFloat())
}
