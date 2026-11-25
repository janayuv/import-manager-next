package workflow

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const defaultTenantID = "tenant-default"

type sqlQueryable interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type immutableRuleDefinition struct {
	RuleName            string `json:"ruleName"`
	RuleType            string `json:"ruleType"`
	ConditionExpression string `json:"conditionExpression"`
	ActionType          string `json:"actionType"`
	Priority            int    `json:"priority"`
	Enabled             bool   `json:"enabled"`
}

type rawRuleDefinition struct {
	RuleName            string          `json:"ruleName"`
	RuleType            string          `json:"ruleType"`
	ConditionExpression string          `json:"conditionExpression"`
	ActionType          string          `json:"actionType"`
	Priority            int             `json:"priority"`
	Enabled             json.RawMessage `json:"enabled"`
}

func computeDefinitionSHA256(rawJSON string) string {
	sum := sha256.Sum256([]byte(rawJSON))
	return hex.EncodeToString(sum[:])
}

func parseFlexibleBool(raw json.RawMessage) (bool, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false, nil
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n != 0, nil
	}
	return false, fmt.Errorf("invalid enabled value %s", trimmed)
}

func decodeAndVerifyRuleDefinition(rawJSON, expectedSHA string) (immutableRuleDefinition, string, error) {
	actualSHA := computeDefinitionSHA256(rawJSON)
	if expectedSHA != "" && !strings.EqualFold(expectedSHA, actualSHA) {
		return immutableRuleDefinition{}, actualSHA, fmt.Errorf("rule version checksum mismatch (expected %s, got %s)", expectedSHA, actualSHA)
	}
	var raw rawRuleDefinition
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return immutableRuleDefinition{}, actualSHA, fmt.Errorf("parse rule definition json: %w", err)
	}
	en, err := parseFlexibleBool(raw.Enabled)
	if err != nil {
		return immutableRuleDefinition{}, actualSHA, err
	}
	def := immutableRuleDefinition{
		RuleName:            strings.TrimSpace(raw.RuleName),
		RuleType:            strings.TrimSpace(raw.RuleType),
		ConditionExpression: strings.TrimSpace(raw.ConditionExpression),
		ActionType:          strings.TrimSpace(raw.ActionType),
		Priority:            raw.Priority,
		Enabled:             en,
	}
	if def.ConditionExpression == "" {
		def.ConditionExpression = "{}"
	}
	return def, actualSHA, nil
}

func loadRuleVersionQuery(ctx context.Context, q sqlQueryable, ruleID, versionID string) (models.WorkflowRuleVersion, error) {
	ruleID = strings.TrimSpace(ruleID)
	versionID = strings.TrimSpace(versionID)
	if ruleID == "" || versionID == "" {
		return models.WorkflowRuleVersion{}, errors.New("ruleId and versionId are required")
	}

	var v models.WorkflowRuleVersion
	var isAct int
	err := q.QueryRowContext(ctx,
		`SELECT v.version_id, v.rule_id, v.tenant_id, v.environment_id, v.version_number,
		        v.rule_definition_json, COALESCE(v.definition_sha256, ''), v.created_by, v.created_at,
		        v.is_active, COALESCE(v.change_reason, ''),
		        COALESCE(a.approval_status, ''), COALESCE(a.approved_by, ''), COALESCE(a.approval_time, ''), COALESCE(a.approved_definition_sha256, '')
		 FROM workflow_rule_versions v
		 LEFT JOIN workflow_rule_approvals a ON a.rule_id = v.rule_id AND a.version_id = v.version_id
		 WHERE v.rule_id = ? AND v.version_id = ?
		 ORDER BY datetime(a.created_at) DESC, a.rowid DESC
		 LIMIT 1`,
		ruleID, versionID,
	).Scan(
		&v.VersionID, &v.RuleID, &v.TenantID, &v.EnvironmentID, &v.VersionNumber,
		&v.RuleDefinitionJSON, &v.DefinitionSHA256, &v.CreatedBy, &v.CreatedAt,
		&isAct, &v.ChangeReason,
		&v.ApprovalStatus, &v.ApprovedBy, &v.ApprovedAt, &v.ApprovedSHA256,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.WorkflowRuleVersion{}, fmt.Errorf("version %q not found for rule %q", versionID, ruleID)
		}
		return models.WorkflowRuleVersion{}, fmt.Errorf("load rule version: %w", err)
	}
	v.IsActive = isAct == 1

	if v.DefinitionSHA256 == "" && !strings.EqualFold(v.CreatedBy, "system") {
		return models.WorkflowRuleVersion{}, fmt.Errorf("integrity error: rule version %q is missing definition_sha256", versionID)
	}
	def, actualSHA, err := decodeAndVerifyRuleDefinition(v.RuleDefinitionJSON, v.DefinitionSHA256)
	if err != nil {
		return models.WorkflowRuleVersion{}, err
	}
	if v.DefinitionSHA256 == "" {
		v.DefinitionSHA256 = actualSHA
	}
	v.RuleName = def.RuleName
	v.RuleType = def.RuleType
	v.ConditionExpression = def.ConditionExpression
	v.ActionType = def.ActionType
	v.Priority = def.Priority
	v.Enabled = def.Enabled
	return v, nil
}

