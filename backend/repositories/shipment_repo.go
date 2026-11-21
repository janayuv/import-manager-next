package repositories

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/models"
)

// ShipmentRepository manages persistence for shipments, linked inspection views, and import logs.
type ShipmentRepository struct {
	db *sql.DB
}

// NewShipmentRepository constructs a ShipmentRepository bound to db.
func NewShipmentRepository(db *sql.DB) *ShipmentRepository {
	return &ShipmentRepository{db: db}
}

var allowedShipmentSortCols = map[string]string{
	"id":            "s.id",
	"invoiceNumber": "s.invoice_number",
	"invoiceDate":   "s.invoice_date",
	"invoiceValue":  "s.invoice_value",
	"supplierName":  "sup.supplier_name",
	"status":        "s.status",
	"eta":           "s.eta",
}

// List returns filtered, sorted, paginated active (non-soft-deleted) shipments.
func (r *ShipmentRepository) List(req models.ShipmentPageRequest) (models.ShipmentPage, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 10
	}
	if size > 500 {
		size = 500
	}

	sortCol := allowedShipmentSortCols[req.SortBy]
	if sortCol == "" {
		sortCol = "s.invoice_date"
	}
	sortDir := "DESC"
	if strings.EqualFold(req.SortDir, "asc") {
		sortDir = "ASC"
	}

	where := []string{"s.deleted_at IS NULL"}
	var args []any

	if q := strings.TrimSpace(req.Search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(s.id LIKE ? OR s.invoice_number LIKE ? OR COALESCE(sup.supplier_name,'') LIKE ? OR COALESCE(s.bl_awb_number,'') LIKE ? OR COALESCE(s.vessel_name,'') LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	if sid := strings.TrimSpace(req.SupplierID); sid != "" {
		where = append(where, "s.supplier_id = ?")
		args = append(args, sid)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		where = append(where, "LOWER(COALESCE(s.status,'')) = LOWER(?)")
		args = append(args, st)
	}
	if req.FrozenOnly {
		where = append(where, "s.is_frozen = 1")
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM shipments s
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return models.ShipmentPage{}, fmt.Errorf("count shipments: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT s.id, s.supplier_id, COALESCE(sup.supplier_name,''), s.invoice_number, s.invoice_date,
		       s.goods_category, s.invoice_value, s.invoice_currency, s.incoterm,
		       COALESCE(s.shipment_mode,''), COALESCE(s.shipment_type,''),
		       COALESCE(s.bl_awb_number,''), COALESCE(s.bl_awb_date,''),
		       COALESCE(s.vessel_name,''), COALESCE(s.container_number,''),
		       COALESCE(s.gross_weight_kg,0), COALESCE(s.etd,''), COALESCE(s.eta,''),
		       COALESCE(s.planned_etd,''), COALESCE(s.actual_etd,''),
		       COALESCE(s.planned_eta,''), COALESCE(s.revised_eta,''), COALESCE(s.actual_eta,''),
		       COALESCE(s.vessel_discharge_date,''), COALESCE(s.customs_ooc_date,''),
		       COALESCE(s.factory_delivery_date,''), COALESCE(s.empty_return_date,''),
		       COALESCE(s.status,'docs-rcvd'), COALESCE(s.date_of_delivery,''),
		       s.is_frozen, COALESCE(s.created_at,'')
		FROM shipments s
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		WHERE %s
		ORDER BY %s %s, s.id DESC
		LIMIT ? OFFSET ?`, whereSQL, sortCol, sortDir)

	listArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := r.db.Query(query, listArgs...)
	if err != nil {
		return models.ShipmentPage{}, fmt.Errorf("query shipments: %w", err)
	}
	defer rows.Close()

	var items []models.Shipment
	for rows.Next() {
		var sh models.Shipment
		if err := scanShipmentRow(rows, &sh); err != nil {
			return models.ShipmentPage{}, err
		}
		items = append(items, sh)
	}
	return models.ShipmentPage{
		Items:      items,
		TotalCount: total,
		Page:       page,
		PageSize:   size,
	}, rows.Err()
}

// GetByID fetches an active shipment joined with supplier name.
func (r *ShipmentRepository) GetByID(id string) (models.Shipment, error) {
	row := r.db.QueryRow(`
		SELECT s.id, s.supplier_id, COALESCE(sup.supplier_name,''), s.invoice_number, s.invoice_date,
		       s.goods_category, s.invoice_value, s.invoice_currency, s.incoterm,
		       COALESCE(s.shipment_mode,''), COALESCE(s.shipment_type,''),
		       COALESCE(s.bl_awb_number,''), COALESCE(s.bl_awb_date,''),
		       COALESCE(s.vessel_name,''), COALESCE(s.container_number,''),
		       COALESCE(s.gross_weight_kg,0), COALESCE(s.etd,''), COALESCE(s.eta,''),
		       COALESCE(s.planned_etd,''), COALESCE(s.actual_etd,''),
		       COALESCE(s.planned_eta,''), COALESCE(s.revised_eta,''), COALESCE(s.actual_eta,''),
		       COALESCE(s.vessel_discharge_date,''), COALESCE(s.customs_ooc_date,''),
		       COALESCE(s.factory_delivery_date,''), COALESCE(s.empty_return_date,''),
		       COALESCE(s.status,'docs-rcvd'), COALESCE(s.date_of_delivery,''),
		       s.is_frozen, COALESCE(s.created_at,'')
		FROM shipments s
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		WHERE s.id = ? AND s.deleted_at IS NULL`, strings.TrimSpace(id))
	var sh models.Shipment
	if err := scanShipmentRow(row, &sh); errors.Is(err, sql.ErrNoRows) {
		return models.Shipment{}, fmt.Errorf("shipment %q not found", id)
	} else if err != nil {
		return models.Shipment{}, err
	}
	return sh, nil
}

// FindDuplicate finds an active shipment with the same supplierID and case-insensitive invoiceNumber.
func (r *ShipmentRepository) FindDuplicate(supplierID, invoiceNumber, excludeID string) (*models.Shipment, error) {
	sid := strings.TrimSpace(supplierID)
	inv := strings.TrimSpace(invoiceNumber)
	if sid == "" || inv == "" {
		return nil, nil
	}
	exc := strings.TrimSpace(excludeID)
	var id string
	var err error
	if exc != "" {
		err = r.db.QueryRow(`
			SELECT id FROM shipments
			WHERE deleted_at IS NULL
			  AND id <> ?
			  AND supplier_id = ?
			  AND LOWER(TRIM(invoice_number)) = LOWER(TRIM(?))
			LIMIT 1`, exc, sid, inv).Scan(&id)
	} else {
		err = r.db.QueryRow(`
			SELECT id FROM shipments
			WHERE deleted_at IS NULL
			  AND supplier_id = ?
			  AND LOWER(TRIM(invoice_number)) = LOWER(TRIM(?))
			LIMIT 1`, sid, inv).Scan(&id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	existing, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

// NextID returns the next sequential SHP-XXX identifier.
func (r *ShipmentRepository) NextID() (string, error) {
	rows, err := r.db.Query(`SELECT id FROM shipments WHERE id LIKE 'SHP-%'`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	maxNum := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "SHP-")); err == nil && n > maxNum {
			maxNum = n
		}
	}
	return fmt.Sprintf("SHP-%03d", maxNum+1), rows.Err()
}

// Create inserts a shipment row.
func (r *ShipmentRepository) Create(in models.ShipmentInput) (models.Shipment, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		var err error
		id, err = r.NextID()
		if err != nil {
			return models.Shipment{}, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)

	plannedETD := in.PlannedETD
	if plannedETD == "" {
		plannedETD = in.ETD
	}
	actualETD := in.ActualETD
	plannedETA := in.PlannedETA
	if plannedETA == "" {
		plannedETA = in.ETA
	}
	revisedETA := in.RevisedETA
	actualETA := in.ActualETA
	vesselDischarge := in.VesselDischargeDate
	customsOOC := in.CustomsOOCDate
	factoryDelivery := in.FactoryDeliveryDate
	if factoryDelivery == "" {
		factoryDelivery = in.DateOfDelivery
	}
	emptyReturn := in.EmptyReturnDate

	legacyETD := in.ETD
	if legacyETD == "" {
		legacyETD = plannedETD
	}
	legacyETA := in.ETA
	if legacyETA == "" {
		legacyETA = plannedETA
	}
	legacyDelivery := in.DateOfDelivery
	if legacyDelivery == "" {
		legacyDelivery = factoryDelivery
	}

	_, err := r.db.Exec(`
		INSERT INTO shipments (
			id, supplier_id, invoice_number, invoice_date, goods_category,
			invoice_value, invoice_currency, incoterm, shipment_mode, shipment_type,
			bl_awb_number, bl_awb_date, vessel_name, container_number, gross_weight_kg,
			etd, eta, planned_etd, actual_etd, planned_eta, revised_eta, actual_eta,
			vessel_discharge_date, customs_ooc_date, factory_delivery_date, empty_return_date,
			status, date_of_delivery, is_frozen, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		id, in.SupplierID, in.InvoiceNumber, in.InvoiceDate, in.GoodsCategory,
		in.InvoiceValue, in.InvoiceCurrency, in.Incoterm, nullIfEmpty(in.ShipmentMode), nullIfEmpty(in.ShipmentType),
		nullIfEmpty(in.BLAWBNumber), nullIfEmpty(in.BLAWBDate), nullIfEmpty(in.VesselName), nullIfEmpty(in.ContainerNumber), in.GrossWeightKg,
		nullIfEmpty(legacyETD), nullIfEmpty(legacyETA), nullIfEmpty(plannedETD), nullIfEmpty(actualETD),
		nullIfEmpty(plannedETA), nullIfEmpty(revisedETA), nullIfEmpty(actualETA),
		nullIfEmpty(vesselDischarge), nullIfEmpty(customsOOC), nullIfEmpty(factoryDelivery), nullIfEmpty(emptyReturn),
		in.Status, nullIfEmpty(legacyDelivery), now,
	)
	if err != nil {
		return models.Shipment{}, fmt.Errorf("insert shipment %s: %w", id, err)
	}

	crepo := NewContainerRepository(r.db)
	if len(in.Containers) > 0 {
		for _, c := range in.Containers {
			c.ShipmentID = id
			_, _ = crepo.Create(c)
		}
	} else if strings.TrimSpace(in.ContainerNumber) != "" {
		st := "40FT"
		if strings.Contains(strings.ToUpper(in.ShipmentType), "LCL") {
			st = "LCL"
		}
		_, _ = crepo.Create(models.ShipmentContainerInput{
			ShipmentID:        id,
			ContainerNumber:   strings.TrimSpace(in.ContainerNumber),
			ContainerSizeType: st,
			RouteType:         "DIRECT",
			DateOfDelivery:    in.DateOfDelivery,
		})
	}

	return r.GetByID(id)
}

// Update updates a non-deleted, non-frozen shipment row.
func (r *ShipmentRepository) Update(id string, in models.ShipmentInput) (models.Shipment, error) {
	plannedETD := in.PlannedETD
	if plannedETD == "" {
		plannedETD = in.ETD
	}
	actualETD := in.ActualETD
	plannedETA := in.PlannedETA
	if plannedETA == "" {
		plannedETA = in.ETA
	}
	revisedETA := in.RevisedETA
	actualETA := in.ActualETA
	vesselDischarge := in.VesselDischargeDate
	customsOOC := in.CustomsOOCDate
	factoryDelivery := in.FactoryDeliveryDate
	if factoryDelivery == "" {
		factoryDelivery = in.DateOfDelivery
	}
	emptyReturn := in.EmptyReturnDate

	legacyETD := in.ETD
	if legacyETD == "" {
		legacyETD = plannedETD
	}
	legacyETA := in.ETA
	if legacyETA == "" {
		legacyETA = plannedETA
	}
	legacyDelivery := in.DateOfDelivery
	if legacyDelivery == "" {
		legacyDelivery = factoryDelivery
	}

	res, err := r.db.Exec(`
		UPDATE shipments SET
			supplier_id = ?,
			invoice_number = ?,
			invoice_date = ?,
			goods_category = ?,
			invoice_value = ?,
			invoice_currency = ?,
			incoterm = ?,
			shipment_mode = ?,
			shipment_type = ?,
			bl_awb_number = ?,
			bl_awb_date = ?,
			vessel_name = ?,
			container_number = ?,
			gross_weight_kg = ?,
			etd = ?,
			eta = ?,
			planned_etd = ?,
			actual_etd = ?,
			planned_eta = ?,
			revised_eta = ?,
			actual_eta = ?,
			vessel_discharge_date = ?,
			customs_ooc_date = ?,
			factory_delivery_date = ?,
			empty_return_date = ?,
			status = ?,
			date_of_delivery = ?
		WHERE id = ? AND deleted_at IS NULL`,
		in.SupplierID, in.InvoiceNumber, in.InvoiceDate, in.GoodsCategory,
		in.InvoiceValue, in.InvoiceCurrency, in.Incoterm, nullIfEmpty(in.ShipmentMode), nullIfEmpty(in.ShipmentType),
		nullIfEmpty(in.BLAWBNumber), nullIfEmpty(in.BLAWBDate), nullIfEmpty(in.VesselName), nullIfEmpty(in.ContainerNumber), in.GrossWeightKg,
		nullIfEmpty(legacyETD), nullIfEmpty(legacyETA),
		nullIfEmpty(plannedETD), nullIfEmpty(actualETD),
		nullIfEmpty(plannedETA), nullIfEmpty(revisedETA), nullIfEmpty(actualETA),
		nullIfEmpty(vesselDischarge), nullIfEmpty(customsOOC), nullIfEmpty(factoryDelivery), nullIfEmpty(emptyReturn),
		in.Status, nullIfEmpty(legacyDelivery),
		strings.TrimSpace(id),
	)
	if err != nil {
		return models.Shipment{}, fmt.Errorf("update shipment %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return models.Shipment{}, fmt.Errorf("shipment %q not found", id)
	}
	return r.GetByID(id)
}

// SetFrozen updates shipments.is_frozen.
func (r *ShipmentRepository) SetFrozen(id string, frozen bool) (models.Shipment, error) {
	val := 0
	if frozen {
		val = 1
	}
	res, err := r.db.Exec(`UPDATE shipments SET is_frozen = ? WHERE id = ? AND deleted_at IS NULL`, val, strings.TrimSpace(id))
	if err != nil {
		return models.Shipment{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return models.Shipment{}, fmt.Errorf("shipment %q not found", id)
	}
	return r.GetByID(id)
}

// SoftDelete marks a shipment row as deleted.
func (r *ShipmentRepository) SoftDelete(id, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`UPDATE shipments SET deleted_at = ?, deleted_by = ? WHERE id = ? AND deleted_at IS NULL`, now, userID, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("shipment %q not found", id)
	}
	return nil
}

// UpdateReadyForDelivery executes the 7-day BOE ready-for-delivery transition across eligible non-delivered shipments.
func (r *ShipmentRepository) UpdateReadyForDelivery(referenceDate string) (int64, error) {
	if strings.TrimSpace(referenceDate) == "" {
		referenceDate = time.Now().UTC().Format("2006-01-02")
	}
	res, err := r.db.Exec(`
		UPDATE shipments
		SET status = 'ready-dly'
		WHERE deleted_at IS NULL
		  AND is_frozen = 0
		  AND (date_of_delivery IS NULL OR TRIM(date_of_delivery) = '')
		  AND LOWER(COALESCE(status, '')) NOT IN ('delivered', 'ready-dly', 'cancelled', 'closed', 'completed')
		  AND EXISTS (
		      SELECT 1
		      FROM boe_calculations bc
		      JOIN boe_details bd ON bd.id = bc.boe_id
		      WHERE bc.shipment_id = shipments.id
		        AND bc.deleted_at IS NULL
		        AND bd.deleted_at IS NULL
		        AND date(bd.be_date, '+7 days') <= date(?)
		  )`, referenceDate)
	if err != nil {
		return 0, fmt.Errorf("update ready-dly: %w", err)
	}
	return res.RowsAffected()
}

// GetDetail fetches a shipment plus linked invoices, BOE details, and expenses.
func (r *ShipmentRepository) GetDetail(id string) (models.ShipmentDetail, error) {
	sh, err := r.GetByID(id)
	if err != nil {
		return models.ShipmentDetail{}, err
	}
	detail := models.ShipmentDetail{Shipment: sh}

	invRepo := NewInvoiceRepository(r.db)
	allInvs, err := invRepo.ListByShipment(sh.ID)
	if err != nil {
		return models.ShipmentDetail{}, err
	}
	detail.Invoices = allInvs

	boeRows, err := r.db.Query(`
		SELECT bd.id, bd.be_number, bd.be_date, bd.location, COALESCE(bd.duty_amount,0), COALESCE(bd.duty_paid,0), COALESCE(bc.status,'')
		FROM boe_calculations bc
		JOIN boe_details bd ON bd.id = bc.boe_id
		WHERE bc.shipment_id = ? AND bc.deleted_at IS NULL AND bd.deleted_at IS NULL
		ORDER BY bd.be_date DESC`, sh.ID)
	if err != nil {
		return models.ShipmentDetail{}, err
	}
	defer boeRows.Close()
	for boeRows.Next() {
		var b models.ShipmentLinkedBOE
		if err := boeRows.Scan(&b.ID, &b.BENumber, &b.BEDate, &b.Location, &b.DutyAmount, &b.DutyPaid, &b.ReconStatus); err != nil {
			return models.ShipmentDetail{}, err
		}
		detail.BOEs = append(detail.BOEs, b)
	}

	expRows, err := r.db.Query(`
		SELECT e.id, COALESCE(et.name, e.expense_type_id), e.amount,
		       COALESCE(NULLIF(e.total_amount_paise, 0)/100.0, ROUND(e.amount * (1.0 + (COALESCE(e.cgst_rate,0) + COALESCE(e.sgst_rate,0) + COALESCE(e.igst_rate,0)) / 100.0), 2)),
		       COALESCE(e.remarks,'')
		FROM expenses e
		LEFT JOIN expense_types et ON et.id = e.expense_type_id
		WHERE e.shipment_id = ? AND e.deleted_at IS NULL
		ORDER BY e.id DESC`, sh.ID)
	if err != nil {
		return models.ShipmentDetail{}, err
	}
	defer expRows.Close()
	for expRows.Next() {
		var ex models.ShipmentLinkedExpense
		if err := expRows.Scan(&ex.ID, &ex.ExpenseType, &ex.Amount, &ex.TotalAmount, &ex.Remarks); err != nil {
			return models.ShipmentDetail{}, err
		}
		detail.Expenses = append(detail.Expenses, ex)
	}

	crepo := NewContainerRepository(r.db)
	containers, err := crepo.ListByShipment(sh.ID)
	if err == nil && len(containers) > 0 {
		detail.Containers = containers
	} else {
		detail.Containers = []models.ShipmentContainer{}
	}

	return detail, nil
}

// RecordImportLog writes an audit entry into shipment_import_log.
func (r *ShipmentRepository) RecordImportLog(fileName string, total, inserted, skipped, errRows int, status string) {
	_, _ = r.db.Exec(`
		INSERT INTO shipment_import_log (file_name, total_rows, inserted_rows, skipped_rows, error_rows, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fileName, total, inserted, skipped, errRows, status, time.Now().UTC().Format(time.RFC3339),
	)
}

func scanShipmentRow(row rowScanner, sh *models.Shipment) error {
	return row.Scan(
		&sh.ID, &sh.SupplierID, &sh.SupplierName, &sh.InvoiceNumber, &sh.InvoiceDate,
		&sh.GoodsCategory, &sh.InvoiceValue, &sh.InvoiceCurrency, &sh.Incoterm,
		&sh.ShipmentMode, &sh.ShipmentType, &sh.BLAWBNumber, &sh.BLAWBDate,
		&sh.VesselName, &sh.ContainerNumber, &sh.GrossWeightKg,
		&sh.ETD, &sh.ETA,
		&sh.PlannedETD, &sh.ActualETD,
		&sh.PlannedETA, &sh.RevisedETA, &sh.ActualETA,
		&sh.VesselDischargeDate, &sh.CustomsOOCDate,
		&sh.FactoryDeliveryDate, &sh.EmptyReturnDate,
		&sh.Status, &sh.DateOfDelivery, &sh.IsFrozen, &sh.CreatedAt,
	)
}
