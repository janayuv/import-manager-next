package models

// ServiceProvider mirrors a row in service_providers.
type ServiceProvider struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	GSTIN         string `json:"gstin"`
	State         string `json:"state"`
	ContactPerson string `json:"contactPerson"`
	ContactEmail  string `json:"contactEmail"`
	ContactPhone  string `json:"contactPhone"`
	CreatedAt     string `json:"createdAt"`
}

// ServiceProviderInput carries fields to create or update a service provider.
type ServiceProviderInput struct {
	Name          string `json:"name"`
	GSTIN         string `json:"gstin"`
	State         string `json:"state"`
	ContactPerson string `json:"contactPerson"`
	ContactEmail  string `json:"contactEmail"`
	ContactPhone  string `json:"contactPhone"`
}

// ExpenseType mirrors a row in expense_types with integer basis point defaults.
type ExpenseType struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	DefaultCGSTRateBP int64  `json:"defaultCgstRateBp"`
	DefaultSGSTRateBP int64  `json:"defaultSgstRateBp"`
	DefaultIGSTRateBP int64  `json:"defaultIgstRateBp"`
	DefaultTDSRateBP  int64  `json:"defaultTdsRateBp"`
	IsActive          bool   `json:"isActive"`
	CreatedAt         string `json:"createdAt"`
}

// ExpenseTypeInput carries fields to create or update an expense type.
type ExpenseTypeInput struct {
	Name              string `json:"name"`
	DefaultCGSTRateBP int64  `json:"defaultCgstRateBp"`
	DefaultSGSTRateBP int64  `json:"defaultSgstRateBp"`
	DefaultIGSTRateBP int64  `json:"defaultIgstRateBp"`
	DefaultTDSRateBP  int64  `json:"defaultTdsRateBp"`
	IsActive          bool   `json:"isActive"`
}

// ExpenseLineInput represents a single expense type line on a multi-line expense invoice.
type ExpenseLineInput struct {
	ExpenseTypeID string `json:"expenseTypeId"`
	AmountPaise   int64  `json:"amountPaise"`
	IsManualGST   bool   `json:"isManualGst"`
	RoundOff      bool   `json:"roundOff"`
	CGSTPaise     int64  `json:"cgstPaise"`
	SGSTPaise     int64  `json:"sgstPaise"`
	IGSTPaise     int64  `json:"igstPaise"`
	CGSTRateBP    int64  `json:"cgstRateBp"`
	SGSTRateBP    int64  `json:"sgstRateBp"`
	IGSTRateBP    int64  `json:"igstRateBp"`
	TDSRateBP     int64  `json:"tdsRateBp"`
	Remarks       string `json:"remarks"`
}

// ExpenseLineCalculation captures exact integer paise calculations for one expense line.
type ExpenseLineCalculation struct {
	ExpenseTypeID string  `json:"expenseTypeId"`
	AmountPaise   int64   `json:"amountPaise"`
	IsManualGST   bool    `json:"isManualGst"`
	CGSTRateBP    int64   `json:"cgstRateBp"`
	SGSTRateBP    int64   `json:"sgstRateBp"`
	IGSTRateBP    int64   `json:"igstRateBp"`
	TDSRateBP     int64   `json:"tdsRateBp"`
	CGSTPaise     int64   `json:"cgstPaise"`
	SGSTPaise     int64   `json:"sgstPaise"`
	IGSTPaise     int64   `json:"igstPaise"`
	TDSPaise      int64   `json:"tdsPaise"`
	TotalPaise    int64   `json:"totalPaise"`
	NetPaise      int64   `json:"netPaise"`
	AmountINR     float64 `json:"amountInr"`
	TotalINR      float64 `json:"totalInr"`
	NetINR        float64 `json:"netInr"`
	Remarks       string  `json:"remarks"`
}

// ExpenseInvoiceCalculation summarizes integer-paise totals across all lines of an invoice.
type ExpenseInvoiceCalculation struct {
	Lines            []ExpenseLineCalculation `json:"lines"`
	BaseAmountPaise  int64                    `json:"baseAmountPaise"`
	TotalCGSTPaise   int64                    `json:"totalCgstPaise"`
	TotalSGSTPaise   int64                    `json:"totalSgstPaise"`
	TotalIGSTPaise   int64                    `json:"totalIgstPaise"`
	TotalTDSPaise    int64                    `json:"totalTdsPaise"`
	RoundOffPaise    int64                    `json:"roundOffPaise"`
	TotalAmountPaise int64                    `json:"totalAmountPaise"`
	NetAmountPaise   int64                    `json:"netAmountPaise"`
	TotalAmountINR   float64                  `json:"totalAmountInr"`
	NetAmountINR     float64                  `json:"netAmountInr"`
	HasDuplicateType bool                     `json:"hasDuplicateType"`
}

