package models

// CBICExchangeRate mirrors a record in cbic_exchange_rates.
type CBICExchangeRate struct {
	ID                 int64   `json:"id"`
	CurrencyCode       string  `json:"currencyCode"`
	ExchangeRate       float64 `json:"exchangeRate"`
	EffectiveFrom      string  `json:"effectiveFrom"`
	EffectiveTo        *string `json:"effectiveTo,omitempty"`
	NotificationNumber string  `json:"notificationNumber"`
	SourceDocument     string  `json:"sourceDocument,omitempty"`
	VerificationStatus string  `json:"verificationStatus"`
	Notes              string  `json:"notes,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	CreatedBy          string  `json:"createdBy"`
}

// CBICExchangeRateInput carries fields for creating or updating a statutory rate entry.
type CBICExchangeRateInput struct {
	CurrencyCode       string  `json:"currencyCode"`
	ExchangeRate       float64 `json:"exchangeRate"`
	EffectiveFrom      string  `json:"effectiveFrom"`
	EffectiveTo        *string `json:"effectiveTo,omitempty"`
	NotificationNumber string  `json:"notificationNumber"`
	SourceDocument     string  `json:"sourceDocument,omitempty"`
	VerificationStatus string  `json:"verificationStatus,omitempty"`
	Notes              string  `json:"notes,omitempty"`
}

// CBICRateQuery filters applicable statutory rates for a given currency and date.
type CBICRateQuery struct {
	CurrencyCode string `json:"currencyCode"`
	Date         string `json:"date"`
}
