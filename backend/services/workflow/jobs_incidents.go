package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// ListBackgroundJobs lists all 6 canonical system background jobs along with their latest execution state.
func (s *Service) ListBackgroundJobs() ([]models.BackgroundJob, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationOpsCenter); err != nil {
		return nil, err
	}
	return s.listBackgroundJobsInternal()
}

func (s *Service) isJobGuardStopped(ctx context.Context, jobID string) bool {
	var guardTime string
	err := s.db.QueryRowContext(ctx,
		`SELECT recovery_time FROM workflow_job_recovery_log
		 WHERE job_id = ? AND result = 'RECOVERY_GUARD_STOP' AND datetime(recovery_time) >= datetime('now', '-24 hours')
		 ORDER BY datetime(recovery_time) DESC, rowid DESC LIMIT 1`,
		jobID,
	).Scan(&guardTime)
	if err != nil || guardTime == "" {
		return false
	}
	var overrideTime string
	_ = s.db.QueryRowContext(ctx,
		`SELECT created_at FROM workflow_job_manual_override_log
		 WHERE job_id = ? AND action = 'OVERRIDE_RECOVERY_GUARD'
		 ORDER BY datetime(created_at) DESC, rowid DESC LIMIT 1`,
		jobID,
	).Scan(&overrideTime)
	if overrideTime == "" {
		return true
	}
	return overrideTime < guardTime
}

