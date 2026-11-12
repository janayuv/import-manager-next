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
    },
  ]),
  listJobExecutions: vi.fn().mockResolvedValue([]),
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
      latestVersionId: 'ver-1',
      latestVersionNum: 1,
      canaryPercent: 25,
      createdAt: '2026-11-12',
      updatedAt: '2026-11-12',
    },
  ]),
  getCostLimits: vi.fn().mockResolvedValue({
    id: 'default',
    maxCostUnitsPerCycle: 500000,
    maxExecutionTimePerCycleMs: 120000,
    maxRecordsProcessedPerCycle: 500000,
    updatedAt: '2026-11-12',
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
    runDailyOperationsTick: vi.fn().mockResolvedValue(undefined),
    listRules: mocks.listRules,
    saveRule: vi.fn(),
    approveRuleVersion: vi.fn(),
    evaluateRuleRisk: vi.fn(),
    promoteRule: vi.fn(),
    configureCanary: vi.fn(),
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

  it('renders Automation Center background jobs and triggers Run Now', async () => {
    render(
      <MemoryRouter>
        <AutomationCenterPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Workflow automation cycle')).toBeTruthy();
    });
    fireEvent.click(screen.getByRole('button', { name: /Run Now/i }));
    await waitFor(() => {
      expect(mocks.runBackgroundJobNow).toHaveBeenCalledWith('automation_cycle');
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
      expect(screen.getByText('Critical Delay Escalator')).toBeTruthy();
      expect(screen.getByText('Tax mismatch on expense line')).toBeTruthy();
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
