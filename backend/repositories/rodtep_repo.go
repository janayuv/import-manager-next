package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
)

// RoDTEPRepository handles persistence, transactions, and audit records for RoDTEP scrips,
// reservations/allocations, append-only debit ledger entries, reconciliations, and port summaries.
type RoDTEPRepository struct {
	db *sql.DB
}

// NewRoDTEPRepository constructs a RoDTEPRepository bound to db.
func NewRoDTEPRepository(db *sql.DB) *RoDTEPRepository {
	return &RoDTEPRepository{db: db}
}

// nullStr converts empty strings to sql.NullString.
func nullStr(s string) sql.NullString {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

// decorateScrip calculates derived display fields (INR approximations and days to expiry).
func decorateScrip(s *models.RoDTEPScrip) {
	s.OriginalValueINR = float64(s.OriginalValuePaise) / 100.0
	s.ConfirmedBalINR = float64(s.ConfirmedBalancePaise) / 100.0
	s.PendingAllocINR = float64(s.PendingAllocatedPaise) / 100.0
	s.NetAvailableINR = float64(s.NetAvailablePaise) / 100.0

	if s.ExpiryDate != "" {
		if t, err := time.Parse("2006-01-02", s.ExpiryDate); err == nil {
			days := int(time.Until(t).Hours() / 24)
			if days < 0 {
				days = 0
			}
			s.DaysToExpiry = days
		}
	}
}

// decorateAllocation calculates INR display fields and outstanding balance.
func decorateAllocation(a *models.RoDTEPAllocation) {
	a.OutstandingPaise = a.AllocatedAmountPaise - a.ConfirmedAmountPaise - a.ReleasedAmountPaise
	if a.OutstandingPaise < 0 {
		a.OutstandingPaise = 0
	}
	a.AllocatedINR = float64(a.AllocatedAmountPaise) / 100.0
	a.ConfirmedINR = float64(a.ConfirmedAmountPaise) / 100.0
	a.OutstandingINR = float64(a.OutstandingPaise) / 100.0
}

// decorateDebitLedger calculates INR display fields.
func decorateDebitLedger(l *models.RoDTEPDebitLedger) {
	l.AmountINR = float64(l.AmountPaise) / 100.0
	l.RemainingINR = float64(l.RemainingConfirmedBalancePaise) / 100.0
}

// decorateReconciliation calculates INR display fields.
func decorateReconciliation(r *models.RoDTEPReconciliation) {
	r.ICEGATEBalanceINR = float64(r.ICEGATEStatementBalancePaise) / 100.0
	r.ERPBalanceINR = float64(r.ERPConfirmedBalancePaise) / 100.0
	r.DiscrepancyINR = float64(r.DiscrepancyPaise) / 100.0
}

// -------------------------------------------------------------------------
// Scrips Master Operations
// -------------------------------------------------------------------------

// CreateScrip records a new RoDTEP scrip into the master repository.
func (r *RoDTEPRepository) CreateScrip(input models.RoDTEPScripInput, operator string) (models.RoDTEPScrip, error) {
	scripNum := strings.TrimSpace(input.ScripNumber)
	if scripNum == "" {
		return models.RoDTEPScrip{}, errors.New("scrip number is required")
	}
	portCode := strings.ToUpper(strings.TrimSpace(input.ScripPortCode))
	if portCode != models.PortINMAA1 && portCode != models.PortINMAA4 {
		return models.RoDTEPScrip{}, fmt.Errorf("invalid scrip port code %q: must be INMAA1 or INMAA4", portCode)
	}
	if input.OriginalValuePaise <= 0 {
		return models.RoDTEPScrip{}, errors.New("original value must be greater than zero")
	}
	if input.OpeningBalancePaise < 0 {
		return models.RoDTEPScrip{}, errors.New("opening balance cannot be negative")
	}
	if input.OpeningBalancePaise > input.OriginalValuePaise {
		return models.RoDTEPScrip{}, errors.New("opening balance cannot exceed original value")
	}

	acqType := strings.TrimSpace(input.AcquisitionType)
	if acqType == "" {
		acqType = models.AcquisitionDirectGeneration
	}
	if acqType != models.AcquisitionDirectGeneration && acqType != models.AcquisitionOpeningBalance {
		return models.RoDTEPScrip{}, fmt.Errorf("invalid acquisition type %q", acqType)
	}

	// Issue date validation
	if len(input.IssueDate) != 10 {
		return models.RoDTEPScrip{}, errors.New("issue date must be in YYYY-MM-DD format")
	}
	issueTime, err := time.Parse("2006-01-02", input.IssueDate)
	if err != nil {
		return models.RoDTEPScrip{}, fmt.Errorf("invalid issue date format: %w", err)
	}

	// Expiry date: if omitted, default to 2 years (24 months) statutory validity
	expiryDate := strings.TrimSpace(input.ExpiryDate)
	if expiryDate == "" {
		expiryDate = issueTime.AddDate(2, 0, 0).Format("2006-01-02")
	} else if len(expiryDate) != 10 {
		return models.RoDTEPScrip{}, errors.New("expiry date must be in YYYY-MM-DD format")
	}

	preUtil := input.PreERPUtilizedPaise
	if preUtil == 0 && input.OriginalValuePaise > input.OpeningBalancePaise {
		preUtil = input.OriginalValuePaise - input.OpeningBalancePaise
	}
	confirmedBal := input.OpeningBalancePaise

	status := models.ScripStatusActive
	if confirmedBal == 0 {
		status = models.ScripStatusExhausted
	}

	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")

	err = transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO rodtep_scrips (
				id, scrip_number, scrip_port_code, acquisition_type,
				issue_date, expiry_date, original_value_paise, opening_balance_paise,
				pre_erp_utilized_paise, confirmed_utilized_paise, reversal_recredit_paise,
				confirmed_balance_paise, pending_allocated_paise, status,
				icegate_user_id, registration_details, document_ref, remarks,
				opening_verified_at, opening_verified_by, opening_evidence_ref,
				created_at, updated_at
			) VALUES (
				?, ?, ?, ?,
				?, ?, ?, ?,
				?, 0, 0,
				?, 0, ?,
				?, ?, ?, ?,
				?, ?, ?,
				?, ?
			)`,
			id, scripNum, portCode, acqType,
			input.IssueDate, expiryDate, input.OriginalValuePaise, input.OpeningBalancePaise,
			preUtil,
			confirmedBal, status,
			nullStr(input.ICEGATEUserID), nullStr(input.RegistrationDetails), nullStr(input.DocumentRef), nullStr(input.Remarks),
			nullStr(input.OpeningVerifiedAt), nullStr(input.OpeningVerifiedBy), nullStr(input.OpeningEvidenceRef),
			now, now,
		)
		if err != nil {
			return fmt.Errorf("insert scrip: %w", err)
		}

		// Insert initial audit record
		auditID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO rodtep_audit_logs (
				id, event_type, scrip_id, amount_paise, reason, performed_by, performed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			auditID, models.AuditEventScripCreated, id, input.OriginalValuePaise,
			fmt.Sprintf("Scrip %s created with opening balance %d paise", scripNum, input.OpeningBalancePaise),
			operator, now,
		)
		if err != nil {
			return fmt.Errorf("insert initial audit log: %w", err)
		}
		return nil
	})

	if err != nil {
		return models.RoDTEPScrip{}, err
	}

	return r.GetScripByID(id)
}

// UpdateScrip updates non-financial metadata of an existing scrip.
func (r *RoDTEPRepository) UpdateScrip(id string, input models.RoDTEPScripInput, operator string) (models.RoDTEPScrip, error) {
	current, err := r.GetScripByID(id)
	if err != nil {
		return models.RoDTEPScrip{}, err
	}

	portCode := strings.ToUpper(strings.TrimSpace(input.ScripPortCode))
	if portCode == "" {
		portCode = current.ScripPortCode
	}
	if portCode != models.PortINMAA1 && portCode != models.PortINMAA4 {
		return models.RoDTEPScrip{}, fmt.Errorf("invalid scrip port code %q: must be INMAA1 or INMAA4", portCode)
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	err = transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			UPDATE rodtep_scrips
			SET scrip_port_code = ?,
			    icegate_user_id = ?,
			    registration_details = ?,
			    document_ref = ?,
			    remarks = ?,
			    updated_at = ?
			WHERE id = ? AND deleted_at IS NULL`,
			portCode,
			nullStr(input.ICEGATEUserID),
			nullStr(input.RegistrationDetails),
			nullStr(input.DocumentRef),
			nullStr(input.Remarks),
			now,
			id,
		)
		if err != nil {
			return fmt.Errorf("update scrip: %w", err)
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			return errors.New("scrip not found or deleted")
		}

		auditID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO rodtep_audit_logs (
				id, event_type, scrip_id, reason, performed_by, performed_at
			) VALUES (?, ?, ?, ?, ?, ?)`,
			auditID, models.AuditEventScripUpdated, id,
			"Scrip metadata updated", operator, now,
		)
		return err
	})

	if err != nil {
		return models.RoDTEPScrip{}, err
	}

	return r.GetScripByID(id)
}

// VerifyOpeningBalance records explicit opening balance verification with supporting evidence.
func (r *RoDTEPRepository) VerifyOpeningBalance(id string, verifiedBy, evidenceRef string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	return transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			UPDATE rodtep_scrips
			SET opening_verified_at = ?,
			    opening_verified_by = ?,
			    opening_evidence_ref = ?,
			    updated_at = ?
			WHERE id = ? AND deleted_at IS NULL`,
			now, verifiedBy, evidenceRef, now, id,
		)
		if err != nil {
			return err
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			return errors.New("scrip not found")
		}

		auditID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO rodtep_audit_logs (
				id, event_type, scrip_id, reason, performed_by, performed_at
			) VALUES (?, ?, ?, ?, ?, ?)`,
			auditID, models.AuditEventScripUpdated, id,
			fmt.Sprintf("Opening balance verified against %s", evidenceRef),
			verifiedBy, now,
		)
		return err
	})
}

// SoftDeleteScrip soft-deletes a scrip if no debits or active allocations exist.
func (r *RoDTEPRepository) SoftDeleteScrip(id string, operator string) error {
	scrip, err := r.GetScripByID(id)
	if err != nil {
		return err
	}
	if scrip.ConfirmedUtilizedPaise > 0 {
		return errors.New("cannot delete scrip with confirmed debits in statutory ledger")
	}
	if scrip.PendingAllocatedPaise > 0 {
		return errors.New("cannot delete scrip with active pending reservations")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	return transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			UPDATE rodtep_scrips
			SET deleted_at = ?, deleted_by = ?
			WHERE id = ?`,
			now, operator, id,
		)
		if err != nil {
			return err
		}

		auditID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO rodtep_audit_logs (
				id, event_type, scrip_id, reason, performed_by, performed_at
			) VALUES (?, ?, ?, ?, ?, ?)`,
			auditID, models.AuditEventScripStatusChange, id,
			"Scrip soft deleted", operator, now,
		)
		return err
	})
}

// GetScripByID retrieves a single scrip by its primary key.
func (r *RoDTEPRepository) GetScripByID(id string) (models.RoDTEPScrip, error) {
	row := r.db.QueryRow(`
		SELECT id, scrip_number, scrip_port_code, acquisition_type,
		       issue_date, expiry_date, original_value_paise, opening_balance_paise,
		       pre_erp_utilized_paise, confirmed_utilized_paise, reversal_recredit_paise,
		       confirmed_balance_paise, pending_allocated_paise, status,
		       COALESCE(icegate_user_id, ''), COALESCE(registration_details, ''),
		       COALESCE(document_ref, ''), COALESCE(remarks, ''),
		       COALESCE(opening_verified_at, ''), COALESCE(opening_verified_by, ''),
		       COALESCE(opening_evidence_ref, ''),
		       created_at, updated_at
		FROM rodtep_scrips
		WHERE id = ? AND deleted_at IS NULL`, id)

	var s models.RoDTEPScrip
	err := row.Scan(
		&s.ID, &s.ScripNumber, &s.ScripPortCode, &s.AcquisitionType,
		&s.IssueDate, &s.ExpiryDate, &s.OriginalValuePaise, &s.OpeningBalancePaise,
		&s.PreERPUtilizedPaise, &s.ConfirmedUtilizedPaise, &s.ReversalRecreditPaise,
		&s.ConfirmedBalancePaise, &s.PendingAllocatedPaise, &s.Status,
		&s.ICEGATEUserID, &s.RegistrationDetails,
		&s.DocumentRef, &s.Remarks,
		&s.OpeningVerifiedAt, &s.OpeningVerifiedBy,
		&s.OpeningEvidenceRef,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.RoDTEPScrip{}, fmt.Errorf("scrip %q not found", id)
	}
	if err != nil {
		return models.RoDTEPScrip{}, fmt.Errorf("get scrip by id: %w", err)
	}

	s.NetAvailablePaise = s.ConfirmedBalancePaise - s.PendingAllocatedPaise
	if s.NetAvailablePaise < 0 {
		s.NetAvailablePaise = 0
	}
	decorateScrip(&s)
	return s, nil
}

// GetScripByNumber retrieves a single active scrip by its unique scrip number.
func (r *RoDTEPRepository) GetScripByNumber(scripNumber string) (models.RoDTEPScrip, error) {
	row := r.db.QueryRow(`
		SELECT id, scrip_number, scrip_port_code, acquisition_type,
		       issue_date, expiry_date, original_value_paise, opening_balance_paise,
		       pre_erp_utilized_paise, confirmed_utilized_paise, reversal_recredit_paise,
		       confirmed_balance_paise, pending_allocated_paise, status,
		       COALESCE(icegate_user_id, ''), COALESCE(registration_details, ''),
		       COALESCE(document_ref, ''), COALESCE(remarks, ''),
		       COALESCE(opening_verified_at, ''), COALESCE(opening_verified_by, ''),
		       COALESCE(opening_evidence_ref, ''),
		       created_at, updated_at
		FROM rodtep_scrips
		WHERE scrip_number = ? AND deleted_at IS NULL`, strings.TrimSpace(scripNumber))

	var s models.RoDTEPScrip
	err := row.Scan(
		&s.ID, &s.ScripNumber, &s.ScripPortCode, &s.AcquisitionType,
		&s.IssueDate, &s.ExpiryDate, &s.OriginalValuePaise, &s.OpeningBalancePaise,
		&s.PreERPUtilizedPaise, &s.ConfirmedUtilizedPaise, &s.ReversalRecreditPaise,
		&s.ConfirmedBalancePaise, &s.PendingAllocatedPaise, &s.Status,
		&s.ICEGATEUserID, &s.RegistrationDetails,
		&s.DocumentRef, &s.Remarks,
		&s.OpeningVerifiedAt, &s.OpeningVerifiedBy,
		&s.OpeningEvidenceRef,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.RoDTEPScrip{}, fmt.Errorf("scrip with number %q not found", scripNumber)
	}
	if err != nil {
		return models.RoDTEPScrip{}, fmt.Errorf("get scrip by number: %w", err)
	}

	s.NetAvailablePaise = s.ConfirmedBalancePaise - s.PendingAllocatedPaise
	if s.NetAvailablePaise < 0 {
		s.NetAvailablePaise = 0
	}
	decorateScrip(&s)
	return s, nil
}

// ListScrips returns scrips matching optional portCode, status, and text search filters.
func (r *RoDTEPRepository) ListScrips(portCode, status, search string) ([]models.RoDTEPScrip, error) {
	var conds []string
	var args []any

	conds = append(conds, "deleted_at IS NULL")

	if p := strings.ToUpper(strings.TrimSpace(portCode)); p != "" && p != "ALL" && p != "COMBINED" {
		conds = append(conds, "scrip_port_code = ?")
		args = append(args, p)
	}
	if s := strings.ToUpper(strings.TrimSpace(status)); s != "" && s != "ALL" {
		conds = append(conds, "status = ?")
		args = append(args, s)
	}
	if q := strings.TrimSpace(search); q != "" {
		conds = append(conds, "(scrip_number LIKE ? OR remarks LIKE ? OR document_ref LIKE ?)")
		pattern := "%" + q + "%"
		args = append(args, pattern, pattern, pattern)
	}

	query := `
		SELECT id, scrip_number, scrip_port_code, acquisition_type,
		       issue_date, expiry_date, original_value_paise, opening_balance_paise,
		       pre_erp_utilized_paise, confirmed_utilized_paise, reversal_recredit_paise,
		       confirmed_balance_paise, pending_allocated_paise, status,
		       COALESCE(icegate_user_id, ''), COALESCE(registration_details, ''),
		       COALESCE(document_ref, ''), COALESCE(remarks, ''),
		       COALESCE(opening_verified_at, ''), COALESCE(opening_verified_by, ''),
		       COALESCE(opening_evidence_ref, ''),
		       created_at, updated_at
		FROM rodtep_scrips
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY expiry_date ASC, issue_date ASC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list scrips: %w", err)
	}
	defer rows.Close()

	result := make([]models.RoDTEPScrip, 0)
	for rows.Next() {
		var s models.RoDTEPScrip
		err := rows.Scan(
			&s.ID, &s.ScripNumber, &s.ScripPortCode, &s.AcquisitionType,
			&s.IssueDate, &s.ExpiryDate, &s.OriginalValuePaise, &s.OpeningBalancePaise,
			&s.PreERPUtilizedPaise, &s.ConfirmedUtilizedPaise, &s.ReversalRecreditPaise,
			&s.ConfirmedBalancePaise, &s.PendingAllocatedPaise, &s.Status,
			&s.ICEGATEUserID, &s.RegistrationDetails,
			&s.DocumentRef, &s.Remarks,
			&s.OpeningVerifiedAt, &s.OpeningVerifiedBy,
			&s.OpeningEvidenceRef,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan scrip: %w", err)
		}
		s.NetAvailablePaise = s.ConfirmedBalancePaise - s.PendingAllocatedPaise
		if s.NetAvailablePaise < 0 {
			s.NetAvailablePaise = 0
		}
		decorateScrip(&s)
		result = append(result, s)
	}
	return result, nil
}

