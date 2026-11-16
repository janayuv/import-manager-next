package shipment_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/janayuv/import-manager-next/backend/database/connection"
	"github.com/janayuv/import-manager-next/backend/database/migrations"
	"github.com/janayuv/import-manager-next/backend/models"
	"github.com/janayuv/import-manager-next/backend/services/audit"
	"github.com/janayuv/import-manager-next/backend/services/security"
	"github.com/janayuv/import-manager-next/backend/services/shipment"
	"github.com/janayuv/import-manager-next/backend/services/supplier"
)

func setupTestService(t *testing.T) (*shipment.Service, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_erp.db")
	mgr, err := connection.OpenEncrypted(dbPath, "test-key-12345")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	if _, err := migrations.Run(context.Background(), mgr.DB(), "1.0.0", nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	auditSvc := audit.NewService(mgr.DB())
	secSvc := security.NewService(mgr.DB(), false, auditSvc)
	if _, err := secSvc.CompleteAdminSetup("admin", "StrongPass1!"); err != nil {
		t.Fatalf("admin setup: %v", err)
	}

	supSvc := supplier.NewService(mgr.DB(), secSvc, auditSvc)
	sup, err := supSvc.Create(models.SupplierInput{SupplierName: "Global Maritime Supplies", Country: "Singapore", IsActive: true})
	if err != nil {
		t.Fatalf("create test supplier: %v", err)
	}

	svc := shipment.NewService(mgr.DB(), secSvc, auditSvc)
	return svc, sup.ID
}

func TestSplitShipment3x40ftDirectAndWarehouseWorkflow(t *testing.T) {
	svc, supID := setupTestService(t)

	// Step 1: Create a shipment with 3x40ft containers
	shInput := models.ShipmentInput{
		SupplierID:      supID,
		InvoiceNumber:   "INV-3X40-001",
		InvoiceDate:     "2026-11-01",
		GoodsCategory:   "Raw Materials",
		InvoiceValue:    75000.0,
		InvoiceCurrency: "USD",
		Incoterm:        "CIF",
		ShipmentMode:    "Sea",
		ShipmentType:    "FCL",
		BLAWBNumber:     "MSCU998877",
		BLAWBDate:       "2026-11-01",
		VesselName:      "EVER GIVEN",
		ContainerNumber: "CONT-A, CONT-B, CONT-C",
		GrossWeightKg:   60000.0,
		Status:          "customs-clearance",
		Containers: []models.ShipmentContainerInput{
			{
				ContainerNumber:   "MSCU-40-A",
				ContainerSizeType: "40FT",
				RouteType:         "DIRECT",
			},
			{
				ContainerNumber:   "MSCU-40-B",
				ContainerSizeType: "40FT",
				RouteType:         "WAREHOUSE",
				FreeDaysAllowed:   3,
				StorageRatePerDay: 500.0,
			},
			{
				ContainerNumber:   "MSCU-40-C",
				ContainerSizeType: "40FT",
				RouteType:         "WAREHOUSE",
				FreeDaysAllowed:   3,
				StorageRatePerDay: 500.0,
			},
		},
	}

	sh, err := svc.Create(shInput)
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	// Verify containers were created
	containers, err := svc.ListContainers(sh.ID)
	if err != nil || len(containers) != 3 {
		t.Fatalf("expected 3 containers, got %d (err=%v)", len(containers), err)
	}

	var cA, cB, cC models.ShipmentContainer
	for _, c := range containers {
		switch c.ContainerNumber {
		case "MSCU-40-A":
			cA = c
		case "MSCU-40-B":
			cB = c
		case "MSCU-40-C":
			cC = c
		}
	}

	// Step 2: Container A delivers directly to factory on 2026-11-05
	err = svc.ProcessContainerMovement(models.ContainerMovementAction{
		ShipmentID:     sh.ID,
		ContainerIDs:   []string{cA.ID},
		ActionType:     "DIRECT_DELIVER",
		DateOfDelivery: "2026-11-05",
		VehicleNumber:  "KA-01-AB-1234",
		Remarks:        "Direct delivered to plant 1",
	})
	if err != nil {
		t.Fatalf("direct deliver cA: %v", err)
	}

	// Shipment status should now be partially-delivered (1 of 3 delivered)
	shUpdated, err := svc.GetByID(sh.ID)
	if err != nil {
		t.Fatalf("get shipment: %v", err)
	}
	if shUpdated.Status != "partially-delivered" {
		t.Fatalf("expected shipment status 'partially-delivered', got %q", shUpdated.Status)
	}

	// Step 3: Containers B and C move to Central CFS Warehouse on 2026-11-06
	err = svc.ProcessContainerMovement(models.ContainerMovementAction{
		ShipmentID:    sh.ID,
		ContainerIDs:  []string{cB.ID, cC.ID},
		ActionType:    "MOVE_TO_WAREHOUSE",
		WarehouseName: "Central CFS Warehouse",
		GateInDate:    "2026-11-06",
		VehicleNumber: "KA-04-WH-5678",
		Remarks:       "Moved to buffer warehouse to avoid detention",
	})
	if err != nil {
		t.Fatalf("move cB and cC to warehouse: %v", err)
	}

	// Check warehouse dwell list
	dwellList, err := svc.GetWarehouseDwellReport("active")
	if err != nil || len(dwellList) != 2 {
		t.Fatalf("expected 2 active warehouse containers, got %d (err=%v)", len(dwellList), err)
	}
	for _, dw := range dwellList {
		if dw.WarehouseName != "Central CFS Warehouse" || dw.GateInDate != "2026-11-06" {
			t.Fatalf("unexpected dwell record: %+v", dw)
		}
	}

	// Step 4: Container B is dispatched from warehouse on 2026-11-16 (10 days in warehouse)
	err = svc.ProcessContainerMovement(models.ContainerMovementAction{
		ShipmentID:     sh.ID,
		ContainerIDs:   []string{cB.ID},
		ActionType:     "DISPATCH_FROM_WAREHOUSE",
		GateOutDate:    "2026-11-16",
		DateOfDelivery: "2026-11-16",
		VehicleNumber:  "KA-01-TR-9999",
		Remarks:        "Delivered from warehouse stage 1",
	})
	if err != nil {
		t.Fatalf("dispatch cB: %v", err)
	}

	cBUpdated, err := svc.ListContainers(sh.ID)
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	for _, c := range cBUpdated {
		if c.ContainerNumber == "MSCU-40-B" {
			if c.DeliveryStatus != "DELIVERED" {
				t.Fatalf("expected cB DELIVERED, got %s", c.DeliveryStatus)
			}
			if c.DwellDays != 10 {
				t.Fatalf("expected cB dwell days 10, got %d", c.DwellDays)
			}
			if c.BillableDays != 7 { // 10 days - 3 free days = 7 billable days
				t.Fatalf("expected cB billable days 7, got %d", c.BillableDays)
			}
			if c.StorageCost != 3500.0 { // 7 * 500 = 3500
				t.Fatalf("expected cB storage cost 3500, got %f", c.StorageCost)
			}
		}
	}

	// Shipment should still be partially-delivered (2 of 3 delivered)
	shUpdated2, err := svc.GetByID(sh.ID)
	if err != nil {
		t.Fatalf("get shipment: %v", err)
	}
	if shUpdated2.Status != "partially-delivered" {
		t.Fatalf("expected shipment status 'partially-delivered', got %q", shUpdated2.Status)
	}

	// Step 5: Container C is dispatched from warehouse on 2026-11-20 (14 days in warehouse)
	err = svc.ProcessContainerMovement(models.ContainerMovementAction{
		ShipmentID:     sh.ID,
		ContainerIDs:   []string{cC.ID},
		ActionType:     "DISPATCH_FROM_WAREHOUSE",
		GateOutDate:    "2026-11-20",
		DateOfDelivery: "2026-11-20",
		VehicleNumber:  "KA-01-TR-8888",
		Remarks:        "Final container delivered",
	})
	if err != nil {
		t.Fatalf("dispatch cC: %v", err)
	}

	// Check final container C dwell time
	cDetail, err := svc.ListContainers(sh.ID)
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	for _, c := range cDetail {
		if c.ContainerNumber == "MSCU-40-C" {
			if c.DwellDays != 14 {
				t.Fatalf("expected cC dwell days 14, got %d", c.DwellDays)
			}
			if c.BillableDays != 11 { // 14 - 3 = 11
				t.Fatalf("expected cC billable days 11, got %d", c.BillableDays)
			}
			if c.StorageCost != 5500.0 { // 11 * 500 = 5500
				t.Fatalf("expected cC storage cost 5500, got %f", c.StorageCost)
			}
		}
	}

	// Step 6: All 3 containers delivered -> Parent shipment status rolls up to 'delivered'
	shFinal, err := svc.GetByID(sh.ID)
	if err != nil {
		t.Fatalf("get shipment: %v", err)
	}
	if shFinal.Status != "delivered" {
		t.Fatalf("expected shipment final status 'delivered', got %q", shFinal.Status)
	}
	if shFinal.DateOfDelivery != "2026-11-20" {
		t.Fatalf("expected shipment final dateOfDelivery '2026-11-20', got %q", shFinal.DateOfDelivery)
	}

	// Step 7: Check shipment detail inspection returns all 3 containers
	shDetail, err := svc.GetDetail(sh.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if len(shDetail.Containers) != 3 {
		t.Fatalf("expected detail to have 3 containers, got %d", len(shDetail.Containers))
	}
}

func TestSplitShipment2Direct1Warehouse(t *testing.T) {
	svc, supID := setupTestService(t)

	shInput := models.ShipmentInput{
		SupplierID:      supID,
		InvoiceNumber:   "INV-2DIR-1WH-001",
		InvoiceDate:     "2026-11-01",
		GoodsCategory:   "Finished Goods",
		InvoiceValue:    50000.0,
		InvoiceCurrency: "EUR",
		Incoterm:        "FOB",
		ShipmentMode:    "Sea",
		ShipmentType:    "FCL",
		BLAWBNumber:     "HAPAG-12345",
		Containers: []models.ShipmentContainerInput{
			{ContainerNumber: "HL-40-1", ContainerSizeType: "40FT", RouteType: "DIRECT"},
			{ContainerNumber: "HL-40-2", ContainerSizeType: "40FT", RouteType: "DIRECT"},
			{ContainerNumber: "HL-40-3", ContainerSizeType: "40FT", RouteType: "WAREHOUSE", FreeDaysAllowed: 5, StorageRatePerDay: 400.0},
		},
	}

	sh, err := svc.Create(shInput)
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	containers, err := svc.ListContainers(sh.ID)
	if err != nil || len(containers) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(containers))
	}

	var c1, c2, c3 models.ShipmentContainer
	for _, c := range containers {
		switch c.ContainerNumber {
		case "HL-40-1":
			c1 = c
		case "HL-40-2":
			c2 = c
		case "HL-40-3":
			c3 = c
		}
	}

	// Deliver 2 containers directly on 2026-11-03
	err = svc.ProcessContainerMovement(models.ContainerMovementAction{
		ShipmentID:     sh.ID,
		ContainerIDs:   []string{c1.ID, c2.ID},
		ActionType:     "DIRECT_DELIVER",
		DateOfDelivery: "2026-11-03",
		VehicleNumber:  "TN-09-TR-1111",
		Remarks:        "2 containers delivered directly to factory",
	})
	if err != nil {
		t.Fatalf("direct deliver 2 containers: %v", err)
	}

	shUpdated, _ := svc.GetByID(sh.ID)
	if shUpdated.Status != "partially-delivered" {
		t.Fatalf("expected partially-delivered, got %s", shUpdated.Status)
	}

	// Move container 3 to warehouse on 2026-11-04
	err = svc.ProcessContainerMovement(models.ContainerMovementAction{
		ShipmentID:    sh.ID,
		ContainerIDs:  []string{c3.ID},
		ActionType:    "MOVE_TO_WAREHOUSE",
		WarehouseName: "Regional 3PL Warehouse",
		GateInDate:    "2026-11-04",
		Remarks:       "Moved 1 container to 3PL warehouse",
	})
	if err != nil {
		t.Fatalf("move to warehouse: %v", err)
	}

	// Dispatch container 3 from warehouse on 2026-11-12 (8 days in warehouse)
	err = svc.ProcessContainerMovement(models.ContainerMovementAction{
		ShipmentID:     sh.ID,
		ContainerIDs:   []string{c3.ID},
		ActionType:     "DISPATCH_FROM_WAREHOUSE",
		GateOutDate:    "2026-11-12",
		DateOfDelivery: "2026-11-12",
		Remarks:        "Dispatched from 3PL warehouse to factory",
	})
	if err != nil {
		t.Fatalf("dispatch from warehouse: %v", err)
	}

	c3List, _ := svc.ListContainers(sh.ID)
	for _, c := range c3List {
		if c.ContainerNumber == "HL-40-3" {
			if c.DwellDays != 8 {
				t.Fatalf("expected dwell days 8, got %d", c.DwellDays)
			}
			if c.BillableDays != 3 { // 8 - 5 free days = 3
				t.Fatalf("expected billable days 3, got %d", c.BillableDays)
			}
			if c.StorageCost != 1200.0 { // 3 * 400 = 1200
				t.Fatalf("expected storage cost 1200, got %f", c.StorageCost)
			}
		}
	}

	// Shipment now fully delivered
	shFinal, _ := svc.GetByID(sh.ID)
	if shFinal.Status != "delivered" {
		t.Fatalf("expected delivered, got %s", shFinal.Status)
	}
	if shFinal.DateOfDelivery != "2026-11-12" {
		t.Fatalf("expected dateOfDelivery 2026-11-12, got %s", shFinal.DateOfDelivery)
	}
}
