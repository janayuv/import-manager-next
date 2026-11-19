import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UpdateDialog } from '@/components/system/UpdateDialog';
import {
  applyUpdate,
  checkForUpdate,
  downloadUpdate,
  fetchBootstrapInfo,
  openExternalURL,
} from '@/services/app-service';

vi.mock('@/services/app-service', () => ({
  fetchBootstrapInfo: vi.fn(),
  checkForUpdate: vi.fn(),
  downloadUpdate: vi.fn(),
  applyUpdate: vi.fn(),
  openExternalURL: vi.fn(),
}));

describe('In-App Updates from GitHub', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(fetchBootstrapInfo).mockResolvedValue({
      appName: 'Import Manager',
      appVersion: '1.0.0',
      goVersion: 'go1.26',
      os: 'windows',
      arch: 'amd64',
      recoveryMode: false,
      schemaVersion: 5,
      paths: {
        dataDir: '',
        databasePath: '',
        backupsDir: '',
        attachmentsDir: '',
        logsDir: '',
        exportsDir: '',
        tempDir: '',
      },
      database: {
        cipherVersion: '4.4.2',
        journalMode: 'wal',
        foreignKeysOn: true,
        integrityCheck: 'ok',
        encrypted: true,
      },
      integrity: {
        integrityOk: true,
        integrityMessages: ['ok'],
        foreignKeyErrors: 0,
        tableCount: 15,
        viewCount: 3,
        recoveredWrites: 0,
      },
      memory: {
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
        memoryLimitBytes: 0,
        lastTrimTime: '',
        lastTrimReclaimed: 0,
        autoTrimEnabled: false,
        isIdle: false,
      },
    });
  });

  it('renders UpdateDialog, checks for updates, and shows available release', async () => {
    vi.mocked(checkForUpdate).mockResolvedValue({
      currentVersion: '1.0.0',
      latestVersion: '1.1.0',
      updateAvailable: true,
      releaseName: 'Import Manager v1.1.0',
      releaseNotes: '* Added GitHub inapp updater\n* Fixed dropdowns',
      publishedAt: '2026-11-14T10:00:00Z',
      releaseURL: 'https://github.com/janayuv/import-manager-next/releases/tag/v1.1.0',
      downloadURL: 'https://github.com/janayuv/import-manager-next/releases/download/v1.1.0/import-manager-installer.exe',
      assetName: 'import-manager-installer.exe',
      assetSize: 52428800,
      checkedAt: new Date().toISOString(),
      repository: 'janayuv/import-manager-next',
    });

    const onOpenChange = vi.fn();
    render(<UpdateDialog open={true} onOpenChange={onOpenChange} />);

    await waitFor(() => {
      expect(screen.getByText(/New version v1\.1\.0 is available!/i)).toBeInTheDocument();
    });

    expect(screen.getByText(/Import Manager v1\.1\.0/i)).toBeInTheDocument();
    expect(screen.getByText(/Added GitHub inapp updater/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Update from GitHub/i })).toBeInTheDocument();
  });

  it('downloads and applies update when clicking Update from GitHub', async () => {
    vi.mocked(checkForUpdate).mockResolvedValue({
      currentVersion: '1.0.0',
      latestVersion: '1.1.0',
      updateAvailable: true,
      releaseName: 'Import Manager v1.1.0',
      releaseNotes: 'Update notes',
      publishedAt: '2026-11-14T10:00:00Z',
      releaseURL: 'https://github.com/janayuv/import-manager-next/releases/tag/v1.1.0',
      downloadURL: 'https://github.com/janayuv/import-manager-next/releases/download/v1.1.0/installer.exe',
      assetName: 'installer.exe',
      assetSize: 50000000,
      checkedAt: new Date().toISOString(),
      repository: 'janayuv/import-manager-next',
    });
    vi.mocked(downloadUpdate).mockResolvedValue('C:\\Users\\AppData\\updates\\installer.exe');
    vi.mocked(applyUpdate).mockResolvedValue(undefined);

    const onOpenChange = vi.fn();
    render(<UpdateDialog open={true} onOpenChange={onOpenChange} />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Update from GitHub/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Update from GitHub/i }));

    await waitFor(() => {
      expect(downloadUpdate).toHaveBeenCalledWith(
        'https://github.com/janayuv/import-manager-next/releases/download/v1.1.0/installer.exe',
        'installer.exe',
      );
      expect(applyUpdate).toHaveBeenCalledWith('C:\\Users\\AppData\\updates\\installer.exe');
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
  });

  it('opens GitHub release page when clicking GitHub Release button', async () => {
    vi.mocked(checkForUpdate).mockResolvedValue({
      currentVersion: '1.0.0',
      latestVersion: '1.0.0',
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
    });

    const onOpenChange = vi.fn();
    render(<UpdateDialog open={true} onOpenChange={onOpenChange} />);

    await waitFor(() => {
      expect(screen.getByText(/You are on the latest version/i)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /GitHub Release/i }));
    expect(openExternalURL).toHaveBeenCalledWith('https://github.com/janayuv/import-manager-next/releases');
  });
});
