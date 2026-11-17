import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { ReconciledBOEPage } from '@/pages/ReconciledBOEPage';
import {
  boeEntryService,
  boeService,
  boeSummaryService,
  type BOECalculationDetail,
  type BOECalculationSummary,
} from '@/services/boe-service';

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
      permissions: ['data.read', 'data.write', 'data.delete', 'boe.view'],
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
  },
  boeSummaryService: {
    list: vi.fn(),
    getById: vi.fn(),
    updateStatus: vi.fn(),
    delete: vi.fn(),
  },
  boeEntryService: {
    loadShipmentContext: vi.fn(),
    loadCalculationContext: vi.fn(),
    previewCalculation: vi.fn(),
    saveCalculation: vi.fn(),
  },
}));

const mockSummaries: BOECalculationSummary[] = [
  {
    id: 'calc-rec-1',
    shipmentId: 'SHP-2026-001',
    boeId: 'boe-101',
    beNumber: '3965278',
    beDate: '2026-09-23',
    supplierName: 'Sony Corp Japan',
    invoiceNumber: 'INV-SONY-01',
    status: 'Reconciled',
    customsDutyTotal: 866103,
    dutyPaid: 866103,
    assessableTotal: 5233248,
    createdAt: '2026-09-24 10:00:00',
  },
  {
    id: 'calc-disc-2',
    shipmentId: 'SHP-2026-002',
    boeId: 'boe-102',
    beNumber: '4455667',
    beDate: '2026-09-25',
    supplierName: 'Bosch GmbH',
    invoiceNumber: 'INV-BOSCH-99',
    status: 'Discrepancy Found',
    customsDutyTotal: 120000,
    dutyPaid: 110000,
    assessableTotal: 1000000,
    createdAt: '2026-09-26 14:00:00',
  },
];

const mockDetail: BOECalculationDetail = {
  id: 'calc-rec-1',
  shipmentId: 'SHP-2026-001',
  boeId: 'boe-101',
  beNumber: '3965278',
  beDate: '2026-09-23',
  location: 'INMAA1',
  challanNumber: 'CH-2063325822',
  paymentDate: '2026-09-24',
  dutyPaid: 866103,
  refId: '007000BEINMAA1',
  transactionId: 'TX-998877',
  supplierName: 'Sony Corp Japan',
  invoiceNumber: 'INV-SONY-01',
  status: 'Reconciled',
  createdAt: '2026-09-24 10:00:00',
  params: {
    id: 'calc-rec-1',
    shipmentId: 'SHP-2026-001',
    boeId: 'boe-101',
    supplierName: 'Sony Corp Japan',
    invoiceNumber: 'INV-SONY-01',
    invoiceCurrency: 'USD',
    incoterm: 'CIF',
    status: 'Reconciled',
    exchangeRate: 85.0,
    freightCost: 0,
    insuranceRate: 1.125,
    insuranceMode: 'deemed_1.125',
    lineItems: [
      {
        itemId: 'itm-1',
        partNumber: 'SNY-CMOS-01',
        description: 'CMOS Image Sensor Module',
        hsnCode: '85299090',
        unit: 'PCS',
        quantity: 1000,
        unitPrice: 61.5,
        actualBcdRate: 7.5,
        actualSwsRate: 10.0,
        actualIgstRate: 18.0,
        calculationMethod: 'Standard',
        boeBcdRate: 7.5,
        boeSwsRate: 10.0,
        boeIgstRate: 18.0,
      },
    ],
  } as unknown as BOECalculationDetail['params'],
  result: {
    totalInvoiceValue: 61500,
    totalValueInr: 5227500,
    totalFreight: 0,
    totalExw: 0,
    totalInsurance: 5748,
    totalOriginCharges: 0,
    totalAssessableValue: 5233248,
    grossBcdTotal: 392494,
    rodtepScripUtilized: 0,
    bcdCashTotal: 392494,
    bcdTotal: 392494,
    swsTotal: 39249,
    igstTotal: 434360,
    interest: 0,
    customsDutyTotal: 866103,
    cashCustomsDutyTotal: 866103,
    hasBlockingBcdOverride: false,
    cifZeroLocked: true,
    incotermRule: {
      code: 'CIF',
      name: 'Cost, Insurance and Freight',
      freightIncluded: true,
      insuranceIncluded: true,
      originChargesExpected: false,
      description: '',
    },
    payloadBytes: 1024,
    payloadWarning: false,
    items: [
      {
        itemId: 'itm-1',
        partNumber: 'SNY-CMOS-01',
        description: 'CMOS Image Sensor Module',
        hsnCode: '85299090',
        unit: 'PCS',
        quantity: 1000,
        unitPrice: 61.5,
        itemForexValue: 61500,
        itemValueInr: 5227500,
        itemFreight: 0,
        itemExw: 0,
        itemInsurance: 5748,
        itemOriginCharges: 0,
        assessableValue: 5233248,
        calculationMethod: 'Standard',
        actualBcdRate: 7.5,
        actualSwsRate: 10.0,
        actualIgstRate: 18.0,
        boeBcdRate: 7.5,
        boeSwsRate: 10.0,
        boeIgstRate: 18.0,
        grossBcd: 392494,
        rodtepUtilized: 0,
        bcdCashPayable: 392494,
        bcdValue: 392494,
        swsValue: 39249,
        igstValue: 434360,
        totalLineDuty: 866103,
        totalLineCashDuty: 866103,
        bcdDiscrepancy: false,
      },
    ],
  },
  attachments: [],
  reconciliation: {
    computedDuty: 866103,
    cashDuty: 866103,
    dutyPaid: 866103,
    dutyDifference: 0,
    isReconciled: true,
    assessableValue: 5233248,
    challanAssessableValue: 5233248,
    assessableDifference: 0,
  },
};

