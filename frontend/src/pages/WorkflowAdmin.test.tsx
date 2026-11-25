import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { AutomationCenterPage } from '@/pages/admin/AutomationCenterPage';
import { AutomationRulesPage } from '@/pages/admin/AutomationRulesPage';
import { ErrorCenterPage } from '@/pages/admin/ErrorCenterPage';
import { OperationsCenterPage } from '@/pages/admin/OperationsCenterPage';
import { SystemToolsPage } from '@/pages/admin/SystemToolsPage';

const mocks = vi.hoisted(() => ({
  getOperationsOverview: vi.fn().mockResolvedValue({
    workflowHealthScore: 95,
    systemStability: 98,
    openExceptions: 1,
    breachedSlaCount: 0,
    openIncidents: 1,
    activeSuppressions: 0,
    recentBursts: 0,
    restoreLocked: false,
    jobs: [],
    forecasts: [
      {
        forecastId: 'fct-1',
        sourceModule: 'workflow',
        eventType: 'SYSTEM_HEALTH',
        forecastTime: '2026-11-12 10:00:00',
        predictedProbability: 0.12,
        confidenceScore: 0.88,
        horizonMinutes: 30,
        detailsJson: '{}',
      },
    ],
    incidents: [
      {
        incidentId: 'inc-1',
        alertId: 'sig-1',
        incidentType: 'SYNC_FAILURE',
        severity: 'CRITICAL',
        status: 'OPEN',
        assignedTo: 'unassigned',
        createdAt: '2026-11-12 09:50:00',
        updatedAt: '2026-11-12 09:55:00',
        escalationCount: 1,
      },
    ],
  }),
  listExceptions: vi.fn().mockResolvedValue([
    {
      id: 'ex-1',
      exceptionType: 'OVERDUE_ETA',
      entityType: 'shipment',
      entityId: 'shp-1',
      status: 'OPEN',
      priority: 'HIGH',
      assignedTo: '',
      assignedAt: '',
      createdAt: '2026-11-12 09:00:00',
      updatedAt: '2026-11-12 09:00:00',
      resolvedBy: '',
      resolvedAt: '',
      slaDeadline: '2026-11-12 21:00:00',
      slaStatus: 'ON_TIME',
    },
  ]),
  detectExceptions: vi.fn().mockResolvedValue({
    createdCount: 1,
    updatedCount: 0,
    selfHealedCount: 0,
    escalatedCount: 0,
    activeOpenCount: 1,
  }),
  listBackgroundJobs: vi.fn().mockResolvedValue([
    {
      jobId: 'automation_cycle',
      jobName: 'Workflow automation cycle',
      jobType: 'AUTOMATION',
      scheduleType: 'DAILY',
      isEnabled: true,
      expectedDurationMs: 180000,
      maxRetries: 3,
      retryDelaySec: 120,
      lastStatus: 'COMPLETED',
      lastCompletedAt: '2026-11-12 10:00:01',
      nextExpectedAt: '2026-11-13 10:00:01',
      dependsOn: ['maintenance_cleanup'],
      guardStopped: false,
      recoveryAttempts24h: 0,
    },
    {
      jobId: 'risk_evaluation',
      jobName: 'Deployment risk signals rollup',
      jobType: 'RISK',
      scheduleType: 'DAILY',
      isEnabled: false,
      expectedDurationMs: 60000,
      maxRetries: 2,
      retryDelaySec: 300,
      lastStatus: 'TIMEOUT',
      lastErrorMessage: 'job execution exceeded timeout threshold',
      guardStopped: true,
      recoveryAttempts24h: 2,
    },
  ]),
  listJobExecutions: vi.fn().mockResolvedValue([
    {
      executionId: 'exec-2',
      jobId: 'risk_evaluation',
      startedAt: '2026-11-12 09:00:00',
      completedAt: '2026-11-12 11:00:00',
      status: 'TIMEOUT',
      recordsProcessed: 0,
      errorMessage: 'job execution exceeded timeout threshold',
      executionTimeMs: 7200000,
      retryCount: 2,
    },
  ]),
  overrideRecoveryGuard: vi.fn().mockResolvedValue(undefined),
  runBackgroundJobNow: vi.fn().mockResolvedValue({
    executionId: 'exec-1',
    jobId: 'automation_cycle',
    startedAt: '2026-11-12 10:00:00',
    completedAt: '2026-11-12 10:00:01',
    status: 'COMPLETED',
    recordsProcessed: 3,
    executionTimeMs: 42,
    retryCount: 0,
  }),
  listRules: vi.fn().mockResolvedValue([
    {
      ruleId: 'rule-1',
      ruleName: 'Critical Delay Escalator',
      ruleType: 'SLA_ESCALATION',
      conditionExpression: 'priority == HIGH',
      actionType: 'AUTO_ESCALATE',
      priority: 10,
      enabled: true,
      tenantId: 'tenant-default',
      latestVersionId: 'ver-2',
      latestVersionNum: 2,
      latestVersionSha256: 'deadbeef12345678',
      latestApprovalStatus: 'APPROVED',
      prodVersionId: 'ver-1',
      prodVersionNum: 1,
      currentEnv: 'env-test',
      canaryPercent: 25,
      createdAt: '2026-11-12',
      updatedAt: '2026-11-12',
    },
  ]),
  listRuleVersions: vi.fn().mockResolvedValue([
    {
      versionId: 'ver-2',
      ruleId: 'rule-1',
      tenantId: 'tenant-default',
      environmentId: 'env-dev',
      versionNumber: 2,
      ruleName: 'Critical Delay Escalator',
      ruleType: 'SLA_ESCALATION',
      conditionExpression: '{}',
      actionType: 'AUTO_ESCALATE',
      priority: 90,
      enabled: true,
      ruleDefinitionJson: '{}',
      definitionSha256: 'feedface12345678',
      createdBy: 'admin',
      createdAt: '2026-11-13',
      isActive: true,
      changeReason: 'v2 escalation',
      approvalStatus: 'APPROVED',
    },
    {
      versionId: 'ver-1',
      ruleId: 'rule-1',
      tenantId: 'tenant-default',
      environmentId: 'env-prod',
      versionNumber: 1,
      ruleName: 'Critical Delay Escalator',
      ruleType: 'SLA_ESCALATION',
      conditionExpression: 'priority == HIGH',
      actionType: 'AUTO_ASSIGN',
      priority: 10,
      enabled: true,
      ruleDefinitionJson: '{}',
      definitionSha256: 'deadbeef12345678',
      createdBy: 'admin',
      createdAt: '2026-11-12',
      isActive: false,
      changeReason: 'v1 initial',
      approvalStatus: 'APPROVED',
    },
  ]),
  getDeploymentHealthOverview: vi.fn().mockResolvedValue({
    totalRules: 1,
    activeProductionRules: 1,
    rulesAwaitingApproval: 0,
    activeCanaryDeployments: 1,
    completedCanaryDeployments: 0,
    failedOrRolledBackDeployments: 1,
    currentProductionRuleId: 'rule-1',
    currentProductionVersionId: 'ver-1',
    currentProductionVersionNum: 1,
    latestDeploymentTimestamp: '2026-11-12 10:00:00',
    overallHealth: 'WARNING',
    healthSummaryReason: '1 rule(s) with operational or performance warnings',
    windowMinutes: 60,
    thresholds: {
      id: 'default',
      minSampleSize: 10,
      maxFailureRate: 0.25,
      maxCostIncreasePct: 50,
      maxDurationIncreasePct: 100,
      monitoringWindowMinutes: 60,
      updatedAt: '2026-11-12',
    },
    ruleDeployments: [
      {
        ruleId: 'rule-1',
        ruleName: 'Critical Delay Escalator',
        ruleType: 'SLA_ESCALATION',
        actionType: 'AUTO_ESCALATE',
        enabled: true,
        currentEnv: 'env-test',
        prodVersionId: 'ver-1',
        prodVersionNum: 1,
        latestVersionId: 'ver-2',
        latestVersionNum: 2,
        latestApprovalStatus: 'APPROVED',
        latestDeployedAt: '2026-11-12 10:00:00',
        canaryStatus: 'ACTIVE',
        canaryVersionId: 'ver-2',
        canaryPercent: 25,
        canaryRolloutStage: 4,
        healthStatus: 'WARNING',
        healthReasons: ['Operational warning active'],
        attemptedCount: 12,
        successCount: 11,
        failureCount: 1,
        skippedCount: 0,
        failureRate: 0.083,
        costUnits: 12,
        avgDurationMs: 4.0,
        costIncreasePct: 10,
        durationIncreasePct: 5,
        openAlertCount: 1,
        acknowledgedAlertCount: 0,
        hasRecentRollback: true,
      },
    ],
    recentAlerts: [],
    recentRollbacks: [
      {
        id: 'rbk-1',
        ruleId: 'rule-1',
        rollbackType: 'AUTO_ROLLBACK_DISABLE',
        targetEntity: 'ver-1',
        performedBy: 'system',
        performedAt: '2026-11-12 09:50:00',
        detailsJson: '{}',
      },
    ],
  }),
  compareRuleVersions: vi.fn().mockResolvedValue({
    ruleId: 'rule-1',
    base: {
      versionId: 'ver-1',
      versionNumber: 1,
      ruleName: 'Critical Delay Escalator',
      ruleType: 'SLA_ESCALATION',
      actionType: 'AUTO_ASSIGN',
      conditionExpression: 'priority == HIGH',
      priority: 10,
      enabled: true,
      definitionSha256: 'deadbeef12345678',
      riskLevel: 'LOW',
      riskScore: 10,
      blocksProd: false,
      riskFactors: [],
      approvalStatus: 'APPROVED',
      deploymentStatus: 'DEPLOYED (env-prod)',
      createdAt: '2026-11-12',
      createdBy: 'admin',
    },
    target: {
      versionId: 'ver-2',
      versionNumber: 2,
      ruleName: 'Critical Delay Escalator',
      ruleType: 'SLA_ESCALATION',
      actionType: 'AUTO_ESCALATE',
      conditionExpression: '{}',
      priority: 90,
      enabled: true,
      definitionSha256: 'feedface12345678',
      riskLevel: 'CRITICAL',
      riskScore: 100,
      blocksProd: true,
      riskFactors: ['high-priority execution (>=80)'],
      approvalStatus: 'APPROVED',
      deploymentStatus: 'STAGED',
      createdAt: '2026-11-13',
      createdBy: 'admin',
    },
    changedFields: ['actionType', 'conditionExpression', 'priority'],
    fieldDiffs: [
      { field: 'actionType', baseValue: 'AUTO_ASSIGN', targetValue: 'AUTO_ESCALATE', changed: true },
      { field: 'priority', baseValue: '10', targetValue: '90', changed: true },
    ],
    importantChanges: ['Action changed from AUTO_ASSIGN to AUTO_ESCALATE'],
    riskScoreDelta: 90,
    riskLevelChanged: true,
    riskDifferenceSummary: 'Risk increased: LOW (10) -> CRITICAL (100) (+90)',
    dryRunImpact: {
      assumedCanaryPercent: 100,
      evaluatedOpenCases: 5,
      baseMatchingRecords: 2,
      targetMatchingRecords: 5,
      matchingRecordsDelta: 3,
      baseAffectedRecords: 2,
      targetAffectedRecords: 5,
      affectedRecordsDelta: 3,
      baseEstimatedCost: 2,
      targetEstimatedCost: 5,
      estimatedCostDelta: 3,
      baseSkippedRecords: 0,
      targetSkippedRecords: 0,
      skippedRecordsDelta: 0,
      baseWithinBudget: true,
      targetWithinBudget: true,
    },
    comparedAt: '2026-11-13 10:00:00',
  }),
  getCostLimits: vi.fn().mockResolvedValue({
    id: 'default',
    maxCostUnitsPerCycle: 500000,
    maxExecutionTimePerCycleMs: 120000,
    maxRecordsProcessedPerCycle: 500000,
    updatedAt: '2026-11-12',
  }),
  getRollbackThresholds: vi.fn().mockResolvedValue({
    id: 'default',
    minSampleSize: 10,
    maxFailureRate: 0.25,
    maxCostIncreasePct: 50,
    maxDurationIncreasePct: 100,
    monitoringWindowMinutes: 60,
    updatedAt: '2026-11-12',
  }),
  listOperationalAlerts: vi.fn().mockResolvedValue([
    {
      id: 'alt-1',
      alertType: 'CANARY_FAILURE_THRESHOLD_EXCEEDED',
      severity: 'CRITICAL',
      ruleId: 'rule-1',
      versionId: 'ver-1',
      message: 'Canary failure rate 40% exceeded max 25%',
      detailsJson: '{}',
      createdAt: '2026-11-12',
      acknowledged: false,
    },
  ]),
  acknowledgeOperationalAlert: vi.fn().mockResolvedValue(undefined),
  advanceCanaryStage: vi.fn().mockResolvedValue({
    id: 'can-1',
    ruleId: 'rule-1',
    versionId: 'ver-1',
    sampleSizePercentage: 50,
    rolloutStage: 5,
    deploymentStatus: 'ACTIVE',
    createdAt: '2026-11-12',
  }),
  getCanaryMonitoring: vi.fn().mockResolvedValue({
    ruleId: 'rule-1',
    versionId: 'ver-1',
    sampleSizePercentage: 25,
    rolloutStage: 4,
    deploymentStatus: 'ACTIVE',
    windowMinutes: 60,
    canary: {
      attemptedCount: 12,
      successCount: 12,
      failureCount: 0,
      skippedCount: 2,
      costUnits: 24,
      avgDurationMs: 4.5,
      failureRate: 0,
      executionCount: 12,
    },
    control: {
      attemptedCount: 36,
      successCount: 36,
      failureCount: 0,
      skippedCount: 0,
      costUnits: 72,
      avgDurationMs: 4.0,
      failureRate: 0,
      executionCount: 36,
    },
    costDeltaUnits: 0,
    costIncreasePct: 0,
    durationDeltaMs: 0.5,
    durationIncreasePct: 12.5,
    healthStatus: 'HEALTHY',
  }),
  listErrors: vi.fn().mockResolvedValue([
    {
      id: 'err-1',
      fingerprint: 'abc',
      occurrenceCount: 4,
      firstSeenAt: '2026-11-12',
      lastSeenAt: '2026-11-12',
      moduleName: 'expenses',
      commandName: 'SaveExpense',
      errorCode: 'VAL_001',
      errorMessage: 'Tax mismatch on expense line',
      severity: 'error',
      status: 'new',
    },
  ]),
  listBugNotes: vi.fn().mockResolvedValue([
    {
      id: 'bug-1',
      title: 'Check CSV quote escaping',
      description: 'Verify comma handling',
      status: 'OPEN',
      context: '',
      createdAt: 1,
      updatedAt: 1,
    },
  ]),
  exportDiagnosticsBundle: vi.fn().mockResolvedValue({
    filePath: 'C:\\Exports\\diag.zip',
    sizeBytes: 4096,
    generatedAt: '2026-11-12T10:00:00Z',
    includedSections: ['manifest.json', 'error_memory.json'],
  }),
}));

