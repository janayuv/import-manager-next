-- Migration 0024: Automation Rules Phase 3A - Progressive Canary, Monitoring, Auto-Rollback & Alerts

-- 1. Progressive rollout metadata on canary_rule_deployment
ALTER TABLE canary_rule_deployment ADD COLUMN rollout_stage INTEGER NOT NULL DEFAULT 0;
ALTER TABLE canary_rule_deployment ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
ALTER TABLE canary_rule_deployment ADD COLUMN completed_at TEXT NOT NULL DEFAULT '';

-- 2. Rebuild rule_deployment_impact_metrics to store separate canary vs. control cohort telemetry per cycle
CREATE TABLE IF NOT EXISTS rule_deployment_impact_metrics_new (
    id TEXT PRIMARY KEY NOT NULL,
    rule_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    cohort TEXT NOT NULL DEFAULT 'canary' CHECK (cohort IN ('canary', 'control')),
    snapshot_at TEXT NOT NULL,
    attempted_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    skipped_count INTEGER NOT NULL DEFAULT 0,
    cost_units REAL NOT NULL DEFAULT 0,
    avg_duration_ms REAL NOT NULL DEFAULT 0,
    failure_rate REAL NOT NULL DEFAULT 0,
    cost_units_delta REAL NOT NULL DEFAULT 0,
    duration_delta_ms REAL NOT NULL DEFAULT 0,
    resolution_gain_delta REAL NOT NULL DEFAULT 0,
    execution_count INTEGER NOT NULL DEFAULT 0,
    factors_json TEXT NOT NULL DEFAULT '{}',
    UNIQUE (rule_id, version_id, cohort, snapshot_at)
);

INSERT INTO rule_deployment_impact_metrics_new (
    id, rule_id, version_id, cohort, snapshot_at, failure_rate, cost_units_delta, resolution_gain_delta, execution_count, factors_json
)
SELECT
    id, rule_id, version_id, 'canary', snapshot_at, failure_rate, cost_units_delta, resolution_gain_delta, execution_count, factors_json
FROM rule_deployment_impact_metrics;

DROP TABLE rule_deployment_impact_metrics;
ALTER TABLE rule_deployment_impact_metrics_new RENAME TO rule_deployment_impact_metrics;

CREATE INDEX IF NOT EXISTS idx_rule_deploy_impact_version
    ON rule_deployment_impact_metrics (rule_id, version_id, cohort, datetime(snapshot_at) DESC);

-- 3. Configurable automatic rollback thresholds (safe defaults)
CREATE TABLE IF NOT EXISTS canary_rollback_thresholds (
    id TEXT PRIMARY KEY NOT NULL DEFAULT 'default',
    min_sample_size INTEGER NOT NULL DEFAULT 10 CHECK (min_sample_size >= 1),
    max_failure_rate REAL NOT NULL DEFAULT 0.25 CHECK (max_failure_rate > 0 AND max_failure_rate <= 1.0),
    max_cost_increase_pct REAL NOT NULL DEFAULT 50 CHECK (max_cost_increase_pct >= 0),
    max_duration_increase_pct REAL NOT NULL DEFAULT 100 CHECK (max_duration_increase_pct >= 0),
    monitoring_window_minutes INTEGER NOT NULL DEFAULT 60 CHECK (monitoring_window_minutes >= 1),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'utc'))
);

INSERT OR IGNORE INTO canary_rollback_thresholds (
    id, min_sample_size, max_failure_rate, max_cost_increase_pct, max_duration_increase_pct, monitoring_window_minutes
) VALUES ('default', 10, 0.25, 50, 100, 60);

-- 4. Operational alert attribution, deduplication & acknowledgement metadata
ALTER TABLE automation_stability_alert ADD COLUMN rule_id TEXT NOT NULL DEFAULT '';
ALTER TABLE automation_stability_alert ADD COLUMN version_id TEXT NOT NULL DEFAULT '';
ALTER TABLE automation_stability_alert ADD COLUMN message TEXT NOT NULL DEFAULT '';
ALTER TABLE automation_stability_alert ADD COLUMN dedupe_key TEXT NOT NULL DEFAULT '';
ALTER TABLE automation_stability_alert ADD COLUMN acknowledged_by TEXT NOT NULL DEFAULT '';
ALTER TABLE automation_stability_alert ADD COLUMN acknowledged_at TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_stability_alert_open_dedupe
    ON automation_stability_alert (dedupe_key)
    WHERE acknowledged = 0 AND dedupe_key != '';
