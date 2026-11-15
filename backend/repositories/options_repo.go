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
	"insurance_rates":   true,
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

var defaultCanonicalOptions = map[string][]models.LookupOption{
	"countries": {
		{Value: "India", Label: "India"},
		{Value: "China", Label: "China"},
		{Value: "S.Korea", Label: "S.Korea"},
		{Value: "France", Label: "France"},
		{Value: "Germany", Label: "Germany"},
		{Value: "Czech Republic", Label: "Czech Republic"},
		{Value: "Vietnam", Label: "Vietnam"},
		{Value: "UK", Label: "UK"},
		{Value: "USA", Label: "USA"},
		{Value: "Japan", Label: "Japan"},
		{Value: "Italy", Label: "Italy"},
		{Value: "Taiwan", Label: "Taiwan"},
		{Value: "Thailand", Label: "Thailand"},
		{Value: "Malaysia", Label: "Malaysia"},
		{Value: "Singapore", Label: "Singapore"},
		{Value: "Switzerland", Label: "Switzerland"},
		{Value: "UAE", Label: "UAE"},
		{Value: "Netherlands", Label: "Netherlands"},
	},
	"currencies": {
		{Value: "USD", Label: "US Dollar (USD)"},
		{Value: "EUR", Label: "Euro (EUR)"},
		{Value: "INR", Label: "Indian Rupee (INR)"},
		{Value: "GBP", Label: "British Pound (GBP)"},
		{Value: "JPY", Label: "Japanese Yen (JPY)"},
		{Value: "CNY", Label: "Chinese Yuan (CNY)"},
		{Value: "KRW", Label: "South Korean Won (KRW)"},
		{Value: "SGD", Label: "Singapore Dollar (SGD)"},
		{Value: "AED", Label: "UAE Dirham (AED)"},
		{Value: "CHF", Label: "Swiss Franc (CHF)"},
	},
	"incoterms": {
		{Value: "CIF", Label: "Cost, Insurance & Freight (CIF)"},
		{Value: "FOB", Label: "Free On Board (FOB)"},
		{Value: "CFR", Label: "Cost & Freight (CFR)"},
		{Value: "EXW", Label: "Ex Works (EXW)"},
		{Value: "DAP", Label: "Delivered at Place (DAP)"},
		{Value: "DDP", Label: "Delivered Duty Paid (DDP)"},
		{Value: "FCA", Label: "Free Carrier (FCA)"},
		{Value: "CPT", Label: "Carriage Paid To (CPT)"},
		{Value: "CIP", Label: "Carriage & Insurance Paid (CIP)"},
	},
	"shipment_modes": {
		{Value: "Sea", Label: "Sea Freight"},
		{Value: "Air", Label: "Air Freight"},
		{Value: "Road", Label: "Road Transport"},
		{Value: "Rail", Label: "Rail Freight"},
		{Value: "Courier", Label: "Express Courier"},
	},
	"shipment_types": {
		{Value: "FCL", Label: "Full Container Load (FCL)"},
		{Value: "LCL", Label: "Less than Container Load (LCL)"},
		{Value: "Air Cargo", Label: "Air Cargo"},
		{Value: "Courier", Label: "Courier Parcel"},
		{Value: "Breakbulk", Label: "Breakbulk"},
	},
	"shipment_statuses": {
		{Value: "docs-rcvd", Label: "Documents Received"},
		{Value: "docu-received", Label: "Docu Received"},
		{Value: "in-transit", Label: "In Transit"},
		{Value: "customs-clearance", Label: "Customs Clearance"},
		{Value: "ready-dly", Label: "Ready for Delivery"},
		{Value: "delivered", Label: "Delivered"},
		{Value: "finalized", Label: "Finalized"},
		{Value: "completed", Label: "Completed"},
		{Value: "cancelled", Label: "Cancelled"},
		{Value: "closed", Label: "Closed"},
	},
	"categories": {
		{Value: "General", Label: "General"},
		{Value: "Raw Materials", Label: "Raw Materials"},
		{Value: "Capital Goods", Label: "Capital Goods"},
		{Value: "Spares & Components", Label: "Spares & Components"},
		{Value: "Consumables", Label: "Consumables"},
		{Value: "Finished Goods", Label: "Finished Goods"},
		{Value: "Electronics", Label: "Electronics"},
		{Value: "Machinery", Label: "Machinery"},
	},
	"end_uses": {
		{Value: "Manufacturing", Label: "Manufacturing"},
		{Value: "Industrial", Label: "Industrial"},
		{Value: "Captive Consumption", Label: "Captive Consumption"},
		{Value: "Trading", Label: "Trading"},
		{Value: "R&D", Label: "Research & Development"},
		{Value: "Commercial", Label: "Commercial"},
	},
	"units": {
		{Value: "PCS", Label: "Pieces (PCS)"},
		{Value: "KGS", Label: "Kilograms (KGS)"},
		{Value: "NOS", Label: "Numbers (NOS)"},
		{Value: "SET", Label: "Set (SET)"},
		{Value: "MTR", Label: "Meters (MTR)"},
		{Value: "LTR", Label: "Liters (LTR)"},
		{Value: "PKT", Label: "Packets (PKT)"},
		{Value: "BOX", Label: "Box (BOX)"},
	},
	"purchase_uoms": {
		{Value: "PCS", Label: "Pieces (PCS)"},
		{Value: "BOX", Label: "Box (BOX)"},
		{Value: "CTN", Label: "Carton (CTN)"},
		{Value: "PKT", Label: "Packet (PKT)"},
		{Value: "ROLL", Label: "Roll (ROLL)"},
		{Value: "SET", Label: "Set (SET)"},
		{Value: "DRUM", Label: "Drum (DRUM)"},
	},
	"bcd_rates": {
		{Value: "0", Label: "0%"},
		{Value: "2.5", Label: "2.5%"},
		{Value: "5", Label: "5%"},
		{Value: "7.5", Label: "7.5%"},
		{Value: "10", Label: "10%"},
		{Value: "15", Label: "15%"},
		{Value: "20", Label: "20%"},
		{Value: "25", Label: "25%"},
	},
	"sws_rates": {
		{Value: "0", Label: "0%"},
		{Value: "10", Label: "10%"},
	},
	"insurance_rates": {
		{Value: "0.015", Label: "0.015% (Open Cover Policy)"},
		{Value: "1.125", Label: "1.125% (Statutory Deemed Rule 10(2))"},
	},
	"igst_rates": {
		{Value: "0", Label: "0%"},
		{Value: "5", Label: "5%"},
		{Value: "12", Label: "12%"},
		{Value: "18", Label: "18%"},
		{Value: "28", Label: "28%"},
	},
}

// SeedDefaultsIfEmpty populates any empty lookup option table with canonical ERP defaults.
func (r *OptionsRepository) SeedDefaultsIfEmpty() error {
	for tbl, defs := range defaultCanonicalOptions {
		var count int
		if err := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, tbl)).Scan(&count); err != nil {
			continue
		}
		if count > 0 {
			continue
		}
		for _, d := range defs {
			if _, err := r.db.Exec(
				fmt.Sprintf(`INSERT OR IGNORE INTO "%s" (value, label) VALUES (?, ?)`, tbl),
				d.Value, d.Label,
			); err != nil {
				return fmt.Errorf("seed default option %s (%s): %w", tbl, d.Value, err)
			}
		}
	}
	return nil
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