func (s *Service) listBackgroundJobsInternal() ([]models.BackgroundJob, error) {
	rows, err := s.db.Query(
		`SELECT j.job_id, j.job_name, j.job_type, j.schedule_type, j.is_enabled,
		        j.expected_duration_ms, j.max_retries, j.retry_delay_sec,
		        COALESCE((SELECT e.status FROM workflow_job_execution_log e WHERE e.job_id = j.job_id ORDER BY datetime(e.started_at) DESC, e.rowid DESC LIMIT 1), 'IDLE'),
		        COALESCE((SELECT e.started_at FROM workflow_job_execution_log e WHERE e.job_id = j.job_id ORDER BY datetime(e.started_at) DESC, e.rowid DESC LIMIT 1), ''),
		        COALESCE((SELECT COALESCE(e.completed_at, '') FROM workflow_job_execution_log e WHERE e.job_id = j.job_id ORDER BY datetime(e.started_at) DESC, e.rowid DESC LIMIT 1), ''),
		        COALESCE((SELECT COALESCE(e.error_message, '') FROM workflow_job_execution_log e WHERE e.job_id = j.job_id ORDER BY datetime(e.started_at) DESC, e.rowid DESC LIMIT 1), ''),
		        COALESCE((SELECT COUNT(*) FROM workflow_job_recovery_log r WHERE r.job_id = j.job_id AND datetime(r.recovery_time) >= datetime('now', '-24 hours')), 0),
		        COALESCE((SELECT exp.expected_interval_minutes FROM workflow_job_schedule_expectations exp WHERE exp.job_id = j.job_id), 1440)
		 FROM workflow_background_jobs j ORDER BY j.job_id ASC`,
	)
	if err != nil {
		return nil, err
	}

	var out []models.BackgroundJob
	var intervals []int
	for rows.Next() {
		var j models.BackgroundJob
		var en int
		var intervalMin int
		if err := rows.Scan(
			&j.JobID, &j.JobName, &j.JobType, &j.ScheduleType, &en,
			&j.ExpectedDurationMs, &j.MaxRetries, &j.RetryDelaySec,
			&j.LastStatus, &j.LastStartedAt, &j.LastCompletedAt, &j.LastErrorMessage,
			&j.RecoveryAttempts24h, &intervalMin,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		j.IsEnabled = en == 1
		out = append(out, j)
		intervals = append(intervals, intervalMin)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	depMap := map[string][]string{}
	if depRows, dErr := s.db.Query(`SELECT dependent_job_id, parent_job_id FROM workflow_job_dependencies ORDER BY parent_job_id ASC`); dErr == nil {
		for depRows.Next() {
			var depID, parentID string
			if depRows.Scan(&depID, &parentID) == nil {
				depMap[depID] = append(depMap[depID], parentID)
			}
		}
		_ = depRows.Close()
	}

	ctx := context.Background()
	for i := range out {
		j := &out[i]
		intervalMin := intervals[i]
		j.GuardStopped = s.isJobGuardStopped(ctx, j.JobID)
		if deps := depMap[j.JobID]; len(deps) > 0 {
			j.DependsOn = deps
		}
		if j.IsEnabled && !j.GuardStopped && intervalMin > 0 {
			if j.LastCompletedAt != "" {
				if t, parseErr := time.Parse("2006-01-02 15:04:05", j.LastCompletedAt); parseErr == nil {
					j.NextExpectedAt = t.Add(time.Duration(intervalMin) * time.Minute).Format("2006-01-02 15:04:05")
				}
			} else {
				j.NextExpectedAt = "Due now"
			}
		}
	}
	if out == nil {
		out = []models.BackgroundJob{}
	}
	return out, nil
}

// ToggleBackgroundJob enables or disables one of the scheduled background jobs.
func (s *Service) ToggleBackgroundJob(jobID string, enabled bool) error {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationMutate, security.PermAutomationOpsCenter)
	if err != nil {
		return err
	}
	if !enabled && jobID == "maintenance_cleanup" {
		return errors.New("CANNOT_DISABLE_MAINTENANCE_JOB: maintenance_cleanup cannot be disabled")
	}
	if enabled && s.isJobGuardStopped(context.Background(), jobID) {
		return fmt.Errorf("job %q is halted by RECOVERY_GUARD_STOP circuit breaker; override recovery guard first", jobID)
	}
	en := 0
	if enabled {
		en = 1
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	res, err := s.db.Exec(
		`UPDATE workflow_background_jobs SET is_enabled = ?, updated_at = ? WHERE job_id = ?`,
		en, nowStr, jobID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("background job %q not found", jobID)
	}
	_ = s.audit.RecordTableAudit("workflow_background_jobs", jobID, "TOGGLE", sess.UserID, nil, map[string]bool{"enabled": enabled}, nil)
	return nil
}

// OverrideRecoveryGuard clears a RECOVERY_GUARD_STOP circuit breaker, logs the operator override, and re-enables the job.
func (s *Service) OverrideRecoveryGuard(jobID, reason string) error {
	sess, err := s.security.RequirePermission(security.PermAutomationOpsCenter)
	if err != nil {
		return err
	}
	trimmedReason := strings.TrimSpace(reason)
	if len(trimmedReason) < 5 {
		return errors.New("override reason must be at least 5 characters")
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE workflow_background_jobs SET is_enabled = 1, updated_at = ? WHERE job_id = ?`,
			nowStr, jobID,
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("background job %q not found", jobID)
		}
		_, err = tx.Exec(
			`INSERT INTO workflow_job_manual_override_log (id, job_id, action, reason, caller_role, created_at)
			 VALUES (?, ?, 'OVERRIDE_RECOVERY_GUARD', ?, ?, ?)`,
			newID("ovr"), jobID, trimmedReason, string(sess.Role), nowStr,
		)
		return err
	})
	if err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("workflow_job_manual_override_log", jobID, "OVERRIDE_RECOVERY_GUARD", sess.UserID, nil, map[string]string{"reason": trimmedReason}, nil)
	return nil
}

// ListJobExecutions returns execution history records from workflow_job_execution_log.
func (s *Service) ListJobExecutions(jobID string, limit int) ([]models.JobExecutionRecord, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationOpsCenter); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if strings.TrimSpace(jobID) != "" {
		rows, err = s.db.Query(
			`SELECT execution_id, job_id, started_at, COALESCE(completed_at, ''), status,
			        records_processed, COALESCE(error_message, ''), COALESCE(execution_time_ms, 0), retry_count
			 FROM workflow_job_execution_log WHERE job_id = ? ORDER BY datetime(started_at) DESC, rowid DESC LIMIT ?`,
			jobID, limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT execution_id, job_id, started_at, COALESCE(completed_at, ''), status,
			        records_processed, COALESCE(error_message, ''), COALESCE(execution_time_ms, 0), retry_count
			 FROM workflow_job_execution_log ORDER BY datetime(started_at) DESC, rowid DESC LIMIT ?`,
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.JobExecutionRecord
	for rows.Next() {
		var r models.JobExecutionRecord
		if err := rows.Scan(&r.ExecutionID, &r.JobID, &r.StartedAt, &r.CompletedAt, &r.Status,
			&r.RecordsProcessed, &r.ErrorMessage, &r.ExecutionTimeMs, &r.RetryCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []models.JobExecutionRecord{}
	}
	return out, rows.Err()
}

// RunBackgroundJobNow executes one of the 6 canonical background jobs synchronously and records execution telemetry.
func (s *Service) RunBackgroundJobNow(jobID string) (models.JobExecutionRecord, error) {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationMutate, security.PermAutomationOpsCenter)
	if err != nil {
		return models.JobExecutionRecord{}, err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return models.JobExecutionRecord{}, err
	}
	rec, err := s.runJobInternal(context.Background(), jobID, sess.UserID)
	if err == nil {
		_ = s.audit.RecordTableAudit("workflow_job_execution_log", rec.ExecutionID, "RUN_JOB", sess.UserID, nil, rec, nil)
	}
	return rec, err
}

func (s *Service) runJobInternal(ctx context.Context, jobID, actor string) (models.JobExecutionRecord, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_background_jobs WHERE job_id = ?`, jobID).Scan(&exists); err != nil || exists == 0 {
		return models.JobExecutionRecord{}, fmt.Errorf("background job %q not found", jobID)
	}
	if s.isJobGuardStopped(ctx, jobID) {
		return models.JobExecutionRecord{}, fmt.Errorf("background job %q is halted by RECOVERY_GUARD_STOP circuit breaker; admin override required", jobID)
	}

	release, ok := s.acquireJobLock(jobID)
	if !ok {
		return models.JobExecutionRecord{}, fmt.Errorf("background job %q is already running", jobID)
	}
	defer release()

	var recoveryRetries int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workflow_job_recovery_log WHERE job_id = ? AND datetime(recovery_time) >= datetime('now', '-24 hours')`,
		jobID,
	).Scan(&recoveryRetries)

	execID := newID("exec")
	started := time.Now().UTC()
	startedStr := started.Format("2006-01-02 15:04:05")

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO workflow_job_execution_log (execution_id, job_id, started_at, status, records_processed, retry_count)
		 VALUES (?, ?, ?, 'RUNNING', 0, ?)`,
		execID, jobID, startedStr, recoveryRetries,
	); err != nil {
		return models.JobExecutionRecord{}, err
	}

	recordsProcessed, runErr := s.executeCanonicalJobLogic(ctx, jobID, actor)

	completed := time.Now().UTC()
	completedStr := completed.Format("2006-01-02 15:04:05")
	durationMs := completed.Sub(started).Milliseconds()
	if durationMs < 1 {
		durationMs = 1
	}

	status := "COMPLETED"
	errMsg := ""
	if runErr != nil {
		status = "FAILED"
		errMsg = runErr.Error()
		_, _ = s.db.ExecContext(ctx,
			`INSERT INTO workflow_job_failure_alerts (alert_id, job_id, detected_at, alert_type, details_json)
			 VALUES (?, ?, ?, 'JOB_EXECUTION_FAILED', ?)`,
			newID("jfa"), jobID, completedStr, fmt.Sprintf(`{"executionId":%q,"error":%q}`, execID, errMsg),
		)
	} else {
		_, _ = s.db.ExecContext(ctx,
			`UPDATE workflow_job_schedule_expectations SET last_expected_run_at = ?, updated_at = ? WHERE job_id = ?`,
			completedStr, completedStr, jobID,
		)
	}

	_, _ = s.db.ExecContext(ctx,
		`UPDATE workflow_job_execution_log
		 SET completed_at = ?, status = ?, records_processed = ?, error_message = ?, execution_time_ms = ?
		 WHERE execution_id = ?`,
		completedStr, status, recordsProcessed, errMsg, durationMs, execID,
	)

	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO workflow_job_performance_metrics (id, job_id, window_label, avg_execution_ms, max_execution_ms, records_processed_total, failure_rate, retry_rate, samples, updated_at)
		 SELECT ?, ?, '30d',
		        COALESCE(AVG(execution_time_ms), 0),
		        COALESCE(MAX(execution_time_ms), 0),
		        COALESCE(SUM(records_processed), 0),
		        CASE WHEN COUNT(*) = 0 THEN 0 ELSE SUM(CASE WHEN status IN ('FAILED','TIMEOUT') THEN 1.0 ELSE 0.0 END)/COUNT(*) END,
		        CASE WHEN COUNT(*) = 0 THEN 0 ELSE SUM(retry_count)*1.0/COUNT(*) END,
		        COUNT(*),
		        ?
		 FROM workflow_job_execution_log WHERE job_id = ?
		 ON CONFLICT(job_id, window_label) DO UPDATE SET
		     avg_execution_ms = excluded.avg_execution_ms,
		     max_execution_ms = excluded.max_execution_ms,
		     records_processed_total = excluded.records_processed_total,
		     failure_rate = excluded.failure_rate,
		     retry_rate = excluded.retry_rate,
		     samples = excluded.samples,
		     updated_at = excluded.updated_at`,
		newID("jpm"), jobID, completedStr, jobID,
	)

	rec := models.JobExecutionRecord{
		ExecutionID:      execID,
		JobID:            jobID,
		StartedAt:        startedStr,
		CompletedAt:      completedStr,
		Status:           status,
		RecordsProcessed: recordsProcessed,
		ErrorMessage:     errMsg,
		ExecutionTimeMs:  durationMs,
		RetryCount:       recoveryRetries,
	}
	return rec, runErr
}

func (s *Service) executeCanonicalJobLogic(ctx context.Context, jobID, actor string) (int, error) {
	now := time.Now().UTC()
	nowStr := now.Format("2006-01-02 15:04:05")
	switch jobID {
	case "maintenance_cleanup":
		res, err := s.RunExceptionScanInternal(ctx, actor)
		if err != nil {
			return 0, err
		}
		_, _ = s.db.ExecContext(ctx,
			`DELETE FROM workflow_alert_signal_log WHERE datetime(created_at) < datetime('now', '-90 days')`,
		)
		// Cap retained failure forecasts to latest 1000 rows
		_, _ = s.db.ExecContext(ctx,
			`DELETE FROM workflow_failure_forecast
			 WHERE forecast_id NOT IN (
				SELECT forecast_id FROM workflow_failure_forecast ORDER BY datetime(forecast_time) DESC, rowid DESC LIMIT 1000
			 )`,
		)
		return res.CreatedCount + res.UpdatedCount + res.SelfHealedCount + res.EscalatedCount, nil

	case "automation_cycle":
		cycleStart := time.Now()
		res, err := s.RunExceptionScanInternal(ctx, actor)
		if err != nil {
			return 0, err
		}
		var maxCostUnits float64 = 500000
		var maxRecords int64 = 100
		var maxExecMs int64 = 120000
		_ = s.db.QueryRowContext(ctx,
			`SELECT max_cost_units_per_cycle, max_records_processed_per_cycle, max_execution_time_per_cycle_ms FROM automation_cost_limits WHERE id = 'default'`,
		).Scan(&maxCostUnits, &maxRecords, &maxExecMs)
		if maxCostUnits <= 0 {
			maxCostUnits = 500000
		}
		if maxRecords <= 0 {
			maxRecords = 100
		}
		if maxExecMs <= 0 {
			maxExecMs = 120000
		}

		costWeights := loadRuleCostWeightsMap(ctx, s.db)

		rulesRows, rErr := s.db.QueryContext(ctx,
			`SELECT d.tenant_id, d.rule_id, d.version_id, v.rule_definition_json, COALESCE(v.definition_sha256, ''), v.created_by,
			        COALESCE((SELECT c.sample_size_percentage FROM canary_rule_deployment c
			                  WHERE c.rule_id = d.rule_id AND c.version_id = d.version_id AND c.deployment_status = 'ACTIVE' LIMIT 1), 100),
			        COALESCE((SELECT 1 FROM canary_rule_deployment c
			                  WHERE c.rule_id = d.rule_id AND c.version_id = d.version_id AND c.deployment_status = 'ACTIVE' LIMIT 1), 0)
			 FROM workflow_environment_deployment_log d
			 JOIN workflow_decision_rules r ON r.rule_id = d.rule_id
			 JOIN workflow_rule_versions v ON v.rule_id = d.rule_id AND v.version_id = d.version_id
			 WHERE d.tenant_id = ?
			   AND d.environment_id = 'env-prod'
			   AND d.status = 'DEPLOYED'
			   AND r.enabled = 1
			 ORDER BY r.priority DESC, r.updated_at ASC`,
			defaultTenantID,
		)
		if rErr != nil {
			return 0, rErr
		}
		var activeRules []evalRuleSpec
		for rulesRows.Next() {
			var tenantID, ruleID, versionID, defJSON, defSHA, createdBy string
			var canaryPct float64
			var isCanary int
			if rulesRows.Scan(&tenantID, &ruleID, &versionID, &defJSON, &defSHA, &createdBy, &canaryPct, &isCanary) != nil {
				continue
			}
			var decErr error
			if defSHA == "" && !strings.EqualFold(createdBy, "system") {
				decErr = errors.New("missing definition_sha256 on non-system rule version")
			}
			var def immutableRuleDefinition
			if decErr == nil {
				def, _, decErr = decodeAndVerifyRuleDefinition(defJSON, defSHA)
			}
			if decErr != nil {
				_, _ = s.db.ExecContext(ctx,
					`INSERT INTO workflow_alert_signal_log (id, created_at, signal_type, severity, entity_id, message, details_json)
					 VALUES (?, ?, 'RULE_CHECKSUM_MISMATCH', 'CRITICAL', ?, ?, ?)`,
					newID("sig"), nowStr, ruleID,
					fmt.Sprintf("Skipped prod rule %s version %s due to integrity error: %v", ruleID, versionID, decErr),
					fmt.Sprintf(`{"ruleId":%q,"versionId":%q}`, ruleID, versionID),
				)
				continue
			}
			if !def.Enabled {
				continue
			}
			costWeight := resolveRuleCostWeight(costWeights, ruleID, def.ActionType, def.RuleType)
			activeRules = append(activeRules, evalRuleSpec{
				id:         ruleID,
				tenantID:   tenantID,
				versionID:  versionID,
				ruleType:   def.RuleType,
				cond:       def.ConditionExpression,
				action:     def.ActionType,
				priority:   def.Priority,
				canaryPct:  canaryPct,
				costWeight: costWeight,
				isCanary:   isCanary == 1,
			})
		}
		_ = rulesRows.Close()

		sort.SliceStable(activeRules, func(i, j int) bool {
			return activeRules[i].priority > activeRules[j].priority
		})

		actionsApplied, skippedIdempotent, usedCostUnits, stopReason, telemetry := s.evaluateAutomationRules(ctx, activeRules, int(maxRecords), maxCostUnits, maxExecMs, cycleStart, nowStr)

		for _, r := range activeRules {
			if ct, ok := telemetry[r.id+"|"+r.versionID]; ok {
				if r.isCanary {
					_ = s.recordCanaryCohortMetricsInternal(ctx, r.id, r.versionID, nowStr, ct.canary, ct.control)
				} else {
					_ = s.recordCanaryCohortMetricsInternal(ctx, r.id, r.versionID, nowStr, models.CanaryCohortMetrics{}, ct.canary)
				}
			}
		}
		autoRollbacks, _ := s.evaluateAllActiveCanaries(ctx, actor)

		resultJSON, _ := json.Marshal(map[string]any{
			"rulesEvaluated":      len(activeRules),
			"actionsApplied":      actionsApplied,
			"skippedIdempotent":   skippedIdempotent,
			"usedCostUnits":       usedCostUnits,
			"exceptionsEvaluated": res.ActiveOpenCount,
			"escalated":           res.EscalatedCount,
			"costCapHit":          stopReason != "",
			"stopReason":          stopReason,
			"autoRollbacks":       autoRollbacks,
		})
		_, _ = s.db.ExecContext(ctx,
			`INSERT INTO workflow_automation_log (automation_id, rule_id, action_taken, target_entity, executed_at, execution_result, estimated_cost_units)
			 VALUES (?, '', 'AUTOMATION_CYCLE_SUMMARY', 'system', ?, ?, ?)`,
			newID("cyc"), nowStr, string(resultJSON), usedCostUnits,
		)
		return res.ActiveOpenCount + actionsApplied, nil

	case "cost_metrics_aggregation":
		var actions int
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_automation_log WHERE date(executed_at) = date('now')`).Scan(&actions)
		today := now.Format("2006-01-02")
		benefitScore := float64(actions) * 1.5
		costScore := float64(actions) * 0.3
		efficiency := 1.0
		if costScore > 0 {
			efficiency = benefitScore / costScore
		}
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO daily_automation_economics_index (snapshot_date, benefit_score, cost_score, efficiency_gain, economics_index, factors_json, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(snapshot_date) DO UPDATE SET
			     benefit_score = excluded.benefit_score,
			     cost_score = excluded.cost_score,
			     efficiency_gain = excluded.efficiency_gain,
			     economics_index = excluded.economics_index`,
			today, benefitScore, costScore, efficiency, benefitScore-costScore,
			fmt.Sprintf(`{"actions":%d}`, actions), nowStr,
		)
		return actions, err

	case "deployment_safety_checks":
		checkStart := time.Now()
		autoRollbacks, _ := s.evaluateAllActiveCanaries(ctx, actor)
		var activeCanaries, recentFailures, criticalIncidents int
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM canary_rule_deployment WHERE deployment_status = 'ACTIVE'`).Scan(&activeCanaries)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_job_execution_log WHERE status IN ('FAILED','TIMEOUT') AND datetime(started_at) >= datetime('now', '-24 hours')`).Scan(&recentFailures)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_incidents WHERE status IN ('OPEN','ACKNOWLEDGED') AND severity IN ('CRITICAL','FATAL')`).Scan(&criticalIncidents)
		elapsedMs := time.Since(checkStart).Milliseconds()
		if elapsedMs < 1 {
			elapsedMs = 1
		}
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO performance_timing_metrics (id, category, entity_id, duration_ms, recorded_at, details_json)
			 VALUES (?, 'SAFETY_CHECK', 'canary_health', ?, ?, ?)`,
			newID("ptm"), elapsedMs, nowStr,
			fmt.Sprintf(`{"activeCanaries":%d,"recentFailures24h":%d,"criticalIncidents":%d,"autoRollbacks":%d}`, activeCanaries, recentFailures, criticalIncidents, autoRollbacks),
		)
		return activeCanaries, err

	case "risk_evaluation":
		evalStart := time.Now()
		rRows, err := s.db.QueryContext(ctx,
			`SELECT r.rule_id,
			        COALESCE((SELECT v.version_id FROM workflow_rule_versions v WHERE v.rule_id = r.rule_id ORDER BY v.version_number DESC LIMIT 1), '')
			 FROM workflow_decision_rules r`,
		)
		if err != nil {
			return 0, err
		}
		type rulePair struct{ id, verID string }
		var pairs []rulePair
		for rRows.Next() {
			var rp rulePair
			if rRows.Scan(&rp.id, &rp.verID) == nil {
				pairs = append(pairs, rp)
			}
		}
		_ = rRows.Close()

		highRiskCount := 0
		for _, rp := range pairs {
			if rp.verID == "" {
				continue
			}
			risk, rErr := s.evaluateRuleRiskInternal(ctx, rp.id, rp.verID)
			if rErr == nil && risk.BlocksProd {
				highRiskCount++
			}
		}
		elapsedMs := time.Since(evalStart).Milliseconds()
		if elapsedMs < 1 {
			elapsedMs = 1
		}
		_, _ = s.db.ExecContext(ctx,
			`INSERT INTO performance_timing_metrics (id, category, entity_id, duration_ms, recorded_at, details_json)
			 VALUES (?, 'RISK_EVALUATION', 'rule_risk_rollup', ?, ?, ?)`,
			newID("ptm"), elapsedMs, nowStr,
			fmt.Sprintf(`{"rulesChecked":%d,"highOrCritical":%d}`, len(pairs), highRiskCount),
		)
		return len(pairs), nil

	case "observability_update":
		alerts, forecasts, err := s.runObservabilityAndIncidentPipeline(ctx)
		return alerts + forecasts, err

	default:
		return 0, fmt.Errorf("unsupported background job %q", jobID)
	}
}

