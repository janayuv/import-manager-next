import * as BackupBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/backup/service.js';
import * as DriveBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/google_drive/service.js';
import * as RecycleBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/recycle_bin/service.js';
import * as DBAdminBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/database_management/service.js';
import * as MobileSyncBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/mobile_sync/controlservice.js';
import type { ServerInfo as MobileSyncServerInfo } from '../../bindings/github.com/janayuv/import-manager-next/backend/services/mobile_sync/models.js';
import type {
  BackupHealthStatus,
  BackupRecord,
  BackupSchedule,
  BrowseTableRequest,
  BrowseTableResponse,
  BulkDeleteRequest,
  BulkDeleteResult,
  CreateBackupRequest,
  DatabaseOverviewStats,
  DependencyCount,
  GoogleDriveConfigInput,
  GoogleDriveRemoteFile,
  GoogleDriveStatus,
  IMKeyOperationResult,
  LegacyInspectRequest,
  LegacyInspectResult,
  LegacyMigrationReport,
  LegacyMigrationRequest,
  MissingParentGroup,
  RecycleBinActionResult,
  RecycleBinListResponse,
  RecycleBinRecord,
  RestoreBackupRequest,
  RestoreVerificationSummary,
  SaveBackupScheduleInput,
  UpdateTableCellRequest,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  BackupHealthStatus,
  BackupRecord,
  BackupSchedule,
  BrowseTableRequest,
  BrowseTableResponse,
  BulkDeleteRequest,
  BulkDeleteResult,
  CreateBackupRequest,
  DatabaseOverviewStats,
  DependencyCount,
  GoogleDriveConfigInput,
  GoogleDriveRemoteFile,
  GoogleDriveStatus,
  IMKeyOperationResult,
  LegacyInspectRequest,
  LegacyInspectResult,
  LegacyMigrationReport,
  LegacyMigrationRequest,
  MissingParentGroup,
  RecycleBinActionResult,
  RecycleBinListResponse,
  RecycleBinRecord,
  RestoreBackupRequest,
  RestoreVerificationSummary,
  SaveBackupScheduleInput,
  UpdateTableCellRequest,
};
export type { MobileSyncServerInfo };

export const backupService = {
  createBackup(req: CreateBackupRequest): Promise<BackupRecord> {
    return BackupBindings.CreateBackup(req);
  },
  listBackups(): Promise<BackupRecord[]> {
    return BackupBindings.ListBackups();
  },
  deleteBackupRecord(id: number, deleteFile: boolean): Promise<void> {
    return BackupBindings.DeleteBackupRecord(id, deleteFile);
  },
  deleteBackupFile(filePath: string): Promise<void> {
    return BackupBindings.DeleteBackupFile(filePath);
  },
  restoreBackup(req: RestoreBackupRequest): Promise<RestoreVerificationSummary> {
    return BackupBindings.RestoreBackup(req);
  },
  saveBackupPassphrase(passphrase: string): Promise<void> {
    return BackupBindings.SaveBackupPassphrase(passphrase);
  },
  hasSavedBackupPassphrase(): Promise<boolean> {
    return BackupBindings.HasSavedBackupPassphrase();
  },
  clearBackupPassphrase(): Promise<void> {
    return BackupBindings.ClearBackupPassphrase();
  },
  exportIMKey(outputPath: string, backupPassphrase: string, transportPassphrase: string): Promise<IMKeyOperationResult> {
    return BackupBindings.ExportIMKey(outputPath, backupPassphrase, transportPassphrase);
  },
  importIMKey(filePath: string, transportPassphrase: string): Promise<IMKeyOperationResult> {
    return BackupBindings.ImportIMKey(filePath, transportPassphrase);
  },
  listSchedules(): Promise<BackupSchedule[]> {
    return BackupBindings.ListBackupSchedules();
  },
  saveSchedule(input: SaveBackupScheduleInput): Promise<BackupSchedule> {
    return BackupBindings.SaveBackupSchedule(input);
  },
  deleteSchedule(id: number): Promise<void> {
    return BackupBindings.DeleteBackupSchedule(id);
  },
  getBackupHealth(): Promise<BackupHealthStatus> {
    return BackupBindings.GetBackupHealth();
  },
};

