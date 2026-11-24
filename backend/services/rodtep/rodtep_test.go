package rodtep_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/rodtep"
	"github.com/janayuv/import-manager-next/backend/services/security"
)

func setupTestService(t *testing.T) (*rodtep.Service, *connection.Manager, *security.Service) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_rodtep.db")
	mgr, err := connection.OpenEncrypted(dbPath, "test-rodtep-key-12345")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	_, err = migrations.Run(context.Background(), mgr.DB(), "1.0.0", nil)
	if err != nil {
		mgr.Close()
		t.Fatalf("run migrations: %v", err)
	}

	auditSvc := audit.NewService(mgr.DB())
	secSvc := security.NewService(mgr.DB(), false, auditSvc)
	if _, err := secSvc.CompleteAdminSetup("admin", "AdminPass123!"); err != nil {
		mgr.Close()
		t.Fatalf("admin setup: %v", err)
	}

	svc := rodtep.NewService(mgr.DB(), secSvc, auditSvc)
	return svc, mgr, secSvc
}

func insertTestBOE(t *testing.T, mgr *connection.Manager, boeID, beNumber, beDate, portCode string, dutyAmount, dutyPaid float64, grossBCD float64) {
	t.Helper()
	shipmentID := "SHP-" + boeID
	_, err := mgr.DB().Exec(`
		INSERT OR IGNORE INTO shipments (id, supplier_id, invoice_number, invoice_date, goods_category, invoice_value, invoice_currency, incoterm, status, is_frozen)
		VALUES (?, 'Sup-001', 'INV-001', ?, 'Machinery', 50000.0, 'EUR', 'CIF', 'customs-clearance', 0)`,
		shipmentID, beDate)
	if err != nil {
		t.Fatalf("insert test shipment: %v", err)
	}

	_, err = mgr.DB().Exec(`
		INSERT INTO boe_details (id, be_number, be_date, location, total_assessment_value, duty_amount, duty_paid)
		VALUES (?, ?, ?, ?, 1000000.0, ?, ?)`,
		boeID, beNumber, beDate, portCode, dutyAmount, dutyPaid,
	)
	if err != nil {
		t.Fatalf("insert test boe_details: %v", err)
	}

	calcJSON := `{"grossBcdTotal": ` + formatFloat(grossBCD) + `, "cashCustomsDutyTotal": ` + formatFloat(dutyAmount) + `}`
	_, err = mgr.DB().Exec(`
		INSERT INTO boe_calculations (id, shipment_id, boe_id, supplier_name, invoice_number, status, form_values_json, item_inputs_json, calculation_result_json)
		VALUES (?, ?, ?, 'Clean and Science Co.Ltd Korea', 'INV-001', 'Reconciled', '{}', '[]', ?)`,
		"CALC-"+boeID, shipmentID, boeID, calcJSON,
	)
	if err != nil {
		t.Fatalf("insert test boe_calculations: %v", err)
	}
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

func TestScripCreationAndPortClassification(t *testing.T) {
	svc, mgr, _ := setupTestService(t)
	defer mgr.Close()

	// 1. Create Sea Port Scrip (INMAA1)
	s1, err := svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2621004567",
		ScripPortCode:       "INMAA1",
		AcquisitionType:     models.AcquisitionDirectGeneration,
		IssueDate:           "2026-05-10",
		OriginalValuePaise:  5000000, // ₹50,000.00
		OpeningBalancePaise: 5000000,
		Remarks:             "Chennai Sea Port RoDTEP Scrip",
	})
	if err != nil {
		t.Fatalf("create scrip 1: %v", err)
	}
	if s1.ExpiryDate != "2028-05-10" {
		t.Errorf("expected 24 months statutory expiry 2028-05-10, got %s", s1.ExpiryDate)
	}
	if s1.ConfirmedBalINR != 50000.0 {
		t.Errorf("expected confirmed INR 50000, got %f", s1.ConfirmedBalINR)
	}

	// 2. Create Air Cargo Scrip (INMAA4)
	s2, err := svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2624009876",
		ScripPortCode:       "INMAA4",
		AcquisitionType:     models.AcquisitionOpeningBalance,
		IssueDate:           "2026-06-15",
		OriginalValuePaise:  10000000, // ₹1,00,000.00
		OpeningBalancePaise: 7500000,  // ₹75,000.00
		PreERPUtilizedPaise: 2500000,
		Remarks:             "Chennai Air Cargo RoDTEP Scrip",
	})
	if err != nil {
		t.Fatalf("create scrip 2: %v", err)
	}
	if s2.ConfirmedBalancePaise != 7500000 {
		t.Errorf("expected confirmed balance 7500000 paise, got %d", s2.ConfirmedBalancePaise)
	}

	// 3. Test Invalid Port Code Rejection
	_, err = svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2620001111",
		ScripPortCode:       "INBOM4",
		IssueDate:           "2026-07-01",
		OriginalValuePaise:  1000000,
		OpeningBalancePaise: 1000000,
	})
	if err == nil {
		t.Fatal("expected error on invalid scrip port code INBOM4, got nil")
	}

	// 4. Verify Port Summary Aggregations
	summary, err := svc.GetPortSummary()
	if err != nil {
		t.Fatalf("get port summary: %v", err)
	}
	if len(summary) != 3 { // INMAA1, INMAA4, COMBINED
		t.Fatalf("expected 3 port summaries, got %d", len(summary))
	}

	var foundCombined, foundINMAA1, foundINMAA4 bool
	for _, s := range summary {
		if s.PortCode == "COMBINED" {
			foundCombined = true
			if s.TotalScripsCount != 2 {
				t.Errorf("expected combined scrips count 2, got %d", s.TotalScripsCount)
			}
			if s.OriginalValuePaise != 15000000 {
				t.Errorf("expected combined original value 15000000 paise, got %d", s.OriginalValuePaise)
			}
			if s.ConfirmedBalancePaise != 12500000 {
				t.Errorf("expected combined confirmed balance 12500000 paise, got %d", s.ConfirmedBalancePaise)
			}
		} else if s.PortCode == "INMAA1" {
			foundINMAA1 = true
			if s.ConfirmedBalancePaise != 5000000 {
				t.Errorf("expected INMAA1 balance 5000000, got %d", s.ConfirmedBalancePaise)
			}
		} else if s.PortCode == "INMAA4" {
			foundINMAA4 = true
			if s.ConfirmedBalancePaise != 7500000 {
				t.Errorf("expected INMAA4 balance 7500000, got %d", s.ConfirmedBalancePaise)
			}
		}
	}
	if !foundCombined || !foundINMAA1 || !foundINMAA4 {
		t.Errorf("missing expected port summary groups: combined=%v inmaa1=%v inmaa4=%v", foundCombined, foundINMAA1, foundINMAA4)
	}
}

