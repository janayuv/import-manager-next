package shipment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/integrity"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/validation"
)

var allowedStatuses = map[string]string{
	"docs-rcvd":           "docs-rcvd",
	"docu-received":       "docs-rcvd",
	"in-transit":          "in-transit",
	"customs-clearance":   "customs-clearance",
	"ready-dly":           "ready-dly",
	"in-warehouse":        "in-warehouse",
	"partially-delivered": "partially-delivered",
	"delivered":           "delivered",
	"cancelled":           "cancelled",
	"finalized":           "finalized",
	"closed":              "closed",
	"completed":           "completed",
}

var shipmentTemplateHeaders = []string{
	"supplierId",
	"supplierName",
	"invoiceNumber",
	"invoiceDate",
	"goodsCategory",
	"invoiceValue",
	"invoiceCurrency",
	"incoterm",
	"shipmentMode",
	"shipmentType",
	"blAwbNumber",
	"blAwbDate",
	"vesselName",
	"containerNumber",
	"grossWeightKg",
	"etd",
	"eta",
	"status",
	"dateOfDelivery",
}

// Service implements Shipment lifecycle, multi-container tracking, freeze workflow, readiness automation, and CSV stream import.
type Service struct {
	db            *sql.DB
	repo          *repositories.ShipmentRepository
	containerRepo *repositories.ContainerRepository
	itemRepo      *repositories.ItemRepository
	supRepo       *repositories.SupplierRepository
	sec           *security.Service
	audit         *audit.Service
}

// NewService constructs a shipment Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{
		db:            db,
		repo:          repositories.NewShipmentRepository(db),
		containerRepo: repositories.NewContainerRepository(db),
		itemRepo:      repositories.NewItemRepository(db),
		supRepo:       repositories.NewSupplierRepository(db),
		sec:           sec,
		audit:         auditSvc,
	}
}

// SetDB rebinds the service and its repositories to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
	s.repo = repositories.NewShipmentRepository(db)
	s.containerRepo = repositories.NewContainerRepository(db)
	s.itemRepo = repositories.NewItemRepository(db)
	s.supRepo = repositories.NewSupplierRepository(db)
}

// List returns filtered, sorted, paginated active shipments.
func (s *Service) List(req models.ShipmentPageRequest) (models.ShipmentPage, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.ShipmentPage{}, err
	}
	return s.repo.List(req)
}

// GetByID fetches a single shipment by ID.
func (s *Service) GetByID(id string) (models.Shipment, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.Shipment{}, err
	}
	return s.repo.GetByID(id)
}

// GetDetail returns shipment details plus linked invoices, BOEs, and expenses.
func (s *Service) GetDetail(id string) (models.ShipmentDetail, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.ShipmentDetail{}, err
	}
	return s.repo.GetDetail(id)
}

// CheckDuplicate checks whether an active shipment already uses the supplierId and invoiceNumber.
func (s *Service) CheckDuplicate(supplierID, invoiceNumber, excludeID string) (*models.Shipment, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.repo.FindDuplicate(supplierID, invoiceNumber, excludeID)
}

// Create validates and inserts a new shipment record.
func (s *Service) Create(in models.ShipmentInput) (models.Shipment, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Shipment{}, err
	}
	if err := s.normalizeAndValidate(&in); err != nil {
		return models.Shipment{}, err
	}
	if dup, err := s.repo.FindDuplicate(in.SupplierID, in.InvoiceNumber, ""); err != nil {
		return models.Shipment{}, err
	} else if dup != nil {
		return models.Shipment{}, fmt.Errorf("duplicate shipment detected: invoice %q already exists for supplier %s (%s)", in.InvoiceNumber, dup.SupplierName, dup.ID)
	}
	created, err := s.repo.Create(in)
	if err != nil {
		return models.Shipment{}, err
	}
	_ = s.audit.RecordTableAudit("shipments", created.ID, "INSERT", sess.UserID, nil, created, nil)
	return created, nil
}