vi.mock('@/services/workflow-service', () => ({
  workflowService: {
    getOperationsOverview: mocks.getOperationsOverview,
    listExceptions: mocks.listExceptions,
    detectExceptions: mocks.detectExceptions,
    transitionException: vi.fn(),
    listIncidents: vi.fn().mockResolvedValue([]),
    assignIncident: vi.fn(),
    addIncidentNote: vi.fn(),
    resolveIncident: vi.fn(),
    listBackgroundJobs: mocks.listBackgroundJobs,
    toggleBackgroundJob: vi.fn(),
    listJobExecutions: mocks.listJobExecutions,
    runBackgroundJobNow: mocks.runBackgroundJobNow,
    overrideRecoveryGuard: mocks.overrideRecoveryGuard,
    runDailyOperationsTick: vi.fn().mockResolvedValue(undefined),
    listRules: mocks.listRules,
    getRuleVersion: vi.fn(),
    listRuleVersions: mocks.listRuleVersions,
    getDeploymentHealthOverview: mocks.getDeploymentHealthOverview,
    compareRuleVersions: mocks.compareRuleVersions,
    saveRule: vi.fn(),
    approveRuleVersion: vi.fn(),
    evaluateRuleRisk: vi.fn().mockResolvedValue({
      ruleId: 'rule-1',
      versionId: 'ver-1',
      definitionSha256: 'deadbeef12345678',
      score: 70,
      level: 'HIGH',
      blocksProd: true,
      factors: ['mutating/high-impact action: AUTO_ESCALATE'],
    }),
    dryRunRuleVersion: vi.fn().mockResolvedValue({
      ruleId: 'rule-1',
      versionId: 'ver-1',
      definitionSha256: 'deadbeef12345678',
      actionType: 'AUTO_ESCALATE',
      ruleEnabled: true,
      canaryPercent: 25,
      totalOpenCases: 4,
      eligibleCount: 2,
      canaryRoutedCount: 1,
      skippedIdempotentCount: 0,
      estimatedCostUnits: 2.5,
      costBudgetCap: 500000,
      recordsBudgetCap: 500000,
      withinBudget: true,
      risk: {
        ruleId: 'rule-1',
        versionId: 'ver-1',
        score: 70,
        level: 'HIGH',
        blocksProd: true,
        factors: [],
      },
      samples: [],
    }),
    promoteRule: vi.fn(),
    restoreRuleVersion: vi.fn(),
    configureCanary: vi.fn(),
    advanceCanaryStage: mocks.advanceCanaryStage,
    getCanaryMonitoring: mocks.getCanaryMonitoring,
    evaluateCanaryAndAutoRollback: vi.fn(),
    getRollbackThresholds: mocks.getRollbackThresholds,
    updateRollbackThresholds: vi.fn(),
    listOperationalAlerts: mocks.listOperationalAlerts,
    acknowledgeOperationalAlert: mocks.acknowledgeOperationalAlert,
    rollbackRule: vi.fn(),
    getCostLimits: mocks.getCostLimits,
    updateCostLimits: vi.fn(),
    recordError: vi.fn(),
    listErrors: mocks.listErrors,
    updateErrorStatus: vi.fn(),
    pruneResolvedErrors: vi.fn().mockResolvedValue(1),
    listBugNotes: mocks.listBugNotes,
    saveBugNote: vi.fn(),
    deleteBugNote: vi.fn(),
    exportDiagnosticsBundle: mocks.exportDiagnosticsBundle,
  },
}));

