package workflow

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const (
	HealthStateHealthy  = "HEALTHY"
	HealthStateWarning  = "WARNING"
	HealthStateCritical = "CRITICAL"
	HealthStateDisabled = "DISABLED"
	HealthStateUnknown  = "UNKNOWN"
)

type ruleHealthTelemetryRow struct {
	attemptedCount int64
	successCount   int64
	failureCount   int64
	skippedCount   int64
	costUnits      float64
	weightedDurMs  float64
}

type ruleAlertRollup struct {
	openCount            int
	ackCount             int
	maxCreatedAtForState string
	hasCritical          bool
	hasWarning           bool
}

// GetDeploymentHealthOverview returns a read-only consolidated snapshot of rule deployments,
// health status with reasons, canary progress, recent rollbacks, and operational alerts.
func (s *Service) GetDeploymentHealthOverview() (models.DeploymentHealthOverview, error) {
	if _, err := s.security.RequireAnyPermission(
		security.PermAutomationView,
		security.PermAutomationMutate,
		security.PermAutomationOpsCenter,
	); err != nil {
		return models.DeploymentHealthOverview{}, err
	}
	return s.getDeploymentHealthOverviewInternal(context.Background(), s.db)
}

func (s *Service) getDeploymentHealthOverviewInternal(ctx context.Context, q sqlQueryable) (models.DeploymentHealthOverview, error) {
	thresholds, err := s.getRollbackThresholdsInternal(ctx, q)
	if err != nil {
		return models.DeploymentHealthOverview{}, err
	}

	out := models.DeploymentHealthOverview{
		WindowMinutes:   thresholds.MonitoringWindowMinutes,
		Thresholds:      thresholds,
		OverallHealth:   HealthStateUnknown,
		RuleDeployments: []models.RuleDeploymentHealth{},
		RecentAlerts:    []models.OperationalAlert{},
		RecentRollbacks: []models.RollbackEventRecord{},
	}

	// Query 1: Single-pass rule + latest version + approval + prod deploy + current env + canary metadata
	type rawRuleRow struct {
		health           models.RuleDeploymentHealth
		tenantID         string
		prodStatus       string
		prodDeployedAt   string
		canaryUpdatedAt  string
		latestAnyDeploy  string
	}

	ruleRows, err := q.QueryContext(ctx,
		`WITH latest_ver AS (
			SELECT rule_id, version_id, version_number,
			       ROW_NUMBER() OVER (PARTITION BY rule_id ORDER BY version_number DESC) AS rn
			FROM workflow_rule_versions
		),
		latest_appr AS (
			SELECT rule_id, version_id, approval_status,
			       ROW_NUMBER() OVER (PARTITION BY rule_id, version_id ORDER BY datetime(created_at) DESC, rowid DESC) AS rn
			FROM workflow_rule_approvals
		),
		prod_dep AS (
			SELECT d.rule_id, d.version_id, d.status, d.timestamp, COALESCE(v.version_number, 0) AS version_number,
			       ROW_NUMBER() OVER (
			           PARTITION BY d.rule_id
			           ORDER BY CASE WHEN d.status = 'DEPLOYED' THEN 0 ELSE 1 END, datetime(d.timestamp) DESC, d.rowid DESC
			       ) AS rn
			FROM workflow_environment_deployment_log d
			LEFT JOIN workflow_rule_versions v ON v.rule_id = d.rule_id AND v.version_id = d.version_id
			WHERE d.environment_id = 'env-prod'
		),
		cur_env AS (
			SELECT rule_id, environment_id,
			       ROW_NUMBER() OVER (
			           PARTITION BY rule_id
			           ORDER BY CASE environment_id WHEN 'env-prod' THEN 3 WHEN 'env-test' THEN 2 WHEN 'env-dev' THEN 1 ELSE 0 END DESC,
			                    datetime(timestamp) DESC, rowid DESC
			       ) AS rn
			FROM workflow_environment_deployment_log
			WHERE status = 'DEPLOYED'
		),
		latest_dep AS (
			SELECT rule_id, timestamp,
			       ROW_NUMBER() OVER (PARTITION BY rule_id ORDER BY datetime(timestamp) DESC, rowid DESC) AS rn
			FROM workflow_environment_deployment_log
		)
		SELECT r.rule_id, r.rule_name, r.rule_type, r.action_type, r.enabled, r.tenant_id,
		       COALESCE(lv.version_id, ''), COALESCE(lv.version_number, 0), COALESCE(la.approval_status, ''),
		       COALESCE(CASE WHEN pd.status = 'DEPLOYED' THEN pd.version_id ELSE '' END, ''),
		       COALESCE(CASE WHEN pd.status = 'DEPLOYED' THEN pd.version_number ELSE 0 END, 0),
		       COALESCE(pd.status, ''), COALESCE(pd.timestamp, ''),
		       COALESCE(ce.environment_id, ''), COALESCE(ld.timestamp, ''),
		       COALESCE(c.deployment_status, ''), COALESCE(c.version_id, ''),
		       COALESCE(c.sample_size_percentage, 0), COALESCE(c.rollout_stage, 0),
		       COALESCE(NULLIF(c.updated_at, ''), c.created_at, '')
		FROM workflow_decision_rules r
		LEFT JOIN latest_ver lv ON lv.rule_id = r.rule_id AND lv.rn = 1
		LEFT JOIN latest_appr la ON la.rule_id = r.rule_id AND la.version_id = lv.version_id AND la.rn = 1
		LEFT JOIN prod_dep pd ON pd.rule_id = r.rule_id AND pd.rn = 1
		LEFT JOIN cur_env ce ON ce.rule_id = r.rule_id AND ce.rn = 1
		LEFT JOIN latest_dep ld ON ld.rule_id = r.rule_id AND ld.rn = 1
		LEFT JOIN canary_rule_deployment c ON c.tenant_id = r.tenant_id AND c.rule_id = r.rule_id
		ORDER BY r.priority DESC, datetime(r.updated_at) DESC`,
	)
	if err != nil {
		return out, fmt.Errorf("query rule deployment overview: %w", err)
	}
	var rawRules []rawRuleRow
	for ruleRows.Next() {
		var rr rawRuleRow
		var enInt int
		if sErr := ruleRows.Scan(
			&rr.health.RuleID, &rr.health.RuleName, &rr.health.RuleType, &rr.health.ActionType, &enInt, &rr.tenantID,
			&rr.health.LatestVersionID, &rr.health.LatestVersionNum, &rr.health.LatestApprovalStatus,
			&rr.health.ProdVersionID, &rr.health.ProdVersionNum,
			&rr.prodStatus, &rr.prodDeployedAt,
			&rr.health.CurrentEnv, &rr.latestAnyDeploy,
			&rr.health.CanaryStatus, &rr.health.CanaryVersionID,
			&rr.health.CanaryPercent, &rr.health.CanaryRolloutStage, &rr.canaryUpdatedAt,
		); sErr != nil {
			_ = ruleRows.Close()
			return out, sErr
		}
		rr.health.Enabled = enInt == 1
		if rr.canaryUpdatedAt > rr.latestAnyDeploy {
			rr.health.LatestDeployedAt = rr.canaryUpdatedAt
		} else {
			rr.health.LatestDeployedAt = rr.latestAnyDeploy
		}
		rawRules = append(rawRules, rr)
	}
	if err := ruleRows.Err(); err != nil {
		_ = ruleRows.Close()
		return out, err
	}
	_ = ruleRows.Close()

	// Query 2: Batch window monitoring metrics grouped by (rule_id, version_id, cohort)
	windowExpr := fmt.Sprintf("-%d minutes", thresholds.MonitoringWindowMinutes)
	metricsByKey := map[string]ruleHealthTelemetryRow{}
	mRows, err := q.QueryContext(ctx,
		`SELECT rule_id, version_id, cohort,
		        COALESCE(SUM(attempted_count), 0),
		        COALESCE(SUM(success_count), 0),
		        COALESCE(SUM(failure_count), 0),
		        COALESCE(SUM(skipped_count), 0),
		        COALESCE(SUM(cost_units), 0),
		        COALESCE(SUM(avg_duration_ms * attempted_count), 0)
		 FROM rule_deployment_impact_metrics
		 WHERE datetime(snapshot_at) >= datetime('now', ?)
		 GROUP BY rule_id, version_id, cohort`,
		windowExpr,
	)
	if err != nil {
		return out, fmt.Errorf("query aggregated impact metrics: %w", err)
	}
	for mRows.Next() {
		var rID, vID, cohort string
		var row ruleHealthTelemetryRow
		if mRows.Scan(&rID, &vID, &cohort, &row.attemptedCount, &row.successCount, &row.failureCount, &row.skippedCount, &row.costUnits, &row.weightedDurMs) == nil {
			metricsByKey[rID+"|"+vID+"|"+cohort] = row
		}
	}
	_ = mRows.Close()

	costWeights := loadRuleCostWeightsMap(ctx, q)

	// Query 3: Batch operational alert rollup (open + active-window alerts, never erased by acknowledgement)
	type rawAlertStat struct {
		versionID    string
		severity     string
		acknowledged bool
		count        int
		maxCreatedAt string
	}
	alertStatsByRule := map[string][]rawAlertStat{}
	aStatRows, err := q.QueryContext(ctx,
		`SELECT COALESCE(rule_id, ''), COALESCE(version_id, ''), UPPER(severity), acknowledged,
		        COUNT(*), COALESCE(MAX(datetime(created_at)), '')
		 FROM automation_stability_alert
		 WHERE COALESCE(rule_id, '') != ''
		   AND (acknowledged = 0 OR datetime(created_at) >= datetime('now', ?))
		 GROUP BY COALESCE(rule_id, ''), COALESCE(version_id, ''), UPPER(severity), acknowledged`,
		windowExpr,
	)
	if err != nil {
		return out, fmt.Errorf("query alert rollup: %w", err)
	}
	for aStatRows.Next() {
		var rID, vID, sev, maxAt string
		var ackInt, cnt int
		if aStatRows.Scan(&rID, &vID, &sev, &ackInt, &cnt, &maxAt) == nil {
			alertStatsByRule[rID] = append(alertStatsByRule[rID], rawAlertStat{
				versionID:    vID,
				severity:     sev,
				acknowledged: ackInt == 1,
				count:        cnt,
				maxCreatedAt: maxAt,
			})
		}
	}
	_ = aStatRows.Close()

	// Query 4: Recent operational alerts (up to 50, includes open and recently acknowledged)
	aRows, err := q.QueryContext(ctx,
		`SELECT id, alert_type, severity, COALESCE(rule_id, ''), COALESCE(version_id, ''),
		        COALESCE(message, ''), details_json, created_at, acknowledged,
		        COALESCE(acknowledged_by, ''), COALESCE(acknowledged_at, '')
		 FROM automation_stability_alert
		 ORDER BY datetime(created_at) DESC, rowid DESC
		 LIMIT 50`,
	)
	if err == nil {
		for aRows.Next() {
			var al models.OperationalAlert
			var ackInt int
			if aRows.Scan(&al.ID, &al.AlertType, &al.Severity, &al.RuleID, &al.VersionID,
				&al.Message, &al.DetailsJSON, &al.CreatedAt, &ackInt, &al.AcknowledgedBy, &al.AcknowledgedAt) == nil {
				al.Acknowledged = ackInt == 1
				out.RecentAlerts = append(out.RecentAlerts, al)
			}
		}
		_ = aRows.Close()
	}

	// Query 5: Rollback timestamps per rule + recent 50 rollback events
	latestAutoDisableAt := map[string]string{}
	latestAnyRollbackAt := map[string]string{}
	rbkStatRows, err := q.QueryContext(ctx,
		`SELECT original_automation_id, rollback_type, COALESCE(MAX(datetime(performed_at)), '')
		 FROM rollback_action_log
		 WHERE COALESCE(original_automation_id, '') != ''
		 GROUP BY original_automation_id, rollback_type`,
	)
	if err == nil {
		for rbkStatRows.Next() {
			var rID, rType, maxAt string
			if rbkStatRows.Scan(&rID, &rType, &maxAt) == nil {
				if rType == "AUTO_ROLLBACK_DISABLE" && maxAt > latestAutoDisableAt[rID] {
					latestAutoDisableAt[rID] = maxAt
				}
				if (rType == "AUTO_ROLLBACK_DISABLE" || rType == "RULE_ROLLBACK") && maxAt > latestAnyRollbackAt[rID] {
					latestAnyRollbackAt[rID] = maxAt
				}
			}
		}
		_ = rbkStatRows.Close()
	}

	rbkRows, err := q.QueryContext(ctx,
		`SELECT id, original_automation_id, rollback_type, target_entity, performed_by, performed_at, details_json
		 FROM rollback_action_log
		 ORDER BY datetime(performed_at) DESC, rowid DESC
		 LIMIT 50`,
	)
	if err == nil {
		for rbkRows.Next() {
			var rec models.RollbackEventRecord
			if rbkRows.Scan(&rec.ID, &rec.RuleID, &rec.RollbackType, &rec.TargetEntity, &rec.PerformedBy, &rec.PerformedAt, &rec.DetailsJSON) == nil {
				out.RecentRollbacks = append(out.RecentRollbacks, rec)
			}
		}
		_ = rbkRows.Close()
	}

	// Query 6: Deduplicated failed/rolled-back deployments count & latest timestamp
	_ = q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT rule_id, version_id FROM canary_rule_deployment WHERE deployment_status IN ('ROLLED_BACK', 'FAILED')
			UNION
			SELECT rule_id, version_id FROM workflow_environment_deployment_log WHERE status IN ('ROLLED_BACK', 'FAILED')
		)`,
	).Scan(&out.FailedOrRolledBackDeployments)

	_ = q.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(ts), '') FROM (
			SELECT MAX(datetime(timestamp)) AS ts FROM workflow_environment_deployment_log
			UNION ALL
			SELECT MAX(datetime(COALESCE(NULLIF(updated_at, ''), created_at))) AS ts FROM canary_rule_deployment
			UNION ALL
			SELECT MAX(datetime(deployment_time)) AS ts FROM workflow_rule_deployment_log
		)`,
	).Scan(&out.LatestDeploymentTimestamp)

	// Evaluate per-rule health without any N+1 DB calls
	latestProdTs := ""
	healthyActiveCount := 0
	warningCount := 0
	criticalCount := 0
	disabledCount := 0
	unknownActiveCount := 0

	for _, rr := range rawRules {
		h := rr.health
		out.TotalRules++

		if h.LatestApprovalStatus == "PENDING" {
			out.RulesAwaitingApproval++
		}
		if h.CanaryStatus == "ACTIVE" {
			out.ActiveCanaryDeployments++
		} else if h.CanaryStatus == "COMPLETED" {
			out.CompletedCanaryDeployments++
		}
		if h.Enabled && h.ProdVersionID != "" {
			out.ActiveProductionRules++
			if rr.prodDeployedAt >= latestProdTs {
				latestProdTs = rr.prodDeployedAt
				out.CurrentProductionRuleID = h.RuleID
				out.CurrentProductionVersionID = h.ProdVersionID
				out.CurrentProductionVersionNum = h.ProdVersionNum
			}
		}

		targetVer := h.ProdVersionID
		if h.CanaryStatus == "ACTIVE" || h.CanaryStatus == "COMPLETED" || (h.CanaryStatus == "ROLLED_BACK" && targetVer == "") {
			if h.CanaryVersionID != "" {
				targetVer = h.CanaryVersionID
			}
		}
		if targetVer == "" {
			targetVer = h.LatestVersionID
		}

		// Collect metrics for targetVer from in-memory map
		canaryM := metricsByKey[h.RuleID+"|"+targetVer+"|canary"]
		controlM := metricsByKey[h.RuleID+"|"+targetVer+"|control"]

		var primary ruleHealthTelemetryRow
		if h.CanaryStatus == "ACTIVE" || canaryM.attemptedCount > 0 {
			primary = canaryM
		} else {
			primary = controlM
		}

		h.AttemptedCount = primary.attemptedCount
		h.SuccessCount = primary.successCount
		h.FailureCount = primary.failureCount
		h.SkippedCount = primary.skippedCount
		h.CostUnits = primary.costUnits
		if h.AttemptedCount > 0 {
			h.FailureRate = float64(h.FailureCount) / float64(h.AttemptedCount)
			h.AvgDurationMs = primary.weightedDurMs / float64(h.AttemptedCount)
		}

		if (h.CanaryStatus == "ACTIVE" || canaryM.attemptedCount > 0) && canaryM.attemptedCount > 0 {
			canaryAvgCost := canaryM.costUnits / float64(canaryM.attemptedCount)
			baselineCost := 0.0
			if controlM.attemptedCount > 0 {
				baselineCost = controlM.costUnits / float64(controlM.attemptedCount)
			} else {
				baselineCost = resolveRuleCostWeight(costWeights, h.RuleID, h.ActionType, h.RuleType)
			}
			if baselineCost > 0 {
				pct := ((canaryAvgCost - baselineCost) / baselineCost) * 100.0
				if pct > 0 {
					h.CostIncreasePct = pct
				}
			}
			if controlM.attemptedCount > 0 {
				controlAvgDur := controlM.weightedDurMs / float64(controlM.attemptedCount)
				if controlAvgDur > 0 {
					durPct := ((h.AvgDurationMs - controlAvgDur) / controlAvgDur) * 100.0
					if durPct > 0 {
						h.DurationIncreasePct = durPct
					}
				}
			}
		}

		// Alert rollup: open alerts + recent/version-scoped acknowledged alerts never auto-clear health
		var rollup ruleAlertRollup
		for _, st := range alertStatsByRule[h.RuleID] {
			if !st.acknowledged {
				rollup.openCount += st.count
			} else {
				rollup.ackCount += st.count
			}
			appliesToActive := !st.acknowledged ||
				(targetVer != "" && st.versionID == targetVer) ||
				(st.versionID == "" && (rr.prodDeployedAt == "" || st.maxCreatedAt >= rr.prodDeployedAt))
			if !appliesToActive {
				continue
			}
			if st.severity == "CRITICAL" || st.severity == "FATAL" {
				rollup.hasCritical = true
			} else if st.severity == "HIGH" || st.severity == "WARNING" {
				rollup.hasWarning = true
			}
		}
		h.OpenAlertCount = rollup.openCount
		h.AcknowledgedAlertCount = rollup.ackCount

		lastAutoDis := latestAutoDisableAt[h.RuleID]
		lastAnyRbk := latestAnyRollbackAt[h.RuleID]
		hasUnrecoveredAutoRollback := lastAutoDis != "" && !h.Enabled && (rr.prodDeployedAt == "" || lastAutoDis >= rr.prodDeployedAt)
		h.HasRecentRollback = rr.prodStatus == "ROLLED_BACK" || h.CanaryStatus == "ROLLED_BACK" || lastAnyRbk != ""

		// Determine HealthStatus & clear explanations
		var reasons []string
		isPromotedOrCanary := h.ProdVersionID != "" || h.CanaryStatus == "ACTIVE" || h.CanaryStatus == "COMPLETED"
		sufficientSamples := h.AttemptedCount >= int64(thresholds.MinSampleSize)

		switch {
		case hasUnrecoveredAutoRollback:
			h.HealthStatus = HealthStateCritical
			reasons = append(reasons, "Automatic canary rollback disabled this rule without safe version recovery")
		case !h.Enabled && (rr.prodStatus == "ROLLED_BACK" || h.CanaryStatus == "ROLLED_BACK"):
			if rollup.hasCritical || (sufficientSamples && h.FailureRate > thresholds.MaxFailureRate) {
				h.HealthStatus = HealthStateCritical
			} else {
				h.HealthStatus = HealthStateDisabled
			}
			reasons = append(reasons, "Rule was rolled back and is currently disabled")
		case !h.Enabled:
			if rollup.hasCritical {
				h.HealthStatus = HealthStateCritical
				reasons = append(reasons, "Rule is disabled and has active CRITICAL operational alerts")
			} else {
				h.HealthStatus = HealthStateDisabled
				reasons = append(reasons, "Rule is disabled")
			}
		case rollup.hasCritical:
			h.HealthStatus = HealthStateCritical
			if h.OpenAlertCount > 0 {
				reasons = append(reasons, fmt.Sprintf("%d unacknowledged operational alert(s) at CRITICAL severity", h.OpenAlertCount))
			} else {
				reasons = append(reasons, "CRITICAL operational alert remains active for current deployment (acknowledged alerts require verified remediation)")
			}
		case sufficientSamples && h.FailureRate > thresholds.MaxFailureRate:
			h.HealthStatus = HealthStateCritical
			reasons = append(reasons, fmt.Sprintf("Failure rate %.1f%% exceeds max %.1f%% (%d/%d executions)",
				h.FailureRate*100, thresholds.MaxFailureRate*100, h.FailureCount, h.AttemptedCount))
		case !isPromotedOrCanary:
			if rollup.hasWarning {
				h.HealthStatus = HealthStateWarning
				reasons = append(reasons, "Operational warning alert active prior to production promotion")
			} else {
				h.HealthStatus = HealthStateUnknown
				reasons = append(reasons, "Rule is staged and has not been promoted to production or canary")
			}
		case rollup.hasWarning:
			h.HealthStatus = HealthStateWarning
			reasons = append(reasons, "WARNING/HIGH operational alert recorded for current deployment")
		case sufficientSamples && (h.CostIncreasePct > thresholds.MaxCostIncreasePct || h.DurationIncreasePct > thresholds.MaxDurationIncreasePct || h.FailureRate > thresholds.MaxFailureRate*0.8):
			h.HealthStatus = HealthStateWarning
			if h.CostIncreasePct > thresholds.MaxCostIncreasePct {
				reasons = append(reasons, fmt.Sprintf("Execution cost +%.1f%% exceeds threshold %.1f%%", h.CostIncreasePct, thresholds.MaxCostIncreasePct))
			}
			if h.DurationIncreasePct > thresholds.MaxDurationIncreasePct {
				reasons = append(reasons, fmt.Sprintf("Latency +%.1f%% exceeds threshold %.1f%%", h.DurationIncreasePct, thresholds.MaxDurationIncreasePct))
			}
			if h.FailureRate > thresholds.MaxFailureRate*0.8 {
				reasons = append(reasons, fmt.Sprintf("Failure rate %.1f%% approaching limit %.1f%%", h.FailureRate*100, thresholds.MaxFailureRate*100))
			}
		case !sufficientSamples:
			h.HealthStatus = HealthStateUnknown
			reasons = append(reasons, fmt.Sprintf("Insufficient monitoring samples (%d of %d required in last %dm)",
				h.AttemptedCount, thresholds.MinSampleSize, thresholds.MonitoringWindowMinutes))
		default:
			h.HealthStatus = HealthStateHealthy
			reasons = append(reasons, fmt.Sprintf("All %d monitored executions within thresholds (fail rate %.1f%%, cost +%.1f%%)",
				h.AttemptedCount, h.FailureRate*100, h.CostIncreasePct))
		}

		if h.AcknowledgedAlertCount > 0 && h.HealthStatus != HealthStateHealthy {
			reasons = append(reasons, fmt.Sprintf("%d acknowledged alert(s) logged", h.AcknowledgedAlertCount))
		}
		h.HealthReasons = reasons

		switch h.HealthStatus {
		case HealthStateCritical:
			criticalCount++
		case HealthStateWarning:
			warningCount++
		case HealthStateDisabled:
			disabledCount++
		case HealthStateUnknown:
			if isPromotedOrCanary {
				unknownActiveCount++
			}
		case HealthStateHealthy:
			healthyActiveCount++
		}

		out.RuleDeployments = append(out.RuleDeployments, h)
	}

	switch {
	case out.TotalRules == 0:
		out.OverallHealth = HealthStateUnknown
		out.HealthSummaryReason = "No automation rules configured"
	case criticalCount > 0:
		out.OverallHealth = HealthStateCritical
		out.HealthSummaryReason = fmt.Sprintf("%d rule(s) in CRITICAL state", criticalCount)
	case warningCount > 0:
		out.OverallHealth = HealthStateWarning
		out.HealthSummaryReason = fmt.Sprintf("%d rule(s) with operational or performance warnings", warningCount)
	case out.ActiveProductionRules == 0 && out.ActiveCanaryDeployments == 0:
		if disabledCount == out.TotalRules {
			out.OverallHealth = HealthStateDisabled
			out.HealthSummaryReason = "All automation rules are currently disabled"
		} else {
			out.OverallHealth = HealthStateUnknown
			out.HealthSummaryReason = "No active production or canary rule deployments"
		}
	case unknownActiveCount > 0 && healthyActiveCount == 0:
		out.OverallHealth = HealthStateUnknown
		out.HealthSummaryReason = fmt.Sprintf("%d active deployment(s) awaiting sufficient monitoring samples", unknownActiveCount)
	case unknownActiveCount > 0:
		out.OverallHealth = HealthStateWarning
		out.HealthSummaryReason = fmt.Sprintf("%d healthy deployment(s); %d active deployment(s) awaiting sufficient samples", healthyActiveCount, unknownActiveCount)
	default:
		out.OverallHealth = HealthStateHealthy
		out.HealthSummaryReason = fmt.Sprintf("All %d active production/canary rule(s) are healthy", healthyActiveCount)
	}

	return out, nil
}

