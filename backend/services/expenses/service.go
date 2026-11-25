package expenses

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/validation"
)

const (
	maxAttachmentBytes = 15 * 1024 * 1024
	maxRoundOffPaise   = int64(10000) // ±₹100 whole-rupee round-off guardrail
	maxInvoiceNoLen    = 100
	maxRemarksLen      = 1000
)

var (
	gstinPattern    = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

// Service manages Service Providers, Expense Types, Multi-line Paise Expense Invoices,
// attachments, and RFC 4180 CSV bulk imports.
type Service struct {
	db             *sql.DB
	sec            *security.Service
	audit          *audit.Service
	attachmentsDir string
}

// NewService constructs the expenses domain service.
func NewService(db *sql.DB, sec *security.Service, aud *audit.Service, attachmentsDir string) *Service {
	return &Service{
		db:             db,
		sec:            sec,
		audit:          aud,
		attachmentsDir: attachmentsDir,
	}
}

// SetDB rebinds the service to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
}

// --- Service Providers ---

func (s *Service) ListServiceProviders(search string) ([]models.ServiceProvider, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	q := `SELECT id, name, COALESCE(gstin,''), COALESCE(state,''), COALESCE(contact_person,''),
	             COALESCE(contact_email,''), COALESCE(contact_phone,''), COALESCE(created_at,'')
	      FROM service_providers
	      WHERE deleted_at IS NULL`
	var args []any
	if st := strings.TrimSpace(search); st != "" {
		like := "%" + st + "%"
		q += ` AND (name LIKE ? OR COALESCE(gstin,'') LIKE ? OR COALESCE(state,'') LIKE ?)`
		args = append(args, like, like, like)
	}
	q += ` ORDER BY LOWER(name) ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ServiceProvider
	for rows.Next() {
		var sp models.ServiceProvider
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.GSTIN, &sp.State, &sp.ContactPerson, &sp.ContactEmail, &sp.ContactPhone, &sp.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *Service) CreateServiceProvider(in models.ServiceProviderInput) (models.ServiceProvider, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ServiceProvider{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.ServiceProvider{}, errors.New("service provider name is required")
	}
	gstin := strings.ToUpper(strings.TrimSpace(in.GSTIN))
	var gstinArg any
	if gstin != "" {
		if !gstinPattern.MatchString(gstin) {
			return models.ServiceProvider{}, errors.New("invalid GSTIN format (expected 15-character Indian GSTIN)")
		}
		gstinArg = gstin
	}

	id := "SP-" + strings.ToUpper(uuid.NewString()[:8])
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO service_providers (id, name, gstin, state, contact_person, contact_email, contact_phone)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, name, gstinArg, strings.TrimSpace(in.State), strings.TrimSpace(in.ContactPerson),
			strings.TrimSpace(in.ContactEmail), strings.TrimSpace(in.ContactPhone),
		); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return errors.New("service provider with matching GSTIN already exists")
			}
			return err
		}
		return s.audit.RecordTableAuditTx(tx, "service_providers", id, "INSERT", sess.UserID, nil, in, nil)
	})
	if err != nil {
		return models.ServiceProvider{}, err
	}
	return s.getServiceProviderByID(id)
}

func (s *Service) UpdateServiceProvider(id string, in models.ServiceProviderInput) (models.ServiceProvider, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ServiceProvider{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.ServiceProvider{}, errors.New("service provider name is required")
	}
	gstin := strings.ToUpper(strings.TrimSpace(in.GSTIN))
	var gstinArg any
	if gstin != "" {
		if !gstinPattern.MatchString(gstin) {
			return models.ServiceProvider{}, errors.New("invalid GSTIN format (expected 15-character Indian GSTIN)")
		}
		gstinArg = gstin
	}

	beforeSP, _ := s.getServiceProviderByID(id)
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			UPDATE service_providers
			SET name = ?, gstin = ?, state = ?, contact_person = ?, contact_email = ?, contact_phone = ?
			WHERE id = ? AND deleted_at IS NULL`,
			name, gstinArg, strings.TrimSpace(in.State), strings.TrimSpace(in.ContactPerson),
			strings.TrimSpace(in.ContactEmail), strings.TrimSpace(in.ContactPhone), id,
		)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return errors.New("service provider with matching GSTIN already exists")
			}
			return err
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			return fmt.Errorf("service provider %q not found", id)
		}
		return s.audit.RecordTableAuditTx(tx, "service_providers", id, "UPDATE", sess.UserID, beforeSP, in, nil)
	})
	if err != nil {
		return models.ServiceProvider{}, err
	}
	return s.getServiceProviderByID(id)
}

func (s *Service) DeleteServiceProvider(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	return transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if depErr, err := integrity.CheckDependenciesTx(context.Background(), tx, "service_providers", id, true); err != nil {
			return err
		} else if depErr != nil {
			return depErr
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := tx.Exec(`UPDATE service_providers SET deleted_at = ?, deleted_by = ? WHERE id = ? AND deleted_at IS NULL`, now, sess.Username, id)
		if err != nil {
			return err
		}
		if aff, _ := res.RowsAffected(); aff == 0 {
			return fmt.Errorf("service provider %q not found", id)
		}
		return s.audit.RecordTableAuditTx(tx, "service_providers", id, "DELETE", sess.UserID, nil, nil, nil)
	})
}

func (s *Service) getServiceProviderByID(id string) (models.ServiceProvider, error) {
	var sp models.ServiceProvider
	err := s.db.QueryRow(`
		SELECT id, name, COALESCE(gstin,''), COALESCE(state,''), COALESCE(contact_person,''),
		       COALESCE(contact_email,''), COALESCE(contact_phone,''), COALESCE(created_at,'')
		FROM service_providers WHERE id = ? AND deleted_at IS NULL`, id,
	).Scan(&sp.ID, &sp.Name, &sp.GSTIN, &sp.State, &sp.ContactPerson, &sp.ContactEmail, &sp.ContactPhone, &sp.CreatedAt)
	return sp, err
}

// --- Expense Types ---

func (s *Service) ListExpenseTypes(activeOnly bool) ([]models.ExpenseType, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	q := `SELECT id, name,
	             COALESCE(default_cgst_rate_bp, CAST(ROUND(COALESCE(default_cgst_rate,0)*100) AS INTEGER)),
	             COALESCE(default_sgst_rate_bp, CAST(ROUND(COALESCE(default_sgst_rate,0)*100) AS INTEGER)),
	             COALESCE(default_igst_rate_bp, CAST(ROUND(COALESCE(default_igst_rate,0)*100) AS INTEGER)),
	             COALESCE(default_tds_rate_bp, 0),
	             COALESCE(is_active, 1),
	             COALESCE(created_at, '')
	      FROM expense_types
	      WHERE deleted_at IS NULL`
	if activeOnly {
		q += ` AND COALESCE(is_active, 1) = 1`
	}
	q += ` ORDER BY LOWER(name) ASC`

	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ExpenseType
	for rows.Next() {
		var et models.ExpenseType
		if err := rows.Scan(&et.ID, &et.Name, &et.DefaultCGSTRateBP, &et.DefaultSGSTRateBP, &et.DefaultIGSTRateBP, &et.DefaultTDSRateBP, &et.IsActive, &et.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, et)
	}
	return out, rows.Err()
}

func (s *Service) CreateExpenseType(in models.ExpenseTypeInput) (models.ExpenseType, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ExpenseType{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.ExpenseType{}, errors.New("expense type name is required")
	}
	if err := validateRateBPs(in.DefaultCGSTRateBP, in.DefaultSGSTRateBP, in.DefaultIGSTRateBP, in.DefaultTDSRateBP); err != nil {
		return models.ExpenseType{}, err
	}
	id := "ET-" + strings.ToUpper(uuid.NewString()[:8])
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO expense_types (
				id, name,
				default_cgst_rate, default_sgst_rate, default_igst_rate,
				default_cgst_rate_bp, default_sgst_rate_bp, default_igst_rate_bp, default_tds_rate_bp,
				is_active
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, name,
			float64(in.DefaultCGSTRateBP)/100.0, float64(in.DefaultSGSTRateBP)/100.0, float64(in.DefaultIGSTRateBP)/100.0,
			in.DefaultCGSTRateBP, in.DefaultSGSTRateBP, in.DefaultIGSTRateBP, in.DefaultTDSRateBP,
			in.IsActive,
		); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return fmt.Errorf("expense type %q already exists", name)
			}
			return err
		}
		return s.audit.RecordTableAuditTx(tx, "expense_types", id, "INSERT", sess.UserID, nil, in, nil)
	})
	if err != nil {
		return models.ExpenseType{}, err
	}
	return s.getExpenseTypeByID(id)
}

