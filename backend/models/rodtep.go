package models

// RoDTEP Port Reference Constants
const (
	PortINMAA1 = "INMAA1" // Chennai Sea Port
	PortINMAA4 = "INMAA4" // Chennai Air Cargo Complex
)

// Acquisition Types
const (
	AcquisitionDirectGeneration = "DIRECT_GENERATION"
	AcquisitionOpeningBalance   = "OPENING_BALANCE"
)

// Scrip Statuses
const (
	ScripStatusActive    = "ACTIVE"
	ScripStatusExhausted = "EXHAUSTED"
	ScripStatusExpired   = "EXPIRED"
	ScripStatusSuspended = "SUSPENDED"
)

// Allocation Statuses
const (
	AllocStatusDraft              = "DRAFT"
	AllocStatusAllocated          = "ALLOCATED"
	AllocStatusPartiallyConfirmed = "PARTIALLY_CONFIRMED"
	AllocStatusConfirmed          = "CONFIRMED"
	AllocStatusCancelled          = "CANCELLED"
	AllocStatusClosed             = "CLOSED"
)

// Debit Ledger Transaction Types
const (
	TxTypeDebitConfirmed = "DEBIT_CONFIRMED"
	TxTypeDebitReversal  = "DEBIT_REVERSAL"
)

// Audit Event Types
const (
	AuditEventScripCreated         = "SCRIP_CREATED"
	AuditEventScripUpdated         = "SCRIP_UPDATED"
	AuditEventScripStatusChange    = "SCRIP_STATUS_CHANGE"
	AuditEventAllocationReserved   = "ALLOCATION_RESERVED"
	AuditEventAllocationReleased   = "ALLOCATION_RELEASED"
	AuditEventDebitConfirmed       = "DEBIT_CONFIRMED"
	AuditEventDebitReversed        = "DEBIT_REVERSED"
	AuditEventReconciliationEntry  = "RECONCILIATION_ENTRY"
)

// RoDTEPScrip mirrors a row in rodtep_scrips.
type RoDTEPScrip struct {
	ID                     string  `json:"id"`
	ScripNumber            string  `json:"scripNumber"`
	ScripPortCode          string  `json:"scripPortCode"` // INMAA1 or INMAA4
	AcquisitionType        string  `json:"acquisitionType"`
	IssueDate              string  `json:"issueDate"`     // YYYY-MM-DD
	ExpiryDate             string  `json:"expiryDate"`    // YYYY-MM-DD
	OriginalValuePaise     int64   `json:"originalValuePaise"`
	OpeningBalancePaise    int64   `json:"openingBalancePaise"`
	PreERPUtilizedPaise    int64   `json:"preErpUtilizedPaise"`
	ConfirmedUtilizedPaise int64   `json:"confirmedUtilizedPaise"`
	ReversalRecreditPaise  int64   `json:"reversalRecreditPaise"`
	ConfirmedBalancePaise  int64   `json:"confirmedBalancePaise"`
	PendingAllocatedPaise  int64   `json:"pendingAllocatedPaise"`
	NetAvailablePaise      int64   `json:"netAvailablePaise"` // confirmed - pending
	Status                 string  `json:"status"`            // ACTIVE, EXHAUSTED, EXPIRED, SUSPENDED
	ICEGATEUserID          string  `json:"icegateUserId"`
	RegistrationDetails    string  `json:"registrationDetails"`
	DocumentRef            string  `json:"documentRef"`
	Remarks                string  `json:"remarks"`
	OpeningVerifiedAt      string  `json:"openingVerifiedAt"`
	OpeningVerifiedBy      string  `json:"openingVerifiedBy"`
	OpeningEvidenceRef     string  `json:"openingEvidenceRef"`
	CreatedAt              string  `json:"createdAt"`
	UpdatedAt              string  `json:"updatedAt"`
	// User-friendly Rupee approximations for UI display
	OriginalValueINR  float64 `json:"originalValueInr"`
	ConfirmedBalINR   float64 `json:"confirmedBalInr"`
	PendingAllocINR   float64 `json:"pendingAllocInr"`
	NetAvailableINR   float64 `json:"netAvailableInr"`
	DaysToExpiry      int     `json:"daysToExpiry"`
}

