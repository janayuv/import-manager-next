package rodtep

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service provides Wails-bound APIs and business orchestration for RoDTEP scrip management,
// manual multi-scrip BOE allocations, two-stage confirmations, append-only ledger entries,
// reversals, and ICEGATE monthly reconciliations.
type Service struct {
	db    *sql.DB
	repo  *repositories.RoDTEPRepository
	sec   *security.Service
	audit *audit.Service
}

// NewService constructs a RoDTEP Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{
		db:    db,
		repo:  repositories.NewRoDTEPRepository(db),
		sec:   sec,
		audit: auditSvc,
	}
}

// SetDB rebinds the service and repository to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
	s.repo = repositories.NewRoDTEPRepository(db)
}

// getOperator resolves current operator credentials from security session.
func (s *Service) getOperator(perm string) (userID, username string, err error) {
	if s.sec == nil {
		return "SYSTEM", "Administrator", nil
	}
	sess, err := s.sec.RequirePermission(perm)
	if err != nil {
		return "", "", err
	}
	uName := sess.Username
	if uName == "" {
		uName = sess.UserID
	}
	return sess.UserID, uName, nil
}

// -------------------------------------------------------------------------
// Scrip Master Management
// -------------------------------------------------------------------------

// CreateScrip validates and inserts a new RoDTEP scrip.
func (s *Service) CreateScrip(input models.RoDTEPScripInput) (models.RoDTEPScrip, error) {
	uID, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return models.RoDTEPScrip{}, err
	}

	scrip, err := s.repo.CreateScrip(input, uName)
	if err != nil {
		return models.RoDTEPScrip{}, err
	}

	if s.audit != nil {
		_ = s.audit.RecordTableAudit("rodtep_scrips", scrip.ID, "INSERT", uID, nil, scrip, nil)
		_ = s.audit.RecordUserActivity(uID, "rodtep_scrip.create", "rodtep_scrips", scrip.ID, "SUCCESS", "INFO", map[string]any{
			"scripNumber":   scrip.ScripNumber,
			"scripPortCode": scrip.ScripPortCode,
			"originalValue": scrip.OriginalValueINR,
			"confirmedBal":  scrip.ConfirmedBalINR,
		})
	}
	return scrip, nil
}

// UpdateScrip updates non-financial metadata of a RoDTEP scrip.
func (s *Service) UpdateScrip(id string, input models.RoDTEPScripInput) (models.RoDTEPScrip, error) {
	uID, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return models.RoDTEPScrip{}, err
	}

	scrip, err := s.repo.UpdateScrip(id, input, uName)
	if err != nil {
		return models.RoDTEPScrip{}, err
	}

	if s.audit != nil {
		_ = s.audit.RecordTableAudit("rodtep_scrips", scrip.ID, "UPDATE", uID, nil, scrip, nil)
	}
	return scrip, nil
}

// GetScrip retrieves a scrip by ID.
func (s *Service) GetScrip(id string) (models.RoDTEPScrip, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
			return models.RoDTEPScrip{}, err
		}
	}
	return s.repo.GetScripByID(id)
}

// ListScrips returns scrips matching port, status, and search filters.
func (s *Service) ListScrips(portCode, status, search string) ([]models.RoDTEPScrip, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
			return nil, err
		}
	}
	return s.repo.ListScrips(portCode, status, search)
}

// DeleteScrip soft-deletes an unused scrip.
func (s *Service) DeleteScrip(id string) error {
	uID, uName, err := s.getOperator(security.PermDataDelete)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDeleteScrip(id, uName); err != nil {
		return err
	}
	if s.audit != nil {
		_ = s.audit.RecordTableAudit("rodtep_scrips", id, "DELETE", uID, nil, nil, map[string]any{"softDelete": true})
	}
	return nil
}

// VerifyOpeningBalance records formal opening balance verification for legacy scrips.
func (s *Service) VerifyOpeningBalance(id string, evidenceRef string) error {
	uID, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return err
	}
	if strings.TrimSpace(evidenceRef) == "" {
		return errors.New("supporting evidence reference is required for opening verification")
	}
	if err := s.repo.VerifyOpeningBalance(id, uName, evidenceRef); err != nil {
		return err
	}
	if s.audit != nil {
		_ = s.audit.RecordUserActivity(uID, "rodtep_scrip.verify_opening", "rodtep_scrips", id, "SUCCESS", "INFO", map[string]any{
			"evidenceRef": evidenceRef,
		})
	}
	return nil
}

// -------------------------------------------------------------------------
// BOE Evaluation Context & Duty Liability Integration
// -------------------------------------------------------------------------

