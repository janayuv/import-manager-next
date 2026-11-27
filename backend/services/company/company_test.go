package company_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/company"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func TestCompanyServiceLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	defaultDBPath := filepath.Join(tmpDir, "import_manager.db")
	dbKey := "test-db-key-123456"

	// 1. Initialize default DB
	mgr, err := connection.OpenEncrypted(defaultDBPath, dbKey)
	if err != nil {
		t.Fatalf("open default db: %v", err)
	}
	defer mgr.Close()

	ctx := context.Background()
	_, err = migrations.Run(ctx, mgr.DB(), "1.8.2", nil)
	if err != nil {
		t.Fatalf("run migrations on default db: %v", err)
	}

	currentMgr := mgr
	activeDB := mgr.DB()
	var switchedPath string

	switchCallback := func(newPath string, compID string) (*sql.DB, error) {
		switchedPath = newPath
		newMgr, err := connection.OpenEncrypted(newPath, dbKey)
		if err != nil {
			return nil, err
		}
		if currentMgr != nil {
			_ = currentMgr.Close()
		}
		currentMgr = newMgr
		activeDB = newMgr.DB()
		return activeDB, nil
	}
	defer func() {
		if currentMgr != nil {
			_ = currentMgr.Close()
		}
	}()

	svc, err := company.NewService(
		tmpDir,
		defaultDBPath,
		dbKey,
		"1.8.2",
		activeDB,
		nil,
		nil,
		switchCallback,
	)
	if err != nil {
		t.Fatalf("init company service: %v", err)
	}

	// Verify initial registry
	list, err := svc.ListCompanies()
	if err != nil {
		t.Fatalf("list companies: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 initial company, got %d", len(list))
	}
	if list[0].ShortCode != "PRIMARY" {
		t.Fatalf("expected shortCode PRIMARY, got %s", list[0].ShortCode)
	}

	active, err := svc.GetActiveCompany()
	if err != nil {
		t.Fatalf("get active company: %v", err)
	}
	if active.ID != "comp_default" {
		t.Fatalf("expected comp_default active, got %s", active.ID)
	}

	// 2. Create a second company
	newComp, err := svc.CreateCompany(models.CreateCompanyInput{
		Name:            "Acme Trading International",
		ShortCode:       "ACME",
		IECCode:         "0123456789",
		GSTIN:           "27AAACA1234A1Z5",
		PAN:             "AAACA1234A",
		City:            "Mumbai",
		State:           "Maharashtra",
		Country:         "India",
		BeneficiaryName: "Acme Trading International",
		BankName:        "HDFC Bank",
		AccountNumber:   "50200012345678",
		IFSCCode:        "HDFC0001234",
		SwiftCode:       "HDFCINBB",
	})
	if err != nil {
		t.Fatalf("create company: %v", err)
	}

	if newComp.ShortCode != "ACME" || newComp.IECCode != "0123456789" {
		t.Fatalf("unexpected created company fields: %+v", newComp)
	}

	// Verify that the new company database is fresh and clean (0 suppliers)
	checkMgr, err := connection.OpenEncrypted(newComp.DatabasePath, dbKey)
	if err != nil {
		t.Fatalf("open new company db: %v", err)
	}
	var supCount int
	if err := checkMgr.DB().QueryRow(`SELECT COUNT(*) FROM suppliers`).Scan(&supCount); err != nil {
		t.Fatalf("count suppliers in new company db: %v", err)
	}
	if supCount != 0 {
		t.Fatalf("expected fresh clean suppliers table (0 suppliers), got %d", supCount)
	}
	checkMgr.Close()

	// 3. Switch to the new company
	switched, err := svc.SwitchCompany(newComp.ID)
	if err != nil {
		t.Fatalf("switch to new company: %v", err)
	}
	if switched.ID != newComp.ID {
		t.Fatalf("expected switched company ID %s, got %s", newComp.ID, switched.ID)
	}
	if switchedPath != newComp.DatabasePath {
		t.Fatalf("expected switchedPath %s, got %s", newComp.DatabasePath, switchedPath)
	}

	// 4. Update the company profile
	updated, err := svc.UpdateCompany(models.UpdateCompanyInput{
		ID:        newComp.ID,
		Name:      "Acme Trading Global Ltd",
		ShortCode: "ACME",
		IECCode:   "9876543210",
		GSTIN:     "27AAACA1234A1Z5",
		PAN:       "AAACA1234A",
		City:      "Pune",
	})
	if err != nil {
		t.Fatalf("update company: %v", err)
	}
	if updated.Name != "Acme Trading Global Ltd" || updated.IECCode != "9876543210" || updated.City != "Pune" {
		t.Fatalf("unexpected updated fields: %+v", updated)
	}

	// 5. Try deleting active company (should fail)
	if err := svc.DeleteCompany(newComp.ID); err == nil {
		t.Fatalf("expected error deleting active company, got nil")
	}

	// Switch back to primary
	_, err = svc.SwitchCompany("comp_default")
	if err != nil {
		t.Fatalf("switch back to default: %v", err)
	}

	// Delete ACME
	if err := svc.DeleteCompany(newComp.ID); err != nil {
		t.Fatalf("delete non-active company: %v", err)
	}

	afterDeleteList, err := svc.ListCompanies()
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(afterDeleteList) != 1 {
		t.Fatalf("expected 1 company remaining after delete, got %d", len(afterDeleteList))
	}
}