// Update modifies a non-frozen shipment.
func (s *Service) Update(id string, in models.ShipmentInput) (models.Shipment, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Shipment{}, err
	}
	before, err := s.repo.GetByID(id)
	if err != nil {
		return models.Shipment{}, err
	}
	if before.IsFrozen {
		return models.Shipment{}, fmt.Errorf("shipment %s is frozen and cannot be edited", before.ID)
	}
	if err := s.normalizeAndValidate(&in); err != nil {
		return models.Shipment{}, err
	}
	if dup, err := s.repo.FindDuplicate(in.SupplierID, in.InvoiceNumber, id); err != nil {
		return models.Shipment{}, err
	} else if dup != nil {
		return models.Shipment{}, fmt.Errorf("duplicate shipment detected: invoice %q already belongs to shipment %s (%s)", in.InvoiceNumber, dup.ID, dup.SupplierName)
	}
	updated, err := s.repo.Update(id, in)
	if err != nil {
		return models.Shipment{}, err
	}
	_ = s.audit.RecordTableAudit("shipments", updated.ID, "UPDATE", sess.UserID, before, updated, nil)
	return updated, nil
}

// Freeze places a terminal commercial lock on a shipment.
func (s *Service) Freeze(id string) (models.Shipment, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Shipment{}, err
	}
	updated, err := s.repo.SetFrozen(id, true)
	if err != nil {
		return models.Shipment{}, err
	}
	_ = s.audit.RecordUserActivity(sess.UserID, "shipment.freeze", "shipment", id, "SUCCESS", "INFO", map[string]any{"isFrozen": true})
	return updated, nil
}

// Unfreeze lifts the commercial freeze lock (admin/privileged role check).
func (s *Service) Unfreeze(id string) (models.Shipment, error) {
	sess, err := s.sec.RequireAnyPermission(security.PermRoleWrite, security.PermDataWrite)
	if err != nil {
		return models.Shipment{}, err
	}
	updated, err := s.repo.SetFrozen(id, false)
	if err != nil {
		return models.Shipment{}, err
	}
	_ = s.audit.RecordUserActivity(sess.UserID, "shipment.unfreeze", "shipment", id, "SUCCESS", "SECURITY", map[string]any{"isFrozen": false})
	return updated, nil
}

// Delete soft-deletes a non-frozen shipment into Recycle Bin after verifying no active dependencies exist.
func (s *Service) Delete(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	before, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if before.IsFrozen {
		return fmt.Errorf("shipment %s is frozen and cannot be deleted", before.ID)
	}
	if depErr, err := integrity.CheckDependencies(context.Background(), s.db, "shipments", id, true); err != nil {
		return err
	} else if depErr != nil {
		return depErr
	}
	if err := s.repo.SoftDelete(id, sess.UserID); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("shipments", id, "DELETE", sess.UserID, before, nil, map[string]any{"softDelete": true})
	return nil
}

// CheckReadyForDelivery runs the 7-day BOE ready-for-delivery transition check and returns rows updated.
func (s *Service) CheckReadyForDelivery(referenceDate string) (int64, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return 0, err
	}
	n, err := s.repo.UpdateReadyForDelivery(referenceDate)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		_ = s.audit.RecordUserActivity(sess.UserID, "shipment.ready_dly_sweep", "shipment", "", "SUCCESS", "INFO", map[string]any{
			"updatedCount":  n,
			"referenceDate": referenceDate,
		})
	}
	return n, nil
}

// DownloadTemplateCSV exports an 18-column BOM-prefixed CRLF CSV template.
func (s *Service) DownloadTemplateCSV() (string, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return "", err
	}
	return validation.EncodeCSV(shipmentTemplateHeaders, [][]string{{
		"Sup-001",
		"Supplier Alpha",
		"CI-2026-001",
		"2026-11-01",
		"Electronics",
		"15000.00",
		"USD",
		"CIF",
		"Sea",
		"FCL",
		"BL-998811",
		"2026-11-02",
		"MV Pacific Star",
		"CONT1234567",
		"850.5",
		"2026-11-03",
		"2026-11-20",
		"docs-rcvd",
		"",
	}})
}