// GetBOEEvalContext loads live BOE details, duty calculation results, net confirmed debits,
// active reservations, and eligible available scrips for the allocation drawer.
func (s *Service) GetBOEEvalContext(boeID string) (models.BOEEvalContext, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
			return models.BOEEvalContext{}, err
		}
	}

	cleanBOEID := strings.TrimSpace(boeID)
	if cleanBOEID == "" {
		return models.BOEEvalContext{}, errors.New("boeId is required")
	}

	// 1. Fetch BOE Detail
	var bd models.BOEDetail
	var payDate, challan, refID, txID sql.NullString
	err := s.db.QueryRow(`
		SELECT id, be_number, be_date, location, total_assessment_value, duty_amount,
		       payment_date, duty_paid, challan_number, ref_id, transaction_id
		FROM boe_details
		WHERE id = ? AND deleted_at IS NULL`, cleanBOEID).Scan(
		&bd.ID, &bd.BENumber, &bd.BEDate, &bd.Location, &bd.TotalAssessmentValue, &bd.DutyAmount,
		&payDate, &bd.DutyPaid, &challan, &refID, &txID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.BOEEvalContext{}, fmt.Errorf("Bill of Entry %q not found", cleanBOEID)
	}
	if err != nil {
		return models.BOEEvalContext{}, fmt.Errorf("query boe detail: %w", err)
	}
	if payDate.Valid {
		bd.PaymentDate = payDate.String
	}
	if challan.Valid {
		bd.ChallanNumber = challan.String
	}
	if refID.Valid {
		bd.RefID = refID.String
	}
	if txID.Valid {
		bd.TransactionID = txID.String
	}

	// 2. Fetch associated calculation result from boe_calculations if available
	var calcResultJSON string
	var calcResult models.BOECalculationResult
	var hasCalc bool
	err = s.db.QueryRow(`
		SELECT calculation_result_json
		FROM boe_calculations
		WHERE boe_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, cleanBOEID).Scan(&calcResultJSON)
	if err == nil && strings.TrimSpace(calcResultJSON) != "" {
		if err := json.Unmarshal([]byte(calcResultJSON), &calcResult); err == nil {
			hasCalc = true
		}
	}

	// 3. Determine Assessed BCD Liability
	var assessedBCD float64
	if hasCalc && calcResult.GrossBCDTotal > 0 {
		assessedBCD = calcResult.GrossBCDTotal
	} else {
		// Fallback if detailed line calculation not yet saved: duty_amount
		assessedBCD = bd.DutyAmount
	}
	assessedBCDPaise := int64(math.Round(assessedBCD * 100))

	// 4. Query Prior Net Confirmed RoDTEP Debits & Active Reservations
	netConfirmedPaise, err := s.repo.GetNetConfirmedDebitsByBOE(cleanBOEID)
	if err != nil {
		return models.BOEEvalContext{}, fmt.Errorf("calculate net confirmed debits: %w", err)
	}

	activeReservedPaise, err := s.repo.GetActiveAllocationsTotalByBOE(cleanBOEID)
	if err != nil {
		return models.BOEEvalContext{}, fmt.Errorf("calculate active reserved debits: %w", err)
	}

	// 5. Evaluate Cash BCD Settled
	// If duty_paid is present:
	// In boe_details, duty_paid represents the entire cash challan amount.
	// If duty_paid >= CashCustomsDutyTotal (when CashCustomsDutyTotal > 0), the cash challan has been paid in full.
	var cashBCDPaidPaise int64
	if hasCalc && calcResult.CashCustomsDutyTotal > 0 && bd.DutyPaid >= calcResult.CashCustomsDutyTotal {
		// Cash challan settled completely -> BCD cash portion also settled
		cashBCDPaidPaise = int64(math.Round(calcResult.BCDCashTotal * 100))
	} else if bd.DutyPaid > 0 && !hasCalc {
		cashBCDPaidPaise = int64(math.Round(bd.DutyPaid * 100))
	}

	// 6. Eligible Unpaid BCD Liability
	eligibleUnpaidPaise := assessedBCDPaise - netConfirmedPaise - activeReservedPaise - cashBCDPaidPaise
	if eligibleUnpaidPaise < 0 {
		eligibleUnpaidPaise = 0
	}

	// 7. Load Active Scrips with Available Balances
	scrips, err := s.repo.ListScrips("ALL", models.ScripStatusActive, "")
	if err != nil {
		return models.BOEEvalContext{}, fmt.Errorf("list available scrips: %w", err)
	}
	availableScrips := make([]models.RoDTEPScrip, 0)
	for _, sc := range scrips {
		if sc.NetAvailablePaise > 0 {
			availableScrips = append(availableScrips, sc)
		}
	}

	// 8. Load Active Allocations for this BOE
	allocs, err := s.repo.ListAllocationsByBOE(cleanBOEID)
	if err != nil {
		return models.BOEEvalContext{}, fmt.Errorf("list allocations for boe: %w", err)
	}
	if allocs == nil {
		allocs = make([]models.RoDTEPAllocation, 0)
	}

	ctxRes := models.BOEEvalContext{
		BOEDetail:              bd,
		AssessedBCDTotalPaise:  assessedBCDPaise,
		NetConfirmedRoDTEP:     netConfirmedPaise,
		ActiveReservedRoDTEP:   activeReservedPaise,
		CashBCDPaidPaise:       cashBCDPaidPaise,
		EligibleUnpaidBCDPaise: eligibleUnpaidPaise,
		AssessedBCDINR:         float64(assessedBCDPaise) / 100.0,
		EligibleUnpaidBCDINR:   float64(eligibleUnpaidPaise) / 100.0,
		AvailableScrips:        availableScrips,
		ActiveAllocations:      allocs,
	}
	if hasCalc {
		ctxRes.CalculationResult = &calcResult
	}
	return ctxRes, nil
}

// -------------------------------------------------------------------------
// Stage 1: Manual Allocation (Intent & Balance Reservation - NO FIFO)
// -------------------------------------------------------------------------

// AllocateScrips reserves RoDTEP balances from explicitly selected scrips against a BOE.
// STRICT BUSINESS REQUIREMENT: Strictly manual selection and amounts entered by operator; NO FIFO.
func (s *Service) AllocateScrips(boeID string, items []models.RoDTEPAllocationItemInput, remarks string) ([]models.RoDTEPAllocation, error) {
	uID, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return nil, err
	}

	cleanBOEID := strings.TrimSpace(boeID)
	if cleanBOEID == "" {
		return nil, errors.New("boeId is required")
	}
	if len(items) == 0 {
		return nil, errors.New("at least one scrip allocation item is required")
	}

	// Calculate total requested allocation
	var totalRequestedPaise int64
	for i, item := range items {
		if strings.TrimSpace(item.ScripID) == "" {
			return nil, fmt.Errorf("item %d: scripId is required", i+1)
		}
		if item.AmountPaise <= 0 {
			return nil, fmt.Errorf("item %d: allocation amount must be greater than zero", i+1)
		}
		totalRequestedPaise += item.AmountPaise
	}

	// 1. Validate against live unpaid BCD liability before beginning transaction to avoid connection lock
	evalCtx, err := s.GetBOEEvalContext(cleanBOEID)
	if err != nil {
		return nil, fmt.Errorf("evaluate boe duty context: %w", err)
	}
	if totalRequestedPaise > evalCtx.EligibleUnpaidBCDPaise {
		return nil, fmt.Errorf("total requested allocation (₹%.2f) exceeds eligible unpaid BCD liability (₹%.2f)",
			float64(totalRequestedPaise)/100.0, float64(evalCtx.EligibleUnpaidBCDPaise)/100.0)
	}

	var createdAllocations []models.RoDTEPAllocation

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		// 1. Fetch BOE Detail inside transaction
		var beNumber, beDate, beLocation string
		err := tx.QueryRow(`
			SELECT be_number, be_date, location
			FROM boe_details
			WHERE id = ? AND deleted_at IS NULL`, cleanBOEID).Scan(&beNumber, &beDate, &beLocation)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("BOE %q not found or deleted", cleanBOEID)
		}
		if err != nil {
			return fmt.Errorf("read boe: %w", err)
		}

		now := time.Now().Format("2006-01-02 15:04:05")
		todayDate := time.Now().Format("2006-01-02")

		for _, item := range items {
			// Query scrip within transaction with lock
			var scripNumber, scripPortCode, status, expiryDate string
			var confirmedBal, pendingAlloc int64
			err := tx.QueryRow(`
				SELECT scrip_number, scrip_port_code, status, expiry_date,
				       confirmed_balance_paise, pending_allocated_paise
				FROM rodtep_scrips
				WHERE id = ? AND deleted_at IS NULL`, item.ScripID).Scan(
				&scripNumber, &scripPortCode, &status, &expiryDate,
				&confirmedBal, &pendingAlloc,
			)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("scrip %q not found", item.ScripID)
			}
			if err != nil {
				return fmt.Errorf("read scrip %q: %w", item.ScripID, err)
			}

			if status != models.ScripStatusActive {
				return fmt.Errorf("scrip %s is %s; only ACTIVE scrips can be allocated", scripNumber, status)
			}
			if expiryDate < todayDate {
				return fmt.Errorf("scrip %s expired on %s", scripNumber, expiryDate)
			}

			availablePaise := confirmedBal - pendingAlloc
			if item.AmountPaise > availablePaise {
				return fmt.Errorf("requested allocation (₹%.2f) exceeds available balance (₹%.2f) on scrip %s",
					float64(item.AmountPaise)/100.0, float64(availablePaise)/100.0, scripNumber)
			}

			// Update scrip pending reservation
			_, err = tx.Exec(`
				UPDATE rodtep_scrips
				SET pending_allocated_paise = pending_allocated_paise + ?,
				    updated_at = ?
				WHERE id = ?`,
				item.AmountPaise, now, item.ScripID,
			)
			if err != nil {
				return fmt.Errorf("reserve scrip balance: %w", err)
			}

			// Generate allocation record
			allocID := uuid.New().String()
			allocRef := fmt.Sprintf("ALC-%s-%s", time.Now().Format("20060102"), strings.ToUpper(uuid.NewString()[:6]))
			allocRemarks := item.Remarks
			if allocRemarks == "" {
				allocRemarks = remarks
			}

			alloc := models.RoDTEPAllocation{
				ID:                   allocID,
				AllocationRef:        allocRef,
				ScripID:              item.ScripID,
				ScripNumber:          scripNumber,
				ScripPortCode:        scripPortCode,
				BOEID:                cleanBOEID,
				BOENumber:            beNumber,
				BOEDate:              beDate,
				BOEPortCode:          beLocation,
				AllocatedAmountPaise: item.AmountPaise,
				ConfirmedAmountPaise: 0,
				ReleasedAmountPaise:  0,
				OutstandingPaise:     item.AmountPaise,
				Status:               models.AllocStatusAllocated,
				OperatorUserID:       uID,
				OperatorName:         uName,
				Remarks:              allocRemarks,
				CreatedAt:            now,
				UpdatedAt:            now,
				AllocatedINR:         float64(item.AmountPaise) / 100.0,
				OutstandingINR:       float64(item.AmountPaise) / 100.0,
			}

			if err := s.repo.CreateAllocation(tx, alloc); err != nil {
				return err
			}

			// Audit entry
			auditID := uuid.New().String()
			_, err = tx.Exec(`
				INSERT INTO rodtep_audit_logs (
					id, event_type, scrip_id, allocation_id, boe_id, amount_paise, reason, performed_by, performed_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				auditID, models.AuditEventAllocationReserved, item.ScripID, allocID, cleanBOEID,
				item.AmountPaise,
				fmt.Sprintf("Reserved ₹%.2f on scrip %s for BOE %s", float64(item.AmountPaise)/100.0, scripNumber, beNumber),
				uName, now,
			)
			if err != nil {
				return fmt.Errorf("record allocation audit: %w", err)
			}

			createdAllocations = append(createdAllocations, alloc)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity(uID, "rodtep_allocation.create", "rodtep_allocations", cleanBOEID, "SUCCESS", "INFO", map[string]any{
			"boeId":       cleanBOEID,
			"totalPaise":  totalRequestedPaise,
			"scripCount":  len(createdAllocations),
		})
	}
	return createdAllocations, nil
}

