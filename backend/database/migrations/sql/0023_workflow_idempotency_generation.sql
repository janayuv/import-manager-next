-- Migration 0023: Lifecycle-generation-aware idempotency for exception reopens & self-healing
-- Preserves strict duplicate protection within the same exception lifecycle generation
-- while allowing legitimate re-processing after self-heal or explicit reopen to OPEN.

ALTER TABLE exception_cases ADD COLUMN lifecycle_generation INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS workflow_rule_action_idempotency_new (
    id TEXT PRIMARY KEY NOT NULL,
    tenant_id TEXT NOT NULL DEFAULT 'tenant-default',
    rule_id TEXT NOT NULL,
    rule_version_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    action_type TEXT NOT NULL,
    lifecycle_generation INTEGER NOT NULL DEFAULT 0,
    execution_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'utc')),
    UNIQUE (tenant_id, rule_version_id, entity_type, entity_id, action_type, lifecycle_generation)
);

INSERT INTO workflow_rule_action_idempotency_new (
    id, tenant_id, rule_id, rule_version_id, entity_type, entity_id, action_type, lifecycle_generation, execution_id, created_at
)
SELECT
    id, tenant_id, rule_id, rule_version_id, entity_type, entity_id, action_type, 0, execution_id, created_at
FROM workflow_rule_action_idempotency;

DROP TABLE workflow_rule_action_idempotency;
ALTER TABLE workflow_rule_action_idempotency_new RENAME TO workflow_rule_action_idempotency;

CREATE INDEX IF NOT EXISTS idx_wf_rule_idempotency_rule_entity
    ON workflow_rule_action_idempotency (rule_id, entity_id);
