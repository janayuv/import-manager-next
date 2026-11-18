import * as React from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ArrowUpCircle,
  BarChart3,
  Box,
  Calculator,
  Command,
  Compass,
  Database,
  FileCheck2,
  FileSpreadsheet,
  FileText,
  History,
  KeyRound,
  LayoutDashboard,
  Moon,
  PackageSearch,
  Receipt,
  Search,
  Settings,
  Shield,
  Ship,
  SlidersHorizontal,
  Sun,
  Trash2,
  Truck,
  Users,
  Wand2,
} from 'lucide-react';
import { useOptionalTheme } from '@/contexts/ThemeContext';
import { cn } from '@/lib/utils';

export interface CommandItem {
  id: string;
  title: string;
  subtitle?: string;
  category: 'Navigation' | 'Customs & ERP' | 'Tools & Actions' | 'Preferences';
  icon: React.ComponentType<{ className?: string }>;
  keywords?: string[];
  action: () => void;
}

interface CommandPaletteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onOpenSimulator?: () => void;
  onOpenIcegate?: () => void;
  onOpenShortcuts?: () => void;
  onOpenUpdates?: () => void;
}

export function CommandPaletteDialog({
  open,
  onOpenChange,
  onOpenSimulator,
  onOpenIcegate,
  onOpenShortcuts,
  onOpenUpdates,
}: CommandPaletteDialogProps) {
  const navigate = useNavigate();
  const themeCtx = useOptionalTheme();
  const [query, setQuery] = React.useState('');
  const [selectedIndex, setSelectedIndex] = React.useState(0);
  const inputRef = React.useRef<HTMLInputElement>(null);

  const nav = React.useCallback(
    (path: string) => {
      onOpenChange(false);
      navigate(path);
    },
    [navigate, onOpenChange],
  );

  const commands: CommandItem[] = React.useMemo(() => {
    const list: CommandItem[] = [
      // Navigation
      {
        id: 'nav-dashboard',
        title: 'Executive Dashboard',
        subtitle: 'Real-time KPIs, charts and financial year trends',
        category: 'Navigation',
        icon: LayoutDashboard,
        keywords: ['home', 'overview', 'stats', 'analytics'],
        action: () => nav('/'),
      },
      {
        id: 'nav-shipments',
        title: 'Shipments Registry',
        subtitle: 'Manage active import shipments, track ETAs and documents',
        category: 'Navigation',
        icon: Ship,
        keywords: ['shipping', 'cargo', 'bl', 'awb', 'vessel'],
        action: () => nav('/shipments'),
      },
      {
        id: 'nav-transit-shipments',
        title: 'Transit Shipment Tracker',
        subtitle: 'Track in-transit shipments, planned vs actual duration and export overview',
        category: 'Navigation',
        icon: Compass,
        keywords: ['transit', 'shipping', 'etd', 'eta', 'tracking', 'voyage', 'export'],
        action: () => nav('/transit-shipments'),
      },
      {
        id: 'nav-invoices',
        title: 'Commercial Invoices',
        subtitle: 'Browse and inspect commercial invoices with supplier lines',
        category: 'Navigation',
        icon: FileText,
        keywords: ['bills', 'vendor invoice', 'commercial'],
        action: () => nav('/invoices'),
      },
      {
        id: 'nav-wizard',
        title: 'Commercial Invoice Wizard',
        subtitle: '3-step draft and finalize wizard with paste line support',
        category: 'Navigation',
        icon: Wand2,
        keywords: ['create invoice', 'new invoice', 'paste lines'],
        action: () => nav('/invoice-wizard'),
      },
      {
        id: 'nav-items',
        title: 'Item Master (Parts)',
        subtitle: 'SKU registry with HSN tariff codes, baseline duty and units',
        category: 'Navigation',
        icon: Box,
        keywords: ['parts', 'catalog', 'products', 'hsn'],
        action: () => nav('/items'),
      },
      {
        id: 'nav-suppliers',
        title: 'Suppliers Directory',
        subtitle: 'Global vendors, country codes, currencies and history',
        category: 'Navigation',
        icon: Users,
        keywords: ['vendors', 'manufacturers', 'exporters'],
        action: () => nav('/suppliers'),
      },
      {
        id: 'nav-boe',
        title: 'BOE Registry & Challans',
        subtitle: 'Customs Bills of Entry, duty paid challans, and ICEGATE refs',
        category: 'Customs & ERP',
        icon: FileSpreadsheet,
        keywords: ['bill of entry', 'challan', 'customs duty', 'icegate'],
        action: () => nav('/boe'),
      },
      {
        id: 'nav-reconciled-boe',
        title: 'Reconciled BOE Module',
        subtitle: 'View, edit, and delete reconciled customs Bill of Entry calculations',
        category: 'Customs & ERP',
        icon: FileCheck2,
        keywords: ['reconciled boe', 'duty calculations', 'reconciled', 'customs reconcile'],
        action: () => nav('/reconciled-boe'),
      },
      {
        id: 'nav-boe-entry',
        title: 'BOE Duty Calculation Entry',
        subtitle: 'Duty calculation engine, exchange rate, BCD/SWS/IGST overrides',
        category: 'Customs & ERP',
        icon: Calculator,
        keywords: ['calculate duty', 'cif', 'freight', 'insurance', 'rate override'],
        action: () => nav('/boe-entry'),
      },
      {
        id: 'nav-expenses',
        title: 'Import Expenses Ledger',
        subtitle: 'Service provider bills, freight, CHA fees, paise/BP precision',
        category: 'Customs & ERP',
        icon: Receipt,
        keywords: ['cha', 'freight', 'transport', 'costs', 'gst', 'tds'],
        action: () => nav('/expenses'),
      },
      {
        id: 'nav-expense-reports',
        title: 'Expense Reports & Analysis',
        subtitle: '5-view expense analysis with invoice summaries and breakdowns',
        category: 'Customs & ERP',
        icon: BarChart3,
        keywords: ['spending', 'vendor spend', 'tax reports'],
        action: () => nav('/expense-reports'),
      },
      {
        id: 'nav-landed-cost',
        title: 'Landed Cost Report',
        subtitle: 'Part-by-part landed cost allocation and margin calculations',
        category: 'Customs & ERP',
        icon: PackageSearch,
        keywords: ['ldc', 'unit cost', 'total cost', 'assessable value'],
        action: () => nav('/reports/landed-cost'),
      },
      {
        id: 'nav-part-analysis',
        title: 'Part / Item Period Analysis',
        subtitle: '8 KPI cards, supplier trends, duty and Excel exports',
        category: 'Customs & ERP',
        icon: History,
        keywords: ['trend', 'price history', 'traceability', 'part lookup'],
        action: () => nav('/part-analysis'),
      },

      // Tools & Actions
      {
        id: 'act-simulator',
        title: 'Landed Cost "What-If" Simulator',
        subtitle: 'Interactive sensitivity analysis for exchange rate, freight & tariffs',
        category: 'Tools & Actions',
        icon: Calculator,
        keywords: ['what if', 'sensitivity', 'simulation', 'forecast'],
        action: () => {
          onOpenChange(false);
          onOpenSimulator?.();
        },
      },
      {
        id: 'act-icegate',
        title: 'ICEGATE BOE Ingestion Assistant',
        subtitle: 'Paste or upload ICEGATE Bill of Entry text to auto-fill duty fields',
        category: 'Tools & Actions',
        icon: FileSpreadsheet,
        keywords: ['icegate', 'import boe', 'parser', 'customs copy'],
        action: () => {
          onOpenChange(false);
          onOpenIcegate?.();
        },
      },
      {
        id: 'act-shortcuts',
        title: 'Keyboard Shortcuts Cheat Sheet',
        subtitle: 'Display hotkeys and navigation shortcuts',
        category: 'Tools & Actions',
        icon: Command,
        keywords: ['hotkeys', 'keys', 'cheat sheet', 'help'],
        action: () => {
          onOpenChange(false);
          onOpenShortcuts?.();
        },
      },
      {
        id: 'nav-database',
        title: 'Database & Backups Console',
        subtitle: 'Encrypted SQLCipher backups, Google Drive sync, and restore',
        category: 'Tools & Actions',
        icon: Database,
        keywords: ['backup', 'restore', 'google drive', 'sqlcipher'],
        action: () => nav('/database'),
      },
      {
        id: 'nav-recycle-bin',
        title: 'Recycle Bin',
        subtitle: 'Inspect soft-deleted records with restore and dependency purge guards',
        category: 'Tools & Actions',
        icon: Trash2,
        keywords: ['trash', 'deleted', 'restore'],
        action: () => nav('/recycle-bin'),
      },
      {
        id: 'nav-settings',
        title: 'ERP Settings & Profile',
        subtitle: 'Pagination, decimal precision, date formats, and themes',
        category: 'Preferences',
        icon: Settings,
        keywords: ['preferences', 'config', 'pagination', 'defaults'],
        action: () => nav('/settings'),
      },
      {
        id: 'nav-security',
        title: 'Security Center & Sessions',
        subtitle: 'Active sessions, lockout history, password policy and argon2id',
        category: 'Preferences',
        icon: Shield,
        keywords: ['security', 'auth', 'passwords', 'sessions'],
        action: () => nav('/admin/security-center'),
      },
      {
        id: 'nav-roles',
        title: 'Roles & Permissions (RBAC)',
        subtitle: 'Role definitions, permission assignments, and audit controls',
        category: 'Preferences',
        icon: KeyRound,
        keywords: ['rbac', 'access control', 'roles'],
        action: () => nav('/admin/roles-permissions'),
      },
      {
        id: 'nav-audit',
        title: 'Audit Trail & Activity Log',
        subtitle: 'Immutable record modifications and user event history',
        category: 'Preferences',
        icon: History,
        keywords: ['audit', 'logs', 'compliance'],
        action: () => nav('/admin/activity-log'),
      },
      {
        id: 'nav-operations',
        title: 'Operations Center & Diagnostics',
        subtitle: 'Workflow exceptions, ETA breach warnings, and health tools',
        category: 'Tools & Actions',
        icon: Truck,
        keywords: ['sla', 'exceptions', 'incidents', 'health'],
        action: () => nav('/admin/operations-center'),
      },
      {
        id: 'nav-automation',
        title: 'Automation Rules Engine',
        subtitle: 'Automated 7-day sweeps, alert rules, and self-healing jobs',
        category: 'Tools & Actions',
        icon: SlidersHorizontal,
        keywords: ['rules', 'triggers', 'automation'],
        action: () => nav('/admin/automation-rules'),
      },
      {
        id: 'act-updates',
        title: 'Check for Software Updates',
        subtitle: 'Check GitHub releases for latest Windows desktop installer',
        category: 'Tools & Actions',
        icon: ArrowUpCircle,
        keywords: ['update', 'version', 'github', 'upgrade'],
        action: () => {
          onOpenChange(false);
          onOpenUpdates?.();
        },
      },
      {
        id: 'pref-theme-dark',
        title: 'Switch to Dark Mode',
        subtitle: 'Set interface appearance to dark',
        category: 'Preferences',
        icon: Moon,
        keywords: ['theme', 'dark', 'night'],
        action: () => {
          void themeCtx?.setMode('dark');
          onOpenChange(false);
        },
      },
      {
        id: 'pref-theme-light',
        title: 'Switch to Light Mode',
        subtitle: 'Set interface appearance to light',
        category: 'Preferences',
        icon: Sun,
        keywords: ['theme', 'light', 'day'],
        action: () => {
          void themeCtx?.setMode('light');
          onOpenChange(false);
        },
      },
    ];

    return list;
  }, [nav, onOpenChange, onOpenSimulator, onOpenIcegate, onOpenShortcuts, onOpenUpdates, themeCtx]);

  // Filter commands
  const filtered = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter((cmd) => {
      if (cmd.title.toLowerCase().includes(q)) return true;
      if (cmd.subtitle?.toLowerCase().includes(q)) return true;
      if (cmd.category.toLowerCase().includes(q)) return true;
      if (cmd.keywords?.some((k) => k.toLowerCase().includes(q))) return true;
      return false;
    });
  }, [commands, query]);

  // Reset selected index on filter
  React.useEffect(() => {
    setSelectedIndex(0);
  }, [filtered.length]);

  // Focus input when dialog opens
  React.useEffect(() => {
    if (open) {
      setQuery('');
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex((prev) => (prev + 1 < filtered.length ? prev + 1 : 0));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex((prev) => (prev - 1 >= 0 ? prev - 1 : filtered.length - 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const selected = filtered[selectedIndex];
      if (selected) {
        selected.action();
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      onOpenChange(false);
    }
  };

  if (!open) return null;

  // Group filtered commands
  const grouped = filtered.reduce<Record<string, CommandItem[]>>((acc, item) => {
    acc[item.category] = acc[item.category] || [];
    acc[item.category]?.push(item);
    return acc;
  }, {});

  let globalIndexCounter = 0;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Command Palette"
      className="fixed inset-0 z-50 flex items-start justify-center pt-16 sm:pt-24 px-4 bg-black/50 backdrop-blur-xs animate-in fade-in duration-150"
      onClick={() => onOpenChange(false)}
    >
      <div
        className="w-full max-w-2xl overflow-hidden rounded-xl border border-border bg-card shadow-2xl transition-all"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        <div className="flex items-center gap-2.5 border-b border-border px-4 py-3">
          <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Type a command or search (e.g. shipments, boe, simulator, items)..."
            aria-label="Command search"
            className="flex-1 bg-transparent text-sm font-medium outline-hidden placeholder:text-muted-foreground/70"
          />
          <kbd className="hidden sm:inline-flex h-5 items-center gap-1 rounded border border-border bg-muted/60 px-1.5 font-mono text-[10px] text-muted-foreground">
            ESC
          </kbd>
        </div>

        <div className="max-h-96 overflow-y-auto p-2 space-y-3">
          {filtered.length === 0 ? (
            <div className="py-10 text-center text-sm text-muted-foreground">
              No matching commands or destinations found for "{query}".
            </div>
          ) : (
            Object.entries(grouped).map(([category, items]) => (
              <div key={category} className="space-y-1">
                <div className="px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                  {category}
                </div>
                {items.map((item) => {
                  const itemIndex = globalIndexCounter++;
                  const isSelected = itemIndex === selectedIndex;
                  const Icon = item.icon;

                  return (
                    <div
                      key={item.id}
                      role="button"
                      tabIndex={0}
                      onClick={() => item.action()}
                      onMouseEnter={() => setSelectedIndex(itemIndex)}
                      className={cn(
                        'flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors cursor-pointer',
                        isSelected
                          ? 'bg-primary text-primary-foreground shadow-xs'
                          : 'text-foreground hover:bg-accent hover:text-accent-foreground',
                      )}
                    >
                      <div
                        className={cn(
                          'flex h-7 w-7 shrink-0 items-center justify-center rounded-md border',
                          isSelected
                            ? 'border-primary-foreground/30 bg-primary-foreground/15 text-primary-foreground'
                            : 'border-border bg-muted/50 text-muted-foreground',
                        )}
                      >
                        <Icon className="h-4 w-4" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="font-medium truncate">{item.title}</div>
                        {item.subtitle && (
                          <div
                            className={cn(
                              'text-xs truncate',
                              isSelected ? 'text-primary-foreground/80' : 'text-muted-foreground',
                            )}
                          >
                            {item.subtitle}
                          </div>
                        )}
                      </div>
                      {isSelected && (
                        <span className="text-[11px] font-medium opacity-90 hidden sm:inline">
                          Press Enter ↵
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            ))
          )}
        </div>

        <div className="flex items-center justify-between border-t border-border bg-muted/40 px-4 py-2 text-xs text-muted-foreground">
          <div className="flex items-center gap-3">
            <span>
              Navigate <kbd className="font-mono">↑</kbd> <kbd className="font-mono">↓</kbd>
            </span>
            <span>
              Select <kbd className="font-mono">↵</kbd>
            </span>
            <span>
              Close <kbd className="font-mono">Esc</kbd>
            </span>
          </div>
          <span className="text-[11px]">Import Manager Pro</span>
        </div>
      </div>
    </div>
  );
}
