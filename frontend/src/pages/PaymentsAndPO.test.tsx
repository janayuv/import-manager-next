import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { AdvanceReconciliationPage } from '@/pages/AdvanceReconciliationPage';
import { PaymentsPage } from '@/pages/PaymentsPage';
import { PurchaseOrdersPage } from '@/pages/PurchaseOrdersPage';

vi.mock('@/services/security-service', () => ({
  securityService: {
    getAuthSetupStatus: vi.fn().mockResolvedValue({ needsSetup: false, adminUsername: 'admin', recoveryMode: false }),
    getMySession: vi.fn().mockResolvedValue({
      sessionId: 's-test',
      userId: 'admin',
      username: 'admin',
      role: 'administrator',
      permissions: ['data.read', 'data.write', 'data.delete'],
      rememberMe: false,
      createdAt: '',
      lastActive: '',
      expiresAt: '',
    }),
    logout: vi.fn(),
  },
}));

vi.mock('@/services/master-service', () => ({
  supplierService: {
    list: vi.fn().mockResolvedValue({
      items: [
        { id: 'sup-1', supplierName: 'Precision Engineering Ltd', country: 'Germany' },
        { id: 'sup-2', supplierName: 'Tokyo Micro Electronics', country: 'Japan' },
      ],
      totalCount: 2,
    }),
  },
  itemService: {
    list: vi.fn().mockResolvedValue({
      items: [
        { id: 'it-1', partNumber: 'MCU-X86', itemDescription: 'Industrial MCU', unitPrice: 45.0, unit: 'NOS' },
      ],
      totalCount: 1,
    }),
  },
}));

vi.mock('@/services/shipment-invoice-service', () => ({
  invoiceService: {
    list: vi.fn().mockResolvedValue([
      { id: 'inv-1', invoiceNumber: 'INV-2026-001', shipmentInvoiceValue: 4800, currency: 'USD', supplierName: 'Precision Engineering Ltd' },
    ]),
  },
}));

