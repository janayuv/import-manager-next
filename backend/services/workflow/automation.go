package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// ListRules returns all workflow decision rules with their latest version & canary state.
func (s *Service) ListRules() ([]models.WorkflowRule, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationOpsCenter); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT r.rule_id, r.rule_name, r.rule_type, r.condition_expression, r.action_type,
		        r.priority, r.enabled, r.tenant_id, r.created_at, r.updated_at,
		        COALESCE((SELECT v.version_id FROM workflow_rule_versions v WHERE v.rule_id = r.rule_id ORDER BY v.version_number DESC LIMIT 1), ''),
		        COALESCE((SELECT v.version_number FROM workflow_rule_versions v WHERE v.rule_id = r.rule_id ORDER BY v.version_number DESC LIMIT 1), 0),
		        COALESCE((SELECT c.sample_size_percentage FROM canary_rule_deployment c WHERE c.rule_id = r.rule_id AND c.deployment_status = 'ACTIVE' LIMIT 1), 0)
		 FROM workflow_decision_rules r ORDER BY r.priority DESC, r.updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.WorkflowRule
	for rows.Next() {
		var r models.WorkflowRule
		var en int
		if err := rows.Scan(&r.RuleID, &r.RuleName, &r.RuleType, &r.ConditionExpression, &r.ActionType,
			&r.Priority, &en, &r.TenantID, &r.CreatedAt, &r.UpdatedAt,
			&r.LatestVersionID, &r.LatestVersionNum, &r.CanaryPercent); err != nil {
			return nil, err
		}
		r.Enabled = en == 1
		out = append(out, r)
	}
	if out == nil {
		out = []models.WorkflowRule{}
	}
	return out, rows.Err()
}

// SaveRule creates or updates a decision rule and appends a sequential workflow_rule_versions entry.
func (s *Service) SaveRule(input models.SaveWorkflowRuleInput) (models.WorkflowRule, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.WorkflowRule{}, err
	}
	if strings.TrimSpace(input.RuleName) == "" || strings.TrimSpace(input.RuleType) == "" {
		return models.WorkflowRule{}, errors.New("ruleName and ruleType are required")
	}
	if strings.TrimSpace(input.ConditionExpression) == "" {
		input.ConditionExpression = "{}"
	}
	ruleID := strings.TrimSpace(input.RuleID)
	if ruleID == "" {
		ruleID = newID("rule")
	}
	enInt := 0
	if input.Enabled {
		enInt = 1
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	defJSON, _ := json.Marshal(input)

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var exists int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM workflow_decision_rules WHERE rule_id = ?`, ruleID).Scan(&exists)
		if exists == 0 {
			if _, err := tx.Exec(
				`INSERT INTO workflow_decision_rules (rule_id, rule_name, rule_type, condition_expression, action_type, priority, enabled, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				ruleID, strings.TrimSpace(input.RuleName), strings.TrimSpace(input.RuleType),
				input.ConditionExpression, strings.TrimSpace(input.ActionType), input.Priority, enInt, nowStr, nowStr,
			); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(
				`UPDATE workflow_decision_rules SET rule_name = ?, rule_type = ?, condition_expression = ?, action_type = ?, priority = ?, enabled = ?, updated_at = ? WHERE rule_id = ?`,
				strings.TrimSpace(input.RuleName), strings.TrimSpace(input.RuleType),
				input.ConditionExpression, strings.TrimSpace(input.ActionType), input.Priority, enInt, nowStr, ruleID,
			); err != nil {
				return err
			}
		}

		var nextVer int
		_ = tx.QueryRow(`SELECT COALESCE(MAX(version_number), 0) + 1 FROM workflow_rule_versions WHERE rule_id = ?`, ruleID).Scan(&nextVer)
		verID := fmt.Sprintf("%s-v%d", ruleID, nextVer)
		_, _ = tx.Exec(`UPDATE workflow_rule_versions SET is_active = 0 WHERE rule_id = ?`, ruleID)
		if _, err := tx.Exec(
			`INSERT INTO workflow_rule_versions (version_id, rule_id, version_number, rule_definition_json, created_by, created_at, is_active, change_reason)
			 VALUES (?, ?, ?, ?, ?, ?, 1, ?)`,
			verID, ruleID, nextVer, string(defJSON), sess.UserID, nowStr, input.ChangeReason,
		); err != nil {
			return err
		}

		_, _ = tx.Exec(
			`INSERT INTO workflow_rule_staging (staging_id, rule_id, version_id, staging_environment, created_at, status)
			 VALUES (?, ?, ?, 'env-dev', ?, 'TESTING')`,
			newID("stg"), ruleID, verID, nowStr,
		)
		_, _ = tx.Exec(
			`INSERT INTO workflow_rule_approvals (approval_id, rule_id, version_id, approval_status, requested_by, created_at)
			 VALUES (?, ?, ?, 'PENDING', ?, ?)`,
			newID("appr"), ruleID, verID, sess.UserID, nowStr,
		)
		return nil
	})
	if err != nil {
		return models.WorkflowRule{}, fmt.Errorf("save workflow rule: %w", err)
	}

	_ = s.audit.RecordTableAudit("workflow_decision_rules", ruleID, "UPSERT", sess.UserID, nil, input, nil)
	list, err := s.ListRules()
	if err != nil {
		return models.WorkflowRule{}, err
	}
	for _, r := range list {
		if r.RuleID == ruleID {
			return r, nil
		}
	}
	return models.WorkflowRule{}, errors.New("saved rule not found")
}

