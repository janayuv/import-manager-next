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
        isPaidByClearingAgent: false,
        clearingAgentId: '',
        clearingAgentName: '',
        chaReimburseInvoiceNo: '',
        chaReimburseInvoiceDate: '',
        baseAmountPaise: 100000,
        totalCgstPaise: 9000,
        totalSgstPaise: 9000,
        totalIgstPaise: 0,
        totalTdsPaise: 2000,
        totalAmountPaise: 118000,
        roundOffPaise: 0,
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
        isManualGst: false,
        roundOff: false,
        cgstRateBp: 900,
        sgstRateBp: 900,
        igstRateBp: 0,
        tdsRateBp: 200,
        cgstPaise: 4500,
        sgstPaise: 4500,
        igstPaise: 0,
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
      roundOffPaise: 0,
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

  it('supports toggling to manual GST amount entry for partial or non-eligible values', async () => {
    vi.mocked(expenseService.listExpenseInvoices).mockResolvedValue([
      {
        id: 'EINV-1',
        shipmentId: 'SH-1',
        shipmentInvoiceNo: 'INV-SH-1',
        serviceProviderId: 'SP-1',
        serviceProviderName: 'Express CHA Services',
        invoiceNo: 'CHA-101',
        invoiceDate: '2026-04-18',
        currency: 'INR',
        isPaidByClearingAgent: false,
        clearingAgentId: '',
        clearingAgentName: '',
        chaReimburseInvoiceNo: '',
        chaReimburseInvoiceDate: '',
        baseAmountPaise: 900000,
        totalCgstPaise: 81000,
        totalSgstPaise: 81000,
        totalIgstPaise: 0,
        totalTdsPaise: 18000,
        totalAmountPaise: 1062000,
        roundOffPaise: 0,
        netAmountPaise: 1044000,
        remarks: 'Sample invoice',
        idempotencyKey: 'idem-1',
        version: 1,
        createdAt: '2026-04-18T00:00:00Z',
        updatedAt: '2026-04-18T00:00:00Z',
        lines: [],
      },
    ]);
    vi.mocked(expenseService.listServiceProviders).mockResolvedValue([
      {
        id: 'SP-1',
        name: 'Express CHA Services',
        gstin: '33AABCE1234F1Z5',
        state: 'Tamil Nadu',
        contactPerson: 'Ramesh',
        contactEmail: 'ramesh@cha.in',
        contactPhone: '9876543210',
        createdAt: '2026-04-01T00:00:00Z',
      },
    ]);
    vi.mocked(expenseService.listExpenseTypes).mockResolvedValue([
      {
        id: 'ET-1',
        name: 'Customs Clearance',
        defaultCgstRateBp: 900,
        defaultSgstRateBp: 900,
        defaultIgstRateBp: 0,
        defaultTdsRateBp: 200,
        isActive: true,
        createdAt: '2026-04-01T00:00:00Z',
      },
    ]);
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'SH-1',
          invoiceNumber: 'INV-SH-1',
          supplierId: 'SUP-1',
          supplierName: 'Bosch GmbH',
          status: 'In Transit',
          isFrozen: false,
        } as any,
      ],
      totalCount: 1,
      page: 1,
      pageSize: 500,
    });

    vi.mocked(expenseService.previewCalculation).mockResolvedValue({
      lines: [
        {
          expenseTypeId: 'ET-1',
          amountPaise: 5000000,
          isManualGst: true,
          cgstRateBp: 180,
          sgstRateBp: 180,
          igstRateBp: 0,
          tdsRateBp: 200,
          cgstPaise: 90000,
          sgstPaise: 90000,
          igstPaise: 0,
          tdsPaise: 100000,
          totalPaise: 5180000,
          netPaise: 5080000,
          amountInr: 50000,
          totalInr: 51800,
          netInr: 50800,
          remarks: 'Partial GST',
        },
      ],
      baseAmountPaise: 5000000,
      totalCgstPaise: 90000,
      totalSgstPaise: 90000,
      totalIgstPaise: 0,
      totalTdsPaise: 100000,
      totalAmountPaise: 5180000,
      roundOffPaise: 0,
      netAmountPaise: 5080000,
      totalAmountInr: 51800,
      netAmountInr: 50800,
      hasDuplicateType: false,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ExpensesPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('CHA-101')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /New Expense Invoice/i }));
    await waitFor(() =>
      expect(screen.getByText('New Multi-Line Expense Invoice')).toBeInTheDocument()
    );

    // Initial mode is Automatic (% Rate)
    expect(screen.getByRole('button', { name: /Automatic \(% Rate\)/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Manual GST Amount \(₹\)/i })).toBeInTheDocument();
    expect(screen.getByText('CGST%')).toBeInTheDocument();

    // Set basic amount so amountPaise > 0
    fireEvent.change(screen.getByLabelText('Basic (INR)'), { target: { value: '50000.00' } });

    // Switch to Manual GST Amount mode
    fireEvent.click(screen.getByRole('button', { name: /Manual GST Amount \(₹\)/i }));

    // Verify manual mode labels and active banner
    await waitFor(() => {
      expect(screen.getByText(/Manual GST Mode active/i)).toBeInTheDocument();
      expect(screen.getByText(/CGST \(₹\)/i)).toBeInTheDocument();
      expect(screen.getByText(/SGST \(₹\)/i)).toBeInTheDocument();
      expect(screen.getByText(/IGST \(₹\)/i)).toBeInTheDocument();
    });

    // Enter manual CGST
    fireEvent.change(screen.getByLabelText('CGST Amount (INR)'), { target: { value: '900.00' } });

    await waitFor(() => {
      expect(expenseService.previewCalculation).toHaveBeenCalledWith(
        expect.arrayContaining([
          expect.objectContaining({
            isManualGst: true,
            cgstPaise: 90000,
          }),
        ])
      );
    });
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
          isPaidByClearingAgent: false,
          clearingAgentId: '',
          clearingAgentName: '',
          chaReimburseInvoiceNo: '',
          chaReimburseInvoiceDate: '',
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
    expect(screen.getByText('Invoice Total')).toBeInTheDocument();
    expect(screen.getByText('INR 1,180')).toBeInTheDocument();

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
          materialValue: 16000,
          assessableValue: 18000,
          bcdAmount: 1350,
          swsAmount: 135,
          igstAmount: 3507.3,
          boeInterest: 0,
          expensesTotal: 6000,
          ldcPerQty: 11742.5,
        },
      ],
      summary: {
        totalRows: 1,
        totalQuantity: 2,
        totalMaterialValue: 16000,
        totalAssessableValue: 18000,
        totalBasicDuties: 1485,
        totalIgst: 3507.3,
        totalBoeInterest: 0,
        totalBasicExpenses: 6000,
        totalLandedCostExIgst: 23485,
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
    expect(screen.getByText('11742.5000')).toBeInTheDocument();
    expect(screen.getAllByText('BOE Interest').length).toBeGreaterThan(0);
  });

  it('opens Manage Service Providers and Manage Expense Types dialogs from the multi-line invoice modal', async () => {
    vi.mocked(expenseService.listExpenseInvoices).mockResolvedValue([]);
    vi.mocked(expenseService.listServiceProviders).mockResolvedValue([
      {
        id: 'SP-1',
        name: 'DHL Express',
        gstin: '27AAAAA0000A1Z5',
        state: 'Maharashtra',
        contactPerson: 'Rahul',
        contactEmail: 'rahul@dhl.com',
        contactPhone: '9876543210',
        createdAt: '2026-04-01',
      },
    ]);
    vi.mocked(expenseService.listExpenseTypes).mockResolvedValue([
      {
        id: 'ET-1',
        name: 'Customs Clearance',
        defaultCgstRateBp: 900,
        defaultSgstRateBp: 900,
        defaultIgstRateBp: 0,
        defaultTdsRateBp: 200,
        isActive: true,
        createdAt: '2026-04-01',
      },
    ]);
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'SH-1',
          supplierId: 'SUP-1',
          supplierName: 'Overseas Supplier Ltd',
          invoiceNumber: 'INV-SH-1',
          invoiceDate: '2026-04-10',
          invoiceValue: 1000,
          invoiceCurrency: 'USD',
          incoterm: 'FOB',
          shipmentMode: 'Air',
          shipmentType: 'Air Cargo',
          goodsCategory: 'Electronics',
          blAwbNumber: '',
          blAwbDate: '',
          vesselName: '',
          containerNumber: '',
          grossWeightKg: 0,
          etd: '',
          eta: '',
          status: 'in-transit',
          dateOfDelivery: '',
          isFrozen: false,
          createdAt: '2026-04-10',
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 500,
    });
    vi.mocked(expenseService.createServiceProvider).mockResolvedValue({
      id: 'SP-2',
      name: 'FedEx Trade Networks',
      gstin: '27BBBBB1111B1Z6',
      state: 'Maharashtra',
      contactPerson: 'Amit',
      contactEmail: 'amit@fedex.com',
      contactPhone: '9123456789',
      createdAt: '2026-04-15',
    });
    vi.mocked(expenseService.createExpenseType).mockResolvedValue({
      id: 'ET-2',
      name: 'Ocean Freight Surcharge',
      defaultCgstRateBp: 0,
      defaultSgstRateBp: 0,
      defaultIgstRateBp: 1800,
      defaultTdsRateBp: 200,
      isActive: true,
      createdAt: '2026-04-15',
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ExpensesPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() =>
      expect(screen.getByRole('button', { name: /New Expense Invoice/i })).toBeInTheDocument()
    );

    // Open invoice dialog
    fireEvent.click(screen.getByRole('button', { name: /New Expense Invoice/i }));
    await waitFor(() =>
      expect(screen.getByText('New Multi-Line Expense Invoice')).toBeInTheDocument()
    );

    // Click "Manage" next to Service Provider *
    const manageButtons = screen.getAllByRole('button', { name: /Manage/i });
    expect(manageButtons.length).toBeGreaterThanOrEqual(2);

    // Open Manage Service Providers dialog
    fireEvent.click(manageButtons[0]!);
    await waitFor(() =>
      expect(screen.getByText('Manage Service Providers')).toBeInTheDocument()
    );

    // Add new provider
    fireEvent.change(screen.getByPlaceholderText(/DHL Global Forwarding/i), {
      target: { value: 'FedEx Trade Networks' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Add Provider/i }));
    await waitFor(() => expect(expenseService.createServiceProvider).toHaveBeenCalled());

    // Close the Manage Service Providers dialog
    fireEvent.click(screen.getByRole('button', { name: /Close/i }));

    // Open Manage Expense Types dialog from invoice modal
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /Manage Types/i })).toBeInTheDocument()
    );
    fireEvent.click(screen.getByRole('button', { name: /Manage Types/i }));
    await waitFor(() =>
      expect(screen.getByText('Manage Expense Types')).toBeInTheDocument()
    );

    // Add new expense type
    fireEvent.change(screen.getByPlaceholderText(/Ocean Freight \/ Customs Clearance/i), {
      target: { value: 'Ocean Freight Surcharge' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Add Expense Type/i }));
    await waitFor(() => expect(expenseService.createExpenseType).toHaveBeenCalled());
  });

  it('expands invoice row in ledger to show added expense line details and switches to itemized tab', async () => {
    vi.mocked(expenseService.listExpenseInvoices).mockResolvedValue([
      {
        id: 'EINV-101',
        shipmentId: 'SH-1',
        shipmentInvoiceNo: 'INV-SH-1',
        serviceProviderId: 'SP-1',
        serviceProviderName: 'DHL Express',
        invoiceNo: 'EXP-INV-999',
        invoiceDate: '2026-04-15',
        currency: 'INR',
        isPaidByClearingAgent: false,
        clearingAgentId: '',
        clearingAgentName: '',
        chaReimburseInvoiceNo: '',
        chaReimburseInvoiceDate: '',
        baseAmountPaise: 5000000,
        totalCgstPaise: 450000,
        totalSgstPaise: 450000,
        totalIgstPaise: 0,
        totalTdsPaise: 100000,
        totalAmountPaise: 5900000,
        roundOffPaise: 0,
        netAmountPaise: 5800000,
        remarks: 'Air shipment customs clearance',
        idempotencyKey: 'k999',
        version: 1,
        createdAt: '2026-04-15',
        updatedAt: '2026-04-15',
        lines: [
          {
            id: 'LINE-1',
            expenseInvoiceId: 'EINV-101',
            shipmentId: 'SH-1',
            serviceProviderId: 'SP-1',
            expenseTypeId: 'ET-1',
            expenseTypeName: 'Customs Duty Clearance',
            amountPaise: 5000000,
            isManualGst: false,
            cgstRateBp: 900,
            sgstRateBp: 900,
            igstRateBp: 0,
            tdsRateBp: 200,
            cgstPaise: 450000,
            sgstPaise: 450000,
            igstPaise: 0,
            tdsPaise: 100000,
            totalPaise: 5900000,
            netPaise: 5800000,
            remarks: 'Port duty pass',
          },
        ],
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
        name: 'Customs Duty Clearance',
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

    render(
      <MemoryRouter>
        <AuthProvider>
          <ExpensesPage />
        </AuthProvider>
      </MemoryRouter>
    );

    // Initial table view displays the invoice
    await waitFor(() => expect(screen.getByText('EXP-INV-999')).toBeInTheDocument());

    // Expand details via button
    const expandBtn = screen.getByRole('button', { name: /Expand details for EXP-INV-999/i });
    fireEvent.click(expandBtn);

    // Verify added expense line details are clearly visible
    await waitFor(() => {
      expect(screen.getByText(/Expense Line Details for Invoice EXP-INV-999/i)).toBeInTheDocument();
      expect(screen.getByText('Customs Duty Clearance')).toBeInTheDocument();
      expect(screen.getByText('Port duty pass')).toBeInTheDocument();
    });

    // Switch to the Itemized Expense Details tab
    const itemizedTab = screen.getByRole('tab', { name: /Itemized Expense Details/i });
    fireEvent.click(itemizedTab);

    // Verify itemized ledger view shows line row and summary
    await waitFor(() => {
      expect(screen.getByText('Total Line Items')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Export CSV/i })).toBeInTheDocument();
    });
  });

  it('supports multi-service invoice entry with roundoff and existing invoice helper', async () => {
    vi.mocked(expenseService.listExpenseInvoices).mockResolvedValue([
      {
        id: 'EINV-377',
        shipmentId: 'SH-1',
        shipmentInvoiceNo: 'INV-SH-1',
        serviceProviderId: 'SP-1',
        serviceProviderName: 'MANTHRA SHIPPING',
        invoiceNo: '377',
        invoiceDate: '2026-10-14',
        currency: 'INR',
        isPaidByClearingAgent: false,
        clearingAgentId: '',
        clearingAgentName: '',
        chaReimburseInvoiceNo: '',
        chaReimburseInvoiceDate: '',
        baseAmountPaise: 625000,
        totalCgstPaise: 56300,
        totalSgstPaise: 56300,
        totalIgstPaise: 0,
        totalTdsPaise: 0,
        totalAmountPaise: 737600,
        netAmountPaise: 737600,
        roundOffPaise: 0,
        remarks: 'Transport',
        idempotencyKey: 'idem-377',
        version: 1,
        createdAt: '2026-10-14T00:00:00Z',
        updatedAt: '2026-10-14T00:00:00Z',
        lines: [
          {
            id: 'line-1',
            expenseInvoiceId: 'EINV-377',
            shipmentId: 'SH-1',
            serviceProviderId: 'SP-1',
            expenseTypeId: 'ET-1',
            expenseTypeName: 'Transport Charges-LCL',
            amountPaise: 625000,
            cgstRateBp: 900,
            sgstRateBp: 900,
            igstRateBp: 0,
            tdsRateBp: 0,
            cgstPaise: 56300,
            sgstPaise: 56300,
            igstPaise: 0,
            tdsPaise: 0,
            totalPaise: 737600,
            netPaise: 737600,
            isManualGst: false,
            remarks: 'Transport only',
          },
        ],
      },
    ]);
    vi.mocked(expenseService.listServiceProviders).mockResolvedValue([
      {
        id: 'SP-1',
        name: 'MANTHRA SHIPPING',
        gstin: '33AABCE1234F1Z5',
        state: 'Tamil Nadu',
        contactPerson: '',
        contactEmail: '',
        contactPhone: '',
        createdAt: '',
      },
    ]);
    vi.mocked(expenseService.listExpenseTypes).mockResolvedValue([
      {
        id: 'ET-1',
        name: 'Transport Charges-LCL',
        defaultCgstRateBp: 900,
        defaultSgstRateBp: 900,
        defaultIgstRateBp: 0,
        defaultTdsRateBp: 0,
        isActive: true,
        createdAt: '',
      },
      {
        id: 'ET-2',
        name: 'Clearing & Forwarding LCL',
        defaultCgstRateBp: 900,
        defaultSgstRateBp: 900,
        defaultIgstRateBp: 0,
        defaultTdsRateBp: 0,
        isActive: true,
        createdAt: '',
      },
    ]);
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'SH-1',
          supplierId: 'SUP-1',
          supplierName: 'DY ELACEN CO LTD',
          invoiceNumber: 'INV-SH-1',
          invoiceDate: '2026-04-10',
          goodsCategory: 'Goods',
          invoiceValue: 5000,
          invoiceCurrency: 'USD',
          incoterm: 'CIF',
          shipmentMode: 'Sea',
          shipmentType: 'Dutiable',
          blAwbNumber: '',
          blAwbDate: '',
          vesselName: '',
          containerNumber: '',
          grossWeightKg: 100,
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
    vi.mocked(expenseService.previewCalculation).mockResolvedValue({
      lines: [],
      baseAmountPaise: 964900,
      totalCgstPaise: 86900,
      totalSgstPaise: 86900,
      totalIgstPaise: 0,
      totalTdsPaise: 0,
      totalAmountPaise: 1138700,
      netAmountPaise: 1138700,
      roundOffPaise: 0,
      totalAmountInr: 11387,
      netAmountInr: 11387,
      hasDuplicateType: false,
    });
    vi.mocked(expenseService.updateExpenseInvoice).mockResolvedValue({} as any);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ExpensesPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('377')).toBeInTheDocument());

    // Click New Expense Invoice
    fireEvent.click(screen.getByRole('button', { name: /New Expense Invoice/i }));
    await waitFor(() =>
      expect(screen.getByText('New Multi-Line Expense Invoice')).toBeInTheDocument()
    );

    // Type invoice number "377"
    const invInput = screen.getByLabelText(/Invoice No/i);
    fireEvent.change(invInput, { target: { value: '377' } });

    // Expect friendly helper indicating invoice 377 already exists and prompting to edit it
    await waitFor(() =>
      expect(
        screen.getByText(/Invoice “377” already exists for this service provider/i)
      ).toBeInTheDocument()
    );

    // Click "Edit Existing Invoice #377"
    const editBtn = screen.getByRole('button', { name: /Edit Existing Invoice #377/i });
    fireEvent.click(editBtn);

    // Verify it switched to edit mode for 377
    await waitFor(() =>
      expect(screen.getByText(/Edit Expense Invoice \(377\)/i)).toBeInTheDocument()
    );

    // Adjust Round Off field
    const roundOffInput = screen.getByLabelText(/Round Off \(INR\)/i);
    fireEvent.change(roundOffInput, { target: { value: '-1' } });

    // Click Save Expense Invoice
    const saveButtons = screen.getAllByRole('button', { name: /Save Expense Invoice/i });
    expect(saveButtons.length).toBeGreaterThan(0);
    fireEvent.click(saveButtons[0]!);

    await waitFor(() => {
      expect(expenseService.updateExpenseInvoice).toHaveBeenCalledWith(
        expect.objectContaining({
          invoiceNo: '377',
          roundOffPaise: -100,
          lines: expect.arrayContaining([
            expect.objectContaining({
              roundOff: true,
            }),
          ]),
        })
      );
    });
  });

  it('supports marking expense invoice as paid by clearing agent and displays CHA reimbursement details', async () => {
    vi.mocked(expenseService.listServiceProviders).mockResolvedValue([
      { id: 'SP-LINER', name: 'Maersk Line India', gstin: '27AABCM1234F1Z1', state: 'MH', contactPerson: '', contactEmail: '', contactPhone: '', createdAt: '' },
      { id: 'SP-CHA', name: 'Express Clearing & Forwarding', gstin: '27AABCE9876F1Z2', state: 'MH', contactPerson: '', contactEmail: '', contactPhone: '', createdAt: '' },
    ]);
    vi.mocked(expenseService.listExpenseTypes).mockResolvedValue([
      { id: 'ET-LINER', name: 'Liner Charges', defaultCgstRateBp: 900, defaultSgstRateBp: 900, defaultIgstRateBp: 0, defaultTdsRateBp: 0, isActive: true, createdAt: '' },
    ]);
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'SH-1',
          supplierId: 'SUP-1',
          supplierName: 'Bosch GmbH',
          invoiceNumber: 'INV-SH-1',
          invoiceDate: '2026-04-15',
          goodsCategory: 'Machinery',
          invoiceValue: 50000,
          invoiceCurrency: 'USD',
          incoterm: 'FOB',
          isFrozen: false,
        } as any,
      ],
      totalCount: 1,
      page: 1,
      pageSize: 500,
    });
    vi.mocked(expenseService.listExpenseInvoices).mockResolvedValue([
      {
        id: 'EINV-CHA-1',
        shipmentId: 'SH-1',
        shipmentInvoiceNo: 'INV-SH-1',
        serviceProviderId: 'SP-LINER',
        serviceProviderName: 'Maersk Line India',
        invoiceNo: 'MSK-9901',
        invoiceDate: '2026-11-20',
        currency: 'INR',
        isPaidByClearingAgent: true,
        clearingAgentId: 'SP-CHA',
        clearingAgentName: 'Express Clearing & Forwarding',
        chaReimburseInvoiceNo: 'CHA-REIMB-0088',
        chaReimburseInvoiceDate: '2026-11-22',
        baseAmountPaise: 2500000,
        totalCgstPaise: 225000,
        totalSgstPaise: 225000,
        totalIgstPaise: 0,
        totalTdsPaise: 0,
        totalAmountPaise: 2950000,
        netAmountPaise: 2950000,
        roundOffPaise: 0,
        remarks: 'Liner charges disbursed by CHA',
        idempotencyKey: 'k-cha-1',
        version: 1,
        createdAt: '2026-11-20T00:00:00Z',
        updatedAt: '2026-11-20T00:00:00Z',
        lines: [
          {
            id: 'line-liner-1',
            expenseInvoiceId: 'EINV-CHA-1',
            shipmentId: 'SH-1',
            serviceProviderId: 'SP-LINER',
            expenseTypeId: 'ET-LINER',
            expenseTypeName: 'Liner Charges',
            amountPaise: 2500000,
            cgstRateBp: 900,
            sgstRateBp: 900,
            igstRateBp: 0,
            tdsRateBp: 0,
            cgstPaise: 225000,
            sgstPaise: 225000,
            igstPaise: 0,
            tdsPaise: 0,
            totalPaise: 2950000,
            netPaise: 2950000,
            remarks: 'Liner charges',
            isManualGst: false,
          },
        ],
      },
    ]);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ExpensesPage />
        </AuthProvider>
      </MemoryRouter>
    );

    // Verify invoice MSK-9901 and Clearing Agent badge appear
    await waitFor(() => {
      expect(screen.getByText('MSK-9901')).toBeInTheDocument();
      expect(screen.getAllByText(/Paid by CHA: Express Clearing & Forwarding/i).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/CHA-REIMB-0088/i).length).toBeGreaterThan(0);
      expect(screen.getByText('CHA Reimbursement Bill')).toBeInTheDocument();
    });

    // Switch to Itemized Expense Details tab
    const itemizedTab = screen.getByRole('tab', { name: /Itemized Expense Details/i });
    fireEvent.click(itemizedTab);

    // Verify line item and CHA reimbursement badge are present
    await waitFor(() => {
      expect(screen.getByText('Liner Charges')).toBeInTheDocument();
      expect(screen.getAllByText(/Paid by CHA: Express Clearing & Forwarding/i).length).toBeGreaterThan(0);
    });
  });
});
