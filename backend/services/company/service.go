package company

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

const (
	registryFileName = "companies_registry.json"
	companiesDirName = "companies"
)

// SwitchDBFunc is a callback provided by Runtime to rebind all services to a newly opened database.
type SwitchDBFunc func(newDBPath string, compID string) (*sql.DB, error)

// Service provides multi-company management, dynamic switching, and company profile operations.
type Service struct {
	mu           sync.RWMutex
	dataDir      string
	registryPath string
	companiesDir string
	defaultDB    string
	dbKey        string
	appVersion   string
	activeID     string
	activeDB     *sql.DB
	switchDB     SwitchDBFunc
	sec          *security.Service
	audit        *audit.Service
}

// NewService initializes the company service, ensuring registry persistence.
func NewService(
	dataDir string,
	defaultDBPath string,
	dbKey string,
	appVersion string,
	activeDB *sql.DB,
	sec *security.Service,
	aud *audit.Service,
	switchDB SwitchDBFunc,
) (*Service, error) {
	if dataDir == "" {
		return nil, errors.New("data directory is required")
	}

	companiesDir := filepath.Join(dataDir, companiesDirName)
	if err := os.MkdirAll(companiesDir, 0o700); err != nil {
		return nil, fmt.Errorf("create companies directory: %w", err)
	}

	svc := &Service{
		dataDir:      dataDir,
		registryPath: filepath.Join(dataDir, registryFileName),
		companiesDir: companiesDir,
		defaultDB:    defaultDBPath,
		dbKey:        dbKey,
		appVersion:   appVersion,
		activeDB:     activeDB,
		sec:          sec,
		audit:        aud,
		switchDB:     switchDB,
	}

	if err := svc.initRegistry(); err != nil {
		return nil, fmt.Errorf("initialize company registry: %w", err)
	}

	return svc, nil
}

// SetActiveDB updates the active database reference held by the company service.
func (s *Service) SetActiveDB(db *sql.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeDB = db
}

// GetActiveID returns the ID of the currently active company.
func (s *Service) GetActiveID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeID
}

// initRegistry creates or validates the companies_registry.json file.
func (s *Service) initRegistry() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.registryPath); os.IsNotExist(err) {
		// First-time setup: establish default primary company pointing to existing databasePath.
		now := time.Now().UTC()
		initialReg := models.CompanyRegistryFile{
			ActiveCompanyID: "comp_default",
			Companies: []models.CompanyRegistryItem{
				{
					ID:           "comp_default",
					Name:         "Primary Company",
					ShortCode:    "PRIMARY",
					DatabasePath: s.defaultDB,
					IsDefault:    true,
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			},
		}

		if writeErr := s.saveRegistryUnlocked(initialReg); writeErr != nil {
			return writeErr
		}
		s.activeID = "comp_default"
		return nil
	}

	reg, err := s.loadRegistryUnlocked()
	if err != nil {
		return err
	}

	if reg.ActiveCompanyID == "" && len(reg.Companies) > 0 {
		reg.ActiveCompanyID = reg.Companies[0].ID
		_ = s.saveRegistryUnlocked(reg)
	}
	s.activeID = reg.ActiveCompanyID
	return nil
}

func (s *Service) loadRegistryUnlocked() (models.CompanyRegistryFile, error) {
	data, err := os.ReadFile(s.registryPath)
	if err != nil {
		return models.CompanyRegistryFile{}, fmt.Errorf("read registry: %w", err)
	}

	var reg models.CompanyRegistryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		return models.CompanyRegistryFile{}, fmt.Errorf("parse registry: %w", err)
	}

	return reg, nil
}

func (s *Service) saveRegistryUnlocked(reg models.CompanyRegistryFile) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	tmpPath := s.registryPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp registry: %w", err)
	}

	if err := os.Rename(tmpPath, s.registryPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace registry file: %w", err)
	}

	return nil
}