// ImportCSV parses CSV/TSV/semicolon shipment batches with Excel serial date support and logs summary stats.
func (s *Service) ImportCSV(fileName, content string) (models.CSVImportResult, error) {
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
	seenInvoices := make(map[string]int, len(rows))

	for i, row := range rows {
		rowNum := i + 2
		invNo := validation.CellByAliases(row, idx, "invoiceNumber", "invoice_number", "invNo")
		invDateRaw := validation.CellByAliases(row, idx, "invoiceDate", "invoice_date", "invDate")
		invValStr := validation.CellByAliases(row, idx, "invoiceValue", "invoice_value", "amount")
		if invNo == "" || invDateRaw == "" || invValStr == "" {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: "invoiceNumber, invoiceDate, and invoiceValue are required",
			})
			continue
		}

		invVal, err := strconv.ParseFloat(strings.ReplaceAll(invValStr, ",", ""), 64)
		if err != nil || invVal < 0 {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("invalid invoiceValue %q", invValStr),
			})
			continue
		}

		invDate, err := validation.NormalizeDate(invDateRaw)
		if err != nil || invDate == "" {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("invalid invoiceDate %q", invDateRaw),
			})
			continue
		}

		supplierID := validation.CellByAliases(row, idx, "supplierId", "supplier_id")
		supplierName := validation.CellByAliases(row, idx, "supplierName", "supplier_name")
		if supplierID == "" && supplierName != "" {
			resolved, err := s.itemRepo.ResolveSupplierIDByName(supplierName)
			if err != nil || resolved == "" {
				res.SkippedRows++
				res.Errors = append(res.Errors, models.CSVImportRowError{
					Row:     rowNum,
					Message: fmt.Sprintf("supplierName %q not found", supplierName),
				})
				continue
			}
			supplierID = resolved
		}
		if supplierID == "" {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: "supplierId or matching supplierName is required",
			})
			continue
		}

		pairKey := strings.ToLower(supplierID + "::" + strings.TrimSpace(invNo))
		if firstRow, seen := seenInvoices[pairKey]; seen {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("duplicate invoice %q for supplier in CSV (first seen at row %d)", invNo, firstRow),
			})
			continue
		}

		if dup, err := s.repo.FindDuplicate(supplierID, invNo, ""); err != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{Row: rowNum, Message: err.Error()})
			continue
		} else if dup != nil {
			res.SkippedRows++
			res.Errors = append(res.Errors, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("duplicate shipment: invoice %q already exists for supplier (%s)", invNo, dup.ID),
			})
			continue
		}
		seenInvoices[pairKey] = rowNum

		grossWt, _ := strconv.ParseFloat(validation.CellByAliases(row, idx, "grossWeightKg", "gross_weight_kg"), 64)
		category := validation.CellByAliases(row, idx, "goodsCategory", "goods_category", "category")
		if category == "" {
			category = "General"
		}
		currency := validation.CellByAliases(row, idx, "invoiceCurrency", "invoice_currency", "currency")
		if currency == "" {
			currency = "USD"
		}
		incoterm := validation.CellByAliases(row, idx, "incoterm")
		if incoterm == "" {
			incoterm = "CIF"
		}

		in := models.ShipmentInput{
			SupplierID:      supplierID,
			InvoiceNumber:   invNo,
			InvoiceDate:     invDate,
			GoodsCategory:   category,
			InvoiceValue:    invVal,
			InvoiceCurrency: currency,
			Incoterm:        incoterm,
			ShipmentMode:    validation.CellByAliases(row, idx, "shipmentMode", "shipment_mode"),
			ShipmentType:    validation.CellByAliases(row, idx, "shipmentType", "shipment_type"),
			BLAWBNumber:     validation.CellByAliases(row, idx, "blAwbNumber", "bl_awb_number"),
			BLAWBDate:       validation.CellByAliases(row, idx, "blAwbDate", "bl_awb_date"),
			VesselName:      validation.CellByAliases(row, idx, "vesselName", "vessel_name"),
			ContainerNumber: validation.CellByAliases(row, idx, "containerNumber", "container_number"),
			GrossWeightKg:   grossWt,
			ETD:             validation.CellByAliases(row, idx, "etd"),
			ETA:             validation.CellByAliases(row, idx, "eta"),
			Status:          validation.CellByAliases(row, idx, "status"),
			DateOfDelivery:  validation.CellByAliases(row, idx, "dateOfDelivery", "date_of_delivery", "deliveryDate", "delivery_date"),
		}
		if err := s.normalizeAndValidate(&in); err != nil {
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
		res.ImportedRows++
		_ = s.audit.RecordTableAudit("shipments", created.ID, "INSERT", sess.UserID, nil, created, map[string]any{"source": "csv_import"})
	}

	status := "SUCCESS"
	if res.SkippedRows > 0 && res.ImportedRows == 0 {
		status = "FAILED"
	} else if res.SkippedRows > 0 {
		status = "PARTIAL"
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = "shipments-import.csv"
	}
	s.repo.RecordImportLog(fileName, res.TotalRows, res.ImportedRows, res.SkippedRows, len(res.Errors), status)
	return res, nil
}