// ListRules returns all workflow decision rules with their latest version, approval state, prod version, current environment, & canary state.
func (s *Service) ListRules() ([]models.WorkflowRule, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationOpsCenter); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT r.rule_id, r.rule_name, r.rule_type, r.condition_expression, r.action_type,
		        r.priority, r.enabled, r.tenant_id, r.created_at, r.updated_at,
		        COALESCE((SELECT v.version_id FROM workflow_rule_versions v WHERE v.rule_id = r.rule_id ORDER BY v.version_number DESC LIMIT 1), ''),
		        COALESCE((SELECT v.version_number FROM workflow_rule_versions v WHERE v.rule_id = r.rule_id ORDER BY v.version_number DESC LIMIT 1), 0),
		        COALESCE((SELECT v.definition_sha256 FROM workflow_rule_versions v WHERE v.rule_id = r.rule_id ORDER BY v.version_number DESC LIMIT 1), ''),
		        COALESCE((SELECT a.approval_status FROM workflow_rule_approvals a
		                  WHERE a.rule_id = r.rule_id
		                    AND a.version_id = (SELECT v2.version_id FROM workflow_rule_versions v2 WHERE v2.rule_id = r.rule_id ORDER BY v2.version_number DESC LIMIT 1)
		                  ORDER BY datetime(a.created_at) DESC, a.rowid DESC LIMIT 1), ''),
		        COALESCE((SELECT d.version_id FROM workflow_environment_deployment_log d WHERE d.rule_id = r.rule_id AND d.environment_id = 'env-prod' AND d.status = 'DEPLOYED' ORDER BY datetime(d.timestamp) DESC, d.rowid DESC LIMIT 1), ''),
		        COALESCE((SELECT pv.version_number FROM workflow_environment_deployment_log d
		                  JOIN workflow_rule_versions pv ON pv.version_id = d.version_id AND pv.rule_id = d.rule_id
		                  WHERE d.rule_id = r.rule_id AND d.environment_id = 'env-prod' AND d.status = 'DEPLOYED'
		                  ORDER BY datetime(d.timestamp) DESC, d.rowid DESC LIMIT 1), 0),
		        COALESCE((SELECT d.environment_id FROM workflow_environment_deployment_log d WHERE d.rule_id = r.rule_id AND d.status = 'DEPLOYED'
		                  ORDER BY CASE d.environment_id WHEN 'env-prod' THEN 3 WHEN 'env-test' THEN 2 WHEN 'env-dev' THEN 1 ELSE 0 END DESC, datetime(d.timestamp) DESC, d.rowid DESC LIMIT 1), ''),
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
			&r.LatestVersionID, &r.LatestVersionNum, &r.LatestVersionSHA256, &r.LatestApprovalStatus,
			&r.ProdVersionID, &r.ProdVersionNum, &r.CurrentEnv, &r.CanaryPercent); err != nil {
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

// GetRuleVersion loads and verifies an immutable historical rule version snapshot.
func (s *Service) GetRuleVersion(ruleID, versionID string) (models.WorkflowRuleVersion, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate, security.PermAutomationOpsCenter); err != nil {
		return models.WorkflowRuleVersion{}, err
	}
	return loadRuleVersionQuery(context.Background(), s.db, ruleID, versionID)
}