// -------------------------------------------------------------------------
// Allocation (Stage 1 Reservation) Operations
// -------------------------------------------------------------------------

// CreateAllocation inserts an allocation record inside the provided transaction.
func (r *RoDTEPRepository) CreateAllocation(tx *sql.Tx, in models.RoDTEPAllocation) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := tx.Exec(`
		INSERT INTO rodtep_allocations (
			id, allocation_ref, scrip_id, boe_id, boe_number, boe_date, boe_port_code,
			allocated_amount_paise, confirmed_amount_paise, released_amount_paise,
			status, operator_user_id, operator_name, remarks,
			created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, 0, 0,
			?, ?, ?, ?,
			?, ?
		)`,
		in.ID, in.AllocationRef, in.ScripID, in.BOEID, in.BOENumber, in.BOEDate, in.BOEPortCode,
		in.AllocatedAmountPaise,
		in.Status, in.OperatorUserID, in.OperatorName, nullStr(in.Remarks),
		now, now,
	)
	if err != nil {
		return fmt.Errorf("create allocation: %w", err)
	}
	return nil
}

// GetAllocationByID retrieves a single allocation by ID.
func (r *RoDTEPRepository) GetAllocationByID(id string) (models.RoDTEPAllocation, error) {
	row := r.db.QueryRow(`
		SELECT a.id, a.allocation_ref, a.scrip_id, s.scrip_number, s.scrip_port_code,
		       a.boe_id, a.boe_number, a.boe_date, a.boe_port_code,
		       a.allocated_amount_paise, a.confirmed_amount_paise, a.released_amount_paise,
		       a.status, a.operator_user_id, a.operator_name, COALESCE(a.remarks, ''),
		       a.created_at, a.updated_at
		FROM rodtep_allocations a
		JOIN rodtep_scrips s ON a.scrip_id = s.id
		WHERE a.id = ?`, id)

	var a models.RoDTEPAllocation
	err := row.Scan(
		&a.ID, &a.AllocationRef, &a.ScripID, &a.ScripNumber, &a.ScripPortCode,
		&a.BOEID, &a.BOENumber, &a.BOEDate, &a.BOEPortCode,
		&a.AllocatedAmountPaise, &a.ConfirmedAmountPaise, &a.ReleasedAmountPaise,
		&a.Status, &a.OperatorUserID, &a.OperatorName, &a.Remarks,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.RoDTEPAllocation{}, fmt.Errorf("allocation %q not found", id)
	}
	if err != nil {
		return models.RoDTEPAllocation{}, fmt.Errorf("get allocation: %w", err)
	}

	decorateAllocation(&a)
	return a, nil
}

