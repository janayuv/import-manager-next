import { lazy, Suspense } from 'react';
import { HashRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Toaster } from 'sonner';
import { AppShell } from '@/components/layout/AppShell';
import { ErrorBoundary } from '@/components/layout/ErrorBoundary';
import { AuthProvider, useAuth } from '@/contexts/AuthContext';
import { CompanyProvider } from '@/contexts/CompanyContext';
import { ThemeProvider } from '@/contexts/ThemeContext';
import { DashboardPage } from '@/pages/DashboardPage';
import { LoginPage } from '@/pages/LoginPage';
import { RequirePermission } from '@/pages/ModuleStagePage';
const SuppliersPage = lazy(() => import('@/pages/SuppliersPage').then((m) => ({ default: m.SuppliersPage })));
const ShipmentsPage = lazy(() => import('@/pages/ShipmentsPage').then((m) => ({ default: m.ShipmentsPage })));
const TransitShipmentsPage = lazy(() => import('@/pages/TransitShipmentsPage').then((m) => ({ default: m.TransitShipmentsPage })));
const InvoicesPage = lazy(() => import('@/pages/InvoicesPage').then((m) => ({ default: m.InvoicesPage })));
const InvoiceWizardPage = lazy(() => import('@/pages/InvoiceWizardPage').then((m) => ({ default: m.InvoiceWizardPage })));
const PurchaseOrdersPage = lazy(() => import('@/pages/PurchaseOrdersPage').then((m) => ({ default: m.PurchaseOrdersPage })));
const PaymentsPage = lazy(() => import('@/pages/PaymentsPage').then((m) => ({ default: m.PaymentsPage })));
const AdvanceReconciliationPage = lazy(() => import('@/pages/AdvanceReconciliationPage').then((m) => ({ default: m.AdvanceReconciliationPage })));
const ItemMasterPage = lazy(() => import('@/pages/ItemMasterPage').then((m) => ({ default: m.ItemMasterPage })));
const BOERegistryPage = lazy(() => import('@/pages/BOERegistryPage').then((m) => ({ default: m.BOERegistryPage })));
const BOEEntryPage = lazy(() => import('@/pages/BOEEntryPage').then((m) => ({ default: m.BOEEntryPage })));
const ReconciledBOEPage = lazy(() => import('@/pages/ReconciledBOEPage').then((m) => ({ default: m.ReconciledBOEPage })));
const CBICRatesPage = lazy(() => import('@/pages/CBICRatesPage').then((m) => ({ default: m.CBICRatesPage })));
const RoDTEPScripsPage = lazy(() => import('@/pages/RoDTEPScripsPage').then((m) => ({ default: m.RoDTEPScripsPage })));
const ExpensesPage = lazy(() => import('@/pages/ExpensesPage').then((m) => ({ default: m.ExpensesPage })));
const ExpenseReportsPage = lazy(() => import('@/pages/ExpenseReportsPage').then((m) => ({ default: m.ExpenseReportsPage })));
const LandedCostReportPage = lazy(() => import('@/pages/LandedCostReportPage').then((m) => ({ default: m.LandedCostReportPage })));
const PartPeriodAnalysisPage = lazy(() => import('@/pages/PartPeriodAnalysisPage').then((m) => ({ default: m.PartPeriodAnalysisPage })));
const DatabaseBackupsPage = lazy(() => import('@/pages/DatabaseBackupsPage').then((m) => ({ default: m.DatabaseBackupsPage })));
const RecycleBinPage = lazy(() => import('@/pages/RecycleBinPage').then((m) => ({ default: m.RecycleBinPage })));
const SettingsPage = lazy(() => import('@/pages/SettingsPage').then((m) => ({ default: m.SettingsPage })));
const WorkspaceOverviewPage = lazy(() => import('@/pages/WorkspaceOverviewPage').then((m) => ({ default: m.WorkspaceOverviewPage })));
const ActivityLogPage = lazy(() => import('@/pages/admin/ActivityLogPage').then((m) => ({ default: m.ActivityLogPage })));
const UserActivityPage = lazy(() => import('@/pages/admin/UserActivityPage').then((m) => ({ default: m.UserActivityPage })));
const SecurityCenterPage = lazy(() => import('@/pages/admin/SecurityCenterPage').then((m) => ({ default: m.SecurityCenterPage })));
const RolesPermissionsPage = lazy(() => import('@/pages/admin/RolesPermissionsPage').then((m) => ({ default: m.RolesPermissionsPage })));
const AutomationRulesPage = lazy(() => import('@/pages/admin/AutomationRulesPage').then((m) => ({ default: m.AutomationRulesPage })));
const AutomationCenterPage = lazy(() => import('@/pages/admin/AutomationCenterPage').then((m) => ({ default: m.AutomationCenterPage })));
const OperationsCenterPage = lazy(() => import('@/pages/admin/OperationsCenterPage').then((m) => ({ default: m.OperationsCenterPage })));
const ErrorCenterPage = lazy(() => import('@/pages/admin/ErrorCenterPage').then((m) => ({ default: m.ErrorCenterPage })));
const SystemToolsPage = lazy(() => import('@/pages/admin/SystemToolsPage').then((m) => ({ default: m.SystemToolsPage })));
const SystemHealthPage = lazy(() => import('@/pages/admin/SystemHealthPage').then((m) => ({ default: m.SystemHealthPage })));

