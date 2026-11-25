package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// CanaryRolloutStages defines the supported progressive canary rollout percentages.
var CanaryRolloutStages = []float64{1, 5, 10, 25, 50, 100}

func canaryStageForPct(pct float64) (int, bool) {
	for i, stagePct := range CanaryRolloutStages {
		if math.Abs(pct-stagePct) < 1e-9 {
			return i + 1, true
		}
	}
	return 0, false
}

// AdvanceCanaryStage advances an active canary along 1% -> 5% -> 10% -> 25% -> 50% -> 100%.
// Advancing to 100% completes the rollout and promotes the verified version into normal production deployment.
func (s *Service) AdvanceCanaryStage(ruleID, versionID string) (models.CanaryStatus, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.CanaryStatus{}, err
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	var out models.CanaryStatus

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

		// Block advancing if an unacknowledged CRITICAL alert exists for this rule version
		var openCrit int
		_ = tx.QueryRow(
			`SELECT COUNT(*) FROM automation_stability_alert
			 WHERE rule_id = ? AND version_id = ? AND severity = 'CRITICAL' AND acknowledged = 0`,
			ruleID, versionID,
		).Scan(&openCrit)
		if openCrit > 0 {
			return errors.New("cannot advance canary stage while unacknowledged CRITICAL operational alerts exist")
		}

		var curPct float64
		var curVerID, curStatus, curCreatedAt string
		var curStage int
		cErr := tx.QueryRow(
			`SELECT version_id, sample_size_percentage, rollout_stage, deployment_status, created_at
			 FROM canary_rule_deployment
			 WHERE tenant_id = ? AND rule_id = ?`,
			ver.TenantID, ruleID,
		).Scan(&curVerID, &curPct, &curStage, &curStatus, &curCreatedAt)

		nextStageIdx := 0 // defaults to stage 1 (1%)
		if cErr == nil && curStatus == "ACTIVE" && curVerID == versionID {
			if st, ok := canaryStageForPct(curPct); ok {
				if st >= len(CanaryRolloutStages) {
					return errors.New("canary rollout is already at 100%")
				}
				nextStageIdx = st
			}
		}

		nextPct := CanaryRolloutStages[nextStageIdx]
		nextStage := nextStageIdx + 1
		nextStatus := "ACTIVE"
		completedAt := ""

		if nextPct >= 100 {
			// Transition 100% rollout cleanly into normal env-prod deployment
			var apprStatus, apprSHA string
			_ = tx.QueryRow(
				`SELECT COALESCE(approval_status, 'PENDING'), COALESCE(approved_definition_sha256, '')
				 FROM workflow_rule_approvals
				 WHERE rule_id = ? AND version_id = ?
				 ORDER BY datetime(created_at) DESC, rowid DESC LIMIT 1`,
				ruleID, versionID,
			).Scan(&apprStatus, &apprSHA)
			if apprStatus != "APPROVED" {
				return errors.New("cannot complete 100% rollout: version requires APPROVED status")
			}
			if !strings.EqualFold(ver.CreatedBy, "system") && apprSHA == "" {
				return errors.New("cannot complete 100% rollout: missing approved_definition_sha256")
			}
			if apprSHA != "" && !strings.EqualFold(apprSHA, ver.DefinitionSHA256) {
				return errors.New("cannot complete 100% rollout: stale approval checksum")
			}

			var prodCount int
			_ = tx.QueryRow(
				`SELECT COUNT(*) FROM workflow_environment_deployment_log
				 WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND version_id = ? AND status = 'DEPLOYED'`,
				ver.TenantID, ruleID, versionID,
			).Scan(&prodCount)

			if prodCount == 0 {
				var testCount int
				_ = tx.QueryRow(
					`SELECT COUNT(*) FROM workflow_environment_deployment_log
					 WHERE tenant_id = ? AND rule_id = ? AND version_id = ? AND environment_id IN ('env-test', 'env-prod')`,
					ver.TenantID, ruleID, versionID,
				).Scan(&testCount)
				if testCount == 0 {
					return errors.New("cannot complete 100% rollout: version must be promoted through env-test first")
				}
				risk, rErr := evaluateRuleRiskQuery(ctx, tx, ruleID, versionID)
				if rErr != nil {
					return rErr
				}
				prodSafety := "1"
				_ = tx.QueryRow(`SELECT COALESCE(value, '1') FROM app_metadata WHERE key = 'workflow_deploy_prod_safety_enforcement'`).Scan(&prodSafety)
				if prodSafety != "0" && risk.BlocksProd && !strings.EqualFold(sess.Role, string(security.RoleAdministrator)) {
					return fmt.Errorf("100%% rollout blocked by %s risk score (%.0f); administrator role required", risk.Level, risk.Score)
				}
				if _, err := tx.Exec(
					`UPDATE workflow_environment_deployment_log
					 SET status = 'SUPERSEDED'
					 WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND status = 'DEPLOYED'`,
					ver.TenantID, ruleID,
				); err != nil {
					return err
				}
				det, _ := json.Marshal(map[string]any{
					"deployedBy":       sess.UserID,
					"definitionSha256": ver.DefinitionSHA256,
					"viaCanary":        true,
				})
				if _, err := tx.Exec(
					`INSERT INTO workflow_environment_deployment_log (deployment_id, tenant_id, environment_id, rule_id, version_id, status, timestamp, details_json)
					 VALUES (?, ?, 'env-prod', ?, ?, 'DEPLOYED', ?, ?)`,
					newID("dep"), ver.TenantID, ruleID, versionID, nowStr, string(det),
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
			}
			nextStatus = "COMPLETED"
			completedAt = nowStr
		}

		canID := newID("can")
		createdAt := nowStr
		if curCreatedAt != "" {
			createdAt = curCreatedAt
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
			canID, ver.TenantID, ruleID, versionID, nextPct, nextStage, nextStatus, createdAt, nowStr, completedAt,
		); err != nil {
			return err
		}

		stageDet, _ := json.Marshal(map[string]any{
			"rolloutStage": nextStage,
			"samplePct":    nextPct,
			"status":       nextStatus,
		})
		if _, err := tx.Exec(
			`INSERT INTO workflow_rule_deployment_log (deployment_id, rule_id, version_id, deployed_by, deployment_status, deployment_time, details_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			newID("rdep"), ruleID, versionID, sess.UserID, fmt.Sprintf("CANARY_STAGE_%d", int(nextPct)), nowStr, string(stageDet),
		); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "canary_rule_deployment", ruleID, "CANARY_ADVANCE", sess.UserID, nil, map[string]any{
				"versionId":    versionID,
				"rolloutStage": nextStage,
				"samplePct":    nextPct,
				"status":       nextStatus,
			}, nil); err != nil {
				return err
			}
		}

		out = models.CanaryStatus{
			ID:                   canID,
			RuleID:               ruleID,
			VersionID:            versionID,
			SampleSizePercentage: nextPct,
			RolloutStage:         nextStage,
			DeploymentStatus:     nextStatus,
			CreatedAt:            createdAt,
			UpdatedAt:            nowStr,
			CompletedAt:          completedAt,
		}
		return nil
	})
	return out, err
}

// RecordCanaryCohortMetrics persists separate canary and control cycle metrics into rule_deployment_impact_metrics.
func (s *Service) RecordCanaryCohortMetrics(
	ruleID, versionID, snapshotAt string,
	canary, control models.CanaryCohortMetrics,
) error {
	if _, err := s.security.RequirePermission(security.PermAutomationMutate); err != nil {
		return err
	}
	return s.recordCanaryCohortMetricsInternal(context.Background(), ruleID, versionID, snapshotAt, canary, control)
}

func (s *Service) recordCanaryCohortMetricsInternal(
	ctx context.Context,
	ruleID, versionID, snapshotAt string,
	canary, control models.CanaryCohortMetrics,
) error {
	if strings.TrimSpace(snapshotAt) == "" {
		snapshotAt = time.Now().UTC().Format("2006-01-02 15:04:05")
	}
	if canary.AttemptedCount == 0 && canary.SkippedCount == 0 && control.AttemptedCount == 0 && control.SkippedCount == 0 {
		return nil
	}

	canaryFailRate := 0.0
	if canary.AttemptedCount > 0 {
		canaryFailRate = float64(canary.FailureCount) / float64(canary.AttemptedCount)
	}
	controlFailRate := 0.0
	if control.AttemptedCount > 0 {
		controlFailRate = float64(control.FailureCount) / float64(control.AttemptedCount)
	}

	canaryAvgCost := 0.0
	if canary.AttemptedCount > 0 {
		canaryAvgCost = canary.CostUnits / float64(canary.AttemptedCount)
	}
	controlAvgCost := 0.0
	if control.AttemptedCount > 0 {
		controlAvgCost = control.CostUnits / float64(control.AttemptedCount)
	}
	costDelta := canaryAvgCost - controlAvgCost
	durDelta := canary.AvgDurationMs - control.AvgDurationMs

	return transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		upsert := func(cohort string, m models.CanaryCohortMetrics, failRate, cDelta, dDelta float64) error {
			if m.AttemptedCount == 0 && m.SkippedCount == 0 {
				return nil
			}
			fJSON, _ := json.Marshal(map[string]any{
				"cohort":         cohort,
				"attemptedCount": m.AttemptedCount,
				"successCount":   m.SuccessCount,
				"failureCount":   m.FailureCount,
				"skippedCount":   m.SkippedCount,
				"costUnits":      m.CostUnits,
				"avgDurationMs":  m.AvgDurationMs,
			})
			_, err := tx.ExecContext(ctx,
				`INSERT INTO rule_deployment_impact_metrics
				 (id, rule_id, version_id, cohort, snapshot_at, attempted_count, success_count, failure_count, skipped_count,
				  cost_units, avg_duration_ms, failure_rate, cost_units_delta, duration_delta_ms, execution_count, factors_json)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				 ON CONFLICT(rule_id, version_id, cohort, snapshot_at) DO UPDATE SET
				   attempted_count = rule_deployment_impact_metrics.attempted_count + excluded.attempted_count,
				   success_count = rule_deployment_impact_metrics.success_count + excluded.success_count,
				   failure_count = rule_deployment_impact_metrics.failure_count + excluded.failure_count,
				   skipped_count = rule_deployment_impact_metrics.skipped_count + excluded.skipped_count,
				   cost_units = rule_deployment_impact_metrics.cost_units + excluded.cost_units,
				   avg_duration_ms = CASE
				     WHEN (rule_deployment_impact_metrics.attempted_count + excluded.attempted_count) > 0 THEN
				       ((rule_deployment_impact_metrics.avg_duration_ms * rule_deployment_impact_metrics.attempted_count) +
				        (excluded.avg_duration_ms * excluded.attempted_count)) /
				       (rule_deployment_impact_metrics.attempted_count + excluded.attempted_count)
				     ELSE 0 END,
				   failure_rate = CASE
				     WHEN (rule_deployment_impact_metrics.attempted_count + excluded.attempted_count) > 0 THEN
				       CAST((rule_deployment_impact_metrics.failure_count + excluded.failure_count) AS REAL) /
				       (rule_deployment_impact_metrics.attempted_count + excluded.attempted_count)
				     ELSE 0 END,
				   cost_units_delta = excluded.cost_units_delta,
				   duration_delta_ms = excluded.duration_delta_ms,
				   execution_count = rule_deployment_impact_metrics.execution_count + excluded.execution_count,
				   factors_json = excluded.factors_json`,
				newID("imp"), ruleID, versionID, cohort, snapshotAt,
				m.AttemptedCount, m.SuccessCount, m.FailureCount, m.SkippedCount,
				m.CostUnits, m.AvgDurationMs, failRate, cDelta, dDelta, m.AttemptedCount, string(fJSON),
			)
			return err
		}
		if err := upsert("canary", canary, canaryFailRate, costDelta, durDelta); err != nil {
			return err
		}
		return upsert("control", control, controlFailRate, 0, 0)
	})
}

// GetRollbackThresholds returns configurable automatic rollback thresholds from canary_rollback_thresholds.
func (s *Service) GetRollbackThresholds() (models.CanaryRollbackThreshold, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate, security.PermAutomationOpsCenter); err != nil {
		return models.CanaryRollbackThreshold{}, err
	}
	return s.getRollbackThresholdsInternal(context.Background(), s.db)
}

func (s *Service) getRollbackThresholdsInternal(ctx context.Context, q sqlQueryable) (models.CanaryRollbackThreshold, error) {
	var out models.CanaryRollbackThreshold
	err := q.QueryRowContext(ctx,
		`SELECT id, min_sample_size, max_failure_rate, max_cost_increase_pct, max_duration_increase_pct, monitoring_window_minutes, updated_at
		 FROM canary_rollback_thresholds WHERE id = 'default'`,
	).Scan(&out.ID, &out.MinSampleSize, &out.MaxFailureRate, &out.MaxCostIncreasePct, &out.MaxDurationIncreasePct, &out.MonitoringWindowMinutes, &out.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.CanaryRollbackThreshold{
			ID:                      "default",
			MinSampleSize:           10,
			MaxFailureRate:          0.25,
			MaxCostIncreasePct:      50,
			MaxDurationIncreasePct:  100,
			MonitoringWindowMinutes: 60,
		}, nil
	}
	return out, err
}

// UpdateRollbackThresholds validates and updates canary_rollback_thresholds.
func (s *Service) UpdateRollbackThresholds(in models.CanaryRollbackThreshold) (models.CanaryRollbackThreshold, error) {
	sess, err := s.security.RequirePermission(security.PermAutomationMutate)
	if err != nil {
		return models.CanaryRollbackThreshold{}, err
	}
	if in.MinSampleSize < 1 {
		return models.CanaryRollbackThreshold{}, errors.New("minSampleSize must be >= 1")
	}
	if math.IsNaN(in.MaxFailureRate) || math.IsInf(in.MaxFailureRate, 0) || in.MaxFailureRate <= 0 || in.MaxFailureRate > 1.0 {
		return models.CanaryRollbackThreshold{}, errors.New("maxFailureRate must be > 0 and <= 1.0")
	}
	if math.IsNaN(in.MaxCostIncreasePct) || math.IsInf(in.MaxCostIncreasePct, 0) || in.MaxCostIncreasePct < 0 {
		return models.CanaryRollbackThreshold{}, errors.New("maxCostIncreasePct must be >= 0")
	}
	if math.IsNaN(in.MaxDurationIncreasePct) || math.IsInf(in.MaxDurationIncreasePct, 0) || in.MaxDurationIncreasePct < 0 {
		return models.CanaryRollbackThreshold{}, errors.New("maxDurationIncreasePct must be >= 0")
	}
	if in.MonitoringWindowMinutes < 1 {
		return models.CanaryRollbackThreshold{}, errors.New("monitoringWindowMinutes must be >= 1")
	}

	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO canary_rollback_thresholds (id, min_sample_size, max_failure_rate, max_cost_increase_pct, max_duration_increase_pct, monitoring_window_minutes, updated_at)
			 VALUES ('default', ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			   min_sample_size = excluded.min_sample_size,
			   max_failure_rate = excluded.max_failure_rate,
			   max_cost_increase_pct = excluded.max_cost_increase_pct,
			   max_duration_increase_pct = excluded.max_duration_increase_pct,
			   monitoring_window_minutes = excluded.monitoring_window_minutes,
			   updated_at = excluded.updated_at`,
			in.MinSampleSize, in.MaxFailureRate, in.MaxCostIncreasePct, in.MaxDurationIncreasePct, in.MonitoringWindowMinutes, nowStr,
		); err != nil {
			return err
		}
		if s.audit != nil {
			return s.audit.RecordTableAuditTx(tx, "canary_rollback_thresholds", "default", "UPDATE", sess.UserID, nil, in, nil)
		}
		return nil
	})
	if err != nil {
		return models.CanaryRollbackThreshold{}, err
	}
	return s.GetRollbackThresholds()
}

