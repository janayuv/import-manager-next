import * as React from 'react';
import {
  AlertTriangle,
  ArrowUpDown,
  Building2,
  CheckCircle2,
  Clock,
  Download,
  Eye,
  FileSpreadsheet,
  FileText,
  HelpCircle,
  Info,
  Package,
  RefreshCw,
  Search,
  Ship,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { SkeletonTable } from '@/components/ui/skeleton';
import {
  exportTransitShipmentExcel,
  exportTransitShipmentTabularCSV,
  exportTransitShipmentTemplateCSV,
} from '@/lib/transit-shipment-export';
import { formatTransitDateDisplay } from '@/lib/format';
import { useDebounce } from '@/hooks/useDebounce';
import { supplierService, type Supplier } from '@/services/master-service';
import {
  shipmentService,
  type TransitShipmentFilter,
  type TransitShipmentItem,
  type TransitShipmentReport,
  type Shipment,
} from '@/services/shipment-invoice-service';

export function computeTransitStatusClient(
  etdStr?: string,
  etaStr?: string,
  dateOfDelivery?: string,
  shipmentStatus?: string,
  refDate: Date = new Date(),
): { status: 'In Transit' | 'Arrived' | 'Delayed' | 'Pending'; transitDays?: number } {
  let transitDays: number | undefined;
  if (etdStr && etaStr) {
    const etd = new Date(etdStr);
    const eta = new Date(etaStr);
    if (!isNaN(etd.getTime()) && !isNaN(eta.getTime())) {
      transitDays = Math.round((eta.getTime() - etd.getTime()) / (1000 * 60 * 60 * 24));
    }
  }

  const isArrived =
    (dateOfDelivery && dateOfDelivery.trim() !== '') ||
    shipmentStatus?.toLowerCase() === 'delivered' ||
    shipmentStatus?.toLowerCase() === 'arrived' ||
    shipmentStatus?.toLowerCase() === 'completed';

  if (isArrived) {
    return { status: 'Arrived', transitDays };
  }

  if (!etaStr || etaStr.trim() === '') {
    return { status: 'Pending', transitDays };
  }

  const eta = new Date(etaStr);
  if (isNaN(eta.getTime())) {
    return { status: 'Pending', transitDays };
  }

  const today = new Date(refDate.getFullYear(), refDate.getMonth(), refDate.getDate());
  const etaMid = new Date(eta.getFullYear(), eta.getMonth(), eta.getDate());

  if (etaMid.getTime() < today.getTime()) {
    return { status: 'Delayed', transitDays };
  }
  return { status: 'In Transit', transitDays };
}

export function TransitShipmentsPage() {
  const [loading, setLoading] = React.useState(false);
  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);

  // Filters
  const [search, setSearch] = React.useState('');
  const debouncedSearch = useDebounce(search, 250);
  const [supplierId, setSupplierId] = React.useState('all');
  const [statusFilter, setStatusFilter] = React.useState('all');
  const [startDate, setStartDate] = React.useState('');
  const [endDate, setEndDate] = React.useState('');

  // Pagination & Sorting
  const [currentPage, setCurrentPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(15);
  const [sortBy, setSortBy] = React.useState<'invoiceDate' | 'etd' | 'eta' | 'transitDays' | 'supplierName' | 'invoiceNumber'>('invoiceDate');
  const [sortDir, setSortDir] = React.useState<'asc' | 'desc'>('desc');

  // UI view toggles
  const [showGuide, setShowGuide] = React.useState(false);
  const [exportingExcel, setExportingExcel] = React.useState(false);

  // Selected item for Inspection / Quick Date Update Dialog
  const [inspectItem, setInspectItem] = React.useState<TransitShipmentItem | null>(null);
  const [editEtd, setEditEtd] = React.useState('');
  const [editEta, setEditEta] = React.useState('');
  const [editAta, setEditAta] = React.useState('');
  const [savingEdit, setSavingEdit] = React.useState(false);

  // Loaded Report State
  const [report, setReport] = React.useState<TransitShipmentReport>({
    summary: {
      totalShipments: 0,
      inTransit: 0,
      arrived: 0,
      delayed: 0,
      pending: 0,
      averagePlannedTransit: 0,
    },
    breakdown: [
      { status: 'In Transit', count: 0, pct: 0 },
      { status: 'Arrived', count: 0, pct: 0 },
      { status: 'Delayed', count: 0, pct: 0 },
      { status: 'Pending', count: 0, pct: 0 },
    ],
    items: [],
  });

  // Load suppliers for filter
  React.useEffect(() => {
    void supplierService
      .list({
        search: '',
        status: 'active',
        sortBy: 'supplierName',
        sortDir: 'asc',
        page: 1,
        pageSize: 500,
      })
      .then((res) => setSuppliers(res.items ?? []))
      .catch(() => {});
  }, []);

  // Fetch report
  const loadData = React.useCallback(async () => {
    setLoading(true);
    try {
      const filter: TransitShipmentFilter = {
        search: debouncedSearch.trim(),
        supplierId: supplierId === 'all' ? '' : supplierId,
        status: statusFilter === 'all' ? '' : statusFilter,
        startDate: startDate || undefined,
        endDate: endDate || undefined,
      };

      // Try backend GetTransitShipmentReport
      try {
        const rep = await shipmentService.getTransitShipmentReport(filter);
        if (rep && (rep.items?.length > 0 || rep.summary?.totalShipments > 0)) {
          setReport(rep);
          return;
        }
      } catch {
        // Fall back to client calculation
      }

      // Client-side fallback using shipmentService.list
      const pageRes = await shipmentService.list({
        search: debouncedSearch.trim(),
        supplierId: supplierId === 'all' ? '' : supplierId,
        status: '',
        frozenOnly: false,
        sortBy: 'invoiceDate',
        sortDir: 'DESC',
        page: 1,
        pageSize: 500,
      });

      const items = pageRes.items ?? [];
      const now = new Date();

      let inTransitCount = 0;
      let arrivedCount = 0;
      let delayedCount = 0;
      let pendingCount = 0;
      let totalTransitDays = 0;
      let transitCount = 0;

      const allTransitItems: TransitShipmentItem[] = items.map((sh, idx) => {
        const calc = computeTransitStatusClient(sh.etd, sh.eta, sh.dateOfDelivery, sh.status, now);
        if (calc.transitDays !== undefined) {
          totalTransitDays += calc.transitDays;
          transitCount++;
        }
        if (calc.status === 'In Transit') inTransitCount++;
        else if (calc.status === 'Arrived') arrivedCount++;
        else if (calc.status === 'Delayed') delayedCount++;
        else pendingCount++;

        return {
          sNo: idx + 1,
          id: sh.id,
          supplierId: sh.supplierId,
          supplierName: sh.supplierName || 'Unknown Supplier',
          invoiceNumber: sh.invoiceNumber,
          invoiceDate: sh.invoiceDate,
          vesselVoyage: sh.vesselName || '',
          etd: sh.etd || '',
          eta: sh.eta || '',
          transitDays: calc.transitDays ?? null,
          status: calc.status,
          remarks: sh.containerNumber ? `Container: ${sh.containerNumber}` : '',
          dateOfDelivery: sh.dateOfDelivery || '',
          blAwbNumber: sh.blAwbNumber || '',
          containerNumber: sh.containerNumber || '',
          grossWeightKg: sh.grossWeightKg || 0,
          invoiceValue: sh.invoiceValue || 0,
          invoiceCurrency: sh.invoiceCurrency || '',
        };
      });

      const total = allTransitItems.length;
      const avgDays = transitCount > 0 ? Math.round((totalTransitDays / transitCount) * 10) / 10 : 0;

      let filtered = allTransitItems;
      if (statusFilter !== 'all') {
        const target = statusFilter.toLowerCase().replace(/-/g, ' ');
        filtered = filtered.filter((i) => i.status.toLowerCase() === target);
      }
      if (startDate) {
        filtered = filtered.filter((i) => !i.invoiceDate || i.invoiceDate >= startDate);
      }
      if (endDate) {
        filtered = filtered.filter((i) => !i.invoiceDate || i.invoiceDate <= endDate);
      }

      const calcPct = (cnt: number) => (total > 0 ? Math.round((cnt / total) * 1000) / 10 : 0);

      setReport({
        summary: {
          totalShipments: total,
          inTransit: inTransitCount,
          arrived: arrivedCount,
          delayed: delayedCount,
          pending: pendingCount,
          averagePlannedTransit: avgDays,
        },
        breakdown: [
          { status: 'In Transit', count: inTransitCount, pct: calcPct(inTransitCount) },
          { status: 'Arrived', count: arrivedCount, pct: calcPct(arrivedCount) },
          { status: 'Delayed', count: delayedCount, pct: calcPct(delayedCount) },
          { status: 'Pending', count: pendingCount, pct: calcPct(pendingCount) },
        ],
        items: filtered,
      });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search, supplierId, statusFilter, startDate, endDate]);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  // Client-side sorting & pagination of report items
  const sortedItems = React.useMemo(() => {
    const arr = [...(report.items || [])];
    arr.sort((a, b) => {
      let valA: string | number = '';
      let valB: string | number = '';
      if (sortBy === 'transitDays') {
        valA = a.transitDays ?? -9999;
        valB = b.transitDays ?? -9999;
      } else {
        valA = a[sortBy] ?? '';
        valB = b[sortBy] ?? '';
      }

      if (valA < valB) return sortDir === 'asc' ? -1 : 1;
      if (valA > valB) return sortDir === 'asc' ? 1 : -1;
      return 0;
    });
    return arr;
  }, [report.items, sortBy, sortDir]);

  const totalPages = Math.max(1, Math.ceil(sortedItems.length / pageSize));
  const paginatedItems = React.useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return sortedItems.slice(start, start + pageSize);
  }, [sortedItems, currentPage, pageSize]);

  const handleSort = (col: typeof sortBy) => {
    if (sortBy === col) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortBy(col);
      setSortDir('asc');
    }
  };

  // Export handlers
  const handleExportExcel = async () => {
    setExportingExcel(true);
    try {
      await exportTransitShipmentExcel(report);
      toast.success('Exported professional 2-sheet Transit Shipment Workbook (.xlsx)');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setExportingExcel(false);
    }
  };

  const handleExportTemplateCsv = async () => {
    try {
      // If backend method works, try it; otherwise use frontend generator
      try {
        const csv = await shipmentService.exportTransitShipmentsCsv({
          search: debouncedSearch.trim(),
          supplierId: supplierId === 'all' ? '' : supplierId,
          status: statusFilter === 'all' ? '' : statusFilter,
          startDate: startDate || undefined,
          endDate: endDate || undefined,
        });
        if (csv) {
          const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = 'Import_Shipment_Transit_Details_Template.csv';
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
          URL.revokeObjectURL(url);
          toast.success('Exported reference-compliant Template CSV');
          return;
        }
      } catch {
        // Fall back to frontend generator
      }
      exportTransitShipmentTemplateCSV(report);
      toast.success('Exported reference-compliant Template CSV');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleExportTabularCsv = () => {
    try {
      exportTransitShipmentTabularCSV(report);
      toast.success('Exported clean Tabular Transit CSV');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  // Open Quick Edit
  const openInspectDialog = (item: TransitShipmentItem) => {
    setInspectItem(item);
    setEditEtd(item.etd || '');
    setEditEta(item.eta || '');
    setEditAta(item.dateOfDelivery || '');
  };

  // Save quick date update
  const handleSaveDates = async () => {
    if (!inspectItem) return;
    setSavingEdit(true);
    try {
      const full = await shipmentService.getById(inspectItem.id);
      const updated: Shipment = {
        ...full,
        etd: editEtd,
        eta: editEta,
        dateOfDelivery: editAta,
      };
      // If ATA is entered, set status to delivered if not cancelled
      if (editAta.trim() && updated.status !== 'cancelled' && updated.status !== 'closed') {
        updated.status = 'delivered';
      }
      await shipmentService.update(inspectItem.id, updated);
      toast.success(`Updated shipment ${inspectItem.invoiceNumber} transit dates`);
      setInspectItem(null);
      void loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSavingEdit(false);
    }
  };

  const clearFilters = () => {
    setSearch('');
    setSupplierId('all');
    setStatusFilter('all');
    setStartDate('');
    setEndDate('');
    setCurrentPage(1);
  };

  const hasActiveFilters = search !== '' || supplierId !== 'all' || statusFilter !== 'all' || startDate !== '' || endDate !== '';

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <Ship className="h-6 w-6 text-primary" />
            Transit Shipment Tracker
          </h1>
          <p className="text-sm text-muted-foreground">
            Live import transit monitoring, planned duration tracking, and overview dashboard
          </p>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => void loadData()}
            disabled={loading}
            title="Refresh transit data"
          >
            <RefreshCw className={`h-4 w-4 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={handleExportTabularCsv}
            disabled={loading || report.items.length === 0}
            className="text-foreground"
          >
            <FileText className="h-4 w-4 mr-1.5 text-muted-foreground" />
            Tabular CSV
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={handleExportTemplateCsv}
            disabled={loading || report.items.length === 0}
            className="text-foreground"
          >
            <Download className="h-4 w-4 mr-1.5 text-blue-600 dark:text-blue-400" />
            Template CSV
          </Button>

          <Button
            size="sm"
            onClick={() => void handleExportExcel()}
            disabled={loading || exportingExcel || report.items.length === 0}
            className="bg-emerald-600 hover:bg-emerald-700 text-white dark:bg-emerald-700 dark:hover:bg-emerald-800"
          >
            <FileSpreadsheet className="h-4 w-4 mr-1.5" />
            {exportingExcel ? 'Exporting...' : 'Export Excel (.xlsx)'}
          </Button>
        </div>
      </div>

      {/* Overview KPI Cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
        <Card className="border-border/60 shadow-xs">
          <CardHeader className="p-4 pb-2">
            <CardDescription className="flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Total Shipments
              <Package className="h-4 w-4 text-muted-foreground" />
            </CardDescription>
            <CardTitle className="text-2xl font-bold text-foreground">
              {report.summary.totalShipments}
            </CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0 text-xs text-muted-foreground">
            Active tracked consignments
          </CardContent>
        </Card>

        <Card className="border-blue-200/80 bg-blue-50/30 dark:border-blue-900/40 dark:bg-blue-950/20 shadow-xs">
          <CardHeader className="p-4 pb-2">
            <CardDescription className="flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-blue-700 dark:text-blue-300">
              In Transit
              <Ship className="h-4 w-4 text-blue-600 dark:text-blue-400" />
            </CardDescription>
            <CardTitle className="text-2xl font-bold text-blue-700 dark:text-blue-300">
              {report.summary.inTransit}
            </CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0 text-xs text-blue-600/80 dark:text-blue-400/80">
            En route (ETA today or future)
          </CardContent>
        </Card>

        <Card className="border-emerald-200/80 bg-emerald-50/30 dark:border-emerald-900/40 dark:bg-emerald-950/20 shadow-xs">
          <CardHeader className="p-4 pb-2">
            <CardDescription className="flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-emerald-700 dark:text-emerald-300">
              Arrived
              <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
            </CardDescription>
            <CardTitle className="text-2xl font-bold text-emerald-700 dark:text-emerald-300">
              {report.summary.arrived}
            </CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0 text-xs text-emerald-600/80 dark:text-emerald-400/80">
            Delivered / arrival confirmed
          </CardContent>
        </Card>

        <Card className="border-rose-200/80 bg-rose-50/30 dark:border-rose-900/40 dark:bg-rose-950/20 shadow-xs">
          <CardHeader className="p-4 pb-2">
            <CardDescription className="flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-rose-700 dark:text-rose-300">
              Delayed
              <AlertTriangle className="h-4 w-4 text-rose-600 dark:text-rose-400" />
            </CardDescription>
            <CardTitle className="text-2xl font-bold text-rose-700 dark:text-rose-300">
              {report.summary.delayed}
            </CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0 text-xs text-rose-600/80 dark:text-rose-400/80">
            ETA passed without arrival
          </CardContent>
        </Card>

        <Card className="border-slate-200/80 bg-slate-50/40 dark:border-slate-800 dark:bg-slate-900/40 shadow-xs">
          <CardHeader className="p-4 pb-2">
            <CardDescription className="flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-slate-700 dark:text-slate-300">
              Pending
              <HelpCircle className="h-4 w-4 text-slate-500" />
            </CardDescription>
            <CardTitle className="text-2xl font-bold text-slate-700 dark:text-slate-300">
              {report.summary.pending}
            </CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0 text-xs text-slate-600/80 dark:text-slate-400/80">
            Awaiting ETA allocation
          </CardContent>
        </Card>

        <Card className="border-border/60 shadow-xs">
          <CardHeader className="p-4 pb-2">
            <CardDescription className="flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Avg Planned Transit
              <Clock className="h-4 w-4 text-muted-foreground" />
            </CardDescription>
            <CardTitle className="text-2xl font-bold text-foreground">
              {report.summary.averagePlannedTransit > 0
                ? `${report.summary.averagePlannedTransit} d`
                : '—'}
            </CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0 text-xs text-muted-foreground">
            Calendar days (ETA − ETD)
          </CardContent>
        </Card>
      </div>

      {/* Status Breakdown & Template Guidelines Toggle */}
      <Card className="border-border/60">
        <CardHeader className="p-4 pb-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base font-semibold">Shipment Status Breakdown</CardTitle>
              <CardDescription className="text-xs">
                Live distribution of tracked consignments across operational states
              </CardDescription>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowGuide((g) => !g)}
              className="text-xs text-muted-foreground hover:text-foreground"
            >
              <Info className="h-4 w-4 mr-1 text-primary" />
              {showGuide ? 'Hide Template Rules' : 'How To Use This Template'}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="p-4 pt-0 space-y-4">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {report.breakdown.map((b) => {
              let badgeColor = 'bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-300';
              if (b.status === 'In Transit') {
                badgeColor = 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300';
              } else if (b.status === 'Arrived') {
                badgeColor = 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/50 dark:text-emerald-300';
              } else if (b.status === 'Delayed') {
                badgeColor = 'bg-rose-100 text-rose-800 dark:bg-rose-900/50 dark:text-rose-300';
              }
              return (
                <div
                  key={b.status}
                  className="flex items-center justify-between rounded-lg border border-border/50 bg-card p-2.5 px-3"
                >
                  <div className="flex items-center gap-2">
                    <span className={`inline-block h-2.5 w-2.5 rounded-full ${badgeColor.split(' ')[0]}`} />
                    <span className="text-sm font-medium">{b.status}</span>
                  </div>
                  <div className="text-right">
                    <span className="text-sm font-bold">{b.count}</span>
                    <span className="ml-1.5 text-xs text-muted-foreground">({b.pct}%)</span>
                  </div>
                </div>
              );
            })}
          </div>

          {/* Collapsible How-To Guide from Template */}
          {showGuide && (
            <div className="rounded-md border border-primary/20 bg-primary/5 p-4 text-xs space-y-2 mt-3">
              <div className="font-semibold text-primary flex items-center gap-1.5">
                <Info className="h-4 w-4" />
                HOW TO USE THIS TEMPLATE (Reference Guide)
              </div>
              <ol className="list-decimal pl-5 space-y-1 text-muted-foreground">
                <li>
                  <strong className="text-foreground">Enter shipment details:</strong> Use the light-blue cells in the Shipment Tracker sheet. One row represents one import shipment / invoice.
                </li>
                <li>
                  <strong className="text-foreground">Required fields:</strong> Enter at least the Shipper / Supplier, Invoice No., ETD and ETA to calculate the planned transit period.
                </li>
                <li>
                  <strong className="text-foreground">Transit Period:</strong> Transit Period (Days) is calculated automatically as ETA minus ETD (calendar days, not working days).
                </li>
                <li>
                  <strong className="text-foreground">Actual Transit:</strong> Enter ATD (Actual Departure) and ATA (Actual Arrival). Actual Transit is calculated automatically.
                </li>
                <li>
                  <strong className="text-foreground">Shipment Status:</strong> Status is automatic:
                  <ul className="list-disc pl-4 mt-0.5 space-y-0.5">
                    <li><strong className="text-slate-700 dark:text-slate-300">Pending:</strong> ETA missing</li>
                    <li><strong className="text-blue-700 dark:text-blue-300">In Transit:</strong> ETA today or later and no ATA</li>
                    <li><strong className="text-rose-700 dark:text-rose-300">Delayed:</strong> ETA passed and no ATA</li>
                    <li><strong className="text-emerald-700 dark:text-emerald-300">Arrived:</strong> ATA entered / delivered</li>
                  </ul>
                </li>
                <li>
                  <strong className="text-foreground">Dashboard:</strong> The Dashboard updates automatically based on the Shipment Tracker entries.
                </li>
                <li>
                  <strong className="text-foreground">Date format:</strong> Enter real dates. The workbook displays dates as <code className="font-mono">dd-mmm-yyyy</code>.
                </li>
                <li>
                  <strong className="text-foreground">Capacity:</strong> The template is preformatted for 200 shipment entries.
                </li>
              </ol>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Filter & Search Bar */}
      <Card className="border-border/60">
        <CardContent className="p-4 space-y-3">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-5">
            {/* Search */}
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search invoice, supplier, vessel..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-8"
              />
            </div>

            {/* Supplier Filter */}
            <div>
              <Select value={supplierId} onValueChange={setSupplierId}>
                <SelectTrigger>
                  <SelectValue placeholder="All Suppliers" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Suppliers</SelectItem>
                  {suppliers.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.supplierName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Date Range Start */}
            <div>
              <Input
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
                placeholder="Invoice Date From"
                title="Filter by Invoice Date From"
              />
            </div>

            {/* Date Range End */}
            <div>
              <Input
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
                placeholder="Invoice Date To"
                title="Filter by Invoice Date To"
              />
            </div>

            {/* Clear Filters */}
            <div className="flex items-center gap-2">
              {hasActiveFilters && (
                <Button variant="ghost" size="sm" onClick={clearFilters} className="text-xs">
                  <X className="h-3.5 w-3.5 mr-1" />
                  Clear Filters
                </Button>
              )}
            </div>
          </div>

          {/* Quick Status Filter Tabs */}
          <div className="flex items-center gap-2 pt-1 border-t border-border/40">
            <span className="text-xs font-semibold text-muted-foreground mr-1">Status:</span>
            <Tabs value={statusFilter} onValueChange={setStatusFilter}>
              <TabsList className="h-8">
                <TabsTrigger value="all" className="text-xs px-2.5 py-1">
                  All ({report.summary.totalShipments})
                </TabsTrigger>
                <TabsTrigger value="in-transit" className="text-xs px-2.5 py-1">
                  In Transit ({report.summary.inTransit})
                </TabsTrigger>
                <TabsTrigger value="delayed" className="text-xs px-2.5 py-1">
                  Delayed ({report.summary.delayed})
                </TabsTrigger>
                <TabsTrigger value="arrived" className="text-xs px-2.5 py-1">
                  Arrived ({report.summary.arrived})
                </TabsTrigger>
                <TabsTrigger value="pending" className="text-xs px-2.5 py-1">
                  Pending ({report.summary.pending})
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </div>
        </CardContent>
      </Card>

      {/* Transit Shipment Details Table (Matching the 10 Template Columns) */}
      <Card className="border-border/60">
        <CardHeader className="p-4 pb-2 flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base font-semibold">IMPORT SHIPMENT TRANSIT DETAILS</CardTitle>
            <CardDescription className="text-xs">
              Showing {report.items.length === 0 ? 0 : (currentPage - 1) * pageSize + 1} to{' '}
              {Math.min(currentPage * pageSize, report.items.length)} of {report.items.length} consignments
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">Rows per page:</span>
            <Select
              value={String(pageSize)}
              onValueChange={(val) => {
                setPageSize(Number(val));
                setCurrentPage(1);
              }}
            >
              <SelectTrigger className="h-8 w-18 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="10">10</SelectItem>
                <SelectItem value="15">15</SelectItem>
                <SelectItem value="25">25</SelectItem>
                <SelectItem value="50">50</SelectItem>
                <SelectItem value="100">100</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {loading ? (
            <div className="p-4">
              <SkeletonTable rows={8} cols={10} />
            </div>
          ) : report.items.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-12 text-center text-muted-foreground">
              <Ship className="h-10 w-10 mb-3 text-muted-foreground/60" />
              <p className="text-sm font-semibold">No transit shipments found</p>
              <p className="text-xs text-muted-foreground mt-1 max-w-sm">
                No shipments match the current search or status criteria. Try clearing filters or creating new shipments.
              </p>
              {hasActiveFilters && (
                <Button variant="outline" size="sm" onClick={clearFilters} className="mt-4 text-xs">
                  Reset All Filters
                </Button>
              )}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow className="bg-muted/40 hover:bg-muted/40">
                    <TableHead className="w-14 text-center font-bold">S.No.</TableHead>
                    <TableHead
                      className="cursor-pointer hover:text-foreground font-bold"
                      onClick={() => handleSort('supplierName')}
                    >
                      <div className="flex items-center gap-1">
                        Shipper / Supplier
                        <ArrowUpDown className="h-3.5 w-3.5 text-muted-foreground" />
                      </div>
                    </TableHead>
                    <TableHead
                      className="cursor-pointer hover:text-foreground font-bold"
                      onClick={() => handleSort('invoiceNumber')}
                    >
                      <div className="flex items-center gap-1">
                        Invoice No.
                        <ArrowUpDown className="h-3.5 w-3.5 text-muted-foreground" />
                      </div>
                    </TableHead>
                    <TableHead
                      className="cursor-pointer hover:text-foreground font-bold text-center"
                      onClick={() => handleSort('invoiceDate')}
                    >
                      <div className="flex items-center justify-center gap-1">
                        Invoice Date
                        <ArrowUpDown className="h-3.5 w-3.5 text-muted-foreground" />
                      </div>
                    </TableHead>
                    <TableHead className="font-bold">Vessel / Voyage</TableHead>
                    <TableHead
                      className="cursor-pointer hover:text-foreground font-bold text-center"
                      onClick={() => handleSort('etd')}
                    >
                      <div className="flex items-center justify-center gap-1">
                        ETD
                        <ArrowUpDown className="h-3.5 w-3.5 text-muted-foreground" />
                      </div>
                    </TableHead>
                    <TableHead
                      className="cursor-pointer hover:text-foreground font-bold text-center"
                      onClick={() => handleSort('eta')}
                    >
                      <div className="flex items-center justify-center gap-1">
                        ETA
                        <ArrowUpDown className="h-3.5 w-3.5 text-muted-foreground" />
                      </div>
                    </TableHead>
                    <TableHead
                      className="cursor-pointer hover:text-foreground font-bold text-center"
                      onClick={() => handleSort('transitDays')}
                    >
                      <div className="flex items-center justify-center gap-1">
                        Transit Period (Days)
                        <ArrowUpDown className="h-3.5 w-3.5 text-muted-foreground" />
                      </div>
                    </TableHead>
                    <TableHead className="font-bold text-center">Shipment Status</TableHead>
                    <TableHead className="font-bold">Remarks</TableHead>
                    <TableHead className="text-right font-bold w-20">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {paginatedItems.map((item, idx) => {
                    const rowNumber = (currentPage - 1) * pageSize + idx + 1;
                    return (
                      <TableRow key={item.id} className="hover:bg-muted/30">
                        {/* 1. S.No. */}
                        <TableCell className="text-center font-mono text-xs text-muted-foreground">
                          {rowNumber}
                        </TableCell>

                        {/* 2. Shipper / Supplier */}
                        <TableCell className="font-medium text-sm">
                          <div className="flex items-center gap-1.5">
                            <Building2 className="h-3.5 w-3.5 text-muted-foreground/70 shrink-0" />
                            <span className="truncate max-w-[200px]" title={item.supplierName}>
                              {item.supplierName}
                            </span>
                          </div>
                        </TableCell>

                        {/* 3. Invoice No. */}
                        <TableCell className="font-mono text-xs font-semibold text-primary">
                          {item.invoiceNumber}
                        </TableCell>

                        {/* 4. Invoice Date */}
                        <TableCell className="text-center font-mono text-xs text-muted-foreground">
                          {formatTransitDateDisplay(item.invoiceDate) || '—'}
                        </TableCell>

                        {/* 5. Vessel / Voyage */}
                        <TableCell className="text-sm">
                          {item.vesselVoyage ? (
                            <span className="truncate max-w-[180px] inline-block" title={item.vesselVoyage}>
                              {item.vesselVoyage}
                            </span>
                          ) : (
                            <span className="text-muted-foreground italic text-xs">—</span>
                          )}
                        </TableCell>

                        {/* 6. ETD */}
                        <TableCell className="text-center font-mono text-xs">
                          {formatTransitDateDisplay(item.etd) || <span className="text-muted-foreground">—</span>}
                        </TableCell>

                        {/* 7. ETA */}
                        <TableCell className="text-center font-mono text-xs">
                          {formatTransitDateDisplay(item.eta) || <span className="text-muted-foreground">—</span>}
                        </TableCell>

                        {/* 8. Transit Period (Days) */}
                        <TableCell className="text-center">
                          {item.transitDays !== undefined && item.transitDays !== null ? (
                            <Badge variant="outline" className="font-mono text-xs font-semibold">
                              {item.transitDays} d
                            </Badge>
                          ) : (
                            <span className="text-muted-foreground text-xs">—</span>
                          )}
                        </TableCell>

                        {/* 9. Shipment Status */}
                        <TableCell className="text-center">
                          {item.status === 'In Transit' && (
                            <Badge className="border-blue-200 bg-blue-100 text-blue-800 dark:border-blue-800 dark:bg-blue-900/40 dark:text-blue-300">
                              In Transit
                            </Badge>
                          )}
                          {item.status === 'Arrived' && (
                            <Badge className="border-emerald-200 bg-emerald-100 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300">
                              Arrived
                            </Badge>
                          )}
                          {item.status === 'Delayed' && (
                            <Badge className="border-rose-200 bg-rose-100 text-rose-800 dark:border-rose-800 dark:bg-rose-900/40 dark:text-rose-300 flex items-center justify-center gap-1">
                              <AlertTriangle className="h-3 w-3" />
                              Delayed
                            </Badge>
                          )}
                          {item.status === 'Pending' && (
                            <Badge variant="secondary" className="text-slate-600 dark:text-slate-300">
                              Pending
                            </Badge>
                          )}
                        </TableCell>

                        {/* 10. Remarks */}
                        <TableCell className="text-xs text-muted-foreground">
                          {item.remarks ? (
                            <span className="truncate max-w-[180px] inline-block" title={item.remarks}>
                              {item.remarks}
                            </span>
                          ) : (
                            <span className="text-muted-foreground/60">—</span>
                          )}
                        </TableCell>

                        {/* Quick View / Edit Action */}
                        <TableCell className="text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => openInspectDialog(item)}
                            className="h-8 w-8 p-0"
                            title="Inspect & Update Transit Dates"
                          >
                            <Eye className="h-4 w-4 text-muted-foreground hover:text-foreground" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}

          {/* Pagination Footer */}
          {report.items.length > 0 && (
            <div className="flex items-center justify-between p-4 border-t border-border/40">
              <div className="text-xs text-muted-foreground">
                Page {currentPage} of {totalPages}
              </div>
              <div className="flex items-center gap-1.5">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  disabled={currentPage <= 1}
                  className="h-8 text-xs"
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  disabled={currentPage >= totalPages}
                  className="h-8 text-xs"
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Quick Inspect & Update Transit Dates Dialog */}
      <Dialog open={!!inspectItem} onOpenChange={(open) => !open && setInspectItem(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Ship className="h-5 w-5 text-primary" />
              Transit Shipment Details
            </DialogTitle>
            <DialogDescription>
              Inspection and transit date updating for {inspectItem?.invoiceNumber}
            </DialogDescription>
          </DialogHeader>

          {inspectItem && (
            <div className="space-y-4 py-2 text-sm">
              <div className="grid grid-cols-2 gap-3 p-3 rounded-lg bg-muted/40 border border-border/40 text-xs">
                <div>
                  <span className="text-muted-foreground block">Supplier:</span>
                  <span className="font-semibold text-foreground">{inspectItem.supplierName}</span>
                </div>
                <div>
                  <span className="text-muted-foreground block">Invoice No:</span>
                  <span className="font-mono font-semibold text-foreground">{inspectItem.invoiceNumber}</span>
                </div>
                <div>
                  <span className="text-muted-foreground block">Vessel / Voyage:</span>
                  <span className="font-medium text-foreground">{inspectItem.vesselVoyage || '—'}</span>
                </div>
                <div>
                  <span className="text-muted-foreground block">Container:</span>
                  <span className="font-mono text-foreground">{inspectItem.containerNumber || '—'}</span>
                </div>
              </div>

              {/* Editable Dates */}
              <div className="space-y-3">
                <div className="space-y-1">
                  <Label htmlFor="edit-etd" className="text-xs">
                    ETD (Estimated Departure)
                  </Label>
                  <Input
                    id="edit-etd"
                    type="date"
                    value={editEtd}
                    onChange={(e) => setEditEtd(e.target.value)}
                  />
                </div>

                <div className="space-y-1">
                  <Label htmlFor="edit-eta" className="text-xs">
                    ETA (Estimated Arrival)
                  </Label>
                  <Input
                    id="edit-eta"
                    type="date"
                    value={editEta}
                    onChange={(e) => setEditEta(e.target.value)}
                  />
                </div>

                <div className="space-y-1">
                  <Label htmlFor="edit-ata" className="text-xs">
                    ATA / Date of Delivery (Actual Arrival)
                  </Label>
                  <Input
                    id="edit-ata"
                    type="date"
                    value={editAta}
                    onChange={(e) => setEditAta(e.target.value)}
                  />
                  <p className="text-[11px] text-muted-foreground">
                    Entering an arrival date marks the shipment as &quot;Arrived&quot; (Rule 5).
                  </p>
                </div>
              </div>

              {/* Live Preview of Calculated Status & Transit Days */}
              {(() => {
                const preview = computeTransitStatusClient(editEtd, editEta, editAta);
                return (
                  <div className="flex items-center justify-between p-2.5 rounded-md border border-border/60 bg-card">
                    <span className="text-xs text-muted-foreground">Computed Status:</span>
                    <div className="flex items-center gap-2">
                      {preview.transitDays !== undefined && (
                        <span className="text-xs font-mono font-semibold">
                          {preview.transitDays} planned days
                        </span>
                      )}
                      <Badge variant="outline">{preview.status}</Badge>
                    </div>
                  </div>
                );
              })()}
            </div>
          )}

          <DialogFooter className="gap-2 sm:gap-0">
            <Button variant="outline" onClick={() => setInspectItem(null)}>
              Cancel
            </Button>
            <Button onClick={() => void handleSaveDates()} disabled={savingEdit}>
              {savingEdit ? 'Saving...' : 'Save Transit Dates'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default TransitShipmentsPage;