type evalRuleSpec struct {
	id, tenantID, versionID, ruleType, cond, action string
	priority                                        int
	canaryPct, costWeight                           float64
	isCanary                                        bool
}

type ruleCohortAccumulator struct {
	canary        models.CanaryCohortMetrics
	control       models.CanaryCohortMetrics
	canaryDurSum  float64
	controlDurSum float64
}

func (s *Service) evaluateAutomationRules(
	ctx context.Context,
	rules []evalRuleSpec,
	maxRecords int,
	maxCostUnits float64,
	maxExecMs int64,
	cycleStart time.Time,
	nowStr string,
) (int, int, float64, string, map[string]*ruleCohortAccumulator) {
	telemetry := map[string]*ruleCohortAccumulator{}
	finalize := func(applied, skipped int, cost float64, reason string) (int, int, float64, string, map[string]*ruleCohortAccumulator) {
		for _, acc := range telemetry {
			if acc.canary.AttemptedCount > 0 {
				acc.canary.AvgDurationMs = acc.canaryDurSum / float64(acc.canary.AttemptedCount)
			}
			if acc.control.AttemptedCount > 0 {
				acc.control.AvgDurationMs = acc.controlDurSum / float64(acc.control.AttemptedCount)
			}
		}
		return applied, skipped, cost, reason, telemetry
	}

	if len(rules) == 0 || maxRecords <= 0 || maxCostUnits <= 0 {
		return finalize(0, 0, 0, "")
	}
	scanLimit := maxRecords * 10
	if scanLimit < 500 {
		scanLimit = 500
	}
	if scanLimit > 50000 {
		scanLimit = 50000
	}
	exRows, err := s.db.QueryContext(ctx,
		`SELECT id, exception_type, entity_id, status, priority, COALESCE(assigned_to, ''), sla_status, lifecycle_generation
		 FROM exception_cases
		 WHERE status IN ('OPEN', 'IN_PROGRESS')
		 ORDER BY created_at ASC LIMIT ?`,
		scanLimit,
	)
	if err != nil {
		return finalize(0, 0, 0, "")
	}
	type exItem struct {
		id, exType, entityID, status, priority, assignedTo, slaStatus string
		lifecycleGen                                                  int
	}
	var cases []exItem
	for exRows.Next() {
		var ex exItem
		if exRows.Scan(&ex.id, &ex.exType, &ex.entityID, &ex.status, &ex.priority, &ex.assignedTo, &ex.slaStatus, &ex.lifecycleGen) == nil {
			cases = append(cases, ex)
		}
	}
	_ = exRows.Close()

	actionsApplied := 0
	skippedIdempotent := 0
	usedCostUnits := 0.0
	for _, r := range rules {
		telKey := r.id + "|" + r.versionID
		acc := telemetry[telKey]
		if acc == nil {
			acc = &ruleCohortAccumulator{}
			telemetry[telKey] = acc
		}
		actionUpper := strings.ToUpper(strings.TrimSpace(r.action))
		if actionUpper == "" {
			actionUpper = strings.ToUpper(strings.TrimSpace(r.ruleType))
		}
		for i := range cases {
			if actionsApplied >= maxRecords {
				return finalize(actionsApplied, skippedIdempotent, usedCostUnits, "MAX_RECORDS_REACHED")
			}
			if time.Since(cycleStart).Milliseconds() >= maxExecMs {
				return finalize(actionsApplied, skippedIdempotent, usedCostUnits, "MAX_EXECUTION_TIME_REACHED")
			}
			if usedCostUnits+r.costWeight > maxCostUnits {
				return finalize(actionsApplied, skippedIdempotent, usedCostUnits, "MAX_COST_UNITS_REACHED")
			}
			ex := &cases[i]
			matchStart := time.Now()
			if !s.doesRuleMatchException(ctx, actionUpper, r.cond, ex.exType, ex.entityID, ex.status, ex.priority, ex.assignedTo, ex.slaStatus) {
				continue
			}
			if !ShouldRouteCanaryVersion(r.tenantID, r.id, r.versionID, ex.id, r.canaryPct) {
				acc.control.AttemptedCount++
				acc.control.SuccessCount++
				acc.controlDurSum += float64(time.Since(matchStart).Microseconds()) / 1000.0
				continue
			}

			execStart := time.Now()
			appliedNow := false
			dupSkip := false
			txErr := transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
				autID := newID("aut")
				idemRes, iErr := tx.ExecContext(ctx,
					`INSERT OR IGNORE INTO workflow_rule_action_idempotency
					 (id, tenant_id, rule_id, rule_version_id, entity_type, entity_id, action_type, lifecycle_generation, execution_id, created_at)
					 VALUES (?, ?, ?, ?, 'exception_case', ?, ?, ?, ?, ?)`,
					newID("idm"), r.tenantID, r.id, r.versionID, ex.id, actionUpper, ex.lifecycleGen, autID, nowStr,
				)
				if iErr != nil {
					return iErr
				}
				if n, _ := idemRes.RowsAffected(); n == 0 {
					dupSkip = true
					return nil
				}

				switch actionUpper {
				case "AUTO_RESOLVE":
					res, uErr := tx.ExecContext(ctx,
						`UPDATE exception_cases SET status = 'RESOLVED', resolved_by = 'automation', resolved_at = ?, updated_at = ?
						 WHERE id = ? AND status IN ('OPEN', 'IN_PROGRESS')`,
						nowStr, nowStr, ex.id,
					)
					if uErr != nil {
						return uErr
					}
					if n, _ := res.RowsAffected(); n == 0 {
						return nil
					}
					if _, eErr := tx.ExecContext(ctx,
						`INSERT INTO exception_lifecycle_events (id, exception_case_id, old_status, new_status, actor, created_at) VALUES (?, ?, ?, 'RESOLVED', ?, ?)`,
						newID("exev"), ex.id, ex.status, "rule:"+r.id+":"+r.versionID, nowStr,
					); eErr != nil {
						return eErr
					}
					if _, lErr := tx.ExecContext(ctx,
						`INSERT INTO workflow_automation_log (automation_id, rule_id, action_taken, target_entity, executed_at, execution_result, estimated_cost_units)
						 VALUES (?, ?, 'AUTO_RESOLVE', ?, ?, '{"status":"RESOLVED"}', ?)`,
						autID, r.id, ex.id, nowStr, r.costWeight,
					); lErr != nil {
						return lErr
					}
					ex.status = "RESOLVED"
					appliedNow = true

				case "AUTO_ASSIGN":
					res, uErr := tx.ExecContext(ctx,
						`UPDATE exception_cases SET status = 'IN_PROGRESS', assigned_to = 'admin', assigned_at = ?, assignment_method = 'AUTO', updated_at = ?
						 WHERE id = ? AND status = 'OPEN' AND COALESCE(assigned_to, '') = ''`,
						nowStr, nowStr, ex.id,
					)
					if uErr != nil {
						return uErr
					}
					if n, _ := res.RowsAffected(); n == 0 {
						return nil
					}
					if _, lErr := tx.ExecContext(ctx,
						`INSERT INTO workflow_automation_log (automation_id, rule_id, action_taken, target_entity, executed_at, execution_result, estimated_cost_units)
						 VALUES (?, ?, 'AUTO_ASSIGN', ?, ?, '{"assignedTo":"admin"}', ?)`,
						autID, r.id, ex.id, nowStr, r.costWeight,
					); lErr != nil {
						return lErr
					}
					ex.status = "IN_PROGRESS"
					ex.assignedTo = "admin"
					appliedNow = true

				case "PRIORITY_ADJUST":
					res, uErr := tx.ExecContext(ctx,
						`UPDATE exception_cases SET priority = 'HIGH', updated_at = ?
						 WHERE id = ? AND sla_status = 'BREACHED' AND priority NOT IN ('CRITICAL', 'HIGH')`,
						nowStr, ex.id,
					)
					if uErr != nil {
						return uErr
					}
					if n, _ := res.RowsAffected(); n == 0 {
						return nil
					}
					if _, lErr := tx.ExecContext(ctx,
						`INSERT INTO workflow_automation_log (automation_id, rule_id, action_taken, target_entity, executed_at, execution_result, estimated_cost_units)
						 VALUES (?, ?, 'PRIORITY_ADJUST', ?, ?, '{"priority":"HIGH"}', ?)`,
						autID, r.id, ex.id, nowStr, r.costWeight,
					); lErr != nil {
						return lErr
					}
					ex.priority = "HIGH"
					appliedNow = true
				}
				return nil
			})
			execDurMs := float64(time.Since(execStart).Microseconds()) / 1000.0
			if dupSkip {
				skippedIdempotent++
				acc.canary.SkippedCount++
				continue
			}
			if txErr != nil {
				acc.canary.AttemptedCount++
				acc.canary.FailureCount++
				acc.canaryDurSum += execDurMs
				continue
			}
			if appliedNow {
				actionsApplied++
				usedCostUnits += r.costWeight
				acc.canary.AttemptedCount++
				acc.canary.SuccessCount++
				acc.canary.CostUnits += r.costWeight
				acc.canaryDurSum += execDurMs
			}
		}
	}
	return finalize(actionsApplied, skippedIdempotent, usedCostUnits, "")
}

