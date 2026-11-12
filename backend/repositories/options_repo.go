package repositories

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
)

// AllowedOptionTables lists every lookup table editable through OptionsRepository.
var AllowedOptionTables = map[string]bool{
	"bcd_rates":         true,
	"categories":        true,
	"countries":         true,
	"currencies":        true,
	"end_uses":          true,
	"igst_rates":        true,
	"incoterms":         true,
	"purchase_uoms":     true,
	"shipment_modes":    true,
	"shipment_statuses": true,
	"shipment_types":    true,
	"sws_rates":         true,
	"units":             true,
}

// OptionsRepository manages the 13 standardized lookup option tables.
type OptionsRepository struct {
	db *sql.DB
}

// NewOptionsRepository constructs an OptionsRepository backed by db.
func NewOptionsRepository(db *sql.DB) *OptionsRepository {
	return &OptionsRepository{db: db}
}

func validateOptionTable(table string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(table))
	if !AllowedOptionTables[key] {
		return "", fmt.Errorf("unsupported lookup option table %q", table)
	}
	return key, nil
}

// List returns all options sorted alphabetically by label.
func (r *OptionsRepository) List(table string) ([]models.LookupOption, error) {
	tbl, err := validateOptionTable(table)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(fmt.Sprintf(`SELECT value, label FROM "%s" ORDER BY label COLLATE NOCASE ASC`, tbl))
	if err != nil {
		return nil, fmt.Errorf("list options %s: %w", tbl, err)
	}
	defer rows.Close()

	var items []models.LookupOption
	for rows.Next() {
		var opt models.LookupOption
		if err := rows.Scan(&opt.Value, &opt.Label); err != nil {
			return nil, fmt.Errorf("scan option %s: %w", tbl, err)
		}
		items = append(items, opt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// Upsert inserts or updates a lookup value/label pair.
func (r *OptionsRepository) Upsert(table, value, label string) (models.LookupOption, error) {
	tbl, err := validateOptionTable(table)
	if err != nil {
		return models.LookupOption{}, err
	}
	val := strings.TrimSpace(value)
	lbl := strings.TrimSpace(label)
	if val == "" {
		val = lbl
	}
	if lbl == "" {
		lbl = val
	}
	if val == "" || lbl == "" {
		return models.LookupOption{}, fmt.Errorf("option value and label are required")
	}

	_, err = r.db.Exec(
		fmt.Sprintf(`INSERT INTO "%s" (value, label) VALUES (?, ?) ON CONFLICT(value) DO UPDATE SET label = excluded.label`, tbl),
		val, lbl,
	)
	if err != nil {
		return models.LookupOption{}, fmt.Errorf("upsert option %s: %w", tbl, err)
	}
	return models.LookupOption{Value: val, Label: lbl}, nil
}

// Delete removes an option by value.
func (r *OptionsRepository) Delete(table, value string) error {
	tbl, err := validateOptionTable(table)
	if err != nil {
		return err
	}
	res, err := r.db.Exec(fmt.Sprintf(`DELETE FROM "%s" WHERE value = ?`, tbl), strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("delete option %s: %w", tbl, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("option %q not found in %s", value, tbl)
	}
	return nil
}