export const googleDriveService = {
  saveClientConfig(input: GoogleDriveConfigInput): Promise<void> {
    return DriveBindings.SaveClientConfig(input);
  },
  getStatus(): Promise<GoogleDriveStatus> {
    return DriveBindings.GetStatus();
  },
  startOAuthFlow(): Promise<string> {
    return DriveBindings.StartOAuthFlow();
  },
  completeOAuthCode(code: string, state: string): Promise<GoogleDriveStatus> {
    return DriveBindings.CompleteOAuthCode(code, state);
  },
  disconnect(): Promise<void> {
    return DriveBindings.Disconnect();
  },
  uploadBackup(localFilePath: string): Promise<GoogleDriveRemoteFile> {
    return DriveBindings.UploadBackup(localFilePath);
  },
  listBackups(): Promise<GoogleDriveRemoteFile[]> {
    return DriveBindings.ListBackups();
  },
  downloadBackup(fileIDOrLocator: string, targetFilename: string): Promise<string> {
    return DriveBindings.DownloadBackup(fileIDOrLocator, targetFilename);
  },
  deleteBackup(fileIDOrLocator: string): Promise<void> {
    return DriveBindings.DeleteBackup(fileIDOrLocator);
  },
};

export const recycleBinService = {
  getSoftDeleteTables(): Promise<string[]> {
    return RecycleBindings.GetSoftDeleteTables();
  },
  getDeletedCount(): Promise<number> {
    return RecycleBindings.GetRecycleBinDeletedCount();
  },
  getDeletedRecords(tableName: string, search: string, page: number, pageSize: number): Promise<RecycleBinListResponse> {
    return RecycleBindings.GetDeletedRecords(tableName, search, page, pageSize);
  },
  restoreRecords(tableName: string, recordIDs: string[]): Promise<RecycleBinActionResult> {
    return RecycleBindings.RestoreDeletedRecords(tableName, recordIDs);
  },
  permanentlyDeleteRecords(tableName: string, recordIDs: string[]): Promise<RecycleBinActionResult> {
    return RecycleBindings.PermanentlyDeleteRecords(tableName, recordIDs);
  },
  purgeExpired(retentionDays: number): Promise<RecycleBinActionResult> {
    return RecycleBindings.PurgeExpiredRecords(retentionDays);
  },
};

export const databaseAdminService = {
  listBrowseableTables(): Promise<string[]> {
    return DBAdminBindings.ListBrowseableTables();
  },
  getOverviewStats(): Promise<DatabaseOverviewStats> {
    return DBAdminBindings.GetOverviewStats();
  },
  browseTable(req: BrowseTableRequest): Promise<BrowseTableResponse> {
    return DBAdminBindings.BrowseTable(req);
  },
  updateTableCell(req: UpdateTableCellRequest): Promise<void> {
    return DBAdminBindings.UpdateTableCell(req);
  },
  setHardDeletePin(currentPassword: string, newPin: string): Promise<void> {
    return DBAdminBindings.SetHardDeletePin(currentPassword, newPin);
  },
  bulkDelete(req: BulkDeleteRequest): Promise<BulkDeleteResult> {
    return DBAdminBindings.BulkDelete(req);
  },
  undoBulkSoftDelete(undoToken: string): Promise<number> {
    return DBAdminBindings.UndoBulkSoftDelete(undoToken);
  },
  runIntegrityAndVacuum(): Promise<string> {
    return DBAdminBindings.RunIntegrityAndVacuum();
  },
  inspectLegacyDatabase(req: LegacyInspectRequest): Promise<LegacyInspectResult> {
    return DBAdminBindings.InspectLegacyDatabase(req);
  },
  runLegacyMigration(req: LegacyMigrationRequest): Promise<LegacyMigrationReport> {
    return DBAdminBindings.RunLegacyMigration(req);
  },
};

export const mobileSyncService = {
  getServerInfo(): Promise<MobileSyncServerInfo> {
    return MobileSyncBindings.GetServerInfo();
  },
  setEnabled(enabled: boolean): Promise<MobileSyncServerInfo> {
    return MobileSyncBindings.SetEnabled(enabled);
  },
};