// ListRuleVersions returns all historical immutable versions for a rule, newest first.
func (s *Service) ListRuleVersions(ruleID string) ([]models.WorkflowRuleVersion, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate, security.PermAutomationOpsCenter); err != nil {
		return nil, err
	}
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return nil, errors.New("ruleId is required")
	}
	rows, err := s.db.Query(
		`SELECT v.version_id
		 FROM workflow_rule_versions v
		 WHERE v.rule_id = ?
		 ORDER BY v.version_number DESC`,
		ruleID,
	)
	if err != nil {
		return nil, err
	}
	var verIDs []string
	for rows.Next() {
		var vid string
		if rows.Scan(&vid) == nil {
			verIDs = append(verIDs, vid)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	out := make([]models.WorkflowRuleVersion, 0, len(verIDs))
	ctx := context.Background()
	for _, vid := range verIDs {
		v, err := loadRuleVersionQuery(ctx, s.db, ruleID, vid)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// SaveRule creates or updates a decision rule and appends an immutable sequential workflow_rule_versions entry.
func (s *Service) SaveRule(input models.SaveWorkflowRuleInput) (models.WorkflowRule, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.WorkflowRule{}, err
	}
	ruleName := strings.TrimSpace(input.RuleName)
	ruleType := strings.TrimSpace(input.RuleType)
	actionType := strings.TrimSpace(input.ActionType)
	condExpr := strings.TrimSpace(input.ConditionExpression)
	changeReason := strings.TrimSpace(input.ChangeReason)
	if ruleName == "" || ruleType == "" {
		return models.WorkflowRule{}, errors.New("ruleName and ruleType are required")
	}
	if input.Priority < 0 {
		return models.WorkflowRule{}, errors.New("priority must be >= 0")
	}
	if condExpr == "" {
		condExpr = "{}"
	}
	if actionType == "" {
		actionType = ruleType
	}
	if changeReason == "" {
		changeReason = "Saved in Automation Rules Console"
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

	def := immutableRuleDefinition{
		RuleName:            ruleName,
		RuleType:            ruleType,
		ConditionExpression: condExpr,
		ActionType:          actionType,
		Priority:            input.Priority,
		Enabled:             input.Enabled,
	}
	defBytes, err := json.Marshal(def)
	if err != nil {
		return models.WorkflowRule{}, fmt.Errorf("marshal rule definition: %w", err)
	}
	defJSON := string(defBytes)
	defSHA := computeDefinitionSHA256(defJSON)

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM workflow_decision_rules WHERE rule_id = ?`, ruleID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			if _, err := tx.Exec(
				`INSERT INTO workflow_decision_rules (rule_id, rule_name, rule_type, condition_expression, action_type, priority, enabled, tenant_id, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				ruleID, ruleName, ruleType, condExpr, actionType, input.Priority, enInt, defaultTenantID, nowStr, nowStr,
			); err != nil {
				return err
			}
		} else {
			// Do not overwrite an active env-prod deployment's runtime specification; only update metadata timestamp
			// when a production version is already deployed, or update the unpromoted draft rule directly.
			var prodDeployed int
			_ = tx.QueryRow(
				`SELECT COUNT(*) FROM workflow_environment_deployment_log
				 WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND status = 'DEPLOYED'`,
				defaultTenantID, ruleID,
			).Scan(&prodDeployed)
			if prodDeployed == 0 {
				if _, err := tx.Exec(
					`UPDATE workflow_decision_rules SET rule_name = ?, rule_type = ?, condition_expression = ?, action_type = ?, priority = ?, enabled = ?, updated_at = ? WHERE rule_id = ?`,
					ruleName, ruleType, condExpr, actionType, input.Priority, enInt, nowStr, ruleID,
				); err != nil {
					return err
				}
			} else {
				if _, err := tx.Exec(
					`UPDATE workflow_decision_rules SET updated_at = ? WHERE rule_id = ?`,
					nowStr, ruleID,
				); err != nil {
					return err
				}
			}
		}

		var nextVer int
		if err := tx.QueryRow(`SELECT COALESCE(MAX(version_number), 0) + 1 FROM workflow_rule_versions WHERE rule_id = ?`, ruleID).Scan(&nextVer); err != nil {
			return err
		}
		verID := fmt.Sprintf("%s-v%d", ruleID, nextVer)
		if _, err := tx.Exec(`UPDATE workflow_rule_versions SET is_active = 0 WHERE rule_id = ? AND environment_id = 'env-dev'`, ruleID); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO workflow_rule_versions (version_id, rule_id, tenant_id, environment_id, version_number, rule_definition_json, definition_sha256, created_by, created_at, is_active, change_reason)
			 VALUES (?, ?, ?, 'env-dev', ?, ?, ?, ?, ?, 1, ?)`,
			verID, ruleID, defaultTenantID, nextVer, defJSON, defSHA, sess.UserID, nowStr, changeReason,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO workflow_rule_staging (staging_id, rule_id, version_id, staging_environment, created_at, status)
			 VALUES (?, ?, ?, 'env-dev', ?, 'TESTING')`,
			newID("stg"), ruleID, verID, nowStr,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO workflow_rule_approvals (approval_id, rule_id, version_id, approval_status, requested_by, created_at, approved_definition_sha256)
			 VALUES (?, ?, ?, 'PENDING', ?, ?, '')`,
			newID("appr"), ruleID, verID, sess.UserID, nowStr,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO workflow_rule_change_log (change_id, rule_id, changed_by, change_type, previous_value, new_value, changed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			newID("chg"), ruleID, sess.UserID, "VERSION_STAGED", "", defJSON, nowStr,
		); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "workflow_decision_rules", ruleID, "UPSERT", sess.UserID, nil, map[string]any{
				"versionId":        verID,
				"versionNumber":    nextVer,
				"definitionSha256": defSHA,
				"changeReason":     changeReason,
			}, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.WorkflowRule{}, fmt.Errorf("save workflow rule: %w", err)
	}

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

// ApproveRuleVersion marks a staged rule version APPROVED and READY for promotion within an atomic transaction.
// When multiple admin/manager approvers exist, separation of duties prevents the creator from self-approving.
func (s *Service) ApproveRuleVersion(ruleID, versionID string) error {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return err
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")

	return transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		ver, err := loadRuleVersionQuery(context.Background(), tx, ruleID, versionID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`UPDATE workflow_rule_versions SET definition_sha256 = ? WHERE rule_id = ? AND version_id = ? AND COALESCE(definition_sha256, '') = ''`,
			ver.DefinitionSHA256, ruleID, versionID,
		); err != nil {
			return err
		}

		var approvalID, approvalStatus, requestedBy string
		err = tx.QueryRow(
			`SELECT approval_id, approval_status, COALESCE(requested_by, '')
			 FROM workflow_rule_approvals
			 WHERE rule_id = ? AND version_id = ?
			 ORDER BY datetime(created_at) DESC, rowid DESC
			 LIMIT 1`,
			ruleID, versionID,
		).Scan(&approvalID, &approvalStatus, &requestedBy)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("no approval request found for rule %q version %q", ruleID, versionID)
			}
			return err
		}
		if approvalStatus != "PENDING" {
			return fmt.Errorf("rule version %q already has approval status %s", versionID, approvalStatus)
		}

		var otherApprovers int
		_ = tx.QueryRow(
			`SELECT COUNT(DISTINCT LOWER(user_id))
			 FROM user_roles
			 WHERE LOWER(role) IN ('administrator', 'manager') AND LOWER(user_id) != LOWER(?)`,
			sess.UserID,
		).Scan(&otherApprovers)
		if otherApprovers > 0 && requestedBy != "" && strings.EqualFold(requestedBy, sess.UserID) {
			return errors.New("separation of duties: rule creator cannot approve their own version")
		}

		res, err := tx.Exec(
			`UPDATE workflow_rule_approvals
			 SET approval_status = 'APPROVED', approved_by = ?, approval_time = ?, approved_definition_sha256 = ?
			 WHERE approval_id = ? AND approval_status = 'PENDING'`,
			sess.UserID, nowStr, ver.DefinitionSHA256, approvalID,
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return fmt.Errorf("expected exactly 1 pending approval updated for %q, updated %d", versionID, n)
		}

		if _, err := tx.Exec(
			`UPDATE workflow_rule_staging SET status = 'READY' WHERE rule_id = ? AND version_id = ?`,
			ruleID, versionID,
		); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "workflow_rule_approvals", versionID, "APPROVE", sess.UserID, nil, map[string]string{
				"ruleId":           ruleID,
				"approvalId":       approvalID,
				"definitionSha256": ver.DefinitionSHA256,
			}, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// EvaluateRuleRisk calculates deterministic deployment blast-radius risk strictly from the requested immutable version.
func (s *Service) EvaluateRuleRisk(ruleID, versionID string) (models.RuleRiskAssessment, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate); err != nil {
		return models.RuleRiskAssessment{}, err
	}
	return s.evaluateRuleRiskInternal(context.Background(), ruleID, versionID)
}

func (s *Service) evaluateRuleRiskInternal(ctx context.Context, ruleID, versionID string) (models.RuleRiskAssessment, error) {
	return evaluateRuleRiskQuery(ctx, s.db, ruleID, versionID)
}

func evaluateRuleRiskQuery(ctx context.Context, q sqlQueryable, ruleID, versionID string) (models.RuleRiskAssessment, error) {
	ver, err := loadRuleVersionQuery(ctx, q, ruleID, versionID)
	if err != nil {
		return models.RuleRiskAssessment{}, err
	}

	score := 10.0
	var factors []string
	if ver.Priority >= 80 {
		score += 30
		factors = append(factors, "high-priority execution (>=80)")
	}
	actionUpper := strings.ToUpper(ver.ActionType)
	if strings.Contains(actionUpper, "DELETE") || strings.Contains(actionUpper, "ESCALATE") || strings.Contains(actionUpper, "FREEZE") {
		score += 40
		factors = append(factors, "mutating/high-impact action: "+ver.ActionType)
	}
	if strings.TrimSpace(ver.ConditionExpression) == "{}" || strings.Contains(ver.ConditionExpression, "*") {
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
		RuleID:           ver.RuleID,
		VersionID:        ver.VersionID,
		DefinitionSHA256: ver.DefinitionSHA256,
		Score:            score,
		Level:            level,
		BlocksProd:       level == "HIGH" || level == "CRITICAL",
		Factors:          factors,
	}, nil
}

// PromoteRule promotes an immutable rule version strictly along env-dev -> env-test -> env-prod inside one atomic transaction.
func (s *Service) PromoteRule(ruleID, versionID, targetEnvID string, adminOverride bool, overrideReason string) (models.RuleRiskAssessment, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.RuleRiskAssessment{}, err
	}
	targetEnv := strings.ToLower(strings.TrimSpace(targetEnvID))
	if targetEnv != "env-dev" && targetEnv != "env-test" && targetEnv != "env-prod" {
		return models.RuleRiskAssessment{}, fmt.Errorf("invalid target environment %q", targetEnvID)
	}

	var risk models.RuleRiskAssessment
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var frozen string
		_ = tx.QueryRow(`SELECT COALESCE(value, '0') FROM app_metadata WHERE key = 'workflow_rule_deployment_frozen'`).Scan(&frozen)
		if frozen == "1" {
			return errors.New("workflow rule deployments are currently frozen by system policy")
		}

		ctx := context.Background()
		ver, err := loadRuleVersionQuery(ctx, tx, ruleID, versionID)
		if err != nil {
			return err
		}

		var alreadyDeployed int
		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM workflow_environment_deployment_log
			 WHERE tenant_id = ? AND environment_id = ? AND rule_id = ? AND version_id = ? AND status = 'DEPLOYED'`,
			ver.TenantID, targetEnv, ruleID, versionID,
		).Scan(&alreadyDeployed); err != nil {
			return err
		}
		if alreadyDeployed > 0 {
			return fmt.Errorf("rule version %q is already deployed to %s", versionID, targetEnv)
		}

		if targetEnv == "env-test" {
			var devCnt int
			_ = tx.QueryRow(
				`SELECT COUNT(*) FROM workflow_environment_deployment_log
				 WHERE tenant_id = ? AND rule_id = ? AND version_id = ? AND environment_id = 'env-dev' AND status IN ('DEPLOYED', 'SUPERSEDED')`,
				ver.TenantID, ruleID, versionID,
			).Scan(&devCnt)
			if devCnt == 0 {
				return errors.New("rule version must be promoted to env-dev before env-test")
			}
		} else if targetEnv == "env-prod" {
			var testCnt int
			_ = tx.QueryRow(
				`SELECT COUNT(*) FROM workflow_environment_deployment_log
				 WHERE tenant_id = ? AND rule_id = ? AND version_id = ? AND environment_id = 'env-test' AND status IN ('DEPLOYED', 'SUPERSEDED')`,
				ver.TenantID, ruleID, versionID,
			).Scan(&testCnt)
			if testCnt == 0 {
				return errors.New("rule version must be promoted through env-test before env-prod")
			}
			var apprStatus, apprSHA string
			_ = tx.QueryRow(
				`SELECT COALESCE(approval_status, 'PENDING'), COALESCE(approved_definition_sha256, '')
				 FROM workflow_rule_approvals
				 WHERE rule_id = ? AND version_id = ?
				 ORDER BY datetime(created_at) DESC, rowid DESC LIMIT 1`,
				ruleID, versionID,
			).Scan(&apprStatus, &apprSHA)
			if apprStatus != "APPROVED" {
				return errors.New("rule version requires APPROVED status before promoting to env-prod")
			}
			if !strings.EqualFold(ver.CreatedBy, "system") && apprSHA == "" {
				return errors.New("stale or unbound approval: missing approved_definition_sha256")
			}
			if apprSHA != "" && !strings.EqualFold(apprSHA, ver.DefinitionSHA256) {
				return errors.New("stale approval: approved definition checksum does not match version checksum")
			}
		}

		risk, err = evaluateRuleRiskQuery(ctx, tx, ruleID, versionID)
		if err != nil {
			return err
		}

		prodSafety := "1"
		_ = tx.QueryRow(`SELECT COALESCE(value, '1') FROM app_metadata WHERE key = 'workflow_deploy_prod_safety_enforcement'`).Scan(&prodSafety)

		trimmedOverrideReason := strings.TrimSpace(overrideReason)
		if targetEnv == "env-prod" && prodSafety != "0" && risk.BlocksProd {
			if !adminOverride {
				return fmt.Errorf("PROD deployment blocked by %s risk score (%.0f); administrator override with reason required", risk.Level, risk.Score)
			}
			if !strings.EqualFold(sess.Role, string(security.RoleAdministrator)) {
				return errors.New("unauthorized production override: administrator role is required to override HIGH/CRITICAL risk")
			}
			if len(trimmedOverrideReason) < 5 {
				return errors.New("production risk override requires a documented reason (>= 5 characters)")
			}
		}

		nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")

		if _, err := tx.Exec(
			`UPDATE workflow_environment_deployment_log
			 SET status = 'SUPERSEDED'
			 WHERE tenant_id = ? AND environment_id = ? AND rule_id = ? AND status = 'DEPLOYED'`,
			ver.TenantID, targetEnv, ruleID,
		); err != nil {
			return err
		}

		detailsJSON, _ := json.Marshal(map[string]any{
			"deployedBy":       sess.UserID,
			"definitionSha256": ver.DefinitionSHA256,
			"riskScore":        risk.Score,
			"riskLevel":        risk.Level,
			"adminOverride":    adminOverride,
			"overrideReason":   trimmedOverrideReason,
		})
		if _, err := tx.Exec(
			`INSERT INTO workflow_environment_deployment_log (deployment_id, tenant_id, environment_id, rule_id, version_id, status, timestamp, details_json)
			 VALUES (?, ?, ?, ?, ?, 'DEPLOYED', ?, ?)`,
			newID("dep"), ver.TenantID, targetEnv, ruleID, versionID, nowStr, string(detailsJSON),
		); err != nil {
			return fmt.Errorf("log environment deployment: %w", err)
		}

		if _, err := tx.Exec(
			`INSERT INTO workflow_rule_deployment_log (deployment_id, rule_id, version_id, deployed_by, deployment_status, deployment_time, details_json)
			 VALUES (?, ?, ?, ?, 'DEPLOYED', ?, ?)`,
			newID("rdep"), ruleID, versionID, sess.UserID, nowStr, string(detailsJSON),
		); err != nil {
			return fmt.Errorf("log rule deployment: %w", err)
		}

		riskJSON, _ := json.Marshal(risk)
		safeInt := 1
		if risk.BlocksProd && !adminOverride {
			safeInt = 0
		}
		if _, err := tx.Exec(
			`INSERT INTO deployment_risk_assessment (assessment_id, assessed_at, rule_id, version_id, tenant_id, environment_id, risk_score, risk_level, safe_to_deploy, assessment_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(rule_id, version_id, tenant_id, environment_id) DO UPDATE SET
			   assessed_at = excluded.assessed_at,
			   risk_score = excluded.risk_score,
			   risk_level = excluded.risk_level,
			   safe_to_deploy = excluded.safe_to_deploy,
			   assessment_json = excluded.assessment_json`,
			newID("risk"), nowStr, ruleID, versionID, ver.TenantID, targetEnv, risk.Score, risk.Level, safeInt, string(riskJSON),
		); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO deployment_risk_timeline (id, deployment_time, risk_score, risk_level, environment_id, tenant_id, rule_id, version_id, result, details_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'DEPLOYED', ?)`,
			newID("rtml"), nowStr, risk.Score, risk.Level, targetEnv, ver.TenantID, ruleID, versionID, string(detailsJSON),
		); err != nil {
			return err
		}

		if targetEnv == "env-prod" {
			enInt := 0
			if ver.Enabled {
				enInt = 1
			}
			if _, err := tx.Exec(
				`UPDATE workflow_decision_rules
				 SET rule_name = ?, rule_type = ?, condition_expression = ?, action_type = ?, priority = ?, enabled = ?, updated_at = ?
				 WHERE rule_id = ?`,
				ver.RuleName, ver.RuleType, ver.ConditionExpression, ver.ActionType, ver.Priority, enInt, nowStr, ruleID,
			); err != nil {
				return err
			}
		}

		if s.audit != nil {
			auditPayload := map[string]any{
				"ruleId":           ruleID,
				"versionId":        versionID,
				"targetEnv":        targetEnv,
				"definitionSha256": ver.DefinitionSHA256,
				"risk":             risk,
				"adminOverride":    adminOverride,
				"overrideReason":   trimmedOverrideReason,
			}
			if err := s.audit.RecordTableAuditTx(tx, "workflow_rule_deployment_log", versionID, "PROMOTE_"+strings.ToUpper(targetEnv), sess.UserID, nil, auditPayload, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return risk, err
	}
	return risk, nil
}

// ConfigureCanary activates version-scoped FNV-1a deterministic progressive canary sampling (1, 5, 10, 25, 50, 100%) for a workflow rule version.
func (s *Service) ConfigureCanary(ruleID, versionID string, samplePct float64) (models.CanaryStatus, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.CanaryStatus{}, err
	}
	if math.IsNaN(samplePct) || math.IsInf(samplePct, 0) || samplePct <= 0 || samplePct > 100 {
		return models.CanaryStatus{}, errors.New("canary sample percentage must be between 1 and 100")
	}
	stage, ok := canaryStageForPct(samplePct)
	if !ok {
		return models.CanaryStatus{}, errors.New("canary sample percentage must be one of progressive stages: 1, 5, 10, 25, 50, 100")
	}

	id := newID("can")
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	status := "ACTIVE"
	completedAt := ""

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		ver, err := loadRuleVersionQuery(context.Background(), tx, ruleID, versionID)
		if err != nil {
			return err
		}
		if samplePct >= 100 {
			var prodCnt int
			_ = tx.QueryRow(
				`SELECT COUNT(*) FROM workflow_environment_deployment_log
				 WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND version_id = ? AND status = 'DEPLOYED'`,
				ver.TenantID, ruleID, versionID,
			).Scan(&prodCnt)
			if prodCnt > 0 {
				status = "COMPLETED"
				completedAt = nowStr
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO canary_rule_deployment (id, tenant_id, rule_id, version_id, sample_size_percentage, rollout_stage, deployment_status, created_at, updated_at, completed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(tenant_id, rule_id) DO UPDATE SET
			   version_id = excluded.version_id,
			   sample_size_percentage = excluded.sample_size_percentage,
			   rollout_stage = excluded.rollout_stage,
			   deployment_status = excluded.deployment_status,
			   updated_at = excluded.updated_at,
			   completed_at = excluded.completed_at`,
			id, ver.TenantID, ruleID, versionID, samplePct, stage, status, nowStr, nowStr, completedAt,
		); err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "canary_rule_deployment", ruleID, "CANARY_SET", sess.UserID, nil, map[string]any{
				"versionId":    versionID,
				"samplePct":    samplePct,
				"rolloutStage": stage,
				"status":       status,
			}, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.CanaryStatus{}, err
	}
	return models.CanaryStatus{
		ID:                   id,
		RuleID:               ruleID,
		VersionID:            versionID,
		SampleSizePercentage: samplePct,
		RolloutStage:         stage,
		DeploymentStatus:     status,
		CreatedAt:            nowStr,
		UpdatedAt:            nowStr,
		CompletedAt:          completedAt,
	}, nil
}

