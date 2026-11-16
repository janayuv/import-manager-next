package repositories

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/janayuv/import-manager-next/backend/models"
)

// ContainerRepository manages persistence and business queries for shipment containers and warehouse tracking.
type ContainerRepository struct {
	db *sql.DB
}

// NewContainerRepository constructs a ContainerRepository.
func NewContainerRepository(db *sql.DB) *ContainerRepository {
	return &ContainerRepository{db: db}
}

// ListByShipment returns all non-deleted containers belonging to a shipment, with dwell time calculated.
func (r *ContainerRepository) ListByShipment(shipmentID string) ([]models.ShipmentContainer, error) {
	rows, err := r.db.Query(`
		SELECT id, shipment_id, container_number, container_size_type,
		       COALESCE(seal_number, ''), route_type,
		       COALESCE(warehouse_name, ''), COALESCE(gate_in_date, ''), COALESCE(gate_out_date, ''),
		       free_days_allowed, storage_rate_per_day,
		       delivery_status, COALESCE(date_of_delivery, ''),
		       COALESCE(transporter_name, ''), COALESCE(vehicle_number, ''),
		       COALESCE(lr_number, ''), COALESCE(empty_return_date, ''),
		       COALESCE(remarks, ''), created_at, updated_at
		FROM shipment_containers
		WHERE shipment_id = ? AND deleted_at IS NULL
		ORDER BY created_at ASC, id ASC`, strings.TrimSpace(shipmentID))
	if err != nil {
		return nil, fmt.Errorf("list containers for shipment %s: %w", shipmentID, err)
	}
	defer rows.Close()

	var list []models.ShipmentContainer
	for rows.Next() {
		var c models.ShipmentContainer
		if err := rows.Scan(
			&c.ID, &c.ShipmentID, &c.ContainerNumber, &c.ContainerSizeType,
			&c.SealNumber, &c.RouteType,
			&c.WarehouseName, &c.GateInDate, &c.GateOutDate,
			&c.FreeDaysAllowed, &c.StorageRatePerDay,
			&c.DeliveryStatus, &c.DateOfDelivery,
			&c.TransporterName, &c.VehicleNumber,
			&c.LRNumber, &c.EmptyReturnDate,
			&c.Remarks, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan container: %w", err)
		}
		c.DwellDays, c.BillableDays, c.StorageCost = computeDwell(c.GateInDate, c.GateOutDate, c.FreeDaysAllowed, c.StorageRatePerDay)
		list = append(list, c)
	}
	return list, rows.Err()
}

// GetByID returns a single container by ID.
func (r *ContainerRepository) GetByID(id string) (models.ShipmentContainer, error) {
	row := r.db.QueryRow(`
		SELECT id, shipment_id, container_number, container_size_type,
		       COALESCE(seal_number, ''), route_type,
		       COALESCE(warehouse_name, ''), COALESCE(gate_in_date, ''), COALESCE(gate_out_date, ''),
		       free_days_allowed, storage_rate_per_day,
		       delivery_status, COALESCE(date_of_delivery, ''),
		       COALESCE(transporter_name, ''), COALESCE(vehicle_number, ''),
		       COALESCE(lr_number, ''), COALESCE(empty_return_date, ''),
		       COALESCE(remarks, ''), created_at, updated_at
		FROM shipment_containers
		WHERE id = ? AND deleted_at IS NULL`, strings.TrimSpace(id))

	var c models.ShipmentContainer
	if err := row.Scan(
		&c.ID, &c.ShipmentID, &c.ContainerNumber, &c.ContainerSizeType,
		&c.SealNumber, &c.RouteType,
		&c.WarehouseName, &c.GateInDate, &c.GateOutDate,
		&c.FreeDaysAllowed, &c.StorageRatePerDay,
		&c.DeliveryStatus, &c.DateOfDelivery,
		&c.TransporterName, &c.VehicleNumber,
		&c.LRNumber, &c.EmptyReturnDate,
		&c.Remarks, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return models.ShipmentContainer{}, fmt.Errorf("get container %s: %w", id, err)
	}
	c.DwellDays, c.BillableDays, c.StorageCost = computeDwell(c.GateInDate, c.GateOutDate, c.FreeDaysAllowed, c.StorageRatePerDay)
	return c, nil
}

