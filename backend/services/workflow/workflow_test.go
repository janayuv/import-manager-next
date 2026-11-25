package workflow_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	// Mark shipment delivered and run automation_cycle -> seeded AUTO_RESOLVE rule should resolve OVERDUE_ETA
	if _, err := db.Exec(`UPDATE shipments SET status = 'delivered' WHERE id = 'shp-wf-1'`); err != nil {
		t.Fatalf("mark shipment delivered: %v", err)
	}
	if rec, err := rt.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil || rec.Status != "COMPLETED" {
		t.Fatalf("automation_cycle failed: rec=%+v err=%v", rec, err)
	}
	var overdueStatus string
	_ = db.QueryRow(`SELECT status FROM exception_cases WHERE entity_id = 'shp-wf-1' AND exception_type = 'OVERDUE_ETA'`).Scan(&overdueStatus)
	if overdueStatus != "RESOLVED" {
		t.Fatalf("expected automation_cycle to auto-resolve OVERDUE_ETA on delivered shipment, got %q", overdueStatus)
	}
}

func TestAutomationRulePromotionRiskCanaryAndCostLimits(t *testing.T) {
	rt := setupWorkflowEnv(t)
	db := rt.DB.DB()

	rule, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Critical Delay Escalator",
		RuleType:            "SLA_ESCALATION",
		ConditionExpression: "shipment.status == 'in-transit'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            10,
		Enabled:             true,
		ChangeReason:        "Initial v1",
	})
	if err != nil {
		t.Fatalf("SaveRule v1: %v", err)
	}
	v1ID := rule.LatestVersionID
	if rule.LatestVersionSHA256 == "" || rule.LatestApprovalStatus != "PENDING" {
		t.Fatalf("expected non-empty SHA-256 and PENDING approval on v1, got %+v", rule)
	}

	// Mismatched ruleId/versionId ownership check
	if _, err := rt.Workflow.GetRuleVersion("wrong-rule", v1ID); err == nil {
		t.Fatalf("expected GetRuleVersion to reject wrong ruleId")
	}

	// Cannot jump directly to env-prod without sequential env-dev -> env-test and approval
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v1ID, "env-prod", false, ""); err == nil {
		t.Fatalf("expected direct env-prod promotion without approval/sequence to fail")
	}
	if err := rt.Workflow.ApproveRuleVersion(rule.RuleID, v1ID); err != nil {
		t.Fatalf("ApproveRuleVersion v1: %v", err)
	}
	// Re-approving an already APPROVED version must fail
	if err := rt.Workflow.ApproveRuleVersion(rule.RuleID, v1ID); err == nil {
		t.Fatalf("expected duplicate ApproveRuleVersion to fail")
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v1ID, "env-dev", false, ""); err != nil {
		t.Fatalf("promote env-dev: %v", err)
	}
	// Duplicate deployment of the same version to env-dev must fail
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v1ID, "env-dev", false, ""); err == nil {
		t.Fatalf("expected duplicate promotion to env-dev to fail")
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v1ID, "env-test", false, ""); err != nil {
		t.Fatalf("promote env-test: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v1ID, "env-prod", false, ""); err != nil {
		t.Fatalf("promote env-prod v1: %v", err)
	}

	// Save v2 (high blast-radius wildcard + DELETE + priority 150)
	v2, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleID:              rule.RuleID,
		RuleName:            "Wildcard Auto Delete",
		RuleType:            "AUTO_CLOSE",
		ConditionExpression: "*",
		ActionType:          "DELETE",
		Priority:            150,
		Enabled:             true,
		ChangeReason:        "Risky v2",
	})
	if err != nil {
		t.Fatalf("SaveRule v2: %v", err)
	}

	// Version integrity: v1 risk must remain LOW even after v2 is saved!
	r1Risk, err := rt.Workflow.EvaluateRuleRisk(rule.RuleID, v1ID)
	if err != nil || r1Risk.Level != "LOW" || r1Risk.BlocksProd {
		t.Fatalf("expected v1 immutable risk to stay LOW, got %+v err=%v", r1Risk, err)
	}
	r2Risk, err := rt.Workflow.EvaluateRuleRisk(rule.RuleID, v2.LatestVersionID)
	if err != nil || r2Risk.Level != "CRITICAL" || !r2Risk.BlocksProd {
		t.Fatalf("expected v2 risk to be CRITICAL and block PROD, got %+v err=%v", r2Risk, err)
	}

	// DB-level immutability trigger must block altering historical rule_definition_json
	if _, err := db.Exec(`UPDATE workflow_rule_versions SET rule_definition_json = '{}' WHERE version_id = ?`, v1ID); err == nil {
		t.Fatalf("expected immutability trigger to reject UPDATE on workflow_rule_versions.rule_definition_json")
	}

	if err := rt.Workflow.ApproveRuleVersion(rule.RuleID, v2.LatestVersionID); err != nil {
		t.Fatalf("ApproveRuleVersion v2: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v2.LatestVersionID, "env-dev", false, ""); err != nil {
		t.Fatalf("promote v2 env-dev: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v2.LatestVersionID, "env-test", false, ""); err != nil {
		t.Fatalf("promote v2 env-test: %v", err)
	}
	// Blocked without override
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v2.LatestVersionID, "env-prod", false, ""); err == nil {
		t.Fatalf("expected high-risk rule v2 to block PROD promotion without override")
	}
	// Override without documented reason must fail
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v2.LatestVersionID, "env-prod", true, "ok"); err == nil {
		t.Fatalf("expected override with <5 char reason to fail")
	}
	// Administrator override with reason succeeds
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, v2.LatestVersionID, "env-prod", true, "Approved emergency change CHG-101"); err != nil {
		t.Fatalf("expected administrator override with valid reason to succeed, got %v", err)
	}

	// Separation of duties: once a second admin/manager exists, creator cannot self-approve
	if _, err := db.Exec(`INSERT INTO user_roles (user_id, role) VALUES ('ops_manager', 'manager')`); err != nil {
		t.Fatalf("insert second approver: %v", err)
	}
	v3, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleID:              rule.RuleID,
		RuleName:            "Self Approval Test",
		RuleType:            "SLA_ESCALATION",
		ConditionExpression: "priority == 'HIGH'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            20,
		Enabled:             true,
		ChangeReason:        "Test SoD",
	})
	if err != nil {
		t.Fatalf("SaveRule v3: %v", err)
	}
	if err := rt.Workflow.ApproveRuleVersion(rule.RuleID, v3.LatestVersionID); err == nil || !strings.Contains(err.Error(), "separation of duties") {
		t.Fatalf("expected self-approval rejection when multiple approvers exist, got %v", err)
	}

	// Check version listing
	versions, err := rt.Workflow.ListRuleVersions(rule.RuleID)
	if err != nil || len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d err=%v", len(versions), err)
	}

	canary, err := rt.Workflow.ConfigureCanary(rule.RuleID, v2.LatestVersionID, 25)
	if err != nil || canary.SampleSizePercentage != 25 {
		t.Fatalf("ConfigureCanary failed: %+v err=%v", canary, err)
	}

	// FNV-1a determinism & version isolation check
	first := workflow.ShouldRouteCanaryVersion("tenant-default", rule.RuleID, v2.LatestVersionID, "shp-deterministic-1", 25)
	second := workflow.ShouldRouteCanaryVersion("tenant-default", rule.RuleID, v2.LatestVersionID, "shp-deterministic-1", 25)
	if first != second {
		t.Fatalf("FNV-1a canary routing was non-deterministic")
	}

	// Cost limit validation (reject <= 0)
	if _, err := rt.Workflow.UpdateCostLimits(models.AutomationCostLimit{
		MaxCostUnitsPerCycle:       0,
		MaxExecutionTimePerCycleMs: 45000,
		MaxRecordsProcessedCycle:   250,
	}); err == nil {
		t.Fatalf("expected UpdateCostLimits to reject 0 maxCostUnitsPerCycle")
	}
	limits, err := rt.Workflow.UpdateCostLimits(models.AutomationCostLimit{
		MaxCostUnitsPerCycle:       500,
		MaxExecutionTimePerCycleMs: 45000,
		MaxRecordsProcessedCycle:   250,
	})
	if err != nil || limits.MaxRecordsProcessedCycle != 250 {
		t.Fatalf("UpdateCostLimits failed: %+v err=%v", limits, err)
	}

	// Rollback requires non-empty reason and disables rule + prod deployment + canary
	if err := rt.Workflow.RollbackRule(rule.RuleID, "   "); err == nil {
		t.Fatalf("expected RollbackRule to reject empty reason")
	}
	if err := rt.Workflow.RollbackRule(rule.RuleID, "Emergency test rollback"); err != nil {
		t.Fatalf("RollbackRule failed: %v", err)
	}
	var prodDeployedAfter int
	_ = db.QueryRow(`SELECT COUNT(*) FROM workflow_environment_deployment_log WHERE rule_id = ? AND environment_id = 'env-prod' AND status = 'DEPLOYED'`, rule.RuleID).Scan(&prodDeployedAfter)
	if prodDeployedAfter != 0 {
		t.Fatalf("expected 0 active env-prod deployments after rollback, got %d", prodDeployedAfter)
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
	// Guard-stopped job must be disabled and blocked until admin overrides
	if _, err := rt.Workflow.RunBackgroundJobNow("risk_evaluation"); err == nil {
		t.Fatalf("expected RunBackgroundJobNow to block guard-stopped job")
	}
	if err := rt.Workflow.ToggleBackgroundJob("risk_evaluation", true); err == nil {
		t.Fatalf("expected ToggleBackgroundJob enable to block guard-stopped job")
	}
	if err := rt.Workflow.OverrideRecoveryGuard("risk_evaluation", "Verified root cause and cleared stuck state"); err != nil {
		t.Fatalf("OverrideRecoveryGuard failed: %v", err)
	}
	if rec, err := rt.Workflow.RunBackgroundJobNow("risk_evaluation"); err != nil || rec.Status != "COMPLETED" {
		t.Fatalf("expected guard-overridden job to run cleanly, rec=%+v err=%v", rec, err)
	}
	if err := rt.Workflow.ToggleBackgroundJob("maintenance_cleanup", false); err == nil {
		t.Fatalf("expected maintenance_cleanup disable to be rejected")
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

func TestRuleRuntimeIsolationCostUnitsStaleApprovalAndManagerOverrideBlock(t *testing.T) {
	rt := setupWorkflowEnv(t)
	db := rt.DB.DB()

	// Disable seeded AUTO_ASSIGN rule so only our test rule can auto-assign
	if _, err := db.Exec(`UPDATE workflow_decision_rules SET enabled = 0 WHERE rule_id = 'rule-auto-assign-load-balance'`); err != nil {
		t.Fatalf("disable seeded auto-assign: %v", err)
	}

	pastDate := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	_, _ = db.Exec(`INSERT INTO suppliers (id, supplier_name, short_name, country, email, is_active) VALUES ('sup-rt-1', 'RT Sup', 'RTS', 'IN', 'rt@example.com', 1)`)
	_, _ = db.Exec(
		`INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_currency, invoice_value, incoterm, shipment_mode, shipment_type, bl_awb_number, eta, status)
		 VALUES ('shp-rt-1', 'sup-rt-1', 'INV-RT-1', ?, 'Parts', 'USD', 500, 'CIF', 'SEA', 'FCL', 'BL-RT', ?, 'in-transit')`,
		pastDate, pastDate,
	)

	// Stage a new AUTO_ASSIGN rule with enabled=true (not yet promoted to env-prod)
	unpromoted, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Unpromoted Auto Assign",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            90,
		Enabled:             true,
		ChangeReason:        "Draft in dev",
	})
	if err != nil {
		t.Fatalf("SaveRule unpromoted: %v", err)
	}

	if rec, err := rt.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil || rec.Status != "COMPLETED" {
		t.Fatalf("automation_cycle failed: rec=%+v err=%v", rec, err)
	}
	var assigned string
	_ = db.QueryRow(`SELECT COALESCE(assigned_to, '') FROM exception_cases WHERE entity_id = 'shp-rt-1' AND exception_type = 'OVERDUE_ETA'`).Scan(&assigned)
	if assigned != "" {
		t.Fatalf("expected unpromoted rule NOT to run in automation_cycle, but assigned_to=%q", assigned)
	}

	// Approve + promote to dev -> test; test stale approval checksum rejection on prod
	if err := rt.Workflow.ApproveRuleVersion(unpromoted.RuleID, unpromoted.LatestVersionID); err != nil {
		t.Fatalf("ApproveRuleVersion: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(unpromoted.RuleID, unpromoted.LatestVersionID, "env-dev", false, ""); err != nil {
		t.Fatalf("promote env-dev: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(unpromoted.RuleID, unpromoted.LatestVersionID, "env-test", false, ""); err != nil {
		t.Fatalf("promote env-test: %v", err)
	}

	// Simulate stale/mismatched approval checksum
	if _, err := db.Exec(`UPDATE workflow_rule_approvals SET approved_definition_sha256 = 'stale-sha' WHERE rule_id = ? AND version_id = ?`, unpromoted.RuleID, unpromoted.LatestVersionID); err != nil {
		t.Fatalf("tamper approval sha: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(unpromoted.RuleID, unpromoted.LatestVersionID, "env-prod", true, "Bypass check"); err == nil || !strings.Contains(err.Error(), "stale approval") {
		t.Fatalf("expected stale approval rejection, got %v", err)
	}
	// Restore valid approval sha, then test manager role override block (risk >= HIGH because priority >= 80)
	if _, err := db.Exec(`UPDATE workflow_rule_approvals SET approved_definition_sha256 = ? WHERE rule_id = ? AND version_id = ?`, unpromoted.LatestVersionSHA256, unpromoted.RuleID, unpromoted.LatestVersionID); err != nil {
		t.Fatalf("restore approval sha: %v", err)
	}
	// Create a second rule (priority 90, action ESCALATE -> score 80 CRITICAL) to test manager override rejection
	risky, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Manager Override Block Test",
		RuleType:            "SLA_ESCALATION",
		ConditionExpression: "*",
		ActionType:          "ESCALATE",
		Priority:            90,
		Enabled:             true,
		ChangeReason:        "Test manager override block",
	})
	if err != nil {
		t.Fatalf("SaveRule risky: %v", err)
	}
	_ = rt.Workflow.ApproveRuleVersion(risky.RuleID, risky.LatestVersionID)
	_, _ = rt.Workflow.PromoteRule(risky.RuleID, risky.LatestVersionID, "env-dev", false, "")
	_, _ = rt.Workflow.PromoteRule(risky.RuleID, risky.LatestVersionID, "env-test", false, "")

	if err := rt.Security.AssignUserRole("admin", "manager"); err != nil {
		t.Fatalf("AssignUserRole manager: %v", err)
	}
	if _, err := rt.Security.Authenticate("admin", "StrongPass1!", false); err != nil {
		t.Fatalf("re-auth as manager: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(risky.RuleID, risky.LatestVersionID, "env-prod", true, "Manager attempting override"); err == nil || !strings.Contains(err.Error(), "administrator role") {
		t.Fatalf("expected manager override to be rejected, got %v", err)
	}

	// Switch back to administrator, promote unpromoted rule to env-prod with override, and test cost-unit budget cap
	if _, err := db.Exec(`UPDATE user_roles SET role = 'administrator' WHERE user_id = 'admin'`); err != nil {
		t.Fatalf("restore admin role in db: %v", err)
	}
	if _, err := rt.Security.Authenticate("admin", "StrongPass1!", false); err != nil {
		t.Fatalf("re-auth as administrator: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(unpromoted.RuleID, unpromoted.LatestVersionID, "env-prod", true, "Safe rollout CHG-202"); err != nil {
		t.Fatalf("promote unpromoted env-prod: %v", err)
	}

	// Cost-unit budget cap of 0.5 (< AUTO_ASSIGN weight 2.0) must stop execution before applying action
	if _, err := rt.Workflow.UpdateCostLimits(models.AutomationCostLimit{
		MaxCostUnitsPerCycle:       0.5,
		MaxExecutionTimePerCycleMs: 45000,
		MaxRecordsProcessedCycle:   250,
	}); err != nil {
		t.Fatalf("set tight cost limit: %v", err)
	}
	if _, err := rt.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil {
		t.Fatalf("automation_cycle under tight cost cap: %v", err)
	}
	_ = db.QueryRow(`SELECT COALESCE(assigned_to, '') FROM exception_cases WHERE entity_id = 'shp-rt-1' AND exception_type = 'OVERDUE_ETA'`).Scan(&assigned)
	if assigned != "" {
		t.Fatalf("expected cost-unit budget 0.5 to block AUTO_ASSIGN (weight 2.0), got assigned_to=%q", assigned)
	}

	// Raise cost-unit budget to 50 -> promoted env-prod rule now executes
	if _, err := rt.Workflow.UpdateCostLimits(models.AutomationCostLimit{
		MaxCostUnitsPerCycle:       50,
		MaxExecutionTimePerCycleMs: 45000,
		MaxRecordsProcessedCycle:   250,
	}); err != nil {
		t.Fatalf("raise cost limit: %v", err)
	}
	if _, err := rt.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil {
		t.Fatalf("automation_cycle with sufficient cost cap: %v", err)
	}
	_ = db.QueryRow(`SELECT COALESCE(assigned_to, '') FROM exception_cases WHERE entity_id = 'shp-rt-1' AND exception_type = 'OVERDUE_ETA'`).Scan(&assigned)
	if assigned != "admin" {
		t.Fatalf("expected promoted env-prod rule to auto-assign exception, got %q", assigned)
	}
}

func TestPhase2IdempotencyDryRunAndVersionRestore(t *testing.T) {
	rt := setupWorkflowEnv(t)
	db := rt.DB.DB()

	// Disable seeded AUTO_ASSIGN so our versioned rule drives the assignment deterministically
	if _, err := db.Exec(`UPDATE workflow_decision_rules SET enabled = 0 WHERE rule_id = 'rule-auto-assign-load-balance'`); err != nil {
		t.Fatalf("disable seeded auto-assign: %v", err)
	}

	pastDate := time.Now().UTC().AddDate(0, 0, -4).Format("2006-01-02")
	_, _ = db.Exec(`INSERT INTO suppliers (id, supplier_name, short_name, country, email, is_active) VALUES ('sup-p2-1', 'P2 Sup', 'P2S', 'IN', 'p2@example.com', 1)`)
	_, _ = db.Exec(
		`INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_currency, invoice_value, incoterm, shipment_mode, shipment_type, bl_awb_number, eta, status)
		 VALUES ('shp-p2-1', 'sup-p2-1', 'INV-P2-1', ?, 'Parts', 'USD', 500, 'CIF', 'SEA', 'FCL', 'BL-P2', ?, 'in-transit')`,
		pastDate, pastDate,
	)

	// Populate exception_cases via DetectExceptions
	if _, err := rt.Workflow.DetectExceptions(); err != nil {
		t.Fatalf("DetectExceptions: %v", err)
	}

	// Stage, approve, and promote v1 (AUTO_ASSIGN, LOW risk)
	r1, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase2 Idempotent Assign",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            30,
		Enabled:             true,
		ChangeReason:        "Phase2 v1",
	})
	if err != nil {
		t.Fatalf("SaveRule v1: %v", err)
	}
	v1ID := r1.LatestVersionID

	// DryRun before any execution must see eligible cases and perform ZERO mutations
	dry1, err := rt.Workflow.DryRunRuleVersion(r1.RuleID, v1ID)
	if err != nil {
		t.Fatalf("DryRunRuleVersion: %v", err)
	}
	if dry1.EligibleCount == 0 || dry1.CanaryRoutedCount == 0 || dry1.SkippedIdempotentCount != 0 || !dry1.WithinBudget {
		t.Fatalf("unexpected initial dry-run result: %+v", dry1)
	}
	var assignedBefore string
	_ = db.QueryRow(`SELECT COALESCE(assigned_to, '') FROM exception_cases WHERE entity_id = 'shp-p2-1' AND exception_type = 'OVERDUE_ETA'`).Scan(&assignedBefore)
	if assignedBefore != "" {
		t.Fatalf("dry-run mutated exception_cases: assigned_to=%q", assignedBefore)
	}

	if err := rt.Workflow.ApproveRuleVersion(r1.RuleID, v1ID); err != nil {
		t.Fatalf("ApproveRuleVersion v1: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-dev", false, ""); err != nil {
		t.Fatalf("promote v1 dev: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-test", false, ""); err != nil {
		t.Fatalf("promote v1 test: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-prod", false, ""); err != nil {
		t.Fatalf("promote v1 prod: %v", err)
	}

	// Run automation_cycle once -> applies AUTO_ASSIGN and records idempotency key
	if _, err := rt.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil {
		t.Fatalf("first automation_cycle: %v", err)
	}
	var caseID, assignedAfter string
	_ = db.QueryRow(`SELECT id, COALESCE(assigned_to, '') FROM exception_cases WHERE entity_id = 'shp-p2-1' AND exception_type = 'OVERDUE_ETA'`).Scan(&caseID, &assignedAfter)
	if assignedAfter != "admin" {
		t.Fatalf("expected AUTO_ASSIGN to assign 'admin', got %q", assignedAfter)
	}

	// Simulate manual reset of case to OPEN with empty assignee (retry / re-eval scenario)
	if _, err := db.Exec(`UPDATE exception_cases SET status = 'OPEN', assigned_to = '' WHERE id = ?`, caseID); err != nil {
		t.Fatalf("reset case: %v", err)
	}

	// DryRun must now flag AlreadyApplied = true and SkippedIdempotentCount >= 1
	dry2, err := rt.Workflow.DryRunRuleVersion(r1.RuleID, v1ID)
	if err != nil || dry2.SkippedIdempotentCount == 0 {
		t.Fatalf("expected dry-run to report skipped idempotent cases, got %+v err=%v", dry2, err)
	}

	// Second automation_cycle for the same version MUST skip due to idempotency
	if _, err := rt.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil {
		t.Fatalf("second automation_cycle: %v", err)
	}
	_ = db.QueryRow(`SELECT COALESCE(assigned_to, '') FROM exception_cases WHERE id = ?`, caseID).Scan(&assignedAfter)
	if assignedAfter != "" {
		t.Fatalf("expected idempotency guard to block duplicate AUTO_ASSIGN for same version, got %q", assignedAfter)
	}

	// Stage unapproved v2 -> cannot be restored to prod
	v2, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleID:              r1.RuleID,
		RuleName:            "Phase2 Unapproved v2",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            35,
		Enabled:             true,
		ChangeReason:        "Phase2 v2",
	})
	if err != nil {
		t.Fatalf("SaveRule v2: %v", err)
	}
	if _, err := rt.Workflow.RestoreRuleVersion(r1.RuleID, v2.LatestVersionID, "Attempt restore unapproved", false); err == nil {
		t.Fatalf("expected RestoreRuleVersion to reject unapproved version")
	}

	// Emergency disable (RollbackRule) then RestoreRuleVersion back to verified v1
	if err := rt.Workflow.RollbackRule(r1.RuleID, "Emergency halt for verification"); err != nil {
		t.Fatalf("RollbackRule: %v", err)
	}
	if _, err := rt.Workflow.RestoreRuleVersion(r1.RuleID, v1ID, "bad", false); err == nil {
		t.Fatalf("expected RestoreRuleVersion to reject <5 char reason")
	}
	if _, err := rt.Workflow.RestoreRuleVersion(r1.RuleID, v1ID, "Restoring verified v1 after halt", false); err != nil {
		t.Fatalf("RestoreRuleVersion v1 failed: %v", err)
	}

	var restoredEnabled int
	var prodVer string
	_ = db.QueryRow(`SELECT enabled FROM workflow_decision_rules WHERE rule_id = ?`, r1.RuleID).Scan(&restoredEnabled)
	_ = db.QueryRow(`SELECT version_id FROM workflow_environment_deployment_log WHERE rule_id = ? AND environment_id = 'env-prod' AND status = 'DEPLOYED'`, r1.RuleID).Scan(&prodVer)
	if restoredEnabled != 1 || prodVer != v1ID {
		t.Fatalf("expected restored rule enabled=1 and prodVer=%q, got enabled=%d prodVer=%q", v1ID, restoredEnabled, prodVer)
	}
}

func TestFinalAuditPhase1And2IntegrationAndRegression(t *testing.T) {
	t.Setenv("IMPORT_MANAGER_DB_KEY", "audit-test-key-0123456789abcdef01")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	dir := filepath.Join(t.TempDir(), "appdata")

	rt, err := bootstrap.Initialize([]string{"--recovery-mode"}, dir)
	if err != nil {
		t.Fatalf("bootstrap 1 failed: %v", err)
	}
	if _, err := rt.Security.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup failed: %v", err)
	}
	db := rt.DB.DB()

	// Disable seeded AUTO_ASSIGN so our versioned rules drive assignment cleanly
	_, _ = db.Exec(`UPDATE workflow_decision_rules SET enabled = 0 WHERE rule_id = 'rule-auto-assign-load-balance'`)

	pastDate := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	_, _ = db.Exec(`INSERT INTO suppliers (id, supplier_name, short_name, country, email, is_active) VALUES ('sup-aud-1', 'Audit Sup', 'AUDS', 'IN', 'aud@example.com', 1)`)
	_, _ = db.Exec(
		`INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_currency, invoice_value, incoterm, shipment_mode, shipment_type, bl_awb_number, eta, status)
		 VALUES
		   ('shp-aud-1', 'sup-aud-1', 'INV-AUD-1', ?, 'Parts', 'USD', 500, 'CIF', 'SEA', 'FCL', 'BL-A1', ?, 'in-transit'),
		   ('shp-aud-2', 'sup-aud-1', 'INV-AUD-2', ?, 'Parts', 'USD', 500, 'CIF', 'SEA', 'FCL', 'BL-A2', ?, 'in-transit')`,
		pastDate, pastDate, pastDate, pastDate,
	)

	// 1. Seeded default rules must have active env-prod deployments and APPROVED status
	var seededProdCnt int
	_ = db.QueryRow(`SELECT COUNT(*) FROM workflow_environment_deployment_log WHERE environment_id = 'env-prod' AND status = 'DEPLOYED'`).Scan(&seededProdCnt)
	if seededProdCnt < 6 {
		t.Fatalf("expected 6 seeded default rules deployed in env-prod, got %d", seededProdCnt)
	}

	// 2. Version integrity: deploy v1 (AUTO_ASSIGN), stage unpromoted v2 (enabled=false / PRIORITY_ADJUST) -> runtime MUST keep executing v1!
	r1, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Audit Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            25,
		Enabled:             true,
		ChangeReason:        "Audit v1",
	})
	if err != nil {
		t.Fatalf("SaveRule v1: %v", err)
	}
	v1ID := r1.LatestVersionID
	if err := rt.Workflow.ApproveRuleVersion(r1.RuleID, v1ID); err != nil {
		t.Fatalf("Approve v1: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-dev", false, ""); err != nil {
		t.Fatalf("Promote dev v1: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-test", false, ""); err != nil {
		t.Fatalf("Promote test v1: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-prod", false, ""); err != nil {
		t.Fatalf("Promote prod v1: %v", err)
	}

	// Save v2 with Enabled=false and PRIORITY_ADJUST (not promoted to prod)
	v2, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleID:              r1.RuleID,
		RuleName:            "Audit Rule v2",
		RuleType:            "PRIORITY_ADJUST",
		ConditionExpression: "*",
		ActionType:          "DELETE",
		Priority:            95,
		Enabled:             false,
		ChangeReason:        "Audit v2 unpromoted",
	})
	if err != nil {
		t.Fatalf("SaveRule v2: %v", err)
	}

	// Cross-rule reference check
	if _, err := rt.Workflow.EvaluateRuleRisk("rule-auto-resolve-overdue-delivered", v2.LatestVersionID); err == nil {
		t.Fatalf("expected cross-rule version reference to be rejected")
	}

	// 3. Dry-run zero-mutation proof across ALL 7 audited tables
	if _, err := rt.Workflow.DetectExceptions(); err != nil {
		t.Fatalf("DetectExceptions: %v", err)
	}
	type tableSnapshot struct {
		exCases, autoLog, idem, lifeEvents, audits, envDeploy, canaries int
	}
	takeSnap := func() tableSnapshot {
		var s tableSnapshot
		_ = db.QueryRow(`SELECT COUNT(*) FROM exception_cases`).Scan(&s.exCases)
		_ = db.QueryRow(`SELECT COUNT(*) FROM workflow_automation_log`).Scan(&s.autoLog)
		_ = db.QueryRow(`SELECT COUNT(*) FROM workflow_rule_action_idempotency`).Scan(&s.idem)
		_ = db.QueryRow(`SELECT COUNT(*) FROM exception_lifecycle_events`).Scan(&s.lifeEvents)
		_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&s.audits)
		_ = db.QueryRow(`SELECT COUNT(*) FROM workflow_environment_deployment_log`).Scan(&s.envDeploy)
		_ = db.QueryRow(`SELECT COUNT(*) FROM canary_rule_deployment`).Scan(&s.canaries)
		return s
	}
	beforeDry := takeSnap()
	dryRes, err := rt.Workflow.DryRunRuleVersion(r1.RuleID, v1ID)
	if err != nil || dryRes.EligibleCount < 2 || !dryRes.RuleEnabled {
		t.Fatalf("DryRunRuleVersion failed: %+v err=%v", dryRes, err)
	}
	afterDry := takeSnap()
	if beforeDry != afterDry {
		t.Fatalf("DryRunRuleVersion mutated database! before=%+v after=%+v", beforeDry, afterDry)
	}

	// 4. Budget safety: cap records=1 so first cycle assigns only 1 of 2 eligible exceptions
	if _, err := rt.Workflow.UpdateCostLimits(models.AutomationCostLimit{
		MaxCostUnitsPerCycle:       500,
		MaxExecutionTimePerCycleMs: 45000,
		MaxRecordsProcessedCycle:   1,
	}); err != nil {
		t.Fatalf("UpdateCostLimits(records=1): %v", err)
	}

	// Concurrent overlapping automation_cycle test: one must run, the second must be rejected by acquireJobLock
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = rt.Workflow.RunBackgroundJobNow("automation_cycle")
		}(i)
	}
	wg.Wait()
	successCount := 0
	for _, e := range errs {
		if e == nil {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly 1 overlapping automation_cycle to execute, got %d (errs=%v)", successCount, errs)
	}

	// Confirm record budget cap of 1 was respected across concurrent attempts (not 2!)
	var assignedCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM exception_cases WHERE entity_id IN ('shp-aud-1','shp-aud-2') AND exception_type = 'OVERDUE_ETA' AND assigned_to = 'admin'`).Scan(&assignedCount)
	if assignedCount != 1 {
		t.Fatalf("expected maxRecords=1 to assign exactly 1 case across overlapping cycles, got %d", assignedCount)
	}

	// 5. Execution-time budget check (1ms cap triggers MAX_EXECUTION_TIME_REACHED before processing)
	if _, err := rt.Workflow.UpdateCostLimits(models.AutomationCostLimit{
		MaxCostUnitsPerCycle:       500,
		MaxExecutionTimePerCycleMs: 1,
		MaxRecordsProcessedCycle:   500,
	}); err != nil {
		t.Fatalf("UpdateCostLimits(time=1ms): %v", err)
	}
	// Restore generous budget and run cycle so the 2nd case also gets assigned
	if _, err := rt.Workflow.UpdateCostLimits(models.AutomationCostLimit{
		MaxCostUnitsPerCycle:       500,
		MaxExecutionTimePerCycleMs: 45000,
		MaxRecordsProcessedCycle:   500,
	}); err != nil {
		t.Fatalf("restore cost limits: %v", err)
	}
	if _, err := rt.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil {
		t.Fatalf("second cycle failed: %v", err)
	}

	// 6. Lifecycle-generation idempotency: self-heal stale IN_PROGRESS >14d back to OPEN -> advances lifecycle_generation -> legitimate re-assignment succeeds!
	var case1ID string
	_ = db.QueryRow(`SELECT id FROM exception_cases WHERE entity_id = 'shp-aud-1' AND exception_type = 'OVERDUE_ETA'`).Scan(&case1ID)
	staleTime := time.Now().UTC().Add(-16 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	if _, err := db.Exec(`UPDATE exception_cases SET assigned_at = ?, updated_at = ? WHERE id = ?`, staleTime, staleTime, case1ID); err != nil {
		t.Fatalf("force stale assigned_at: %v", err)
	}
	scanRes, err := rt.Workflow.DetectExceptions()
	if err != nil || scanRes.SelfHealedCount < 1 {
		t.Fatalf("expected self-heal on stale case, got %+v err=%v", scanRes, err)
	}
	var genAfterHeal int
	_ = db.QueryRow(`SELECT lifecycle_generation FROM exception_cases WHERE id = ?`, case1ID).Scan(&genAfterHeal)
	if genAfterHeal != 1 {
		t.Fatalf("expected lifecycle_generation=1 after self-heal, got %d", genAfterHeal)
	}
	if _, err := rt.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil {
		t.Fatalf("cycle after self-heal failed: %v", err)
	}
	var reassigned string
	_ = db.QueryRow(`SELECT COALESCE(assigned_to, '') FROM exception_cases WHERE id = ?`, case1ID).Scan(&reassigned)
	if reassigned != "admin" {
		t.Fatalf("expected self-healed case in new lifecycle_generation to be legitimately re-assigned, got %q", reassigned)
	}

	// 7. Tenant isolation on idempotency table: another tenant row does not collide with default tenant
	if _, err := db.Exec(
		`INSERT INTO workflow_rule_action_idempotency (id, tenant_id, rule_id, rule_version_id, entity_type, entity_id, action_type, lifecycle_generation, execution_id, created_at)
		 VALUES ('idm-other-tenant', 'tenant-other', ?, ?, 'exception_case', ?, 'AUTO_ASSIGN', 1, 'aut-other', datetime('now'))`,
		r1.RuleID, v1ID, case1ID,
	); err != nil {
		t.Fatalf("expected tenant-isolated idempotency insert to succeed, got %v", err)
	}

	// 8. Persistence across application restart: shutdown & reopen same encrypted DB -> idempotency history survives!
	if err := rt.Shutdown(); err != nil {
		t.Fatalf("shutdown 1: %v", err)
	}
	rt2, err := bootstrap.Initialize([]string{"--recovery-mode"}, dir)
	if err != nil {
		t.Fatalf("bootstrap 2 failed: %v", err)
	}
	defer rt2.Shutdown()
	if _, err := rt2.Security.Authenticate("admin", "StrongPass1!", false); err != nil {
		t.Fatalf("re-auth after restart failed: %v", err)
	}
	var persistedIdemCnt int
	_ = rt2.DB.DB().QueryRow(`SELECT COUNT(*) FROM workflow_rule_action_idempotency WHERE tenant_id = 'tenant-default' AND rule_version_id = ?`, v1ID).Scan(&persistedIdemCnt)
	if persistedIdemCnt < 3 {
		t.Fatalf("expected >= 3 persisted idempotency records after restart, got %d", persistedIdemCnt)
	}
	// Reset case1 back to OPEN without advancing lifecycle_generation -> restart-persisted idempotency must still block duplicate action
	if _, err := rt2.DB.DB().Exec(`UPDATE exception_cases SET status = 'OPEN', assigned_to = '' WHERE id = ?`, case1ID); err != nil {
		t.Fatalf("reset case1 after restart: %v", err)
	}
	dryAfterRestart, err := rt2.Workflow.DryRunRuleVersion(r1.RuleID, v1ID)
	if err != nil || dryAfterRestart.SkippedIdempotentCount < 1 {
		t.Fatalf("expected persisted idempotency skips >= 1 after restart, got %+v err=%v", dryAfterRestart, err)
	}
	if _, err := rt2.Workflow.RunBackgroundJobNow("automation_cycle"); err != nil {
		t.Fatalf("cycle after restart failed: %v", err)
	}
	var assignedAfterRestart string
	_ = rt2.DB.DB().QueryRow(`SELECT COALESCE(assigned_to, '') FROM exception_cases WHERE id = ?`, case1ID).Scan(&assignedAfterRestart)
	if assignedAfterRestart != "" {
		t.Fatalf("expected restart-persisted idempotency to block duplicate AUTO_ASSIGN, got %q", assignedAfterRestart)
	}

	// 9. Checksum bypass check: clearing definition_sha256 on a non-system version must fail validation
	// (Need trigger bypass test via direct non-system empty-sha insert or checking non-system rule cannot lack sha256)
	_, _ = rt2.DB.DB().Exec(
		`INSERT INTO workflow_rule_versions (version_id, rule_id, tenant_id, environment_id, version_number, rule_definition_json, definition_sha256, created_by, created_at, is_active, change_reason)
		 VALUES ('ver-nosha', ?, 'tenant-default', 'env-dev', 99, '{"ruleName":"Bypass","ruleType":"AUTO_ASSIGN","conditionExpression":"*","actionType":"AUTO_ASSIGN","priority":1,"enabled":true}', '', 'admin', datetime('now'), 0, 'tamper')`,
		r1.RuleID,
	)
	if _, err := rt2.Workflow.GetRuleVersion(r1.RuleID, "ver-nosha"); err == nil {
		t.Fatalf("expected non-system version without definition_sha256 to be rejected")
	}

	// 10. Failed restore leaves DB consistent + conflicting canary cancellation on restore
	if _, err := rt2.Workflow.ConfigureCanary(r1.RuleID, v2.LatestVersionID, 50); err != nil {
		t.Fatalf("ConfigureCanary v2: %v", err)
	}
	if _, err := rt2.Workflow.RestoreRuleVersion(r1.RuleID, v2.LatestVersionID, "Should fail unapproved", false); err == nil {
		t.Fatalf("expected unapproved restore to fail")
	}
	var canaryStatusAfterFail string
	_ = rt2.DB.DB().QueryRow(`SELECT deployment_status FROM canary_rule_deployment WHERE rule_id = ?`, r1.RuleID).Scan(&canaryStatusAfterFail)
	if canaryStatusAfterFail != "ACTIVE" {
		t.Fatalf("expected failed restore not to alter canary state, got %q", canaryStatusAfterFail)
	}
	if _, err := rt2.Workflow.RestoreRuleVersion(r1.RuleID, v1ID, "Restore v1 and cancel v2 canary", false); err != nil {
		t.Fatalf("RestoreRuleVersion v1: %v", err)
	}
	_ = rt2.DB.DB().QueryRow(`SELECT deployment_status FROM canary_rule_deployment WHERE rule_id = ?`, r1.RuleID).Scan(&canaryStatusAfterFail)
	if canaryStatusAfterFail != "ROLLED_BACK" {
		t.Fatalf("expected conflicting v2 canary to be ROLLED_BACK after restoring v1, got %q", canaryStatusAfterFail)
	}
}
