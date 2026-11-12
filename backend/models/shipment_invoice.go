package models

// Shipment mirrors a row in shipments joined with supplier_name.
type Shipment struct {
	ID              string  `json:"id"`
	SupplierID      string  `json:"supplierId"`
	SupplierName    string  `json:"supplierName,omitempty"`
	InvoiceNumber   string  `json:"invoiceNumber"`
	InvoiceDate     string  `json:"invoiceDate"`
	GoodsCategory   string  `json:"goodsCategory"`
	InvoiceValue    float64 `json:"invoiceValue"`
	InvoiceCurrency string  `json:"invoiceCurrency"`
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
	CreatedAt       string  `json:"createdAt"`
}

// ShipmentInput carries fields for creating or updating a shipment.
type ShipmentInput struct {
	ID              string  `json:"id,omitempty"`
	SupplierID      string  `json:"supplierId"`
	SupplierName    string  `json:"supplierName,omitempty"`
	InvoiceNumber   string  `json:"invoiceNumber"`
	InvoiceDate     string  `json:"invoiceDate"`
	GoodsCategory   string  `json:"goodsCategory"`
	InvoiceValue    float64 `json:"invoiceValue"`
	InvoiceCurrency string  `json:"invoiceCurrency"`
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
}

// ShipmentPageRequest filters, sorts, and paginates shipments.
type ShipmentPageRequest struct {
	Search     string `json:"search"`
	SupplierID string `json:"supplierId"`
	Status     string `json:"status"`
	FrozenOnly bool   `json:"frozenOnly"`
	SortBy     string `json:"sortBy"`
	SortDir    string `json:"sortDir"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// ShipmentPage returns paginated shipment records and totals.
type ShipmentPage struct {
	Items      []Shipment `json:"items"`
	TotalCount int        `json:"totalCount"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}

// ShipmentLinkedBOE summarizes a linked row in boe_details for shipment inspection.
type ShipmentLinkedBOE struct {
	ID          string  `json:"id"`
	BENumber    string  `json:"beNumber"`
	BEDate      string  `json:"beDate"`
	Location    string  `json:"location"`
	DutyAmount  float64 `json:"dutyAmount"`
	DutyPaid    float64 `json:"dutyPaid"`
	ReconStatus string  `json:"reconStatus"`
}

// ShipmentLinkedExpense summarizes a linked row in expenses for shipment inspection.
type ShipmentLinkedExpense struct {
	ID          string  `json:"id"`
	ExpenseType string  `json:"expenseType"`
	Amount      float64 `json:"amount"`
	TotalAmount float64 `json:"totalAmount"`
	Remarks     string  `json:"remarks"`
}

// ShipmentDetail bundles a shipment with related invoices, BOEs, and expenses.
type ShipmentDetail struct {
	Shipment Shipment                `json:"shipment"`
	Invoices []InvoiceSummary        `json:"invoices"`
	BOEs     []ShipmentLinkedBOE     `json:"boes"`
	Expenses []ShipmentLinkedExpense `json:"expenses"`
}

// InvoiceSummary represents a row in invoices enriched with shipment and line totals.
type InvoiceSummary struct {
	ID                   string  `json:"id"`
	ShipmentID           string  `json:"shipmentId"`
	SupplierID           string  `json:"supplierId"`
	SupplierName         string  `json:"supplierName"`
	InvoiceNumber        string  `json:"invoiceNumber"`
	InvoiceDate          string  `json:"invoiceDate"`
	Currency             string  `json:"currency"`
	ShipmentInvoiceValue float64 `json:"shipmentInvoiceValue"`
	ComputedTotal        float64 `json:"computedTotal"`
	LineCount            int     `json:"lineCount"`
	Status               string  `json:"status"`
	LineTotalDecimals    int     `json:"lineTotalDecimals"`
	InvoiceTotalDecimals int     `json:"invoiceTotalDecimals"`
	ShipmentFrozen       bool    `json:"shipmentFrozen"`
}

// InvoiceLineItem mirrors an enriched row in invoice_line_items.
type InvoiceLineItem struct {
	ID              string  `json:"id"`
	InvoiceID       string  `json:"invoiceId"`
	ItemID          string  `json:"itemId"`
	PartNumber      string  `json:"partNumber"`
	ItemDescription string  `json:"itemDescription"`
	Unit            string  `json:"unit"`
	HSNCode         string  `json:"hsnCode"`
	Quantity        float64 `json:"quantity"`
	UnitPrice       float64 `json:"unitPrice"`
	MasterUnitPrice float64 `json:"masterUnitPrice"`
	LineTotal       float64 `json:"lineTotal"`
	DutyPercent     float64 `json:"dutyPercent"`
	SWSPercent      float64 `json:"swsPercent"`
	IGSTPercent     float64 `json:"igstPercent"`
	PriceDiverged   bool    `json:"priceDiverged"`
}

// InvoiceDetail holds full invoice header context and enriched line items.
type InvoiceDetail struct {
	Invoice InvoiceSummary    `json:"invoice"`
	Lines   []InvoiceLineItem `json:"lines"`
}

// InvoiceLineInput carries a line item payload sent from the 3-step Invoice Wizard.
type InvoiceLineInput struct {
	ItemID      string  `json:"itemId"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	DutyPercent float64 `json:"dutyPercent"`
	SWSPercent  float64 `json:"swsPercent"`
	IGSTPercent float64 `json:"igstPercent"`
}

// SaveInvoiceRequest submits a Draft or Finalized commercial invoice from the Invoice Wizard.
type SaveInvoiceRequest struct {
	InvoiceID            string             `json:"invoiceId,omitempty"`
	ShipmentID           string             `json:"shipmentId"`
	Finalize             bool               `json:"finalize"`
	FreezeShipment       bool               `json:"freezeShipment"`
	LineTotalDecimals    int                `json:"lineTotalDecimals"`
	InvoiceTotalDecimals int                `json:"invoiceTotalDecimals"`
	Lines                []InvoiceLineInput `json:"lines"`
}

// ParsedPasteLine represents one parsed and Item-Master-matched row from multi-line clipboard paste.
type ParsedPasteLine struct {
	Row             int     `json:"row"`
	PartNumber      string  `json:"partNumber"`
	ItemID          string  `json:"itemId"`
	ItemDescription string  `json:"itemDescription"`
	Unit            string  `json:"unit"`
	Quantity        float64 `json:"quantity"`
	UnitPrice       float64 `json:"unitPrice"`
	MasterUnitPrice float64 `json:"masterUnitPrice"`
	DutyPercent     float64 `json:"dutyPercent"`
	SWSPercent      float64 `json:"swsPercent"`
	IGSTPercent     float64 `json:"igstPercent"`
	PriceDiverged   bool    `json:"priceDiverged"`
	Matched         bool    `json:"matched"`
	Error           string  `json:"error,omitempty"`
}