// ApproveRuleVersion marks a staged rule version APPROVED and READY for promotion.
func (s *Service) ApproveRuleVersion(ruleID, versionID string) error {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return err
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := s.db.Exec(
		`UPDATE workflow_rule_approvals SET approval_status = 'APPROVED', approved_by = ?, approval_time = ?
		 WHERE rule_id = ? AND version_id = ?`,
		sess.UserID, nowStr, ruleID, versionID,
	); err != nil {
		return err
	}
	_, _ = s.db.Exec(
		`UPDATE workflow_rule_staging SET status = 'READY' WHERE rule_id = ? AND version_id = ?`,
		ruleID, versionID,
	)
	_ = s.audit.RecordTableAudit("workflow_rule_approvals", versionID, "APPROVE", sess.UserID, nil, map[string]string{"ruleId": ruleID}, nil)
	return nil
}

// EvaluateRuleRisk calculates deterministic deployment blast-radius risk (<25 LOW, 25..49 MEDIUM, 50..74 HIGH, >=75 CRITICAL).
func (s *Service) EvaluateRuleRisk(ruleID, versionID string) (models.RuleRiskAssessment, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate); err != nil {
		return models.RuleRiskAssessment{}, err
	}
	var priority int
	var condExpr, actionType string
	if err := s.db.QueryRow(
		`SELECT priority, condition_expression, action_type FROM workflow_decision_rules WHERE rule_id = ?`,
		ruleID,
	).Scan(&priority, &condExpr, &actionType); err != nil {
		return models.RuleRiskAssessment{}, fmt.Errorf("rule %q not found: %w", ruleID, err)
	}

	score := 10.0
	var factors []string
	if priority >= 80 {
		score += 30
		factors = append(factors, "high-priority execution (>=80)")
	}
	if strings.Contains(strings.ToUpper(actionType), "DELETE") || strings.Contains(strings.ToUpper(actionType), "ESCALATE") || strings.Contains(strings.ToUpper(actionType), "FREEZE") {
		score += 40
		factors = append(factors, "mutating/high-impact action: "+actionType)
	}
	if strings.TrimSpace(condExpr) == "{}" || strings.Contains(condExpr, "*") {
		score += 20
		factors = append(factors, "wildcard/unfiltered scope condition")
	}
	if score > 100 {
		score = 100
	}

	level := "LOW"
	switch {
	case score >= 75:
		level = "CRITICAL"
	case score >= 50:
		level = "HIGH"
	case score >= 25:
		level = "MEDIUM"
	}

	return models.RuleRiskAssessment{
		RuleID:     ruleID,
		VersionID:  versionID,
		Score:      score,
		Level:      level,
		BlocksProd: level == "HIGH" || level == "CRITICAL",
		Factors:    factors,
	}, nil
}