// ListAllocationsByBOE returns all allocations associated with a BOE.
func (r *RoDTEPRepository) ListAllocationsByBOE(boeID string) ([]models.RoDTEPAllocation, error) {
	rows, err := r.db.Query(`
		SELECT a.id, a.allocation_ref, a.scrip_id, s.scrip_number, s.scrip_port_code,
		       a.boe_id, a.boe_number, a.boe_date, a.boe_port_code,
		       a.allocated_amount_paise, a.confirmed_amount_paise, a.released_amount_paise,
		       a.status, a.operator_user_id, a.operator_name, COALESCE(a.remarks, ''),
		       a.created_at, a.updated_at
		FROM rodtep_allocations a
		JOIN rodtep_scrips s ON a.scrip_id = s.id
		WHERE a.boe_id = ?
		ORDER BY a.created_at ASC`, boeID)
	if err != nil {
		return nil, fmt.Errorf("list allocations by boe: %w", err)
	}
	defer rows.Close()

	result := make([]models.RoDTEPAllocation, 0)
	for rows.Next() {
		var a models.RoDTEPAllocation
		err := rows.Scan(
			&a.ID, &a.AllocationRef, &a.ScripID, &a.ScripNumber, &a.ScripPortCode,
			&a.BOEID, &a.BOENumber, &a.BOEDate, &a.BOEPortCode,
			&a.AllocatedAmountPaise, &a.ConfirmedAmountPaise, &a.ReleasedAmountPaise,
			&a.Status, &a.OperatorUserID, &a.OperatorName, &a.Remarks,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan allocation: %w", err)
		}
		decorateAllocation(&a)
		result = append(result, a)
	}
	return result, nil
}

