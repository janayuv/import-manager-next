package supplier

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/validation"
)

var supplierTemplateHeaders = []string{
	"supplierName",
	"shortName",
	"country",
	"email",
	"phone",
	"beneficiaryName",
	"bankName",
	"branch",
	"bankAddress",
	"accountNo",
	"iban",
	"swiftCode",
	"isActive",
}

// Service implements Supplier CRUD, duplicate detection, history queries, and RFC 4180 CSV import/export.
type Service struct {
	db    *sql.DB
	repo  *repositories.SupplierRepository
	sec   *security.Service
	audit *audit.Service
}

// NewService constructs a supplier Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{
		db:    db,
		repo:  repositories.NewSupplierRepository(db),
		sec:   sec,
		audit: auditSvc,
	}
}

// SetDB rebinds the service and its repositories to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
	s.repo = repositories.NewSupplierRepository(db)
}

// List returns paginated, filtered, sorted active suppliers.
func (s *Service) List(req models.SupplierPageRequest) (models.SupplierPage, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.SupplierPage{}, err
	}
	return s.repo.List(req)
}

// GetByID fetches a single non-deleted supplier.
func (s *Service) GetByID(id string) (models.Supplier, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.Supplier{}, err
	}
	return s.repo.GetByID(id)
}

// CheckDuplicate checks whether an active supplier already uses the supplied name or email.
func (s *Service) CheckDuplicate(name, email, excludeID string) (*models.Supplier, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.repo.FindDuplicate(name, email, excludeID)
}

// GetHistory summarizes shipments and invoices associated with supplierID.
func (s *Service) GetHistory(supplierID string) (models.SupplierHistorySummary, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.SupplierHistorySummary{}, err
	}
	return s.repo.GetHistory(supplierID)
}

// Create validates and creates a new supplier record with audit logging.
func (s *Service) Create(in models.SupplierInput) (models.Supplier, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Supplier{}, err
	}
	if err := validateSupplierInput(&in); err != nil {
		return models.Supplier{}, err
	}
	if dup, err := s.repo.FindDuplicate(in.SupplierName, in.Email, ""); err != nil {
		return models.Supplier{}, err
	} else if dup != nil {
		return models.Supplier{}, fmt.Errorf("duplicate supplier detected: matches %s (%s)", dup.SupplierName, dup.ID)
	}

	created, err := s.repo.Create(in)
	if err != nil {
		return models.Supplier{}, err
	}
	_ = s.audit.RecordTableAudit("suppliers", created.ID, "INSERT", sess.UserID, nil, created, nil)
	_ = s.audit.RecordUserActivity(sess.UserID, "supplier.create", "supplier", created.ID, "SUCCESS", "INFO", map[string]any{
		"supplierName": created.SupplierName,
		"country":      created.Country,
	})
	return created, nil
}

// Update validates and updates an existing supplier record.
func (s *Service) Update(id string, in models.SupplierInput) (models.Supplier, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Supplier{}, err
	}
	if err := validateSupplierInput(&in); err != nil {
		return models.Supplier{}, err
	}
	before, err := s.repo.GetByID(id)
	if err != nil {
		return models.Supplier{}, err
	}
	if dup, err := s.repo.FindDuplicate(in.SupplierName, in.Email, id); err != nil {
		return models.Supplier{}, err
	} else if dup != nil {
		return models.Supplier{}, fmt.Errorf("duplicate supplier detected: matches %s (%s)", dup.SupplierName, dup.ID)
	}

	updated, err := s.repo.Update(id, in)
	if err != nil {
		return models.Supplier{}, err
	}
	_ = s.audit.RecordTableAudit("suppliers", updated.ID, "UPDATE", sess.UserID, before, updated, nil)
	return updated, nil
}

// Delete soft-deletes a supplier into the Recycle Bin after verifying no active dependencies exist.
func (s *Service) Delete(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	before, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if depErr, err := integrity.CheckDependencies(context.Background(), s.db, "suppliers", id, true); err != nil {
		return err
	} else if depErr != nil {
		return depErr
	}
	if err := s.repo.SoftDelete(id, sess.UserID); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("suppliers", id, "DELETE", sess.UserID, before, nil, map[string]any{"softDelete": true})
	_ = s.audit.RecordUserActivity(sess.UserID, "supplier.delete", "supplier", id, "SUCCESS", "INFO", map[string]any{
		"supplierName": before.SupplierName,
	})
	return nil
}

// DownloadTemplateCSV returns an RFC 4180 BOM-prefixed supplier CSV template (13 columns).
func (s *Service) DownloadTemplateCSV() (string, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return "", err
	}
	return validation.EncodeCSV(supplierTemplateHeaders, [][]string{{
		"Acme Electronics Ltd",
		"ACME",
		"China",
		"sales@acme-elec.example",
		"+86-755-1234567",
		"Acme Electronics Ltd",
		"Bank of China",
		"Shenzhen Main",
		"123 Trade Rd, Shenzhen",
		"100200300400",
		"",
		"BKCHCNBJ45A",
		"true",
	}})
}

