package workflow_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/workflow"
)

func promoteRuleToProdForTest(t *testing.T, rt *bootstrap.Runtime, ruleID, versionID string) {
	t.Helper()
	if err := rt.Workflow.ApproveRuleVersion(ruleID, versionID); err != nil {
		t.Fatalf("ApproveRuleVersion(%s, %s): %v", ruleID, versionID, err)
	}
	if _, err := rt.Workflow.PromoteRule(ruleID, versionID, "env-dev", false, ""); err != nil {
		t.Fatalf("Promote dev(%s, %s): %v", ruleID, versionID, err)
	}
	if _, err := rt.Workflow.PromoteRule(ruleID, versionID, "env-test", false, ""); err != nil {
		t.Fatalf("Promote test(%s, %s): %v", ruleID, versionID, err)
	}
	if _, err := rt.Workflow.PromoteRule(ruleID, versionID, "env-prod", false, ""); err != nil {
		t.Fatalf("Promote prod(%s, %s): %v", ruleID, versionID, err)
	}
}

func findRuleHealth(t *testing.T, ov models.DeploymentHealthOverview, ruleID string) models.RuleDeploymentHealth {
	t.Helper()
	for _, r := range ov.RuleDeployments {
		if r.RuleID == ruleID {
			return r
		}
	}
	t.Fatalf("rule %s not found in DeploymentHealthOverview", ruleID)
	return models.RuleDeploymentHealth{}
}

