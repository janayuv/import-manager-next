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
	"time"

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
	defaultExchangeRate := 84.15
	cbicNotif := ""
	refDate := sh.InvoiceDate

	// Prioritize statutory BOE filing date if a linked BOE detail exists for this shipment
	var linkedBEDate string
	if err := s.db.QueryRow(`
		SELECT b.be_date
		FROM boe_details b
		JOIN boe_calculations bc ON bc.boe_id = b.id
		WHERE bc.shipment_id = ? AND bc.deleted_at IS NULL AND b.be_date != ''
		ORDER BY b.be_date DESC LIMIT 1`, sh.ID,
	).Scan(&linkedBEDate); err == nil && strings.TrimSpace(linkedBEDate) != "" {
		refDate = strings.TrimSpace(linkedBEDate)
	}

	if refDate == "" {
		refDate = time.Now().UTC().Format("2006-01-02")
	}
	var queriedRate float64
	var queriedNotif string
	if err := s.db.QueryRow(`
		SELECT exchange_rate, notification_number
		FROM cbic_exchange_rates
		WHERE currency_code = ?
		  AND effective_from <= ?
		  AND (effective_to IS NULL OR effective_to >= ?)
		  AND verification_status = 'Verified'
		ORDER BY effective_from DESC, id DESC LIMIT 1`,
		strings.ToUpper(strings.TrimSpace(sh.InvoiceCurrency)), refDate, refDate,
	).Scan(&queriedRate, &queriedNotif); err == nil && queriedRate > 0 {
		defaultExchangeRate = queriedRate
		cbicNotif = queriedNotif
	}

	params := models.BOECalcParams{
		ShipmentID:             sh.ID,
		SupplierName:           sh.SupplierName,
		InvoiceNumber:          sh.InvoiceNumber,
		InvoiceCurrency:        sh.InvoiceCurrency,
		Incoterm:               sh.Incoterm,
		Status:                 "Awaiting BOE Data",
		ExchangeRate:           defaultExchangeRate,
		CBICNotificationNumber: cbicNotif,
		FreightCost:            0,
		EXWCost:                0,
		OriginInlandFreight:    0,
		ExportHandling:         0,
		LoadingCharges:         0,
		PackingCharges:         0,
		OtherRule10Additions:   0,
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

var (
	overridePartAliases = []string{
		"partNo", "partNumber", "part_number", "part_no", "partno", "part",
		"item", "itemCode", "item_code", "itemno", "item_no", "partcode",
		"part_code", "part#", "material", "materialno",
	}
	overrideMethodAliases = []string{
		"calculationMethod", "calculation_method", "calcMethod", "calc_method",
		"method", "scheme", "dutymethod", "duty_method", "type",
	}
	overrideBcdAliases = []string{
		"boeBcdRate", "boe_bcd_rate", "boeBcd", "boe_bcd", "bcdRate",
		"bcd_rate", "bcd", "bcd%", "basiccustomsduty", "customsduty", "customsdutyrate",
	}
	overrideSwsAliases = []string{
		"boeSwsRate", "boe_sws_rate", "boeSws", "boe_sws", "swsRate",
		"sws_rate", "sws", "sws%", "socialwelfare", "socialwelfaresurcharge",
	}
	overrideIgstAliases = []string{
		"boeIgstRate", "boe_igst_rate", "boeIgst", "boe_igst", "igstRate",
		"igst_rate", "igst", "igst%", "integratedgst", "gst", "gstrate", "gst%",
	}
)

func cleanPartNumber(p string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(p)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func cleanNumberStr(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimPrefix(s, "₹")
	s = strings.TrimPrefix(s, "$")
	return strings.TrimSpace(s)
}

func isRecognizedOverrideHeader(headers []string) bool {
	idx := validation.HeaderIndex(headers)
	for _, a := range overridePartAliases {
		if _, ok := idx[validation.NormalizeHeaderKey(a)]; ok {
			return true
		}
	}
	for _, a := range overrideBcdAliases {
		if _, ok := idx[validation.NormalizeHeaderKey(a)]; ok {
			return true
		}
	}
	return false
}

// ParseOverrideCSV parses rate overrides (`partNo, calculationMethod, boeBcdRate, boeSwsRate, boeIgstRate`),
// supporting TSV/CSV with or without headers, skipping unknown parts or IGST mismatches with clear warnings.
func (s *Service) ParseOverrideCSV(csvContent string, currentItems []models.BOECalculationItemInput) (models.BOEOverrideImportResult, error) {
	if s.sec != nil {
		if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
			return models.BOEOverrideImportResult{}, err
		}
	}
	headers, rows, err := validation.ParseCSV(csvContent, true)
	if err != nil {
		return models.BOEOverrideImportResult{}, err
	}

	outItems := append([]models.BOECalculationItemInput(nil), currentItems...)
	itemIndex := make(map[string]int, len(outItems)*2)
	cleanItemIndex := make(map[string]int, len(outItems)*2)
	for i, it := range outItems {
		rawKey := strings.ToUpper(strings.TrimSpace(it.PartNumber))
		if rawKey != "" {
			itemIndex[rawKey] = i
		}
		cleanKey := cleanPartNumber(it.PartNumber)
		if cleanKey != "" {
			cleanItemIndex[cleanKey] = i
		}
	}

	hasHeader := isRecognizedOverrideHeader(headers)
	var allRows [][]string
	var startRowOffset int
	var idx map[string]int

	if hasHeader {
		idx = validation.HeaderIndex(headers)
		allRows = rows
		startRowOffset = 2
	} else {
		allRows = append([][]string{headers}, rows...)
		startRowOffset = 1
	}

	res := models.BOEOverrideImportResult{}
	for i, row := range allRows {
		rowNum := i + startRowOffset

		// Skip completely blank rows
		if len(row) == 0 {
			continue
		}
		allBlank := true
		for _, c := range row {
			if strings.TrimSpace(c) != "" {
				allBlank = false
				break
			}
		}
		if allBlank {
			continue
		}

		var partNo, methodRaw, bcdStr, swsStr, igstStr string

		if hasHeader {
			partNo = validation.CellByAliases(row, idx, overridePartAliases...)
			methodRaw = validation.CellByAliases(row, idx, overrideMethodAliases...)
			bcdStr = validation.CellByAliases(row, idx, overrideBcdAliases...)
			swsStr = validation.CellByAliases(row, idx, overrideSwsAliases...)
			igstStr = validation.CellByAliases(row, idx, overrideIgstAliases...)

			// Positional fallback if some columns weren't matched by header aliases
			if partNo == "" && len(row) > 0 {
				partNo = strings.TrimSpace(row[0])
			}
			if bcdStr == "" && len(row) >= 3 {
				bcdStr = strings.TrimSpace(row[2])
			}
		} else {
			// Headerless row: infer columns based on count and cell values
			partNo = strings.TrimSpace(row[0])

			if len(row) >= 5 {
				col1Clean := cleanNumberStr(row[1])
				_, errCol1Num := strconv.ParseFloat(col1Clean, 64)
				isCol1Method := false
				switch strings.ToLower(strings.TrimSpace(row[1])) {
				case "standard", "cepa", "rodtep":
					isCol1Method = true
				}

				if errCol1Num == nil && !isCol1Method {
					// 5 columns: [partNo, bcd, sws, igst, other]
					methodRaw = "Standard"
					bcdStr = row[1]
					swsStr = row[2]
					igstStr = row[3]
				} else {
					// 5 columns: [partNo, method, bcd, sws, igst]
					methodRaw = row[1]
					bcdStr = row[2]
					swsStr = row[3]
					igstStr = row[4]
				}
			} else if len(row) == 4 {
				col1Clean := cleanNumberStr(row[1])
				_, errCol1Num := strconv.ParseFloat(col1Clean, 64)
				if errCol1Num == nil {
					// 4 columns: [partNo, bcd, sws, igst]
					methodRaw = "Standard"
					bcdStr = row[1]
					swsStr = row[2]
					igstStr = row[3]
				} else {
					// 4 columns: [partNo, method, bcd, sws]
					methodRaw = row[1]
					bcdStr = row[2]
					swsStr = row[3]
				}
			} else if len(row) == 3 {
				// 3 columns: [partNo, bcd, sws]
				methodRaw = "Standard"
				bcdStr = row[1]
				swsStr = row[2]
			} else if len(row) == 2 {
				// 2 columns: [partNo, bcd]
				methodRaw = "Standard"
				bcdStr = row[1]
			}
		}

		if partNo == "" {
			res.SkippedCount++
			res.Warnings = append(res.Warnings, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("row %d has empty part number", rowNum),
			})
			continue
		}

		// Find part in current line items
		pos, ok := itemIndex[strings.ToUpper(strings.TrimSpace(partNo))]
		if !ok {
			pos, ok = cleanItemIndex[cleanPartNumber(partNo)]
		}
		if !ok {
			res.SkippedCount++
			res.Warnings = append(res.Warnings, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("unknown partNo %q (not found in invoice items)", partNo),
			})
			continue
		}

		method, mErr := NormalizeMethod(methodRaw)
		if mErr != nil {
			res.SkippedCount++
			res.Warnings = append(res.Warnings, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("part %q: %v", partNo, mErr),
			})
			continue
		}

		cleanedBCD := cleanNumberStr(bcdStr)
		boeBCD, err1 := strconv.ParseFloat(cleanedBCD, 64)
		if err1 != nil || boeBCD < 0 {
			res.SkippedCount++
			res.Warnings = append(res.Warnings, models.CSVImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("part %q: invalid BCD rate %q", partNo, bcdStr),
			})
			continue
		}

		var boeSWS float64 = 10.0
		if cleanedSWS := cleanNumberStr(swsStr); cleanedSWS != "" {
			var err2 error
			boeSWS, err2 = strconv.ParseFloat(cleanedSWS, 64)
			if err2 != nil || boeSWS < 0 {
				res.SkippedCount++
				res.Warnings = append(res.Warnings, models.CSVImportRowError{
					Row:     rowNum,
					Message: fmt.Sprintf("part %q: invalid SWS rate %q", partNo, swsStr),
				})
				continue
			}
		}

		var boeIGST float64
		if cleanedIGST := cleanNumberStr(igstStr); cleanedIGST != "" {
			var err3 error
			boeIGST, err3 = strconv.ParseFloat(cleanedIGST, 64)
			if err3 != nil || boeIGST < 0 {
				res.SkippedCount++
				res.Warnings = append(res.Warnings, models.CSVImportRowError{
					Row:     rowNum,
					Message: fmt.Sprintf("part %q: invalid IGST rate %q", partNo, igstStr),
				})
				continue
			}
		} else {
			boeIGST = outItems[pos].ActualIGSTRate
		}

		// IGST cell is statutory; skip if override IGST diverges from actual IGST
		if outItems[pos].ActualIGSTRate > 0 && math.Abs(boeIGST-outItems[pos].ActualIGSTRate) > 0.001 {
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
		if outItems[pos].ActualIGSTRate <= 0 && boeIGST > 0 {
			outItems[pos].ActualIGSTRate = boeIGST
		}
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
		grossDiff := math.Abs(paidRound - grossRound)
		cashDiff := math.Abs(paidRound - cashRound)
		// Allow ±1 rupee statutory round-off tolerance under Section 154 Customs Act
		if grossDiff > 1 && cashDiff > 1 {
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