// ListAllocationsByScrip returns all allocations associated with a scrip.
func (r *RoDTEPRepository) ListAllocationsByScrip(scripID string) ([]models.RoDTEPAllocation, error) {
	rows, err := r.db.Query(`
		SELECT a.id, a.allocation_ref, a.scrip_id, s.scrip_number, s.scrip_port_code,
		       a.boe_id, a.boe_number, a.boe_date, a.boe_port_code,
		       a.allocated_amount_paise, a.confirmed_amount_paise, a.released_amount_paise,
		       a.status, a.operator_user_id, a.operator_name, COALESCE(a.remarks, ''),
		       a.created_at, a.updated_at
		FROM rodtep_allocations a
		JOIN rodtep_scrips s ON a.scrip_id = s.id
		WHERE a.scrip_id = ?
		ORDER BY a.created_at DESC`, scripID)
	if err != nil {
		return nil, fmt.Errorf("list allocations by scrip: %w", err)
	}
	defer rows.Close()

	result := make([]models.RoDTEPAllocation, 0)
	for rows.Next() {
		var a models.RoDTEPAllocation
		err := rows.Scan(
			&a.ID, &a.AllocationRef, &a.ScripID, &a.ScripNumber, &a.ScripPortCode,
			&a.BOEID, &a.BOENumber, &a.BOEDate, &a.BOEPortCode,
			&a.AllocatedAmountPaise, &a.ConfirmedAmountPaise, &a.ReleasedAmountPaise,
			&a.Status, &a.OperatorUserID, &a.OperatorName, &a.Remarks,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan allocation: %w", err)
		}
		decorateAllocation(&a)
		result = append(result, a)
	}
	return result, nil
}