// PromoteRule promotes a rule strictly along env-dev -> env-test -> env-prod after approval & risk gating.
func (s *Service) PromoteRule(ruleID, versionID, targetEnvID string, adminOverride bool) (models.RuleRiskAssessment, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.RuleRiskAssessment{}, err
	}
	targetEnv := strings.ToLower(strings.TrimSpace(targetEnvID))
	if targetEnv != "env-dev" && targetEnv != "env-test" && targetEnv != "env-prod" {
		return models.RuleRiskAssessment{}, fmt.Errorf("invalid target environment %q", targetEnvID)
	}

	// Verify sequential environment order: env-test requires prior env-dev, env-prod requires prior env-test
	if targetEnv == "env-test" {
		var devCnt int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM workflow_environment_deployment_log WHERE rule_id = ? AND version_id = ? AND environment_id = 'env-dev'`, ruleID, versionID).Scan(&devCnt)
		if devCnt == 0 {
			return models.RuleRiskAssessment{}, errors.New("rule version must be promoted to env-dev before env-test")
		}
	} else if targetEnv == "env-prod" {
		var testCnt int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM workflow_environment_deployment_log WHERE rule_id = ? AND version_id = ? AND environment_id = 'env-test'`, ruleID, versionID).Scan(&testCnt)
		if testCnt == 0 {
			return models.RuleRiskAssessment{}, errors.New("rule version must be promoted through env-test before env-prod")
		}
		var apprStatus string
		_ = s.db.QueryRow(`SELECT COALESCE(approval_status, 'PENDING') FROM workflow_rule_approvals WHERE rule_id = ? AND version_id = ?`, ruleID, versionID).Scan(&apprStatus)
		if apprStatus != "APPROVED" {
			return models.RuleRiskAssessment{}, errors.New("rule version requires APPROVED status before promoting to env-prod")
		}
	}

	risk, err := s.EvaluateRuleRisk(ruleID, versionID)
	if err != nil {
		return models.RuleRiskAssessment{}, err
	}
	if targetEnv == "env-prod" && risk.BlocksProd && !adminOverride {
		return risk, fmt.Errorf("PROD deployment blocked by %s risk score (%.0f); adminOverride required", risk.Level, risk.Score)
	}

	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	detailsJSON, _ := json.Marshal(map[string]any{"deployedBy": sess.UserID, "riskScore": risk.Score, "riskLevel": risk.Level})
	if _, err := s.db.Exec(
		`INSERT INTO workflow_environment_deployment_log (deployment_id, tenant_id, environment_id, rule_id, version_id, status, timestamp, details_json)
		 VALUES (?, 'tenant-default', ?, ?, ?, 'DEPLOYED', ?, ?)`,
		newID("dep"), targetEnv, ruleID, versionID, nowStr, string(detailsJSON),
	); err != nil {
		return models.RuleRiskAssessment{}, fmt.Errorf("log environment deployment: %w", err)
	}
	_, _ = s.db.Exec(
		`INSERT INTO workflow_rule_deployment_log (deployment_id, rule_id, version_id, deployed_by, deployment_status, deployment_time, notes)
		 VALUES (?, ?, ?, ?, 'DEPLOYED', ?, ?)`,
		newID("rdep"), ruleID, versionID, sess.UserID, nowStr, "Promoted to "+targetEnv,
	)

	_ = s.audit.RecordTableAudit("workflow_rule_deployment_log", versionID, "PROMOTE_"+strings.ToUpper(targetEnv), sess.UserID, nil, risk, nil)
	return risk, nil
}