func (s *Service) shouldAutoResolveException(ctx context.Context, condExpr, exType, entityID string) bool {
	condLower := strings.ToLower(condExpr)
	switch exType {
	case "OVERDUE_ETA":
		if !strings.Contains(condLower, "shipment_delivered") && !strings.Contains(condLower, "overdue_eta") {
			return false
		}
		var cnt int
		_ = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM shipments WHERE id = ? AND (deleted_at IS NOT NULL OR LOWER(COALESCE(status, '')) IN ('delivered', 'cancelled'))`,
			entityID,
		).Scan(&cnt)
		return cnt > 0
	case "MISSING_BOE":
		if !strings.Contains(condLower, "has_boe") && !strings.Contains(condLower, "missing_boe") {
			return false
		}
		var cnt int
		_ = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM boe_calculations WHERE shipment_id = ? AND deleted_at IS NULL`,
			entityID,
		).Scan(&cnt)
		return cnt > 0
	case "MISSING_EXPENSE":
		if !strings.Contains(condLower, "has_expenses") && !strings.Contains(condLower, "missing_expense") {
			return false
		}
		var cnt int
		_ = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM expenses WHERE shipment_id = ? AND deleted_at IS NULL`,
			entityID,
		).Scan(&cnt)
		return cnt > 0
	default:
		return false
	}
}

// RunDailyOperationsTick runs timeout recovery (circuit breaker), due scheduled jobs, alert correlation/suppression, burst detection, stabilization, and 30m forecasting.
// Safe to invoke from the background scheduler.
func (s *Service) RunDailyOperationsTick(actor string) error {
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return err
	}
	ctx := context.Background()
	if err := s.detectAndRecoverTimedOutJobs(ctx); err != nil {
		return err
	}
	s.runDueScheduledJobs(ctx, actor)
	_, _, err := s.runObservabilityAndIncidentPipeline(ctx)
	return err
}

func (s *Service) runDueScheduledJobs(ctx context.Context, actor string) {
	// Ordered respect for canonical dependencies: maintenance_cleanup -> automation_cycle -> cost_metrics_aggregation -> deployment_safety_checks -> risk_evaluation.
	// observability_update runs on every tick via runObservabilityAndIncidentPipeline above.
	orderedJobs := []string{
		"maintenance_cleanup",
		"automation_cycle",
		"cost_metrics_aggregation",
		"deployment_safety_checks",
		"risk_evaluation",
	}
	for _, jobID := range orderedJobs {
		var enabled int
		if err := s.db.QueryRowContext(ctx, `SELECT is_enabled FROM workflow_background_jobs WHERE job_id = ?`, jobID).Scan(&enabled); err != nil || enabled != 1 {
			continue
		}
		if s.isJobGuardStopped(ctx, jobID) {
			continue
		}
		var intervalMin int
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(expected_interval_minutes, 1440) FROM workflow_job_schedule_expectations WHERE job_id = ?`, jobID).Scan(&intervalMin)
		if intervalMin <= 0 {
			intervalMin = 1440
		}
		var lastCompleted string
		_ = s.db.QueryRowContext(ctx,
			`SELECT COALESCE(completed_at, '') FROM workflow_job_execution_log
			 WHERE job_id = ? AND status = 'COMPLETED'
			 ORDER BY datetime(started_at) DESC, rowid DESC LIMIT 1`,
			jobID,
		).Scan(&lastCompleted)

		due := lastCompleted == ""
		if !due {
			if t, parseErr := time.Parse("2006-01-02 15:04:05", lastCompleted); parseErr == nil {
				due = time.Since(t) >= time.Duration(intervalMin)*time.Minute
			}
		}
		if due {
			_, _ = s.runJobInternal(ctx, jobID, actor)
		}
	}
}

