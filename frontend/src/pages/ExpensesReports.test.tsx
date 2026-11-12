import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { ExpenseReportsPage } from '@/pages/ExpenseReportsPage';
import { ExpensesPage } from '@/pages/ExpensesPage';
import { LandedCostReportPage } from '@/pages/LandedCostReportPage';
import {
  expenseReportsService,
  expenseService,
  reportsService,
} from '@/services/expense-service';
import { supplierService } from '@/services/master-service';
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
      permissions: ['data.read', 'data.write', 'data.delete'],
      rememberMe: false,
      createdAt: '',
      lastActive: '',
      expiresAt: '',
    }),
    logout: vi.fn(),
  },
}));

vi.mock('@/services/expense-service', async (importOriginal) => {
  const mod = await importOriginal<typeof import('@/services/expense-service')>();
  return {
    ...mod,
    expenseService: {
      listServiceProviders: vi.fn(),
      createServiceProvider: vi.fn(),
      updateServiceProvider: vi.fn(),
      deleteServiceProvider: vi.fn(),
      listExpenseTypes: vi.fn(),
      createExpenseType: vi.fn(),
      updateExpenseType: vi.fn(),
      deleteExpenseType: vi.fn(),
      previewCalculation: vi.fn(),
      mergeDuplicateLines: vi.fn(),
      listExpenseInvoices: vi.fn(),
      getExpenseInvoice: vi.fn(),
      createExpenseInvoice: vi.fn(),
      updateExpenseInvoice: vi.fn(),
      deleteExpenseInvoice: vi.fn(),
      downloadBulkTemplateCsv: vi.fn(),
      importBulkCsv: vi.fn(),
    },
    expenseReportsService: {
      getReport: vi.fn(),
    },
    reportsService: {
      getLandedCostReport: vi.fn(),
      exportLandedCostCsv: vi.fn(),
    },
  };
});

vi.mock('@/services/shipment-invoice-service', () => ({
  shipmentService: {
    list: vi.fn(),
  },
}));

vi.mock('@/services/master-service', () => ({
  supplierService: {
    list: vi.fn().mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 }),
  },
}));