func TestOpeningBalanceVerification(t *testing.T) {
	svc, mgr, _ := setupTestService(t)
	defer mgr.Close()

	scrip, err := svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2621008888",
		ScripPortCode:       "INMAA1",
		AcquisitionType:     models.AcquisitionOpeningBalance,
		IssueDate:           "2026-01-10",
		OriginalValuePaise:  20000000,
		OpeningBalancePaise: 12000000,
		Remarks:             "Pending opening verification",
	})
	if err != nil {
		t.Fatalf("create scrip: %v", err)
	}

	if scrip.OpeningVerifiedAt != "" {
		t.Errorf("expected unverified opening, got %s", scrip.OpeningVerifiedAt)
	}

	evidenceRef := "ICEGATE_ECL_STMT_2026_10_26.PDF"
	if err := svc.VerifyOpeningBalance(scrip.ID, evidenceRef); err != nil {
		t.Fatalf("verify opening balance: %v", err)
	}

	updated, err := svc.GetScrip(scrip.ID)
	if err != nil {
		t.Fatalf("get scrip: %v", err)
	}
	if updated.OpeningEvidenceRef != evidenceRef {
		t.Errorf("expected evidence ref %s, got %s", evidenceRef, updated.OpeningEvidenceRef)
	}
	if updated.OpeningVerifiedAt == "" {
		t.Error("expected OpeningVerifiedAt timestamp to be populated")
	}

	logs, err := svc.ListAuditLogs(scrip.ID)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", len(logs))
	}
}

