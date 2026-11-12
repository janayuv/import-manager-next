import * as React from 'react';
import { toast } from 'sonner';
import { Check, Plus, RotateCcw, ShieldCheck } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  workflowService,
  type AutomationCostLimit,
  type RuleRiskAssessment,
  type WorkflowRule,
} from '@/services/workflow-service';

export function AutomationRulesPage() {
  const [rules, setRules] = React.useState<WorkflowRule[]>([]);
  const [costLimits, setCostLimits] = React.useState<AutomationCostLimit | null>(null);
  const [lastRisk, setLastRisk] = React.useState<RuleRiskAssessment | null>(null);

  const [ruleName, setRuleName] = React.useState('');
  const [ruleType, setRuleType] = React.useState('SLA_ESCALATION');
  const [conditionExpr, setConditionExpr] = React.useState("priority == 'HIGH'");
  const [actionType, setActionType] = React.useState('AUTO_ESCALATE');
  const [canaryPct, setCanaryPct] = React.useState('25');

  const refresh = React.useCallback(async () => {
    try {
      const [r, cl] = await Promise.all([workflowService.listRules(), workflowService.getCostLimits()]);
      setRules(r);
      setCostLimits(cl);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load automation rules');
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleCreateRule = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await workflowService.saveRule({
        ruleName,
        ruleType,
        conditionExpression: conditionExpr,
        actionType,
        priority: 10,
        enabled: true,
        changeReason: 'Created in Automation Rules Console',
      });
      setRuleName('');
      toast.success('Rule version staged in env-dev');
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

  const handlePromote = async (rule: WorkflowRule, envId: string) => {
    if (!rule.latestVersionId) return;
    try {
      const risk = await workflowService.promoteRule(rule.ruleId, rule.latestVersionId, envId, false);
      setLastRisk(risk);
      toast.success(`Promoted to ${envId}`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : `Promotion to ${envId} blocked`);
    }
  };

  const handleSetCanary = async (rule: WorkflowRule) => {
    if (!rule.latestVersionId) return;
    try {
      await workflowService.configureCanary(rule.ruleId, rule.latestVersionId, Number(canaryPct));
      toast.success(`Canary set to ${canaryPct}% (FNV-1a deterministic)`);
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Canary update failed');
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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Automation Rules & Promotion</h1>
        <p className="text-sm text-muted-foreground">
          Rule versioning, sequential env-dev &rarr; env-test &rarr; env-prod promotion, blast-radius risk scoring, FNV-1a canary rollouts, and cycle cost caps.
        </p>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Stage New Decision Rule</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={(e) => void handleCreateRule(e)} className="space-y-3">
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
              <div>
                <Label htmlFor="rule-action">Action Type</Label>
                <Input id="rule-action" value={actionType} onChange={(e) => setActionType(e.target.value)} required />
              </div>
              <Button type="submit">
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
                <Button variant="outline" onClick={() => void handleSaveCostLimits()}>
                  Save Cost Limits
                </Button>
              </>
            )}
            {lastRisk && (
              <div className="rounded border bg-muted p-3 text-xs">
                Last Risk Evaluation: <strong>{lastRisk.level}</strong> ({lastRisk.score.toFixed(0)}){' '}
                {lastRisk.blocksProd ? '• Blocks PROD without override' : '• Safe for PROD'}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Rule Registry ({rules.length})</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {rules.length === 0 ? (
            <p className="text-sm text-muted-foreground">No workflow decision rules configured yet.</p>
          ) : (
            rules.map((r) => (
              <div key={r.ruleId} className="space-y-2 rounded border p-3 text-sm">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <span className="font-semibold">{r.ruleName}</span>{' '}
                    <Badge variant="outline">v{r.latestVersionNum}</Badge>{' '}
                    <Badge variant={r.enabled ? 'default' : 'secondary'}>
                      {r.enabled ? 'Active' : 'Disabled'}
                    </Badge>
                    {(r.canaryPercent ?? 0) > 0 && <Badge variant="secondary">Canary {r.canaryPercent}%</Badge>}
                  </div>
                  <div className="flex flex-wrap gap-1">
                    <Button size="sm" variant="outline" onClick={() => void handleApprove(r)}>
                      <Check className="mr-1 h-3 w-3" />
                      Approve
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => void handleEvaluateRisk(r)}>
                      <ShieldCheck className="mr-1 h-3 w-3" />
                      Risk
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => void handlePromote(r, 'env-dev')}>
                      &rarr; Dev
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => void handlePromote(r, 'env-test')}>
                      &rarr; Test
                    </Button>
                    <Button size="sm" onClick={() => void handlePromote(r, 'env-prod')}>
                      &rarr; Prod
                    </Button>
                    <Button size="sm" variant="destructive" onClick={() => void handleRollback(r.ruleId)}>
                      <RotateCcw className="mr-1 h-3 w-3" />
                      Rollback
                    </Button>
                  </div>
                </div>
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span>Condition: {r.conditionExpression}</span>
                  <span>• Action: {r.actionType}</span>
                  <Input
                    className="h-7 w-20"
                    type="number"
                    min={1}
                    max={100}
                    value={canaryPct}
                    onChange={(e) => setCanaryPct(e.target.value)}
                  />
                  <Button size="sm" variant="ghost" onClick={() => void handleSetCanary(r)}>
                    Set Canary %
                  </Button>
                </div>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    </div>
  );
}
