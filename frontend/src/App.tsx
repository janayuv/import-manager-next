import { HashRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Toaster } from 'sonner';
import { AppShell } from '@/components/layout/AppShell';
import { AuthProvider, useAuth } from '@/contexts/AuthContext';
import { BOEEntryPage } from '@/pages/BOEEntryPage';
import { BOERegistryPage } from '@/pages/BOERegistryPage';
import { DashboardPage } from '@/pages/DashboardPage';
import { DatabaseBackupsPage } from '@/pages/DatabaseBackupsPage';
import { ExpenseReportsPage } from '@/pages/ExpenseReportsPage';
import { ExpensesPage } from '@/pages/ExpensesPage';
import { InvoiceWizardPage } from '@/pages/InvoiceWizardPage';
import { InvoicesPage } from '@/pages/InvoicesPage';
import { ItemMasterPage } from '@/pages/ItemMasterPage';
import { LandedCostReportPage } from '@/pages/LandedCostReportPage';
import { LoginPage } from '@/pages/LoginPage';
import { RequirePermission } from '@/pages/ModuleStagePage';
import { RecycleBinPage } from '@/pages/RecycleBinPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { ShipmentsPage } from '@/pages/ShipmentsPage';
import { SuppliersPage } from '@/pages/SuppliersPage';
import { WorkspaceOverviewPage } from '@/pages/WorkspaceOverviewPage';
import { ActivityLogPage } from '@/pages/admin/ActivityLogPage';
import { AutomationCenterPage } from '@/pages/admin/AutomationCenterPage';
import { AutomationRulesPage } from '@/pages/admin/AutomationRulesPage';
import { ErrorCenterPage } from '@/pages/admin/ErrorCenterPage';
import { OperationsCenterPage } from '@/pages/admin/OperationsCenterPage';
import { RolesPermissionsPage } from '@/pages/admin/RolesPermissionsPage';
import { SecurityCenterPage } from '@/pages/admin/SecurityCenterPage';
import { SystemToolsPage } from '@/pages/admin/SystemToolsPage';
import { UserActivityPage } from '@/pages/admin/UserActivityPage';

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
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<DashboardPage />} />

          <Route path="suppliers" element={<SuppliersPage />} />
          <Route path="shipments" element={<ShipmentsPage />} />
          <Route path="invoices" element={<InvoicesPage />} />
          <Route path="invoice-wizard" element={<InvoiceWizardPage />} />
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
    </HashRouter>
  );
}

export function App() {
  return (
    <AuthProvider>
      <AuthenticatedRoutes />
      <Toaster richColors position="top-right" />
    </AuthProvider>
  );
}

export default App;
