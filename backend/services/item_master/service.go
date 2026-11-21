package item_master

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/validation"
)

var itemExportHeaders = []string{
	"id",
	"partNumber",
	"itemDescription",
	"unit",
	"currency",
	"unitPrice",
	"hsnCode",
	"supplierId",
	"supplierName",
	"isActive",
	"countryOfOrigin",
	"bcd",
	"sws",
	"igst",
	"technicalWriteUp",
	"category",
	"endUse",
	"netWeightKg",
	"purchaseUom",
	"grossWeightPerUomKg",
}

// Service implements Item Master CRUD, photo attachment safety, and 20-column CSV import/export.
type Service struct {
	db             *sql.DB
	repo           *repositories.ItemRepository
	sec            *security.Service
	audit          *audit.Service
	attachmentsDir string
}

// NewService constructs an item master Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service, attachmentsDir string) *Service {
	return &Service{
		db:             db,
		repo:           repositories.NewItemRepository(db),
		sec:            sec,
		audit:          auditSvc,
		attachmentsDir: attachmentsDir,
	}
}

// SetDB rebinds the service and its repositories to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
	s.repo = repositories.NewItemRepository(db)
}

// List returns filtered, sorted, paginated item master records.
func (s *Service) List(req models.ItemPageRequest) (models.ItemPage, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.ItemPage{}, err
	}
	return s.repo.List(req)
}

// GetByID fetches an active item record by ID.
func (s *Service) GetByID(id string) (models.Item, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.Item{}, err
	}
	return s.repo.GetByID(id)
}

// CheckPartNumber checks whether partNumber already exists on another active item.
func (s *Service) CheckPartNumber(partNumber, excludeID string) (*models.Item, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.repo.FindByPartNumber(partNumber, excludeID)
}

// Create validates input, blocks duplicate part numbers, assigns ITM-XXX ID, and inserts into DB.
func (s *Service) Create(in models.ItemInput) (models.Item, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Item{}, err
	}
	if err := s.normalizeAndValidate(&in); err != nil {
		return models.Item{}, err
	}
	if existing, err := s.repo.FindByPartNumber(in.PartNumber, ""); err != nil {
		return models.Item{}, err
	} else if existing != nil {
		return models.Item{}, fmt.Errorf("part number %q already exists (%s)", existing.PartNumber, existing.ID)
	}

	created, err := s.repo.Create(in)
	if err != nil {
		return models.Item{}, err
	}
	_ = s.audit.RecordTableAudit("items", created.ID, "INSERT", sess.UserID, nil, created, nil)
	return created, nil
}

// Update validates changes, prevents duplicate part numbers, and persists updates.
func (s *Service) Update(id string, in models.ItemInput) (models.Item, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Item{}, err
	}
	before, err := s.repo.GetByID(id)
	if err != nil {
		return models.Item{}, err
	}
	if err := s.normalizeAndValidate(&in); err != nil {
		return models.Item{}, err
	}
	if existing, err := s.repo.FindByPartNumber(in.PartNumber, id); err != nil {
		return models.Item{}, err
	} else if existing != nil {
		return models.Item{}, fmt.Errorf("part number %q already belongs to item %s", existing.PartNumber, existing.ID)
	}

	updated, err := s.repo.Update(id, in)
	if err != nil {
		return models.Item{}, err
	}
	_ = s.audit.RecordTableAudit("items", updated.ID, "UPDATE", sess.UserID, before, updated, nil)
	return updated, nil
}

// Delete soft-deletes an item into the Recycle Bin after verifying no active dependencies exist.
func (s *Service) Delete(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	before, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if depErr, err := integrity.CheckDependencies(context.Background(), s.db, "items", id, true); err != nil {
		return err
	} else if depErr != nil {
		return depErr
	}
	if err := s.repo.SoftDelete(id, sess.UserID); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("items", id, "DELETE", sess.UserID, before, nil, map[string]any{"softDelete": true})
	return nil
}

