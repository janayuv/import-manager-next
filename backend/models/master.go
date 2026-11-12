package models

// LookupOption represents a key/label pair stored in any of the 13 domain lookup tables.
type LookupOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Supplier mirrors a row in suppliers plus soft-delete metadata.
type Supplier struct {
	ID              string `json:"id"`
	SupplierName    string `json:"supplierName"`
	ShortName       string `json:"shortName"`
	Country         string `json:"country"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	BeneficiaryName string `json:"beneficiaryName"`
	BankName        string `json:"bankName"`
	Branch          string `json:"branch"`
	BankAddress     string `json:"bankAddress"`
	AccountNo       string `json:"accountNo"`
	IBAN            string `json:"iban"`
	SwiftCode       string `json:"swiftCode"`
	IsActive        bool   `json:"isActive"`
}

// SupplierInput carries fields for creating or updating a supplier.
type SupplierInput struct {
	ID              string `json:"id,omitempty"`
	SupplierName    string `json:"supplierName"`
	ShortName       string `json:"shortName"`
	Country         string `json:"country"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	BeneficiaryName string `json:"beneficiaryName"`
	BankName        string `json:"bankName"`
	Branch          string `json:"branch"`
	BankAddress     string `json:"bankAddress"`
	AccountNo       string `json:"accountNo"`
	IBAN            string `json:"iban"`
	SwiftCode       string `json:"swiftCode"`
	IsActive        bool   `json:"isActive"`
}

// SupplierPageRequest filters, sorts, and paginates suppliers.
type SupplierPageRequest struct {
	Search   string `json:"search"`
	Status   string `json:"status"` // "", "active", "inactive"
	SortBy   string `json:"sortBy"`
	SortDir  string `json:"sortDir"` // "asc", "desc"
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// SupplierPage contains paginated supplier records and totals.
type SupplierPage struct {
	Items      []Supplier `json:"items"`
	TotalCount int        `json:"totalCount"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}

// SupplierHistoryItem summarizes linked shipment/invoice records for a supplier.
type SupplierHistoryItem struct {
	ShipmentID    string  `json:"shipmentId"`
	InvoiceNumber string  `json:"invoiceNumber"`
	InvoiceDate   string  `json:"invoiceDate"`
	Currency      string  `json:"currency"`
	InvoiceValue  float64 `json:"invoiceValue"`
	Status        string  `json:"status"`
}

// SupplierHistorySummary reports linked counts, totals, and recent shipments/invoices for a supplier.
type SupplierHistorySummary struct {
	SupplierID     string                `json:"supplierId"`
	ShipmentCount  int                   `json:"shipmentCount"`
	InvoiceCount   int                   `json:"invoiceCount"`
	TotalValue     float64               `json:"totalValue"`
	RecentRecords  []SupplierHistoryItem `json:"recentRecords"`
}

// Item mirrors a 20-column record in items.
type Item struct {
	ID                  string  `json:"id"`
	PartNumber          string  `json:"partNumber"`
	ItemDescription     string  `json:"itemDescription"`
	Unit                string  `json:"unit"`
	Currency            string  `json:"currency"`
	UnitPrice           float64 `json:"unitPrice"`
	HSNCode             string  `json:"hsnCode"`
	SupplierID          string  `json:"supplierId"`
	SupplierName        string  `json:"supplierName,omitempty"`
	IsActive            bool    `json:"isActive"`
	CountryOfOrigin     string  `json:"countryOfOrigin"`
	BCD                 string  `json:"bcd"`
	SWS                 string  `json:"sws"`
	IGST                string  `json:"igst"`
	TechnicalWriteUp    string  `json:"technicalWriteUp"`
	Category            string  `json:"category"`
	EndUse              string  `json:"endUse"`
	NetWeightKg         float64 `json:"netWeightKg"`
	PurchaseUOM         string  `json:"purchaseUom"`
	GrossWeightPerUOMKg float64 `json:"grossWeightPerUomKg"`
	PhotoPath           string  `json:"photoPath"`
}

// ItemInput carries fields for creating or updating an item.
type ItemInput struct {
	ID                  string  `json:"id,omitempty"`
	PartNumber          string  `json:"partNumber"`
	ItemDescription     string  `json:"itemDescription"`
	Unit                string  `json:"unit"`
	Currency            string  `json:"currency"`
	UnitPrice           float64 `json:"unitPrice"`
	HSNCode             string  `json:"hsnCode"`
	SupplierID          string  `json:"supplierId"`
	SupplierName        string  `json:"supplierName,omitempty"`
	IsActive            bool    `json:"isActive"`
	CountryOfOrigin     string  `json:"countryOfOrigin"`
	BCD                 string  `json:"bcd"`
	SWS                 string  `json:"sws"`
	IGST                string  `json:"igst"`
	TechnicalWriteUp    string  `json:"technicalWriteUp"`
	Category            string  `json:"category"`
	EndUse              string  `json:"endUse"`
	NetWeightKg         float64 `json:"netWeightKg"`
	PurchaseUOM         string  `json:"purchaseUom"`
	GrossWeightPerUOMKg float64 `json:"grossWeightPerUomKg"`
	PhotoPath           string  `json:"photoPath"`
}

// ItemPageRequest filters, sorts, and paginates item master records.
type ItemPageRequest struct {
	Search     string `json:"search"`
	SupplierID string `json:"supplierId"`
	Status     string `json:"status"` // "", "active", "inactive"
	SortBy     string `json:"sortBy"`
	SortDir    string `json:"sortDir"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// ItemPage returns paginated items plus total count.
type ItemPage struct {
	Items      []Item `json:"items"`
	TotalCount int    `json:"totalCount"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// CSVImportRowError records a skipped or warned CSV row during bulk import.
type CSVImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// CSVImportResult summarizes outcomes across supplier, item, shipment, or expense imports.
type CSVImportResult struct {
	TotalRows    int                 `json:"totalRows"`
	ImportedRows int                 `json:"importedRows"`
	SkippedRows  int                 `json:"skippedRows"`
	Errors       []CSVImportRowError `json:"errors"`
	Warnings     []CSVImportRowError `json:"warnings"`
}