func (s *Service) UpdateExpenseType(id string, in models.ExpenseTypeInput) (models.ExpenseType, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ExpenseType{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.ExpenseType{}, errors.New("expense type name is required")
	}
	if err := validateRateBPs(in.DefaultCGSTRateBP, in.DefaultSGSTRateBP, in.DefaultIGSTRateBP, in.DefaultTDSRateBP); err != nil {
		return models.ExpenseType{}, err
	}
	beforeET, _ := s.getExpenseTypeByID(id)
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			UPDATE expense_types
			SET name = ?,
			    default_cgst_rate = ?, default_sgst_rate = ?, default_igst_rate = ?,
			    default_cgst_rate_bp = ?, default_sgst_rate_bp = ?, default_igst_rate_bp = ?, default_tds_rate_bp = ?,
			    is_active = ?
			WHERE id = ? AND deleted_at IS NULL`,
			name,
			float64(in.DefaultCGSTRateBP)/100.0, float64(in.DefaultSGSTRateBP)/100.0, float64(in.DefaultIGSTRateBP)/100.0,
			in.DefaultCGSTRateBP, in.DefaultSGSTRateBP, in.DefaultIGSTRateBP, in.DefaultTDSRateBP,
			in.IsActive, id,
		)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return fmt.Errorf("expense type %q already exists", name)
			}
			return err
		}
		if aff, _ := res.RowsAffected(); aff == 0 {
			return fmt.Errorf("expense type %q not found", id)
		}
		return s.audit.RecordTableAuditTx(tx, "expense_types", id, "UPDATE", sess.UserID, beforeET, in, nil)
	})
	if err != nil {
		return models.ExpenseType{}, err
	}
	return s.getExpenseTypeByID(id)
}

func (s *Service) DeleteExpenseType(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	return transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if depErr, err := integrity.CheckDependenciesTx(context.Background(), tx, "expense_types", id, true); err != nil {
			return err
		} else if depErr != nil {
			return depErr
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := tx.Exec(`UPDATE expense_types SET deleted_at = ?, deleted_by = ? WHERE id = ? AND deleted_at IS NULL`, now, sess.Username, id)
		if err != nil {
			return err
		}
		if aff, _ := res.RowsAffected(); aff == 0 {
			return fmt.Errorf("expense type %q not found", id)
		}
		return s.audit.RecordTableAuditTx(tx, "expense_types", id, "DELETE", sess.UserID, nil, nil, nil)
	})
}

func (s *Service) getExpenseTypeByID(id string) (models.ExpenseType, error) {
	var et models.ExpenseType
	err := s.db.QueryRow(`
		SELECT id, name,
		       COALESCE(default_cgst_rate_bp, 0),
		       COALESCE(default_sgst_rate_bp, 0),
		       COALESCE(default_igst_rate_bp, 0),
		       COALESCE(default_tds_rate_bp, 0),
		       COALESCE(is_active, 1),
		       COALESCE(created_at, '')
		FROM expense_types WHERE id = ? AND deleted_at IS NULL`, id,
	).Scan(&et.ID, &et.Name, &et.DefaultCGSTRateBP, &et.DefaultSGSTRateBP, &et.DefaultIGSTRateBP, &et.DefaultTDSRateBP, &et.IsActive, &et.CreatedAt)
	return et, err
}

func validateRateBPs(rates ...int64) error {
	for _, r := range rates {
		if r < 0 || r > 10000 {
			return errors.New("tax and TDS basis points must be between 0 and 10000")
		}
	}
	return nil
}

// --- Expense Invoice Preview, Combine & CRUD ---

// PreviewCalculation computes deterministic integer paise taxes & net payable.
func (s *Service) PreviewCalculation(lines []models.ExpenseLineInput) (models.ExpenseInvoiceCalculation, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.ExpenseInvoiceCalculation{}, err
	}
	return CalculateInvoice(lines)
}

// MergeDuplicateLinesAction combines duplicate expense types on draft lines for 1-click UI merge.
func (s *Service) MergeDuplicateLinesAction(lines []models.ExpenseLineInput) ([]models.ExpenseLineInput, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return MergeDuplicateLines(lines)
}

// ListExpenseInvoices returns expense invoices optionally filtered by shipmentId and search text.
func (s *Service) ListExpenseInvoices(shipmentID string, search string) ([]models.ExpenseInvoice, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	q := `SELECT ei.id, ei.shipment_id, COALESCE(sh.invoice_number,''),
	             ei.service_provider_id, COALESCE(sp.name,''),
	             ei.invoice_no, ei.invoice_date, COALESCE(ei.currency,'INR'),
	             COALESCE(ei.is_paid_by_clearing_agent, 0),
	             COALESCE(ei.clearing_agent_id, ''),
	             COALESCE(cha.name, ''),
	             COALESCE(ei.cha_reimburse_invoice_no, ''),
	             COALESCE(ei.cha_reimburse_invoice_date, ''),
	             COALESCE(ei.total_amount_paise, CAST(ROUND(ei.total_amount*100) AS INTEGER)),
	             COALESCE(ei.total_cgst_amount_paise, CAST(ROUND(ei.total_cgst_amount*100) AS INTEGER)),
	             COALESCE(ei.total_sgst_amount_paise, CAST(ROUND(ei.total_sgst_amount*100) AS INTEGER)),
	             COALESCE(ei.total_igst_amount_paise, CAST(ROUND(ei.total_igst_amount*100) AS INTEGER)),
	             COALESCE(ei.total_tds_amount_paise, 0),
	             COALESCE(ei.round_off_paise, 0),
	             COALESCE(ei.net_amount_paise, CAST(ROUND(ei.total_amount*100) AS INTEGER)),
	             COALESCE(ei.remarks,''), COALESCE(ei.idempotency_key,''), COALESCE(ei.version, 1),
	             COALESCE(ei.created_at,''), COALESCE(ei.updated_at,'')
	      FROM expense_invoices ei
	      LEFT JOIN shipments sh ON sh.id = ei.shipment_id
	      LEFT JOIN service_providers sp ON sp.id = ei.service_provider_id
	      LEFT JOIN service_providers cha ON cha.id = ei.clearing_agent_id
	      WHERE ei.deleted_at IS NULL`
	var args []any
	if sid := strings.TrimSpace(shipmentID); sid != "" {
		q += ` AND ei.shipment_id = ?`
		args = append(args, sid)
	}
	if st := strings.TrimSpace(search); st != "" {
		like := "%" + st + "%"
		q += ` AND (ei.invoice_no LIKE ? OR sp.name LIKE ? OR sh.invoice_number LIKE ? OR COALESCE(ei.remarks,'') LIKE ? OR COALESCE(cha.name,'') LIKE ? OR COALESCE(ei.cha_reimburse_invoice_no,'') LIKE ?)`
		args = append(args, like, like, like, like, like, like)
	}
	q += ` ORDER BY ei.invoice_date DESC, ei.created_at DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}

	var out []models.ExpenseInvoice
	for rows.Next() {
		var inv models.ExpenseInvoice
		var isPaidByCA int
		if err := rows.Scan(
			&inv.ID, &inv.ShipmentID, &inv.ShipmentInvoiceNo,
			&inv.ServiceProviderID, &inv.ServiceProviderName,
			&inv.InvoiceNo, &inv.InvoiceDate, &inv.Currency,
			&isPaidByCA, &inv.ClearingAgentID, &inv.ClearingAgentName,
			&inv.CHAReimburseInvoiceNo, &inv.CHAReimburseInvoiceDate,
			&inv.TotalAmountPaise, &inv.TotalCGSTPaise, &inv.TotalSGSTPaise,
			&inv.TotalIGSTPaise, &inv.TotalTDSPaise, &inv.RoundOffPaise, &inv.NetAmountPaise,
			&inv.Remarks, &inv.IdempotencyKey, &inv.Version, &inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		inv.IsPaidByClearingAgent = isPaidByCA == 1
		inv.Lines = make([]models.ExpenseLine, 0)
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	if len(out) > 0 {
		ids := make([]string, len(out))
		for i := range out {
			ids[i] = out[i].ID
		}
		linesByInv, err := s.loadInvoiceLinesBatch(ids)
		if err != nil {
			return nil, err
		}
		for i := range out {
			lines := linesByInv[out[i].ID]
			if lines == nil {
				lines = make([]models.ExpenseLine, 0)
			}
			out[i].Lines = lines
			var base int64
			for _, l := range lines {
				base += l.AmountPaise
			}
			out[i].BaseAmountPaise = base
		}
	}
	return out, nil
}

// GetExpenseInvoice fetches a single expense invoice along with all its lines.
func (s *Service) GetExpenseInvoice(id string) (models.ExpenseInvoice, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.ExpenseInvoice{}, err
	}
	var inv models.ExpenseInvoice
	var isPaidByCA int
	err := s.db.QueryRow(`
		SELECT ei.id, ei.shipment_id, COALESCE(sh.invoice_number,''),
		       ei.service_provider_id, COALESCE(sp.name,''),
		       ei.invoice_no, ei.invoice_date, COALESCE(ei.currency,'INR'),
		       COALESCE(ei.is_paid_by_clearing_agent, 0),
		       COALESCE(ei.clearing_agent_id, ''),
		       COALESCE(cha.name, ''),
		       COALESCE(ei.cha_reimburse_invoice_no, ''),
		       COALESCE(ei.cha_reimburse_invoice_date, ''),
		       COALESCE(ei.total_amount_paise, CAST(ROUND(ei.total_amount*100) AS INTEGER)),
		       COALESCE(ei.total_cgst_amount_paise, CAST(ROUND(ei.total_cgst_amount*100) AS INTEGER)),
		       COALESCE(ei.total_sgst_amount_paise, CAST(ROUND(ei.total_sgst_amount*100) AS INTEGER)),
		       COALESCE(ei.total_igst_amount_paise, CAST(ROUND(ei.total_igst_amount*100) AS INTEGER)),
		       COALESCE(ei.total_tds_amount_paise, 0),
		       COALESCE(ei.round_off_paise, 0),
		       COALESCE(ei.net_amount_paise, CAST(ROUND(ei.total_amount*100) AS INTEGER)),
		       COALESCE(ei.remarks,''), COALESCE(ei.idempotency_key,''), COALESCE(ei.version, 1),
		       COALESCE(ei.created_at,''), COALESCE(ei.updated_at,'')
		FROM expense_invoices ei
		LEFT JOIN shipments sh ON sh.id = ei.shipment_id
		LEFT JOIN service_providers sp ON sp.id = ei.service_provider_id
		LEFT JOIN service_providers cha ON cha.id = ei.clearing_agent_id
		WHERE ei.id = ? AND ei.deleted_at IS NULL`, id,
	).Scan(
		&inv.ID, &inv.ShipmentID, &inv.ShipmentInvoiceNo,
		&inv.ServiceProviderID, &inv.ServiceProviderName,
		&inv.InvoiceNo, &inv.InvoiceDate, &inv.Currency,
		&isPaidByCA, &inv.ClearingAgentID, &inv.ClearingAgentName,
		&inv.CHAReimburseInvoiceNo, &inv.CHAReimburseInvoiceDate,
		&inv.TotalAmountPaise, &inv.TotalCGSTPaise, &inv.TotalSGSTPaise,
		&inv.TotalIGSTPaise, &inv.TotalTDSPaise, &inv.RoundOffPaise, &inv.NetAmountPaise,
		&inv.Remarks, &inv.IdempotencyKey, &inv.Version, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return models.ExpenseInvoice{}, err
	}
	inv.IsPaidByClearingAgent = isPaidByCA == 1
	lines, err := s.loadInvoiceLines(inv.ID)
	if err != nil {
		return models.ExpenseInvoice{}, err
	}
	inv.Lines = lines
	for _, l := range lines {
		inv.BaseAmountPaise += l.AmountPaise
	}
	return inv, nil
}

