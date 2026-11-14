package expense_reports

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service provides 5 analytical Expense Report views with SQL-parameterized paise & date filtering.
type Service struct {
	db  *sql.DB
	sec *security.Service
}

func NewService(db *sql.DB, sec *security.Service) *Service {
	return &Service{db: db, sec: sec}
}

// GetExpenseReport evaluates one of 5 views ('detailed', 'summary-by-type', 'summary-by-provider',
// 'summary-by-shipment', 'summary-by-month') along with filtered line-level rows & summary totals.
func (s *Service) GetExpenseReport(filter models.ExpenseReportFilter) (models.ExpenseReportResult, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.ExpenseReportResult{}, err
	}

	viewMode := strings.TrimSpace(filter.ViewMode)
	if viewMode == "" {
		viewMode = "detailed"
	}
	if filter.StartDate != "" && filter.EndDate != "" && filter.StartDate > filter.EndDate {
		return models.ExpenseReportResult{}, fmt.Errorf("startDate (%s) cannot be after endDate (%s)", filter.StartDate, filter.EndDate)
	}

	q := `SELECT e.id, e.expense_invoice_id, e.shipment_id, COALESCE(sh.invoice_number,''),
	             e.service_provider_id, COALESCE(sp.name,''),
	             e.expense_type_id, COALESCE(NULLIF(et.name,''), e.expense_type_id, 'Expense Item'),
	             e.invoice_no, e.invoice_date,
	             COALESCE(NULLIF(e.amount_paise,0), CAST(ROUND(COALESCE(e.amount,0)*100) AS INTEGER), 0) AS amt_p,
	             COALESCE(e.cgst_amount_paise, 0) AS cgst_p,
	             COALESCE(e.sgst_amount_paise, 0) AS sgst_p,
	             COALESCE(e.igst_amount_paise, 0) AS igst_p,
	             COALESCE(e.tds_amount_paise, 0) AS tds_p,
	             COALESCE(NULLIF(e.total_amount_paise,0), CAST(ROUND(COALESCE(e.total_amount, e.amount, 0)*100) AS INTEGER), 0) AS tot_p,
	             COALESCE(NULLIF(e.net_amount_paise,0), CAST(ROUND(COALESCE(e.total_amount, e.amount, 0)*100) AS INTEGER), 0) AS net_p,
	             COALESCE(e.remarks,'')
	      FROM expenses e
	      JOIN expense_invoices ei ON ei.id = e.expense_invoice_id AND ei.deleted_at IS NULL
	      LEFT JOIN shipments sh ON sh.id = e.shipment_id
	      LEFT JOIN service_providers sp ON sp.id = e.service_provider_id
	      LEFT JOIN expense_types et ON et.id = e.expense_type_id
	      WHERE e.deleted_at IS NULL`

	var args []any
	if filter.StartDate != "" {
		q += ` AND e.invoice_date >= ?`
		args = append(args, filter.StartDate)
	}
	if filter.EndDate != "" {
		q += ` AND e.invoice_date <= ?`
		args = append(args, filter.EndDate)
	}
	if filter.ShipmentID != "" {
		q += ` AND e.shipment_id = ?`
		args = append(args, filter.ShipmentID)
	}
	if filter.ServiceProviderID != "" {
		q += ` AND e.service_provider_id = ?`
		args = append(args, filter.ServiceProviderID)
	}
	if filter.ExpenseTypeID != "" {
		q += ` AND e.expense_type_id = ?`
		args = append(args, filter.ExpenseTypeID)
	}
	if filter.MinAmountPaise > 0 {
		q += ` AND COALESCE(NULLIF(e.amount_paise,0), CAST(ROUND(e.amount*100) AS INTEGER)) >= ?`
		args = append(args, filter.MinAmountPaise)
	}
	if filter.MaxAmountPaise > 0 {
		q += ` AND COALESCE(NULLIF(e.amount_paise,0), CAST(ROUND(e.amount*100) AS INTEGER)) <= ?`
		args = append(args, filter.MaxAmountPaise)
	}
	if st := strings.TrimSpace(filter.Search); st != "" {
		like := "%" + st + "%"
		q += ` AND (e.invoice_no LIKE ? OR sp.name LIKE ? OR et.name LIKE ? OR sh.invoice_number LIKE ? OR COALESCE(e.remarks,'') LIKE ?)`
		args = append(args, like, like, like, like, like)
	}
	q += ` ORDER BY e.invoice_date DESC, e.invoice_no ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return models.ExpenseReportResult{}, err
	}
	defer rows.Close()

	res := models.ExpenseReportResult{
		ViewMode:     viewMode,
		DetailedRows: make([]models.ExpenseReportDetailedRow, 0),
		GroupedRows:  make([]models.ExpenseReportGroup, 0),
	}

	groupOrder := make([]string, 0)
	groupMap := make(map[string]*models.ExpenseReportGroup)

	for rows.Next() {
		var r models.ExpenseReportDetailedRow
		if err := rows.Scan(
			&r.ExpenseID, &r.ExpenseInvoiceID, &r.ShipmentID, &r.ShipmentInvoiceNo,
			&r.ServiceProviderID, &r.ServiceProviderName, &r.ExpenseTypeID, &r.ExpenseTypeName,
			&r.InvoiceNo, &r.InvoiceDate,
			&r.AmountPaise, &r.CGSTPaise, &r.SGSTPaise, &r.IGSTPaise, &r.TDSPaise,
			&r.TotalPaise, &r.NetPaise, &r.Remarks,
		); err != nil {
			return models.ExpenseReportResult{}, err
		}

		res.DetailedRows = append(res.DetailedRows, r)
		res.TotalLines++
		res.TotalBasePaise += r.AmountPaise
		res.TotalTaxPaise += r.CGSTPaise + r.SGSTPaise + r.IGSTPaise
		res.TotalTDSPaise += r.TDSPaise
		res.TotalGrossPaise += r.TotalPaise
		res.TotalNetPaise += r.NetPaise

		key, label := groupKeyAndLabel(viewMode, r)
		if key != "" {
			grp, ok := groupMap[key]
			if !ok {
				grp = &models.ExpenseReportGroup{GroupKey: key, GroupLabel: label}
				groupMap[key] = grp
				groupOrder = append(groupOrder, key)
			}
			grp.LineCount++
			grp.AmountPaise += r.AmountPaise
			grp.CGSTPaise += r.CGSTPaise
			grp.SGSTPaise += r.SGSTPaise
			grp.IGSTPaise += r.IGSTPaise
			grp.TDSPaise += r.TDSPaise
			grp.TotalPaise += r.TotalPaise
			grp.NetPaise += r.NetPaise
		}
	}
	if err := rows.Err(); err != nil {
		return models.ExpenseReportResult{}, err
	}

	for _, k := range groupOrder {
		res.GroupedRows = append(res.GroupedRows, *groupMap[k])
	}
	return res, nil
}

func groupKeyAndLabel(viewMode string, r models.ExpenseReportDetailedRow) (string, string) {
	switch viewMode {
	case "summary-by-type":
		return r.ExpenseTypeID, r.ExpenseTypeName
	case "summary-by-provider":
		return r.ServiceProviderID, r.ServiceProviderName
	case "summary-by-shipment":
		return r.ShipmentID, r.ShipmentInvoiceNo
	case "summary-by-month":
		month := r.InvoiceDate
		if len(month) >= 7 {
			month = month[:7]
		}
		return month, month
	default:
		return "", ""
	}
}
