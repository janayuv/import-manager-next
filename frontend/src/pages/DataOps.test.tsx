import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { DatabaseBackupsPage } from '@/pages/DatabaseBackupsPage';
import { RecycleBinPage } from '@/pages/RecycleBinPage';

const mockGetOverview = vi.fn().mockResolvedValue({
  databasePath: 'C:\\AppData\\ImportManager\\import_manager.db',
  cipherVersion: '4.4.2 community',
  journalMode: 'wal',
  foreignKeysOn: true,
  integrityCheck: 'ok',
  pageCount: 256,
  pageSize: 4096,
  fileSizeBytes: 1048576,
  schemaVersion: 5,
  restoreLocked: false,
  tableCounts: { suppliers: 11, shipments: 4 },
  deletedCounts: { suppliers: 1, shipments: 0 },
});

const mockListTables = vi.fn().mockResolvedValue(['suppliers', 'shipments']);
const mockListBackups = vi.fn().mockResolvedValue([]);
const mockListSchedules = vi.fn().mockResolvedValue([]);
const mockGetDriveStatus = vi.fn().mockResolvedValue({
  connected: false,
  hasClientConfig: false,
  folderName: 'ImportManagerBackups',
  retentionLimit: 30,
});
const mockHasSavedPass = vi.fn().mockResolvedValue(true);
const mockInspectLegacy = vi.fn().mockResolvedValue({
  sourcePath: 'C:\\legacy\\import_manager.db',
  fileSizeBytes: 20480,
  isPlaintextSqlite: true,
  isEncrypted: false,
  keyValid: true,
  migrationVersion: '80',
  tables: [
    { tableName: 'suppliers', sourceExists: true, sourceRows: 12, targetRows: 11, matchedCols: ['id', 'supplier_name'] },
  ],
  sourceFinancials: { invoiceValueSum: 50000, boeTotalDutyPaise: 450000, expenseAmountPaise: 100000, expenseTotalPaise: 118000 },
});
const mockRunLegacy = vi.fn().mockResolvedValue({
  dryRun: true,
  sourcePath: 'C:\\legacy\\import_manager.db',
  integrityCheck: 'ok',
  foreignKeysOk: true,
  foreignKeyErrors: 0,
  rowsCopiedByTable: { suppliers: 12 },
  rowParityOk: true,
  sourceFinancials: { invoiceValueSum: 50000, boeTotalDutyPaise: 450000, expenseAmountPaise: 100000, expenseTotalPaise: 118000 },
  targetFinancials: { invoiceValueSum: 50000, boeTotalDutyPaise: 450000, expenseAmountPaise: 100000, expenseTotalPaise: 118000 },
  financialParityOk: true,
  completedAt: '2026-11-12T10:00:00Z',
});

const mockGetSoftTables = vi.fn().mockResolvedValue([
  'expenses', 'expense_invoices', 'boe_calculations', 'boe_details',
  'invoice_line_items', 'invoices', 'shipments', 'items', 'suppliers',
  'service_providers', 'expense_types',
]);
const mockGetDeletedRecords = vi.fn().mockResolvedValue({
  tableName: 'all',
  search: '',
  page: 1,
  pageSize: 50,
  total: 1,
  aggregateDeleted: 1,
  tableSumDeleted: 1,
  countMismatch: false,
  perTableCounts: { shipments: 1 },
  rows: [
    {
      tableName: 'shipments',
      recordId: 'shp-del-1',
      label: 'INV-OLD-1 (AIR)',
      summary: '2026-09-01 · USD',
      deletedAt: '2026-11-01T12:00:00Z',
      deletedBy: 'admin',
    },
  ],
});
const mockRestoreRecords = vi.fn().mockResolvedValue({
  action: 'restore',
  tableName: 'shipments',
  processedCount: 0,
  skippedCount: 1,
  blocked: true,
  errorCode: 'missing_parent',
  message: 'Cannot restore record(s) because referenced parent records are missing or deleted.',
  missingParents: [{ tableName: 'suppliers', parentIds: ['sup-99'] }],
});

vi.mock('@/services/data-ops-service', () => ({
  databaseAdminService: {
    getOverviewStats: () => mockGetOverview(),
    listBrowseableTables: () => mockListTables(),
    browseTable: vi.fn(),
    updateTableCell: vi.fn(),
    bulkDelete: vi.fn(),
    undoBulkSoftDelete: vi.fn(),
    runIntegrityAndVacuum: vi.fn().mockResolvedValue('integrity_check=ok'),
    inspectLegacyDatabase: (req: unknown) => mockInspectLegacy(req),
    runLegacyMigration: (req: unknown) => mockRunLegacy(req),
  },
  backupService: {
    listBackups: () => mockListBackups(),
    listSchedules: () => mockListSchedules(),
    hasSavedBackupPassphrase: () => mockHasSavedPass(),
    createBackup: vi.fn(),
    restoreBackup: vi.fn(),
    exportIMKey: vi.fn(),
    importIMKey: vi.fn(),
    saveSchedule: vi.fn(),
    deleteSchedule: vi.fn(),
  },
  googleDriveService: {
    getStatus: () => mockGetDriveStatus(),
    listBackups: vi.fn().mockResolvedValue([]),
    saveClientConfig: vi.fn(),
    startOAuthFlow: vi.fn(),
    completeOAuthCode: vi.fn(),
    disconnect: vi.fn(),
    uploadBackup: vi.fn(),
    downloadBackup: vi.fn(),
  },
  recycleBinService: {
    getSoftDeleteTables: () => mockGetSoftTables(),
    getDeletedRecords: (...args: unknown[]) => mockGetDeletedRecords(...args),
    restoreRecords: (...args: unknown[]) => mockRestoreRecords(...args),
    permanentlyDeleteRecords: vi.fn(),
    purgeExpired: vi.fn(),
  },
}));

describe('Stage 9 Data Ops Pages', () => {
  it('renders Database & Backups Console overview and runs Legacy Migration dry-run parity', async () => {
    render(
      <MemoryRouter>
        <DatabaseBackupsPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('4.4.2 community')).toBeInTheDocument();
      expect(screen.getByText('Schema v5')).toBeInTheDocument();
    });

    const legacyTab = screen.getByRole('tab', { name: 'Legacy Migration' });
    fireEvent.mouseDown(legacyTab, { button: 0 });
    fireEvent.click(legacyTab);

    const pathInput = await screen.findByPlaceholderText(/legacy import_manager\.db/i);
    fireEvent.change(pathInput, { target: { value: 'C:\\legacy\\import_manager.db' } });
    fireEvent.click(screen.getByRole('button', { name: 'Inspect DB' }));

    await waitFor(() => {
      expect(screen.getByText(/Plaintext SQLite/i)).toBeInTheDocument();
      expect(screen.getByText(/Legacy Schema v80/i)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Dry-Run Parity' }));
    await waitFor(() => {
      expect(screen.getByText(/Dry-Run Simulation Report/i)).toBeInTheDocument();
    });
  });

  it('renders Recycle Bin and displays missing_parent blocker with copyable parent IDs', async () => {
    render(
      <MemoryRouter>
        <RecycleBinPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('INV-OLD-1 (AIR)')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Restore' }));
    await waitFor(() => {
      expect(screen.getByText(/Operation Blocked \(Missing Parent Records\)/i)).toBeInTheDocument();
      expect(screen.getByText('sup-99')).toBeInTheDocument();
    });
  });
});