func (s *Service) normalizeAndValidate(in *models.ShipmentInput) error {
	in.SupplierID = strings.TrimSpace(in.SupplierID)
	if in.SupplierID == "" && strings.TrimSpace(in.SupplierName) != "" {
		resolved, err := s.itemRepo.ResolveSupplierIDByName(in.SupplierName)
		if err != nil {
			return err
		}
		in.SupplierID = resolved
	}
	if in.SupplierID == "" {
		return errors.New("supplierId is required")
	}
	if _, err := s.supRepo.GetByID(in.SupplierID); err != nil {
		return fmt.Errorf("invalid supplier %q: %w", in.SupplierID, err)
	}

	in.InvoiceNumber = strings.TrimSpace(in.InvoiceNumber)
	if in.InvoiceNumber == "" {
		return errors.New("invoiceNumber is required")
	}

	normInvDate, err := validation.NormalizeDate(in.InvoiceDate)
	if err != nil || normInvDate == "" {
		return errors.New("valid invoiceDate is required")
	}
	in.InvoiceDate = normInvDate

	if in.InvoiceValue < 0 {
		return errors.New("invoiceValue cannot be negative")
	}
	in.GoodsCategory = strings.TrimSpace(in.GoodsCategory)
	if in.GoodsCategory == "" {
		in.GoodsCategory = "General"
	}
	in.InvoiceCurrency = strings.ToUpper(strings.TrimSpace(in.InvoiceCurrency))
	if in.InvoiceCurrency == "" {
		in.InvoiceCurrency = "USD"
	}
	in.Incoterm = strings.ToUpper(strings.TrimSpace(in.Incoterm))
	if in.Incoterm == "" {
		in.Incoterm = "CIF"
	}

	rawStatus := strings.ToLower(strings.TrimSpace(in.Status))
	if rawStatus == "" {
		in.Status = "docs-rcvd"
	} else if canonical, ok := allowedStatuses[rawStatus]; ok {
		in.Status = canonical
	} else {
		return fmt.Errorf("invalid shipment status %q", in.Status)
	}

	for _, ptr := range []*string{&in.BLAWBDate, &in.ETD, &in.ETA, &in.DateOfDelivery} {
		norm, dErr := validation.NormalizeDate(*ptr)
		if dErr != nil {
			return dErr
		}
		*ptr = norm
	}

	if strings.TrimSpace(in.DateOfDelivery) != "" {
		if in.Status != "cancelled" && in.Status != "closed" && in.Status != "partially-delivered" && in.Status != "in-warehouse" {
			in.Status = "delivered"
		}
	} else if in.Status == "delivered" {
		in.DateOfDelivery = time.Now().UTC().Format("2006-01-02")
	}

	return nil
}

