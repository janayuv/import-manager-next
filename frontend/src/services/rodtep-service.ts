import * as RoDTEPBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/rodtep/service.js';
import type {
  RoDTEPScrip,
  RoDTEPScripInput,
  RoDTEPAllocation,
  RoDTEPAllocationItemInput,
  RoDTEPAllocationConfirmInput,
  RoDTEPDebitLedger,
  RoDTEPReversalInput,
  RoDTEPAuditLog,
  RoDTEPReconciliation,
  RoDTEPReconciliationInput,
  RoDTEPPortSummary,
  RoDTEPBOEDebitSummary,
  BOEEvalContext,
} from '../types/rodtep';

export const rodtepService = {
  // Scrip Master Operations
  createScrip(input: RoDTEPScripInput): Promise<RoDTEPScrip> {
    return RoDTEPBindings.CreateScrip(input as any) as any;
  },
  updateScrip(id: string, input: RoDTEPScripInput): Promise<RoDTEPScrip> {
    return RoDTEPBindings.UpdateScrip(id, input as any) as any;
  },
  getScrip(id: string): Promise<RoDTEPScrip> {
    return RoDTEPBindings.GetScrip(id) as any;
  },
  listScrips(portCode = 'ALL', status = 'ALL', search = ''): Promise<RoDTEPScrip[]> {
    return RoDTEPBindings.ListScrips(portCode, status, search).then((items) => (items ?? []) as any);
  },
  deleteScrip(id: string): Promise<void> {
    return RoDTEPBindings.DeleteScrip(id);
  },
  verifyOpeningBalance(id: string, evidenceRef: string): Promise<void> {
    return RoDTEPBindings.VerifyOpeningBalance(id, evidenceRef);
  },

  // BOE Evaluation Context
  getBOEEvalContext(boeID: string): Promise<BOEEvalContext> {
    return RoDTEPBindings.GetBOEEvalContext(boeID).then((ctx: any) => {
      if (ctx) {
        ctx.availableScrips = (ctx.availableScrips ?? []) as any;
        ctx.activeAllocations = (ctx.activeAllocations ?? []) as any;
      }
      return ctx as any;
    });
  },

  // Stage 1: Allocation
  allocateScrips(boeID: string, items: RoDTEPAllocationItemInput[], remarks = ''): Promise<RoDTEPAllocation[]> {
    return RoDTEPBindings.AllocateScrips(boeID, items as any, remarks).then((items) => (items ?? []) as any);
  },

  // Stage 2: Confirmation
  confirmActualDebit(input: RoDTEPAllocationConfirmInput): Promise<RoDTEPDebitLedger> {
    return RoDTEPBindings.ConfirmActualDebit(input as any) as any;
  },

  // Closeout
  closeAllocation(allocationID: string, reason = ''): Promise<void> {
    return RoDTEPBindings.CloseAllocation(allocationID, reason);
  },

  // Reversal
  reverseDebit(input: RoDTEPReversalInput): Promise<RoDTEPDebitLedger> {
    return RoDTEPBindings.ReverseDebit(input as any) as any;
  },

  // Reconciliation
  saveReconciliation(input: RoDTEPReconciliationInput): Promise<RoDTEPReconciliation> {
    return RoDTEPBindings.SaveReconciliation(input as any) as any;
  },
  listReconciliations(scripID: string): Promise<RoDTEPReconciliation[]> {
    return RoDTEPBindings.ListReconciliations(scripID).then((items) => (items ?? []) as any);
  },

  // Audit Logs & Port Health
  listAuditLogs(scripID: string): Promise<RoDTEPAuditLog[]> {
    return RoDTEPBindings.ListAuditLogs(scripID).then((items) => (items ?? []) as any);
  },
  getPortSummary(): Promise<RoDTEPPortSummary[]> {
    return RoDTEPBindings.GetPortSummary().then((items) => (items ?? []) as any);
  },
  getBOEDebitReport(): Promise<RoDTEPBOEDebitSummary[]> {
    return RoDTEPBindings.GetBOEDebitReport().then((items) => (items ?? []) as any);
  },
  ingestInitialScrips(scrips: RoDTEPScripInput[], verifiedBy = ''): Promise<number> {
    return RoDTEPBindings.IngestInitialScrips(scrips as any, verifiedBy);
  },
};
