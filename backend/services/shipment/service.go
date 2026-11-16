package shipment

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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
		repo:          repositories.NewShipmentRepository(db),
		containerRepo: repositories.NewContainerRepository(db),
		itemRepo:      repositories.NewItemRepository(db),
		supRepo:       repositories.NewSupplierRepository(db),
		sec:           sec,
		audit:         auditSvc,
	}
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

// Create validates and inserts a new shipment record.
func (s *Service) Create(in models.ShipmentInput) (models.Shipment, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.Shipment{}, err
	}
	if err := s.normalizeAndValidate(&in); err != nil {
		return models.Shipment{}, err
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
	sess, err := s.sec.RequirePermission(security.PermRoleWrite)
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

// Delete soft-deletes a non-frozen shipment into Recycle Bin.
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