// Create inserts a new container record and syncs the parent shipment status.
func (r *ContainerRepository) Create(in models.ShipmentContainerInput) (models.ShipmentContainer, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = "CONT-" + uuid.New().String()[:8]
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	sizeType := strings.TrimSpace(in.ContainerSizeType)
	if sizeType == "" {
		sizeType = "40FT"
	}
	routeType := strings.ToUpper(strings.TrimSpace(in.RouteType))
	if routeType == "" {
		routeType = "DIRECT"
	}
	status := strings.ToUpper(strings.TrimSpace(in.DeliveryStatus))
	if status == "" {
		if strings.TrimSpace(in.DateOfDelivery) != "" {
			status = "DELIVERED"
		} else if routeType == "WAREHOUSE" && strings.TrimSpace(in.GateInDate) != "" {
			status = "IN_WAREHOUSE"
		} else {
			status = "PENDING"
		}
	}

	_, err := r.db.Exec(`
		INSERT INTO shipment_containers (
			id, shipment_id, container_number, container_size_type,
			seal_number, route_type,
			warehouse_name, gate_in_date, gate_out_date,
			free_days_allowed, storage_rate_per_day,
			delivery_status, date_of_delivery,
			transporter_name, vehicle_number, lr_number, empty_return_date,
			remarks, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, strings.TrimSpace(in.ShipmentID), strings.TrimSpace(in.ContainerNumber), sizeType,
		nullIfEmpty(in.SealNumber), routeType,
		nullIfEmpty(in.WarehouseName), nullIfEmpty(in.GateInDate), nullIfEmpty(in.GateOutDate),
		in.FreeDaysAllowed, in.StorageRatePerDay,
		status, nullIfEmpty(in.DateOfDelivery),
		nullIfEmpty(in.TransporterName), nullIfEmpty(in.VehicleNumber), nullIfEmpty(in.LRNumber), nullIfEmpty(in.EmptyReturnDate),
		nullIfEmpty(in.Remarks), now, now,
	)
	if err != nil {
		return models.ShipmentContainer{}, fmt.Errorf("insert container: %w", err)
	}

	_ = r.SyncShipmentRollupStatus(in.ShipmentID)
	return r.GetByID(id)
}

// Update updates an existing container record and syncs the parent shipment status.
func (r *ContainerRepository) Update(id string, in models.ShipmentContainerInput) (models.ShipmentContainer, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	sizeType := strings.TrimSpace(in.ContainerSizeType)
	if sizeType == "" {
		sizeType = "40FT"
	}
	routeType := strings.ToUpper(strings.TrimSpace(in.RouteType))
	if routeType == "" {
		routeType = "DIRECT"
	}
	status := strings.ToUpper(strings.TrimSpace(in.DeliveryStatus))
	if status == "" {
		if strings.TrimSpace(in.DateOfDelivery) != "" {
			status = "DELIVERED"
		} else if routeType == "WAREHOUSE" && strings.TrimSpace(in.GateInDate) != "" && strings.TrimSpace(in.GateOutDate) == "" {
			status = "IN_WAREHOUSE"
		} else {
			status = "PENDING"
		}
	}

	res, err := r.db.Exec(`
		UPDATE shipment_containers SET
			container_number = ?,
			container_size_type = ?,
			seal_number = ?,
			route_type = ?,
			warehouse_name = ?,
			gate_in_date = ?,
			gate_out_date = ?,
			free_days_allowed = ?,
			storage_rate_per_day = ?,
			delivery_status = ?,
			date_of_delivery = ?,
			transporter_name = ?,
			vehicle_number = ?,
			lr_number = ?,
			empty_return_date = ?,
			remarks = ?,
			updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`,
		strings.TrimSpace(in.ContainerNumber), sizeType,
		nullIfEmpty(in.SealNumber), routeType,
		nullIfEmpty(in.WarehouseName), nullIfEmpty(in.GateInDate), nullIfEmpty(in.GateOutDate),
		in.FreeDaysAllowed, in.StorageRatePerDay,
		status, nullIfEmpty(in.DateOfDelivery),
		nullIfEmpty(in.TransporterName), nullIfEmpty(in.VehicleNumber), nullIfEmpty(in.LRNumber), nullIfEmpty(in.EmptyReturnDate),
		nullIfEmpty(in.Remarks), now,
		strings.TrimSpace(id),
	)
	if err != nil {
		return models.ShipmentContainer{}, fmt.Errorf("update container %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return models.ShipmentContainer{}, fmt.Errorf("container %q not found", id)
	}

	_ = r.SyncShipmentRollupStatus(in.ShipmentID)
	return r.GetByID(id)
}

// Delete marks a container record as soft-deleted.
func (r *ContainerRepository) Delete(id string) error {
	var shipmentID string
	_ = r.db.QueryRow(`SELECT shipment_id FROM shipment_containers WHERE id = ?`, id).Scan(&shipmentID)

	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := r.db.Exec(`UPDATE shipment_containers SET deleted_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	if err != nil {
		return fmt.Errorf("delete container %s: %w", id, err)
	}
	if shipmentID != "" {
		_ = r.SyncShipmentRollupStatus(shipmentID)
	}
	return nil
}

// ProcessMovement applies a batch operational move to one or more containers.
func (r *ContainerRepository) ProcessMovement(action models.ContainerMovementAction) error {
	if len(action.ContainerIDs) == 0 {
		return fmt.Errorf("no containers selected for movement")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, cid := range action.ContainerIDs {
		cid = strings.TrimSpace(cid)
		if cid == "" {
			continue
		}

		switch strings.ToUpper(action.ActionType) {
		case "DIRECT_DELIVER":
			dDate := strings.TrimSpace(action.DateOfDelivery)
			if dDate == "" {
				dDate = time.Now().Format("2006-01-02")
			}
			_, err = tx.Exec(`
				UPDATE shipment_containers SET
					route_type = 'DIRECT',
					delivery_status = 'DELIVERED',
					date_of_delivery = ?,
					transporter_name = COALESCE(?, transporter_name),
					vehicle_number = COALESCE(?, vehicle_number),
					lr_number = COALESCE(?, lr_number),
					empty_return_date = COALESCE(?, empty_return_date),
					remarks = CASE WHEN ? != '' THEN ? ELSE remarks END,
					updated_at = ?
				WHERE id = ? AND deleted_at IS NULL`,
				dDate,
				nullIfEmpty(action.TransporterName),
				nullIfEmpty(action.VehicleNumber),
				nullIfEmpty(action.LRNumber),
				nullIfEmpty(action.EmptyReturnDate),
				action.Remarks, action.Remarks,
				now, cid,
			)

		case "MOVE_TO_WAREHOUSE":
			inDate := strings.TrimSpace(action.GateInDate)
			if inDate == "" {
				inDate = time.Now().Format("2006-01-02")
			}
			whName := strings.TrimSpace(action.WarehouseName)
			if whName == "" {
				whName = "Main Warehouse"
			}
			_, err = tx.Exec(`
				UPDATE shipment_containers SET
					route_type = 'WAREHOUSE',
					delivery_status = 'IN_WAREHOUSE',
					warehouse_name = ?,
					gate_in_date = ?,
					gate_out_date = NULL,
					transporter_name = COALESCE(?, transporter_name),
					vehicle_number = COALESCE(?, vehicle_number),
					remarks = CASE WHEN ? != '' THEN ? ELSE remarks END,
					updated_at = ?
				WHERE id = ? AND deleted_at IS NULL`,
				whName, inDate,
				nullIfEmpty(action.TransporterName),
				nullIfEmpty(action.VehicleNumber),
				action.Remarks, action.Remarks,
				now, cid,
			)

		case "DISPATCH_FROM_WAREHOUSE":
			outDate := strings.TrimSpace(action.GateOutDate)
			if outDate == "" {
				outDate = time.Now().Format("2006-01-02")
			}
			dDate := strings.TrimSpace(action.DateOfDelivery)
			if dDate == "" {
				dDate = outDate
			}
			_, err = tx.Exec(`
				UPDATE shipment_containers SET
					delivery_status = 'DELIVERED',
					gate_out_date = ?,
					date_of_delivery = ?,
					transporter_name = COALESCE(?, transporter_name),
					vehicle_number = COALESCE(?, vehicle_number),
					lr_number = COALESCE(?, lr_number),
					empty_return_date = COALESCE(?, empty_return_date),
					remarks = CASE WHEN ? != '' THEN ? ELSE remarks END,
					updated_at = ?
				WHERE id = ? AND deleted_at IS NULL`,
				outDate, dDate,
				nullIfEmpty(action.TransporterName),
				nullIfEmpty(action.VehicleNumber),
				nullIfEmpty(action.LRNumber),
				nullIfEmpty(action.EmptyReturnDate),
				action.Remarks, action.Remarks,
				now, cid,
			)
		default:
			return fmt.Errorf("unknown action type: %s", action.ActionType)
		}

		if err != nil {
			return fmt.Errorf("process action %s on container %s: %w", action.ActionType, cid, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit movement tx: %w", err)
	}

	if action.ShipmentID != "" {
		_ = r.SyncShipmentRollupStatus(action.ShipmentID)
	}
	return nil
}

// ListWarehouseDwell returns active and historical warehouse containers across shipments.
func (r *ContainerRepository) ListWarehouseDwell(filter string) ([]models.WarehouseDwellSummary, error) {
	where := "sc.deleted_at IS NULL AND (sc.route_type = 'WAREHOUSE' OR sc.warehouse_name IS NOT NULL OR sc.gate_in_date IS NOT NULL)"
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "active" || filter == "in-warehouse" {
		where += " AND sc.delivery_status = 'IN_WAREHOUSE' AND (sc.gate_out_date IS NULL OR TRIM(sc.gate_out_date) = '')"
	} else if filter == "dispatched" || filter == "delivered" {
		where += " AND sc.delivery_status = 'DELIVERED' AND sc.gate_in_date IS NOT NULL"
	}

	rows, err := r.db.Query(`
		SELECT sc.id, sc.shipment_id, COALESCE(sup.supplier_name, ''),
		       COALESCE(s.invoice_number, ''), COALESCE(s.bl_awb_number, ''),
		       sc.container_number, sc.container_size_type,
		       COALESCE(sc.warehouse_name, ''), COALESCE(sc.gate_in_date, ''), COALESCE(sc.gate_out_date, ''),
		       sc.delivery_status, COALESCE(sc.date_of_delivery, ''),
		       sc.free_days_allowed, sc.storage_rate_per_day
		FROM shipment_containers sc
		JOIN shipments s ON s.id = sc.shipment_id
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		WHERE ` + where + `
		ORDER BY sc.gate_in_date DESC, sc.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list warehouse dwell: %w", err)
	}
	defer rows.Close()

	var summaries []models.WarehouseDwellSummary
	for rows.Next() {
		var s models.WarehouseDwellSummary
		if err := rows.Scan(
			&s.ContainerID, &s.ShipmentID, &s.SupplierName,
			&s.InvoiceNumber, &s.BLAWBNumber,
			&s.ContainerNumber, &s.ContainerSizeType,
			&s.WarehouseName, &s.GateInDate, &s.GateOutDate,
			&s.DeliveryStatus, &s.DateOfDelivery,
			&s.FreeDaysAllowed, &s.StorageRatePerDay,
		); err != nil {
			return nil, fmt.Errorf("scan dwell summary: %w", err)
		}
		s.DwellDays, s.BillableDays, s.EstimatedCharges = computeDwell(s.GateInDate, s.GateOutDate, s.FreeDaysAllowed, s.StorageRatePerDay)
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// SyncShipmentRollupStatus updates parent shipment status and delivery date based on container statuses.
func (r *ContainerRepository) SyncShipmentRollupStatus(shipmentID string) error {
	shipmentID = strings.TrimSpace(shipmentID)
	if shipmentID == "" {
		return nil
	}

	var currentStatus string
	var isFrozen bool
	if err := r.db.QueryRow(`SELECT COALESCE(status, 'docs-rcvd'), is_frozen FROM shipments WHERE id = ? AND deleted_at IS NULL`, shipmentID).Scan(&currentStatus, &isFrozen); err != nil {
		return err
	}
	if isFrozen || currentStatus == "cancelled" || currentStatus == "closed" {
		return nil
	}

	rows, err := r.db.Query(`
		SELECT container_number, container_size_type, delivery_status, COALESCE(date_of_delivery, '')
		FROM shipment_containers
		WHERE shipment_id = ? AND deleted_at IS NULL
		ORDER BY created_at ASC`, shipmentID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var total, delivered, inWarehouse int
	var latestDeliveryDate string
	var containerStrings []string

	for rows.Next() {
		var num, size, status, dDate string
		if err := rows.Scan(&num, &size, &status, &dDate); err != nil {
			return err
		}
		total++
		if num != "" {
			containerStrings = append(containerStrings, num)
		}
		status = strings.ToUpper(status)
		if status == "DELIVERED" {
			delivered++
			if dDate > latestDeliveryDate {
				latestDeliveryDate = dDate
			}
		} else if status == "IN_WAREHOUSE" {
			inWarehouse++
		}
	}

	if total == 0 {
		return nil
	}

	var newStatus string
	var newDate string
	if delivered == total {
		newStatus = "delivered"
		newDate = latestDeliveryDate
		if newDate == "" {
			newDate = time.Now().Format("2006-01-02")
		}
	} else if delivered > 0 {
		newStatus = "partially-delivered"
	} else if inWarehouse > 0 {
		newStatus = "in-warehouse"
	}

	containerSummary := strings.Join(containerStrings, ", ")
	if newStatus != "" {
		if newDate != "" {
			_, err = r.db.Exec(`UPDATE shipments SET status = ?, date_of_delivery = ?, container_number = CASE WHEN ? != '' THEN ? ELSE container_number END WHERE id = ?`,
				newStatus, newDate, containerSummary, containerSummary, shipmentID)
		} else {
			_, err = r.db.Exec(`UPDATE shipments SET status = ?, container_number = CASE WHEN ? != '' THEN ? ELSE container_number END WHERE id = ?`,
				newStatus, containerSummary, containerSummary, shipmentID)
		}
	} else if containerSummary != "" {
		_, err = r.db.Exec(`UPDATE shipments SET container_number = ? WHERE id = ?`, containerSummary, shipmentID)
	}

	return err
}

func computeDwell(gateIn, gateOut string, freeDays int, rate float64) (dwellDays, billableDays int, storageCost float64) {
	if strings.TrimSpace(gateIn) == "" {
		return 0, 0, 0
	}
	inDate, err := time.Parse("2006-01-02", strings.TrimSpace(gateIn))
	if err != nil {
		return 0, 0, 0
	}
	endDate := time.Now().UTC()
	if strings.TrimSpace(gateOut) != "" {
		if outDate, err := time.Parse("2006-01-02", strings.TrimSpace(gateOut)); err == nil {
			endDate = outDate
		}
	}
	inMid := time.Date(inDate.Year(), inDate.Month(), inDate.Day(), 0, 0, 0, 0, time.UTC)
	endMid := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
	days := int(endMid.Sub(inMid).Hours() / 24)
	if days < 0 {
		days = 0
	}
	billable := days - freeDays
	if billable < 0 {
		billable = 0
	}
	cost := float64(billable) * rate
	return days, billable, cost
}
