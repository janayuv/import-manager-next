package boe_entry_test

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/boe"
	"github.com/janayuv/import-manager-next/backend/services/boe_entry"
	"github.com/janayuv/import-manager-next/backend/services/boe_summary"
	"github.com/janayuv/import-manager-next/backend/services/invoice_wizard"
	"github.com/janayuv/import-manager-next/backend/services/item_master"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/shipment"
	"github.com/janayuv/import-manager-next/backend/services/supplier"
)

func almostEq(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestCalculateDutiesStandardCEPARodtepAndCIFLock(t *testing.T) {
	// 1. FOB calculation under Standard, CEPA, and Rodtep
	baseLine := models.BOECalculationItemInput{
		ItemID:            "ITM-1",
		PartNumber:        "PN-1",
		Quantity:          10,
		UnitPrice:         100, // Forex = 1000.00
		ActualBCDRate:     7.5,
		ActualSWSRate:     10,
		ActualIGSTRate:    18,
		CalculationMethod: "Standard",
		BOEBCDRate:        7.5,
		BOESWSRate:        10,
		BOEIGSTRate:       18,
	}

	// FOB with ExRate=80 => INR=80,000, Freight=4000, EXW=1000, InsRate=1% on (80000+1000)=810 -> AV = 85810.0
	resStd, err := boe_entry.CalculateDuties(models.BOECalcParams{
		Incoterm:      "FOB",
		ExchangeRate:  80,
		FreightCost:   4000,
		EXWCost:       1000,
		InsuranceRate: 1.0,
		Interest:      50,
		LineItems:     []models.BOECalculationItemInput{baseLine},
	})
	if err != nil {
		t.Fatalf("standard calc error: %v", err)
	}
	ln := resStd.Items[0]
	if !almostEq(ln.AssessableValue, 85810.0, 0.01) {
		t.Fatalf("expected AV 85810.0, got %.2f", ln.AssessableValue)
	}
	// BCD = 85810 * 7.5% = 6435.75 -> 6435.8; SWS = 643.575 -> 643.6
	// IGST = (85810 + 6435.75 + 643.575) * 18% = 16720.0785 -> 16720.1
	if !almostEq(ln.BCDValue, 6435.8, 0.11) || !almostEq(ln.SWSValue, 643.6, 0.11) || !almostEq(ln.IGSTValue, 16720.1, 0.11) {
		t.Fatalf("unexpected Standard duties: %+v", ln)
	}

	// 2. Rodtep: scrip boeBcdRate=0, actualBcdRate=7.5 -> SWS & IGST still computed on actualBCD!
	rodLine := baseLine
	rodLine.CalculationMethod = "Rodtep"
	rodLine.BOEBCDRate = 0
	resRod, err := boe_entry.CalculateDuties(models.BOECalcParams{
		Incoterm:      "FOB",
		ExchangeRate:  80,
		FreightCost:   4000,
		EXWCost:       1000,
		InsuranceRate: 1.0,
		LineItems:     []models.BOECalculationItemInput{rodLine},
	})
	if err != nil {
		t.Fatalf("rodtep calc error: %v", err)
	}
	rLn := resRod.Items[0]
	if rLn.BCDValue != 0 || !almostEq(rLn.SWSValue, 643.6, 0.11) || !almostEq(rLn.IGSTValue, 16720.1, 0.11) {
		t.Fatalf("unexpected Rodtep duties (expected BCD=0, SWS & IGST on actual BCD): %+v", rLn)
	}

	// 3. CEPA preferential rate: boeBcdRate = 2.5 < actualBcdRate 7.5
	cepaLine := baseLine
	cepaLine.CalculationMethod = "CEPA"
	cepaLine.BOEBCDRate = 2.5
	resCEPA, err := boe_entry.CalculateDuties(models.BOECalcParams{
		Incoterm: "CIF", ExchangeRate: 80, FreightCost: 5000, EXWCost: 2000, InsuranceRate: 2.0,
		LineItems: []models.BOECalculationItemInput{cepaLine},
	})
	if err != nil {
		t.Fatalf("cepa CIF calc error: %v", err)
	}
	if !resCEPA.CIFZeroLocked || resCEPA.TotalFreight != 0 || resCEPA.TotalEXW != 0 || resCEPA.TotalInsurance != 0 {
		t.Fatalf("expected CIF zero lock enforced, got %+v", resCEPA)
	}
	if resCEPA.HasBlockingBCDOverride {
		t.Fatalf("expected no blocking override when boeBcdRate <= actualBcdRate")
	}
}

func TestBOEEntrySaveBlockersOverridesAndTransitions(t *testing.T) {
	tmp := t.TempDir()
	mgr, err := connection.OpenEncrypted(filepath.Join(tmp, "boe_entry.db"), "test-key-32-chars-minimum-123456")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = mgr.Close() }()
	db := mgr.DB()

	if _, err := migrations.Run(context.Background(), db, "1.0.0", nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	auditSvc := audit.NewService(db)
	secSvc := security.NewService(db, false, auditSvc)
	if _, err := secSvc.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("setup admin: %v", err)
	}

	supSvc := supplier.NewService(db, secSvc, auditSvc)
	sup, _ := supSvc.Create(models.SupplierInput{SupplierName: "Seiko", Country: "Japan", IsActive: true})

	itemSvc := item_master.NewService(db, secSvc, auditSvc, tmp)
	it, _ := itemSvc.Create(models.ItemInput{
		PartNumber: "SK-10", ItemDescription: "Spindle", Unit: "PCS", Currency: "USD",
		UnitPrice: 100, HSNCode: "8483", SupplierID: sup.ID, IsActive: true, BCD: "7.5", SWS: "10", IGST: "18",
	})

	shipSvc := shipment.NewService(db, secSvc, auditSvc)
	sh, _ := shipSvc.Create(models.ShipmentInput{
		SupplierID: sup.ID, InvoiceNumber: "INV-BE-1", InvoiceDate: "2026-11-01",
		GoodsCategory: "Machinery", InvoiceValue: 1000, InvoiceCurrency: "USD", Incoterm: "CIF",
	})

	wiz := invoice_wizard.NewService(db, secSvc, auditSvc)
	if _, err := wiz.SaveInvoice(models.SaveInvoiceRequest{
		ShipmentID: sh.ID, Finalize: true, LineTotalDecimals: 2, InvoiceTotalDecimals: 2,
		Lines: []models.InvoiceLineInput{{ItemID: it.ID, Quantity: 10, UnitPrice: 100, DutyPercent: 7.5, SWSPercent: 10, IGSTPercent: 18}},
	}); err != nil {
		t.Fatalf("finalize invoice: %v", err)
	}

	boeSvc := boe.NewService(db, secSvc, auditSvc, tmp)
	entrySvc := boe_entry.NewService(db, secSvc, auditSvc)
	sumSvc := boe_summary.NewService(db, secSvc, auditSvc)

	ctxData, err := entrySvc.LoadShipmentContext(sh.ID)
	if err != nil {
		t.Fatalf("load shipment context: %v", err)
	}
	if len(ctxData.Params.LineItems) != 1 || ctxData.Result.CIFZeroLocked != true {
		t.Fatalf("unexpected context: %+v", ctxData)
	}

	// Override CSV skips IGST mismatch & unknown partNo while applying CEPA valid override
	ovCSV := "partNo,calculationMethod,boeBcdRate,boeSwsRate,boeIgstRate\n" +
		"SK-10,CEPA,5.0,10,18\n" +
		"SK-10,Standard,7.5,10,12\n" +
		"NOPE,Standard,7.5,10,18\n"
	ovRes, err := entrySvc.ParseOverrideCSV(ovCSV, ctxData.Params.LineItems)
	if err != nil {
		t.Fatalf("override csv: %v", err)
	}
	if ovRes.AppliedCount != 1 || ovRes.SkippedCount != 2 {
		t.Fatalf("unexpected override csv summary: %+v", ovRes)
	}

	// Save blocker 1: boeBcdRate > actualBcdRate
	badParams := ctxData.Params
	badParams.LineItems[0].BOEBCDRate = 15.0
	if _, err := entrySvc.SaveCalculation(badParams); err == nil {
		t.Fatalf("expected save blocker for boeBcdRate > actualBcdRate")
	}

	// Create BOE with mismatched dutyPaid vs valid calc total
	goodParams := ctxData.Params
	goodParams.LineItems = ovRes.Items
	preview, _ := entrySvc.PreviewCalculation(goodParams)
	badBOE, _ := boeSvc.Create(models.BOEDetailInput{
		BENumber: "BE-9001", BEDate: "2026-11-10", Location: "INMAA4", DutyPaid: preview.CustomsDutyTotal + 100,
	})
	goodParams.BOEID = badBOE.ID
	if _, err := entrySvc.SaveCalculation(goodParams); err == nil {
		t.Fatalf("expected save blocker for dutyPaid mismatch")
	}

	// Create matching BOE and save cleanly -> transitions shipment to ready-dly/customs-clearance
	goodBOE, err := boeSvc.Create(models.BOEDetailInput{
		BENumber: "BE-9002", BEDate: "2026-11-10", Location: "INMAA4",
		TotalAssessmentValue: preview.TotalAssessableValue, DutyAmount: preview.CustomsDutyTotal, DutyPaid: preview.CustomsDutyTotal,
	})
	if err != nil {
		t.Fatalf("create matching BOE: %v", err)
	}
	goodParams.BOEID = goodBOE.ID
	saved, err := entrySvc.SaveCalculation(goodParams)
	if err != nil {
		t.Fatalf("expected valid save to succeed, got: %v", err)
	}
	if saved.Status != "Reconciled" {
		t.Fatalf("expected Reconciled status, got %s", saved.Status)
	}

	shAfter, _ := shipSvc.GetByID(sh.ID)
	if shAfter.Status != "customs-clearance" && shAfter.Status != "ready-dly" {
		t.Fatalf("expected shipment status updated after BOE save, got %s", shAfter.Status)
	}

	summaries, err := sumSvc.List("", "")
	if err != nil || len(summaries) != 1 || summaries[0].BENumber != "BE-9002" {
		t.Fatalf("unexpected boe summary list: %+v err=%v", summaries, err)
	}
}
