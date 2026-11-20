import * as React from 'react';
import {
  AlertTriangle,
  Boxes,
  ChevronDown,
  ChevronRight,
  Clock,
  DollarSign,
  Link as LinkIcon,
  PackageSearch,
  RefreshCw,
  Scale,
  Search,
  Truck,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { supplierService, type Supplier } from '@/services/master-service';
import {
  paymentPOService,
  type AdvanceReconciliationSummary,
  type MaterialTrackingItem,
  type MaterialTrackingSummary,
} from '@/services/payment-po-service';
import { invoiceService, type InvoiceSummary } from '@/services/shipment-invoice-service';

export function AdvanceReconciliationPage() {
  const [activeTab, setActiveTab] = React.useState<'reconciliation' | 'material_tracking'>('reconciliation');
  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);
  const [supplierFilter, setSupplierFilter] = React.useState('all');
  const [search, setSearch] = React.useState('');
  const [loading, setLoading] = React.useState(true);

  // Tab 1: Advance Reconciliation Data
  const [reconciliations, setReconciliations] = React.useState<AdvanceReconciliationSummary[]>([]);
  const [expandedPOs, setExpandedPOs] = React.useState<Record<string, boolean>>({});

  // Link Invoice to PO Modal
  const [linkModalOpen, setLinkModalOpen] = React.useState(false);
  const [linkingPOId, setLinkingPOId] = React.useState('');
  const [unlinkedInvoices, setUnlinkedInvoices] = React.useState<InvoiceSummary[]>([]);
  const [selectedInvoiceIdToLink, setSelectedInvoiceIdToLink] = React.useState('');
  const [linkingSubmitting, setLinkingSubmitting] = React.useState(false);

  // Tab 2: Material Tracking Data
  const [materialItems, setMaterialItems] = React.useState<MaterialTrackingItem[]>([]);
  const [materialSummary, setMaterialSummary] = React.useState<MaterialTrackingSummary>({
    totalPoTracked: 0,
    totalItemsTracked: 0,
    totalAdvanceCapitalLocked: 0,
    totalPendingQuantity: 0,
    totalAdvancePendingQuantity: 0,
    delayedItemsCount: 0,
  });

  // Inward Receipt Modal
  const [receiptModalOpen, setReceiptModalOpen] = React.useState(false);
  const [receiptLineItem, setReceiptLineItem] = React.useState<MaterialTrackingItem | null>(null);
  const [receiptQty, setReceiptQty] = React.useState<number>(0);
  const [receiptDate, setReceiptDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [receiptRemarks, setReceiptRemarks] = React.useState('');
  const [receiptSubmitting, setReceiptSubmitting] = React.useState(false);

  const loadData = React.useCallback(async () => {
    setLoading(true);
    try {
      const sup = supplierFilter === 'all' ? '' : supplierFilter;

      if (activeTab === 'reconciliation') {
        const rows = await paymentPOService.getAdvanceReconciliation(sup);
        setReconciliations(rows || []);
      } else {
        const res = await paymentPOService.getMaterialTracking(sup, search.trim());
        setMaterialItems(res.items || []);
        setMaterialSummary(res.summary);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [activeTab, supplierFilter, search]);

  React.useEffect(() => {
    loadData();
  }, [loadData]);

  React.useEffect(() => {
    supplierService.list({ page: 1, pageSize: 200, search: '', status: '', sortBy: '', sortDir: '' }).then((res) => {
      setSuppliers(res.items || []);
    }).catch(() => {});
  }, []);

  const toggleExpand = (poId: string) => {
    setExpandedPOs((prev) => ({ ...prev, [poId]: !prev[poId] }));
  };

  const handleOpenLinkModal = async (poId: string) => {
    setLinkingPOId(poId);
    setSelectedInvoiceIdToLink('');
    setLinkModalOpen(true);
    try {
      // Fetch commercial invoices
      const allInvoices = await invoiceService.list('', '');
      setUnlinkedInvoices(allInvoices || []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleConfirmLinkInvoice = async () => {
    if (!selectedInvoiceIdToLink) {
      toast.error('Please select an invoice to link');
      return;
    }
    setLinkingSubmitting(true);
    try {
      await paymentPOService.linkInvoiceToPO(selectedInvoiceIdToLink, linkingPOId);
      toast.success('Invoice linked to Purchase Order successfully');
      setLinkModalOpen(false);
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLinkingSubmitting(false);
    }
  };

  const handleOpenReceiptModal = (item: MaterialTrackingItem) => {
    setReceiptLineItem(item);
    setReceiptQty(item.pendingQuantity);
    setReceiptDate(new Date().toISOString().slice(0, 10));
    setReceiptRemarks(`Inward receipt against ${item.poNumber}`);
    setReceiptModalOpen(true);
  };

  const handleSubmitReceipt = async () => {
    if (!receiptLineItem) return;
    if (receiptQty <= 0) {
      toast.error('Received quantity must be greater than zero');
      return;
    }
    setReceiptSubmitting(true);
    try {
      await paymentPOService.recordMaterialReceipt({
        poLineItemId: receiptLineItem.poLineItemId,
        quantityReceived: Number(receiptQty),
        receiptDate,
        remarks: receiptRemarks,
      });
      toast.success(`Received ${receiptQty} ${receiptLineItem.unit} of ${receiptLineItem.partNumber}`);
      setReceiptModalOpen(false);
      loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setReceiptSubmitting(false);
    }
  };

  const getVarianceBadge = (varianceType: string, varianceAmount: number, currency: string) => {
    switch (varianceType) {
      case 'MATCHED':
        return (
          <Badge className="bg-emerald-600/15 text-emerald-700 hover:bg-emerald-600/20 font-medium">
            Exact Match (0.00)
          </Badge>
        );
      case 'UNDER_INVOICED':
        return (
          <Badge className="bg-amber-600/15 text-amber-800 border-amber-300 font-medium">
            Excess Advance: +{currency} {Math.abs(varianceAmount).toFixed(2)}
          </Badge>
        );
      case 'OVER_INVOICED':
        return (
          <Badge className="bg-purple-600/15 text-purple-800 border-purple-300 font-medium">
            Balance Due: -{currency} {Math.abs(varianceAmount).toFixed(2)}
          </Badge>
        );
      case 'AWAITING_INVOICE':
      default:
        return (
          <Badge variant="outline" className="text-blue-700 border-blue-300 bg-blue-50/50">
            Awaiting Invoices
          </Badge>
        );
    }
  };

  return (
    <div className="space-y-6 p-6">
      {/* Header */}
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Advance Reconciliation & Material Tracker
          </h1>
          <p className="text-sm text-muted-foreground">
            Match PO advance payments against commercial invoices, highlight variances (under/over invoiced), and track undelivered advance-paid quantities.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={loadData}>
            <RefreshCw className="mr-2 h-4 w-4" /> Refresh
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={(val: any) => setActiveTab(val)}>
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <TabsList className="w-full sm:w-auto">
            <TabsTrigger value="reconciliation" className="text-xs gap-1.5">
              <Scale className="h-4 w-4 text-primary" /> Advance vs Invoice Reconciliation
            </TabsTrigger>
            <TabsTrigger value="material_tracking" className="text-xs gap-1.5">
              <Boxes className="h-4 w-4 text-blue-600" /> Advance-Paid Material Tracker
            </TabsTrigger>
          </TabsList>

          <div className="flex items-center gap-2">
            <Select value={supplierFilter} onValueChange={setSupplierFilter}>
              <SelectTrigger className="w-[200px] text-xs">
                <SelectValue placeholder="All Suppliers" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Suppliers</SelectItem>
                {suppliers.map((s) => (
                  <SelectItem key={s.id} value={s.id}>{s.supplierName}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        {/* ===================== TAB 1: RECONCILIATION ===================== */}
        <TabsContent value="reconciliation" className="space-y-4 mt-4">
          {/* Variance Legend Banner */}
          <div className="rounded-lg border bg-muted/20 p-3.5 flex flex-wrap items-center justify-between gap-3 text-xs">
            <div className="flex flex-wrap items-center gap-3">
              <span className="font-semibold text-foreground">Variance Engine Legend:</span>
              <span className="flex items-center gap-1.5">
                <span className="h-2.5 w-2.5 rounded-full bg-emerald-500" />
                <strong>Matched:</strong> Advance equals commercial invoice value
              </span>
              <span className="flex items-center gap-1.5">
                <span className="h-2.5 w-2.5 rounded-full bg-amber-500" />
                <strong>Under-Invoiced:</strong> Excess advance held by supplier (credit balance)
              </span>
              <span className="flex items-center gap-1.5">
                <span className="h-2.5 w-2.5 rounded-full bg-purple-500" />
                <strong>Over-Invoiced:</strong> Invoice exceeds advance (balance due on delivery)
              </span>
            </div>
            <div className="text-muted-foreground">
              Total POs Evaluated: <strong>{reconciliations.length}</strong>
            </div>
          </div>

          {/* Reconciliation Table */}
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[30px]"></TableHead>
                    <TableHead className="w-[140px]">PO Number</TableHead>
                    <TableHead>PO Date</TableHead>
                    <TableHead>Supplier</TableHead>
                    <TableHead className="text-right">PO Total Amount</TableHead>
                    <TableHead className="text-right">Advance Paid</TableHead>
                    <TableHead className="text-right">Total Invoiced</TableHead>
                    <TableHead className="text-right">Variance / Difference</TableHead>
                    <TableHead>Reconciliation Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={10} className="h-32 text-center text-sm text-muted-foreground">
                        Calculating reconciliation matrices...
                      </TableCell>
                    </TableRow>
                  ) : reconciliations.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={10} className="h-32 text-center text-sm text-muted-foreground">
                        No purchase orders found for reconciliation.
                      </TableCell>
                    </TableRow>
                  ) : (
                    reconciliations.map((rec) => {
                      const isExpanded = !!expandedPOs[rec.poId];
                      return (
                        <React.Fragment key={rec.poId}>
                          <TableRow className="hover:bg-muted/40 cursor-pointer" onClick={() => toggleExpand(rec.poId)}>
                            <TableCell className="p-2 text-center">
                              {isExpanded ? <ChevronDown className="h-4 w-4 text-muted-foreground" /> : <ChevronRight className="h-4 w-4 text-muted-foreground" />}
                            </TableCell>
                            <TableCell className="font-semibold text-primary">{rec.poNumber}</TableCell>
                            <TableCell className="text-xs text-muted-foreground">{rec.poDate}</TableCell>
                            <TableCell className="font-medium text-xs">{rec.supplierName}</TableCell>
                            <TableCell className="text-right font-medium text-xs">
                              {rec.currency} {rec.poTotalAmount.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                            </TableCell>
                            <TableCell className="text-right font-semibold text-emerald-600 text-xs">
                              {rec.currency} {rec.advancePaidAmount.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                            </TableCell>
                            <TableCell className="text-right font-semibold text-blue-600 text-xs">
                              {rec.currency} {rec.totalInvoicedAmount.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                            </TableCell>
                            <TableCell className="text-right">
                              {getVarianceBadge(rec.varianceType, rec.varianceAmount, rec.currency)}
                            </TableCell>
                            <TableCell>
                              <Badge
                                variant="outline"
                                className={
                                  rec.reconciliationStatus === 'FULLY_RECONCILED'
                                    ? 'border-emerald-600 text-emerald-700 bg-emerald-50'
                                    : rec.reconciliationStatus === 'DISCREPANCY'
                                    ? 'border-amber-600 text-amber-800 bg-amber-50'
                                    : 'border-blue-600 text-blue-700 bg-blue-50'
                                }
                              >
                                {rec.reconciliationStatus.replace(/_/g, ' ')}
                              </Badge>
                            </TableCell>
                            <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                              <Button
                                variant="outline"
                                size="sm"
                                className="h-7 text-xs gap-1"
                                onClick={() => handleOpenLinkModal(rec.poId)}
                              >
                                <LinkIcon className="h-3 w-3" /> Link Invoice
                              </Button>
                            </TableCell>
                          </TableRow>

                          {/* Expanded Invoices Breakdown */}
                          {isExpanded && (
                            <TableRow className="bg-muted/30">
                              <TableCell colSpan={10} className="p-3">
                                <div className="rounded-md border bg-card p-3 space-y-2">
                                  <div className="flex items-center justify-between text-xs">
                                    <span className="font-bold text-foreground">
                                      Commercial Invoices Linked to {rec.poNumber} ({rec.invoices.length})
                                    </span>
                                    <span className="text-muted-foreground">
                                      Advance Adjusted: <strong>{rec.currency} {rec.advanceAdjustedAmount.toFixed(2)}</strong> |{' '}
                                      Unadjusted Advance: <strong>{rec.currency} {rec.unadjustedAdvance.toFixed(2)}</strong>
                                    </span>
                                  </div>

                                  {rec.invoices.length === 0 ? (
                                    <div className="text-xs text-muted-foreground py-2 text-center">
                                      No commercial invoices currently attached to this purchase order. Use &quot;Link Invoice&quot; to attach.
                                    </div>
                                  ) : (
                                    <Table>
                                      <TableHeader className="text-[11px] bg-muted/60">
                                        <TableRow>
                                          <TableHead>Invoice Number</TableHead>
                                          <TableHead>Invoice Date</TableHead>
                                          <TableHead className="text-right">Invoice Value</TableHead>
                                          <TableHead className="text-right">Advance Settled</TableHead>
                                          <TableHead className="text-right">Direct Paid</TableHead>
                                          <TableHead className="text-right">Balance Due</TableHead>
                                          <TableHead>Status</TableHead>
                                        </TableRow>
                                      </TableHeader>
                                      <TableBody>
                                        {rec.invoices.map((inv) => (
                                          <TableRow key={inv.invoiceId} className="text-xs">
                                            <TableCell className="font-semibold text-primary">{inv.invoiceNumber}</TableCell>
                                            <TableCell className="text-muted-foreground">{inv.invoiceDate}</TableCell>
                                            <TableCell className="text-right font-medium">
                                              {inv.currency} {inv.invoiceValue.toFixed(2)}
                                            </TableCell>
                                            <TableCell className="text-right text-emerald-600 font-medium">
                                              {inv.advanceAllocated > 0 ? `${inv.currency} ${inv.advanceAllocated.toFixed(2)}` : '-'}
                                            </TableCell>
                                            <TableCell className="text-right text-blue-600 font-medium">
                                              {inv.directPaid > 0 ? `${inv.currency} ${inv.directPaid.toFixed(2)}` : '-'}
                                            </TableCell>
                                            <TableCell className="text-right font-bold text-orange-600">
                                              {inv.currency} {inv.balanceRemaining.toFixed(2)}
                                            </TableCell>
                                            <TableCell>
                                              <Badge variant="outline" className="text-[10px]">
                                                {inv.status}
                                              </Badge>
                                            </TableCell>
                                          </TableRow>
                                        ))}
                                      </TableBody>
                                    </Table>
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

        {/* ===================== TAB 2: MATERIAL TRACKING ===================== */}
        <TabsContent value="material_tracking" className="space-y-4 mt-4">
          {/* KPI Cards */}
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Card className="border-border/50">
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Locked Advance Capital</CardTitle>
                <DollarSign className="h-4 w-4 text-destructive" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-destructive">
                  {materialSummary.totalAdvanceCapitalLocked.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                </div>
                <p className="text-xs text-muted-foreground">Paid for in advance, goods not yet received</p>
              </CardContent>
            </Card>

            <Card className="border-border/50">
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Adv. Paid Pending Qty</CardTitle>
                <Clock className="h-4 w-4 text-orange-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-orange-600">
                  {materialSummary.totalAdvancePendingQuantity.toLocaleString()}
                </div>
                <p className="text-xs text-muted-foreground">Units funded by advance pending shipment</p>
              </CardContent>
            </Card>

            <Card className="border-border/50">
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Total Pending Order Qty</CardTitle>
                <PackageSearch className="h-4 w-4 text-blue-600" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-blue-600">
                  {materialSummary.totalPendingQuantity.toLocaleString()}
                </div>
                <p className="text-xs text-muted-foreground">Total undelivered units across POs</p>
              </CardContent>
            </Card>

            <Card className="border-border/50">
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Delayed Past ETA</CardTitle>
                <AlertTriangle className="h-4 w-4 text-amber-500" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-amber-600">
                  {materialSummary.delayedItemsCount}
                </div>
                <p className="text-xs text-muted-foreground">Items overdue based on delivery timeline</p>
              </CardContent>
            </Card>
          </div>

          {/* Search Toolbar */}
          <Card>
            <CardContent className="p-4">
              <div className="relative max-w-sm">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Filter part number, description, PO#..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="pl-9 text-xs"
                />
              </div>
            </CardContent>
          </Card>

          {/* Material Items Table */}
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>PO Number</TableHead>
                    <TableHead>Supplier</TableHead>
                    <TableHead>Part Number / Description</TableHead>
                    <TableHead className="text-right">Unit Price</TableHead>
                    <TableHead className="text-right">Ordered</TableHead>
                    <TableHead className="text-right">Adv. Covered</TableHead>
                    <TableHead className="text-right">Received</TableHead>
                    <TableHead className="text-right">Pending Qty</TableHead>
                    <TableHead className="text-right">Locked Capital</TableHead>
                    <TableHead>Fulfillment Status</TableHead>
                    <TableHead className="text-right">Inward</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={11} className="h-32 text-center text-sm text-muted-foreground">
                        Tracking advance paid materials...
                      </TableCell>
                    </TableRow>
                  ) : materialItems.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={11} className="h-32 text-center text-sm text-muted-foreground">
                        No materials tracked under current criteria.
                      </TableCell>
                    </TableRow>
                  ) : (
                    materialItems.map((item) => {
                      const pctReceived = Math.min(100, Math.round((item.receivedQuantity / (item.orderedQuantity || 1)) * 100));
                      return (
                        <TableRow key={item.poLineItemId} className="hover:bg-muted/40 text-xs">
                          <TableCell className="font-semibold text-primary">{item.poNumber}</TableCell>
                          <TableCell className="text-muted-foreground">{item.supplierName}</TableCell>
                          <TableCell>
                            <div className="font-bold">{item.partNumber}</div>
                            <div className="text-[11px] text-muted-foreground line-clamp-1">{item.itemDescription}</div>
                          </TableCell>
                          <TableCell className="text-right">
                            {item.currency} {item.unitPrice.toFixed(2)}
                          </TableCell>
                          <TableCell className="text-right font-medium">
                            {item.orderedQuantity} {item.unit}
                          </TableCell>
                          <TableCell className="text-right font-medium text-blue-700">
                            {item.advancePaidQuantity} {item.unit} ({item.advancePaidPercentage}%)
                          </TableCell>
                          <TableCell className="text-right font-semibold text-emerald-600">
                            {item.receivedQuantity} {item.unit}
                          </TableCell>
                          <TableCell className="text-right font-semibold text-orange-600">
                            {item.pendingQuantity} {item.unit}
                          </TableCell>
                          <TableCell className="text-right font-bold text-destructive">
                            {item.advanceLockedCapital > 0 ? `${item.currency} ${item.advanceLockedCapital.toFixed(2)}` : '-'}
                          </TableCell>
                          <TableCell>
                            <div className="space-y-1">
                              <Badge
                                variant="outline"
                                className={
                                  item.fulfillmentStatus === 'FULLY_RECEIVED'
                                    ? 'border-emerald-600 text-emerald-700 bg-emerald-50'
                                    : item.fulfillmentStatus === 'PARTIALLY_RECEIVED'
                                    ? 'border-blue-600 text-blue-700 bg-blue-50'
                                    : 'border-amber-600 text-amber-700 bg-amber-50'
                                }
                              >
                                {item.fulfillmentStatus.replace(/_/g, ' ')}
                              </Badge>
                              {item.isDelayed && (
                                <Badge className="bg-red-100 text-red-800 text-[10px] ml-1">
                                  {item.daysDelayed}d delayed
                                </Badge>
                              )}
                              <div className="h-1.5 w-24 bg-muted rounded-full overflow-hidden">
                                <div
                                  className="h-full bg-emerald-600 rounded-full"
                                  style={{ width: `${pctReceived}%` }}
                                />
                              </div>
                            </div>
                          </TableCell>
                          <TableCell className="text-right">
                            {item.pendingQuantity > 0 && (
                              <Button
                                variant="outline"
                                size="sm"
                                className="h-7 text-[11px]"
                                onClick={() => handleOpenReceiptModal(item)}
                              >
                                <Truck className="mr-1 h-3 w-3" /> Receive
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
      </Tabs>

      {/* Link Invoice to PO Modal */}
      <Dialog open={linkModalOpen} onOpenChange={setLinkModalOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Link Commercial Invoice to PO</DialogTitle>
            <DialogDescription>
              Select an unlinked commercial invoice to reconcile against this purchase order.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 py-2 text-xs">
            <div>
              <Label className="text-xs">Choose Invoice to Link *</Label>
              <Select value={selectedInvoiceIdToLink} onValueChange={setSelectedInvoiceIdToLink}>
                <SelectTrigger className="text-xs mt-1">
                  <SelectValue placeholder="Select Commercial Invoice..." />
                </SelectTrigger>
                <SelectContent>
                  {unlinkedInvoices.map((inv) => (
                    <SelectItem key={inv.id} value={inv.id}>
                      {inv.invoiceNumber} - Value: {inv.currency} {inv.shipmentInvoiceValue.toFixed(2)} ({inv.supplierName})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setLinkModalOpen(false)}>Cancel</Button>
            <Button onClick={handleConfirmLinkInvoice} disabled={linkingSubmitting}>
              {linkingSubmitting ? 'Linking...' : 'Confirm Link'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Inward Material Receipt Modal */}
      {receiptLineItem && (
        <Dialog open={receiptModalOpen} onOpenChange={setReceiptModalOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Record Inward Material Receipt</DialogTitle>
              <DialogDescription>
                Record partial or complete quantity arrival for {receiptLineItem.partNumber} ({receiptLineItem.poNumber}).
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3 py-2 text-xs">
              <div className="rounded-lg bg-muted/40 p-3 space-y-1">
                <div>Part: <strong>{receiptLineItem.partNumber}</strong> ({receiptLineItem.itemDescription})</div>
                <div>Ordered: <strong>{receiptLineItem.orderedQuantity} {receiptLineItem.unit}</strong></div>
                <div>Already Received: <strong>{receiptLineItem.receivedQuantity} {receiptLineItem.unit}</strong></div>
                <div>Pending Delivery: <strong className="text-orange-600">{receiptLineItem.pendingQuantity} {receiptLineItem.unit}</strong></div>
              </div>

              <div>
                <Label className="text-xs">Quantity Received Now ({receiptLineItem.unit}) *</Label>
                <Input
                  type="number"
                  min="0.1"
                  max={receiptLineItem.pendingQuantity}
                  value={receiptQty}
                  onChange={(e) => setReceiptQty(Number(e.target.value))}
                  className="text-xs mt-1 font-bold text-primary"
                />
              </div>

              <div>
                <Label className="text-xs">Receipt Date *</Label>
                <Input
                  type="date"
                  value={receiptDate}
                  onChange={(e) => setReceiptDate(e.target.value)}
                  className="text-xs mt-1"
                />
              </div>

              <div>
                <Label className="text-xs">Remarks / Delivery Challan Ref</Label>
                <Input
                  value={receiptRemarks}
                  onChange={(e) => setReceiptRemarks(e.target.value)}
                  placeholder="e.g. Challan #DC-4889, gate pass #GP-102"
                  className="text-xs mt-1"
                />
              </div>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setReceiptModalOpen(false)}>Cancel</Button>
              <Button onClick={handleSubmitReceipt} disabled={receiptSubmitting}>
                {receiptSubmitting ? 'Recording...' : 'Save Inward Receipt'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
