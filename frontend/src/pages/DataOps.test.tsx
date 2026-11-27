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
const mockDeleteBackupRecord = vi.fn().mockResolvedValue(undefined);
const mockDeleteBackupFile = vi.fn().mockResolvedValue(undefined);
const mockListSchedules = vi.fn().mockResolvedValue([]);
const mockGetDriveStatus = vi.fn().mockResolvedValue({
  connected: false,
  hasClientConfig: false,
  folderName: 'ImportManagerBackups',
  retentionLimit: 30,
});
const mockListDriveBackups = vi.fn().mockResolvedValue([]);
const mockDeleteDriveBackup = vi.fn().mockResolvedValue(undefined);
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
    deleteBackupRecord: (...args: unknown[]) => mockDeleteBackupRecord(...args),
    deleteBackupFile: (...args: unknown[]) => mockDeleteBackupFile(...args),
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
    listBackups: () => mockListDriveBackups(),
    deleteBackup: (...args: unknown[]) => mockDeleteDriveBackup(...args),
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
  mobileSyncService: {
    getServerInfo: vi.fn().mockResolvedValue({
      enabled: false,
      isRunning: false,
      port: 8766,
      addresses: [],
      version: '1.9.5',
    }),
    setEnabled: vi.fn().mockResolvedValue({
      enabled: true,
      isRunning: true,
      port: 8766,
      addresses: ['http://192.168.1.100:8766'],
      version: '1.9.5',
    }),
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

  it('allows removing local backup file and Google Drive backup file with confirmation', async () => {
    mockListBackups.mockResolvedValueOnce([
      {
        id: 101,
        filename: 'backup_20261117.imbk2',
        path: 'C:\\backups\\backup_20261117.imbk2',
        sizeBytes: 2048,
        sha256: 'abcdef1234567890',
        createdAt: '2026-11-17T10:00:00Z',
        notes: 'Test backup',
      },
    ]);
    mockGetDriveStatus.mockResolvedValueOnce({
      connected: true,
      hasClientConfig: true,
      folderName: 'ImportManagerBackups',
      retentionLimit: 30,
      accountEmail: 'test@example.com',
    });
    mockListDriveBackups.mockResolvedValueOnce([
      {
        id: 'gdrive-file-1',
        name: 'gdrive_backup.imbk2',
        locator: 'gdrive:gdrive-file-1',
        sizeBytes: 4096,
        createdTime: '2026-11-17T10:00:00Z',
      },
    ]);

    render(
      <MemoryRouter>
        <DatabaseBackupsPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('4.4.2 community')).toBeInTheDocument();
    });

    // Navigate to Backups & Drive tab
    const backupTab = screen.getByRole('tab', { name: 'Backups & Drive' });
    fireEvent.mouseDown(backupTab, { button: 0 });
    fireEvent.click(backupTab);

    // Verify local backup and Drive backup are visible
    await waitFor(() => {
      expect(screen.getByText('backup_20261117.imbk2')).toBeInTheDocument();
      expect(screen.getByText(/gdrive_backup\.imbk2/)).toBeInTheDocument();
    });

    // 1. Delete Local Backup
    const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
    expect(deleteButtons.length).toBeGreaterThanOrEqual(2);

    // Click Delete on local backup
    fireEvent.click(deleteButtons[1]!); // local backup delete button is in local backups section

    await waitFor(() => {
      expect(screen.getByText('Delete Local Backup')).toBeInTheDocument();
      expect(screen.getByText('Delete backup file (.imbk2) and checksum (.sha256) from disk')).toBeInTheDocument();
    });

    // Click confirm delete in dialog
    const confirmLocalBtn = screen.getByRole('button', { name: 'Delete Backup' });
    fireEvent.click(confirmLocalBtn);

    await waitFor(() => {
      expect(mockDeleteBackupRecord).toHaveBeenCalledWith(101, true);
    });

    // 2. Delete Google Drive Backup
    // Click Delete on Google Drive backup
    fireEvent.click(deleteButtons[0]!); // first delete button is on Drive backup

    await waitFor(() => {
      expect(screen.getByText('Delete Google Drive Backup')).toBeInTheDocument();
      expect(screen.getByText('ID: gdrive-file-1')).toBeInTheDocument();
    });

    // Click confirm delete in dialog
    const confirmDriveBtn = screen.getByRole('button', { name: 'Delete from Drive' });
    fireEvent.click(confirmDriveBtn);

    await waitFor(() => {
      expect(mockDeleteDriveBackup).toHaveBeenCalledWith('gdrive:gdrive-file-1');
    });
  });
});
