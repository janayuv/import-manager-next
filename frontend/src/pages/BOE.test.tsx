import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { BOEEntryPage } from '@/pages/BOEEntryPage';
import { BOERegistryPage } from '@/pages/BOERegistryPage';
import { boeEntryService, boeService, boeSummaryService } from '@/services/boe-service';
import { securityService } from '@/services/security-service';
import { shipmentService } from '@/services/shipment-invoice-service';

vi.mock('@/services/security-service', () => ({
  securityService: {
    getAuthSetupStatus: vi.fn().mockResolvedValue({
      needsSetup: false,
      adminUsername: 'admin',
      recoveryMode: false,
    }),
    getMySession: vi.fn().mockResolvedValue({
      sessionId: 's1',
      userId: 'admin',
      username: 'admin',
      role: 'administrator',
      permissions: ['data.read', 'data.write', 'data.delete', 'boe.view', 'boe.create'],
      rememberMe: false,
      createdAt: '',
      lastActive: '',
      expiresAt: '',
    }),
    logout: vi.fn(),
  },
}));

vi.mock('@/services/boe-service', () => ({
  boeService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    importCsv: vi.fn(),
  },
  boeSummaryService: {
    list: vi.fn(),
    updateStatus: vi.fn(),
  },
  boeEntryService: {
    loadShipmentContext: vi.fn(),
    previewCalculation: vi.fn(),
    parseRateOverrideCsv: vi.fn(),
    saveCalculation: vi.fn(),
    listAttachments: vi.fn(),
    uploadAttachment: vi.fn(),
    deleteAttachment: vi.fn(),
  },
}));

vi.mock('@/services/shipment-invoice-service', () => ({
  shipmentService: {
    list: vi.fn(),
  },
}));

describe('Stage 6: BOE Registry & Duty Calculation Engine', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('switches between Duty Calculations and BE Challan Registry tabs in BOERegistryPage', async () => {
    vi.mocked(boeService.list).mockResolvedValue([
      {
        id: 'boe-1',
        beNumber: 'BE-2026-001',
        beDate: '2026-04-10',
        location: 'INMAA4',
        totalAssessmentValue: 100000,
        dutyAmount: 29800,
        paymentDate: '2026-04-11',
        dutyPaid: 29800,
        challanNumber: 'CH-7788',
        refId: '',
        transactionId: '',
      },
    ]);
    vi.mocked(boeSummaryService.list).mockResolvedValue([
      {
        id: 'calc-1',
        shipmentId: 'sh-1',
        boeId: 'boe-1',
        beNumber: 'BE-2026-001',
        beDate: '2026-04-10',
        supplierName: 'Bosch GmbH',
        invoiceNumber: 'INV-100',
        status: 'Reconciled',
        customsDutyTotal: 29800,
        dutyPaid: 29800,
        assessableTotal: 100000,
        createdAt: '2026-04-12T10:00:00Z',
      },
    ]);

    render(
      <MemoryRouter>
        <AuthProvider>
          <BOERegistryPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('INV-100')).toBeInTheDocument());
    expect(screen.getByText('Bosch GmbH')).toBeInTheDocument();

    const registryTab = screen.getByRole('tab', { name: /BE Challan Registry/i });
    fireEvent.mouseDown(registryTab);
    fireEvent.click(registryTab);

    await waitFor(() => expect(screen.getByText('CH-7788')).toBeInTheDocument());
    expect(screen.getByText('INMAA4')).toBeInTheDocument();
  });

  it('shows CIF Zero-Locked state and disables Save when BCD override or dutyPaid mismatch blocks saving', async () => {
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'sh-1',
          supplierId: 'sup-1',
          supplierName: 'Siemens AG',
          invoiceNumber: 'INV-CIF-1',
          invoiceDate: '2026-04-15',
          goodsCategory: 'Electronics',
          invoiceValue: 1000,
          invoiceCurrency: 'USD',
          incoterm: 'CIF',
          shipmentMode: 'Air',
          shipmentType: 'Dutiable',
          blAwbNumber: 'AWB1',
          blAwbDate: '2026-04-15',
          vesselName: '',
          containerNumber: '',
          grossWeightKg: 10,
          etd: '',
          eta: '',
          status: 'Documents Pending',
          dateOfDelivery: '',
          isFrozen: false,
          createdAt: '',
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 500,
    });

    vi.mocked(boeService.list).mockResolvedValue([
      {
        id: 'boe-1',
        beNumber: 'BE-99',
        beDate: '2026-04-16',
        location: 'INMAA4',
        totalAssessmentValue: 85000,
        dutyAmount: 25000,
        paymentDate: '2026-04-16',
        dutyPaid: 10000,
        challanNumber: 'CH-99',
        refId: '',
        transactionId: '',
      },
    ]);

    vi.mocked(boeEntryService.loadShipmentContext).mockResolvedValue({
      shipment: {
        id: 'sh-1',
        supplierId: 'sup-1',
        supplierName: 'Siemens AG',
        invoiceNumber: 'INV-CIF-1',
        invoiceDate: '2026-04-15',
        goodsCategory: 'Electronics',
        invoiceValue: 1000,
        invoiceCurrency: 'USD',
        incoterm: 'CIF',
        shipmentMode: 'Air',
        shipmentType: 'Dutiable',
        blAwbNumber: 'AWB1',
        blAwbDate: '2026-04-15',
        vesselName: '',
        containerNumber: '',
        grossWeightKg: 10,
        etd: '',
        eta: '',
        status: 'Documents Pending',
        dateOfDelivery: '',
        isFrozen: false,
        createdAt: '',
      },
      params: {
        shipmentId: 'sh-1',
        boeId: 'boe-1',
        supplierName: 'Siemens AG',
        invoiceNumber: 'INV-CIF-1',
        invoiceCurrency: 'USD',
        incoterm: 'CIF',
        status: 'Awaiting BOE Data',
        exchangeRate: 85,
        freightCost: 0,
        exwCost: 0,
        insuranceRate: 0,
        interest: 0,
        lineItems: [],
      },
      result: {
        items: [],
        totalInvoiceValue: 1000,
        totalValueInr: 85000,
        totalFreight: 0,
        totalExw: 0,
        totalInsurance: 0,
        totalAssessableValue: 85000,
        bcdTotal: 6375,
        swsTotal: 637.5,
        igstTotal: 16562.25,
        interest: 0,
        customsDutyTotal: 23574.75,
        hasBlockingBcdOverride: true,
        cifZeroLocked: true,
        payloadBytes: 512,
        payloadWarning: false,
      },
      hasSaved: true,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <BOEEntryPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('CIF Zero-Locked')).toBeInTheDocument());

    expect(screen.getByLabelText('Freight (INR)')).toBeDisabled();
    expect(screen.getByLabelText('EXW (INR)')).toBeDisabled();
    expect(screen.getByLabelText('Insurance (%)')).toBeDisabled();

    expect(
      screen.getByText(/Save blocked: One or more line items have BOE BCD rate higher than Item Master Actual BCD rate/i)
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Save blocked: Linked BOE duty paid \(INR 10000\.00\) does not match computed Customs Duty Total \(INR 23574\.75\)/i)
    ).toBeInTheDocument();

    expect(screen.getByRole('button', { name: /Save Calculation/i })).toBeDisabled();
    expect(securityService.getMySession).toHaveBeenCalled();
  });
});
