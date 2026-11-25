import * as WorkflowBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/workflow/service.js';
import type {
  AutoRollbackResult,
  AutomationCostLimit,
  BackgroundJob,
  BugNote,
  CanaryCohortMetrics,
  CanaryMonitoringSummary,
  CanaryRollbackThreshold,
  CanaryStatus,
  DeploymentHealthOverview,
  DiagnosticsExportResult,
  DryRunComparisonDelta,
  ErrorMemoryRecord,
  ExceptionCase,
  ExceptionDetectionResult,
  FailureForecast,
  IncidentNote,
  JobExecutionRecord,
  OperationalAlert,
  OperationsOverview,
  RecordErrorInput,
  ResolveIncidentInput,
  RollbackEventRecord,
  RuleDeploymentHealth,
  RuleDryRunMatch,
  RuleDryRunResult,
  RuleRiskAssessment,
  RuleVersionComparison,
  RuleVersionFieldDiff,
  RuleVersionSummary,
  SaveBugNoteInput,
  SaveWorkflowRuleInput,
  WorkflowIncident,
  WorkflowRule,
  WorkflowRuleVersion,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  AutoRollbackResult,
  AutomationCostLimit,
  BackgroundJob,
  BugNote,
  CanaryCohortMetrics,
  CanaryMonitoringSummary,
  CanaryRollbackThreshold,
  CanaryStatus,
  DeploymentHealthOverview,
  DiagnosticsExportResult,
  DryRunComparisonDelta,
  ErrorMemoryRecord,
  ExceptionCase,
  ExceptionDetectionResult,
  FailureForecast,
  IncidentNote,
  JobExecutionRecord,
  OperationalAlert,
  OperationsOverview,
  RecordErrorInput,
  ResolveIncidentInput,
  RollbackEventRecord,
  RuleDeploymentHealth,
  RuleDryRunMatch,
  RuleDryRunResult,
  RuleRiskAssessment,
  RuleVersionComparison,
  RuleVersionFieldDiff,
  RuleVersionSummary,
  SaveBugNoteInput,
  SaveWorkflowRuleInput,
  WorkflowIncident,
  WorkflowRule,
  WorkflowRuleVersion,
};

