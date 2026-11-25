import * as React from 'react';
import { toast } from 'sonner';
import { AlertTriangle, Clock, Loader2, Play, RefreshCw, ShieldOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { useOptionalAuth } from '@/contexts/AuthContext';
import { workflowService, type BackgroundJob, type JobExecutionRecord } from '@/services/workflow-service';

function statusBadgeVariant(status: string): 'success' | 'default' | 'destructive' | 'secondary' | 'outline' {
  switch (status) {
    case 'COMPLETED':
      return 'success';
    case 'RUNNING':
      return 'default';
    case 'FAILED':
    case 'TIMEOUT':
      return 'destructive';
    default:
      return 'secondary';
  }
}

export function AutomationCenterPage() {
  const auth = useOptionalAuth();
  const canMutate = auth ? (auth.hasPermission('automation.mutate') || auth.hasPermission('automation.ops_center')) : true;
  const canOverrideGuard = auth ? auth.hasPermission('automation.ops_center') : true;

  const [jobs, setJobs] = React.useState<BackgroundJob[]>([]);
  const [executions, setExecutions] = React.useState<JobExecutionRecord[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [refreshing, setRefreshing] = React.useState(false);
  const [autoRefresh, setAutoRefresh] = React.useState(false);
  const [runningJobId, setRunningJobId] = React.useState<string | null>(null);
  const [togglingJobId, setTogglingJobId] = React.useState<string | null>(null);
  const [overridingJobId, setOverridingJobId] = React.useState<string | null>(null);
  const [overrideReasons, setOverrideReasons] = React.useState<Record<string, string>>({});
  const [selectedJobFilter, setSelectedJobFilter] = React.useState<string>('all');
  const [statusFilter, setStatusFilter] = React.useState<string>('all');

  const loadData = React.useCallback(async (silent = false) => {
    if (!silent) setRefreshing(true);
    try {
      const [j, ex] = await Promise.all([
        workflowService.listBackgroundJobs(),
        workflowService.listJobExecutions('', 100),
      ]);
      setJobs(j);
      setExecutions(ex);
    } catch (err) {
      if (!silent) {
        toast.error(err instanceof Error ? err.message : 'Failed to load automation jobs');
      }
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  React.useEffect(() => {
    if (!autoRefresh) return;
    const timer = window.setInterval(() => {
      void loadData(true);
    }, 15000);
    return () => window.clearInterval(timer);
  }, [autoRefresh, loadData]);

  const handleToggle = async (job: BackgroundJob) => {
    setTogglingJobId(job.jobId);
    try {
      await workflowService.toggleBackgroundJob(job.jobId, !job.isEnabled);
      toast.success(`${job.jobName} ${!job.isEnabled ? 'enabled' : 'paused'}`);
      await loadData(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to toggle job');
    } finally {
      setTogglingJobId(null);
    }
  };

  const handleRunNow = async (jobId: string) => {
    setRunningJobId(jobId);
    try {
      const rec = await workflowService.runBackgroundJobNow(jobId);
      toast.success(`Job ${jobId} ${rec.status} (${rec.recordsProcessed} records, ${rec.executionTimeMs}ms)`);
      await loadData(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Job execution failed');
    } finally {
      setRunningJobId(null);
    }
  };

  const handleOverrideGuard = async (jobId: string) => {
    const reason = (overrideReasons[jobId] ?? '').trim();
    if (reason.length < 5) {
      toast.error('Enter an override reason (at least 5 characters)');
      return;
    }
    setOverridingJobId(jobId);
    try {
      await workflowService.overrideRecoveryGuard(jobId, reason);
      toast.success(`Recovery guard cleared and ${jobId} re-enabled`);
      setOverrideReasons((prev) => ({ ...prev, [jobId]: '' }));
      await loadData(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to override recovery guard');
    } finally {
      setOverridingJobId(null);
    }
  };

  const enabledCount = jobs.filter((j) => j.isEnabled).length;
  const guardStoppedCount = jobs.filter((j) => j.guardStopped).length;
  const failedCount = executions.filter((e) => e.status === 'FAILED' || e.status === 'TIMEOUT').length;

  const filteredExecutions = React.useMemo(() => {
    return executions.filter((ex) => {
      if (selectedJobFilter !== 'all' && ex.jobId !== selectedJobFilter) return false;
      if (statusFilter !== 'all' && ex.status !== statusFilter) return false;
      return true;
    });
  }, [executions, selectedJobFilter, statusFilter]);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Automation Center</h1>
          <p className="text-sm text-muted-foreground">
            Manage the 6 canonical background jobs, schedules, circuit-breaker retries, and execution history.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAutoRefresh((prev) => !prev)}
          >
            {autoRefresh ? 'Auto-Refresh: ON (15s)' : 'Auto-Refresh: OFF'}
          </Button>
          <Button variant="outline" disabled={refreshing} onClick={() => void loadData()}>
            <RefreshCw className={`mr-2 h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Scheduled Jobs</CardDescription>
            <CardTitle className="text-2xl">
              {enabledCount} / {jobs.length}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            {jobs.length - enabledCount} paused
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Circuit Breaker Halted</CardDescription>
            <CardTitle className={`text-2xl ${guardStoppedCount > 0 ? 'text-destructive' : ''}`}>
              {guardStoppedCount}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            {guardStoppedCount > 0 ? 'Admin override required' : 'All guards clear'}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Recent Failures / Timeouts</CardDescription>
            <CardTitle className={`text-2xl ${failedCount > 0 ? 'text-amber-600' : ''}`}>
              {failedCount}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            Across last {executions.length} recorded runs
          </CardContent>
        </Card>
      </div>

      {loading && jobs.length === 0 ? (
        <div className="flex min-h-[160px] items-center justify-center rounded-md border text-sm text-muted-foreground">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          Loading background jobs...
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {jobs.map((job) => {
            const isMaintenance = job.jobId === 'maintenance_cleanup';
            const isRunning = runningJobId === job.jobId;
            return (
              <Card key={job.jobId} className={job.guardStopped ? 'border-destructive/60' : undefined}>
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between gap-2">
                    <CardTitle className="text-base">{job.jobName}</CardTitle>
                    <div className="flex shrink-0 items-center gap-1">
                      {job.guardStopped && (
                        <Badge variant="destructive">Guard Stopped</Badge>
                      )}
                      <Badge variant={job.isEnabled ? 'default' : 'secondary'}>
                        {job.isEnabled ? 'Enabled' : 'Paused'}
                      </Badge>
                    </div>
                  </div>
                  <CardDescription className="flex items-center gap-2 font-mono text-xs">
                    <span>{job.jobId}</span>
                    <Badge variant="outline" className="text-[10px]">{job.scheduleType}</Badge>
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-3 text-xs">
                  <div className="space-y-1 text-muted-foreground">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span>Type: {job.jobType}</span>
                      <span>• Max Retries: {job.maxRetries}</span>
                      <span>• SLA: {Math.round(job.expectedDurationMs / 1000)}s</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span>Last Status:</span>
                      <Badge variant={statusBadgeVariant(job.lastStatus || 'IDLE')} className="text-[10px]">
                        {job.lastStatus || 'IDLE'}
                      </Badge>
                    </div>
                    <div className="flex items-center gap-1">
                      <Clock className="h-3 w-3 shrink-0" />
                      <span>Last Run: {job.lastCompletedAt || job.lastStartedAt || 'Never'}</span>
                    </div>
                    {job.nextExpectedAt && (
                      <div>Next Due: {job.nextExpectedAt}</div>
                    )}
                    {job.dependsOn && job.dependsOn.length > 0 && (
                      <div>Depends On: <span className="font-mono">{job.dependsOn.join(', ')}</span></div>
                    )}
                  </div>

                  {job.lastErrorMessage && (
                    <div className="rounded border border-destructive/30 bg-destructive/10 p-2 text-[11px] text-destructive">
                      <AlertTriangle className="mr-1 inline h-3 w-3" />
                      {job.lastErrorMessage}
                    </div>
                  )}

                  {job.guardStopped && (
                    <div className="space-y-2 rounded border border-destructive/40 bg-destructive/5 p-2">
                      <div className="font-medium text-destructive">
                        Circuit breaker engaged ({job.recoveryAttempts24h ?? 0} recovery attempts in 24h)
                      </div>
                      {canOverrideGuard && (
                        <div className="flex gap-1.5">
                          <Input
                            className="h-7 text-xs"
                            placeholder="Override reason (>=5 chars)..."
                            value={overrideReasons[job.jobId] ?? ''}
                            onChange={(e) =>
                              setOverrideReasons((prev) => ({ ...prev, [job.jobId]: e.target.value }))
                            }
                          />
                          <Button
                            size="sm"
                            variant="destructive"
                            disabled={overridingJobId === job.jobId}
                            onClick={() => void handleOverrideGuard(job.jobId)}
                          >
                            <ShieldOff className="mr-1 h-3 w-3" />
                            Override
                          </Button>
                        </div>
                      )}
                    </div>
                  )}

                  <div className="flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      onClick={() => void handleRunNow(job.jobId)}
                      disabled={!canMutate || isRunning || Boolean(job.guardStopped)}
                    >
                      {isRunning ? (
                        <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                      ) : (
                        <Play className="mr-1 h-3 w-3" />
                      )}
                      {isRunning ? 'Running...' : 'Run Now'}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={
                        !canMutate ||
                        togglingJobId === job.jobId ||
                        Boolean(job.guardStopped) ||
                        (job.isEnabled && isMaintenance)
                      }
                      title={
                        job.isEnabled && isMaintenance
                          ? 'Core retention maintenance job cannot be disabled'
                          : undefined
                      }
                      onClick={() => void handleToggle(job)}
                    >
                      {job.isEnabled ? 'Disable' : 'Enable'}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setSelectedJobFilter(job.jobId)}
                    >
                      Filter Log
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <CardTitle>Recent Execution Log ({filteredExecutions.length})</CardTitle>
              <CardDescription>Showing latest executions across background jobs.</CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <select
                aria-label="Filter by job"
                className="h-9 rounded-md border border-input bg-background px-2.5 text-xs"
                value={selectedJobFilter}
                onChange={(e) => setSelectedJobFilter(e.target.value)}
              >
                <option value="all">All Jobs</option>
                {jobs.map((j) => (
                  <option key={j.jobId} value={j.jobId}>
                    {j.jobId}
                  </option>
                ))}
              </select>
              <select
                aria-label="Filter by status"
                className="h-9 rounded-md border border-input bg-background px-2.5 text-xs"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
              >
                <option value="all">All Statuses</option>
                <option value="COMPLETED">COMPLETED</option>
                <option value="RUNNING">RUNNING</option>
                <option value="FAILED">FAILED</option>
                <option value="TIMEOUT">TIMEOUT</option>
              </select>
              {(selectedJobFilter !== 'all' || statusFilter !== 'all') && (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setSelectedJobFilter('all');
                    setStatusFilter('all');
                  }}
                >
                  Clear Filters
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="py-2 pr-3">Job ID</th>
                <th className="py-2 pr-3">Status</th>
                <th className="py-2 pr-3">Records</th>
                <th className="py-2 pr-3">Retries</th>
                <th className="py-2 pr-3">Duration</th>
                <th className="py-2 pr-3">Started At</th>
                <th className="py-2 pr-3">Completed At</th>
                <th className="py-2">Error</th>
              </tr>
            </thead>
            <tbody>
              {filteredExecutions.length === 0 ? (
                <tr>
                  <td colSpan={8} className="py-6 text-center text-muted-foreground">
                    No job runs match the current filters.
                  </td>
                </tr>
              ) : (
                filteredExecutions.map((ex) => (
                  <tr key={ex.executionId} className="border-b">
                    <td className="py-2 pr-3 font-mono text-xs">{ex.jobId}</td>
                    <td className="py-2 pr-3">
                      <Badge variant={statusBadgeVariant(ex.status)}>{ex.status}</Badge>
                    </td>
                    <td className="py-2 pr-3">{ex.recordsProcessed}</td>
                    <td className="py-2 pr-3">{ex.retryCount}</td>
                    <td className="py-2 pr-3">{ex.executionTimeMs} ms</td>
                    <td className="py-2 pr-3 text-xs text-muted-foreground">{ex.startedAt}</td>
                    <td className="py-2 pr-3 text-xs text-muted-foreground">{ex.completedAt || '—'}</td>
                    <td className="max-w-[260px] truncate py-2 text-xs text-destructive" title={ex.errorMessage || ''}>
                      {ex.errorMessage || '—'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </CardContent>
      </Card>
    </div>
  );
}