// ShouldRouteCanary deterministically computes FNV-1a bucketing (< samplePct * 100 mod 10000).
func ShouldRouteCanary(ruleID, entityID string, samplePct float64) bool {
	return ShouldRouteCanaryVersion("", ruleID, "", entityID, samplePct)
}

// ShouldRouteCanaryVersion computes tenant- and version-scoped FNV-1a deterministic canary routing.
func ShouldRouteCanaryVersion(tenantID, ruleID, versionID, entityID string, samplePct float64) bool {
	if samplePct <= 0 {
		return false
	}
	if samplePct >= 100 {
		return true
	}
	key := ruleID + ":" + entityID
	if tenantID != "" || versionID != "" {
		if tenantID == "" {
			tenantID = defaultTenantID
		}
		key = tenantID + ":" + ruleID + ":" + versionID + ":" + entityID
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	bucket := h.Sum64() % 10000
	threshold := uint64(samplePct * 100)
	return bucket < threshold
}

// RollbackRule performs an atomic emergency disable of an automation rule, marks active canary & prod deployments ROLLED_BACK, and records rollback_action_log.
func (s *Service) RollbackRule(ruleID, reason string) error {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return err
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return errors.New("rollback reason is required")
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")

	return transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var prodVersionID string
		_ = tx.QueryRow(
			`SELECT COALESCE(version_id, '') FROM workflow_environment_deployment_log
			 WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND status = 'DEPLOYED'
			 LIMIT 1`,
			defaultTenantID, ruleID,
		).Scan(&prodVersionID)

		res, err := tx.Exec(`UPDATE workflow_decision_rules SET enabled = 0, updated_at = ? WHERE rule_id = ?`, nowStr, ruleID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("rule %q not found", ruleID)
		}

		if _, err := tx.Exec(`UPDATE canary_rule_deployment SET deployment_status = 'ROLLED_BACK' WHERE rule_id = ? AND deployment_status = 'ACTIVE'`, ruleID); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`UPDATE workflow_environment_deployment_log SET status = 'ROLLED_BACK'
			 WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND status = 'DEPLOYED'`,
			defaultTenantID, ruleID,
		); err != nil {
			return err
		}

		detailsJSON, _ := json.Marshal(map[string]string{
			"reason":          trimmedReason,
			"rolledBackVerId": prodVersionID,
		})
		if _, err := tx.Exec(
			`INSERT INTO rollback_action_log (id, original_automation_id, rollback_type, target_entity, performed_by, performed_at, details_json)
			 VALUES (?, ?, 'RULE_ROLLBACK', ?, ?, ?, ?)`,
			newID("rbk"), ruleID, ruleID, sess.UserID, nowStr, string(detailsJSON),
		); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "workflow_decision_rules", ruleID, "ROLLBACK", sess.UserID, nil, map[string]string{
				"reason":          trimmedReason,
				"rolledBackVerId": prodVersionID,
			}, nil); err != nil {
				return err
			}
		}
		return verifyRollbackStateTx(tx, defaultTenantID, ruleID)
	})
}

