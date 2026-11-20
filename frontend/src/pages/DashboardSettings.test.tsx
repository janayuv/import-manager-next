import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { DashboardPage } from '@/pages/DashboardPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { dashboardService, settingsService } from '@/services/dashboard-settings-service';

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

vi.mock('@/services/dashboard-settings-service', () => ({
  dashboardService: {
    getOverview: vi.fn(),
    invalidateCache: vi.fn(),
    rebuildDailySnapshot: vi.fn(),
  },
  settingsService: {
    getProfile: vi.fn(),
    saveProfile: vi.fn(),
    resetModuleDefaults: vi.fn(),
  },
  applyThemeProfile: vi.fn(),
}));

describe('Stage 8: Real-Data Dashboard & Settings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders live KPIs on DashboardPage and triggers snapshot rebuild', async () => {
    vi.mocked(dashboardService.getOverview).mockResolvedValue({
      financialYear: 'all',
      startDate: '',
      endDate: '',
      availableFys: ['2026-27', '2025-26'],
      kpis: {
        totalSuppliers: 12,
        activeSuppliers: 11,
        totalShipments: 4,
        pendingShipments: 3,
        completedShipments: 1,
        overdueEtaCount: 1,
        shipmentsMissingBoe: 2,
        shipmentsMissingExpense: 1,
        totalInvoices: 4,
        pendingBoes: 2,
        reconciledBoes: 2,
        totalImportValueInr: 450000,
        totalCustomsDutyInr: 125000,
        totalBasicExpensesInr: 32000,
        totalGrossExpensesInr: 37760,
        openExceptionCount: 1,
        slaBreachCount: 0,
        openIncidentCount: 0,
        workflowHealthScore: 99.6,
      },
      monthlyTrends: [
        {
          period: '2026-04',
          shipments: 2,
          importValueInr: 450000,
          customsDutyInr: 125000,
          basicExpensesInr: 32000,
        },
      ],
      supplierBreakdown: [
        {
          supplierId: 'sup-1',
          supplierName: 'Bosch GmbH',
          shipmentCount: 2,
          totalAssessableInr: 450000,
          totalDutyInr: 125000,
        },
      ],
      statusBreakdown: [{ status: 'in-transit', count: 3 }],
      recentActivity: [],
      alerts: [
        {
          kind: 'OVERDUE_ETA',
          severity: 'critical',
          title: 'Shipments past ETA',
          count: 1,
          navigationTarget: '/shipments',
        },
      ],
      cacheHit: false,
      generatedAt: '2026-11-12T10:00:00Z',
      lastSnapshotDate: '2026-11-12',
    });
    vi.mocked(dashboardService.rebuildDailySnapshot).mockResolvedValue('2026-11-12');

    render(
      <MemoryRouter>
        <AuthProvider>
          <DashboardPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('11 / 12')).toBeInTheDocument());
    expect(screen.getByText('99.6 / 100')).toBeInTheDocument();
    expect(screen.getByText('Shipments past ETA')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Rebuild Snapshot/i }));
    await waitFor(() => expect(dashboardService.rebuildDailySnapshot).toHaveBeenCalled());
  });

  it('loads settings profile with locked uppercase shipmentType and saves changes', async () => {
    const mockProfile = {
      schemaVersion: 1,
      theme: 'default',
      mode: 'light',
      lineTotalDecimals: 2,
      invoiceTotalDecimals: 2,
      numberFormat: 'en-IN',
      dateFormat: 'DD/MM/YYYY',
      textFormat: 'preserve',
      modules: {
        shipment: {
          visibleFields: [],
          itemsPerPage: 10,
          decimalPrecision: 2,
          dateFormat: 'DD/MM/YYYY',
          fieldCases: { shipmentType: 'uppercase' },
        },
      },
      updatedAt: '2026-11-12T10:00:00Z',
    };
    vi.mocked(settingsService.getProfile).mockResolvedValue(mockProfile);
    vi.mocked(settingsService.saveProfile).mockResolvedValue(mockProfile);

    render(
      <MemoryRouter>
        <AuthProvider>
          <SettingsPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() =>
      expect(screen.getByText(/shipmentType case locked: uppercase/i)).toBeInTheDocument()
    );

    fireEvent.click(screen.getByRole('button', { name: /Save Settings/i }));
    await waitFor(() => expect(settingsService.saveProfile).toHaveBeenCalledWith(mockProfile));
  });

  it('renders at most 5 recent activities in Recent ERP Activity table', async () => {
    vi.mocked(dashboardService.getOverview).mockResolvedValue({
      financialYear: 'all',
      startDate: '',
      endDate: '',
      availableFys: ['2026-27'],
      kpis: {
        totalSuppliers: 1,
        activeSuppliers: 1,
        totalShipments: 1,
        pendingShipments: 0,
        completedShipments: 1,
        overdueEtaCount: 0,
        shipmentsMissingBoe: 0,
        shipmentsMissingExpense: 0,
        totalInvoices: 1,
        pendingBoes: 0,
        reconciledBoes: 1,
        totalImportValueInr: 1000,
        totalCustomsDutyInr: 100,
        totalBasicExpensesInr: 50,
        totalGrossExpensesInr: 59,
        openExceptionCount: 0,
        slaBreachCount: 0,
        openIncidentCount: 0,
        workflowHealthScore: 100,
      },
      monthlyTrends: [],
      supplierBreakdown: [],
      statusBreakdown: [],
      recentActivity: [
        { id: 1, tableName: 'shipments', rowId: 'SH-1', action: 'INSERT', userId: 'admin', createdAt: '2026-11-18 10:01' },
        { id: 2, tableName: 'shipments', rowId: 'SH-2', action: 'INSERT', userId: 'admin', createdAt: '2026-11-18 10:02' },
        { id: 3, tableName: 'shipments', rowId: 'SH-3', action: 'INSERT', userId: 'admin', createdAt: '2026-11-18 10:03' },
        { id: 4, tableName: 'shipments', rowId: 'SH-4', action: 'INSERT', userId: 'admin', createdAt: '2026-11-18 10:04' },
        { id: 5, tableName: 'shipments', rowId: 'SH-5', action: 'INSERT', userId: 'admin', createdAt: '2026-11-18 10:05' },
        { id: 6, tableName: 'shipments', rowId: 'SH-6', action: 'INSERT', userId: 'admin', createdAt: '2026-11-18 10:06' },
        { id: 7, tableName: 'shipments', rowId: 'SH-7', action: 'INSERT', userId: 'admin', createdAt: '2026-11-18 10:07' },
      ],
      alerts: [],
      cacheHit: false,
      generatedAt: '2026-11-18T10:00:00Z',
      lastSnapshotDate: '2026-11-18',
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <DashboardPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() => expect(screen.getByText('Recent ERP Activity')).toBeInTheDocument());
    expect(screen.getByText('shipments (SH-1)')).toBeInTheDocument();
    expect(screen.getByText('shipments (SH-5)')).toBeInTheDocument();
    expect(screen.queryByText('shipments (SH-6)')).not.toBeInTheDocument();
    expect(screen.queryByText('shipments (SH-7)')).not.toBeInTheDocument();
  });

  it('renders Per-Module Table Preferences (14 Modules) including CBIC Rates and Reconciled BOE', async () => {
    const mockProfile = {
      schemaVersion: 1,
      theme: 'default',
      mode: 'system',
      lineTotalDecimals: 2,
      invoiceTotalDecimals: 2,
      numberFormat: 'en-IN',
      dateFormat: 'DD/MM/YYYY',
      textFormat: 'preserve',
      modules: {
        shipment: { visibleFields: [], itemsPerPage: 10, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: { shipmentType: 'uppercase' } },
        cbicRates: { visibleFields: [], itemsPerPage: 25, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
        reconciledBoe: { visibleFields: [], itemsPerPage: 15, decimalPrecision: 2, dateFormat: 'DD/MM/YYYY', fieldCases: {} },
      },
      updatedAt: '2026-11-19T10:00:00Z',
    };
    vi.mocked(settingsService.getProfile).mockResolvedValue(mockProfile);

    render(
      <MemoryRouter>
        <AuthProvider>
          <SettingsPage />
        </AuthProvider>
      </MemoryRouter>
    );

    await waitFor(() =>
      expect(screen.getByText(/Per-Module Table Preferences \(14 Modules\)/i)).toBeInTheDocument()
    );
  });
});
