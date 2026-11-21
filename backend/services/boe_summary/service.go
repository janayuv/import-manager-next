package boe_summary

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

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

// SetDB rebinds the service to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
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

// GetByID returns the complete calculation dossier, linked BOE record, attachments, and reconciliation metrics.
func (s *Service) GetByID(calcID string) (models.BOECalculationDetail, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.BOECalculationDetail{}, err
	}
	cleanID := strings.TrimSpace(calcID)
	if cleanID == "" {
		return models.BOECalculationDetail{}, errors.New("calculation ID cannot be empty")
	}

	var (
		d         models.BOECalculationDetail
		formJSON  string
		resJSON   string
		boeID     sql.NullString
		beNum     sql.NullString
		beDate    sql.NullString
		loc       sql.NullString
		boeAssess sql.NullFloat64
		boeDuty   sql.NullFloat64
		payDate   sql.NullString
		dutyPaid  sql.NullFloat64
		challan   sql.NullString
		refID     sql.NullString
		txID      sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT bc.id, bc.shipment_id, COALESCE(bc.boe_id,''), bc.supplier_name, bc.invoice_number,
		       bc.status, bc.form_values_json, bc.calculation_result_json, bc.created_at,
		       bd.be_number, bd.be_date, bd.location, bd.total_assessment_value, bd.duty_amount,
		       bd.payment_date, bd.duty_paid, bd.challan_number, bd.ref_id, bd.transaction_id
		FROM boe_calculations bc
		LEFT JOIN boe_details bd ON bd.id = bc.boe_id
		WHERE bc.id = ? AND bc.deleted_at IS NULL`, cleanID).Scan(
		&d.ID, &d.ShipmentID, &boeID, &d.SupplierName, &d.InvoiceNumber,
		&d.Status, &formJSON, &resJSON, &d.CreatedAt,
		&beNum, &beDate, &loc, &boeAssess, &boeDuty,
		&payDate, &dutyPaid, &challan, &refID, &txID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.BOECalculationDetail{}, fmt.Errorf("BOE calculation %q not found", cleanID)
		}
		return models.BOECalculationDetail{}, err
	}

	d.BOEID = boeID.String
	d.BENumber = beNum.String
	d.BEDate = beDate.String
	d.Location = loc.String
	d.PaymentDate = payDate.String
	d.DutyPaid = dutyPaid.Float64
	d.ChallanNumber = challan.String
	d.RefID = refID.String
	d.TransactionID = txID.String

	_ = json.Unmarshal([]byte(formJSON), &d.Params)
	_ = json.Unmarshal([]byte(resJSON), &d.Result)

	// Fetch attachments if any
	attRows, err := s.db.Query(`
		SELECT id, boe_calculation_id, file_name, file_path, uploaded_at
		FROM boe_attachments
		WHERE boe_calculation_id = ?
		ORDER BY uploaded_at DESC`, cleanID)
	if err == nil {
		defer attRows.Close()
		for attRows.Next() {
			var a models.BOEAttachment
			if err := attRows.Scan(&a.ID, &a.BOECalculationID, &a.FileName, &a.FilePath, &a.UploadedAt); err == nil {
				d.Attachments = append(d.Attachments, a)
			}
		}
	}

	computedDuty := d.Result.CustomsDutyTotal
	cashDuty := d.Result.CashCustomsDutyTotal
	challanDuty := d.DutyPaid
	dutyDiff := challanDuty - computedDuty
	if math.Abs(dutyDiff) > 0.01 && math.Abs(challanDuty-cashDuty) < 0.01 {
		dutyDiff = challanDuty - cashDuty
	}

	d.Reconciliation = models.BOEReconciliationDiff{
		ComputedDuty:           computedDuty,
		CashDuty:               cashDuty,
		DutyPaid:               challanDuty,
		DutyDifference:         dutyDiff,
		IsReconciled:           math.Abs(dutyDiff) <= 1.0,
		AssessableValue:        d.Result.TotalAssessableValue,
		ChallanAssessableValue: boeAssess.Float64,
		AssessableDifference:   boeAssess.Float64 - d.Result.TotalAssessableValue,
	}

	return d, nil
}

// Delete soft-deletes a saved BOE calculation record and moves it to Recycle Bin.
func (s *Service) Delete(calcID string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	cleanID := strings.TrimSpace(calcID)
	if cleanID == "" {
		return errors.New("calculation ID cannot be empty")
	}

	var shipmentID, status string
	err = s.db.QueryRow(`SELECT shipment_id, status FROM boe_calculations WHERE id = ? AND deleted_at IS NULL`, cleanID).Scan(&shipmentID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("BOE calculation %q not found", cleanID)
	}
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE boe_calculations SET deleted_at = ?, deleted_by = ? WHERE id = ? AND deleted_at IS NULL`, now, sess.UserID, cleanID)
	if err != nil {
		return fmt.Errorf("soft delete BOE calculation: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("BOE calculation %q not found", cleanID)
	}

	// If no other active calculation exists on this shipment and shipment is customs-clearance, revert to docu-received
	var otherCalcs int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM boe_calculations WHERE shipment_id = ? AND deleted_at IS NULL`, shipmentID).Scan(&otherCalcs)
	if otherCalcs == 0 {
		_, _ = s.db.Exec(`UPDATE shipments SET status = 'docu-received' WHERE id = ? AND status = 'customs-clearance' AND deleted_at IS NULL`, shipmentID)
	}

	_ = s.audit.RecordTableAudit("boe_calculations", cleanID, "DELETE", sess.UserID, map[string]any{"status": status, "shipmentId": shipmentID}, nil, map[string]any{"softDelete": true})
	_ = s.audit.RecordUserActivity(sess.UserID, "boe_calc.delete", "boe_calculation", cleanID, "SUCCESS", "INFO", map[string]any{
		"shipmentId":  shipmentID,
		"priorStatus": status,
	})
	return nil
}