func TestTwoStageAllocationAndDebitConfirmation(t *testing.T) {
	svc, mgr, _ := setupTestService(t)
	defer mgr.Close()

	// 1. Create Scrip with ₹50,000 balance
	scrip, err := svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2621009999",
		ScripPortCode:       "INMAA1",
		IssueDate:           "2026-08-01",
		OriginalValuePaise:  5000000,
		OpeningBalancePaise: 5000000,
	})
	if err != nil {
		t.Fatalf("create scrip: %v", err)
	}

	// 2. Insert BOE with Assessed BCD = ₹40,000.00 (duty_paid = 0)
	boeID := "BOE-TEST-001"
	insertTestBOE(t, mgr, boeID, "8822001", "2026-10-15", "INMAA1", 40000.0, 0.0, 40000.0)

	// 3. Check BOE Eval Context
	evalCtx, err := svc.GetBOEEvalContext(boeID)
	if err != nil {
		t.Fatalf("get boe eval context: %v", err)
	}
	if evalCtx.EligibleUnpaidBCDPaise != 4000000 {
		t.Fatalf("expected eligible unpaid BCD 4000000 paise, got %d", evalCtx.EligibleUnpaidBCDPaise)
	}

	// 4. Stage 1: Manual Allocation (Intent & Reservation) of ₹30,000.00
	allocs, err := svc.AllocateScrips(boeID, []models.RoDTEPAllocationItemInput{
		{
			ScripID:     scrip.ID,
			AmountPaise: 3000000, // ₹30,000
			Remarks:     "Initial allocation against BE 8822001",
		},
	}, "Manual BOE allocation")
	if err != nil {
		t.Fatalf("allocate scrip: %v", err)
	}
	if len(allocs) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(allocs))
	}
	allocID := allocs[0].ID

	// Verify Scrip balance: confirmed balance unchanged, pending reservation updated
	scripAfterAlloc, err := svc.GetScrip(scrip.ID)
	if err != nil {
		t.Fatalf("get scrip after alloc: %v", err)
	}
	if scripAfterAlloc.ConfirmedBalancePaise != 5000000 {
		t.Errorf("Stage 1 must NOT reduce confirmed balance: expected 5000000, got %d", scripAfterAlloc.ConfirmedBalancePaise)
	}
	if scripAfterAlloc.PendingAllocatedPaise != 3000000 {
		t.Errorf("Stage 1 must reserve pending balance: expected 3000000, got %d", scripAfterAlloc.PendingAllocatedPaise)
	}
	if scripAfterAlloc.NetAvailablePaise != 2000000 {
		t.Errorf("expected net available 2000000, got %d", scripAfterAlloc.NetAvailablePaise)
	}

	// Check updated BOE Eval context: Eligible unpaid BCD should now be ₹10,000
	evalCtx2, err := svc.GetBOEEvalContext(boeID)
	if err != nil {
		t.Fatalf("get boe eval context 2: %v", err)
	}
	if evalCtx2.EligibleUnpaidBCDPaise != 1000000 {
		t.Errorf("expected remaining unpaid BCD 1000000 paise, got %d", evalCtx2.EligibleUnpaidBCDPaise)
	}

	// 5. Stage 2: Partial Confirmation 1: ₹20,000 confirmed via ICEGATE
	ledger1, err := svc.ConfirmActualDebit(models.RoDTEPAllocationConfirmInput{
		AllocationID:          allocID,
		AmountPaise:           2000000,
		ICEGATETransactionRef: "ICE-TX-998811",
		ChallanNumber:         "CH-2026-10-01",
		DebitDate:             "2026-10-20",
		Remarks:               "First partial debit confirmed on customs portal",
	})
	if err != nil {
		t.Fatalf("confirm debit 1: %v", err)
	}
	if ledger1.AmountPaise != 2000000 {
		t.Errorf("expected confirmed ledger amount 2000000, got %d", ledger1.AmountPaise)
	}

	// Verify scrip state after partial confirmation:
	// Confirmed balance should be 50,000 - 20,000 = 30,000
	// Pending allocation should be 30,000 - 20,000 = 10,000
	scripAfterConf1, err := svc.GetScrip(scrip.ID)
	if err != nil {
		t.Fatalf("get scrip after conf 1: %v", err)
	}
	if scripAfterConf1.ConfirmedBalancePaise != 3000000 {
		t.Errorf("expected confirmed balance 3000000 paise, got %d", scripAfterConf1.ConfirmedBalancePaise)
	}
	if scripAfterConf1.PendingAllocatedPaise != 1000000 {
		t.Errorf("expected pending allocated 1000000 paise, got %d", scripAfterConf1.PendingAllocatedPaise)
	}

	// 6. Stage 2: Partial Confirmation 2: Remaining ₹10,000 confirmed
	ledger2, err := svc.ConfirmActualDebit(models.RoDTEPAllocationConfirmInput{
		AllocationID:          allocID,
		AmountPaise:           1000000,
		ICEGATETransactionRef: "ICE-TX-998812",
		ChallanNumber:         "CH-2026-10-02",
		DebitDate:             "2026-10-21",
		Remarks:               "Second partial debit confirmed",
	})
	if err != nil {
		t.Fatalf("confirm debit 2: %v", err)
	}
	if ledger2.RemainingConfirmedBalancePaise != 2000000 {
		t.Errorf("expected remaining balance 2000000, got %d", ledger2.RemainingConfirmedBalancePaise)
	}

	// Scrip state after full confirmation of allocation
	scripAfterConf2, err := svc.GetScrip(scrip.ID)
	if err != nil {
		t.Fatalf("get scrip after conf 2: %v", err)
	}
	if scripAfterConf2.ConfirmedBalancePaise != 2000000 {
		t.Errorf("expected confirmed balance 2000000, got %d", scripAfterConf2.ConfirmedBalancePaise)
	}
	if scripAfterConf2.PendingAllocatedPaise != 0 {
		t.Errorf("expected pending allocated 0, got %d", scripAfterConf2.PendingAllocatedPaise)
	}
	if scripAfterConf2.ConfirmedUtilizedPaise != 3000000 {
		t.Errorf("expected confirmed utilized 3000000, got %d", scripAfterConf2.ConfirmedUtilizedPaise)
	}

	// 7. Verify Net Prior Confirmed Debits for BOE
	report, err := svc.GetBOEDebitReport()
	if err != nil {
		t.Fatalf("get boe debit report: %v", err)
	}
	if len(report) != 1 {
		t.Fatalf("expected 1 boe in debit report, got %d", len(report))
	}
	if report[0].NetConfirmedPaise != 3000000 {
		t.Errorf("expected net confirmed 3000000 paise, got %d", report[0].NetConfirmedPaise)
	}
}

