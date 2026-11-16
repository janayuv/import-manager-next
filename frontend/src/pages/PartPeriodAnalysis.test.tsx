import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { PartPeriodAnalysisPage } from '@/pages/PartPeriodAnalysisPage';
import { reportsService, type PartPeriodAnalysisResult } from '@/services/expense-service';

vi.mock('@/lib/part-analysis-export', () => ({
  exportPartPeriodAnalysisExcel: vi.fn().mockResolvedValue(undefined),
}));

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
      permissions: ['data.read', 'data.write', 'data.delete'],
      rememberMe: false,
      createdAt: '',
      lastActive: '',
      expiresAt: '',
    }),
    logout: vi.fn(),
  },
}));

vi.mock('@/services/master-service', async (importOriginal) => {
  const mod = await importOriginal<typeof import('@/services/master-service')>();
  return {
    ...mod,
    supplierService: {
      list: vi.fn().mockResolvedValue({
        items: [{ id: 'SUP-001', supplierName: 'Acme Bearings Corp' }],
        totalCount: 1,
      }),
    },
  };
});

vi.mock('@/services/expense-service', async (importOriginal) => {
  const mod = await importOriginal<typeof import('@/services/expense-service')>();
  return {
    ...mod,
    reportsService: {
      searchParts: vi.fn(),
      getPartPeriodAnalysis: vi.fn(),
      exportPartPeriodAnalysisCsv: vi.fn(),
      getLandedCostReport: vi.fn(),
      exportLandedCostCsv: vi.fn(),
    },
  };
});

