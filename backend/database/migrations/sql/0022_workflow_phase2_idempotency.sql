-- Migration 0022: Automation Rules Phase 2 - Execution Idempotency
-- Prevents duplicate rule actions per (tenant, rule_version, entity_type, entity_id, action_type)
-- across retries, overlapping cycles, or re-evaluations.

CREATE TABLE IF NOT EXISTS workflow_rule_action_idempotency (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT NOT NULL DEFAULT 'tenant-default',
    rule_id TEXT NOT NULL,
    rule_version_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    action_type TEXT NOT NULL,
    execution_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'utc')),
    UNIQUE (tenant_id, rule_version_id, entity_type, entity_id, action_type)
);

CREATE INDEX IF NOT EXISTS idx_wf_rule_idempotency_rule_entity
    ON workflow_rule_action_idempotency (rule_id, entity_id);