func TestAllocationExceedingUnpaidBCDFails(t *testing.T) {
	svc, mgr, _ := setupTestService(t)
	defer mgr.Close()

	scrip, _ := svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2621001122",
		ScripPortCode:       "INMAA1",
		IssueDate:           "2026-08-01",
		OriginalValuePaise:  10000000, // ₹1,00,000
		OpeningBalancePaise: 10000000,
	})

	boeID := "BOE-TEST-002"
	insertTestBOE(t, mgr, boeID, "8822002", "2026-10-16", "INMAA1", 25000.0, 0.0, 25000.0) // BCD = ₹25,000

	// Attempt to allocate ₹30,000 against ₹25,000 BCD liability
	_, err := svc.AllocateScrips(boeID, []models.RoDTEPAllocationItemInput{
		{
			ScripID:     scrip.ID,
			AmountPaise: 3000000,
		},
	}, "Over-allocation test")

	if err == nil {
		t.Fatal("expected error when allocation exceeds eligible unpaid BCD liability, got nil")
	}
}

func TestCloseAllocationReleasesReservation(t *testing.T) {
	svc, mgr, _ := setupTestService(t)
	defer mgr.Close()

	scrip, _ := svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2621003344",
		ScripPortCode:       "INMAA1",
		IssueDate:           "2026-08-01",
		OriginalValuePaise:  5000000, // ₹50,000
		OpeningBalancePaise: 5000000,
	})

	boeID := "BOE-TEST-003"
	insertTestBOE(t, mgr, boeID, "8822003", "2026-10-17", "INMAA1", 40000.0, 0.0, 40000.0)

	// Allocate ₹30,000
	allocs, err := svc.AllocateScrips(boeID, []models.RoDTEPAllocationItemInput{
		{
			ScripID:     scrip.ID,
			AmountPaise: 3000000,
		},
	}, "Closeout test")
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	allocID := allocs[0].ID

	// Confirm ₹10,000 only
	_, err = svc.ConfirmActualDebit(models.RoDTEPAllocationConfirmInput{
		AllocationID:          allocID,
		AmountPaise:           1000000,
		ICEGATETransactionRef: "ICE-PART-01",
		DebitDate:             "2026-10-22",
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// Close allocation releasing remaining ₹20,000
	if err := svc.CloseAllocation(allocID, "Remaining customs duty paid in cash"); err != nil {
		t.Fatalf("close allocation: %v", err)
	}

	// Verify scrip pending allocation has been completely freed
	scripAfterClose, err := svc.GetScrip(scrip.ID)
	if err != nil {
		t.Fatalf("get scrip: %v", err)
	}
	if scripAfterClose.PendingAllocatedPaise != 0 {
		t.Errorf("expected pending allocation 0 after closeout, got %d", scripAfterClose.PendingAllocatedPaise)
	}
	if scripAfterClose.ConfirmedBalancePaise != 4000000 { // 50,000 - 10,000 confirmed
		t.Errorf("expected confirmed balance 4000000, got %d", scripAfterClose.ConfirmedBalancePaise)
	}
	if scripAfterClose.NetAvailablePaise != 4000000 {
		t.Errorf("expected net available 4000000, got %d", scripAfterClose.NetAvailablePaise)
	}
}

func TestDebitReversalAndCumulativeLimit(t *testing.T) {
	svc, mgr, _ := setupTestService(t)
	defer mgr.Close()

	scrip, _ := svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2621005566",
		ScripPortCode:       "INMAA1",
		IssueDate:           "2026-08-01",
		OriginalValuePaise:  5000000,
		OpeningBalancePaise: 5000000,
	})

	boeID := "BOE-TEST-004"
	insertTestBOE(t, mgr, boeID, "8822004", "2026-10-18", "INMAA1", 40000.0, 0.0, 40000.0)

	allocs, _ := svc.AllocateScrips(boeID, []models.RoDTEPAllocationItemInput{
		{ScripID: scrip.ID, AmountPaise: 4000000},
	}, "Reversal test")

	// Confirm debit of ₹40,000 (4,000,000 paise)
	ledger, err := svc.ConfirmActualDebit(models.RoDTEPAllocationConfirmInput{
		AllocationID:          allocs[0].ID,
		AmountPaise:           4000000,
		ICEGATETransactionRef: "ICE-ORIG-01",
		DebitDate:             "2026-10-23",
	})
	if err != nil {
		t.Fatalf("confirm debit: %v", err)
	}

	// 1. First partial reversal of ₹15,000
	rev1, err := svc.ReverseDebit(models.RoDTEPReversalInput{
		DebitLedgerID: ledger.ID,
		AmountPaise:   1500000,
		Reason:        "Section 149 customs reassessment reduction",
	})
	if err != nil {
		t.Fatalf("reverse 1: %v", err)
	}
	if rev1.RemainingConfirmedBalancePaise != 2500000 { // 10,000 + 15,000 = 25,000
		t.Errorf("expected remaining balance 2500000, got %d", rev1.RemainingConfirmedBalancePaise)
	}

	// 2. Second partial reversal of ₹25,000 (cumulative now equals original ₹40,000)
	rev2, err := svc.ReverseDebit(models.RoDTEPReversalInput{
		DebitLedgerID: ledger.ID,
		AmountPaise:   2500000,
		Reason:        "Full duty reassessment exemption",
	})
	if err != nil {
		t.Fatalf("reverse 2: %v", err)
	}
	if rev2.RemainingConfirmedBalancePaise != 5000000 { // Back to initial 50,000
		t.Errorf("expected remaining balance 5000000, got %d", rev2.RemainingConfirmedBalancePaise)
	}

	// 3. Attempt third reversal of even 1 paisa: must be rejected!
	_, err = svc.ReverseDebit(models.RoDTEPReversalInput{
		DebitLedgerID: ledger.ID,
		AmountPaise:   1,
		Reason:        "Excess reversal attempt",
	})
	if err == nil {
		t.Fatal("expected failure on cumulative reversal exceeding original debit, got nil")
	}

	// Verify Net Prior Confirmed Debits for this BOE is now 0
	evalCtx, err := svc.GetBOEEvalContext(boeID)
	if err != nil {
		t.Fatalf("get eval ctx: %v", err)
	}
	if evalCtx.NetConfirmedRoDTEP != 0 {
		t.Errorf("expected net confirmed RoDTEP 0 after complete reversal, got %d", evalCtx.NetConfirmedRoDTEP)
	}
}