describe('Reconciled BOE Module', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(boeSummaryService.list).mockResolvedValue(mockSummaries);
    vi.mocked(boeSummaryService.getById).mockResolvedValue(mockDetail);
    vi.mocked(boeService.list).mockResolvedValue([
      {
        id: 'boe-101',
        beNumber: '3965278',
        beDate: '2026-09-23',
        location: 'INMAA1',
        totalAssessmentValue: 5233248,
        dutyAmount: 866103,
        paymentDate: '2026-09-24',
        dutyPaid: 866103,
        challanNumber: 'CH-2063325822',
        refId: '',
        transactionId: '',
      },
    ]);
  });

  it('renders module page with KPI summary cards and table in Reconciled Only view', async () => {
    render(
      <AuthProvider>
        <MemoryRouter>
          <ReconciledBOEPage />
        </MemoryRouter>
      </AuthProvider>
    );

    // Module Title
    expect(await screen.findByRole('heading', { name: /Reconciled Bill of Entry \(BOE\)/i })).toBeInTheDocument();

    // Verify KPI Card for Reconciled BOEs
    expect(screen.getByText('Reconciled BOEs')).toBeInTheDocument();

    // Reconciled row present
    expect(await screen.findByText('INV-SONY-01')).toBeInTheDocument();
    expect(screen.getByText('3965278')).toBeInTheDocument();
    expect(screen.getByText('Sony Corp Japan')).toBeInTheDocument();

    // Discrepancy row should NOT be present in Reconciled Only tab
    expect(screen.queryByText('INV-BOSCH-99')).not.toBeInTheDocument();

    // Switch to All Calculations tab
    const allTab = screen.getByRole('tab', { name: /All Calculations/i });
    fireEvent.mouseDown(allTab, { button: 0 });
    fireEvent.click(allTab);

    // Now Discrepancy row should be visible
    expect(await screen.findByText('INV-BOSCH-99')).toBeInTheDocument();
  });

  it('opens View modal and displays comprehensive reconciled BOE dossier with line items', async () => {
    render(
      <AuthProvider>
        <MemoryRouter>
          <ReconciledBOEPage />
        </MemoryRouter>
      </AuthProvider>
    );

    await screen.findByText('INV-SONY-01');

    // Click View Button
    const viewBtn = screen.getByTitle('View Reconciled BOE Dossier');
    fireEvent.click(viewBtn);

    // Verify service call
    expect(boeSummaryService.getById).toHaveBeenCalledWith('calc-rec-1');

    // Dialog appears
    expect(await screen.findByText(/Reconciled BOE Dossier · INV-SONY-01/i)).toBeInTheDocument();
    expect(screen.getByText('100% Financial Reconciliation Verified')).toBeInTheDocument();
    expect(screen.getByText('CMOS Image Sensor Module')).toBeInTheDocument();
    expect(screen.getByText('CH-2063325822')).toBeInTheDocument();

    // Verify search filter in ViewReconciledBoeDialog
    const searchInput = screen.getByPlaceholderText('Search part no, description...');
    expect(searchInput).toBeInTheDocument();
    fireEvent.change(searchInput, { target: { value: 'sensor' } });
    expect(screen.getByText('CMOS Image Sensor Module')).toBeInTheDocument();
    fireEvent.change(searchInput, { target: { value: 'no-match-part' } });
    expect(screen.getByText(/No line items matching “no-match-part”/i)).toBeInTheDocument();
    expect(screen.queryByText('CMOS Image Sensor Module')).not.toBeInTheDocument();
    const clearBtn = screen.getByLabelText('Clear search');
    fireEvent.click(clearBtn);
    expect(screen.getByText('CMOS Image Sensor Module')).toBeInTheDocument();

    // Close Dialog
    const closeButtons = screen.getAllByRole('button', { name: /Close/i });
    const lastCloseBtn = closeButtons[closeButtons.length - 1];
    if (lastCloseBtn) {
      fireEvent.click(lastCloseBtn);
    }
    await waitFor(() => {
      expect(screen.queryByText(/Reconciled BOE Dossier · INV-SONY-01/i)).not.toBeInTheDocument();
    });
  });

  it('opens Edit modal, updates valuation fields, and saves updated calculation', async () => {
    vi.mocked(boeEntryService.previewCalculation).mockResolvedValue(mockDetail.result);
    vi.mocked(boeEntryService.saveCalculation).mockResolvedValue({
      id: 'calc-rec-1',
      shipmentId: 'SHP-2026-001',
      boeId: 'boe-101',
      beNumber: '3965278',
      beDate: '2026-09-23',
      supplierName: 'Sony Corp Japan',
      invoiceNumber: 'INV-SONY-01',
      status: 'Reconciled',
      customsDutyTotal: 866103,
      dutyPaid: 866103,
      assessableTotal: 5233248,
      createdAt: '2026-09-24',
    });

    render(
      <AuthProvider>
        <MemoryRouter>
          <ReconciledBOEPage />
        </MemoryRouter>
      </AuthProvider>
    );

    await screen.findByText('INV-SONY-01');

    // Click Edit Button
    const editBtn = screen.getByTitle('Edit Reconciled BOE');
    fireEvent.click(editBtn);

    // Dialog appears
    expect(await screen.findByText(/Edit Reconciled BOE · INV-SONY-01/i)).toBeInTheDocument();

    // Modify exchange rate
    const exchInput = screen.getByLabelText(/Customs Exchange Rate/i);
    fireEvent.change(exchInput, { target: { value: '86.5' } });

    // Click Save
    const saveBtn = screen.getByRole('button', { name: /Save Reconciled BOE/i });
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(boeEntryService.saveCalculation).toHaveBeenCalled();
    });
  });

  it('opens Delete confirmation dialog and deletes reconciled calculation', async () => {
    vi.mocked(boeSummaryService.delete).mockResolvedValue();

    render(
      <AuthProvider>
        <MemoryRouter>
          <ReconciledBOEPage />
        </MemoryRouter>
      </AuthProvider>
    );

    await screen.findByText('INV-SONY-01');

    // Click Delete Button
    const delBtn = screen.getByTitle('Delete BOE Calculation');
    fireEvent.click(delBtn);

    // Delete dialog appears
    expect(await screen.findByText(/Delete BOE Calculation\?/i)).toBeInTheDocument();
    expect(screen.getByText(/Are you sure you want to delete this reconciled BOE calculation\?/i)).toBeInTheDocument();

    // Confirm deletion
    const confirmBtn = screen.getByRole('button', { name: /Move to Recycle Bin/i });
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(boeSummaryService.delete).toHaveBeenCalledWith('calc-rec-1');
    });
  });
});
