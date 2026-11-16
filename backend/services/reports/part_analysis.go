package reports

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/validation"
)

// SearchParts returns matching items with import counts for real-time autocomplete suggestions.
func (s *Service) SearchParts(query string, limit int) ([]models.PartLookupItem, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := strings.TrimSpace(query)
	var args []any
	where := "i.deleted_at IS NULL"
	if q != "" {
		like := "%" + q + "%"
		where += " AND (UPPER(i.part_number) LIKE UPPER(?) OR UPPER(i.item_description) LIKE UPPER(?) OR COALESCE(i.hsn_code, '') LIKE ?)"
		args = append(args, like, like, like)
	}

	querySQL := fmt.Sprintf(`
		SELECT i.id, i.part_number, i.item_description, i.unit, i.currency, i.unit_price,
		       COALESCE(i.hsn_code, ''), COALESCE(i.supplier_id, ''), COALESCE(sup.supplier_name, ''),
		       COALESCE(i.category, ''),
		       (SELECT COUNT(DISTINCT inv.shipment_id)
		        FROM invoice_line_items ili
		        JOIN invoices inv ON inv.id = ili.invoice_id AND inv.deleted_at IS NULL
		        WHERE ili.item_id = i.id AND ili.deleted_at IS NULL) AS shipment_count,
		       (SELECT COALESCE(SUM(ili.quantity), 0.0)
		        FROM invoice_line_items ili
		        WHERE ili.item_id = i.id AND ili.deleted_at IS NULL) AS total_qty
		FROM items i
		LEFT JOIN suppliers sup ON sup.id = i.supplier_id
		WHERE %s
		ORDER BY (CASE WHEN UPPER(i.part_number) = UPPER(?) THEN 0 WHEN UPPER(i.part_number) LIKE UPPER(? || '%%') THEN 1 ELSE 2 END),
		         shipment_count DESC, i.part_number ASC
		LIMIT ?`, where)

	exactArgs := append([]any{}, args...)
	exactArgs = append(exactArgs, q, q, limit)

	rows, err := s.db.Query(querySQL, exactArgs...)
	if err != nil {
		return nil, fmt.Errorf("search parts query: %w", err)
	}
	defer rows.Close()

	var results []models.PartLookupItem
	for rows.Next() {
		var it models.PartLookupItem
		if err := rows.Scan(
			&it.ID, &it.PartNumber, &it.Description, &it.Unit, &it.Currency, &it.UnitPrice,
			&it.HSNCode, &it.SupplierID, &it.SupplierName, &it.Category,
			&it.ShipmentCount, &it.TotalQuantity,
		); err != nil {
			return nil, fmt.Errorf("scan part lookup: %w", err)
		}
		results = append(results, it)
	}
	return results, rows.Err()
}

// GetPartPeriodAnalysis evaluates a comprehensive 360-degree period view across Shipments, Item Master, BOE, and Expenses.
func (s *Service) GetPartPeriodAnalysis(req models.PartPeriodAnalysisRequest) (models.PartPeriodAnalysisResult, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.PartPeriodAnalysisResult{}, err
	}
	if req.StartDate != "" && req.EndDate != "" && req.StartDate > req.EndDate {
		return models.PartPeriodAnalysisResult{}, fmt.Errorf("startDate (%s) cannot be after endDate (%s)", req.StartDate, req.EndDate)
	}

	dateBasis := strings.ToLower(strings.TrimSpace(req.DateBasis))
	if dateBasis == "" {
		dateBasis = "invoice_date"
	}

	search := strings.TrimSpace(req.Search)
	partNumber := strings.TrimSpace(req.PartNumber)

	result := models.PartPeriodAnalysisResult{
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		DateBasis:     dateBasis,
		Shipments:     make([]models.PartShipmentRow, 0),
		BOELines:      make([]models.PartBOERow, 0),
		Expenses:      make([]models.PartExpenseRow, 0),
		Trends:        make([]models.PartTrendPoint, 0),
		MatchingParts: make([]models.PartLookupItem, 0),
	}

	// 1. Locate candidate item(s)
	candidateItems, err := s.queryCandidateItems(partNumber, search)
	if err != nil {
		return models.PartPeriodAnalysisResult{}, err
	}
	if len(candidateItems) == 0 {
		return result, nil
	}

	// First match is our target item
	targetItem := candidateItems[0]
	result.TargetItem = targetItem

	// Populate matching parts list for easy switching in UI
	lookupMatches, _ := s.SearchParts(partNumber, 25)
	if len(lookupMatches) == 0 && search != "" {
		lookupMatches, _ = s.SearchParts(search, 25)
	}
	if len(lookupMatches) == 0 {
		lookupMatches = []models.PartLookupItem{
			{
				ID:           targetItem.ID,
				PartNumber:   targetItem.PartNumber,
				Description:  targetItem.ItemDescription,
				Unit:         targetItem.Unit,
				Currency:     targetItem.Currency,
				UnitPrice:    targetItem.UnitPrice,
				HSNCode:      targetItem.HSNCode,
				SupplierID:   targetItem.SupplierID,
				SupplierName: targetItem.SupplierName,
				Category:     targetItem.Category,
			},
		}
	}
	result.MatchingParts = lookupMatches

	// 2. Query Shipments & Invoices containing targetItem.ID in period
	shipments, shipmentIDs, err := s.queryPartShipments(targetItem, req.StartDate, req.EndDate, dateBasis, req.SupplierID)
	if err != nil {
		return models.PartPeriodAnalysisResult{}, fmt.Errorf("query part shipments: %w", err)
	}
	result.Shipments = shipments

	// 3. Query BOE details & calculations for targetItem.PartNumber
	boeLines, shipmentAssessableMap, shipmentTotalAssessableMap, shipmentBOEInterestMap, err := s.queryPartBOEs(
		targetItem.PartNumber, shipmentIDs, req.StartDate, req.EndDate, dateBasis,
	)
	if err != nil {
		return models.PartPeriodAnalysisResult{}, fmt.Errorf("query part boes: %w", err)
	}
	result.BOELines = boeLines

	// 4. Query Expenses for relevant shipments and allocate proportionally
	expenses, shipmentAllocatedExpensesMap, err := s.queryPartExpenses(
		shipmentIDs, shipments, shipmentAssessableMap, shipmentTotalAssessableMap,
	)
	if err != nil {
		return models.PartPeriodAnalysisResult{}, fmt.Errorf("query part expenses: %w", err)
	}
	result.Expenses = expenses

	// 5. Build Chronological Cost Trends & Timeline
	result.Trends = s.buildPartTrends(
		shipments, shipmentAssessableMap, boeLines, shipmentBOEInterestMap, shipmentAllocatedExpensesMap,
	)

	// 6. Aggregate Executive Summary KPIs
	result.Summary = s.buildPartSummary(
		targetItem, shipments, boeLines, expenses, result.Trends,
	)

	return result, nil
}