// GetActiveAllocationsTotalByBOE calculates active outstanding reserved paise for a BOE.
func (r *RoDTEPRepository) GetActiveAllocationsTotalByBOE(boeID string) (int64, error) {
	var total int64
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM(allocated_amount_paise - confirmed_amount_paise - released_amount_paise), 0)
		FROM rodtep_allocations
		WHERE boe_id = ? AND status IN ('ALLOCATED', 'PARTIALLY_CONFIRMED')`, boeID).Scan(&total)
	return total, err
}

// -------------------------------------------------------------------------
// Debit Ledger (Stage 2 Confirmation & Reversals) Operations
// -------------------------------------------------------------------------

// CreateDebitLedgerEntry inserts an append-only row into rodtep_debit_ledger inside tx.
func (r *RoDTEPRepository) CreateDebitLedgerEntry(tx *sql.Tx, in models.RoDTEPDebitLedger) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := tx.Exec(`
		INSERT INTO rodtep_debit_ledger (
			id, transaction_ref, allocation_id, scrip_id, scrip_number, scrip_port_code,
			boe_id, boe_number, boe_date, boe_port_code,
			tx_type, amount_paise, previous_confirmed_balance_paise, remaining_confirmed_balance_paise,
			icegate_transaction_ref, challan_number, debit_date,
			operator_user_id, operator_name, supporting_doc_ref, remarks, reversal_of_id,
			created_at
		) VALUES (
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?, ?,
			?
		)`,
		in.ID, in.TransactionRef, nullStr(in.AllocationID), in.ScripID, in.ScripNumber, in.ScripPortCode,
		in.BOEID, in.BOENumber, in.BOEDate, in.BOEPortCode,
		in.TxType, in.AmountPaise, in.PreviousConfirmedBalancePaise, in.RemainingConfirmedBalancePaise,
		nullStr(in.ICEGATETransactionRef), nullStr(in.ChallanNumber), in.DebitDate,
		in.OperatorUserID, in.OperatorName, nullStr(in.SupportingDocRef), nullStr(in.Remarks), nullStr(in.ReversalOfID),
		now,
	)
	if err != nil {
		return fmt.Errorf("create debit ledger entry: %w", err)
	}
	return nil
}

// GetDebitLedgerByID retrieves a ledger entry by ID.
func (r *RoDTEPRepository) GetDebitLedgerByID(id string) (models.RoDTEPDebitLedger, error) {
	row := r.db.QueryRow(`
		SELECT id, transaction_ref, COALESCE(allocation_id, ''),
		       scrip_id, scrip_number, scrip_port_code,
		       boe_id, boe_number, boe_date, boe_port_code,
		       tx_type, amount_paise, previous_confirmed_balance_paise, remaining_confirmed_balance_paise,
		       COALESCE(icegate_transaction_ref, ''), COALESCE(challan_number, ''), debit_date,
		       operator_user_id, operator_name, COALESCE(supporting_doc_ref, ''),
		       COALESCE(remarks, ''), COALESCE(reversal_of_id, ''), created_at
		FROM rodtep_debit_ledger
		WHERE id = ?`, id)

	var l models.RoDTEPDebitLedger
	err := row.Scan(
		&l.ID, &l.TransactionRef, &l.AllocationID,
		&l.ScripID, &l.ScripNumber, &l.ScripPortCode,
		&l.BOEID, &l.BOENumber, &l.BOEDate, &l.BOEPortCode,
		&l.TxType, &l.AmountPaise, &l.PreviousConfirmedBalancePaise, &l.RemainingConfirmedBalancePaise,
		&l.ICEGATETransactionRef, &l.ChallanNumber, &l.DebitDate,
		&l.OperatorUserID, &l.OperatorName, &l.SupportingDocRef,
		&l.Remarks, &l.ReversalOfID, &l.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.RoDTEPDebitLedger{}, fmt.Errorf("debit ledger entry %q not found", id)
	}
	if err != nil {
		return models.RoDTEPDebitLedger{}, fmt.Errorf("get debit ledger: %w", err)
	}

	decorateDebitLedger(&l)
	return l, nil
}

// ListDebitLedgerByScrip lists all ledger transactions for a scrip.
func (r *RoDTEPRepository) ListDebitLedgerByScrip(scripID string) ([]models.RoDTEPDebitLedger, error) {
	rows, err := r.db.Query(`
		SELECT id, transaction_ref, COALESCE(allocation_id, ''),
		       scrip_id, scrip_number, scrip_port_code,
		       boe_id, boe_number, boe_date, boe_port_code,
		       tx_type, amount_paise, previous_confirmed_balance_paise, remaining_confirmed_balance_paise,
		       COALESCE(icegate_transaction_ref, ''), COALESCE(challan_number, ''), debit_date,
		       operator_user_id, operator_name, COALESCE(supporting_doc_ref, ''),
		       COALESCE(remarks, ''), COALESCE(reversal_of_id, ''), created_at
		FROM rodtep_debit_ledger
		WHERE scrip_id = ?
		ORDER BY debit_date DESC, created_at DESC`, scripID)
	if err != nil {
		return nil, fmt.Errorf("list debit ledger by scrip: %w", err)
	}
	defer rows.Close()

	result := make([]models.RoDTEPDebitLedger, 0)
	for rows.Next() {
		var l models.RoDTEPDebitLedger
		err := rows.Scan(
			&l.ID, &l.TransactionRef, &l.AllocationID,
			&l.ScripID, &l.ScripNumber, &l.ScripPortCode,
			&l.BOEID, &l.BOENumber, &l.BOEDate, &l.BOEPortCode,
			&l.TxType, &l.AmountPaise, &l.PreviousConfirmedBalancePaise, &l.RemainingConfirmedBalancePaise,
			&l.ICEGATETransactionRef, &l.ChallanNumber, &l.DebitDate,
			&l.OperatorUserID, &l.OperatorName, &l.SupportingDocRef,
			&l.Remarks, &l.ReversalOfID, &l.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan debit ledger: %w", err)
		}
		decorateDebitLedger(&l)
		result = append(result, l)
	}
	return result, nil
}

// ListDebitLedgerByBOE lists all ledger transactions for a BOE.
func (r *RoDTEPRepository) ListDebitLedgerByBOE(boeID string) ([]models.RoDTEPDebitLedger, error) {
	rows, err := r.db.Query(`
		SELECT id, transaction_ref, COALESCE(allocation_id, ''),
		       scrip_id, scrip_number, scrip_port_code,
		       boe_id, boe_number, boe_date, boe_port_code,
		       tx_type, amount_paise, previous_confirmed_balance_paise, remaining_confirmed_balance_paise,
		       COALESCE(icegate_transaction_ref, ''), COALESCE(challan_number, ''), debit_date,
		       operator_user_id, operator_name, COALESCE(supporting_doc_ref, ''),
		       COALESCE(remarks, ''), COALESCE(reversal_of_id, ''), created_at
		FROM rodtep_debit_ledger
		WHERE boe_id = ?
		ORDER BY debit_date ASC, created_at ASC`, boeID)
	if err != nil {
		return nil, fmt.Errorf("list debit ledger by boe: %w", err)
	}
	defer rows.Close()

	result := make([]models.RoDTEPDebitLedger, 0)
	for rows.Next() {
		var l models.RoDTEPDebitLedger
		err := rows.Scan(
			&l.ID, &l.TransactionRef, &l.AllocationID,
			&l.ScripID, &l.ScripNumber, &l.ScripPortCode,
			&l.BOEID, &l.BOENumber, &l.BOEDate, &l.BOEPortCode,
			&l.TxType, &l.AmountPaise, &l.PreviousConfirmedBalancePaise, &l.RemainingConfirmedBalancePaise,
			&l.ICEGATETransactionRef, &l.ChallanNumber, &l.DebitDate,
			&l.OperatorUserID, &l.OperatorName, &l.SupportingDocRef,
			&l.Remarks, &l.ReversalOfID, &l.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan debit ledger: %w", err)
		}
		decorateDebitLedger(&l)
		result = append(result, l)
	}
	return result, nil
}

// GetNetConfirmedDebitsByBOE calculates net confirmed debits (CONFIRMED - REVERSAL) for a BOE.
func (r *RoDTEPRepository) GetNetConfirmedDebitsByBOE(boeID string) (int64, error) {
	var netPaise int64
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM(
			CASE 
				WHEN tx_type = 'DEBIT_CONFIRMED' THEN amount_paise
				WHEN tx_type = 'DEBIT_REVERSAL' THEN -amount_paise
				ELSE 0
			END
		), 0)
		FROM rodtep_debit_ledger
		WHERE boe_id = ?`, boeID).Scan(&netPaise)
	return netPaise, err
}

