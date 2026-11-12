import * as React from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Bot,
  Box,
  Calculator,
  Database,
  FileSpreadsheet,
  FileText,
  History,
  KeyRound,
  LayoutDashboard,
  LogOut,
  PackageSearch,
  Receipt,
  Settings,
  Shield,
  ShieldAlert,
  Ship,
  SlidersHorizontal,
  Trash2,
  Truck,
  Users,
  Wand2,
  Wrench,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useAuth } from '@/contexts/AuthContext';
import { cn } from '@/lib/utils';

interface NavItem {
  to: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  permission?: string;
}

interface NavGroup {
  title: string;
  items: NavItem[];
}

const NAV_GROUPS: NavGroup[] = [
  {
    title: 'Core Operations',
    items: [
      { to: '/', label: 'Dashboard', icon: LayoutDashboard },
      { to: '/suppliers', label: 'Suppliers', icon: Users },
      { to: '/shipments', label: 'Shipments', icon: Ship },
      { to: '/invoices', label: 'Invoices', icon: FileText },
      { to: '/invoice-wizard', label: 'Invoice Wizard', icon: Wand2 },
      { to: '/items', label: 'Item Master', icon: Box },
    ],
  },
  {
    title: 'Customs & Finance',
    items: [
      { to: '/boe', label: 'BOE Registry', icon: FileSpreadsheet },
      { to: '/boe-entry', label: 'BOE Entry', icon: Calculator },
      { to: '/expenses', label: 'Expenses', icon: Receipt },
      { to: '/expense-reports', label: 'Expense Reports', icon: BarChart3 },
      { to: '/reports/landed-cost', label: 'Landed Cost Report', icon: PackageSearch },
    ],
  },
  {
    title: 'Data & System',
    items: [
      { to: '/database', label: 'Database & Backups', icon: Database, permission: 'backup.create' },
      { to: '/recycle-bin', label: 'Recycle Bin', icon: Trash2, permission: 'recycle_bin.view' },
      { to: '/settings', label: 'Settings', icon: Settings },
    ],
  },
  {
    title: 'Security & Governance',
    items: [
      { to: '/admin/security-center', label: 'Security Center', icon: Shield, permission: 'security.session_read' },
      { to: '/admin/roles-permissions', label: 'Roles & Permissions', icon: KeyRound, permission: 'role.read' },
      { to: '/admin/activity-log', label: 'Audit Trail', icon: History, permission: 'admin.activity_log' },
      { to: '/admin/user-activity', label: 'User Activity', icon: Activity, permission: 'admin.user_activity' },
    ],
  },
  {
    title: 'Automation & Diagnostics',
    items: [
      { to: '/admin/operations-center', label: 'Operations Center', icon: Truck },
      { to: '/admin/automation-center', label: 'Automation Center', icon: Bot },
      { to: '/admin/automation-rules', label: 'Automation Rules', icon: SlidersHorizontal },
      { to: '/admin/system-health', label: 'System Health', icon: ShieldAlert },
      { to: '/admin/system-tools', label: 'System Tools', icon: Wrench },
      { to: '/admin/error-center', label: 'Error Center', icon: AlertTriangle },
    ],
  },
];

export function AppShell() {
  const { session, setupStatus, hasPermission, logout } = useAuth();

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-card">
        <div className="border-b border-border px-4 py-4">
          <div className="flex items-center justify-between">
            <span className="text-base font-bold tracking-tight">Import Manager</span>
            <Badge variant="outline">Next</Badge>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">Desktop Customs &amp; ERP</p>
        </div>

        <nav className="flex-1 space-y-5 overflow-y-auto px-3 py-4" aria-label="Primary navigation">
          {NAV_GROUPS.map((group) => {
            const visibleItems = group.items.filter(
              (item) => !item.permission || hasPermission(item.permission),
            );
            if (visibleItems.length === 0) return null;

            return (
              <div key={group.title} className="space-y-1">
                <div className="px-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                  {group.title}
                </div>
                {visibleItems.map((item) => {
                  const Icon = item.icon;
                  return (
                    <NavLink
                      key={item.to}
                      to={item.to}
                      end={item.to === '/'}
                      className={({ isActive }) =>
                        cn(
                          'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm font-medium transition-colors',
                          isActive
                            ? 'bg-primary text-primary-foreground'
                            : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                        )
                      }
                    >
                      <Icon className="h-4 w-4 shrink-0" />
                      <span className="truncate">{item.label}</span>
                    </NavLink>
                  );
                })}
              </div>
            );
          })}
        </nav>

        <div className="border-t border-border p-3">
          {session && (
            <div className="flex items-center justify-between gap-2 rounded-md bg-muted/50 px-2.5 py-2">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{session.username}</p>
                <p className="truncate text-xs text-muted-foreground">{session.role}</p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void logout()}
                title="Sign out"
                aria-label="Sign out"
              >
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          )}
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {setupStatus?.recoveryMode && (
          <div className="flex items-center justify-between border-b border-amber-500/40 bg-amber-500/15 px-6 py-2 text-xs text-amber-900 dark:text-amber-200">
            <span className="font-medium">
              Emergency Recovery Mode Active (--recovery). Use Sign-In Recovery Tools only for local administration recovery.
            </span>
            <Badge variant="warning">Recovery</Badge>
          </div>
        )}

        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
