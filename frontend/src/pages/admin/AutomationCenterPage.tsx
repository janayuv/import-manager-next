import * as React from 'react';
import { toast } from 'sonner';
import { Play, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { workflowService, type BackgroundJob, type JobExecutionRecord } from '@/services/workflow-service';

export function AutomationCenterPage() {
  const [jobs, setJobs] = React.useState<BackgroundJob[]>([]);
  const [executions, setExecutions] = React.useState<JobExecutionRecord[]>([]);
  const [runningJobId, setRunningJobId] = React.useState<string | null>(null);

  const loadData = React.useCallback(async () => {
    try {
      const [j, ex] = await Promise.all([
        workflowService.listBackgroundJobs(),
        workflowService.listJobExecutions('', 50),
      ]);
      setJobs(j);
      setExecutions(ex);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load automation jobs');
    }
  }, []);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  const handleToggle = async (job: BackgroundJob) => {
    try {
      await workflowService.toggleBackgroundJob(job.jobId, !job.isEnabled);
      toast.success(`${job.jobName} ${!job.isEnabled ? 'enabled' : 'paused'}`);
      await loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to toggle job');
    }
  };

  const handleRunNow = async (jobId: string) => {
    setRunningJobId(jobId);
    try {
      const rec = await workflowService.runBackgroundJobNow(jobId);
      toast.success(`Job ${jobId} ${rec.status} (${rec.recordsProcessed} records, ${rec.executionTimeMs}ms)`);
      await loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Job execution failed');
    } finally {
      setRunningJobId(null);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Automation Center</h1>
          <p className="text-sm text-muted-foreground">
            Manage the 6 canonical background jobs, schedules, circuit-breaker retries, and execution history.
          </p>
        </div>
        <Button variant="outline" onClick={() => void loadData()}>
          <RefreshCw className="mr-2 h-4 w-4" />
          Refresh
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {jobs.map((job) => (
          <Card key={job.jobId}>
            <CardHeader className="pb-2">
              <div className="flex items-start justify-between gap-2">
                <CardTitle className="text-base">{job.jobName}</CardTitle>
                <Badge variant={job.isEnabled ? 'default' : 'secondary'}>
                  {job.isEnabled ? 'Enabled' : 'Paused'}
                </Badge>
              </div>
              <CardDescription className="font-mono text-xs">{job.jobId}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-xs">
              <div className="text-muted-foreground">
                Type: {job.jobType} • Max Retries: {job.maxRetries} • Last Status: <strong>{job.lastStatus || 'IDLE'}</strong>
              </div>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  onClick={() => void handleRunNow(job.jobId)}
                  disabled={runningJobId === job.jobId}
                >
                  <Play className="mr-1 h-3 w-3" />
                  Run Now
                </Button>
                <Button size="sm" variant="outline" onClick={() => void handleToggle(job)}>
                  {job.isEnabled ? 'Disable' : 'Enable'}
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Recent Execution Log</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="py-2 pr-3">Job ID</th>
                <th className="py-2 pr-3">Status</th>
                <th className="py-2 pr-3">Records</th>
                <th className="py-2 pr-3">Duration</th>
                <th className="py-2">Started At</th>
              </tr>
            </thead>
            <tbody>
              {executions.length === 0 ? (
                <tr>
                  <td colSpan={5} className="py-6 text-center text-muted-foreground">
                    No job runs recorded yet.
                  </td>
                </tr>
              ) : (
                executions.map((ex) => (
                  <tr key={ex.executionId} className="border-b">
                    <td className="py-2 pr-3 font-mono text-xs">{ex.jobId}</td>
                    <td className="py-2 pr-3">
                      <Badge variant={ex.status === 'COMPLETED' ? 'outline' : 'destructive'}>{ex.status}</Badge>
                    </td>
                    <td className="py-2 pr-3">{ex.recordsProcessed}</td>
                    <td className="py-2 pr-3">{ex.executionTimeMs} ms</td>
                    <td className="py-2 text-xs text-muted-foreground">{ex.startedAt}</td>
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