func TestMonthlyStatementReconciliation(t *testing.T) {
	svc, mgr, _ := setupTestService(t)
	defer mgr.Close()

	scrip, _ := svc.CreateScrip(models.RoDTEPScripInput{
		ScripNumber:         "2621007788",
		ScripPortCode:       "INMAA1",
		IssueDate:           "2026-08-01",
		OriginalValuePaise:  10000000,
		OpeningBalancePaise: 10000000,
	})

	// 1. Reconciliation with zero discrepancy
	rec1, err := svc.SaveReconciliation(models.RoDTEPReconciliationInput{
		ScripID:                      scrip.ID,
		ReconciliationDate:           "2026-10-31",
		StatementPeriodEnd:           "2026-10-31",
		ICEGATEStatementBalancePaise: 10000000,
		DiscrepancyReason:            "Matched perfectly with ICEGATE e-scrip ledger",
	})
	if err != nil {
		t.Fatalf("save reconciliation 1: %v", err)
	}
	if rec1.DiscrepancyPaise != 0 {
		t.Errorf("expected discrepancy 0, got %d", rec1.DiscrepancyPaise)
	}

	// 2. Reconciliation with discrepancy (e.g. ICEGATE has ₹95,000 due to unrecorded fee/debit)
	rec2, err := svc.SaveReconciliation(models.RoDTEPReconciliationInput{
		ScripID:                      scrip.ID,
		ReconciliationDate:           "2026-11-30",
		StatementPeriodEnd:           "2026-11-30",
		ICEGATEStatementBalancePaise: 9500000,
		DiscrepancyReason:            "ICEGATE shows ₹95,000; investigating unconfirmed debit",
	})
	if err != nil {
		t.Fatalf("save reconciliation 2: %v", err)
	}
	if rec2.DiscrepancyPaise != -500000 { // 95,000 - 100,000 = -5,000
		t.Errorf("expected discrepancy -500000 paise, got %d", rec2.DiscrepancyPaise)
	}

	recs, err := svc.ListReconciliations(scrip.ID)
	if err != nil {
		t.Fatalf("list reconciliations: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("expected 2 reconciliation records, got %d", len(recs))
	}
}

func TestInitialScripsIngestion(t *testing.T) {
	svc, mgr, _ := setupTestService(t)
	defer mgr.Close()

	sampleScrips := []models.RoDTEPScripInput{
		{
			ScripNumber:         "2621001001",
			ScripPortCode:       "INMAA1",
			AcquisitionType:     models.AcquisitionOpeningBalance,
			IssueDate:           "2026-06-01",
			OriginalValuePaise:  12500000,
			OpeningBalancePaise: 12500000,
			OpeningEvidenceRef:  "RodTep_Oct-26.xlsx",
		},
		{
			ScripNumber:         "2624002002",
			ScripPortCode:       "INMAA4",
			AcquisitionType:     models.AcquisitionOpeningBalance,
			IssueDate:           "2026-06-05",
			OriginalValuePaise:  8500000,
			OpeningBalancePaise: 8500000,
			OpeningEvidenceRef:  "RodTep_Oct-26.xlsx",
		},
	}

	count, err := svc.IngestInitialScrips(sampleScrips, "CustomsManager")
	if err != nil {
		t.Fatalf("ingest initial scrips: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 scrips ingested, got %d", count)
	}

	// Re-running ingestion should be idempotent and skip duplicates
	count2, err := svc.IngestInitialScrips(sampleScrips, "CustomsManager")
	if err != nil {
		t.Fatalf("re-run ingest initial scrips: %v", err)
	}
	if count2 != 0 {
		t.Errorf("expected 0 duplicates ingested, got %d", count2)
	}
}
