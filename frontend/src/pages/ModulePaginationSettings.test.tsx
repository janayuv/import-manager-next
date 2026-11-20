import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { ThemeProvider } from '@/contexts/ThemeContext';
import { SuppliersPage } from '@/pages/SuppliersPage';
import { ShipmentsPage } from '@/pages/ShipmentsPage';
import { ItemMasterPage } from '@/pages/ItemMasterPage';
import { InvoicesPage } from '@/pages/InvoicesPage';
import { BOERegistryPage } from '@/pages/BOERegistryPage';
import { ExpensesPage } from '@/pages/ExpensesPage';
import { ReconciledBOEPage } from '@/pages/ReconciledBOEPage';
import { CBICRatesPage } from '@/pages/CBICRatesPage';
import { PurchaseOrdersPage } from '@/pages/PurchaseOrdersPage';
import { PaymentsPage } from '@/pages/PaymentsPage';
import { AdvanceReconciliationPage } from '@/pages/AdvanceReconciliationPage';
import { LandedCostReportPage } from '@/pages/LandedCostReportPage';
import { PartPeriodAnalysisPage } from '@/pages/PartPeriodAnalysisPage';
import { TransitShipmentsPage } from '@/pages/TransitShipmentsPage';
import { supplierService, itemService } from '@/services/master-service';
import { shipmentService, invoiceService } from '@/services/shipment-invoice-service';
import { boeService, boeSummaryService } from '@/services/boe-service';
import { expenseService, reportsService, type ExpenseInvoice } from '@/services/expense-service';
import { paymentPOService } from '@/services/payment-po-service';
import { cbicRatesService, type CBICExchangeRate } from '@/services/cbic-rates-service';
import { settingsService, type AppSettingsProfile } from '@/services/dashboard-settings-service';

vi.mock('@/services/security-service', () => ({
  securityService: {
    getAuthSetupStatus: vi.fn().mockResolvedValue({ needsSetup: false, adminUsername: 'admin', recoveryMode: false }),
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

vi.mock('@/services/dashboard-settings-service', () => ({
  applyThemeProfile: vi.fn(),
  settingsService: {
    getProfile: vi.fn(),
    saveProfile: vi.fn(),
    resetModuleDefaults: vi.fn(),
  },
}));

vi.mock('@/services/master-service', () => ({
  optionsService: {
    list: vi.fn().mockResolvedValue([]),
    listAll: vi.fn().mockResolvedValue({}),
    upsert: vi.fn(),
    delete: vi.fn(),
  },
  supplierService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    checkDuplicate: vi.fn().mockResolvedValue(null),
    getHistory: vi.fn(),
    exportCsv: vi.fn(),
    downloadTemplateCsv: vi.fn(),
    importCsv: vi.fn(),
  },
  itemService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    checkPartNumber: vi.fn().mockResolvedValue(null),
    exportCsv: vi.fn(),
    downloadTemplateCsv: vi.fn(),
    importCsv: vi.fn(),
  },
}));

vi.mock('@/services/shipment-invoice-service', () => ({
  shipmentService: {
    list: vi.fn(),
    getDetail: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    freeze: vi.fn(),
    unfreeze: vi.fn(),
    checkReadyForDelivery: vi.fn().mockResolvedValue(0),
    getTransitShipmentReport: vi.fn(),
    updateShipmentDates: vi.fn(),
    exportCsv: vi.fn(),
    downloadTemplateCsv: vi.fn(),
    importCsv: vi.fn(),
  },
  invoiceService: {
    list: vi.fn(),
    getDetail: vi.fn(),
    delete: vi.fn(),
  },
}));

vi.mock('@/services/boe-service', () => ({
  boeService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    downloadTemplateCsv: vi.fn(),
    importCsv: vi.fn(),
  },
  boeSummaryService: {
    list: vi.fn(),
    updateStatus: vi.fn(),
  },
}));

vi.mock('@/services/expense-service', () => ({
  expenseService: {
    listExpenseInvoices: vi.fn(),
    listServiceProviders: vi.fn(),
    listExpenseTypes: vi.fn(),
    previewCalculation: vi.fn(),
    saveExpenseInvoice: vi.fn(),
    deleteExpenseInvoice: vi.fn(),
    createServiceProvider: vi.fn(),
    updateServiceProvider: vi.fn(),
    deleteServiceProvider: vi.fn(),
    createExpenseType: vi.fn(),
    updateExpenseType: vi.fn(),
    deleteExpenseType: vi.fn(),
    exportLedgerCsv: vi.fn(),
  },
  reportsService: {
    getLandedCostReport: vi.fn(),
    exportLandedCostCsv: vi.fn(),
    getPartPeriodAnalysis: vi.fn(),
    searchParts: vi.fn().mockResolvedValue([]),
  },
  formatPaiseINR: (paise: number) => `₹${(paise / 100).toFixed(2)}`,
}));

vi.mock('@/services/payment-po-service', () => ({
  paymentPOService: {
    listPurchaseOrders: vi.fn(),
    listPayments: vi.fn(),
    getAdvanceReconciliation: vi.fn(),
    getMaterialTracking: vi.fn(),
    getPurchaseOrder: vi.fn(),
    createPurchaseOrder: vi.fn(),
    deletePurchaseOrder: vi.fn(),
    createPayment: vi.fn(),
    getPayment: vi.fn(),
    linkInvoiceToPO: vi.fn(),
    recordMaterialInward: vi.fn(),
  },
}));

vi.mock('@/services/cbic-rates-service', () => ({
  cbicRatesService: {
    list: vi.fn(),
    getEffectiveRate: vi.fn(),
    create: vi.fn(),
    createBatch: vi.fn(),
    delete: vi.fn(),
  },
}));

const mockProfileWith5Rows: AppSettingsProfile = {
  schemaVersion: 1,
  theme: 'default',
  mode: 'system',
  lineTotalDecimals: 2,
  invoiceTotalDecimals: 2,
  numberFormat: 'en-IN',
  dateFormat: 'DD/MM/YYYY',
  textFormat: 'preserve',
  modules: {
    supplier: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    transitTracker: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    shipment: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: { shipmentType: 'uppercase' } },
    itemMaster: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    invoice: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    boe: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    boeSummary: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    expenses: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    cbicRates: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    reconciledBoe: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    purchaseOrders: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    payments: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    advanceRecon: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    landedCost: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    partPeriodAnalysis: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
  },
  updatedAt: '2026-11-16T10:00:00Z',
};

