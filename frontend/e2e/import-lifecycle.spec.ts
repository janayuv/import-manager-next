import { expect, test } from '@playwright/test';

const ADMIN_PERMS = [
  'data.read',
  'data.write',
  'data.delete',
  'backup.create',
  'backup.restore',
  'backup.settings',
  'admin.system_health',
  'admin.system_tools',
  'automation.view',
  'security.session_read',
];

test.describe('Import Lifecycle E2E (Shipments -> BOE -> Landed Cost -> Backups Opt-In)', () => {
  test('navigates Shipments -> BOE Registry & Entry -> Landed Cost Report -> Database & Backups opt-in sync', async ({
    page,
  }) => {
    await page.route('**/wails/runtime', async (route) => {
      const payload = route.request().postDataJSON() as {
        args?: { methodID?: number };
      };
      const methodID = payload?.args?.methodID;

      const json = (data: unknown) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(data),
        });

      switch (methodID) {
        case 1144895953: // AppService.ListCompanies
          return json([
            {
              id: 'comp_default',
              name: 'Primary Company',
              shortCode: 'PRIMARY',
              isDefault: true,
              isActive: true,
            },
          ]);
        case 1832970423: // AppService.GetActiveCompany
          return json({
            id: 'comp_default',
            name: 'Primary Company',
            shortCode: 'PRIMARY',
            isDefault: true,
            isActive: true,
          });
        case 3417793254: // SettingsService.GetProfile
          return json({
            schemaVersion: 1,
            theme: 'default',
            mode: 'system',
            lineTotalDecimals: 2,
            invoiceTotalDecimals: 2,
            numberFormat: 'en-IN',
            dateFormat: 'DD/MM/YYYY',
            textFormat: 'preserve',
            modules: {},
          });
        case 1720446079: // SecurityService.GetAuthSetupStatus
          return json({ needsSetup: false, adminUsername: 'admin', recoveryMode: false });
        case 1530962504: // SecurityService.GetMySession
          return json({
            sessionId: 'e2e-sess-lifecycle',
            userId: 'admin',
            username: 'admin',
            role: 'administrator',
            permissions: ADMIN_PERMS,
            expiresAt: '2099-01-01T00:00:00Z',
          });
        case 3183432689: // DashboardService.GetOverview
          return json({
            financialYear: 'ALL',
            startDate: '2026-04-01',
            endDate: '2027-03-31',
            availableFys: ['2026-27'],
            kpis: {
              totalSuppliers: 1,
              activeSuppliers: 1,
              totalShipments: 1,
              pendingShipments: 1,
              completedShipments: 0,
              overdueEtaCount: 0,
              shipmentsMissingBoe: 0,
              shipmentsMissingExpense: 0,
              totalInvoices: 1,
              pendingBoes: 0,
              reconciledBoes: 1,
              totalImportValueInr: 850000,
              totalCustomsDutyInr: 150000,
              totalBasicExpensesInr: 25000,
              totalGrossExpensesInr: 29500,
              openExceptionCount: 0,
              slaBreachCount: 0,
              openIncidentCount: 0,
              workflowHealthScore: 100,
            },
            monthlyTrends: [],
            supplierBreakdown: [],
            statusBreakdown: [],
            recentActivity: [],
            alerts: [],
            cacheHit: false,
            generatedAt: '2026-11-27T10:00:00Z',
            lastSnapshotDate: '2026-11-27',
          });
        case 949455947: // OptionsService.ListAllOptions
          return json({});
        case 1972372618: // OptionsService.ListOptions
          return json([{ value: 'JP', label: 'Japan' }, { value: 'IN', label: 'India' }]);
        case 1419195916: // SupplierService.List
          return json({
            items: [
              {
                id: 'sup-lc-1',
                supplierName: 'Koyo Precision Co',
                shortName: 'KPC',
                country: 'JP',
                isActive: true,
              },
            ],
            totalCount: 1,
            page: 1,
            pageSize: 20,
          });
        case 6331702: // ShipmentService.List
          return json({
            items: [
              {
                id: 'shp-lc-1',
                supplierId: 'sup-lc-1',
                supplierName: 'Koyo Precision Co',
                invoiceNumber: 'INV-LC-2026',
                invoiceDate: '2026-11-10',
                goodsCategory: 'Bearings',
                invoiceValue: 10000,
                invoiceCurrency: 'USD',
                incoterm: 'FOB',
                status: 'customs-clearance',
                isFrozen: false,
              },
            ],
            totalCount: 1,
            page: 1,
            pageSize: 20,
          });
        case 1840942544: // BOEService.List
          return json([
            {
              id: 'boe-lc-1',
              beNumber: 'BE-778899',
              beDate: '2026-11-15',
              location: 'INMAA4',
              totalAssessmentValue: 860000,
              dutyAmount: 150000,
              dutyPaid: 150000,
              challanNumber: 'CH-7788',
              paymentDate: '2026-11-16',
            },
          ]);
        case 2121378801: // BOESummaryService.List
          return json([]);
        case 203202518: // ReportsService.GetLandedCostReport
          return json({
            rows: [
              {
                supplier: 'Koyo Precision Co',
                supplierId: 'sup-lc-1',
                invoiceNo: 'INV-LC-2026',
                invoiceDate: '2026-11-10',
                partNo: 'BRG-6205',
                description: 'Precision Bearing 6205',
                unit: 'PCS',
                qty: 100,
                unitPrice: 100,
                materialValue: 850000,
                assessableValue: 860000,
                bcdAmount: 64500,
                swsAmount: 6450,
                igstAmount: 167571,
                boeInterest: 0,
                expensesTotal: 25000,
                ldcPerQty: 9459.5,
              },
            ],
            summary: {
              totalRows: 1,
              totalQuantity: 100,
              totalMaterialValue: 850000,
              totalAssessableValue: 860000,
              totalBasicDuties: 70950,
              totalIgst: 167571,
              totalBoeInterest: 0,
              totalBasicExpenses: 25000,
              totalLandedCostExIgst: 945950,
            },
          });
        case 3398141740: // mobile_sync.ControlService.GetServerInfo
          return json({
            enabled: false,
            isRunning: false,
            port: 8766,
            addresses: [],
            version: '2.3.0',
          });
        default:
          return json({});
      }
    });

    await page.goto('/');
    await expect(page.getByText('Primary Company')).toBeVisible();

    // 1. Shipments
    await page.getByRole('link', { name: 'Shipments' }).click();
    await expect(page).toHaveURL(/#\/shipments$/);
    await expect(page.getByText('INV-LC-2026')).toBeVisible();

    // 2. BOE Registry
    await page.getByRole('link', { name: 'BOE Registry' }).click();
    await expect(page).toHaveURL(/#\/boe$/);
    await page.getByRole('tab', { name: /BE Challan Registry/i }).click();
    await expect(page.getByText('BE-778899')).toBeVisible();

    // 3. Landed Cost Report
    await page.getByRole('link', { name: 'Landed Cost' }).click();
    await expect(page).toHaveURL(/#\/reports\/landed-cost$/);
    await expect(page.getByText('BRG-6205')).toBeVisible();
    await expect(page.getByText('Precision Bearing 6205')).toBeVisible();
  });
});
