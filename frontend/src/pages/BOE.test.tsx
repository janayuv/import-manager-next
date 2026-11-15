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
    vi.mocked(boeSummaryService.list).mockResolvedValue([]);

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
        swsTotal: 638,
        igstTotal: 16562,
        interest: 0,
        customsDutyTotal: 23575,
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
      screen.getByText(/Save blocked: Linked BOE duty paid \(INR 10,000\) does not match computed Customs Duty Total \(INR 23,575\)/i)
    ).toBeInTheDocument();

    expect(screen.getByRole('button', { name: /Save Calculation/i })).toBeDisabled();
    expect(securityService.getMySession).toHaveBeenCalled();
  });

  it('excludes finalized shipments and linked BOEs from BOE Duty Calculation Engine dropdowns', async () => {
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'sh-pending',
          supplierId: 'sup-1',
          supplierName: 'Pending Supplier',
          invoiceNumber: 'INV-PENDING',
          invoiceDate: '2026-04-18',
          goodsCategory: 'Parts',
          invoiceValue: 500,
          invoiceCurrency: 'USD',
          incoterm: 'FOB',
          shipmentMode: 'Air',
          shipmentType: 'Dutiable',
          blAwbNumber: '',
          blAwbDate: '',
          vesselName: '',
          containerNumber: '',
          grossWeightKg: 5,
          etd: '',
          eta: '',
          status: 'in-transit',
          dateOfDelivery: '',
          isFrozen: false,
          createdAt: '',
        },
        {
          id: 'sh-finalized',
          supplierId: 'sup-2',
          supplierName: 'Finalized Supplier',
          invoiceNumber: 'INV-FINALIZED',
          invoiceDate: '2026-04-10',
          goodsCategory: 'Parts',
          invoiceValue: 1200,
          invoiceCurrency: 'USD',
          incoterm: 'CIF',
          shipmentMode: 'Sea',
          shipmentType: 'Dutiable',
          blAwbNumber: '',
          blAwbDate: '',
          vesselName: '',
          containerNumber: '',
          grossWeightKg: 10,
          etd: '',
          eta: '',
          status: 'customs-clearance',
          dateOfDelivery: '',
          isFrozen: false,
          createdAt: '',
        },
      ],
      totalCount: 2,
      page: 1,
      pageSize: 500,
    });

    vi.mocked(boeService.list).mockResolvedValue([
      {
        id: 'boe-available',
        beNumber: 'BE-AVAIL-1',
        beDate: '2026-04-18',
        location: 'INMAA4',
        totalAssessmentValue: 50000,
        dutyAmount: 15000,
        paymentDate: '2026-04-18',
        dutyPaid: 15000,
        challanNumber: 'CH-AVAIL',
        refId: '',
        transactionId: '',
      },
      {
        id: 'boe-linked-final',
        beNumber: 'BE-FINAL-1',
        beDate: '2026-04-10',
        location: 'INMAA4',
        totalAssessmentValue: 100000,
        dutyAmount: 32460,
        paymentDate: '2026-04-10',
        dutyPaid: 32460,
        challanNumber: 'CH-FINAL',
        refId: '',
        transactionId: '',
      },
    ]);

    vi.mocked(boeSummaryService.list).mockResolvedValue([
      {
        id: 'calc-fin',
        shipmentId: 'sh-finalized',
        boeId: 'boe-linked-final',
        beNumber: 'BE-FINAL-1',
        beDate: '2026-04-10',
        supplierName: 'Finalized Supplier',
        invoiceNumber: 'INV-FINALIZED',
        status: 'Reconciled',
        customsDutyTotal: 32460,
        dutyPaid: 32460,
        assessableTotal: 100000,
        createdAt: '2026-04-10T10:00:00Z',
      },
    ]);

    vi.mocked(boeEntryService.loadShipmentContext).mockResolvedValue({
      shipment: {
        id: 'sh-pending',
        supplierId: 'sup-1',
        supplierName: 'Pending Supplier',
        invoiceNumber: 'INV-PENDING',
        invoiceDate: '2026-04-18',
        goodsCategory: 'Parts',
        invoiceValue: 500,
        invoiceCurrency: 'USD',
        incoterm: 'FOB',
        shipmentMode: 'Air',
        shipmentType: 'Dutiable',
        blAwbNumber: '',
        blAwbDate: '',
        vesselName: '',
        containerNumber: '',
        grossWeightKg: 5,
        etd: '',
        eta: '',
        status: 'in-transit',
        dateOfDelivery: '',
        isFrozen: false,
        createdAt: '',
      },
      params: {
        shipmentId: 'sh-pending',
        boeId: '',
        supplierName: 'Pending Supplier',
        invoiceNumber: 'INV-PENDING',
        invoiceCurrency: 'USD',
        incoterm: 'FOB',
        status: 'Awaiting BOE Data',
        exchangeRate: 83.5,
        freightCost: 0,
        exwCost: 0,
        insuranceRate: 0.015,
        interest: 0,
        lineItems: [
          {
            itemId: 'ITM-1',
            partNumber: 'PN-101',
            description: 'Item 1',
            hsnCode: '8471',
            unit: 'PCS',
            quantity: 1,
            unitPrice: 104776,
            actualBcdRate: 10,
            actualSwsRate: 10,
            actualIgstRate: 18,
            calculationMethod: 'Standard',
            boeBcdRate: 10,
            boeSwsRate: 10,
            boeIgstRate: 18,
          },
        ],
      },
      result: {
        items: [
          {
            itemId: 'ITM-1',
            partNumber: 'PN-101',
            description: 'Item 1',
            hsnCode: '8471',
            unit: 'PCS',
            quantity: 1,
            unitPrice: 104776,
            itemForexValue: 104776,
            itemValueInr: 104776,
            itemFreight: 0,
            itemExw: 0,
            itemInsurance: 0,
            assessableValue: 104776,
            calculationMethod: 'Standard',
            actualBcdRate: 10,
            actualSwsRate: 10,
            actualIgstRate: 18,
            boeBcdRate: 10,
            boeSwsRate: 10,
            boeIgstRate: 18,
            bcdValue: 10477.6,
            swsValue: 1047.8,
            igstValue: 20934.3,
            totalLineDuty: 32459.7,
            bcdDiscrepancy: false,
          },
        ],
        totalInvoiceValue: 104776,
        totalValueInr: 104776,
        totalFreight: 0,
        totalExw: 0,
        totalInsurance: 0,
        totalAssessableValue: 104776,
        bcdTotal: 10478,
        swsTotal: 1048,
        igstTotal: 20934,
        interest: 0,
        customsDutyTotal: 32460,
        hasBlockingBcdOverride: false,
        cifZeroLocked: false,
        payloadBytes: 256,
        payloadWarning: false,
      },
      hasSaved: false,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <BOEEntryPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText(/INV-PENDING/i)).toBeInTheDocument());
    expect(screen.queryByText(/INV-FINALIZED/i)).not.toBeInTheDocument();

    // Verify 1 decimal place for line items and 0 decimal places for totals
    expect(screen.getByText('10477.6 / 1047.8 / 20934.3')).toBeInTheDocument();
    expect(screen.getByText('10,478')).toBeInTheDocument();
    expect(screen.getByText('1,048')).toBeInTheDocument();
    expect(screen.getByText('20,934')).toBeInTheDocument();
    expect(screen.getByText('INR 32,460')).toBeInTheDocument();
  });
});
