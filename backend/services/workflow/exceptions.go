package workflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func adaptiveSLAHours(priority string) int {
	switch strings.ToUpper(priority) {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 12
	case "MEDIUM":
		return 24
	default:
		return 72
	}
}

func priorityForExceptionType(exType string) string {
	switch exType {
	case "OVERDUE_ETA":
		return "HIGH"
	case "MISSING_BOE":
		return "HIGH"
	case "MISSING_EXPENSE":
		return "MEDIUM"
	default:
		return "MEDIUM"
	}
}

// DetectExceptions scans active non-delivered shipments for OVERDUE_ETA, MISSING_BOE (>7d), and MISSING_EXPENSE,
// computes adaptive SLA deadlines, self-heals stale IN_PROGRESS (>14d) cases back to OPEN, and logs SLA escalations.
func (s *Service) DetectExceptions() (models.ExceptionDetectionResult, error) {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationOpsCenter, security.PermDataRead)
	if err != nil {
		return models.ExceptionDetectionResult{}, err
	}
	res, err := s.RunExceptionScanInternal(context.Background(), sess.UserID)
	if err != nil {
		return models.ExceptionDetectionResult{}, err
	}
	_ = s.audit.RecordTableAudit("exception_cases", "scan", "DETECT", sess.UserID, nil, res, nil)
	return res, nil
}