func verifyRollbackStateTx(tx *sql.Tx, tenantID, ruleID string) error {
	var enabled int
	if err := tx.QueryRow(`SELECT enabled FROM workflow_decision_rules WHERE rule_id = ?`, ruleID).Scan(&enabled); err != nil {
		return fmt.Errorf("rollback verification failed reading rule: %w", err)
	}
	if enabled != 0 {
		return errors.New("rollback verification failed: rule remains enabled")
	}
	var activeCanaries, activeProd int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM canary_rule_deployment WHERE rule_id = ? AND deployment_status = 'ACTIVE'`, ruleID).Scan(&activeCanaries)
	_ = tx.QueryRow(`SELECT COUNT(*) FROM workflow_environment_deployment_log WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND status = 'DEPLOYED'`, tenantID, ruleID).Scan(&activeProd)
	if activeCanaries != 0 || activeProd != 0 {
		return fmt.Errorf("rollback verification failed: activeCanaries=%d activeProd=%d", activeCanaries, activeProd)
	}
	return nil
}

// RestoreRuleVersion restores a previously approved & tested immutable version to env-prod with atomic audit & rollback log.
func (s *Service) RestoreRuleVersion(ruleID, versionID, reason string, adminOverride bool) (models.RuleRiskAssessment, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.RuleRiskAssessment{}, err
	}
	trimmedReason := strings.TrimSpace(reason)
	if len(trimmedReason) < 5 {
		return models.RuleRiskAssessment{}, errors.New("version restore requires a documented reason (>= 5 characters)")
	}

	var risk models.RuleRiskAssessment
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var frozen string
		_ = tx.QueryRow(`SELECT COALESCE(value, '0') FROM app_metadata WHERE key = 'workflow_rule_deployment_frozen'`).Scan(&frozen)
		if frozen == "1" {
			return errors.New("workflow rule deployments are currently frozen by system policy")
		}

		ctx := context.Background()
		ver, err := loadRuleVersionQuery(ctx, tx, ruleID, versionID)
		if err != nil {
			return err
		}

		var apprStatus, apprSHA string
		_ = tx.QueryRow(
			`SELECT COALESCE(approval_status, 'PENDING'), COALESCE(approved_definition_sha256, '')
			 FROM workflow_rule_approvals
			 WHERE rule_id = ? AND version_id = ?
			 ORDER BY datetime(created_at) DESC, rowid DESC LIMIT 1`,
			ruleID, versionID,
		).Scan(&apprStatus, &apprSHA)
		if apprStatus != "APPROVED" {
			return errors.New("cannot restore unapproved rule version to env-prod")
		}
		if !strings.EqualFold(ver.CreatedBy, "system") && apprSHA == "" {
			return errors.New("cannot restore version: missing approved_definition_sha256")
		}
		if apprSHA != "" && !strings.EqualFold(apprSHA, ver.DefinitionSHA256) {
			return errors.New("cannot restore version: approved checksum mismatch")
		}

		var priorTested int
		_ = tx.QueryRow(
			`SELECT COUNT(*) FROM workflow_environment_deployment_log
			 WHERE tenant_id = ? AND rule_id = ? AND version_id = ? AND environment_id IN ('env-test', 'env-prod')`,
			ver.TenantID, ruleID, versionID,
		).Scan(&priorTested)
		if priorTested == 0 {
			return errors.New("cannot restore version that was never promoted through env-test or env-prod")
		}

		risk, err = evaluateRuleRiskQuery(ctx, tx, ruleID, versionID)
		if err != nil {
			return err
		}
		prodSafety := "1"
		_ = tx.QueryRow(`SELECT COALESCE(value, '1') FROM app_metadata WHERE key = 'workflow_deploy_prod_safety_enforcement'`).Scan(&prodSafety)
		if prodSafety != "0" && risk.BlocksProd {
			if !adminOverride {
				return fmt.Errorf("restore blocked by %s risk score (%.0f); administrator override required", risk.Level, risk.Score)
			}
			if !strings.EqualFold(sess.Role, string(security.RoleAdministrator)) {
				return errors.New("unauthorized production override: administrator role is required to restore HIGH/CRITICAL risk version")
			}
		}

		nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
		if _, err := tx.Exec(
			`UPDATE workflow_environment_deployment_log
			 SET status = 'SUPERSEDED'
			 WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND status = 'DEPLOYED'`,
			ver.TenantID, ruleID,
		); err != nil {
			return err
		}
		// Cancel any conflicting canary on a different version
		if _, err := tx.Exec(
			`UPDATE canary_rule_deployment
			 SET deployment_status = 'ROLLED_BACK'
			 WHERE tenant_id = ? AND rule_id = ? AND version_id != ? AND deployment_status = 'ACTIVE'`,
			ver.TenantID, ruleID, versionID,
		); err != nil {
			return err
		}

		detailsJSON, _ := json.Marshal(map[string]any{
			"restoredBy":       sess.UserID,
			"reason":           trimmedReason,
			"definitionSha256": ver.DefinitionSHA256,
			"adminOverride":    adminOverride,
		})
		if _, err := tx.Exec(
			`INSERT INTO workflow_environment_deployment_log (deployment_id, tenant_id, environment_id, rule_id, version_id, status, timestamp, details_json)
			 VALUES (?, ?, 'env-prod', ?, ?, 'DEPLOYED', ?, ?)`,
			newID("dep"), ver.TenantID, ruleID, versionID, nowStr, string(detailsJSON),
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO workflow_rule_deployment_log (deployment_id, rule_id, version_id, deployed_by, deployment_status, deployment_time, rollback_flag, details_json)
			 VALUES (?, ?, ?, ?, 'RESTORED', ?, 1, ?)`,
			newID("rdep"), ruleID, versionID, sess.UserID, nowStr, string(detailsJSON),
		); err != nil {
			return err
		}
		riskJSON, _ := json.Marshal(risk)
		safeInt := 1
		if risk.BlocksProd && !adminOverride {
			safeInt = 0
		}
		if _, err := tx.Exec(
			`INSERT INTO deployment_risk_assessment (assessment_id, assessed_at, rule_id, version_id, tenant_id, environment_id, risk_score, risk_level, safe_to_deploy, assessment_json)
			 VALUES (?, ?, ?, ?, ?, 'env-prod', ?, ?, ?, ?)
			 ON CONFLICT(rule_id, version_id, tenant_id, environment_id) DO UPDATE SET
			   assessed_at = excluded.assessed_at,
			   risk_score = excluded.risk_score,
			   risk_level = excluded.risk_level,
			   safe_to_deploy = excluded.safe_to_deploy,
			   assessment_json = excluded.assessment_json`,
			newID("risk"), nowStr, ruleID, versionID, ver.TenantID, risk.Score, risk.Level, safeInt, string(riskJSON),
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO deployment_risk_timeline (id, deployment_time, risk_score, risk_level, environment_id, tenant_id, rule_id, version_id, result, details_json)
			 VALUES (?, ?, ?, ?, 'env-prod', ?, ?, ?, 'RESTORED', ?)`,
			newID("rtml"), nowStr, risk.Score, risk.Level, ver.TenantID, ruleID, versionID, string(detailsJSON),
		); err != nil {
			return err
		}
		enInt := 0
		if ver.Enabled {
			enInt = 1
		}
		if _, err := tx.Exec(
			`UPDATE workflow_decision_rules
			 SET rule_name = ?, rule_type = ?, condition_expression = ?, action_type = ?, priority = ?, enabled = ?, updated_at = ?
			 WHERE rule_id = ?`,
			ver.RuleName, ver.RuleType, ver.ConditionExpression, ver.ActionType, ver.Priority, enInt, nowStr, ruleID,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO rollback_action_log (id, original_automation_id, rollback_type, target_entity, performed_by, performed_at, details_json)
			 VALUES (?, ?, 'VERSION_RESTORE', ?, ?, ?, ?)`,
			newID("rbk"), ruleID, versionID, sess.UserID, nowStr, string(detailsJSON),
		); err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "workflow_decision_rules", ruleID, "RESTORE_VERSION", sess.UserID, nil, map[string]any{
				"versionId":        versionID,
				"reason":           trimmedReason,
				"definitionSha256": ver.DefinitionSHA256,
				"adminOverride":    adminOverride,
			}, nil); err != nil {
				return err
			}
		}
		return nil
	})
	return risk, err
}

