import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { BOEEntryPage } from '@/pages/BOEEntryPage';
import { BOERegistryPage } from '@/pages/BOERegistryPage';
import {
  boeEntryService,
  boeService,
  boeSummaryService,
  type BOECalcLineResult,
  type BOECalcParams,
  type BOECalculationResult,
} from '@/services/boe-service';
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

    // Verify View, Edit, and Delete options exist for calculation rows
    expect(screen.getByTitle('View Reconciled BOE Dossier')).toBeInTheDocument();
    expect(screen.getByTitle('Edit Reconciled BOE')).toBeInTheDocument();
    expect(screen.getByTitle('Delete BOE Calculation')).toBeInTheDocument();

    // Open Delete confirmation
    fireEvent.click(screen.getByTitle('Delete BOE Calculation'));
    expect(await screen.findByText(/Delete BOE Calculation\?/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

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
      } as unknown as BOECalcParams,
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
      } as unknown as BOECalculationResult,
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
      } as unknown as BOECalcParams,
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
          } as unknown as BOECalcLineResult,
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
      } as unknown as BOECalculationResult,
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
    await waitFor(() => expect(screen.getByText('10477.6 / 1047.8 / 20934.3')).toBeInTheDocument());
    expect(screen.getByText('10,478')).toBeInTheDocument();
    expect(screen.getByText('1,048')).toBeInTheDocument();
    expect(screen.getByText('20,934')).toBeInTheDocument();
    expect(screen.getByText('INR 32,460')).toBeInTheDocument();
  });

  it('renders 11 Incoterm valuation rules, statutory insurance basis, and RoDTEP scrip debiting cleanly', async () => {
    vi.mocked(boeSummaryService.list).mockResolvedValue([]);
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'sh-exw',
          supplierId: 'sup-exw',
          supplierName: 'Origin Supplier Co',
          invoiceNumber: 'INV-EXW-01',
          invoiceDate: '2026-04-20',
          goodsCategory: 'Raw Materials',
          invoiceValue: 1000,
          invoiceCurrency: 'USD',
          incoterm: 'EXW',
          shipmentMode: 'Sea',
          shipmentType: 'Dutiable',
          blAwbNumber: '',
          blAwbDate: '',
          vesselName: '',
          containerNumber: '',
          grossWeightKg: 100,
          etd: '',
          eta: '',
          status: 'in-transit',
          dateOfDelivery: '',
          isFrozen: false,
          createdAt: '',
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 500,
    });
    vi.mocked(boeService.list).mockResolvedValue([]);
    vi.mocked(boeEntryService.loadShipmentContext).mockResolvedValue({
      shipment: {
        id: 'sh-exw',
        supplierId: 'sup-exw',
        supplierName: 'Origin Supplier Co',
        invoiceNumber: 'INV-EXW-01',
        invoiceDate: '2026-04-20',
        goodsCategory: 'Raw Materials',
        invoiceValue: 1000,
        invoiceCurrency: 'USD',
        incoterm: 'EXW',
        shipmentMode: 'Sea',
        shipmentType: 'Dutiable',
        blAwbNumber: '',
        blAwbDate: '',
        vesselName: '',
        containerNumber: '',
        grossWeightKg: 100,
        etd: '',
        eta: '',
        status: 'in-transit',
        dateOfDelivery: '',
        isFrozen: false,
        createdAt: '',
      },
      params: {
        shipmentId: 'sh-exw',
        boeId: '',
        supplierName: 'Origin Supplier Co',
        invoiceNumber: 'INV-EXW-01',
        invoiceCurrency: 'USD',
        incoterm: 'EXW',
        status: 'Awaiting BOE Data',
        exchangeRate: 80,
        freightCost: 4000,
        exwCost: 1500,
        originInlandFreight: 1000,
        exportHandling: 500,
        insuranceMode: 'deemed_1.125',
        insuranceRate: 1.125,
        rodtepScripBalance: 5000,
        interest: 0,
        lineItems: [
          {
            itemId: 'itm-1',
            partNumber: 'RM-101',
            description: 'Raw Polymer',
            hsnCode: '3901',
            unit: 'KG',
            quantity: 1000,
            unitPrice: 1,
            actualBcdRate: 10,
            actualSwsRate: 10,
            actualIgstRate: 18,
            calculationMethod: 'Standard',
            boeBcdRate: 10,
            boeSwsRate: 10,
            boeIgstRate: 18,
          },
        ],
      } as unknown as BOECalcParams,
      result: {
        items: [
          {
            itemId: 'itm-1',
            partNumber: 'RM-101',
            description: 'Raw Polymer',
            hsnCode: '3901',
            unit: 'KG',
            quantity: 1000,
            unitPrice: 1,
            itemForexValue: 1000,
            itemValueInr: 80000,
            itemFreight: 4000,
            itemExw: 1500,
            itemInsurance: 916.9,
            itemOriginCharges: 1500,
            assessableValue: 86416.9,
            calculationMethod: 'Standard',
            actualBcdRate: 10,
            actualSwsRate: 10,
            actualIgstRate: 18,
            boeBcdRate: 10,
            boeSwsRate: 10,
            boeIgstRate: 18,
            grossBcd: 8641.7,
            rodtepUtilized: 5000.0,
            bcdCashPayable: 3641.7,
            bcdValue: 3641.7,
            swsValue: 864.2,
            igstValue: 17266.1,
            totalLineDuty: 26772.0,
            totalLineCashDuty: 21772.0,
            bcdDiscrepancy: false,
          } as unknown as BOECalcLineResult,
        ],
        totalInvoiceValue: 1000,
        totalValueInr: 80000,
        totalFreight: 4000,
        totalExw: 1500,
        totalInsurance: 916.9,
        totalOriginCharges: 1500,
        totalAssessableValue: 86416.9,
        grossBcdTotal: 8642,
        rodtepScripUtilized: 5000,
        bcdCashTotal: 3642,
        bcdTotal: 8642,
        swsTotal: 864,
        igstTotal: 17266,
        interest: 0,
        customsDutyTotal: 26772,
        cashCustomsDutyTotal: 21772,
        hasBlockingBcdOverride: false,
        cifZeroLocked: false,
        incotermRule: {
          code: 'EXW',
          name: 'Ex Works',
          freightIncluded: false,
          insuranceIncluded: false,
          originChargesExpected: true,
          description: 'Origin inland transport, export handling, international freight and insurance must be added.',
        },
        payloadBytes: 512,
        payloadWarning: false,
      } as unknown as BOECalculationResult,
      hasSaved: false,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <BOEEntryPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('Incoterm: EXW')).toBeInTheDocument());
    expect(screen.getByLabelText('Freight (INR)')).not.toBeDisabled();
    expect(screen.getByLabelText('EXW (INR)')).not.toBeDisabled();

    // Verify RoDTEP debit card and cash challan total display
    expect(screen.getByText(/RoDTEP Electronic Scrip Debited \(from BCD\)/i)).toBeInTheDocument();
    expect(screen.getByText(/- INR 5,000/i)).toBeInTheDocument();
    expect(screen.getByText(/Net Cash Challan Payable/i)).toBeInTheDocument();
    expect(screen.getByText('INR 21,772')).toBeInTheDocument();

    // Verify gross assessed duty is also visible
    expect(screen.getByText('INR 26,772')).toBeInTheDocument();

    // Verify + Manage Rates button opens insurance rates modal
    expect(screen.getByText('+ Manage Rates')).toBeInTheDocument();
    fireEvent.click(screen.getByText('+ Manage Rates'));
    await waitFor(() => expect(screen.getByText('Manage Insurance Rates')).toBeInTheDocument());
  });
});