func (s *Service) loadInvoiceLines(invoiceID string) ([]models.ExpenseLine, error) {
	byInv, err := s.loadInvoiceLinesBatch([]string{invoiceID})
	if err != nil {
		return nil, err
	}
	lines := byInv[invoiceID]
	if lines == nil {
		return make([]models.ExpenseLine, 0), nil
	}
	return lines, nil
}

func (s *Service) loadInvoiceLinesBatch(invoiceIDs []string) (map[string][]models.ExpenseLine, error) {
	out := make(map[string][]models.ExpenseLine, len(invoiceIDs))
	if len(invoiceIDs) == 0 {
		return out, nil
	}
	const chunkSize = 400
	for start := 0; start < len(invoiceIDs); start += chunkSize {
		end := min(start+chunkSize, len(invoiceIDs))
		batch := invoiceIDs[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}
		q := fmt.Sprintf(`
			SELECT e.id, e.expense_invoice_id, e.shipment_id, e.service_provider_id,
			       e.expense_type_id, COALESCE(NULLIF(et.name,''), e.expense_type_id, 'Expense Item'),
			       COALESCE(NULLIF(e.amount_paise,0), CAST(ROUND(COALESCE(e.amount,0)*100) AS INTEGER), 0),
			       CAST(ROUND(COALESCE(e.cgst_rate,0)*100) AS INTEGER),
			       CAST(ROUND(COALESCE(e.sgst_rate,0)*100) AS INTEGER),
			       CAST(ROUND(COALESCE(e.igst_rate,0)*100) AS INTEGER),
			       CAST(ROUND(COALESCE(e.tds_rate,0)*100) AS INTEGER),
			       COALESCE(e.cgst_amount_paise, 0),
			       COALESCE(e.sgst_amount_paise, 0),
			       COALESCE(e.igst_amount_paise, 0),
			       COALESCE(e.tds_amount_paise, 0),
			       COALESCE(NULLIF(e.total_amount_paise,0), CAST(ROUND(COALESCE(e.total_amount, e.amount, 0)*100) AS INTEGER), 0),
			       COALESCE(NULLIF(e.net_amount_paise,0), CAST(ROUND(COALESCE(e.total_amount, e.amount, 0)*100) AS INTEGER), 0),
			       COALESCE(e.is_manual_gst, 0),
			       COALESCE(e.remarks,'')
			FROM expenses e
			LEFT JOIN expense_types et ON et.id = e.expense_type_id
			WHERE e.expense_invoice_id IN (%s) AND e.deleted_at IS NULL
			ORDER BY e.created_at ASC, e.rowid ASC`, placeholders)
		rows, err := s.db.Query(q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var l models.ExpenseLine
			var isManualInt int
			if err := rows.Scan(
				&l.ID, &l.ExpenseInvoiceID, &l.ShipmentID, &l.ServiceProviderID,
				&l.ExpenseTypeID, &l.ExpenseTypeName,
				&l.AmountPaise, &l.CGSTRateBP, &l.SGSTRateBP, &l.IGSTRateBP, &l.TDSRateBP,
				&l.CGSTPaise, &l.SGSTPaise, &l.IGSTPaise, &l.TDSPaise, &l.TotalPaise, &l.NetPaise,
				&isManualInt, &l.Remarks,
			); err != nil {
				_ = rows.Close()
				return nil, err
			}
			l.IsManualGST = (isManualInt != 0)
			out[l.ExpenseInvoiceID] = append(out[l.ExpenseInvoiceID], l)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return out, nil
}

// CreateExpenseInvoice atomically stores a multi-line expense invoice with idempotency protection
// and exact integer paise math.
func (s *Service) CreateExpenseInvoice(req models.ExpenseInvoiceRequest) (models.ExpenseInvoice, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ExpenseInvoice{}, err
	}
	preparedLines, calc, normDate, normChaDate, cur, err := s.prepareAndValidateInvoice(req)
	if err != nil {
		return models.ExpenseInvoice{}, err
	}

	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey != "" {
		var existingID string
		err := s.db.QueryRow(`SELECT id FROM expense_invoices WHERE idempotency_key = ? AND deleted_at IS NULL`, idemKey).Scan(&existingID)
		if err == nil && existingID != "" {
			existing, getErr := s.GetExpenseInvoice(existingID)
			if getErr != nil {
				return models.ExpenseInvoice{}, getErr
			}
			if !sameInvoiceIdempotentPayload(existing, req, calc) {
				return models.ExpenseInvoice{}, fmt.Errorf("idempotency key %q is already associated with a different expense invoice", idemKey)
			}
			return existing, nil
		}
	}

	invID, err := s.insertSingleInvoiceWithTx(sess, req, preparedLines, calc, normDate, normChaDate, cur, idemKey)
	if err != nil {
		return models.ExpenseInvoice{}, err
	}
	_, _ = s.RecordExpenseAllocation(context.Background(), req.ShipmentID, "CREATE_EXPENSE_INVOICE: "+req.InvoiceNo)
	return s.GetExpenseInvoice(invID)
}

// CreateExpenseInvoicesBatch atomically creates multiple service-provider expense invoices
// (for one or more shipments, including shared CHA reimbursement bills) inside a single transaction.
func (s *Service) CreateExpenseInvoicesBatch(reqs []models.ExpenseInvoiceRequest) ([]models.ExpenseInvoice, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return nil, err
	}
	if len(reqs) == 0 {
		return nil, errors.New("at least one expense invoice is required in batch")
	}
	if len(reqs) > 50 {
		return nil, errors.New("batch cannot exceed 50 expense invoices at once")
	}

	type preparedItem struct {
		req           models.ExpenseInvoiceRequest
		preparedLines []models.ExpenseLineInput
		calc          models.ExpenseInvoiceCalculation
		normDate      string
		normChaDate   string
		cur           string
		idemKey       string
	}

	items := make([]preparedItem, 0, len(reqs))
	seenProviderInv := make(map[string]int, len(reqs))
	seenIdemKeys := make(map[string]int, len(reqs))

	for i, req := range reqs {
		preparedLines, calc, normDate, normChaDate, cur, prepErr := s.prepareAndValidateInvoice(req)
		if prepErr != nil {
			return nil, fmt.Errorf("invoice #%d (%s): %w", i+1, strings.TrimSpace(req.InvoiceNo), prepErr)
		}
		spInvKey := req.ServiceProviderID + "|" + strings.ToLower(strings.TrimSpace(req.InvoiceNo))
		if prevIdx, dup := seenProviderInv[spInvKey]; dup {
			return nil, fmt.Errorf("invoice #%d (%s) duplicates provider & invoice number from invoice #%d", i+1, strings.TrimSpace(req.InvoiceNo), prevIdx+1)
		}
		seenProviderInv[spInvKey] = i

		idemKey := strings.TrimSpace(req.IdempotencyKey)
		if idemKey != "" {
			if prevIdx, dup := seenIdemKeys[idemKey]; dup {
				return nil, fmt.Errorf("invoice #%d shares duplicate idempotencyKey with invoice #%d", i+1, prevIdx+1)
			}
			seenIdemKeys[idemKey] = i
		}
		items = append(items, preparedItem{
			req:           req,
			preparedLines: preparedLines,
			calc:          calc,
			normDate:      normDate,
			normChaDate:   normChaDate,
			cur:           cur,
			idemKey:       idemKey,
		})
	}

	createdIDs := make([]string, len(items))
	reusedIDs := make(map[int]string)

	for i, it := range items {
		if it.idemKey != "" {
			var existingID string
			err := s.db.QueryRow(`SELECT id FROM expense_invoices WHERE idempotency_key = ? AND deleted_at IS NULL`, it.idemKey).Scan(&existingID)
			if err == nil && existingID != "" {
				existing, getErr := s.GetExpenseInvoice(existingID)
				if getErr != nil {
					return nil, getErr
				}
				if !sameInvoiceIdempotentPayload(existing, it.req, it.calc) {
					return nil, fmt.Errorf("invoice #%d: idempotency key %q is already associated with a different expense invoice", i+1, it.idemKey)
				}
				reusedIDs[i] = existingID
				createdIDs[i] = existingID
			}
		}
	}

	shipmentSet := make(map[string]struct{})
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		for i, it := range items {
			if _, reused := reusedIDs[i]; reused {
				continue
			}
			invID, txErr := s.insertInvoiceTx(tx, sess, it.req, it.preparedLines, it.calc, it.normDate, it.normChaDate, it.cur, it.idemKey)
			if txErr != nil {
				return fmt.Errorf("invoice #%d (%s): %w", i+1, strings.TrimSpace(it.req.InvoiceNo), txErr)
			}
			createdIDs[i] = invID
			shipmentSet[it.req.ShipmentID] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for shipID := range shipmentSet {
		_, _ = s.RecordExpenseAllocation(context.Background(), shipID, "BATCH_CREATE_EXPENSE_INVOICES")
	}

	out := make([]models.ExpenseInvoice, 0, len(createdIDs))
	for _, invID := range createdIDs {
		inv, getErr := s.GetExpenseInvoice(invID)
		if getErr != nil {
			return nil, getErr
		}
		out = append(out, inv)
	}
	return out, nil
}

func (s *Service) insertSingleInvoiceWithTx(
	sess security.SessionInfo,
	req models.ExpenseInvoiceRequest,
	preparedLines []models.ExpenseLineInput,
	calc models.ExpenseInvoiceCalculation,
	normDate, normChaDate, cur, idemKey string,
) (string, error) {
	var invID string
	err := transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		id, txErr := s.insertInvoiceTx(tx, sess, req, preparedLines, calc, normDate, normChaDate, cur, idemKey)
		if txErr != nil {
			return txErr
		}
		invID = id
		return nil
	})
	return invID, err
}