// ConfigureCanary activates FNV-1a deterministic canary sampling for a workflow rule version.
func (s *Service) ConfigureCanary(ruleID, versionID string, samplePct float64) (models.CanaryStatus, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.CanaryStatus{}, err
	}
	if samplePct <= 0 || samplePct > 100 {
		return models.CanaryStatus{}, errors.New("canary sample percentage must be between 1 and 100")
	}
	id := newID("can")
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := s.db.Exec(
		`INSERT INTO canary_rule_deployment (id, tenant_id, rule_id, version_id, sample_size_percentage, deployment_status, created_at)
		 VALUES (?, 'tenant-default', ?, ?, ?, 'ACTIVE', ?)
		 ON CONFLICT(tenant_id, rule_id) DO UPDATE SET version_id = excluded.version_id, sample_size_percentage = excluded.sample_size_percentage, deployment_status = 'ACTIVE'`,
		id, ruleID, versionID, samplePct, nowStr,
	); err != nil {
		return models.CanaryStatus{}, err
	}
	_ = s.audit.RecordTableAudit("canary_rule_deployment", ruleID, "CANARY_SET", sess.UserID, nil, map[string]any{"samplePct": samplePct}, nil)
	return models.CanaryStatus{
		ID:                   id,
		RuleID:               ruleID,
		VersionID:            versionID,
		SampleSizePercentage: samplePct,
		DeploymentStatus:     "ACTIVE",
		CreatedAt:            nowStr,
	}, nil
}

// ShouldRouteCanary deterministically computes FNV-1a bucketing (< samplePct * 100 mod 10000).
func ShouldRouteCanary(ruleID, entityID string, samplePct float64) bool {
	if samplePct <= 0 {
		return false
	}
	if samplePct >= 100 {
		return true
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(ruleID + ":" + entityID))
	bucket := h.Sum64() % 10000
	threshold := uint64(samplePct * 100)
	return bucket < threshold
}

// RollbackRule reverts an automation rule to disabled/previous version and records rollback_action_log.
func (s *Service) RollbackRule(ruleID, reason string) error {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return err
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, _ = s.db.Exec(`UPDATE workflow_decision_rules SET enabled = 0, updated_at = ? WHERE rule_id = ?`, nowStr, ruleID)
	_, _ = s.db.Exec(`UPDATE canary_rule_deployment SET deployment_status = 'ROLLED_BACK' WHERE rule_id = ?`, ruleID)
	detailsJSON, _ := json.Marshal(map[string]string{"reason": reason})
	_, err = s.db.Exec(
		`INSERT INTO rollback_action_log (id, original_automation_id, rollback_type, target_entity, performed_by, performed_at, details_json)
		 VALUES (?, ?, 'RULE_ROLLBACK', ?, ?, ?, ?)`,
		newID("rbk"), ruleID, ruleID, sess.UserID, nowStr, string(detailsJSON),
	)
	if err == nil {
		_ = s.audit.RecordTableAudit("workflow_decision_rules", ruleID, "ROLLBACK", sess.UserID, nil, map[string]string{"reason": reason}, nil)
	}
	return err
}

// GetCostLimits returns current cycle safety guardrails from automation_cost_limits.
func (s *Service) GetCostLimits() (models.AutomationCostLimit, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate, security.PermAutomationOpsCenter); err != nil {
		return models.AutomationCostLimit{}, err
	}
	var out models.AutomationCostLimit
	err := s.db.QueryRow(
		`SELECT id, max_cost_units_per_cycle, max_execution_time_per_cycle_ms, max_records_processed_per_cycle, updated_at
		 FROM automation_cost_limits WHERE id = 'default'`,
	).Scan(&out.ID, &out.MaxCostUnitsPerCycle, &out.MaxExecutionTimePerCycleMs, &out.MaxRecordsProcessedCycle, &out.UpdatedAt)
	return out, err
}

// UpdateCostLimits updates automation cycle cost & execution caps.
func (s *Service) UpdateCostLimits(in models.AutomationCostLimit) (models.AutomationCostLimit, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.AutomationCostLimit{}, err
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := s.db.Exec(
		`UPDATE automation_cost_limits SET max_cost_units_per_cycle = ?, max_execution_time_per_cycle_ms = ?, max_records_processed_per_cycle = ?, updated_at = ? WHERE id = 'default'`,
		in.MaxCostUnitsPerCycle, in.MaxExecutionTimePerCycleMs, in.MaxRecordsProcessedCycle, nowStr,
	); err != nil {
		return models.AutomationCostLimit{}, err
	}
	_ = s.audit.RecordTableAudit("automation_cost_limits", "default", "UPDATE", sess.UserID, nil, in, nil)
	return s.GetCostLimits()
}