// ExpenseInvoiceRequest creates or updates an expense invoice atomically with lines.
type ExpenseInvoiceRequest struct {
	ID                string             `json:"id,omitempty"`
	ShipmentID        string             `json:"shipmentId"`
	ServiceProviderID string             `json:"serviceProviderId"`
	InvoiceNo         string             `json:"invoiceNo"`
	InvoiceDate       string             `json:"invoiceDate"`
	Currency          string             `json:"currency"`
	Remarks           string             `json:"remarks"`
	IdempotencyKey    string             `json:"idempotencyKey"`
	Version           int                `json:"version"`
	MergeDuplicates   bool               `json:"mergeDuplicates"`
	RoundOffPaise     int64              `json:"roundOffPaise"`
	Lines             []ExpenseLineInput `json:"lines"`
}

// ExpenseLine mirrors a persisted row in expenses joined with expense_types.
type ExpenseLine struct {
	ID                string `json:"id"`
	ExpenseInvoiceID  string `json:"expenseInvoiceId"`
	ShipmentID        string `json:"shipmentId"`
	ServiceProviderID string `json:"serviceProviderId"`
	ExpenseTypeID     string `json:"expenseTypeId"`
	ExpenseTypeName   string `json:"expenseTypeName"`
	AmountPaise       int64  `json:"amountPaise"`
	IsManualGST       bool   `json:"isManualGst"`
	CGSTRateBP        int64  `json:"cgstRateBp"`
	SGSTRateBP        int64  `json:"sgstRateBp"`
	IGSTRateBP        int64  `json:"igstRateBp"`
	TDSRateBP         int64  `json:"tdsRateBp"`
	CGSTPaise         int64  `json:"cgstPaise"`
	SGSTPaise         int64  `json:"sgstPaise"`
	IGSTPaise         int64  `json:"igstPaise"`
	TDSPaise          int64  `json:"tdsPaise"`
	TotalPaise        int64  `json:"totalPaise"`
	NetPaise          int64  `json:"netPaise"`
	Remarks           string `json:"remarks"`
}

// ExpenseInvoice mirrors a persisted row in expense_invoices plus lines.
type ExpenseInvoice struct {
	ID                  string        `json:"id"`
	ShipmentID          string        `json:"shipmentId"`
	ShipmentInvoiceNo   string        `json:"shipmentInvoiceNo"`
	ServiceProviderID   string        `json:"serviceProviderId"`
	ServiceProviderName string        `json:"serviceProviderName"`
	InvoiceNo           string        `json:"invoiceNo"`
	InvoiceDate         string        `json:"invoiceDate"`
	Currency            string        `json:"currency"`
	BaseAmountPaise     int64         `json:"baseAmountPaise"`
	TotalCGSTPaise      int64         `json:"totalCgstPaise"`
	TotalSGSTPaise      int64         `json:"totalSgstPaise"`
	TotalIGSTPaise      int64         `json:"totalIgstPaise"`
	TotalTDSPaise       int64         `json:"totalTdsPaise"`
	TotalAmountPaise    int64         `json:"totalAmountPaise"`
	RoundOffPaise       int64         `json:"roundOffPaise"`
	NetAmountPaise      int64         `json:"netAmountPaise"`
	Remarks             string        `json:"remarks"`
	IdempotencyKey      string        `json:"idempotencyKey"`
	Version             int           `json:"version"`
	CreatedAt           string        `json:"createdAt"`
	UpdatedAt           string        `json:"updatedAt"`
	Lines               []ExpenseLine `json:"lines"`
}

// ExpenseAttachment mirrors a row in expense_attachments.
type ExpenseAttachment struct {
	ID         string `json:"id"`
	ExpenseID  string `json:"expenseId"`
	FileName   string `json:"fileName"`
	FilePath   string `json:"filePath"`
	FileType   string `json:"fileType"`
	UploadedAt string `json:"uploadedAt"`
	UploadedBy string `json:"uploadedBy"`
}

// ExpenseReportFilter filters the 5 views in Expense Reports.
type ExpenseReportFilter struct {
	ViewMode          string `json:"viewMode"` // detailed, summary-by-type, summary-by-provider, summary-by-shipment, summary-by-month
	StartDate         string `json:"startDate"`
	EndDate           string `json:"endDate"`
	ShipmentID        string `json:"shipmentId"`
	ServiceProviderID string `json:"serviceProviderId"`
	ExpenseTypeID     string `json:"expenseTypeId"`
	MinAmountPaise    int64  `json:"minAmountPaise"`
	MaxAmountPaise    int64  `json:"maxAmountPaise"`
	Search            string `json:"search"`
}

