package models

// PurchaseOrder mirrors a row in purchase_orders with supplier metadata and computed financial balances.
type PurchaseOrder struct {
	ID                   string                 `json:"id"`
	PONumber             string                 `json:"poNumber"`
	PODate               string                 `json:"poDate"`
	SupplierID           string                 `json:"supplierId"`
	SupplierName         string                 `json:"supplierName,omitempty"`
	Currency             string                 `json:"currency"`
	ExchangeRate         float64                `json:"exchangeRate"`
	Incoterm             string                 `json:"incoterm"`
	PaymentTermsPreset   string                 `json:"paymentTermsPreset"` // "100_ADVANCE", "40_40_20", "30_30_40", "50_50", "CUSTOM"
	PaymentTermsDesc     string                 `json:"paymentTermsDesc"`
	TotalAmount          float64                `json:"totalAmount"`
	Status               string                 `json:"status"` // "DRAFT", "CONFIRMED", "PARTIALLY_PAID", "FULLY_PAID", "COMPLETED", "CANCELLED"
	ExpectedDeliveryDate string                 `json:"expectedDeliveryDate,omitempty"`
	Notes                string                 `json:"notes,omitempty"`
	AdvanceRequired      float64                `json:"advanceRequired"`
	AdvancePaid          float64                `json:"advancePaid"`
	AdvanceBalance       float64                `json:"advanceBalance"`
	TotalPaid            float64                `json:"totalPaid"`
	TotalBalance         float64                `json:"totalBalance"`
	LineItems            []POLineItem           `json:"lineItems,omitempty"`
	Milestones           []POPaymentMilestone   `json:"milestones,omitempty"`
	CreatedAt            string                 `json:"createdAt"`
	UpdatedAt            string                 `json:"updatedAt"`
}

// POLineItem mirrors a row in po_line_items with tracking metrics.
type POLineItem struct {
	ID                     string  `json:"id"`
	POID                   string  `json:"poId"`
	ItemID                 string  `json:"itemId,omitempty"`
	PartNumber             string  `json:"partNumber"`
	ItemDescription        string  `json:"itemDescription"`
	Quantity               float64 `json:"quantity"`
	Unit                   string  `json:"unit"`
	UnitPrice              float64 `json:"unitPrice"`
	LineTotal              float64 `json:"lineTotal"`
	HSNCode                string  `json:"hsnCode,omitempty"`
	ReceivedQuantity       float64 `json:"receivedQuantity"`
	PendingQuantity        float64 `json:"pendingQuantity"`
	AdvanceCoveredQuantity float64 `json:"advanceCoveredQuantity"`
	AdvancePendingQuantity float64 `json:"advancePendingQuantity"`
	FulfillmentStatus      string  `json:"fulfillmentStatus"` // "AWAITING_SHIPMENT", "PARTIALLY_RECEIVED", "FULLY_RECEIVED", "OVER_SUPPLIED"
}

// POPaymentMilestone represents a staged milestone payment (e.g. 40% Advance, 40% on BL, 20% on Delivery).
type POPaymentMilestone struct {
	ID            string  `json:"id"`
	POID          string  `json:"poId"`
	Sequence      int     `json:"sequence"`
	MilestoneName string  `json:"milestoneName"`
	StageType     string  `json:"stageType"` // "ADVANCE", "ON_DISPATCH", "ON_DELIVERY", "CUSTOM"
	Percentage    float64 `json:"percentage"`
	Amount        float64 `json:"amount"`
	PaidAmount    float64 `json:"paidAmount"`
	BalanceAmount float64 `json:"balanceAmount"`
	DueDate       string  `json:"dueDate,omitempty"`
	Status        string  `json:"status"` // "PENDING", "PARTIALLY_PAID", "PAID"
	Notes         string  `json:"notes,omitempty"`
}

// POLineItemInput carries line item data for creating or editing a PO.
type POLineItemInput struct {
	ItemID          string  `json:"itemId,omitempty"`
	PartNumber      string  `json:"partNumber"`
	ItemDescription string  `json:"itemDescription"`
	Quantity        float64 `json:"quantity"`
	Unit            string  `json:"unit"`
	UnitPrice       float64 `json:"unitPrice"`
	HSNCode         string  `json:"hsnCode,omitempty"`
}