// RoDTEPScripInput carries parameters to create or update a RoDTEP scrip.
type RoDTEPScripInput struct {
	ScripNumber         string  `json:"scripNumber"`
	ScripPortCode       string  `json:"scripPortCode"`
	AcquisitionType     string  `json:"acquisitionType"`
	IssueDate           string  `json:"issueDate"`
	ExpiryDate          string  `json:"expiryDate"`
	OriginalValuePaise  int64   `json:"originalValuePaise"`
	OpeningBalancePaise int64   `json:"openingBalancePaise"`
	PreERPUtilizedPaise int64   `json:"preErpUtilizedPaise"`
	ICEGATEUserID       string  `json:"icegateUserId"`
	RegistrationDetails string  `json:"registrationDetails"`
	DocumentRef         string  `json:"documentRef"`
	Remarks             string  `json:"remarks"`
	OpeningVerifiedAt   string  `json:"openingVerifiedAt"`
	OpeningVerifiedBy   string  `json:"openingVerifiedBy"`
	OpeningEvidenceRef  string  `json:"openingEvidenceRef"`
}

// RoDTEPAllocation mirrors a row in rodtep_allocations.
type RoDTEPAllocation struct {
	ID                   string  `json:"id"`
	AllocationRef        string  `json:"allocationRef"`
	ScripID              string  `json:"scripId"`
	ScripNumber          string  `json:"scripNumber,omitempty"`
	ScripPortCode        string  `json:"scripPortCode,omitempty"`
	BOEID                string  `json:"boeId"`
	BOENumber            string  `json:"boeNumber"`
	BOEDate              string  `json:"boeDate"`
	BOEPortCode          string  `json:"boePortCode"`
	AllocatedAmountPaise int64   `json:"allocatedAmountPaise"`
	ConfirmedAmountPaise int64   `json:"confirmedAmountPaise"`
	ReleasedAmountPaise  int64   `json:"releasedAmountPaise"`
	OutstandingPaise     int64   `json:"outstandingPaise"` // allocated - confirmed - released
	Status               string  `json:"status"`           // ALLOCATED, PARTIALLY_CONFIRMED, CONFIRMED, CANCELLED, CLOSED
	OperatorUserID       string  `json:"operatorUserId"`
	OperatorName         string  `json:"operatorName"`
	Remarks              string  `json:"remarks"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt"`
	AllocatedINR         float64 `json:"allocatedInr"`
	ConfirmedINR         float64 `json:"confirmedInr"`
	OutstandingINR       float64 `json:"outstandingInr"`
}

// RoDTEPAllocationItemInput specifies a single scrip manual allocation request.
type RoDTEPAllocationItemInput struct {
	ScripID      string `json:"scripId"`
	AmountPaise  int64  `json:"amountPaise"`
	Remarks      string `json:"remarks"`
}

// RoDTEPAllocationConfirmInput parameters to confirm an actual ICEGATE debit against an allocation.
type RoDTEPAllocationConfirmInput struct {
	AllocationID          string `json:"allocationId"`
	AmountPaise           int64  `json:"amountPaise"` // confirmed debit amount in paise
	ICEGATETransactionRef string `json:"icegateTransactionRef"`
	ChallanNumber         string `json:"challanNumber"`
	DebitDate             string `json:"debitDate"`
	SupportingDocRef      string `json:"supportingDocRef"`
	Remarks               string `json:"remarks"`
}