func loadRuleCostWeightsMap(ctx context.Context, q sqlQueryable) map[string]float64 {
	costWeights := map[string]float64{}
	if cwRows, cwErr := q.QueryContext(ctx, `SELECT rule_id, UPPER(action_type), cost_weight FROM rule_execution_cost_estimate`); cwErr == nil {
		for cwRows.Next() {
			var rID, act string
			var w float64
			if cwRows.Scan(&rID, &act, &w) == nil {
				costWeights[rID+"|"+act] = w
			}
		}
		_ = cwRows.Close()
	}
	return costWeights
}

func resolveRuleCostWeight(costWeights map[string]float64, ruleID, actionType, ruleType string) float64 {
	actUpper := strings.ToUpper(strings.TrimSpace(actionType))
	if actUpper == "" {
		actUpper = strings.ToUpper(strings.TrimSpace(ruleType))
	}
	w := costWeights[ruleID+"|"+actUpper]
	if w <= 0 {
		w = costWeights["|"+actUpper]
	}
	if w <= 0 {
		w = costWeights["|"+strings.ToUpper(strings.TrimSpace(ruleType))]
	}
	if w <= 0 {
		w = 1.0
	}
	return w
}

func (s *Service) doesRuleMatchException(
	ctx context.Context,
	actionUpper, condExpr, exType, entityID, status, priority, assignedTo, slaStatus string,
) bool {
	if status == "RESOLVED" || status == "IGNORED" {
		return false
	}
	switch actionUpper {
	case "AUTO_RESOLVE":
		return s.shouldAutoResolveException(ctx, condExpr, exType, entityID)
	case "AUTO_ASSIGN":
		return assignedTo == "" && status == "OPEN"
	case "PRIORITY_ADJUST":
		return slaStatus == "BREACHED" && priority != "CRITICAL" && priority != "HIGH"
	default:
		return false
	}
}