// ListCompanies returns all registered companies with their details.
func (s *Service) ListCompanies() ([]models.Company, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reg, err := s.loadRegistryUnlocked()
	if err != nil {
		return nil, err
	}

	result := make([]models.Company, 0, len(reg.Companies))
	for _, item := range reg.Companies {
		isActive := item.ID == s.activeID
		comp := models.Company{
			ID:           item.ID,
			Name:         item.Name,
			ShortCode:    item.ShortCode,
			DatabasePath: item.DatabasePath,
			IsDefault:    item.IsDefault,
			IsActive:     isActive,
			CreatedAt:    item.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    item.UpdatedAt.Format(time.RFC3339),
		}

		if isActive && s.activeDB != nil {
			// Enrich with full details from the active database
			profile, err := s.readCompanyProfileFromDB(s.activeDB, item.ID)
			if err == nil {
				comp = profile
				comp.DatabasePath = item.DatabasePath
				comp.IsDefault = item.IsDefault
				comp.IsActive = true
			}
		}

		result = append(result, comp)
	}

	return result, nil
}

// GetActiveCompany returns detailed corporate identity and banking fields for the currently active database.
func (s *Service) GetActiveCompany() (models.Company, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reg, err := s.loadRegistryUnlocked()
	if err != nil {
		return models.Company{}, err
	}

	var regItem *models.CompanyRegistryItem
	for i := range reg.Companies {
		if reg.Companies[i].ID == s.activeID {
			regItem = &reg.Companies[i]
			break
		}
	}

	if regItem == nil {
		if len(reg.Companies) > 0 {
			regItem = &reg.Companies[0]
		} else {
			return models.Company{}, errors.New("no registered companies found")
		}
	}

	if s.activeDB != nil {
		profile, err := s.readCompanyProfileFromDB(s.activeDB, regItem.ID)
		if err == nil {
			profile.DatabasePath = regItem.DatabasePath
			profile.IsDefault = regItem.IsDefault
			profile.IsActive = true
			return profile, nil
		}
	}

	return models.Company{
		ID:           regItem.ID,
		Name:         regItem.Name,
		ShortCode:    regItem.ShortCode,
		DatabasePath: regItem.DatabasePath,
		IsDefault:    regItem.IsDefault,
		IsActive:     true,
		CreatedAt:    regItem.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    regItem.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// CreateCompany provisions an isolated encrypted SQLCipher database for a new corporate entity.
// It applies all versioned migrations, seeds clean lookup tables, and leaves operational tables fresh.
func (s *Service) CreateCompany(input models.CreateCompanyInput) (models.Company, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.Company{}, errors.New("company name is required")
	}

	shortCode := strings.ToUpper(strings.TrimSpace(input.ShortCode))
	if shortCode == "" {
		shortCode = generateShortCode(name)
	}
	cleanCodeRegex := regexp.MustCompile(`^[A-Z0-9_\-]{2,12}$`)
	if !cleanCodeRegex.MatchString(shortCode) {
		return models.Company{}, errors.New("short code must be 2-12 alphanumeric uppercase characters")
	}

	s.mu.Lock()
	reg, err := s.loadRegistryUnlocked()
	if err != nil {
		s.mu.Unlock()
		return models.Company{}, err
	}

	for _, c := range reg.Companies {
		if strings.EqualFold(c.ShortCode, shortCode) {
			s.mu.Unlock()
			return models.Company{}, fmt.Errorf("short code '%s' is already in use by company '%s'", shortCode, c.Name)
		}
		if strings.EqualFold(c.Name, name) {
			s.mu.Unlock()
			return models.Company{}, fmt.Errorf("company name '%s' is already registered", name)
		}
	}

	compID := fmt.Sprintf("comp_%s_%s", strings.ToLower(shortCode), randomHex(4))
	dbFileName := fmt.Sprintf("company_%s.db", strings.ToLower(shortCode))
	targetDBPath := filepath.Join(s.companiesDir, dbFileName)

	// Ensure target database does not overwrite existing file
	if _, statErr := os.Stat(targetDBPath); statErr == nil {
		dbFileName = fmt.Sprintf("company_%s_%s.db", strings.ToLower(shortCode), randomHex(4))
		targetDBPath = filepath.Join(s.companiesDir, dbFileName)
	}

	activeDBCopy := s.activeDB
	s.mu.Unlock()

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	baseCurrency := input.BaseCurrency
	if strings.TrimSpace(baseCurrency) == "" {
		baseCurrency = "INR"
	}
	fyFrom := input.FinancialYearFrom
	if strings.TrimSpace(fyFrom) == "" {
		fyFrom = "04-01"
	}
	fyTo := input.FinancialYearTo
	if strings.TrimSpace(fyTo) == "" {
		fyTo = "03-31"
	}

	// 1-5. Provision, migrate, and seed the new database in an isolated scope.
	// Closing newMgr immediately when done ensures file locks are completely released
	// before any subsequent switch or database operations on Windows.
	initErr := func() error {
		newMgr, err := connection.OpenEncrypted(targetDBPath, s.dbKey)
		if err != nil {
			return fmt.Errorf("provision encrypted company database: %w", err)
		}
		defer newMgr.Close()

		ctx := context.Background()
		_, err = migrations.Run(ctx, newMgr.DB(), s.appVersion, nil)
		if err != nil {
			return fmt.Errorf("initialize schema migrations for new company: %w", err)
		}

		// Clean dummy suppliers from 0004 seed defaults as requested ("fresh/clean tables for different companies")
		_, _ = newMgr.DB().Exec(`DELETE FROM suppliers WHERE id LIKE 'Sup-%';`)
		// Ensure company profile starts clean for this specific corporate entity
		_, _ = newMgr.DB().Exec(`DELETE FROM company_profile;`)

		// Inherit admin user credentials so login works if opened directly
		if activeDBCopy != nil {
			s.copyAdminSettings(activeDBCopy, newMgr.DB())
		}

		// Store company profile in the new database
		_, err = newMgr.DB().Exec(`
			INSERT INTO company_profile (
				id, name, short_code, iec_code, gstin, pan,
				address_line1, address_line2, city, state, postal_code, country,
				phone, email, website,
				beneficiary_name, bank_name, account_number, ifsc_code, swift_code, bank_branch, bank_address,
				logo_data, base_currency, financial_year_from, financial_year_to, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
		`,
			compID, name, shortCode, input.IECCode, input.GSTIN, input.PAN,
			input.AddressLine1, input.AddressLine2, input.City, input.State, input.PostalCode, input.Country,
			input.Phone, input.Email, input.Website,
			input.BeneficiaryName, input.BankName, input.AccountNumber, input.IFSCCode, input.SwiftCode, input.BankBranch, input.BankAddress,
			input.LogoData, baseCurrency, fyFrom, fyTo, nowStr, nowStr,
		)
		if err != nil {
			return fmt.Errorf("seed company profile record: %w", err)
		}

		return nil
	}()

	if initErr != nil {
		_ = os.Remove(targetDBPath)
		return models.Company{}, initErr
	}

	// 6. Register company in central companies_registry.json
	regItem := models.CompanyRegistryItem{
		ID:           compID,
		Name:         name,
		ShortCode:    shortCode,
		DatabasePath: targetDBPath,
		IsDefault:    false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.mu.Lock()
	reg, err = s.loadRegistryUnlocked()
	if err != nil {
		s.mu.Unlock()
		return models.Company{}, err
	}
	reg.Companies = append(reg.Companies, regItem)
	if saveErr := s.saveRegistryUnlocked(reg); saveErr != nil {
		s.mu.Unlock()
		return models.Company{}, saveErr
	}
	s.mu.Unlock()

	newCompany := models.Company{
		ID:                compID,
		Name:              name,
		ShortCode:         shortCode,
		IECCode:           input.IECCode,
		GSTIN:             input.GSTIN,
		PAN:               input.PAN,
		AddressLine1:      input.AddressLine1,
		AddressLine2:      input.AddressLine2,
		City:              input.City,
		State:             input.State,
		PostalCode:        input.PostalCode,
		Country:           input.Country,
		Phone:             input.Phone,
		Email:             input.Email,
		Website:           input.Website,
		BeneficiaryName:   input.BeneficiaryName,
		BankName:          input.BankName,
		AccountNumber:     input.AccountNumber,
		IFSCCode:          input.IFSCCode,
		SwiftCode:         input.SwiftCode,
		BankBranch:        input.BankBranch,
		BankAddress:       input.BankAddress,
		LogoData:          input.LogoData,
		BaseCurrency:      baseCurrency,
		FinancialYearFrom: fyFrom,
		FinancialYearTo:   fyTo,
		DatabasePath:      targetDBPath,
		IsDefault:         false,
		IsActive:          false,
		CreatedAt:         nowStr,
		UpdatedAt:         nowStr,
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity("admin", "COMPANY_CREATED", "company", compID, "SUCCESS", "INFO", map[string]any{
			"name":      name,
			"shortCode": shortCode,
			"dbPath":    targetDBPath,
		})
	}

	// 7. If requested, switch to the new company immediately (outside any lock)
	if input.SwitchImmediately && s.switchDB != nil {
		switched, switchErr := s.SwitchCompany(compID)
		if switchErr == nil {
			return switched, nil
		}
	}

	return newCompany, nil
}

// UpdateCompany updates company identity, tax numbers, and banking details.
func (s *Service) UpdateCompany(input models.UpdateCompanyInput) (models.Company, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = s.activeID
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.Company{}, errors.New("company name is required")
	}

	shortCode := strings.ToUpper(strings.TrimSpace(input.ShortCode))
	if shortCode == "" {
		shortCode = generateShortCode(name)
	}

	reg, err := s.loadRegistryUnlocked()
	if err != nil {
		return models.Company{}, err
	}

	var regItem *models.CompanyRegistryItem
	for i := range reg.Companies {
		if reg.Companies[i].ID == id {
			regItem = &reg.Companies[i]
		} else {
			if strings.EqualFold(reg.Companies[i].ShortCode, shortCode) {
				return models.Company{}, fmt.Errorf("short code '%s' already used by another company", shortCode)
			}
		}
	}

	if regItem == nil {
		return models.Company{}, fmt.Errorf("company with id '%s' not found in registry", id)
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// Update in registry
	regItem.Name = name
	regItem.ShortCode = shortCode
	regItem.UpdatedAt = now
	_ = s.saveRegistryUnlocked(reg)

	// Update in database (active or specific DB)
	targetDB := s.activeDB
	var tempMgr *connection.Manager
	if id != s.activeID {
		var openErr error
		tempMgr, openErr = connection.OpenEncrypted(regItem.DatabasePath, s.dbKey)
		if openErr == nil {
			defer tempMgr.Close()
			targetDB = tempMgr.DB()
		}
	}

	if targetDB != nil {
		_, _ = targetDB.Exec(`
			INSERT INTO company_profile (
				id, name, short_code, iec_code, gstin, pan,
				address_line1, address_line2, city, state, postal_code, country,
				phone, email, website,
				beneficiary_name, bank_name, account_number, ifsc_code, swift_code, bank_branch, bank_address,
				logo_data, base_currency, financial_year_from, financial_year_to, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name = excluded.name,
				short_code = excluded.short_code,
				iec_code = excluded.iec_code,
				gstin = excluded.gstin,
				pan = excluded.pan,
				address_line1 = excluded.address_line1,
				address_line2 = excluded.address_line2,
				city = excluded.city,
				state = excluded.state,
				postal_code = excluded.postal_code,
				country = excluded.country,
				phone = excluded.phone,
				email = excluded.email,
				website = excluded.website,
				beneficiary_name = excluded.beneficiary_name,
				bank_name = excluded.bank_name,
				account_number = excluded.account_number,
				ifsc_code = excluded.ifsc_code,
				swift_code = excluded.swift_code,
				bank_branch = excluded.bank_branch,
				bank_address = excluded.bank_address,
				logo_data = excluded.logo_data,
				base_currency = excluded.base_currency,
				financial_year_from = excluded.financial_year_from,
				financial_year_to = excluded.financial_year_to,
				updated_at = excluded.updated_at;
		`,
			id, name, shortCode, input.IECCode, input.GSTIN, input.PAN,
			input.AddressLine1, input.AddressLine2, input.City, input.State, input.PostalCode, input.Country,
			input.Phone, input.Email, input.Website,
			input.BeneficiaryName, input.BankName, input.AccountNumber, input.IFSCCode, input.SwiftCode, input.BankBranch, input.BankAddress,
			input.LogoData, input.BaseCurrency, input.FinancialYearFrom, input.FinancialYearTo, nowStr,
		)
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity("admin", "COMPANY_UPDATED", "company", id, "SUCCESS", "INFO", map[string]any{
			"name":      name,
			"shortCode": shortCode,
			"gstin":     input.GSTIN,
			"iec":       input.IECCode,
		})
	}

	return models.Company{
		ID:                id,
		Name:              name,
		ShortCode:         shortCode,
		IECCode:           input.IECCode,
		GSTIN:             input.GSTIN,
		PAN:               input.PAN,
		AddressLine1:      input.AddressLine1,
		AddressLine2:      input.AddressLine2,
		City:              input.City,
		State:             input.State,
		PostalCode:        input.PostalCode,
		Country:           input.Country,
		Phone:             input.Phone,
		Email:             input.Email,
		Website:           input.Website,
		BeneficiaryName:   input.BeneficiaryName,
		BankName:          input.BankName,
		AccountNumber:     input.AccountNumber,
		IFSCCode:          input.IFSCCode,
		SwiftCode:         input.SwiftCode,
		BankBranch:        input.BankBranch,
		BankAddress:       input.BankAddress,
		LogoData:          input.LogoData,
		BaseCurrency:      input.BaseCurrency,
		FinancialYearFrom: input.FinancialYearFrom,
		FinancialYearTo:   input.FinancialYearTo,
		DatabasePath:      regItem.DatabasePath,
		IsDefault:         regItem.IsDefault,
		IsActive:          id == s.activeID,
		UpdatedAt:         nowStr,
	}, nil
}

// SwitchCompany dynamically switches the active application database to the specified company.
func (s *Service) SwitchCompany(id string) (models.Company, error) {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return models.Company{}, errors.New("company ID is required")
	}

	s.mu.RLock()
	reg, err := s.loadRegistryUnlocked()
	if err != nil {
		s.mu.RUnlock()
		return models.Company{}, err
	}

	var targetItem *models.CompanyRegistryItem
	for i := range reg.Companies {
		if reg.Companies[i].ID == cleanID {
			targetItem = &reg.Companies[i]
			break
		}
	}

	if targetItem == nil {
		s.mu.RUnlock()
		return models.Company{}, fmt.Errorf("company with id '%s' not found", cleanID)
	}

	targetPath := targetItem.DatabasePath
	targetName := targetItem.Name
	targetShortCode := targetItem.ShortCode
	targetIsDefault := targetItem.IsDefault
	s.mu.RUnlock()

	if s.switchDB == nil {
		return models.Company{}, errors.New("dynamic database switcher callback not configured")
	}

	// CRITICAL: Execute database switch OUTSIDE of s.mu lock!
	// Inside s.switchDB, rt.SwitchDatabase calls r.Company.SetActiveDB(...) and performs
	// WAL checkpoint, migration, integrity check, and service rebinding.
	// Holding s.mu while switchDB runs causes a circular deadlock on SetActiveDB.
	newDB, err := s.switchDB(targetPath, cleanID)
	if err != nil {
		return models.Company{}, fmt.Errorf("switch company database: %w", err)
	}

	s.mu.Lock()
	s.activeDB = newDB
	s.activeID = cleanID

	// Persist active ID in registry
	reg, err = s.loadRegistryUnlocked()
	if err == nil {
		reg.ActiveCompanyID = cleanID
		_ = s.saveRegistryUnlocked(reg)
	}
	s.mu.Unlock()

	if s.audit != nil {
		_ = s.audit.RecordUserActivity("admin", "COMPANY_SWITCHED", "company", cleanID, "SUCCESS", "INFO", map[string]any{
			"companyName": targetName,
			"shortCode":   targetShortCode,
			"dbPath":      targetPath,
		})
	}

	profile, err := s.readCompanyProfileFromDB(newDB, cleanID)
	if err == nil {
		profile.DatabasePath = targetPath
		profile.IsDefault = targetIsDefault
		profile.IsActive = true
		return profile, nil
	}

	return models.Company{
		ID:           cleanID,
		Name:         targetName,
		ShortCode:    targetShortCode,
		DatabasePath: targetPath,
		IsDefault:    targetIsDefault,
		IsActive:     true,
	}, nil
}

// DeleteCompany removes a company from registry if it is not currently active or the only one remaining.
func (s *Service) DeleteCompany(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanID := strings.TrimSpace(id)
	if cleanID == s.activeID {
		return errors.New("cannot delete the currently active company; switch to another company first")
	}

	reg, err := s.loadRegistryUnlocked()
	if err != nil {
		return err
	}

	if len(reg.Companies) <= 1 {
		return errors.New("cannot delete the only registered company")
	}

	var targetIndex = -1
	var targetItem models.CompanyRegistryItem
	for i, c := range reg.Companies {
		if c.ID == cleanID {
			targetIndex = i
			targetItem = c
			break
		}
	}

	if targetIndex == -1 {
		return fmt.Errorf("company '%s' not found in registry", cleanID)
	}

	if targetItem.IsDefault {
		return errors.New("cannot delete the primary default company")
	}

	// Remove from list
	reg.Companies = append(reg.Companies[:targetIndex], reg.Companies[targetIndex+1:]...)
	if err := s.saveRegistryUnlocked(reg); err != nil {
		return err
	}

	// Safely rename database file to .deleted
	if targetItem.DatabasePath != "" {
		_ = os.Rename(targetItem.DatabasePath, targetItem.DatabasePath+".deleted")
	}

	if s.audit != nil {
		_ = s.audit.RecordUserActivity("admin", "COMPANY_DELETED", "company", cleanID, "SUCCESS", "WARNING", map[string]any{
			"name":      targetItem.Name,
			"shortCode": targetItem.ShortCode,
		})
	}

	return nil
}

func (s *Service) readCompanyProfileFromDB(db *sql.DB, fallbackID string) (models.Company, error) {
	var c models.Company
	row := db.QueryRow(`
		SELECT id, name, short_code, iec_code, gstin, pan,
			address_line1, address_line2, city, state, postal_code, country,
			phone, email, website,
			beneficiary_name, bank_name, account_number, ifsc_code, swift_code, bank_branch, bank_address,
			logo_data, base_currency, financial_year_from, financial_year_to, created_at, updated_at
		FROM company_profile
		WHERE id = ? OR ? = ''
		ORDER BY CASE WHEN id = ? THEN 0 ELSE 1 END, created_at DESC
		LIMIT 1;
	`, fallbackID, fallbackID, fallbackID)

	err := row.Scan(
		&c.ID, &c.Name, &c.ShortCode, &c.IECCode, &c.GSTIN, &c.PAN,
		&c.AddressLine1, &c.AddressLine2, &c.City, &c.State, &c.PostalCode, &c.Country,
		&c.Phone, &c.Email, &c.Website,
		&c.BeneficiaryName, &c.BankName, &c.AccountNumber, &c.IFSCCode, &c.SwiftCode, &c.BankBranch, &c.BankAddress,
		&c.LogoData, &c.BaseCurrency, &c.FinancialYearFrom, &c.FinancialYearTo, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return models.Company{ID: fallbackID}, err
	}

	return c, nil
}

func (s *Service) copyAdminSettings(srcDB, destDB *sql.DB) {
	rows, err := srcDB.Query(`SELECT key, value FROM app_settings WHERE key LIKE 'auth.%'`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			_, _ = destDB.Exec(`INSERT OR REPLACE INTO app_settings (key, value, updated_at) VALUES (?, ?, datetime('now', 'utc'))`, k, v)
		}
	}
}

func generateShortCode(name string) string {
	clean := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(name, "")
	if len(clean) >= 4 {
		return strings.ToUpper(clean[:4])
	}
	if len(clean) >= 2 {
		return strings.ToUpper(clean)
	}
	return "COMP"
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
