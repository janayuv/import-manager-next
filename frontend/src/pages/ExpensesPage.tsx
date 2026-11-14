import * as React from 'react';
import {
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  ChevronsDownUp,
  FileSpreadsheet,
  Layers,
  Merge,
  Pencil,
  Plus,
  Receipt,
  RefreshCw,
  Search,
  Settings2,
  Trash2,
  Upload,
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
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { CsvImportDialog } from '@/components/master/CsvImportDialog';
import { ManageExpenseTypesDialog } from '@/components/master/ManageExpenseTypesDialog';
import { ManageServiceProvidersDialog } from '@/components/master/ManageServiceProvidersDialog';
import { useAuth } from '@/contexts/AuthContext';
import { downloadTextFile } from '@/lib/download';
import {
  expenseService,
  formatPaiseINR,
  type ExpenseInvoice,
  type ExpenseInvoiceCalculation,
  type ExpenseLineInput,
  type ExpenseType,
  type ExpenseTypeInput,
  type ServiceProvider,
  type ServiceProviderInput,
} from '@/services/expense-service';
import { shipmentService, type Shipment } from '@/services/shipment-invoice-service';

interface DraftLine {
  expenseTypeId: string;
  amountInr: string;
  cgstPct: string;
  sgstPct: string;
  igstPct: string;
  tdsPct: string;
  remarks: string;
}

const emptyLine = (et?: ExpenseType): DraftLine => ({
  expenseTypeId: et?.id ?? '',
  amountInr: '0.00',
  cgstPct: et ? (et.defaultCgstRateBp / 100).toFixed(2) : '0',
  sgstPct: et ? (et.defaultSgstRateBp / 100).toFixed(2) : '0',
  igstPct: et ? (et.defaultIgstRateBp / 100).toFixed(2) : '0',
  tdsPct: et ? (et.defaultTdsRateBp / 100).toFixed(2) : '0',
  remarks: '',
});

function toLineInputs(lines: DraftLine[]): ExpenseLineInput[] {
  return lines.map((l) => ({
    expenseTypeId: l.expenseTypeId,
    amountPaise: Math.round((Number.parseFloat(l.amountInr) || 0) * 100),
    cgstRateBp: Math.round((Number.parseFloat(l.cgstPct) || 0) * 100),
    sgstRateBp: Math.round((Number.parseFloat(l.sgstPct) || 0) * 100),
    igstRateBp: Math.round((Number.parseFloat(l.igstPct) || 0) * 100),
    tdsRateBp: Math.round((Number.parseFloat(l.tdsPct) || 0) * 100),
    remarks: l.remarks,
  }));
}

export function ExpensesPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');
  const canDelete = hasPermission('data.delete');

  const [invoices, setInvoices] = React.useState<ExpenseInvoice[]>([]);
  const [providers, setProviders] = React.useState<ServiceProvider[]>([]);
  const [types, setTypes] = React.useState<ExpenseType[]>([]);
  const [shipments, setShipments] = React.useState<Shipment[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [search, setSearch] = React.useState('');
  const [shipmentFilter, setShipmentFilter] = React.useState('');

  // Multi-line Expense Invoice dialog state
  const [invOpen, setInvOpen] = React.useState(false);
  const [editingInv, setEditingInv] = React.useState<ExpenseInvoice | null>(null);
  const [shipmentId, setShipmentId] = React.useState('');
  const [serviceProviderId, setServiceProviderId] = React.useState('');
  const [invoiceNo, setInvoiceNo] = React.useState('');
  const [invoiceDate, setInvoiceDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [remarks, setRemarks] = React.useState('');
  const [idempotencyKey, setIdempotencyKey] = React.useState('');
  const [draftLines, setDraftLines] = React.useState<DraftLine[]>([]);
  const [preview, setPreview] = React.useState<ExpenseInvoiceCalculation | null>(null);
  const [savingInv, setSavingInv] = React.useState(false);

  // Bulk import dialog
  const [bulkOpen, setBulkOpen] = React.useState(false);
  const [bulkShipmentId, setBulkShipmentId] = React.useState('');

  // Service Provider dialog
  const [spOpen, setSpOpen] = React.useState(false);
  const [editingSp, setEditingSp] = React.useState<ServiceProvider | null>(null);
  const [spForm, setSpForm] = React.useState<ServiceProviderInput>({
    name: '',
    gstin: '',
    state: '',
    contactPerson: '',
    contactEmail: '',
    contactPhone: '',
  });

  // Expense Type dialog
  const [etOpen, setEtOpen] = React.useState(false);
  const [editingEt, setEditingEt] = React.useState<ExpenseType | null>(null);
  const [etForm, setEtForm] = React.useState<ExpenseTypeInput>({
    name: '',
    defaultCgstRateBp: 0,
    defaultSgstRateBp: 0,
    defaultIgstRateBp: 0,
    defaultTdsRateBp: 0,
    isActive: true,
  });

  // Dedicated Manage Dialogs
  const [manageSpOpen, setManageSpOpen] = React.useState(false);
  const [manageEtOpen, setManageEtOpen] = React.useState(false);
  const [targetLineIdx, setTargetLineIdx] = React.useState<number | null>(null);

  // Tab state
  const [activeTab, setActiveTab] = React.useState('invoices');

  // Expandable invoice rows in Ledger
  const [expandedInvoices, setExpandedInvoices] = React.useState<Set<string>>(new Set());

  // Itemized Ledger Filter states
  const [itemizedSearch, setItemizedSearch] = React.useState('');
  const [itemizedTypeFilter, setItemizedTypeFilter] = React.useState('all');
  const [itemizedProviderFilter, setItemizedProviderFilter] = React.useState('all');
  const [itemizedShipmentFilter, setItemizedShipmentFilter] = React.useState('all');

  const loadAll = React.useCallback(async () => {
    setLoading(true);
    try {
      const [invs, sps, ets, shPage] = await Promise.all([
        expenseService.listExpenseInvoices(shipmentFilter, search),
        expenseService.listServiceProviders(''),
        expenseService.listExpenseTypes(false),
        shipmentService.list({
          search: '',
          supplierId: '',
          status: '',
          frozenOnly: false,
          sortBy: 'invoiceDate',
          sortDir: 'desc',
          page: 1,
          pageSize: 500,
        }),
      ]);
      setInvoices(invs);
      setProviders(sps);
      setTypes(ets);
      setShipments(shPage.items ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search, shipmentFilter]);

  React.useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const refreshPreview = React.useCallback(async (lines: DraftLine[]) => {
    const inputs = toLineInputs(lines).filter((l) => l.expenseTypeId && l.amountPaise > 0);
    if (inputs.length === 0) {
      setPreview(null);
      return;
    }
    try {
      const p = await expenseService.previewCalculation(inputs);
      setPreview(p);
    } catch {
      setPreview(null);
    }
  }, []);

  const hasDuplicateType = React.useMemo(() => {
    const ids = draftLines.map((l) => l.expenseTypeId).filter(Boolean);
    return new Set(ids).size !== ids.length;
  }, [draftLines]);

  const toggleInvoiceExpand = (id: string) => {
    setExpandedInvoices((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const allInvoicesExpanded =
    invoices.length > 0 && invoices.every((inv) => expandedInvoices.has(inv.id));

  const toggleAllInvoicesExpand = () => {
    if (allInvoicesExpanded) {
      setExpandedInvoices(new Set());
    } else {
      setExpandedInvoices(new Set(invoices.map((inv) => inv.id)));
    }
  };

  const allItemizedLines = React.useMemo(() => {
    const list: Array<{
      lineId: string;
      invoiceId: string;
      invoiceNo: string;
      invoiceDate: string;
      shipmentId: string;
      shipmentInvoiceNo: string;
      serviceProviderId: string;
      serviceProviderName: string;
      expenseTypeId: string;
      expenseTypeName: string;
      amountPaise: number;
      cgstRateBp: number;
      sgstRateBp: number;
      igstRateBp: number;
      tdsRateBp: number;
      cgstPaise: number;
      sgstPaise: number;
      igstPaise: number;
      tdsPaise: number;
      totalPaise: number;
      netPaise: number;
      remarks: string;
      parentInvoice: ExpenseInvoice;
    }> = [];

    for (const inv of invoices) {
      if (Array.isArray(inv.lines) && inv.lines.length > 0) {
        for (const line of inv.lines) {
          list.push({
            lineId: line.id || `${inv.id}-${line.expenseTypeId}`,
            invoiceId: inv.id,
            invoiceNo: inv.invoiceNo,
            invoiceDate: inv.invoiceDate,
            shipmentId: inv.shipmentId,
            shipmentInvoiceNo: inv.shipmentInvoiceNo,
            serviceProviderId: inv.serviceProviderId,
            serviceProviderName: inv.serviceProviderName,
            expenseTypeId: line.expenseTypeId,
            expenseTypeName: line.expenseTypeName || 'Expense Item',
            amountPaise: line.amountPaise,
            cgstRateBp: line.cgstRateBp,
            sgstRateBp: line.sgstRateBp,
            igstRateBp: line.igstRateBp,
            tdsRateBp: line.tdsRateBp,
            cgstPaise: line.cgstPaise,
            sgstPaise: line.sgstPaise,
            igstPaise: line.igstPaise,
            tdsPaise: line.tdsPaise,
            totalPaise: line.totalPaise,
            netPaise: line.netPaise,
            remarks: line.remarks || '',
            parentInvoice: inv,
          });
        }
      }
    }
    return list;
  }, [invoices]);

  const filteredItemizedLines = React.useMemo(() => {
    return allItemizedLines.filter((l) => {
      if (itemizedTypeFilter !== 'all' && l.expenseTypeId !== itemizedTypeFilter) {
        return false;
      }
      if (itemizedProviderFilter !== 'all' && l.serviceProviderId !== itemizedProviderFilter) {
        return false;
      }
      if (itemizedShipmentFilter !== 'all' && l.shipmentId !== itemizedShipmentFilter) {
        return false;
      }
      if (itemizedSearch.trim()) {
        const query = itemizedSearch.toLowerCase().trim();
        const matches =
          l.invoiceNo.toLowerCase().includes(query) ||
          l.serviceProviderName.toLowerCase().includes(query) ||
          l.expenseTypeName.toLowerCase().includes(query) ||
          l.shipmentInvoiceNo.toLowerCase().includes(query) ||
          l.remarks.toLowerCase().includes(query);
        if (!matches) return false;
      }
      return true;
    });
  }, [allItemizedLines, itemizedTypeFilter, itemizedProviderFilter, itemizedShipmentFilter, itemizedSearch]);

  const itemizedSummary = React.useMemo(() => {
    let basic = 0;
    let cgst = 0;
    let sgst = 0;
    let igst = 0;
    let tds = 0;
    let net = 0;
    for (const l of filteredItemizedLines) {
      basic += l.amountPaise;
      cgst += l.cgstPaise;
      sgst += l.sgstPaise;
      igst += l.igstPaise;
      tds += l.tdsPaise;
      net += l.netPaise;
    }
    return {
      count: filteredItemizedLines.length,
      basicPaise: basic,
      gstPaise: cgst + sgst + igst,
      tdsPaise: tds,
      netPaise: net,
    };
  }, [filteredItemizedLines]);

  const exportItemizedCsv = () => {
    const headers = [
      'Invoice Date',
      'Invoice No',
      'Service Provider',
      'Shipment Invoice',
      'Expense Type',
      'Basic (INR)',
      'CGST %',
      'CGST (INR)',
      'SGST %',
      'SGST (INR)',
      'IGST %',
      'IGST (INR)',
      'TDS %',
      'TDS (INR)',
      'Net Payable (INR)',
      'Remarks',
    ];
    const rows = filteredItemizedLines.map((l) => [
      l.invoiceDate,
      `"${l.invoiceNo.replaceAll('"', '""')}"`,
      `"${l.serviceProviderName.replaceAll('"', '""')}"`,
      `"${l.shipmentInvoiceNo.replaceAll('"', '""')}"`,
      `"${l.expenseTypeName.replaceAll('"', '""')}"`,
      (l.amountPaise / 100).toFixed(2),
      (l.cgstRateBp / 100).toFixed(2),
      (l.cgstPaise / 100).toFixed(2),
      (l.sgstRateBp / 100).toFixed(2),
      (l.sgstPaise / 100).toFixed(2),
      (l.igstRateBp / 100).toFixed(2),
      (l.igstPaise / 100).toFixed(2),
      (l.tdsRateBp / 100).toFixed(2),
      (l.tdsPaise / 100).toFixed(2),
      (l.netPaise / 100).toFixed(2),
      `"${(l.remarks || '').replaceAll('"', '""')}"`,
    ]);
    downloadTextFile(
      `expenses_itemized_ledger_${new Date().toISOString().slice(0, 10)}.csv`,
      [headers.join(','), ...rows.map((r) => r.join(','))].join('\r\n'),
    );
  };

  const openCreateInvoice = () => {
    setEditingInv(null);
    const activeSh = shipments.find((s) => !s.isFrozen);
    setShipmentId(activeSh?.id ?? '');
    setServiceProviderId(providers[0]?.id ?? '');
    setInvoiceNo('');
    setInvoiceDate(new Date().toISOString().slice(0, 10));
    setRemarks('');
    setIdempotencyKey(`exp-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`);
    const initial = [emptyLine(types[0])];
    setDraftLines(initial);
    setPreview(null);
    setInvOpen(true);
  };

  const openEditInvoice = (inv: ExpenseInvoice) => {
    setEditingInv(inv);
    setShipmentId(inv.shipmentId);
    setServiceProviderId(inv.serviceProviderId);
    setInvoiceNo(inv.invoiceNo);
    setInvoiceDate(inv.invoiceDate);
    setRemarks(inv.remarks);
    setIdempotencyKey(inv.idempotencyKey);
    const lines = (inv.lines || []).map((l) => ({
      expenseTypeId: l.expenseTypeId,
      amountInr: (l.amountPaise / 100).toFixed(2),
      cgstPct: (l.cgstRateBp / 100).toFixed(2),
      sgstPct: (l.sgstRateBp / 100).toFixed(2),
      igstPct: (l.igstRateBp / 100).toFixed(2),
      tdsPct: (l.tdsRateBp / 100).toFixed(2),
      remarks: l.remarks,
    }));
    setDraftLines(lines);
    setInvOpen(true);
    void refreshPreview(lines);
  };

  const updateLine = (idx: number, patch: Partial<DraftLine>) => {
    const next = draftLines.map((l, i) => (i === idx ? { ...l, ...patch } : l));
    setDraftLines(next);
    void refreshPreview(next);
  };

  const handleCombineDuplicates = async () => {
    try {
      const merged = await expenseService.mergeDuplicateLines(toLineInputs(draftLines));
      const next = merged.map((m) => ({
        expenseTypeId: m.expenseTypeId,
        amountInr: (m.amountPaise / 100).toFixed(2),
        cgstPct: (m.cgstRateBp / 100).toFixed(2),
        sgstPct: (m.sgstRateBp / 100).toFixed(2),
        igstPct: (m.igstRateBp / 100).toFixed(2),
        tdsPct: (m.tdsRateBp / 100).toFixed(2),
        remarks: m.remarks,
      }));
      setDraftLines(next);
      await refreshPreview(next);
      toast.success('Duplicate expense lines combined cleanly');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleSaveInvoice = async () => {
    setSavingInv(true);
    try {
      const req = {
        id: editingInv?.id ?? '',
        shipmentId,
        serviceProviderId,
        invoiceNo,
        invoiceDate,
        currency: 'INR',
        remarks,
        idempotencyKey,
        version: editingInv?.version ?? 1,
        mergeDuplicates: false,
        lines: toLineInputs(draftLines),
      };
      if (editingInv) {
        await expenseService.updateExpenseInvoice(req);
        toast.success('Expense invoice updated');
      } else {
        await expenseService.createExpenseInvoice(req);
        toast.success('Expense invoice recorded');
      }
      setInvOpen(false);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSavingInv(false);
    }
  };

  const handleSaveProvider = async () => {
    try {
      if (editingSp) {
        await expenseService.updateServiceProvider(editingSp.id, spForm);
        toast.success('Service provider updated');
      } else {
        await expenseService.createServiceProvider(spForm);
        toast.success('Service provider added');
      }
      setSpOpen(false);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleSaveType = async () => {
    try {
      if (editingEt) {
        await expenseService.updateExpenseType(editingEt.id, etForm);
        toast.success('Expense type updated');
      } else {
        await expenseService.createExpenseType(etForm);
        toast.success('Expense type added');
      }
      setEtOpen(false);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Import Expenses Ledger</h1>
          <p className="text-sm text-muted-foreground">
            Exact integer-paise &amp; basis-point GST/TDS expense invoices, providers, and bulk import.
          </p>
        </div>
        {canWrite && (
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              onClick={() => {
                setBulkShipmentId(shipments.find((s) => !s.isFrozen)?.id ?? '');
                setBulkOpen(true);
              }}
            >
              <Upload className="h-4 w-4" />
              Bulk CSV Import
            </Button>
            <Button onClick={openCreateInvoice}>
              <Plus className="h-4 w-4" />
              New Expense Invoice
            </Button>
          </div>
        )}
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="grid grid-cols-2 sm:grid-cols-4 w-full sm:w-auto h-auto">
          <TabsTrigger value="invoices" className="gap-2" onClick={() => setActiveTab('invoices')}>
            <Layers className="h-4 w-4" />
            <span>Expense Invoices ({invoices.length})</span>
          </TabsTrigger>
          <TabsTrigger value="itemized" className="gap-2" onClick={() => setActiveTab('itemized')}>
            <Receipt className="h-4 w-4" />
            <span>Itemized Expense Details ({allItemizedLines.length})</span>
          </TabsTrigger>
          <TabsTrigger value="providers" onClick={() => setActiveTab('providers')}>Service Providers ({providers.length})</TabsTrigger>
          <TabsTrigger value="types" onClick={() => setActiveTab('types')}>Expense Types ({types.length})</TabsTrigger>
        </TabsList>

        <TabsContent value="invoices" className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-3">
              <div className="relative w-72">
                <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  className="pl-8"
                  placeholder="Search provider, invoice no..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <Select
                value={shipmentFilter || 'all'}
                onValueChange={(v) => setShipmentFilter(v === 'all' ? '' : v)}
              >
                <SelectTrigger className="w-64">
                  <SelectValue placeholder="All shipments" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Shipments</SelectItem>
                  {shipments.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.invoiceNumber} — {s.supplierName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {invoices.length > 0 && (
              <Button
                variant="outline"
                size="sm"
                onClick={toggleAllInvoicesExpand}
                className="gap-1.5"
              >
                {allInvoicesExpanded ? (
                  <>
                    <ChevronsDownUp className="h-4 w-4" />
                    <span>Collapse All Details</span>
                  </>
                ) : (
                  <>
                    <ChevronDown className="h-4 w-4" />
                    <span>Expand All Details</span>
                  </>
                )}
              </Button>
            )}
          </div>

          <Card>
            <CardContent className="pt-6">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-10 text-center">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 w-6 p-0 hover:bg-muted"
                        onClick={toggleAllInvoicesExpand}
                        title={allInvoicesExpanded ? 'Collapse All' : 'Expand All'}
                      >
                        {allInvoicesExpanded ? (
                          <ChevronDown className="h-4 w-4 text-primary" />
                        ) : (
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
                        )}
                      </Button>
                    </TableHead>
                    <TableHead>Invoice No / Date</TableHead>
                    <TableHead>Service Provider</TableHead>
                    <TableHead>Shipment</TableHead>
                    <TableHead>Lines</TableHead>
                    <TableHead className="text-right">Basic (INR)</TableHead>
                    <TableHead className="text-right">GST (INR)</TableHead>
                    <TableHead className="text-right">TDS (INR)</TableHead>
                    <TableHead className="text-right">Net Payable (INR)</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={10} className="text-center text-muted-foreground py-8">
                        <div className="flex items-center justify-center gap-2">
                          <RefreshCw className="h-4 w-4 animate-spin text-primary" />
                          <span>Loading expense ledger...</span>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : invoices.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={10} className="text-center text-muted-foreground py-8">
                        No expense invoices recorded yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    invoices.map((inv) => {
                      const isExpanded = expandedInvoices.has(inv.id);
                      const gstPaise =
                        inv.totalCgstPaise + inv.totalSgstPaise + inv.totalIgstPaise;
                      const lineCount = inv.lines?.length ?? 0;
                      return (
                        <React.Fragment key={inv.id}>
                          <TableRow className={isExpanded ? 'bg-muted/20 border-b-0' : undefined}>
                            <TableCell className="text-center p-1">
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 w-7 text-muted-foreground hover:text-foreground p-0"
                                aria-label={isExpanded ? `Collapse details for ${inv.invoiceNo}` : `Expand details for ${inv.invoiceNo}`}
                                onClick={() => toggleInvoiceExpand(inv.id)}
                              >
                                {isExpanded ? (
                                  <ChevronDown className="h-4 w-4 text-primary" />
                                ) : (
                                  <ChevronRight className="h-4 w-4" />
                                )}
                              </Button>
                            </TableCell>
                            <TableCell>
                              <div className="font-medium flex items-center gap-1.5">
                                <span>{inv.invoiceNo}</span>
                              </div>
                              <div className="text-xs text-muted-foreground">
                                {inv.invoiceDate} · v{inv.version}
                              </div>
                            </TableCell>
                            <TableCell>{inv.serviceProviderName || '—'}</TableCell>
                            <TableCell className="font-mono text-xs">{inv.shipmentInvoiceNo || '—'}</TableCell>
                            <TableCell>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 px-2 font-normal text-xs gap-1 border border-border/60 hover:bg-muted"
                                onClick={() => toggleInvoiceExpand(inv.id)}
                                title="Click to view detailed line items"
                              >
                                <Badge variant="secondary" className="px-1.5 py-0 text-xs">
                                  {lineCount}
                                </Badge>
                                <span>{lineCount === 1 ? 'line' : 'lines'}</span>
                                {isExpanded ? (
                                  <ChevronDown className="h-3 w-3 text-muted-foreground" />
                                ) : (
                                  <ChevronRight className="h-3 w-3 text-muted-foreground" />
                                )}
                              </Button>
                            </TableCell>
                            <TableCell className="text-right font-mono">
                              {formatPaiseINR(inv.baseAmountPaise)}
                            </TableCell>
                            <TableCell className="text-right font-mono">
                              {formatPaiseINR(gstPaise)}
                            </TableCell>
                            <TableCell className="text-right font-mono">
                              {formatPaiseINR(inv.totalTdsPaise)}
                            </TableCell>
                            <TableCell className="text-right font-mono font-semibold">
                              {formatPaiseINR(inv.netAmountPaise)}
                            </TableCell>
                            <TableCell className="text-right">
                              <div className="flex justify-end gap-1">
                                {canWrite && (
                                  <Button
                                    size="sm"
                                    variant="ghost"
                                    aria-label={`Edit ${inv.invoiceNo}`}
                                    onClick={() => openEditInvoice(inv)}
                                  >
                                    <Pencil className="h-4 w-4" />
                                  </Button>
                                )}
                                {canDelete && (
                                  <Button
                                    size="sm"
                                    variant="ghost"
                                    aria-label={`Delete ${inv.invoiceNo}`}
                                    onClick={() =>
                                      void expenseService
                                        .deleteExpenseInvoice(inv.id)
                                        .then(loadAll)
                                        .then(() => toast.success('Expense invoice moved to Recycle Bin'))
                                        .catch((err) => toast.error(String(err)))
                                    }
                                  >
                                    <Trash2 className="h-4 w-4 text-destructive" />
                                  </Button>
                                )}
                              </div>
                            </TableCell>
                          </TableRow>

                          {/* Expanded Line Items Breakdown */}
                          {isExpanded && (
                            <TableRow className="bg-muted/10 hover:bg-muted/10 border-t-0">
                              <TableCell colSpan={10} className="p-0">
                                <div className="p-4 pl-10 pr-4 border-b border-border/60 bg-slate-950/30 dark:bg-slate-900/40 space-y-3">
                                  <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                                    <div className="flex items-center gap-2 font-medium text-foreground">
                                      <Receipt className="h-4 w-4 text-primary" />
                                      <span className="font-semibold">
                                        Expense Line Details for Invoice {inv.invoiceNo}
                                      </span>
                                      <Badge variant="outline" className="text-[11px]">
                                        {lineCount} line item{lineCount === 1 ? '' : 's'}
                                      </Badge>
                                      {inv.remarks && (
                                        <span className="text-muted-foreground italic">
                                          &ldquo;{inv.remarks}&rdquo;
                                        </span>
                                      )}
                                    </div>
                                    {canWrite && (
                                      <Button
                                        size="sm"
                                        variant="outline"
                                        className="h-7 text-xs gap-1.5"
                                        onClick={() => openEditInvoice(inv)}
                                      >
                                        <Pencil className="h-3 w-3" />
                                        Edit / Add Lines
                                      </Button>
                                    )}
                                  </div>

                                  {!inv.lines || inv.lines.length === 0 ? (
                                    <div className="p-4 text-xs text-muted-foreground bg-muted/20 rounded border border-dashed text-center">
                                      No individual line items found for this invoice.
                                    </div>
                                  ) : (
                                    <div className="rounded-md border border-border/70 overflow-hidden bg-background">
                                      <Table>
                                        <TableHeader className="bg-muted/50">
                                          <TableRow className="hover:bg-transparent text-xs">
                                            <TableHead className="w-10 text-center py-2">#</TableHead>
                                            <TableHead className="py-2">Expense Type</TableHead>
                                            <TableHead className="text-right py-2">Basic (INR)</TableHead>
                                            <TableHead className="text-right py-2">CGST</TableHead>
                                            <TableHead className="text-right py-2">SGST</TableHead>
                                            <TableHead className="text-right py-2">IGST</TableHead>
                                            <TableHead className="text-right py-2">TDS</TableHead>
                                            <TableHead className="text-right py-2 font-semibold">Net Payable (INR)</TableHead>
                                            <TableHead className="py-2">Remarks</TableHead>
                                          </TableRow>
                                        </TableHeader>
                                        <TableBody>
                                          {inv.lines.map((line, idx) => (
                                            <TableRow key={line.id || `line-${idx}`} className="text-xs hover:bg-muted/30">
                                              <TableCell className="text-center font-mono text-muted-foreground py-2">
                                                {idx + 1}
                                              </TableCell>
                                              <TableCell className="font-medium py-2">
                                                <Badge variant="secondary" className="font-medium">
                                                  {line.expenseTypeName || 'Expense'}
                                                </Badge>
                                              </TableCell>
                                              <TableCell className="text-right font-mono py-2 font-medium">
                                                {formatPaiseINR(line.amountPaise)}
                                              </TableCell>
                                              <TableCell className="text-right font-mono text-muted-foreground py-2">
                                                {line.cgstRateBp > 0 ? (
                                                  <span>
                                                    {(line.cgstRateBp / 100).toFixed(1)}% ·{' '}
                                                    <span className="text-foreground">{formatPaiseINR(line.cgstPaise)}</span>
                                                  </span>
                                                ) : (
                                                  '—'
                                                )}
                                              </TableCell>
                                              <TableCell className="text-right font-mono text-muted-foreground py-2">
                                                {line.sgstRateBp > 0 ? (
                                                  <span>
                                                    {(line.sgstRateBp / 100).toFixed(1)}% ·{' '}
                                                    <span className="text-foreground">{formatPaiseINR(line.sgstPaise)}</span>
                                                  </span>
                                                ) : (
                                                  '—'
                                                )}
                                              </TableCell>
                                              <TableCell className="text-right font-mono text-muted-foreground py-2">
                                                {line.igstRateBp > 0 ? (
                                                  <span>
                                                    {(line.igstRateBp / 100).toFixed(1)}% ·{' '}
                                                    <span className="text-foreground">{formatPaiseINR(line.igstPaise)}</span>
                                                  </span>
                                                ) : (
                                                  '—'
                                                )}
                                              </TableCell>
                                              <TableCell className="text-right font-mono text-amber-500 py-2">
                                                {line.tdsRateBp > 0 ? (
                                                  <span>
                                                    -{(line.tdsRateBp / 100).toFixed(1)}% ·{' '}
                                                    <span>{formatPaiseINR(line.tdsPaise)}</span>
                                                  </span>
                                                ) : (
                                                  '—'
                                                )}
                                              </TableCell>
                                              <TableCell className="text-right font-mono font-semibold py-2">
                                                {formatPaiseINR(line.netPaise)}
                                              </TableCell>
                                              <TableCell className="text-muted-foreground max-w-xs truncate py-2" title={line.remarks}>
                                                {line.remarks || '—'}
                                              </TableCell>
                                            </TableRow>
                                          ))}
                                        </TableBody>
                                      </Table>
                                    </div>
                                  )}
                                </div>
                              </TableCell>
                            </TableRow>
                          )}
                        </React.Fragment>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="itemized" className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total Line Items</div>
              <div className="text-xl font-bold">{itemizedSummary.count}</div>
            </Card>
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total Basic (INR)</div>
              <div className="text-xl font-bold font-mono">{formatPaiseINR(itemizedSummary.basicPaise)}</div>
            </Card>
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total GST (INR)</div>
              <div className="text-xl font-bold font-mono text-blue-500">{formatPaiseINR(itemizedSummary.gstPaise)}</div>
            </Card>
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total Net Payable (INR)</div>
              <div className="text-xl font-bold font-mono text-green-500">{formatPaiseINR(itemizedSummary.netPaise)}</div>
            </Card>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative w-64">
                <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  className="pl-8 h-9 text-xs"
                  placeholder="Filter lines by invoice, type, remarks..."
                  value={itemizedSearch}
                  onChange={(e) => setItemizedSearch(e.target.value)}
                />
              </div>

              <Select
                value={itemizedTypeFilter}
                onValueChange={setItemizedTypeFilter}
              >
                <SelectTrigger className="w-48 h-9 text-xs">
                  <SelectValue placeholder="All Expense Types" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Expense Types</SelectItem>
                  {types.map((t) => (
                    <SelectItem key={t.id} value={t.id}>
                      {t.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              <Select
                value={itemizedProviderFilter}
                onValueChange={setItemizedProviderFilter}
              >
                <SelectTrigger className="w-48 h-9 text-xs">
                  <SelectValue placeholder="All Providers" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Providers</SelectItem>
                  {providers.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              <Select
                value={itemizedShipmentFilter}
                onValueChange={setItemizedShipmentFilter}
              >
                <SelectTrigger className="w-52 h-9 text-xs">
                  <SelectValue placeholder="All Shipments" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Shipments</SelectItem>
                  {shipments.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.invoiceNumber} — {s.supplierName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={exportItemizedCsv}
              disabled={filteredItemizedLines.length === 0}
              className="h-9 gap-1.5"
            >
              <FileSpreadsheet className="h-4 w-4" />
              <span>Export CSV</span>
            </Button>
          </div>

          <Card>
            <CardContent className="pt-6">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Date / Invoice</TableHead>
                    <TableHead>Service Provider</TableHead>
                    <TableHead>Shipment</TableHead>
                    <TableHead>Expense Type</TableHead>
                    <TableHead className="text-right">Basic (INR)</TableHead>
                    <TableHead className="text-right">GST (INR)</TableHead>
                    <TableHead className="text-right">TDS (INR)</TableHead>
                    <TableHead className="text-right">Net Payable (INR)</TableHead>
                    <TableHead>Remarks</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredItemizedLines.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={10} className="text-center text-muted-foreground py-8">
                        No itemized expense lines match the current filters.
                      </TableCell>
                    </TableRow>
                  ) : (
                    filteredItemizedLines.map((line) => {
                      const gstPaise = line.cgstPaise + line.sgstPaise + line.igstPaise;
                      return (
                        <TableRow key={line.lineId}>
                          <TableCell>
                            <div className="font-medium">{line.invoiceNo}</div>
                            <div className="text-xs text-muted-foreground">{line.invoiceDate}</div>
                          </TableCell>
                          <TableCell>{line.serviceProviderName}</TableCell>
                          <TableCell className="font-mono text-xs">{line.shipmentInvoiceNo}</TableCell>
                          <TableCell>
                            <Badge variant="secondary" className="font-medium">
                              {line.expenseTypeName}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right font-mono">
                            {formatPaiseINR(line.amountPaise)}
                          </TableCell>
                          <TableCell className="text-right font-mono text-muted-foreground">
                            {formatPaiseINR(gstPaise)}
                          </TableCell>
                          <TableCell className="text-right font-mono text-amber-500">
                            {line.tdsPaise > 0 ? `-${formatPaiseINR(line.tdsPaise)}` : '—'}
                          </TableCell>
                          <TableCell className="text-right font-mono font-semibold">
                            {formatPaiseINR(line.netPaise)}
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground max-w-xs truncate" title={line.remarks}>
                            {line.remarks || '—'}
                          </TableCell>
                          <TableCell className="text-right">
                            {canWrite && (
                              <Button
                                size="sm"
                                variant="ghost"
                                title="Edit parent invoice"
                                onClick={() => openEditInvoice(line.parentInvoice)}
                              >
                                <Pencil className="h-4 w-4" />
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="providers">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle className="text-base">Service Providers (CHA / Forwarders)</CardTitle>
                <CardDescription>Manage GSTIN and contact details for expense vendors.</CardDescription>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setManageSpOpen(true)}
                >
                  <Settings2 className="h-4 w-4 mr-1" />
                  Manage Providers
                </Button>
                {canWrite && (
                  <Button
                    size="sm"
                    onClick={() => {
                      setEditingSp(null);
                      setSpForm({
                        name: '',
                        gstin: '',
                        state: '',
                        contactPerson: '',
                        contactEmail: '',
                        contactPhone: '',
                      });
                      setSpOpen(true);
                    }}
                  >
                    <Plus className="h-4 w-4 mr-1" />
                    Add Provider
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Provider Name</TableHead>
                    <TableHead>GSTIN</TableHead>
                    <TableHead>State</TableHead>
                    <TableHead>Contact</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {providers.map((p) => (
                    <TableRow key={p.id}>
                      <TableCell className="font-medium">{p.name}</TableCell>
                      <TableCell className="font-mono text-xs">{p.gstin || '—'}</TableCell>
                      <TableCell>{p.state || '—'}</TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {p.contactPerson || p.contactEmail || p.contactPhone || '—'}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-1">
                          {canWrite && (
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => {
                                setEditingSp(p);
                                setSpForm({
                                  name: p.name,
                                  gstin: p.gstin,
                                  state: p.state,
                                  contactPerson: p.contactPerson,
                                  contactEmail: p.contactEmail,
                                  contactPhone: p.contactPhone,
                                });
                                setSpOpen(true);
                              }}
                            >
                              <Pencil className="h-4 w-4" />
                            </Button>
                          )}
                          {canDelete && (
                            <Button
                              size="sm"
                              variant="ghost"
                              aria-label={`Delete ${p.name}`}
                              onClick={() =>
                                void expenseService
                                  .deleteServiceProvider(p.id)
                                  .then(loadAll)
                                  .then(() => toast.success(`Service provider "${p.name}" removed`))
                                  .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
                              }
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="types">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle className="text-base">Expense Types &amp; Statutory Tax Defaults</CardTitle>
                <CardDescription>Default CGST/SGST/IGST/TDS rates stored in basis points (100 bp = 1%).</CardDescription>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setTargetLineIdx(null);
                    setManageEtOpen(true);
                  }}
                >
                  <Settings2 className="h-4 w-4 mr-1" />
                  Manage Types
                </Button>
                {canWrite && (
                  <Button
                    size="sm"
                    onClick={() => {
                      setEditingEt(null);
                      setEtForm({
                        name: '',
                        defaultCgstRateBp: 900,
                        defaultSgstRateBp: 900,
                        defaultIgstRateBp: 0,
                        defaultTdsRateBp: 200,
                        isActive: true,
                      });
                      setEtOpen(true);
                    }}
                  >
                    <Plus className="h-4 w-4 mr-1" />
                    Add Expense Type
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>CGST</TableHead>
                    <TableHead>SGST</TableHead>
                    <TableHead>IGST</TableHead>
                    <TableHead>TDS</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {types.map((t) => (
                    <TableRow key={t.id}>
                      <TableCell className="font-medium">{t.name}</TableCell>
                      <TableCell className="font-mono text-xs">{(t.defaultCgstRateBp / 100).toFixed(2)}%</TableCell>
                      <TableCell className="font-mono text-xs">{(t.defaultSgstRateBp / 100).toFixed(2)}%</TableCell>
                      <TableCell className="font-mono text-xs">{(t.defaultIgstRateBp / 100).toFixed(2)}%</TableCell>
                      <TableCell className="font-mono text-xs">{(t.defaultTdsRateBp / 100).toFixed(2)}%</TableCell>
                      <TableCell>
                        <Badge variant={t.isActive ? 'success' : 'secondary'}>
                          {t.isActive ? 'Active' : 'Inactive'}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-1">
                          {canWrite && (
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => {
                                setEditingEt(t);
                                setEtForm({
                                  name: t.name,
                                  defaultCgstRateBp: t.defaultCgstRateBp,
                                  defaultSgstRateBp: t.defaultSgstRateBp,
                                  defaultIgstRateBp: t.defaultIgstRateBp,
                                  defaultTdsRateBp: t.defaultTdsRateBp,
                                  isActive: t.isActive,
                                });
                                setEtOpen(true);
                              }}
                            >
                              <Pencil className="h-4 w-4" />
                            </Button>
                          )}
                          {canDelete && (
                            <Button
                              size="sm"
                              variant="ghost"
                              aria-label={`Delete ${t.name}`}
                              onClick={() =>
                                void expenseService
                                  .deleteExpenseType(t.id)
                                  .then(loadAll)
                                  .then(() => toast.success(`Expense type "${t.name}" removed`))
                                  .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
                              }
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Multi-line Expense Invoice Dialog */}
      <Dialog open={invOpen} onOpenChange={setInvOpen}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingInv ? `Edit Expense Invoice (${editingInv.invoiceNo})` : 'New Multi-Line Expense Invoice'}
            </DialogTitle>
            <DialogDescription>
              All amounts calculate in integer paise without floating-point drift.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-3 md:grid-cols-4">
            <div className="space-y-1">
              <Label>Shipment *</Label>
              <Select value={shipmentId} onValueChange={setShipmentId}>
                <SelectTrigger>
                  <SelectValue placeholder="Choose shipment" />
                </SelectTrigger>
                <SelectContent>
                  {shipments
                    .filter((s) => !s.isFrozen || s.id === shipmentId)
                    .map((s) => (
                      <SelectItem key={s.id} value={s.id}>
                        {s.invoiceNumber} — {s.supplierName}
                      </SelectItem>
                    ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <div className="flex items-center justify-between">
                <Label>Service Provider *</Label>
                <button
                  type="button"
                  className="inline-flex items-center gap-1 text-xs text-primary hover:underline cursor-pointer"
                  onClick={() => setManageSpOpen(true)}
                >
                  <Settings2 className="h-3 w-3" />
                  Manage
                </button>
              </div>
              <Select value={serviceProviderId} onValueChange={setServiceProviderId}>
                <SelectTrigger>
                  <SelectValue placeholder="Choose provider" />
                </SelectTrigger>
                <SelectContent>
                  {providers.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label htmlFor="expInvNo">Invoice No *</Label>
              <Input id="expInvNo" value={invoiceNo} onChange={(e) => setInvoiceNo(e.target.value)} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="expInvDate">Invoice Date *</Label>
              <Input
                id="expInvDate"
                type="date"
                value={invoiceDate}
                onChange={(e) => setInvoiceDate(e.target.value)}
              />
            </div>
          </div>

          {hasDuplicateType && (
            <div className="flex items-center justify-between rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs">
              <span className="flex items-center gap-2 font-medium text-amber-800 dark:text-amber-300">
                <AlertTriangle className="h-4 w-4" />
                Duplicate expense type detected across lines. Combine matching lines before saving.
              </span>
              <Button size="sm" variant="outline" onClick={() => void handleCombineDuplicates()}>
                <Merge className="h-3.5 w-3.5" />
                Combine Duplicate Lines
              </Button>
            </div>
          )}

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Expense Lines</Label>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  type="button"
                  className="h-8 gap-1 text-xs"
                  onClick={() => {
                    setTargetLineIdx(null);
                    setManageEtOpen(true);
                  }}
                >
                  <Settings2 className="h-3.5 w-3.5" />
                  Manage Types
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  type="button"
                  className="h-8 gap-1 text-xs"
                  onClick={() => setDraftLines((prev) => [...prev, emptyLine(types[0])])}
                >
                  <Plus className="h-3.5 w-3.5" />
                  Add Line
                </Button>
              </div>
            </div>

            {draftLines.map((line, idx) => (
              <div key={idx} className="grid grid-cols-12 items-end gap-2 rounded-md border p-2">
                <div className="col-span-3 space-y-1">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs">Expense Type</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-[11px] text-primary hover:underline cursor-pointer"
                      onClick={() => {
                        setTargetLineIdx(idx);
                        setManageEtOpen(true);
                      }}
                    >
                      <Settings2 className="h-2.5 w-2.5" />
                      Manage
                    </button>
                  </div>
                  <Select
                    value={line.expenseTypeId}
                    onValueChange={(v) => {
                      const et = types.find((t) => t.id === v);
                      updateLine(idx, {
                        expenseTypeId: v,
                        cgstPct: et ? (et.defaultCgstRateBp / 100).toFixed(2) : line.cgstPct,
                        sgstPct: et ? (et.defaultSgstRateBp / 100).toFixed(2) : line.sgstPct,
                        igstPct: et ? (et.defaultIgstRateBp / 100).toFixed(2) : line.igstPct,
                        tdsPct: et ? (et.defaultTdsRateBp / 100).toFixed(2) : line.tdsPct,
                      });
                    }}
                  >
                    <SelectTrigger className="h-8 text-xs">
                      <SelectValue placeholder="Type" />
                    </SelectTrigger>
                    <SelectContent>
                      {types.map((t) => (
                        <SelectItem key={t.id} value={t.id}>
                          {t.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="col-span-2 space-y-1">
                  <Label className="text-xs">Basic (INR)</Label>
                  <Input
                    className="h-8 text-xs"
                    type="number"
                    step="0.01"
                    value={line.amountInr}
                    onChange={(e) => updateLine(idx, { amountInr: e.target.value })}
                  />
                </div>
                <div className="col-span-1 space-y-1">
                  <Label className="text-xs">CGST%</Label>
                  <Input
                    className="h-8 text-xs"
                    type="number"
                    step="0.01"
                    value={line.cgstPct}
                    onChange={(e) => updateLine(idx, { cgstPct: e.target.value })}
                  />
                </div>
                <div className="col-span-1 space-y-1">
                  <Label className="text-xs">SGST%</Label>
                  <Input
                    className="h-8 text-xs"
                    type="number"
                    step="0.01"
                    value={line.sgstPct}
                    onChange={(e) => updateLine(idx, { sgstPct: e.target.value })}
                  />
                </div>
                <div className="col-span-1 space-y-1">
                  <Label className="text-xs">IGST%</Label>
                  <Input
                    className="h-8 text-xs"
                    type="number"
                    step="0.01"
                    value={line.igstPct}
                    onChange={(e) => updateLine(idx, { igstPct: e.target.value })}
                  />
                </div>
                <div className="col-span-1 space-y-1">
                  <Label className="text-xs">TDS%</Label>
                  <Input
                    className="h-8 text-xs"
                    type="number"
                    step="0.01"
                    value={line.tdsPct}
                    onChange={(e) => updateLine(idx, { tdsPct: e.target.value })}
                  />
                </div>
                <div className="col-span-2 space-y-1">
                  <Label className="text-xs">Remarks</Label>
                  <Input
                    className="h-8 text-xs"
                    value={line.remarks}
                    onChange={(e) => updateLine(idx, { remarks: e.target.value })}
                  />
                </div>
                <div className="col-span-1 flex justify-end">
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-8 w-8 p-0"
                    disabled={draftLines.length === 1}
                    onClick={() => {
                      const next = draftLines.filter((_, i) => i !== idx);
                      setDraftLines(next);
                      void refreshPreview(next);
                    }}
                  >
                    <Trash2 className="h-3.5 w-3.5 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
          </div>

          {preview && (
            <div className="grid grid-cols-4 gap-3 rounded-md bg-muted/40 p-3 text-xs">
              <div>
                <span className="text-muted-foreground">Base (Excl. GST)</span>
                <div className="font-mono text-sm font-semibold">
                  INR {formatPaiseINR(preview.baseAmountPaise)}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">GST (CGST+SGST+IGST)</span>
                <div className="font-mono text-sm font-semibold">
                  INR {formatPaiseINR(preview.totalCgstPaise + preview.totalSgstPaise + preview.totalIgstPaise)}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">TDS Deducted</span>
                <div className="font-mono text-sm font-semibold">
                  INR {formatPaiseINR(preview.totalTdsPaise)}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Net Payable</span>
                <div className="font-mono text-sm font-bold text-primary">
                  INR {formatPaiseINR(preview.netAmountPaise)}
                </div>
              </div>
            </div>
          )}

          <div className="space-y-1">
            <Label htmlFor="expInvRemarks">Invoice Remarks</Label>
            <Input
              id="expInvRemarks"
              value={remarks}
              onChange={(e) => setRemarks(e.target.value)}
            />
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setInvOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={savingInv || hasDuplicateType || !shipmentId || !serviceProviderId || !invoiceNo}
              onClick={() => void handleSaveInvoice()}
            >
              {savingInv ? 'Saving...' : 'Save Expense Invoice'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Service Provider Dialog */}
      <Dialog open={spOpen} onOpenChange={setSpOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingSp ? 'Edit Service Provider' : 'Add Service Provider'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="spName">Name *</Label>
              <Input
                id="spName"
                value={spForm.name}
                onChange={(e) => setSpForm({ ...spForm, name: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label htmlFor="spGstin">GSTIN</Label>
                <Input
                  id="spGstin"
                  value={spForm.gstin}
                  onChange={(e) => setSpForm({ ...spForm, gstin: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="spState">State</Label>
                <Input
                  id="spState"
                  value={spForm.state}
                  onChange={(e) => setSpForm({ ...spForm, state: e.target.value })}
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label htmlFor="spEmail">Contact Email</Label>
                <Input
                  id="spEmail"
                  value={spForm.contactEmail}
                  onChange={(e) => setSpForm({ ...spForm, contactEmail: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="spPhone">Phone</Label>
                <Input
                  id="spPhone"
                  value={spForm.contactPhone}
                  onChange={(e) => setSpForm({ ...spForm, contactPhone: e.target.value })}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSpOpen(false)}>Cancel</Button>
            <Button onClick={() => void handleSaveProvider()}>Save Provider</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Expense Type Dialog */}
      <Dialog open={etOpen} onOpenChange={setEtOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingEt ? 'Edit Expense Type' : 'Add Expense Type'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="etName">Name *</Label>
              <Input
                id="etName"
                value={etForm.name}
                onChange={(e) => setEtForm({ ...etForm, name: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Default CGST (bp)</Label>
                <Input
                  type="number"
                  value={etForm.defaultCgstRateBp}
                  onChange={(e) =>
                    setEtForm({ ...etForm, defaultCgstRateBp: Number.parseInt(e.target.value, 10) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label>Default SGST (bp)</Label>
                <Input
                  type="number"
                  value={etForm.defaultSgstRateBp}
                  onChange={(e) =>
                    setEtForm({ ...etForm, defaultSgstRateBp: Number.parseInt(e.target.value, 10) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label>Default IGST (bp)</Label>
                <Input
                  type="number"
                  value={etForm.defaultIgstRateBp}
                  onChange={(e) =>
                    setEtForm({ ...etForm, defaultIgstRateBp: Number.parseInt(e.target.value, 10) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label>Default TDS (bp)</Label>
                <Input
                  type="number"
                  value={etForm.defaultTdsRateBp}
                  onChange={(e) =>
                    setEtForm({ ...etForm, defaultTdsRateBp: Number.parseInt(e.target.value, 10) || 0 })
                  }
                />
              </div>
            </div>
            <div className="flex items-center gap-2 pt-1">
              <Switch
                checked={etForm.isActive}
                onCheckedChange={(v) => setEtForm({ ...etForm, isActive: v })}
              />
              <Label>Active</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEtOpen(false)}>Cancel</Button>
            <Button onClick={() => void handleSaveType()}>Save Expense Type</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <CsvImportDialog
        open={bulkOpen}
        onOpenChange={setBulkOpen}
        title="Bulk Expense CSV Import"
        description="Imports the 11-column Expense CSV into the selected active shipment."
        templateFilename="expenses_bulk_template.csv"
        onDownloadTemplate={() => expenseService.downloadBulkTemplateCsv()}
        onImport={(content) => expenseService.importBulkCsv(bulkShipmentId, content)}
        onCompleted={loadAll}
      />

      <ManageServiceProvidersDialog
        open={manageSpOpen}
        onOpenChange={setManageSpOpen}
        providers={providers}
        selectedId={serviceProviderId}
        onUpdated={loadAll}
        onSelect={(id) => {
          setServiceProviderId(id);
          toast.success('Selected service provider');
        }}
      />

      <ManageExpenseTypesDialog
        open={manageEtOpen}
        onOpenChange={setManageEtOpen}
        types={types}
        selectedId={
          targetLineIdx !== null && draftLines[targetLineIdx]
            ? draftLines[targetLineIdx].expenseTypeId
            : undefined
        }
        onUpdated={loadAll}
        onSelect={(et) => {
          if (targetLineIdx !== null && draftLines[targetLineIdx]) {
            updateLine(targetLineIdx, {
              expenseTypeId: et.id,
              cgstPct: (et.defaultCgstRateBp / 100).toFixed(2),
              sgstPct: (et.defaultSgstRateBp / 100).toFixed(2),
              igstPct: (et.defaultIgstRateBp / 100).toFixed(2),
              tdsPct: (et.defaultTdsRateBp / 100).toFixed(2),
            });
            toast.success(`Selected expense type "${et.name}"`);
          }
        }}
      />
    </div>
  );
}
