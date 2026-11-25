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

func TestPhase3ACanaryProgressiveCohortsAndMonitoring(t *testing.T) {
	rt := setupWorkflowEnv(t)

	// 1 & 2. Progressive canary cohorts are strictly nested and deterministic
	stages := []float64{1, 5, 10, 25, 50, 100}
	prevCohort := map[string]bool{}
	for idx, pct := range stages {
		currCohort := map[string]bool{}
		for i := 0; i < 1000; i++ {
			eid := fmt.Sprintf("ex-cohort-%04d", i)
			first := workflow.ShouldRouteCanaryVersion("tenant-default", "rule-coh-1", "rule-coh-1-v2", eid, pct)
			second := workflow.ShouldRouteCanaryVersion("tenant-default", "rule-coh-1", "rule-coh-1-v2", eid, pct)
			if first != second {
				t.Fatalf("non-deterministic routing for %s at %.0f%%", eid, pct)
			}
			if first {
				currCohort[eid] = true
			}
		}
		// Every member of previous stage must remain in expanded cohort
		for k := range prevCohort {
			if !currCohort[k] {
				t.Fatalf("stage %.0f%% dropped entity %s from previous stage %.0f%%", pct, k, stages[idx-1])
			}
		}
		if pct == 100 && len(currCohort) != 1000 {
			t.Fatalf("expected 100%% cohort to include all 1000 entities, got %d", len(currCohort))
		}
		prevCohort = currCohort
	}

	// Non-stage canary percentage must be rejected
	rule, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase 3A Progressive Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            20,
		Enabled:             true,
		ChangeReason:        "v1 progressive",
	})
	if err != nil {
		t.Fatalf("SaveRule: %v", err)
	}
	if _, err := rt.Workflow.ConfigureCanary(rule.RuleID, rule.LatestVersionID, 17); err == nil {
		t.Fatalf("expected non-stage canary percentage 17%% to be rejected")
	}

	// Approve + promote through dev -> test, then advance progressive stages 1 -> 5 -> 10 -> 25 -> 50 -> 100
	if err := rt.Workflow.ApproveRuleVersion(rule.RuleID, rule.LatestVersionID); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, rule.LatestVersionID, "env-dev", false, ""); err != nil {
		t.Fatalf("Promote dev: %v", err)
	}
	if _, err := rt.Workflow.PromoteRule(rule.RuleID, rule.LatestVersionID, "env-test", false, ""); err != nil {
		t.Fatalf("Promote test: %v", err)
	}

	for i, expectedPct := range stages {
		st, err := rt.Workflow.AdvanceCanaryStage(rule.RuleID, rule.LatestVersionID)
		if err != nil {
			t.Fatalf("AdvanceCanaryStage step %d (%.0f%%): %v", i+1, expectedPct, err)
		}
		if st.SampleSizePercentage != expectedPct || st.RolloutStage != i+1 {
			t.Fatalf("expected stage %d (%.0f%%), got stage %d (%.0f%%)", i+1, expectedPct, st.RolloutStage, st.SampleSizePercentage)
		}
		if expectedPct == 100 && st.DeploymentStatus != "COMPLETED" {
			t.Fatalf("expected 100%% rollout to transition to COMPLETED, got %s", st.DeploymentStatus)
		}
	}

	// Verify 100% completion promoted rule into normal env-prod deployment
	var prodCnt int
	_ = rt.DB.DB().QueryRow(`SELECT COUNT(*) FROM workflow_environment_deployment_log WHERE rule_id=? AND version_id=? AND environment_id='env-prod' AND status='DEPLOYED'`,
		rule.RuleID, rule.LatestVersionID).Scan(&prodCnt)
	if prodCnt != 1 {
		t.Fatalf("expected 100%% completion to activate env-prod DEPLOYED, got %d", prodCnt)
	}
}

