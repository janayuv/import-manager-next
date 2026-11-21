package invoice_wizard

import (
	"context"
	"database/sql"
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
)

// Service implements 3-step Invoice Wizard multi-line paste parsing, precision validation,
// Draft save, Finalize integer-cent matching gate, shipment transition, and optional freeze.
type Service struct {
	db       *sql.DB
	invRepo  *repositories.InvoiceRepository
	shipRepo *repositories.ShipmentRepository
	itemRepo *repositories.ItemRepository
	sec      *security.Service
	audit    *audit.Service
}

// NewService constructs an invoice_wizard Service.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	return &Service{
		db:       db,
		invRepo:  repositories.NewInvoiceRepository(db),
		shipRepo: repositories.NewShipmentRepository(db),
		itemRepo: repositories.NewItemRepository(db),
		sec:      sec,
		audit:    auditSvc,
	}
}

// SetDB rebinds the service and its repositories to an active database handle.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
	s.invRepo = repositories.NewInvoiceRepository(db)
	s.shipRepo = repositories.NewShipmentRepository(db)
	s.itemRepo = repositories.NewItemRepository(db)
}

// ParseMultiLinePaste parses clipboard text supporting \t, comma, semicolon, or pipe delimiters
// across Format A (partNo, qty, unitPrice) and Format B (partNo, desc, unit, qty, unitPrice)
// and resolves Item Master matches and price divergence flags.
func (s *Service) ParseMultiLinePaste(rawText string) ([]models.ParsedPasteLine, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(rawText, "\r\n", "\n"), "\n")
	var out []models.ParsedPasteLine

	for idx, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		cols := splitPasteColumns(trimmed)
		if len(cols) < 2 {
			out = append(out, models.ParsedPasteLine{
				Row:   idx + 1,
				Error: "expected at least 2 columns (partNumber, quantity)",
			})
			continue
		}

		// Skip obvious header line if first row contains words like "part" and "qty".
		if idx == 0 && strings.Contains(strings.ToLower(cols[0]), "part") &&
			(strings.Contains(strings.ToLower(trimmed), "qty") || strings.Contains(strings.ToLower(trimmed), "quantity")) {
			continue
		}

		partNo := strings.TrimSpace(cols[0])
		var qtyStr, priceStr string
		switch {
		case len(cols) >= 5:
			// Format B: partNumber, description, unit, qty, unitPrice
			qtyStr = cols[len(cols)-2]
			priceStr = cols[len(cols)-1]
		case len(cols) == 4:
			// partNumber, descOrUnit, qty, unitPrice
			qtyStr = cols[2]
			priceStr = cols[3]
		case len(cols) == 3:
			// Format A: partNumber, qty, unitPrice
			qtyStr = cols[1]
			priceStr = cols[2]
		default:
			// 2 cols: partNumber, qty (use master price)
			qtyStr = cols[1]
		}

		qty, err := strconv.ParseFloat(strings.ReplaceAll(qtyStr, ",", ""), 64)
		if err != nil || qty <= 0 {
			out = append(out, models.ParsedPasteLine{
				Row:        idx + 1,
				PartNumber: partNo,
				Error:      fmt.Sprintf("invalid quantity %q", qtyStr),
			})
			continue
		}

		item, findErr := s.itemRepo.FindByPartNumber(partNo, "")
		if findErr != nil {
			return nil, findErr
		}
		if item == nil {
			out = append(out, models.ParsedPasteLine{
				Row:        idx + 1,
				PartNumber: partNo,
				Quantity:   qty,
				Error:      fmt.Sprintf("part number %q not found in Item Master", partNo),
			})
			continue
		}

		unitPrice := item.UnitPrice
		if strings.TrimSpace(priceStr) != "" {
			parsedPrice, pErr := strconv.ParseFloat(strings.ReplaceAll(priceStr, ",", ""), 64)
			if pErr != nil || parsedPrice < 0 {
				out = append(out, models.ParsedPasteLine{
					Row:        idx + 1,
					PartNumber: partNo,
					Quantity:   qty,
					Error:      fmt.Sprintf("invalid unitPrice %q", priceStr),
				})
				continue
			}
			unitPrice = parsedPrice
		}

		bcd, _ := strconv.ParseFloat(item.BCD, 64)
		sws, _ := strconv.ParseFloat(item.SWS, 64)
		igst, _ := strconv.ParseFloat(item.IGST, 64)
		diverged := item.UnitPrice > 0 && math.Abs(unitPrice-item.UnitPrice) > 0.001

		out = append(out, models.ParsedPasteLine{
			Row:             idx + 1,
			PartNumber:      item.PartNumber,
			ItemID:          item.ID,
			ItemDescription: item.ItemDescription,
			Unit:            item.Unit,
			Quantity:        qty,
			UnitPrice:       unitPrice,
			MasterUnitPrice: item.UnitPrice,
			DutyPercent:     bcd,
			SWSPercent:      sws,
			IGSTPercent:     igst,
			PriceDiverged:   diverged,
			Matched:         true,
		})
	}
	return out, nil
}

