package options_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/repositories"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/options"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func setupOptionsSvc(t *testing.T) *options.Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "options_test.db")
	mgr, err := connection.OpenEncrypted(dbPath, "test-key-32-chars-minimum-123456")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	db := mgr.DB()

	if _, err := migrations.Run(context.Background(), db, "1.0.0", nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	auditSvc := audit.NewService(db)
	secSvc := security.NewService(db, false, auditSvc)
	if _, err := secSvc.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup: %v", err)
	}
	return options.NewService(db, secSvc, auditSvc)
}

func TestOptionsDefaultsSeededAndCRUD(t *testing.T) {
	svc := setupOptionsSvc(t)

	all, err := svc.ListAllOptions()
	if err != nil {
		t.Fatalf("ListAllOptions: %v", err)
	}
	for tbl := range repositories.AllowedOptionTables {
		if len(all[tbl]) == 0 {
			t.Fatalf("expected table %s to have seeded defaults, got 0", tbl)
		}
	}

	countries, err := svc.ListOptions("countries")
	if err != nil {
		t.Fatalf("ListOptions countries: %v", err)
	}
	hasIndia := false
	for _, c := range countries {
		if c.Value == "India" && c.Label == "India" {
			hasIndia = true
		}
		if c.Value == "PCS" {
			t.Fatalf("unexpected unit option PCS in countries table: %+v", c)
		}
	}
	if !hasIndia {
		t.Fatalf("expected seeded country India in countries options")
	}

	added, err := svc.UpsertOption("countries", "Spain", "Spain")
	if err != nil {
		t.Fatalf("UpsertOption: %v", err)
	}
	if added.Value != "Spain" || added.Label != "Spain" {
		t.Fatalf("unexpected upserted option: %+v", added)
	}

	if err := svc.DeleteOption("countries", "Spain"); err != nil {
		t.Fatalf("DeleteOption: %v", err)
	}
}
