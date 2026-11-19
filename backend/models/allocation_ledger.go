package models

// ExpenseAllocationRecord mirrors a row in expense_allocation_ledger.
type ExpenseAllocationRecord struct {
	ID                     int64   `json:"id"`
	RunID                  string  `json:"runId"`
	ShipmentID             string  `json:"shipmentId"`
	BOEID                  string  `json:"boeId,omitempty"`
	ExpenseInvoiceID       string  `json:"expenseInvoiceId,omitempty"`
	ExpenseTypeID          string  `json:"expenseTypeId,omitempty"`
	ExpenseCategory        string  `json:"expenseCategory"`
	ItemID                 string  `json:"itemId,omitempty"`
	PartNumber             string  `json:"partNumber"`
	BaseExpenseAmountPaise int64   `json:"baseExpenseAmountPaise"`
	AllocationMethod       string  `json:"allocationMethod"`
	AllocationBasisValue   float64 `json:"allocationBasisValue"`
	AllocationPercentage   float64 `json:"allocationPercentage"`
	AllocatedAmountPaise   int64   `json:"allocatedAmountPaise"`
	AllocatedAmountINR     float64 `json:"allocatedAmountInr"`
	RecalculationReason    string  `json:"recalculationReason"`
	CalculatedAt           string  `json:"calculatedAt"`
	CalculatedBy           string  `json:"calculatedBy"`
}

// AllocationLedgerFilter defines query criteria for querying historical expense allocations.
type AllocationLedgerFilter struct {
	ShipmentID string `json:"shipmentId,omitempty"`
	RunID      string `json:"runId,omitempty"`
	PartNumber string `json:"partNumber,omitempty"`
	StartDate  string `json:"startDate,omitempty"`
	EndDate    string `json:"endDate,omitempty"`
}

// AllocationRunSummary provides high-level metrics for an allocation execution run.
type AllocationRunSummary struct {
	RunID              string  `json:"runId"`
	ShipmentID         string  `json:"shipmentId"`
	TotalAllocatedINR  float64 `json:"totalAllocatedInr"`
	ItemCount          int     `json:"itemCount"`
	RecalculationReason string  `json:"recalculationReason"`
	CalculatedAt       string  `json:"calculatedAt"`
	CalculatedBy       string  `json:"calculatedBy"`
}