// RoDTEPDebitLedger mirrors a row in rodtep_debit_ledger.
type RoDTEPDebitLedger struct {
	ID                            string  `json:"id"`
	TransactionRef                string  `json:"transactionRef"`
	AllocationID                  string  `json:"allocationId,omitempty"`
	ScripID                       string  `json:"scripId"`
	ScripNumber                   string  `json:"scripNumber"`
	ScripPortCode                 string  `json:"scripPortCode"`
	BOEID                         string  `json:"boeId"`
	BOENumber                     string  `json:"boeNumber"`
	BOEDate                       string  `json:"boeDate"`
	BOEPortCode                   string  `json:"boePortCode"`
	TxType                        string  `json:"txType"` // DEBIT_CONFIRMED, DEBIT_REVERSAL
	AmountPaise                   int64   `json:"amountPaise"`
	PreviousConfirmedBalancePaise int64   `json:"previousConfirmedBalancePaise"`
	RemainingConfirmedBalancePaise int64   `json:"remainingConfirmedBalancePaise"`
	ICEGATETransactionRef         string  `json:"icegateTransactionRef,omitempty"`
	ChallanNumber                 string  `json:"challanNumber,omitempty"`
	DebitDate                     string  `json:"debitDate"`
	OperatorUserID                string  `json:"operatorUserId"`
	OperatorName                  string  `json:"operatorName"`
	SupportingDocRef              string  `json:"supportingDocRef,omitempty"`
	Remarks                       string  `json:"remarks,omitempty"`
	ReversalOfID                  string  `json:"reversalOfId,omitempty"`
	CreatedAt                     string  `json:"createdAt"`
	AmountINR                     float64 `json:"amountInr"`
	RemainingINR                  float64 `json:"remainingInr"`
}

// RoDTEPReversalInput parameters to reverse a confirmed debit.
type RoDTEPReversalInput struct {
	DebitLedgerID string `json:"debitLedgerID"`
	AmountPaise   int64  `json:"amountPaise"`
	ReversalDate  string `json:"reversalDate"`
	Reason        string `json:"reason"`
	DocRef        string `json:"docRef,omitempty"`
}

// RoDTEPAuditLog mirrors a row in rodtep_audit_logs.
type RoDTEPAuditLog struct {
	ID             string `json:"id"`
	EventType      string `json:"eventType"`
	ScripID        string `json:"scripId"`
	AllocationID   string `json:"allocationId,omitempty"`
	LedgerID       string `json:"ledgerId,omitempty"`
	BOEID          string `json:"boeId,omitempty"`
	AmountPaise    int64  `json:"amountPaise,omitempty"`
	OldValuesJSON  string `json:"oldValuesJson,omitempty"`
	NewValuesJSON  string `json:"newValuesJson,omitempty"`
	Reason         string `json:"reason"`
	PerformedBy    string `json:"performedBy"`
	PerformedAt    string `json:"performedAt"`
}

// RoDTEPReconciliation mirrors a row in rodtep_reconciliations.
type RoDTEPReconciliation struct {
	ID                          string  `json:"id"`
	ScripID                     string  `json:"scripId"`
	ScripNumber                 string  `json:"scripNumber,omitempty"`
	ReconciliationDate          string  `json:"reconciliationDate"`
	StatementPeriodEnd          string  `json:"statementPeriodEnd"`
	ICEGATEStatementBalancePaise int64   `json:"icegateStatementBalancePaise"`
	ERPConfirmedBalancePaise    int64   `json:"erpConfirmedBalancePaise"`
	ERPPendingAllocatedPaise    int64   `json:"erpPendingAllocatedPaise"`
	DiscrepancyPaise            int64   `json:"discrepancyPaise"`
	DiscrepancyReason           string  `json:"discrepancyReason,omitempty"`
	SupportingStatementPath     string  `json:"supportingStatementPath,omitempty"`
	ReconciledBy                string  `json:"reconciledBy"`
	CreatedAt                   string  `json:"createdAt"`
	ICEGATEBalanceINR           float64 `json:"icegateBalanceInr"`
	ERPBalanceINR               float64 `json:"erpBalanceInr"`
	DiscrepancyINR              float64 `json:"discrepancyInr"`
}