// SaveInvoice saves a Draft or Finalized commercial invoice inside one atomic SQL transaction.
// Finalize strictly checks integer-cent agreement with shipment.invoice_value; Draft bypasses it.
func (s *Service) SaveInvoice(req models.SaveInvoiceRequest) (models.InvoiceDetail, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.InvoiceDetail{}, err
	}

	lineDec := req.LineTotalDecimals
	invDec := req.InvoiceTotalDecimals
	if (lineDec != 0 && lineDec != 2) || (invDec != 0 && invDec != 2) {
		return models.InvoiceDetail{}, errors.New("decimal precision must be 0 or 2")
	}
	if len(req.Lines) == 0 {
		return models.InvoiceDetail{}, errors.New("at least one invoice line item is required")
	}

	sh, err := s.shipRepo.GetByID(req.ShipmentID)
	if err != nil {
		return models.InvoiceDetail{}, err
	}
	if sh.IsFrozen {
		return models.InvoiceDetail{}, fmt.Errorf("shipment %s is frozen and cannot accept invoice changes", sh.ID)
	}

	var computedSum float64
	for i, ln := range req.Lines {
		if strings.TrimSpace(ln.ItemID) == "" {
			return models.InvoiceDetail{}, fmt.Errorf("line %d: itemId is required", i+1)
		}
		if ln.Quantity <= 0 {
			return models.InvoiceDetail{}, fmt.Errorf("line %d: quantity must be positive", i+1)
		}
		if ln.UnitPrice < 0 {
			return models.InvoiceDetail{}, fmt.Errorf("line %d: unitPrice cannot be negative", i+1)
		}
		if _, itemErr := s.itemRepo.GetByID(ln.ItemID); itemErr != nil {
			return models.InvoiceDetail{}, fmt.Errorf("line %d: %w", i+1, itemErr)
		}
		lineTotal := repositories.RoundDecimals(ln.Quantity*ln.UnitPrice, lineDec)
		computedSum += lineTotal
	}
	computedInvoiceTotal := repositories.RoundDecimals(computedSum, invDec)

	if req.Finalize {
		expectedCents := math.Round(sh.InvoiceValue * 100)
		actualCents := math.Round(computedInvoiceTotal * 100)
		if expectedCents != actualCents {
			delta := (actualCents - expectedCents) / 100.0
			return models.InvoiceDetail{}, fmt.Errorf(
				"cannot finalize invoice: line total (%.2f) does not match shipment invoice value (%.2f), delta %.2f",
				computedInvoiceTotal, sh.InvoiceValue, delta,
			)
		}
	}

	status := "Draft"
	if req.Finalize {
		status = "Finalized"
	}

	invoiceID := strings.TrimSpace(req.InvoiceID)
	if invoiceID == "" {
		var existingInvoiceID string
		qErr := s.db.QueryRow(`SELECT id FROM invoices WHERE shipment_id = ? AND deleted_at IS NULL LIMIT 1`, sh.ID).Scan(&existingInvoiceID)
		if qErr == nil && existingInvoiceID != "" {
			invoiceID = existingInvoiceID
		} else {
			invoiceID, err = s.invRepo.NextInvoiceID()
			if err != nil {
				return models.InvoiceDetail{}, err
			}
		}
	}

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		var exists int
		if qErr := tx.QueryRow(`SELECT COUNT(*) FROM invoices WHERE id = ?`, invoiceID).Scan(&exists); qErr != nil {
			return qErr
		}
		if exists > 0 {
			if _, uErr := tx.Exec(`
				UPDATE invoices
				SET shipment_id = ?, status = ?, line_total_decimals = ?, invoice_total_decimals = ?, deleted_at = NULL, deleted_by = NULL
				WHERE id = ?`,
				sh.ID, status, lineDec, invDec, invoiceID,
			); uErr != nil {
				return uErr
			}
			if _, dErr := tx.Exec(`DELETE FROM invoice_line_items WHERE invoice_id = ?`, invoiceID); dErr != nil {
				return dErr
			}
		} else {
			if _, iErr := tx.Exec(`
				INSERT INTO invoices (id, shipment_id, status, line_total_decimals, invoice_total_decimals)
				VALUES (?, ?, ?, ?, ?)`,
				invoiceID, sh.ID, status, lineDec, invDec,
			); iErr != nil {
				return iErr
			}
		}

		for _, ln := range req.Lines {
			lineID := "IL-" + uuid.NewString()[:8]
			if _, lErr := tx.Exec(`
				INSERT INTO invoice_line_items (id, invoice_id, item_id, quantity, unit_price, duty_percent, sws_percent, igst_percent)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				lineID, invoiceID, strings.TrimSpace(ln.ItemID), ln.Quantity, ln.UnitPrice,
				ln.DutyPercent, ln.SWSPercent, ln.IGSTPercent,
			); lErr != nil {
				return lErr
			}
		}

		// Auto-transition shipment from docs-rcvd -> in-transit (unless already delivered/terminal).
		curSt := strings.ToLower(strings.TrimSpace(sh.Status))
		if curSt == "" || curSt == "docs-rcvd" || curSt == "docu-received" {
			if _, sErr := tx.Exec(`UPDATE shipments SET status = 'in-transit' WHERE id = ? AND deleted_at IS NULL`, sh.ID); sErr != nil {
				return sErr
			}
		}

		if req.Finalize && req.FreezeShipment {
			if _, fErr := tx.Exec(`UPDATE shipments SET is_frozen = 1 WHERE id = ? AND deleted_at IS NULL`, sh.ID); fErr != nil {
				return fErr
			}
		}
		return nil
	})
	if err != nil {
		return models.InvoiceDetail{}, fmt.Errorf("save invoice: %w", err)
	}

	detail, err := s.invRepo.GetDetail(invoiceID)
	if err != nil {
		return models.InvoiceDetail{}, err
	}
	_ = s.audit.RecordTableAudit("invoices", invoiceID, "UPSERT", sess.UserID, nil, detail, map[string]any{
		"finalize": req.Finalize,
		"freeze":   req.FreezeShipment,
	})
	return detail, nil
}

func splitPasteColumns(line string) []string {
	var delim string
	switch {
	case strings.Contains(line, "\t"):
		delim = "\t"
	case strings.Contains(line, "|"):
		delim = "|"
	case strings.Contains(line, ";"):
		delim = ";"
	default:
		delim = ","
	}
	rawCols := strings.Split(line, delim)
	out := make([]string, len(rawCols))
	for i, c := range rawCols {
		out[i] = strings.TrimSpace(c)
	}
	return out
}