function PageFallback() {
  return (
    <div className="flex min-h-[300px] w-full items-center justify-center p-8">
      <div className="flex flex-col items-center gap-2">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
        <span className="text-xs text-muted-foreground">Loading workspace module...</span>
      </div>
    </div>
  );
}

function AuthenticatedRoutes() {
  const { loading, session } = useAuth();

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        Initializing encrypted desktop workspace...
      </div>
    );
  }

  if (!session) {
    return <LoginPage />;
  }

  return (
    <HashRouter>
      <ErrorBoundary fallbackTitle="Module Render Error">
        <Suspense fallback={<PageFallback />}>
          <Routes>
          <Route element={<AppShell />}>
            <Route index element={<DashboardPage />} />

          <Route path="suppliers" element={<SuppliersPage />} />
          <Route path="shipments" element={<ShipmentsPage />} />
          <Route
            path="transit-shipments"
            element={
              <RequirePermission permission="data.read">
                <TransitShipmentsPage />
              </RequirePermission>
            }
          />
          <Route path="invoices" element={<InvoicesPage />} />
          <Route path="invoice-wizard" element={<InvoiceWizardPage />} />
          <Route
            path="purchase-orders"
            element={
              <RequirePermission permission="data.read">
                <PurchaseOrdersPage />
              </RequirePermission>
            }
          />
          <Route
            path="payments"
            element={
              <RequirePermission permission="data.read">
                <PaymentsPage />
              </RequirePermission>
            }
          />
          <Route
            path="advance-reconciliation"
            element={
              <RequirePermission permission="data.read">
                <AdvanceReconciliationPage />
              </RequirePermission>
            }
          />
          <Route path="items" element={<ItemMasterPage />} />
          <Route
            path="boe"
            element={
              <RequirePermission permission="data.read">
                <BOERegistryPage />
              </RequirePermission>
            }
          />
          <Route
            path="boe-entry"
            element={
              <RequirePermission permission="data.write">
                <BOEEntryPage />
              </RequirePermission>
            }
          />
          <Route
            path="reconciled-boe"
            element={
              <RequirePermission permission="data.read">
                <ReconciledBOEPage />
              </RequirePermission>
            }
          />
          <Route
            path="cbic-rates"
            element={
              <RequirePermission permission="data.read">
                <CBICRatesPage />
              </RequirePermission>
            }
          />
          <Route
            path="rodtep"
            element={
              <RequirePermission permission="data.read">
                <RoDTEPScripsPage />
              </RequirePermission>
            }
          />
          <Route
            path="expenses"
            element={
              <RequirePermission permission="data.read">
                <ExpensesPage />
              </RequirePermission>
            }
          />
          <Route
            path="expense-reports"
            element={
              <RequirePermission permission="data.read">
                <ExpenseReportsPage />
              </RequirePermission>
            }
          />
          <Route
            path="reports/landed-cost"
            element={
              <RequirePermission permission="data.read">
                <LandedCostReportPage />
              </RequirePermission>
            }
          />
          <Route
            path="part-analysis"
            element={
              <RequirePermission permission="data.read">
                <PartPeriodAnalysisPage />
              </RequirePermission>
            }
          />
          <Route
            path="reports/part-analysis"
            element={
              <RequirePermission permission="data.read">
                <PartPeriodAnalysisPage />
              </RequirePermission>
            }
          />
          <Route
            path="database"
            element={
              <RequirePermission permission="backup.create">
                <DatabaseBackupsPage />
              </RequirePermission>
            }
          />
          <Route
            path="recycle-bin"
            element={
              <RequirePermission permission="recycle_bin.view">
                <RecycleBinPage />
              </RequirePermission>
            }
          />
          <Route path="settings" element={<SettingsPage />} />

          <Route
            path="admin/security-center"
            element={
              <RequirePermission permission="security.session_read">
                <SecurityCenterPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/roles-permissions"
            element={
              <RequirePermission permission="role.read">
                <RolesPermissionsPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/activity-log"
            element={
              <RequirePermission permission="admin.activity_log">
                <ActivityLogPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/user-activity"
            element={
              <RequirePermission permission="admin.user_activity">
                <UserActivityPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/operations-center"
            element={
              <RequirePermission permission="automation.ops_center">
                <OperationsCenterPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/automation-center"
            element={
              <RequirePermission permission="automation.view">
                <AutomationCenterPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/automation-rules"
            element={
              <RequirePermission permission="automation.view">
                <AutomationRulesPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/system-health"
            element={
              <RequirePermission permission="admin.system_health">
                <SystemHealthPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/workspace-overview"
            element={
              <RequirePermission permission="admin.system_health">
                <WorkspaceOverviewPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/system-tools"
            element={
              <RequirePermission permission="admin.system_tools">
                <SystemToolsPage />
              </RequirePermission>
            }
          />
          <Route
            path="admin/error-center"
            element={
              <RequirePermission permission="admin.system_health">
                <ErrorCenterPage />
              </RequirePermission>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </Suspense>
  </ErrorBoundary>
  </HashRouter>
);
}

export function App() {
  return (
    <AuthProvider>
      <ThemeProvider>
        <CompanyProvider>
          <AuthenticatedRoutes />
          <Toaster richColors position="top-right" />
        </CompanyProvider>
      </ThemeProvider>
    </AuthProvider>
  );
}

export default App;