// POMilestoneInput carries staged milestone data for a PO.
type POMilestoneInput struct {
	Sequence      int     `json:"sequence"`
	MilestoneName string  `json:"milestoneName"`
	StageType     string  `json:"stageType"`
	Percentage    float64 `json:"percentage"`
	DueDate       string  `json:"dueDate,omitempty"`
	Notes         string  `json:"notes,omitempty"`
}

// PurchaseOrderInput submits or updates a complete Purchase Order dossier.
type PurchaseOrderInput struct {
	ID                   string             `json:"id,omitempty"`
	PONumber             string             `json:"poNumber"`
	PODate               string             `json:"poDate"`
	SupplierID           string             `json:"supplierId"`
	Currency             string             `json:"currency"`
	ExchangeRate         float64            `json:"exchangeRate"`
	Incoterm             string             `json:"incoterm"`
	PaymentTermsPreset   string             `json:"paymentTermsPreset"`
	PaymentTermsDesc     string             `json:"paymentTermsDesc,omitempty"`
	Status               string             `json:"status,omitempty"`
	ExpectedDeliveryDate string             `json:"expectedDeliveryDate,omitempty"`
	Notes                string             `json:"notes,omitempty"`
	LineItems            []POLineItemInput  `json:"lineItems"`
	Milestones           []POMilestoneInput `json:"milestones,omitempty"`
}

// PurchaseOrderPageRequest specifies filters, sorting, and pagination for POs.
type PurchaseOrderPageRequest struct {
	Search     string `json:"search,omitempty"`
	SupplierID string `json:"supplierId,omitempty"`
	Status     string `json:"status,omitempty"`
	SortBy     string `json:"sortBy,omitempty"`
	SortDir    string `json:"sortDir,omitempty"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// PurchaseOrderPage returns paginated PO records and metadata totals.
type PurchaseOrderPage struct {
	Items      []PurchaseOrder `json:"items"`
	TotalCount int             `json:"totalCount"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
}

// Payment mirrors a row in payments (Payment Voucher).
type Payment struct {
	ID                 string              `json:"id"`
	VoucherNumber      string              `json:"voucherNumber"`
	PaymentDate        string              `json:"paymentDate"`
	SupplierID         string              `json:"supplierId"`
	SupplierName       string              `json:"supplierName,omitempty"`
	PaymentType        string              `json:"paymentType"` // "PO_ADVANCE", "SINGLE_INVOICE", "MULTI_INVOICE_BATCH"
	PaymentMode        string              `json:"paymentMode"` // "WIRE_TRANSFER", "TT", "RTGS", "NEFT", "LC", "CHEQUE", "CASH"
	Currency           string              `json:"currency"`
	ExchangeRate       float64             `json:"exchangeRate"`
	TotalAmount        float64             `json:"totalAmount"`
	BankName           string              `json:"bankName"`
	BankAccountNo      string              `json:"bankAccountNo"`
	ReferenceNumber    string              `json:"referenceNumber"` // UTR / Cheque / Swift Ref
	BeneficiaryName    string              `json:"beneficiaryName"`
	BeneficiaryBank    string              `json:"beneficiaryBank"`
	BeneficiaryAccount string              `json:"beneficiaryAccount"`
	BeneficiarySWIFT   string              `json:"beneficiarySwift"`
	BeneficiaryIBAN    string              `json:"beneficiaryIban"`
	Status             string              `json:"status"` // "DRAFT", "PROCESSED", "CANCELLED"
	Notes              string              `json:"notes,omitempty"`
	CreatedBy          string              `json:"createdBy,omitempty"`
	Allocations        []PaymentAllocation `json:"allocations,omitempty"`
	CreatedAt          string              `json:"createdAt"`
	UpdatedAt          string              `json:"updatedAt"`
}

