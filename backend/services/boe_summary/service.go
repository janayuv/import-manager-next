package boe_summary

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

var validStatuses = map[string]bool{
	"Awaiting BOE Data": true,
	"Discrepancy Found": true,
	"Reconciled":        true,
	"Investigation":     true,
	"Closed":            true,
}

// Service lists saved BOE calculation summaries and manages reconciliation statuses.
type Service struct {
	db    *sql.DB
	sec   *security.Service
	audit *audit.Service
}

// NewService constructs a boe_summary Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{db: db, sec: sec, audit: auditSvc}
}

// List returns saved BOE calculation rows joined with linked boe_details.
func (s *Service) List(search, status string) ([]models.BOECalculationSummary, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	where := []string{"bc.deleted_at IS NULL"}
	var args []any
	if q := strings.TrimSpace(search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(bc.id LIKE ? OR bc.shipment_id LIKE ? OR bc.invoice_number LIKE ? OR bc.supplier_name LIKE ? OR COALESCE(bd.be_number,'') LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	if st := strings.TrimSpace(status); st != "" {
		where = append(where, "bc.status = ?")
		args = append(args, st)
	}

	rows, err := s.db.Query(`
		SELECT bc.id, bc.shipment_id, COALESCE(bc.boe_id,''), COALESCE(bd.be_number,''), COALESCE(bd.be_date,''),
		       bc.supplier_name, bc.invoice_number, bc.status, COALESCE(bd.duty_paid,0), bc.calculation_result_json, bc.created_at
		FROM boe_calculations bc
		LEFT JOIN boe_details bd ON bd.id = bc.boe_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY bc.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.BOECalculationSummary
	for rows.Next() {
		var row models.BOECalculationSummary
		var resJSON string
		if err := rows.Scan(
			&row.ID, &row.ShipmentID, &row.BOEID, &row.BENumber, &row.BEDate,
			&row.SupplierName, &row.InvoiceNumber, &row.Status, &row.DutyPaid, &resJSON, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		var calc models.BOECalculationResult
		if json.Unmarshal([]byte(resJSON), &calc) == nil {
			row.CustomsDutyTotal = calc.CustomsDutyTotal
			row.AssessableTotal = calc.TotalAssessableValue
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// UpdateStatus updates reconciliation status of a boe_calculations row.
func (s *Service) UpdateStatus(calcID, newStatus string) error {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return err
	}
	if !validStatuses[newStatus] {
		return fmt.Errorf("invalid reconciliation status %q", newStatus)
	}
	res, err := s.db.Exec(`UPDATE boe_calculations SET status = ? WHERE id = ? AND deleted_at IS NULL`, newStatus, strings.TrimSpace(calcID))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("BOE calculation %q not found", calcID)
	}
	_ = s.audit.RecordUserActivity(sess.UserID, "boe_calc.status_update", "boe_calculation", calcID, "SUCCESS", "INFO", map[string]any{
		"status": newStatus,
	})
	return nil
}
