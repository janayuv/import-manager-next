package boe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/validation"
)

var boeTemplateHeaders = []string{
	"beNumber",
	"beDate",
	"location",
	"totalAssessmentValue",
	"dutyAmount",
	"paymentDate",
	"dutyPaid",
	"challanNumber",
	"refId",
	"transactionId",
}

// Service manages Bill of Entry registry records (boe_details), attachments, and CSV import.
type Service struct {
	db             *sql.DB
	sec            *security.Service
	audit          *audit.Service
	attachmentsDir string
}

// NewService constructs a BOE registry Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service, attachmentsDir string) *Service {
	return &Service{
		db:             db,
		sec:            sec,
		audit:          auditSvc,
		attachmentsDir: attachmentsDir,
	}
}

// List returns active BOE detail records filtered optionally by BE number, location, or challan.
func (s *Service) List(search string) ([]models.BOEDetail, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	where := "deleted_at IS NULL"
	var args []any
	if q := strings.TrimSpace(search); q != "" {
		like := "%" + q + "%"
		where += " AND (be_number LIKE ? OR location LIKE ? OR COALESCE(challan_number,'') LIKE ? OR COALESCE(ref_id,'') LIKE ?)"
		args = append(args, like, like, like, like)
	}
	rows, err := s.db.Query(`
		SELECT id, be_number, be_date, location, total_assessment_value, duty_amount,
		       COALESCE(payment_date,''), COALESCE(duty_paid,0), COALESCE(challan_number,''),
		       COALESCE(ref_id,''), COALESCE(transaction_id,'')
		FROM boe_details
		WHERE `+where+`
		ORDER BY be_date DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.BOEDetail
	for rows.Next() {
		var b models.BOEDetail
		if err := rows.Scan(
			&b.ID, &b.BENumber, &b.BEDate, &b.Location, &b.TotalAssessmentValue, &b.DutyAmount,
			&b.PaymentDate, &b.DutyPaid, &b.ChallanNumber, &b.RefID, &b.TransactionID,
		); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetByID fetches a single non-deleted BOE detail record.
func (s *Service) GetByID(id string) (models.BOEDetail, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.BOEDetail{}, err
	}
	return s.getByIDRaw(id)
}

func (s *Service) getByIDRaw(id string) (models.BOEDetail, error) {
	var b models.BOEDetail
	err := s.db.QueryRow(`
		SELECT id, be_number, be_date, location, total_assessment_value, duty_amount,
		       COALESCE(payment_date,''), COALESCE(duty_paid,0), COALESCE(challan_number,''),
		       COALESCE(ref_id,''), COALESCE(transaction_id,'')
		FROM boe_details
		WHERE id = ? AND deleted_at IS NULL`, strings.TrimSpace(id)).Scan(
		&b.ID, &b.BENumber, &b.BEDate, &b.Location, &b.TotalAssessmentValue, &b.DutyAmount,
		&b.PaymentDate, &b.DutyPaid, &b.ChallanNumber, &b.RefID, &b.TransactionID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.BOEDetail{}, fmt.Errorf("BOE %q not found", id)
	}
	return b, err
}

// Create validates and inserts a new BOE registry entry.
func (s *Service) Create(in models.BOEDetailInput) (models.BOEDetail, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.BOEDetail{}, err
	}
	if err := normalizeBOEInput(&in); err != nil {
		return models.BOEDetail{}, err
	}
	var exists int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM boe_details WHERE be_number = ? AND be_date = ? AND deleted_at IS NULL`, in.BENumber, in.BEDate).Scan(&exists)
	if exists > 0 {
		return models.BOEDetail{}, fmt.Errorf("active BOE record already exists for BE Number %s and Date %s", in.BENumber, in.BEDate)
	}
	// Purge any soft-deleted phantom with the same (be_number, be_date) so the new active record can be inserted cleanly.
	_, _ = s.db.Exec(`DELETE FROM boe_details WHERE be_number = ? AND be_date = ? AND deleted_at IS NOT NULL`, in.BENumber, in.BEDate)

	id := "BOE-" + strings.ToUpper(uuid.NewString()[:8])
	_, err = s.db.Exec(`
		INSERT INTO boe_details (
			id, be_number, be_date, location, total_assessment_value, duty_amount,
			payment_date, duty_paid, challan_number, ref_id, transaction_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.BENumber, in.BEDate, in.Location, in.TotalAssessmentValue, in.DutyAmount,
		nullStr(in.PaymentDate), in.DutyPaid, nullStr(in.ChallanNumber), nullStr(in.RefID), nullStr(in.TransactionID),
	)
	if err != nil {
		return models.BOEDetail{}, fmt.Errorf("create BOE: %w", err)
	}
	created, err := s.getByIDRaw(id)
	if err != nil {
		return models.BOEDetail{}, err
	}
	_ = s.audit.RecordTableAudit("boe_details", id, "INSERT", sess.UserID, nil, created, nil)
	return created, nil
}

// Update updates an existing BOE registry record.
func (s *Service) Update(id string, in models.BOEDetailInput) (models.BOEDetail, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.BOEDetail{}, err
	}
	before, err := s.getByIDRaw(id)
	if err != nil {
		return models.BOEDetail{}, err
	}
	if err := normalizeBOEInput(&in); err != nil {
		return models.BOEDetail{}, err
	}
	_, err = s.db.Exec(`
		UPDATE boe_details SET
			be_number = ?, be_date = ?, location = ?,
			total_assessment_value = ?, duty_amount = ?,
			payment_date = ?, duty_paid = ?, challan_number = ?,
			ref_id = ?, transaction_id = ?
		WHERE id = ? AND deleted_at IS NULL`,
		in.BENumber, in.BEDate, in.Location, in.TotalAssessmentValue, in.DutyAmount,
		nullStr(in.PaymentDate), in.DutyPaid, nullStr(in.ChallanNumber), nullStr(in.RefID), nullStr(in.TransactionID),
		strings.TrimSpace(id),
	)
	if err != nil {
		return models.BOEDetail{}, fmt.Errorf("update BOE %s: %w", id, err)
	}
	after, err := s.getByIDRaw(id)
	if err != nil {
		return models.BOEDetail{}, err
	}
	_ = s.audit.RecordTableAudit("boe_details", id, "UPDATE", sess.UserID, before, after, nil)
	return after, nil
}

// Delete soft-deletes a BOE detail entry after verifying no active calculations depend on it.
func (s *Service) Delete(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	before, err := s.getByIDRaw(id)
	if err != nil {
		return err
	}
	if depErr, err := integrity.CheckDependencies(context.Background(), s.db, "boe_details", id, true); err != nil {
		return err
	} else if depErr != nil {
		return depErr
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE boe_details SET deleted_at = ?, deleted_by = ? WHERE id = ? AND deleted_at IS NULL`, now, sess.UserID, strings.TrimSpace(id)); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("boe_details", id, "DELETE", sess.UserID, before, nil, map[string]any{"softDelete": true})
	return nil
}

