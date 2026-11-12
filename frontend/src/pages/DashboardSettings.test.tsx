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
});