func (s *Service) detectAndRecoverTimedOutJobs(ctx context.Context) error {
	now := time.Now().UTC()
	nowStr := now.Format("2006-01-02 15:04:05")

	rows, err := s.db.QueryContext(ctx,
		`SELECT e.execution_id, e.job_id, e.started_at, j.expected_duration_ms, j.max_retries
		 FROM workflow_job_execution_log e
		 JOIN workflow_background_jobs j ON j.job_id = e.job_id
		 WHERE e.status = 'RUNNING'`,
	)
	if err != nil {
		return err
	}
	type runningJob struct {
		execID     string
		jobID      string
		startedAt  string
		expectedMs int64
		maxRetries int
	}
	var candidates []runningJob
	for rows.Next() {
		var r runningJob
		if err := rows.Scan(&r.execID, &r.jobID, &r.startedAt, &r.expectedMs, &r.maxRetries); err != nil {
			rows.Close()
			return err
		}
		candidates = append(candidates, r)
	}
	rows.Close()

	for _, c := range candidates {
		started, parseErr := time.Parse("2006-01-02 15:04:05", c.startedAt)
		if parseErr != nil {
			continue
		}
		timeoutDur := 2 * time.Hour
		if twice := time.Duration(c.expectedMs*2) * time.Millisecond; twice > timeoutDur {
			timeoutDur = twice
		}
		elapsed := now.Sub(started)
		if elapsed < timeoutDur {
			continue
		}

		alertID := newID("jfa")
		_, _ = s.db.ExecContext(ctx,
			`UPDATE workflow_job_execution_log
			 SET status = 'TIMEOUT', completed_at = ?, execution_time_ms = ?, error_message = 'job execution exceeded timeout threshold'
			 WHERE execution_id = ?`,
			nowStr, elapsed.Milliseconds(), c.execID,
		)
		_, _ = s.db.ExecContext(ctx,
			`INSERT INTO workflow_job_failure_alerts (alert_id, job_id, detected_at, alert_type, details_json)
			 VALUES (?, ?, ?, 'JOB_TIMEOUT', ?)`,
			alertID, c.jobID, nowStr, fmt.Sprintf(`{"executionId":%q,"elapsedMs":%d}`, c.execID, elapsed.Milliseconds()),
		)

		var priorAttempts int
		_ = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM workflow_job_recovery_log WHERE job_id = ? AND datetime(recovery_time) >= datetime('now', '-24 hours')`,
			c.jobID,
		).Scan(&priorAttempts)

		attemptNum := priorAttempts + 1
		if priorAttempts >= c.maxRetries {
			_, _ = s.db.ExecContext(ctx,
				`UPDATE workflow_background_jobs SET is_enabled = 0, updated_at = ? WHERE job_id = ?`,
				nowStr, c.jobID,
			)
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO workflow_job_recovery_log (recovery_id, job_id, alert_id, recovery_time, result, attempt_count, error_message)
				 VALUES (?, ?, ?, ?, 'RECOVERY_GUARD_STOP', ?, 'max_retries circuit breaker engaged')`,
				newID("rec"), c.jobID, alertID, nowStr, attemptNum,
			)
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO workflow_alert_signal_log (id, created_at, signal_type, severity, entity_id, message, details_json)
				 VALUES (?, ?, 'JOB_RECOVERY_GUARD_STOP', 'CRITICAL', ?, ?, ?)`,
				newID("sig"), nowStr, c.jobID,
				fmt.Sprintf("Recovery circuit breaker engaged for %s after %d attempts", c.jobID, attemptNum),
				fmt.Sprintf(`{"jobId":%q,"maxRetries":%d}`, c.jobID, c.maxRetries),
			)
			continue
		}

		_, _ = s.db.ExecContext(ctx,
			`INSERT INTO workflow_job_recovery_log (recovery_id, job_id, alert_id, recovery_time, result, attempt_count, error_message)
			 VALUES (?, ?, ?, ?, 'RECOVERED', ?, '')`,
			newID("rec"), c.jobID, alertID, nowStr, attemptNum,
		)
	}
	return nil
}

func (s *Service) runObservabilityAndIncidentPipeline(ctx context.Context) (int, int, error) {
	now := time.Now().UTC()
	nowStr := now.Format("2006-01-02 15:04:05")
	today := now.Format("2006-01-02")

	// 1. Detect failure bursts (>= 3 signals within 15 minutes per module/signal_type)
	burstRows, err := s.db.QueryContext(ctx,
		`SELECT COALESCE(entity_id, 'system'), signal_type, COUNT(*) as cnt, MAX(severity)
		 FROM workflow_alert_signal_log
		 WHERE datetime(created_at) >= datetime('now', '-15 minutes')
		 GROUP BY COALESCE(entity_id, 'system'), signal_type
		 HAVING COUNT(*) >= 3`,
	)
	if err != nil {
		return 0, 0, err
	}
	type burstCand struct {
		module   string
		sigType  string
		count    int
		severity string
	}
	var bursts []burstCand
	for burstRows.Next() {
		var b burstCand
		if err := burstRows.Scan(&b.module, &b.sigType, &b.count, &b.severity); err != nil {
			burstRows.Close()
			return 0, 0, err
		}
		bursts = append(bursts, b)
	}
	burstRows.Close()

	for _, b := range bursts {
		var existingBurst int
		_ = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM workflow_failure_burst_log
			 WHERE source_module = ? AND event_type = ? AND datetime(burst_end_time) >= datetime('now', '-15 minutes')`,
			b.module, b.sigType,
		).Scan(&existingBurst)
		if existingBurst == 0 {
			burstID := newID("brs")
			startStr := now.Add(-15 * time.Minute).Format("2006-01-02 15:04:05")
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO workflow_failure_burst_log (burst_id, source_module, event_type, burst_start_time, burst_end_time, event_count, baseline_rate, current_rate, severity, root_cause_hint, confidence_score)
				 VALUES (?, ?, ?, ?, ?, ?, 0.2, ?, ?, 'repeated alert signal spike within 15m window', 0.85)`,
				burstID, b.module, b.sigType, startStr, nowStr, b.count, float64(b.count)/15.0, b.severity,
			)
			// Open a bounded 15m (10-30m policy) suppression window
			suppEnd := now.Add(15 * time.Minute).Format("2006-01-02 15:04:05")
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO workflow_incident_suppression (suppression_id, source_module, event_type, suppression_start, suppression_end, last_activity_at, suppressed_event_count, confidence_score, reason, created_at)
				 VALUES (?, ?, ?, ?, ?, ?, 0, 0.85, 'Auto burst dampening (15m window)', ?)`,
				newID("sup"), b.module, b.sigType, nowStr, suppEnd, nowStr, nowStr,
			)
		}
	}

	// 2. Promote recent alert signals into correlated incidents unless actively suppressed
	sigRows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.signal_type, s.severity, COALESCE(s.entity_id, 'system'), s.created_at, s.message
		 FROM workflow_alert_signal_log s
		 LEFT JOIN workflow_incidents i ON i.linked_alert_id = s.id
		 WHERE i.incident_id IS NULL AND datetime(s.created_at) >= datetime('now', '-24 hours')
		 ORDER BY datetime(s.created_at) ASC`,
	)
	if err != nil {
		return 0, 0, err
	}
	type sigItem struct {
		id, sigType, sev, entityID, createdAt, msg string
	}
	var pendingSignals []sigItem
	for sigRows.Next() {
		var si sigItem
		if err := sigRows.Scan(&si.id, &si.sigType, &si.sev, &si.entityID, &si.createdAt, &si.msg); err != nil {
			sigRows.Close()
			return 0, 0, err
		}
		pendingSignals = append(pendingSignals, si)
	}
	sigRows.Close()

	promoted := 0
	for _, si := range pendingSignals {
		var suppID string
		err := s.db.QueryRowContext(ctx,
			`SELECT suppression_id FROM workflow_incident_suppression
			 WHERE source_module = ? AND event_type = ? AND datetime(suppression_end) >= datetime(?)
			 ORDER BY datetime(suppression_end) DESC LIMIT 1`,
			si.entityID, si.sigType, nowStr,
		).Scan(&suppID)
		if err == nil && suppID != "" {
			_, _ = s.db.ExecContext(ctx,
				`UPDATE workflow_incident_suppression SET suppressed_event_count = suppressed_event_count + 1, last_activity_at = ? WHERE suppression_id = ?`,
				nowStr, suppID,
			)
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO workflow_incident_suppression_metrics (metric_date, alerts_suppressed, suppression_windows, noise_reduction_gain, confidence_score, updated_at)
				 VALUES (?, 1, 1, 0.25, 0.80, ?)
				 ON CONFLICT(metric_date) DO UPDATE SET
				     alerts_suppressed = alerts_suppressed + 1,
				     updated_at = excluded.updated_at`,
				today, nowStr,
			)
			continue
		}

		// Correlate with existing OPEN incident of same type/entity if present
		corrKey := si.entityID + ":" + si.sigType
		var openIncID string
		_ = s.db.QueryRowContext(ctx,
			`SELECT incident_id FROM workflow_incidents
			 WHERE correlation_key = ? AND status IN ('OPEN', 'ACKNOWLEDGED')
			 ORDER BY datetime(created_at) DESC LIMIT 1`,
			corrKey,
		).Scan(&openIncID)

		if openIncID != "" {
			_, _ = s.db.ExecContext(ctx,
				`UPDATE workflow_incidents
				 SET correlated_event_count = correlated_event_count + 1,
				     last_correlated_at = ?
				 WHERE incident_id = ?`,
				nowStr, openIncID,
			)
			continue
		}

		incID := newID("inc")
		corrID := newID("corr")
		errCtx, _ := json.Marshal(map[string]any{"entityId": si.entityID, "message": si.msg})
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO workflow_incidents (incident_id, created_at, severity, status, error_context_json, source_module, linked_alert_id, trigger_event_type, correlation_id, correlation_key, correlated_event_count, last_correlated_at)
			 VALUES (?, ?, ?, 'OPEN', ?, ?, ?, ?, ?, ?, 1, ?)`,
			incID, nowStr, si.sev, string(errCtx), si.entityID, si.id, si.sigType, corrID, corrKey, nowStr,
		)
		if err == nil {
			promoted++
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO workflow_incident_metrics (metric_date, incidents_created_today, updated_at)
				 VALUES (?, 1, ?)
				 ON CONFLICT(metric_date) DO UPDATE SET incidents_created_today = incidents_created_today + 1, updated_at = excluded.updated_at`,
				today, nowStr,
			)
		}
	}

	// 3. Stabilization & 30m predictive forecast computation
	var recentAlerts, openIncidents, resolvedIncidents int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_alert_signal_log WHERE datetime(created_at) >= datetime('now', '-30 minutes')`).Scan(&recentAlerts)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_incidents WHERE status IN ('OPEN', 'ACKNOWLEDGED')`).Scan(&openIncidents)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_incidents WHERE status = 'RESOLVED'`).Scan(&resolvedIncidents)

	totalInc := openIncidents + resolvedIncidents
	stabilityScore := 100.0
	if totalInc > 0 {
		stabilityScore = math.Max(10, 100.0-float64(openIncidents)*12.0-float64(recentAlerts)*2.0)
	}
	if recentAlerts == 0 && openIncidents == 0 {
		_, _ = s.db.ExecContext(ctx,
			`INSERT INTO workflow_incident_stabilization (stabilization_id, source_module, event_type, stabilization_start, stabilization_confirmed, stability_duration_minutes, confidence_score, created_at)
			 VALUES (?, 'workflow', 'ALL', ?, ?, 30, 0.95, ?)`,
			newID("stb"), nowStr, nowStr, nowStr,
		)
	}
	_, _ = s.db.ExecContext(ctx,
		`UPDATE workflow_system_stability_score
		 SET stability_score = ?, successful_stabilizations = ?, total_incidents = ?, updated_at = ?
		 WHERE id = 1`,
		stabilityScore, resolvedIncidents, totalInc, nowStr,
	)

	prob := math.Min(0.95, float64(recentAlerts)*0.12+float64(openIncidents)*0.08)
	conf := 0.78
	if recentAlerts == 0 && openIncidents == 0 {
		prob = 0.05
		conf = 0.90
	}
	detailsBytes, _ := json.Marshal(map[string]any{
		"recentAlerts30m": recentAlerts,
		"openIncidents":   openIncidents,
		"horizonMinutes":  30,
	})
	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO workflow_failure_forecast (forecast_id, source_module, event_type, forecast_time, predicted_failure_probability, confidence_score, forecast_horizon_minutes, details_json, primary_trigger)
		 VALUES (?, 'workflow', 'SYSTEM_HEALTH', ?, ?, ?, 30, ?, ?)`,
		newID("fct"), nowStr, prob, conf, string(detailsBytes),
		fmt.Sprintf("recent_signals=%d,open_incidents=%d", recentAlerts, openIncidents),
	)
	highRisk := 0
	if prob >= 0.5 {
		highRisk = 1
	}
	_, _ = s.db.ExecContext(ctx,
		`UPDATE workflow_forecast_risk_score SET forecast_risk_score = ?, high_risk_forecasts = high_risk_forecasts + ?, total_forecasts = total_forecasts + 1, updated_at = ? WHERE id = 1`,
		prob*100.0, highRisk, nowStr,
	)

	return promoted, 1, nil
}