// ExpenseReportDetailedRow represents a row in the 'detailed' Expense Report view.
type ExpenseReportDetailedRow struct {
	ExpenseID           string `json:"expenseId"`
	ExpenseInvoiceID    string `json:"expenseInvoiceId"`
	ShipmentID          string `json:"shipmentId"`
	ShipmentInvoiceNo   string `json:"shipmentInvoiceNo"`
	ServiceProviderID   string `json:"serviceProviderId"`
	ServiceProviderName string `json:"serviceProviderName"`
	ExpenseTypeID       string `json:"expenseTypeId"`
	ExpenseTypeName     string `json:"expenseTypeName"`
	InvoiceNo           string `json:"invoiceNo"`
	InvoiceDate         string `json:"invoiceDate"`
	AmountPaise         int64  `json:"amountPaise"`
	CGSTPaise           int64  `json:"cgstPaise"`
	SGSTPaise           int64  `json:"sgstPaise"`
	IGSTPaise           int64  `json:"igstPaise"`
	TDSPaise            int64  `json:"tdsPaise"`
	TotalPaise          int64  `json:"totalPaise"`
	NetPaise            int64  `json:"netPaise"`
	Remarks             string `json:"remarks"`
}

// ExpenseReportGroup represents an aggregated bucket for the 4 summary views.
type ExpenseReportGroup struct {
	GroupKey    string `json:"groupKey"`
	GroupLabel  string `json:"groupLabel"`
	LineCount   int    `json:"lineCount"`
	AmountPaise int64  `json:"amountPaise"`
	CGSTPaise   int64  `json:"cgstPaise"`
	SGSTPaise   int64  `json:"sgstPaise"`
	IGSTPaise   int64  `json:"igstPaise"`
	TDSPaise    int64  `json:"tdsPaise"`
	TotalPaise  int64  `json:"totalPaise"`
	NetPaise    int64  `json:"netPaise"`
}

// ExpenseReportResult holds filtered expense report rows, summary groupings, and grand totals.
type ExpenseReportResult struct {
	ViewMode          string                     `json:"viewMode"`
	DetailedRows      []ExpenseReportDetailedRow `json:"detailedRows"`
	GroupedRows       []ExpenseReportGroup       `json:"groupedRows"`
	TotalLines        int                        `json:"totalLines"`
	TotalBasePaise    int64                      `json:"totalBasePaise"`
	TotalTaxPaise     int64                      `json:"totalTaxPaise"`
	TotalTDSPaise     int64                      `json:"totalTdsPaise"`
	TotalGrossPaise   int64                      `json:"totalGrossPaise"`
	TotalNetPaise     int64                      `json:"totalNetPaise"`
}

// LandedCostFilter filters the report_view Landed Cost Report.
type LandedCostFilter struct {
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
	SupplierID string `json:"supplierId"`
	InvoiceNo  string `json:"invoiceNo"`
	PartNo     string `json:"partNo"`
	Search     string `json:"search"`
}

// LandedCostRow mirrors the columns exposed by report_view (16 export columns + supplierId).
type LandedCostRow struct {
	Supplier        string  `json:"supplier"`
	SupplierID      string  `json:"supplierId"`
	InvoiceNo       string  `json:"invoiceNo"`
	InvoiceDate     string  `json:"invoiceDate"`
	PartNo          string  `json:"partNo"`
	Description     string  `json:"description"`
	Unit            string  `json:"unit"`
	Qty             float64 `json:"qty"`
	UnitPrice       float64 `json:"unitPrice"`
	MaterialValue   float64 `json:"materialValue"`
	AssessableValue float64 `json:"assessableValue"`
	BCDAmount       float64 `json:"bcdAmount"`
	SWSAmount       float64 `json:"swsAmount"`
	IGSTAmount      float64 `json:"igstAmount"`
	BoeInterest     float64 `json:"boeInterest"`
	ExpensesTotal   float64 `json:"expensesTotal"`
	LDCPerQty       float64 `json:"ldcPerQty"`
}

// LandedCostSummary provides KPI totals for Landed Cost analysis.
type LandedCostSummary struct {
	TotalRows             int     `json:"totalRows"`
	TotalQuantity         float64 `json:"totalQuantity"`
	TotalMaterialValue    float64 `json:"totalMaterialValue"`
	TotalAssessableValue  float64 `json:"totalAssessableValue"`
	TotalBasicDuties      float64 `json:"totalBasicDuties"` // BCD + SWS
	TotalIGST             float64 `json:"totalIgst"`
	TotalBoeInterest      float64 `json:"totalBoeInterest"`
	TotalBasicExpenses    float64 `json:"totalBasicExpenses"`
	TotalLandedCostExIGST float64 `json:"totalLandedCostExIgst"`
}

// LandedCostReport bundles the filtered rows and summary KPIs.
type LandedCostReport struct {
	Rows    []LandedCostRow   `json:"rows"`
	Summary LandedCostSummary `json:"summary"`
}
