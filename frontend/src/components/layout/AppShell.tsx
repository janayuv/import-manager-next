import * as React from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import {
  Activity,
  AlertTriangle,
  ArrowUpCircle,
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
  PanelLeftClose,
  PanelLeftOpen,
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
import { ThemeModeToggle } from '@/components/layout/ThemeModeToggle';
import { UpdateDialog } from '@/components/system/UpdateDialog';
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

const SIDEBAR_STORAGE_KEY = 'im_sidebar_hidden';

function readInitialSidebarHidden(): boolean {
  try {
    return window.localStorage.getItem(SIDEBAR_STORAGE_KEY) === 'true';
  } catch {
    return false;
  }
}

export function AppShell() {
  const { session, setupStatus, hasPermission, logout } = useAuth();
  const [sidebarHidden, setSidebarHidden] = React.useState<boolean>(() => readInitialSidebarHidden());
  const [updateOpen, setUpdateOpen] = React.useState(false);

  const toggleSidebar = React.useCallback((hidden: boolean) => {
    setSidebarHidden(hidden);
    try {
      window.localStorage.setItem(SIDEBAR_STORAGE_KEY, String(hidden));
    } catch {
      // Ignore storage errors in restricted WebViews.
    }
  }, []);

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <aside
        aria-label="Sidebar"
        data-collapsed={sidebarHidden}
        className={cn(
          'flex shrink-0 flex-col border-r border-border bg-card transition-[width] duration-200',
          sidebarHidden ? 'w-16' : 'w-64',
        )}
      >
        <div className={cn('border-b border-border py-4', sidebarHidden ? 'px-2' : 'px-4')}>
          {sidebarHidden ? (
            <div className="flex justify-center">
              <Button
                variant="ghost"
                size="sm"
                className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
                onClick={() => toggleSidebar(false)}
                title="Show sidebar"
                aria-label="Show sidebar"
              >
                <PanelLeftOpen className="h-4 w-4" />
              </Button>
            </div>
          ) : (
            <>
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <span className="text-base font-bold tracking-tight">Import Manager</span>
                  <Badge variant="outline">Next</Badge>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 w-7 shrink-0 p-0 text-muted-foreground hover:text-foreground"
                  onClick={() => toggleSidebar(true)}
                  title="Hide sidebar"
                  aria-label="Hide sidebar"
                >
                  <PanelLeftClose className="h-4 w-4" />
                </Button>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">Desktop Customs &amp; ERP</p>
            </>
          )}
        </div>

        <nav
          className={cn('flex-1 overflow-y-auto py-4', sidebarHidden ? 'space-y-2 px-2' : 'space-y-5 px-3')}
          aria-label="Primary navigation"
        >
          {NAV_GROUPS.map((group, groupIdx) => {
            const visibleItems = group.items.filter(
              (item) => !item.permission || hasPermission(item.permission),
            );
            if (visibleItems.length === 0) return null;

            return (
              <div key={group.title} className="space-y-1">
                {sidebarHidden ? (
                  groupIdx > 0 && <div className="mx-auto my-2 h-px w-8 bg-border" />
                ) : (
                  <div className="px-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    {group.title}
                  </div>
                )}
                {visibleItems.map((item) => {
                  const Icon = item.icon;
                  return (
                    <NavLink
                      key={item.to}
                      to={item.to}
                      end={item.to === '/'}
                      title={item.label}
                      aria-label={item.label}
                      className={({ isActive }) =>
                        cn(
                          'flex items-center rounded-md text-sm font-medium transition-colors',
                          sidebarHidden ? 'justify-center px-2 py-2' : 'gap-2.5 px-2.5 py-1.5',
                          isActive
                            ? 'bg-primary text-primary-foreground'
                            : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                        )
                      }
                    >
                      <Icon className="h-4 w-4 shrink-0" />
                      {!sidebarHidden && <span className="truncate">{item.label}</span>}
                    </NavLink>
                  );
                })}
              </div>
            );
          })}
        </nav>

        <div className={cn('space-y-2 border-t border-border', sidebarHidden ? 'p-2' : 'p-3')}>
          <div className={sidebarHidden ? 'flex flex-col items-center gap-2' : 'flex items-center justify-between gap-2'}>
            <ThemeModeToggle vertical={sidebarHidden} />
            <Button
              variant="outline"
              size="sm"
              className={sidebarHidden ? 'h-8 w-8 p-0' : 'h-8 flex-1 justify-start gap-1.5 px-2 text-xs'}
              onClick={() => setUpdateOpen(true)}
              title="Check for updates from GitHub"
              aria-label="Check for updates"
            >
              <ArrowUpCircle className="h-4 w-4 shrink-0 text-primary" />
              {!sidebarHidden && <span>Updates</span>}
            </Button>
          </div>
          {session &&
            (sidebarHidden ? (
              <div className="flex justify-center">
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 w-8 p-0"
                  onClick={() => void logout()}
                  title={`Sign out (${session.username})`}
                  aria-label="Sign out"
                >
                  <LogOut className="h-4 w-4" />
                </Button>
              </div>
            ) : (
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
            ))}
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

      <UpdateDialog open={updateOpen} onOpenChange={setUpdateOpen} />
    </div>
  );
}
