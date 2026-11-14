import * as React from 'react';
import {
  AlertCircle,
  ArrowUpCircle,
  CheckCircle2,
  Download,
  ExternalLink,
  Loader2,
  RefreshCw,
  Sparkles,
} from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  applyUpdate,
  checkForUpdate,
  downloadUpdate,
  fetchBootstrapInfo,
  openExternalURL,
  type UpdateCheckResult,
} from '@/services/app-service';

interface UpdateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UpdateDialog({ open, onOpenChange }: UpdateDialogProps) {
  const [checking, setChecking] = React.useState(false);
  const [downloading, setDownloading] = React.useState(false);
  const [downloadedPath, setDownloadedPath] = React.useState<string | null>(null);
  const [result, setResult] = React.useState<UpdateCheckResult | null>(null);
  const [currentVer, setCurrentVer] = React.useState('1.0.0');

  React.useEffect(() => {
    if (open) {
      void fetchBootstrapInfo()
        .then((info) => {
          if (info.appVersion) setCurrentVer(info.appVersion);
        })
        .catch(() => {});
    }
  }, [open]);

  const handleCheck = React.useCallback(async () => {
    setChecking(true);
    setDownloadedPath(null);
    try {
      const res = await checkForUpdate();
      setResult(res);
      if (res.error) {
        toast.error(`Update check: ${res.error}`);
      } else if (res.updateAvailable) {
        toast.info(`New version v${res.latestVersion} is available!`);
      } else {
        toast.success(`You are already using the latest version (v${res.currentVersion}).`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setChecking(false);
    }
  }, []);

  // Automatically check once when dialog opens if not checked yet
  React.useEffect(() => {
    if (open && !result && !checking) {
      void handleCheck();
    }
  }, [open, result, checking, handleCheck]);

  const handleDownloadAndInstall = async () => {
    if (!result?.downloadURL) return;

    // If already downloaded, launch it directly
    if (downloadedPath) {
      try {
        await applyUpdate(downloadedPath);
        toast.success('Update installer launched. Follow the installer prompts to complete update.');
        onOpenChange(false);
      } catch (err) {
        toast.error(`Failed to launch installer: ${err instanceof Error ? err.message : String(err)}`);
      }
      return;
    }

    setDownloading(true);
    try {
      toast.info(`Downloading update from GitHub (${result.assetName})...`);
      const filePath = await downloadUpdate(result.downloadURL, result.assetName);
      setDownloadedPath(filePath);
      toast.success('Download completed successfully! Launching installer...');
      await applyUpdate(filePath);
      onOpenChange(false);
    } catch (err) {
      toast.error(`Download error: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setDownloading(false);
    }
  };

  const handleOpenReleasePage = async () => {
    const url = result?.releaseURL || 'https://github.com/janayuv/import-manager-next/releases';
    try {
      await openExternalURL(url);
    } catch {
      window.open(url, '_blank');
    }
  };

  const formatSize = (bytes: number): string => {
    if (!bytes || bytes <= 0) return '';
    const mb = bytes / (1024 * 1024);
    return `${mb.toFixed(1)} MB`;
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <ArrowUpCircle className="h-5 w-5" />
            </div>
            <div>
              <DialogTitle>Software Updates</DialogTitle>
              <DialogDescription>
                Check and install application updates directly from GitHub Releases.
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Version status header */}
          <div className="flex items-center justify-between rounded-lg border bg-muted/40 p-3">
            <div>
              <p className="text-xs text-muted-foreground">Installed Version</p>
              <p className="font-semibold text-foreground">v{currentVer}</p>
            </div>
            <div className="text-right">
              <p className="text-xs text-muted-foreground">Latest on GitHub</p>
              <p className="font-semibold text-foreground">
                {result?.latestVersion ? `v${result.latestVersion}` : 'Checking...'}
              </p>
            </div>
          </div>

          {/* Status Alert / Information */}
          {checking ? (
            <div className="flex flex-col items-center justify-center py-6 text-center text-sm text-muted-foreground">
              <Loader2 className="mb-2 h-7 w-7 animate-spin text-primary" />
              <span>Checking GitHub for updates...</span>
            </div>
          ) : result?.updateAvailable ? (
            <div className="space-y-3">
              <div className="flex items-start gap-2.5 rounded-lg border border-primary/30 bg-primary/5 p-3 text-sm text-primary">
                <Sparkles className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
                <div className="space-y-1">
                  <div className="font-semibold text-foreground">
                    New version v{result.latestVersion} is available!
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {result.releaseName || `Release v${result.latestVersion}`}
                    {result.assetSize > 0 && ` • ${formatSize(result.assetSize)}`}
                  </div>
                </div>
              </div>

              {result.releaseNotes && (
                <div className="space-y-1.5">
                  <span className="text-xs font-medium text-muted-foreground">What's New:</span>
                  <div className="max-h-48 overflow-y-auto rounded-md border bg-card p-3 text-xs leading-relaxed text-card-foreground">
                    <pre className="font-sans whitespace-pre-wrap">{result.releaseNotes}</pre>
                  </div>
                </div>
              )}

              {downloadedPath && (
                <div className="flex items-center gap-2 rounded border border-emerald-500/30 bg-emerald-500/10 p-2 text-xs text-emerald-800 dark:text-emerald-300">
                  <CheckCircle2 className="h-4 w-4 shrink-0" />
                  <span>Update downloaded and ready to install.</span>
                </div>
              )}
            </div>
          ) : result && !result.error ? (
            <div className="flex flex-col items-center justify-center py-5 text-center">
              <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 className="h-6 w-6" />
              </div>
              <p className="font-medium">You are on the latest version</p>
              <p className="text-xs text-muted-foreground">
                Import Manager v{currentVer} is up to date with GitHub Releases.
              </p>
            </div>
          ) : result?.error ? (
            <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-900 dark:text-amber-200">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <div>
                <p className="font-medium">Update Status</p>
                <p className="mt-0.5">{result.error}</p>
              </div>
            </div>
          ) : null}

          {result?.checkedAt && (
            <div className="text-[11px] text-muted-foreground text-center">
              Last checked: {new Date(result.checkedAt).toLocaleTimeString()} • Repository:{' '}
              {result.repository || 'janayuv/import-manager-next'}
            </div>
          )}
        </div>

        <DialogFooter className="gap-2 sm:justify-between">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void handleOpenReleasePage()}
            className="gap-1.5"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            GitHub Release
          </Button>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={checking || downloading}
              onClick={() => void handleCheck()}
              className="gap-1.5"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${checking ? 'animate-spin' : ''}`} />
              Check Again
            </Button>

            {result?.updateAvailable && (
              <Button
                type="button"
                size="sm"
                disabled={downloading}
                onClick={() => void handleDownloadAndInstall()}
                className="gap-1.5"
              >
                {downloading ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    Downloading...
                  </>
                ) : downloadedPath ? (
                  <>
                    <ArrowUpCircle className="h-3.5 w-3.5" />
                    Install Now
                  </>
                ) : (
                  <>
                    <Download className="h-3.5 w-3.5" />
                    Update from GitHub
                  </>
                )}
              </Button>
            )}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