// PaymentAllocation records an itemized allocation to a PO milestone or commercial invoice.
type PaymentAllocation struct {
	ID              string  `json:"id"`
	PaymentID       string  `json:"paymentId"`
	POID            string  `json:"poId,omitempty"`
	PONumber        string  `json:"poNumber,omitempty"`
	POMilestoneID   string  `json:"poMilestoneId,omitempty"`
	MilestoneName   string  `json:"milestoneName,omitempty"`
	InvoiceID       string  `json:"invoiceId,omitempty"`
	InvoiceNumber   string  `json:"invoiceNumber,omitempty"`
	ShipmentID      string  `json:"shipmentId,omitempty"`
	GrossAmount     float64 `json:"grossAmount"`
	AdvanceAdjusted float64 `json:"advanceAdjusted"`
	Deductions      float64 `json:"deductions"`
	NetPaidAmount   float64 `json:"netPaidAmount"`
	Notes           string  `json:"notes,omitempty"`
}

// PaymentAllocationInput carries allocation details for payment processing.
type PaymentAllocationInput struct {
	POID            string  `json:"poId,omitempty"`
	POMilestoneID   string  `json:"poMilestoneId,omitempty"`
	InvoiceID       string  `json:"invoiceId,omitempty"`
	ShipmentID      string  `json:"shipmentId,omitempty"`
	GrossAmount     float64 `json:"grossAmount"`
	AdvanceAdjusted float64 `json:"advanceAdjusted"`
	Deductions      float64 `json:"deductions"`
	Notes           string  `json:"notes,omitempty"`
}

// PaymentInput carries the payload for creating a payment voucher.
type PaymentInput struct {
	PaymentDate        string                   `json:"paymentDate"`
	SupplierID         string                   `json:"supplierId"`
	PaymentType        string                   `json:"paymentType"`
	PaymentMode        string                   `json:"paymentMode"`
	Currency           string                   `json:"currency"`
	ExchangeRate       float64                  `json:"exchangeRate"`
	TotalAmount        float64                  `json:"totalAmount"`
	BankName           string                   `json:"bankName"`
	BankAccountNo      string                   `json:"bankAccountNo"`
	ReferenceNumber    string                   `json:"referenceNumber"`
	BeneficiaryName    string                   `json:"beneficiaryName,omitempty"`
	BeneficiaryBank    string                   `json:"beneficiaryBank,omitempty"`
	BeneficiaryAccount string                   `json:"beneficiaryAccount,omitempty"`
	BeneficiarySWIFT   string                   `json:"beneficiarySwift,omitempty"`
	BeneficiaryIBAN    string                   `json:"beneficiaryIban,omitempty"`
	Notes              string                   `json:"notes,omitempty"`
	Allocations        []PaymentAllocationInput `json:"allocations"`
}

// PaymentPageRequest specifies filters, sorting, and pagination for Payments.
type PaymentPageRequest struct {
	Search      string `json:"search,omitempty"`
	SupplierID  string `json:"supplierId,omitempty"`
	PaymentType string `json:"paymentType,omitempty"`
	Status      string `json:"status,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	EndDate     string `json:"endDate,omitempty"`
	Page        int    `json:"page"`
	PageSize    int    `json:"pageSize"`
}

// PaymentPage returns paginated Payment records and metadata totals.
type PaymentPage struct {
	Items      []Payment `json:"items"`
	TotalCount int       `json:"totalCount"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
}

// InvoicePaymentCandidate represents an outstanding commercial invoice available for batch or individual payment.
type InvoicePaymentCandidate struct {
	InvoiceID          string  `json:"invoiceId"`
	ShipmentID         string  `json:"shipmentId"`
	SupplierID         string  `json:"supplierId"`
	SupplierName       string  `json:"supplierName"`
	InvoiceNumber      string  `json:"invoiceNumber"`
	InvoiceDate        string  `json:"invoiceDate"`
	Currency           string  `json:"currency"`
	GrossAmount        float64 `json:"grossAmount"`
	AlreadyPaid        float64 `json:"alreadyPaid"`
	AdvanceAdjusted    float64 `json:"advanceAdjusted"`
	PendingBalance     float64 `json:"pendingBalance"`
	PaymentTerm        string  `json:"paymentTerm"`
	DueDate            string  `json:"dueDate"`
	DaysOverdue        int     `json:"daysOverdue"`
	IsOverdue          bool    `json:"isOverdue"`
	LinkedPONumber     string  `json:"linkedPoNumber,omitempty"`
	AvailablePOAdvance float64 `json:"availablePoAdvance,omitempty"`
}