// GetCumulativeReversalsForTx calculates cumulative reversals already posted for an original debit tx.
func (r *RoDTEPRepository) GetCumulativeReversalsForTx(originalTxID string) (int64, error) {
	var total int64
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM(amount_paise), 0)
		FROM rodtep_debit_ledger
		WHERE reversal_of_id = ? AND tx_type = 'DEBIT_REVERSAL'`, originalTxID).Scan(&total)
	return total, err
}

// -------------------------------------------------------------------------
// Reconciliation & Audit Operations
// -------------------------------------------------------------------------

// CreateReconciliation saves an ICEGATE monthly statement reconciliation record.
func (r *RoDTEPRepository) CreateReconciliation(in models.RoDTEPReconciliationInput, reconciledBy string) (models.RoDTEPReconciliation, error) {
	scrip, err := r.GetScripByID(in.ScripID)
	if err != nil {
		return models.RoDTEPReconciliation{}, err
	}

	discrepancy := in.ICEGATEStatementBalancePaise - scrip.ConfirmedBalancePaise
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")

	err = transactions.WithinTx(context.Background(), r.db, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO rodtep_reconciliations (
				id, scrip_id, reconciliation_date, statement_period_end,
				icegate_statement_balance_paise, erp_confirmed_balance_paise, erp_pending_allocated_paise,
				discrepancy_paise, discrepancy_reason, supporting_statement_path,
				reconciled_by, created_at
			) VALUES (
				?, ?, ?, ?,
				?, ?, ?,
				?, ?, ?,
				?, ?
			)`,
			id, in.ScripID, in.ReconciliationDate, in.StatementPeriodEnd,
			in.ICEGATEStatementBalancePaise, scrip.ConfirmedBalancePaise, scrip.PendingAllocatedPaise,
			discrepancy, nullStr(in.DiscrepancyReason), nullStr(in.SupportingStatementPath),
			reconciledBy, now,
		)
		if err != nil {
			return fmt.Errorf("insert reconciliation: %w", err)
		}

		auditID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO rodtep_audit_logs (
				id, event_type, scrip_id, amount_paise, reason, performed_by, performed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			auditID, models.AuditEventReconciliationEntry, in.ScripID, discrepancy,
			fmt.Sprintf("Reconciled against ICEGATE statement ending %s: discrepancy %d paise", in.StatementPeriodEnd, discrepancy),
			reconciledBy, now,
		)
		return err
	})

	if err != nil {
		return models.RoDTEPReconciliation{}, err
	}

	return r.GetReconciliationByID(id)
}

