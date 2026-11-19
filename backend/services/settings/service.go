package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/transactions"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const (
	SettingsSchemaVersion = 1
	profileStorageKey     = "erp_settings_profile"
)

var managedModules = []string{
	"shipment",
	"invoice",
	"boe",
	"boeSummary",
	"supplier",
	"itemMaster",
	"expenses",
	"cbicRates",
	"reconciledBoe",
}

// Service manages versioned ERP theme, formatting, invoice precision, and module display profiles.
type Service struct {
	db    *sql.DB
	sec   *security.Service
	audit *audit.Service
}

func NewService(db *sql.DB, sec *security.Service, aud *audit.Service) *Service {
	return &Service{db: db, sec: sec, audit: aud}
}

// DefaultProfile builds canonical version-1 settings matching legacy defaults.
func DefaultProfile() models.AppSettingsProfile {
	mods := make(map[string]models.ModuleDisplaySettings, len(managedModules))
	for _, m := range managedModules {
		mods[m] = models.ModuleDisplaySettings{
			VisibleFields:    []string{},
			ItemsPerPage:     10,
			DecimalPrecision: 2,
			DateFormat:       "DD/MM/YYYY",
			FieldCases:       map[string]string{},
		}
	}
	mods["shipment"].FieldCases["shipmentType"] = "uppercase"

	return models.AppSettingsProfile{
		SchemaVersion:        SettingsSchemaVersion,
		Theme:                "default",
		Mode:                 "system",
		LineTotalDecimals:    2,
		InvoiceTotalDecimals: 2,
		NumberFormat:         "en-IN",
		DateFormat:           "DD/MM/YYYY",
		TextFormat:           "preserve",
		Modules:              mods,
	}
}

// GetProfile loads the persisted ERP settings profile or falls back cleanly to defaults.
func (s *Service) GetProfile() (models.AppSettingsProfile, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return models.AppSettingsProfile{}, err
	}
	var raw, updatedAt string
	err := s.db.QueryRow(`SELECT value, COALESCE(updated_at,'') FROM app_settings WHERE key = ?`, profileStorageKey).Scan(&raw, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultProfile(), nil
	}
	if err != nil {
		return models.AppSettingsProfile{}, err
	}
	p := DefaultProfile()
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return DefaultProfile(), nil
	}
	enforceInvariants(&p)
	p.UpdatedAt = updatedAt
	return p, nil
}

// SaveProfile validates schema invariants (0|2 decimals, locked uppercase shipmentType),
// saves into app_settings, and records an audit trail entry.
func (s *Service) SaveProfile(p models.AppSettingsProfile) (models.AppSettingsProfile, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.AppSettingsProfile{}, err
	}
	if p.LineTotalDecimals != 0 && p.LineTotalDecimals != 2 {
		return models.AppSettingsProfile{}, fmt.Errorf("lineTotalDecimals must be 0 or 2 (got %d)", p.LineTotalDecimals)
	}
	if p.InvoiceTotalDecimals != 0 && p.InvoiceTotalDecimals != 2 {
		return models.AppSettingsProfile{}, fmt.Errorf("invoiceTotalDecimals must be 0 or 2 (got %d)", p.InvoiceTotalDecimals)
	}
	if p.Theme != "default" && p.Theme != "steel" {
		return models.AppSettingsProfile{}, fmt.Errorf("unsupported theme preset %q", p.Theme)
	}
	if p.Mode != "light" && p.Mode != "dark" && p.Mode != "system" {
		return models.AppSettingsProfile{}, fmt.Errorf("unsupported color mode %q", p.Mode)
	}
	enforceInvariants(&p)

	blob, err := json.Marshal(p)
	if err != nil {
		return models.AppSettingsProfile{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	err = transactions.WithinTx(context.Background(), s.db, func(tx *sql.Tx) error {
		upserts := map[string]string{
			profileStorageKey:      string(blob),
			"theme":                p.Theme,
			"mode":                 p.Mode,
			"lineTotalDecimals":    fmt.Sprintf("%d", p.LineTotalDecimals),
			"invoiceTotalDecimals": fmt.Sprintf("%d", p.InvoiceTotalDecimals),
			"numberFormat":         p.NumberFormat,
			"dateFormat":           p.DateFormat,
		}
		for k, v := range upserts {
			if _, err := tx.Exec(`
				INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
				ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
				k, v, now,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.AppSettingsProfile{}, err
	}
	_ = s.audit.RecordTableAudit("app_settings", profileStorageKey, "UPDATE", sess.UserID, nil, p, nil)
	p.UpdatedAt = now
	return p, nil
}

// ResetModuleDefaults restores one module's display profile to defaults while preserving global settings.
func (s *Service) ResetModuleDefaults(moduleKey string) (models.AppSettingsProfile, error) {
	cur, err := s.GetProfile()
	if err != nil {
		return models.AppSettingsProfile{}, err
	}
	def := DefaultProfile()
	modDef, ok := def.Modules[moduleKey]
	if !ok {
		return models.AppSettingsProfile{}, fmt.Errorf("unknown module key %q", moduleKey)
	}
	if cur.Modules == nil {
		cur.Modules = map[string]models.ModuleDisplaySettings{}
	}
	cur.Modules[moduleKey] = modDef
	return s.SaveProfile(cur)
}

func enforceInvariants(p *models.AppSettingsProfile) {
	p.SchemaVersion = SettingsSchemaVersion
	if p.Modules == nil {
		p.Modules = DefaultProfile().Modules
	}
	for _, key := range managedModules {
		mod, ok := p.Modules[key]
		if !ok {
			mod = DefaultProfile().Modules[key]
		}
		if mod.ItemsPerPage < 5 || mod.ItemsPerPage > 200 {
			mod.ItemsPerPage = 10
		}
		if mod.FieldCases == nil {
			mod.FieldCases = map[string]string{}
		}
		p.Modules[key] = mod
	}
	// Legacy contract: shipment.fields.shipmentType.case locked to uppercase.
	sh := p.Modules["shipment"]
	sh.FieldCases["shipmentType"] = "uppercase"
	p.Modules["shipment"] = sh
}