// AdvanceReconciliationSummary captures matching metrics between PO advance payments and commercial invoices.
type AdvanceReconciliationSummary struct {
	POID                   string                  `json:"poId"`
	PONumber               string                  `json:"poNumber"`
	PODate                 string                  `json:"poDate"`
	SupplierID             string                  `json:"supplierId"`
	SupplierName           string                  `json:"supplierName"`
	Currency               string                  `json:"currency"`
	POTotalAmount          float64                 `json:"poTotalAmount"`
	AdvanceRequired        float64                 `json:"advanceRequired"`
	AdvancePaidAmount      float64                 `json:"advancePaidAmount"`
	AdvanceBalance         float64                 `json:"advanceBalance"`
	TotalInvoicedAmount    float64                 `json:"totalInvoicedAmount"`
	AdvanceAdjustedAmount  float64                 `json:"advanceAdjustedAmount"`
	UnadjustedAdvance      float64                 `json:"unadjustedAdvance"`
	VarianceAmount         float64                 `json:"varianceAmount"`         // AdvancePaid - Invoiced
	VarianceType           string                  `json:"varianceType"`           // "MATCHED", "UNDER_INVOICED", "OVER_INVOICED", "AWAITING_INVOICE"
	ReconciliationStatus   string                  `json:"reconciliationStatus"`   // "FULLY_RECONCILED", "PARTIALLY_RECONCILED", "DISCREPANCY", "PENDING_INVOICE"
	Invoices               []ReconciledInvoiceItem `json:"invoices"`
}

// ReconciledInvoiceItem represents an invoice matched against a PO advance.
type ReconciledInvoiceItem struct {
	InvoiceID        string  `json:"invoiceId"`
	ShipmentID       string  `json:"shipmentId"`
	InvoiceNumber    string  `json:"invoiceNumber"`
	InvoiceDate      string  `json:"invoiceDate"`
	InvoiceValue     float64 `json:"invoiceValue"`
	Currency         string  `json:"currency"`
	AdvanceAllocated float64 `json:"advanceAllocated"`
	DirectPaid       float64 `json:"directPaid"`
	BalanceRemaining float64 `json:"balanceRemaining"`
	Status           string  `json:"status"`
}

// MaterialTrackingItem tracks line-item ordered, advance-paid, and physical/invoiced receipts.
type MaterialTrackingItem struct {
	POLineItemID               string  `json:"poLineItemId"`
	POID                       string  `json:"poId"`
	PONumber                   string  `json:"poNumber"`
	PODate                     string  `json:"poDate"`
	SupplierID                 string  `json:"supplierId"`
	SupplierName               string  `json:"supplierName"`
	Currency                   string  `json:"currency"`
	PartNumber                 string  `json:"partNumber"`
	ItemDescription            string  `json:"itemDescription"`
	Unit                       string  `json:"unit"`
	UnitPrice                  float64 `json:"unitPrice"`
	OrderedQuantity            float64 `json:"orderedQuantity"`
	OrderedAmount              float64 `json:"orderedAmount"`
	AdvancePaidPercentage      float64 `json:"advancePaidPercentage"`
	AdvancePaidQuantity        float64 `json:"advancePaidQuantity"`
	ReceivedQuantity           float64 `json:"receivedQuantity"`
	PendingQuantity            float64 `json:"pendingQuantity"`
	AdvancePaidPendingQuantity float64 `json:"advancePaidPendingQuantity"` // Paid for in advance but not yet received
	AdvanceLockedCapital       float64 `json:"advanceLockedCapital"`       // Currency value of advance-paid unreceived material
	FulfillmentStatus          string  `json:"fulfillmentStatus"`          // "AWAITING_SHIPMENT", "PARTIALLY_RECEIVED", "FULLY_RECEIVED", "OVER_SUPPLIED"
	ExpectedDeliveryDate       string  `json:"expectedDeliveryDate,omitempty"`
	DaysDelayed                int     `json:"daysDelayed"`
	IsDelayed                  bool    `json:"isDelayed"`
}

