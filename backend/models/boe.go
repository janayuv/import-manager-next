package models

// BOEDetail mirrors a row in boe_details.
type BOEDetail struct {
	ID                   string  `json:"id"`
	BENumber             string  `json:"beNumber"`
	BEDate               string  `json:"beDate"`
	Location             string  `json:"location"`
	TotalAssessmentValue float64 `json:"totalAssessmentValue"`
	DutyAmount           float64 `json:"dutyAmount"`
	PaymentDate          string  `json:"paymentDate"`
	DutyPaid             float64 `json:"dutyPaid"`
	ChallanNumber        string  `json:"challanNumber"`
	RefID                string  `json:"refId"`
	TransactionID        string  `json:"transactionId"`
}

// BOEDetailInput carries fields to create or update a boe_details record.
type BOEDetailInput struct {
	BENumber             string  `json:"beNumber"`
	BEDate               string  `json:"beDate"`
	Location             string  `json:"location"`
	TotalAssessmentValue float64 `json:"totalAssessmentValue"`
	DutyAmount           float64 `json:"dutyAmount"`
	PaymentDate          string  `json:"paymentDate"`
	DutyPaid             float64 `json:"dutyPaid"`
	ChallanNumber        string  `json:"challanNumber"`
	RefID                string  `json:"refId"`
	TransactionID        string  `json:"transactionId"`
}

// BOEAttachment mirrors a row in boe_attachments.
type BOEAttachment struct {
	ID               string `json:"id"`
	BOECalculationID string `json:"boeCalculationId"`
	FileName         string `json:"fileName"`
	FilePath         string `json:"filePath"`
	UploadedAt       string `json:"uploadedAt"`
}

// IncotermRule captures statutory valuation cost inclusion treatment for Incoterms 2020.
type IncotermRule struct {
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	FreightIncluded       bool   `json:"freightIncluded"`
	InsuranceIncluded     bool   `json:"insuranceIncluded"`
	OriginChargesExpected bool   `json:"originChargesExpected"`
	Description           string `json:"description"`
}

// BOECalculationItemInput carries per-item rate inputs and calculation method selection.
type BOECalculationItemInput struct {
	ItemID            string  `json:"itemId"`
	PartNumber        string  `json:"partNumber"`
	Description       string  `json:"description"`
	HSNCode           string  `json:"hsnCode"`
	Unit              string  `json:"unit"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unitPrice"`
	ActualBCDRate     float64 `json:"actualBcdRate"`
	ActualSWSRate     float64 `json:"actualSwsRate"`
	ActualIGSTRate    float64 `json:"actualIgstRate"`
	CalculationMethod string  `json:"calculationMethod"` // Standard, CEPA, Rodtep
	BOEBCDRate        float64 `json:"boeBcdRate"`
	BOESWSRate        float64 `json:"boeSwsRate"`
	BOEIGSTRate       float64 `json:"boeIgstRate"`
}

// BOECalcParams holds header parameters and line items for duty computation & persistence.
type BOECalcParams struct {
	ID                      string                    `json:"id,omitempty"`
	ShipmentID              string                    `json:"shipmentId"`
	BOEID                   string                    `json:"boeId"`
	SupplierName            string                    `json:"supplierName"`
	InvoiceNumber           string                    `json:"invoiceNumber"`
	InvoiceCurrency         string                    `json:"invoiceCurrency"`
	Incoterm                string                    `json:"incoterm"`
	Status                  string                    `json:"status"`
	ExchangeRate            float64                   `json:"exchangeRate"`
	FreightCost             float64                   `json:"freightCost"`          // INR
	EXWCost                 float64                   `json:"exwCost"`              // INR (backward compatible)
	OriginInlandFreight     float64                   `json:"originInlandFreight"`  // INR
	ExportHandling          float64                   `json:"exportHandling"`       // INR
	LoadingCharges          float64                   `json:"loadingCharges"`       // INR
	PackingCharges          float64                   `json:"packingCharges"`       // INR
	OtherRule10Additions    float64                   `json:"otherRule10Additions"` // INR (Rule 10 assists/royalties)
	InsuranceMode           string                    `json:"insuranceMode"`        // "deemed_1.125", "actual", or "included"
	ActualInsurance         float64                   `json:"actualInsurance"`      // INR (actual policy)
	InsuranceRate           float64                   `json:"insuranceRate"`        // % (statutory 1.125 deemed or custom)
	RoDTEPScripBalance      float64                   `json:"rodtepScripBalance"`   // Available e-scrip in INR
	CertificateOfOriginNo   string                    `json:"certificateOfOriginNo,omitempty"`
	CertificateOfOriginDate string                    `json:"certificateOfOriginDate,omitempty"`
	TradeAgreement          string                    `json:"tradeAgreement,omitempty"`
	Interest                float64                   `json:"interest"` // INR
	LineItems               []BOECalculationItemInput `json:"lineItems"`
}