// -------------------------------------------------------------------------
// Stage 2: Actual ICEGATE Debit Confirmation
// -------------------------------------------------------------------------

// ConfirmActualDebit records the actual customs challan/ICEGATE debit confirmation,
// debiting confirmed scrip balances and appending to rodtep_debit_ledger.
func (s *Service) ConfirmActualDebit(input models.RoDTEPAllocationConfirmInput) (models.RoDTEPDebitLedger, error) {
	uID, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return models.RoDTEPDebitLedger{}, err
	}

	allocID := strings.TrimSpace(input.AllocationID)
	if allocID == "" {
		return models.RoDTEPDebitLedger{}, errors.New("allocationId is required")
	}
	if input.AmountPaise <= 0 {
		return models.RoDTEPDebitLedger{}, errors.New("confirmed amount must be greater than zero")
	}
	if len(input.DebitDate) != 10 {
		return models.RoDTEPDebitLedger{}, errors.New("debit date must be in YYYY-MM-DD format")
	}

	icegateRef := strings.TrimSpace(input.ICEGATETransactionRef)
	docRef := strings.TrimSpace(input.SupportingDocRef)
	if icegateRef == "" && docRef == "" {
		return models.RoDTEPDebitLedger{}, errors.New("either ICEGATE transaction reference or supporting document reference is required")
	}

	var createdLedger models.RoDTEPDebitLedger

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		// 1. Fetch Allocation
		var scripID, boeID, boeNumber, boeDate, boePortCode, allocStatus string
		var allocAmount, confirmedAmount, releasedAmount int64
		err := tx.QueryRow(`
			SELECT scrip_id, boe_id, boe_number, boe_date, boe_port_code, status,
			       allocated_amount_paise, confirmed_amount_paise, released_amount_paise
			FROM rodtep_allocations
			WHERE id = ?`, allocID).Scan(
			&scripID, &boeID, &boeNumber, &boeDate, &boePortCode, &allocStatus,
			&allocAmount, &confirmedAmount, &releasedAmount,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("allocation %q not found", allocID)
		}
		if err != nil {
			return fmt.Errorf("read allocation: %w", err)
		}

		if allocStatus != models.AllocStatusAllocated && allocStatus != models.AllocStatusPartiallyConfirmed {
			return fmt.Errorf("cannot confirm debit against allocation with status %s", allocStatus)
		}

		outstandingPaise := allocAmount - confirmedAmount - releasedAmount
		if input.AmountPaise > outstandingPaise {
			return fmt.Errorf("confirmed amount (₹%.2f) exceeds outstanding allocated balance (₹%.2f)",
				float64(input.AmountPaise)/100.0, float64(outstandingPaise)/100.0)
		}

		// 2. Fetch Scrip details inside transaction
		var scripNumber, scripPortCode string
		var confirmedBal, confirmedUtil, reversalRecredit, pendingAlloc int64
		err = tx.QueryRow(`
			SELECT scrip_number, scrip_port_code,
			       confirmed_balance_paise, confirmed_utilized_paise, reversal_recredit_paise, pending_allocated_paise
			FROM rodtep_scrips
			WHERE id = ? AND deleted_at IS NULL`, scripID).Scan(
			&scripNumber, &scripPortCode,
			&confirmedBal, &confirmedUtil, &reversalRecredit, &pendingAlloc,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("scrip %q not found", scripID)
		}
		if err != nil {
			return fmt.Errorf("read scrip: %w", err)
		}

		if input.AmountPaise > confirmedBal {
			return fmt.Errorf("confirmed amount (₹%.2f) exceeds scrip confirmed balance (₹%.2f)",
				float64(input.AmountPaise)/100.0, float64(confirmedBal)/100.0)
		}

		newConfirmedBal := confirmedBal - input.AmountPaise
		newPendingAlloc := pendingAlloc - input.AmountPaise
		if newPendingAlloc < 0 {
			newPendingAlloc = 0
		}
		newConfirmedUtil := confirmedUtil + input.AmountPaise

		scripStatus := models.ScripStatusActive
		if newConfirmedBal == 0 {
			scripStatus = models.ScripStatusExhausted
		}

		now := time.Now().Format("2006-01-02 15:04:05")

		// 3. Update Scrip Balances
		_, err = tx.Exec(`
			UPDATE rodtep_scrips
			SET confirmed_utilized_paise = ?,
			    confirmed_balance_paise = ?,
			    pending_allocated_paise = ?,
			    status = ?,
			    updated_at = ?
			WHERE id = ?`,
			newConfirmedUtil, newConfirmedBal, newPendingAlloc, scripStatus, now, scripID,
		)
		if err != nil {
			return fmt.Errorf("update scrip balance: %w", err)
		}

		// 4. Update Allocation Status
		newConfirmedAlloc := confirmedAmount + input.AmountPaise
		var newAllocStatus string
		if newConfirmedAlloc+releasedAmount >= allocAmount {
			newAllocStatus = models.AllocStatusConfirmed
		} else {
			newAllocStatus = models.AllocStatusPartiallyConfirmed
		}

		_, err = tx.Exec(`
			UPDATE rodtep_allocations
			SET confirmed_amount_paise = ?,
			    status = ?,
			    updated_at = ?
			WHERE id = ?`,
			newConfirmedAlloc, newAllocStatus, now, allocID,
		)
		if err != nil {
			return fmt.Errorf("update allocation: %w", err)
		}

		// 5. Append to Debit Ledger
		ledgerID := uuid.New().String()
		txRef := fmt.Sprintf("TX-%s-%s", time.Now().Format("20060102"), strings.ToUpper(uuid.NewString()[:6]))

		createdLedger = models.RoDTEPDebitLedger{
			ID:                             ledgerID,
			TransactionRef:                 txRef,
			AllocationID:                   allocID,
			ScripID:                        scripID,
			ScripNumber:                    scripNumber,
			ScripPortCode:                  scripPortCode,
			BOEID:                          boeID,
			BOENumber:                      boeNumber,
			BOEDate:                        boeDate,
			BOEPortCode:                    boePortCode,
			TxType:                         models.TxTypeDebitConfirmed,
			AmountPaise:                    input.AmountPaise,
			PreviousConfirmedBalancePaise:  confirmedBal,
			RemainingConfirmedBalancePaise: newConfirmedBal,
			ICEGATETransactionRef:          icegateRef,
			ChallanNumber:                  input.ChallanNumber,
			DebitDate:                      input.DebitDate,
			OperatorUserID:                 uID,
			OperatorName:                   uName,
			SupportingDocRef:               docRef,
			Remarks:                        input.Remarks,
			CreatedAt:                      now,
			AmountINR:                      float64(input.AmountPaise) / 100.0,
			RemainingINR:                   float64(newConfirmedBal) / 100.0,
		}

		if err := s.repo.CreateDebitLedgerEntry(tx, createdLedger); err != nil {
			return fmt.Errorf("insert debit ledger: %w", err)
		}

		// 6. Audit Trail Entry
		auditID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO rodtep_audit_logs (
				id, event_type, scrip_id, allocation_id, ledger_id, boe_id, amount_paise, reason, performed_by, performed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			auditID, models.AuditEventDebitConfirmed, scripID, allocID, ledgerID, boeID,
			input.AmountPaise,
			fmt.Sprintf("Confirmed debit of ₹%.2f on scrip %s for BOE %s (ICEGATE Ref: %s)",
				float64(input.AmountPaise)/100.0, scripNumber, boeNumber, icegateRef),
			uName, now,
		)
		if err != nil {
			return fmt.Errorf("record debit audit: %w", err)
		}
		return nil
	})

	if err != nil {
		return models.RoDTEPDebitLedger{}, err
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity(uID, "rodtep_debit.confirm", "rodtep_debit_ledger", createdLedger.ID, "SUCCESS", "INFO", map[string]any{
			"scripNumber": createdLedger.ScripNumber,
			"boeNumber":   createdLedger.BOENumber,
			"amountInr":   createdLedger.AmountINR,
			"icegateRef":  input.ICEGATETransactionRef,
		})
	}
	return createdLedger, nil
}

