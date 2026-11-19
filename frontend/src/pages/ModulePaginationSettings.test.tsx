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
import { supplierService, itemService } from '@/services/master-service';
import { shipmentService, invoiceService } from '@/services/shipment-invoice-service';
import { boeService, boeSummaryService } from '@/services/boe-service';
import { expenseService, type ExpenseInvoice } from '@/services/expense-service';
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
  formatPaiseINR: (paise: number) => `₹${(paise / 100).toFixed(2)}`,
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
    shipment: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: { shipmentType: 'uppercase' } },
    itemMaster: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    invoice: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    boe: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    boeSummary: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    expenses: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    cbicRates: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
    reconciledBoe: { visibleFields: [], itemsPerPage: 5, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
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
});