type dryRunExceptionCase struct {
	id, exType, entityID, status, priority, assignedTo, slaStatus string
	lifecycleGen                                                  int
}

func (s *Service) loadDryRunBudgetAndCases(ctx context.Context) (float64, int64, map[string]float64, []dryRunExceptionCase, error) {
	var maxCostUnits float64 = 500000
	var maxRecords int64 = 500000
	_ = s.db.QueryRowContext(ctx,
		`SELECT max_cost_units_per_cycle, max_records_processed_per_cycle FROM automation_cost_limits WHERE id = 'default'`,
	).Scan(&maxCostUnits, &maxRecords)
	if maxCostUnits <= 0 {
		maxCostUnits = 500000
	}
	if maxRecords <= 0 {
		maxRecords = 500000
	}
	costWeights := loadRuleCostWeightsMap(ctx, s.db)

	exRows, err := s.db.QueryContext(ctx,
		`SELECT id, exception_type, entity_id, status, priority, COALESCE(assigned_to, ''), sla_status, lifecycle_generation
		 FROM exception_cases
		 WHERE status IN ('OPEN', 'IN_PROGRESS')
		 ORDER BY created_at ASC LIMIT 500`,
	)
	if err != nil {
		return 0, 0, nil, nil, err
	}
	defer exRows.Close()

	var cases []dryRunExceptionCase
	for exRows.Next() {
		var c dryRunExceptionCase
		if exRows.Scan(&c.id, &c.exType, &c.entityID, &c.status, &c.priority, &c.assignedTo, &c.slaStatus, &c.lifecycleGen) == nil {
			cases = append(cases, c)
		}
	}
	if err := exRows.Err(); err != nil {
		return 0, 0, nil, nil, err
	}
	return maxCostUnits, maxRecords, costWeights, cases, nil
}

