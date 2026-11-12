import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Cloud,
  Database,
  FileKey2,
  HardDriveDownload,
  History,
  KeyRound,
  RefreshCw,
  RotateCcw,
  ShieldAlert,
  Upload,
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
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  backupService,
  databaseAdminService,
  googleDriveService,
  type BackupRecord,
  type BackupSchedule,
  type BrowseTableResponse,
  type BulkDeleteResult,
  type DatabaseOverviewStats,
  type GoogleDriveRemoteFile,
  type GoogleDriveStatus,
  type LegacyInspectResult,
  type LegacyMigrationReport,
  type RestoreVerificationSummary,
} from '@/services/data-ops-service';

export function DatabaseBackupsPage() {
  const [activeTab, setActiveTab] = useState<string>('overview');
  const [stats, setStats] = useState<DatabaseOverviewStats | null>(null);
  const [tables, setTables] = useState<string[]>([]);
  const [backups, setBackups] = useState<BackupRecord[]>([]);
  const [schedules, setSchedules] = useState<BackupSchedule[]>([]);
  const [driveStatus, setDriveStatus] = useState<GoogleDriveStatus | null>(null);
  const [driveFiles, setDriveFiles] = useState<GoogleDriveRemoteFile[]>([]);
  const [hasSavedPass, setHasSavedPass] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);

  // Browse / Inline Edit state
  const [browseTable, setBrowseTable] = useState<string>('suppliers');
  const [browseSearch, setBrowseSearch] = useState<string>('');
  const [includeDeleted, setIncludeDeleted] = useState<boolean>(false);
  const [browsePage, setBrowsePage] = useState<number>(1);
  const [browseData, setBrowseData] = useState<BrowseTableResponse | null>(null);
  const [editRowId, setEditRowId] = useState<string>('');
  const [editCol, setEditCol] = useState<string>('');
  const [editVal, setEditVal] = useState<string>('');

  // Bulk Delete + Undo state
  const [bulkTable, setBulkTable] = useState<string>('suppliers');
  const [bulkIdsRaw, setBulkIdsRaw] = useState<string>('');
  const [bulkHard, setBulkHard] = useState<boolean>(false);
  const [bulkPin, setBulkPin] = useState<string>('');
  const [bulkHighConfirm, setBulkHighConfirm] = useState<string>('');
  const [lastBulkResult, setLastBulkResult] = useState<BulkDeleteResult | null>(null);
  const [undoSecondsLeft, setUndoSecondsLeft] = useState<number>(0);

  // Backup / Restore / IMKey state
  const [backupPass, setBackupPass] = useState<string>('');
  const [backupNotes, setBackupNotes] = useState<string>('');
  const [savePassToKeyring, setSavePassToKeyring] = useState<boolean>(true);
  const [restorePath, setRestorePath] = useState<string>('');
  const [restorePass, setRestorePass] = useState<string>('');
  const [verifySidecar, setVerifySidecar] = useState<boolean>(true);
  const [restoreSummary, setRestoreSummary] = useState<RestoreVerificationSummary | null>(null);
  const [imkeyPath, setImkeyPath] = useState<string>('');
  const [imkeyTransportPass, setImkeyTransportPass] = useState<string>('');

  // Google Drive state
  const [gdriveClientId, setGdriveClientId] = useState<string>('');
  const [gdriveClientSecret, setGdriveClientSecret] = useState<string>('');
  const [gdriveAuthUrl, setGdriveAuthUrl] = useState<string>('');
  const [gdriveAuthCode, setGdriveAuthCode] = useState<string>('');

  // Schedule state
  const [schedName, setSchedName] = useState<string>('Nightly Vault Backup');
  const [schedCron, setSchedCron] = useState<string>('0 2 * * *');
  const [schedDest, setSchedDest] = useState<string>('local');
  const [schedRetCount, setSchedRetCount] = useState<string>('7');

  // Legacy Migration Wizard state
  const [legacyPath, setLegacyPath] = useState<string>('');
  const [legacyKey, setLegacyKey] = useState<string>('');
  const [legacyInspect, setLegacyInspect] = useState<LegacyInspectResult | null>(null);
  const [legacyReport, setLegacyReport] = useState<LegacyMigrationReport | null>(null);

  const loadAll = async () => {
    setLoading(true);
    try {
      const [ov, tbls, bkList, schList, drvStat, savedPass] = await Promise.all([
        databaseAdminService.getOverviewStats(),
        databaseAdminService.listBrowseableTables(),
        backupService.listBackups(),
        backupService.listSchedules().catch(() => [] as BackupSchedule[]),
        googleDriveService.getStatus().catch(() => null),
        backupService.hasSavedBackupPassphrase().catch(() => false),
      ]);
      setStats(ov);
      setTables(tbls ?? []);
      setBackups(bkList ?? []);
      setSchedules(schList ?? []);
      setDriveStatus(drvStat);
      setHasSavedPass(savedPass);
      if (drvStat?.connected) {
        googleDriveService.listBackups().then(setDriveFiles).catch(() => setDriveFiles([]));
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load database console');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadAll();
  }, []);

  useEffect(() => {
    if (!lastBulkResult?.undoExpiresAt || !lastBulkResult.undoToken) {
      setUndoSecondsLeft(0);
      return;
    }
    const tick = () => {
      const ms = new Date(lastBulkResult.undoExpiresAt!).getTime() - Date.now();
      setUndoSecondsLeft(Math.max(0, Math.floor(ms / 1000)));
    };
    tick();
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [lastBulkResult]);

  const loadBrowse = async (tbl = browseTable, q = browseSearch, del = includeDeleted, pg = browsePage) => {
    try {
      const resp = await databaseAdminService.browseTable({
        tableName: tbl,
        search: q,
        includeDeleted: del,
        page: pg,
        pageSize: 25,
      });
      setBrowseData(resp);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Browse failed');
    }
  };

  const handleSaveCell = async () => {
    try {
      await databaseAdminService.updateTableCell({
        tableName: browseTable,
        rowId: editRowId,
        column: editCol,
        value: editVal,
        isNull: false,
      });
      toast.success(`Updated ${browseTable}.${editCol} for ${editRowId}`);
      setEditRowId('');
      await loadBrowse();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Cell update failed');
    }
  };

  const handleBulkDelete = async () => {
    const ids = bulkIdsRaw
      .split(/[\s,;]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    if (ids.length === 0) {
      toast.error('Enter at least one record ID');
      return;
    }
    try {
      const res = await databaseAdminService.bulkDelete({
        tableName: bulkTable,
        recordIds: ids,
        hardDelete: bulkHard,
        pin: bulkHard ? bulkPin : undefined,
        highImpactConfirm: bulkHard ? bulkHighConfirm : undefined,
      });
      setLastBulkResult(res);
      toast.success(`Processed ${res.processedCount} row(s) across ${res.chunkCount} batch(es).`);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Bulk operation failed');
    }
  };

  const handleUndoBulk = async () => {
    if (!lastBulkResult?.undoToken) return;
    try {
      const count = await databaseAdminService.undoBulkSoftDelete(lastBulkResult.undoToken);
      toast.success(`Undid bulk delete: restored ${count} record(s).`);
      setLastBulkResult(null);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Undo failed');
    }
  };

  const handleCreateBackup = async () => {
    try {
      const rec = await backupService.createBackup({
        passphrase: backupPass || undefined,
        savePassToKey: savePassToKeyring,
        destination: 'local',
        notes: backupNotes,
      });
      toast.success(`Encrypted IMBK2 backup created: ${rec.filename}`);
      setRestorePath(rec.path);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Backup creation failed');
    }
  };

  const handleRestore = async (dryRun: boolean) => {
    try {
      const summary = await backupService.restoreBackup({
        backupPath: restorePath,
        passphrase: restorePass || undefined,
        dryRun,
        verifySidecar,
      });
      setRestoreSummary(summary);
      toast.success(dryRun ? 'Dry-run verification passed!' : 'Backup restored safely!');
      if (!dryRun) {
        await loadAll();
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Restore failed');
    }
  };

  const handleInspectLegacy = async () => {
    try {
      const res = await databaseAdminService.inspectLegacyDatabase({
        sourcePath: legacyPath,
        sourceKey: legacyKey || undefined,
      });
      setLegacyInspect(res);
      setLegacyReport(null);
      toast.success(res.keyValid ? 'Legacy database inspected cleanly' : 'Legacy inspection completed with warnings');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Legacy inspect failed');
    }
  };

  const handleMigrateLegacy = async (dryRun: boolean) => {
    try {
      const rep = await databaseAdminService.runLegacyMigration({
        sourcePath: legacyPath,
        sourceKey: legacyKey || undefined,
        dryRun,
      });
      setLegacyReport(rep);
      toast.success(dryRun ? 'Legacy dry-run parity verified' : 'Legacy migration committed cleanly');
      if (!dryRun) await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Legacy migration failed');
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Database, Encrypted Backups &amp; Migration Console</h1>
          <p className="text-sm text-muted-foreground">
            SQLCipher administration, IMBK2 backups, Google Drive sync, chunked bulk ops, and legacy database migration.
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            onClick={() =>
              databaseAdminService
                .runIntegrityAndVacuum()
                .then((msg) => toast.success(msg))
                .catch((e) => toast.error(e.message))
            }
          >
            Run Integrity &amp; VACUUM
          </Button>
          <Button variant="outline" disabled={loading} onClick={() => void loadAll()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Refresh
          </Button>
        </div>
      </div>

      {stats?.restoreLocked && (
        <div className="flex items-center gap-2 rounded-md border border-destructive bg-destructive/10 p-3 text-sm text-destructive">
          <ShieldAlert className="h-4 w-4" />
          <span>Global restore lock active: background jobs and mutating writes are paused.</span>
        </div>
      )}

      <Tabs
        value={activeTab}
        onValueChange={(v) => {
          setActiveTab(v);
          if (v === 'browse' && !browseData) {
            void loadBrowse();
          }
        }}
      >
        <TabsList className="flex flex-wrap">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="browse">Browse / Edit</TabsTrigger>
          <TabsTrigger value="bulk">Bulk Operations</TabsTrigger>
          <TabsTrigger value="backup">Backups &amp; Drive</TabsTrigger>
          <TabsTrigger value="schedules">Schedules</TabsTrigger>
          <TabsTrigger value="legacy">Legacy Migration</TabsTrigger>
          <TabsTrigger value="roles">Roles</TabsTrigger>
          <TabsTrigger value="settings">Audit &amp; Settings</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-4">
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>SQLCipher Engine</CardDescription>
                <CardTitle className="text-lg">{stats?.cipherVersion || 'Checking...'}</CardTitle>
              </CardHeader>
              <CardContent className="text-xs text-muted-foreground">
                Journal: {stats?.journalMode?.toUpperCase() || 'WAL'} · FK: {stats?.foreignKeysOn ? 'ON' : 'OFF'}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>Integrity Check</CardDescription>
                <CardTitle className="text-lg uppercase">{stats?.integrityCheck || '—'}</CardTitle>
              </CardHeader>
              <CardContent className="text-xs text-muted-foreground">
                Schema v{stats?.schemaVersion ?? 0}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>Database Footprint</CardDescription>
                <CardTitle className="text-lg">
                  {stats ? `${(stats.fileSizeBytes / 1024).toFixed(1)} KB` : '—'}
                </CardTitle>
              </CardHeader>
              <CardContent className="text-xs text-muted-foreground">
                {stats?.pageCount ?? 0} pages ({stats?.pageSize ?? 4096} B/page)
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>Credential Vault</CardDescription>
                <CardTitle className="text-lg">{hasSavedPass ? 'Passphrase Saved' : 'Manual Key'}</CardTitle>
              </CardHeader>
              <CardContent className="text-xs text-muted-foreground">
                Drive: {driveStatus?.connected ? driveStatus.accountEmail || 'Connected' : 'Disconnected'}
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Live Table Inventory</CardTitle>
              <CardDescription>{stats?.databasePath}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-2 sm:grid-cols-3 lg:grid-cols-4">
                {stats &&
                  Object.entries(stats.tableCounts).map(([tbl, count]) => (
                    <div key={tbl} className="flex items-center justify-between rounded-md border p-2.5 text-sm">
                      <span className="font-medium">{tbl}</span>
                      <div className="flex items-center gap-1.5">
                        <Badge variant="secondary">{count}</Badge>
                        {(stats.deletedCounts?.[tbl] ?? 0) > 0 && (
                          <Badge variant="outline">-{stats.deletedCounts[tbl]}</Badge>
                        )}
                      </div>
                    </div>
                  ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="browse" className="space-y-4">
          <Card>
            <CardHeader className="pb-3">
              <div className="flex flex-wrap items-center gap-3">
                <Select
                  value={browseTable}
                  onValueChange={(v) => {
                    setBrowseTable(v);
                    setBrowsePage(1);
                    void loadBrowse(v, browseSearch, includeDeleted, 1);
                  }}
                >
                  <SelectTrigger className="w-56" aria-label="Select Table">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {tables.map((t) => (
                      <SelectItem key={t} value={t}>
                        {t}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Input
                  className="w-64"
                  placeholder="Filter rows..."
                  value={browseSearch}
                  onChange={(e) => setBrowseSearch(e.target.value)}
                />
                <div className="flex items-center gap-2 text-sm">
                  <Switch
                    checked={includeDeleted}
                    onCheckedChange={(v) => {
                      setIncludeDeleted(v);
                      void loadBrowse(browseTable, browseSearch, v, 1);
                    }}
                  />
                  <span>Include Deleted</span>
                </div>
                <Button onClick={() => void loadBrowse(browseTable, browseSearch, includeDeleted, 1)}>
                  Query
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {browseData?.editable && (
                <div className="grid gap-2 rounded-md border bg-muted/30 p-3 md:grid-cols-4">
                  <Input
                    placeholder={`Row ${browseData.primaryKey}`}
                    value={editRowId}
                    onChange={(e) => setEditRowId(e.target.value)}
                  />
                  <Input
                    placeholder="Column name"
                    value={editCol}
                    onChange={(e) => setEditCol(e.target.value)}
                  />
                  <Input
                    placeholder="New value"
                    value={editVal}
                    onChange={(e) => setEditVal(e.target.value)}
                  />
                  <Button onClick={handleSaveCell}>Save Cell Edit</Button>
                </div>
              )}

              <div className="overflow-x-auto rounded-md border">
                <table className="w-full text-xs">
                  <thead className="bg-muted/50 text-left">
                    <tr>
                      {(browseData?.columns ?? []).slice(0, 8).map((c) => (
                        <th key={c} className="p-2 font-medium">
                          {c}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {(browseData?.rows ?? []).length === 0 ? (
                      <tr>
                        <td colSpan={8} className="p-4 text-center text-muted-foreground">
                          No rows found.
                        </td>
                      </tr>
                    ) : (
                      (browseData?.rows ?? []).map((row, idx) => (
                        <tr key={idx} className="border-t">
                          {(browseData?.columns ?? []).slice(0, 8).map((c) => (
                            <td key={c} className="max-w-[200px] truncate p-2">
                              {String(row[c] ?? '')}
                            </td>
                          ))}
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="bulk" className="space-y-4">
          {lastBulkResult?.undoToken && undoSecondsLeft > 0 && (
            <div className="flex items-center justify-between rounded-md border border-blue-500/40 bg-blue-50 p-3 text-sm dark:bg-blue-950/30">
              <span>
                Bulk soft-deleted {lastBulkResult.processedCount} row(s). Undo window expires in{' '}
                <strong>{undoSecondsLeft}s</strong>.
              </span>
              <Button size="sm" onClick={handleUndoBulk}>
                <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                Undo Bulk Delete
              </Button>
            </div>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Chunked Bulk Delete (Batch Size 500)</CardTitle>
              <CardDescription>
                Soft delete issues a 5-minute Undo Token; hard delete requires Security PIN &amp; DELETE confirmation if dependencies &gt; 5000.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid gap-3 md:grid-cols-2">
                <div className="space-y-1.5">
                  <Label>Target Table</Label>
                  <Select value={bulkTable} onValueChange={setBulkTable}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {['suppliers', 'items', 'shipments', 'invoices', 'boe_details', 'expenses'].map((t) => (
                        <SelectItem key={t} value={t}>
                          {t}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex items-end gap-2 pb-1">
                  <Switch checked={bulkHard} onCheckedChange={setBulkHard} />
                  <Label>Permanent Hard Delete (PIN required)</Label>
                </div>
              </div>

              <div className="space-y-1.5">
                <Label>Record IDs (comma or newline separated)</Label>
                <Input
                  placeholder="id-1, id-2, id-3"
                  value={bulkIdsRaw}
                  onChange={(e) => setBulkIdsRaw(e.target.value)}
                />
              </div>

              {bulkHard && (
                <div className="grid gap-3 md:grid-cols-2">
                  <Input
                    type="password"
                    placeholder="Security PIN / Admin Password"
                    value={bulkPin}
                    onChange={(e) => setBulkPin(e.target.value)}
                  />
                  <Input
                    placeholder="Type DELETE if dependencies > 5000"
                    value={bulkHighConfirm}
                    onChange={(e) => setBulkHighConfirm(e.target.value)}
                  />
                </div>
              )}

              <Button variant={bulkHard ? 'destructive' : 'default'} onClick={handleBulkDelete}>
                Execute Bulk {bulkHard ? 'Hard' : 'Soft'} Delete
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="backup" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Database className="h-4 w-4" /> Create Encrypted IMBK2 Backup
                </CardTitle>
                <CardDescription>Hot VACUUM INTO snapshot encrypted with AES-256-GCM + .sha256 sidecar.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <Input
                  type="password"
                  placeholder={hasSavedPass ? 'Leave blank to use Credential Manager passphrase' : 'Encryption Passphrase'}
                  value={backupPass}
                  onChange={(e) => setBackupPass(e.target.value)}
                />
                <Input
                  placeholder="Backup notes (optional)"
                  value={backupNotes}
                  onChange={(e) => setBackupNotes(e.target.value)}
                />
                <div className="flex items-center gap-2 text-sm">
                  <Switch checked={savePassToKeyring} onCheckedChange={setSavePassToKeyring} />
                  <span>Remember passphrase in Windows Credential Manager</span>
                </div>
                <Button onClick={handleCreateBackup}>Create IMBK2 Backup</Button>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <FileKey2 className="h-4 w-4" /> Restore Backup &amp; .imkey Vault
                </CardTitle>
                <CardDescription>Verify integrity &amp; foreign keys in dry-run before atomic live restore.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <Input
                  placeholder="Path to .imbk2 / .imbk1 archive"
                  value={restorePath}
                  onChange={(e) => setRestorePath(e.target.value)}
                />
                <Input
                  type="password"
                  placeholder="Archive passphrase (optional if saved)"
                  value={restorePass}
                  onChange={(e) => setRestorePass(e.target.value)}
                />
                <div className="flex items-center gap-2 text-sm">
                  <Switch checked={verifySidecar} onCheckedChange={setVerifySidecar} />
                  <span>Enforce .sha256 sidecar checksum</span>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button variant="outline" onClick={() => void handleRestore(true)}>
                    Dry-Run Verify
                  </Button>
                  <Button variant="destructive" onClick={() => void handleRestore(false)}>
                    Execute Live Restore
                  </Button>
                </div>

                {restoreSummary && (
                  <div className="rounded border bg-muted/40 p-2.5 text-xs">
                    <div>
                      Format: <strong>{restoreSummary.backupFormat}</strong> · Integrity:{' '}
                      <strong>{restoreSummary.integrityCheck}</strong> · FK:{' '}
                      <strong>{restoreSummary.foreignKeysOk ? 'OK' : 'FAIL'}</strong>
                    </div>
                    <div>Total Rows Verified: {restoreSummary.totalRows}</div>
                    {restoreSummary.safetyBackupPath && (
                      <div className="truncate">Safety Backup: {restoreSummary.safetyBackupPath}</div>
                    )}
                  </div>
                )}

                <div className="border-t pt-3 space-y-2">
                  <div className="flex items-center gap-1.5 text-xs font-medium">
                    <KeyRound className="h-3.5 w-3.5" /> Key File (.imkey) Export / Import
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <Input
                      placeholder="Path for .imkey bundle"
                      value={imkeyPath}
                      onChange={(e) => setImkeyPath(e.target.value)}
                    />
                    <Input
                      type="password"
                      placeholder="Transport passphrase"
                      value={imkeyTransportPass}
                      onChange={(e) => setImkeyTransportPass(e.target.value)}
                    />
                  </div>
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        backupService
                          .exportIMKey(imkeyPath, backupPass, imkeyTransportPass)
                          .then((r) => {
                            setImkeyPath(r.filePath);
                            toast.success(`Exported ${r.filePath}`);
                          })
                          .catch((e) => toast.error(e.message))
                      }
                    >
                      Export .imkey
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        backupService
                          .importIMKey(imkeyPath, imkeyTransportPass)
                          .then(() => {
                            toast.success('Imported backup passphrase into Credential Manager');
                            void loadAll();
                          })
                          .catch((e) => toast.error(e.message))
                      }
                    >
                      Import .imkey
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Cloud className="h-4 w-4" /> Google Drive Cloud Sync ({driveStatus?.folderName || 'ImportManagerBackups'})
              </CardTitle>
              <CardDescription>
                Loopback OAuth2 (`127.0.0.1:8765`) · Retains up to {driveStatus?.retentionLimit ?? 30} encrypted cloud archives.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {!driveStatus?.connected ? (
                <div className="grid gap-2 md:grid-cols-3">
                  <Input
                    placeholder="Google OAuth Client ID"
                    value={gdriveClientId}
                    onChange={(e) => setGdriveClientId(e.target.value)}
                  />
                  <Input
                    type="password"
                    placeholder="Google OAuth Client Secret"
                    value={gdriveClientSecret}
                    onChange={(e) => setGdriveClientSecret(e.target.value)}
                  />
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      onClick={() =>
                        googleDriveService
                          .saveClientConfig({ clientId: gdriveClientId, clientSecret: gdriveClientSecret })
                          .then(() => toast.success('OAuth client saved'))
                          .catch((e) => toast.error(e.message))
                      }
                    >
                      Save Client
                    </Button>
                    <Button
                      onClick={() =>
                        googleDriveService
                          .startOAuthFlow()
                          .then((url) => {
                            setGdriveAuthUrl(url);
                            toast.info('OAuth listener ready on 127.0.0.1:8765');
                          })
                          .catch((e) => toast.error(e.message))
                      }
                    >
                      Connect Drive
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="flex flex-wrap items-center justify-between gap-2 rounded border p-2.5 text-sm">
                  <span>
                    Connected as <strong>{driveStatus.accountEmail || 'Authorized Account'}</strong>
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      googleDriveService
                        .disconnect()
                        .then(() => void loadAll())
                        .catch((e) => toast.error(e.message))
                    }
                  >
                    Disconnect
                  </Button>
                </div>
              )}

              {gdriveAuthUrl && (
                <div className="space-y-2 rounded border p-2.5 text-xs">
                  <div className="truncate">Auth URL: {gdriveAuthUrl}</div>
                  <div className="flex gap-2">
                    <Input
                      placeholder="Paste auth code if manual fallback needed"
                      value={gdriveAuthCode}
                      onChange={(e) => setGdriveAuthCode(e.target.value)}
                    />
                    <Button
                      size="sm"
                      onClick={() =>
                        googleDriveService
                          .completeOAuthCode(gdriveAuthCode, '')
                          .then(() => {
                            setGdriveAuthUrl('');
                            void loadAll();
                          })
                          .catch((e) => toast.error(e.message))
                      }
                    >
                      Exchange Code
                    </Button>
                  </div>
                </div>
              )}

              {driveFiles.length > 0 && (
                <div className="space-y-1">
                  {driveFiles.map((f) => (
                    <div key={f.id} className="flex items-center justify-between rounded border px-2.5 py-1.5 text-xs">
                      <span>
                        {f.name} ({f.locator})
                      </span>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() =>
                          googleDriveService
                            .downloadBackup(f.locator, f.name)
                            .then((p) => {
                              setRestorePath(p);
                              toast.success(`Downloaded to ${p}`);
                            })
                            .catch((e) => toast.error(e.message))
                        }
                      >
                        <HardDriveDownload className="mr-1 h-3.5 w-3.5" />
                        Download
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Local Backup History</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {backups.length === 0 ? (
                <p className="text-sm text-muted-foreground">No backups recorded yet.</p>
              ) : (
                backups.map((b) => (
                  <div key={b.id} className="flex flex-wrap items-center justify-between gap-2 rounded border p-2.5 text-xs">
                    <div>
                      <div className="font-medium">{b.filename}</div>
                      <div className="text-muted-foreground">
                        {b.path} · {(b.sizeBytes / 1024).toFixed(1)} KB · SHA256 {b.sha256.slice(0, 12)}...
                      </div>
                    </div>
                    <div className="flex gap-1.5">
                      <Button size="sm" variant="outline" onClick={() => setRestorePath(b.path)}>
                        Use for Restore
                      </Button>
                      {driveStatus?.connected && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() =>
                            googleDriveService
                              .uploadBackup(b.path)
                              .then(() => {
                                toast.success('Uploaded to Google Drive');
                                void loadAll();
                              })
                              .catch((e) => toast.error(e.message))
                          }
                        >
                          <Upload className="mr-1 h-3.5 w-3.5" />
                          Sync Drive
                        </Button>
                      )}
                    </div>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="schedules" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Automated Backup Schedules</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-2 md:grid-cols-5">
                <Input placeholder="Schedule Name" value={schedName} onChange={(e) => setSchedName(e.target.value)} />
                <Input placeholder="Cron (e.g. 0 2 * * *)" value={schedCron} onChange={(e) => setSchedCron(e.target.value)} />
                <Input placeholder="Destination (local|gdrive)" value={schedDest} onChange={(e) => setSchedDest(e.target.value)} />
                <Input placeholder="Retention Count" value={schedRetCount} onChange={(e) => setSchedRetCount(e.target.value)} />
                <Button
                  onClick={() =>
                    backupService
                      .saveSchedule({
                        name: schedName,
                        cronExpr: schedCron,
                        timeZone: 'UTC',
                        destination: schedDest,
                        retentionCount: Number(schedRetCount) || 7,
                        retentionDays: 30,
                        enabled: true,
                        notes: '',
                      })
                      .then(() => {
                        toast.success('Backup schedule saved');
                        void loadAll();
                      })
                      .catch((e) => toast.error(e.message))
                  }
                >
                  Add Schedule
                </Button>
              </div>
              <div className="space-y-2">
                {schedules.map((sc) => (
                  <div key={sc.id} className="flex items-center justify-between rounded border p-2.5 text-sm">
                    <div>
                      <span className="font-medium">{sc.name}</span> · <code>{sc.cronExpr}</code> · {sc.destination}
                    </div>
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() =>
                        backupService
                          .deleteSchedule(sc.id)
                          .then(() => void loadAll())
                          .catch((e) => toast.error(e.message))
                      }
                    >
                      Remove
                    </Button>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="legacy" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <History className="h-4 w-4" /> Non-Destructive Legacy Database Migration Utility
              </CardTitle>
              <CardDescription>
                Inspect legacy Tauri/SQLite databases read-only, validate schema &amp; financial parity in dry-run, and import safely without modifying the original .db file.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid gap-2 md:grid-cols-3">
                <Input
                  placeholder="Path to legacy import_manager.db"
                  value={legacyPath}
                  onChange={(e) => setLegacyPath(e.target.value)}
                />
                <Input
                  type="password"
                  placeholder="Legacy SQLCipher key (leave blank if plaintext)"
                  value={legacyKey}
                  onChange={(e) => setLegacyKey(e.target.value)}
                />
                <div className="flex gap-2">
                  <Button variant="outline" onClick={handleInspectLegacy}>
                    Inspect DB
                  </Button>
                  <Button variant="outline" onClick={() => void handleMigrateLegacy(true)}>
                    Dry-Run Parity
                  </Button>
                  <Button onClick={() => void handleMigrateLegacy(false)}>
                    Commit Import
                  </Button>
                </div>
              </div>

              {legacyInspect && (
                <div className="space-y-2 rounded-md border p-3 text-xs">
                  <div className="flex flex-wrap gap-2">
                    <Badge variant="outline">
                      Format: {legacyInspect.isPlaintextSqlite ? 'Plaintext SQLite' : 'SQLCipher'}
                    </Badge>
                    <Badge variant={legacyInspect.keyValid ? 'secondary' : 'destructive'}>
                      Key: {legacyInspect.keyValid ? 'Valid' : 'Invalid'}
                    </Badge>
                    <Badge variant="outline">Legacy Schema v{legacyInspect.migrationVersion || '?'}</Badge>
                  </div>
                  <div>
                    Source Financials — Invoice Sum: {legacyInspect.sourceFinancials.invoiceValueSum.toFixed(2)} ·
                    BOE Duty Paise: {legacyInspect.sourceFinancials.boeTotalDutyPaise} ·
                    Expenses Paise: {legacyInspect.sourceFinancials.expenseTotalPaise}
                  </div>
                  <div className="grid gap-1 sm:grid-cols-3">
                    {legacyInspect.tables?.filter((t) => t.sourceExists).map((t) => (
                      <div key={t.tableName} className="rounded border px-2 py-1">
                        {t.tableName}: {t.sourceRows} src → {t.targetRows} dst ({t.matchedCols.length} cols)
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {legacyReport && (
                <div className="space-y-1 rounded-md border bg-muted/40 p-3 text-xs">
                  <div className="font-medium">
                    {legacyReport.dryRun ? 'Dry-Run Simulation Report' : 'Committed Migration Report'}
                  </div>
                  <div>
                    Integrity: <strong>{legacyReport.integrityCheck}</strong> · FK OK:{' '}
                    <strong>{String(legacyReport.foreignKeysOk)}</strong> · Row Parity:{' '}
                    <strong>{String(legacyReport.rowParityOk)}</strong> · Financial Parity:{' '}
                    <strong>{String(legacyReport.financialParityOk)}</strong>
                  </div>
                  {legacyReport.safetyBackupPath && (
                    <div>Pre-Migration Safety Backup: {legacyReport.safetyBackupPath}</div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="roles" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Role Matrix &amp; Access Control</CardTitle>
              <CardDescription>Manage the 4x25 RBAC matrix from the dedicated Roles &amp; Permissions console.</CardDescription>
            </CardHeader>
            <CardContent>
              <Button asChild>
                <Link to="/admin/roles-permissions">Open Roles &amp; Permissions Matrix</Link>
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="settings" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Security, Audit Log &amp; Workspace Settings</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              <Button asChild variant="outline">
                <Link to="/admin/activity-log">Open Audit Activity Log</Link>
              </Button>
              <Button asChild variant="outline">
                <Link to="/admin/security-center">Open Security Center</Link>
              </Button>
              <Button asChild variant="outline">
                <Link to="/settings">Open Application Settings</Link>
              </Button>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