func (s *Service) insertInvoiceTx(
	tx *sql.Tx,
	sess security.SessionInfo,
	req models.ExpenseInvoiceRequest,
	preparedLines []models.ExpenseLineInput,
	calc models.ExpenseInvoiceCalculation,
	normDate, normChaDate, cur, idemKey string,
) (string, error) {
	if err := s.ensureMutableRefs(tx, "", req.ShipmentID, req.ServiceProviderID, req.ClearingAgentID, req.IsPaidByClearingAgent, preparedLines); err != nil {
		return "", err
	}
	var dupCount int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM expense_invoices
		WHERE service_provider_id = ? AND LOWER(invoice_no) = LOWER(?) AND deleted_at IS NULL`,
		req.ServiceProviderID, strings.TrimSpace(req.InvoiceNo),
	).Scan(&dupCount); err != nil {
		return "", err
	}
	if dupCount > 0 {
		return "", fmt.Errorf("duplicate expense invoice %q already exists for this service provider. To add services, edit the existing invoice", req.InvoiceNo)
	}

	invID := "EINV-" + strings.ToUpper(uuid.NewString()[:8])
	var idemArg any
	if idemKey != "" {
		idemArg = idemKey
	}

	now := time.Now().UTC().Format(time.RFC3339)
	isPaidByCA := 0
	var caID, chaReimbNo, chaReimbDate any
	if req.IsPaidByClearingAgent {
		isPaidByCA = 1
		if trimmed := strings.TrimSpace(req.ClearingAgentID); trimmed != "" {
			caID = trimmed
		}
		if trimmed := strings.TrimSpace(req.CHAReimburseInvoiceNo); trimmed != "" {
			chaReimbNo = trimmed
		}
		if normChaDate != "" {
			chaReimbDate = normChaDate
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO expense_invoices (
			id, shipment_id, service_provider_id, invoice_no, invoice_number, invoice_date, currency,
			is_paid_by_clearing_agent, clearing_agent_id, cha_reimburse_invoice_no, cha_reimburse_invoice_date,
			total_amount, total_cgst_amount, total_sgst_amount, total_igst_amount,
			total_amount_paise, total_cgst_amount_paise, total_sgst_amount_paise,
			total_igst_amount_paise, total_tds_amount_paise, net_amount_paise, round_off_paise,
			remarks, idempotency_key, version, created_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?)`,
		invID, req.ShipmentID, req.ServiceProviderID,
		strings.TrimSpace(req.InvoiceNo), strings.TrimSpace(req.InvoiceNo), normDate, cur,
		isPaidByCA, caID, chaReimbNo, chaReimbDate,
		float64(calc.TotalAmountPaise)/100.0, float64(calc.TotalCGSTPaise)/100.0,
		float64(calc.TotalSGSTPaise)/100.0, float64(calc.TotalIGSTPaise)/100.0,
		calc.TotalAmountPaise, calc.TotalCGSTPaise, calc.TotalSGSTPaise,
		calc.TotalIGSTPaise, calc.TotalTDSPaise, calc.NetAmountPaise, calc.RoundOffPaise,
		strings.TrimSpace(req.Remarks), idemArg, sess.Username, now, now,
	); err != nil {
		return "", err
	}

	if err := insertExpenseLinesTx(tx, invID, req.ShipmentID, req.ServiceProviderID, strings.TrimSpace(req.InvoiceNo), normDate, sess.Username, now, calc.Lines); err != nil {
		return "", err
	}
	if err := s.audit.RecordTableAuditTx(tx, "expense_invoices", invID, "INSERT", sess.UserID, nil, req, nil); err != nil {
		return "", err
	}
	return invID, nil
}