// MaterialReceiptInput submits an inward delivery against a PO line item.
type MaterialReceiptInput struct {
	POLineItemID      string  `json:"poLineItemId"`
	InvoiceLineItemID string  `json:"invoiceLineItemId,omitempty"`
	ShipmentID        string  `json:"shipmentId,omitempty"`
	ReceiptDate       string  `json:"receiptDate"`
	QuantityReceived  float64 `json:"quantityReceived"`
	Remarks           string  `json:"remarks,omitempty"`
}

// MaterialTrackingSummary aggregates KPI metrics for advance-paid material tracking.
type MaterialTrackingSummary struct {
	TotalPOTracked              int     `json:"totalPoTracked"`
	TotalItemsTracked           int     `json:"totalItemsTracked"`
	TotalAdvanceCapitalLocked   float64 `json:"totalAdvanceCapitalLocked"`
	TotalPendingQuantity        float64 `json:"totalPendingQuantity"`
	TotalAdvancePendingQuantity float64 `json:"totalAdvancePendingQuantity"`
	DelayedItemsCount           int     `json:"delayedItemsCount"`
}

// PaymentAnnexureItem represents a line item in a formal payment annexure document.
type PaymentAnnexureItem struct {
	SlNo                 int     `json:"slNo"`
	DocumentType         string  `json:"documentType"` // "PO Advance", "Commercial Invoice"
	DocumentNumber       string  `json:"documentNumber"`
	DocumentDate         string  `json:"documentDate"`
	GrossAmount          float64 `json:"grossAmount"`
	AdvanceAdjusted      float64 `json:"advanceAdjusted"`
	Deductions           float64 `json:"deductions"`
	NetAmount            float64 `json:"netAmount"`
	PaymentTermMilestone string  `json:"paymentTermMilestone"`
	Remarks              string  `json:"remarks"`
}

// PaymentAnnexure encapsulates the complete dossier for bank remittance / finance approval.
type PaymentAnnexure struct {
	AnnexureNumber       string                `json:"annexureNumber"`
	PaymentVoucherNumber string                `json:"paymentVoucherNumber"`
	GeneratedDate        string                `json:"generatedDate"`
	PaymentDate          string                `json:"paymentDate"`
	PaymentMode          string                `json:"paymentMode"`
	RemitterCompany      string                `json:"remitterCompany"`
	RemitterBank         string                `json:"remitterBank"`
	RemitterAccount      string                `json:"remitterAccount"`
	ReferenceNumber      string                `json:"referenceNumber"` // UTR / Cheque Ref
	SupplierID           string                `json:"supplierId"`
	SupplierName         string                `json:"supplierName"`
	SupplierAddress      string                `json:"supplierAddress"`
	SupplierCountry      string                `json:"supplierCountry"`
	BeneficiaryName      string                `json:"beneficiaryName"`
	BeneficiaryBank      string                `json:"beneficiaryBank"`
	BeneficiaryAccount   string                `json:"beneficiaryAccount"`
	BeneficiarySWIFT     string                `json:"beneficiarySwift"`
	BeneficiaryIBAN      string                `json:"beneficiaryIban"`
	Currency             string                `json:"currency"`
	ExchangeRate         float64               `json:"exchangeRate"`
	Items                []PaymentAnnexureItem `json:"items"`
	TotalGrossAmount     float64               `json:"totalGrossAmount"`
	TotalAdvanceAdjusted float64               `json:"totalAdvanceAdjusted"`
	TotalDeductions      float64               `json:"totalDeductions"`
	TotalNetAmount       float64               `json:"totalNetAmount"`
	NetAmountInWords     string                `json:"netAmountInWords"`
	Notes                string                `json:"notes"`
	PreparedBy           string                `json:"preparedBy"`
	CheckedBy            string                `json:"checkedBy"`
	ApprovedBy           string                `json:"approvedBy"`
}