vi.mock('@/services/payment-po-service', () => ({
  paymentPOService: {
    listPurchaseOrders: vi.fn().mockResolvedValue({
      items: [
        {
          id: 'po-1',
          poNumber: 'PO-2026-001',
          poDate: '2026-11-20',
          supplierId: 'sup-1',
          supplierName: 'Precision Engineering Ltd',
          currency: 'USD',
          paymentTermsPreset: '40_40_20',
          totalAmount: 10000.0,
          advanceRequired: 4000.0,
          advancePaid: 4000.0,
          advanceBalance: 0.0,
          status: 'PARTIALLY_PAID',
        },
      ],
      totalCount: 1,
    }),
    getPurchaseOrder: vi.fn().mockResolvedValue({
      id: 'po-1',
      poNumber: 'PO-2026-001',
      poDate: '2026-11-20',
      supplierName: 'Precision Engineering Ltd',
      currency: 'USD',
      paymentTermsPreset: '40_40_20',
      totalAmount: 10000.0,
      advanceRequired: 4000.0,
      advancePaid: 4000.0,
      advanceBalance: 0.0,
      milestones: [
        { id: 'm1', sequence: 1, milestoneName: '40% Advance', percentage: 40, amount: 4000, paidAmount: 4000, balanceAmount: 0, status: 'PAID' },
        { id: 'm2', sequence: 2, milestoneName: '40% on BL', percentage: 40, amount: 4000, paidAmount: 0, balanceAmount: 4000, status: 'PENDING' },
        { id: 'm3', sequence: 3, milestoneName: '20% on Delivery', percentage: 20, amount: 2000, paidAmount: 0, balanceAmount: 2000, status: 'PENDING' },
      ],
      lineItems: [
        { id: 'l1', partNumber: 'MCU-X86', itemDescription: 'Industrial MCU', quantity: 200, unit: 'NOS', receivedQuantity: 80, pendingQuantity: 120, advanceCoveredQuantity: 80, fulfillmentStatus: 'PARTIALLY_RECEIVED' },
      ],
    }),
    createPurchaseOrder: vi.fn(),
    deletePurchaseOrder: vi.fn(),
    listPayments: vi.fn().mockResolvedValue({
      items: [
        {
          id: 'pay-1',
          voucherNumber: 'PAY-2026-0001',
          paymentDate: '2026-11-20',
          supplierId: 'sup-1',
          supplierName: 'Precision Engineering Ltd',
          paymentType: 'PO_ADVANCE',
          paymentMode: 'WIRE_TRANSFER',
          currency: 'USD',
          totalAmount: 4000.0,
          referenceNumber: 'UTR-TEST-123',
          bankName: 'State Bank of India',
        },
      ],
      totalCount: 1,
    }),
    listInvoiceCandidates: vi.fn().mockResolvedValue([
      {
        invoiceId: 'inv-1',
        shipmentId: 'ship-1',
        supplierId: 'sup-1',
        supplierName: 'Precision Engineering Ltd',
        invoiceNumber: 'INV-2026-001',
        invoiceDate: '2026-11-15',
        currency: 'USD',
        grossAmount: 5000.0,
        alreadyPaid: 0.0,
        pendingBalance: 5000.0,
        paymentTerm: 'Net 30 Days',
        dueDate: '2026-12-15',
        daysOverdue: 0,
        isOverdue: false,
      },
    ]),
    createPayment: vi.fn(),
    getPayment: vi.fn(),
    getAdvanceReconciliation: vi.fn().mockResolvedValue([
      {
        poId: 'po-1',
        poNumber: 'PO-2026-001',
        poDate: '2026-11-20',
        supplierId: 'sup-1',
        supplierName: 'Precision Engineering Ltd',
        currency: 'USD',
        poTotalAmount: 10000.0,
        advancePaidAmount: 4000.0,
        totalInvoicedAmount: 4800.0,
        varianceAmount: -800.0,
        varianceType: 'OVER_INVOICED',
        reconciliationStatus: 'PARTIALLY_RECONCILED',
        invoices: [
          {
            invoiceId: 'inv-1',
            invoiceNumber: 'INV-2026-001',
            invoiceDate: '2026-11-15',
            invoiceValue: 4800.0,
            currency: 'USD',
            advanceAllocated: 4000.0,
            directPaid: 0.0,
            balanceRemaining: 800.0,
            status: 'PENDING_BALANCE',
          },
        ],
      },
    ]),
    linkInvoiceToPO: vi.fn(),
    getMaterialTracking: vi.fn().mockResolvedValue({
      items: [
        {
          poLineItemId: 'pli-1',
          poId: 'po-1',
          poNumber: 'PO-2026-001',
          poDate: '2026-11-20',
          supplierId: 'sup-1',
          supplierName: 'Precision Engineering Ltd',
          currency: 'USD',
          partNumber: 'MCU-X86',
          itemDescription: 'Industrial MCU',
          unit: 'NOS',
          unitPrice: 50.0,
          orderedQuantity: 100.0,
          orderedAmount: 5000.0,
          advancePaidPercentage: 40.0,
          advancePaidQuantity: 40.0,
          receivedQuantity: 30.0,
          pendingQuantity: 70.0,
          advancePaidPendingQuantity: 10.0,
          advanceLockedCapital: 500.0,
          fulfillmentStatus: 'PARTIALLY_RECEIVED',
        },
      ],
      summary: {
        totalPoTracked: 1,
        totalItemsTracked: 1,
        totalAdvanceCapitalLocked: 500.0,
        totalPendingQuantity: 70.0,
        totalAdvancePendingQuantity: 10.0,
        delayedItemsCount: 0,
      },
    }),
    recordMaterialReceipt: vi.fn(),
    generatePaymentAnnexure: vi.fn(),
  },
}));

describe('Purchase Orders, Payments & Advance Tracking Module', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders PurchaseOrdersPage with PO listing and advance terms indicators', async () => {
    render(
      <AuthProvider>
        <MemoryRouter>
          <PurchaseOrdersPage />
        </MemoryRouter>
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByText('Purchase Orders & Advance Terms')).toBeInTheDocument();
      expect(screen.getByText('PO-2026-001')).toBeInTheDocument();
      expect(screen.getByText('Precision Engineering Ltd')).toBeInTheDocument();
      expect(screen.getAllByText('40-40-20').length).toBeGreaterThan(0);
    });
  });

  it('renders PaymentsPage with vouchers and Process Payment trigger', async () => {
    render(
      <AuthProvider>
        <MemoryRouter>
          <PaymentsPage />
        </MemoryRouter>
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByText('Payments & Annexure Generator')).toBeInTheDocument();
      expect(screen.getByText('PAY-2026-0001')).toBeInTheDocument();
      expect(screen.getByText('PO ADVANCE')).toBeInTheDocument();
    });
  });

  it('renders AdvanceReconciliationPage with variance highlighting and material tracking', async () => {
    render(
      <AuthProvider>
        <MemoryRouter>
          <AdvanceReconciliationPage />
        </MemoryRouter>
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByText('Advance Reconciliation & Material Tracker')).toBeInTheDocument();
      expect(screen.getByText('PO-2026-001')).toBeInTheDocument();
      expect(screen.getByText(/Balance Due: -USD 800.00/)).toBeInTheDocument();
    });
  });
});
