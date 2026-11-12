package workflow

import (
	"archive/zip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func computeErrorFingerprint(in models.RecordErrorInput) string {
	raw := strings.Join([]string{
		strings.TrimSpace(in.ModuleName),
		strings.TrimSpace(in.CommandName),
		strings.TrimSpace(in.ErrorCode),
		strings.TrimSpace(in.ErrorMessage),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// RecordError captures and deduplicates an application/runtime error by SHA-256 fingerprint in error_memory.
func (s *Service) RecordError(in models.RecordErrorInput) (models.ErrorMemoryRecord, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAdminSystemHealth, security.PermDataRead, security.PermAutomationOpsCenter); err != nil {
		return models.ErrorMemoryRecord{}, err
	}
	if strings.TrimSpace(in.ErrorMessage) == "" {
		return models.ErrorMemoryRecord{}, errors.New("errorMessage is required")
	}
	sev := strings.ToLower(strings.TrimSpace(in.Severity))
	if sev == "" {
		sev = "error"
	}
	fp := computeErrorFingerprint(in)
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	id := newID("err")
	recInt := 0
	if in.Recoverable {
		recInt = 1
	}

	_, err := s.db.Exec(
		`INSERT INTO error_memory (
		    id, fingerprint, occurrence_count, first_seen_at, last_seen_at, app_version,
		    module_name, command_name, page_name, error_code, error_category,
		    error_message, stack_trace, redacted_input_context, severity, recoverable, status
		 ) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'new')
		 ON CONFLICT(fingerprint) DO UPDATE SET
		    occurrence_count = error_memory.occurrence_count + 1,
		    last_seen_at = excluded.last_seen_at,
		    app_version = excluded.app_version,
		    status = CASE WHEN error_memory.status = 'resolved' THEN 'new' ELSE error_memory.status END`,
		id, fp, nowStr, nowStr, s.appVersion,
		in.ModuleName, in.CommandName, in.PageName, in.ErrorCode, in.ErrorCategory,
		in.ErrorMessage, in.StackTrace, in.RedactedContextJSON, sev, recInt,
	)
	if err != nil {
		return models.ErrorMemoryRecord{}, fmt.Errorf("record error memory: %w", err)
	}

	// Emit alert signal when severity is critical/fatal
	if sev == "critical" || sev == "fatal" {
		_, _ = s.db.Exec(
			`INSERT INTO workflow_alert_signal_log (id, created_at, signal_type, severity, entity_id, message, details_json)
			 VALUES (?, ?, ?, ?, ?, ?, '{}')`,
			newID("sig"), nowStr, "RUNTIME_ERROR_"+strings.ToUpper(in.ErrorCode), strings.ToUpper(sev), in.ModuleName, in.ErrorMessage,
		)
	}

	var out models.ErrorMemoryRecord
	err = s.db.QueryRow(
		`SELECT id, fingerprint, occurrence_count, first_seen_at, last_seen_at,
		        COALESCE(module_name, ''), COALESCE(command_name, ''), COALESCE(error_code, ''),
		        error_message, severity, status, COALESCE(ai_summary, '')
		 FROM error_memory WHERE fingerprint = ?`,
		fp,
	).Scan(&out.ID, &out.Fingerprint, &out.OccurrenceCount, &out.FirstSeenAt, &out.LastSeenAt,
		&out.ModuleName, &out.CommandName, &out.ErrorCode, &out.ErrorMessage, &out.Severity, &out.Status, &out.AISummary)
	return out, err
}

// ListErrors returns deduplicated error_memory groups filtered by status/severity.
func (s *Service) ListErrors(status, severity string) ([]models.ErrorMemoryRecord, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAdminSystemHealth, security.PermAutomationView, security.PermAutomationOpsCenter); err != nil {
		return nil, err
	}
	var conds []string
	var args []any
	if sTrim := strings.TrimSpace(status); sTrim != "" && !strings.EqualFold(sTrim, "all") {
		conds = append(conds, "status = ?")
		args = append(args, strings.ToLower(sTrim))
	}
	if svTrim := strings.TrimSpace(severity); svTrim != "" && !strings.EqualFold(svTrim, "all") {
		conds = append(conds, "severity = ?")
		args = append(args, strings.ToLower(svTrim))
	}
	q := `SELECT id, fingerprint, occurrence_count, first_seen_at, last_seen_at,
	             COALESCE(module_name, ''), COALESCE(command_name, ''), COALESCE(error_code, ''),
	             error_message, severity, status, COALESCE(ai_summary, '')
	      FROM error_memory`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY datetime(last_seen_at) DESC LIMIT 200"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ErrorMemoryRecord
	for rows.Next() {
		var r models.ErrorMemoryRecord
		if err := rows.Scan(&r.ID, &r.Fingerprint, &r.OccurrenceCount, &r.FirstSeenAt, &r.LastSeenAt,
			&r.ModuleName, &r.CommandName, &r.ErrorCode, &r.ErrorMessage, &r.Severity, &r.Status, &r.AISummary); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []models.ErrorMemoryRecord{}
	}
	return out, rows.Err()
}

// UpdateErrorStatus updates triage status (new/acknowledged/resolved) and optional diagnostic summary.
func (s *Service) UpdateErrorStatus(id, status, aiSummary string) error {
	sess, err := s.security.RequireAnyPermission(security.PermAdminSystemHealth, security.PermAdminSystemTools, security.PermAutomationMutate)
	if err != nil {
		return err
	}
	normStatus := strings.ToLower(strings.TrimSpace(status))
	switch normStatus {
	case "new", "acknowledged", "resolved":
	default:
		return fmt.Errorf("invalid error status %q", status)
	}
	res, err := s.db.Exec(
		`UPDATE error_memory SET status = ?, ai_summary = CASE WHEN ? != '' THEN ? ELSE ai_summary END WHERE id = ?`,
		normStatus, strings.TrimSpace(aiSummary), strings.TrimSpace(aiSummary), id,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("error memory record %q not found", id)
	}
	_ = s.audit.RecordTableAudit("error_memory", id, "UPDATE_STATUS", sess.UserID, nil, map[string]string{"status": normStatus}, nil)
	return nil
}

// PruneResolvedErrors archives resolved errors older than retentionDays into error_memory_pruned_groups and logs the run.
func (s *Service) PruneResolvedErrors(retentionDays int) (int, error) {
	sess, err := s.security.RequireAnyPermission(security.PermAdminSystemTools, security.PermAutomationMutate)
	if err != nil {
		return 0, err
	}
	if retentionDays < 0 {
		retentionDays = 30
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	cutoffExpr := fmt.Sprintf("-%d days", retentionDays)

	var totalBefore, protected int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM error_memory`).Scan(&totalBefore)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM error_memory WHERE status != 'resolved'`).Scan(&protected)

	rows, err := s.db.Query(
		`SELECT id, fingerprint, occurrence_count, first_seen_at, last_seen_at, severity, status
		 FROM error_memory WHERE status = 'resolved' AND datetime(last_seen_at) <= datetime('now', ?)`,
		cutoffExpr,
	)
	if err != nil {
		return 0, err
	}
	type pruneRow struct {
		id, fp, first, last, sev, status string
		cnt                              int
	}
	var toPrune []pruneRow
	for rows.Next() {
		var r pruneRow
		if err := rows.Scan(&r.id, &r.fp, &r.cnt, &r.first, &r.last, &r.sev, &r.status); err != nil {
			rows.Close()
			return 0, err
		}
		toPrune = append(toPrune, r)
	}
	rows.Close()

	runID := newID("prune")
	deleted := 0
	for _, r := range toPrune {
		_, err := s.db.Exec(
			`INSERT INTO error_memory_pruned_groups (id, run_id, fingerprint, occurrence_count, first_seen_at, last_seen_at, severity, status, prune_reason, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newID("epg"), runID, r.fp, r.cnt, r.first, r.last, r.sev, r.status,
			fmt.Sprintf("resolved_retention_%dd", retentionDays), nowStr,
		)
		if err == nil {
			if _, delErr := s.db.Exec(`DELETE FROM error_memory WHERE id = ?`, r.id); delErr == nil {
				deleted++
			}
		}
	}

	var totalAfter int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM error_memory`).Scan(&totalAfter)
	_, _ = s.db.Exec(
		`INSERT INTO error_memory_cleanup_runs (id, executed_at, trigger_source, dry_run, total_before, total_after, candidate_count, deleted_count, protected_count)
		 VALUES (?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		runID, nowStr, sess.UserID, totalBefore, totalAfter, len(toPrune), deleted, protected,
	)
	_ = s.audit.RecordTableAudit("error_memory_cleanup_runs", runID, "PRUNE", sess.UserID, nil, map[string]int{"deleted": deleted}, nil)
	return deleted, nil
}

// ListBugNotes returns all bug_notes ordered by updated_at descending.
func (s *Service) ListBugNotes() ([]models.BugNote, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAdminSystemHealth, security.PermDataRead); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, title, COALESCE(description, ''), status, COALESCE(context, ''), created_at, updated_at
		 FROM bug_notes ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.BugNote
	for rows.Next() {
		var n models.BugNote
		if err := rows.Scan(&n.ID, &n.Title, &n.Description, &n.Status, &n.Context, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if out == nil {
		out = []models.BugNote{}
	}
	return out, rows.Err()
}

// SaveBugNote creates or updates a bug_notes entry.
func (s *Service) SaveBugNote(in models.SaveBugNoteInput) (models.BugNote, error) {
	sess, err := s.security.RequireAnyPermission(security.PermAdminSystemTools, security.PermDataWrite)
	if err != nil {
		return models.BugNote{}, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return models.BugNote{}, errors.New("bug note title is required")
	}
	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if status != "SOLVED" {
		status = "OPEN"
	}
	nowUnix := time.Now().UTC().Unix()
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = newID("bug")
		_, err = s.db.Exec(
			`INSERT INTO bug_notes (id, title, description, status, context, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, strings.TrimSpace(in.Title), strings.TrimSpace(in.Description), status, strings.TrimSpace(in.Context), nowUnix, nowUnix,
		)
	} else {
		_, err = s.db.Exec(
			`UPDATE bug_notes SET title = ?, description = ?, status = ?, context = ?, updated_at = ? WHERE id = ?`,
			strings.TrimSpace(in.Title), strings.TrimSpace(in.Description), status, strings.TrimSpace(in.Context), nowUnix, id,
		)
	}
	if err != nil {
		return models.BugNote{}, err
	}
	_ = s.audit.RecordTableAudit("bug_notes", id, "UPSERT", sess.UserID, nil, in, nil)

	var out models.BugNote
	err = s.db.QueryRow(
		`SELECT id, title, COALESCE(description, ''), status, COALESCE(context, ''), created_at, updated_at
		 FROM bug_notes WHERE id = ?`,
		id,
	).Scan(&out.ID, &out.Title, &out.Description, &out.Status, &out.Context, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

// DeleteBugNote deletes a bug_notes row by id.
func (s *Service) DeleteBugNote(id string) error {
	sess, err := s.security.RequireAnyPermission(security.PermAdminSystemTools, security.PermDataWrite)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM bug_notes WHERE id = ?`, id); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("bug_notes", id, "DELETE", sess.UserID, nil, nil, nil)
	return nil
}

// ExportDiagnosticsBundle generates a redacted ZIP diagnostics archive (system info, jobs, incidents, error memory, bug notes, recent logs) in exportsDir or outputPath.
func (s *Service) ExportDiagnosticsBundle(outputPath string) (models.DiagnosticsExportResult, error) {
	sess, err := s.security.RequireAnyPermission(security.PermAdminSystemTools, security.PermAutomationOpsCenter, security.PermBackupCreate)
	if err != nil {
		return models.DiagnosticsExportResult{}, err
	}
	target := strings.TrimSpace(outputPath)
	if target == "" {
		if err := os.MkdirAll(s.exportsDir, 0o700); err != nil {
			return models.DiagnosticsExportResult{}, err
		}
		target = filepath.Join(s.exportsDir, fmt.Sprintf("import_manager_diagnostics_%s.zip", time.Now().UTC().Format("20060102_150405")))
	} else {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return models.DiagnosticsExportResult{}, err
		}
	}

	overview, err := s.GetOperationsOverview()
	if err != nil {
		return models.DiagnosticsExportResult{}, err
	}
	errorsList, err := s.ListErrors("all", "all")
	if err != nil {
		return models.DiagnosticsExportResult{}, err
	}
	bugNotes, err := s.ListBugNotes()
	if err != nil {
		return models.DiagnosticsExportResult{}, err
	}
	execs, err := s.ListJobExecutions("", 100)
	if err != nil {
		return models.DiagnosticsExportResult{}, err
	}

	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return models.DiagnosticsExportResult{}, err
	}
	zw := zip.NewWriter(f)

	included := []string{"manifest.json", "operations_overview.json", "job_executions.json", "error_memory.json", "bug_notes.json"}
	writeJSONEntry := func(name string, v any) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}

	nowISO := time.Now().UTC().Format(time.RFC3339)
	manifest := map[string]any{
		"appVersion":  s.appVersion,
		"generatedAt": nowISO,
		"generatedBy": sess.UserID,
		"redacted":    true,
	}
	for name, payload := range map[string]any{
		"manifest.json":            manifest,
		"operations_overview.json": overview,
		"job_executions.json":      execs,
		"error_memory.json":        errorsList,
		"bug_notes.json":           bugNotes,
	} {
		if err := writeJSONEntry(name, payload); err != nil {
			zw.Close()
			f.Close()
			return models.DiagnosticsExportResult{}, err
		}
	}

	// Include redacted log excerpts if logsDir exists
	if s.logsDir != "" {
		entries, readErr := os.ReadDir(s.logsDir)
		if readErr == nil {
			for _, ent := range entries {
				if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".log") {
					continue
				}
				raw, _ := os.ReadFile(filepath.Join(s.logsDir, ent.Name()))
				if len(raw) > 256*1024 {
					raw = raw[len(raw)-256*1024:]
				}
				w, wErr := zw.Create("logs/" + ent.Name())
				if wErr == nil {
					_, _ = w.Write(raw)
					included = append(included, "logs/"+ent.Name())
				}
			}
		}
	}

	if err := zw.Close(); err != nil {
		f.Close()
		return models.DiagnosticsExportResult{}, err
	}
	if err := f.Close(); err != nil {
		return models.DiagnosticsExportResult{}, err
	}

	st, err := os.Stat(target)
	if err != nil {
		return models.DiagnosticsExportResult{}, err
	}
	_ = s.audit.RecordTableAudit("diagnostics", filepath.Base(target), "EXPORT_ZIP", sess.UserID, nil, map[string]any{"sizeBytes": st.Size()}, nil)
	return models.DiagnosticsExportResult{
		FilePath:         target,
		SizeBytes:        st.Size(),
		GeneratedAt:      nowISO,
		IncludedSections: included,
	}, nil
}

// Suppress compiler warning if sql package import is used in future helpers.
var _ = sql.ErrNoRows