// SaveItemPhoto stores image bytes inside the sandboxed attachmentsDir/items directory and returns the safe filename.
func (s *Service) SaveItemPhoto(itemID, filename string, data []byte) (string, error) {
	if _, err := s.sec.RequirePermission(security.PermDataWrite); err != nil {
		return "", err
	}
	if s.attachmentsDir == "" {
		return "", errors.New("attachments directory is not configured")
	}
	if len(data) == 0 || len(data) > 5*1024*1024 {
		return "", errors.New("photo size must be between 1 byte and 5 MB")
	}
	ext := strings.ToLower(filepath.Ext(filepath.Base(filename)))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return "", fmt.Errorf("unsupported photo extension %q", ext)
	}

	safeID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, strings.TrimSpace(itemID))
	if safeID == "" {
		safeID = "item"
	}

	dir := filepath.Join(s.attachmentsDir, "items")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	rel := filepath.Join("items", safeID+ext)
	fullPath := filepath.Join(s.attachmentsDir, rel)
	if err := os.WriteFile(fullPath, data, 0o600); err != nil {
		return "", err
	}
	return rel, nil
}

// DownloadTemplateCSV returns a 20-column Item Master CSV template.
func (s *Service) DownloadTemplateCSV() (string, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return "", err
	}
	return validation.EncodeCSV(itemExportHeaders, [][]string{{
		"",
		"PN-1001",
		"Precision Control Module",
		"PCS",
		"USD",
		"125.50",
		"85371000",
		"Sup-001",
		"Supplier Alpha",
		"true",
		"Germany",
		"7.5",
		"10",
		"18",
		"Industrial PLC sub-assembly",
		"Electronics",
		"Manufacturing",
		"1.25",
		"PCS",
		"1.40",
	}})
}

// ExportCSV exports all non-deleted items in the 20-column canonical format.
func (s *Service) ExportCSV() (string, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return "", err
	}
	all, err := s.repo.AllForExport()
	if err != nil {
		return "", err
	}
	rows := make([][]string, 0, len(all))
	for _, it := range all {
		rows = append(rows, []string{
			it.ID,
			it.PartNumber,
			it.ItemDescription,
			it.Unit,
			it.Currency,
			fmt.Sprintf("%.4f", it.UnitPrice),
			it.HSNCode,
			it.SupplierID,
			it.SupplierName,
			fmt.Sprintf("%t", it.IsActive),
			it.CountryOfOrigin,
			it.BCD,
			it.SWS,
			it.IGST,
			it.TechnicalWriteUp,
			it.Category,
			it.EndUse,
			fmt.Sprintf("%.4f", it.NetWeightKg),
			it.PurchaseUOM,
			fmt.Sprintf("%.4f", it.GrossWeightPerUOMKg),
		})
	}
	return validation.EncodeCSV(itemExportHeaders, rows)
}