// DownloadTemplateCSV returns a 10-column BOE registry CSV template.
func (s *Service) DownloadTemplateCSV() (string, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return "", err
	}
	return validation.EncodeCSV(boeTemplateHeaders, [][]string{{
		"BE-8822110",
		"2026-11-10",
		"INMAA4",
		"1250000",
		"315000",
		"2026-11-11",
		"315000",
		"CH-99881",
		"REF-01",
		"TXN-4455",
	}})
}

// ImportCSV parses BOE registry CSV (requires beNumber, beDate) with duplicate rejection on (beNumber, beDate).
func (s *Service) ImportCSV(content string) (models.CSVImportResult, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.CSVImportResult{}, err
	}
	headers, rows, err := validation.ParseCSV(content, true)
	if err != nil {
		return models.CSVImportResult{}, err
	}
	idx := validation.HeaderIndex(headers)
	res := models.CSVImportResult{TotalRows: len(rows)}

	for i, row := range rows {
		rowNum := i + 2
		beNo := validation.CellByAliases(row, idx, "beNumber", "be_number")
		beDateRaw := validation.CellByAliases(row, idx, "beDate", "be_date")
		if beNo == "" || beDateRaw == "" {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: "beNumber and beDate are required"})
			continue
		}
		beDate, dErr := validation.NormalizeDate(beDateRaw)
		if dErr != nil || beDate == "" {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("invalid beDate %q", beDateRaw)})
			continue
		}

		var exists int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM boe_details WHERE be_number = ? AND be_date = ? AND deleted_at IS NULL`, beNo, beDate).Scan(&exists)
		if exists > 0 {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("duplicate BE (%s, %s)", beNo, beDate)})
			continue
		}

		// Purge any soft-deleted phantom with the same (be_number, be_date) before inserting.
		_, _ = s.db.Exec(`DELETE FROM boe_details WHERE be_number = ? AND be_date = ? AND deleted_at IS NOT NULL`, beNo, beDate)

		location := validation.CellByAliases(row, idx, "location")
		if location == "" {
			location = "INMAA4"
		}
		assVal, _ := strconv.ParseFloat(strings.ReplaceAll(validation.CellByAliases(row, idx, "totalAssessmentValue", "total_assessment_value"), ",", ""), 64)
		dutyAmt, _ := strconv.ParseFloat(strings.ReplaceAll(validation.CellByAliases(row, idx, "dutyAmount", "duty_amount"), ",", ""), 64)
		dutyPaid, _ := strconv.ParseFloat(strings.ReplaceAll(validation.CellByAliases(row, idx, "dutyPaid", "duty_paid"), ",", ""), 64)
		payDate, _ := validation.NormalizeDate(validation.CellByAliases(row, idx, "paymentDate", "payment_date"))

		id := "BOE-" + strings.ToUpper(uuid.NewString()[:8])
		_, err := s.db.Exec(`
			INSERT INTO boe_details (id, be_number, be_date, location, total_assessment_value, duty_amount, payment_date, duty_paid, challan_number, ref_id, transaction_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, beNo, beDate, location, assVal, dutyAmt,
			nullStr(payDate), dutyPaid,
			nullStr(validation.CellByAliases(row, idx, "challanNumber", "challan_number")),
			nullStr(validation.CellByAliases(row, idx, "refId", "ref_id")),
			nullStr(validation.CellByAliases(row, idx, "transactionId", "transaction_id")),
		)
		if err != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: err.Error()})
			continue
		}
		res.ImportedRows++
		_ = s.audit.RecordTableAudit("boe_details", id, "INSERT", sess.UserID, nil, map[string]any{"beNumber": beNo, "beDate": beDate}, nil)
	}
	return res, nil
}