// RunExceptionScanInternal executes the exception detector + SLA escalation pass (shared with background scheduler).
func (s *Service) RunExceptionScanInternal(ctx context.Context, actor string) (models.ExceptionDetectionResult, error) {
	if err := s.restoreCtrl.RequireIdle(); err != nil {
		return models.ExceptionDetectionResult{}, err
	}
	now := time.Now().UTC()
	nowStr := now.Format("2006-01-02 15:04:05")
	todayStr := now.Format("2006-01-02")
	sevenDaysAgo := now.AddDate(0, 0, -7).Format("2006-01-02")
	fourteenDaysAgo := now.AddDate(0, 0, -14).Format("2006-01-02 15:04:05")

	var out models.ExceptionDetectionResult

	type candidate struct {
		exType     string
		shipmentID string
	}
	var candidates []candidate

	// 1. OVERDUE_ETA: eta before today on active non-delivered shipments
	q1 := `SELECT id FROM shipments
	       WHERE deleted_at IS NULL
	         AND LOWER(COALESCE(status, '')) NOT IN ('delivered', 'cancelled')
	         AND COALESCE(eta, '') != ''
	         AND substr(eta, 1, 10) < ?`
	rows1, err := s.db.QueryContext(ctx, q1, todayStr)
	if err != nil {
		return models.ExceptionDetectionResult{}, fmt.Errorf("query overdue eta: %w", err)
	}
	for rows1.Next() {
		var id string
		if rows1.Scan(&id) == nil {
			candidates = append(candidates, candidate{"OVERDUE_ETA", id})
		}
	}
	_ = rows1.Close()

	// 2. MISSING_BOE: invoice_date <= 7 days ago and no active boe_calculations record
	q2 := `SELECT sh.id FROM shipments sh
	       WHERE sh.deleted_at IS NULL
	         AND LOWER(COALESCE(sh.status, '')) IN ('in-transit', 'customs-clearance', 'docs-rcvd')
	         AND substr(COALESCE(sh.invoice_date, ''), 1, 10) <= ?
	         AND NOT EXISTS (
	           SELECT 1 FROM boe_calculations bc WHERE bc.shipment_id = sh.id AND bc.deleted_at IS NULL
	         )`
	rows2, err := s.db.QueryContext(ctx, q2, sevenDaysAgo)
	if err != nil {
		return models.ExceptionDetectionResult{}, fmt.Errorf("query missing boe: %w", err)
	}
	for rows2.Next() {
		var id string
		if rows2.Scan(&id) == nil {
			candidates = append(candidates, candidate{"MISSING_BOE", id})
		}
	}
	_ = rows2.Close()

	// 3. MISSING_EXPENSE: customs-clearance / ready-dly shipment without active expenses
	q3 := `SELECT sh.id FROM shipments sh
	       WHERE sh.deleted_at IS NULL
	         AND LOWER(COALESCE(sh.status, '')) IN ('customs-clearance', 'ready-dly')
	         AND NOT EXISTS (
	           SELECT 1 FROM expenses ex WHERE ex.shipment_id = sh.id AND ex.deleted_at IS NULL
	         )`
	rows3, err := s.db.QueryContext(ctx, q3)
	if err != nil {
		return models.ExceptionDetectionResult{}, fmt.Errorf("query missing expense: %w", err)
	}
	for rows3.Next() {
		var id string
		if rows3.Scan(&id) == nil {
			candidates = append(candidates, candidate{"MISSING_EXPENSE", id})
		}
	}
	_ = rows3.Close()

	err = transactions.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		for _, c := range candidates {
			var existingID string
			err := tx.QueryRowContext(ctx,
				`SELECT id FROM exception_cases
				 WHERE exception_type = ? AND entity_id = ? AND status IN ('OPEN', 'IN_PROGRESS')
				 LIMIT 1`,
				c.exType, c.shipmentID,
			).Scan(&existingID)
			if err == nil && existingID != "" {
				out.UpdatedCount++
				continue
			}

			prio := priorityForExceptionType(c.exType)
			slaDeadline := now.Add(time.Duration(adaptiveSLAHours(prio)) * time.Hour).Format("2006-01-02 15:04:05")
			caseID := newID("ex")
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO exception_cases
				 (id, exception_type, entity_type, entity_id, status, priority, created_at, updated_at, sla_deadline, sla_status)
				 VALUES (?, ?, 'shipment', ?, 'OPEN', ?, ?, ?, ?, 'ON_TIME')`,
				caseID, c.exType, c.shipmentID, prio, nowStr, nowStr, slaDeadline,
			); err != nil {
				return err
			}
			_, _ = tx.ExecContext(ctx,
				`INSERT INTO exception_lifecycle_events (id, exception_case_id, old_status, new_status, actor, created_at)
				 VALUES (?, ?, 'NONE', 'OPEN', ?, ?)`,
				newID("exev"), caseID, actor, nowStr,
			)
			out.CreatedCount++
		}

		// Self-heal stale IN_PROGRESS > 14d back to OPEN
		res, err := tx.ExecContext(ctx,
			`UPDATE exception_cases SET status = 'OPEN', assigned_to = NULL, updated_at = ?
			 WHERE status = 'IN_PROGRESS' AND COALESCE(assigned_at, updated_at) < ?`,
			nowStr, fourteenDaysAgo,
		)
		if err == nil {
			n, _ := res.RowsAffected()
			out.SelfHealedCount = int(n)
		}

		// Mark SLA breaches & log escalations cleanly
		breachRows, err := tx.QueryContext(ctx,
			`SELECT id, exception_type FROM exception_cases
			 WHERE status IN ('OPEN', 'IN_PROGRESS') AND sla_status = 'ON_TIME' AND sla_deadline IS NOT NULL AND sla_deadline < ?`,
			nowStr,
		)
		if err == nil {
			var breached [][2]string
			for breachRows.Next() {
				var id, exType string
				if breachRows.Scan(&id, &exType) == nil {
					breached = append(breached, [2]string{id, exType})
				}
			}
			_ = breachRows.Close()

			for _, b := range breached {
				_, _ = tx.ExecContext(ctx,
					`UPDATE exception_cases SET sla_status = 'BREACHED', priority = 'CRITICAL', updated_at = ? WHERE id = ?`,
					nowStr, b[0],
				)
				_, _ = tx.ExecContext(ctx,
					`INSERT INTO exception_escalation_log (id, exception_case_id, escalation_level, escalated_to, reason, created_at)
					 VALUES (?, ?, 1, 'admin', 'SLA deadline exceeded', ?)`,
					newID("esc"), b[0], nowStr,
				)
				out.EscalatedCount++
			}
		}

		_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM exception_cases WHERE status IN ('OPEN', 'IN_PROGRESS')`).Scan(&out.ActiveOpenCount)
		return nil
	})
	if err != nil {
		return models.ExceptionDetectionResult{}, fmt.Errorf("detect exceptions: %w", err)
	}
	return out, nil
}

