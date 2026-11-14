import * as React from 'react';
import { toast } from 'sonner';
import { Archive, ArrowUpCircle, Play, RefreshCw, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { UpdateDialog } from '@/components/system/UpdateDialog';
import { workflowService, type DiagnosticsExportResult } from '@/services/workflow-service';

export function SystemToolsPage() {
  const [exportPath, setExportPath] = React.useState('');
  const [retentionDays, setRetentionDays] = React.useState('30');
  const [lastExport, setLastExport] = React.useState<DiagnosticsExportResult | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [updateOpen, setUpdateOpen] = React.useState(false);

  const handleExportDiagnostics = async () => {
    setBusy(true);
    try {
      const res = await workflowService.exportDiagnosticsBundle(exportPath);
      setLastExport(res);
      toast.success(`Diagnostics bundle saved (${res.sizeBytes} bytes)`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Diagnostics export failed');
    } finally {
      setBusy(false);
    }
  };

  const handleRunOperationsTick = async () => {
    setBusy(true);
    try {
      await workflowService.runDailyOperationsTick('admin');
      toast.success('Daily operations, burst & forecast tick completed');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Operations tick failed');
    } finally {
      setBusy(false);
    }
  };

  const handlePruneErrors = async () => {
    setBusy(true);
    try {
      const count = await workflowService.pruneResolvedErrors(Number(retentionDays) || 30);
      toast.success(`Pruned ${count} resolved error group(s)`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Prune failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">System Tools</h1>
        <p className="text-sm text-muted-foreground">
          Generate redacted diagnostics ZIP bundles, run workflow operations cycles, and prune resolved telemetry.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Diagnostics ZIP Exporter</CardTitle>
            <CardDescription>
              Exports redacted runtime logs, jobs, incidents, error memory, and bug notes into a ZIP archive.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="diag-path">Optional Custom Output Path</Label>
              <Input
                id="diag-path"
                value={exportPath}
                onChange={(e) => setExportPath(e.target.value)}
                placeholder="Leave blank to save under Exports directory"
              />
            </div>
            <Button onClick={() => void handleExportDiagnostics()} disabled={busy}>
              <Archive className="mr-2 h-4 w-4" />
              Export Diagnostics Bundle (.zip)
            </Button>
            {lastExport && (
              <div className="rounded border bg-muted p-3 text-xs space-y-1">
                <div className="font-mono break-all">{lastExport.filePath}</div>
                <div>
                  Size: {lastExport.sizeBytes} bytes • Sections: {lastExport.includedSections.join(', ')}
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Maintenance & Scheduler Operations</CardTitle>
            <CardDescription>Trigger operational sweeps and telemetry cleanup manually.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" onClick={() => void handleRunOperationsTick()} disabled={busy}>
                <Play className="mr-2 h-4 w-4" />
                Run Operations Tick
              </Button>
              <Button
                variant="outline"
                onClick={() => void workflowService.runBackgroundJobNow('maintenance_cleanup').then(() => toast.success('Maintenance cleanup executed'))}
                disabled={busy}
              >
                <RefreshCw className="mr-2 h-4 w-4" />
                Run Maintenance Job
              </Button>
            </div>
            <div className="flex items-end gap-2 pt-2">
              <div className="flex-1 space-y-1">
                <Label htmlFor="prune-days">Error Memory Prune Retention (days)</Label>
                <Input
                  id="prune-days"
                  type="number"
                  min={0}
                  value={retentionDays}
                  onChange={(e) => setRetentionDays(e.target.value)}
                />
              </div>
              <Button variant="secondary" onClick={() => void handlePruneErrors()} disabled={busy}>
                <Sparkles className="mr-2 h-4 w-4" />
                Prune Resolved
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="md:col-span-2">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <ArrowUpCircle className="h-5 w-5 text-primary" />
              Software Updates (GitHub Releases)
            </CardTitle>
            <CardDescription>
              Check for new client releases, view release notes, and install in-app updates directly from GitHub.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <p className="text-sm text-muted-foreground">
              Sync with official GitHub releases from repository <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">janayuv/import-manager-next</code>.
            </p>
            <Button onClick={() => setUpdateOpen(true)}>
              <ArrowUpCircle className="mr-2 h-4 w-4" />
              Check &amp; Install Updates
            </Button>
          </CardContent>
        </Card>
      </div>

      <UpdateDialog open={updateOpen} onOpenChange={setUpdateOpen} />
    </div>
  );
}