// ListAttachments returns stored file attachments for a boe_calculations ID.
func (s *Service) ListAttachments(calcID string) ([]models.BOEAttachment, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT id, boe_calculation_id, file_name, file_path, uploaded_at
		FROM boe_attachments
		WHERE boe_calculation_id = ?
		ORDER BY uploaded_at DESC`, strings.TrimSpace(calcID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.BOEAttachment
	for rows.Next() {
		var a models.BOEAttachment
		if err := rows.Scan(&a.ID, &a.BOECalculationID, &a.FileName, &a.FilePath, &a.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SaveAttachment writes a BOE attachment safely into the sandboxed attachmentsDir/boe folder and indexes it in DB.
func (s *Service) SaveAttachment(calcID, fileName string, content []byte) (models.BOEAttachment, error) {
	if _, err := s.sec.RequirePermission(security.PermDataWrite); err != nil {
		return models.BOEAttachment{}, err
	}
	if s.attachmentsDir == "" {
		return models.BOEAttachment{}, errors.New("attachments directory is not configured")
	}
	if len(content) == 0 || len(content) > 25*1024*1024 {
		return models.BOEAttachment{}, errors.New("attachment size must be between 1 byte and 25 MB")
	}
	baseName := filepath.Base(strings.TrimSpace(fileName))
	if baseName == "" || baseName == "." || baseName == ".." {
		return models.BOEAttachment{}, errors.New("invalid attachment file name")
	}

	safeDir := filepath.Join(s.attachmentsDir, "boe", strings.TrimSpace(calcID))
	if err := os.MkdirAll(safeDir, 0o700); err != nil {
		return models.BOEAttachment{}, err
	}
	attID := "ATT-" + strings.ToUpper(uuid.NewString()[:8])
	relPath := filepath.Join("boe", strings.TrimSpace(calcID), attID+"_"+baseName)
	if err := os.WriteFile(filepath.Join(s.attachmentsDir, relPath), content, 0o600); err != nil {
		return models.BOEAttachment{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		INSERT INTO boe_attachments (id, boe_calculation_id, file_name, file_path, uploaded_at)
		VALUES (?, ?, ?, ?, ?)`, attID, strings.TrimSpace(calcID), baseName, relPath, now); err != nil {
		return models.BOEAttachment{}, err
	}
	return models.BOEAttachment{
		ID:               attID,
		BOECalculationID: strings.TrimSpace(calcID),
		FileName:         baseName,
		FilePath:         relPath,
		UploadedAt:       now,
	}, nil
}

// DeleteAttachment removes a BOE attachment DB record cleanly.
func (s *Service) DeleteAttachment(id string) error {
	if _, err := s.sec.RequirePermission(security.PermDataDelete); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM boe_attachments WHERE id = ?`, strings.TrimSpace(id))
	return err
}

func normalizeBOEInput(in *models.BOEDetailInput) error {
	in.BENumber = strings.TrimSpace(in.BENumber)
	in.Location = strings.TrimSpace(in.Location)
	if in.BENumber == "" {
		return errors.New("beNumber is required")
	}
	beDate, err := validation.NormalizeDate(in.BEDate)
	if err != nil || beDate == "" {
		return errors.New("valid beDate is required")
	}
	in.BEDate = beDate
	if in.Location == "" {
		in.Location = "INMAA4"
	}
	if in.PaymentDate != "" {
		pDate, pErr := validation.NormalizeDate(in.PaymentDate)
		if pErr != nil {
			return pErr
		}
		in.PaymentDate = pDate
	}
	if in.TotalAssessmentValue < 0 || in.DutyAmount < 0 || in.DutyPaid < 0 {
		return errors.New("monetary BOE values cannot be negative")
	}
	return nil
}

func nullStr(s string) any {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return t
}