// -------------------------------------------------------------------------
// Allocation Closeout / Release
// -------------------------------------------------------------------------

// CloseAllocation releases unconfirmed reserved balance back to the scrip.
func (s *Service) CloseAllocation(allocationID string, reason string) error {
	uID, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return err
	}

	cleanAllocID := strings.TrimSpace(allocationID)
	if cleanAllocID == "" {
		return errors.New("allocationId is required")
	}

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var scripID, boeID, boeNumber, status string
		var allocatedPaise, confirmedPaise, releasedPaise int64
		err := tx.QueryRow(`
			SELECT scrip_id, boe_id, boe_number, status,
			       allocated_amount_paise, confirmed_amount_paise, released_amount_paise
			FROM rodtep_allocations
			WHERE id = ?`, cleanAllocID).Scan(
			&scripID, &boeID, &boeNumber, &status,
			&allocatedPaise, &confirmedPaise, &releasedPaise,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("allocation %q not found", cleanAllocID)
		}
		if err != nil {
			return fmt.Errorf("read allocation: %w", err)
		}

		if status != models.AllocStatusAllocated && status != models.AllocStatusPartiallyConfirmed {
			return fmt.Errorf("allocation %s cannot be closed (current status: %s)", cleanAllocID, status)
		}

		remainder := allocatedPaise - confirmedPaise - releasedPaise
		if remainder <= 0 {
			return errors.New("no unconfirmed balance left to release")
		}

		now := time.Now().Format("2006-01-02 15:04:05")

		// Release scrip pending reservation
		_, err = tx.Exec(`
			UPDATE rodtep_scrips
			SET pending_allocated_paise = pending_allocated_paise - ?,
			    updated_at = ?
			WHERE id = ?`,
			remainder, now, scripID,
		)
		if err != nil {
			return fmt.Errorf("release scrip reservation: %w", err)
		}

		// Update allocation status
		finalStatus := models.AllocStatusClosed
		if confirmedPaise == 0 {
			finalStatus = models.AllocStatusCancelled
		}

		_, err = tx.Exec(`
			UPDATE rodtep_allocations
			SET released_amount_paise = released_amount_paise + ?,
			    status = ?,
			    updated_at = ?
			WHERE id = ?`,
			remainder, finalStatus, now, cleanAllocID,
		)
		if err != nil {
			return fmt.Errorf("update allocation status: %w", err)
		}

		// Audit log
		auditID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO rodtep_audit_logs (
				id, event_type, scrip_id, allocation_id, boe_id, amount_paise, reason, performed_by, performed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			auditID, models.AuditEventAllocationReleased, scripID, cleanAllocID, boeID,
			remainder,
			fmt.Sprintf("Released unconfirmed balance of ₹%.2f on allocation %s for BOE %s. Reason: %s",
				float64(remainder)/100.0, cleanAllocID, boeNumber, reason),
			uName, now,
		)
		return err
	})

	if err != nil {
		return err
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity(uID, "rodtep_allocation.close", "rodtep_allocations", cleanAllocID, "SUCCESS", "INFO", map[string]any{
			"allocationId": cleanAllocID,
			"reason":       reason,
		})
	}
	return nil
}

