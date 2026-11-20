import * as React from 'react';
import {
  AlertCircle,
  Clock,
  DollarSign,
  Eye,
  FileSpreadsheet,
  Plus,
  RefreshCw,
  Search,
  Trash2,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
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
import { useAuth } from '@/contexts/AuthContext';
import { useModuleSettings } from '@/contexts/ThemeContext';
import { useDebounce } from '@/hooks/useDebounce';
import { itemService, supplierService, type Item, type Supplier } from '@/services/master-service';
import {
  paymentPOService,
  type POMilestoneInput,
  type PurchaseOrder,
  type PurchaseOrderInput,
} from '@/services/payment-po-service';

export function PurchaseOrdersPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');
  const canDelete = hasPermission('data.delete');

  const [orders, setOrders] = React.useState<PurchaseOrder[]>([]);
  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);
  const [itemsCatalog, setItemsCatalog] = React.useState<Item[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [search, setSearch] = React.useState('');
  const debouncedSearch = useDebounce(search, 250);
  const [statusFilter, setStatusFilter] = React.useState('all');
  const [supplierFilter, setSupplierFilter] = React.useState('all');
  const { itemsPerPage: pageSize } = useModuleSettings('purchaseOrders');
  const [page, setPage] = React.useState(1);
  const [totalCount, setTotalCount] = React.useState(0);

  React.useEffect(() => {
    setPage(1);
  }, [pageSize, debouncedSearch, supplierFilter, statusFilter]);

  // Modals
  const [createModalOpen, setCreateModalOpen] = React.useState(false);
  const [viewPO, setViewPO] = React.useState<PurchaseOrder | null>(null);
  const [submitting, setSubmitting] = React.useState(false);

  // Form State
  const [poNumber, setPoNumber] = React.useState('');
  const [poDate, setPoDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [supplierId, setSupplierId] = React.useState('');
  const [currency, setCurrency] = React.useState('USD');
  const [exchangeRate, setExchangeRate] = React.useState(85.0);
  const [incoterm, setIncoterm] = React.useState('FOB');
  const [preset, setPreset] = React.useState('100_ADVANCE');
  const [expDeliveryDate, setExpDeliveryDate] = React.useState('');
  const [notes, setNotes] = React.useState('');

  // Line items state
  const [lines, setLines] = React.useState<Array<{
    itemId: string;
    partNumber: string;
    itemDescription: string;
    quantity: number;
    unit: string;
    unitPrice: number;
    hsnCode: string;
  }>>([
    { itemId: '', partNumber: '', itemDescription: '', quantity: 100, unit: 'NOS', unitPrice: 0, hsnCode: '' },
  ]);

  // Custom milestone state
  const [customMilestones] = React.useState<POMilestoneInput[]>([
    { sequence: 1, milestoneName: 'Initial Advance', stageType: 'ADVANCE', percentage: 50, notes: '' },
    { sequence: 2, milestoneName: 'Balance on Delivery', stageType: 'ON_DELIVERY', percentage: 50, notes: '' },
  ]);

  const loadData = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await paymentPOService.listPurchaseOrders({
        search: debouncedSearch.trim(),
        supplierId: supplierFilter === 'all' ? '' : supplierFilter,
        status: statusFilter === 'all' ? '' : statusFilter,
        page,
        pageSize,
      });
      setOrders(res.items || []);
      setTotalCount(res.totalCount || 0);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [debouncedSearch, supplierFilter, statusFilter, page, pageSize]);

  React.useEffect(() => {
    loadData();
  }, [loadData]);

  React.useEffect(() => {
    // Load suppliers and items for dropdowns
    supplierService.list({ page: 1, pageSize: 200, search: '', status: '', sortBy: '', sortDir: '' }).then((res) => {
      setSuppliers(res.items || []);
    }).catch(() => {});

    itemService.list({ page: 1, pageSize: 300, search: '', supplierId: '', status: '', sortBy: '', sortDir: '' }).then((res) => {
      setItemsCatalog(res.items || []);
    }).catch(() => {});
  }, []);

  const handleOpenCreate = () => {
    const generatedNo = `PO-${new Date().getFullYear()}-${Math.floor(1000 + Math.random() * 9000)}`;
    setPoNumber(generatedNo);
    setPoDate(new Date().toISOString().slice(0, 10));
    setSupplierId(suppliers[0]?.id || '');
    setCurrency('USD');
    setExchangeRate(85.0);
    setIncoterm('FOB');
    setPreset('100_ADVANCE');
    setExpDeliveryDate('');
    setNotes('');
    setLines([
      { itemId: '', partNumber: '', itemDescription: '', quantity: 100, unit: 'NOS', unitPrice: 0, hsnCode: '' },
    ]);
    setCreateModalOpen(true);
  };

  const handleLineChange = (index: number, field: string, val: any) => {
    setLines((prev) => {
      const updated = [...prev];
      const current = updated[index] || { itemId: '', partNumber: '', itemDescription: '', quantity: 1, unit: 'NOS', unitPrice: 0, hsnCode: '' };
      updated[index] = {
        itemId: current.itemId,
        partNumber: field === 'partNumber' ? String(val) : current.partNumber,
        itemDescription: field === 'itemDescription' ? String(val) : current.itemDescription,
        quantity: field === 'quantity' ? Number(val) || 0 : current.quantity,
        unit: field === 'unit' ? String(val) : current.unit,
        unitPrice: field === 'unitPrice' ? Number(val) || 0 : current.unitPrice,
        hsnCode: field === 'hsnCode' ? String(val) : current.hsnCode,
      };
      return updated;
    });
  };

  const handleSelectCatalogItem = (index: number, itemId: string) => {
    const it = itemsCatalog.find((x) => x.id === itemId);
    if (!it) return;
    setLines((prev) => {
      const updated = [...prev];
      updated[index] = {
        itemId: it.id,
        partNumber: it.partNumber,
        itemDescription: it.itemDescription,
        unitPrice: it.unitPrice || 0,
        unit: it.unit || 'NOS',
        hsnCode: it.hsnCode || '',
        quantity: updated[index]?.quantity || 1,
      };
      return updated;
    });
  };

  const addLine = () => {
    setLines((prev) => [
      ...prev,
      { itemId: '', partNumber: '', itemDescription: '', quantity: 1, unit: 'NOS', unitPrice: 0, hsnCode: '' },
    ]);
  };

  const removeLine = (idx: number) => {
    if (lines.length === 1) return;
    setLines((prev) => prev.filter((_, i) => i !== idx));
  };

  const computedTotal = React.useMemo(() => {
    return lines.reduce((acc, l) => acc + (Number(l.quantity) || 0) * (Number(l.unitPrice) || 0), 0);
  }, [lines]);

  const handleSubmitPO = async () => {
    if (!poNumber.trim()) {
      toast.error('PO Number is required');
      return;
    }
    if (!supplierId) {
      toast.error('Please select a supplier');
      return;
    }
    const validLines = lines.filter((l) => l.partNumber.trim() && l.quantity > 0);
    if (validLines.length === 0) {
      toast.error('Please enter at least one valid line item with part number and quantity > 0');
      return;
    }

    setSubmitting(true);
    try {
      const payload: PurchaseOrderInput = {
        poNumber: poNumber.trim(),
        poDate,
        supplierId,
        currency,
        exchangeRate: Number(exchangeRate) || 1.0,
        incoterm,
        paymentTermsPreset: preset,
        paymentTermsDesc: preset === '100_ADVANCE' ? '100% Full Advance' : preset.replace(/_/g, '-'),
        expectedDeliveryDate: expDeliveryDate,
        notes,
        lineItems: validLines,
        milestones: preset === 'CUSTOM' ? customMilestones : undefined,
      };

      await paymentPOService.createPurchaseOrder(payload);
      toast.success(`Purchase Order ${poNumber} created successfully`);
      setCreateModalOpen(false);
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeletePO = async (id: string, poNum: string) => {
    if (!confirm(`Are you sure you want to delete Purchase Order ${poNum}?`)) return;
    try {
      await paymentPOService.deletePurchaseOrder(id);
      toast.success(`Purchase order ${poNum} deleted`);
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleOpenView = async (id: string) => {
    try {
      const detail = await paymentPOService.getPurchaseOrder(id);
      setViewPO(detail);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'FULLY_PAID':
        return <Badge className="bg-emerald-600/15 text-emerald-700 hover:bg-emerald-600/20">Fully Paid</Badge>;
      case 'PARTIALLY_PAID':
        return <Badge className="bg-blue-600/15 text-blue-700 hover:bg-blue-600/20">Partially Paid</Badge>;
      case 'CONFIRMED':
        return <Badge className="bg-amber-600/15 text-amber-700 hover:bg-amber-600/20">Confirmed</Badge>;
      case 'DRAFT':
        return <Badge variant="outline">Draft</Badge>;
      default:
        return <Badge variant="secondary">{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6 p-6">
      {/* Header */}
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Purchase Orders & Advance Terms</h1>
          <p className="text-sm text-muted-foreground">
            Manage purchase orders, line items, and staged milestone payment schedules (100% advance, 40-40-20, 30-30-40, 50-50).
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={loadData}>
            <RefreshCw className="mr-2 h-4 w-4" /> Refresh
          </Button>
          {canWrite && (
            <Button size="sm" onClick={handleOpenCreate}>
              <Plus className="mr-2 h-4 w-4" /> New Purchase Order
            </Button>
          )}
        </div>
      </div>

      {/* KPI Overview Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card className="border-border/50">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Total Active Orders</CardTitle>
            <FileSpreadsheet className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalCount}</div>
            <p className="text-xs text-muted-foreground">Across all registered suppliers</p>
          </CardContent>
        </Card>

        <Card className="border-border/50">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Advance Terms Handled</CardTitle>
            <Clock className="h-4 w-4 text-amber-500" />
          </CardHeader>
          <CardContent>
            <div className="text-sm font-semibold flex flex-wrap gap-1">
              <span className="rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-800">100% Advance</span>
              <span className="rounded bg-blue-100 px-1.5 py-0.5 text-xs text-blue-800">40-40-20</span>
              <span className="rounded bg-purple-100 px-1.5 py-0.5 text-xs text-purple-800">30-30-40</span>
              <span className="rounded bg-teal-100 px-1.5 py-0.5 text-xs text-teal-800">50-50</span>
            </div>
            <p className="mt-1 text-xs text-muted-foreground">Staged payment milestones</p>
          </CardContent>
        </Card>

        <Card className="border-border/50">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Total Advance Paid</CardTitle>
            <DollarSign className="h-4 w-4 text-emerald-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-emerald-600">
              {orders.reduce((acc, o) => acc + (o.advancePaid || 0), 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
            </div>
            <p className="text-xs text-muted-foreground">Disbursed milestone advances</p>
          </CardContent>
        </Card>

        <Card className="border-border/50">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Remaining Advance Due</CardTitle>
            <AlertCircle className="h-4 w-4 text-orange-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-orange-600">
              {orders.reduce((acc, o) => acc + (o.advanceBalance || 0), 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
            </div>
            <p className="text-xs text-muted-foreground">Unpaid advance milestones</p>
          </CardContent>
        </Card>
      </div>

      {/* Filter Toolbar */}
      <Card>
        <CardContent className="p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex flex-1 flex-wrap items-center gap-2">
              <div className="relative min-w-[240px] flex-1 sm:max-w-xs">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search PO#, supplier, notes..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="pl-9 text-xs"
                />
              </div>

              <Select value={supplierFilter} onValueChange={setSupplierFilter}>
                <SelectTrigger className="w-[180px] text-xs">
                  <SelectValue placeholder="All Suppliers" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Suppliers</SelectItem>
                  {suppliers.map((s) => (
                    <SelectItem key={s.id} value={s.id}>{s.supplierName}</SelectItem>
                  ))}
                </SelectContent>
              </Select>

              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="w-[150px] text-xs">
                  <SelectValue placeholder="All Statuses" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Statuses</SelectItem>
                  <SelectItem value="CONFIRMED">Confirmed</SelectItem>
                  <SelectItem value="PARTIALLY_PAID">Partially Paid</SelectItem>
                  <SelectItem value="FULLY_PAID">Fully Paid</SelectItem>
                  <SelectItem value="DRAFT">Draft</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Orders Table */}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[140px]">PO Number</TableHead>
                <TableHead>Date</TableHead>
                <TableHead>Supplier</TableHead>
                <TableHead>Payment Terms</TableHead>
                <TableHead className="text-right">Total Amount</TableHead>
                <TableHead className="text-right">Advance Req.</TableHead>
                <TableHead className="text-right">Advance Paid</TableHead>
                <TableHead className="text-right">Advance Bal.</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={10} className="h-32 text-center text-sm text-muted-foreground">
                    Loading Purchase Orders...
                  </TableCell>
                </TableRow>
              ) : orders.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={10} className="h-32 text-center text-sm text-muted-foreground">
                    No purchase orders found matching your filters.
                  </TableCell>
                </TableRow>
              ) : (
                orders.map((po) => (
                  <TableRow key={po.id} className="hover:bg-muted/40">
                    <TableCell className="font-semibold text-primary">{po.poNumber}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{po.poDate}</TableCell>
                    <TableCell className="font-medium">{po.supplierName}</TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs">
                        {po.paymentTermsPreset.replace(/_/g, '-')}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right font-medium">
                      {po.currency} {po.totalAmount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </TableCell>
                    <TableCell className="text-right text-xs text-muted-foreground">
                      {po.currency} {po.advanceRequired.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </TableCell>
                    <TableCell className="text-right text-xs font-semibold text-emerald-600">
                      {po.currency} {po.advancePaid.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </TableCell>
                    <TableCell className="text-right text-xs font-semibold text-orange-600">
                      {po.currency} {po.advanceBalance.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </TableCell>
                    <TableCell>{getStatusBadge(po.status)}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => handleOpenView(po.id)}>
                          <Eye className="h-4 w-4" />
                        </Button>
                        {canDelete && po.totalPaid === 0 && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 w-8 p-0 text-destructive hover:bg-destructive/10"
                            onClick={() => handleDeletePO(po.id, po.poNumber)}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>

          <div className="flex items-center justify-between text-sm text-muted-foreground pt-4">
            <span>
              Page {page} of {Math.max(1, Math.ceil(totalCount / pageSize))}
            </span>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= Math.max(1, Math.ceil(totalCount / pageSize))}
                onClick={() => setPage((p) => Math.min(Math.max(1, Math.ceil(totalCount / pageSize)), p + 1))}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Create Purchase Order Modal */}
      <Dialog open={createModalOpen} onOpenChange={setCreateModalOpen}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Create New Purchase Order</DialogTitle>
            <DialogDescription>
              Configure line items and staged milestone payment terms (100% Advance, 40-40-20, 30-30-40, 50-50, or Custom).
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-2">
            {/* Header Form */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <Label className="text-xs">PO Number *</Label>
                <Input value={poNumber} onChange={(e) => setPoNumber(e.target.value)} className="text-xs mt-1" />
              </div>
              <div>
                <Label className="text-xs">PO Date *</Label>
                <Input type="date" value={poDate} onChange={(e) => setPoDate(e.target.value)} className="text-xs mt-1" />
              </div>
              <div>
                <Label className="text-xs">Supplier *</Label>
                <Select value={supplierId} onValueChange={setSupplierId}>
                  <SelectTrigger className="text-xs mt-1">
                    <SelectValue placeholder="Select supplier" />
                  </SelectTrigger>
                  <SelectContent>
                    {suppliers.map((s) => (
                      <SelectItem key={s.id} value={s.id}>{s.supplierName} ({s.country})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <div>
                <Label className="text-xs">Currency</Label>
                <Select value={currency} onValueChange={setCurrency}>
                  <SelectTrigger className="text-xs mt-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="USD">USD ($)</SelectItem>
                    <SelectItem value="EUR">EUR (€)</SelectItem>
                    <SelectItem value="INR">INR (₹)</SelectItem>
                    <SelectItem value="GBP">GBP (£)</SelectItem>
                    <SelectItem value="CNY">CNY (¥)</SelectItem>
                    <SelectItem value="JPY">JPY (¥)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs">Exchange Rate (to INR)</Label>
                <Input type="number" step="0.01" value={exchangeRate} onChange={(e) => setExchangeRate(Number(e.target.value))} className="text-xs mt-1" />
              </div>
              <div>
                <Label className="text-xs">Incoterm</Label>
                <Select value={incoterm} onValueChange={setIncoterm}>
                  <SelectTrigger className="text-xs mt-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="FOB">FOB (Free On Board)</SelectItem>
                    <SelectItem value="CIF">CIF (Cost, Insurance & Freight)</SelectItem>
                    <SelectItem value="EXW">EXW (Ex Works)</SelectItem>
                    <SelectItem value="CFR">CFR (Cost and Freight)</SelectItem>
                    <SelectItem value="DDP">DDP (Delivered Duty Paid)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs">Expected Delivery Date</Label>
                <Input type="date" value={expDeliveryDate} onChange={(e) => setExpDeliveryDate(e.target.value)} className="text-xs mt-1" />
              </div>
            </div>

            {/* Payment Milestone Preset */}
            <div className="rounded-lg border p-4 bg-muted/30">
              <Label className="text-xs font-semibold text-foreground">Payment Terms Milestone Preset *</Label>
              <div className="grid grid-cols-1 md:grid-cols-5 gap-2 mt-2">
                {[
                  { id: '100_ADVANCE', label: '100% Advance', desc: 'Full payment on PO confirmation' },
                  { id: '40_40_20', label: '40, 40, 20', desc: '40% Adv / 40% BL / 20% Delivery' },
                  { id: '30_30_40', label: '30, 30, 40', desc: '30% Adv / 30% BL / 40% Delivery' },
                  { id: '50_50', label: '50, 50', desc: '50% Adv / 50% on BL' },
                  { id: 'CUSTOM', label: 'Custom %', desc: 'Define custom stages' },
                ].map((term) => (
                  <button
                    key={term.id}
                    type="button"
                    onClick={() => setPreset(term.id)}
                    className={`flex flex-col items-start p-2.5 rounded-md border text-left transition-colors ${
                      preset === term.id
                        ? 'border-primary bg-primary/10 text-primary font-medium ring-1 ring-primary'
                        : 'border-border bg-card hover:bg-muted/50 text-foreground'
                    }`}
                  >
                    <span className="text-xs font-bold">{term.label}</span>
                    <span className="text-[10px] text-muted-foreground mt-0.5">{term.desc}</span>
                  </button>
                ))}
              </div>

              {/* Milestone Breakdown Preview */}
              <div className="mt-3 text-xs text-muted-foreground">
                <span className="font-semibold text-foreground">Calculated Milestones ({currency}): </span>
                {preset === '100_ADVANCE' && (
                  <span className="text-emerald-700 font-medium">Stage 1: 100% Advance = {currency} {computedTotal.toFixed(2)}</span>
                )}
                {preset === '40_40_20' && (
                  <span>
                    <strong className="text-blue-700">Stage 1 (40% Adv):</strong> {currency} {(computedTotal * 0.4).toFixed(2)} |{' '}
                    <strong className="text-purple-700">Stage 2 (40% BL):</strong> {currency} {(computedTotal * 0.4).toFixed(2)} |{' '}
                    <strong className="text-emerald-700">Stage 3 (20% Delivery):</strong> {currency} {(computedTotal * 0.2).toFixed(2)}
                  </span>
                )}
                {preset === '30_30_40' && (
                  <span>
                    <strong className="text-blue-700">Stage 1 (30% Adv):</strong> {currency} {(computedTotal * 0.3).toFixed(2)} |{' '}
                    <strong className="text-purple-700">Stage 2 (30% BL):</strong> {currency} {(computedTotal * 0.3).toFixed(2)} |{' '}
                    <strong className="text-emerald-700">Stage 3 (40% Delivery):</strong> {currency} {(computedTotal * 0.4).toFixed(2)}
                  </span>
                )}
                {preset === '50_50' && (
                  <span>
                    <strong className="text-blue-700">Stage 1 (50% Adv):</strong> {currency} {(computedTotal * 0.5).toFixed(2)} |{' '}
                    <strong className="text-purple-700">Stage 2 (50% Delivery):</strong> {currency} {(computedTotal * 0.5).toFixed(2)}
                  </span>
                )}
              </div>
            </div>

            {/* Line Items Table */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label className="text-xs font-semibold">Ordered Line Items ({lines.length})</Label>
                <Button type="button" variant="outline" size="sm" onClick={addLine} className="h-7 text-xs">
                  <Plus className="mr-1 h-3 w-3" /> Add Item
                </Button>
              </div>

              <div className="rounded-md border overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/50 text-[11px]">
                      <TableHead className="w-[180px]">Select Master Item</TableHead>
                      <TableHead>Part Number *</TableHead>
                      <TableHead>Description</TableHead>
                      <TableHead className="w-[90px]">Qty *</TableHead>
                      <TableHead className="w-[70px]">Unit</TableHead>
                      <TableHead className="w-[110px]">Unit Price *</TableHead>
                      <TableHead className="w-[110px] text-right">Line Total</TableHead>
                      <TableHead className="w-[50px]"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {lines.map((line, idx) => {
                      const lineTotal = (Number(line.quantity) || 0) * (Number(line.unitPrice) || 0);
                      return (
                        <TableRow key={idx}>
                          <TableCell className="p-1.5">
                            <Select onValueChange={(val) => handleSelectCatalogItem(idx, val)}>
                              <SelectTrigger className="h-7 text-[11px]">
                                <SelectValue placeholder="Item Master..." />
                              </SelectTrigger>
                              <SelectContent>
                                {itemsCatalog.map((it) => (
                                  <SelectItem key={it.id} value={it.id}>
                                    {it.partNumber} - {it.itemDescription}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </TableCell>
                          <TableCell className="p-1.5">
                            <Input
                              value={line.partNumber}
                              onChange={(e) => handleLineChange(idx, 'partNumber', e.target.value)}
                              placeholder="Part #"
                              className="h-7 text-xs"
                            />
                          </TableCell>
                          <TableCell className="p-1.5">
                            <Input
                              value={line.itemDescription}
                              onChange={(e) => handleLineChange(idx, 'itemDescription', e.target.value)}
                              placeholder="Description"
                              className="h-7 text-xs"
                            />
                          </TableCell>
                          <TableCell className="p-1.5">
                            <Input
                              type="number"
                              min="1"
                              value={line.quantity}
                              onChange={(e) => handleLineChange(idx, 'quantity', Number(e.target.value))}
                              className="h-7 text-xs text-right"
                            />
                          </TableCell>
                          <TableCell className="p-1.5">
                            <Input
                              value={line.unit}
                              onChange={(e) => handleLineChange(idx, 'unit', e.target.value)}
                              className="h-7 text-xs"
                            />
                          </TableCell>
                          <TableCell className="p-1.5">
                            <Input
                              type="number"
                              step="0.01"
                              value={line.unitPrice}
                              onChange={(e) => handleLineChange(idx, 'unitPrice', Number(e.target.value))}
                              className="h-7 text-xs text-right"
                            />
                          </TableCell>
                          <TableCell className="p-1.5 text-right font-medium text-xs">
                            {currency} {lineTotal.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                          </TableCell>
                          <TableCell className="p-1.5 text-center">
                            {lines.length > 1 && (
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="h-7 w-7 p-0 text-destructive"
                                onClick={() => removeLine(idx)}
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>

              <div className="flex justify-end p-2 bg-muted/20 rounded font-semibold text-sm">
                Total PO Value: {currency} {computedTotal.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
              </div>
            </div>

            <div>
              <Label className="text-xs">Notes & Commercial Terms</Label>
              <Input value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Delivery instructions, bank details, warranty terms..." className="text-xs mt-1" />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateModalOpen(false)}>Cancel</Button>
            <Button onClick={handleSubmitPO} disabled={submitting}>
              {submitting ? 'Creating PO...' : 'Save Purchase Order'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* View PO Detail Modal */}
      {viewPO && (
        <Dialog open={!!viewPO} onOpenChange={() => setViewPO(null)}>
          <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
            <DialogHeader>
              <div className="flex items-center justify-between">
                <div>
                  <DialogTitle className="text-lg flex items-center gap-2">
                    Purchase Order: {viewPO.poNumber} {getStatusBadge(viewPO.status)}
                  </DialogTitle>
                  <DialogDescription>
                    PO Date: {viewPO.poDate} | Supplier: {viewPO.supplierName} | Terms: {viewPO.paymentTermsPreset.replace(/_/g, '-')}
                  </DialogDescription>
                </div>
              </div>
            </DialogHeader>

            <div className="space-y-4 py-2 text-xs">
              {/* Financial Snapshot */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 bg-muted/40 p-3 rounded-lg">
                <div>
                  <span className="text-muted-foreground">Order Total:</span>
                  <div className="text-sm font-bold">{viewPO.currency} {viewPO.totalAmount.toLocaleString(undefined, { minimumFractionDigits: 2 })}</div>
                </div>
                <div>
                  <span className="text-muted-foreground">Advance Required:</span>
                  <div className="text-sm font-semibold text-blue-600">{viewPO.currency} {viewPO.advanceRequired.toLocaleString(undefined, { minimumFractionDigits: 2 })}</div>
                </div>
                <div>
                  <span className="text-muted-foreground">Advance Paid:</span>
                  <div className="text-sm font-semibold text-emerald-600">{viewPO.currency} {viewPO.advancePaid.toLocaleString(undefined, { minimumFractionDigits: 2 })}</div>
                </div>
                <div>
                  <span className="text-muted-foreground">Advance Balance Due:</span>
                  <div className="text-sm font-semibold text-orange-600">{viewPO.currency} {viewPO.advanceBalance.toLocaleString(undefined, { minimumFractionDigits: 2 })}</div>
                </div>
              </div>

              {/* Milestones Schedule */}
              <div>
                <h4 className="font-semibold text-sm mb-2">Payment Milestones Schedule</h4>
                <div className="border rounded-md overflow-hidden">
                  <Table>
                    <TableHeader className="bg-muted/50 text-[11px]">
                      <TableRow>
                        <TableHead className="w-[40px]">#</TableHead>
                        <TableHead>Stage Name</TableHead>
                        <TableHead>Stage Type</TableHead>
                        <TableHead className="text-right">%</TableHead>
                        <TableHead className="text-right">Milestone Amount</TableHead>
                        <TableHead className="text-right">Paid Amount</TableHead>
                        <TableHead className="text-right">Balance</TableHead>
                        <TableHead>Status</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {viewPO.milestones?.map((ms) => (
                        <TableRow key={ms.id}>
                          <TableCell className="font-medium">{ms.sequence}</TableCell>
                          <TableCell className="font-semibold">{ms.milestoneName}</TableCell>
                          <TableCell><Badge variant="outline">{ms.stageType}</Badge></TableCell>
                          <TableCell className="text-right">{ms.percentage}%</TableCell>
                          <TableCell className="text-right font-medium">
                            {viewPO.currency} {ms.amount.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                          </TableCell>
                          <TableCell className="text-right font-semibold text-emerald-600">
                            {viewPO.currency} {ms.paidAmount.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                          </TableCell>
                          <TableCell className="text-right font-semibold text-orange-600">
                            {viewPO.currency} {ms.balanceAmount.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                          </TableCell>
                          <TableCell>
                            {ms.status === 'PAID' ? (
                              <Badge className="bg-emerald-600/15 text-emerald-700">Paid</Badge>
                            ) : ms.status === 'PARTIALLY_PAID' ? (
                              <Badge className="bg-blue-600/15 text-blue-700">Partially Paid</Badge>
                            ) : (
                              <Badge variant="outline">Pending</Badge>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </div>

              {/* Line Items & Material Status */}
              <div>
                <h4 className="font-semibold text-sm mb-2">Line Items & Fulfillment Tracking</h4>
                <div className="border rounded-md overflow-hidden">
                  <Table>
                    <TableHeader className="bg-muted/50 text-[11px]">
                      <TableRow>
                        <TableHead>Part Number</TableHead>
                        <TableHead>Description</TableHead>
                        <TableHead className="text-right">Ordered</TableHead>
                        <TableHead className="text-right">Adv. Covered</TableHead>
                        <TableHead className="text-right">Received</TableHead>
                        <TableHead className="text-right">Pending</TableHead>
                        <TableHead>Fulfillment Status</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {viewPO.lineItems?.map((li) => (
                        <TableRow key={li.id}>
                          <TableCell className="font-semibold text-primary">{li.partNumber}</TableCell>
                          <TableCell>{li.itemDescription}</TableCell>
                          <TableCell className="text-right font-medium">{li.quantity} {li.unit}</TableCell>
                          <TableCell className="text-right text-blue-600 font-medium">{li.advanceCoveredQuantity} {li.unit}</TableCell>
                          <TableCell className="text-right text-emerald-600 font-semibold">{li.receivedQuantity} {li.unit}</TableCell>
                          <TableCell className="text-right text-orange-600 font-semibold">{li.pendingQuantity} {li.unit}</TableCell>
                          <TableCell>
                            <Badge
                              variant="outline"
                              className={
                                li.fulfillmentStatus === 'FULLY_RECEIVED'
                                  ? 'border-emerald-600 text-emerald-700 bg-emerald-50'
                                  : li.fulfillmentStatus === 'PARTIALLY_RECEIVED'
                                  ? 'border-blue-600 text-blue-700 bg-blue-50'
                                  : 'border-amber-600 text-amber-700 bg-amber-50'
                              }
                            >
                              {li.fulfillmentStatus.replace(/_/g, ' ')}
                            </Badge>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </div>
            </div>

            <DialogFooter>
              <Button onClick={() => setViewPO(null)}>Close</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
