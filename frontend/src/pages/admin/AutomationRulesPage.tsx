import * as React from 'react';
import { toast } from 'sonner';
import {
  Activity,
  AlertTriangle,
  ArrowUpRight,
  Check,
  FlaskConical,
  GitCompare,
  History,
  Pencil,
  Plus,
  RotateCcw,
  ShieldAlert,
  ShieldCheck,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useOptionalAuth } from '@/contexts/AuthContext';
import {
  workflowService,
  type AutomationCostLimit,
  type CanaryMonitoringSummary,
  type CanaryRollbackThreshold,
  type DeploymentHealthOverview,
  type OperationalAlert,
  type RuleDryRunResult,
  type RuleRiskAssessment,
  type RuleVersionComparison,
  type WorkflowRule,
  type WorkflowRuleVersion,
} from '@/services/workflow-service';

const CANARY_STAGES = ['1', '5', '10', '25', '50', '100'] as const;

function healthBadgeVariant(status: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'HEALTHY':
      return 'default';
    case 'CRITICAL':
      return 'destructive';
    case 'WARNING':
      return 'secondary';
    default:
      return 'outline';
  }
}

export function AutomationRulesPage() {
  const auth = useOptionalAuth();
  const canMutate = auth ? auth.hasPermission('automation.mutate') : true;
  const canOverrideProd = auth ? auth.session?.role === 'administrator' : true;

  const [rules, setRules] = React.useState<WorkflowRule[]>([]);
  const [costLimits, setCostLimits] = React.useState<AutomationCostLimit | null>(null);
  const [lastRisk, setLastRisk] = React.useState<RuleRiskAssessment | null>(null);

  const [editingRuleId, setEditingRuleId] = React.useState<string>('');
  const [ruleName, setRuleName] = React.useState('');
  const [ruleType, setRuleType] = React.useState('SLA_ESCALATION');
  const [conditionExpr, setConditionExpr] = React.useState("priority == 'HIGH'");
  const [actionType, setActionType] = React.useState('AUTO_ESCALATE');
  const [priority, setPriority] = React.useState('10');
  const [enabled, setEnabled] = React.useState(true);
  const [changeReason, setChangeReason] = React.useState('Updated in Automation Rules Console');
  const [canaryInputs, setCanaryInputs] = React.useState<Record<string, string>>({});
  const [overrideReasons, setOverrideReasons] = React.useState<Record<string, string>>({});
  const [dryRunResult, setDryRunResult] = React.useState<RuleDryRunResult | null>(null);
  const [canaryMonitoring, setCanaryMonitoring] = React.useState<CanaryMonitoringSummary | null>(null);
  const [thresholds, setThresholds] = React.useState<CanaryRollbackThreshold | null>(null);
  const [alerts, setAlerts] = React.useState<OperationalAlert[]>([]);
  const [healthOverview, setHealthOverview] = React.useState<DeploymentHealthOverview | null>(null);
  const [compareRuleId, setCompareRuleId] = React.useState<string>('');
  const [baseVersionId, setBaseVersionId] = React.useState<string>('');
  const [targetVersionId, setTargetVersionId] = React.useState<string>('');
  const [versionComparison, setVersionComparison] = React.useState<RuleVersionComparison | null>(null);
  const [comparingVersions, setComparingVersions] = React.useState(false);
  const [expandedVersionsRuleId, setExpandedVersionsRuleId] = React.useState<string>('');
  const [ruleVersions, setRuleVersions] = React.useState<Record<string, WorkflowRuleVersion[]>>({});

  const loadVersionsForComparison = React.useCallback(async (ruleId: string, preferredBase = '', preferredTarget = '') => {
    if (!ruleId) return;
    try {
      const list = await workflowService.listRuleVersions(ruleId);
      setRuleVersions((prev) => ({ ...prev, [ruleId]: list }));
      if (list.length > 0) {
        const defaultTarget = preferredTarget || list[0]!.versionId;
        const defaultBase =
          preferredBase ||
          (list.length > 1 ? list[1]!.versionId : list[0]!.versionId);
        setTargetVersionId(defaultTarget);
        setBaseVersionId(defaultBase);
      } else {
        setBaseVersionId('');
        setTargetVersionId('');
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load versions for comparison');
    }
  }, []);

  const refresh = React.useCallback(async () => {
    try {
      const [r, cl, th, al, ov] = await Promise.all([
        workflowService.listRules(),
        workflowService.getCostLimits(),
        workflowService.getRollbackThresholds(),
        workflowService.listOperationalAlerts(false),
        workflowService.getDeploymentHealthOverview(),
      ]);
      setRules(r);
      setCostLimits(cl);
      setThresholds(th);
      setAlerts(al);
      setHealthOverview(ov);
      setCanaryInputs((prev) => {
        const next: Record<string, string> = { ...prev };
        for (const item of r) {
          if (next[item.ruleId] === undefined) {
            next[item.ruleId] = item.canaryPercent && item.canaryPercent > 0 ? String(item.canaryPercent) : '25';
          }
        }
        return next;
      });
      if (!compareRuleId && r.length > 0) {
        const first = r[0]!;
        setCompareRuleId(first.ruleId);
        void loadVersionsForComparison(first.ruleId, first.prodVersionId || '', first.latestVersionId || '');
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load automation rules');
    }
  }, [compareRuleId, loadVersionsForComparison]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const resetForm = () => {
    setEditingRuleId('');
    setRuleName('');
    setRuleType('SLA_ESCALATION');
    setConditionExpr("priority == 'HIGH'");
    setActionType('AUTO_ESCALATE');
    setPriority('10');
    setEnabled(true);
    setChangeReason('Created in Automation Rules Console');
  };

  const handleStartEdit = (r: WorkflowRule) => {
    setEditingRuleId(r.ruleId);
    setRuleName(r.ruleName);
    setRuleType(r.ruleType);
    setConditionExpr(r.conditionExpression);
    setActionType(r.actionType);
    setPriority(String(r.priority));
    setEnabled(r.enabled);
    setChangeReason('Updated rule configuration');
  };

  const handleSaveRule = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await workflowService.saveRule({
        ruleId: editingRuleId || undefined,
        ruleName,
        ruleType,
        conditionExpression: conditionExpr,
        actionType,
        priority: Number(priority) || 0,
        enabled,
        changeReason: changeReason.trim() || 'Saved in Automation Rules Console',
      });
      toast.success(editingRuleId ? 'New rule version staged in env-dev' : 'Rule version staged in env-dev');
      resetForm();
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save rule');
    }
  };

  const handleApprove = async (rule: WorkflowRule) => {
    if (!rule.latestVersionId) return;
    try {
      await workflowService.approveRuleVersion(rule.ruleId, rule.latestVersionId);
      toast.success(`Version ${rule.latestVersionNum} approved`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Approval failed');
    }
  };

  const handleEvaluateRisk = async (rule: WorkflowRule) => {
    if (!rule.latestVersionId) return;
    try {
      const assessment = await workflowService.evaluateRuleRisk(rule.ruleId, rule.latestVersionId);
      setLastRisk(assessment);
      toast.info(`Risk ${assessment.level} (score ${assessment.score.toFixed(0)})`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Risk assessment failed');
    }
  };

  const handleDryRun = async (rule: WorkflowRule) => {
    if (!rule.latestVersionId) return;
    try {
      const sim = await workflowService.dryRunRuleVersion(rule.ruleId, rule.latestVersionId);
      setDryRunResult(sim);
      setLastRisk(sim.risk);
      toast.info(`Dry run: ${sim.eligibleCount} eligible (${sim.canaryRoutedCount} in canary cohort)`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Dry run failed');
    }
  };

  const handleToggleVersions = async (ruleId: string) => {
    if (expandedVersionsRuleId === ruleId) {
      setExpandedVersionsRuleId('');
      return;
    }
    try {
      const list = await workflowService.listRuleVersions(ruleId);
      setRuleVersions((prev) => ({ ...prev, [ruleId]: list }));
      setExpandedVersionsRuleId(ruleId);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load rule versions');
    }
  };

  const handleRestoreVersion = async (ruleId: string, versionId: string, adminOverride = false) => {
    const rawReason = (overrideReasons[ruleId] ?? '').trim();
    const reason = adminOverride ? rawReason : rawReason || 'Restored verified historical version';
    if (adminOverride && reason.length < 5) {
      toast.error('Enter an override reason (>= 5 characters) before forcing PROD version restore');
      return;
    }
    try {
      await workflowService.restoreRuleVersion(ruleId, versionId, reason, adminOverride);
      toast.success(`Restored ${versionId} to env-prod${adminOverride ? ' (admin override)' : ''}`);
      await refresh();
      const list = await workflowService.listRuleVersions(ruleId);
      setRuleVersions((prev) => ({ ...prev, [ruleId]: list }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Version restore failed');
    }
  };

  const handlePromote = async (rule: WorkflowRule, envId: string, adminOverride = false) => {
    if (!rule.latestVersionId) return;
    const reason = adminOverride ? (overrideReasons[rule.ruleId] ?? '').trim() : '';
    if (adminOverride && reason.length < 5) {
      toast.error('Enter an override reason (>= 5 characters) before forcing PROD promotion');
      return;
    }
    try {
      const risk = await workflowService.promoteRule(rule.ruleId, rule.latestVersionId, envId, adminOverride, reason);
      setLastRisk(risk);
      toast.success(`Promoted to ${envId}${adminOverride ? ' (admin override)' : ''}`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : `Promotion to ${envId} blocked`);
    }
  };

  const handleSetCanary = async (rule: WorkflowRule) => {
    if (!rule.latestVersionId) return;
    const pct = Number(canaryInputs[rule.ruleId] ?? '25');
    try {
      await workflowService.configureCanary(rule.ruleId, rule.latestVersionId, pct);
      toast.success(`Canary set to ${pct}% (FNV-1a deterministic)`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Canary update failed');
    }
  };

  const handleAdvanceCanary = async (rule: WorkflowRule) => {
    if (!rule.latestVersionId) return;
    try {
      const st = await workflowService.advanceCanaryStage(rule.ruleId, rule.latestVersionId);
      setCanaryInputs((prev) => ({ ...prev, [rule.ruleId]: String(st.sampleSizePercentage) }));
      toast.success(
        st.deploymentStatus === 'COMPLETED'
          ? 'Canary rollout completed at 100% in production'
          : `Canary advanced to stage ${st.rolloutStage} (${st.sampleSizePercentage}%)`,
      );
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Advance canary stage failed');
    }
  };

  const handleInspectMonitoring = async (rule: WorkflowRule) => {
    const vid = rule.prodVersionId || rule.latestVersionId;
    if (!vid) return;
    try {
      const mon = await workflowService.getCanaryMonitoring(rule.ruleId, vid);
      setCanaryMonitoring(mon);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load canary monitoring');
    }
  };

  const handleSaveThresholds = async () => {
    if (!thresholds) return;
    try {
      const updated = await workflowService.updateRollbackThresholds(thresholds);
      setThresholds(updated);
      toast.success('Auto-rollback thresholds updated');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update rollback thresholds');
    }
  };

  const handleAcknowledgeAlert = async (alertId: string) => {
    try {
      await workflowService.acknowledgeOperationalAlert(alertId);
      toast.success('Operational alert acknowledged');
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to acknowledge alert');
    }
  };

  const handleRollback = async (ruleId: string) => {
    try {
      await workflowService.rollbackRule(ruleId, 'Manual operator rollback');
      toast.success('Rule rolled back and disabled');
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Rollback failed');
    }
  };

  const handleSaveCostLimits = async () => {
    if (!costLimits) return;
    try {
      const updated = await workflowService.updateCostLimits(costLimits);
      setCostLimits(updated);
      toast.success('Automation cost limits updated');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update cost limits');
    }
  };

  const handleCompareRuleChange = (ruleId: string) => {
    setCompareRuleId(ruleId);
    setVersionComparison(null);
    const match = rules.find((item) => item.ruleId === ruleId);
    void loadVersionsForComparison(ruleId, match?.prodVersionId || '', match?.latestVersionId || '');
  };

  const handleRunVersionComparison = async () => {
    if (!compareRuleId || !baseVersionId || !targetVersionId) {
      toast.error('Select a rule, base version, and target version to compare');
      return;
    }
    setComparingVersions(true);
    try {
      const cmp = await workflowService.compareRuleVersions(compareRuleId, baseVersionId, targetVersionId);
      setVersionComparison(cmp);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Version comparison failed');
    } finally {
      setComparingVersions(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Automation Rules & Promotion</h1>
        <p className="text-sm text-muted-foreground">
          Rule versioning, sequential env-dev &rarr; env-test &rarr; env-prod promotion, blast-radius risk scoring, FNV-1a canary rollouts, and cycle cost caps.
        </p>
      </div>

      {healthOverview && (
        <div className="space-y-4">
          <Card>
            <CardHeader className="pb-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Activity className="h-4 w-4" />
                    Deployment Health Overview
                  </CardTitle>
                  <CardDescription>{healthOverview.healthSummaryReason}</CardDescription>
                </div>
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <Badge variant={healthBadgeVariant(healthOverview.overallHealth)}>
                    {healthOverview.overallHealth}
                  </Badge>
                  {healthOverview.currentProductionVersionId && (
                    <Badge variant="outline" className="font-mono">
                      Prod: {healthOverview.currentProductionVersionId}
                    </Badge>
                  )}
                  {healthOverview.latestDeploymentTimestamp && (
                    <span className="text-muted-foreground">
                      Latest deploy: {healthOverview.latestDeploymentTimestamp}
                    </span>
                  )}
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-6">
                <div className="rounded-md border p-2.5">
                  <div className="text-[11px] text-muted-foreground">Total Rules</div>
                  <div className="text-xl font-bold">{healthOverview.totalRules}</div>
                </div>
                <div className="rounded-md border p-2.5">
                  <div className="text-[11px] text-muted-foreground">Active Production</div>
                  <div className="text-xl font-bold">{healthOverview.activeProductionRules}</div>
                </div>
                <div className="rounded-md border p-2.5">
                  <div className="text-[11px] text-muted-foreground">Awaiting Approval</div>
                  <div className="text-xl font-bold">{healthOverview.rulesAwaitingApproval}</div>
                </div>
                <div className="rounded-md border p-2.5">
                  <div className="text-[11px] text-muted-foreground">Active Canaries</div>
                  <div className="text-xl font-bold">{healthOverview.activeCanaryDeployments}</div>
                </div>
                <div className="rounded-md border p-2.5">
                  <div className="text-[11px] text-muted-foreground">Completed Canaries</div>
                  <div className="text-xl font-bold">{healthOverview.completedCanaryDeployments}</div>
                </div>
                <div className="rounded-md border p-2.5">
                  <div className="text-[11px] text-muted-foreground">Failed / Rolled Back</div>
                  <div className="text-xl font-bold text-destructive">{healthOverview.failedOrRolledBackDeployments}</div>
                </div>
              </div>

              <div className="space-y-2">
                <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Production Deployment &amp; Health
                </div>
                {(healthOverview.ruleDeployments ?? []).length === 0 ? (
                  <p className="text-xs text-muted-foreground">No rule deployments recorded yet.</p>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Rule</TableHead>
                        <TableHead>Env / Version</TableHead>
                        <TableHead>Canary Progress</TableHead>
                        <TableHead>Health</TableHead>
                        <TableHead>Failure &amp; Cost ({healthOverview.windowMinutes}m)</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {healthOverview.ruleDeployments.map((dep) => (
                        <TableRow key={dep.ruleId}>
                          <TableCell>
                            <div className="font-medium">{dep.ruleName}</div>
                            <div className="text-[11px] text-muted-foreground">
                              {dep.ruleType} &bull; {dep.actionType}
                            </div>
                          </TableCell>
                          <TableCell className="text-xs">
                            <div className="flex flex-wrap items-center gap-1">
                              <Badge variant="outline">{dep.currentEnv || 'staged'}</Badge>
                              {(dep.prodVersionNum ?? 0) > 0 && (
                                <Badge variant="secondary">v{dep.prodVersionNum}</Badge>
                              )}
                              {dep.latestApprovalStatus === 'PENDING' && (
                                <Badge variant="outline">PENDING</Badge>
                              )}
                            </div>
                            {dep.latestDeployedAt && (
                              <div className="mt-0.5 text-[11px] text-muted-foreground">{dep.latestDeployedAt}</div>
                            )}
                          </TableCell>
                          <TableCell className="text-xs">
                            {dep.canaryStatus ? (
                              <div className="space-y-1">
                                <div className="flex items-center gap-1.5">
                                  <Badge variant={dep.canaryStatus === 'ROLLED_BACK' ? 'destructive' : 'outline'}>
                                    {dep.canaryStatus}
                                  </Badge>
                                  <span>{dep.canaryPercent ?? 0}% (Stage {dep.canaryRolloutStage ?? 0}/6)</span>
                                </div>
                                <div className="h-1.5 w-28 overflow-hidden rounded-full bg-muted">
                                  <div
                                    className="h-full bg-primary transition-all"
                                    style={{ width: `${Math.min(100, Math.max(0, dep.canaryPercent ?? 0))}%` }}
                                  />
                                </div>
                              </div>
                            ) : (
                              <span className="text-muted-foreground">—</span>
                            )}
                          </TableCell>
                          <TableCell className="text-xs">
                            <Badge variant={healthBadgeVariant(dep.healthStatus)}>{dep.healthStatus}</Badge>
                            {dep.healthReasons && dep.healthReasons.length > 0 && (
                              <div className="mt-1 max-w-xs text-[11px] text-muted-foreground">
                                {dep.healthReasons[0]}
                              </div>
                            )}
                          </TableCell>
                          <TableCell className="text-xs font-mono">
                            <div>
                              Fail: {dep.failureCount}/{dep.attemptedCount} ({(dep.failureRate * 100).toFixed(1)}%)
                            </div>
                            <div className="text-muted-foreground">
                              Cost: {dep.costUnits.toFixed(1)} (+{dep.costIncreasePct.toFixed(1)}%)
                              {dep.openAlertCount > 0 ? ` • ${dep.openAlertCount} open alert(s)` : ''}
                            </div>
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => handleCompareRuleChange(dep.ruleId)}
                            >
                              <GitCompare className="mr-1 h-3 w-3" />
                              Select Diff
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between gap-2">
              <CardTitle>{editingRuleId ? 'Edit Decision Rule (New Version)' : 'Stage New Decision Rule'}</CardTitle>
              {editingRuleId && (
                <Button type="button" size="sm" variant="ghost" onClick={resetForm}>
                  <X className="mr-1 h-3 w-3" />
                  Cancel
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent>
            <form onSubmit={(e) => void handleSaveRule(e)} className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label htmlFor="rule-name">Rule Name</Label>
                  <Input id="rule-name" value={ruleName} onChange={(e) => setRuleName(e.target.value)} required />
                </div>
                <div>
                  <Label htmlFor="rule-type">Rule Type</Label>
                  <Input id="rule-type" value={ruleType} onChange={(e) => setRuleType(e.target.value)} required />
                </div>
              </div>
              <div>
                <Label htmlFor="rule-cond">Condition Expression</Label>
                <Input
                  id="rule-cond"
                  value={conditionExpr}
                  onChange={(e) => setConditionExpr(e.target.value)}
                  required
                />
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <Label htmlFor="rule-action">Action Type</Label>
                  <Input id="rule-action" value={actionType} onChange={(e) => setActionType(e.target.value)} required />
                </div>
                <div>
                  <Label htmlFor="rule-priority">Priority</Label>
                  <Input
                    id="rule-priority"
                    type="number"
                    value={priority}
                    onChange={(e) => setPriority(e.target.value)}
                  />
                </div>
                <div className="flex items-end pb-2">
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={enabled}
                      onChange={(e) => setEnabled(e.target.checked)}
                    />
                    Enabled
                  </label>
                </div>
              </div>
              <div>
                <Label htmlFor="rule-reason">Change Reason</Label>
                <Input
                  id="rule-reason"
                  value={changeReason}
                  onChange={(e) => setChangeReason(e.target.value)}
                />
              </div>
              <Button type="submit" disabled={!canMutate}>
                <Plus className="mr-2 h-4 w-4" />
                Save & Stage Version
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Cycle Cost Guardrails</CardTitle>
            <CardDescription>Hard circuit limits enforced per automation cycle.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {costLimits && (
              <>
                <div className="grid grid-cols-3 gap-2">
                  <div>
                    <Label htmlFor="cost-units">Max Cost Units</Label>
                    <Input
                      id="cost-units"
                      type="number"
                      value={costLimits.maxCostUnitsPerCycle}
                      onChange={(e) =>
                        setCostLimits({ ...costLimits, maxCostUnitsPerCycle: Number(e.target.value) })
                      }
                    />
                  </div>
                  <div>
                    <Label htmlFor="cost-ms">Max Time (ms)</Label>
                    <Input
                      id="cost-ms"
                      type="number"
                      value={costLimits.maxExecutionTimePerCycleMs}
                      onChange={(e) =>
                        setCostLimits({ ...costLimits, maxExecutionTimePerCycleMs: Number(e.target.value) })
                      }
                    />
                  </div>
                  <div>
                    <Label htmlFor="cost-records">Max Records</Label>
                    <Input
                      id="cost-records"
                      type="number"
                      value={costLimits.maxRecordsProcessedPerCycle}
                      onChange={(e) =>
                        setCostLimits({ ...costLimits, maxRecordsProcessedPerCycle: Number(e.target.value) })
                      }
                    />
                  </div>
                </div>
                <Button variant="outline" disabled={!canMutate} onClick={() => void handleSaveCostLimits()}>
                  Save Cost Limits
                </Button>
              </>
            )}
            {lastRisk && (
              <div className="rounded border bg-muted p-3 text-xs">
                <div>
                  Last Risk Evaluation ({lastRisk.ruleId}): <strong>{lastRisk.level}</strong> ({lastRisk.score.toFixed(0)}){' '}
                  {lastRisk.blocksProd ? '• Blocks PROD without override' : '• Safe for PROD'}
                </div>
                {lastRisk.factors && lastRisk.factors.length > 0 && (
                  <div className="mt-1 text-muted-foreground">Factors: {lastRisk.factors.join('; ')}</div>
                )}
              </div>
            )}
            {dryRunResult && (
              <div className="rounded border bg-muted/60 p-3 text-xs space-y-1.5">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="font-semibold">
                    Dry Run Preview ({dryRunResult.versionId} • {dryRunResult.actionType} •{' '}
                    {dryRunResult.ruleEnabled ? 'Enabled' : 'Disabled'})
                  </span>
                  <Badge variant={dryRunResult.withinBudget ? 'default' : 'destructive'}>
                    {dryRunResult.withinBudget ? 'Within Budget' : 'Exceeds Budget'}
                  </Badge>
                </div>
                <div className="text-muted-foreground">
                  Open Cases: {dryRunResult.totalOpenCases} • Eligible: {dryRunResult.eligibleCount} • Canary Routed:{' '}
                  {dryRunResult.canaryRoutedCount} • Skipped (Idempotent): {dryRunResult.skippedIdempotentCount} • Est.
                  Cost: {dryRunResult.estimatedCostUnits.toFixed(1)} / {dryRunResult.costBudgetCap}
                </div>
                {dryRunResult.samples && dryRunResult.samples.length > 0 && (
                  <div className="space-y-1 pt-1">
                    {dryRunResult.samples.map((m) => (
                      <div key={m.exceptionCaseId} className="flex flex-wrap items-center gap-2 font-mono text-[11px]">
                        <span>{m.exceptionCaseId}</span>
                        <span>({m.exceptionType})</span>
                        <span>&rarr; {m.proposedAction}</span>
                        <Badge variant="outline" className="text-[10px]">
                          {m.alreadyApplied ? 'Already Applied' : m.canaryIncluded ? 'In Canary' : 'Control'}
                        </Badge>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
            {canaryMonitoring && (
              <div className="rounded border bg-muted/60 p-3 text-xs space-y-1.5">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="font-semibold">
                    Canary vs Control ({canaryMonitoring.versionId} • {canaryMonitoring.windowMinutes}m window)
                  </span>
                  <Badge variant={canaryMonitoring.healthStatus === 'DEGRADED' || canaryMonitoring.healthStatus === 'ROLLED_BACK' ? 'destructive' : 'default'}>
                    {canaryMonitoring.healthStatus}
                  </Badge>
                </div>
                <div>
                  <strong>Canary:</strong> attempted {canaryMonitoring.canary.attemptedCount} (ok{' '}
                  {canaryMonitoring.canary.successCount}, fail {canaryMonitoring.canary.failureCount}, skipped{' '}
                  {canaryMonitoring.canary.skippedCount}) • fail rate{' '}
                  {(canaryMonitoring.canary.failureRate * 100).toFixed(1)}% • cost{' '}
                  {canaryMonitoring.canary.costUnits.toFixed(1)} • avg {canaryMonitoring.canary.avgDurationMs.toFixed(1)}ms
                </div>
                <div className="text-muted-foreground">
                  <strong>Control:</strong> attempted {canaryMonitoring.control.attemptedCount} (ok{' '}
                  {canaryMonitoring.control.successCount}, fail {canaryMonitoring.control.failureCount}) • avg{' '}
                  {canaryMonitoring.control.avgDurationMs.toFixed(1)}ms • Cost +{canaryMonitoring.costIncreasePct.toFixed(1)}% • Latency +{canaryMonitoring.durationIncreasePct.toFixed(1)}%
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {thresholds && (
        <Card>
          <CardHeader>
            <CardTitle>Automatic Canary Rollback Thresholds</CardTitle>
            <CardDescription>
              Two-stage recovery triggers Stage 1 emergency disable and Stage 2 safe previous-version restore when breached.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid gap-3 sm:grid-cols-5">
              <div>
                <Label htmlFor="th-min-sample">Min Sample Size</Label>
                <Input
                  id="th-min-sample"
                  type="number"
                  min={1}
                  value={thresholds.minSampleSize}
                  onChange={(e) => setThresholds({ ...thresholds, minSampleSize: Number(e.target.value) })}
                />
              </div>
              <div>
                <Label htmlFor="th-fail-rate">Max Failure Rate (0-1)</Label>
                <Input
                  id="th-fail-rate"
                  type="number"
                  step="0.05"
                  min={0.01}
                  max={1}
                  value={thresholds.maxFailureRate}
                  onChange={(e) => setThresholds({ ...thresholds, maxFailureRate: Number(e.target.value) })}
                />
              </div>
              <div>
                <Label htmlFor="th-cost-pct">Max Cost Increase %</Label>
                <Input
                  id="th-cost-pct"
                  type="number"
                  min={0}
                  value={thresholds.maxCostIncreasePct}
                  onChange={(e) => setThresholds({ ...thresholds, maxCostIncreasePct: Number(e.target.value) })}
                />
              </div>
              <div>
                <Label htmlFor="th-dur-pct">Max Latency Increase %</Label>
                <Input
                  id="th-dur-pct"
                  type="number"
                  min={0}
                  value={thresholds.maxDurationIncreasePct}
                  onChange={(e) => setThresholds({ ...thresholds, maxDurationIncreasePct: Number(e.target.value) })}
                />
              </div>
              <div>
                <Label htmlFor="th-window">Window (minutes)</Label>
                <Input
                  id="th-window"
                  type="number"
                  min={1}
                  value={thresholds.monitoringWindowMinutes}
                  onChange={(e) => setThresholds({ ...thresholds, monitoringWindowMinutes: Number(e.target.value) })}
                />
              </div>
            </div>
            <Button variant="outline" disabled={!canMutate} onClick={() => void handleSaveThresholds()}>
              Save Rollback Thresholds
            </Button>
          </CardContent>
        </Card>
      )}

      {alerts.length > 0 && (
        <Card className="border-destructive/40">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <AlertTriangle className="h-4 w-4 text-destructive" />
              Operational Alerts ({alerts.length})
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {alerts.map((al) => (
              <div key={al.id} className="flex flex-wrap items-center justify-between gap-2 rounded border p-2 text-xs">
                <div className="space-y-0.5">
                  <div className="flex items-center gap-1.5">
                    <Badge variant={al.severity === 'CRITICAL' ? 'destructive' : 'secondary'}>{al.severity}</Badge>
                    <span className="font-mono font-semibold">{al.alertType}</span>
                    {al.ruleId && <span className="text-muted-foreground">({al.ruleId})</span>}
                  </div>
                  <div>{al.message}</div>
                </div>
                <Button size="sm" variant="outline" disabled={!canMutate} onClick={() => void handleAcknowledgeAlert(al.id)}>
                  Acknowledge
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {healthOverview && (
        <div className="grid gap-4 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <GitCompare className="h-4 w-4" />
                Rule Version Comparison &amp; Dry-Run Impact
              </CardTitle>
              <CardDescription>
                Compare two immutable versions of the same rule under equivalent read-only dry-run assumptions.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid gap-2 sm:grid-cols-3">
                <div>
                  <Label htmlFor="cmp-rule">Rule</Label>
                  <select
                    id="cmp-rule"
                    aria-label="Comparison Rule"
                    className="h-9 w-full rounded-md border border-input bg-background px-2 text-xs"
                    value={compareRuleId}
                    onChange={(e) => handleCompareRuleChange(e.target.value)}
                  >
                    <option value="">Select rule</option>
                    {rules.map((r) => (
                      <option key={r.ruleId} value={r.ruleId}>
                        {r.ruleName}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <Label htmlFor="cmp-base">Base Version</Label>
                  <select
                    id="cmp-base"
                    aria-label="Base Version"
                    className="h-9 w-full rounded-md border border-input bg-background px-2 text-xs"
                    value={baseVersionId}
                    onChange={(e) => setBaseVersionId(e.target.value)}
                  >
                    <option value="">Select base</option>
                    {(ruleVersions[compareRuleId] ?? []).map((v) => (
                      <option key={v.versionId} value={v.versionId}>
                        v{v.versionNumber} ({v.approvalStatus || 'PENDING'})
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <Label htmlFor="cmp-target">Target Version</Label>
                  <select
                    id="cmp-target"
                    aria-label="Target Version"
                    className="h-9 w-full rounded-md border border-input bg-background px-2 text-xs"
                    value={targetVersionId}
                    onChange={(e) => setTargetVersionId(e.target.value)}
                  >
                    <option value="">Select target</option>
                    {(ruleVersions[compareRuleId] ?? []).map((v) => (
                      <option key={v.versionId} value={v.versionId}>
                        v{v.versionNumber} ({v.approvalStatus || 'PENDING'})
                      </option>
                    ))}
                  </select>
                </div>
              </div>
              <Button
                type="button"
                variant="outline"
                disabled={comparingVersions || !compareRuleId || !baseVersionId || !targetVersionId}
                onClick={() => void handleRunVersionComparison()}
              >
                <GitCompare className="mr-1.5 h-4 w-4" />
                Compare Versions
              </Button>

              {versionComparison && (
                <div className="space-y-3 rounded-md border bg-muted/40 p-3 text-xs">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="font-semibold">
                      v{versionComparison.base.versionNumber} &rarr; v{versionComparison.target.versionNumber} ({versionComparison.ruleId})
                    </span>
                    <Badge variant={versionComparison.riskScoreDelta > 0 ? 'destructive' : 'outline'}>
                      {versionComparison.riskDifferenceSummary}
                    </Badge>
                  </div>
                  {(versionComparison.importantChanges ?? []).length > 0 && (
                    <ul className="list-disc space-y-0.5 pl-4 text-muted-foreground">
                      {versionComparison.importantChanges.map((chg, idx) => (
                        <li key={idx}>{chg}</li>
                      ))}
                    </ul>
                  )}
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Attribute</TableHead>
                        <TableHead>Base</TableHead>
                        <TableHead>Target</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(versionComparison.fieldDiffs ?? []).map((d) => (
                        <TableRow key={d.field} className={d.changed ? 'bg-amber-500/5' : ''}>
                          <TableCell className="font-mono text-xs">{d.field}</TableCell>
                          <TableCell className="font-mono text-xs">{d.baseValue}</TableCell>
                          <TableCell className="font-mono text-xs font-medium">{d.targetValue}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                  <div className="rounded border bg-background p-2.5 space-y-1">
                    <div className="font-semibold">
                      Dry-Run Impact Comparison ({versionComparison.dryRunImpact.evaluatedOpenCases} open cases, {versionComparison.dryRunImpact.assumedCanaryPercent}% scope)
                    </div>
                    <div className="grid gap-2 sm:grid-cols-4 font-mono text-[11px]">
                      <div>
                        Matching: {versionComparison.dryRunImpact.baseMatchingRecords} &rarr;{' '}
                        {versionComparison.dryRunImpact.targetMatchingRecords} (
                        {versionComparison.dryRunImpact.matchingRecordsDelta >= 0 ? '+' : ''}
                        {versionComparison.dryRunImpact.matchingRecordsDelta})
                      </div>
                      <div>
                        Affected: {versionComparison.dryRunImpact.baseAffectedRecords} &rarr;{' '}
                        {versionComparison.dryRunImpact.targetAffectedRecords} (
                        {versionComparison.dryRunImpact.affectedRecordsDelta >= 0 ? '+' : ''}
                        {versionComparison.dryRunImpact.affectedRecordsDelta})
                      </div>
                      <div>
                        Est. Cost: {versionComparison.dryRunImpact.baseEstimatedCost.toFixed(1)} &rarr;{' '}
                        {versionComparison.dryRunImpact.targetEstimatedCost.toFixed(1)} (
                        {versionComparison.dryRunImpact.estimatedCostDelta >= 0 ? '+' : ''}
                        {versionComparison.dryRunImpact.estimatedCostDelta.toFixed(1)})
                      </div>
                      <div>
                        Skipped: {versionComparison.dryRunImpact.baseSkippedRecords} &rarr;{' '}
                        {versionComparison.dryRunImpact.targetSkippedRecords} (
                        {versionComparison.dryRunImpact.skippedRecordsDelta >= 0 ? '+' : ''}
                        {versionComparison.dryRunImpact.skippedRecordsDelta})
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <RotateCcw className="h-4 w-4" />
                Recent Rollback Activity ({(healthOverview.recentRollbacks ?? []).length})
              </CardTitle>
              <CardDescription>
                Automatic Stage 1/Stage 2 canary recovery and manual rollback audit log.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              {(healthOverview.recentRollbacks ?? []).length === 0 ? (
                <p className="text-xs text-muted-foreground">No rollback events recorded.</p>
              ) : (
                healthOverview.recentRollbacks.slice(0, 10).map((rbk) => (
                  <div key={rbk.id} className="rounded border p-2 text-xs space-y-0.5">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex items-center gap-1.5">
                        <Badge variant={rbk.rollbackType === 'AUTO_ROLLBACK_DISABLE' ? 'destructive' : 'outline'}>
                          {rbk.rollbackType}
                        </Badge>
                        <span className="font-mono font-medium">{rbk.ruleId}</span>
                        {rbk.targetEntity && rbk.targetEntity !== rbk.ruleId && (
                          <span className="text-muted-foreground">&rarr; {rbk.targetEntity}</span>
                        )}
                      </div>
                      <span className="text-[11px] text-muted-foreground">{rbk.performedAt}</span>
                    </div>
                    <div className="text-[11px] text-muted-foreground">Performed by: {rbk.performedBy}</div>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Rule Registry ({rules.length})</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {rules.length === 0 ? (
            <p className="text-sm text-muted-foreground">No workflow decision rules configured yet.</p>
          ) : (
            rules.map((r) => {
              const rowCanary = canaryInputs[r.ruleId] ?? '25';
              const isHighRisk = lastRisk?.ruleId === r.ruleId && lastRisk.blocksProd;
              return (
                <div key={r.ruleId} className="space-y-2 rounded border p-3 text-sm">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span className="font-semibold">{r.ruleName}</span>
                      <Badge variant="outline">v{r.latestVersionNum}</Badge>
                      {r.latestApprovalStatus && (
                        <Badge variant={r.latestApprovalStatus === 'APPROVED' ? 'default' : 'secondary'}>
                          {r.latestApprovalStatus}
                        </Badge>
                      )}
                      <Badge variant={r.enabled ? 'default' : 'secondary'}>
                        {r.enabled ? 'Active' : 'Disabled'}
                      </Badge>
                      <Badge variant="outline">{r.currentEnv || 'staged'}</Badge>
                      {(r.prodVersionNum ?? 0) > 0 && (
                        <Badge variant="outline">Prod v{r.prodVersionNum}</Badge>
                      )}
                      {r.latestVersionSha256 && (
                        <Badge variant="outline" className="font-mono text-[10px]" title={r.latestVersionSha256}>
                          sha:{r.latestVersionSha256.slice(0, 8)}
                        </Badge>
                      )}
                      {(r.canaryPercent ?? 0) > 0 && <Badge variant="secondary">Canary {r.canaryPercent}%</Badge>}
                    </div>
                    <div className="flex flex-wrap gap-1">
                      <Button size="sm" variant="ghost" disabled={!canMutate} onClick={() => handleStartEdit(r)}>
                        <Pencil className="mr-1 h-3 w-3" />
                        Edit
                      </Button>
                      <Button size="sm" variant="outline" disabled={!canMutate} onClick={() => void handleApprove(r)}>
                        <Check className="mr-1 h-3 w-3" />
                        Approve
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => void handleEvaluateRisk(r)}>
                        <ShieldCheck className="mr-1 h-3 w-3" />
                        Risk
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => void handleDryRun(r)}>
                        <FlaskConical className="mr-1 h-3 w-3" />
                        Dry Run
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => void handleToggleVersions(r.ruleId)}>
                        <History className="mr-1 h-3 w-3" />
                        Versions
                      </Button>
                      <Button size="sm" variant="outline" disabled={!canMutate} onClick={() => void handlePromote(r, 'env-dev')}>
                        &rarr; Dev
                      </Button>
                      <Button size="sm" variant="outline" disabled={!canMutate} onClick={() => void handlePromote(r, 'env-test')}>
                        &rarr; Test
                      </Button>
                      <Button size="sm" disabled={!canMutate} onClick={() => void handlePromote(r, 'env-prod')}>
                        &rarr; Prod
                      </Button>
                      {isHighRisk && canMutate && canOverrideProd && (
                        <div className="flex items-center gap-1">
                          <Input
                            className="h-8 w-44 text-xs"
                            placeholder="Override reason (>=5 chars)"
                            value={overrideReasons[r.ruleId] ?? ''}
                            onChange={(e) =>
                              setOverrideReasons((prev) => ({ ...prev, [r.ruleId]: e.target.value }))
                            }
                          />
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => void handlePromote(r, 'env-prod', true)}
                            title="Override HIGH/CRITICAL risk block for PROD"
                          >
                            <ShieldAlert className="mr-1 h-3 w-3" />
                            Force Prod
                          </Button>
                        </div>
                      )}
                      <Button size="sm" variant="destructive" disabled={!canMutate} onClick={() => void handleRollback(r.ruleId)}>
                        <RotateCcw className="mr-1 h-3 w-3" />
                        Rollback
                      </Button>
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    <span>Priority: {r.priority}</span>
                    <span>• Condition: {r.conditionExpression}</span>
                    <span>• Action: {r.actionType}</span>
                    <select
                      aria-label={`Canary stage for ${r.ruleName}`}
                      className="h-7 rounded-md border border-input bg-background px-2 text-xs"
                      value={CANARY_STAGES.includes(rowCanary as (typeof CANARY_STAGES)[number]) ? rowCanary : '25'}
                      onChange={(e) =>
                        setCanaryInputs((prev) => ({ ...prev, [r.ruleId]: e.target.value }))
                      }
                    >
                      {CANARY_STAGES.map((st) => (
                        <option key={st} value={st}>
                          {st}%
                        </option>
                      ))}
                    </select>
                    <Button size="sm" variant="ghost" disabled={!canMutate} onClick={() => void handleSetCanary(r)}>
                      Set Canary %
                    </Button>
                    <Button size="sm" variant="outline" disabled={!canMutate} onClick={() => void handleAdvanceCanary(r)}>
                      <ArrowUpRight className="mr-1 h-3 w-3" />
                      Advance Stage
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => void handleInspectMonitoring(r)}>
                      <Activity className="mr-1 h-3 w-3" />
                      Monitor
                    </Button>
                  </div>
                  {expandedVersionsRuleId === r.ruleId && (
                    <div className="mt-2 space-y-1.5 rounded border bg-muted/40 p-2 text-xs">
                      <div className="font-semibold">Immutable Version History</div>
                      {(ruleVersions[r.ruleId] ?? []).map((ver) => (
                        <div key={ver.versionId} className="flex flex-wrap items-center justify-between gap-2 border-b pb-1 last:border-b-0 last:pb-0">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="font-mono font-medium">v{ver.versionNumber}</span>
                            <Badge variant="outline">{ver.approvalStatus || 'PENDING'}</Badge>
                            <span>{ver.actionType} (prio {ver.priority})</span>
                            <span className="font-mono text-[10px] text-muted-foreground">
                              sha:{ver.definitionSha256.slice(0, 8)}
                            </span>
                            {ver.changeReason && (
                              <span className="text-muted-foreground">• {ver.changeReason}</span>
                            )}
                          </div>
                          {canMutate && ver.approvalStatus === 'APPROVED' && (
                            <div className="flex items-center gap-1">
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => void handleRestoreVersion(r.ruleId, ver.versionId, false)}
                              >
                                Restore to Prod
                              </Button>
                              {canOverrideProd && (
                                <Button
                                  size="sm"
                                  variant="secondary"
                                  onClick={() => void handleRestoreVersion(r.ruleId, ver.versionId, true)}
                                  title="Force restore HIGH/CRITICAL historical version with override reason"
                                >
                                  Force Restore
                                </Button>
                              )}
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              );
            })
          )}
        </CardContent>
      </Card>
    </div>
  );
}