// -------------------------------------------------------------------------
// Stage 2 Reversal: Append-Only Statutory Re-Credits
// -------------------------------------------------------------------------

// ReverseDebit records an append-only reversal transaction re-crediting the scrip balance.
func (s *Service) ReverseDebit(input models.RoDTEPReversalInput) (models.RoDTEPDebitLedger, error) {
	uID, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return models.RoDTEPDebitLedger{}, err
	}

	origTxID := strings.TrimSpace(input.DebitLedgerID)
	if origTxID == "" {
		return models.RoDTEPDebitLedger{}, errors.New("debitLedgerID is required")
	}
	if input.AmountPaise <= 0 {
		return models.RoDTEPDebitLedger{}, errors.New("reversal amount must be greater than zero")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return models.RoDTEPDebitLedger{}, errors.New("reversal reason is required")
	}
	revDate := strings.TrimSpace(input.ReversalDate)
	if len(revDate) != 10 {
		revDate = time.Now().Format("2006-01-02")
	}

	var createdReversal models.RoDTEPDebitLedger

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		// 1. Fetch original debit transaction
		var scripID, scripNumber, scripPortCode, boeID, boeNumber, boeDate, boePortCode, txType string
		var origAmount int64
		err := tx.QueryRow(`
			SELECT scrip_id, scrip_number, scrip_port_code,
			       boe_id, boe_number, boe_date, boe_port_code,
			       tx_type, amount_paise
			FROM rodtep_debit_ledger
			WHERE id = ?`, origTxID).Scan(
			&scripID, &scripNumber, &scripPortCode,
			&boeID, &boeNumber, &boeDate, &boePortCode,
			&txType, &origAmount,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("debit ledger record %q not found", origTxID)
		}
		if err != nil {
			return fmt.Errorf("read debit ledger: %w", err)
		}

		if txType != models.TxTypeDebitConfirmed {
			return fmt.Errorf("cannot reverse a transaction of type %s; only DEBIT_CONFIRMED can be reversed", txType)
		}

		// 2. Check cumulative prior reversals for this debit
		var priorReversals int64
		err = tx.QueryRow(`
			SELECT COALESCE(SUM(amount_paise), 0)
			FROM rodtep_debit_ledger
			WHERE reversal_of_id = ? AND tx_type = 'DEBIT_REVERSAL'`, origTxID).Scan(&priorReversals)
		if err != nil {
			return fmt.Errorf("calculate prior reversals: %w", err)
		}

		maxReversible := origAmount - priorReversals
		if input.AmountPaise > maxReversible {
			return fmt.Errorf("reversal amount (₹%.2f) exceeds maximum reversible balance (₹%.2f)",
				float64(input.AmountPaise)/100.0, float64(maxReversible)/100.0)
		}

		// 3. Fetch scrip confirmed balance
		var confirmedBal, reversalRecred int64
		var expiryDate string
		err = tx.QueryRow(`
			SELECT confirmed_balance_paise, reversal_recredit_paise, expiry_date
			FROM rodtep_scrips
			WHERE id = ?`, scripID).Scan(&confirmedBal, &reversalRecred, &expiryDate)
		if err != nil {
			return fmt.Errorf("read scrip balance: %w", err)
		}

		newConfirmedBal := confirmedBal + input.AmountPaise
		newReversalRecred := reversalRecred + input.AmountPaise

		now := time.Now().Format("2006-01-02 15:04:05")
		todayDate := time.Now().Format("2006-01-02")

		newStatus := models.ScripStatusActive
		if expiryDate < todayDate {
			newStatus = models.ScripStatusExpired
		}

		// Update scrip
		_, err = tx.Exec(`
			UPDATE rodtep_scrips
			SET reversal_recredit_paise = ?,
			    confirmed_balance_paise = ?,
			    status = ?,
			    updated_at = ?
			WHERE id = ?`,
			newReversalRecred, newConfirmedBal, newStatus, now, scripID,
		)
		if err != nil {
			return fmt.Errorf("update scrip for reversal: %w", err)
		}

		// 4. Create append-only reversal entry in debit ledger
		revID := uuid.New().String()
		txRef := fmt.Sprintf("REV-%s-%s", time.Now().Format("20060102"), strings.ToUpper(uuid.NewString()[:6]))

		createdReversal = models.RoDTEPDebitLedger{
			ID:                             revID,
			TransactionRef:                 txRef,
			ScripID:                        scripID,
			ScripNumber:                    scripNumber,
			ScripPortCode:                  scripPortCode,
			BOEID:                          boeID,
			BOENumber:                      boeNumber,
			BOEDate:                        boeDate,
			BOEPortCode:                    boePortCode,
			TxType:                         models.TxTypeDebitReversal,
			AmountPaise:                    input.AmountPaise,
			PreviousConfirmedBalancePaise:  confirmedBal,
			RemainingConfirmedBalancePaise: newConfirmedBal,
			DebitDate:                      revDate,
			OperatorUserID:                 uID,
			OperatorName:                   uName,
			SupportingDocRef:               input.DocRef,
			Remarks:                        input.Reason,
			ReversalOfID:                   origTxID,
			CreatedAt:                      now,
			AmountINR:                      float64(input.AmountPaise) / 100.0,
			RemainingINR:                   float64(newConfirmedBal) / 100.0,
		}

		if err := s.repo.CreateDebitLedgerEntry(tx, createdReversal); err != nil {
			return fmt.Errorf("insert reversal ledger entry: %w", err)
		}

		// 5. Audit record
		auditID := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO rodtep_audit_logs (
				id, event_type, scrip_id, ledger_id, boe_id, amount_paise, reason, performed_by, performed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			auditID, models.AuditEventDebitReversed, scripID, revID, boeID,
			input.AmountPaise,
			fmt.Sprintf("Reversed ₹%.2f on scrip %s for BOE %s. Reason: %s",
				float64(input.AmountPaise)/100.0, scripNumber, boeNumber, input.Reason),
			uName, now,
		)
		return err
	})

	if err != nil {
		return models.RoDTEPDebitLedger{}, err
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity(uID, "rodtep_debit.reverse", "rodtep_debit_ledger", createdReversal.ID, "SUCCESS", "WARNING", map[string]any{
			"originalTxId": origTxID,
			"amountInr":    createdReversal.AmountINR,
			"reason":       input.Reason,
		})
	}
	return createdReversal, nil
}

