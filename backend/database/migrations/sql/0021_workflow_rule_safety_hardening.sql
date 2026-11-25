-- Migration 0021: Automation Rules & Promotion Phase 1 Safety Hardening
-- Adds SHA-256 definition checksums, DB-level version immutability triggers,
-- approval checksum binding, and unique active per-environment deployment enforcement.

ALTER TABLE workflow_rule_versions ADD COLUMN definition_sha256 TEXT NOT NULL DEFAULT '';
ALTER TABLE workflow_rule_approvals ADD COLUMN approved_definition_sha256 TEXT NOT NULL DEFAULT '';

-- Prevent silent mutation of historical rule definitions, checksums, or ownership metadata.
-- Allows toggling is_active and one-time backfill when definition_sha256 was initially empty.
CREATE TRIGGER IF NOT EXISTS trg_prevent_update_workflow_rule_versions_definition
BEFORE UPDATE ON workflow_rule_versions
FOR EACH ROW
BEGIN
    SELECT CASE
        WHEN OLD.rule_id != NEW.rule_id THEN
            RAISE(ABORT, 'IMMUTABLE RULE VERSION: rule_id cannot be modified')
        WHEN OLD.tenant_id != NEW.tenant_id THEN
            RAISE(ABORT, 'IMMUTABLE RULE VERSION: tenant_id cannot be modified')
        WHEN OLD.version_number != NEW.version_number THEN
            RAISE(ABORT, 'IMMUTABLE RULE VERSION: version_number cannot be modified')
        WHEN OLD.rule_definition_json != NEW.rule_definition_json THEN
            RAISE(ABORT, 'IMMUTABLE RULE VERSION: rule_definition_json cannot be modified; create a new version instead')
        WHEN OLD.definition_sha256 != '' AND OLD.definition_sha256 != NEW.definition_sha256 THEN
            RAISE(ABORT, 'IMMUTABLE RULE VERSION: definition_sha256 cannot be modified')
        WHEN OLD.created_by != NEW.created_by THEN
            RAISE(ABORT, 'IMMUTABLE RULE VERSION: created_by cannot be modified')
        WHEN OLD.created_at != NEW.created_at THEN
            RAISE(ABORT, 'IMMUTABLE RULE VERSION: created_at cannot be modified')
    END;
END;

CREATE TRIGGER IF NOT EXISTS trg_prevent_delete_workflow_rule_versions
BEFORE DELETE ON workflow_rule_versions
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'IMMUTABLE RULE VERSION: deleting historical rule versions is prohibited');
END;

-- Mark any duplicate historical DEPLOYED rows (if any exist on upgraded DBs) as SUPERSEDED
-- before creating the partial unique index for active environment deployments.
UPDATE workflow_environment_deployment_log
SET status = 'SUPERSEDED'
WHERE status = 'DEPLOYED'
  AND rowid NOT IN (
      SELECT MAX(rowid)
      FROM workflow_environment_deployment_log
      WHERE status = 'DEPLOYED'
      GROUP BY tenant_id, environment_id, rule_id
  );

-- Enforce at most one active DEPLOYED version per (tenant, environment, rule).
CREATE UNIQUE INDEX IF NOT EXISTS idx_wf_env_deploy_active_env
    ON workflow_environment_deployment_log (tenant_id, environment_id, rule_id)
    WHERE status = 'DEPLOYED';

-- Preserve compatibility for the 6 system-seeded default rules: ensure they have
-- an APPROVED approval and an active env-prod deployment entry if not already present.
INSERT INTO workflow_rule_approvals (
    approval_id, rule_id, version_id, approved_by, approval_status, approval_time, requested_by, created_at, approved_definition_sha256
)
SELECT
    'appr-seed-' || v.rule_id,
    v.rule_id,
    v.version_id,
    'system',
    'APPROVED',
    strftime('%Y-%m-%d %H:%M:%S', 'now', 'utc'),
    'system',
    v.created_at,
    v.definition_sha256
FROM workflow_rule_versions v
WHERE v.created_by = 'system'
  AND v.environment_id = 'env-prod'
  AND v.is_active = 1
  AND NOT EXISTS (
      SELECT 1 FROM workflow_rule_approvals a
      WHERE a.rule_id = v.rule_id AND a.version_id = v.version_id
  );

INSERT INTO workflow_environment_deployment_log (
    deployment_id, tenant_id, environment_id, rule_id, version_id, status, timestamp, details_json
)
SELECT
    'dep-seed-' || v.rule_id,
    v.tenant_id,
    'env-prod',
    v.rule_id,
    v.version_id,
    'DEPLOYED',
    strftime('%Y-%m-%d %H:%M:%S', 'now', 'utc'),
    '{"seededBy":"migration-0021"}'
FROM workflow_rule_versions v
WHERE v.created_by = 'system'
  AND v.environment_id = 'env-prod'
  AND v.is_active = 1
  AND NOT EXISTS (
      SELECT 1 FROM workflow_environment_deployment_log d
      WHERE d.tenant_id = v.tenant_id AND d.environment_id = 'env-prod' AND d.rule_id = v.rule_id AND d.status = 'DEPLOYED'
  );