// GetOperationsOverview aggregates live workflow health, jobs, forecasts, and active incidents for /admin/operations-center.
func (s *Service) GetOperationsOverview() (models.OperationsOverview, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationOpsCenter); err != nil {
		return models.OperationsOverview{}, err
	}
	jobs, err := s.listBackgroundJobsInternal()
	if err != nil {
		return models.OperationsOverview{}, err
	}
	incidents, err := s.listIncidentsInternal("all")
	if err != nil {
		return models.OperationsOverview{}, err
	}

	var out models.OperationsOverview
	out.Jobs = jobs
	out.Incidents = incidents
	out.RestoreLocked = s.restoreCtrl.IsRestoring()
	out.SystemStability = 100.0
	_ = s.db.QueryRow(`SELECT COALESCE(stability_score, 100) FROM workflow_system_stability_score WHERE id = 1`).Scan(&out.SystemStability)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM exception_cases WHERE status IN ('OPEN', 'IN_PROGRESS')`).Scan(&out.OpenExceptions)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM exception_cases WHERE status IN ('OPEN', 'IN_PROGRESS') AND sla_status = 'BREACHED'`).Scan(&out.BreachedSLACount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM workflow_incidents WHERE status IN ('OPEN', 'ACKNOWLEDGED')`).Scan(&out.OpenIncidents)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM workflow_incident_suppression WHERE datetime(suppression_end) >= datetime('now')`).Scan(&out.ActiveSuppressions)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM workflow_failure_burst_log WHERE datetime(burst_end_time) >= datetime('now', '-24 hours')`).Scan(&out.RecentBursts)

	out.WorkflowHealthScore = math.Max(0, 100.0-float64(out.BreachedSLACount)*5.0-float64(out.OpenIncidents)*10.0)

	fRows, err := s.db.Query(
		`SELECT forecast_id, source_module, event_type, forecast_time, predicted_failure_probability, confidence_score, forecast_horizon_minutes, details_json
		 FROM workflow_failure_forecast ORDER BY datetime(forecast_time) DESC, rowid DESC LIMIT 10`,
	)
	if err == nil {
		defer fRows.Close()
		for fRows.Next() {
			var f models.FailureForecast
			if err := fRows.Scan(&f.ForecastID, &f.SourceModule, &f.EventType, &f.ForecastTime,
				&f.PredictedProbability, &f.ConfidenceScore, &f.HorizonMinutes, &f.DetailsJSON); err == nil {
				out.Forecasts = append(out.Forecasts, f)
			}
		}
	}
	if out.Forecasts == nil {
		out.Forecasts = []models.FailureForecast{}
	}
	return out, nil
}

// ListIncidents lists workflow incidents optionally filtered by status.
func (s *Service) ListIncidents(status string) ([]models.WorkflowIncident, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationOpsCenter); err != nil {
		return nil, err
	}
	return s.listIncidentsInternal(status)
}

func (s *Service) listIncidentsInternal(status string) ([]models.WorkflowIncident, error) {
	query := `SELECT i.incident_id, i.linked_alert_id, i.trigger_event_type, i.severity, i.status,
	                 COALESCE((SELECT h.notes FROM workflow_incident_history h WHERE h.incident_id = i.incident_id AND h.event_type = 'ASSIGNED' ORDER BY datetime(h.event_timestamp) DESC LIMIT 1), 'unassigned'),
	                 i.created_at, COALESCE(i.last_correlated_at, i.created_at), COALESCE(i.resolved_at, ''), COALESCE(i.root_cause_summary, ''), i.correlated_event_count
	          FROM workflow_incidents i`
	var args []any
	if sTrim := strings.TrimSpace(status); sTrim != "" && !strings.EqualFold(sTrim, "all") {
		query += " WHERE i.status = ?"
		args = append(args, strings.ToUpper(sTrim))
	}
	query += " ORDER BY datetime(i.created_at) DESC LIMIT 100"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.WorkflowIncident
	for rows.Next() {
		var inc models.WorkflowIncident
		if err := rows.Scan(&inc.IncidentID, &inc.AlertID, &inc.IncidentType, &inc.Severity, &inc.Status,
			&inc.AssignedTo, &inc.CreatedAt, &inc.UpdatedAt, &inc.ResolvedAt, &inc.ResolutionNotes, &inc.EscalationCount); err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	if out == nil {
		out = []models.WorkflowIncident{}
	}
	return out, rows.Err()
}

// AssignIncident assigns an incident to an operator and moves it to ACKNOWLEDGED if currently OPEN.
func (s *Service) AssignIncident(incidentID, assignee string) error {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationMutate, security.PermAutomationOpsCenter)
	if err != nil {
		return err
	}
	if strings.TrimSpace(assignee) == "" {
		return errors.New("assignee is required")
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	trimmedAssignee := strings.TrimSpace(assignee)
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE workflow_incidents
			 SET status = CASE WHEN status = 'OPEN' THEN 'ACKNOWLEDGED' ELSE status END
			 WHERE incident_id = ?`,
			incidentID,
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("incident %q not found", incidentID)
		}
		_, err = tx.Exec(
			`INSERT INTO workflow_incident_history (history_id, incident_id, event_type, event_timestamp, notes, details_json)
			 VALUES (?, ?, 'ASSIGNED', ?, ?, ?)`,
			newID("inh"), incidentID, nowStr, trimmedAssignee, fmt.Sprintf(`{"assignedBy":%q}`, sess.UserID),
		)
		return err
	})
	if err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("workflow_incidents", incidentID, "ASSIGN", sess.UserID, nil, map[string]string{"assignedTo": trimmedAssignee}, nil)
	return nil
}