// -------------------------------------------------------------------------
// Reconciliation & Port Reports
// -------------------------------------------------------------------------

// SaveReconciliation records an ICEGATE monthly statement balance check.
func (s *Service) SaveReconciliation(input models.RoDTEPReconciliationInput) (models.RoDTEPReconciliation, error) {
	uID, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return models.RoDTEPReconciliation{}, err
	}

	if strings.TrimSpace(input.ScripID) == "" {
		return models.RoDTEPReconciliation{}, errors.New("scripId is required")
	}
	if len(input.ReconciliationDate) != 10 || len(input.StatementPeriodEnd) != 10 {
		return models.RoDTEPReconciliation{}, errors.New("reconciliationDate and statementPeriodEnd must be YYYY-MM-DD")
	}

	rec, err := s.repo.CreateReconciliation(input, uName)
	if err != nil {
		return models.RoDTEPReconciliation{}, err
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity(uID, "rodtep_recon.save", "rodtep_reconciliations", rec.ID, "SUCCESS", "INFO", map[string]any{
			"scripId":         rec.ScripID,
			"discrepancyInr":  rec.DiscrepancyINR,
			"periodEnd":       rec.StatementPeriodEnd,
		})
	}
	return rec, nil
}

// ListReconciliations lists reconciliations for a scrip.
func (s *Service) ListReconciliations(scripID string) ([]models.RoDTEPReconciliation, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
			return nil, err
		}
	}
	return s.repo.ListReconciliations(scripID)
}