// ImportCSV parses Item Master CSV, enforces [partNumber, itemDescription, unit, currency, unitPrice],
// prevents intra-batch and database partNumber duplicates, and resolves supplierName to supplierId when omitted.
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
	seenParts := make(map[string]int, len(rows))

	for i, row := range rows {
		rowNum := i + 2
		partNumber := validation.CellByAliases(row, idx, "partNumber", "part_number", "partNo")
		desc := validation.CellByAliases(row, idx, "itemDescription", "item_description", "description")
		unit := validation.CellByAliases(row, idx, "unit")
		currency := validation.CellByAliases(row, idx, "currency")
		priceStr := validation.CellByAliases(row, idx, "unitPrice", "unit_price")

		if partNumber == "" || desc == "" || unit == "" || currency == "" || priceStr == "" {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: "partNumber, itemDescription, unit, currency, and unitPrice are required",
			})
			continue
		}

		unitPrice, err := strconv.ParseFloat(strings.ReplaceAll(priceStr, ",", ""), 64)
		if err != nil || unitPrice < 0 {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("invalid unitPrice %q", priceStr),
			})
			continue
		}

		lowerPart := strings.ToLower(partNumber)
		if firstRow, dup := seenParts[lowerPart]; dup {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("duplicate partNumber %q in CSV (first seen row %d)", partNumber, firstRow),
			})
			continue
		}
		if existing, err := s.repo.FindByPartNumber(partNumber, ""); err != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: err.Error()})
			continue
		} else if existing != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("duplicate partNumber %q already exists (%s)", partNumber, existing.ID),
			})
			continue
		}

		supplierID := validation.CellByAliases(row, idx, "supplierId", "supplier_id")
		supplierName := validation.CellByAliases(row, idx, "supplierName", "supplier_name")
		if supplierID == "" && supplierName != "" {
			resolved, err := s.repo.ResolveSupplierIDByName(supplierName)
			if err != nil {
				res.Warnings = append(res.Warnings, models.CSVImportRowError{Row: rowNum, Message: err.Error()})
			} else if resolved == "" {
				res.Warnings = append(res.Warnings, models.CSVImportRowError{
					Row:     rowNum,
					Message: fmt.Sprintf("supplierName %q did not match any active supplier; left unlinked", supplierName),
				})
			} else {
				supplierID = resolved
			}
		}

		activeStr := strings.ToLower(validation.CellByAliases(row, idx, "isActive", "is_active"))
		isActive := activeStr != "false" && activeStr != "0"

		netW, _ := strconv.ParseFloat(validation.CellByAliases(row, idx, "netWeightKg", "net_weight_kg"), 64)
		grossW, _ := strconv.ParseFloat(validation.CellByAliases(row, idx, "grossWeightPerUomKg", "gross_weight_per_uom_kg"), 64)

		in := models.ItemInput{
			PartNumber:          partNumber,
			ItemDescription:     desc,
			Unit:                unit,
			Currency:            strings.ToUpper(currency),
			UnitPrice:           unitPrice,
			HSNCode:             validation.CellByAliases(row, idx, "hsnCode", "hsn_code"),
			SupplierID:          supplierID,
			IsActive:            isActive,
			CountryOfOrigin:     validation.CellByAliases(row, idx, "countryOfOrigin", "country_of_origin"),
			BCD:                 validation.CellByAliases(row, idx, "bcd"),
			SWS:                 validation.CellByAliases(row, idx, "sws"),
			IGST:                validation.CellByAliases(row, idx, "igst"),
			TechnicalWriteUp:    validation.CellByAliases(row, idx, "technicalWriteUp", "technical_write_up"),
			Category:            validation.CellByAliases(row, idx, "category"),
			EndUse:              validation.CellByAliases(row, idx, "endUse", "end_use"),
			NetWeightKg:         netW,
			PurchaseUOM:         validation.CellByAliases(row, idx, "purchaseUom", "purchase_uom"),
			GrossWeightPerUOMKg: grossW,
		}
		created, err := s.repo.Create(in)
		if err != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: err.Error()})
			continue
		}
		seenParts[lowerPart] = rowNum
		res.ImportedRows++
		_ = s.audit.RecordTableAudit("items", created.ID, "INSERT", sess.UserID, nil, created, map[string]any{"source": "csv_import"})
	}

	_ = s.audit.RecordUserActivity(sess.UserID, "item.import_csv", "item", "", "SUCCESS", "INFO", map[string]any{
		"totalRows":    res.TotalRows,
		"importedRows": res.ImportedRows,
		"skippedRows":  res.SkippedRows,
	})
	return res, nil
}

func (s *Service) normalizeAndValidate(in *models.ItemInput) error {
	in.PartNumber = strings.TrimSpace(in.PartNumber)
	in.ItemDescription = strings.TrimSpace(in.ItemDescription)
	in.Unit = strings.TrimSpace(in.Unit)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.HSNCode = strings.TrimSpace(in.HSNCode)
	in.SupplierID = strings.TrimSpace(in.SupplierID)

	if in.PartNumber == "" {
		return errors.New("partNumber is required")
	}
	if in.ItemDescription == "" {
		return errors.New("itemDescription is required")
	}
	if in.Unit == "" {
		return errors.New("unit is required")
	}
	if in.Currency == "" {
		return errors.New("currency is required")
	}
	if in.UnitPrice < 0 {
		return errors.New("unitPrice cannot be negative")
	}
	if in.SupplierID == "" && strings.TrimSpace(in.SupplierName) != "" {
		resolved, err := s.repo.ResolveSupplierIDByName(in.SupplierName)
		if err != nil {
			return err
		}
		in.SupplierID = resolved
	}
	return nil
}