func sameInvoiceIdempotentPayload(existing models.ExpenseInvoice, req models.ExpenseInvoiceRequest, calc models.ExpenseInvoiceCalculation) bool {
	return existing.ShipmentID == req.ShipmentID &&
		existing.ServiceProviderID == req.ServiceProviderID &&
		strings.EqualFold(strings.TrimSpace(existing.InvoiceNo), strings.TrimSpace(req.InvoiceNo)) &&
		existing.BaseAmountPaise == calc.BaseAmountPaise &&
		existing.RoundOffPaise == req.RoundOffPaise &&
		len(existing.Lines) == len(calc.Lines)
}

// UpdateExpenseInvoice implements full optimistic-locked updating of an expense invoice,
// preserving existing line IDs (and attachments) while updating/soft-deleting/inserting lines.
func (s *Service) UpdateExpenseInvoice(req models.ExpenseInvoiceRequest) (models.ExpenseInvoice, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ExpenseInvoice{}, err
	}
	invID := strings.TrimSpace(req.ID)
	if invID == "" {
		return models.ExpenseInvoice{}, errors.New("expense invoice id is required for update")
	}
	if req.Version <= 0 {
		return models.ExpenseInvoice{}, errors.New("version is required for optimistic locking")
	}

	preparedLines, calc, normDate, normChaDate, cur, err := s.prepareAndValidateInvoice(req)
	if err != nil {
		return models.ExpenseInvoice{}, err
	}

	beforeInv, err := s.GetExpenseInvoice(invID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.ExpenseInvoice{}, fmt.Errorf("expense invoice %q not found", invID)
		}
		return models.ExpenseInvoice{}, err
	}

	var oldShipmentID string
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var curVersion int
		err := tx.QueryRow(`SELECT COALESCE(version,1), shipment_id FROM expense_invoices WHERE id = ? AND deleted_at IS NULL`, invID).Scan(&curVersion, &oldShipmentID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("expense invoice %q not found", invID)
		}
		if err != nil {
			return err
		}
		if req.Version != curVersion {
			return fmt.Errorf("optimistic lock conflict: expected version %d but database is at version %d", req.Version, curVersion)
		}

		if err := s.ensureMutableRefs(tx, invID, oldShipmentID, req.ServiceProviderID, req.ClearingAgentID, req.IsPaidByClearingAgent, preparedLines); err != nil {
			return err
		}
		if oldShipmentID != req.ShipmentID {
			if err := s.ensureMutableRefs(tx, invID, req.ShipmentID, req.ServiceProviderID, req.ClearingAgentID, req.IsPaidByClearingAgent, preparedLines); err != nil {
				return err
			}
		}

		var dupCount int
		if err := tx.QueryRow(`
			SELECT COUNT(*) FROM expense_invoices
			WHERE service_provider_id = ? AND LOWER(invoice_no) = LOWER(?) AND id != ? AND deleted_at IS NULL`,
			req.ServiceProviderID, strings.TrimSpace(req.InvoiceNo), invID,
		).Scan(&dupCount); err != nil {
			return err
		}
		if dupCount > 0 {
			return fmt.Errorf("duplicate expense invoice %q already exists for this service provider. To add services, edit the existing invoice", req.InvoiceNo)
		}

		now := time.Now().UTC().Format(time.RFC3339)
		isPaidByCA := 0
		var caID, chaReimbNo, chaReimbDate any
		if req.IsPaidByClearingAgent {
			isPaidByCA = 1
			if trimmed := strings.TrimSpace(req.ClearingAgentID); trimmed != "" {
				caID = trimmed
			}
			if trimmed := strings.TrimSpace(req.CHAReimburseInvoiceNo); trimmed != "" {
				chaReimbNo = trimmed
			}
			if normChaDate != "" {
				chaReimbDate = normChaDate
			}
		}

		res, err := tx.Exec(`
			UPDATE expense_invoices
			SET shipment_id = ?, service_provider_id = ?, invoice_no = ?, invoice_number = ?, invoice_date = ?, currency = ?,
			    is_paid_by_clearing_agent = ?, clearing_agent_id = ?, cha_reimburse_invoice_no = ?, cha_reimburse_invoice_date = ?,
			    total_amount = ?, total_cgst_amount = ?, total_sgst_amount = ?, total_igst_amount = ?,
			    total_amount_paise = ?, total_cgst_amount_paise = ?, total_sgst_amount_paise = ?,
			    total_igst_amount_paise = ?, total_tds_amount_paise = ?, net_amount_paise = ?, round_off_paise = ?,
			    remarks = ?, version = version + 1, updated_at = ?
			WHERE id = ? AND deleted_at IS NULL AND COALESCE(version,1) = ?`,
			req.ShipmentID, req.ServiceProviderID, strings.TrimSpace(req.InvoiceNo), strings.TrimSpace(req.InvoiceNo), normDate, cur,
			isPaidByCA, caID, chaReimbNo, chaReimbDate,
			float64(calc.TotalAmountPaise)/100.0, float64(calc.TotalCGSTPaise)/100.0,
			float64(calc.TotalSGSTPaise)/100.0, float64(calc.TotalIGSTPaise)/100.0,
			calc.TotalAmountPaise, calc.TotalCGSTPaise, calc.TotalSGSTPaise,
			calc.TotalIGSTPaise, calc.TotalTDSPaise, calc.NetAmountPaise, calc.RoundOffPaise,
			strings.TrimSpace(req.Remarks), now, invID, req.Version,
		)
		if err != nil {
			return err
		}
		if aff, _ := res.RowsAffected(); aff == 0 {
			return fmt.Errorf("optimistic lock conflict: expense invoice %q was modified concurrently", invID)
		}

		if err := upsertExpenseLinesTx(tx, invID, req.ShipmentID, req.ServiceProviderID, strings.TrimSpace(req.InvoiceNo), normDate, sess.Username, now, calc.Lines); err != nil {
			return err
		}
		return s.audit.RecordTableAuditTx(tx, "expense_invoices", invID, "UPDATE", sess.UserID, beforeInv, req, nil)
	})
	if err != nil {
		return models.ExpenseInvoice{}, err
	}
	if oldShipmentID != "" && oldShipmentID != req.ShipmentID {
		_, _ = s.RecordExpenseAllocation(context.Background(), oldShipmentID, "MOVE_OUT_EXPENSE_INVOICE: "+req.InvoiceNo)
	}
	_, _ = s.RecordExpenseAllocation(context.Background(), req.ShipmentID, "UPDATE_EXPENSE_INVOICE: "+req.InvoiceNo)
	return s.GetExpenseInvoice(invID)
}

