import { Call } from '@wailsio/runtime';
import type { BackupRecord } from './data-ops-service';

export interface JobHealthSummary {
  jobId: string;
  jobName: string;
  isEnabled: boolean;
  lastStatus: string;
  lastStartedAt: string;
  lastCompletedAt: string;
  expectedDurationMs: number;
  isStuck: boolean;
  errorMessage?: string;
}

export interface SystemHealthSummary {
  overallStatus: 'HEALTHY' | 'WARNING' | 'CRITICAL' | string;
  evaluatedAt: string;

  // 1. Database & SQLCipher
  databaseIntegrity: string;
  lastIntegrityCheckAt: string;
  foreignKeysEnforced: boolean;
  sqlCipherConnected: boolean;
  diskHeaderEncrypted: boolean;
  cipherVersion: string;
  databasePath: string;
  databaseSizeBytes: number;
  databasePageCount: number;
  schemaVersion: number;

  // 2. Storage
  databaseDiskTotalBytes: number;
  databaseDiskFreeBytes: number;
  backupDiskTotalBytes: number;
  backupDiskFreeBytes: number;
  lowDiskWarning: boolean;

  // 3. Application Process Memory
  memoryWorkingSetBytes: number;
  memoryPrivateUsageBytes: number;
  goHeapAllocBytes: number;
  goHeapSysBytes: number;

  // 4. Background Job Engine
  activeJobsCount: number;
  runningJobsCount: number;
  stuckJobsCount: number;
  failedJobs24hCount: number;
  jobSummaries: JobHealthSummary[];

  // 5. Encrypted Backup Verification
  backupReadinessStatus: 'HEALTHY' | 'WARNING' | 'CRITICAL' | string;
  totalBackupsCount: number;
  verifiedBackupsCount: number;
  lastSuccessfulBackupAt: string;
  lastSuccessfulFilename: string;
  lastTestedRestoreAt: string;
  hoursSinceLastBackup: number;

  // 6. Companion & Remote Synchronizations
  mobileSyncRunning: boolean;
  mobileSyncPort: number;
  mobileConnectedDevices: number;
  googleDriveConnected: boolean;
  googleDriveEmailMasked: string;
  googleDriveTokenExpiry: string;
}

export async function fetchSystemHealthSummary(): Promise<SystemHealthSummary> {
  return Call.ByName('main.AppService.GetSystemHealthSummary');
}

export async function runManualIntegrityCheck(): Promise<string> {
  return Call.ByName('main.AppService.RunManualIntegrityCheck');
}

export async function runVerifiedBackup(): Promise<BackupRecord> {
  return Call.ByName('main.AppService.RunVerifiedBackup');
}

export async function runIdempotentJobRecovery(jobId: string): Promise<void> {
  return Call.ByName('main.AppService.RunIdempotentJobRecovery', jobId);
}
