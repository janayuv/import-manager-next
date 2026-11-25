package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuditEntry mirrors a row in audit_logs.
type AuditEntry struct {
	ID         int64  `json:"id"`
	TableName  string `json:"tableName"`
	RowID      string `json:"rowId"`
	Action     string `json:"action"`
	UserID     string `json:"userId"`
	BeforeJSON string `json:"beforeJson"`
	AfterJSON  string `json:"afterJson"`
	Metadata   string `json:"metadata"`
	CreatedAt  string `json:"createdAt"`
}

// UserActivityEntry mirrors a row in user_activity_audit_logs.
type UserActivityEntry struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	ActionName  string `json:"actionName"`
	EntityType  string `json:"entityType"`
	EntityID    string `json:"entityId"`
	DetailsJSON string `json:"detailsJson"`
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	Severity    string `json:"severity"`
}

// Service records structured database mutation audits and user activity events.
type Service struct {
	db       *sql.DB
	permGate func(perm string) error
}

// NewService constructs an audit logging service bound to db.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// SetDB rebinds the service to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
}

// SetPermissionGate injects an authorization check invoked by UI-facing query methods.
func (s *Service) SetPermissionGate(gate func(perm string) error) {
	s.permGate = gate
}

// RecordTableAudit appends an immutable before/after record into audit_logs, redacting secret keys.
func (s *Service) RecordTableAudit(tableName, rowID, action, userID string, beforeObj, afterObj, metadata any) error {
	return s.recordTableAuditWithExec(s.db.Exec, tableName, rowID, action, userID, beforeObj, afterObj, metadata)
}

// RecordTableAuditTx appends an immutable before/after record into audit_logs inside an active SQL transaction.
func (s *Service) RecordTableAuditTx(tx *sql.Tx, tableName, rowID, action, userID string, beforeObj, afterObj, metadata any) error {
	return s.recordTableAuditWithExec(tx.Exec, tableName, rowID, action, userID, beforeObj, afterObj, metadata)
}

func (s *Service) recordTableAuditWithExec(
	execFn func(query string, args ...any) (sql.Result, error),
	tableName, rowID, action, userID string,
	beforeObj, afterObj, metadata any,
) error {
	beforeStr := marshalRedacted(beforeObj)
	afterStr := marshalRedacted(afterObj)
	metaStr := marshalRedacted(metadata)

	_, err := execFn(`
		INSERT INTO audit_logs (table_name, "tableName", row_id, action, user_id, before_json, after_json, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		tableName, tableName, rowID, strings.ToUpper(action), userID, beforeStr, afterStr, metaStr,
	)
	if err != nil {
		return fmt.Errorf("insert audit_logs: %w", err)
	}
	return nil
}

// RecordUserActivity appends an entry into user_activity_audit_logs with normalized severity.
func (s *Service) RecordUserActivity(userID, actionName, entityType, entityID, status, severity string, details any) error {
	sev := strings.ToUpper(strings.TrimSpace(severity))
	switch sev {
	case "INFO", "WARNING", "SECURITY", "CRITICAL":
	default:
		sev = "INFO"
	}
	det := marshalRedacted(details)
	if det == "" {
		det = "{}"
	}
	_, err := s.db.Exec(`
		INSERT INTO user_activity_audit_logs (id, user_id, action_name, entity_type, entity_id, details_json, status, timestamp, severity)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), userID, actionName, entityType, entityID, det, status, time.Now().UTC().Format(time.RFC3339), sev,
	)
	if err != nil {
		return fmt.Errorf("insert user_activity_audit_logs: %w", err)
	}
	return nil
}

// QueryAuditLogs lists recent table audit rows filtered optionally by tableName and action.
func (s *Service) QueryAuditLogs(tableName, action string, limit int) ([]AuditEntry, error) {
	if s.permGate != nil {
		if err := s.permGate("admin.activity_log"); err != nil {
			return nil, err
		}
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, COALESCE(table_name, "tableName", ''), COALESCE(row_id, ''), action,
		       COALESCE(user_id, ''), COALESCE(before_json, ''), COALESCE(after_json, ''),
		       COALESCE(metadata, ''), COALESCE(created_at, '')
		FROM audit_logs
		WHERE (? = '' OR table_name = ? OR "tableName" = ?)
		  AND (? = '' OR UPPER(action) = UPPER(?))
		ORDER BY id DESC
		LIMIT ?`,
		tableName, tableName, tableName, action, action, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query audit_logs: %w", err)
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if scanErr := rows.Scan(&e.ID, &e.TableName, &e.RowID, &e.Action, &e.UserID, &e.BeforeJSON, &e.AfterJSON, &e.Metadata, &e.CreatedAt); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// QueryUserActivityLogs returns recent user activity entries filtered by optional severity.
func (s *Service) QueryUserActivityLogs(severity string, limit int) ([]UserActivityEntry, error) {
	if s.permGate != nil {
		if err := s.permGate("admin.user_activity"); err != nil {
			return nil, err
		}
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, COALESCE(user_id, ''), action_name, COALESCE(entity_type, ''), COALESCE(entity_id, ''),
		       COALESCE(details_json, '{}'), status, timestamp, severity
		FROM user_activity_audit_logs
		WHERE (? = '' OR UPPER(severity) = UPPER(?))
		ORDER BY timestamp DESC
		LIMIT ?`,
		severity, severity, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query user_activity_audit_logs: %w", err)
	}
	defer rows.Close()

	var out []UserActivityEntry
	for rows.Next() {
		var e UserActivityEntry
		if scanErr := rows.Scan(&e.ID, &e.UserID, &e.ActionName, &e.EntityType, &e.EntityID, &e.DetailsJSON, &e.Status, &e.Timestamp, &e.Severity); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func marshalRedacted(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err == nil {
		redactMap(m)
		clean, err := json.Marshal(m)
		if err == nil {
			return string(clean)
		}
	}
	return string(b)
}

func redactMap(m map[string]any) {
	for k, v := range m {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") {
			m[k] = "[REDACTED]"
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			redactMap(sub)
		}
	}
}
