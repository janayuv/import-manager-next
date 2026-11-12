import * as WorkflowBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/workflow/service.js';
import type {
  AutomationCostLimit,
  BackgroundJob,
  BugNote,
  CanaryStatus,
  DiagnosticsExportResult,
  ErrorMemoryRecord,
  ExceptionCase,
  ExceptionDetectionResult,
  FailureForecast,
  IncidentNote,
  JobExecutionRecord,
  OperationsOverview,
  RecordErrorInput,
  ResolveIncidentInput,
  RuleRiskAssessment,
  SaveBugNoteInput,
  SaveWorkflowRuleInput,
  WorkflowIncident,
  WorkflowRule,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  AutomationCostLimit,
  BackgroundJob,
  BugNote,
  CanaryStatus,
  DiagnosticsExportResult,
  ErrorMemoryRecord,
  ExceptionCase,
  ExceptionDetectionResult,
  FailureForecast,
  IncidentNote,
  JobExecutionRecord,
  OperationsOverview,
  RecordErrorInput,
  ResolveIncidentInput,
  RuleRiskAssessment,
  SaveBugNoteInput,
  SaveWorkflowRuleInput,
  WorkflowIncident,
  WorkflowRule,
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
  runDailyOperationsTick: (actor = 'operator'): Promise<void> => WorkflowBindings.RunDailyOperationsTick(actor),

  // Automation Rules, Promotion, Canary, Cost Limits
  listRules: (): Promise<WorkflowRule[]> => WorkflowBindings.ListRules(),
  saveRule: (input: SaveWorkflowRuleInput): Promise<WorkflowRule> => WorkflowBindings.SaveRule(input),
  approveRuleVersion: (ruleId: string, versionId: string): Promise<void> =>
    WorkflowBindings.ApproveRuleVersion(ruleId, versionId),
  evaluateRuleRisk: (ruleId: string, versionId: string): Promise<RuleRiskAssessment> =>
    WorkflowBindings.EvaluateRuleRisk(ruleId, versionId),
  promoteRule: (
    ruleId: string,
    versionId: string,
    targetEnvId: string,
    adminOverride = false,
  ): Promise<RuleRiskAssessment> => WorkflowBindings.PromoteRule(ruleId, versionId, targetEnvId, adminOverride),
  configureCanary: (ruleId: string, versionId: string, samplePct: number): Promise<CanaryStatus> =>
    WorkflowBindings.ConfigureCanary(ruleId, versionId, samplePct),
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