describe('Stage 7: Import Expenses, 5-View Expense Reports & Landed Cost Report', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(supplierService.list).mockResolvedValue({
      items: [],
      totalCount: 0,
      page: 1,
      pageSize: 500,
    });
  });

  it('renders expense ledger and handles duplicate line merge in multi-line invoice dialog', async () => {
    vi.mocked(expenseService.listExpenseInvoices).mockResolvedValue([
      {
        id: 'EINV-1',
        shipmentId: 'SH-1',
        shipmentInvoiceNo: 'INV-SH-1',
        serviceProviderId: 'SP-1',
        serviceProviderName: 'DHL Express',
        invoiceNo: 'DHL-101',
        invoiceDate: '2026-04-15',
        currency: 'INR',
        baseAmountPaise: 100000,
        totalCgstPaise: 9000,
        totalSgstPaise: 9000,
        totalIgstPaise: 0,
        totalTdsPaise: 2000,
        totalAmountPaise: 118000,
        netAmountPaise: 116000,
        remarks: '',
        idempotencyKey: 'k1',
        version: 1,
        createdAt: '',
        updatedAt: '',
        lines: [],
      },
    ]);
    vi.mocked(expenseService.listServiceProviders).mockResolvedValue([
      {
        id: 'SP-1',
        name: 'DHL Express',
        gstin: '29AAACD1234F1Z5',
        state: 'Karnataka',
        contactPerson: '',
        contactEmail: '',
        contactPhone: '',
        createdAt: '',
      },
    ]);
    vi.mocked(expenseService.listExpenseTypes).mockResolvedValue([
      {
        id: 'ET-1',
        name: 'Clearance',
        defaultCgstRateBp: 900,
        defaultSgstRateBp: 900,
        defaultIgstRateBp: 0,
        defaultTdsRateBp: 200,
        isActive: true,
        createdAt: '',
      },
    ]);
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'SH-1',
          supplierId: 'SUP-1',
          supplierName: 'Bosch GmbH',
          invoiceNumber: 'INV-SH-1',
          invoiceDate: '2026-04-10',
          goodsCategory: 'Machinery',
          invoiceValue: 1000,
          invoiceCurrency: 'USD',
          incoterm: 'CIF',
          shipmentMode: 'Air',
          shipmentType: 'Dutiable',
          blAwbNumber: '',
          blAwbDate: '',
          vesselName: '',
          containerNumber: '',
          grossWeightKg: 10,
          etd: '',
          eta: '',
          status: 'Active',
          dateOfDelivery: '',
          isFrozen: false,
          createdAt: '',
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 500,
    });
    vi.mocked(expenseService.mergeDuplicateLines).mockResolvedValue([
      {
        expenseTypeId: 'ET-1',
        amountPaise: 50000,
        cgstRateBp: 900,
        sgstRateBp: 900,
        igstRateBp: 0,
        tdsRateBp: 200,
        remarks: 'Combined',
      },
    ]);
    vi.mocked(expenseService.previewCalculation).mockResolvedValue({
      lines: [],
      baseAmountPaise: 50000,
      totalCgstPaise: 4500,
      totalSgstPaise: 4500,
      totalIgstPaise: 0,
      totalTdsPaise: 1000,
      totalAmountPaise: 59000,
      netAmountPaise: 58000,
      totalAmountInr: 590,
      netAmountInr: 580,
      hasDuplicateType: false,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ExpensesPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('DHL-101')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /New Expense Invoice/i }));
    await waitFor(() =>
      expect(screen.getByText('New Multi-Line Expense Invoice')).toBeInTheDocument()
    );

    // Add a second line with same default expenseTypeId -> triggers duplicate warning + 1-click combine
    fireEvent.click(screen.getByRole('button', { name: /Add Line/i }));
    await waitFor(() =>
      expect(screen.getByText(/Duplicate expense type detected across lines/i)).toBeInTheDocument()
    );

    fireEvent.click(screen.getByRole('button', { name: /Combine Duplicate Lines/i }));
    await waitFor(() => expect(expenseService.mergeDuplicateLines).toHaveBeenCalled());
  });

  it('switches Expense Reports across views and renders Landed Cost rows excluding IGST', async () => {
    vi.mocked(expenseService.listServiceProviders).mockResolvedValue([]);
    vi.mocked(expenseService.listExpenseTypes).mockResolvedValue([]);
    vi.mocked(expenseReportsService.getReport).mockResolvedValue({
      viewMode: 'detailed',
      detailedRows: [
        {
          expenseId: 'EXP-1',
          expenseInvoiceId: 'EINV-1',
          shipmentId: 'SH-1',
          shipmentInvoiceNo: 'INV-100',
          serviceProviderId: 'SP-1',
          serviceProviderName: 'DHL Express',
          expenseTypeId: 'ET-1',
          expenseTypeName: 'Clearance',
          invoiceNo: 'DHL-101',
          invoiceDate: '2026-04-15',
          amountPaise: 100000,
          cgstPaise: 9000,
          sgstPaise: 9000,
          igstPaise: 0,
          tdsPaise: 2000,
          totalPaise: 118000,
          netPaise: 116000,
          remarks: '',
        },
      ],
      groupedRows: [],
      totalLines: 1,
      totalBasePaise: 100000,
      totalTaxPaise: 18000,
      totalTdsPaise: 2000,
      totalGrossPaise: 118000,
      totalNetPaise: 116000,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ExpenseReportsPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('DHL-101')).toBeInTheDocument());

    const byTypeTab = screen.getByRole('tab', { name: /By Expense Type/i });
    fireEvent.mouseDown(byTypeTab);
    fireEvent.click(byTypeTab);

    await waitFor(() =>
      expect(expenseReportsService.getReport).toHaveBeenCalledWith(
        expect.objectContaining({ viewMode: 'summary-by-type' })
      )
    );

    vi.mocked(reportsService.getLandedCostReport).mockResolvedValue({
      rows: [
        {
          supplier: 'Bosch GmbH',
          supplierId: 'SUP-1',
          invoiceNo: 'INV-LDC-1',
          invoiceDate: '2026-04-15',
          partNo: 'PN-100',
          description: 'High Pressure Pump',
          unit: 'PCS',
          qty: 2,
          unitPrice: 100,
          assessableValue: 18000,
          bcdAmount: 1350,
          swsAmount: 135,
          igstAmount: 3507.3,
          expensesTotal: 6000,
          ldcPerQty: 12742.5,
        },
      ],
      summary: {
        totalRows: 1,
        totalQuantity: 2,
        totalAssessableValue: 18000,
        totalBasicDuties: 1485,
        totalIgst: 3507.3,
        totalBasicExpenses: 6000,
        totalLandedCostExIgst: 25485,
      },
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <LandedCostReportPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('PN-100')).toBeInTheDocument());
    expect(screen.getByText('12742.5000')).toBeInTheDocument();
  });
});