// ExportCSV exports all active suppliers in RFC 4180 CSV format.
func (s *Service) ExportCSV() (string, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return "", err
	}
	all, err := s.repo.AllForExport()
	if err != nil {
		return "", err
	}
	headers := append([]string{"id"}, supplierTemplateHeaders...)
	rows := make([][]string, 0, len(all))
	for _, sup := range all {
		rows = append(rows, []string{
			sup.ID,
			sup.SupplierName,
			sup.ShortName,
			sup.Country,
			sup.Email,
			sup.Phone,
			sup.BeneficiaryName,
			sup.BankName,
			sup.Branch,
			sup.BankAddress,
			sup.AccountNo,
			sup.IBAN,
			sup.SwiftCode,
			fmt.Sprintf("%t", sup.IsActive),
		})
	}
	return validation.EncodeCSV(headers, rows)
}

// ImportCSV parses RFC 4180 CSV content, validates required columns [supplierName, country],
// blocks intra-batch & DB duplicates, and inserts valid rows with audit tracking.
func (s *Service) ImportCSV(csvContent string) (models.CSVImportResult, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.CSVImportResult{}, err
	}
	headers, rows, err := validation.ParseCSV(csvContent, false)
	if err != nil {
		return models.CSVImportResult{}, err
	}
	idx := validation.HeaderIndex(headers)

	res := models.CSVImportResult{TotalRows: len(rows)}
	seenNames := make(map[string]int, len(rows))

	for i, row := range rows {
		rowNum := i + 2
		name := validation.CellByAliases(row, idx, "supplierName", "supplier_name", "name")
		country := validation.CellByAliases(row, idx, "country")
		if name == "" || country == "" {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: "supplierName and country are required",
			})
			continue
		}

		lowerName := strings.ToLower(name)
		if prevRow, dupBatch := seenNames[lowerName]; dupBatch {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("duplicate supplierName %q inside CSV (first seen at row %d)", name, prevRow),
			})
			continue
		}

		email := validation.CellByAliases(row, idx, "email")
		if dup, err := s.repo.FindDuplicate(name, email, ""); err != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: err.Error()})
			continue
		} else if dup != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("duplicate supplier %q already exists (%s)", dup.SupplierName, dup.ID),
			})
			continue
		}

		activeStr := strings.ToLower(validation.CellByAliases(row, idx, "isActive", "is_active"))
		isActive := activeStr != "false" && activeStr != "0"

		in := models.SupplierInput{
			SupplierName:    name,
			ShortName:       validation.CellByAliases(row, idx, "shortName", "short_name"),
			Country:         country,
			Email:           email,
			Phone:           validation.CellByAliases(row, idx, "phone"),
			BeneficiaryName: validation.CellByAliases(row, idx, "beneficiaryName", "beneficiary_name"),
			BankName:        validation.CellByAliases(row, idx, "bankName", "bank_name"),
			Branch:          validation.CellByAliases(row, idx, "branch"),
			BankAddress:     validation.CellByAliases(row, idx, "bankAddress", "bank_address"),
			AccountNo:       validation.CellByAliases(row, idx, "accountNo", "account_no"),
			IBAN:            validation.CellByAliases(row, idx, "iban"),
			SwiftCode:       validation.CellByAliases(row, idx, "swiftCode", "swift_code"),
			IsActive:        isActive,
		}
		if err := validateSupplierInput(&in); err != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: err.Error()})
			continue
		}
		created, err := s.repo.Create(in)
		if err != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: err.Error()})
			continue
		}
		seenNames[lowerName] = rowNum
		res.ImportedRows++
		_ = s.audit.RecordTableAudit("suppliers", created.ID, "INSERT", sess.UserID, nil, created, map[string]any{"source": "csv_import"})
	}

	_ = s.audit.RecordUserActivity(sess.UserID, "supplier.import_csv", "supplier", "", "SUCCESS", "INFO", map[string]any{
		"totalRows":    res.TotalRows,
		"importedRows": res.ImportedRows,
		"skippedRows":  res.SkippedRows,
	})
	return res, nil
}

func validateSupplierInput(in *models.SupplierInput) error {
	in.SupplierName = strings.TrimSpace(in.SupplierName)
	in.Country = strings.TrimSpace(in.Country)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = strings.TrimSpace(in.Phone)

	if in.SupplierName == "" {
		return errors.New("supplierName is required")
	}
	if in.Country == "" {
		return errors.New("country is required")
	}
	if in.Email != "" && !strings.Contains(in.Email, "@") {
		return fmt.Errorf("invalid email address %q", in.Email)
	}
	return nil
}