// GetReconciliationByID retrieves a single reconciliation record by ID.
func (r *RoDTEPRepository) GetReconciliationByID(id string) (models.RoDTEPReconciliation, error) {
	row := r.db.QueryRow(`
		SELECT r.id, r.scrip_id, s.scrip_number,
		       r.reconciliation_date, r.statement_period_end,
		       r.icegate_statement_balance_paise, r.erp_confirmed_balance_paise, r.erp_pending_allocated_paise,
		       r.discrepancy_paise, COALESCE(r.discrepancy_reason, ''),
		       COALESCE(r.supporting_statement_path, ''), r.reconciled_by, r.created_at
		FROM rodtep_reconciliations r
		JOIN rodtep_scrips s ON r.scrip_id = s.id
		WHERE r.id = ?`, id)

	var rec models.RoDTEPReconciliation
	err := row.Scan(
		&rec.ID, &rec.ScripID, &rec.ScripNumber,
		&rec.ReconciliationDate, &rec.StatementPeriodEnd,
		&rec.ICEGATEStatementBalancePaise, &rec.ERPConfirmedBalancePaise, &rec.ERPPendingAllocatedPaise,
		&rec.DiscrepancyPaise, &rec.DiscrepancyReason,
		&rec.SupportingStatementPath, &rec.ReconciledBy, &rec.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.RoDTEPReconciliation{}, fmt.Errorf("reconciliation %q not found", id)
	}
	if err != nil {
		return models.RoDTEPReconciliation{}, fmt.Errorf("get reconciliation: %w", err)
	}

	decorateReconciliation(&rec)
	return rec, nil
}

// ListReconciliations lists statement reconciliation records for a scrip.
func (r *RoDTEPRepository) ListReconciliations(scripID string) ([]models.RoDTEPReconciliation, error) {
	rows, err := r.db.Query(`
		SELECT r.id, r.scrip_id, s.scrip_number,
		       r.reconciliation_date, r.statement_period_end,
		       r.icegate_statement_balance_paise, r.erp_confirmed_balance_paise, r.erp_pending_allocated_paise,
		       r.discrepancy_paise, COALESCE(r.discrepancy_reason, ''),
		       COALESCE(r.supporting_statement_path, ''), r.reconciled_by, r.created_at
		FROM rodtep_reconciliations r
		JOIN rodtep_scrips s ON r.scrip_id = s.id
		WHERE r.scrip_id = ?
		ORDER BY r.statement_period_end DESC, r.created_at DESC`, scripID)
	if err != nil {
		return nil, fmt.Errorf("list reconciliations: %w", err)
	}
	defer rows.Close()

	result := make([]models.RoDTEPReconciliation, 0)
	for rows.Next() {
		var rec models.RoDTEPReconciliation
		err := rows.Scan(
			&rec.ID, &rec.ScripID, &rec.ScripNumber,
			&rec.ReconciliationDate, &rec.StatementPeriodEnd,
			&rec.ICEGATEStatementBalancePaise, &rec.ERPConfirmedBalancePaise, &rec.ERPPendingAllocatedPaise,
			&rec.DiscrepancyPaise, &rec.DiscrepancyReason,
			&rec.SupportingStatementPath, &rec.ReconciledBy, &rec.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan reconciliation: %w", err)
		}
		decorateReconciliation(&rec)
		result = append(result, rec)
	}
	return result, nil
}