// DeleteExpenseInvoice soft-deletes an expense invoice and its line items unless the shipment is frozen.
func (s *Service) DeleteExpenseInvoice(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	var deletedShipmentID string
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var shipmentID string
		var frozen bool
		err := tx.QueryRow(`
			SELECT ei.shipment_id, COALESCE(sh.is_frozen, 0)
			FROM expense_invoices ei
			JOIN shipments sh ON sh.id = ei.shipment_id
			WHERE ei.id = ? AND ei.deleted_at IS NULL`, id,
		).Scan(&shipmentID, &frozen)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("expense invoice %q not found", id)
		}
		if err != nil {
			return err
		}
		if frozen {
			return fmt.Errorf("shipment %s is frozen; cannot delete linked expense invoice", shipmentID)
		}
		deletedShipmentID = shipmentID
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := tx.Exec(`UPDATE expenses SET deleted_at = ?, deleted_by = ? WHERE expense_invoice_id = ? AND deleted_at IS NULL`, now, sess.Username, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE expense_invoices SET deleted_at = ?, deleted_by = ? WHERE id = ? AND deleted_at IS NULL`, now, sess.Username, id); err != nil {
			return err
		}
		return s.audit.RecordTableAuditTx(tx, "expense_invoices", id, "DELETE", sess.UserID, nil, nil, nil)
	})
	if err != nil {
		return err
	}
	if deletedShipmentID != "" {
		_, _ = s.RecordExpenseAllocation(context.Background(), deletedShipmentID, "DELETE_EXPENSE_INVOICE: "+id)
	}
	return nil
}

func (s *Service) prepareAndValidateInvoice(req models.ExpenseInvoiceRequest) ([]models.ExpenseLineInput, models.ExpenseInvoiceCalculation, string, string, string, error) {
	if strings.TrimSpace(req.ShipmentID) == "" {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("shipmentId is required")
	}
	if strings.TrimSpace(req.ServiceProviderID) == "" {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("serviceProviderId is required")
	}
	invNo := strings.TrimSpace(req.InvoiceNo)
	if invNo == "" {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("invoiceNo is required")
	}
	if len(invNo) > maxInvoiceNoLen {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", fmt.Errorf("invoiceNo cannot exceed %d characters", maxInvoiceNoLen)
	}
	if len(strings.TrimSpace(req.Remarks)) > maxRemarksLen {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", fmt.Errorf("remarks cannot exceed %d characters", maxRemarksLen)
	}
	normDate, err := validation.NormalizeDate(req.InvoiceDate)
	if err != nil || normDate == "" {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("valid invoiceDate is required")
	}
	cur := strings.ToUpper(strings.TrimSpace(req.Currency))
	if cur == "" {
		cur = "INR"
	}
	if !currencyPattern.MatchString(cur) {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("currency must be a 3-letter ISO code")
	}

	var normChaDate string
	if req.IsPaidByClearingAgent {
		caID := strings.TrimSpace(req.ClearingAgentID)
		if caID == "" {
			return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("clearingAgentId is required when invoice is marked as paid by clearing agent")
		}
		if caID == strings.TrimSpace(req.ServiceProviderID) {
			return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("clearing agent cannot be the same as the original service provider")
		}
		if len(strings.TrimSpace(req.CHAReimburseInvoiceNo)) > maxInvoiceNoLen {
			return nil, models.ExpenseInvoiceCalculation{}, "", "", "", fmt.Errorf("chaReimburseInvoiceNo cannot exceed %d characters", maxInvoiceNoLen)
		}
		if strings.TrimSpace(req.CHAReimburseInvoiceDate) != "" {
			normChaDate, err = validation.NormalizeDate(req.CHAReimburseInvoiceDate)
			if err != nil {
				return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("valid chaReimburseInvoiceDate is required")
			}
		}
	}

	lines := req.Lines
	if req.MergeDuplicates {
		lines, err = MergeDuplicateLines(lines)
		if err != nil {
			return nil, models.ExpenseInvoiceCalculation{}, "", "", "", err
		}
	} else if DetectDuplicateExpenseTypes(lines) {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("duplicate expense type found on invoice; combine duplicate lines before saving")
	}

	if req.RoundOffPaise < -maxRoundOffPaise || req.RoundOffPaise > maxRoundOffPaise {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", fmt.Errorf("round-off adjustment must be within ±₹%d", maxRoundOffPaise/100)
	}

	calc, err := CalculateInvoice(lines)
	if err != nil {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", err
	}
	if req.RoundOffPaise != 0 {
		calc.RoundOffPaise = req.RoundOffPaise
		calc.TotalAmountPaise += req.RoundOffPaise
		calc.NetAmountPaise += req.RoundOffPaise
		calc.TotalAmountINR = float64(calc.TotalAmountPaise) / 100.0
		calc.NetAmountINR = float64(calc.NetAmountPaise) / 100.0
	}
	if calc.TotalAmountPaise <= 0 || calc.NetAmountPaise < 0 {
		return nil, models.ExpenseInvoiceCalculation{}, "", "", "", errors.New("invoice total must be positive and net payable cannot be negative")
	}
	return lines, calc, normDate, normChaDate, cur, nil
}

func (s *Service) ensureMutableRefs(tx *sql.Tx, existingInvID, shipmentID, spID, clearingAgentID string, isPaidByCA bool, lines []models.ExpenseLineInput) error {
	var isFrozen bool
	err := tx.QueryRow(`SELECT COALESCE(is_frozen, 0) FROM shipments WHERE id = ? AND deleted_at IS NULL`, shipmentID).Scan(&isFrozen)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("shipment %q not found", shipmentID)
	}
	if err != nil {
		return err
	}
	if isFrozen {
		return fmt.Errorf("shipment %q is frozen and cannot accept expense updates", shipmentID)
	}

	var spExists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM service_providers WHERE id = ? AND deleted_at IS NULL`, spID).Scan(&spExists); err != nil {
		return err
	}
	if spExists == 0 {
		return fmt.Errorf("service provider %q not found", spID)
	}

	if isPaidByCA && strings.TrimSpace(clearingAgentID) != "" {
		var caExists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM service_providers WHERE id = ? AND deleted_at IS NULL`, strings.TrimSpace(clearingAgentID)).Scan(&caExists); err != nil {
			return err
		}
		if caExists == 0 {
			return fmt.Errorf("clearing agent %q not found", clearingAgentID)
		}
	}

	existingTypes := make(map[string]bool)
	if existingInvID != "" {
		rows, err := tx.Query(`SELECT expense_type_id FROM expenses WHERE expense_invoice_id = ? AND deleted_at IS NULL`, existingInvID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var etID string
			if scanErr := rows.Scan(&etID); scanErr == nil {
				existingTypes[etID] = true
			}
		}
		_ = rows.Close()
	}

	for _, l := range lines {
		var isActive bool
		err := tx.QueryRow(`SELECT COALESCE(is_active, 1) FROM expense_types WHERE id = ? AND deleted_at IS NULL`, l.ExpenseTypeID).Scan(&isActive)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("expense type %q not found", l.ExpenseTypeID)
		}
		if err != nil {
			return err
		}
		if !isActive && !existingTypes[l.ExpenseTypeID] {
			return fmt.Errorf("expense type %q is inactive and cannot be added to new invoices", l.ExpenseTypeID)
		}
	}
	return nil
}

func upsertExpenseLinesTx(tx *sql.Tx, invID, shipmentID, spID, invNo, invDate, user, now string, lines []models.ExpenseLineCalculation) error {
	rows, err := tx.Query(`SELECT id, expense_type_id FROM expenses WHERE expense_invoice_id = ? AND deleted_at IS NULL`, invID)
	if err != nil {
		return err
	}
	existingByType := make(map[string]string)
	for rows.Next() {
		var id, etID string
		if scanErr := rows.Scan(&id, &etID); scanErr != nil {
			_ = rows.Close()
			return scanErr
		}
		existingByType[etID] = id
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	desiredTypes := make(map[string]bool, len(lines))
	for _, l := range lines {
		desiredTypes[l.ExpenseTypeID] = true
	}

	// Soft-delete removed lines first so the active partial unique index stays clean
	// without breaking foreign key links on expense_attachments.
	for etID, existingLineID := range existingByType {
		if !desiredTypes[etID] {
			if _, err := tx.Exec(`UPDATE expenses SET deleted_at = ?, deleted_by = ? WHERE id = ?`, now, user, existingLineID); err != nil {
				return err
			}
		}
	}

	for _, l := range lines {
		cgstRate := float64(l.CGSTRateBP) / 100.0
		sgstRate := float64(l.SGSTRateBP) / 100.0
		igstRate := float64(l.IGSTRateBP) / 100.0
		if l.IsManualGST && l.AmountPaise > 0 {
			cgstRate = (float64(l.CGSTPaise) / float64(l.AmountPaise)) * 100.0
			sgstRate = (float64(l.SGSTPaise) / float64(l.AmountPaise)) * 100.0
			igstRate = (float64(l.IGSTPaise) / float64(l.AmountPaise)) * 100.0
		}
		isManualInt := 0
		if l.IsManualGST {
			isManualInt = 1
		}
		if existingLineID, ok := existingByType[l.ExpenseTypeID]; ok {
			if _, err := tx.Exec(`
				UPDATE expenses
				SET shipment_id = ?, service_provider_id = ?, invoice_no = ?, invoice_date = ?,
				    amount = ?, cgst_rate = ?, sgst_rate = ?, igst_rate = ?, tds_rate = ?,
				    amount_paise = ?, cgst_amount_paise = ?, sgst_amount_paise = ?, igst_amount_paise = ?,
				    tds_amount_paise = ?, total_amount_paise = ?, net_amount_paise = ?,
				    is_manual_gst = ?, remarks = ?, updated_at = ?
				WHERE id = ? AND deleted_at IS NULL`,
				shipmentID, spID, invNo, invDate,
				l.AmountINR, cgstRate, sgstRate, igstRate, float64(l.TDSRateBP)/100.0,
				l.AmountPaise, l.CGSTPaise, l.SGSTPaise, l.IGSTPaise,
				l.TDSPaise, l.TotalPaise, l.NetPaise,
				isManualInt, l.Remarks, now, existingLineID,
			); err != nil {
				return err
			}
			continue
		}

		lineID := "EXP-" + strings.ToUpper(uuid.NewString()[:8])
		if _, err := tx.Exec(`
			INSERT INTO expenses (
				id, expense_invoice_id, shipment_id, service_provider_id, invoice_no, invoice_date, expense_type_id,
				amount, cgst_rate, sgst_rate, igst_rate, tds_rate,
				amount_paise, cgst_amount_paise, sgst_amount_paise, igst_amount_paise, tds_amount_paise, total_amount_paise, net_amount_paise,
				is_manual_gst, remarks, created_by, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			lineID, invID, shipmentID, spID, invNo, invDate, l.ExpenseTypeID,
			l.AmountINR, cgstRate, sgstRate, igstRate, float64(l.TDSRateBP)/100.0,
			l.AmountPaise, l.CGSTPaise, l.SGSTPaise, l.IGSTPaise, l.TDSPaise, l.TotalPaise, l.NetPaise,
			isManualInt, l.Remarks, user, now, now,
		); err != nil {
			return err
		}
	}
	return nil
}

