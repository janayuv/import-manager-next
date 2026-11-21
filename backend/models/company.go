package models

import "time"

// Company represents a distinct corporate/legal entity with its own independent encrypted database.
type Company struct {
	ID                string `json:"id"`
	Name              string `json:"name"`              // Legal / Trade Name
	ShortCode         string `json:"shortCode"`         // Short uppercase code (e.g. "ACME", "APEX")
	IECCode           string `json:"iecCode"`           // 10-digit Import Export Code
	GSTIN             string `json:"gstin"`             // 15-char GSTIN
	PAN               string `json:"pan"`               // 10-char PAN
	AddressLine1      string `json:"addressLine1"`      // Street address line 1
	AddressLine2      string `json:"addressLine2"`      // Street address line 2
	City              string `json:"city"`
	State             string `json:"state"`
	PostalCode        string `json:"postalCode"`        // PIN / ZIP
	Country           string `json:"country"`           // Default: India
	Phone             string `json:"phone"`
	Email             string `json:"email"`
	Website           string `json:"website"`
	BeneficiaryName   string `json:"beneficiaryName"`   // Bank Beneficiary Name
	BankName          string `json:"bankName"`          // Bank Name
	AccountNumber     string `json:"accountNumber"`     // Bank Account Number
	IFSCCode          string `json:"ifscCode"`          // IFSC / Sort Code
	SwiftCode         string `json:"swiftCode"`         // SWIFT / BIC Code
	BankBranch        string `json:"bankBranch"`        // Bank Branch Name
	BankAddress       string `json:"bankAddress"`       // Bank Branch Address
	LogoData          string `json:"logoData"`          // Base64 data URI or image URL
	BaseCurrency      string `json:"baseCurrency"`      // Default currency, e.g. "INR", "USD"
	FinancialYearFrom string `json:"financialYearFrom"` // Default "04-01" (MM-DD)
	FinancialYearTo   string `json:"financialYearTo"`   // Default "03-31" (MM-DD)
	DatabasePath      string `json:"databasePath"`      // Path to company SQLite DB file
	IsDefault         bool   `json:"isDefault"`         // True for the primary default company
	IsActive          bool   `json:"isActive"`          // True if currently active
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

// CreateCompanyInput holds validated payload required to provision a brand-new company database.
type CreateCompanyInput struct {
	Name              string `json:"name"`
	ShortCode         string `json:"shortCode"`
	IECCode           string `json:"iecCode"`
	GSTIN             string `json:"gstin"`
	PAN               string `json:"pan"`
	AddressLine1      string `json:"addressLine1"`
	AddressLine2      string `json:"addressLine2"`
	City              string `json:"city"`
	State             string `json:"state"`
	PostalCode        string `json:"postalCode"`
	Country           string `json:"country"`
	Phone             string `json:"phone"`
	Email             string `json:"email"`
	Website           string `json:"website"`
	BeneficiaryName   string `json:"beneficiaryName"`
	BankName          string `json:"bankName"`
	AccountNumber     string `json:"accountNumber"`
	IFSCCode          string `json:"ifscCode"`
	SwiftCode         string `json:"swiftCode"`
	BankBranch        string `json:"bankBranch"`
	BankAddress       string `json:"bankAddress"`
	LogoData          string `json:"logoData"`
	BaseCurrency      string `json:"baseCurrency"`
	FinancialYearFrom string `json:"financialYearFrom"`
	FinancialYearTo   string `json:"financialYearTo"`
	SwitchImmediately bool   `json:"switchImmediately"` // If true, switches active company immediately
}

// UpdateCompanyInput specifies mutable profile fields for an existing company entity.
type UpdateCompanyInput struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ShortCode         string `json:"shortCode"`
	IECCode           string `json:"iecCode"`
	GSTIN             string `json:"gstin"`
	PAN               string `json:"pan"`
	AddressLine1      string `json:"addressLine1"`
	AddressLine2      string `json:"addressLine2"`
	City              string `json:"city"`
	State             string `json:"state"`
	PostalCode        string `json:"postalCode"`
	Country           string `json:"country"`
	Phone             string `json:"phone"`
	Email             string `json:"email"`
	Website           string `json:"website"`
	BeneficiaryName   string `json:"beneficiaryName"`
	BankName          string `json:"bankName"`
	AccountNumber     string `json:"accountNumber"`
	IFSCCode          string `json:"ifscCode"`
	SwiftCode         string `json:"swiftCode"`
	BankBranch        string `json:"bankBranch"`
	BankAddress       string `json:"bankAddress"`
	LogoData          string `json:"logoData"`
	BaseCurrency      string `json:"baseCurrency"`
	FinancialYearFrom string `json:"financialYearFrom"`
	FinancialYearTo   string `json:"financialYearTo"`
}

// CompanyRegistryItem stores metadata in central companies_registry.json.
type CompanyRegistryItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ShortCode    string    `json:"shortCode"`
	DatabasePath string    `json:"databasePath"`
	IsDefault    bool      `json:"isDefault"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// CompanyRegistryFile represents the persisted central companies_registry.json document.
type CompanyRegistryFile struct {
	ActiveCompanyID string                `json:"activeCompanyId"`
	Companies       []CompanyRegistryItem `json:"companies"`
}