// ListAuditLogs returns all audit trail entries for a scrip.
func (r *RoDTEPRepository) ListAuditLogs(scripID string) ([]models.RoDTEPAuditLog, error) {
	rows, err := r.db.Query(`
		SELECT id, event_type, scrip_id, COALESCE(allocation_id, ''),
		       COALESCE(ledger_id, ''), COALESCE(boe_id, ''),
		       COALESCE(amount_paise, 0), COALESCE(old_values_json, ''),
		       COALESCE(new_values_json, ''), reason, performed_by, performed_at
		FROM rodtep_audit_logs
		WHERE scrip_id = ?
		ORDER BY performed_at DESC, id DESC`, scripID)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	result := make([]models.RoDTEPAuditLog, 0)
	for rows.Next() {
		var a models.RoDTEPAuditLog
		err := rows.Scan(
			&a.ID, &a.EventType, &a.ScripID, &a.AllocationID,
			&a.LedgerID, &a.BOEID,
			&a.AmountPaise, &a.OldValuesJSON,
			&a.NewValuesJSON, &a.Reason, &a.PerformedBy, &a.PerformedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		result = append(result, a)
	}
	return result, nil
}

// -------------------------------------------------------------------------
// Aggregated Reports
// -------------------------------------------------------------------------

// GetPortSummary computes port-wise (INMAA1, INMAA4) and COMBINED portfolio health.
func (r *RoDTEPRepository) GetPortSummary() ([]models.RoDTEPPortSummary, error) {
	rows, err := r.db.Query(`
		SELECT scrip_port_code,
		       COUNT(*) AS total_scrips,
		       SUM(CASE WHEN status = 'ACTIVE' THEN 1 ELSE 0 END) AS active_scrips,
		       SUM(CASE WHEN status = 'EXHAUSTED' THEN 1 ELSE 0 END) AS exhausted_scrips,
		       SUM(CASE WHEN status = 'EXPIRED' THEN 1 ELSE 0 END) AS expired_scrips,
		       COALESCE(SUM(original_value_paise), 0) AS original_value,
		       COALESCE(SUM(confirmed_utilized_paise), 0) AS confirmed_utilized,
		       COALESCE(SUM(confirmed_balance_paise), 0) AS confirmed_balance,
		       COALESCE(SUM(pending_allocated_paise), 0) AS pending_allocated,
		       COALESCE(SUM(confirmed_balance_paise - pending_allocated_paise), 0) AS net_available
		FROM rodtep_scrips
		WHERE deleted_at IS NULL
		GROUP BY scrip_port_code
		ORDER BY scrip_port_code ASC`)
	if err != nil {
		return nil, fmt.Errorf("query port summary: %w", err)
	}
	defer rows.Close()

	portMap := map[string]models.RoDTEPPortSummary{
		models.PortINMAA1: {
			PortCode: models.PortINMAA1,
			PortName: "Chennai Sea (INMAA1)",
		},
		models.PortINMAA4: {
			PortCode: models.PortINMAA4,
			PortName: "Chennai Air Cargo (INMAA4)",
		},
	}
	var combined models.RoDTEPPortSummary
	combined.PortCode = "COMBINED"
	combined.PortName = "Combined (Pan-India RoDTEP Portfolio)"

	for rows.Next() {
		var s models.RoDTEPPortSummary
		err := rows.Scan(
			&s.PortCode,
			&s.TotalScripsCount,
			&s.ActiveScripsCount,
			&s.ExhaustedScripsCount,
			&s.ExpiredScripsCount,
			&s.OriginalValuePaise,
			&s.ConfirmedUtilizedPaise,
			&s.ConfirmedBalancePaise,
			&s.PendingAllocatedPaise,
			&s.NetAvailablePaise,
		)
		if err != nil {
			return nil, fmt.Errorf("scan port summary: %w", err)
		}
		if s.PortCode == models.PortINMAA1 {
			s.PortName = "Chennai Sea (INMAA1)"
		} else if s.PortCode == models.PortINMAA4 {
			s.PortName = "Chennai Air Cargo (INMAA4)"
		}

		s.OriginalValueINR = float64(s.OriginalValuePaise) / 100.0
		s.ConfirmedUtilizedINR = float64(s.ConfirmedUtilizedPaise) / 100.0
		s.ConfirmedBalanceINR = float64(s.ConfirmedBalancePaise) / 100.0
		s.PendingAllocatedINR = float64(s.PendingAllocatedPaise) / 100.0
		s.NetAvailableINR = float64(s.NetAvailablePaise) / 100.0

		portMap[s.PortCode] = s

		// Accumulate into combined
		combined.TotalScripsCount += s.TotalScripsCount
		combined.ActiveScripsCount += s.ActiveScripsCount
		combined.ExhaustedScripsCount += s.ExhaustedScripsCount
		combined.ExpiredScripsCount += s.ExpiredScripsCount
		combined.OriginalValuePaise += s.OriginalValuePaise
		combined.ConfirmedUtilizedPaise += s.ConfirmedUtilizedPaise
		combined.ConfirmedBalancePaise += s.ConfirmedBalancePaise
		combined.PendingAllocatedPaise += s.PendingAllocatedPaise
		combined.NetAvailablePaise += s.NetAvailablePaise
	}

	combined.OriginalValueINR = float64(combined.OriginalValuePaise) / 100.0
	combined.ConfirmedUtilizedINR = float64(combined.ConfirmedUtilizedPaise) / 100.0
	combined.ConfirmedBalanceINR = float64(combined.ConfirmedBalancePaise) / 100.0
	combined.PendingAllocatedINR = float64(combined.PendingAllocatedPaise) / 100.0
	combined.NetAvailableINR = float64(combined.NetAvailablePaise) / 100.0

	summaries := []models.RoDTEPPortSummary{
		combined,
		portMap[models.PortINMAA1],
		portMap[models.PortINMAA4],
	}
	return summaries, nil
}