// GetCanaryMonitoring aggregates canary vs control cohort metrics within the configured monitoring window.
func (s *Service) GetCanaryMonitoring(ruleID, versionID string) (models.CanaryMonitoringSummary, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate, security.PermAutomationOpsCenter); err != nil {
		return models.CanaryMonitoringSummary{}, err
	}
	return s.getCanaryMonitoringInternal(context.Background(), s.db, ruleID, versionID)
}

func (s *Service) getCanaryMonitoringInternal(ctx context.Context, q sqlQueryable, ruleID, versionID string) (models.CanaryMonitoringSummary, error) {
	thresholds, err := s.getRollbackThresholdsInternal(ctx, q)
	if err != nil {
		return models.CanaryMonitoringSummary{}, err
	}
	ver, err := loadRuleVersionQuery(ctx, q, ruleID, versionID)
	if err != nil {
		return models.CanaryMonitoringSummary{}, err
	}

	out := models.CanaryMonitoringSummary{
		RuleID:        ruleID,
		VersionID:     versionID,
		WindowMinutes: thresholds.MonitoringWindowMinutes,
		HealthStatus:  "INSUFFICIENT_SAMPLES",
	}

	_ = q.QueryRowContext(ctx,
		`SELECT sample_size_percentage, rollout_stage, deployment_status
		 FROM canary_rule_deployment
		 WHERE tenant_id = ? AND rule_id = ? AND version_id = ?`,
		ver.TenantID, ruleID, versionID,
	).Scan(&out.SampleSizePercentage, &out.RolloutStage, &out.DeploymentStatus)

	windowExpr := fmt.Sprintf("-%d minutes", thresholds.MonitoringWindowMinutes)
	loadCohort := func(cohort string) (models.CanaryCohortMetrics, error) {
		var m models.CanaryCohortMetrics
		var weightedDur float64
		err := q.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(attempted_count), 0),
			        COALESCE(SUM(success_count), 0),
			        COALESCE(SUM(failure_count), 0),
			        COALESCE(SUM(skipped_count), 0),
			        COALESCE(SUM(cost_units), 0),
			        COALESCE(SUM(avg_duration_ms * attempted_count), 0),
			        COALESCE(SUM(execution_count), 0)
			 FROM rule_deployment_impact_metrics
			 WHERE rule_id = ? AND version_id = ? AND cohort = ?
			   AND datetime(snapshot_at) >= datetime('now', ?)`,
			ruleID, versionID, cohort, windowExpr,
		).Scan(&m.AttemptedCount, &m.SuccessCount, &m.FailureCount, &m.SkippedCount, &m.CostUnits, &weightedDur, &m.ExecutionCount)
		if err != nil {
			return m, err
		}
		if m.AttemptedCount > 0 {
			m.FailureRate = float64(m.FailureCount) / float64(m.AttemptedCount)
			m.AvgDurationMs = weightedDur / float64(m.AttemptedCount)
		}
		return m, nil
	}

	if out.Canary, err = loadCohort("canary"); err != nil {
		return models.CanaryMonitoringSummary{}, err
	}
	if out.Control, err = loadCohort("control"); err != nil {
		return models.CanaryMonitoringSummary{}, err
	}

	canaryAvgCost := 0.0
	if out.Canary.AttemptedCount > 0 {
		canaryAvgCost = out.Canary.CostUnits / float64(out.Canary.AttemptedCount)
	}
	controlAvgCost := 0.0
	if out.Control.AttemptedCount > 0 {
		controlAvgCost = out.Control.CostUnits / float64(out.Control.AttemptedCount)
	}
	baselineCost := controlAvgCost
	if baselineCost <= 0 {
		costWeights := loadRuleCostWeightsMap(ctx, q)
		baselineCost = resolveRuleCostWeight(costWeights, ver.RuleID, ver.ActionType, ver.RuleType)
	}
	out.CostDeltaUnits = canaryAvgCost - controlAvgCost
	if out.Canary.AttemptedCount > 0 && baselineCost > 0 {
		out.CostIncreasePct = ((canaryAvgCost - baselineCost) / baselineCost) * 100.0
		if out.CostIncreasePct < 0 {
			out.CostIncreasePct = 0
		}
	}

	out.DurationDeltaMs = out.Canary.AvgDurationMs - out.Control.AvgDurationMs
	if out.Canary.AttemptedCount > 0 && out.Control.AvgDurationMs > 0 {
		out.DurationIncreasePct = ((out.Canary.AvgDurationMs - out.Control.AvgDurationMs) / out.Control.AvgDurationMs) * 100.0
		if out.DurationIncreasePct < 0 {
			out.DurationIncreasePct = 0
		}
	}

	switch out.DeploymentStatus {
	case "ROLLED_BACK":
		out.HealthStatus = "ROLLED_BACK"
	case "COMPLETED":
		out.HealthStatus = "COMPLETED"
	default:
		if out.Canary.AttemptedCount < int64(thresholds.MinSampleSize) {
			out.HealthStatus = "INSUFFICIENT_SAMPLES"
		} else if out.Canary.FailureRate > thresholds.MaxFailureRate ||
			out.CostIncreasePct > thresholds.MaxCostIncreasePct ||
			out.DurationIncreasePct > thresholds.MaxDurationIncreasePct {
			out.HealthStatus = "DEGRADED"
		} else {
			out.HealthStatus = "HEALTHY"
		}
	}
	return out, nil
}

func emitOperationalAlertTx(
	ctx context.Context,
	tx *sql.Tx,
	alertType, severity, ruleID, versionID, dedupeKey, message string,
	details any,
	nowStr string,
) (string, error) {
	detBytes, _ := json.Marshal(details)
	detJSON := string(detBytes)
	if detJSON == "" || detJSON == "null" {
		detJSON = "{}"
	}

	alertID := newID("alt")
	if dedupeKey != "" {
		var existingID string
		_ = tx.QueryRowContext(ctx,
			`SELECT id FROM automation_stability_alert WHERE dedupe_key = ? AND acknowledged = 0 LIMIT 1`,
			dedupeKey,
		).Scan(&existingID)
		if existingID != "" {
			return existingID, nil
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO automation_stability_alert (id, alert_type, severity, rule_id, version_id, message, dedupe_key, details_json, created_at, acknowledged)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		alertID, alertType, severity, ruleID, versionID, message, dedupeKey, detJSON, nowStr,
	); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO workflow_alert_signal_log (id, created_at, signal_type, severity, entity_id, message, details_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		newID("sig"), nowStr, alertType, severity, ruleID, message, detJSON,
	); err != nil {
		return "", err
	}
	return alertID, nil
}

// ListOperationalAlerts returns operational alerts ordered newest first.
func (s *Service) ListOperationalAlerts(includeAcknowledged bool) ([]models.OperationalAlert, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationMutate, security.PermAutomationOpsCenter); err != nil {
		return nil, err
	}
	query := `SELECT id, alert_type, severity, COALESCE(rule_id, ''), COALESCE(version_id, ''),
	                 COALESCE(message, ''), details_json, created_at, acknowledged,
	                 COALESCE(acknowledged_by, ''), COALESCE(acknowledged_at, '')
	          FROM automation_stability_alert`
	if !includeAcknowledged {
		query += ` WHERE acknowledged = 0`
	}
	query += ` ORDER BY datetime(created_at) DESC, rowid DESC LIMIT 100`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.OperationalAlert
	for rows.Next() {
		var a models.OperationalAlert
		var ack int
		if err := rows.Scan(&a.ID, &a.AlertType, &a.Severity, &a.RuleID, &a.VersionID,
			&a.Message, &a.DetailsJSON, &a.CreatedAt, &ack, &a.AcknowledgedBy, &a.AcknowledgedAt); err != nil {
			return nil, err
		}
		a.Acknowledged = ack == 1
		out = append(out, a)
	}
	if out == nil {
		out = []models.OperationalAlert{}
	}
	return out, rows.Err()
}

// AcknowledgeOperationalAlert marks an operational alert as acknowledged by the current operator.
func (s *Service) AcknowledgeOperationalAlert(alertID string) error {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationMutate, security.PermAutomationOpsCenter)
	if err != nil {
		return err
	}
	alertID = strings.TrimSpace(alertID)
	if alertID == "" {
		return errors.New("alertId is required")
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	return transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE automation_stability_alert
			 SET acknowledged = 1, acknowledged_by = ?, acknowledged_at = ?
			 WHERE id = ? AND acknowledged = 0`,
			sess.UserID, nowStr, alertID,
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("open alert %q not found", alertID)
		}
		if s.audit != nil {
			return s.audit.RecordTableAuditTx(tx, "automation_stability_alert", alertID, "ACKNOWLEDGE", sess.UserID, nil, map[string]string{
				"acknowledgedBy": sess.UserID,
				"acknowledgedAt": nowStr,
			}, nil)
		}
		return nil
	})
}

