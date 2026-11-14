package options

import (
	"database/sql"
	"strings"

	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

// Service exposes CRUD operations across all 13 ERP lookup tables with RBAC and audit logging.
type Service struct {
	repo  *repositories.OptionsRepository
	sec   *security.Service
	audit *audit.Service
}

// NewService constructs an options Service and seeds empty lookup tables with defaults.
func NewService(db *sql.DB, sec *security.Service, auditSvc *audit.Service) *Service {
	repo := repositories.NewOptionsRepository(db)
	_ = repo.SeedDefaultsIfEmpty()
	return &Service{
		repo:  repo,
		sec:   sec,
		audit: auditSvc,
	}
}

// ListOptions returns all rows in one of the 13 allowed lookup tables.
func (s *Service) ListOptions(table string) ([]models.LookupOption, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	return s.repo.List(table)
}

// ListAllOptions fetches all 13 lookup option lists in one desktop RPC call.
func (s *Service) ListAllOptions() (map[string][]models.LookupOption, error) {
	if _, err := s.sec.RequirePermission(security.PermDataRead); err != nil {
		return nil, err
	}
	out := make(map[string][]models.LookupOption, len(repositories.AllowedOptionTables))
	for tbl := range repositories.AllowedOptionTables {
		rows, err := s.repo.List(tbl)
		if err != nil {
			return nil, err
		}
		out[tbl] = rows
	}
	return out, nil
}

// UpsertOption creates or updates an option label/value in table.
func (s *Service) UpsertOption(table, value, label string) (models.LookupOption, error) {
	sess, err := s.sec.RequirePermission(security.PermDataWrite)
	if err != nil {
		return models.LookupOption{}, err
	}
	opt, err := s.repo.Upsert(table, value, label)
	if err != nil {
		return models.LookupOption{}, err
	}
	_ = s.audit.RecordTableAudit(strings.ToLower(table), opt.Value, "UPSERT", sess.UserID, nil, opt, nil)
	return opt, nil
}

// DeleteOption removes an option value from table.
func (s *Service) DeleteOption(table, value string) error {
	sess, err := s.sec.RequirePermission(security.PermDataDelete)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(table, value); err != nil {
		return err
	}
	_ = s.audit.RecordTableAudit(strings.ToLower(table), value, "DELETE", sess.UserID, map[string]string{"value": value}, nil, nil)
	return nil
}
