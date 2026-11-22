import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ActivityLogPage } from '@/pages/admin/ActivityLogPage';
import { SecurityCenterPage } from '@/pages/admin/SecurityCenterPage';
import { UserActivityPage } from '@/pages/admin/UserActivityPage';
import { WorkspaceOverviewPage } from '@/pages/WorkspaceOverviewPage';
import { auditService, type AuditEntry, type UserActivityEntry } from '@/services/audit-service';
import { securityService } from '@/services/security-service';
import { fetchBootstrapInfo, listSchemaMigrations, type MigrationRecord } from '@/services/app-service';

vi.mock('@/contexts/AuthContext', () => ({
  useAuth: () => ({
    session: {
      userId: 'admin',
      username: 'admin',
      role: 'administrator',
      permissions: ['data.read', 'data.write', 'data.delete', 'security.session_read'],
    },
  }),
}));

vi.mock('@/services/audit-service', () => ({
  auditService: {
    queryAuditLogs: vi.fn(),
    queryUserActivityLogs: vi.fn(),
  },
}));

vi.mock('@/services/security-service', () => ({
  securityService: {
    getSecurityDashboardMetrics: vi.fn(),
    getSecurityTrendSeries: vi.fn(),
    getSecuritySettings: vi.fn(),
    getActiveSessions: vi.fn(),
    getLockoutStatus: vi.fn(),
    getRecentAuthEvents: vi.fn(),
    updateSecuritySettings: vi.fn(),
    terminateSession: vi.fn(),
    adminUnlockAccount: vi.fn(),
  },
}));

vi.mock('@/services/app-service', () => ({
  fetchBootstrapInfo: vi.fn(),
  listSchemaMigrations: vi.fn(),
}));