// ListContainers returns all containers for a shipment with calculated dwell days.
func (s *Service) ListContainers(shipmentID string) ([]models.ShipmentContainer, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.containerRepo.ListByShipment(shipmentID)
}

// CreateContainer adds a new container/lot to an existing shipment and recalculates status.
func (s *Service) CreateContainer(in models.ShipmentContainerInput) (models.ShipmentContainer, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ShipmentContainer{}, err
	}
	sh, err := s.repo.GetByID(in.ShipmentID)
	if err != nil {
		return models.ShipmentContainer{}, fmt.Errorf("shipment %s not found: %w", in.ShipmentID, err)
	}
	if sh.IsFrozen {
		return models.ShipmentContainer{}, fmt.Errorf("shipment %s is frozen and cannot be modified", sh.ID)
	}
	in.ContainerNumber = strings.TrimSpace(in.ContainerNumber)
	if in.ContainerNumber == "" {
		return models.ShipmentContainer{}, errors.New("containerNumber is required")
	}

	created, err := s.containerRepo.Create(in)
	if err != nil {
		return models.ShipmentContainer{}, err
	}
	_ = s.audit.RecordTableAudit("shipment_containers", created.ID, "INSERT", sess.UserID, nil, created, nil)
	return created, nil
}

// UpdateContainer updates container routing, warehouse dates, and delivery status.
func (s *Service) UpdateContainer(id string, in models.ShipmentContainerInput) (models.ShipmentContainer, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.ShipmentContainer{}, err
	}
	before, err := s.containerRepo.GetByID(id)
	if err != nil {
		return models.ShipmentContainer{}, err
	}
	sh, err := s.repo.GetByID(before.ShipmentID)
	if err != nil {
		return models.ShipmentContainer{}, err
	}
	if sh.IsFrozen {
		return models.ShipmentContainer{}, fmt.Errorf("shipment %s is frozen and cannot be modified", sh.ID)
	}

	in.ShipmentID = before.ShipmentID
	updated, err := s.containerRepo.Update(id, in)
	if err != nil {
		return models.ShipmentContainer{}, err
	}
	_ = s.audit.RecordTableAudit("shipment_containers", updated.ID, "UPDATE", sess.UserID, before, updated, nil)
	return updated, nil
}

// DeleteContainer soft-deletes a container from a non-frozen shipment.
func (s *Service) DeleteContainer(id string) error {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return err
	}
	before, err := s.containerRepo.GetByID(id)
	if err != nil {
		return err
	}
	sh, err := s.repo.GetByID(before.ShipmentID)
	if err != nil {
		return err
	}
	if sh.IsFrozen {
		return fmt.Errorf("shipment %s is frozen and cannot be modified", sh.ID)
	}

	if err := s.containerRepo.Delete(id); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit("shipment_containers", id, "DELETE", sess.UserID, before, nil, nil)
	return nil
}

// ProcessContainerMovement executes staged delivery actions (Direct, Move to Warehouse, Dispatch).
func (s *Service) ProcessContainerMovement(action models.ContainerMovementAction) error {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return err
	}
	if len(action.ContainerIDs) == 0 {
		return errors.New("no containers selected")
	}
	if action.ShipmentID != "" {
		sh, err := s.repo.GetByID(action.ShipmentID)
		if err == nil && sh.IsFrozen {
			return fmt.Errorf("shipment %s is frozen and cannot be modified", sh.ID)
		}
	}

	if err := s.containerRepo.ProcessMovement(action); err != nil {
		return err
	}
	_ = s.audit.RecordUserActivity(sess.UserID, "shipment.container_movement", "container_movement", action.ActionType, "SUCCESS", "INFO", map[string]any{
		"shipmentId":   action.ShipmentID,
		"containerIds": action.ContainerIDs,
		"actionType":   action.ActionType,
	})
	return nil
}