describe('Per-Module Table Pagination Settings (5 rows per page)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    window.localStorage.setItem('im_settings_profile', JSON.stringify(mockProfileWith5Rows));
    vi.mocked(settingsService.getProfile).mockResolvedValue(mockProfileWith5Rows);
  });

  it('SuppliersPage applies itemsPerPage = 5 from settings and shows Page 1 of 4 for 16 suppliers', async () => {
    const mockItems = Array.from({ length: 5 }, (_, i) => ({
      id: `Sup-00${i + 1}`,
      supplierName: `Supplier ${i + 1}`,
      shortName: `S${i + 1}`,
      country: 'Germany',
      email: `sup${i + 1}@example.com`,
      phone: '',
      beneficiaryName: '',
      bankName: '',
      branch: '',
      bankAddress: '',
      accountNo: '',
      iban: '',
      swiftCode: '',
      isActive: true,
    }));

    vi.mocked(supplierService.list).mockResolvedValue({
      items: mockItems,
      totalCount: 16,
      page: 1,
      pageSize: 5,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <SuppliersPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(supplierService.list).toHaveBeenCalledWith(
        expect.objectContaining({
          page: 1,
          pageSize: 5,
        }),
      );
    });

    await waitFor(() => {
      expect(screen.getByText('Page 1 of 4')).toBeInTheDocument();
      expect(screen.getByText('Suppliers (16)')).toBeInTheDocument();
    });
  });

  it('ShipmentsPage applies itemsPerPage = 5 from settings', async () => {
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [],
      totalCount: 0,
      page: 1,
      pageSize: 5,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <ShipmentsPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(shipmentService.list).toHaveBeenCalledWith(
        expect.objectContaining({
          page: 1,
          pageSize: 5,
        }),
      );
    });
  });

  it('ItemMasterPage applies itemsPerPage = 5 from settings', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 });
    vi.mocked(itemService.list).mockResolvedValue({
      items: [],
      totalCount: 0,
      page: 1,
      pageSize: 5,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <ItemMasterPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(itemService.list).toHaveBeenCalledWith(
        expect.objectContaining({
          page: 1,
          pageSize: 5,
        }),
      );
    });
  });

  it('InvoicesPage paginates 12 invoices into 5 per page (Page 1 of 3)', async () => {
    const mockInvoices = Array.from({ length: 12 }, (_, i) => ({
      id: `INV-${i + 1}`,
      shipmentId: `SHP-${i + 1}`,
      supplierId: 'SUP-1',
      supplierName: 'Test Supplier',
      invoiceNumber: `INV-NUM-${i + 1}`,
      invoiceDate: '2026-11-01',
      currency: 'USD',
      shipmentInvoiceValue: 100,
      computedTotal: 100,
      lineCount: 1,
      status: 'Finalized',
      lineTotalDecimals: 2,
      invoiceTotalDecimals: 2,
      shipmentFrozen: false,
    }));

    vi.mocked(invoiceService.list).mockResolvedValue(mockInvoices);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <InvoicesPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();
      // Only first 5 invoices should be shown on page 1
      expect(screen.getByText('INV-NUM-1')).toBeInTheDocument();
      expect(screen.getByText('INV-NUM-5')).toBeInTheDocument();
      expect(screen.queryByText('INV-NUM-6')).not.toBeInTheDocument();
    });
  });

  it('BOERegistryPage paginates duty calculations and registry records by itemsPerPage = 5', async () => {
    const mockSummaries = Array.from({ length: 9 }, (_, i) => ({
      id: `calc-${i + 1}`,
      shipmentId: `sh-${i + 1}`,
      boeId: `boe-${i + 1}`,
      beNumber: `BE-00${i + 1}`,
      beDate: '2026-11-01',
      supplierName: `Supplier ${i + 1}`,
      invoiceNumber: `INV-${i + 1}`,
      status: 'Reconciled',
      customsDutyTotal: 1000,
      dutyPaid: 1000,
      assessableTotal: 5000,
      createdAt: '2026-11-01T00:00:00Z',
    }));

    vi.mocked(boeService.list).mockResolvedValue([]);
    vi.mocked(boeSummaryService.list).mockResolvedValue(mockSummaries);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <BOERegistryPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Page 1 of 2')).toBeInTheDocument();
      expect(screen.getByText('INV-1')).toBeInTheDocument();
      expect(screen.getByText('INV-5')).toBeInTheDocument();
      expect(screen.queryByText('INV-6')).not.toBeInTheDocument();
    });
  });

  it('ExpensesPage paginates invoices by itemsPerPage = 5', async () => {
    const mockExpenseInvoices = Array.from({ length: 8 }, (_, i) => ({
      id: `exp-${i + 1}`,
      serviceProviderId: 'sp-1',
      serviceProviderName: 'Speed CHA',
      shipmentId: 'sh-1',
      shipmentInvoiceNo: 'CI-1',
      invoiceNo: `EXP-INV-${i + 1}`,
      invoiceDate: '2026-11-01',
      remarks: '',
      totalBasicPaise: 10000,
      totalCgstPaise: 900,
      totalSgstPaise: 900,
      totalIgstPaise: 0,
      totalTdsPaise: 200,
      totalNetPaise: 11600,
      roundOffPaise: 0,
      lines: [],
      createdAt: '2026-11-01T00:00:00Z',
    }));

    vi.mocked(expenseService.listExpenseInvoices).mockResolvedValue(mockExpenseInvoices as unknown as ExpenseInvoice[]);
    vi.mocked(expenseService.listServiceProviders).mockResolvedValue([]);
    vi.mocked(expenseService.listExpenseTypes).mockResolvedValue([]);
    vi.mocked(shipmentService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <ExpensesPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Page 1 of 2')).toBeInTheDocument();
      expect(screen.getByText('EXP-INV-1')).toBeInTheDocument();
      expect(screen.getByText('EXP-INV-5')).toBeInTheDocument();
      expect(screen.queryByText('EXP-INV-6')).not.toBeInTheDocument();
    });
  });

  it('ReconciledBOEPage paginates calculations by itemsPerPage = 5 from reconciledBoe settings', async () => {
    const mockReconciled = Array.from({ length: 8 }, (_, i) => ({
      id: `calc-${i + 1}`,
      shipmentId: `sh-${i + 1}`,
      boeId: `boe-${i + 1}`,
      beNumber: `BE-REC-${i + 1}`,
      beDate: '2026-11-01',
      supplierName: `Supplier ${i + 1}`,
      invoiceNumber: `INV-REC-${i + 1}`,
      status: 'Reconciled',
      customsDutyTotal: 1000,
      dutyPaid: 1000,
      assessableTotal: 5000,
      createdAt: '2026-11-01T00:00:00Z',
    }));

    vi.mocked(boeSummaryService.list).mockResolvedValue(mockReconciled);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <ReconciledBOEPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText(/Page 1 of 2/i)).toBeInTheDocument();
      expect(screen.getByText('BE-REC-1')).toBeInTheDocument();
      expect(screen.getByText('BE-REC-5')).toBeInTheDocument();
      expect(screen.queryByText('BE-REC-6')).not.toBeInTheDocument();
    });
  });

  it('CBICRatesPage paginates exchange rates by itemsPerPage = 5 from cbicRates settings', async () => {
    const mockRates: CBICExchangeRate[] = Array.from({ length: 8 }, (_, i) => ({
      id: i + 1,
      currencyCode: `CUR${i + 1}`,
      exchangeRate: 10 + i,
      effectiveFrom: '2026-11-01',
      effectiveTo: null,
      notificationNumber: `NT-0${i + 1}/2026`,
      sourceDocument: 'Gazette of India',
      verificationStatus: 'VERIFIED',
      createdAt: '2026-11-01 10:00:00',
      createdBy: 'admin',
    }));

    vi.mocked(cbicRatesService.list).mockResolvedValue(mockRates);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <CBICRatesPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText(/Page 1 of 2/i)).toBeInTheDocument();
      expect(screen.getByText('CUR1')).toBeInTheDocument();
      expect(screen.getByText('CUR5')).toBeInTheDocument();
      expect(screen.queryByText('CUR6')).not.toBeInTheDocument();
    });
  });

  it('PurchaseOrdersPage applies itemsPerPage = 5 from purchaseOrders settings', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 200 });
    vi.mocked(itemService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 300 });

    const mockPOs = Array.from({ length: 5 }, (_, i) => ({
      id: `po-${i + 1}`,
      poNumber: `PO-2026-00${i + 1}`,
      poDate: '2026-11-20',
      supplierId: 'sup-1',
      supplierName: 'Precision Engineering Ltd',
      currency: 'USD',
      paymentTermsPreset: '100_ADVANCE',
      totalAmount: 5000,
      advanceRequired: 5000,
      advancePaid: 0,
      advanceBalance: 5000,
      status: 'ISSUED',
    }));

    vi.mocked(paymentPOService.listPurchaseOrders).mockResolvedValue({
      items: mockPOs as any,
      totalCount: 12,
      page: 1,
      pageSize: 5,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <PurchaseOrdersPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(paymentPOService.listPurchaseOrders).toHaveBeenCalledWith(
        expect.objectContaining({
          page: 1,
          pageSize: 5,
        }),
      );
      expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();
    });
  });

  it('PaymentsPage applies itemsPerPage = 5 from payments settings', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 200 });

    const mockPayments = Array.from({ length: 5 }, (_, i) => ({
      id: `pay-${i + 1}`,
      voucherNumber: `PAY-2026-000${i + 1}`,
      paymentDate: '2026-11-20',
      supplierId: 'sup-1',
      supplierName: 'Precision Engineering Ltd',
      paymentType: 'PO_ADVANCE',
      paymentMode: 'WIRE_TRANSFER',
      currency: 'USD',
      totalAmount: 1000,
      referenceNumber: `REF-${i + 1}`,
      bankName: 'SBI',
    }));

    vi.mocked(paymentPOService.listPayments).mockResolvedValue({
      items: mockPayments as any,
      totalCount: 14,
      page: 1,
      pageSize: 5,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <PaymentsPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(paymentPOService.listPayments).toHaveBeenCalledWith(
        expect.objectContaining({
          page: 1,
          pageSize: 5,
        }),
      );
      expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();
    });
  });

  it('AdvanceReconciliationPage paginates advance reconciliations by itemsPerPage = 5 from advanceRecon settings', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 200 });

    const mockRecons = Array.from({ length: 11 }, (_, i) => ({
      poId: `po-${i + 1}`,
      poNumber: `PO-ADV-${i + 1}`,
      poDate: '2026-11-20',
      supplierId: 'sup-1',
      supplierName: 'Precision Engineering Ltd',
      currency: 'USD',
      poTotalAmount: 10000,
      advancePaidAmount: 4000,
      totalInvoicedAmount: 4000,
      varianceAmount: 0,
      varianceType: 'EXACT_MATCH' as const,
      reconciliationStatus: 'FULLY_RECONCILED',
      invoices: [],
    }));

    vi.mocked(paymentPOService.getAdvanceReconciliation).mockResolvedValue(mockRecons as any);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <AdvanceReconciliationPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();
      expect(screen.getByText('PO-ADV-1')).toBeInTheDocument();
      expect(screen.getByText('PO-ADV-5')).toBeInTheDocument();
      expect(screen.queryByText('PO-ADV-6')).not.toBeInTheDocument();
    });
  });

  it('LandedCostReportPage paginates landed cost rows by itemsPerPage = 5 from landedCost settings', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 });

    const mockRows = Array.from({ length: 12 }, (_, i) => ({
      supplier: 'Test Supplier',
      invoiceNo: `INV-LDC-${i + 1}`,
      invoiceDate: '2026-11-20',
      partNo: `PART-${i + 1}`,
      description: `Part Description ${i + 1}`,
      unit: 'NOS',
      qty: 100,
      unitPrice: 10,
      assessableValue: 85000,
      bcdAmount: 6375,
      swsAmount: 637.5,
      igstAmount: 16562.25,
      boeInterest: 0,
      expensesTotal: 1200,
      ldcPerQty: 932.12,
    }));

    vi.mocked(reportsService.getLandedCostReport).mockResolvedValue({
      rows: mockRows as any,
      summary: {
        totalRows: 12,
        totalQuantity: 1200,
        totalAssessableValue: 1020000,
        totalBasicDuties: 7012.5,
        totalIgst: 16562.25,
        totalBoeInterest: 0,
        totalBasicExpenses: 14400,
        totalLandedCostExIgst: 1041412.5,
      },
    } as any);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <LandedCostReportPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();
      expect(screen.getByText('INV-LDC-1')).toBeInTheDocument();
      expect(screen.getByText('INV-LDC-5')).toBeInTheDocument();
      expect(screen.queryByText('INV-LDC-6')).not.toBeInTheDocument();
    });
  });

  it('PartPeriodAnalysisPage paginates shipments table by itemsPerPage = 5 from partPeriodAnalysis settings', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 });

    const mockShipments = Array.from({ length: 9 }, (_, i) => ({
      shipmentId: `SHP-PA-${i + 1}`,
      invoiceNumber: `INV-PA-${i + 1}`,
      invoiceDate: '2026-11-20',
      supplierName: 'Tokyo Micro Electronics',
      incoterm: 'FOB',
      shipmentMode: 'Air',
      blAwbNumber: `AWB-${i + 1}`,
      vesselName: '',
      status: 'Delivered',
      quantity: 50,
      unit: 'NOS',
      unitPrice: 20,
      currency: 'USD',
      lineTotal: 1000,
      priceDiverged: false,
      masterUnitPrice: 20,
    }));

    vi.mocked(reportsService.getPartPeriodAnalysis).mockResolvedValue({
      targetItem: {
        id: 'it-1',
        partNumber: 'MCU-X86',
        itemDescription: 'Industrial MCU',
        unit: 'NOS',
        currency: 'USD',
        unitPrice: 20,
        hsnCode: '85423100',
        isActive: true,
      },
      summary: {
        totalShipments: 9,
        totalInvoices: 9,
        totalBoes: 0,
        totalQuantity: 450,
        unit: 'NOS',
        totalForexValue: 9000,
        currency: 'USD',
        totalAssessableValueInr: 765000,
        totalBcdAmount: 60000,
        totalSwsAmount: 6000,
        totalIgstAmount: 140000,
        totalBoeInterest: 0,
        totalCustomsDutyInr: 66000,
        totalDutyPaidInr: 66000,
        totalAllocatedExpensesInr: 14400,
        totalAllocatedExpensesWithGstInr: 16992,
        totalLandedCostExIgst: 845400,
        avgLandedCostPerUnit: 1878.67,
        minLandedCostPerUnit: 1800,
        maxLandedCostPerUnit: 1950,
        firstImportDate: '2026-01-01',
        latestImportDate: '2026-11-20',
      },
      shipments: mockShipments,
      boeLines: [],
      expenses: [],
      trends: [],
    } as any);

    render(
      <MemoryRouter initialEntries={['/part-analysis?partNo=MCU-X86']}>
        <AuthProvider>
          <ThemeProvider>
            <PartPeriodAnalysisPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Page 1 of 2')).toBeInTheDocument();
      expect(screen.getByText('SHP-PA-1')).toBeInTheDocument();
      expect(screen.getByText('SHP-PA-5')).toBeInTheDocument();
      expect(screen.queryByText('SHP-PA-6')).not.toBeInTheDocument();
    });
  });

  it('BOERegistryPage directly uses boe settings for both duty calculations and registry records even without boeSummary', async () => {
    const profileOnlyBoe: AppSettingsProfile = {
      ...mockProfileWith5Rows,
      modules: {
        ...mockProfileWith5Rows.modules,
        boe: { visibleFields: [], itemsPerPage: 6, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
        boeSummary: { visibleFields: [], itemsPerPage: 10, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
      },
    };

    window.localStorage.setItem('im_settings_profile', JSON.stringify(profileOnlyBoe));
    vi.mocked(settingsService.getProfile).mockResolvedValue(profileOnlyBoe);

    const mockSummaries = Array.from({ length: 14 }, (_, i) => ({
      id: `calc-boe-${i + 1}`,
      shipmentId: `sh-boe-${i + 1}`,
      boeId: `boe-${i + 1}`,
      beNumber: `BE-TEST-${i + 1}`,
      beDate: '2026-11-01',
      supplierName: `Supplier ${i + 1}`,
      invoiceNumber: `INV-BOE-${i + 1}`,
      status: 'Reconciled',
      customsDutyTotal: 1000,
      dutyPaid: 1000,
      assessableTotal: 5000,
      createdAt: '2026-11-01T00:00:00Z',
    }));

    vi.mocked(boeService.list).mockResolvedValue([]);
    vi.mocked(boeSummaryService.list).mockResolvedValue(mockSummaries);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <BOERegistryPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      // 14 items sliced by 6 per page = 3 pages (Page 1 of 3)
      expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();
      expect(screen.getByText('INV-BOE-1')).toBeInTheDocument();
      expect(screen.getByText('INV-BOE-6')).toBeInTheDocument();
      expect(screen.queryByText('INV-BOE-7')).not.toBeInTheDocument();
    });
  });

  it('TransitShipmentsPage applies itemsPerPage = 5 from transitTracker settings and paginates consignments', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({
      items: [],
      totalCount: 0,
      page: 1,
      pageSize: 500,
    });

    const mockTransitItems = Array.from({ length: 12 }, (_, i) => ({
      id: `tr-item-${i + 1}`,
      shipmentId: `sh-tr-${i + 1}`,
      supplierName: `Supplier ${i + 1}`,
      country: 'Germany',
      invoiceNumber: `INV-TR-${i + 1}`,
      invoiceDate: '2026-11-01',
      vesselVoyage: `Vessel ${i + 1}`,
      etd: '2026-11-01',
      eta: '2026-11-10',
      dateOfDelivery: '',
      transitDays: 9,
      status: 'In Transit' as const,
      remarks: 'Fast track',
    }));

    vi.mocked(shipmentService.getTransitShipmentReport).mockResolvedValue({
      summary: {
        totalShipments: 12,
        inTransit: 12,
        arrived: 0,
        delayed: 0,
        pending: 0,
        averagePlannedTransit: 9,
      },
      breakdown: [
        { status: 'In Transit', count: 12, pct: 100 },
        { status: 'Arrived', count: 0, pct: 0 },
        { status: 'Delayed', count: 0, pct: 0 },
        { status: 'Pending', count: 0, pct: 0 },
      ],
      items: mockTransitItems as any,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ThemeProvider>
            <TransitShipmentsPage />
          </ThemeProvider>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      // 12 items with 5 rows per page => "Showing 1 to 5 of 12 consignments" and "Page 1 of 3"
      expect(screen.getByText(/Showing 1 to 5 of 12 consignments/i)).toBeInTheDocument();
      expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();
      expect(screen.getByText('INV-TR-1')).toBeInTheDocument();
      expect(screen.getByText('INV-TR-5')).toBeInTheDocument();
      expect(screen.queryByText('INV-TR-6')).not.toBeInTheDocument();
    });
  });
});
