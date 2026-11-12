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

// SupplierRepository manages persistence for the suppliers table.
type SupplierRepository struct {
	db *sql.DB
}

// NewSupplierRepository constructs a SupplierRepository bound to db.
func NewSupplierRepository(db *sql.DB) *SupplierRepository {
	return &SupplierRepository{db: db}
}

var allowedSupplierSortCols = map[string]string{
	"id":           "id",
	"supplierName": "supplier_name",
	"shortName":    "short_name",
	"country":      "country",
	"email":        "email",
	"isActive":     "is_active",
}

// List returns filtered, sorted, paginated active (non-soft-deleted) suppliers.
func (r *SupplierRepository) List(req models.SupplierPageRequest) (models.SupplierPage, error) {
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

	sortCol := allowedSupplierSortCols[req.SortBy]
	if sortCol == "" {
		sortCol = "supplier_name"
	}
	sortDir := "ASC"
	if strings.EqualFold(req.SortDir, "desc") {
		sortDir = "DESC"
	}

	where := []string{"deleted_at IS NULL"}
	var args []any

	if q := strings.TrimSpace(req.Search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(id LIKE ? OR supplier_name LIKE ? OR COALESCE(short_name,'') LIKE ? OR country LIKE ? OR email LIKE ?)")
		args = append(args, like, like, like, like, like)
	}

	switch strings.ToLower(strings.TrimSpace(req.Status)) {
	case "active":
		where = append(where, "is_active = 1")
	case "inactive":
		where = append(where, "is_active = 0")
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM suppliers WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return models.SupplierPage{}, fmt.Errorf("count suppliers: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, supplier_name, COALESCE(short_name,''), country, email, phone,
		       COALESCE(beneficiary_name,''), COALESCE(bank_name,''), COALESCE(branch,''),
		       COALESCE(bank_address,''), COALESCE(account_no,''), COALESCE(iban,''),
		       COALESCE(swift_code,''), is_active
		FROM suppliers
		WHERE %s
		ORDER BY %s COLLATE NOCASE %s, id ASC
		LIMIT ? OFFSET ?`, whereSQL, sortCol, sortDir)

	listArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := r.db.Query(query, listArgs...)
	if err != nil {
		return models.SupplierPage{}, fmt.Errorf("query suppliers: %w", err)
	}
	defer rows.Close()

	var items []models.Supplier
	for rows.Next() {
		var s models.Supplier
		if err := rows.Scan(
			&s.ID, &s.SupplierName, &s.ShortName, &s.Country, &s.Email, &s.Phone,
			&s.BeneficiaryName, &s.BankName, &s.Branch, &s.BankAddress,
			&s.AccountNo, &s.IBAN, &s.SwiftCode, &s.IsActive,
		); err != nil {
			return models.SupplierPage{}, fmt.Errorf("scan supplier: %w", err)
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return models.SupplierPage{}, err
	}

	return models.SupplierPage{
		Items:      items,
		TotalCount: total,
		Page:       page,
		PageSize:   size,
	}, nil
}

// GetByID fetches an active (non-soft-deleted) supplier by primary key.
func (r *SupplierRepository) GetByID(id string) (models.Supplier, error) {
	var s models.Supplier
	err := r.db.QueryRow(`
		SELECT id, supplier_name, COALESCE(short_name,''), country, email, phone,
		       COALESCE(beneficiary_name,''), COALESCE(bank_name,''), COALESCE(branch,''),
		       COALESCE(bank_address,''), COALESCE(account_no,''), COALESCE(iban,''),
		       COALESCE(swift_code,''), is_active
		FROM suppliers
		WHERE id = ? AND deleted_at IS NULL`, strings.TrimSpace(id)).Scan(
		&s.ID, &s.SupplierName, &s.ShortName, &s.Country, &s.Email, &s.Phone,
		&s.BeneficiaryName, &s.BankName, &s.Branch, &s.BankAddress,
		&s.AccountNo, &s.IBAN, &s.SwiftCode, &s.IsActive,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Supplier{}, fmt.Errorf("supplier %q not found", id)
	}
	if err != nil {
		return models.Supplier{}, err
	}
	return s, nil
}

// FindDuplicate checks case-insensitive duplicate supplier_name (and non-empty email) among non-deleted suppliers.
func (r *SupplierRepository) FindDuplicate(name, email, excludeID string) (*models.Supplier, error) {
	n := strings.TrimSpace(name)
	e := strings.TrimSpace(email)
	var id string
	var err error
	if e != "" {
		err = r.db.QueryRow(`
			SELECT id FROM suppliers
			WHERE deleted_at IS NULL AND id <> ?
			  AND (LOWER(TRIM(supplier_name)) = LOWER(?) OR (TRIM(email) <> '' AND LOWER(TRIM(email)) = LOWER(?)))
			LIMIT 1`, excludeID, n, e).Scan(&id)
	} else {
		err = r.db.QueryRow(`
			SELECT id FROM suppliers
			WHERE deleted_at IS NULL AND id <> ?
			  AND LOWER(TRIM(supplier_name)) = LOWER(?)
			LIMIT 1`, excludeID, n).Scan(&id)
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

// NextID computes the next sequential Sup-XXX identifier across existing supplier IDs.
func (r *SupplierRepository) NextID() (string, error) {
	rows, err := r.db.Query(`SELECT id FROM suppliers WHERE id LIKE 'Sup-%'`)
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
		numPart := strings.TrimPrefix(id, "Sup-")
		if n, err := strconv.Atoi(numPart); err == nil && n > maxNum {
			maxNum = n
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Sup-%03d", maxNum+1), nil
}

// Create inserts a new supplier row.
func (r *SupplierRepository) Create(in models.SupplierInput) (models.Supplier, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		var err error
		id, err = r.NextID()
		if err != nil {
			return models.Supplier{}, err
		}
	}
	_, err := r.db.Exec(`
		INSERT INTO suppliers (
			id, supplier_name, short_name, country, email, phone,
			beneficiary_name, bank_name, branch, bank_address,
			account_no, iban, swift_code, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		strings.TrimSpace(in.SupplierName),
		nullIfEmpty(in.ShortName),
		strings.TrimSpace(in.Country),
		strings.TrimSpace(in.Email),
		strings.TrimSpace(in.Phone),
		nullIfEmpty(in.BeneficiaryName),
		nullIfEmpty(in.BankName),
		nullIfEmpty(in.Branch),
		nullIfEmpty(in.BankAddress),
		nullIfEmpty(in.AccountNo),
		nullIfEmpty(in.IBAN),
		nullIfEmpty(in.SwiftCode),
		in.IsActive,
	)
	if err != nil {
		return models.Supplier{}, fmt.Errorf("insert supplier %s: %w", id, err)
	}
	return r.GetByID(id)
}

// Update modifies an existing non-deleted supplier row.
func (r *SupplierRepository) Update(id string, in models.SupplierInput) (models.Supplier, error) {
	res, err := r.db.Exec(`
		UPDATE suppliers SET
			supplier_name = ?,
			short_name = ?,
			country = ?,
			email = ?,
			phone = ?,
			beneficiary_name = ?,
			bank_name = ?,
			branch = ?,
			bank_address = ?,
			account_no = ?,
			iban = ?,
			swift_code = ?,
			is_active = ?
		WHERE id = ? AND deleted_at IS NULL`,
		strings.TrimSpace(in.SupplierName),
		nullIfEmpty(in.ShortName),
		strings.TrimSpace(in.Country),
		strings.TrimSpace(in.Email),
		strings.TrimSpace(in.Phone),
		nullIfEmpty(in.BeneficiaryName),
		nullIfEmpty(in.BankName),
		nullIfEmpty(in.Branch),
		nullIfEmpty(in.BankAddress),
		nullIfEmpty(in.AccountNo),
		nullIfEmpty(in.IBAN),
		nullIfEmpty(in.SwiftCode),
		in.IsActive,
		strings.TrimSpace(id),
	)
	if err != nil {
		return models.Supplier{}, fmt.Errorf("update supplier %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return models.Supplier{}, fmt.Errorf("supplier %q not found", id)
	}
	return r.GetByID(id)
}

// SoftDelete marks a supplier as deleted for Recycle Bin recovery.
func (r *SupplierRepository) SoftDelete(id, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		UPDATE suppliers SET deleted_at = ?, deleted_by = ?
		WHERE id = ? AND deleted_at IS NULL`, now, userID, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("soft delete supplier %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("supplier %q not found", id)
	}
	return nil
}

// GetHistory aggregates shipment and invoice history for a supplier.
func (r *SupplierRepository) GetHistory(supplierID string) (models.SupplierHistorySummary, error) {
	sid := strings.TrimSpace(supplierID)
	summary := models.SupplierHistorySummary{SupplierID: sid}

	if err := r.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(invoice_value), 0)
		FROM shipments
		WHERE supplier_id = ? AND deleted_at IS NULL`, sid).Scan(&summary.ShipmentCount, &summary.TotalValue); err != nil {
		return summary, fmt.Errorf("supplier shipment summary: %w", err)
	}

	if err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM invoices
		WHERE supplier_id = ? AND deleted_at IS NULL`, sid).Scan(&summary.InvoiceCount); err != nil {
		return summary, fmt.Errorf("supplier invoice summary: %w", err)
	}

	rows, err := r.db.Query(`
		SELECT id, invoice_number, invoice_date, invoice_currency, invoice_value, COALESCE(status,'')
		FROM shipments
		WHERE supplier_id = ? AND deleted_at IS NULL
		ORDER BY invoice_date DESC, id DESC
		LIMIT 25`, sid)
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	for rows.Next() {
		var h models.SupplierHistoryItem
		if err := rows.Scan(&h.ShipmentID, &h.InvoiceNumber, &h.InvoiceDate, &h.Currency, &h.InvoiceValue, &h.Status); err != nil {
			return summary, err
		}
		summary.RecentRecords = append(summary.RecentRecords, h)
	}
	return summary, rows.Err()
}

// AllForExport returns all non-deleted suppliers ordered by ID for CSV export.
func (r *SupplierRepository) AllForExport() ([]models.Supplier, error) {
	page, err := r.List(models.SupplierPageRequest{Page: 1, PageSize: 500, SortBy: "id", SortDir: "asc"})
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func nullIfEmpty(s string) any {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return t
}