// ListAuditLogs lists audit history for a scrip.
func (s *Service) ListAuditLogs(scripID string) ([]models.RoDTEPAuditLog, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
			return nil, err
		}
	}
	return s.repo.ListAuditLogs(scripID)
}

// GetPortSummary returns port-wise (INMAA1, INMAA4) and combined balance statistics.
func (s *Service) GetPortSummary() ([]models.RoDTEPPortSummary, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
			return nil, err
		}
	}
	return s.repo.GetPortSummary()
}

// GetBOEDebitReport generates a comprehensive ledger report summarizing scrip debits across BOEs.
func (s *Service) GetBOEDebitReport() ([]models.RoDTEPBOEDebitSummary, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
			return nil, err
		}
	}

	// Query distinct BOEs that have allocations or confirmed debits
	rows, err := s.db.Query(`
		SELECT DISTINCT boe_id
		FROM (
			SELECT boe_id FROM rodtep_allocations
			UNION
			SELECT boe_id FROM rodtep_debit_ledger
		)
		WHERE boe_id IS NOT NULL AND boe_id != ''`)
	if err != nil {
		return nil, fmt.Errorf("query distinct boes with rodtep: %w", err)
	}
	defer rows.Close()

	var boeIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			boeIDs = append(boeIDs, id)
		}
	}

	summaries := make([]models.RoDTEPBOEDebitSummary, 0)
	for _, bID := range boeIDs {
		eval, err := s.GetBOEEvalContext(bID)
		if err != nil {
			continue
		}

		debits, err := s.repo.ListDebitLedgerByBOE(bID)
		if err != nil {
			continue
		}

		sum := models.RoDTEPBOEDebitSummary{
			BOEID:                 bID,
			BENumber:              eval.BOEDetail.BENumber,
			BEDate:                eval.BOEDetail.BEDate,
			PortOfImport:          eval.BOEDetail.Location,
			AssessedBCDTotalPaise: eval.AssessedBCDTotalPaise,
			CashBCDPaidPaise:      eval.CashBCDPaidPaise,
			NetConfirmedPaise:     eval.NetConfirmedRoDTEP,
			ActiveReservedPaise:   eval.ActiveReservedRoDTEP,
			EligibleUnpaidPaise:   eval.EligibleUnpaidBCDPaise,
			AssessedBCDINR:        eval.AssessedBCDINR,
			NetConfirmedINR:       float64(eval.NetConfirmedRoDTEP) / 100.0,
			EligibleUnpaidINR:     eval.EligibleUnpaidBCDINR,
			Allocations:           eval.ActiveAllocations,
			ConfirmedDebits:       debits,
		}
		summaries = append(summaries, sum)
	}
	return summaries, nil
}

// IngestInitialScrips is an administrative helper to batch-create scrips (e.g. from the attached Excel reference).
func (s *Service) IngestInitialScrips(scrips []models.RoDTEPScripInput, verifiedBy string) (int, error) {
	_, uName, err := s.getOperator(security.PermDataWrite)
	if err != nil {
		return 0, err
	}
	if verifiedBy == "" {
		verifiedBy = uName
	}

	count := 0
	for _, input := range scrips {
		if strings.TrimSpace(input.ScripNumber) == "" {
			continue
		}
		// Check if scrip already exists
		if _, err := s.repo.GetScripByNumber(input.ScripNumber); err == nil {
			// Already exists, skip
			continue
		}
		if input.OpeningVerifiedBy == "" && input.OpeningEvidenceRef != "" {
			input.OpeningVerifiedBy = verifiedBy
			input.OpeningVerifiedAt = time.Now().Format("2006-01-02 15:04:05")
		}
		if _, err := s.repo.CreateScrip(input, uName); err != nil {
			return count, fmt.Errorf("ingest scrip %s: %w", input.ScripNumber, err)
		}
		count++
	}
	return count, nil
}
