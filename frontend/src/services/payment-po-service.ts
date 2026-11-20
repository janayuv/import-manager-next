import * as PaymentPOBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/payment_po/service.js';
import type {
  AdvanceReconciliationSummary,
  InvoicePaymentCandidate,
  MaterialReceiptInput,
  MaterialTrackingItem,
  MaterialTrackingSummary,
  Payment,
  PaymentAllocation,
  PaymentAllocationInput,
  PaymentAnnexure,
  PaymentAnnexureItem,
  PaymentInput,
  PaymentPage,
  PaymentPageRequest,
  POLineItem,
  POLineItemInput,
  POMilestoneInput,
  POPaymentMilestone,
  PurchaseOrder,
  PurchaseOrderInput,
  PurchaseOrderPage,
  PurchaseOrderPageRequest,
  ReconciledInvoiceItem,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  AdvanceReconciliationSummary,
  InvoicePaymentCandidate,
  MaterialReceiptInput,
  MaterialTrackingItem,
  MaterialTrackingSummary,
  Payment,
  PaymentAllocation,
  PaymentAllocationInput,
  PaymentAnnexure,
  PaymentAnnexureItem,
  PaymentInput,
  PaymentPage,
  PaymentPageRequest,
  POLineItem,
  POLineItemInput,
  POMilestoneInput,
  POPaymentMilestone,
  PurchaseOrder,
  PurchaseOrderInput,
  PurchaseOrderPage,
  PurchaseOrderPageRequest,
  ReconciledInvoiceItem,
};

export const paymentPOService = {
  // Purchase Orders
  createPurchaseOrder(input: PurchaseOrderInput): Promise<PurchaseOrder> {
    return PaymentPOBindings.CreatePurchaseOrder(input);
  },
  getPurchaseOrder(id: string): Promise<PurchaseOrder> {
    return PaymentPOBindings.GetPurchaseOrder(id);
  },
  listPurchaseOrders(req: PurchaseOrderPageRequest): Promise<PurchaseOrderPage> {
    return PaymentPOBindings.ListPurchaseOrders(req);
  },
  deletePurchaseOrder(id: string): Promise<void> {
    return PaymentPOBindings.DeletePurchaseOrder(id);
  },

  // Payments / Vouchers
  createPayment(input: PaymentInput): Promise<Payment> {
    return PaymentPOBindings.CreatePayment(input);
  },
  getPayment(id: string): Promise<Payment> {
    return PaymentPOBindings.GetPayment(id);
  },
  listPayments(req: PaymentPageRequest): Promise<PaymentPage> {
    return PaymentPOBindings.ListPayments(req);
  },
  listInvoiceCandidates(supplierId: string): Promise<InvoicePaymentCandidate[]> {
    return PaymentPOBindings.ListInvoicePaymentCandidates(supplierId);
  },

  // Reconciliation
  getAdvanceReconciliation(supplierId: string): Promise<AdvanceReconciliationSummary[]> {
    return PaymentPOBindings.GetAdvanceReconciliation(supplierId);
  },
  linkInvoiceToPO(invoiceId: string, poId: string): Promise<void> {
    return PaymentPOBindings.LinkInvoiceToPO(invoiceId, poId);
  },

  // Material Tracking
  async getMaterialTracking(
    supplierId: string,
    search: string
  ): Promise<{ items: MaterialTrackingItem[]; summary: MaterialTrackingSummary }> {
    const [items, summary] = await PaymentPOBindings.GetMaterialTracking(supplierId, search);
    return { items, summary };
  },
  recordMaterialReceipt(input: MaterialReceiptInput): Promise<void> {
    return PaymentPOBindings.RecordMaterialReceipt(input);
  },

  // Payment Annexure
  generatePaymentAnnexure(paymentId: string, companyName: string): Promise<PaymentAnnexure> {
    return PaymentPOBindings.GeneratePaymentAnnexure(paymentId, companyName);
  },
};
