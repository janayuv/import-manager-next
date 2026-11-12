import { expect, test } from '@playwright/test';

const ADMIN_PERMS = [
  'data.read',
  'data.write',
  'data.delete',
  'backup.create',
  'backup.restore',
  'recycle_bin.view',
  'recycle_bin.restore',
  'admin.system_health',
  'admin.system_tools',
  'admin.activity_log',
  'admin.user_activity',
  'automation.view',
  'automation.mutate',
  'automation.ops_center',
  'role.read',
  'security.session_read',
];

test.describe('Import Manager Desktop WebView UI E2E', () => {
  test('authenticates session and navigates Dashboard, Suppliers, Shipments, Operations Center', async ({
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
        // SecurityService.GetAuthSetupStatus
        case 1720446079:
          return json({
            needsSetup: false,
            adminUsername: 'admin',
            recoveryMode: false,
          });
        // SecurityService.GetMySession
        case 1530962504:
          return json({
            sessionId: 'e2e-sess-1',
            userId: 'admin',
            username: 'admin',
            role: 'administrator',
            permissions: ADMIN_PERMS,
            expiresAt: '2099-01-01T00:00:00Z',
          });
        // DashboardService.GetOverview
        case 3183432689:
          return json({
            financialYear: 'ALL',
            startDate: '2026-04-01',
            endDate: '2027-03-31',
            availableFys: ['2026-27'],
            kpis: {
              totalSuppliers: 11,
              activeSuppliers: 11,
              totalShipments: 4,
              pendingShipments: 2,
              completedShipments: 2,
              overdueEtaCount: 1,
              shipmentsMissingBoe: 0,
              shipmentsMissingExpense: 0,
              totalInvoices: 4,
              pendingBoes: 1,
              reconciledBoes: 2,
              totalImportValueInr: 250000,
              totalCustomsDutyInr: 62000,
              totalBasicExpensesInr: 38000,
              totalGrossExpensesInr: 45000,
              openExceptionCount: 1,
              slaBreachCount: 0,
              openIncidentCount: 0,
              workflowHealthScore: 95,
            },
            monthlyTrends: [
              { month: '2026-10', shipments: 2, importValueInr: 120000, dutyInr: 30000, expensesInr: 20000 },
              { month: '2026-11', shipments: 2, importValueInr: 130000, dutyInr: 32000, expensesInr: 25000 },
            ],
            supplierBreakdown: [{ supplierId: 'sup-e2e-1', supplierName: 'Global Components Ltd', shipmentCount: 2, importValueInr: 150000 }],
            statusBreakdown: [{ status: 'delivered', count: 2 }, { status: 'in-transit', count: 2 }],
            recentActivity: [],
            alerts: [],
            cacheHit: false,
            generatedAt: '2026-11-12T10:00:00Z',
            lastSnapshotDate: '2026-11-12',
          });
        // OptionsService.ListAllOptions
        case 949455947:
          return json({});
        // OptionsService.ListOptions
        case 1972372618:
          return json([{ value: 'DE', label: 'Germany' }, { value: 'IN', label: 'India' }]);
        // SupplierService.List
        case 1419195916:
          return json({
            items: [
              {
                id: 'sup-e2e-1',
                supplierName: 'Global Components Ltd',
                shortName: 'GCL',
                country: 'DE',
                email: 'ops@gcl.example',
                phone: '+49-1234',
                isActive: true,
              },
            ],
            totalCount: 1,
            page: 1,
            pageSize: 20,
          });
        // ShipmentService.List
        case 6331702:
          return json({
            items: [
              {
                id: 'shp-e2e-1',
                supplierId: 'sup-e2e-1',
                supplierName: 'Global Components Ltd',
                invoiceNumber: 'INV-E2E-100',
                invoiceDate: '2026-11-01',
                goodsCategory: 'Sensors',
                invoiceValue: 50000,
                invoiceCurrency: 'USD',
                incoterm: 'CIF',
                status: 'in-transit',
                isFrozen: false,
              },
            ],
            totalCount: 1,
            page: 1,
            pageSize: 20,
          });
        // WorkflowService.GetOperationsOverview
        case 1053061900:
          return json({
            workflowHealthScore: 95,
            systemStability: 99,
            openExceptions: 1,
            breachedSlaCount: 0,
            openIncidents: 0,
            activeSuppressions: 0,
            recentBursts: 0,
            restoreLocked: false,
            jobs: [],
            forecasts: [],
            incidents: [],
          });
        // WorkflowService.ListExceptions
        case 2374146129:
          return json([
            {
              id: 'ex-e2e-1',
              exceptionType: 'OVERDUE_ETA',
              entityType: 'shipment',
              entityId: 'shp-e2e-1',
              status: 'OPEN',
              priority: 'HIGH',
              assignedTo: '',
              assignedAt: '',
              createdAt: '2026-11-12',
              updatedAt: '2026-11-12',
              resolvedBy: '',
              resolvedAt: '',
              slaDeadline: '2026-11-13',
              slaStatus: 'ON_TIME',
            },
          ]);
        default:
          return json({});
      }
    });

    await page.goto('/');
    await expect(page.getByText('Import Manager')).toBeVisible();
    await expect(page.getByText('11 / 11')).toBeVisible();

    await page.getByRole('link', { name: 'Suppliers' }).click();
    await expect(page).toHaveURL(/#\/suppliers$/);
    await expect(page.getByRole('cell', { name: 'Global Components Ltd' }).first()).toBeVisible();

    await page.getByRole('link', { name: 'Shipments' }).click();
    await expect(page).toHaveURL(/#\/shipments$/);
    await expect(page.getByText('INV-E2E-100')).toBeVisible();

    await page.getByRole('link', { name: 'Operations Center' }).click();
    await expect(page).toHaveURL(/#\/admin\/operations-center$/);
    await expect(page.getByRole('cell', { name: 'OVERDUE_ETA' })).toBeVisible();
  });
});
