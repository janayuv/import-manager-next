import * as React from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import {
  BarChart3,
  Box,
  Calculator,
  Calendar,
  CheckCircle2,
  ChevronRight,
  ExternalLink,
  FileSpreadsheet,
  History,
  Info,
  Layers,
  PackageSearch,
  Printer,
  Receipt,
  RefreshCw,
  Search,
  Ship,
  SlidersHorizontal,
  TrendingUp,
  Truck,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import {
  ResponsiveContainer,
  ComposedChart,
  Line,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartsTooltip,
  Legend,
} from 'recharts';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { downloadTextFile } from '@/lib/download';
import { exportPartPeriodAnalysisExcel } from '@/lib/part-analysis-export';
import {
  reportsService,
  type PartPeriodAnalysisResult,
  type PartLookupItem,
  type PartPeriodAnalysisRequest,
} from '@/services/expense-service';
import { supplierService, type Supplier } from '@/services/master-service';
import { formatINR, formatCurr } from '@/lib/format';

export { formatINR, formatCurr };

// Preset date ranges
function getPresetDates(preset: string): { startDate: string; endDate: string } {
  const now = new Date();
  const y = now.getFullYear();
  const m = now.getMonth(); // 0-indexed

  const toISO = (date: Date): string => date.toISOString().split('T')[0] ?? '';

  switch (preset) {
    case 'this_month': {
      const first = new Date(y, m, 1);
      const last = new Date(y, m + 1, 0);
      return { startDate: toISO(first), endDate: toISO(last) };
    }
    case 'last_month': {
      const first = new Date(y, m - 1, 1);
      const last = new Date(y, m, 0);
      return { startDate: toISO(first), endDate: toISO(last) };
    }
    case 'this_quarter': {
      const qStartMonth = Math.floor(m / 3) * 3;
      const first = new Date(y, qStartMonth, 1);
      const last = new Date(y, qStartMonth + 3, 0);
      return { startDate: toISO(first), endDate: toISO(last) };
    }
    case 'this_fy': {
      // Indian Financial Year: Apr 1 to Mar 31
      const fyStartYear = m >= 3 ? y : y - 1;
      return {
        startDate: `${fyStartYear}-04-01`,
        endDate: `${fyStartYear + 1}-03-31`,
      };
    }
    case 'last_fy': {
      const fyStartYear = (m >= 3 ? y : y - 1) - 1;
      return {
        startDate: `${fyStartYear}-04-01`,
        endDate: `${fyStartYear + 1}-03-31`,
      };
    }
    case 'last_30_days': {
      const past = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
      return { startDate: toISO(past), endDate: toISO(now) };
    }
    case 'last_90_days': {
      const past = new Date(now.getTime() - 90 * 24 * 60 * 60 * 1000);
      return { startDate: toISO(past), endDate: toISO(now) };
    }
    case 'last_365_days': {
      const past = new Date(now.getTime() - 365 * 24 * 60 * 60 * 1000);
      return { startDate: toISO(past), endDate: toISO(now) };
    }
    case 'all_time':
    default:
      return { startDate: '', endDate: '' };
  }
}

export function PartPeriodAnalysisPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();

  // Search and filter states
  const [searchQuery, setSearchQuery] = React.useState(searchParams.get('search') || searchParams.get('partNo') || '');
  const [partNumber, setPartNumber] = React.useState(searchParams.get('partNo') || '');
  const [startDate, setStartDate] = React.useState(searchParams.get('startDate') || '');
  const [endDate, setEndDate] = React.useState(searchParams.get('endDate') || '');
  const [dateBasis, setDateBasis] = React.useState(searchParams.get('dateBasis') || 'invoice_date');
  const [supplierId, setSupplierId] = React.useState(searchParams.get('supplierId') || '');
  const [selectedPreset, setSelectedPreset] = React.useState('all_time');

  // Autocomplete / suggestions
  const [suggestions, setSuggestions] = React.useState<PartLookupItem[]>([]);
  const [showSuggestions, setShowSuggestions] = React.useState(false);
  const [popularParts, setPopularParts] = React.useState<PartLookupItem[]>([]);

  // Suppliers list for dropdown
  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);

  // Analysis Result
  const [data, setData] = React.useState<PartPeriodAnalysisResult | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [activeTab, setActiveTab] = React.useState('shipments');

  // Load suppliers and popular parts on mount
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

    void reportsService
      .searchParts('', 15)
      .then((res) => setPopularParts(res ?? []))
      .catch(() => {});
  }, []);

  // Fetch Autocomplete Suggestions
  React.useEffect(() => {
    const q = searchQuery.trim();
    if (!q || q.length < 2) {
      setSuggestions([]);
      return;
    }
    const timer = setTimeout(() => {
      reportsService
        .searchParts(q, 10)
        .then((items) => setSuggestions(items ?? []))
        .catch(() => {});
    }, 200);

    return () => clearTimeout(timer);
  }, [searchQuery]);

  // Main Analysis Query Runner
  const runAnalysis = React.useCallback(
    async (overridePartNo?: string, overrideSearch?: string) => {
      const activePart = overridePartNo !== undefined ? overridePartNo : partNumber;
      const activeSearch = overrideSearch !== undefined ? overrideSearch : searchQuery;

      if (!activePart.trim() && !activeSearch.trim()) {
        toast.info('Please enter a Part Number, Description, or HSN Code to search.');
        return;
      }

      setLoading(true);
      try {
        const req: PartPeriodAnalysisRequest = {
          search: activeSearch.trim(),
          partNumber: activePart.trim(),
          startDate: startDate.trim(),
          endDate: endDate.trim(),
          dateBasis: dateBasis.trim(),
          supplierId: supplierId.trim(),
        };

        const res = await reportsService.getPartPeriodAnalysis(req);
        setData(res);

        if (res.targetItem && res.targetItem.partNumber) {
          setPartNumber(res.targetItem.partNumber);
          // Sync URL search parameters
          const params = new URLSearchParams();
          params.set('partNo', res.targetItem.partNumber);
          if (startDate) params.set('startDate', startDate);
          if (endDate) params.set('endDate', endDate);
          if (dateBasis) params.set('dateBasis', dateBasis);
          if (supplierId) params.set('supplierId', supplierId);
          setSearchParams(params, { replace: true });
        } else {
          toast.warning(`No items found matching "${activeSearch || activePart}".`);
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    },
    [partNumber, searchQuery, startDate, endDate, dateBasis, supplierId, setSearchParams],
  );

  // Auto-run if URL contains initial partNo or search
  React.useEffect(() => {
    const initialPart = searchParams.get('partNo');
    const initialSearch = searchParams.get('search');
    if (initialPart || initialSearch) {
      void runAnalysis(initialPart || undefined, initialSearch || undefined);
    }
  }, []); // Run once on mount

  // Handle Preset Change
  const handlePresetChange = (preset: string) => {
    setSelectedPreset(preset);
    const { startDate: s, endDate: e } = getPresetDates(preset);
    setStartDate(s);
    setEndDate(e);
  };

  // Handle Select Suggestion
  const handleSelectSuggestion = (item: PartLookupItem) => {
    setSearchQuery(item.partNumber);
    setPartNumber(item.partNumber);
    setShowSuggestions(false);
    void runAnalysis(item.partNumber, '');
  };

  // Export CSV
  const handleExportCsv = async () => {
    if (!data?.targetItem?.partNumber) return;
    try {
      const csv = await reportsService.exportPartPeriodAnalysisCsv({
        partNumber: data.targetItem.partNumber,
        search: searchQuery,
        startDate,
        endDate,
        dateBasis,
        supplierId,
      });
      const fname = `${data.targetItem.partNumber}_period_analysis_${startDate || 'all'}_to_${endDate || 'all'}.csv`;
      downloadTextFile(fname, csv);
      toast.success(`Exported comprehensive period analysis CSV: ${fname}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  // Export Excel (.xlsx) using Professional Workbook Template
  const [exportingExcel, setExportingExcel] = React.useState(false);
  const handleExportExcel = async () => {
    if (!data?.targetItem?.partNumber) return;
    setExportingExcel(true);
    try {
      await exportPartPeriodAnalysisExcel(data);
      toast.success(`Exported professional Excel workbook for ${data.targetItem.partNumber}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setExportingExcel(false);
    }
  };

  // Print
  const handlePrint = () => {
    window.print();
  };

  // Status Badge Helper
  const renderStatusBadge = (status: string) => {
    const st = (status || '').toLowerCase();
    if (st === 'delivered' || st === 'reconciled' || st === 'closed' || st === 'finalized') {
      return <Badge variant="success">{status}</Badge>;
    }
    if (st === 'customs-clearance' || st === 'ready-dly' || st === 'awaiting boe data') {
      return <Badge variant="warning">{status}</Badge>;
    }
    if (st === 'discrepancy found' || st === 'investigation') {
      return <Badge variant="destructive">{status}</Badge>;
    }
    return <Badge variant="secondary">{status}</Badge>;
  };

  const item = data?.targetItem;
  const summary = data?.summary;
  const hasData = Boolean(item && item.partNumber);

  return (
    <div className="space-y-6 print:space-y-4">
      {/* Page Title & Actions */}
      <div className="flex flex-wrap items-center justify-between gap-4 border-b pb-4">
        <div>
          <h1 className="flex items-center gap-2.5 text-2xl font-bold tracking-tight text-foreground">
            <PackageSearch className="h-7 w-7 text-primary" />
            Part / Item Period Analysis & Traceability
          </h1>
          <p className="text-sm text-muted-foreground">
            Search any Part Number or Description to inspect all related Shipments, Invoices, Bill of Entry (BOE) duties, and allocated Expenses in the period.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2 print:hidden">
          {hasData && (
            <>
              <Button
                variant="outline"
                size="sm"
                disabled={exportingExcel}
                onClick={() => void handleExportExcel()}
                className="bg-emerald-50 hover:bg-emerald-100 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:hover:bg-emerald-950/70 dark:text-emerald-300 dark:border-emerald-800"
              >
                <FileSpreadsheet className="h-4 w-4 mr-1 text-emerald-600 dark:text-emerald-400" />
                {exportingExcel ? 'Exporting...' : 'Export Excel (.xlsx)'}
              </Button>
              <Button variant="outline" size="sm" onClick={() => void handleExportCsv()}>
                <FileSpreadsheet className="h-4 w-4 mr-1 text-muted-foreground" />
                Export Full CSV
              </Button>
              <Button variant="outline" size="sm" onClick={handlePrint}>
                <Printer className="h-4 w-4 mr-1 text-muted-foreground" />
                Print / Save PDF
              </Button>
            </>
          )}
          <Button
            variant="default"
            size="sm"
            disabled={loading}
            onClick={() => void runAnalysis()}
          >
            <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
            {loading ? 'Analyzing...' : 'Refresh Analysis'}
          </Button>
        </div>
      </div>

      {/* SEARCH & PERIOD FILTER CARD */}
      <Card className="border shadow-sm print:hidden">
        <CardHeader className="pb-3 pt-4">
          <CardTitle className="text-base font-semibold flex items-center gap-2">
            <SlidersHorizontal className="h-4 w-4 text-primary" />
            Search Part & Define Analysis Period
          </CardTitle>
          <CardDescription>
            Search by Part No, Description, or HSN Code. Filter by custom period or quick accounting presets.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-12">
            {/* Search Input with Autocomplete */}
            <div className="relative md:col-span-5">
              <Label className="text-xs font-medium">Search Part No / Description / HSN</Label>
              <div className="relative mt-1">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  className="pl-9 pr-8 font-mono text-sm"
                  placeholder="e.g. BEAR-6204, Ball Bearing, 848210..."
                  value={searchQuery}
                  onChange={(e) => {
                    setSearchQuery(e.target.value);
                    setShowSuggestions(true);
                  }}
                  onFocus={() => setShowSuggestions(true)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      setShowSuggestions(false);
                      void runAnalysis();
                    }
                  }}
                />
                {searchQuery && (
                  <button
                    className="absolute right-2.5 top-2.5 text-muted-foreground hover:text-foreground"
                    onClick={() => {
                      setSearchQuery('');
                      setPartNumber('');
                      setSuggestions([]);
                    }}
                  >
                    <X className="h-4 w-4" />
                  </button>
                )}
              </div>

              {/* Suggestions Dropdown */}
              {showSuggestions && suggestions.length > 0 && (
                <div className="absolute z-50 mt-1 w-full rounded-md border bg-popover text-popover-foreground shadow-lg">
                  <div className="p-1 max-h-60 overflow-y-auto">
                    {suggestions.map((s) => (
                      <div
                        key={s.id}
                        className="flex cursor-pointer items-center justify-between rounded px-3 py-2 text-xs hover:bg-accent hover:text-accent-foreground"
                        onClick={() => handleSelectSuggestion(s)}
                      >
                        <div>
                          <span className="font-mono font-semibold text-primary">{s.partNumber}</span>
                          <span className="ml-2 text-muted-foreground">{s.description}</span>
                          {s.hsnCode && <span className="ml-2 text-[10px] text-muted-foreground">HSN: {s.hsnCode}</span>}
                        </div>
                        <div className="flex items-center gap-2">
                          <span className="text-[11px] text-muted-foreground">{s.supplierName}</span>
                          <Badge variant="secondary" className="text-[10px]">
                            {s.shipmentCount} {s.shipmentCount === 1 ? 'shp' : 'shps'}
                          </Badge>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* Quick Period Presets */}
            <div className="md:col-span-3">
              <Label className="text-xs font-medium">Period Preset</Label>
              <Select value={selectedPreset} onValueChange={handlePresetChange}>
                <SelectTrigger className="mt-1">
                  <SelectValue placeholder="Select preset period" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all_time">All Time</SelectItem>
                  <SelectItem value="this_month">This Month</SelectItem>
                  <SelectItem value="last_month">Last Month</SelectItem>
                  <SelectItem value="this_quarter">This Quarter</SelectItem>
                  <SelectItem value="this_fy">This Financial Year (Apr-Mar)</SelectItem>
                  <SelectItem value="last_fy">Last Financial Year</SelectItem>
                  <SelectItem value="last_30_days">Last 30 Days</SelectItem>
                  <SelectItem value="last_90_days">Last 90 Days</SelectItem>
                  <SelectItem value="last_365_days">Last 365 Days</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Date Basis */}
            <div className="md:col-span-2">
              <Label className="text-xs font-medium">Date Basis</Label>
              <Select value={dateBasis} onValueChange={setDateBasis}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="invoice_date">Invoice Date</SelectItem>
                  <SelectItem value="be_date">BOE / BE Date</SelectItem>
                  <SelectItem value="shipment_date">B/L or Delivery Date</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Supplier Filter */}
            <div className="md:col-span-2">
              <Label className="text-xs font-medium">Supplier (Optional)</Label>
              <Select value={supplierId} onValueChange={setSupplierId}>
                <SelectTrigger className="mt-1">
                  <SelectValue placeholder="All Suppliers" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">All Suppliers</SelectItem>
                  {suppliers.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.supplierName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Date Pickers & Action Buttons */}
          <div className="flex flex-wrap items-center justify-between gap-4 border-t pt-3">
            <div className="flex flex-wrap items-center gap-3">
              <div className="flex items-center gap-1.5">
                <Calendar className="h-4 w-4 text-muted-foreground" />
                <span className="text-xs text-muted-foreground font-medium">From:</span>
                <Input
                  type="date"
                  className="h-8 w-36 text-xs"
                  value={startDate}
                  onChange={(e) => {
                    setStartDate(e.target.value);
                    setSelectedPreset('custom');
                  }}
                />
              </div>

              <div className="flex items-center gap-1.5">
                <span className="text-xs text-muted-foreground font-medium">To:</span>
                <Input
                  type="date"
                  className="h-8 w-36 text-xs"
                  value={endDate}
                  onChange={(e) => {
                    setEndDate(e.target.value);
                    setSelectedPreset('custom');
                  }}
                />
              </div>

              {(startDate || endDate) && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 px-2 text-xs text-muted-foreground"
                  onClick={() => {
                    setStartDate('');
                    setEndDate('');
                    setSelectedPreset('all_time');
                  }}
                >
                  Clear Dates
                </Button>
              )}
            </div>

            <Button
              className="h-8 px-4 text-xs font-semibold"
              onClick={() => {
                setShowSuggestions(false);
                void runAnalysis();
              }}
              disabled={loading}
            >
              <Search className="h-3.5 w-3.5 mr-1.5" />
              Analyze Part Period
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* MULTIPLE MATCHES SELECTION BAR */}
      {data?.matchingParts && data.matchingParts.length > 1 && (
        <div className="rounded-lg border bg-muted/40 p-3 print:hidden">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-semibold text-muted-foreground flex items-center gap-1">
              <Info className="h-3.5 w-3.5 text-primary" />
              Found {data.matchingParts.length} matching parts:
            </span>
            <div className="flex flex-wrap gap-1.5">
              {data.matchingParts.map((mp) => {
                const isSelected = item?.partNumber === mp.partNumber;
                return (
                  <button
                    key={mp.id}
                    className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-mono transition-colors ${
                      isSelected
                        ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
                        : 'bg-background text-foreground border hover:bg-accent'
                    }`}
                    onClick={() => {
                      setPartNumber(mp.partNumber);
                      setSearchQuery(mp.partNumber);
                      void runAnalysis(mp.partNumber, '');
                    }}
                  >
                    <span>{mp.partNumber}</span>
                    <span className="text-[10px] opacity-75">({mp.shipmentCount} shp)</span>
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      )}

      {/* CONTENT REGION: When a Part is Loaded */}
      {hasData && item && summary ? (
        <div className="space-y-6">
          {/* ITEM MASTER TECHNICAL HEADER CARD */}
          <Card className="border bg-card/60 shadow-sm backdrop-blur">
            <CardContent className="p-5">
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div className="space-y-1.5">
                  <div className="flex flex-wrap items-center gap-2.5">
                    <span className="font-mono text-xl font-bold tracking-tight text-primary">
                      {item.partNumber}
                    </span>
                    <Badge variant={item.isActive ? 'success' : 'secondary'}>
                      {item.isActive ? 'Active in Master' : 'Inactive'}
                    </Badge>
                    {item.hsnCode && (
                      <Badge variant="outline" className="font-mono text-xs">
                        HSN: {item.hsnCode}
                      </Badge>
                    )}
                    {item.category && (
                      <Badge variant="secondary" className="text-xs">
                        {item.category}
                      </Badge>
                    )}
                  </div>
                  <h2 className="text-base font-semibold text-foreground">
                    {item.itemDescription}
                  </h2>
                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                    <span>
                      <strong className="text-foreground">Supplier:</strong> {item.supplierName || 'Unlinked'}
                    </span>
                    <span>
                      <strong className="text-foreground">Origin:</strong> {item.countryOfOrigin || 'N/A'}
                    </span>
                    <span>
                      <strong className="text-foreground">Master Unit Price:</strong>{' '}
                      {formatCurr(item.unitPrice, item.currency)} / {item.unit}
                    </span>
                    <span>
                      <strong className="text-foreground">Tariff Rates:</strong> BCD: {item.bcd || '0'}% | SWS: {item.sws || '0'}% | IGST: {item.igst || '0'}%
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-2 print:hidden">
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-xs"
                    onClick={() => navigate('/items')}
                  >
                    <ExternalLink className="h-3.5 w-3.5 mr-1 text-muted-foreground" />
                    Open Item Master
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* EXECUTIVE PERIOD KPI SUMMARY CARDS */}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-4">
            <Card className="p-4 shadow-sm border-l-4 border-l-blue-500">
              <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
                <span>Total Imported Qty</span>
                <Box className="h-4 w-4 text-blue-500" />
              </div>
              <div className="mt-2 text-xl font-bold font-mono">
                {summary.totalQuantity.toLocaleString('en-IN', { maximumFractionDigits: 2 })} {summary.unit}
              </div>
              <div className="mt-1 text-[11px] text-muted-foreground">
                Across {summary.totalShipments} {summary.totalShipments === 1 ? 'shipment' : 'shipments'} ({summary.totalInvoices} inv)
              </div>
            </Card>

            <Card className="p-4 shadow-sm border-l-4 border-l-emerald-500">
              <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
                <span>Total Assessable Value</span>
                <Calculator className="h-4 w-4 text-emerald-500" />
              </div>
              <div className="mt-2 text-xl font-bold font-mono text-emerald-700 dark:text-emerald-400">
                {formatINR(summary.totalAssessableValueInr)}
              </div>
              <div className="mt-1 text-[11px] text-muted-foreground">
                Invoiced: {formatCurr(summary.totalForexValue, summary.currency)}
              </div>
            </Card>

            <Card className="p-4 shadow-sm border-l-4 border-l-amber-500">
              <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
                <span>Total Customs Duties</span>
                <Receipt className="h-4 w-4 text-amber-500" />
              </div>
              <div className="mt-2 text-xl font-bold font-mono text-amber-700 dark:text-amber-400">
                {formatINR(summary.totalCustomsDutyInr)}
              </div>
              <div className="mt-1 text-[11px] text-muted-foreground">
                BCD: {formatINR(summary.totalBcdAmount)} | IGST: {formatINR(summary.totalIgstAmount)}
              </div>
            </Card>

            <Card className="p-4 shadow-sm border-l-4 border-l-indigo-500">
              <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
                <span>Allocated Expenses</span>
                <Truck className="h-4 w-4 text-indigo-500" />
              </div>
              <div className="mt-2 text-xl font-bold font-mono text-indigo-700 dark:text-indigo-400">
                {formatINR(summary.totalAllocatedExpensesInr)}
              </div>
              <div className="mt-1 text-[11px] text-muted-foreground">
                With GST: {formatINR(summary.totalAllocatedExpensesWithGstInr)}
              </div>
            </Card>

            <Card className="p-4 shadow-sm border-l-4 border-l-purple-500">
              <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
                <span>Total Landed Cost (Ex. IGST)</span>
                <PackageSearch className="h-4 w-4 text-purple-500" />
              </div>
              <div className="mt-2 text-xl font-bold font-mono text-purple-700 dark:text-purple-400">
                {formatINR(summary.totalLandedCostExIgst)}
              </div>
              <div className="mt-1 text-[11px] text-muted-foreground">
                Assessable + BCD + SWS + Interest + Exp
              </div>
            </Card>

            <Card className="p-4 shadow-sm border-l-4 border-l-rose-500">
              <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
                <span>Avg Landed Cost / Qty</span>
                <TrendingUp className="h-4 w-4 text-rose-500" />
              </div>
              <div className="mt-2 text-xl font-bold font-mono text-rose-700 dark:text-rose-400">
                {formatINR(summary.avgLandedCostPerUnit)} / {summary.unit}
              </div>
              <div className="mt-1 text-[11px] text-muted-foreground">
                Range: {formatINR(summary.minLandedCostPerUnit)} – {formatINR(summary.maxLandedCostPerUnit)}
              </div>
            </Card>

            <Card className="p-4 shadow-sm border-l-4 border-l-teal-500">
              <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
                <span>Duty Paid Status</span>
                <CheckCircle2 className="h-4 w-4 text-teal-500" />
              </div>
              <div className="mt-2 text-xl font-bold font-mono text-teal-700 dark:text-teal-400">
                {formatINR(summary.totalDutyPaidInr)}
              </div>
              <div className="mt-1 text-[11px] text-muted-foreground">
                Across {summary.totalBoes} {summary.totalBoes === 1 ? 'BOE' : 'BOEs'} registered
              </div>
            </Card>

            <Card className="p-4 shadow-sm border-l-4 border-l-sky-500">
              <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
                <span>Activity Timeline</span>
                <History className="h-4 w-4 text-sky-500" />
              </div>
              <div className="mt-2 text-sm font-semibold">
                {summary.firstImportDate || 'N/A'}
              </div>
              <div className="mt-1 text-[11px] text-muted-foreground">
                Latest: {summary.latestImportDate || 'N/A'}
              </div>
            </Card>
          </div>

          {/* TABBED DEEP DIVE SECTION */}
          <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
            <TabsList className="grid w-full grid-cols-2 md:grid-cols-5 h-auto p-1 bg-muted/60">
              <TabsTrigger value="shipments" className="py-2 text-xs font-medium flex items-center gap-1.5">
                <Ship className="h-3.5 w-3.5" />
                Shipments & Invoices ({data.shipments.length})
              </TabsTrigger>
              <TabsTrigger value="boe" className="py-2 text-xs font-medium flex items-center gap-1.5">
                <FileSpreadsheet className="h-3.5 w-3.5" />
                Bill of Entry (BOE) ({data.boeLines.length})
              </TabsTrigger>
              <TabsTrigger value="expenses" className="py-2 text-xs font-medium flex items-center gap-1.5">
                <Truck className="h-3.5 w-3.5" />
                Allocated Expenses ({data.expenses.length})
              </TabsTrigger>
              <TabsTrigger value="trends" className="py-2 text-xs font-medium flex items-center gap-1.5">
                <BarChart3 className="h-3.5 w-3.5" />
                Cost Trends & History
              </TabsTrigger>
              <TabsTrigger value="specs" className="py-2 text-xs font-medium flex items-center gap-1.5">
                <Layers className="h-3.5 w-3.5" />
                Technical Specifications
              </TabsTrigger>
            </TabsList>

            {/* TAB 1: SHIPMENTS & INVOICES */}
            <TabsContent value="shipments">
              <Card>
                <CardHeader className="py-3 px-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <CardTitle className="text-sm font-semibold">Related Shipments & Commercial Invoices</CardTitle>
                      <CardDescription className="text-xs">
                        All commercial shipments importing {item.partNumber} during the specified period.
                      </CardDescription>
                    </div>
                    <Badge variant="outline" className="text-xs">
                      {data.shipments.length} Records
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent className="p-0">
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-[110px]">Shipment ID</TableHead>
                          <TableHead>Invoice No & Date</TableHead>
                          <TableHead>Supplier</TableHead>
                          <TableHead>Incoterm & Mode</TableHead>
                          <TableHead>BL/AWB / Vessel</TableHead>
                          <TableHead>Status</TableHead>
                          <TableHead className="text-right">Quantity</TableHead>
                          <TableHead className="text-right">Unit Price</TableHead>
                          <TableHead className="text-right">Line Total</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {data.shipments.length === 0 ? (
                          <TableRow>
                            <TableCell colSpan={9} className="h-24 text-center text-muted-foreground text-xs">
                              No shipments found for this part in the selected period.
                            </TableCell>
                          </TableRow>
                        ) : (
                          data.shipments.map((sh, idx) => (
                            <TableRow key={`${sh.shipmentId}-${sh.invoiceNumber}-${idx}`}>
                              <TableCell className="font-mono text-xs font-semibold text-primary">
                                {sh.shipmentId}
                              </TableCell>
                              <TableCell>
                                <div className="font-medium text-xs">{sh.invoiceNumber}</div>
                                <div className="text-[11px] text-muted-foreground">{sh.invoiceDate}</div>
                              </TableCell>
                              <TableCell className="text-xs">{sh.supplierName || '—'}</TableCell>
                              <TableCell className="text-xs">
                                <span className="font-semibold">{sh.incoterm || '—'}</span>{' '}
                                <span className="text-muted-foreground">({sh.shipmentMode || 'Sea'})</span>
                              </TableCell>
                              <TableCell className="text-xs">
                                <div>{sh.blAwbNumber || '—'}</div>
                                {sh.vesselName && (
                                  <div className="text-[11px] text-muted-foreground">{sh.vesselName}</div>
                                )}
                              </TableCell>
                              <TableCell>{renderStatusBadge(sh.status)}</TableCell>
                              <TableCell className="text-right font-mono text-xs font-semibold">
                                {sh.quantity.toLocaleString('en-IN')} {sh.unit}
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs">
                                <div>{formatCurr(sh.unitPrice, sh.currency)}</div>
                                {sh.priceDiverged && (
                                  <div className="text-[10px] text-amber-600 dark:text-amber-400">
                                    vs Master {formatCurr(sh.masterUnitPrice, sh.currency)}
                                  </div>
                                )}
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs font-bold">
                                {formatCurr(sh.lineTotal, sh.currency)}
                              </TableCell>
                            </TableRow>
                          ))
                        )}
                      </TableBody>
                    </Table>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            {/* TAB 2: BILL OF ENTRY (BOE) */}
            <TabsContent value="boe">
              <Card>
                <CardHeader className="py-3 px-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <CardTitle className="text-sm font-semibold">Bill of Entry (BOE) Customs Duty Valuation</CardTitle>
                      <CardDescription className="text-xs">
                        Customs assessment values, statutory duties (BCD, SWS, IGST), and duty payments for {item.partNumber}.
                      </CardDescription>
                    </div>
                    <Badge variant="outline" className="text-xs">
                      {data.boeLines.length} Entries
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent className="p-0">
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Shipment / Inv</TableHead>
                          <TableHead>BE No & Date</TableHead>
                          <TableHead>Port / Location</TableHead>
                          <TableHead>Recon Status</TableHead>
                          <TableHead className="text-right">Assessable Value</TableHead>
                          <TableHead className="text-right">BCD Amount</TableHead>
                          <TableHead className="text-right">SWS Amount</TableHead>
                          <TableHead className="text-right">IGST Amount</TableHead>
                          <TableHead className="text-right">BOE Interest</TableHead>
                          <TableHead className="text-right">Total Duty</TableHead>
                          <TableHead className="text-right">Challan / Paid</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {data.boeLines.length === 0 ? (
                          <TableRow>
                            <TableCell colSpan={11} className="h-24 text-center text-muted-foreground text-xs">
                              No BOE calculation entries found for this part in the selected period.
                            </TableCell>
                          </TableRow>
                        ) : (
                          data.boeLines.map((b, idx) => (
                            <TableRow key={`${b.boeCalcId}-${idx}`}>
                              <TableCell className="text-xs">
                                <div className="font-mono font-semibold text-primary">{b.shipmentId}</div>
                                <div className="text-[11px] text-muted-foreground">{b.invoiceNumber}</div>
                              </TableCell>
                              <TableCell className="text-xs">
                                <div className="font-mono font-bold">{b.beNumber || 'Awaiting BE'}</div>
                                <div className="text-[11px] text-muted-foreground">{b.beDate || '—'}</div>
                              </TableCell>
                              <TableCell className="text-xs font-mono">{b.location || '—'}</TableCell>
                              <TableCell>{renderStatusBadge(b.status)}</TableCell>
                              <TableCell className="text-right font-mono text-xs font-semibold text-emerald-700 dark:text-emerald-400">
                                {formatINR(b.assessableValue)}
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs">
                                <div>{formatINR(b.bcdAmount)}</div>
                                <div className="text-[10px] text-muted-foreground">@{b.bcdRate}%</div>
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs">
                                <div>{formatINR(b.swsAmount)}</div>
                                <div className="text-[10px] text-muted-foreground">@{b.swsRate}%</div>
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs">
                                <div>{formatINR(b.igstAmount)}</div>
                                <div className="text-[10px] text-muted-foreground">@{b.igstRate}%</div>
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs text-amber-600 dark:text-amber-400">
                                {formatINR(b.boeInterest)}
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs font-bold text-amber-700 dark:text-amber-300">
                                {formatINR(b.totalDuty)}
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs">
                                <div className="font-semibold text-teal-600 dark:text-teal-400">
                                  {formatINR(b.dutyPaid)}
                                </div>
                                <div className="text-[10px] text-muted-foreground">{b.challanNumber || 'No Challan'}</div>
                              </TableCell>
                            </TableRow>
                          ))
                        )}
                      </TableBody>
                    </Table>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            {/* TAB 3: ALLOCATED EXPENSES */}
            <TabsContent value="expenses">
              <Card>
                <CardHeader className="py-3 px-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <CardTitle className="text-sm font-semibold">Proportionally Allocated Shipment Expenses</CardTitle>
                      <CardDescription className="text-xs">
                        Shipment expenses (Freight, CHA, Port/CFS, Transport, Handling) allocated based on assessable ratio.
                      </CardDescription>
                    </div>
                    <Badge variant="outline" className="text-xs">
                      {data.expenses.length} Expense Lines
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent className="p-0">
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Shipment ID</TableHead>
                          <TableHead>Service Provider</TableHead>
                          <TableHead>Expense Type</TableHead>
                          <TableHead>Invoice No & Date</TableHead>
                          <TableHead className="text-right">Shipment Total</TableHead>
                          <TableHead className="text-right">Part Allocation %</TableHead>
                          <TableHead className="text-right">Allocated Basic</TableHead>
                          <TableHead className="text-right">Allocated GST</TableHead>
                          <TableHead className="text-right">Allocated Total</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {data.expenses.length === 0 ? (
                          <TableRow>
                            <TableCell colSpan={9} className="h-24 text-center text-muted-foreground text-xs">
                              No shipment expenses recorded for the shipments of this part in the period.
                            </TableCell>
                          </TableRow>
                        ) : (
                          data.expenses.map((ex, idx) => (
                            <TableRow key={`${ex.expenseId}-${idx}`}>
                              <TableCell className="font-mono text-xs font-semibold text-primary">
                                {ex.shipmentId}
                              </TableCell>
                              <TableCell className="text-xs font-medium">{ex.serviceProviderName}</TableCell>
                              <TableCell>
                                <Badge variant="outline" className="text-xs">
                                  {ex.expenseTypeName}
                                </Badge>
                              </TableCell>
                              <TableCell className="text-xs">
                                <div className="font-mono">{ex.expenseInvoiceNo || '—'}</div>
                                <div className="text-[11px] text-muted-foreground">{ex.expenseInvoiceDate || '—'}</div>
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs text-muted-foreground">
                                {formatINR(ex.shipmentExpenseTotalInr)}
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs font-semibold">
                                {(ex.allocationRatio * 100).toFixed(1)}%
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs font-semibold text-indigo-600 dark:text-indigo-400">
                                {formatINR(ex.allocatedBasicInr)}
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs text-muted-foreground">
                                {formatINR(ex.allocatedGstInr)}
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs font-bold text-indigo-700 dark:text-indigo-300">
                                {formatINR(ex.allocatedTotalInr)}
                              </TableCell>
                            </TableRow>
                          ))
                        )}
                      </TableBody>
                    </Table>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            {/* TAB 4: COST TRENDS & HISTORY */}
            <TabsContent value="trends" className="space-y-4">
              {/* Recharts Timeline Chart */}
              {data.trends.length > 0 && (
                <Card>
                  <CardHeader className="py-3 px-4">
                    <CardTitle className="text-sm font-semibold flex items-center gap-2">
                      <TrendingUp className="h-4 w-4 text-primary" />
                      Landed Cost Per Unit Components Over Time
                    </CardTitle>
                    <CardDescription className="text-xs">
                      Chronological progression of Assessable Value, Duty, and Allocated Expenses per unit across shipments.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="h-72 pt-2">
                    <ResponsiveContainer width="100%" height="100%">
                      <ComposedChart data={data.trends} margin={{ top: 10, right: 20, bottom: 20, left: 10 }}>
                        <CartesianGrid strokeDasharray="3 3" opacity={0.3} />
                        <XAxis dataKey="date" tick={{ fontSize: 11 }} />
                        <YAxis tick={{ fontSize: 11 }} tickFormatter={(val) => `₹${val}`} />
                        <RechartsTooltip
                          formatter={(value: any, name: any) => [formatINR(Number(value)), name]}
                          labelFormatter={(label) => `Shipment Date: ${label}`}
                        />
                        <Legend wrapperStyle={{ fontSize: 12, paddingTop: 10 }} />
                        <Bar dataKey="assessableValuePerUnit" name="Assessable Value / Qty" fill="#10b981" stackId="a" />
                        <Bar dataKey="dutyPerUnit" name="Duty / Qty" fill="#f59e0b" stackId="a" />
                        <Bar dataKey="expensePerUnit" name="Expense / Qty" fill="#6366f1" stackId="a" />
                        <Line
                          type="monotone"
                          dataKey="landedCostPerUnit"
                          name="Total Landed Cost / Qty"
                          stroke="#ef4444"
                          strokeWidth={2}
                          dot={{ r: 4 }}
                        />
                      </ComposedChart>
                    </ResponsiveContainer>
                  </CardContent>
                </Card>
              )}

              {/* Chronological Trends Table */}
              <Card>
                <CardHeader className="py-3 px-4">
                  <CardTitle className="text-sm font-semibold">Shipment Cost History & Breakdown</CardTitle>
                  <CardDescription className="text-xs">
                    Per-unit landed cost analysis for each commercial shipment.
                  </CardDescription>
                </CardHeader>
                <CardContent className="p-0">
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Date</TableHead>
                          <TableHead>Shipment ID</TableHead>
                          <TableHead>Invoice No</TableHead>
                          <TableHead className="text-right">Quantity</TableHead>
                          <TableHead className="text-right">Forex Unit Price</TableHead>
                          <TableHead className="text-right">Assessable / Unit</TableHead>
                          <TableHead className="text-right">Duty / Unit</TableHead>
                          <TableHead className="text-right">Expense / Unit</TableHead>
                          <TableHead className="text-right">Landed Cost / Unit</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {data.trends.map((tr, idx) => (
                          <TableRow key={`${tr.shipmentId}-${idx}`}>
                            <TableCell className="text-xs font-medium">{tr.date || '—'}</TableCell>
                            <TableCell className="font-mono text-xs font-semibold text-primary">
                              {tr.shipmentId}
                            </TableCell>
                            <TableCell className="text-xs font-mono">{tr.invoiceNumber}</TableCell>
                            <TableCell className="text-right font-mono text-xs">
                              {tr.quantity.toLocaleString('en-IN')} {summary.unit}
                            </TableCell>
                            <TableCell className="text-right font-mono text-xs">
                              {formatCurr(tr.unitPriceForex, tr.currency)}
                            </TableCell>
                            <TableCell className="text-right font-mono text-xs text-emerald-600 dark:text-emerald-400">
                              {formatINR(tr.assessableValuePerUnit)}
                            </TableCell>
                            <TableCell className="text-right font-mono text-xs text-amber-600 dark:text-amber-400">
                              {formatINR(tr.dutyPerUnit)}
                            </TableCell>
                            <TableCell className="text-right font-mono text-xs text-indigo-600 dark:text-indigo-400">
                              {formatINR(tr.expensePerUnit)}
                            </TableCell>
                            <TableCell className="text-right font-mono text-xs font-bold text-rose-600 dark:text-rose-400">
                              {formatINR(tr.landedCostPerUnit)}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            {/* TAB 5: TECHNICAL SPECIFICATIONS */}
            <TabsContent value="specs">
              <Card>
                <CardHeader className="py-3 px-4">
                  <CardTitle className="text-sm font-semibold">Item Master Profile & Catalog Specifications</CardTitle>
                  <CardDescription className="text-xs">
                    Comprehensive master data parameters for {item.partNumber}.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4 pt-2">
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 text-xs">
                    <div className="rounded-md border p-3 space-y-1">
                      <div className="text-muted-foreground font-medium">Part Identification</div>
                      <div className="font-mono font-bold text-sm text-primary">{item.partNumber}</div>
                      <div className="text-muted-foreground">ID: {item.id}</div>
                      <div>HSN / ITC-HS: <strong className="font-mono">{item.hsnCode || '—'}</strong></div>
                      <div>Category: <strong>{item.category || '—'}</strong></div>
                      <div>End Use: <strong>{item.endUse || '—'}</strong></div>
                    </div>

                    <div className="rounded-md border p-3 space-y-1">
                      <div className="text-muted-foreground font-medium">Pricing & Supplier</div>
                      <div className="font-bold text-sm">{item.supplierName || 'Unlinked'}</div>
                      <div>Supplier ID: <span className="font-mono">{item.supplierId || '—'}</span></div>
                      <div>Country of Origin: <strong>{item.countryOfOrigin || '—'}</strong></div>
                      <div>Catalog Price: <strong className="font-mono">{formatCurr(item.unitPrice, item.currency)}</strong> / {item.unit}</div>
                      <div>Purchase UOM: <strong>{item.purchaseUom || item.unit}</strong></div>
                    </div>

                    <div className="rounded-md border p-3 space-y-1">
                      <div className="text-muted-foreground font-medium">Customs Duty Rates</div>
                      <div>Basic Customs Duty (BCD): <strong className="font-mono">{item.bcd || '0'}%</strong></div>
                      <div>Social Welfare Surcharge (SWS): <strong className="font-mono">{item.sws || '0'}%</strong></div>
                      <div>Integrated GST (IGST): <strong className="font-mono">{item.igst || '0'}%</strong></div>
                      <div className="pt-1 text-muted-foreground font-medium">Weights</div>
                      <div>Net Weight: <strong className="font-mono">{item.netWeightKg ? `${item.netWeightKg} kg` : '—'}</strong></div>
                      <div>Gross Weight / UOM: <strong className="font-mono">{item.grossWeightPerUomKg ? `${item.grossWeightPerUomKg} kg` : '—'}</strong></div>
                    </div>
                  </div>

                  {item.technicalWriteUp && (
                    <div className="rounded-md border p-3 space-y-1 text-xs">
                      <div className="text-muted-foreground font-medium">Technical Write-up / Specifications</div>
                      <p className="whitespace-pre-wrap text-foreground">{item.technicalWriteUp}</p>
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      ) : (
        /* WELCOME / DISCOVERY STATE */
        <Card className="border-dashed py-12 text-center">
          <CardContent className="space-y-4 max-w-lg mx-auto">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-primary/10 text-primary">
              <PackageSearch className="h-7 w-7" />
            </div>
            <div className="space-y-1.5">
              <h2 className="text-lg font-bold">Search any Part Number or Description</h2>
              <p className="text-xs text-muted-foreground">
                Enter a part number, description keyword, or HSN code above to explore all related shipments, commercial invoices, bill of entry duties, and allocated freight/clearance expenses in any period.
              </p>
            </div>

            {popularParts.length > 0 && (
              <div className="pt-4 text-left">
                <div className="text-xs font-semibold text-muted-foreground mb-2 flex items-center gap-1">
                  <History className="h-3.5 w-3.5 text-primary" />
                  Quick Explore Available Parts ({popularParts.length}):
                </div>
                <div className="flex flex-wrap gap-2 max-h-48 overflow-y-auto p-1">
                  {popularParts.map((p) => (
                    <button
                      key={p.id}
                      className="group flex items-center gap-2 rounded-lg border bg-card px-3 py-1.5 text-xs hover:border-primary hover:bg-accent/50 transition-all text-left"
                      onClick={() => handleSelectSuggestion(p)}
                    >
                      <div>
                        <div className="font-mono font-semibold text-primary group-hover:underline">
                          {p.partNumber}
                        </div>
                        <div className="text-[10px] text-muted-foreground truncate max-w-[150px]">
                          {p.description}
                        </div>
                      </div>
                      <Badge variant="secondary" className="text-[10px] ml-1">
                        {p.shipmentCount} {p.shipmentCount === 1 ? 'shp' : 'shps'}
                      </Badge>
                      <ChevronRight className="h-3.5 w-3.5 text-muted-foreground opacity-50 group-hover:opacity-100" />
                    </button>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

export default PartPeriodAnalysisPage;