// GetWarehouseDwellReport returns container dwell time metrics and storage estimates across warehouses.
func (s *Service) GetWarehouseDwellReport(filter string) ([]models.WarehouseDwellSummary, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.containerRepo.ListWarehouseDwell(filter)
}

func parseTransitDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("empty date")
	}
	formats := []string{
		"2006-01-02",
		"02-01-2006",
		"02-Jan-2006",
		"2006/01/02",
		"02/01/2006",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %q", raw)
}

func formatTransitDate(raw string) string {
	t, err := parseTransitDate(raw)
	if err != nil {
		return raw
	}
	return t.Format("02-Jan-2006")
}

// ComputeTransitStatusAndDays determines transit period and automated status according to the template rules:
// - Status is automatic:
//   Pending (ETA missing)
//   In Transit (ETA today or later and no ATA)
//   Delayed (ETA passed and no ATA)
//   Arrived (ATA entered)
// - Transit Period (Days) is calculated automatically as ETA minus ETD (calendar days).
func ComputeTransitStatusAndDays(etdStr, etaStr, dateOfDelivery, shipmentStatus string, today time.Time) (string, *int) {
	var transitDays *int
	etdStr = strings.TrimSpace(etdStr)
	etaStr = strings.TrimSpace(etaStr)
	dateOfDelivery = strings.TrimSpace(dateOfDelivery)

	if etdStr != "" && etaStr != "" {
		etd, err1 := parseTransitDate(etdStr)
		eta, err2 := parseTransitDate(etaStr)
		if err1 == nil && err2 == nil {
			days := int(eta.Sub(etd).Hours() / 24)
			transitDays = &days
		}
	}

	isArrived := (dateOfDelivery != "") ||
		strings.EqualFold(shipmentStatus, "delivered") ||
		strings.EqualFold(shipmentStatus, "arrived") ||
		strings.EqualFold(shipmentStatus, "completed")

	if isArrived {
		return "Arrived", transitDays
	}

	if etaStr == "" {
		return "Pending", transitDays
	}

	eta, err := parseTransitDate(etaStr)
	if err != nil {
		return "Pending", transitDays
	}

	todayMidnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	etaMidnight := time.Date(eta.Year(), eta.Month(), eta.Day(), 0, 0, 0, 0, today.Location())

	if etaMidnight.Before(todayMidnight) {
		return "Delayed", transitDays
	}
	return "In Transit", transitDays
}