func TestPhase3ACanaryMonitoringAutoRollbackRecoveryAndAlerts(t *testing.T) {
	t.Setenv("IMPORT_MANAGER_DB_KEY", "p3a-test-key-0123456789abcdef0123")
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	dir := filepath.Join(t.TempDir(), "appdata")
	rt, err := bootstrap.Initialize([]string{"--recovery-mode"}, dir)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if _, err := rt.Security.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup failed: %v", err)
	}
	db := rt.DB.DB()

	// Configure thresholds: minSampleSize=5, maxFailureRate=0.20, maxCostIncreasePct=50, maxDurationIncreasePct=100
	th, err := rt.Workflow.UpdateRollbackThresholds(models.CanaryRollbackThreshold{
		MinSampleSize:           5,
		MaxFailureRate:          0.20,
		MaxCostIncreasePct:      50,
		MaxDurationIncreasePct:  100,
		MonitoringWindowMinutes: 60,
	})
	if err != nil || th.MinSampleSize != 5 {
		t.Fatalf("UpdateRollbackThresholds failed: %+v err=%v", th, err)
	}

	// Create and promote safe v1 (LOW risk) to env-prod
	r1, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Phase 3A Safe Base",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            15,
		Enabled:             true,
		ChangeReason:        "Safe v1",
	})
	if err != nil {
		t.Fatalf("SaveRule v1: %v", err)
	}
	v1ID := r1.LatestVersionID
	_ = rt.Workflow.ApproveRuleVersion(r1.RuleID, v1ID)
	_, _ = rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-dev", false, "")
	_, _ = rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-test", false, "")
	_, _ = rt.Workflow.PromoteRule(r1.RuleID, v1ID, "env-prod", false, "")

	// Stage, approve, and promote v2 to env-prod with 25% canary
	v2, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleID:              r1.RuleID,
		RuleName:            "Phase 3A Canary v2",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            25,
		Enabled:             true,
		ChangeReason:        "Canary v2",
	})
	if err != nil {
		t.Fatalf("SaveRule v2: %v", err)
	}
	v2ID := v2.LatestVersionID
	_ = rt.Workflow.ApproveRuleVersion(r1.RuleID, v2ID)
	_, _ = rt.Workflow.PromoteRule(r1.RuleID, v2ID, "env-dev", false, "")
	_, _ = rt.Workflow.PromoteRule(r1.RuleID, v2ID, "env-test", false, "")
	_, _ = rt.Workflow.PromoteRule(r1.RuleID, v2ID, "env-prod", false, "")
	if _, err := rt.Workflow.ConfigureCanary(r1.RuleID, v2ID, 25); err != nil {
		t.Fatalf("ConfigureCanary v2: %v", err)
	}

	// 4 & 7. Record insufficient sample size (2 attempts, 2 failures, PLUS 20 idempotency skips) -> MUST NOT count skips as failures or trigger rollback!
	t1 := time.Now().UTC().Format("2006-01-02 15:04:05")
	if err := rt.Workflow.RecordCanaryCohortMetrics(r1.RuleID, v2ID, t1,
		models.CanaryCohortMetrics{AttemptedCount: 2, SuccessCount: 0, FailureCount: 2, SkippedCount: 20, CostUnits: 4, AvgDurationMs: 10},
		models.CanaryCohortMetrics{AttemptedCount: 10, SuccessCount: 10, FailureCount: 0, SkippedCount: 0, CostUnits: 20, AvgDurationMs: 10},
	); err != nil {
		t.Fatalf("RecordCanaryCohortMetrics 1: %v", err)
	}
	mon1, err := rt.Workflow.GetCanaryMonitoring(r1.RuleID, v2ID)
	if err != nil || mon1.HealthStatus != "INSUFFICIENT_SAMPLES" || mon1.Canary.SkippedCount != 20 {
		t.Fatalf("expected INSUFFICIENT_SAMPLES with 20 skips, got %+v err=%v", mon1, err)
	}
	rbk1, err := rt.Workflow.EvaluateCanaryAndAutoRollback(r1.RuleID, v2ID)
	if err != nil || rbk1.Triggered {
		t.Fatalf("expected insufficient sample size NOT to trigger rollback, got %+v err=%v", rbk1, err)
	}

	// 3, 5, 8, 9. Add 8 more canary attempts (4 failures -> total 6/10 = 60% > 20%) while control stays healthy (0% failure) -> triggers Stage 1 disable + Stage 2 restore of v1!
	t2 := time.Now().UTC().Add(1 * time.Second).Format("2006-01-02 15:04:05")
	if err := rt.Workflow.RecordCanaryCohortMetrics(r1.RuleID, v2ID, t2,
		models.CanaryCohortMetrics{AttemptedCount: 8, SuccessCount: 4, FailureCount: 4, SkippedCount: 5, CostUnits: 16, AvgDurationMs: 12},
		models.CanaryCohortMetrics{AttemptedCount: 20, SuccessCount: 20, FailureCount: 0, SkippedCount: 0, CostUnits: 40, AvgDurationMs: 10},
	); err != nil {
		t.Fatalf("RecordCanaryCohortMetrics 2: %v", err)
	}
	rbk2, err := rt.Workflow.EvaluateCanaryAndAutoRollback(r1.RuleID, v2ID)
	if err != nil || !rbk2.Triggered || !rbk2.Stage1Disabled || !rbk2.Stage2Restored || rbk2.RestoredVersionID != v1ID {
		t.Fatalf("expected Stage 1 + Stage 2 restore to %s, got %+v err=%v", v1ID, rbk2, err)
	}

	// Confirm canary is ROLLED_BACK and v1 is active DEPLOYED in env-prod
	var canStat, prodVer string
	_ = db.QueryRow(`SELECT deployment_status FROM canary_rule_deployment WHERE rule_id=?`, r1.RuleID).Scan(&canStat)
	_ = db.QueryRow(`SELECT version_id FROM workflow_environment_deployment_log WHERE rule_id=? AND environment_id='env-prod' AND status='DEPLOYED'`, r1.RuleID).Scan(&prodVer)
	if canStat != "ROLLED_BACK" || prodVer != v1ID {
		t.Fatalf("expected canary ROLLED_BACK and prodVer=%s, got canStat=%s prodVer=%s", v1ID, canStat, prodVer)
	}

	// 11. Repeated evaluation MUST NOT create duplicate rollback operations
	rbkDup, err := rt.Workflow.EvaluateCanaryAndAutoRollback(r1.RuleID, v2ID)
	if err != nil || rbkDup.Triggered {
		t.Fatalf("expected repeated evaluation not to trigger duplicate rollback, got %+v err=%v", rbkDup, err)
	}
	var disableRows int
	_ = db.QueryRow(`SELECT COUNT(*) FROM rollback_action_log WHERE original_automation_id=? AND rollback_type='AUTO_ROLLBACK_DISABLE'`, r1.RuleID).Scan(&disableRows)
	if disableRows != 1 {
		t.Fatalf("expected exactly 1 AUTO_ROLLBACK_DISABLE log row, got %d", disableRows)
	}

	// 13. Alerts persisted & deduplicated; acknowledge works
	alerts, err := rt.Workflow.ListOperationalAlerts(false)
	if err != nil || len(alerts) == 0 {
		t.Fatalf("expected persisted operational alerts, got len=%d err=%v", len(alerts), err)
	}
	if err := rt.Workflow.AcknowledgeOperationalAlert(alerts[0].ID); err != nil {
		t.Fatalf("AcknowledgeOperationalAlert: %v", err)
	}

	// 6 & 10. Cost threshold breach with NO safe previous version leaves rule disabled and emits CRITICAL AUTO_ROLLBACK_NO_SAFE_VERSION alert
	rNoSafe, err := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "No Safe Prior Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            10,
		Enabled:             true,
		ChangeReason:        "Only v1",
	})
	if err != nil {
		t.Fatalf("SaveRule rNoSafe: %v", err)
	}
	_ = rt.Workflow.ApproveRuleVersion(rNoSafe.RuleID, rNoSafe.LatestVersionID)
	_, _ = rt.Workflow.PromoteRule(rNoSafe.RuleID, rNoSafe.LatestVersionID, "env-dev", false, "")
	_, _ = rt.Workflow.PromoteRule(rNoSafe.RuleID, rNoSafe.LatestVersionID, "env-test", false, "")
	_, _ = rt.Workflow.PromoteRule(rNoSafe.RuleID, rNoSafe.LatestVersionID, "env-prod", false, "")
	_, _ = rt.Workflow.ConfigureCanary(rNoSafe.RuleID, rNoSafe.LatestVersionID, 10)

	// Cost 10.0/exec vs control 2.0/exec = +400% > 50% threshold
	if err := rt.Workflow.RecordCanaryCohortMetrics(rNoSafe.RuleID, rNoSafe.LatestVersionID, t2,
		models.CanaryCohortMetrics{AttemptedCount: 6, SuccessCount: 6, FailureCount: 0, CostUnits: 60, AvgDurationMs: 10},
		models.CanaryCohortMetrics{AttemptedCount: 10, SuccessCount: 10, FailureCount: 0, CostUnits: 20, AvgDurationMs: 10},
	); err != nil {
		t.Fatalf("RecordCanaryCohortMetrics cost: %v", err)
	}
	rbkCost, err := rt.Workflow.EvaluateCanaryAndAutoRollback(rNoSafe.RuleID, rNoSafe.LatestVersionID)
	if err != nil || !rbkCost.Triggered || !rbkCost.Stage1Disabled || rbkCost.Stage2Restored {
		t.Fatalf("expected cost breach to Stage1 disable without Stage2 restore, got %+v err=%v", rbkCost, err)
	}
	var noSafeEn int
	_ = db.QueryRow(`SELECT enabled FROM workflow_decision_rules WHERE rule_id=?`, rNoSafe.RuleID).Scan(&noSafeEn)
	if noSafeEn != 0 {
		t.Fatalf("expected rule with no safe previous version to remain disabled, got enabled=%d", noSafeEn)
	}
	var critNoSafe int
	_ = db.QueryRow(`SELECT COUNT(*) FROM automation_stability_alert WHERE rule_id=? AND alert_type='AUTO_ROLLBACK_NO_SAFE_VERSION' AND severity='CRITICAL'`, rNoSafe.RuleID).Scan(&critNoSafe)
	if critNoSafe != 1 {
		t.Fatalf("expected CRITICAL AUTO_ROLLBACK_NO_SAFE_VERSION alert, got %d", critNoSafe)
	}

	// 12. Deployment freeze prevents Stage 2 automatic restoration
	rFrozen, _ := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleName:            "Frozen Recovery Rule",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            10,
		Enabled:             true,
		ChangeReason:        "Base v1",
	})
	_ = rt.Workflow.ApproveRuleVersion(rFrozen.RuleID, rFrozen.LatestVersionID)
	_, _ = rt.Workflow.PromoteRule(rFrozen.RuleID, rFrozen.LatestVersionID, "env-dev", false, "")
	_, _ = rt.Workflow.PromoteRule(rFrozen.RuleID, rFrozen.LatestVersionID, "env-test", false, "")
	_, _ = rt.Workflow.PromoteRule(rFrozen.RuleID, rFrozen.LatestVersionID, "env-prod", false, "")

	rFrozenV2, _ := rt.Workflow.SaveRule(models.SaveWorkflowRuleInput{
		RuleID:              rFrozen.RuleID,
		RuleName:            "Frozen Recovery v2",
		RuleType:            "AUTO_ASSIGN",
		ConditionExpression: "status == 'OPEN'",
		ActionType:          "AUTO_ASSIGN",
		Priority:            12,
		Enabled:             true,
		ChangeReason:        "Canary v2",
	})
	_ = rt.Workflow.ApproveRuleVersion(rFrozen.RuleID, rFrozenV2.LatestVersionID)
	_, _ = rt.Workflow.PromoteRule(rFrozen.RuleID, rFrozenV2.LatestVersionID, "env-dev", false, "")
	_, _ = rt.Workflow.PromoteRule(rFrozen.RuleID, rFrozenV2.LatestVersionID, "env-test", false, "")
	_, _ = rt.Workflow.PromoteRule(rFrozen.RuleID, rFrozenV2.LatestVersionID, "env-prod", false, "")
	_, _ = rt.Workflow.ConfigureCanary(rFrozen.RuleID, rFrozenV2.LatestVersionID, 10)
	if err := rt.Workflow.RecordCanaryCohortMetrics(rFrozen.RuleID, rFrozenV2.LatestVersionID, t2,
		models.CanaryCohortMetrics{AttemptedCount: 6, SuccessCount: 1, FailureCount: 5, CostUnits: 12, AvgDurationMs: 10},
		models.CanaryCohortMetrics{AttemptedCount: 10, SuccessCount: 10, FailureCount: 0, CostUnits: 20, AvgDurationMs: 10},
	); err != nil {
		t.Fatalf("RecordCanaryCohortMetrics frozen: %v", err)
	}
	_, _ = db.Exec(`UPDATE app_metadata SET value = '1' WHERE key = 'workflow_rule_deployment_frozen'`)
	rbkFrozen, err := rt.Workflow.EvaluateCanaryAndAutoRollback(rFrozen.RuleID, rFrozenV2.LatestVersionID)
	if err != nil || !rbkFrozen.Triggered || !rbkFrozen.Stage1Disabled || rbkFrozen.Stage2Restored {
		t.Fatalf("expected deployment freeze to block Stage 2 restore, got %+v err=%v", rbkFrozen, err)
	}

	// 14. Restart preserves rollout, monitoring, thresholds, and alert state
	if err := rt.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	rt2, err := bootstrap.Initialize([]string{"--recovery-mode"}, dir)
	if err != nil {
		t.Fatalf("bootstrap 2 failed: %v", err)
	}
	defer rt2.Shutdown()
	if _, err := rt2.Security.Authenticate("admin", "StrongPass1!", false); err != nil {
		t.Fatalf("re-auth failed: %v", err)
	}
	th2, err := rt2.Workflow.GetRollbackThresholds()
	if err != nil || th2.MinSampleSize != 5 {
		t.Fatalf("expected persisted thresholds minSampleSize=5 after restart, got %+v err=%v", th2, err)
	}
	monAfter, err := rt2.Workflow.GetCanaryMonitoring(r1.RuleID, v2ID)
	if err != nil || monAfter.Canary.AttemptedCount != 10 || monAfter.Canary.SkippedCount != 25 || !strings.EqualFold(monAfter.HealthStatus, "ROLLED_BACK") {
		t.Fatalf("expected persisted monitoring after restart, got %+v err=%v", monAfter, err)
	}
	openAlertsAfter, err := rt2.Workflow.ListOperationalAlerts(false)
	if err != nil || len(openAlertsAfter) == 0 {
		t.Fatalf("expected persisted open alerts after restart, got %d err=%v", len(openAlertsAfter), err)
	}
}
