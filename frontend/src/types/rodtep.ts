export type RoDTEPPortCode = 'INMAA1' | 'INMAA4' | 'COMBINED' | string;
export type RoDTEPAcquisitionType = 'DIRECT_GENERATION' | 'OPENING_BALANCE' | string;
export type RoDTEPScripStatus = 'ACTIVE' | 'EXHAUSTED' | 'EXPIRED' | 'SUSPENDED' | string;
export type RoDTEPAllocStatus = 'DRAFT' | 'ALLOCATED' | 'PARTIALLY_CONFIRMED' | 'CONFIRMED' | 'CANCELLED' | 'CLOSED' | string;
export type RoDTEPTxType = 'DEBIT_CONFIRMED' | 'DEBIT_REVERSAL' | string;

export interface RoDTEPScrip {
  id: string;
  scripNumber: string;
  scripPortCode: string;
  acquisitionType: RoDTEPAcquisitionType;
  issueDate: string; // YYYY-MM-DD
  expiryDate: string; // YYYY-MM-DD
  originalValuePaise: number;
  openingBalancePaise: number;
  preErpUtilizedPaise: number;
  confirmedUtilizedPaise: number;
  reversalRecreditPaise: number;
  confirmedBalancePaise: number;
  pendingAllocatedPaise: number;
  netAvailablePaise: number;
  status: RoDTEPScripStatus;
  icegateUserId?: string;
  registrationDetails?: string;
  documentRef?: string;
  remarks?: string;
  openingVerifiedAt?: string;
  openingVerifiedBy?: string;
  openingEvidenceRef?: string;
  createdAt: string;
  updatedAt: string;
  originalValueInr: number;
  confirmedBalInr: number;
  pendingAllocInr: number;
  netAvailableInr: number;
  daysToExpiry: number;
}

export interface RoDTEPScripInput {
  scripNumber: string;
  scripPortCode: string;
  acquisitionType?: string;
  issueDate: string;
  expiryDate?: string;
  originalValuePaise: number;
  openingBalancePaise: number;
  preErpUtilizedPaise?: number;
  icegateUserId?: string;
  registrationDetails?: string;
  documentRef?: string;
  remarks?: string;
  openingVerifiedAt?: string;
  openingVerifiedBy?: string;
  openingEvidenceRef?: string;
}

export interface RoDTEPAllocation {
  id: string;
  allocationRef: string;
  scripId: string;
  scripNumber?: string;
  scripPortCode?: string;
  boeId: string;
  boeNumber: string;
  boeDate: string;
  boePortCode: string;
  allocatedAmountPaise: number;
  confirmedAmountPaise: number;
  releasedAmountPaise: number;
  outstandingPaise: number;
  status: RoDTEPAllocStatus;
  operatorUserId: string;
  operatorName: string;
  remarks?: string;
  createdAt: string;
  updatedAt: string;
  allocatedInr: number;
  confirmedInr: number;
  outstandingInr: number;
}

export interface RoDTEPAllocationItemInput {
  scripId: string;
  amountPaise: number;
  remarks?: string;
}

export interface RoDTEPAllocationConfirmInput {
  allocationId: string;
  amountPaise: number;
  icegateTransactionRef?: string;
  challanNumber?: string;
  debitDate: string;
  supportingDocRef?: string;
  remarks?: string;
}

export interface RoDTEPDebitLedger {
  id: string;
  transactionRef: string;
  allocationId?: string;
  scripId: string;
  scripNumber: string;
  scripPortCode: string;
  boeId: string;
  boeNumber: string;
  boeDate: string;
  boePortCode: string;
  txType: RoDTEPTxType;
  amountPaise: number;
  previousConfirmedBalancePaise: number;
  remainingConfirmedBalancePaise: number;
  icegateTransactionRef?: string;
  challanNumber?: string;
  debitDate: string;
  operatorUserId: string;
  operatorName: string;
  supportingDocRef?: string;
  remarks?: string;
  reversalOfId?: string;
  createdAt: string;
  amountInr: number;
  remainingInr: number;
}

export interface RoDTEPReversalInput {
  debitLedgerID: string;
  amountPaise: number;
  reversalDate?: string;
  reason: string;
  docRef?: string;
}

export interface RoDTEPAuditLog {
  id: string;
  eventType: string;
  scripId: string;
  allocationId?: string;
  ledgerId?: string;
  boeId?: string;
  amountPaise?: number;
  oldValuesJson?: string;
  newValuesJson?: string;
  reason: string;
  performedBy: string;
  performedAt: string;
}

export interface RoDTEPReconciliation {
  id: string;
  scripId: string;
  scripNumber?: string;
  reconciliationDate: string;
  statementPeriodEnd: string;
  icegateStatementBalancePaise: number;
  erpConfirmedBalancePaise: number;
  erpPendingAllocatedPaise: number;
  discrepancyPaise: number;
  discrepancyReason?: string;
  supportingStatementPath?: string;
  reconciledBy: string;
  createdAt: string;
  icegateBalanceInr: number;
  erpBalanceInr: number;
  discrepancyInr: number;
}

export interface RoDTEPReconciliationInput {
  scripId: string;
  reconciliationDate: string;
  statementPeriodEnd: string;
  icegateStatementBalancePaise: number;
  discrepancyReason?: string;
  supportingStatementPath?: string;
}

export interface RoDTEPPortSummary {
  portCode: string;
  portName: string;
  totalScripsCount: number;
  activeScripsCount: number;
  exhaustedScripsCount: number;
  expiredScripsCount: number;
  originalValuePaise: number;
  confirmedUtilizedPaise: number;
  confirmedBalancePaise: number;
  pendingAllocatedPaise: number;
  netAvailablePaise: number;
  originalValueInr: number;
  confirmedUtilizedInr: number;
  confirmedBalanceInr: number;
  pendingAllocatedInr: number;
  netAvailableInr: number;
}

export interface RoDTEPBOEDebitSummary {
  boeId: string;
  beNumber: string;
  beDate: string;
  portOfImport: string;
  assessedBcdTotalPaise: number;
  cashBcdPaidPaise: number;
  netConfirmedPaise: number;
  activeReservedPaise: number;
  eligibleUnpaidPaise: number;
  assessedBcdInr: number;
  netConfirmedInr: number;
  eligibleUnpaidInr: number;
  allocations: RoDTEPAllocation[];
  confirmedDebits: RoDTEPDebitLedger[];
}

export interface BOEEvalContext {
  boeDetail: {
    id: string;
    beNumber: string;
    beDate: string;
    location: string;
    totalAssessmentValue: number;
    dutyAmount: number;
    dutyPaid: number;
  };
  calculationResult?: any;
  assessedBcdTotalPaise: number;
  netConfirmedRodtepPaise: number;
  activeReservedRodtepPaise: number;
  cashBcdPaidPaise: number;
  eligibleUnpaidBcdPaise: number;
  assessedBcdInr: number;
  eligibleUnpaidBcdInr: number;
  availableScrips: RoDTEPScrip[];
  activeAllocations: RoDTEPAllocation[];
}
