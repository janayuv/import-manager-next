package reports

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/validation"
)

// Service queries report_view for Landed Cost analysis (excluding IGST) and 14-column CSV exports.
type Service struct {
	db  *sql.DB
	sec *security.Service
}

func NewService(db *sql.DB, sec *security.Service) *Service {
	return &Service{db: db, sec: sec}
}

// GetLandedCostReport returns filtered landed cost rows from report_view plus 6 KPI metrics.
func (s *Service) GetLandedCostReport(filter models.LandedCostFilter) (models.LandedCostReport, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.LandedCostReport{}, err
	}
	if filter.StartDate != "" && filter.EndDate != "" && filter.StartDate > filter.EndDate {
		return models.LandedCostReport{}, fmt.Errorf("startDate (%s) cannot be after endDate (%s)", filter.StartDate, filter.EndDate)
	}

	q := `SELECT COALESCE(supplier,''), COALESCE(supplier_id,''), COALESCE(invoice_no,''), COALESCE(invoice_date,''),
	             COALESCE(part_no,''), COALESCE(description,''), COALESCE(unit,''),
	             COALESCE(qty, 0), COALESCE(unit_price, 0), COALESCE(assessable_value, 0),
	             COALESCE(bcd_amount, 0), COALESCE(sws_amount, 0), COALESCE(igst_amount, 0),
	             COALESCE(expenses_total, 0), COALESCE(ldc_per_qty, 0)
	      FROM report_view
	      WHERE 1=1`

	var args []any
	if filter.StartDate != "" {
		q += ` AND invoice_date >= ?`
		args = append(args, filter.StartDate)
	}
	if filter.EndDate != "" {
		q += ` AND invoice_date <= ?`
		args = append(args, filter.EndDate)
	}
	if filter.SupplierID != "" {
		q += ` AND supplier_id = ?`
		args = append(args, filter.SupplierID)
	}
	if st := strings.TrimSpace(filter.InvoiceNo); st != "" {
		q += ` AND invoice_no LIKE ?`
		args = append(args, "%"+st+"%")
	}
	if st := strings.TrimSpace(filter.PartNo); st != "" {
		q += ` AND part_no LIKE ?`
		args = append(args, "%"+st+"%")
	}
	if st := strings.TrimSpace(filter.Search); st != "" {
		like := "%" + st + "%"
		q += ` AND (supplier LIKE ? OR invoice_no LIKE ? OR part_no LIKE ? OR description LIKE ?)`
		args = append(args, like, like, like, like)
	}
	q += ` ORDER BY invoice_date DESC, invoice_no ASC, part_no ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return models.LandedCostReport{}, err
	}
	defer rows.Close()

	report := models.LandedCostReport{
		Rows: make([]models.LandedCostRow, 0),
	}
	for rows.Next() {
		var r models.LandedCostRow
		if err := rows.Scan(
			&r.Supplier, &r.SupplierID, &r.InvoiceNo, &r.InvoiceDate,
			&r.PartNo, &r.Description, &r.Unit,
			&r.Qty, &r.UnitPrice, &r.AssessableValue,
			&r.BCDAmount, &r.SWSAmount, &r.IGSTAmount,
			&r.ExpensesTotal, &r.LDCPerQty,
		); err != nil {
			return models.LandedCostReport{}, err
		}
		report.Rows = append(report.Rows, r)
		report.Summary.TotalRows++
		report.Summary.TotalQuantity += r.Qty
		report.Summary.TotalAssessableValue += r.AssessableValue
		report.Summary.TotalBasicDuties += r.BCDAmount + r.SWSAmount
		report.Summary.TotalIGST += r.IGSTAmount
		report.Summary.TotalBasicExpenses += r.ExpensesTotal
		report.Summary.TotalLandedCostExIGST += r.AssessableValue + r.BCDAmount + r.SWSAmount + r.ExpensesTotal
	}
	if err := rows.Err(); err != nil {
		return models.LandedCostReport{}, err
	}
	return report, nil
}

// ExportLandedCostCSV exports the exact 14-column Landed Cost CSV with formula-cell protection.
func (s *Service) ExportLandedCostCSV(filter models.LandedCostFilter) (string, error) {
	rep, err := s.GetLandedCostReport(filter)
	if err != nil {
		return "", err
	}
	headers := []string{
		"Supplier", "Invoice No", "Date", "Part No", "Description", "Unit",
		"Qty", "Unit Price", "Assessable Value", "BCD", "SWS", "IGST",
		"Expenses (Basic)", "LDC per qty",
	}
	records := make([][]string, 0, len(rep.Rows))
	for _, r := range rep.Rows {
		records = append(records, []string{
			r.Supplier,
			r.InvoiceNo,
			r.InvoiceDate,
			r.PartNo,
			r.Description,
			r.Unit,
			fmt.Sprintf("%.4f", r.Qty),
			fmt.Sprintf("%.4f", r.UnitPrice),
			fmt.Sprintf("%.2f", r.AssessableValue),
			fmt.Sprintf("%.2f", r.BCDAmount),
			fmt.Sprintf("%.2f", r.SWSAmount),
			fmt.Sprintf("%.2f", r.IGSTAmount),
			fmt.Sprintf("%.2f", r.ExpensesTotal),
			fmt.Sprintf("%.4f", r.LDCPerQty),
		})
	}
	return validation.EncodeCSV(headers, records)
}