// ListExceptions returns exception cases optionally filtered by status or priority.
func (s *Service) ListExceptions(statusFilter, priorityFilter string) ([]models.ExceptionCase, error) {
	if _, err := s.security.RequireAnyPermission(security.PermAutomationView, security.PermAutomationOpsCenter, security.PermDataRead); err != nil {
		return nil, err
	}
	where := []string{"1=1"}
	var args []any
	if st := strings.TrimSpace(statusFilter); st != "" && !strings.EqualFold(st, "all") {
		where = append(where, "status = ?")
		args = append(args, strings.ToUpper(st))
	}
	if pr := strings.TrimSpace(priorityFilter); pr != "" && !strings.EqualFold(pr, "all") {
		where = append(where, "priority = ?")
		args = append(args, strings.ToUpper(pr))
	}

	rows, err := s.db.Query(
		`SELECT id, exception_type, entity_type, entity_id, status, priority,
		        COALESCE(assigned_to, ''), COALESCE(assigned_at, ''), created_at, updated_at,
		        COALESCE(resolved_by, ''), COALESCE(resolved_at, ''), COALESCE(sla_deadline, ''), sla_status
		 FROM exception_cases WHERE `+strings.Join(where, " AND ")+` ORDER BY created_at DESC LIMIT 200`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ExceptionCase
	for rows.Next() {
		var ex models.ExceptionCase
		if err := rows.Scan(&ex.ID, &ex.ExceptionType, &ex.EntityType, &ex.EntityID, &ex.Status, &ex.Priority,
			&ex.AssignedTo, &ex.AssignedAt, &ex.CreatedAt, &ex.UpdatedAt, &ex.ResolvedBy, &ex.ResolvedAt, &ex.SLADeadline, &ex.SLAStatus); err != nil {
			return nil, err
		}
		out = append(out, ex)
	}
	if out == nil {
		out = []models.ExceptionCase{}
	}
	return out, rows.Err()
}

// TransitionException enforces valid lifecycle transitions (terminal RESOLVED/IGNORED cannot reopen).
func (s *Service) TransitionException(caseID, newStatus, noteText string) (models.ExceptionCase, error) {
	sess, err := s.security.RequireAnyPermission(security.PermAutomationMutate, security.PermDataWrite)
	if err != nil {
		return models.ExceptionCase{}, err
	}
	target := strings.ToUpper(strings.TrimSpace(newStatus))
	if target != "OPEN" && target != "IN_PROGRESS" && target != "RESOLVED" && target != "IGNORED" {
		return models.ExceptionCase{}, fmt.Errorf("invalid exception status %q", newStatus)
	}

	var curStatus string
	if err := s.db.QueryRow(`SELECT status FROM exception_cases WHERE id = ?`, caseID).Scan(&curStatus); err != nil {
		return models.ExceptionCase{}, fmt.Errorf("exception case %q not found: %w", caseID, err)
	}
	if (curStatus == "RESOLVED" || curStatus == "IGNORED") && (target == "OPEN" || target == "IN_PROGRESS") {
		return models.ExceptionCase{}, fmt.Errorf("cannot transition terminal exception from %s back to %s", curStatus, target)
	}

	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if target == "RESOLVED" || target == "IGNORED" {
			if _, err := tx.Exec(
				`UPDATE exception_cases SET status = ?, resolved_by = ?, resolved_at = ?, updated_at = ? WHERE id = ?`,
				target, sess.UserID, nowStr, nowStr, caseID,
			); err != nil {
				return err
			}
			_, _ = tx.Exec(
				`INSERT INTO exception_resolution_log (id, exception_case_id, resolution_type, resolved_by, notes, created_at)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				newID("exres"), caseID, target, sess.UserID, noteText, nowStr,
			)
		} else {
			if _, err := tx.Exec(
				`UPDATE exception_cases SET status = ?, assigned_to = COALESCE(NULLIF(assigned_to, ''), ?), assigned_at = COALESCE(assigned_at, ?), updated_at = ? WHERE id = ?`,
				target, sess.UserID, nowStr, nowStr, caseID,
			); err != nil {
				return err
			}
		}
		_, _ = tx.Exec(
			`INSERT INTO exception_lifecycle_events (id, exception_case_id, old_status, new_status, actor, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			newID("exev"), caseID, curStatus, target, sess.UserID, nowStr,
		)
		if strings.TrimSpace(noteText) != "" {
			_, _ = tx.Exec(
				`INSERT INTO exception_notes (note_id, exception_case_id, user_id, note_text, created_at)
				 VALUES (?, ?, ?, ?, ?)`,
				newID("exnote"), caseID, sess.UserID, strings.TrimSpace(noteText), nowStr,
			)
		}
		return nil
	})
	if err != nil {
		return models.ExceptionCase{}, err
	}

	_ = s.audit.RecordTableAudit("exception_cases", caseID, "TRANSITION", sess.UserID, map[string]string{"status": curStatus}, map[string]string{"status": target}, nil)
	list, err := s.ListExceptions("all", "all")
	if err != nil {
		return models.ExceptionCase{}, err
	}
	for _, ex := range list {
		if ex.ID == caseID {
			return ex, nil
		}
	}
	return models.ExceptionCase{}, errors.New("updated exception not found")
}
