package models

// PartPeriodAnalysisRequest defines the search query, period filters, and date basis.
type PartPeriodAnalysisRequest struct {
	Search     string `json:"search"`     // Free-text search across part number, description, HSN
	PartNumber string `json:"partNumber"` // Specific part number if selected
	StartDate  string `json:"startDate"`  // Format: YYYY-MM-DD
	EndDate    string `json:"endDate"`    // Format: YYYY-MM-DD
	DateBasis  string `json:"dateBasis"`  // "invoice_date" (default), "be_date", "shipment_date"
	SupplierID string `json:"supplierId"` // Optional supplier filter
}

// PartLookupItem provides autocomplete / search match metadata with import counts.
type PartLookupItem struct {
	ID            string  `json:"id"`
	PartNumber    string  `json:"partNumber"`
	Description   string  `json:"description"`
	Unit          string  `json:"unit"`
	Currency      string  `json:"currency"`
	UnitPrice     float64 `json:"unitPrice"`
	HSNCode       string  `json:"hsnCode"`
	SupplierID    string  `json:"supplierId"`
	SupplierName  string  `json:"supplierName"`
	Category      string  `json:"category"`
	ShipmentCount int     `json:"shipmentCount"`
	TotalQuantity float64 `json:"totalQuantity"`
}