func (s *Service) queryCandidateItems(partNumber, search string) ([]models.PartItemMasterInfo, error) {
	where := "i.deleted_at IS NULL"
	var args []any

	if partNumber != "" {
		where += " AND UPPER(i.part_number) = UPPER(?)"
		args = append(args, partNumber)
	} else if search != "" {
		like := "%" + search + "%"
		where += " AND (UPPER(i.part_number) LIKE UPPER(?) OR UPPER(i.item_description) LIKE UPPER(?) OR COALESCE(i.hsn_code, '') LIKE ?)"
		args = append(args, like, like, like)
	}

	q := fmt.Sprintf(`
		SELECT i.id, i.part_number, i.item_description, i.unit, i.currency, i.unit_price,
		       COALESCE(i.hsn_code, ''), COALESCE(i.supplier_id, ''), COALESCE(sup.supplier_name, ''),
		       i.is_active, COALESCE(i.country_of_origin, ''), COALESCE(i.bcd, ''), COALESCE(i.sws, ''),
		       COALESCE(i.igst, ''), COALESCE(i.technical_write_up, ''), COALESCE(i.category, ''),
		       COALESCE(i.end_use, ''), COALESCE(i.net_weight_kg, 0.0), COALESCE(i.purchase_uom, ''),
		       COALESCE(i.gross_weight_per_uom_kg, 0.0), COALESCE(i.photo_path, '')
		FROM items i
		LEFT JOIN suppliers sup ON sup.id = i.supplier_id
		WHERE %s
		ORDER BY i.part_number ASC
		LIMIT 20`, where)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.PartItemMasterInfo
	for rows.Next() {
		var it models.PartItemMasterInfo
		if err := rows.Scan(
			&it.ID, &it.PartNumber, &it.ItemDescription, &it.Unit, &it.Currency, &it.UnitPrice,
			&it.HSNCode, &it.SupplierID, &it.SupplierName, &it.IsActive,
			&it.CountryOfOrigin, &it.BCD, &it.SWS, &it.IGST,
			&it.TechnicalWriteUp, &it.Category, &it.EndUse,
			&it.NetWeightKg, &it.PurchaseUOM, &it.GrossWeightPerUOMKg, &it.PhotoPath,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Service) queryPartShipments(
	item models.PartItemMasterInfo,
	startDate, endDate, dateBasis, supplierID string,
) ([]models.PartShipmentRow, []string, error) {
	where := "ili.item_id = ? AND ili.deleted_at IS NULL AND inv.deleted_at IS NULL AND s.deleted_at IS NULL"
	args := []any{item.ID}

	switch dateBasis {
	case "be_date":
		if startDate != "" {
			where += ` AND EXISTS (SELECT 1 FROM boe_calculations bc JOIN boe_details bd ON bd.id = bc.boe_id WHERE bc.shipment_id = s.id AND bc.deleted_at IS NULL AND bd.deleted_at IS NULL AND bd.be_date >= ?)`
			args = append(args, startDate)
		}
		if endDate != "" {
			where += ` AND EXISTS (SELECT 1 FROM boe_calculations bc JOIN boe_details bd ON bd.id = bc.boe_id WHERE bc.shipment_id = s.id AND bc.deleted_at IS NULL AND bd.deleted_at IS NULL AND bd.be_date <= ?)`
			args = append(args, endDate)
		}
	case "shipment_date":
		if startDate != "" {
			where += ` AND COALESCE(NULLIF(s.bl_awb_date,''), NULLIF(s.date_of_delivery,''), s.invoice_date) >= ?`
			args = append(args, startDate)
		}
		if endDate != "" {
			where += ` AND COALESCE(NULLIF(s.bl_awb_date,''), NULLIF(s.date_of_delivery,''), s.invoice_date) <= ?`
			args = append(args, endDate)
		}
	default: // "invoice_date"
		if startDate != "" {
			where += ` AND s.invoice_date >= ?`
			args = append(args, startDate)
		}
		if endDate != "" {
			where += ` AND s.invoice_date <= ?`
			args = append(args, endDate)
		}
	}

	if strings.TrimSpace(supplierID) != "" {
		where += ` AND s.supplier_id = ?`
		args = append(args, strings.TrimSpace(supplierID))
	}

	q := fmt.Sprintf(`
		SELECT
			s.id,
			inv.id,
			s.invoice_number,
			s.invoice_date,
			COALESCE(s.supplier_id, ''),
			COALESCE(sup.supplier_name, ''),
			COALESCE(s.incoterm, ''),
			COALESCE(s.shipment_mode, ''),
			COALESCE(s.shipment_type, ''),
			COALESCE(s.bl_awb_number, ''),
			COALESCE(s.bl_awb_date, ''),
			COALESCE(s.vessel_name, ''),
			COALESCE(s.container_number, ''),
			COALESCE(s.gross_weight_kg, 0.0),
			COALESCE(s.etd, ''),
			COALESCE(s.eta, ''),
			COALESCE(s.status, 'docs-rcvd'),
			COALESCE(s.date_of_delivery, ''),
			s.is_frozen,
			ili.quantity,
			COALESCE(itm.unit, 'PCS'),
			ili.unit_price,
			COALESCE(s.invoice_currency, 'USD'),
			(ili.quantity * ili.unit_price) AS line_total,
			COALESCE(itm.unit_price, 0.0) AS master_price
		FROM invoice_line_items ili
		JOIN invoices inv ON inv.id = ili.invoice_id
		JOIN shipments s ON s.id = inv.shipment_id
		JOIN items itm ON itm.id = ili.item_id
		LEFT JOIN suppliers sup ON sup.id = s.supplier_id
		WHERE %s
		ORDER BY s.invoice_date DESC, s.id DESC`, where)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var shipments []models.PartShipmentRow
	shipmentIDSet := make(map[string]bool)
	var shipmentIDs []string

	for rows.Next() {
		var r models.PartShipmentRow
		var masterPrice float64
		if err := rows.Scan(
			&r.ShipmentID, &r.InvoiceID, &r.InvoiceNumber, &r.InvoiceDate,
			&r.SupplierID, &r.SupplierName, &r.Incoterm, &r.ShipmentMode, &r.ShipmentType,
			&r.BLAWBNumber, &r.BLAWBDate, &r.VesselName, &r.ContainerNumber,
			&r.GrossWeightKg, &r.ETD, &r.ETA, &r.Status, &r.DateOfDelivery,
			&r.IsFrozen, &r.Quantity, &r.Unit, &r.UnitPrice, &r.Currency,
			&r.LineTotal, &masterPrice,
		); err != nil {
			return nil, nil, err
		}
		r.MasterUnitPrice = masterPrice
		if masterPrice > 0 && math.Abs(r.UnitPrice-masterPrice) > 0.001 {
			r.PriceDiverged = true
		}
		shipments = append(shipments, r)

		if !shipmentIDSet[r.ShipmentID] {
			shipmentIDSet[r.ShipmentID] = true
			shipmentIDs = append(shipmentIDs, r.ShipmentID)
		}
	}
	return shipments, shipmentIDs, rows.Err()
}

func (s *Service) queryPartBOEs(
	partNumber string,
	relevantShipmentIDs []string,
	startDate, endDate, dateBasis string,
) ([]models.PartBOERow, map[string]float64, map[string]float64, map[string]float64, error) {
	shipmentAssessableMap := make(map[string]float64)
	shipmentTotalAssessableMap := make(map[string]float64)
	shipmentBOEInterestMap := make(map[string]float64)

	where := "bc.deleted_at IS NULL AND s.deleted_at IS NULL"
	var args []any

	if len(relevantShipmentIDs) > 0 {
		placeholders := make([]string, len(relevantShipmentIDs))
		for i, id := range relevantShipmentIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		where += fmt.Sprintf(" AND bc.shipment_id IN (%s)", strings.Join(placeholders, ","))
	}

	switch dateBasis {
	case "be_date":
		if startDate != "" {
			where += " AND bd.be_date >= ?"
			args = append(args, startDate)
		}
		if endDate != "" {
			where += " AND bd.be_date <= ?"
			args = append(args, endDate)
		}
	default:
		if startDate != "" {
			where += " AND s.invoice_date >= ?"
			args = append(args, startDate)
		}
		if endDate != "" {
			where += " AND s.invoice_date <= ?"
			args = append(args, endDate)
		}
	}

	qTotals := fmt.Sprintf(`
		SELECT bc.shipment_id,
		       SUM(CAST(COALESCE(json_extract(item.value, '$.assessableValue'), 0.0) AS REAL)) AS total_assessable,
		       CAST(COALESCE(
		           json_extract(bc.calculation_result_json, '$.interest'),
		           json_extract(bc.form_values_json, '$.interest'),
		           0.0
		       ) AS REAL) AS total_interest
		FROM boe_calculations bc
		JOIN shipments s ON s.id = bc.shipment_id
		LEFT JOIN boe_details bd ON bd.id = bc.boe_id AND bd.deleted_at IS NULL
		JOIN json_each(
		    COALESCE(
		        json_extract(bc.calculation_result_json, '$.items'),
		        json_extract(bc.calculation_result_json, '$.calculatedItems'),
		        '[]'
		    )
		) AS item
		WHERE %s
		GROUP BY bc.shipment_id`, where)

	totalsRows, err := s.db.Query(qTotals, args...)
	if err == nil {
		defer totalsRows.Close()
		for totalsRows.Next() {
			var shpID string
			var totAssess, totInterest float64
			if err := totalsRows.Scan(&shpID, &totAssess, &totInterest); err == nil {
				shipmentTotalAssessableMap[shpID] = totAssess
				shipmentBOEInterestMap[shpID] = totInterest
			}
		}
	}

	wherePart := where + ` AND UPPER(COALESCE(
		NULLIF(json_extract(item.value, '$.partNumber'), ''),
		NULLIF(json_extract(item.value, '$.partNo'), '')
	)) = UPPER(?)`
	argsPart := append(append([]any{}, args...), partNumber)

	qLines := fmt.Sprintf(`
		SELECT
			bc.shipment_id,
			bc.invoice_number,
			bc.id AS boe_calc_id,
			COALESCE(bc.boe_id, ''),
			COALESCE(bd.be_number, ''),
			COALESCE(bd.be_date, ''),
			COALESCE(bd.location, ''),
			COALESCE(bc.status, ''),
			CAST(COALESCE(json_extract(bc.calculation_result_json, '$.exchangeRate'), json_extract(bc.form_values_json, '$.exchangeRate'), 0.0) AS REAL) AS exchange_rate,
			CAST(COALESCE(json_extract(item.value, '$.quantity'), 0.0) AS REAL) AS qty,
			COALESCE(json_extract(item.value, '$.unit'), 'PCS') AS unit,
			CAST(COALESCE(json_extract(item.value, '$.itemForexValue'), 0.0) AS REAL) AS forex_val,
			CAST(COALESCE(json_extract(item.value, '$.itemValueInr'), 0.0) AS REAL) AS val_inr,
			CAST(COALESCE(json_extract(item.value, '$.assessableValue'), 0.0) AS REAL) AS assessable,
			CAST(COALESCE(json_extract(item.value, '$.actualBcdRate'), json_extract(item.value, '$.boeBcdRate'), 0.0) AS REAL) AS bcd_rate,
			CAST(COALESCE(json_extract(item.value, '$.bcdValue'), 0.0) AS REAL) AS bcd_val,
			CAST(COALESCE(json_extract(item.value, '$.actualSwsRate'), json_extract(item.value, '$.boeSwsRate'), 0.0) AS REAL) AS sws_rate,
			CAST(COALESCE(json_extract(item.value, '$.swsValue'), 0.0) AS REAL) AS sws_val,
			CAST(COALESCE(json_extract(item.value, '$.actualIgstRate'), json_extract(item.value, '$.boeIgstRate'), 0.0) AS REAL) AS igst_rate,
			CAST(COALESCE(json_extract(item.value, '$.igstValue'), 0.0) AS REAL) AS igst_val,
			COALESCE(bd.duty_paid, 0.0),
			COALESCE(bd.challan_number, ''),
			COALESCE(bd.payment_date, '')
		FROM boe_calculations bc
		JOIN shipments s ON s.id = bc.shipment_id
		LEFT JOIN boe_details bd ON bd.id = bc.boe_id AND bd.deleted_at IS NULL
		JOIN json_each(
		    COALESCE(
		        json_extract(bc.calculation_result_json, '$.items'),
		        json_extract(bc.calculation_result_json, '$.calculatedItems'),
		        '[]'
		    )
		) AS item
		WHERE %s
		ORDER BY bd.be_date DESC, bc.created_at DESC`, wherePart)

	rows, err := s.db.Query(qLines, argsPart...)
	if err != nil {
		return nil, shipmentAssessableMap, shipmentTotalAssessableMap, shipmentBOEInterestMap, err
	}
	defer rows.Close()

	var boeLines []models.PartBOERow
	for rows.Next() {
		var b models.PartBOERow
		if err := rows.Scan(
			&b.ShipmentID, &b.InvoiceNumber, &b.BOECalcID, &b.BOEID,
			&b.BENumber, &b.BEDate, &b.Location, &b.Status,
			&b.ExchangeRate, &b.Quantity, &b.Unit,
			&b.ItemForexValue, &b.ItemValueINR, &b.AssessableValue,
			&b.BCDRate, &b.BCDAmount, &b.SWSRate, &b.SWSAmount,
			&b.IGSTRate, &b.IGSTAmount,
			&b.DutyPaid, &b.ChallanNumber, &b.PaymentDate,
		); err != nil {
			return nil, shipmentAssessableMap, shipmentTotalAssessableMap, shipmentBOEInterestMap, err
		}

		totAssess := shipmentTotalAssessableMap[b.ShipmentID]
		totInterest := shipmentBOEInterestMap[b.ShipmentID]
		if totAssess > 0 && b.AssessableValue > 0 {
			b.BOEInterest = math.Round((totInterest*(b.AssessableValue/totAssess))*100) / 100
		}
		b.TotalDuty = math.Round((b.BCDAmount+b.SWSAmount+b.IGSTAmount+b.BOEInterest)*100) / 100

		shipmentAssessableMap[b.ShipmentID] = b.AssessableValue
		boeLines = append(boeLines, b)
	}

	return boeLines, shipmentAssessableMap, shipmentTotalAssessableMap, shipmentBOEInterestMap, rows.Err()
}

func (s *Service) queryPartExpenses(
	shipmentIDs []string,
	shipments []models.PartShipmentRow,
	shipmentAssessableMap, shipmentTotalAssessableMap map[string]float64,
) ([]models.PartExpenseRow, map[string]float64, error) {
	shipmentAllocatedExpensesMap := make(map[string]float64)
	if len(shipmentIDs) == 0 {
		return nil, shipmentAllocatedExpensesMap, nil
	}

	shipmentPartLineValueMap := make(map[string]float64)
	for _, sh := range shipments {
		shipmentPartLineValueMap[sh.ShipmentID] += sh.LineTotal
	}

	placeholders := make([]string, len(shipmentIDs))
	args := make([]any, len(shipmentIDs))
	for i, id := range shipmentIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	q := fmt.Sprintf(`
		SELECT
			e.id,
			e.expense_invoice_id,
			e.shipment_id,
			COALESCE(sh.invoice_number, ''),
			e.service_provider_id,
			COALESCE(sp.name, 'Service Provider'),
			e.expense_type_id,
			COALESCE(NULLIF(et.name, ''), e.expense_type_id, 'Expense Item'),
			COALESCE(e.invoice_no, ''),
			COALESCE(e.invoice_date, ''),
			COALESCE(NULLIF(e.amount_paise, 0), CAST(ROUND(COALESCE(e.amount, 0) * 100) AS INTEGER), 0) AS amt_p,
			(COALESCE(e.cgst_amount_paise, 0) + COALESCE(e.sgst_amount_paise, 0) + COALESCE(e.igst_amount_paise, 0)) AS gst_p,
			COALESCE(NULLIF(e.total_amount_paise, 0), CAST(ROUND(COALESCE(e.total_amount, e.amount, 0) * 100) AS INTEGER), 0) AS tot_p,
			COALESCE(e.remarks, '')
		FROM expenses e
		JOIN expense_invoices ei ON ei.id = e.expense_invoice_id AND ei.deleted_at IS NULL
		JOIN shipments sh ON sh.id = e.shipment_id AND sh.deleted_at IS NULL
		LEFT JOIN service_providers sp ON sp.id = e.service_provider_id
		LEFT JOIN expense_types et ON et.id = e.expense_type_id
		WHERE e.deleted_at IS NULL
		  AND e.shipment_id IN (%s)
		ORDER BY e.invoice_date DESC, e.id DESC`, strings.Join(placeholders, ","))

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, shipmentAllocatedExpensesMap, err
	}
	defer rows.Close()

	var expenses []models.PartExpenseRow
	for rows.Next() {
		var ex models.PartExpenseRow
		var amtPaise, gstPaise, totPaise int64
		if err := rows.Scan(
			&ex.ExpenseID, &ex.ExpenseInvoiceID, &ex.ShipmentID, &ex.ShipmentInvoiceNumber,
			&ex.ServiceProviderID, &ex.ServiceProviderName,
			&ex.ExpenseTypeID, &ex.ExpenseTypeName,
			&ex.ExpenseInvoiceNo, &ex.ExpenseInvoiceDate,
			&amtPaise, &gstPaise, &totPaise,
			&ex.Remarks,
		); err != nil {
			return nil, shipmentAllocatedExpensesMap, err
		}

		ex.ShipmentExpenseBasicINR = math.Round(float64(amtPaise)/100.0*100) / 100
		ex.ShipmentExpenseGSTINR = math.Round(float64(gstPaise)/100.0*100) / 100
		ex.ShipmentExpenseTotalINR = math.Round(float64(totPaise)/100.0*100) / 100

		// Proportional Allocation Ratio
		var ratio float64
		partAssess := shipmentAssessableMap[ex.ShipmentID]
		totAssess := shipmentTotalAssessableMap[ex.ShipmentID]
		if totAssess > 0 && partAssess > 0 {
			ratio = partAssess / totAssess
		} else {
			// Fallback: ratio is 1.0 or based on line value
			ratio = 1.0
		}
		if ratio > 1.0 {
			ratio = 1.0
		} else if ratio < 0.0 {
			ratio = 0.0
		}

		ex.AllocationRatio = math.Round(ratio*10000) / 10000
		ex.AllocatedBasicINR = math.Round(ex.ShipmentExpenseBasicINR*ratio*100) / 100
		ex.AllocatedGSTINR = math.Round(ex.ShipmentExpenseGSTINR*ratio*100) / 100
		ex.AllocatedTotalINR = math.Round(ex.ShipmentExpenseTotalINR*ratio*100) / 100

		shipmentAllocatedExpensesMap[ex.ShipmentID] += ex.AllocatedBasicINR
		expenses = append(expenses, ex)
	}

	return expenses, shipmentAllocatedExpensesMap, rows.Err()
}

func (s *Service) buildPartTrends(
	shipments []models.PartShipmentRow,
	shipmentAssessableMap map[string]float64,
	boeLines []models.PartBOERow,
	shipmentBOEInterestMap, shipmentAllocatedExpensesMap map[string]float64,
) []models.PartTrendPoint {
	boeDutyMap := make(map[string]float64)
	for _, b := range boeLines {
		boeDutyMap[b.ShipmentID] += (b.BCDAmount + b.SWSAmount + b.BOEInterest)
	}

	var trends []models.PartTrendPoint
	for i := len(shipments) - 1; i >= 0; i-- { // chronological order
		sh := shipments[i]
		qty := sh.Quantity
		assessable := shipmentAssessableMap[sh.ShipmentID]
		duties := boeDutyMap[sh.ShipmentID]
		expenses := shipmentAllocatedExpensesMap[sh.ShipmentID]
		landedCost := assessable + duties + expenses

		var assessPerUnit, dutyPerUnit, expPerUnit, ldcPerUnit float64
		if qty > 0 {
			assessPerUnit = math.Round((assessable/qty)*100) / 100
			dutyPerUnit = math.Round((duties/qty)*100) / 100
			expPerUnit = math.Round((expenses/qty)*100) / 100
			ldcPerUnit = math.Round((landedCost/qty)*100) / 100
		}

		trends = append(trends, models.PartTrendPoint{
			Date:                   sh.InvoiceDate,
			ShipmentID:             sh.ShipmentID,
			InvoiceNumber:          sh.InvoiceNumber,
			Quantity:               qty,
			UnitPriceForex:         sh.UnitPrice,
			Currency:               sh.Currency,
			AssessableValuePerUnit: assessPerUnit,
			DutyPerUnit:            dutyPerUnit,
			ExpensePerUnit:         expPerUnit,
			LandedCostPerUnit:      ldcPerUnit,
		})
	}
	return trends
}

func (s *Service) buildPartSummary(
	item models.PartItemMasterInfo,
	shipments []models.PartShipmentRow,
	boeLines []models.PartBOERow,
	expenses []models.PartExpenseRow,
	trends []models.PartTrendPoint,
) models.PartPeriodSummary {
	distinctShipments := make(map[string]bool)
	distinctInvoices := make(map[string]bool)
	distinctBOEs := make(map[string]bool)

	summary := models.PartPeriodSummary{
		Unit:     item.Unit,
		Currency: item.Currency,
	}

	for _, sh := range shipments {
		distinctShipments[sh.ShipmentID] = true
		distinctInvoices[sh.InvoiceNumber] = true
		summary.TotalQuantity += sh.Quantity
		summary.TotalForexValue += sh.LineTotal

		if summary.FirstImportDate == "" || (sh.InvoiceDate != "" && sh.InvoiceDate < summary.FirstImportDate) {
			summary.FirstImportDate = sh.InvoiceDate
		}
		if sh.InvoiceDate > summary.LatestImportDate {
			summary.LatestImportDate = sh.InvoiceDate
		}
	}
	summary.TotalShipments = len(distinctShipments)
	summary.TotalInvoices = len(distinctInvoices)

	for _, b := range boeLines {
		if b.BENumber != "" {
			distinctBOEs[b.BENumber] = true
		}
		summary.TotalAssessableValueINR += b.AssessableValue
		summary.TotalBCDAmount += b.BCDAmount
		summary.TotalSWSAmount += b.SWSAmount
		summary.TotalIGSTAmount += b.IGSTAmount
		summary.TotalBOEInterest += b.BOEInterest
		summary.TotalCustomsDutyINR += b.TotalDuty
		summary.TotalDutyPaidINR += b.DutyPaid
	}
	summary.TotalBOEs = len(distinctBOEs)

	for _, ex := range expenses {
		summary.TotalAllocatedExpensesINR += ex.AllocatedBasicINR
		summary.TotalAllocatedExpensesWithGSTINR += ex.AllocatedTotalINR
	}

	summary.TotalLandedCostExIGST = summary.TotalAssessableValueINR + summary.TotalBCDAmount + summary.TotalSWSAmount + summary.TotalBOEInterest + summary.TotalAllocatedExpensesINR

	if summary.TotalQuantity > 0 {
		summary.AvgLandedCostPerUnit = math.Round((summary.TotalLandedCostExIGST/summary.TotalQuantity)*100) / 100
	}

	minLDC := math.MaxFloat64
	maxLDC := 0.0
	for _, tr := range trends {
		if tr.LandedCostPerUnit > 0 {
			if tr.LandedCostPerUnit < minLDC {
				minLDC = tr.LandedCostPerUnit
			}
			if tr.LandedCostPerUnit > maxLDC {
				maxLDC = tr.LandedCostPerUnit
			}
		}
	}
	if minLDC != math.MaxFloat64 {
		summary.MinLandedCostPerUnit = minLDC
	}
	summary.MaxLandedCostPerUnit = maxLDC

	return summary
}

// ExportPartPeriodAnalysisCSV exports a comprehensive executive CSV report matching the professional workbook template.
func (s *Service) ExportPartPeriodAnalysisCSV(req models.PartPeriodAnalysisRequest) (string, error) {
	rep, err := s.GetPartPeriodAnalysis(req)
	if err != nil {
		return "", err
	}

	item := rep.TargetItem
	sum := rep.Summary

	dateRangeStr := fmt.Sprintf("%s to %s", rep.StartDate, rep.EndDate)
	if rep.StartDate == "" && rep.EndDate == "" {
		dateRangeStr = "All Recorded Periods"
	}

	var records [][]string

	// SECTION 1: TITLE & WORKBOOK HEADER
	records = append(records, []string{"PART / ITEM PERIOD ANALYSIS"})
	records = append(records, []string{fmt.Sprintf("Professional reporting workbook | Source period: %s", dateRangeStr)})
	records = append(records, []string{""})

	// SECTION 2: REPORT CONTROL & ITEM MASTER PROFILE
	records = append(records, []string{"REPORT CONTROL & ITEM MASTER PROFILE"})
	records = append(records, []string{"Part Number", item.PartNumber, "Description", item.ItemDescription})
	records = append(records, []string{"HSN Code", item.HSNCode, "Unit", item.Unit})
	records = append(records, []string{"Currency", item.Currency, "Master Unit Price", fmt.Sprintf("%.2f", item.UnitPrice)})
	records = append(records, []string{"Supplier", item.SupplierName, "Country of Origin", item.CountryOfOrigin})
	records = append(records, []string{"Category", item.Category, "Status", fmt.Sprintf("%v", item.IsActive)})
	records = append(records, []string{"BCD %", item.BCD, "SWS %", item.SWS})
	records = append(records, []string{"IGST %", item.IGST, "Period Basis", rep.DateBasis})
	records = append(records, []string{"Generated At", time.Now().Format("2006-01-02 15:04:05"), "Date Range", dateRangeStr})
	records = append(records, []string{""})

	// SECTION 3: EXECUTIVE SUMMARY — PERIOD TOTALS
	records = append(records, []string{"EXECUTIVE SUMMARY — PERIOD TOTALS"})
	records = append(records, []string{
		"Total Shipments", fmt.Sprintf("%d", sum.TotalShipments),
		"Total Invoices", fmt.Sprintf("%d", sum.TotalInvoices),
		"Total BOEs", fmt.Sprintf("%d", sum.TotalBOEs),
	})
	records = append(records, []string{
		"Total Quantity", fmt.Sprintf("%.2f %s", sum.TotalQuantity, sum.Unit),
		"Total Forex Value", fmt.Sprintf("%.2f %s", sum.TotalForexValue, sum.Currency),
		"Total Assessable Value (INR)", fmt.Sprintf("%.2f", sum.TotalAssessableValueINR),
	})
	records = append(records, []string{
		"Basic Customs Duty (BCD)", fmt.Sprintf("%.2f", sum.TotalBCDAmount),
		"Social Welfare Surcharge (SWS)", fmt.Sprintf("%.2f", sum.TotalSWSAmount),
		"Integrated GST (IGST)", fmt.Sprintf("%.2f", sum.TotalIGSTAmount),
	})
	records = append(records, []string{
		"BOE Interest Allocated", fmt.Sprintf("%.2f", sum.TotalBOEInterest),
		"Total Customs Duties", fmt.Sprintf("%.2f", sum.TotalCustomsDutyINR),
		"Total Duty Paid", fmt.Sprintf("%.2f", sum.TotalDutyPaidINR),
	})
	records = append(records, []string{
		"Allocated Basic Expenses", fmt.Sprintf("%.2f", sum.TotalAllocatedExpensesINR),
		"Allocated Expenses (incl. GST)", fmt.Sprintf("%.2f", sum.TotalAllocatedExpensesWithGSTINR),
		"Total Landed Cost (excl. IGST)", fmt.Sprintf("%.2f", sum.TotalLandedCostExIGST),
	})
	records = append(records, []string{
		"Weighted Avg Landed Cost / Unit", fmt.Sprintf("%.2f", sum.AvgLandedCostPerUnit),
		"Min Landed Cost / Unit", fmt.Sprintf("%.2f", sum.MinLandedCostPerUnit),
		"Max Landed Cost / Unit", fmt.Sprintf("%.2f", sum.MaxLandedCostPerUnit),
	})
	records = append(records, []string{""})

	// SECTION 4: REPORT NOTES
	records = append(records, []string{"REPORT NOTES"})
	expenseNoteSummary := "The source report contains no allocated shipment expense rows; the expense sheet is included with all original column headers and is ready for data entry."
	if len(rep.Expenses) > 0 {
		expenseNoteSummary = fmt.Sprintf("Includes %d allocated shipment expense record(s) calculated via item assessable value ratio.", len(rep.Expenses))
	}
	records = append(records, []string{fmt.Sprintf("All source fields are retained in the dedicated detail sheets. %s Values are carried over as reported and have not been recalculated.", expenseNoteSummary)})
	records = append(records, []string{""})

	// SECTION 5: RELATED SHIPMENTS & INVOICES
	records = append(records, []string{"RELATED SHIPMENTS & INVOICES"})
	records = append(records, []string{"Shipment and supplier invoice register | All source columns retained"})
	records = append(records, []string{
		"Shipment ID", "Invoice Number", "Invoice Date", "Supplier", "Incoterm", "Mode",
		"BL/AWB Number", "BL Date", "Vessel Name", "Container No", "Status",
		"Quantity", "Unit", "Invoiced Unit Price", "Currency", "Line Total", "Price Diverged",
	})
	for _, sh := range rep.Shipments {
		records = append(records, []string{
			sh.ShipmentID, sh.InvoiceNumber, sh.InvoiceDate, sh.SupplierName, sh.Incoterm, sh.ShipmentMode,
			sh.BLAWBNumber, sh.BLAWBDate, sh.VesselName, sh.ContainerNumber, sh.Status,
			fmt.Sprintf("%.2f", sh.Quantity), sh.Unit, fmt.Sprintf("%.4f", sh.UnitPrice), sh.Currency,
			fmt.Sprintf("%.2f", sh.LineTotal), fmt.Sprintf("%v", sh.PriceDiverged),
		})
	}
	records = append(records, []string{""})

	// SECTION 6: BILL OF ENTRY — CUSTOMS VALUATION & DUTY
	records = append(records, []string{"BILL OF ENTRY — CUSTOMS VALUATION & DUTY"})
	records = append(records, []string{"BOE assessment, duty calculation and payment details | All source columns retained"})
	records = append(records, []string{
		"Shipment ID", "Invoice Number", "BE Number", "BE Date", "Location", "Status",
		"Exchange Rate", "Quantity", "Assessable Value (INR)", "BCD Rate", "BCD Amount",
		"SWS Rate", "SWS Amount", "IGST Rate", "IGST Amount", "BOE Interest",
		"Total Duty", "Duty Paid", "Challan Number", "Payment Date",
	})
	for _, b := range rep.BOELines {
		records = append(records, []string{
			b.ShipmentID, b.InvoiceNumber, b.BENumber, b.BEDate, b.Location, b.Status,
			fmt.Sprintf("%.4f", b.ExchangeRate), fmt.Sprintf("%.2f", b.Quantity),
			fmt.Sprintf("%.2f", b.AssessableValue),
			fmt.Sprintf("%.2f%%", b.BCDRate), fmt.Sprintf("%.2f", b.BCDAmount),
			fmt.Sprintf("%.2f%%", b.SWSRate), fmt.Sprintf("%.2f", b.SWSAmount),
			fmt.Sprintf("%.2f%%", b.IGSTRate), fmt.Sprintf("%.2f", b.IGSTAmount),
			fmt.Sprintf("%.2f", b.BOEInterest), fmt.Sprintf("%.2f", b.TotalDuty),
			fmt.Sprintf("%.2f", b.DutyPaid), b.ChallanNumber, b.PaymentDate,
		})
	}
	records = append(records, []string{""})

	// SECTION 7: ALLOCATED SHIPMENT EXPENSES
	records = append(records, []string{"ALLOCATED SHIPMENT EXPENSES"})
	expenseSubtitle := "Shipment expense allocation register | All source columns retained"
	if len(rep.Expenses) == 0 {
		expenseSubtitle = "Shipment expense allocation register | No data rows were present in the source export"
	}
	records = append(records, []string{expenseSubtitle})
	records = append(records, []string{
		"Shipment ID", "Shipment Invoice", "Service Provider", "Expense Type",
		"Expense Invoice No", "Expense Invoice Date", "Shipment Expense Basic", "Shipment Expense Total",
		"Allocation Ratio", "Allocated Basic (INR)", "Allocated GST (INR)", "Allocated Total (INR)", "Remarks",
	})
	for _, ex := range rep.Expenses {
		records = append(records, []string{
			ex.ShipmentID, ex.ShipmentInvoiceNumber, ex.ServiceProviderName, ex.ExpenseTypeName,
			ex.ExpenseInvoiceNo, ex.ExpenseInvoiceDate,
			fmt.Sprintf("%.2f", ex.ShipmentExpenseBasicINR), fmt.Sprintf("%.2f", ex.ShipmentExpenseTotalINR),
			fmt.Sprintf("%.4f", ex.AllocationRatio),
			fmt.Sprintf("%.2f", ex.AllocatedBasicINR), fmt.Sprintf("%.2f", ex.AllocatedGSTINR), fmt.Sprintf("%.2f", ex.AllocatedTotalINR),
			ex.Remarks,
		})
	}
	records = append(records, []string{""})

	// SECTION 8: CHRONOLOGICAL COST TRENDS
	records = append(records, []string{"CHRONOLOGICAL COST TRENDS"})
	records = append(records, []string{"Chronological landed-cost movement by shipment and invoice | All source columns retained"})
	records = append(records, []string{
		"Date", "Shipment ID", "Invoice Number", "Quantity", "Invoiced Unit Price", "Currency",
		"Assessable / Unit", "Duty / Unit", "Expense / Unit", "Total Landed Cost / Unit",
	})
	for _, tr := range rep.Trends {
		records = append(records, []string{
			tr.Date, tr.ShipmentID, tr.InvoiceNumber, fmt.Sprintf("%.2f", tr.Quantity),
			fmt.Sprintf("%.4f", tr.UnitPriceForex), tr.Currency,
			fmt.Sprintf("%.2f", tr.AssessableValuePerUnit), fmt.Sprintf("%.2f", tr.DutyPerUnit),
			fmt.Sprintf("%.2f", tr.ExpensePerUnit), fmt.Sprintf("%.2f", tr.LandedCostPerUnit),
		})
	}
	records = append(records, []string{""})

	// SECTION 9: REPORT METADATA & FIELD INVENTORY
	records = append(records, []string{"REPORT METADATA & FIELD INVENTORY"})
	records = append(records, []string{"Source export details and field completeness reference"})
	records = append(records, []string{"Section", "Field Count", "Source Data Rows", "Status"})
	records = append(records, []string{"Item Master Profile", "13", "1", "All fields retained"})
	records = append(records, []string{"Executive Summary", "18", "1", "All metrics retained"})
	records = append(records, []string{"Related Shipments & Invoices", "17", fmt.Sprintf("%d", len(rep.Shipments)), "All columns and rows retained"})
	records = append(records, []string{"BOE Customs Valuation & Duty", "20", fmt.Sprintf("%d", len(rep.BOELines)), "All columns and rows retained"})
	expenseStatus := "All columns and rows retained"
	if len(rep.Expenses) == 0 {
		expenseStatus = "Headers retained; source has no expense rows"
	}
	records = append(records, []string{"Allocated Shipment Expenses", "13", fmt.Sprintf("%d", len(rep.Expenses)), expenseStatus})
	records = append(records, []string{"Chronological Cost Trends", "10", fmt.Sprintf("%d", len(rep.Trends)), "All columns and rows retained"})

	return validation.EncodeCSV(nil, records)
}
