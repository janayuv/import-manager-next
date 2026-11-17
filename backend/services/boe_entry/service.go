package boe_entry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/validation"
)

var allowedReconStatuses = map[string]bool{
	"Awaiting BOE Data": true,
	"Discrepancy Found": true,
	"Reconciled":        true,
	"Investigation":     true,
	"Closed":            true,
}

// Service implements interactive BOE valuation, CSV override validation, crash-safe persistence,
// and customs-clearance shipment transition.
type Service struct {
	db       *sql.DB
	shipRepo *repositories.ShipmentRepository
	invRepo  *repositories.InvoiceRepository
	sec      *security.Service
	audit    *audit.Service
}

// NewService constructs a boe_entry Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{
		db:       db,
		shipRepo: repositories.NewShipmentRepository(db),
		invRepo:  repositories.NewInvoiceRepository(db),
		sec:      sec,
		audit:    auditSvc,
	}
}

// PreviewCalculation computes duties on demand without persisting to DB.
func (s *Service) PreviewCalculation(params models.BOECalcParams) (models.BOECalculationResult, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.BOECalculationResult{}, err
	}
	return CalculateDuties(params)
}

// LoadShipmentContext loads shipment info, saved BOE calculation if present, or invoice lines + Item Master rates.
func (s *Service) LoadShipmentContext(shipmentID string) (models.BOECalculationContext, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.BOECalculationContext{}, err
	}
	sh, err := s.shipRepo.GetByID(shipmentID)
	if err != nil {
		return models.BOECalculationContext{}, err
	}

	// Check existing saved calculation first.
	var calcID, boeID, status, formJSON, itemJSON, resJSON string
	err = s.db.QueryRow(`
		SELECT id, COALESCE(boe_id,''), status, form_values_json, item_inputs_json, calculation_result_json
		FROM boe_calculations
		WHERE shipment_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, sh.ID).Scan(&calcID, &boeID, &status, &formJSON, &itemJSON, &resJSON)
	if err == nil {
		var params models.BOECalcParams
		var result models.BOECalculationResult
		if json.Unmarshal([]byte(formJSON), &params) == nil && json.Unmarshal([]byte(resJSON), &result) == nil {
			params.ID = calcID
			params.BOEID = boeID
			params.Status = status
			if len(params.LineItems) == 0 {
				_ = json.Unmarshal([]byte(itemJSON), &params.LineItems)
			}
			return models.BOECalculationContext{
				Shipment: sh,
				Params:   params,
				Result:   result,
				HasSaved: true,
			}, nil
		}
	}

	// Otherwise build default params from linked invoice lines (or finalized first).
	invs, err := s.invRepo.ListByShipment(sh.ID)
	if err != nil {
		return models.BOECalculationContext{}, err
	}
	if len(invs) == 0 {
		return models.BOECalculationContext{}, fmt.Errorf("shipment %s has no commercial invoice lines; create an invoice first", sh.ID)
	}
	targetInv := invs[0]
	for _, iv := range invs {
		if strings.EqualFold(iv.Status, "Finalized") {
			targetInv = iv
			break
		}
	}
	detail, err := s.invRepo.GetDetail(targetInv.ID)
	if err != nil {
		return models.BOECalculationContext{}, err
	}

	lineInputs := make([]models.BOECalculationItemInput, 0, len(detail.Lines))
	for _, ln := range detail.Lines {
		lineInputs = append(lineInputs, models.BOECalculationItemInput{
			ItemID:            ln.ItemID,
			PartNumber:        ln.PartNumber,
			Description:       ln.ItemDescription,
			HSNCode:           ln.HSNCode,
			Unit:              ln.Unit,
			Quantity:          ln.Quantity,
			UnitPrice:         ln.UnitPrice,
			ActualBCDRate:     ln.DutyPercent,
			ActualSWSRate:     ln.SWSPercent,
			ActualIGSTRate:    ln.IGSTPercent,
			CalculationMethod: "Standard",
			BOEBCDRate:        ln.DutyPercent,
			BOESWSRate:        ln.SWSPercent,
			BOEIGSTRate:       ln.IGSTPercent,
		})
	}

	incoterm := strings.ToUpper(strings.TrimSpace(sh.Incoterm))
	if incoterm == "" {
		incoterm = "CIF"
	}
	rule := GetIncotermRule(incoterm)
	insMode := "rate"
	insRate := 0.015 // Default 0.015% Open Cover Policy
	if rule.InsuranceIncluded {
		insMode = "included"
		insRate = 0
	}
	params := models.BOECalcParams{
		ShipmentID:          sh.ID,
		SupplierName:        sh.SupplierName,
		InvoiceNumber:       sh.InvoiceNumber,
		InvoiceCurrency:     sh.InvoiceCurrency,
		Incoterm:            sh.Incoterm,
		Status:              "Awaiting BOE Data",
		ExchangeRate:        83.5,
		FreightCost:         0,
		EXWCost:             0,
		OriginInlandFreight: 0,
		ExportHandling:      0,
		LoadingCharges:      0,
		PackingCharges:      0,
		OtherRule10Additions: 0,
		InsuranceMode:       insMode,
		ActualInsurance:     0,
		InsuranceRate:       insRate,
		RoDTEPScripBalance:  0,
		Interest:            0,
		LineItems:           lineInputs,
	}
	result, err := CalculateDuties(params)
	if err != nil {
		return models.BOECalculationContext{}, err
	}
	return models.BOECalculationContext{
		Shipment: sh,
		Params:   params,
		Result:   result,
		HasSaved: false,
	}, nil
}

// LoadCalculationContext loads a specific BOE calculation by ID along with its parent shipment context.
func (s *Service) LoadCalculationContext(calcID string) (models.BOECalculationContext, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.BOECalculationContext{}, err
	}
	var shipmentID, boeID, status, formJSON, itemJSON, resJSON string
	err := s.db.QueryRow(`
		SELECT shipment_id, COALESCE(boe_id,''), status, form_values_json, item_inputs_json, calculation_result_json
		FROM boe_calculations
		WHERE id = ? AND deleted_at IS NULL`, strings.TrimSpace(calcID)).Scan(&shipmentID, &boeID, &status, &formJSON, &itemJSON, &resJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.BOECalculationContext{}, fmt.Errorf("BOE calculation %q not found", calcID)
		}
		return models.BOECalculationContext{}, err
	}
	sh, err := s.shipRepo.GetByID(shipmentID)
	if err != nil {
		return models.BOECalculationContext{}, err
	}
	var params models.BOECalcParams
	var result models.BOECalculationResult
	if err := json.Unmarshal([]byte(formJSON), &params); err != nil {
		return models.BOECalculationContext{}, fmt.Errorf("unmarshal form params: %w", err)
	}
	if err := json.Unmarshal([]byte(resJSON), &result); err != nil {
		return models.BOECalculationContext{}, fmt.Errorf("unmarshal result: %w", err)
	}
	params.ID = calcID
	params.BOEID = boeID
	params.Status = status
	if len(params.LineItems) == 0 {
		_ = json.Unmarshal([]byte(itemJSON), &params.LineItems)
	}
	return models.BOECalculationContext{
		Shipment: sh,
		Params:   params,
		Result:   result,
		HasSaved: true,
	}, nil
}

// ParseOverrideCSV parses rate overrides (`partNo, calculationMethod, boeBcdRate, boeSwsRate, boeIgstRate`),
// skipping unknown parts or IGST mismatches with warnings.
func (s *Service) ParseOverrideCSV(csvContent string, currentItems []models.BOECalculationItemInput) (models.BOEOverrideImportResult, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.BOEOverrideImportResult{}, err
	}
	headers, rows, err := validation.ParseCSV(csvContent, true)
	if err != nil {
		return models.BOEOverrideImportResult{}, err
	}
	idx := validation.HeaderIndex(headers)
	outItems := append([]models.BOECalculationItemInput(nil), currentItems...)
	itemIndex := make(map[string]int, len(outItems))
	for i, it := range outItems {
		itemIndex[strings.ToUpper(strings.TrimSpace(it.PartNumber))] = i
	}

	res := models.BOEOverrideImportResult{}
	for i, row := range rows {
		rowNum := i + 2
		partNo := validation.CellByAliases(row, idx, "partNo", "partNumber", "part_number")
		methodRaw := validation.CellByAliases(row, idx, "calculationMethod", "method")
		bcdStr := validation.CellByAliases(row, idx, "boeBcdRate", "boeBcd")
		swsStr := validation.CellByAliases(row, idx, "boeSwsRate", "boeSws")
		igstStr := validation.CellByAliases(row, idx, "boeIgstRate", "boeIgst")

		pos, ok := itemIndex[strings.ToUpper(strings.TrimSpace(partNo))]
		if !ok {
			res.SkippedCount++
			res.Warnings = append(res.Warnings, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("unknown partNo %q skipped", partNo),
			})
			continue
		}

		method, mErr := NormalizeMethod(methodRaw)
		if mErr != nil {
			res.SkippedCount++
			res.Warnings = append(res.Warnings, models.CSVImportRowError{Row: rowNum, Message: mErr.Error()})
			continue
		}
		boeBCD, err1 := strconv.ParseFloat(bcdStr, 64)
		boeSWS, err2 := strconv.ParseFloat(swsStr, 64)
		boeIGST, err3 := strconv.ParseFloat(igstStr, 64)
		if err1 != nil || err2 != nil || err3 != nil || boeBCD < 0 || boeSWS < 0 || boeIGST < 0 {
			res.SkippedCount++
			res.Warnings = append(res.Warnings, models.CSVImportRowError{
				Row:     rowNum,
				Message: "invalid negative or non-numeric rate",
			})
			continue
		}

		// IGST cell is read-only / statutory; skip if override IGST diverges from actual IGST.
		if math.Abs(boeIGST-outItems[pos].ActualIGSTRate) > 0.001 {
			res.SkippedCount++
			res.Warnings = append(res.Warnings, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("row %q skipped: boeIgstRate (%.2f) must match actualIgstRate (%.2f)", partNo, boeIGST, outItems[pos].ActualIGSTRate),
			})
			continue
		}

		outItems[pos].CalculationMethod = method
		outItems[pos].BOEBCDRate = boeBCD
		outItems[pos].BOESWSRate = boeSWS
		res.AppliedCount++
	}
	res.Items = outItems
	return res, nil
}

// SaveCalculation validates duties server-side, enforces BCD override & dutyPaid reconciliation blockers,
// guards JSON size (<1MB), records recovery write markers, persists boe_calculations + boe_items,
// and transitions non-terminal shipments -> customs-clearance (or ready-dly).
func (s *Service) SaveCalculation(params models.BOECalcParams) (models.BOECalculationSummary, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.BOECalculationSummary{}, err
	}

	sh, err := s.shipRepo.GetByID(params.ShipmentID)
	if err != nil {
		return models.BOECalculationSummary{}, err
	}
	if sh.IsFrozen {
		return models.BOECalculationSummary{}, fmt.Errorf("shipment %s is frozen and cannot accept BOE updates", sh.ID)
	}

	params.SupplierName = sh.SupplierName
	params.InvoiceNumber = sh.InvoiceNumber
	params.InvoiceCurrency = sh.InvoiceCurrency
	params.Incoterm = sh.Incoterm

	result, err := CalculateDuties(params)
	if err != nil {
		return models.BOECalculationSummary{}, err
	}

	// Blocker 1: boeBcdRate > actualBcdRate when > 0
	if result.HasBlockingBCDOverride {
		return models.BOECalculationSummary{}, errors.New("save blocked: one or more line items have boeBcdRate > actualBcdRate")
	}

	// Blocker 2: Linked BOE dutyPaid reconciliation check
	boeID := strings.TrimSpace(params.BOEID)
	if boeID != "" {
		var dutyPaid float64
		err := s.db.QueryRow(`SELECT COALESCE(duty_paid, 0) FROM boe_details WHERE id = ? AND deleted_at IS NULL`, boeID).Scan(&dutyPaid)
		if errors.Is(err, sql.ErrNoRows) {
			return models.BOECalculationSummary{}, fmt.Errorf("linked BOE %q not found", boeID)
		}
		if err != nil {
			return models.BOECalculationSummary{}, err
		}
		paidRound := math.Round(dutyPaid)
		grossRound := math.Round(result.CustomsDutyTotal)
		cashRound := math.Round(result.CashCustomsDutyTotal)
		if paidRound != grossRound && paidRound != cashRound {
			return models.BOECalculationSummary{}, fmt.Errorf(
				"save blocked: linked BOE dutyPaid (%.0f) does not match computed customsDutyTotal (%.0f) or cash challan (%.0f)",
				paidRound, grossRound, cashRound,
			)
		}
	}

	reconStatus := strings.TrimSpace(params.Status)
	if reconStatus == "" || (boeID != "" && reconStatus == "Awaiting BOE Data") {
		if boeID != "" {
			reconStatus = "Reconciled"
		} else {
			reconStatus = "Awaiting BOE Data"
		}
	}
	if !allowedReconStatuses[reconStatus] {
		return models.BOECalculationSummary{}, fmt.Errorf("invalid reconciliation status %q", reconStatus)
	}

	formBlob, err := json.Marshal(params)
	if err != nil {
		return models.BOECalculationSummary{}, err
	}
	itemsBlob, err := json.Marshal(params.LineItems)
	if err != nil {
		return models.BOECalculationSummary{}, err
	}
	resBlob, err := json.Marshal(result)
	if err != nil {
		return models.BOECalculationSummary{}, err
	}
	if len(formBlob)+len(itemsBlob)+len(resBlob) > MaxBOEPayloadBytes {
		return models.BOECalculationSummary{}, fmt.Errorf("BOE JSON payload exceeds 1 MB limit (%d bytes)", len(formBlob)+len(itemsBlob)+len(resBlob))
	}

	calcID := strings.TrimSpace(params.ID)
	if calcID == "" {
		calcID = "BOEC-" + strings.ToUpper(uuid.NewString()[:8])
	}
	recoveryID := "REC-" + strings.ToUpper(uuid.NewString()[:8])

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		if _, rErr := tx.Exec(`
			INSERT INTO boe_write_recovery (id, boe_calculation_id, operation, status)
			VALUES (?, ?, 'SAVE_BOE_CALCULATION', 'PENDING')`,
			recoveryID, calcID,
		); rErr != nil {
			return rErr
		}

		var exists int
		if qErr := tx.QueryRow(`SELECT COUNT(*) FROM boe_calculations WHERE id = ?`, calcID).Scan(&exists); qErr != nil {
			return qErr
		}
		if exists > 0 {
			if _, uErr := tx.Exec(`
				UPDATE boe_calculations
				SET shipment_id = ?, boe_id = ?, supplier_name = ?, invoice_number = ?, status = ?,
				    form_values_json = ?, item_inputs_json = ?, calculation_result_json = ?,
				    deleted_at = NULL, deleted_by = NULL
				WHERE id = ?`,
				sh.ID, nullStr(boeID), params.SupplierName, params.InvoiceNumber, reconStatus,
				string(formBlob), string(itemsBlob), string(resBlob), calcID,
			); uErr != nil {
				return uErr
			}
			if _, dErr := tx.Exec(`DELETE FROM boe_items WHERE boe_calculation_id = ?`, calcID); dErr != nil {
				return dErr
			}
		} else {
			if _, iErr := tx.Exec(`
				INSERT INTO boe_calculations (
					id, shipment_id, boe_id, supplier_name, invoice_number, status,
					form_values_json, item_inputs_json, calculation_result_json
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				calcID, sh.ID, nullStr(boeID), params.SupplierName, params.InvoiceNumber, reconStatus,
				string(formBlob), string(itemsBlob), string(resBlob),
			); iErr != nil {
				return iErr
			}
		}

		for _, ln := range result.Items {
			rowID := "BOEI-" + strings.ToUpper(uuid.NewString()[:8])
			if _, lErr := tx.Exec(`
				INSERT INTO boe_items (
					id, boe_calculation_id, item_id, assessable_value, bcd_rate, sws_rate, igst_rate, total
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				rowID, calcID, ln.ItemID, ln.AssessableValue, ln.BOEBCDRate, ln.BOESWSRate, ln.BOEIGSTRate, ln.TotalLineDuty,
			); lErr != nil {
				return lErr
			}
		}

		// Transition shipment status -> customs-clearance unless already delivered/terminal.
		curSt := strings.ToLower(strings.TrimSpace(sh.Status))
		if curSt != "delivered" && curSt != "ready-dly" && curSt != "cancelled" && curSt != "closed" && curSt != "completed" {
			if _, sErr := tx.Exec(`UPDATE shipments SET status = 'customs-clearance' WHERE id = ? AND deleted_at IS NULL`, sh.ID); sErr != nil {
				return sErr
			}
		}

		if _, mErr := tx.Exec(`UPDATE boe_write_recovery SET status = 'COMPLETED' WHERE id = ?`, recoveryID); mErr != nil {
			return mErr
		}
		return nil
	})
	if err != nil {
		return models.BOECalculationSummary{}, fmt.Errorf("save BOE calculation: %w", err)
	}

	if boeID != "" {
		_, _ = s.shipRepo.UpdateReadyForDelivery("")
	}
	_ = s.audit.RecordTableAudit("boe_calculations", calcID, "UPSERT", sess.UserID, nil, result, map[string]any{
		"shipmentId": sh.ID,
		"boeId":      boeID,
	})

	return models.BOECalculationSummary{
		ID:               calcID,
		ShipmentID:       sh.ID,
		BOEID:            boeID,
		SupplierName:     params.SupplierName,
		InvoiceNumber:    params.InvoiceNumber,
		Status:           reconStatus,
		CustomsDutyTotal: result.CustomsDutyTotal,
		AssessableTotal:  result.TotalAssessableValue,
	}, nil
}

func nullStr(s string) any {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return t
}
