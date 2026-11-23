import * as React from 'react';
import { toast } from 'sonner';
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Clock,
  HardDrive,
  Lock,
  Play,
  RefreshCw,
  Server,
  Shield,
  Smartphone,
  Cloud,
  Zap,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  fetchSystemHealthSummary,
  runManualIntegrityCheck,
  runVerifiedBackup,
  runIdempotentJobRecovery,
  type SystemHealthSummary,
} from '@/services/health-service';
import { trimMemory } from '@/services/app-service';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
}

export function SystemHealthPage() {
  const [health, setHealth] = React.useState<SystemHealthSummary | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [refreshing, setRefreshing] = React.useState(false);
  const [busyAction, setBusyAction] = React.useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = React.useState(true);

  const loadHealth = React.useCallback(async (silent = false) => {
    if (!silent) setRefreshing(true);
    try {
      const data = await fetchSystemHealthSummary();
      setHealth(data);
    } catch (err) {
      if (!silent) {
        toast.error(err instanceof Error ? err.message : 'Failed to retrieve system health');
      }
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  React.useEffect(() => {
    void loadHealth();
    if (!autoRefresh) return;
    const interval = window.setInterval(() => {
      void loadHealth(true);
    }, 10000);
    return () => window.clearInterval(interval);
  }, [loadHealth, autoRefresh]);

  const handleRunIntegrityCheck = async () => {
    setBusyAction('integrity');
    try {
      const result = await runManualIntegrityCheck();
      toast.success(`Database integrity check: ${result}`);
      await loadHealth(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Integrity check failed');
    } finally {
      setBusyAction(null);
    }
  };

  const handleRunBackup = async () => {
    setBusyAction('backup');
    try {
      const bRec = await runVerifiedBackup();
      toast.success(`Verified backup completed: ${bRec.filename}`);
      await loadHealth(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Backup trigger failed');
    } finally {
      setBusyAction(null);
    }
  };

  const handleOptimizeMemory = async () => {
    setBusyAction('memory');
    try {
      const res = await trimMemory(true);
      const reclaimedMB = (res.reclaimedBytes / (1024 * 1024)).toFixed(1);
      toast.success(`Memory optimized: Reclaimed ${reclaimedMB} MB in ${res.durationMs}ms`);
      await loadHealth(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Memory trim failed');
    } finally {
      setBusyAction(null);
    }
  };

  const handleRecoverJob = async (jobId: string) => {
    setBusyAction(`job-${jobId}`);
    try {
      await runIdempotentJobRecovery(jobId);
      toast.success(`Initiated recovery for job: ${jobId}`);
      await loadHealth(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : `Failed to recover job ${jobId}`);
    } finally {
      setBusyAction(null);
    }
  };

  if (loading && !health) {
    return (
      <div className="flex min-h-[400px] w-full items-center justify-center p-8">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="h-8 w-8 animate-spin text-primary" />
          <p className="text-sm text-muted-foreground">Evaluating system health & diagnostics...</p>
        </div>
      </div>
    );
  }

  const overallStatus = health?.overallStatus ?? 'UNKNOWN';

  return (
    <div className="flex-1 space-y-6 p-6">
      {/* Header and Refresh Bar */}
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight">System Health & Diagnostics</h1>
            <Badge
              variant={
                overallStatus === 'HEALTHY'
                  ? 'default'
                  : overallStatus === 'WARNING'
                  ? 'secondary'
                  : 'destructive'
              }
              className="text-xs uppercase"
            >
              {overallStatus}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground">
            Centralized operational monitoring, SQLCipher security, storage capacity, and background job telemetry.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAutoRefresh(!autoRefresh)}
            className="text-xs"
          >
            {autoRefresh ? 'Auto-Refresh: ON (10s)' : 'Auto-Refresh: OFF'}
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={refreshing}
            onClick={() => void loadHealth()}
            className="gap-2 text-xs"
          >
            <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      {health?.lowDiskWarning && (
        <div className="flex items-center gap-3 rounded-lg border border-amber-500/30 bg-amber-50 p-4 text-amber-800 dark:bg-amber-950/20 dark:text-amber-300">
          <AlertTriangle className="h-5 w-5 flex-shrink-0 text-amber-600 dark:text-amber-400" />
          <div className="text-sm">
            <span className="font-semibold">Low Disk Space Warning: </span>
            One or more volumes has less than 2 GB of free storage. Database transactions or verified backups may be at risk.
          </div>
        </div>
      )}

      {/* Grid of 4 Key Overview Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {/* Card 1: Database & Encryption */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Database Security</CardTitle>
            <Lock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold flex items-center gap-2">
              {health?.sqlCipherConnected ? (
                <>
                  <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                  <span>SQLCipher</span>
                </>
              ) : (
                <>
                  <AlertTriangle className="h-5 w-5 text-amber-500" />
                  <span>Plain SQLite</span>
                </>
              )}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Header: {health?.diskHeaderEncrypted ? 'Encrypted (Verified)' : 'Warning: Plaintext'}
            </p>
            <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground border-t pt-2">
              <span>Schema v{health?.schemaVersion}</span>
              <span>Foreign Keys: {health?.foreignKeysEnforced ? 'ON' : 'OFF'}</span>
            </div>
          </CardContent>
        </Card>

        {/* Card 2: Storage Volume */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Database Storage</CardTitle>
            <HardDrive className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatBytes(health?.databaseDiskFreeBytes ?? 0)}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Free of {formatBytes(health?.databaseDiskTotalBytes ?? 0)}
            </p>
            <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground border-t pt-2">
              <span>DB Size: {formatBytes(health?.databaseSizeBytes ?? 0)}</span>
              <span>Pages: {health?.databasePageCount ?? 0}</span>
            </div>
          </CardContent>
        </Card>

        {/* Card 3: Memory Footprint */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Process Memory</CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatBytes(health?.memoryWorkingSetBytes ?? 0)}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Working Set (Private: {formatBytes(health?.memoryPrivateUsageBytes ?? 0)})
            </p>
            <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground border-t pt-2">
              <span>Go Heap: {formatBytes(health?.goHeapAllocBytes ?? 0)}</span>
              <span>Sys: {formatBytes(health?.goHeapSysBytes ?? 0)}</span>
            </div>
          </CardContent>
        </Card>

        {/* Card 4: Backup Readiness */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Backup Readiness</CardTitle>
            <Shield className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold flex items-center gap-2">
              <span
                className={
                  health?.backupReadinessStatus === 'HEALTHY'
                    ? 'text-emerald-600 dark:text-emerald-400'
                    : health?.backupReadinessStatus === 'WARNING'
                    ? 'text-amber-600 dark:text-amber-400'
                    : 'text-rose-600 dark:text-rose-400'
                }
              >
                {health?.backupReadinessStatus ?? 'UNKNOWN'}
              </span>
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {health?.verifiedBackupsCount ?? 0} of {health?.totalBackupsCount ?? 0} backups verified
            </p>
            <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground border-t pt-2">
              <span>
                {health?.hoursSinceLastBackup !== undefined
                  ? `${health.hoursSinceLastBackup.toFixed(1)}h ago`
                  : 'No backups'}
              </span>
              <span>Tested: {health?.lastTestedRestoreAt ? 'Yes' : 'None'}</span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Synchronizations & Maintenance Actions */}
      <div className="grid gap-4 md:grid-cols-2">
        {/* Sync Status Card */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <Activity className="h-4 w-4 text-primary" />
              Sync & Companion Services
            </CardTitle>
            <CardDescription>
              Local area network companion server and Google Drive cloud mirror
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between rounded-md border p-3">
              <div className="flex items-center gap-3">
                <Smartphone className="h-5 w-5 text-muted-foreground" />
                <div>
                  <div className="text-sm font-medium">Mobile Companion Sync</div>
                  <div className="text-xs text-muted-foreground">
                    Port: {health?.mobileSyncPort ?? 8085} • Connected Devices: {health?.mobileConnectedDevices ?? 0}
                  </div>
                </div>
              </div>
              <Badge variant={health?.mobileSyncRunning ? 'default' : 'secondary'}>
                {health?.mobileSyncRunning ? 'RUNNING' : 'STOPPED'}
              </Badge>
            </div>

            <div className="flex items-center justify-between rounded-md border p-3">
              <div className="flex items-center gap-3">
                <Cloud className="h-5 w-5 text-muted-foreground" />
                <div>
                  <div className="text-sm font-medium">Google Drive Backup Mirror</div>
                  <div className="text-xs text-muted-foreground">
                    {health?.googleDriveEmailMasked
                      ? `Account: ${health.googleDriveEmailMasked}`
                      : 'No cloud account connected'}
                  </div>
                </div>
              </div>
              <Badge variant={health?.googleDriveConnected ? 'default' : 'secondary'}>
                {health?.googleDriveConnected ? 'CONNECTED' : 'DISCONNECTED'}
              </Badge>
            </div>
          </CardContent>
        </Card>

        {/* Safe Operational Maintenance Actions */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <Zap className="h-4 w-4 text-primary" />
              Safe Maintenance Actions
            </CardTitle>
            <CardDescription>
              Non-destructive integrity, backup verification, and memory optimizations
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-sm font-medium">Database Integrity Check</div>
                <div className="text-xs text-muted-foreground">
                  Status: {health?.databaseIntegrity} • Checked: {health?.lastIntegrityCheckAt ? new Date(health.lastIntegrityCheckAt).toLocaleTimeString() : 'Startup'}
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                disabled={busyAction === 'integrity'}
                onClick={handleRunIntegrityCheck}
              >
                {busyAction === 'integrity' ? 'Checking...' : 'Run Integrity Check'}
              </Button>
            </div>

            <div className="flex items-center justify-between border-t pt-3">
              <div>
                <div className="text-sm font-medium">Trigger Verified Backup</div>
                <div className="text-xs text-muted-foreground">
                  Creates an on-demand encrypted backup and immediately verifies SHA-256
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                disabled={busyAction === 'backup'}
                onClick={handleRunBackup}
              >
                {busyAction === 'backup' ? 'Backing up...' : 'Create Backup'}
              </Button>
            </div>

            <div className="flex items-center justify-between border-t pt-3">
              <div>
                <div className="text-sm font-medium">Process Working Set Trim</div>
                <div className="text-xs text-muted-foreground">
                  Purges SQLite page cache and releases unreferenced memory to Windows OS
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                disabled={busyAction === 'memory'}
                onClick={handleOptimizeMemory}
              >
                {busyAction === 'memory' ? 'Optimizing...' : 'Trim Memory'}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Background Job Telemetry & Stuck Job Recovery Table */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base flex items-center gap-2">
                <Clock className="h-4 w-4 text-primary" />
                Background Job Engine Telemetry
              </CardTitle>
              <CardDescription>
                Live monitoring of automated background tasks, runtime durations, and stuck job recovery
              </CardDescription>
            </div>
            <div className="flex items-center gap-3 text-xs">
              <span className="text-muted-foreground">
                Active: <strong className="text-foreground">{health?.activeJobsCount ?? 0}</strong>
              </span>
              <span className="text-muted-foreground">
                Running: <strong className="text-foreground">{health?.runningJobsCount ?? 0}</strong>
              </span>
              <span className="text-muted-foreground">
                Stuck: <strong className={health?.stuckJobsCount ? 'text-rose-600' : 'text-foreground'}>{health?.stuckJobsCount ?? 0}</strong>
              </span>
              <span className="text-muted-foreground">
                24h Failures: <strong className="text-foreground">{health?.failedJobs24hCount ?? 0}</strong>
              </span>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {(!health?.jobSummaries || health.jobSummaries.length === 0) ? (
            <div className="py-6 text-center text-sm text-muted-foreground">
              No scheduled background jobs registered.
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Job ID</TableHead>
                  <TableHead>Job Name</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Last Started</TableHead>
                  <TableHead>Last Completed</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {health.jobSummaries.map((job) => (
                  <TableRow key={job.jobId}>
                    <TableCell className="font-mono text-xs">{job.jobId}</TableCell>
                    <TableCell className="text-sm font-medium">{job.jobName}</TableCell>
                    <TableCell>
                      {job.isStuck ? (
                        <Badge variant="destructive" className="gap-1 text-xs">
                          <AlertTriangle className="h-3 w-3" /> STUCK
                        </Badge>
                      ) : (
                        <Badge
                          variant={
                            job.lastStatus === 'COMPLETED'
                              ? 'default'
                              : job.lastStatus === 'RUNNING'
                              ? 'secondary'
                              : 'destructive'
                          }
                          className="text-xs"
                        >
                          {job.lastStatus || 'IDLE'}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {job.lastStartedAt || 'Never'}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {job.lastCompletedAt || 'Never'}
                    </TableCell>
                    <TableCell className="text-right">
                      {job.isStuck ? (
                        <Button
                          variant="destructive"
                          size="sm"
                          disabled={busyAction === `job-${job.jobId}`}
                          onClick={() => handleRecoverJob(job.jobId)}
                          className="h-7 text-xs"
                        >
                          Recover
                        </Button>
                      ) : (
                        <Button
                          variant="ghost"
                          size="sm"
                          disabled={busyAction === `job-${job.jobId}`}
                          onClick={() => handleRecoverJob(job.jobId)}
                          className="h-7 text-xs gap-1"
                        >
                          <Play className="h-3 w-3" /> Run
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