func TestPhase3BDeploymentHealthAllStatesAndAlertAck(t *testing.T) {
	rt := setupWorkflowEnv(t)

	// Configure deterministic thresholds
	_, err := rt.Workflow.UpdateRollbackThresholds(models.CanaryRollbackThreshold{
		MinSampleSize:           10,
		MaxFailureRate:          0.25,
		MaxCostIncreasePct:      50,
		MaxDurationIncreasePct:  100,
		MonitoringWindowMinutes: 60,
	})
	if err != nil {
		t.Fatalf("UpdateRollbackThresholds: %v", err)
	}

	// Disable seeded default rules so health assertions isolate our test rules cleanly
	if _, err := rt.DB.DB().Exec(`UPDATE workflow_decision_rules SET enabled = 0`); err != nil {
		t.Fatalf("disable seeded rules: %v", err)
	}

	// Rule 1: HEALTHY active production rule with >=10 clean executions
	rHealthy, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase3B Healthy Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            20,
		Enabled:             true,
		ChangeReason:        "healthy baseline",
	})
	if err != nil {
		t.Fatalf("SaveRule healthy: %v", err)
	}
	promoteRuleToProdForTest(t, rt, rHealthy.RuleID, rHealthy.LatestVersionID)
	if err := rt.Workflow.RecordCanaryCohortMetrics(rHealthy.RuleID, rHealthy.LatestVersionID, "",
		models.CanaryCohortMetrics{},
		models.CanaryCohortMetrics{AttemptedCount: 15, SuccessCount: 15, FailureCount: 0, CostUnits: 15, AvgDurationMs: 2.0},
	); err != nil {
		t.Fatalf("RecordCanaryCohortMetrics healthy: %v", err)
	}

	// Rule 2: INSUFFICIENT SAMPLES -> UNKNOWN (even with 0 failures!)
	rUnknown, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase3B Insufficient Samples Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            15,
		Enabled:             true,
		ChangeReason:        "insufficient samples",
	})
	if err != nil {
		t.Fatalf("SaveRule unknown: %v", err)
	}
	promoteRuleToProdForTest(t, rt, rUnknown.RuleID, rUnknown.LatestVersionID)
	if err := rt.Workflow.RecordCanaryCohortMetrics(rUnknown.RuleID, rUnknown.LatestVersionID, "",
		models.CanaryCohortMetrics{},
		models.CanaryCohortMetrics{AttemptedCount: 4, SuccessCount: 4, FailureCount: 0, CostUnits: 4, AvgDurationMs: 2.0},
	); err != nil {
		t.Fatalf("RecordCanaryCohortMetrics unknown: %v", err)
	}

	// Rule 3: DISABLED rule (manually disabled, 0 failures)
	rDisabled, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase3B Disabled Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            10,
		Enabled:             false,
		ChangeReason:        "disabled test",
	})
	if err != nil {
		t.Fatalf("SaveRule disabled: %v", err)
	}

	// Rule 4: WARNING active canary rule (cost spike > maxCostIncreasePct without failure threshold breach)
	rWarning, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase3B Warning Cost Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            25,
		Enabled:             true,
		ChangeReason:        "warning cost",
	})
	if err != nil {
		t.Fatalf("SaveRule warning: %v", err)
	}
	if err := rt.Workflow.ApproveRuleVersion(rWarning.RuleID, rWarning.LatestVersionID); err != nil {
		t.Fatalf("Approve warning: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rWarning.RuleID, rWarning.LatestVersionID, "env-dev", false, ""); err != nil {
		t.Fatalf("Promote dev warning: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rWarning.RuleID, rWarning.LatestVersionID, "env-test", false, ""); err != nil {
		t.Fatalf("Promote test warning: %v", err)
	}
	if _, err := rt.Workflow.AdvanceCanaryStage(rWarning.RuleID, rWarning.LatestVersionID); err != nil {
		t.Fatalf("AdvanceCanaryStage warning: %v", err)
	}
	if err := rt.Workflow.RecordCanaryCohortMetrics(rWarning.RuleID, rWarning.LatestVersionID, "",
		models.CanaryCohortMetrics{AttemptedCount: 12, SuccessCount: 12, FailureCount: 0, CostUnits: 48, AvgDurationMs: 2.0},
		models.CanaryCohortMetrics{AttemptedCount: 12, SuccessCount: 12, FailureCount: 0, CostUnits: 12, AvgDurationMs: 2.0},
	); err != nil {
		t.Fatalf("RecordCanaryCohortMetrics warning: %v", err)
	}

	// Rule 5: CRITICAL via automatic rollback without safe prior version + operational alert
	rCritical, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase3B Critical AutoRollback Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            30,
		Enabled:             true,
		ChangeReason:        "critical breach",
	})
	if err != nil {
		t.Fatalf("SaveRule critical: %v", err)
	}
	if err := rt.Workflow.ApproveRuleVersion(rCritical.RuleID, rCritical.LatestVersionID); err != nil {
		t.Fatalf("Approve critical: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rCritical.RuleID, rCritical.LatestVersionID, "env-dev", false, ""); err != nil {
		t.Fatalf("Promote dev critical: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rCritical.RuleID, rCritical.LatestVersionID, "env-test", false, ""); err != nil {
		t.Fatalf("Promote test critical: %v", err)
	}
	if _, err := rt.Workflow.ConfigureCanary(rCritical.RuleID, rCritical.LatestVersionID, 10); err != nil {
		t.Fatalf("ConfigureCanary critical: %v", err)
	}
	if err := rt.Workflow.RecordCanaryCohortMetrics(rCritical.RuleID, rCritical.LatestVersionID, "",
		models.CanaryCohortMetrics{AttemptedCount: 12, SuccessCount: 6, FailureCount: 6, CostUnits: 12, AvgDurationMs: 2.0},
		models.CanaryCohortMetrics{AttemptedCount: 12, SuccessCount: 12, FailureCount: 0, CostUnits: 12, AvgDurationMs: 2.0},
	); err != nil {
		t.Fatalf("RecordCanaryCohortMetrics critical: %v", err)
	}
	rbkRes, err := rt.Workflow.EvaluateCanaryAndAutoRollback(rCritical.RuleID, rCritical.LatestVersionID)
	if err != nil {
		t.Fatalf("EvaluateCanaryAndAutoRollback: %v", err)
	}
	if !rbkRes.Triggered || !rbkRes.Stage1Disabled || rbkRes.Stage2Restored {
		t.Fatalf("expected triggered Stage 1 auto-rollback without Stage 2 recovery, got %+v", rbkRes)
	}

	// Rule 6: Awaiting approval rule (PENDING) + Manual rollback rule (DISABLED, never HEALTHY)
	rPending, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase3B Pending Rule",
		RuleType:            "PRIORITY_ADJUST",
		ConditionExpression: "sla_status == 'BREACHED'",
		ActionType:          "PRIORITY_ADJUST",
		Priority:            12,
		Enabled:             true,
		ChangeReason:        "pending approval",
	})
	if err != nil {
		t.Fatalf("SaveRule pending: %v", err)
	}

	rManualRbk, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase3B Manual Rollback Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            18,
		Enabled:             true,
		ChangeReason:        "manual rollback test",
	})
	if err != nil {
		t.Fatalf("SaveRule manual rollback: %v", err)
	}
	promoteRuleToProdForTest(t, rt, rManualRbk.RuleID, rManualRbk.LatestVersionID)
	if err := rt.Workflow.RollbackRule(rManualRbk.RuleID, "Operator safety pause"); err != nil {
		t.Fatalf("RollbackRule: %v", err)
	}

	ov, err := rt.Workflow.GetDeploymentHealthOverview()
	if err != nil {
		t.Fatalf("GetDeploymentHealthOverview: %v", err)
	}
	if ov.OverallHealth != workflow.HealthStateCritical {
		t.Fatalf("expected OverallHealth CRITICAL, got %s (%s)", ov.OverallHealth, ov.HealthSummaryReason)
	}
	if ov.ActiveProductionRules != 2 {
		t.Fatalf("expected ActiveProductionRules=2 (healthy + unknown), got %d", ov.ActiveProductionRules)
	}
	if ov.ActiveCanaryDeployments != 1 {
		t.Fatalf("expected ActiveCanaryDeployments=1, got %d", ov.ActiveCanaryDeployments)
	}
	if ov.RulesAwaitingApproval < 1 {
		t.Fatalf("expected >=1 rule awaiting approval, got %d", ov.RulesAwaitingApproval)
	}
	if ov.FailedOrRolledBackDeployments < 2 {
		t.Fatalf("expected >=2 failed/rolled-back deployments, got %d", ov.FailedOrRolledBackDeployments)
	}
	if ov.LatestDeploymentTimestamp == "" || ov.CurrentProductionVersionID == "" {
		t.Fatalf("expected non-empty LatestDeploymentTimestamp and CurrentProductionVersionID")
	}
	if len(ov.RecentAlerts) == 0 || len(ov.RecentRollbacks) == 0 {
		t.Fatalf("expected RecentAlerts and RecentRollbacks to be populated, got alerts=%d rollbacks=%d", len(ov.RecentAlerts), len(ov.RecentRollbacks))
	}

	if h := findRuleHealth(t, ov, rHealthy.RuleID); h.HealthStatus != workflow.HealthStateHealthy || len(h.HealthReasons) == 0 {
		t.Fatalf("expected HEALTHY with explanation, got %+v", h)
	}
	if h := findRuleHealth(t, ov, rUnknown.RuleID); h.HealthStatus != workflow.HealthStateUnknown || !strings.Contains(strings.ToLower(h.HealthReasons[0]), "insufficient") {
		t.Fatalf("expected UNKNOWN (insufficient samples) for <MinSampleSize, got %+v", h)
	}
	if h := findRuleHealth(t, ov, rDisabled.RuleID); h.HealthStatus != workflow.HealthStateDisabled {
		t.Fatalf("expected DISABLED for disabled rule, got %+v", h)
	}
	if h := findRuleHealth(t, ov, rWarning.RuleID); h.HealthStatus != workflow.HealthStateWarning {
		t.Fatalf("expected WARNING for cost-spike canary, got %+v", h)
	}
	if h := findRuleHealth(t, ov, rCritical.RuleID); h.HealthStatus != workflow.HealthStateCritical {
		t.Fatalf("expected CRITICAL for auto-rolled-back rule, got %+v", h)
	}
	if h := findRuleHealth(t, ov, rManualRbk.RuleID); h.HealthStatus == workflow.HealthStateHealthy {
		t.Fatalf("manually rolled-back rule must never appear HEALTHY, got %+v", h)
	}
	if h := findRuleHealth(t, ov, rPending.RuleID); h.HealthStatus != workflow.HealthStateUnknown {
		t.Fatalf("unpromoted staged rule should be UNKNOWN, got %+v", h)
	}

	// Acknowledge all alerts for rCritical -> HealthStatus must STILL NOT flip to HEALTHY!
	openAlerts, err := rt.Workflow.ListOperationalAlerts(false)
	if err != nil {
		t.Fatalf("ListOperationalAlerts: %v", err)
	}
	for _, al := range openAlerts {
		if al.RuleID == rCritical.RuleID {
			if err := rt.Workflow.AcknowledgeOperationalAlert(al.ID); err != nil {
				t.Fatalf("AcknowledgeOperationalAlert(%s): %v", al.ID, err)
			}
		}
	}
	ovAfterAck, err := rt.Workflow.GetDeploymentHealthOverview()
	if err != nil {
		t.Fatalf("GetDeploymentHealthOverview after ack: %v", err)
	}
	if h := findRuleHealth(t, ovAfterAck, rCritical.RuleID); h.HealthStatus != workflow.HealthStateCritical {
		t.Fatalf("acknowledging alert must NOT make rule healthy; expected CRITICAL, got %+v", h)
	}
}