// RoDTEPReconciliationInput parameters to save a statement reconciliation record.
type RoDTEPReconciliationInput struct {
	ScripID                      string `json:"scripId"`
	ReconciliationDate           string `json:"reconciliationDate"`
	StatementPeriodEnd           string `json:"statementPeriodEnd"`
	ICEGATEStatementBalancePaise int64  `json:"icegateStatementBalancePaise"`
	DiscrepancyReason            string `json:"discrepancyReason,omitempty"`
	SupportingStatementPath      string `json:"supportingStatementPath,omitempty"`
}

// RoDTEPPortSummary aggregates scrips, confirmed balances, and reservations per port.
type RoDTEPPortSummary struct {
	PortCode               string  `json:"portCode"` // INMAA1, INMAA4, or COMBINED
	PortName               string  `json:"portName"`
	TotalScripsCount       int     `json:"totalScripsCount"`
	ActiveScripsCount      int     `json:"activeScripsCount"`
	ExhaustedScripsCount   int     `json:"exhaustedScripsCount"`
	ExpiredScripsCount     int     `json:"expiredScripsCount"`
	OriginalValuePaise     int64   `json:"originalValuePaise"`
	ConfirmedUtilizedPaise int64   `json:"confirmedUtilizedPaise"`
	ConfirmedBalancePaise  int64   `json:"confirmedBalancePaise"`
	PendingAllocatedPaise  int64   `json:"pendingAllocatedPaise"`
	NetAvailablePaise      int64   `json:"netAvailablePaise"`
	OriginalValueINR       float64 `json:"originalValueInr"`
	ConfirmedUtilizedINR   float64 `json:"confirmedUtilizedInr"`
	ConfirmedBalanceINR    float64 `json:"confirmedBalanceInr"`
	PendingAllocatedINR    float64 `json:"pendingAllocatedInr"`
	NetAvailableINR        float64 `json:"netAvailableInr"`
}

// RoDTEPBOEDebitSummary summarizes all scrip allocations and debits for a single BOE.
type RoDTEPBOEDebitSummary struct {
	BOEID                 string              `json:"boeId"`
	BENumber              string              `json:"beNumber"`
	BEDate                string              `json:"beDate"`
	PortOfImport          string              `json:"portOfImport"`
	AssessedBCDTotalPaise int64               `json:"assessedBcdTotalPaise"`
	CashBCDPaidPaise      int64               `json:"cashBcdPaidPaise"`
	NetConfirmedPaise     int64               `json:"netConfirmedPaise"`
	ActiveReservedPaise   int64               `json:"activeReservedPaise"`
	EligibleUnpaidPaise   int64               `json:"eligibleUnpaidPaise"`
	AssessedBCDINR        float64             `json:"assessedBcdInr"`
	NetConfirmedINR       float64             `json:"netConfirmedInr"`
	EligibleUnpaidINR     float64             `json:"eligibleUnpaidInr"`
	Allocations           []RoDTEPAllocation  `json:"allocations"`
	ConfirmedDebits       []RoDTEPDebitLedger `json:"confirmedDebits"`
}

// BOEEvalContext loads live BOE customs duty details for the allocation modal.
type BOEEvalContext struct {
	BOEDetail             BOEDetail            `json:"boeDetail"`
	CalculationResult     *BOECalculationResult `json:"calculationResult,omitempty"`
	AssessedBCDTotalPaise int64                `json:"assessedBcdTotalPaise"`
	NetConfirmedRoDTEP    int64                `json:"netConfirmedRodtepPaise"`
	ActiveReservedRoDTEP  int64                `json:"activeReservedRodtepPaise"`
	CashBCDPaidPaise      int64                `json:"cashBcdPaidPaise"`
	EligibleUnpaidBCDPaise int64               `json:"eligibleUnpaidBcdPaise"`
	AssessedBCDINR        float64              `json:"assessedBcdInr"`
	EligibleUnpaidBCDINR  float64              `json:"eligibleUnpaidBcdInr"`
	AvailableScrips       []RoDTEPScrip        `json:"availableScrips"`
	ActiveAllocations     []RoDTEPAllocation   `json:"activeAllocations"`
}