describe('Admin and Workspace Tables Pagination (5 rows per page)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Database Audit Trail (ActivityLogPage)', () => {
    it('defaults to 5 rows per page and paginates through records', async () => {
      const mockAuditEntries: AuditEntry[] = Array.from({ length: 12 }, (_, i) => ({
        id: i + 1,
        tableName: `table_${i + 1}`,
        rowId: `row-${i + 1}`,
        action: i % 2 === 0 ? 'INSERT' : 'UPDATE',
        userId: 'admin',
        beforeJson: '',
        afterJson: `{"item": ${i + 1}}`,
        metadata: '',
        createdAt: `2026-11-22 10:0${i}:00`,
      }));

      vi.mocked(auditService.queryAuditLogs).mockResolvedValue(mockAuditEntries);

      render(
        <MemoryRouter>
          <ActivityLogPage />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Database Audit Trail')).toBeInTheDocument();
      });

      // Check pagination text
      expect(screen.getByText(/Showing 1 to 5 of 12 entries/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 1 of 3/i)).toBeInTheDocument();

      // Check rows: table_1 to table_5 should be rendered, table_6 should not
      expect(screen.getByText('table_1')).toBeInTheDocument();
      expect(screen.getByText('table_5')).toBeInTheDocument();
      expect(screen.queryByText('table_6')).not.toBeInTheDocument();

      // Previous button should be disabled, Next enabled
      const prevBtn = screen.getByRole('button', { name: /previous/i });
      const nextBtn = screen.getByRole('button', { name: /next/i });
      expect(prevBtn).toBeDisabled();
      expect(nextBtn).toBeEnabled();

      // Click Next
      fireEvent.click(nextBtn);

      expect(screen.getByText(/Showing 6 to 10 of 12 entries/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 2 of 3/i)).toBeInTheDocument();
      expect(screen.queryByText('table_1')).not.toBeInTheDocument();
      expect(screen.getByText('table_6')).toBeInTheDocument();
      expect(screen.getByText('table_10')).toBeInTheDocument();

      // Click Next again to page 3
      fireEvent.click(nextBtn);
      expect(screen.getByText(/Showing 11 to 12 of 12 entries/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 3 of 3/i)).toBeInTheDocument();
      expect(nextBtn).toBeDisabled();

      // Click Previous back to page 2
      fireEvent.click(prevBtn);
      expect(screen.getByText(/Page 2 of 3/i)).toBeInTheDocument();

      // Check rows per page dropdown selection
      expect(screen.getByText('Rows per page:')).toBeInTheDocument();
    });

    it('displays empty state message when no audit records match', async () => {
      vi.mocked(auditService.queryAuditLogs).mockResolvedValue([]);

      render(
        <MemoryRouter>
          <ActivityLogPage />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText(/No matching audit records found/i)).toBeInTheDocument();
      });
      expect(screen.getByText(/Showing 0 to 0 of 0 entries/i)).toBeInTheDocument();
    });
  });

  describe('Activity Stream (UserActivityPage)', () => {
    it('defaults to 5 rows per page and allows page navigation', async () => {
      const mockUserActivities: UserActivityEntry[] = Array.from({ length: 11 }, (_, i) => ({
        id: `act-${i + 1}`,
        timestamp: `2026-11-22 12:0${i}:00`,
        userId: 'admin',
        actionName: `ACTION_${i + 1}`,
        entityType: 'user',
        entityId: `u-${i + 1}`,
        status: 'SUCCESS',
        severity: 'INFO',
        detailsJson: `{"op": ${i + 1}}`,
      }));

      vi.mocked(auditService.queryUserActivityLogs).mockResolvedValue(mockUserActivities);

      render(
        <MemoryRouter>
          <UserActivityPage />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Activity Stream')).toBeInTheDocument();
      });

      expect(screen.getByText(/Showing 1 to 5 of 11 entries/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 1 of 3/i)).toBeInTheDocument();

      expect(screen.getByText('ACTION_1')).toBeInTheDocument();
      expect(screen.getByText('ACTION_5')).toBeInTheDocument();
      expect(screen.queryByText('ACTION_6')).not.toBeInTheDocument();

      const nextBtn = screen.getByRole('button', { name: /next/i });
      fireEvent.click(nextBtn);

      expect(screen.getByText(/Showing 6 to 10 of 11 entries/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 2 of 3/i)).toBeInTheDocument();
      expect(screen.getByText('ACTION_6')).toBeInTheDocument();
    });
  });

  describe('Applied Schema Migrations (WorkspaceOverviewPage)', () => {
    it('defaults to 5 rows per page for schema migrations', async () => {
      const mockMigrations: MigrationRecord[] = Array.from({ length: 14 }, (_, i) => ({
        version: i + 1,
        name: `migration_step_${i + 1}`,
        status: 'APPLIED',
        checksum: `checksum_hash_${i + 1}_abcdef123456`,
        executedAt: `2026-11-22 08:00:00`,
        appVersion: '1.9.4',
        durationMs: 10,
      }));

      vi.mocked(fetchBootstrapInfo).mockResolvedValue({
        appName: 'Import Manager',
        appVersion: '1.9.4',
        buildTimestamp: '',
        goVersion: 'go1.22.0',
        os: 'windows',
        arch: 'amd64',
        recoveryMode: false,
        paths: {
          dataDir: 'C:\\data',
          databasePath: 'C:\\data\\test.db',
          backupsDir: '',
          attachmentsDir: '',
          logsDir: '',
          exportsDir: '',
          tempDir: '',
        },
        database: {
          path: 'C:\\data\\test.db',
          cipherVersion: '4.4.2',
          journalMode: 'wal',
          foreignKeys: true,
          integrityCheck: 'ok',
          schemaVersion: 14,
          encrypted: true,
        },
        keystore: {
          initialized: true,
          keySource: 'embedded',
          slot: 0,
        },
      } as any);
      vi.mocked(listSchemaMigrations).mockResolvedValue(mockMigrations);

      render(
        <MemoryRouter>
          <WorkspaceOverviewPage />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Applied Schema Migrations')).toBeInTheDocument();
      });

      expect(screen.getByText(/Showing 1 to 5 of 14 migrations/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 1 of 3/i)).toBeInTheDocument();

      expect(screen.getByText('migration_step_1')).toBeInTheDocument();
      expect(screen.getByText('migration_step_5')).toBeInTheDocument();
      expect(screen.queryByText('migration_step_6')).not.toBeInTheDocument();

      const nextBtn = screen.getByRole('button', { name: /next/i });
      fireEvent.click(nextBtn);

      expect(screen.getByText(/Showing 6 to 10 of 14 migrations/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 2 of 3/i)).toBeInTheDocument();
      expect(screen.getByText('migration_step_6')).toBeInTheDocument();
    });
  });

  describe('Recent Security & Authentication Events (SecurityCenterPage)', () => {
    it('defaults to 5 rows per page for security audit events', async () => {
      const mockEvents: UserActivityEntry[] = Array.from({ length: 9 }, (_, i) => ({
        id: `sec-ev-${i + 1}`,
        timestamp: `2026-11-22 14:0${i}:00`,
        userId: 'admin',
        actionName: `AUTH_EVENT_${i + 1}`,
        entityType: 'session',
        entityId: `sess-${i + 1}`,
        status: 'SUCCESS',
        severity: 'INFO',
        detailsJson: '{}',
      }));

      vi.mocked(securityService.getSecurityDashboardMetrics).mockResolvedValue({
        activeSessions: 1,
        lockedAccounts: 0,
        failedLogins24h: 0,
        totalUsersWithRoles: 1,
        policyVersion: 1,
      });
      vi.mocked(securityService.getSecurityTrendSeries).mockResolvedValue([]);
      vi.mocked(securityService.getSecuritySettings).mockResolvedValue({
        policyVersion: 1,
        idleTimeoutMinutes: 30,
        lockoutThreshold: 5,
        lockoutWindowMinutes: 15,
        lockoutDurationMinutes: 15,
        passwordHistoryDepth: 5,
      });
      vi.mocked(securityService.getActiveSessions).mockResolvedValue([]);
      vi.mocked(securityService.getLockoutStatus).mockResolvedValue({
        userId: 'admin',
        isLocked: false,
        failureCount: 0,
        threshold: 5,
        remainingSecs: 0,
      });
      vi.mocked(securityService.getRecentAuthEvents).mockResolvedValue(mockEvents);

      render(
        <MemoryRouter>
          <SecurityCenterPage />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Recent Security & Authentication Events')).toBeInTheDocument();
      });

      expect(screen.getByText(/Showing 1 to 5 of 9 events/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 1 of 2/i)).toBeInTheDocument();

      expect(screen.getByText('AUTH_EVENT_1')).toBeInTheDocument();
      expect(screen.getByText('AUTH_EVENT_5')).toBeInTheDocument();
      expect(screen.queryByText('AUTH_EVENT_6')).not.toBeInTheDocument();

      const nextBtn = screen.getByRole('button', { name: /next/i });
      fireEvent.click(nextBtn);

      expect(screen.getByText(/Showing 6 to 9 of 9 events/i)).toBeInTheDocument();
      expect(screen.getByText(/Page 2 of 2/i)).toBeInTheDocument();
      expect(screen.getByText('AUTH_EVENT_6')).toBeInTheDocument();
    });
  });
});