// GetTransitShipmentReport compiles the Transit Shipment tracker dataset, live summary KPIs, and status breakdown.
func (s *Service) GetTransitShipmentReport(filter models.TransitShipmentFilter) (models.TransitShipmentReport, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.TransitShipmentReport{}, err
	}

	pageReq := models.ShipmentPageRequest{
		Search:     filter.Search,
		SupplierID: filter.SupplierID,
		Page:       1,
		PageSize:   500,
		SortBy:     "invoiceDate",
		SortDir:    "DESC",
	}

	page, err := s.repo.List(pageReq)
	if err != nil {
		return models.TransitShipmentReport{}, err
	}

	now := time.Now()
	var allItems []models.TransitShipmentItem
	var filteredItems []models.TransitShipmentItem

	var totalTransitDays int
	var transitDaysCount int
	inTransitCount := 0
	arrivedCount := 0
	delayedCount := 0
	pendingCount := 0

	sNo := 1
	for _, sh := range page.Items {
		if filter.StartDate != "" && sh.InvoiceDate != "" && sh.InvoiceDate < filter.StartDate {
			continue
		}
		if filter.EndDate != "" && sh.InvoiceDate != "" && sh.InvoiceDate > filter.EndDate {
			continue
		}

		status, transitDays := ComputeTransitStatusAndDays(sh.ETD, sh.ETA, sh.DateOfDelivery, sh.Status, now)

		if transitDays != nil {
			totalTransitDays += *transitDays
			transitDaysCount++
		}

		switch status {
		case "In Transit":
			inTransitCount++
		case "Arrived":
			arrivedCount++
		case "Delayed":
			delayedCount++
		case "Pending":
			pendingCount++
		}

		remarks := ""
		if sh.ContainerNumber != "" {
			remarks = "Container: " + sh.ContainerNumber
		}

		item := models.TransitShipmentItem{
			SNo:             sNo,
			ID:              sh.ID,
			SupplierID:      sh.SupplierID,
			SupplierName:    sh.SupplierName,
			InvoiceNumber:   sh.InvoiceNumber,
			InvoiceDate:     sh.InvoiceDate,
			VesselVoyage:    sh.VesselName,
			ETD:             sh.ETD,
			ETA:             sh.ETA,
			TransitDays:     transitDays,
			Status:          status,
			Remarks:         remarks,
			DateOfDelivery:  sh.DateOfDelivery,
			BLAWBNumber:     sh.BLAWBNumber,
			ContainerNumber: sh.ContainerNumber,
			GrossWeightKg:   sh.GrossWeightKg,
			InvoiceValue:    sh.InvoiceValue,
			InvoiceCurrency: sh.InvoiceCurrency,
		}
		sNo++

		allItems = append(allItems, item)

		statusFilter := strings.ToLower(strings.TrimSpace(filter.Status))
		if statusFilter != "" && statusFilter != "all" {
			normalizedStatus := strings.ToLower(strings.ReplaceAll(status, " ", "-"))
			if normalizedStatus != statusFilter {
				continue
			}
		}
		filteredItems = append(filteredItems, item)
	}

	totalCount := len(allItems)
	avgTransit := 0.0
	if transitDaysCount > 0 {
		avgTransit = float64(totalTransitDays) / float64(transitDaysCount)
		avgTransit = float64(int(avgTransit*10+0.5)) / 10.0
	}

	calcPct := func(count int) float64 {
		if totalCount == 0 {
			return 0
		}
		return float64(int((float64(count)/float64(totalCount)*1000)+0.5)) / 10.0
	}

	summary := models.TransitShipmentSummary{
		TotalShipments:        totalCount,
		InTransit:             inTransitCount,
		Arrived:               arrivedCount,
		Delayed:               delayedCount,
		Pending:               pendingCount,
		AveragePlannedTransit: avgTransit,
	}

	breakdown := []models.TransitStatusCount{
		{Status: "In Transit", Count: inTransitCount, Pct: calcPct(inTransitCount)},
		{Status: "Arrived", Count: arrivedCount, Pct: calcPct(arrivedCount)},
		{Status: "Delayed", Count: delayedCount, Pct: calcPct(delayedCount)},
		{Status: "Pending", Count: pendingCount, Pct: calcPct(pendingCount)},
	}

	return models.TransitShipmentReport{
		Summary:   summary,
		Breakdown: breakdown,
		Items:     filteredItems,
	}, nil
}