func insertExpenseLinesTx(tx *sql.Tx, invID, shipmentID, spID, invNo, invDate, user, now string, lines []models.ExpenseLineCalculation) error {
	for _, l := range lines {
		lineID := "EXP-" + strings.ToUpper(uuid.NewString()[:8])
		cgstRate := float64(l.CGSTRateBP) / 100.0
		sgstRate := float64(l.SGSTRateBP) / 100.0
		igstRate := float64(l.IGSTRateBP) / 100.0
		if l.IsManualGST && l.AmountPaise > 0 {
			cgstRate = (float64(l.CGSTPaise) / float64(l.AmountPaise)) * 100.0
			sgstRate = (float64(l.SGSTPaise) / float64(l.AmountPaise)) * 100.0
			igstRate = (float64(l.IGSTPaise) / float64(l.AmountPaise)) * 100.0
		}
		isManualInt := 0
		if l.IsManualGST {
			isManualInt = 1
		}
		if _, err := tx.Exec(`
			INSERT INTO expenses (
				id, expense_invoice_id, shipment_id, service_provider_id, invoice_no, invoice_date, expense_type_id,
				amount, cgst_rate, sgst_rate, igst_rate, tds_rate,
				amount_paise, cgst_amount_paise, sgst_amount_paise, igst_amount_paise, tds_amount_paise, total_amount_paise, net_amount_paise,
				is_manual_gst, remarks, created_by, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			lineID, invID, shipmentID, spID, invNo, invDate, l.ExpenseTypeID,
			l.AmountINR, cgstRate, sgstRate, igstRate, float64(l.TDSRateBP)/100.0,
			l.AmountPaise, l.CGSTPaise, l.SGSTPaise, l.IGSTPaise, l.TDSPaise, l.TotalPaise, l.NetPaise,
			isManualInt, l.Remarks, user, now, now,
		); err != nil {
			return err
		}
	}
	return nil
}

// --- Attachments ---

func (s *Service) ListAttachments(expenseID string) ([]models.ExpenseAttachment, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT id, expense_id, file_name, file_path, COALESCE(file_type,''), COALESCE(uploaded_at,''), COALESCE(uploaded_by,'')
		FROM expense_attachments WHERE expense_id = ? ORDER BY uploaded_at DESC`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ExpenseAttachment
	for rows.Next() {
		var a models.ExpenseAttachment
		if err := rows.Scan(&a.ID, &a.ExpenseID, &a.FileName, &a.FilePath, &a.FileType, &a.UploadedAt, &a.UploadedBy); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) UploadAttachment(expenseID, fileName, fileType, base64Data string) (models.ExpenseAttachment, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ExpenseAttachment{}, err
	}
	cleanName := filepath.Base(strings.TrimSpace(fileName))
	if cleanName == "" || cleanName == "." {
		return models.ExpenseAttachment{}, errors.New("valid filename is required")
	}
	ext := strings.ToLower(filepath.Ext(cleanName))
	switch ext {
	case ".pdf", ".png", ".jpg", ".jpeg", ".webp", ".csv", ".xlsx", ".txt":
	default:
		return models.ExpenseAttachment{}, fmt.Errorf("unsupported attachment file type %q", ext)
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64Data))
	if err != nil {
		return models.ExpenseAttachment{}, errors.New("invalid base64 attachment payload")
	}
	if len(payload) == 0 || len(payload) > maxAttachmentBytes {
		return models.ExpenseAttachment{}, errors.New("attachment size must be between 1 byte and 15 MB")
	}

	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM expenses WHERE id = ? AND deleted_at IS NULL`, expenseID).Scan(&exists); err != nil {
		return models.ExpenseAttachment{}, err
	}
	if exists == 0 {
		return models.ExpenseAttachment{}, fmt.Errorf("expense line %q not found", expenseID)
	}

	attID := "EXPA-" + strings.ToUpper(uuid.NewString()[:8])
	dir := filepath.Join(s.attachmentsDir, "expenses", expenseID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return models.ExpenseAttachment{}, err
	}
	dest := filepath.Join(dir, attID+"_"+cleanName)
	if err := os.WriteFile(dest, payload, 0600); err != nil {
		return models.ExpenseAttachment{}, err
	}

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO expense_attachments (id, expense_id, file_name, file_path, file_type, uploaded_by)
			VALUES (?, ?, ?, ?, ?, ?)`,
			attID, expenseID, cleanName, dest, strings.TrimSpace(fileType), sess.Username,
		); err != nil {
			return err
		}
		return s.audit.RecordTableAuditTx(tx, "expense_attachments", attID, "INSERT", sess.UserID, nil, cleanName, nil)
	})
	if err != nil {
		_ = os.Remove(dest)
		return models.ExpenseAttachment{}, err
	}
	return models.ExpenseAttachment{
		ID:         attID,
		ExpenseID:  expenseID,
		FileName:   cleanName,
		FilePath:   dest,
		FileType:   strings.TrimSpace(fileType),
		UploadedBy: sess.Username,
	}, nil
}

