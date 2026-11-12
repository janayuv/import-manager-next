import * as React from 'react';
import { toast } from 'sonner';
import { Activity, AlertTriangle, CheckCircle2, RefreshCw, ShieldAlert } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import {
  workflowService,
  type ExceptionCase,
  type OperationsOverview,
  type WorkflowIncident,
} from '@/services/workflow-service';

export function OperationsCenterPage() {
  const [overview, setOverview] = React.useState<OperationsOverview | null>(null);
  const [exceptions, setExceptions] = React.useState<ExceptionCase[]>([]);
  const [loading, setLoading] = React.useState(false);

  const [selectedIncident, setSelectedIncident] = React.useState<WorkflowIncident | null>(null);
  const [assignee, setAssignee] = React.useState('ops-lead');
  const [noteText, setNoteText] = React.useState('');
  const [resolutionSummary, setResolutionSummary] = React.useState('');
  const [skipQuietCheck, setSkipQuietCheck] = React.useState(false);

  const refresh = React.useCallback(async () => {
    setLoading(true);
    try {
      const [ov, ex] = await Promise.all([
        workflowService.getOperationsOverview(),
        workflowService.listExceptions('all', 'all'),
      ]);
      setOverview(ov);
      setExceptions(ex);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load operations overview');
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleDetectExceptions = async () => {
    try {
      const res = await workflowService.detectExceptions();
      toast.success(
        `Scan complete: ${res.createdCount} created, ${res.escalatedCount} escalated, ${res.selfHealedCount} self-healed`,
      );
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Exception detection failed');
    }
  };

  const handleTransitionException = async (caseId: string, status: string) => {
    try {
      await workflowService.transitionException(caseId, status, `Transitioned to ${status} via Operations Center`);
      toast.success(`Exception marked ${status}`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Transition failed');
    }
  };

  const handleAssignIncident = async (incId: string) => {
    try {
      await workflowService.assignIncident(incId, assignee);
      toast.success(`Incident assigned to ${assignee}`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Assignment failed');
    }
  };

  const handleAddNote = async () => {
    if (!selectedIncident) return;
    try {
      await workflowService.addIncidentNote(selectedIncident.incidentId, noteText);
      setNoteText('');
      toast.success('Investigation note attached');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to attach note');
    }
  };

  const handleResolveIncident = async () => {
    if (!selectedIncident) return;
    try {
      await workflowService.resolveIncident({
        incidentId: selectedIncident.incidentId,
        resolutionNotes: resolutionSummary,
        skipQuietWindowCheck: skipQuietCheck,
      });
      toast.success('Incident resolved cleanly');
      setSelectedIncident(null);
      setResolutionSummary('');
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Resolve gate blocked resolution');
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Operations Center</h1>
          <p className="text-sm text-muted-foreground">
            Shipment exception SLAs, incident correlation & resolve gates, bursts, and 30-minute forecasts.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void refresh()} disabled={loading}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Refresh
          </Button>
          <Button onClick={() => void handleDetectExceptions()}>
            <Activity className="mr-2 h-4 w-4" />
            Run Exception Scan
          </Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Workflow Health</CardDescription>
            <CardTitle className="text-2xl">
              {overview ? `${overview.workflowHealthScore.toFixed(0)}%` : '—'}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            Stability score: {overview ? `${overview.systemStability.toFixed(0)}%` : '—'}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Open Exceptions</CardDescription>
            <CardTitle className="text-2xl">{overview?.openExceptions ?? 0}</CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-destructive">
            {overview?.breachedSlaCount ?? 0} SLA breached
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Open Incidents</CardDescription>
            <CardTitle className="text-2xl">{overview?.openIncidents ?? 0}</CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            {overview?.activeSuppressions ?? 0} active suppressions
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Bursts (24h)</CardDescription>
            <CardTitle className="text-2xl">{overview?.recentBursts ?? 0}</CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            {overview?.restoreLocked ? 'Restore Lock Active' : 'Scheduler Active'}
          </CardContent>
        </Card>
      </div>

      <Tabs defaultValue="exceptions">
        <TabsList>
          <TabsTrigger value="exceptions">Exceptions ({exceptions.length})</TabsTrigger>
          <TabsTrigger value="incidents">Incidents ({overview?.incidents.length ?? 0})</TabsTrigger>
          <TabsTrigger value="forecasts">30m Forecasts ({overview?.forecasts.length ?? 0})</TabsTrigger>
        </TabsList>

        <TabsContent value="exceptions" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Shipment & Customs Exception Cases</CardTitle>
              <CardDescription>
                OVERDUE_ETA, MISSING_BOE (&gt;7d), and MISSING_EXPENSE cases with adaptive SLAs (4h/12h/24h/72h).
              </CardDescription>
            </CardHeader>
            <CardContent className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-muted-foreground">
                    <th className="py-2 pr-3">Type</th>
                    <th className="py-2 pr-3">Entity</th>
                    <th className="py-2 pr-3">Priority</th>
                    <th className="py-2 pr-3">SLA Status</th>
                    <th className="py-2 pr-3">Status</th>
                    <th className="py-2">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {exceptions.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="py-6 text-center text-muted-foreground">
                        No exception cases detected.
                      </td>
                    </tr>
                  ) : (
                    exceptions.map((ex) => (
                      <tr key={ex.id} className="border-b">
                        <td className="py-2 pr-3 font-medium">{ex.exceptionType}</td>
                        <td className="py-2 pr-3 font-mono text-xs">{ex.entityId}</td>
                        <td className="py-2 pr-3">
                          <Badge variant={ex.priority === 'CRITICAL' ? 'destructive' : 'secondary'}>
                            {ex.priority}
                          </Badge>
                        </td>
                        <td className="py-2 pr-3">
                          <Badge variant={ex.slaStatus === 'BREACHED' ? 'destructive' : 'outline'}>
                            {ex.slaStatus}
                          </Badge>
                        </td>
                        <td className="py-2 pr-3">{ex.status}</td>
                        <td className="flex gap-2 py-2">
                          {ex.status !== 'RESOLVED' && ex.status !== 'IGNORED' && (
                            <>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => void handleTransitionException(ex.id, 'RESOLVED')}
                              >
                                <CheckCircle2 className="mr-1 h-3 w-3" />
                                Resolve
                              </Button>
                              <Button
                                size="sm"
                                variant="ghost"
                                onClick={() => void handleTransitionException(ex.id, 'IGNORED')}
                              >
                                Ignore
                              </Button>
                            </>
                          )}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="incidents" className="space-y-4">
          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Correlated Incidents</CardTitle>
                <CardDescription>Select an incident to assign, document, or pass the Resolve Gate.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {(overview?.incidents ?? []).length === 0 ? (
                  <p className="text-sm text-muted-foreground">No incidents logged.</p>
                ) : (
                  (overview?.incidents ?? []).map((inc) => (
                    <div
                      key={inc.incidentId}
                      className="flex items-center justify-between rounded-md border p-3 text-sm"
                    >
                      <div>
                        <div className="flex items-center gap-2 font-medium">
                          <AlertTriangle className="h-4 w-4 text-amber-500" />
                          {inc.incidentType}
                          <Badge variant={inc.status === 'RESOLVED' ? 'outline' : 'destructive'}>
                            {inc.status}
                          </Badge>
                        </div>
                        <div className="text-xs text-muted-foreground">
                          Severity: {inc.severity} • Correlated: {inc.escalationCount} • Assignee: {inc.assignedTo}
                        </div>
                      </div>
                      <Button size="sm" variant="outline" onClick={() => setSelectedIncident(inc)}>
                        Manage
                      </Button>
                    </div>
                  ))
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Incident Resolve Gate</CardTitle>
                <CardDescription>
                  Requires an investigation note (&ge;10 chars), resolution summary (&ge;50 chars), and 10m quiet window.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                {!selectedIncident ? (
                  <p className="text-sm text-muted-foreground">Select an incident on the left to manage.</p>
                ) : (
                  <>
                    <div className="rounded bg-muted p-2 text-xs">
                      Selected: <span className="font-mono font-semibold">{selectedIncident.incidentId}</span>
                    </div>
                    <div className="flex gap-2">
                      <Input value={assignee} onChange={(e) => setAssignee(e.target.value)} placeholder="Assignee" />
                      <Button variant="outline" onClick={() => void handleAssignIncident(selectedIncident.incidentId)}>
                        Assign
                      </Button>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="incident-note">Attach Investigation Note (&ge;10 chars)</Label>
                      <div className="flex gap-2">
                        <Input
                          id="incident-note"
                          value={noteText}
                          onChange={(e) => setNoteText(e.target.value)}
                          placeholder="Root cause findings..."
                        />
                        <Button variant="secondary" onClick={() => void handleAddNote()}>
                          Add Note
                        </Button>
                      </div>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="resolution-summary">Resolution Summary (&ge;50 chars)</Label>
                      <Textarea
                        id="resolution-summary"
                        value={resolutionSummary}
                        onChange={(e) => setResolutionSummary(e.target.value)}
                        rows={3}
                        placeholder="Document permanent corrective action and verification evidence..."
                      />
                    </div>
                    <label className="flex items-center gap-2 text-xs text-muted-foreground">
                      <input
                        type="checkbox"
                        checked={skipQuietCheck}
                        onChange={(e) => setSkipQuietCheck(e.target.checked)}
                      />
                      Admin override: skip 10-minute quiet signal window check
                    </label>
                    <Button onClick={() => void handleResolveIncident()}>
                      <ShieldAlert className="mr-2 h-4 w-4" />
                      Resolve Incident
                    </Button>
                  </>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="forecasts">
          <Card>
            <CardHeader>
              <CardTitle>30-Minute Predictive Failure Forecasts</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {(overview?.forecasts ?? []).length === 0 ? (
                <p className="text-muted-foreground">No forecasts generated yet.</p>
              ) : (
                (overview?.forecasts ?? []).map((f) => (
                  <div key={f.forecastId} className="flex items-center justify-between rounded border p-3">
                    <div>
                      <div className="font-medium">
                        {f.sourceModule} / {f.eventType}
                      </div>
                      <div className="text-xs text-muted-foreground">{f.forecastTime}</div>
                    </div>
                    <div className="text-right">
                      <Badge variant={f.predictedProbability >= 0.5 ? 'destructive' : 'secondary'}>
                        Risk {(f.predictedProbability * 100).toFixed(0)}%
                      </Badge>
                      <div className="text-xs text-muted-foreground">
                        Confidence {(f.confidenceScore * 100).toFixed(0)}%
                      </div>
                    </div>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