// BOECalcLineResult captures exact computed INR valuation & duties for a single item.
type BOECalcLineResult struct {
	ItemID            string  `json:"itemId"`
	PartNumber        string  `json:"partNumber"`
	Description       string  `json:"description"`
	HSNCode           string  `json:"hsnCode"`
	Unit              string  `json:"unit"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unitPrice"`
	ItemForexValue    float64 `json:"itemForexValue"`
	ItemValueINR      float64 `json:"itemValueInr"`
	ItemFreight       float64 `json:"itemFreight"`
	ItemEXW           float64 `json:"itemExw"`
	ItemInsurance     float64 `json:"itemInsurance"`
	ItemOriginCharges float64 `json:"itemOriginCharges"`
	AssessableValue   float64 `json:"assessableValue"`
	CalculationMethod string  `json:"calculationMethod"`
	ActualBCDRate     float64 `json:"actualBcdRate"`
	ActualSWSRate     float64 `json:"actualSwsRate"`
	ActualIGSTRate    float64 `json:"actualIgstRate"`
	BOEBCDRate        float64 `json:"boeBcdRate"`
	BOESWSRate        float64 `json:"boeSwsRate"`
	BOEIGSTRate       float64 `json:"boeIgstRate"`
	GrossBCD          float64 `json:"grossBcd"`
	RoDTEPUtilized    float64 `json:"rodtepUtilized"`
	BCDCashPayable    float64 `json:"bcdCashPayable"`
	BCDValue          float64 `json:"bcdValue"` // Keeps gross BCD liability
	SWSValue          float64 `json:"swsValue"`
	IGSTValue         float64 `json:"igstValue"`
	TotalLineDuty     float64 `json:"totalLineDuty"`     // Gross assessed line duty
	TotalLineCashDuty float64 `json:"totalLineCashDuty"` // Net cash duty payable
	BCDDiscrepancy    bool    `json:"bcdDiscrepancy"`
}

// BOECalculationResult holds complete calculated totals and validation status flags.
type BOECalculationResult struct {
	Items                  []BOECalcLineResult `json:"items"`
	TotalInvoiceValue      float64             `json:"totalInvoiceValue"`
	TotalValueINR          float64             `json:"totalValueInr"`
	TotalFreight           float64             `json:"totalFreight"`
	TotalEXW               float64             `json:"totalExw"`
	TotalInsurance         float64             `json:"totalInsurance"`
	TotalOriginCharges     float64             `json:"totalOriginCharges"`
	TotalAssessableValue   float64             `json:"totalAssessableValue"`
	GrossBCDTotal          float64             `json:"grossBcdTotal"`
	RoDTEPScripUtilized    float64             `json:"rodtepScripUtilized"`
	BCDCashTotal           float64             `json:"bcdCashTotal"`
	BCDTotal               float64             `json:"bcdTotal"` // matches GrossBCDTotal
	SWSTotal               float64             `json:"swsTotal"`
	IGSTTotal              float64             `json:"igstTotal"`
	Interest               float64             `json:"interest"`
	CustomsDutyTotal       float64             `json:"customsDutyTotal"`     // Gross assessed duty liability
	CashCustomsDutyTotal   float64             `json:"cashCustomsDutyTotal"` // Net cash challan amount
	HasBlockingBCDOverride bool                `json:"hasBlockingBcdOverride"`
	CIFZeroLocked          bool                `json:"cifZeroLocked"`
	IncotermRule           IncotermRule        `json:"incotermRule"`
	PayloadBytes           int                 `json:"payloadBytes"`
	PayloadWarning         bool                `json:"payloadWarning"`
}

// BOECalculationContext preloads invoice lines and any existing saved calculation for a shipment.
type BOECalculationContext struct {
	Shipment Shipment             `json:"shipment"`
	Params   BOECalcParams        `json:"params"`
	Result   BOECalculationResult `json:"result"`
	HasSaved bool                 `json:"hasSaved"`
}

// BOECalculationSummary summarizes a saved row in boe_calculations joined with boe_details.
type BOECalculationSummary struct {
	ID              string  `json:"id"`
	ShipmentID      string  `json:"shipmentId"`
	BOEID           string  `json:"boeId"`
	BENumber        string  `json:"beNumber"`
	BEDate          string  `json:"beDate"`
	SupplierName    string  `json:"supplierName"`
	InvoiceNumber   string  `json:"invoiceNumber"`
	Status          string  `json:"status"`
	CustomsDutyTotal float64 `json:"customsDutyTotal"`
	DutyPaid        float64 `json:"dutyPaid"`
	AssessableTotal float64 `json:"assessableTotal"`
	CreatedAt       string  `json:"createdAt"`
}

// BOEOverrideImportResult holds applied item overrides plus skipped row warnings.
type BOEOverrideImportResult struct {
	Items        []BOECalculationItemInput `json:"items"`
	AppliedCount int                       `json:"appliedCount"`
	SkippedCount int                       `json:"skippedCount"`
	Warnings     []CSVImportRowError       `json:"warnings"`
}
