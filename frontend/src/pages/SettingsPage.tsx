import * as React from 'react';
import {
  AlertCircle,
  ArrowUpCircle,
  CheckCircle2,
  Download,
  ExternalLink,
  Loader2,
  Palette,
  RefreshCw,
  RotateCcw,
  Save,
  Settings2,
  Sparkles,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useAuth } from '@/contexts/AuthContext';
import { useOptionalTheme } from '@/contexts/ThemeContext';
import {
  applyThemeProfile,
  settingsService,
  type AppSettingsProfile,
} from '@/services/dashboard-settings-service';
import {
  applyUpdate,
  checkForUpdate,
  downloadUpdate,
  fetchBootstrapInfo,
  openExternalURL,
  type UpdateCheckResult,
} from '@/services/app-service';

const MODULE_LABELS: Record<string, string> = {
  shipment: 'Shipments',
  invoice: 'Invoices',
  boe: 'BOE Registry',
  boeSummary: 'BOE Summary',
  supplier: 'Suppliers',
  itemMaster: 'Item Master',
  expenses: 'Expenses',
};

export function SettingsPage() {
  const { hasPermission } = useAuth();
  const themeCtx = useOptionalTheme();
  const syncTheme = themeCtx?.syncTheme;
  const canWrite = hasPermission('data.write');

  const [profile, setProfile] = React.useState<AppSettingsProfile | null>(null);
  const [selectedModule, setSelectedModule] = React.useState('shipment');
  const [saving, setSaving] = React.useState(false);

  const [appVersion, setAppVersion] = React.useState('1.0.0');
  const [checkingUpdate, setCheckingUpdate] = React.useState(false);
  const [downloadingUpdate, setDownloadingUpdate] = React.useState(false);
  const [downloadedUpdatePath, setDownloadedUpdatePath] = React.useState<string | null>(null);
  const [updateResult, setUpdateResult] = React.useState<UpdateCheckResult | null>(null);

  React.useEffect(() => {
    void fetchBootstrapInfo()
      .then((info) => {
        if (info.appVersion) setAppVersion(info.appVersion);
      })
      .catch(() => {});
  }, []);

  const handleCheckUpdate = async () => {
    setCheckingUpdate(true);
    setDownloadedUpdatePath(null);
    try {
      const res = await checkForUpdate();
      setUpdateResult(res);
      if (res.error) {
        toast.error(`Update check: ${res.error}`);
      } else if (res.updateAvailable) {
        toast.info(`New version v${res.latestVersion} is available!`);
      } else {
        toast.success(`You are on the latest version (v${res.currentVersion}).`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setCheckingUpdate(false);
    }
  };

  const handleApplyUpdate = async () => {
    if (!updateResult?.downloadURL) return;
    if (downloadedUpdatePath) {
      try {
        await applyUpdate(downloadedUpdatePath);
        toast.success('Update installer launched. Follow the installer prompts to complete update.');
      } catch (err) {
        toast.error(`Failed to launch installer: ${err instanceof Error ? err.message : String(err)}`);
      }
      return;
    }
    setDownloadingUpdate(true);
    try {
      toast.info(`Downloading update from GitHub (${updateResult.assetName})...`);
      const filePath = await downloadUpdate(updateResult.downloadURL, updateResult.assetName);
      setDownloadedUpdatePath(filePath);
      toast.success('Download complete! Launching installer...');
      await applyUpdate(filePath);
    } catch (err) {
      toast.error(`Download error: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setDownloadingUpdate(false);
    }
  };

  const handleOpenRelease = async () => {
    const url = updateResult?.releaseURL || 'https://github.com/janayuv/import-manager-next/releases';
    try {
      await openExternalURL(url);
    } catch {
      window.open(url, '_blank');
    }
  };

  React.useEffect(() => {
    void settingsService
      .getProfile()
      .then((p) => {
        setProfile(p);
        applyThemeProfile(p);
        syncTheme?.(p);
      })
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)));
  }, [syncTheme]);

  const handleSave = async () => {
    if (!profile) return;
    setSaving(true);
    try {
      const saved = await settingsService.saveProfile(profile);
      setProfile(saved);
      applyThemeProfile(saved);
      themeCtx?.syncTheme(saved);
      toast.success('Application settings saved');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleResetModule = async () => {
    try {
      const next = await settingsService.resetModuleDefaults(selectedModule);
      setProfile(next);
      toast.success(`Reset ${MODULE_LABELS[selectedModule] ?? selectedModule} defaults`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  if (!profile) {
    return (
      <div className="text-sm text-muted-foreground">Loading application settings...</div>
    );
  }

  const modCfg = profile.modules[selectedModule] ?? {
    visibleFields: [],
    itemsPerPage: 10,
    decimalPrecision: 2,
    dateFormat: profile.dateFormat,
    fieldCases: {},
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <Settings2 className="h-6 w-6 text-primary" />
            Application Settings &amp; Formatting
          </h1>
          <p className="text-sm text-muted-foreground">
            Theme preset, dark/light mode, invoice precision, regional formatting, and module table preferences.
          </p>
        </div>
        {canWrite && (
          <Button disabled={saving} onClick={() => void handleSave()}>
            <Save className="h-4 w-4" />
            {saving ? 'Saving...' : 'Save Settings'}
          </Button>
        )}
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Palette className="h-4 w-4 text-primary" />
              Appearance &amp; Theme
            </CardTitle>
            <CardDescription>Schema v{profile.schemaVersion}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1">
              <Label>Theme Preset</Label>
              <Select
                value={profile.theme}
                onValueChange={(v) => {
                  const next = { ...profile, theme: v };
                  setProfile(next);
                  applyThemeProfile(next);
                  themeCtx?.syncTheme(next);
                }}
              >
                <SelectTrigger aria-label="Theme Preset">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">Default ERP Slate</SelectItem>
                  <SelectItem value="steel">Industrial Steel</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>Color Mode</Label>
              <Select
                value={profile.mode}
                onValueChange={(v) => {
                  const next = { ...profile, mode: v };
                  setProfile(next);
                  applyThemeProfile(next);
                  themeCtx?.syncTheme(next);
                }}
              >
                <SelectTrigger aria-label="Color Mode">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="system">System</SelectItem>
                  <SelectItem value="light">Light</SelectItem>
                  <SelectItem value="dark">Dark</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Invoice Precision &amp; Formatting</CardTitle>
            <CardDescription>Decimal precision restricted to 0 or 2 places per business rules</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1">
              <Label>Line Total Decimals</Label>
              <Select
                value={String(profile.lineTotalDecimals)}
                onValueChange={(v) =>
                  setProfile({ ...profile, lineTotalDecimals: Number.parseInt(v, 10) })
                }
              >
                <SelectTrigger aria-label="Line Total Decimals">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="2">2 Decimals (Standard)</SelectItem>
                  <SelectItem value="0">0 Decimals (Rounded)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Invoice Total Decimals</Label>
              <Select
                value={String(profile.invoiceTotalDecimals)}
                onValueChange={(v) =>
                  setProfile({ ...profile, invoiceTotalDecimals: Number.parseInt(v, 10) })
                }
              >
                <SelectTrigger aria-label="Invoice Total Decimals">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="2">2 Decimals (Standard)</SelectItem>
                  <SelectItem value="0">0 Decimals (Rounded)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Date Display Format</Label>
              <Select
                value={profile.dateFormat}
                onValueChange={(v) => setProfile({ ...profile, dateFormat: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="DD/MM/YYYY">DD/MM/YYYY (Default)</SelectItem>
                  <SelectItem value="YYYY-MM-DD">YYYY-MM-DD (ISO)</SelectItem>
                  <SelectItem value="MM/DD/YYYY">MM/DD/YYYY</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Number Grouping</Label>
              <Select
                value={profile.numberFormat}
                onValueChange={(v) => setProfile({ ...profile, numberFormat: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="en-IN">Indian (1,00,000.00)</SelectItem>
                  <SelectItem value="en-US">International (100,000.00)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base">Per-Module Table Preferences (7 Modules)</CardTitle>
            <CardDescription>
              Pagination size and formatting defaults per ERP view.
            </CardDescription>
          </div>
          {canWrite && (
            <Button variant="outline" size="sm" onClick={() => void handleResetModule()}>
              <RotateCcw className="h-3.5 w-3.5" />
              Reset Module Defaults
            </Button>
          )}
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="space-y-1">
              <Label>Module</Label>
              <Select value={selectedModule} onValueChange={setSelectedModule}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(MODULE_LABELS).map(([k, lbl]) => (
                    <SelectItem key={k} value={k}>
                      {lbl}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Rows Per Page (5–200)</Label>
              <Input
                type="number"
                min={5}
                max={200}
                value={modCfg.itemsPerPage}
                onChange={(e) => {
                  const nextVal = Number.parseInt(e.target.value, 10) || 10;
                  setProfile({
                    ...profile,
                    modules: {
                      ...profile.modules,
                      [selectedModule]: { ...modCfg, itemsPerPage: nextVal },
                    },
                  });
                }}
              />
            </div>

            <div className="flex items-end pb-2">
              {selectedModule === 'shipment' && (
                <Badge variant="outline">
                  shipmentType case locked: {modCfg.fieldCases?.shipmentType ?? 'uppercase'}
                </Badge>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="md:col-span-2">
        <CardHeader>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <CardTitle className="flex items-center gap-2 text-base">
                <ArrowUpCircle className="h-5 w-5 text-primary" />
                In-App Updates (GitHub Releases)
              </CardTitle>
              <CardDescription>
                Check for new releases, view release notes, and install desktop updates directly from GitHub.
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Badge variant="outline">Current: v{appVersion}</Badge>
              <Button
                variant="outline"
                size="sm"
                disabled={checkingUpdate || downloadingUpdate}
                onClick={() => void handleCheckUpdate()}
                className="gap-1.5"
              >
                <RefreshCw className={`h-3.5 w-3.5 ${checkingUpdate ? 'animate-spin' : ''}`} />
                {checkingUpdate ? 'Checking...' : 'Check for Updates'}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {checkingUpdate ? (
            <div className="flex items-center justify-center gap-2 py-6 text-sm text-muted-foreground">
              <Loader2 className="h-5 w-5 animate-spin text-primary" />
              <span>Querying GitHub Releases API...</span>
            </div>
          ) : updateResult?.updateAvailable ? (
            <div className="space-y-4 rounded-lg border border-primary/30 bg-primary/5 p-4">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-2">
                  <Sparkles className="h-5 w-5 text-primary" />
                  <div>
                    <h3 className="font-semibold text-foreground">
                      New Release Available: v{updateResult.latestVersion}
                    </h3>
                    <p className="text-xs text-muted-foreground">
                      {updateResult.releaseName || `Release v${updateResult.latestVersion}`}
                      {updateResult.assetSize > 0 &&
                        ` • ${(updateResult.assetSize / (1024 * 1024)).toFixed(1)} MB`}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void handleOpenRelease()}
                    className="gap-1"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                    GitHub Release
                  </Button>
                  <Button
                    size="sm"
                    disabled={downloadingUpdate}
                    onClick={() => void handleApplyUpdate()}
                    className="gap-1.5"
                  >
                    {downloadingUpdate ? (
                      <>
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        Downloading...
                      </>
                    ) : downloadedUpdatePath ? (
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
                </div>
              </div>

              {updateResult.releaseNotes && (
                <div className="space-y-1">
                  <span className="text-xs font-medium text-muted-foreground">Release Notes:</span>
                  <div className="max-h-48 overflow-y-auto rounded border bg-card p-3 text-xs leading-relaxed">
                    <pre className="font-sans whitespace-pre-wrap">{updateResult.releaseNotes}</pre>
                  </div>
                </div>
              )}

              {downloadedUpdatePath && (
                <div className="flex items-center gap-2 text-xs text-emerald-600 dark:text-emerald-400">
                  <CheckCircle2 className="h-4 w-4" />
                  <span>Update downloaded to {downloadedUpdatePath}. Ready to launch installer.</span>
                </div>
              )}
            </div>
          ) : updateResult && !updateResult.error ? (
            <div className="flex items-center justify-between rounded-lg border bg-muted/30 p-3">
              <div className="flex items-center gap-2.5">
                <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                <div>
                  <p className="text-sm font-medium">Application is up to date</p>
                  <p className="text-xs text-muted-foreground">
                    You are running the latest version of Import Manager (v{appVersion}).
                  </p>
                </div>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void handleOpenRelease()}
                className="gap-1 text-xs text-muted-foreground hover:text-foreground"
              >
                <ExternalLink className="h-3 w-3" />
                Releases
              </Button>
            </div>
          ) : updateResult?.error ? (
            <div className="flex items-start gap-2.5 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-900 dark:text-amber-200">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <div>
                <p className="font-medium">GitHub Update Notice</p>
                <p className="mt-0.5">{updateResult.error}</p>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-between rounded-lg border bg-muted/20 p-3 text-xs text-muted-foreground">
              <span>Repository target: janayuv/import-manager-next</span>
              <span>Click "Check for Updates" to fetch the latest GitHub releases.</span>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