func TestCreateCompanyWithSwitchImmediatelyAndRebind(t *testing.T) {
	tmpDir := t.TempDir()
	defaultDBPath := filepath.Join(tmpDir, "import_manager.db")
	dbKey := "test-db-key-switch-imm"

	mgr, err := connection.OpenEncrypted(defaultDBPath, dbKey)
	if err != nil {
		t.Fatalf("open default db: %v", err)
	}
	defer mgr.Close()

	ctx := context.Background()
	_, err = migrations.Run(ctx, mgr.DB(), "1.8.2", nil)
	if err != nil {
		t.Fatalf("run migrations on default db: %v", err)
	}

	currentMgr := mgr
	activeDB := mgr.DB()

	var svc *company.Service
	switchCallback := func(newPath string, compID string) (*sql.DB, error) {
		newMgr, err := connection.OpenEncrypted(newPath, dbKey)
		if err != nil {
			return nil, err
		}
		if currentMgr != nil {
			_ = currentMgr.Close()
		}
		currentMgr = newMgr
		activeDB = newMgr.DB()
		if svc != nil {
			// Tests that SetActiveDB does NOT deadlock when called from switchCallback
			svc.SetActiveDB(activeDB)
		}
		return activeDB, nil
	}
	defer func() {
		if currentMgr != nil {
			_ = currentMgr.Close()
		}
	}()

	svc, err = company.NewService(
		tmpDir,
		defaultDBPath,
		dbKey,
		"1.8.2",
		activeDB,
		nil,
		nil,
		switchCallback,
	)
	if err != nil {
		t.Fatalf("init company service: %v", err)
	}

	// Create company with SwitchImmediately: true (user's exact scenario)
	created, err := svc.CreateCompany(models.CreateCompanyInput{
		Name:              "INZI Auto",
		ShortCode:         "ICAN",
		IECCode:           "AAECI7628F",
		GSTIN:             "37AAECI7628F1ZF",
		PAN:               "AAECI7628F",
		AddressLine1:      "PLOT NO.6F, SITE-B, INDUSTRIAL PARK\nBANGALORE HYDERABAD HIGHWAY\nANANTAPUR, ANDRA PARADESH",
		PostalCode:        "515122",
		SwitchImmediately: true,
	})
	if err != nil {
		t.Fatalf("create company with switch immediately failed: %v", err)
	}

	if created.ShortCode != "ICAN" || created.Name != "INZI Auto" {
		t.Fatalf("unexpected created company: %+v", created)
	}

	if svc.GetActiveID() != created.ID {
		t.Fatalf("expected active company ID to be %s, got %s", created.ID, svc.GetActiveID())
	}
}

func TestCompanyServicePermissions(t *testing.T) {
	t.Setenv("IMPORT_MANAGER_KEYRING_FILE", filepath.Join(t.TempDir(), "keyring.json"))
	tmpDir := t.TempDir()
	defaultDBPath := filepath.Join(tmpDir, "import_manager.db")
	dbKey := "test-db-key-perms"

	mgr, err := connection.OpenEncrypted(defaultDBPath, dbKey)
	if err != nil {
		t.Fatalf("open default db: %v", err)
	}
	defer mgr.Close()

	if _, err := migrations.Run(context.Background(), mgr.DB(), "1.8.2", nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	aud := audit.NewService(mgr.DB())
	sec := security.NewService(mgr.DB(), false, aud)
	if _, err := sec.CompleteAdminSetup("admin", "Admin!Pass#2026"); err != nil {
		t.Fatalf("CompleteAdminSetup: %v", err)
	}

	svc, err := company.NewService(tmpDir, defaultDBPath, dbKey, "1.8.2", mgr.DB(), sec, aud, func(newPath, _ string) (*sql.DB, error) {
		return mgr.DB(), nil
	})
	if err != nil {
		t.Fatalf("init company service: %v", err)
	}

	// Unauthenticated must be rejected on all mutating/switching methods
	_ = sec.Logout()
	if _, err := svc.CreateCompany(models.CreateCompanyInput{Name: "Blocked Co", ShortCode: "BLCK"}); err == nil {
		t.Fatalf("expected unauthenticated CreateCompany to fail")
	}
	if _, err := svc.UpdateCompany(models.UpdateCompanyInput{ID: "comp_default", Name: "Blocked"}); err == nil {
		t.Fatalf("expected unauthenticated UpdateCompany to fail")
	}
	if _, err := svc.SwitchCompany("comp_default"); err == nil {
		t.Fatalf("expected unauthenticated SwitchCompany to fail")
	}
	if err := svc.DeleteCompany("comp_default"); err == nil {
		t.Fatalf("expected unauthenticated DeleteCompany to fail")
	}

	// Operator (PermDataRead + PermDataWrite, no PermAdminSystemTools) cannot create/update/delete companies
	if _, err := sec.Authenticate("admin", "Admin!Pass#2026", false); err != nil {
		t.Fatalf("Authenticate admin: %v", err)
	}
	if err := sec.AssignUserRole("admin", "operator"); err != nil {
		t.Fatalf("AssignUserRole: %v", err)
	}
	_ = sec.Logout()
	if _, err := sec.Authenticate("admin", "Admin!Pass#2026", false); err != nil {
		t.Fatalf("Authenticate operator: %v", err)
	}
	if _, err := svc.CreateCompany(models.CreateCompanyInput{Name: "Op Co", ShortCode: "OPCO"}); err == nil {
		t.Fatalf("expected operator CreateCompany to fail")
	}
	if _, err := svc.UpdateCompany(models.UpdateCompanyInput{ID: "comp_default", Name: "Op Update"}); err == nil {
		t.Fatalf("expected operator UpdateCompany to fail")
	}
	if err := svc.DeleteCompany("comp_default"); err == nil {
		t.Fatalf("expected operator DeleteCompany to fail")
	}
	// Operator with PermDataRead can switch to an existing company
	if _, err := svc.SwitchCompany("comp_default"); err != nil {
		t.Fatalf("expected operator SwitchCompany to succeed, got %v", err)
	}
}