func (s *Service) runDryRunWithSnapshot(
	ctx context.Context,
	ver models.WorkflowRuleVersion,
	risk models.RuleRiskAssessment,
	cases []dryRunExceptionCase,
	maxCostUnits float64,
	maxRecords int64,
	costWeights map[string]float64,
	canaryPct float64,
) models.RuleDryRunResult {
	actionUpper := strings.ToUpper(strings.TrimSpace(ver.ActionType))
	if actionUpper == "" {
		actionUpper = strings.ToUpper(strings.TrimSpace(ver.RuleType))
	}
	costWeight := resolveRuleCostWeight(costWeights, ver.RuleID, ver.ActionType, ver.RuleType)

	// Preload idempotency set (entity + lifecycle_generation) for this version so dry-run stays read-only and fast
	appliedSet := map[string]bool{}
	if iRows, iErr := s.db.QueryContext(ctx,
		`SELECT entity_id, lifecycle_generation FROM workflow_rule_action_idempotency
		 WHERE tenant_id = ? AND rule_version_id = ? AND entity_type = 'exception_case' AND action_type = ?`,
		ver.TenantID, ver.VersionID, actionUpper,
	); iErr == nil {
		for iRows.Next() {
			var eid string
			var gen int
			if iRows.Scan(&eid, &gen) == nil {
				appliedSet[eid+"|"+strconv.Itoa(gen)] = true
			}
		}
		_ = iRows.Close()
	}

	out := models.RuleDryRunResult{
		RuleID:           ver.RuleID,
		VersionID:        ver.VersionID,
		DefinitionSHA256: ver.DefinitionSHA256,
		ActionType:       actionUpper,
		RuleEnabled:      ver.Enabled,
		CanaryPercent:    canaryPct,
		TotalOpenCases:   len(cases),
		CostBudgetCap:    maxCostUnits,
		RecordsBudgetCap: maxRecords,
		Risk:             risk,
		Samples:          []models.RuleDryRunMatch{},
	}

	for _, c := range cases {
		if !s.doesRuleMatchException(ctx, actionUpper, ver.ConditionExpression, c.exType, c.entityID, c.status, c.priority, c.assignedTo, c.slaStatus) {
			continue
		}
		out.EligibleCount++
		already := appliedSet[c.id+"|"+strconv.Itoa(c.lifecycleGen)]
		if already {
			out.SkippedIdempotentCount++
		}
		inCanary := ShouldRouteCanaryVersion(ver.TenantID, ver.RuleID, ver.VersionID, c.id, canaryPct)
		if inCanary && !already {
			out.CanaryRoutedCount++
		}
		if len(out.Samples) < 10 {
			out.Samples = append(out.Samples, models.RuleDryRunMatch{
				ExceptionCaseID: c.id,
				ExceptionType:   c.exType,
				EntityID:        c.entityID,
				CurrentStatus:   c.status,
				CurrentPriority: c.priority,
				ProposedAction:  actionUpper,
				CanaryIncluded:  inCanary,
				AlreadyApplied:  already,
			})
		}
	}
	out.EstimatedCostUnits = float64(out.CanaryRoutedCount) * costWeight
	out.WithinBudget = out.EstimatedCostUnits <= maxCostUnits && int64(out.CanaryRoutedCount) <= maxRecords
	return out
}

// DryRunRuleVersion simulates a rule version against live open exceptions without performing any mutations.
func (s *Service) DryRunRuleVersion(ruleID, versionID string) (models.RuleDryRunResult, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate, security.PermAutomationOpsCenter); err != nil {
		return models.RuleDryRunResult{}, err
	}
	ctx := context.Background()
	ver, err := loadRuleVersionQuery(ctx, s.db, ruleID, versionID)
	if err != nil {
		return models.RuleDryRunResult{}, err
	}
	risk, err := evaluateRuleRiskQuery(ctx, s.db, ruleID, versionID)
	if err != nil {
		return models.RuleDryRunResult{}, err
	}

	maxCostUnits, maxRecords, costWeights, cases, err := s.loadDryRunBudgetAndCases(ctx)
	if err != nil {
		return models.RuleDryRunResult{}, err
	}

	canaryPct := 100.0
	var activeCanaryPct float64
	if cErr := s.db.QueryRowContext(ctx,
		`SELECT sample_size_percentage FROM canary_rule_deployment
		 WHERE tenant_id = ? AND rule_id = ? AND version_id = ? AND deployment_status = 'ACTIVE' LIMIT 1`,
		ver.TenantID, ver.RuleID, ver.VersionID,
	).Scan(&activeCanaryPct); cErr == nil && activeCanaryPct > 0 {
		canaryPct = activeCanaryPct
	}

	return s.runDryRunWithSnapshot(ctx, ver, risk, cases, maxCostUnits, maxRecords, costWeights, canaryPct), nil
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

// UpdateCostLimits validates and updates automation cycle cost & execution caps.
func (s *Service) UpdateCostLimits(in models.AutomationCostLimit) (models.AutomationCostLimit, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.AutomationCostLimit{}, err
	}
	if math.IsNaN(in.MaxCostUnitsPerCycle) || math.IsInf(in.MaxCostUnitsPerCycle, 0) || in.MaxCostUnitsPerCycle <= 0 {
		return models.AutomationCostLimit{}, errors.New("maxCostUnitsPerCycle must be > 0")
	}
	if in.MaxExecutionTimePerCycleMs <= 0 {
		return models.AutomationCostLimit{}, errors.New("maxExecutionTimePerCycleMs must be > 0")
	}
	if in.MaxRecordsProcessedCycle <= 0 {
		return models.AutomationCostLimit{}, errors.New("maxRecordsProcessedPerCycle must be > 0")
	}

	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`UPDATE automation_cost_limits SET max_cost_units_per_cycle = ?, max_execution_time_per_cycle_ms = ?, max_records_processed_per_cycle = ?, updated_at = ? WHERE id = 'default'`,
			in.MaxCostUnitsPerCycle, in.MaxExecutionTimePerCycleMs, in.MaxRecordsProcessedCycle, nowStr,
		); err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "automation_cost_limits", "default", "UPDATE", sess.UserID, nil, in, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.AutomationCostLimit{}, err
	}
	return s.GetCostLimits()
}