describe('Stage 10 Workflow & Diagnostics Admin Pages', () => {
  it('renders Operations Center exceptions, incidents, and forecasts tabs', async () => {
    render(
      <MemoryRouter>
        <OperationsCenterPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('OVERDUE_ETA')).toBeTruthy();
    });

    const incidentsTab = screen.getByRole('tab', { name: /Incidents/i });
    fireEvent.mouseDown(incidentsTab, { button: 0 });
    fireEvent.click(incidentsTab);
    expect(screen.getByText('SYNC_FAILURE')).toBeTruthy();
  });

  it('renders Automation Center background jobs, guard override, and triggers Run Now', async () => {
    render(
      <MemoryRouter>
        <AutomationCenterPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Workflow automation cycle')).toBeTruthy();
      expect(screen.getByText('Guard Stopped')).toBeTruthy();
    });
    const runButtons = screen.getAllByRole('button', { name: /Run Now/i });
    expect(runButtons.length).toBeGreaterThan(0);
    fireEvent.click(runButtons[0]!);
    await waitFor(() => {
      expect(mocks.runBackgroundJobNow).toHaveBeenCalledWith('automation_cycle');
    });

    const reasonInput = screen.getByPlaceholderText(/Override reason/i);
    fireEvent.change(reasonInput, { target: { value: 'Cleared stuck lock manually' } });
    fireEvent.click(screen.getByRole('button', { name: /Override/i }));
    await waitFor(() => {
      expect(mocks.overrideRecoveryGuard).toHaveBeenCalledWith('risk_evaluation', 'Cleared stuck lock manually');
    });
  });

  it('renders Automation Rules, System Tools ZIP export, and Error Center bug notes', async () => {
    render(
      <MemoryRouter>
        <AutomationRulesPage />
        <SystemToolsPage />
        <ErrorCenterPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getAllByText('Critical Delay Escalator').length).toBeGreaterThan(0);
      expect(screen.getAllByText('APPROVED').length).toBeGreaterThan(0);
      expect(screen.getByText('Prod v1')).toBeTruthy();
      expect(screen.getByText('sha:deadbeef')).toBeTruthy();
      expect(screen.getByText('Tax mismatch on expense line')).toBeTruthy();
      expect(screen.getByText(/Deployment Health Overview/i)).toBeTruthy();
      expect(screen.getByText(/Production Deployment & Health/i)).toBeTruthy();
      expect(screen.getByText(/Recent Rollback Activity/i)).toBeTruthy();
      expect(screen.getByText('AUTO_ROLLBACK_DISABLE')).toBeTruthy();
    });

    fireEvent.click(screen.getByRole('button', { name: /^Risk$/i }));
    await waitFor(() => {
      expect(screen.getByText(/Blocks PROD without override/i)).toBeTruthy();
      expect(screen.getByPlaceholderText(/Override reason/i)).toBeTruthy();
    });

    fireEvent.click(screen.getByRole('button', { name: /Dry Run/i }));
    await waitFor(() => {
      expect(screen.getByText(/Dry Run Preview/i)).toBeTruthy();
      expect(screen.getByText(/Within Budget/i)).toBeTruthy();
    });

    await waitFor(() => {
      expect(screen.getByText(/Automatic Canary Rollback Thresholds/i)).toBeTruthy();
      expect(screen.getByText(/CANARY_FAILURE_THRESHOLD_EXCEEDED/i)).toBeTruthy();
    });

    fireEvent.click(screen.getByRole('button', { name: /Advance Stage/i }));
    await waitFor(() => {
      expect(mocks.advanceCanaryStage).toHaveBeenCalledWith('rule-1', 'ver-2');
    });

    fireEvent.click(screen.getByRole('button', { name: /Compare Versions/i }));
    await waitFor(() => {
      expect(mocks.compareRuleVersions).toHaveBeenCalledWith('rule-1', 'ver-1', 'ver-2');
      expect(screen.getByText(/Dry-Run Impact Comparison/i)).toBeTruthy();
      expect(screen.getByText(/Action changed from AUTO_ASSIGN to AUTO_ESCALATE/i)).toBeTruthy();
    });

    fireEvent.click(screen.getByRole('button', { name: /^Monitor$/i }));
    await waitFor(() => {
      expect(screen.getByText(/Canary vs Control/i)).toBeTruthy();
      expect(screen.getByText('HEALTHY')).toBeTruthy();
    });

    const ackButtons = screen.getAllByRole('button', { name: /Acknowledge/i });
    fireEvent.click(ackButtons[0]!);
    await waitFor(() => {
      expect(mocks.acknowledgeOperationalAlert).toHaveBeenCalledWith('alt-1');
    });

    fireEvent.click(screen.getByRole('button', { name: /Export Diagnostics Bundle/i }));
    await waitFor(() => {
      expect(mocks.exportDiagnosticsBundle).toHaveBeenCalled();
    });

    const bugsTab = screen.getByRole('tab', { name: /Bug Notes/i });
    fireEvent.mouseDown(bugsTab, { button: 0 });
    fireEvent.click(bugsTab);
    expect(screen.getByText('Check CSV quote escaping')).toBeTruthy();
  });
});