// EvaluateCanaryAndAutoRollback evaluates canary telemetry against thresholds and performs two-stage automatic rollback when breached.
func (s *Service) EvaluateCanaryAndAutoRollback(ruleID, versionID string) (models.AutoRollbackResult, error) {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationMutate, security.PermAutomationOpsCenter)
	if err != nil {
		return models.AutoRollbackResult{}, err
	}
	return s.evaluateCanaryAndAutoRollbackInternal(context.Background(), ruleID, versionID, sess.UserID)
}

func (s *Service) evaluateAllActiveCanaries(ctx context.Context, actor string) (int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rule_id, version_id FROM canary_rule_deployment WHERE deployment_status = 'ACTIVE'`,
	)
	if err != nil {
		return 0, err
	}
	type pair struct{ ruleID, verID string }
	var active []pair
	for rows.Next() {
		var p pair
		if rows.Scan(&p.ruleID, &p.verID) == nil {
			active = append(active, p)
		}
	}
	_ = rows.Close()

	rollbacks := 0
	for _, p := range active {
		res, rErr := s.evaluateCanaryAndAutoRollbackInternal(ctx, p.ruleID, p.verID, actor)
		if rErr == nil && res.Triggered {
			rollbacks++
		}
	}
	return rollbacks, nil
}

func (s *Service) evaluateCanaryAndAutoRollbackInternal(ctx context.Context, ruleID, versionID, actor string) (models.AutoRollbackResult, error) {
	out := models.AutoRollbackResult{
		RuleID:          ruleID,
		FailedVersionID: versionID,
	}

	thresholds, err := s.getRollbackThresholdsInternal(ctx, s.db)
	if err != nil {
		return out, err
	}
	mon, err := s.getCanaryMonitoringInternal(ctx, s.db, ruleID, versionID)
	if err != nil {
		return out, err
	}
	if mon.DeploymentStatus != "ACTIVE" {
		return out, nil
	}
	if mon.Canary.AttemptedCount < int64(thresholds.MinSampleSize) {
		return out, nil
	}

	type breachSpec struct {
		alertType string
		severity  string
		reason    string
	}
	var breaches []breachSpec
	if mon.Canary.FailureRate > thresholds.MaxFailureRate {
		breaches = append(breaches, breachSpec{
			alertType: "CANARY_FAILURE_THRESHOLD_EXCEEDED",
			severity:  "CRITICAL",
			reason:    fmt.Sprintf("failure rate %.2f%% exceeded max %.2f%%", mon.Canary.FailureRate*100, thresholds.MaxFailureRate*100),
		})
	}
	if mon.CostIncreasePct > thresholds.MaxCostIncreasePct {
		breaches = append(breaches, breachSpec{
			alertType: "CANARY_COST_THRESHOLD_EXCEEDED",
			severity:  "HIGH",
			reason:    fmt.Sprintf("cost increase %.1f%% exceeded max %.1f%%", mon.CostIncreasePct, thresholds.MaxCostIncreasePct),
		})
	}
	if mon.DurationIncreasePct > thresholds.MaxDurationIncreasePct {
		breaches = append(breaches, breachSpec{
			alertType: "CANARY_LATENCY_THRESHOLD_EXCEEDED",
			severity:  "HIGH",
			reason:    fmt.Sprintf("latency increase %.1f%% exceeded max %.1f%%", mon.DurationIncreasePct, thresholds.MaxDurationIncreasePct),
		})
	}
	if len(breaches) == 0 {
		return out, nil
	}

	for _, b := range breaches {
		out.BreachReasons = append(out.BreachReasons, b.reason)
	}

	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	ver, err := loadRuleVersionQuery(ctx, s.db, ruleID, versionID)
	if err != nil {
		return out, err
	}

	// STAGE 1 — Emergency Disable (atomic transaction + verifyRollbackStateTx)
	err = transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		var curStatus, curVer string
		if qErr := tx.QueryRowContext(ctx,
			`SELECT deployment_status, version_id FROM canary_rule_deployment WHERE tenant_id = ? AND rule_id = ?`,
			ver.TenantID, ruleID,
		).Scan(&curStatus, &curVer); qErr != nil {
			return qErr
		}
		if curStatus != "ACTIVE" || curVer != versionID {
			// Already disabled / rolled back — prevent duplicate rollback loops
			return nil
		}

		// Prevent duplicate auto-rollback for the same failed version within the active monitoring window
		var recentAutoRbk int
		_ = tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM rollback_action_log
			 WHERE original_automation_id = ? AND rollback_type = 'AUTO_ROLLBACK_DISABLE'
			   AND json_extract(details_json, '$.failedVersionId') = ?
			   AND datetime(performed_at) >= datetime('now', ?)`,
			ruleID, versionID, fmt.Sprintf("-%d minutes", thresholds.MonitoringWindowMinutes),
		).Scan(&recentAutoRbk)
		if recentAutoRbk > 0 {
			return nil
		}

		if _, err := tx.ExecContext(ctx, `UPDATE workflow_decision_rules SET enabled = 0, updated_at = ? WHERE rule_id = ?`, nowStr, ruleID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE canary_rule_deployment SET deployment_status = 'ROLLED_BACK', updated_at = ? WHERE tenant_id = ? AND rule_id = ? AND deployment_status = 'ACTIVE'`,
			nowStr, ver.TenantID, ruleID,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE workflow_environment_deployment_log SET status = 'ROLLED_BACK'
			 WHERE tenant_id = ? AND environment_id = 'env-prod' AND rule_id = ? AND status = 'DEPLOYED'`,
			ver.TenantID, ruleID,
		); err != nil {
			return err
		}
		if err := verifyRollbackStateTx(tx, ver.TenantID, ruleID); err != nil {
			return err
		}

		rbkDetails := map[string]any{
			"failedVersionId": versionID,
			"breachReasons":   out.BreachReasons,
			"metrics":         mon,
			"thresholds":      thresholds,
		}
		rbkJSON, _ := json.Marshal(rbkDetails)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rollback_action_log (id, original_automation_id, rollback_type, target_entity, performed_by, performed_at, details_json)
			 VALUES (?, ?, 'AUTO_ROLLBACK_DISABLE', ?, ?, ?, ?)`,
			newID("rbk"), ruleID, versionID, actor, nowStr, string(rbkJSON),
		); err != nil {
			return err
		}

		for _, b := range breaches {
			aid, aErr := emitOperationalAlertTx(ctx, tx, b.alertType, b.severity, ruleID, versionID,
				b.alertType+":"+ruleID+":"+versionID,
				fmt.Sprintf("Canary %s (%s): %s", ruleID, versionID, b.reason),
				rbkDetails, nowStr,
			)
			if aErr != nil {
				return aErr
			}
			if out.AlertID == "" {
				out.AlertID = aid
			}
		}
		if _, err := emitOperationalAlertTx(ctx, tx, "DEPLOYMENT_HEALTH_DEGRADED", "HIGH", ruleID, versionID,
			"DEPLOYMENT_HEALTH_DEGRADED:"+ruleID+":"+versionID,
			fmt.Sprintf("Deployment health degraded for %s (%s); Stage 1 emergency disable executed", ruleID, versionID),
			rbkDetails, nowStr,
		); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "workflow_decision_rules", ruleID, "AUTO_ROLLBACK_DISABLE", actor, nil, rbkDetails, nil); err != nil {
				return err
			}
		}
		out.Triggered = true
		out.Stage1Disabled = true
		return nil
	})
	if err != nil || !out.Triggered {
		return out, err
	}

	// STAGE 2 — Safe Previous-Version Recovery (only if Stage 1 verified & safe candidate passes all 6 checks)
	err = transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		var frozen string
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(value, '0') FROM app_metadata WHERE key = 'workflow_rule_deployment_frozen'`).Scan(&frozen)
		if frozen == "1" {
			aid, aErr := emitOperationalAlertTx(ctx, tx, "AUTO_ROLLBACK_NO_SAFE_VERSION", "CRITICAL", ruleID, versionID,
				"AUTO_ROLLBACK_NO_SAFE_VERSION:"+ruleID+":"+versionID,
				fmt.Sprintf("Automatic restore blocked for %s because deployments are frozen; rule remains disabled", ruleID),
				map[string]any{"failedVersionId": versionID, "reason": "deployment_frozen"}, nowStr,
			)
			if aErr == nil && aid != "" {
				out.AlertID = aid
			}
			return aErr
		}

		prodSafety := "1"
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(value, '1') FROM app_metadata WHERE key = 'workflow_deploy_prod_safety_enforcement'`).Scan(&prodSafety)

		candRows, cErr := tx.QueryContext(ctx,
			`SELECT version_id FROM workflow_rule_versions
			 WHERE tenant_id = ? AND rule_id = ? AND version_number < ?
			 ORDER BY version_number DESC`,
			ver.TenantID, ruleID, ver.VersionNumber,
		)
		if cErr != nil {
			return cErr
		}
		var candIDs []string
		for candRows.Next() {
			var cid string
			if candRows.Scan(&cid) == nil {
				candIDs = append(candIDs, cid)
			}
		}
		_ = candRows.Close()

		var safeVer *models.WorkflowRuleVersion
		for _, cid := range candIDs {
			cand, lErr := loadRuleVersionQuery(ctx, tx, ruleID, cid)
			if lErr != nil || !cand.Enabled {
				continue
			}
			if cand.ApprovalStatus != "APPROVED" {
				continue
			}
			if !strings.EqualFold(cand.CreatedBy, "system") && (cand.ApprovedSHA256 == "" || !strings.EqualFold(cand.ApprovedSHA256, cand.DefinitionSHA256)) {
				continue
			}
			var priorPromoted int
			_ = tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM workflow_environment_deployment_log
				 WHERE tenant_id = ? AND rule_id = ? AND version_id = ? AND environment_id IN ('env-test', 'env-prod')`,
				ver.TenantID, ruleID, cid,
			).Scan(&priorPromoted)
			if priorPromoted == 0 {
				continue
			}
			risk, rErr := evaluateRuleRiskQuery(ctx, tx, ruleID, cid)
			if rErr != nil {
				continue
			}
			if prodSafety != "0" && risk.BlocksProd {
				continue
			}
			safeVer = &cand
			break
		}

		if safeVer == nil {
			aid, aErr := emitOperationalAlertTx(ctx, tx, "AUTO_ROLLBACK_NO_SAFE_VERSION", "CRITICAL", ruleID, versionID,
				"AUTO_ROLLBACK_NO_SAFE_VERSION:"+ruleID+":"+versionID,
				fmt.Sprintf("Automatic rollback disabled %s (%s); no safe approved previous version available for recovery — administrator intervention required", ruleID, versionID),
				map[string]any{"failedVersionId": versionID, "breachReasons": out.BreachReasons}, nowStr,
			)
			if aErr == nil && aid != "" {
				out.AlertID = aid
			}
			return aErr
		}

		restoredDet, _ := json.Marshal(map[string]any{
			"restoredBy":        actor,
			"failedVersionId":   versionID,
			"restoredVersionId": safeVer.VersionID,
			"definitionSha256":  safeVer.DefinitionSHA256,
			"breachReasons":     out.BreachReasons,
		})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO workflow_environment_deployment_log (deployment_id, tenant_id, environment_id, rule_id, version_id, status, timestamp, details_json)
			 VALUES (?, ?, 'env-prod', ?, ?, 'DEPLOYED', ?, ?)`,
			newID("dep"), ver.TenantID, ruleID, safeVer.VersionID, nowStr, string(restoredDet),
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO workflow_rule_deployment_log (deployment_id, rule_id, version_id, deployed_by, deployment_status, deployment_time, rollback_flag, details_json)
			 VALUES (?, ?, ?, ?, 'AUTO_RESTORED', ?, 1, ?)`,
			newID("rdep"), ruleID, safeVer.VersionID, actor, nowStr, string(restoredDet),
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE workflow_decision_rules
			 SET rule_name = ?, rule_type = ?, condition_expression = ?, action_type = ?, priority = ?, enabled = 1, updated_at = ?
			 WHERE rule_id = ?`,
			safeVer.RuleName, safeVer.RuleType, safeVer.ConditionExpression, safeVer.ActionType, safeVer.Priority, nowStr, ruleID,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rollback_action_log (id, original_automation_id, rollback_type, target_entity, performed_by, performed_at, details_json)
			 VALUES (?, ?, 'AUTO_ROLLBACK_RESTORE', ?, ?, ?, ?)`,
			newID("rbk"), ruleID, safeVer.VersionID, actor, nowStr, string(restoredDet),
		); err != nil {
			return err
		}
		if _, err := emitOperationalAlertTx(ctx, tx, "AUTO_ROLLBACK_COMPLETED", "WARNING", ruleID, safeVer.VersionID,
			"AUTO_ROLLBACK_COMPLETED:"+ruleID+":"+versionID,
			fmt.Sprintf("Automatic rollback disabled %s and safely restored approved version %s", versionID, safeVer.VersionID),
			map[string]any{"failedVersionId": versionID, "restoredVersionId": safeVer.VersionID}, nowStr,
		); err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.RecordTableAuditTx(tx, "workflow_decision_rules", ruleID, "AUTO_ROLLBACK_RESTORE", actor, nil, map[string]any{
				"failedVersionId":   versionID,
				"restoredVersionId": safeVer.VersionID,
				"definitionSha256":  safeVer.DefinitionSHA256,
			}, nil); err != nil {
				return err
			}
		}

		out.Stage2Restored = true
		out.RestoredVersionID = safeVer.VersionID
		return nil
	})
	return out, err
}
