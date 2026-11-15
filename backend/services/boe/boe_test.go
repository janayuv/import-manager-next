package boe_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/boe"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func setupBOESvc(t *testing.T) (*boe.Service, *security.Service) {
	t.Helper()
	tmp := t.TempDir()
	mgr, err := connection.OpenEncrypted(filepath.Join(tmp, "boe_test.db"), "test-key-32-chars-minimum-123456")
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
	return boe.NewService(db, secSvc, auditSvc, filepath.Join(tmp, "boe_att")), secSvc
}

func TestBOECRUDAndSoftDeleteReImport(t *testing.T) {
	svc, _ := setupBOESvc(t)

	// 1. Create a BOE record
	input := models.BOEDetailInput{
		BENumber:             "3965278",
		BEDate:               "2026-09-23",
		Location:             "INMAA1",
		TotalAssessmentValue: 5233248,
		DutyAmount:           866103,
		PaymentDate:          "2026-09-24",
		DutyPaid:             866103,
		ChallanNumber:        "2063325822",
		RefID:                "007000BEINMAA10933031910831741",
		TransactionID:        "IG1092420260703103555124028370",
	}
	created, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create BOE failed: %v", err)
	}
	if created.BENumber != "3965278" || created.BEDate != "2026-09-23" {
		t.Fatalf("unexpected created BOE: %+v", created)
	}

	// 2. Verify it is listed in active records
	list, err := svc.List("")
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 active BOE, got %d (err=%v)", len(list), err)
	}

	// 3. Soft-delete the BOE record
	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("delete BOE failed: %v", err)
	}

	// Active list must now be empty
	listAfterDel, err := svc.List("")
	if err != nil || len(listAfterDel) != 0 {
		t.Fatalf("expected 0 active BOE after delete, got %d", len(listAfterDel))
	}

	// 4. Re-import CSV containing the soft-deleted BOE.
	// Previously, this failed with "UNIQUE constraint failed: boe_details.be_number, boe_details.be_date".
	// It must now succeed cleanly!
	csvContent := `beNumber,beDate,location,totalAssessmentValue,dutyAmount,paymentDate,dutyPaid,challanNumber,refId,transactionId
3965278,23-09-2026,INMAA1,5233248,866103,24-09-2026,866103,2063325822,007000BEINMAA10933031910831741,IG1092420260703103555124028370`

	res, err := svc.ImportCSV(csvContent)
	if err != nil {
		t.Fatalf("import CSV returned error: %v", err)
	}
	if res.ImportedRows != 1 || res.SkippedRows != 0 {
		t.Fatalf("expected 1 imported, 0 skipped, got: %+v (errors: %+v)", res, res.Errors)
	}

	// Verify the re-imported BOE is active
	listReimported, err := svc.List("")
	if err != nil || len(listReimported) != 1 {
		t.Fatalf("expected 1 active BOE after re-import, got %d", len(listReimported))
	}

	// 5. Re-importing while active should skip as duplicate
	resDup, err := svc.ImportCSV(csvContent)
	if err != nil {
		t.Fatalf("import duplicate CSV returned error: %v", err)
	}
	if resDup.ImportedRows != 0 || resDup.SkippedRows != 1 {
		t.Fatalf("expected 0 imported, 1 skipped for duplicate, got: %+v", resDup)
	}

	// 6. Delete again, then test Create with same (beNumber, beDate)
	if err := svc.Delete(listReimported[0].ID); err != nil {
		t.Fatalf("delete re-imported BOE failed: %v", err)
	}
	created2, err := svc.Create(input)
	if err != nil {
		t.Fatalf("create after soft-delete failed: %v", err)
	}
	if created2.BENumber != "3965278" {
		t.Fatalf("expected created2 BE number 3965278, got %s", created2.BENumber)
	}
}