// AddIncidentNote attaches a root-cause investigation note to an incident in workflow_incident_history (required >= 10 chars before resolve).
func (s *Service) AddIncidentNote(incidentID, noteText string) (models.IncidentNote, error) {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationMutate, security.PermAutomationOpsCenter)
	if err != nil {
		return models.IncidentNote{}, err
	}
	trimmed := strings.TrimSpace(noteText)
	if trimmed == "" {
		return models.IncidentNote{}, errors.New("incident note cannot be empty")
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	note := models.IncidentNote{
		NoteID:     newID("inote"),
		IncidentID: incidentID,
		NoteText:   trimmed,
		CreatedBy:  sess.UserID,
		CreatedAt:  nowStr,
	}
	_, err = s.db.Exec(
		`INSERT INTO workflow_incident_history (history_id, incident_id, event_type, event_timestamp, notes, details_json)
		 VALUES (?, ?, 'NOTE', ?, ?, ?)`,
		note.NoteID, note.IncidentID, note.CreatedAt, note.NoteText, fmt.Sprintf(`{"createdBy":%q}`, note.CreatedBy),
	)
	if err == nil {
		_ = s.audit.RecordTableAudit("workflow_incident_history", note.NoteID, "INSERT", sess.UserID, nil, note, nil)
	}
	return note, err
}