func TestPhase3BVersionComparisonDryRunNoMutationAndOwnership(t *testing.T) {
	rt := setupWorkflowEnv(t)

	// Seed parent supplier/shipments and open overdue exceptions to compare dry-run matching
	if _, err := rt.DB.DB().Exec(
		`INSERT INTO suppliers (id, supplier_name, short_name, country, email, is_active) VALUES ('sup-cmp-1', 'Cmp Supplier', 'CMP', 'IN', 'cmp@example.com', 1)`,
	); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}
	if _, err := rt.DB.DB().Exec(
		`INSERT INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, shipment_mode, shipment_type, bl_awb_number, eta, status)
		 VALUES ('shp-cmp-1', 'sup-cmp-1', 'INV-CMP-1', '2026-11-10', 'General', 1000, 'USD', 'CIF', 'SEA', 'FCL', 'BL-C1', '2026-11-15', 'customs-clearance'),
		        ('shp-cmp-2', 'sup-cmp-1', 'INV-CMP-2', '2026-11-10', 'General', 1000, 'USD', 'CIF', 'SEA', 'FCL', 'BL-C2', '2026-11-15', 'customs-clearance')`,
	); err != nil {
		t.Fatalf("seed shipments: %v", err)
	}
	if _, err := rt.DB.DB().Exec(
		`INSERT INTO exception_cases (id, exception_type, entity_type, entity_id, status, priority, sla_status, lifecycle_generation, created_at, updated_at)
		 VALUES ('ex-cmp-1', 'OVERDUE_ETA', 'shipment', 'shp-cmp-1', 'OPEN', 'LOW', 'BREACHED', 1, '2026-11-20 00:00:00', '2026-11-20 00:00:00'),
		        ('ex-cmp-2', 'MISSING_BOE', 'shipment', 'shp-cmp-2', 'OPEN', 'MEDIUM', 'BREACHED', 1, '2026-11-20 00:00:00', '2026-11-20 00:00:00')`,
	); err != nil {
		t.Fatalf("seed exceptions: %v", err)
	}

	// Rule A v1: AUTO_ASSIGN, priority 20, condition status=='OPEN'
	rA, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Compare Target Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            20,
		Enabled:             true,
		ChangeReason:        "v1 low risk",
	})
	if err != nil {
		t.Fatalf("SaveRule v1: %v", err)
	}
	v1ID := rA.LatestVersionID
	promoteRuleToProdForTest(t, rt, rA.RuleID, v1ID)

	// Rule A v2: PRIORITY_ADJUST (or escalate DELETE), priority 90, wildcard condition
	rAv2, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleID:              rA.RuleID,
		RuleName:            "Compare Target Rule",
		RuleType:            "PRIORITY_ADJUST",
		ConditionExpression: "{}",
		ActionType:          "PRIORITY_ADJUST",
		Priority:            90,
		Enabled:             true,
		ChangeReason:        "v2 priority adjust wildcard",
	})
	if err != nil {
		t.Fatalf("SaveRule v2: %v", err)
	}
	v2ID := rAv2.LatestVersionID

	// Other rule for cross-rule ownership rejection check
	rOther, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Foreign Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            10,
		Enabled:             true,
		ChangeReason:        "foreign",
	})
	if err != nil {
		t.Fatalf("SaveRule foreign: %v", err)
	}

	// Cross-rule reference MUST be rejected
	if _, err := rt.Workflow.CompareRuleVersions(rA.RuleID, v1ID, rOther.LatestVersionID); err == nil {
		t.Fatalf("expected cross-rule CompareRuleVersions to fail ownership validation")
	}

	// Record DB counts prior to CompareRuleVersions to prove zero-mutation
	countTable := func(tbl string) int {
		var c int
		if qErr := rt.DB.DB().QueryRow(`SELECT COUNT(*) FROM ` + tbl).Scan(&c); qErr != nil {
			t.Fatalf("count %s: %v", tbl, qErr)
		}
		return c
	}
	tables := []string{
		"exception_cases",
		"workflow_automation_log",
		"workflow_rule_action_idempotency",
		"exception_lifecycle_events",
		"workflow_environment_deployment_log",
		"canary_rule_deployment",
		"workflow_rule_versions",
		"audit_logs",
	}
	beforeCounts := map[string]int{}
	for _, tbl := range tables {
		beforeCounts[tbl] = countTable(tbl)
	}

	cmp, err := rt.Workflow.CompareRuleVersions(rA.RuleID, v1ID, v2ID)
	if err != nil {
		t.Fatalf("CompareRuleVersions: %v", err)
	}

	// Verify all compared fields and deltas
	if cmp.RuleID != rA.RuleID || cmp.Base.VersionID != v1ID || cmp.Target.VersionID != v2ID {
		t.Fatalf("unexpected comparison header: %+v", cmp)
	}
	if cmp.Base.ActionType != "AUTO_ASSIGN" || cmp.Target.ActionType != "PRIORITY_ADJUST" {
		t.Fatalf("expected AUTO_ASSIGN vs PRIORITY_ADJUST, got %s vs %s", cmp.Base.ActionType, cmp.Target.ActionType)
	}
	if cmp.Base.Priority != 20 || cmp.Target.Priority != 90 {
		t.Fatalf("expected priority 20 vs 90, got %d vs %d", cmp.Base.Priority, cmp.Target.Priority)
	}
	if cmp.Base.DefinitionSHA256 == cmp.Target.DefinitionSHA256 {
		t.Fatalf("expected different definition SHA256 hashes")
	}
	if cmp.Base.ApprovalStatus != "APPROVED" || cmp.Target.ApprovalStatus != "PENDING" {
		t.Fatalf("expected approval APPROVED vs PENDING, got %s vs %s", cmp.Base.ApprovalStatus, cmp.Target.ApprovalStatus)
	}
	if !strings.Contains(cmp.Base.DeploymentStatus, "DEPLOYED") || cmp.Target.DeploymentStatus != "STAGED" {
		t.Fatalf("expected deployment DEPLOYED vs STAGED, got %s vs %s", cmp.Base.DeploymentStatus, cmp.Target.DeploymentStatus)
	}
	if !cmp.RiskLevelChanged || cmp.RiskScoreDelta <= 0 {
		t.Fatalf("expected positive risk delta & level change, got %+v", cmp)
	}
	if len(cmp.FieldDiffs) != 9 || len(cmp.ImportantChanges) == 0 {
		t.Fatalf("expected 9 field diffs and non-empty importantChanges, got %d / %d", len(cmp.FieldDiffs), len(cmp.ImportantChanges))
	}
	if cmp.DryRunImpact.BaseMatchingRecords == 0 || cmp.DryRunImpact.TargetMatchingRecords == 0 {
		t.Fatalf("expected dry-run comparison to match open cases under equivalent assumptions, got %+v", cmp.DryRunImpact)
	}

	// Confirm ZERO database writes during CompareRuleVersions
	for _, tbl := range tables {
		if after := countTable(tbl); after != beforeCounts[tbl] {
			t.Fatalf("CompareRuleVersions mutated table %s: before=%d after=%d", tbl, beforeCounts[tbl], after)
		}
	}

	// DB immutability trigger must block direct update, and even if trigger is dropped out-of-band,
	// tampered checksum must still cause CompareRuleVersions to reject.
	if _, err := rt.DB.DB().Exec(`UPDATE workflow_rule_versions SET definition_sha256 = 'deadbeef0000' WHERE version_id = ?`, v2ID); err == nil {
		t.Fatalf("expected DB immutability trigger to block direct definition_sha256 update")
	}
	if _, err := rt.DB.DB().Exec(`DROP TRIGGER trg_prevent_update_workflow_rule_versions_definition`); err != nil {
		t.Fatalf("drop trigger for tamper simulation: %v", err)
	}
	if _, err := rt.DB.DB().Exec(`UPDATE workflow_rule_versions SET definition_sha256 = 'deadbeef0000' WHERE version_id = ?`, v2ID); err != nil {
		t.Fatalf("simulate out-of-band tamper: %v", err)
	}
	if _, err := rt.Workflow.CompareRuleVersions(rA.RuleID, v1ID, v2ID); err == nil {
		t.Fatalf("expected tampered checksum on target version to fail CompareRuleVersions")
	}

	if _, err := rt.Workflow.GetDeploymentHealthOverview(); err != nil {
		t.Fatalf("GetDeploymentHealthOverview for admin failed: %v", err)
	}
	rt.Security.Logout()
	if _, err := rt.Workflow.GetDeploymentHealthOverview(); err == nil {
		t.Fatalf("expected unauthenticated GetDeploymentHealthOverview to be rejected")
	}
	if _, err := rt.Workflow.CompareRuleVersions(rA.RuleID, v1ID, v1ID); err == nil {
		t.Fatalf("expected auth/permission rejection when logged out")
	}
}

