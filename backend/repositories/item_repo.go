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

// ItemRepository manages persistence for the items table.
type ItemRepository struct {
	db *sql.DB
}

// NewItemRepository constructs an ItemRepository bound to db.
func NewItemRepository(db *sql.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

var allowedItemSortCols = map[string]string{
	"id":              "i.id",
	"partNumber":      "i.part_number",
	"itemDescription": "i.item_description",
	"hsnCode":         "i.hsn_code",
	"unitPrice":       "i.unit_price",
	"currency":        "i.currency",
	"category":        "i.category",
	"isActive":        "i.is_active",
}

// List returns filtered, sorted, paginated active (non-deleted) items joined with supplier names.
func (r *ItemRepository) List(req models.ItemPageRequest) (models.ItemPage, error) {
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

	sortCol := allowedItemSortCols[req.SortBy]
	if sortCol == "" {
		sortCol = "i.part_number"
	}
	sortDir := "ASC"
	if strings.EqualFold(req.SortDir, "desc") {
		sortDir = "DESC"
	}

	where := []string{"i.deleted_at IS NULL"}
	var args []any

	if q := strings.TrimSpace(req.Search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(i.id LIKE ? OR i.part_number LIKE ? OR i.item_description LIKE ? OR i.hsn_code LIKE ? OR COALESCE(i.category,'') LIKE ? OR COALESCE(s.supplier_name,'') LIKE ?)")
		args = append(args, like, like, like, like, like, like)
	}
	if sid := strings.TrimSpace(req.SupplierID); sid != "" {
		where = append(where, "i.supplier_id = ?")
		args = append(args, sid)
	}
	switch strings.ToLower(strings.TrimSpace(req.Status)) {
	case "active":
		where = append(where, "i.is_active = 1")
	case "inactive":
		where = append(where, "i.is_active = 0")
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM items i
		LEFT JOIN suppliers s ON s.id = i.supplier_id
		WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return models.ItemPage{}, fmt.Errorf("count items: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT i.id, i.part_number, i.item_description, i.unit, i.currency, i.unit_price, i.hsn_code,
		       COALESCE(i.supplier_id,''), COALESCE(s.supplier_name,''), i.is_active,
		       COALESCE(i.country_of_origin,''), COALESCE(i.bcd,''), COALESCE(i.sws,''), COALESCE(i.igst,''),
		       COALESCE(i.technical_write_up,''), COALESCE(i.category,''), COALESCE(i.end_use,''),
		       COALESCE(i.net_weight_kg,0), COALESCE(i.purchase_uom,''), COALESCE(i.gross_weight_per_uom_kg,0),
		       COALESCE(i.photo_path,'')
		FROM items i
		LEFT JOIN suppliers s ON s.id = i.supplier_id
		WHERE %s
		ORDER BY %s %s, i.id ASC
		LIMIT ? OFFSET ?`, whereSQL, sortCol, sortDir)

	listArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := r.db.Query(query, listArgs...)
	if err != nil {
		return models.ItemPage{}, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		it, err := scanItemRow(rows)
		if err != nil {
			return models.ItemPage{}, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return models.ItemPage{}, err
	}

	return models.ItemPage{
		Items:      items,
		TotalCount: total,
		Page:       page,
		PageSize:   size,
	}, nil
}

// GetByID fetches an active item by primary key.
func (r *ItemRepository) GetByID(id string) (models.Item, error) {
	row := r.db.QueryRow(`
		SELECT i.id, i.part_number, i.item_description, i.unit, i.currency, i.unit_price, i.hsn_code,
		       COALESCE(i.supplier_id,''), COALESCE(s.supplier_name,''), i.is_active,
		       COALESCE(i.country_of_origin,''), COALESCE(i.bcd,''), COALESCE(i.sws,''), COALESCE(i.igst,''),
		       COALESCE(i.technical_write_up,''), COALESCE(i.category,''), COALESCE(i.end_use,''),
		       COALESCE(i.net_weight_kg,0), COALESCE(i.purchase_uom,''), COALESCE(i.gross_weight_per_uom_kg,0),
		       COALESCE(i.photo_path,'')
		FROM items i
		LEFT JOIN suppliers s ON s.id = i.supplier_id
		WHERE i.id = ? AND i.deleted_at IS NULL`, strings.TrimSpace(id))
	it, err := scanItemRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Item{}, fmt.Errorf("item %q not found", id)
	}
	return it, err
}

// FindByPartNumber checks case-insensitive uniqueness of part_number across non-deleted items.
func (r *ItemRepository) FindByPartNumber(partNumber, excludeID string) (*models.Item, error) {
	pn := strings.TrimSpace(partNumber)
	if pn == "" {
		return nil, nil
	}
	var id string
	err := r.db.QueryRow(`
		SELECT id FROM items
		WHERE deleted_at IS NULL AND id <> ? AND LOWER(TRIM(part_number)) = LOWER(?)
		LIMIT 1`, excludeID, pn).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	it, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// ResolveSupplierIDByName looks up an active supplier ID case-insensitively by supplier_name or short_name.
func (r *ItemRepository) ResolveSupplierIDByName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return "", nil
	}
	var id string
	err := r.db.QueryRow(`
		SELECT id FROM suppliers
		WHERE deleted_at IS NULL
		  AND (LOWER(TRIM(supplier_name)) = LOWER(?) OR LOWER(TRIM(COALESCE(short_name,''))) = LOWER(?))
		LIMIT 1`, n, n).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// NextID returns the next sequential ITM-XXX primary key.
func (r *ItemRepository) NextID() (string, error) {
	rows, err := r.db.Query(`SELECT id FROM items WHERE id LIKE 'ITM-%'`)
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
		numPart := strings.TrimPrefix(id, "ITM-")
		if n, err := strconv.Atoi(numPart); err == nil && n > maxNum {
			maxNum = n
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("ITM-%03d", maxNum+1), nil
}

// Create inserts a new item into items.
func (r *ItemRepository) Create(in models.ItemInput) (models.Item, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		var err error
		id, err = r.NextID()
		if err != nil {
			return models.Item{}, err
		}
	}
	_, err := r.db.Exec(`
		INSERT INTO items (
			id, part_number, item_description, unit, currency, unit_price, hsn_code,
			supplier_id, is_active, country_of_origin, bcd, sws, igst,
			technical_write_up, category, end_use, net_weight_kg,
			purchase_uom, gross_weight_per_uom_kg, photo_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		strings.TrimSpace(in.PartNumber),
		strings.TrimSpace(in.ItemDescription),
		strings.TrimSpace(in.Unit),
		strings.TrimSpace(in.Currency),
		in.UnitPrice,
		strings.TrimSpace(in.HSNCode),
		nullIfEmpty(in.SupplierID),
		in.IsActive,
		nullIfEmpty(in.CountryOfOrigin),
		nullIfEmpty(in.BCD),
		nullIfEmpty(in.SWS),
		nullIfEmpty(in.IGST),
		nullIfEmpty(in.TechnicalWriteUp),
		nullIfEmpty(in.Category),
		nullIfEmpty(in.EndUse),
		in.NetWeightKg,
		nullIfEmpty(in.PurchaseUOM),
		in.GrossWeightPerUOMKg,
		nullIfEmpty(in.PhotoPath),
	)
	if err != nil {
		return models.Item{}, fmt.Errorf("insert item %s: %w", id, err)
	}
	return r.GetByID(id)
}

// Update modifies an existing active item record.
func (r *ItemRepository) Update(id string, in models.ItemInput) (models.Item, error) {
	res, err := r.db.Exec(`
		UPDATE items SET
			part_number = ?,
			item_description = ?,
			unit = ?,
			currency = ?,
			unit_price = ?,
			hsn_code = ?,
			supplier_id = ?,
			is_active = ?,
			country_of_origin = ?,
			bcd = ?,
			sws = ?,
			igst = ?,
			technical_write_up = ?,
			category = ?,
			end_use = ?,
			net_weight_kg = ?,
			purchase_uom = ?,
			gross_weight_per_uom_kg = ?,
			photo_path = ?
		WHERE id = ? AND deleted_at IS NULL`,
		strings.TrimSpace(in.PartNumber),
		strings.TrimSpace(in.ItemDescription),
		strings.TrimSpace(in.Unit),
		strings.TrimSpace(in.Currency),
		in.UnitPrice,
		strings.TrimSpace(in.HSNCode),
		nullIfEmpty(in.SupplierID),
		in.IsActive,
		nullIfEmpty(in.CountryOfOrigin),
		nullIfEmpty(in.BCD),
		nullIfEmpty(in.SWS),
		nullIfEmpty(in.IGST),
		nullIfEmpty(in.TechnicalWriteUp),
		nullIfEmpty(in.Category),
		nullIfEmpty(in.EndUse),
		in.NetWeightKg,
		nullIfEmpty(in.PurchaseUOM),
		in.GrossWeightPerUOMKg,
		nullIfEmpty(in.PhotoPath),
		strings.TrimSpace(id),
	)
	if err != nil {
		return models.Item{}, fmt.Errorf("update item %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return models.Item{}, fmt.Errorf("item %q not found", id)
	}
	return r.GetByID(id)
}

// SoftDelete marks an item row as deleted.
func (r *ItemRepository) SoftDelete(id, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		UPDATE items SET deleted_at = ?, deleted_by = ?
		WHERE id = ? AND deleted_at IS NULL`, now, userID, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("soft delete item %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("item %q not found", id)
	}
	return nil
}

// AllForExport returns up to 10,000 non-deleted items for CSV export.
func (r *ItemRepository) AllForExport() ([]models.Item, error) {
	rows, err := r.db.Query(`
		SELECT i.id, i.part_number, i.item_description, i.unit, i.currency, i.unit_price, i.hsn_code,
		       COALESCE(i.supplier_id,''), COALESCE(s.supplier_name,''), i.is_active,
		       COALESCE(i.country_of_origin,''), COALESCE(i.bcd,''), COALESCE(i.sws,''), COALESCE(i.igst,''),
		       COALESCE(i.technical_write_up,''), COALESCE(i.category,''), COALESCE(i.end_use,''),
		       COALESCE(i.net_weight_kg,0), COALESCE(i.purchase_uom,''), COALESCE(i.gross_weight_per_uom_kg,0),
		       COALESCE(i.photo_path,'')
		FROM items i
		LEFT JOIN suppliers s ON s.id = i.supplier_id
		WHERE i.deleted_at IS NULL
		ORDER BY i.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		it, err := scanItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItemRow(row rowScanner) (models.Item, error) {
	var it models.Item
	err := row.Scan(
		&it.ID, &it.PartNumber, &it.ItemDescription, &it.Unit, &it.Currency, &it.UnitPrice, &it.HSNCode,
		&it.SupplierID, &it.SupplierName, &it.IsActive,
		&it.CountryOfOrigin, &it.BCD, &it.SWS, &it.IGST,
		&it.TechnicalWriteUp, &it.Category, &it.EndUse,
		&it.NetWeightKg, &it.PurchaseUOM, &it.GrossWeightPerUOMKg, &it.PhotoPath,
	)
	return it, err
}