// ResolveIncident enforces the multi-signal Incident Resolve Gate:
// 1. At least one investigation note >= 10 characters exists in workflow_incident_history.
// 2. Resolution summary is >= 50 characters.
// 3. Quiet correlation window of 10 minutes with no recent alert signals (unless SkipQuietWindowCheck is true).
func (s *Service) ResolveIncident(in models.ResolveIncidentInput) error {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationMutate, security.PermAutomationOpsCenter)
	if err != nil {
		return err
	}
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return err
	}

	notesSummary := strings.TrimSpace(in.ResolutionNotes)
	if len(notesSummary) < 50 {
		return fmt.Errorf("resolution summary must be at least 50 characters (got %d)", len(notesSummary))
	}

	var validNoteCount int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM workflow_incident_history WHERE incident_id = ? AND event_type = 'NOTE' AND LENGTH(TRIM(COALESCE(notes, ''))) >= 10`,
		in.IncidentID,
	).Scan(&validNoteCount); err != nil {
		return err
	}
	if validNoteCount == 0 {
		return errors.New("resolve blocked: attach at least one investigation note (>= 10 characters) before resolving")
	}

	if !in.SkipQuietWindowCheck {
		var recentSignals int
		if err := s.db.QueryRow(
			`SELECT COUNT(*) FROM workflow_alert_signal_log WHERE datetime(created_at) >= datetime('now', '-10 minutes')`,
		).Scan(&recentSignals); err != nil {
			return err
		}
		if recentSignals > 0 {
			return fmt.Errorf("resolve blocked: %d alert signal(s) arrived within the last 10 minutes (quiet window required or override)", recentSignals)
		}
	}

	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	today := time.Now().UTC().Format("2006-01-02")
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE workflow_incidents
			 SET status = 'RESOLVED', resolved_at = ?, root_cause_summary = ?
			 WHERE incident_id = ? AND status != 'RESOLVED'`,
			nowStr, notesSummary, in.IncidentID,
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("incident %q not found or already resolved", in.IncidentID)
		}
		_, _ = tx.Exec(
			`INSERT INTO workflow_incident_history (history_id, incident_id, event_type, event_timestamp, notes, details_json)
			 VALUES (?, ?, 'RESOLVED', ?, ?, ?)`,
			newID("inh"), in.IncidentID, nowStr, notesSummary, fmt.Sprintf(`{"resolvedBy":%q}`, sess.UserID),
		)
		_, err = tx.Exec(
			`INSERT INTO workflow_incident_metrics (metric_date, incidents_resolved_today, updated_at)
			 VALUES (?, 1, ?)
			 ON CONFLICT(metric_date) DO UPDATE SET incidents_resolved_today = incidents_resolved_today + 1, updated_at = excluded.updated_at`,
			today, nowStr,
		)
		return err
	})
	if err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("workflow_incidents", in.IncidentID, "RESOLVE", sess.UserID, nil, in, nil)
	return nil
}