func TestPhase3BLargeHistoryQueryPerformance(t *testing.T) {
	rt := setupWorkflowEnv(t)

	const numRules = 40
	const metricsPerRule = 50
	const deployLogsTotal = 2000

	tx, err := rt.DB.DB().Begin()
	if err != nil {
		t.Fatalf("begin seed tx: %v", err)
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	for i := 0; i < numRules; i++ {
		rID := fmt.Sprintf("perf-rule-%03d", i)
		vID := rID + "-v1"
		_, _ = tx.Exec(
			`INSERT INTO workflow_decision_rules (rule_id, rule_name, rule_type, condition_expression, action_type, priority, enabled, tenant_id, created_at, updated_at)
			 VALUES (?, ?, 'AUTO_ASSIGN', 'status==OPEN', 'AUTO_ASSIGN', 10, 1, 'tenant-default', ?, ?)`,
			rID, fmt.Sprintf("Perf Rule %03d", i), nowStr, nowStr,
		)
		_, _ = tx.Exec(
			`INSERT INTO workflow_rule_versions (version_id, rule_id, tenant_id, environment_id, version_number, rule_definition_json, definition_sha256, created_by, created_at, is_active, change_reason)
			 VALUES (?, ?, 'tenant-default', 'env-prod', 1, '{"ruleName":"Perf","ruleType":"AUTO_ASSIGN","conditionExpression":"status==OPEN","actionType":"AUTO_ASSIGN","priority":10,"enabled":true}', '', 'system', ?, 1, 'seed')`,
			vID, rID, nowStr,
		)
		_, _ = tx.Exec(
			`INSERT INTO workflow_environment_deployment_log (deployment_id, tenant_id, environment_id, rule_id, version_id, status, timestamp, details_json)
			 VALUES (?, 'tenant-default', 'env-prod', ?, ?, 'DEPLOYED', ?, '{}')`,
			fmt.Sprintf("pdep-%03d", i), rID, vID, nowStr,
		)
		for m := 0; m < metricsPerRule; m++ {
			snapAt := time.Now().UTC().Add(-time.Duration(m) * time.Minute).Format("2006-01-02 15:04:05")
			_, _ = tx.Exec(
				`INSERT INTO rule_deployment_impact_metrics (id, rule_id, version_id, cohort, snapshot_at, attempted_count, success_count, failure_count, cost_units, avg_duration_ms, execution_count)
				 VALUES (?, ?, ?, 'control', ?, 1, 1, 0, 1.0, 2.0, 1)`,
				fmt.Sprintf("pm-%03d-%03d", i, m), rID, vID, snapAt,
			)
		}
	}
	for d := 0; d < deployLogsTotal; d++ {
		rIdx := d % numRules
		rID := fmt.Sprintf("perf-rule-%03d", rIdx)
		_, _ = tx.Exec(
			`INSERT INTO workflow_environment_deployment_log (deployment_id, tenant_id, environment_id, rule_id, version_id, status, timestamp, details_json)
			 VALUES (?, 'tenant-default', 'env-test', ?, ?, 'SUPERSEDED', ?, '{}')`,
			fmt.Sprintf("phist-%05d", d), rID, rID+"-v1", nowStr,
		)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed tx: %v", err)
	}

	start := time.Now()
	ov, err := rt.Workflow.GetDeploymentHealthOverview()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GetDeploymentHealthOverview scale test: %v", err)
	}
	if ov.TotalRules < numRules {
		t.Fatalf("expected >= %d rules, got %d", numRules, ov.TotalRules)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("GetDeploymentHealthOverview exceeded 2s budget on large history: took %v", elapsed)
	}
	_ = filepath.Clean("")
}