func loadVersionDeploymentStatus(ctx context.Context, q sqlQueryable, tenantID, ruleID, versionID string) string {
	var canaryStatus string
	var canaryPct float64
	if err := q.QueryRowContext(ctx,
		`SELECT deployment_status, sample_size_percentage
		 FROM canary_rule_deployment
		 WHERE tenant_id = ? AND rule_id = ? AND version_id = ?
		 LIMIT 1`,
		tenantID, ruleID, versionID,
	).Scan(&canaryStatus, &canaryPct); err == nil && canaryStatus != "" {
		if canaryStatus == "ACTIVE" {
			return fmt.Sprintf("CANARY_ACTIVE (%.0f%%)", canaryPct)
		}
		return "CANARY_" + canaryStatus
	}

	var envID, depStatus string
	if err := q.QueryRowContext(ctx,
		`SELECT environment_id, status
		 FROM workflow_environment_deployment_log
		 WHERE tenant_id = ? AND rule_id = ? AND version_id = ?
		 ORDER BY CASE environment_id WHEN 'env-prod' THEN 3 WHEN 'env-test' THEN 2 WHEN 'env-dev' THEN 1 ELSE 0 END DESC,
		          datetime(timestamp) DESC, rowid DESC
		 LIMIT 1`,
		tenantID, ruleID, versionID,
	).Scan(&envID, &depStatus); err == nil && depStatus != "" {
		return fmt.Sprintf("%s (%s)", depStatus, envID)
	}
	return "STAGED"
}