func (s *Service) DeleteAttachment(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	var filePath string
	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if err := tx.QueryRow(`SELECT file_path FROM expense_attachments WHERE id = ?`, id).Scan(&filePath); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM expense_attachments WHERE id = ?`, id); err != nil {
			return err
		}
		return s.audit.RecordTableAuditTx(tx, "expense_attachments", id, "DELETE", sess.UserID, nil, nil, nil)
	})
	if err != nil {
		return err
	}
	if filePath != "" {
		_ = os.Remove(filePath)
	}
	return nil
}

// --- Bulk CSV Import ---

// DownloadBulkTemplateCSV returns the 12-column expense bulk CSV template supporting per-row shipment allocation.
func (s *Service) DownloadBulkTemplateCSV() (string, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return "", err
	}
	headers := []string{
		"Shipment Invoice No", "Expense Type", "Service Provider", "Invoice No", "Invoice Date",
		"Amount", "CGST Amount", "SGST Amount", "IGST Amount", "TDS Amount", "Total Amount", "Remarks",
	}
	sample1 := []string{
		"INV-2026-001", "Custom Clearance", "DHL Express India", "DHL-2026-001", "2026-04-15",
		"15000.00", "1350.00", "1350.00", "0.00", "300.00", "17700.00", "Bill of entry clearance charges",
	}
	sample2 := []string{
		"INV-2026-002", "Freight Charges", "Maersk Logistics", "MSK-9901", "2026-04-16",
		"25000.00", "0.00", "0.00", "4500.00", "500.00", "29500.00", "Ocean freight charges",
	}
	return validation.EncodeCSV(headers, [][]string{sample1, sample2})
}

// ImportBulkCSV parses RFC 4180 CSV rows, resolves Expense Type & Service Provider case-insensitively,
// resolves Shipment by Invoice Number or ID (with fallback to default shipmentID), derives integer paise + basis points,
// groups lines by (TargetShipmentID, ServiceProviderID, InvoiceNo), and creates atomic multi-line expense invoices.
func (s *Service) ImportBulkCSV(shipmentID, content string) (models.CSVImportResult, error) {
	if _, err := s.sec.RequirePermission(security.PermDataWrite); err != nil {
		return models.CSVImportResult{}, err
	}
	headers, rows, err := validation.ParseCSV(content, true)
	if err != nil {
		return models.CSVImportResult{}, err
	}
	idx := validation.HeaderIndex(headers)

	// Fetch active shipments for resolving per-row shipments and checking frozen status.
	shipmentRows, err := s.db.Query(`
		SELECT id, invoice_number, COALESCE(is_frozen, 0)
		FROM shipments
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return models.CSVImportResult{}, err
	}
	defer shipmentRows.Close()

	type shipmentMeta struct {
		id       string
		invNo    string
		isFrozen bool
	}
	shipmentsByInvNo := make(map[string]shipmentMeta)
	shipmentsByID := make(map[string]shipmentMeta)
	for shipmentRows.Next() {
		var sm shipmentMeta
		if err := shipmentRows.Scan(&sm.id, &sm.invNo, &sm.isFrozen); err != nil {
			return models.CSVImportResult{}, err
		}
		shipmentsByInvNo[strings.ToLower(strings.TrimSpace(sm.invNo))] = sm
		shipmentsByID[strings.ToLower(strings.TrimSpace(sm.id))] = sm
	}
	if err := shipmentRows.Err(); err != nil {
		return models.CSVImportResult{}, err
	}

	providers, err := s.ListServiceProviders("")
	if err != nil {
		return models.CSVImportResult{}, err
	}
	spByName := make(map[string]models.ServiceProvider, len(providers))
	for _, p := range providers {
		spByName[strings.ToLower(strings.TrimSpace(p.Name))] = p
	}

	types, err := s.ListExpenseTypes(false)
	if err != nil {
		return models.CSVImportResult{}, err
	}
	etByName := make(map[string]models.ExpenseType, len(types))
	for _, et := range types {
		etByName[strings.ToLower(strings.TrimSpace(et.Name))] = et
	}

	type groupedInvoice struct {
		shipmentID string
		spID       string
		invNo      string
		invDate    string
		remarks    string
		lines      []models.ExpenseLineInput
		rowNums    []int
	}
	groups := make(map[string]*groupedInvoice)
	var groupOrder []string

	res := models.CSVImportResult{}
	for i, row := range rows {
		rowNum := i + 2
		shpRef := validation.CellByAliases(row, idx, "Shipment Invoice No", "Shipment No", "Shipment", "Shipment ID", "shipment_invoice_no", "shipment_no")
		etName := validation.CellByAliases(row, idx, "Expense Type", "expense_type")
		spName := validation.CellByAliases(row, idx, "Service Provider", "service_provider")
		invNo := validation.CellByAliases(row, idx, "Invoice No", "invoice_no", "invoiceNumber")
		invDateRaw := validation.CellByAliases(row, idx, "Invoice Date", "invoice_date")
		amtStr := validation.CellByAliases(row, idx, "Amount", "amount")
		cgstStr := validation.CellByAliases(row, idx, "CGST Amount", "cgst_amount")
		sgstStr := validation.CellByAliases(row, idx, "SGST Amount", "sgst_amount")
		igstStr := validation.CellByAliases(row, idx, "IGST Amount", "igst_amount")
		tdsStr := validation.CellByAliases(row, idx, "TDS Amount", "tds_amount")
		remarks := validation.CellByAliases(row, idx, "Remarks", "remarks")

		// 1. Resolve target shipment for this row
		var rowShipmentID string
		if strings.TrimSpace(shpRef) != "" {
			refKey := strings.ToLower(strings.TrimSpace(shpRef))
			if sm, ok := shipmentsByInvNo[refKey]; ok {
				if sm.isFrozen {
					res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("shipment %q is frozen and cannot accept new expense entries", shpRef)})
					res.SkippedRows++
					continue
				}
				rowShipmentID = sm.id
			} else if sm, ok := shipmentsByID[refKey]; ok {
				if sm.isFrozen {
					res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("shipment %q is frozen and cannot accept new expense entries", shpRef)})
					res.SkippedRows++
					continue
				}
				rowShipmentID = sm.id
			} else {
				res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("unknown Shipment %q", shpRef)})
				res.SkippedRows++
				continue
			}
		} else if strings.TrimSpace(shipmentID) != "" {
			defKey := strings.ToLower(strings.TrimSpace(shipmentID))
			if sm, ok := shipmentsByID[defKey]; ok {
				if sm.isFrozen {
					res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("target shipment %q is frozen and cannot accept new expense entries", sm.invNo)})
					res.SkippedRows++
					continue
				}
				rowShipmentID = sm.id
			} else if sm, ok := shipmentsByInvNo[defKey]; ok {
				if sm.isFrozen {
					res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("target shipment %q is frozen and cannot accept new expense entries", sm.invNo)})
					res.SkippedRows++
					continue
				}
				rowShipmentID = sm.id
			} else {
				res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("target shipment %q not found", shipmentID)})
				res.SkippedRows++
				continue
			}
		} else {
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: "Shipment Invoice No is required when no default target shipment is selected"})
			res.SkippedRows++
			continue
		}

		et, ok := etByName[strings.ToLower(strings.TrimSpace(etName))]
		if !ok {
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("unknown Expense Type %q", etName)})
			res.SkippedRows++
			continue
		}
		sp, ok := spByName[strings.ToLower(strings.TrimSpace(spName))]
		if !ok {
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("unknown Service Provider %q", spName)})
			res.SkippedRows++
			continue
		}
		if strings.TrimSpace(invNo) == "" {
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: "Invoice No is required"})
			res.SkippedRows++
			continue
		}
		normDate, err := validation.NormalizeDate(invDateRaw)
		if err != nil || normDate == "" {
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("invalid Invoice Date %q", invDateRaw)})
			res.SkippedRows++
			continue
		}
		amtINR, err := strconv.ParseFloat(amtStr, 64)
		if err != nil || amtINR <= 0 {
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: fmt.Sprintf("invalid positive Amount %q", amtStr)})
			res.SkippedRows++
			continue
		}
		amtPaise := int64(math.Round(amtINR * 100))

		cgstBP := et.DefaultCGSTRateBP
		sgstBP := et.DefaultSGSTRateBP
		igstBP := et.DefaultIGSTRateBP
		tdsBP := et.DefaultTDSRateBP
		var taxParseErr error
		if cgstStr != "" {
			cgstBP, taxParseErr = taxAmountToBP(cgstStr, amtPaise)
		}
		if taxParseErr == nil && sgstStr != "" {
			sgstBP, taxParseErr = taxAmountToBP(sgstStr, amtPaise)
		}
		if taxParseErr == nil && igstStr != "" {
			igstBP, taxParseErr = taxAmountToBP(igstStr, amtPaise)
		}
		if taxParseErr == nil && tdsStr != "" {
			tdsBP, taxParseErr = taxAmountToBP(tdsStr, amtPaise)
		}
		if taxParseErr != nil {
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: taxParseErr.Error()})
			res.SkippedRows++
			continue
		}

		gKey := rowShipmentID + "|" + sp.ID + "|" + strings.ToLower(strings.TrimSpace(invNo))
		g, exists := groups[gKey]
		if !exists {
			g = &groupedInvoice{
				shipmentID: rowShipmentID,
				spID:       sp.ID,
				invNo:      strings.TrimSpace(invNo),
				invDate:    normDate,
				remarks:    remarks,
			}
			groups[gKey] = g
			groupOrder = append(groupOrder, gKey)
		}
		g.lines = append(g.lines, models.ExpenseLineInput{
			ExpenseTypeID: et.ID,
			AmountPaise:   amtPaise,
			CGSTRateBP:    cgstBP,
			SGSTRateBP:    sgstBP,
			IGSTRateBP:    igstBP,
			TDSRateBP:     tdsBP,
			Remarks:       remarks,
		})
		g.rowNums = append(g.rowNums, rowNum)
	}

	for _, gKey := range groupOrder {
		g := groups[gKey]
		_, err := s.CreateExpenseInvoice(models.ExpenseInvoiceRequest{
			ShipmentID:        g.shipmentID,
			ServiceProviderID: g.spID,
			InvoiceNo:         g.invNo,
			InvoiceDate:       g.invDate,
			Currency:          "INR",
			Remarks:           g.remarks,
			MergeDuplicates:   true,
			Lines:             g.lines,
		})
		if err != nil {
			for _, rn := range g.rowNums {
				res.Errors = append(res.Errors, models.CSVImportRowError{Row: rn, Message: err.Error()})
				res.SkippedRows++
			}
			continue
		}
		res.ImportedRows += len(g.lines)
	}
	res.TotalRows = len(rows)
	return res, nil
}

func taxAmountToBP(raw string, basePaise int64) (int64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid non-negative tax/TDS amount %q", raw)
	}
	if basePaise <= 0 {
		return 0, nil
	}
	taxPaise := math.Round(v * 100)
	bp := int64(math.Round((taxPaise * 10000.0) / float64(basePaise)))
	if bp > 10000 {
		return 0, fmt.Errorf("tax amount %q exceeds 100%% of base amount", raw)
	}
	return bp, nil
}
