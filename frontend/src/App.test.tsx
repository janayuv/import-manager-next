import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { applyThemeProfile, settingsService } from '@/services/dashboard-settings-service';
import { securityService } from '@/services/security-service';

vi.mock('@/services/dashboard-settings-service', () => ({
  dashboardService: {
    getOverview: vi.fn().mockResolvedValue({
      financialYear: 'all',
      startDate: '',
      endDate: '',
      availableFys: ['2026-27'],
      kpis: {
        totalSuppliers: 11,
        activeSuppliers: 11,
        totalShipments: 0,
        pendingShipments: 0,
        completedShipments: 0,
        overdueEtaCount: 0,
        shipmentsMissingBoe: 0,
        shipmentsMissingExpense: 0,
        totalInvoices: 0,
        pendingBoes: 0,
        reconciledBoes: 0,
        totalImportValueInr: 0,
        totalCustomsDutyInr: 0,
        totalBasicExpensesInr: 0,
        totalGrossExpensesInr: 0,
        openExceptionCount: 0,
        slaBreachCount: 0,
        openIncidentCount: 0,
        workflowHealthScore: 100,
      },
      monthlyTrends: [],
      supplierBreakdown: [],
      statusBreakdown: [],
      recentActivity: [],
      alerts: [],
      cacheHit: false,
      generatedAt: '2026-11-12T10:00:00Z',
      lastSnapshotDate: '',
    }),
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

vi.mock('@/services/app-service', () => ({
  fetchBootstrapInfo: vi.fn().mockResolvedValue({
    appName: 'Import Manager',
    appVersion: '1.0.0',
    goVersion: 'go1.26.11',
    os: 'windows',
    arch: 'amd64',
    recoveryMode: false,
    paths: {
      dataDir: 'C:\\Users\\Test\\AppData\\Roaming\\ImportManager',
      databasePath: 'C:\\Users\\Test\\AppData\\Roaming\\ImportManager\\import_manager.db',
      backupsDir: '',
      attachmentsDir: '',
      logsDir: '',
      exportsDir: '',
      tempDir: '',
    },
    database: {
      cipherVersion: '4.4.2 community',
      journalMode: 'wal',
      foreignKeysOn: true,
      integrityCheck: 'ok',
      encrypted: true,
    },
  }),
  listSchemaMigrations: vi.fn().mockResolvedValue([
    {
      version: 1,
      name: 'initial_schema',
      checksum: 'abcdef1234567890abcdef',
      executedAt: '2026-11-12T10:00:00Z',
      appVersion: '1.0.0',
      durationMs: 12,
      status: 'SUCCESS',
    },
  ]),
}));

vi.mock('@/services/security-service', () => ({
  securityService: {
    getAuthSetupStatus: vi.fn(),
    completeAdminSetup: vi.fn(),
    authenticate: vi.fn(),
    getMySession: vi.fn(),
    getMyPermissions: vi.fn(),
    logout: vi.fn(),
    changePassword: vi.fn(),
    getSecuritySettings: vi.fn(),
    updateSecuritySettings: vi.fn(),
    getLockoutStatus: vi.fn(),
    resetAccountLockout: vi.fn(),
    getActiveSessions: vi.fn(),
    terminateSessionForAdmin: vi.fn(),
    getRolePermissionsMatrix: vi.fn(),
    listUserRoles: vi.fn(),
    assignUserRole: vi.fn(),
    bootstrapFirstAdminIfEligible: vi.fn(),
    getSecurityDashboardMetrics: vi.fn(),
    getSecurityTrendSeries: vi.fn(),
    getRecentAuthEvents: vi.fn(),
    isRecoveryModeActive: vi.fn(),
    recoveryClearLockout: vi.fn(),
    recoveryResetSecurityPolicy: vi.fn(),
    recoverySetAdminPassword: vi.fn(),
  },
}));

describe('Stage 3 Auth & Shell', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.location.hash = '#/';
  });

  it('shows initial admin setup when needsSetup is true and transitions to ERP shell', async () => {
    vi.mocked(securityService.getAuthSetupStatus).mockResolvedValue({
      needsSetup: true,
      recoveryMode: false,
    });
    vi.mocked(securityService.completeAdminSetup).mockResolvedValue({
      sessionId: 'sess-1',
      userId: 'admin',
      username: 'admin',
      role: 'administrator',
      permissions: ['security.session_read', 'role.read', 'admin.activity_log', 'admin.user_activity'],
      rememberMe: false,
      createdAt: '2026-11-12T10:00:00Z',
      lastActive: '2026-11-12T10:00:00Z',
      expiresAt: '2026-11-12T22:00:00Z',
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('Initial Administrator Setup')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'StrongPass1!' } });
    fireEvent.change(screen.getByLabelText('Confirm Password'), { target: { value: 'StrongPass1!' } });
    fireEvent.click(screen.getByRole('button', { name: /Create Administrator & Continue/i }));

    await waitFor(() => {
      expect(securityService.completeAdminSetup).toHaveBeenCalledWith('admin', 'StrongPass1!');
      expect(screen.getByText(/ERP Executive Dashboard/i)).toBeInTheDocument();
      expect(screen.getByText(/Live SQL Aggregation/i)).toBeInTheDocument();
    });
  });

  it('renders sign-in screen with recovery panel when recoveryMode is active and signs in cleanly', async () => {
    vi.mocked(securityService.getAuthSetupStatus).mockResolvedValue({
      needsSetup: false,
      adminUsername: 'admin',
      recoveryMode: true,
    });
    vi.mocked(securityService.getMySession).mockRejectedValue(new Error('not signed in'));
    vi.mocked(securityService.authenticate).mockResolvedValue({
      sessionId: 'sess-2',
      userId: 'admin',
      username: 'admin',
      role: 'administrator',
      permissions: ['security.session_read'],
      rememberMe: true,
      createdAt: '2026-11-12T10:00:00Z',
      lastActive: '2026-11-12T10:00:00Z',
      expiresAt: '2026-12-12T10:00:00Z',
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('Desktop Sign-In')).toBeInTheDocument();
      expect(screen.getByText('Emergency Recovery Mode Active')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Recovery Tools/i }));
    expect(screen.getByRole('button', { name: /Clear Account Lockout/i })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'StrongPass1' } });
    fireEvent.click(screen.getByRole('button', { name: /Sign In/i }));

    await waitFor(() => {
      expect(securityService.authenticate).toHaveBeenCalledWith('admin', 'StrongPass1', false);
      expect(screen.getByText('Security Center')).toBeInTheDocument();
    });
  });

  it('toggles sidebar hide/unhide and switches Light/System/Dark mode', async () => {
    window.localStorage.clear();
    vi.mocked(securityService.getAuthSetupStatus).mockResolvedValue({
      needsSetup: false,
      adminUsername: 'admin',
      recoveryMode: false,
    });
    vi.mocked(securityService.getMySession).mockResolvedValue({
      sessionId: 'sess-3',
      userId: 'admin',
      username: 'admin',
      role: 'administrator',
      permissions: ['data.read', 'data.write'],
      rememberMe: false,
      createdAt: '',
      lastActive: '',
      expiresAt: '',
    });
    vi.mocked(settingsService.getProfile).mockResolvedValue({
      schemaVersion: 1,
      theme: 'default',
      mode: 'system',
      lineTotalDecimals: 2,
      invoiceTotalDecimals: 2,
      numberFormat: 'en-IN',
      dateFormat: 'DD/MM/YYYY',
      textFormat: 'preserve',
      modules: {},
      updatedAt: '',
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Hide sidebar' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Hide sidebar' }));
    expect(screen.getByRole('complementary', { name: 'Sidebar' })).toHaveAttribute('data-collapsed', 'true');
    expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Dashboard' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Shipments' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show sidebar' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show sidebar' }));
    expect(screen.getByRole('complementary', { name: 'Sidebar' })).toHaveAttribute('data-collapsed', 'false');
    expect(screen.getByText('Core Operations')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Dark mode' }));
    await waitFor(() => {
      expect(applyThemeProfile).toHaveBeenCalledWith({ theme: 'default', mode: 'dark' });
    });

    fireEvent.click(screen.getByRole('button', { name: 'Light mode' }));
    await waitFor(() => {
      expect(applyThemeProfile).toHaveBeenCalledWith({ theme: 'default', mode: 'light' });
    });
  });
});