// PartItemMasterInfo encapsulates the Item Master technical & duty profile.
type PartItemMasterInfo struct {
	ID                  string  `json:"id"`
	PartNumber          string  `json:"partNumber"`
	ItemDescription     string  `json:"itemDescription"`
	Unit                string  `json:"unit"`
	Currency            string  `json:"currency"`
	UnitPrice           float64 `json:"unitPrice"`
	HSNCode             string  `json:"hsnCode"`
	SupplierID          string  `json:"supplierId"`
	SupplierName        string  `json:"supplierName"`
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

// PartPeriodSummary aggregates volume, forex, duties, expenses, and landed cost for the period.
type PartPeriodSummary struct {
	TotalShipments                  int     `json:"totalShipments"`
	TotalInvoices                   int     `json:"totalInvoices"`
	TotalBOEs                       int     `json:"totalBoes"`
	TotalQuantity                   float64 `json:"totalQuantity"`
	Unit                            string  `json:"unit"`
	TotalForexValue                 float64 `json:"totalForexValue"`
	Currency                        string  `json:"currency"`
	TotalMaterialValueINR           float64 `json:"totalMaterialValueInr"`
	TotalAssessableValueINR         float64 `json:"totalAssessableValueInr"`
	TotalBCDAmount                  float64 `json:"totalBcdAmount"`
	TotalSWSAmount                  float64 `json:"totalSwsAmount"`
	TotalIGSTAmount                 float64 `json:"totalIgstAmount"`
	TotalBOEInterest                float64 `json:"totalBoeInterest"`
	TotalCustomsDutyINR             float64 `json:"totalCustomsDutyInr"`
	TotalDutyPaidINR                float64 `json:"totalDutyPaidInr"`
	TotalAllocatedExpensesINR       float64 `json:"totalAllocatedExpensesInr"`
	TotalAllocatedExpensesWithGSTINR float64 `json:"totalAllocatedExpensesWithGstInr"`
	TotalLandedCostExIGST           float64 `json:"totalLandedCostExIgst"`
	AvgLandedCostPerUnit            float64 `json:"avgLandedCostPerUnit"`
	MinLandedCostPerUnit            float64 `json:"minLandedCostPerUnit"`
	MaxLandedCostPerUnit            float64 `json:"maxLandedCostPerUnit"`
	FirstImportDate                 string  `json:"firstImportDate"`
	LatestImportDate                string  `json:"latestImportDate"`
}

// PartShipmentRow details an individual shipment and commercial invoice line item for this part.
type PartShipmentRow struct {
	ShipmentID      string  `json:"shipmentId"`
	InvoiceID       string  `json:"invoiceId"`
	InvoiceNumber   string  `json:"invoiceNumber"`
	InvoiceDate     string  `json:"invoiceDate"`
	SupplierID      string  `json:"supplierId"`
	SupplierName    string  `json:"supplierName"`
	Incoterm        string  `json:"incoterm"`
	ShipmentMode    string  `json:"shipmentMode"`
	ShipmentType    string  `json:"shipmentType"`
	BLAWBNumber     string  `json:"blAwbNumber"`
	BLAWBDate       string  `json:"blAwbDate"`
	VesselName      string  `json:"vesselName"`
	ContainerNumber string  `json:"containerNumber"`
	GrossWeightKg   float64 `json:"grossWeightKg"`
	ETD             string  `json:"etd"`
	ETA             string  `json:"eta"`
	Status          string  `json:"status"`
	DateOfDelivery  string  `json:"dateOfDelivery"`
	IsFrozen        bool    `json:"isFrozen"`
	Quantity        float64 `json:"quantity"`
	Unit            string  `json:"unit"`
	UnitPrice       float64 `json:"unitPrice"`
	Currency        string  `json:"currency"`
	LineTotal       float64 `json:"lineTotal"`
	MasterUnitPrice float64 `json:"masterUnitPrice"`
	PriceDiverged   bool    `json:"priceDiverged"`
}

// PartBOERow details an individual Bill of Entry duty computation line for this part.
type PartBOERow struct {
	ShipmentID      string  `json:"shipmentId"`
	InvoiceNumber   string  `json:"invoiceNumber"`
	BOECalcID       string  `json:"boeCalcId"`
	BOEID           string  `json:"boeId"`
	BENumber        string  `json:"beNumber"`
	BEDate          string  `json:"beDate"`
	Location        string  `json:"location"`
	Status          string  `json:"status"`
	ExchangeRate    float64 `json:"exchangeRate"`
	Quantity        float64 `json:"quantity"`
	Unit            string  `json:"unit"`
	ItemForexValue  float64 `json:"itemForexValue"`
	ItemValueINR    float64 `json:"itemValueInr"`
	AssessableValue float64 `json:"assessableValue"`
	BCDRate         float64 `json:"bcdRate"`
	BCDAmount       float64 `json:"bcdAmount"`
	SWSRate         float64 `json:"swsRate"`
	SWSAmount       float64 `json:"swsAmount"`
	IGSTRate        float64 `json:"igstRate"`
	IGSTAmount      float64 `json:"igstAmount"`
	BOEInterest     float64 `json:"boeInterest"`
	TotalDuty       float64 `json:"totalDuty"`
	DutyPaid        float64 `json:"dutyPaid"`
	ChallanNumber   string  `json:"challanNumber"`
	PaymentDate     string  `json:"paymentDate"`
}

// PartExpenseRow details an individual shipment expense and its proportional allocation to this part.
type PartExpenseRow struct {
	ShipmentID              string  `json:"shipmentId"`
	ShipmentInvoiceNumber   string  `json:"shipmentInvoiceNumber"`
	ExpenseInvoiceID        string  `json:"expenseInvoiceId"`
	ExpenseID               string  `json:"expenseId"`
	ServiceProviderID       string  `json:"serviceProviderId"`
	ServiceProviderName     string  `json:"serviceProviderName"`
	ExpenseTypeID           string  `json:"expenseTypeId"`
	ExpenseTypeName         string  `json:"expenseTypeName"`
	ExpenseInvoiceNo        string  `json:"expenseInvoiceNo"`
	ExpenseInvoiceDate      string  `json:"expenseInvoiceDate"`
	ShipmentExpenseBasicINR float64 `json:"shipmentExpenseBasicInr"`
	ShipmentExpenseGSTINR   float64 `json:"shipmentExpenseGstInr"`
	ShipmentExpenseTotalINR float64 `json:"shipmentExpenseTotalInr"`
	AllocationRatio         float64 `json:"allocationRatio"`
	AllocatedBasicINR       float64 `json:"allocatedBasicInr"`
	AllocatedGSTINR         float64 `json:"allocatedGstInr"`
	AllocatedTotalINR       float64 `json:"allocatedTotalInr"`
	Remarks                 string  `json:"remarks"`
}

// PartTrendPoint plots chronological landed cost components across shipments.
type PartTrendPoint struct {
	Date                  string  `json:"date"`
	ShipmentID            string  `json:"shipmentId"`
	InvoiceNumber         string  `json:"invoiceNumber"`
	Quantity              float64 `json:"quantity"`
	UnitPriceForex        float64 `json:"unitPriceForex"`
	Currency              string  `json:"currency"`
	MaterialValuePerUnit  float64 `json:"materialValuePerUnit"`
	AssessableValuePerUnit float64 `json:"assessableValuePerUnit"`
	DutyPerUnit           float64 `json:"dutyPerUnit"`
	ExpensePerUnit        float64 `json:"expensePerUnit"`
	LandedCostPerUnit     float64 `json:"landedCostPerUnit"`
}

// PartPeriodAnalysisResult bundles the complete 360-degree period view for a part.
type PartPeriodAnalysisResult struct {
	TargetItem    PartItemMasterInfo `json:"targetItem"`
	Summary       PartPeriodSummary  `json:"summary"`
	Shipments     []PartShipmentRow  `json:"shipments"`
	BOELines      []PartBOERow       `json:"boeLines"`
	Expenses      []PartExpenseRow   `json:"expenses"`
	Trends        []PartTrendPoint   `json:"trends"`
	MatchingParts []PartLookupItem   `json:"matchingParts"`
	StartDate     string             `json:"startDate"`
	EndDate       string             `json:"endDate"`
	DateBasis     string             `json:"dateBasis"`
}