// ExportTransitShipmentsCSV generates the exact template-compliant CSV for Import Shipment Transit Details.
func (s *Service) ExportTransitShipmentsCSV(filter models.TransitShipmentFilter) (string, error) {
	report, err := s.GetTransitShipmentReport(filter)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("IMPORT SHIPMENT TRANSIT DETAILS,,,,,,,,,\r\n")
	sb.WriteString(",,,,,,,,,\r\n")
	sb.WriteString("S.No.,Shipper / Supplier,Invoice No.,Invoice Date,Vessel / Voyage,ETD,ETA,Transit Period (Days),Shipment Status,Remarks\r\n")

	for idx, itm := range report.Items {
		transitDaysStr := ""
		if itm.TransitDays != nil {
			transitDaysStr = strconv.Itoa(*itm.TransitDays)
		}
		row := []string{
			strconv.Itoa(idx + 1),
			itm.SupplierName,
			itm.InvoiceNumber,
			formatTransitDate(itm.InvoiceDate),
			itm.VesselVoyage,
			formatTransitDate(itm.ETD),
			formatTransitDate(itm.ETA),
			transitDaysStr,
			itm.Status,
			itm.Remarks,
		}
		escaped := make([]string, len(row))
		for i, v := range row {
			if strings.ContainsAny(v, ",\"\r\n") {
				escaped[i] = `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
			} else {
				escaped[i] = v
			}
		}
		sb.WriteString(strings.Join(escaped, ",") + "\r\n")
	}

	sb.WriteString("IMPORT SHIPMENT OVERVIEW,,,,\r\n")
	sb.WriteString("Live summary based on entries in the Shipment Tracker sheet,,,,\r\n")
	sb.WriteString(",,,,\r\n")
	sb.WriteString("Total Shipments,,In Transit,,Arrived\r\n")
	sb.WriteString(fmt.Sprintf("%d,,%d,,%d\r\n", report.Summary.TotalShipments, report.Summary.InTransit, report.Summary.Arrived))
	sb.WriteString(",,,,\r\n")
	sb.WriteString(",,,,\r\n")
	sb.WriteString("Delayed,,Pending,,Average Planned Transit (Days)\r\n")
	sb.WriteString(fmt.Sprintf("%d,,%d,,%.1f\r\n", report.Summary.Delayed, report.Summary.Pending, report.Summary.AveragePlannedTransit))
	sb.WriteString(",,,,\r\n")
	sb.WriteString(",,,,\r\n")
	sb.WriteString(",,,,\r\n")
	sb.WriteString("Shipment Status Breakdown,,,,\r\n")
	sb.WriteString("Status,Count,,,\r\n")
	sb.WriteString(fmt.Sprintf("In Transit,%d,,,\r\n", report.Summary.InTransit))
	sb.WriteString(fmt.Sprintf("Arrived,%d,,,\r\n", report.Summary.Arrived))
	sb.WriteString(fmt.Sprintf("Delayed,%d,,,\r\n", report.Summary.Delayed))
	sb.WriteString(fmt.Sprintf("Pending,%d,,,\r\n", report.Summary.Pending))
	sb.WriteString("HOW TO USE THIS TEMPLATE,\r\n")
	sb.WriteString(",\r\n")
	sb.WriteString("1. Enter shipment details,Use the light-blue cells in the Shipment Tracker sheet. One row represents one import shipment / invoice.\r\n")
	sb.WriteString("2. Required fields,\"Enter at least the Shipper / Supplier, Invoice No., ETD and ETA to calculate the planned transit period.\"\r\n")
	sb.WriteString("3. Transit Period,Transit Period (Days) is calculated automatically as ETA minus ETD.\r\n")
	sb.WriteString("4. Actual Transit,Enter ATD (Actual Time of Departure) and ATA (Actual Time of Arrival). Actual Transit (Days) is calculated automatically.\r\n")
	sb.WriteString("5. Shipment Status,\"Status is automatic: Pending (ETA missing), In Transit (ETA today or later and no ATA), Delayed (ETA passed and no ATA), Arrived (ATA entered).\"\r\n")
	sb.WriteString("6. Dashboard,The Dashboard updates automatically based on the Shipment Tracker entries.\r\n")
	sb.WriteString("7. Date format,Enter real Excel dates. The workbook displays dates as dd-mmm-yyyy.\r\n")
	sb.WriteString("8. Capacity,The template is preformatted for 200 shipment entries. Extend the formulas/table if you need more rows.\r\n")
	sb.WriteString("Note,Transit period is calendar days (not working days).\r\n")

	return "\xEF\xBB\xBF" + sb.String(), nil
}