// CompareRuleVersions compares two immutable versions of the same rule along with risk and read-only dry-run impact.
func (s *Service) CompareRuleVersions(ruleID, baseVersionID, targetVersionID string) (models.RuleVersionComparison, error) {
	if _, err := s.security.RequireAnyPermission(
		security.PermAutomationView,
		security.PermAutomationMutate,
		security.PermAutomationOpsCenter,
	); err != nil {
		return models.RuleVersionComparison{}, err
	}
	ruleID = strings.TrimSpace(ruleID)
	baseVersionID = strings.TrimSpace(baseVersionID)
	targetVersionID = strings.TrimSpace(targetVersionID)
	if ruleID == "" || baseVersionID == "" || targetVersionID == "" {
		return models.RuleVersionComparison{}, errors.New("ruleId, baseVersionId, and targetVersionId are required")
	}

	ctx := context.Background()
	baseVer, err := loadRuleVersionQuery(ctx, s.db, ruleID, baseVersionID)
	if err != nil {
		return models.RuleVersionComparison{}, err
	}
	targetVer, err := loadRuleVersionQuery(ctx, s.db, ruleID, targetVersionID)
	if err != nil {
		return models.RuleVersionComparison{}, err
	}
	if baseVer.RuleID != ruleID || targetVer.RuleID != ruleID {
		return models.RuleVersionComparison{}, errors.New("cross-rule version comparison is rejected")
	}

	baseRisk, err := evaluateRuleRiskQuery(ctx, s.db, ruleID, baseVersionID)
	if err != nil {
		return models.RuleVersionComparison{}, err
	}
	targetRisk, err := evaluateRuleRiskQuery(ctx, s.db, ruleID, targetVersionID)
	if err != nil {
		return models.RuleVersionComparison{}, err
	}

	baseDepStatus := loadVersionDeploymentStatus(ctx, s.db, baseVer.TenantID, ruleID, baseVersionID)
	targetDepStatus := loadVersionDeploymentStatus(ctx, s.db, targetVer.TenantID, ruleID, targetVersionID)

	maxCostUnits, maxRecords, costWeights, cases, err := s.loadDryRunBudgetAndCases(ctx)
	if err != nil {
		return models.RuleVersionComparison{}, err
	}

	// Evaluate both versions against the exact same open-case snapshot and equivalent 100% scope
	const equivalentCanaryPct = 100.0
	baseDry := s.runDryRunWithSnapshot(ctx, baseVer, baseRisk, cases, maxCostUnits, maxRecords, costWeights, equivalentCanaryPct)
	targetDry := s.runDryRunWithSnapshot(ctx, targetVer, targetRisk, cases, maxCostUnits, maxRecords, costWeights, equivalentCanaryPct)

	toSummary := func(v models.WorkflowRuleVersion, r models.RuleRiskAssessment, depStatus string) models.RuleVersionSummary {
		app := v.ApprovalStatus
		if app == "" {
			app = "PENDING"
		}
		factors := r.Factors
		if factors == nil {
			factors = []string{}
		}
		return models.RuleVersionSummary{
			VersionID:           v.VersionID,
			VersionNumber:       v.VersionNumber,
			RuleName:            v.RuleName,
			RuleType:            v.RuleType,
			ActionType:          v.ActionType,
			ConditionExpression: v.ConditionExpression,
			Priority:            v.Priority,
			Enabled:             v.Enabled,
			DefinitionSHA256:    v.DefinitionSHA256,
			RiskLevel:           r.Level,
			RiskScore:           r.Score,
			BlocksProd:          r.BlocksProd,
			RiskFactors:         factors,
			ApprovalStatus:      app,
			DeploymentStatus:    depStatus,
			CreatedAt:           v.CreatedAt,
			CreatedBy:           v.CreatedBy,
		}
	}

	baseSum := toSummary(baseVer, baseRisk, baseDepStatus)
	targetSum := toSummary(targetVer, targetRisk, targetDepStatus)

	diffs := []models.RuleVersionFieldDiff{
		{Field: "ruleType", BaseValue: baseSum.RuleType, TargetValue: targetSum.RuleType, Changed: baseSum.RuleType != targetSum.RuleType},
		{Field: "actionType", BaseValue: baseSum.ActionType, TargetValue: targetSum.ActionType, Changed: baseSum.ActionType != targetSum.ActionType},
		{Field: "conditionExpression", BaseValue: baseSum.ConditionExpression, TargetValue: targetSum.ConditionExpression, Changed: baseSum.ConditionExpression != targetSum.ConditionExpression},
		{Field: "priority", BaseValue: strconv.Itoa(baseSum.Priority), TargetValue: strconv.Itoa(targetSum.Priority), Changed: baseSum.Priority != targetSum.Priority},
		{Field: "enabled", BaseValue: strconv.FormatBool(baseSum.Enabled), TargetValue: strconv.FormatBool(targetSum.Enabled), Changed: baseSum.Enabled != targetSum.Enabled},
		{Field: "definitionSha256", BaseValue: baseSum.DefinitionSHA256, TargetValue: targetSum.DefinitionSHA256, Changed: !strings.EqualFold(baseSum.DefinitionSHA256, targetSum.DefinitionSHA256)},
		{Field: "riskLevel", BaseValue: fmt.Sprintf("%s (%.0f)", baseSum.RiskLevel, baseSum.RiskScore), TargetValue: fmt.Sprintf("%s (%.0f)", targetSum.RiskLevel, targetSum.RiskScore), Changed: baseSum.RiskLevel != targetSum.RiskLevel || math.Abs(baseSum.RiskScore-targetSum.RiskScore) > 1e-6},
		{Field: "approvalStatus", BaseValue: baseSum.ApprovalStatus, TargetValue: targetSum.ApprovalStatus, Changed: baseSum.ApprovalStatus != targetSum.ApprovalStatus},
		{Field: "deploymentStatus", BaseValue: baseSum.DeploymentStatus, TargetValue: targetSum.DeploymentStatus, Changed: baseSum.DeploymentStatus != targetSum.DeploymentStatus},
	}

	var changedFields []string
	var important []string
	for i := range diffs {
		d := &diffs[i]
		if !d.Changed {
			continue
		}
		changedFields = append(changedFields, d.Field)
		switch d.Field {
		case "actionType":
			note := fmt.Sprintf("Action changed from %s to %s", d.BaseValue, d.TargetValue)
			d.ImpactNote = note
			important = append(important, note)
		case "conditionExpression":
			note := fmt.Sprintf("Condition expression changed (%s -> %s)", d.BaseValue, d.TargetValue)
			if strings.TrimSpace(d.TargetValue) == "{}" || strings.Contains(d.TargetValue, "*") {
				note += " [broad/wildcard scope]"
			}
			d.ImpactNote = note
			important = append(important, note)
		case "priority":
			note := fmt.Sprintf("Priority changed from %s to %s", d.BaseValue, d.TargetValue)
			d.ImpactNote = note
			important = append(important, note)
		case "enabled":
			note := fmt.Sprintf("Enabled state changed from %s to %s", d.BaseValue, d.TargetValue)
			d.ImpactNote = note
			important = append(important, note)
		case "ruleType":
			note := fmt.Sprintf("Rule type changed from %s to %s", d.BaseValue, d.TargetValue)
			d.ImpactNote = note
			important = append(important, note)
		}
	}

	riskDelta := targetSum.RiskScore - baseSum.RiskScore
	riskChanged := baseSum.RiskLevel != targetSum.RiskLevel
	riskSummary := fmt.Sprintf("No risk change (%s, %.0f)", targetSum.RiskLevel, targetSum.RiskScore)
	if riskDelta > 0 || (riskChanged && targetSum.RiskScore >= baseSum.RiskScore) {
		riskSummary = fmt.Sprintf("Risk increased: %s (%.0f) -> %s (%.0f) (+%.0f)",
			baseSum.RiskLevel, baseSum.RiskScore, targetSum.RiskLevel, targetSum.RiskScore, riskDelta)
		important = append(important, riskSummary)
	} else if riskDelta < 0 || riskChanged {
		riskSummary = fmt.Sprintf("Risk decreased: %s (%.0f) -> %s (%.0f) (%.0f)",
			baseSum.RiskLevel, baseSum.RiskScore, targetSum.RiskLevel, targetSum.RiskScore, riskDelta)
		important = append(important, riskSummary)
	}
	if targetSum.ApprovalStatus != "APPROVED" {
		important = append(important, fmt.Sprintf("Target version approval status is %s (requires APPROVED before production rollout)", targetSum.ApprovalStatus))
	}
	if len(changedFields) == 0 {
		changedFields = []string{}
	}
	if len(important) == 0 {
		important = []string{"No material specification or risk changes between selected versions"}
	}

	return models.RuleVersionComparison{
		RuleID:                ruleID,
		Base:                  baseSum,
		Target:                targetSum,
		ChangedFields:         changedFields,
		FieldDiffs:            diffs,
		ImportantChanges:      important,
		RiskScoreDelta:        riskDelta,
		RiskLevelChanged:      riskChanged,
		RiskDifferenceSummary: riskSummary,
		DryRunImpact: models.DryRunComparisonDelta{
			AssumedCanaryPercent:  equivalentCanaryPct,
			EvaluatedOpenCases:    len(cases),
			BaseMatchingRecords:   baseDry.EligibleCount,
			TargetMatchingRecords: targetDry.EligibleCount,
			MatchingRecordsDelta:  targetDry.EligibleCount - baseDry.EligibleCount,
			BaseAffectedRecords:   baseDry.CanaryRoutedCount,
			TargetAffectedRecords: targetDry.CanaryRoutedCount,
			AffectedRecordsDelta:  targetDry.CanaryRoutedCount - baseDry.CanaryRoutedCount,
			BaseEstimatedCost:     baseDry.EstimatedCostUnits,
			TargetEstimatedCost:   targetDry.EstimatedCostUnits,
			EstimatedCostDelta:    targetDry.EstimatedCostUnits - baseDry.EstimatedCostUnits,
			BaseSkippedRecords:    baseDry.SkippedIdempotentCount,
			TargetSkippedRecords:  targetDry.SkippedIdempotentCount,
			SkippedRecordsDelta:   targetDry.SkippedIdempotentCount - baseDry.SkippedIdempotentCount,
			BaseWithinBudget:      baseDry.WithinBudget,
			TargetWithinBudget:    targetDry.WithinBudget,
		},
		ComparedAt: time.Now().UTC().Format("2006-01-02 15:04:05"),
	}, nil
}