export const workflowService = {
  // Operations & Exceptions
  getOperationsOverview: (): Promise<OperationsOverview> => WorkflowBindings.GetOperationsOverview(),
  detectExceptions: (): Promise<ExceptionDetectionResult> => WorkflowBindings.DetectExceptions(),
  listExceptions: (statusFilter = 'all', priorityFilter = 'all'): Promise<ExceptionCase[]> =>
    WorkflowBindings.ListExceptions(statusFilter, priorityFilter),
  transitionException: (caseId: string, newStatus: string, noteText: string): Promise<ExceptionCase> =>
    WorkflowBindings.TransitionException(caseId, newStatus, noteText),

  // Incidents
  listIncidents: (status = 'all'): Promise<WorkflowIncident[]> => WorkflowBindings.ListIncidents(status),
  assignIncident: (incidentId: string, assignee: string): Promise<void> =>
    WorkflowBindings.AssignIncident(incidentId, assignee),
  addIncidentNote: (incidentId: string, noteText: string): Promise<IncidentNote> =>
    WorkflowBindings.AddIncidentNote(incidentId, noteText),
  resolveIncident: (input: ResolveIncidentInput): Promise<void> => WorkflowBindings.ResolveIncident(input),

  // Background Jobs
  listBackgroundJobs: (): Promise<BackgroundJob[]> => WorkflowBindings.ListBackgroundJobs(),
  toggleBackgroundJob: (jobId: string, enabled: boolean): Promise<void> =>
    WorkflowBindings.ToggleBackgroundJob(jobId, enabled),
  listJobExecutions: (jobId = '', limit = 50): Promise<JobExecutionRecord[]> =>
    WorkflowBindings.ListJobExecutions(jobId, limit),
  runBackgroundJobNow: (jobId: string): Promise<JobExecutionRecord> => WorkflowBindings.RunBackgroundJobNow(jobId),
  overrideRecoveryGuard: (jobId: string, reason: string): Promise<void> =>
    WorkflowBindings.OverrideRecoveryGuard(jobId, reason),
  runDailyOperationsTick: (actor = 'operator'): Promise<void> => WorkflowBindings.RunDailyOperationsTick(actor),

  // Automation Rules, Promotion, Canary, Cost Limits
  listRules: (): Promise<WorkflowRule[]> => WorkflowBindings.ListRules(),
  getRuleVersion: (ruleId: string, versionId: string): Promise<WorkflowRuleVersion> =>
    WorkflowBindings.GetRuleVersion(ruleId, versionId),
  listRuleVersions: (ruleId: string): Promise<WorkflowRuleVersion[]> => WorkflowBindings.ListRuleVersions(ruleId),
  saveRule: (input: SaveWorkflowRuleInput): Promise<WorkflowRule> => WorkflowBindings.SaveRule(input),
  approveRuleVersion: (ruleId: string, versionId: string): Promise<void> =>
    WorkflowBindings.ApproveRuleVersion(ruleId, versionId),
  evaluateRuleRisk: (ruleId: string, versionId: string): Promise<RuleRiskAssessment> =>
    WorkflowBindings.EvaluateRuleRisk(ruleId, versionId),
  dryRunRuleVersion: (ruleId: string, versionId: string): Promise<RuleDryRunResult> =>
    WorkflowBindings.DryRunRuleVersion(ruleId, versionId),
  promoteRule: (
    ruleId: string,
    versionId: string,
    targetEnvId: string,
    adminOverride = false,
    overrideReason = '',
  ): Promise<RuleRiskAssessment> =>
    WorkflowBindings.PromoteRule(ruleId, versionId, targetEnvId, adminOverride, overrideReason),
  restoreRuleVersion: (
    ruleId: string,
    versionId: string,
    reason: string,
    adminOverride = false,
  ): Promise<RuleRiskAssessment> => WorkflowBindings.RestoreRuleVersion(ruleId, versionId, reason, adminOverride),
  configureCanary: (ruleId: string, versionId: string, samplePct: number): Promise<CanaryStatus> =>
    WorkflowBindings.ConfigureCanary(ruleId, versionId, samplePct),
  advanceCanaryStage: (ruleId: string, versionId: string): Promise<CanaryStatus> =>
    WorkflowBindings.AdvanceCanaryStage(ruleId, versionId),
  getCanaryMonitoring: (ruleId: string, versionId: string): Promise<CanaryMonitoringSummary> =>
    WorkflowBindings.GetCanaryMonitoring(ruleId, versionId),
  evaluateCanaryAndAutoRollback: (ruleId: string, versionId: string): Promise<AutoRollbackResult> =>
    WorkflowBindings.EvaluateCanaryAndAutoRollback(ruleId, versionId),
  getRollbackThresholds: (): Promise<CanaryRollbackThreshold> => WorkflowBindings.GetRollbackThresholds(),
  updateRollbackThresholds: (input: CanaryRollbackThreshold): Promise<CanaryRollbackThreshold> =>
    WorkflowBindings.UpdateRollbackThresholds(input),
  listOperationalAlerts: (includeAcknowledged = false): Promise<OperationalAlert[]> =>
    WorkflowBindings.ListOperationalAlerts(includeAcknowledged),
  acknowledgeOperationalAlert: (alertId: string): Promise<void> =>
    WorkflowBindings.AcknowledgeOperationalAlert(alertId),
  getDeploymentHealthOverview: (): Promise<DeploymentHealthOverview> =>
    WorkflowBindings.GetDeploymentHealthOverview(),
  compareRuleVersions: (
    ruleId: string,
    baseVersionId: string,
    targetVersionId: string,
  ): Promise<RuleVersionComparison> => WorkflowBindings.CompareRuleVersions(ruleId, baseVersionId, targetVersionId),
  rollbackRule: (ruleId: string, reason: string): Promise<void> => WorkflowBindings.RollbackRule(ruleId, reason),
  getCostLimits: (): Promise<AutomationCostLimit> => WorkflowBindings.GetCostLimits(),
  updateCostLimits: (input: AutomationCostLimit): Promise<AutomationCostLimit> =>
    WorkflowBindings.UpdateCostLimits(input),

  // Error Memory, Bug Notes & Diagnostics
  recordError: (input: RecordErrorInput): Promise<ErrorMemoryRecord> => WorkflowBindings.RecordError(input),
  listErrors: (status = 'all', severity = 'all'): Promise<ErrorMemoryRecord[]> =>
    WorkflowBindings.ListErrors(status, severity),
  updateErrorStatus: (id: string, status: string, aiSummary = ''): Promise<void> =>
    WorkflowBindings.UpdateErrorStatus(id, status, aiSummary),
  pruneResolvedErrors: (retentionDays: number): Promise<number> =>
    WorkflowBindings.PruneResolvedErrors(retentionDays),
  listBugNotes: (): Promise<BugNote[]> => WorkflowBindings.ListBugNotes(),
  saveBugNote: (input: SaveBugNoteInput): Promise<BugNote> => WorkflowBindings.SaveBugNote(input),
  deleteBugNote: (id: string): Promise<void> => WorkflowBindings.DeleteBugNote(id),
  exportDiagnosticsBundle: (outputPath = ''): Promise<DiagnosticsExportResult> =>
    WorkflowBindings.ExportDiagnosticsBundle(outputPath),
};
