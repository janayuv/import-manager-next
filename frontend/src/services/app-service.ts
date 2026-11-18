import * as AppBindings from '../../bindings/github.com/janayuv/import-manager-next/appservice.js';
import type { BootstrapInfo } from '../../bindings/github.com/janayuv/import-manager-next/models.js';
import type { HealthInfo } from '../../bindings/github.com/janayuv/import-manager-next/backend/database/connection/models.js';
import type { AppliedRecord } from '../../bindings/github.com/janayuv/import-manager-next/backend/database/migrations/models.js';

export type BootstrapInfoDto = BootstrapInfo;
export type MigrationRecord = AppliedRecord;
export type { HealthInfo };

export interface UpdateCheckResult {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  releaseName: string;
  releaseNotes: string;
  publishedAt: string;
  releaseURL: string;
  downloadURL: string;
  assetName: string;
  assetSize: number;
  checkedAt: string;
  repository?: string;
  error?: string;
}

export function fetchBootstrapInfo(): Promise<BootstrapInfo> {
  return AppBindings.GetBootstrapInfo();
}

export function checkDatabaseHealth(): Promise<HealthInfo> {
  return AppBindings.CheckDatabaseHealth();
}

export function listSchemaMigrations(): Promise<AppliedRecord[]> {
  return AppBindings.ListSchemaMigrations().then((items) => items ?? []);
}

export async function checkForUpdate(): Promise<UpdateCheckResult> {
  try {
    return await AppBindings.CheckForUpdate();
  } catch (err) {
    const current = '1.0.0';
    return {
      currentVersion: current,
      latestVersion: current,
      updateAvailable: false,
      releaseName: '',
      releaseNotes: '',
      publishedAt: '',
      releaseURL: 'https://github.com/janayuv/import-manager-next/releases',
      downloadURL: '',
      assetName: '',
      assetSize: 0,
      checkedAt: new Date().toISOString(),
      repository: 'janayuv/import-manager-next',
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

export async function downloadUpdate(downloadURL: string, assetName: string): Promise<string> {
  return AppBindings.DownloadUpdate(downloadURL, assetName);
}

export async function applyUpdate(installerPath: string): Promise<void> {
  return AppBindings.ApplyUpdate(installerPath);
}

export async function openExternalURL(url: string): Promise<void> {
  return AppBindings.OpenExternalURL(url);
}

export interface MemoryStats {
  workingSetBytes: number;
  peakWorkingSetBytes: number;
  privateUsageBytes: number;
  heapAlloc: number;
  heapSys: number;
  heapIdle: number;
  heapInuse: number;
  heapReleased: number;
  stackInuse: number;
  numGC: number;
  lastGCTime: string;
  memoryLimitBytes: number;
  lastTrimTime: string;
  lastTrimReclaimed: number;
  autoTrimEnabled: boolean;
  isIdle: boolean;
}

export interface TrimResult {
  workingSetBefore: number;
  workingSetAfter: number;
  heapAllocBefore: number;
  heapAllocAfter: number;
  reclaimedBytes: number;
  durationMs: number;
  aggressive: boolean;
}

export async function getMemoryStats(): Promise<MemoryStats> {
  try {
    const stats = await AppBindings.GetMemoryStats();
    return stats ?? {
      workingSetBytes: 0,
      peakWorkingSetBytes: 0,
      privateUsageBytes: 0,
      heapAlloc: 0,
      heapSys: 0,
      heapIdle: 0,
      heapInuse: 0,
      heapReleased: 0,
      stackInuse: 0,
      numGC: 0,
      lastGCTime: '',
      memoryLimitBytes: 512 * 1024 * 1024,
      lastTrimTime: '',
      lastTrimReclaimed: 0,
      autoTrimEnabled: true,
      isIdle: false,
    };
  } catch {
    return {
      workingSetBytes: 0,
      peakWorkingSetBytes: 0,
      privateUsageBytes: 0,
      heapAlloc: 0,
      heapSys: 0,
      heapIdle: 0,
      heapInuse: 0,
      heapReleased: 0,
      stackInuse: 0,
      numGC: 0,
      lastGCTime: '',
      memoryLimitBytes: 512 * 1024 * 1024,
      lastTrimTime: '',
      lastTrimReclaimed: 0,
      autoTrimEnabled: true,
      isIdle: false,
    };
  }
}

export async function trimMemory(aggressive = true): Promise<TrimResult> {
  try {
    const res = await AppBindings.TrimMemory(aggressive);
    return res ?? {
      workingSetBefore: 0,
      workingSetAfter: 0,
      heapAllocBefore: 0,
      heapAllocAfter: 0,
      reclaimedBytes: 0,
      durationMs: 0,
      aggressive,
    };
  } catch {
    return {
      workingSetBefore: 0,
      workingSetAfter: 0,
      heapAllocBefore: 0,
      heapAllocAfter: 0,
      reclaimedBytes: 0,
      durationMs: 0,
      aggressive,
    };
  }
}