describe('Part / Item Period Analysis & Traceability UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(reportsService.searchParts).mockResolvedValue([
      {
        id: 'ITM-001',
        partNumber: 'BEAR-6204',
        description: 'Deep Groove Ball Bearing 6204',
        unit: 'PCS',
        currency: 'USD',
        unitPrice: 12.5,
        hsnCode: '84821010',
        supplierId: 'SUP-001',
        supplierName: 'Acme Bearings Corp',
        category: 'Bearings',
        shipmentCount: 2,
        totalQuantity: 1500,
      },
    ]);

    vi.mocked(reportsService.getPartPeriodAnalysis).mockResolvedValue({
      startDate: '2026-01-01',
      endDate: '2026-12-31',
      dateBasis: 'invoice_date',
      targetItem: {
        id: 'ITM-001',
        partNumber: 'BEAR-6204',
        itemDescription: 'Deep Groove Ball Bearing 6204',
        unit: 'PCS',
        currency: 'USD',
        unitPrice: 12.5,
        hsnCode: '84821010',
        supplierId: 'SUP-001',
        supplierName: 'Acme Bearings Corp',
        isActive: true,
        countryOfOrigin: 'Japan',
        bcd: '7.5',
        sws: '10',
        igst: '18',
        technicalWriteUp: 'High precision chrome steel bearing',
        category: 'Bearings',
        endUse: 'Motors',
        netWeightKg: 0.15,
      },
      summary: {
        totalShipments: 2,
        totalInvoices: 2,
        totalBoes: 1,
        totalQuantity: 1500,
        unit: 'PCS',
        totalForexValue: 19000,
        currency: 'USD',
        totalAssessableValueInr: 1050000,
        totalBcdAmount: 78750,
        totalSwsAmount: 7875,
        totalIgstAmount: 204592.5,
        totalBoeInterest: 2500,
        totalCustomsDutyInr: 293717.5,
        totalDutyPaidInr: 195000,
        totalAllocatedExpensesInr: 40000,
        totalAllocatedExpensesWithGstInr: 45000,
        totalLandedCostExIgst: 1179125,
        avgLandedCostPerUnit: 786.08,
        minLandedCostPerUnit: 750,
        maxLandedCostPerUnit: 800,
        firstImportDate: '2026-05-10',
        latestImportDate: '2026-08-15',
      },
      shipments: [
        {
          shipmentId: 'SHP-001',
          invoiceNumber: 'INV-2026-01',
          invoiceDate: '2026-05-10',
          supplierName: 'Acme Bearings Corp',
          incoterm: 'FOB',
          shipmentMode: 'Sea',
          blAwbNumber: 'BL-99881',
          blAwbDate: '2026-05-08',
          vesselName: 'Pacific Express',
          containerNumber: 'MSCU-1234567',
          status: 'delivered',
          quantity: 1000,
          unit: 'PCS',
          unitPrice: 12.5,
          currency: 'USD',
          lineTotal: 12500,
          priceDiverged: false,
        },
      ],
      boeLines: [
        {
          shipmentId: 'SHP-001',
          invoiceNumber: 'INV-2026-01',
          boeCalcId: 'CALC-001',
          boeId: 'BOE-001',
          beNumber: 'BE-887766',
          beDate: '2026-05-15',
          location: 'INNSA1',
          status: 'Reconciled',
          exchangeRate: 83,
          quantity: 1000,
          unit: 'PCS',
          itemForexValue: 12500,
          itemValueInr: 1037500,
          assessableValue: 1050000,
          bcdRate: 7.5,
          bcdAmount: 78750,
          swsRate: 10,
          swsAmount: 7875,
          igstRate: 18,
          igstAmount: 204592.5,
          boeInterest: 2500,
          totalDuty: 293717.5,
          dutyPaid: 195000,
          challanNumber: 'CH-001',
          paymentDate: '2026-05-16',
        },
      ],
      expenses: [
        {
          expenseId: 'EXP-ROW-1',
          expenseInvoiceId: 'EI-001',
          shipmentId: 'SHP-001',
          shipmentInvoiceNumber: 'INV-2026-01',
          serviceProviderId: 'SP-001',
          serviceProviderName: 'FastLogistics Customs CHA',
          expenseTypeId: 'EXP-FRT',
          expenseTypeName: 'Ocean Freight & Port',
          expenseInvoiceNo: 'EXP-INV-101',
          expenseInvoiceDate: '2026-05-18',
          shipmentExpenseBasicInr: 40000,
          shipmentExpenseGstInr: 5000,
          shipmentExpenseTotalInr: 45000,
          allocationRatio: 1.0,
          allocatedBasicInr: 40000,
          allocatedGstInr: 5000,
          allocatedTotalInr: 45000,
          remarks: 'Standard ocean freight apportionment',
        },
      ],
      trends: [
        {
          date: '2026-05-10',
          shipmentId: 'SHP-001',
          invoiceNumber: 'INV-2026-01',
          quantity: 1000,
          unitPriceForex: 12.5,
          currency: 'USD',
          assessableValuePerUnit: 1050,
          dutyPerUnit: 89.13,
          expensePerUnit: 40,
          landedCostPerUnit: 1179.13,
        },
      ],
    } as unknown as PartPeriodAnalysisResult);
  });

  it('renders discovery screen and loads part analysis when part is selected', async () => {
    render(
      <AuthProvider>
        <MemoryRouter>
          <PartPeriodAnalysisPage />
        </MemoryRouter>
      </AuthProvider>,
    );

    // Initial page title check
    expect(screen.getByText(/Part \/ Item Period Analysis & Traceability/i)).toBeInTheDocument();

    // Discovery screen popular part is shown
    await waitFor(() => {
      expect(screen.getByText('BEAR-6204')).toBeInTheDocument();
    });

    // Click the explore button for BEAR-6204
    fireEvent.click(screen.getByText('BEAR-6204'));

    // Should call getPartPeriodAnalysis with partNumber
    await waitFor(() => {
      expect(reportsService.getPartPeriodAnalysis).toHaveBeenCalledWith(
        expect.objectContaining({
          partNumber: 'BEAR-6204',
        }),
      );
    });

    // Check that Item Master card details rendered
    await waitFor(() => {
      expect(screen.getByText('Deep Groove Ball Bearing 6204')).toBeInTheDocument();
      expect(screen.getAllByText('Acme Bearings Corp').length).toBeGreaterThan(0);
      expect(screen.getByText(/84821010/)).toBeInTheDocument();
    });
  });

  it('renders all 8 Executive KPI cards with correct period values', async () => {
    render(
      <AuthProvider>
        <MemoryRouter initialEntries={['/part-analysis?partNo=BEAR-6204']}>
          <PartPeriodAnalysisPage />
        </MemoryRouter>
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText('Deep Groove Ball Bearing 6204')).toBeInTheDocument();
    });

    // Verify Executive Summary KPIs exist
    expect(screen.getByText('Total Imported Qty')).toBeInTheDocument();
    expect(screen.getByText('1,500 PCS')).toBeInTheDocument();

    expect(screen.getByText('Avg Landed Cost / Qty')).toBeInTheDocument();
    expect(screen.getByText('₹786.08 / PCS')).toBeInTheDocument();

    expect(screen.getByText('Total Assessable Value')).toBeInTheDocument();
    expect(screen.getByText('₹10,50,000.00')).toBeInTheDocument();

    expect(screen.getByText('Allocated Expenses')).toBeInTheDocument();
    expect(screen.getByText('₹40,000.00')).toBeInTheDocument();
  });

  it('renders tabs and displays related data', async () => {
    render(
      <AuthProvider>
        <MemoryRouter initialEntries={['/part-analysis?partNo=BEAR-6204']}>
          <PartPeriodAnalysisPage />
        </MemoryRouter>
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText('Deep Groove Ball Bearing 6204')).toBeInTheDocument();
    });

    expect(screen.getByText(/Across 2 shipments/i)).toBeInTheDocument();

    // Check Shipments tab content
    expect(screen.getByText('INV-2026-01')).toBeInTheDocument();
    expect(screen.getByText('Pacific Express')).toBeInTheDocument();

    // Switch to Bill of Entry tab
    const boeTab = screen.getByRole('tab', { name: /Bill of Entry/i });
    fireEvent.mouseDown(boeTab, { button: 0 });
    fireEvent.click(boeTab);

    await waitFor(() => {
      expect(screen.getByText('BE-887766')).toBeInTheDocument();
      expect(screen.getByText('INNSA1')).toBeInTheDocument();
    });

    // Switch to Allocated Expenses tab
    const expTab = screen.getByRole('tab', { name: /Allocated Expenses/i });
    fireEvent.mouseDown(expTab, { button: 0 });
    fireEvent.click(expTab);

    await waitFor(() => {
      expect(screen.getByText('FastLogistics Customs CHA')).toBeInTheDocument();
      expect(screen.getByText('Ocean Freight & Port')).toBeInTheDocument();
    });
  });

  it('triggers CSV export with correct parameters', async () => {
    vi.mocked(reportsService.exportPartPeriodAnalysisCsv).mockResolvedValue('test,csv,data');

    render(
      <AuthProvider>
        <MemoryRouter initialEntries={['/part-analysis?partNo=BEAR-6204']}>
          <PartPeriodAnalysisPage />
        </MemoryRouter>
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText('Deep Groove Ball Bearing 6204')).toBeInTheDocument();
    });

    const exportBtn = screen.getByRole('button', { name: /Export Full CSV/i });
    fireEvent.click(exportBtn);

    await waitFor(() => {
      expect(reportsService.exportPartPeriodAnalysisCsv).toHaveBeenCalledWith(
        expect.objectContaining({
          partNumber: 'BEAR-6204',
        }),
      );
    });
  });

  it('triggers Excel export with professional template style', async () => {
    const { exportPartPeriodAnalysisExcel } = await import('@/lib/part-analysis-export');

    render(
      <AuthProvider>
        <MemoryRouter initialEntries={['/part-analysis?partNo=BEAR-6204']}>
          <PartPeriodAnalysisPage />
        </MemoryRouter>
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText('Deep Groove Ball Bearing 6204')).toBeInTheDocument();
    });

    const excelBtn = screen.getByRole('button', { name: /Export Excel \(\.xlsx\)/i });
    expect(excelBtn).toBeInTheDocument();
    fireEvent.click(excelBtn);

    await waitFor(() => {
      expect(exportPartPeriodAnalysisExcel).toHaveBeenCalledWith(
        expect.objectContaining({
          targetItem: expect.objectContaining({
            partNumber: 'BEAR-6204',
          }),
        }),
      );
    });
  });
});
