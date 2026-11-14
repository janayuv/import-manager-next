package expenses

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
)

const BasisPointsScale = 10000

// CalcLinePaise implements exact integer paise arithmetic:
// In Automatic mode: tax_paise = (amount_paise * rate_bp) / 10000.
// In Manual mode: tax_paise is taken directly from manual CGST/SGST/IGST paise (supporting values not eligible for full GST).
// total_paise = amount_paise + cgst + sgst + igst; net_paise = total_paise - tds.
func CalcLinePaise(in models.ExpenseLineInput) (models.ExpenseLineCalculation, error) {
	if strings.TrimSpace(in.ExpenseTypeID) == "" {
		return models.ExpenseLineCalculation{}, errors.New("expenseTypeId is required")
	}
	if in.AmountPaise <= 0 {
		return models.ExpenseLineCalculation{}, errors.New("expense amount must be positive")
	}
	if in.TDSRateBP < 0 || in.TDSRateBP > 10000 {
		return models.ExpenseLineCalculation{}, errors.New("TDS rate must be between 0% and 100% (0-10000 bp)")
	}

	var cgst, sgst, igst int64
	cgstRateBP := in.CGSTRateBP
	sgstRateBP := in.SGSTRateBP
	igstRateBP := in.IGSTRateBP

	if in.IsManualGST {
		if in.CGSTPaise < 0 || in.SGSTPaise < 0 || in.IGSTPaise < 0 {
			return models.ExpenseLineCalculation{}, errors.New("manual GST amounts cannot be negative")
		}
		cgst = in.CGSTPaise
		sgst = in.SGSTPaise
		igst = in.IGSTPaise

		// If nominal rates weren't explicitly supplied, derive effective basis points from amount
		if cgstRateBP == 0 && in.AmountPaise > 0 && cgst > 0 {
			cgstRateBP = int64(math.Round(float64(cgst*BasisPointsScale) / float64(in.AmountPaise)))
		}
		if sgstRateBP == 0 && in.AmountPaise > 0 && sgst > 0 {
			sgstRateBP = int64(math.Round(float64(sgst*BasisPointsScale) / float64(in.AmountPaise)))
		}
		if igstRateBP == 0 && in.AmountPaise > 0 && igst > 0 {
			igstRateBP = int64(math.Round(float64(igst*BasisPointsScale) / float64(in.AmountPaise)))
		}
	} else {
		if in.CGSTRateBP < 0 || in.SGSTRateBP < 0 || in.IGSTRateBP < 0 {
			return models.ExpenseLineCalculation{}, errors.New("tax and TDS rates cannot be negative")
		}
		if in.CGSTRateBP > 10000 || in.SGSTRateBP > 10000 || in.IGSTRateBP > 10000 {
			return models.ExpenseLineCalculation{}, errors.New("tax and TDS rates cannot exceed 100% (10000 bp)")
		}
		cgst = (in.AmountPaise * in.CGSTRateBP) / BasisPointsScale
		sgst = (in.AmountPaise * in.SGSTRateBP) / BasisPointsScale
		igst = (in.AmountPaise * in.IGSTRateBP) / BasisPointsScale
	}

	tds := (in.AmountPaise * in.TDSRateBP) / BasisPointsScale

	total := in.AmountPaise + cgst + sgst + igst
	net := total - tds

	return models.ExpenseLineCalculation{
		ExpenseTypeID: in.ExpenseTypeID,
		AmountPaise:   in.AmountPaise,
		IsManualGST:   in.IsManualGST,
		CGSTRateBP:    cgstRateBP,
		SGSTRateBP:    sgstRateBP,
		IGSTRateBP:    igstRateBP,
		TDSRateBP:     in.TDSRateBP,
		CGSTPaise:     cgst,
		SGSTPaise:     sgst,
		IGSTPaise:     igst,
		TDSPaise:      tds,
		TotalPaise:    total,
		NetPaise:      net,
		AmountINR:     float64(in.AmountPaise) / 100.0,
		TotalINR:      float64(total) / 100.0,
		NetINR:        float64(net) / 100.0,
		Remarks:       strings.TrimSpace(in.Remarks),
	}, nil
}

