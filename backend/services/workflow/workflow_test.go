package workflow_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/workflow"
)

func setupWorkflowEnv(t *testing.T) *bootstrap.Runtime {
	t.Helper()
	t.Setenv("IMPORT_MANAGER_DB_KEY", "workflow-test-key-0123456789abcdef01")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	dir := filepath.Join(t.TempDir(), "appdata")
	rt, err := bootstrap.Initialize([]string{"--recovery-mode"}, dir)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	t.Cleanup(func() { _ = rt.Shutdown() })

	if _, err := rt.Security.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup failed: %v", err)
	}
	return rt
}

func TestExceptionDetectionSLABreachSelfHealAndTerminalGuard(t *testing.T) {
	rt := setupWorkflowEnv(t)
	db := rt.DB.DB()

	pastDate := time.Now().UTC().AddDate(0, 0, -10).Format("2006-01-02")
	_, err := db.Exec(
		`INSERT INTO suppliers (id, supplier_name, short_name, country, email, is_active) VALUES ('sup-wf-1', 'WF Supplier', 'WFS', 'IN', 'wf@supplier.example', 1)`,
	)
	if err != nil {
		t.Fatalf("insert supplier: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, shipment_mode, shipment_type, bl_awb_number, eta, status)
		 VALUES ('shp-wf-1', 'sup-wf-1', 'INV-WF-1', ?, 'Electronics', 1000, 'USD', 'CIF', 'SEA', 'FCL', 'BL-1', ?, 'customs-clearance')`,
		pastDate, pastDate,
	)
	if err != nil {
		t.Fatalf("insert shipment: %v", err)
	}

	res, err := rt.Workflow.DetectExceptions()
	if err != nil {
		t.Fatalf("DetectExceptions: %v", err)
	}
	if res.CreatedCount < 3 {
		t.Fatalf("expected OVERDUE_ETA, MISSING_BOE, MISSING_EXPENSE created, got %+v", res)
	}

	cases, err := rt.Workflow.ListExceptions("", "")
	if err != nil || len(cases) < 3 {
		t.Fatalf("ListExceptions failed: len=%d err=%v", len(cases), err)
	}

	// Simulate SLA breach and stale IN_PROGRESS (>14d) for self-heal
	targetID := cases[0].ID
	staleTime := time.Now().UTC().Add(-16 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	pastSLA := time.Now().UTC().Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	if _, err := db.Exec(`UPDATE exception_cases SET status = 'IN_PROGRESS', updated_at = ?, sla_deadline = ? WHERE id = ?`, staleTime, pastSLA, targetID); err != nil {
		t.Fatalf("force stale SLA: %v", err)
	}

	res2, err := rt.Workflow.DetectExceptions()
	if err != nil {
		t.Fatalf("second DetectExceptions: %v", err)
	}
	if res2.SelfHealedCount < 1 || res2.EscalatedCount < 1 {
		t.Fatalf("expected self-heal and SLA escalation, got %+v", res2)
	}

	// Resolve and test terminal state guard
	if _, err := rt.Workflow.TransitionException(targetID, "RESOLVED", "Verified documentation and cleared shipment"); err != nil {
		t.Fatalf("resolve exception: %v", err)
	}
	if _, err := rt.Workflow.TransitionException(targetID, "OPEN", "Reopen without override"); err == nil {
		t.Fatalf("expected terminal guard error when reopening RESOLVED exception")
	}
}

func TestAutomationRulePromotionRiskCanaryAndCostLimits(t *testing.T) {
	rt := setupWorkflowEnv(t)

	rule, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Critical Delay Escalator",
		RuleType:            "SLA_ESCALATION",
		ConditionExpression: "shipment.status == 'in-transit'",
		ActionType:          "ESCALATE",
		Priority:            10,
		Enabled:             true,
		ChangeReason:        "Initial v1",
	})
	if err != nil {
		t.Fatalf("SaveRule v1: %v", err)
	}

	// Cannot jump directly to env-prod without sequential env-dev -> env-test and approval
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, rule.LatestVersionID, "env-prod", false); err == nil {
		t.Fatalf("expected direct env-prod promotion without approval/sequence to fail")
	}
	if err := rt.Workflow.ApproveRuleVersion(rule.RuleID, rule.LatestVersionID); err != nil {
		t.Fatalf("ApproveRuleVersion: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, rule.LatestVersionID, "env-dev", false); err != nil {
		t.Fatalf("promote env-dev: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, rule.LatestVersionID, "env-test", false); err != nil {
		t.Fatalf("promote env-test: %v", err)
	}

	// High blast-radius wildcard rule without canary should trigger >= HIGH risk block on PROD unless override
	v2, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleID:              rule.RuleID,
		RuleName:            "Wildcard Auto Resolve",
		RuleType:            "AUTO_CLOSE",
		ConditionExpression: "*",
		ActionType:          "AUTO_RESOLVE",
		Priority:            150,
		Enabled:             true,
		ChangeReason:        "Risky v2",
	})
	if err != nil {
		t.Fatalf("SaveRule v2: %v", err)
	}
	_ = rt.Workflow.ApproveRuleVersion(rule.RuleID, v2.LatestVersionID)
	_, _ = rt.Workflow.PromoteRule(rule.RuleID, v2.LatestVersionID, "env-dev", false)
	_, _ = rt.Workflow.PromoteRule(rule.RuleID, v2.LatestVersionID, "env-test", false)
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v2.LatestVersionID, "env-prod", false); err == nil {
		t.Fatalf("expected high-risk rule v2 to block PROD promotion without override")
	}

	canary, err := rt.Workflow.ConfigureCanary(rule.RuleID, v2.LatestVersionID, 25)
	if err != nil || canary.SampleSizePercentage != 25 {
		t.Fatalf("ConfigureCanary failed: %+v err=%v", canary, err)
	}

	// FNV-1a determinism check
	first := workflow.ShouldRouteCanary(rule.RuleID, "shp-deterministic-1", 25)
	second := workflow.ShouldRouteCanary(rule.RuleID, "shp-deterministic-1", 25)
	if first != second {
		t.Fatalf("FNV-1a canary routing was non-deterministic")
	}

	limits, err := rt.Workflow.UpdateCostLimits(models.AutomationCostLimit{
		MaxCostUnitsPerCycle:       500,
		MaxExecutionTimePerCycleMs: 45000,
		MaxRecordsProcessedCycle:   250,
	})
	if err != nil || limits.MaxRecordsProcessedCycle != 250 {
		t.Fatalf("UpdateCostLimits failed: %+v err=%v", limits, err)
	}

	if err := rt.Workflow.RollbackRule(rule.RuleID, "Emergency test rollback"); err != nil {
		t.Fatalf("RollbackRule failed: %v", err)
	}
}

func TestBackgroundJobsRecoveryGuardBurstSuppressionForecastAndResolveGate(t *testing.T) {
	rt := setupWorkflowEnv(t)
	db := rt.DB.DB()

	jobs, err := rt.Workflow.ListBackgroundJobs()
	if err != nil || len(jobs) != 6 {
		t.Fatalf("expected 6 seeded jobs, got %d err=%v", len(jobs), err)
	}
	for _, j := range jobs {
		rec, err := rt.Workflow.RunBackgroundJobNow(j.JobID)
		if err != nil || rec.Status != "COMPLETED" {
			t.Fatalf("job %s failed: rec=%+v err=%v", j.JobID, rec, err)
		}
	}

	// Inject a stuck RUNNING job > 3 hours ago and exhaust max_retries to verify RECOVERY_GUARD_STOP circuit breaker
	stuckStart := time.Now().UTC().Add(-4 * time.Hour).Format("2006-01-02 15:04:05")
	_, _ = db.Exec(
		`INSERT INTO workflow_job_execution_log (execution_id, job_id, started_at, status, records_processed, retry_count)
		 VALUES ('exec-stuck-1', 'risk_evaluation', ?, 'RUNNING', 0, 0)`, stuckStart,
	)
	_, _ = db.Exec(
		`INSERT INTO workflow_job_recovery_log (recovery_id, job_id, recovery_time, result, attempt_count)
		 VALUES ('rec-1', 'risk_evaluation', datetime('now'), 'RECOVERED', 1),
		        ('rec-2', 'risk_evaluation', datetime('now'), 'RECOVERED', 2)`,
	)
	if err := rt.Workflow.RunDailyOperationsTick("test"); err != nil {
		t.Fatalf("RunDailyOperationsTick: %v", err)
	}
	var guardStops int
	_ = db.QueryRow(`SELECT COUNT(*) FROM workflow_job_recovery_log WHERE job_id = 'risk_evaluation' AND result = 'RECOVERY_GUARD_STOP'`).Scan(&guardStops)
	if guardStops == 0 {
		t.Fatalf("expected RECOVERY_GUARD_STOP circuit breaker entry")
	}

	// Inject burst of 3 signals to test burst log + suppression + incident promotion
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, _ = db.Exec(
		`INSERT INTO workflow_alert_signal_log (id, created_at, signal_type, severity, entity_id, message, details_json) VALUES
		 ('sig-b-1', ?, 'DB_TIMEOUT', 'HIGH', 'boe_engine', 'timeout 1', '{}'),
		 ('sig-b-2', ?, 'DB_TIMEOUT', 'HIGH', 'boe_engine', 'timeout 2', '{}'),
		 ('sig-b-3', ?, 'DB_TIMEOUT', 'HIGH', 'boe_engine', 'timeout 3', '{}')`,
		nowStr, nowStr, nowStr,
	)
	// Also inject 1 unrelated unsuppressed signal so an incident is created
	_, _ = db.Exec(
		`INSERT INTO workflow_alert_signal_log (id, created_at, signal_type, severity, entity_id, message, details_json)
		 VALUES ('sig-open-1', ?, 'SYNC_FAILURE', 'CRITICAL', 'gdrive', 'Token expired', '{}')`, nowStr,
	)
	if err := rt.Workflow.RunDailyOperationsTick("test"); err != nil {
		t.Fatalf("second tick: %v", err)
	}

	ov, err := rt.Workflow.GetOperationsOverview()
	if err != nil {
		t.Fatalf("GetOperationsOverview: %v", err)
	}
	if ov.RecentBursts == 0 || ov.ActiveSuppressions == 0 || len(ov.Forecasts) == 0 || len(ov.Incidents) == 0 {
		t.Fatalf("expected bursts/suppressions/forecasts/incidents, got %+v", ov)
	}

	incID := ov.Incidents[0].IncidentID
	if err := rt.Workflow.AssignIncident(incID, "ops-lead"); err != nil {
		t.Fatalf("AssignIncident: %v", err)
	}

	// Gate 1: short resolution summary (<50 chars) must fail
	if err := rt.Workflow.ResolveIncident(models.ResolveIncidentInput{IncidentID: incID, ResolutionNotes: "too short", SkipQuietWindowCheck: true}); err == nil {
		t.Fatalf("expected ResolveIncident to reject <50 char summary")
	}
	longSummary := strings.Repeat("Root cause verified and permanent corrective action deployed. ", 2)
	// Gate 2: missing investigation note >= 10 chars must fail
	if err := rt.Workflow.ResolveIncident(models.ResolveIncidentInput{IncidentID: incID, ResolutionNotes: longSummary, SkipQuietWindowCheck: true}); err == nil {
		t.Fatalf("expected ResolveIncident to reject missing >=10 char investigation note")
	}
	if _, err := rt.Workflow.AddIncidentNote(incID, "Inspected logs and confirmed transient lock resolved"); err != nil {
		t.Fatalf("AddIncidentNote: %v", err)
	}
	// Gate 3: active quiet window (<10m signals exist) must fail when SkipQuietWindowCheck=false
	if err := rt.Workflow.ResolveIncident(models.ResolveIncidentInput{IncidentID: incID, ResolutionNotes: longSummary, SkipQuietWindowCheck: false}); err == nil {
		t.Fatalf("expected ResolveIncident to block inside 10m quiet window")
	}
	// Allowed when SkipQuietWindowCheck=true and note + summary pass
	if err := rt.Workflow.ResolveIncident(models.ResolveIncidentInput{IncidentID: incID, ResolutionNotes: longSummary, SkipQuietWindowCheck: true}); err != nil {
		t.Fatalf("ResolveIncident should succeed once gates pass: %v", err)
	}
}

func TestErrorMemoryBugNotesAndDiagnosticsZIP(t *testing.T) {
	rt := setupWorkflowEnv(t)

	in := models.RecordErrorInput{
		ModuleName:   "expenses",
		CommandName:  "SaveExpense",
		ErrorCode:    "VAL_001",
		ErrorMessage: "TDS amount exceeds taxable value",
		Severity:     "error",
		Recoverable:  true,
	}
	e1, err := rt.Workflow.RecordError(in)
	if err != nil || e1.OccurrenceCount != 1 {
		t.Fatalf("RecordError 1: %+v err=%v", e1, err)
	}
	e2, err := rt.Workflow.RecordError(in)
	if err != nil || e2.OccurrenceCount != 2 || e1.Fingerprint != e2.Fingerprint {
		t.Fatalf("expected deduplication count=2, got %+v err=%v", e2, err)
	}

	if err := rt.Workflow.UpdateErrorStatus(e2.ID, "resolved", "Fixed validation guard"); err != nil {
		t.Fatalf("UpdateErrorStatus: %v", err)
	}
	pruned, err := rt.Workflow.PruneResolvedErrors(0)
	if err != nil || pruned != 1 {
		t.Fatalf("PruneResolvedErrors(0): pruned=%d err=%v", pruned, err)
	}

	bn, err := rt.Workflow.SaveBugNote(models.SaveBugNoteInput{
		Title:       "UI alignment glitch on Expenses",
		Description: "Check column width",
		Status:      "OPEN",
	})
	if err != nil || bn.ID == "" {
		t.Fatalf("SaveBugNote: %+v err=%v", bn, err)
	}

	zipPath := filepath.Join(t.TempDir(), "diag.zip")
	exp, err := rt.Workflow.ExportDiagnosticsBundle(zipPath)
	if err != nil || exp.SizeBytes == 0 {
		t.Fatalf("ExportDiagnosticsBundle: %+v err=%v", exp, err)
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("missing zip file: %v", err)
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open diagnostics zip: %v", err)
	}
	defer zr.Close()
	if len(zr.File) < 5 {
		t.Fatalf("expected at least 5 files in diagnostics zip, got %d", len(zr.File))
	}

	if err := rt.Workflow.DeleteBugNote(bn.ID); err != nil {
		t.Fatalf("DeleteBugNote: %v", err)
	}
}