// DetectDuplicateExpenseTypes returns true when two or more lines share the same expenseTypeId.
func DetectDuplicateExpenseTypes(lines []models.ExpenseLineInput) bool {
	seen := make(map[string]struct{}, len(lines))
	for _, l := range lines {
		key := strings.TrimSpace(l.ExpenseTypeID)
		if _, exists := seen[key]; exists {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

// MergeDuplicateLines combines multiple lines sharing an expenseTypeId by summing AmountPaise
// and joining non-empty remarks cleanly. Mismatched rates on duplicate lines fail loudly so
// tax basis integrity is never silently distorted.
func MergeDuplicateLines(lines []models.ExpenseLineInput) ([]models.ExpenseLineInput, error) {
	order := make([]string, 0, len(lines))
	merged := make(map[string]models.ExpenseLineInput, len(lines))
	for _, raw := range lines {
		key := strings.TrimSpace(raw.ExpenseTypeID)
		if key == "" {
			return nil, errors.New("expenseTypeId is required")
		}
		cur, exists := merged[key]
		if !exists {
			raw.ExpenseTypeID = key
			raw.Remarks = strings.TrimSpace(raw.Remarks)
			merged[key] = raw
			order = append(order, key)
			continue
		}
		if cur.IsManualGST != raw.IsManualGST {
			return nil, fmt.Errorf("cannot merge duplicate expense type %q with mixed auto/manual GST modes", key)
		}
		if cur.TDSRateBP != raw.TDSRateBP {
			return nil, fmt.Errorf("cannot merge duplicate expense type %q with differing TDS rates", key)
		}
		if !cur.IsManualGST {
			if cur.CGSTRateBP != raw.CGSTRateBP ||
				cur.SGSTRateBP != raw.SGSTRateBP ||
				cur.IGSTRateBP != raw.IGSTRateBP {
				return nil, fmt.Errorf("cannot merge duplicate expense type %q with differing tax/TDS rates", key)
			}
		} else {
			cur.CGSTPaise += raw.CGSTPaise
			cur.SGSTPaise += raw.SGSTPaise
			cur.IGSTPaise += raw.IGSTPaise
		}
		cur.AmountPaise += raw.AmountPaise
		rem := strings.TrimSpace(raw.Remarks)
		if rem != "" {
			if cur.Remarks == "" {
				cur.Remarks = rem
			} else if !strings.Contains(cur.Remarks, rem) {
				cur.Remarks = cur.Remarks + "; " + rem
			}
		}
		merged[key] = cur
	}
	out := make([]models.ExpenseLineInput, 0, len(order))
	for _, k := range order {
		out = append(out, merged[k])
	}
	return out, nil
}

// CalculateInvoice aggregates exact integer paise totals across all provided lines.
func CalculateInvoice(lines []models.ExpenseLineInput) (models.ExpenseInvoiceCalculation, error) {
	if len(lines) == 0 {
		return models.ExpenseInvoiceCalculation{}, errors.New("at least one expense line is required")
	}
	res := models.ExpenseInvoiceCalculation{
		Lines:            make([]models.ExpenseLineCalculation, 0, len(lines)),
		HasDuplicateType: DetectDuplicateExpenseTypes(lines),
	}
	for _, line := range lines {
		calc, err := CalcLinePaise(line)
		if err != nil {
			return models.ExpenseInvoiceCalculation{}, err
		}
		res.Lines = append(res.Lines, calc)
		res.BaseAmountPaise += calc.AmountPaise
		res.TotalCGSTPaise += calc.CGSTPaise
		res.TotalSGSTPaise += calc.SGSTPaise
		res.TotalIGSTPaise += calc.IGSTPaise
		res.TotalTDSPaise += calc.TDSPaise
		res.TotalAmountPaise += calc.TotalPaise
		res.NetAmountPaise += calc.NetPaise
	}
	res.TotalAmountINR = float64(res.TotalAmountPaise) / 100.0
	res.NetAmountINR = float64(res.NetAmountPaise) / 100.0
	return res, nil
}
