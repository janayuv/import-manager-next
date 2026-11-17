import * as React from 'react';
import { Link } from 'react-router-dom';
import { CheckCircle2, Eye, Lock, Pencil, Plus, RefreshCw, Search, Trash2 } from 'lucide-react';
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { useAuth } from '@/contexts/AuthContext';
import { useModuleSettings } from '@/contexts/ThemeContext';
import { cn } from '@/lib/utils';
import { itemService, type Item } from '@/services/master-service';
import {
  invoiceService,
  invoiceWizardService,
  type InvoiceDetail,
  type InvoiceLineItem,
  type InvoiceSummary,
} from '@/services/shipment-invoice-service';

function roundPrecision(val: number, decimals: number): number {
  if (decimals <= 0) return Math.round(val);
  const f = Math.pow(10, decimals);
  return Math.round(val * f) / f;
}

export function InvoicesPage() {
  const { hasPermission } = useAuth();
  const canDelete = hasPermission('data.delete');

  const { itemsPerPage: pageSize } = useModuleSettings('invoice');
  const [invoices, setInvoices] = React.useState<InvoiceSummary[]>([]);
  const [page, setPage] = React.useState(1);
  const [search, setSearch] = React.useState('');
  const [detail, setDetail] = React.useState<InvoiceDetail | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [actionLoading, setActionLoading] = React.useState<string | null>(null);

  // Line item inline editing states
  const [editingLine, setEditingLine] = React.useState<InvoiceLineItem | null>(null);
  const [editQty, setEditQty] = React.useState<number>(1);
  const [editUnitPrice, setEditUnitPrice] = React.useState<number>(0);
  const [editDutyPercent, setEditDutyPercent] = React.useState<number>(0);
  const [editSwsPercent, setEditSwsPercent] = React.useState<number>(0);
  const [editIgstPercent, setEditIgstPercent] = React.useState<number>(0);

  // Line item delete confirmation state
  const [lineToDelete, setLineToDelete] = React.useState<InvoiceLineItem | null>(null);

  // Add line item dialog states
  const [addLineOpen, setAddLineOpen] = React.useState(false);
  const [catalogItems, setCatalogItems] = React.useState<Item[]>([]);
  const [addItemId, setAddItemId] = React.useState('');
  const [addSearch, setAddSearch] = React.useState('');
  const [addQty, setAddQty] = React.useState<number>(1);
  const [addUnitPrice, setAddUnitPrice] = React.useState<number>(0);
  const [addDutyPercent, setAddDutyPercent] = React.useState<number>(0);
  const [addSwsPercent, setAddSwsPercent] = React.useState<number>(0);
  const [addIgstPercent, setAddIgstPercent] = React.useState<number>(0);

  const [lineActionLoading, setLineActionLoading] = React.useState(false);

  React.useEffect(() => {
    setPage(1);
  }, [pageSize, search]);

  const totalPages = Math.max(1, Math.ceil(invoices.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pagedInvoices = React.useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return invoices.slice(start, start + pageSize);
  }, [invoices, currentPage, pageSize]);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const rows = await invoiceService.list(search, '');
      setInvoices(rows);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const handleInspect = async (id: string) => {
    try {
      const d = await invoiceService.getDetail(id);
      setDetail(d);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleQuickFinalize = async (inv: InvoiceSummary) => {
    try {
      setActionLoading(inv.id);
      const d = await invoiceService.getDetail(inv.id);
      await invoiceWizardService.saveInvoice({
        invoiceId: d.invoice.id,
        shipmentId: d.invoice.shipmentId,
        finalize: true,
        freezeShipment: false,
        lineTotalDecimals: d.invoice.lineTotalDecimals,
        invoiceTotalDecimals: d.invoice.invoiceTotalDecimals,
        lines: (d.lines ?? []).map((ln) => ({
          itemId: ln.itemId,
          quantity: ln.quantity,
          unitPrice: ln.unitPrice,
          dutyPercent: ln.dutyPercent,
          swsPercent: ln.swsPercent,
          igstPercent: ln.igstPercent,
        })),
      });
      toast.success(`Invoice ${inv.id} finalized successfully!`);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setActionLoading(null);
    }
  };

  const handleDelete = async (inv: InvoiceSummary) => {
    try {
      await invoiceService.delete(inv.id);
      toast.success(`Moved invoice ${inv.id} to Recycle Bin`);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  // Open edit modal for a line item
  const handleOpenEditLine = (ln: InvoiceLineItem) => {
    setEditingLine(ln);
    setEditQty(ln.quantity);
    setEditUnitPrice(ln.unitPrice);
    setEditDutyPercent(ln.dutyPercent);
    setEditSwsPercent(ln.swsPercent);
    setEditIgstPercent(ln.igstPercent);
  };

  // Save changes to edited line item
  const handleSaveEditLine = async () => {
    if (!detail || !editingLine) return;
    if (editQty <= 0) {
      toast.error('Quantity must be greater than 0');
      return;
    }
    if (editUnitPrice < 0) {
      toast.error('Unit price cannot be negative');
      return;
    }
    setLineActionLoading(true);
    try {
      const updatedLines = (detail.lines ?? []).map((l) => {
        if (l.id === editingLine.id) {
          return {
            itemId: l.itemId,
            quantity: editQty,
            unitPrice: editUnitPrice,
            dutyPercent: editDutyPercent,
            swsPercent: editSwsPercent,
            igstPercent: editIgstPercent,
          };
        }
        return {
          itemId: l.itemId,
          quantity: l.quantity,
          unitPrice: l.unitPrice,
          dutyPercent: l.dutyPercent,
          swsPercent: l.swsPercent,
          igstPercent: l.igstPercent,
        };
      });

      const lineDec = detail.invoice.lineTotalDecimals;
      const invDec = detail.invoice.invoiceTotalDecimals;
      const computedSum = updatedLines.reduce(
        (acc, l) => acc + roundPrecision(l.quantity * l.unitPrice, lineDec),
        0,
      );
      const newTotal = roundPrecision(computedSum, invDec);
      const matches =
        Math.round(newTotal * 100) === Math.round(detail.invoice.shipmentInvoiceValue * 100);
      const willFinalize = detail.invoice.status === 'Finalized' && matches;

      const saved = await invoiceWizardService.saveInvoice({
        invoiceId: detail.invoice.id,
        shipmentId: detail.invoice.shipmentId,
        finalize: willFinalize,
        freezeShipment: false,
        lineTotalDecimals: lineDec,
        invoiceTotalDecimals: invDec,
        lines: updatedLines,
      });

      setDetail(saved);
      setEditingLine(null);
      await load();

      if (detail.invoice.status === 'Finalized' && !matches) {
        toast.warning(
          `Line item updated. New total (${newTotal.toFixed(invDec)}) does not match shipment invoice value (${detail.invoice.shipmentInvoiceValue.toFixed(2)}); invoice saved as Draft.`,
        );
      } else {
        toast.success(`Updated line item ${editingLine.partNumber} successfully`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLineActionLoading(false);
    }
  };

  // Confirm removal of line item
  const handleConfirmRemoveLine = async () => {
    if (!detail || !lineToDelete) return;
    if ((detail.lines ?? []).length <= 1) {
      toast.error('Cannot remove the only line item. An invoice must contain at least one line item.');
      return;
    }
    setLineActionLoading(true);
    try {
      const remainingLines = (detail.lines ?? [])
        .filter((l) => l.id !== lineToDelete.id)
        .map((l) => ({
          itemId: l.itemId,
          quantity: l.quantity,
          unitPrice: l.unitPrice,
          dutyPercent: l.dutyPercent,
          swsPercent: l.swsPercent,
          igstPercent: l.igstPercent,
        }));

      const lineDec = detail.invoice.lineTotalDecimals;
      const invDec = detail.invoice.invoiceTotalDecimals;
      const computedSum = remainingLines.reduce(
        (acc, l) => acc + roundPrecision(l.quantity * l.unitPrice, lineDec),
        0,
      );
      const newTotal = roundPrecision(computedSum, invDec);
      const matches =
        Math.round(newTotal * 100) === Math.round(detail.invoice.shipmentInvoiceValue * 100);
      const willFinalize = detail.invoice.status === 'Finalized' && matches;

      const saved = await invoiceWizardService.saveInvoice({
        invoiceId: detail.invoice.id,
        shipmentId: detail.invoice.shipmentId,
        finalize: willFinalize,
        freezeShipment: false,
        lineTotalDecimals: lineDec,
        invoiceTotalDecimals: invDec,
        lines: remainingLines,
      });

      setDetail(saved);
      const removedPart = lineToDelete.partNumber;
      setLineToDelete(null);
      await load();

      if (detail.invoice.status === 'Finalized' && !matches) {
        toast.warning(
          `Removed ${removedPart}. New total (${newTotal.toFixed(invDec)}) differs from shipment value (${detail.invoice.shipmentInvoiceValue.toFixed(2)}); invoice saved as Draft.`,
        );
      } else {
        toast.success(`Removed line item ${removedPart} successfully`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLineActionLoading(false);
    }
  };

  // Open Add Line Item modal
  const handleOpenAddLine = async () => {
    if (!detail) return;
    setAddLineOpen(true);
    setAddSearch('');
    setAddQty(1);
    try {
      const res = await itemService.list({
        search: '',
        supplierId: detail.invoice.supplierId || '',
        status: 'active',
        sortBy: 'partNumber',
        sortDir: 'asc',
        page: 1,
        pageSize: 500,
      });
      let itms = res?.items ?? [];
      if (itms.length === 0) {
        const allRes = await itemService.list({
          search: '',
          supplierId: '',
          status: 'active',
          sortBy: 'partNumber',
          sortDir: 'asc',
          page: 1,
          pageSize: 500,
        });
        itms = allRes?.items ?? [];
      }
      setCatalogItems(itms);
      const first = itms[0];
      if (first) {
        setAddItemId(first.id);
        setAddUnitPrice(first.unitPrice || 0);
        setAddDutyPercent(Number.parseFloat(first.bcd || '0') || 0);
        setAddSwsPercent(Number.parseFloat(first.sws || '0') || 0);
        setAddIgstPercent(Number.parseFloat(first.igst || '0') || 0);
      }
    } catch (err) {
      toast.error('Failed to load items: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  const handleSelectAddItem = (id: string) => {
    setAddItemId(id);
    const it = catalogItems.find((x) => x.id === id);
    if (it) {
      setAddUnitPrice(it.unitPrice || 0);
      setAddDutyPercent(Number.parseFloat(it.bcd || '0') || 0);
      setAddSwsPercent(Number.parseFloat(it.sws || '0') || 0);
      setAddIgstPercent(Number.parseFloat(it.igst || '0') || 0);
    }
  };

  const handleSaveAddLine = async () => {
    if (!detail || !addItemId) return;
    if (addQty <= 0) {
      toast.error('Quantity must be greater than 0');
      return;
    }
    if (addUnitPrice < 0) {
      toast.error('Unit price cannot be negative');
      return;
    }
    setLineActionLoading(true);
    try {
      const existingLines = (detail.lines ?? []).map((l) => ({
        itemId: l.itemId,
        quantity: l.quantity,
        unitPrice: l.unitPrice,
        dutyPercent: l.dutyPercent,
        swsPercent: l.swsPercent,
        igstPercent: l.igstPercent,
      }));
      const newLines = [
        ...existingLines,
        {
          itemId: addItemId,
          quantity: addQty,
          unitPrice: addUnitPrice,
          dutyPercent: addDutyPercent,
          swsPercent: addSwsPercent,
          igstPercent: addIgstPercent,
        },
      ];

      const lineDec = detail.invoice.lineTotalDecimals;
      const invDec = detail.invoice.invoiceTotalDecimals;
      const computedSum = newLines.reduce(
        (acc, l) => acc + roundPrecision(l.quantity * l.unitPrice, lineDec),
        0,
      );
      const newTotal = roundPrecision(computedSum, invDec);
      const matches =
        Math.round(newTotal * 100) === Math.round(detail.invoice.shipmentInvoiceValue * 100);
      const willFinalize = detail.invoice.status === 'Finalized' && matches;

      const saved = await invoiceWizardService.saveInvoice({
        invoiceId: detail.invoice.id,
        shipmentId: detail.invoice.shipmentId,
        finalize: willFinalize,
        freezeShipment: false,
        lineTotalDecimals: lineDec,
        invoiceTotalDecimals: invDec,
        lines: newLines,
      });

      setDetail(saved);
      setAddLineOpen(false);
      await load();

      const it = catalogItems.find((x) => x.id === addItemId);
      toast.success(`Added ${it?.partNumber || 'new line item'} to invoice`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLineActionLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Commercial Invoices</h1>
          <p className="text-sm text-muted-foreground">
            Draft &amp; finalized commercial invoices linked to shipments and BOE valuation.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => void load()} disabled={loading}>
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Button asChild>
            <Link to="/invoice-wizard">
              <Plus className="h-4 w-4" />
              Open Invoice Wizard
            </Link>
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base">Invoice Registry ({invoices.length})</CardTitle>
              <CardDescription>Compares line-item computed sums against shipment invoice value.</CardDescription>
            </div>
            <div className="relative w-64">
              <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-8"
                placeholder="Search invoice, shipment, supplier..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Invoice ID</TableHead>
                <TableHead>Shipment / Ref</TableHead>
                <TableHead>Supplier</TableHead>
                <TableHead>Lines</TableHead>
                <TableHead>Line Sum</TableHead>
                <TableHead>Shipment Value</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pagedInvoices.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center text-muted-foreground">
                    No commercial invoices recorded yet.
                  </TableCell>
                </TableRow>
              ) : (
                pagedInvoices.map((inv) => {
                  const matched =
                    Math.round(inv.computedTotal * 100) === Math.round(inv.shipmentInvoiceValue * 100);
                  return (
                    <TableRow key={inv.id}>
                      <TableCell className="font-mono text-xs font-semibold">
                        <span className="inline-flex items-center gap-1">
                          {inv.id}
                          {inv.shipmentFrozen && <Lock className="h-3 w-3 text-amber-600" />}
                        </span>
                      </TableCell>
                      <TableCell>
                        <div className="font-medium">{inv.invoiceNumber}</div>
                        <div className="font-mono text-xs text-muted-foreground">{inv.shipmentId}</div>
                      </TableCell>
                      <TableCell>{inv.supplierName}</TableCell>
                      <TableCell>{inv.lineCount}</TableCell>
                      <TableCell className="font-mono text-xs">
                        {inv.currency} {inv.computedTotal.toFixed(inv.invoiceTotalDecimals)}
                      </TableCell>
                      <TableCell className="font-mono text-xs">
                        <span>
                          {inv.currency} {inv.shipmentInvoiceValue.toFixed(2)}
                        </span>
                        {!matched && (
                          <Badge variant="warning" className="ml-2">
                            Delta {(inv.computedTotal - inv.shipmentInvoiceValue).toFixed(2)}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant={inv.status === 'Finalized' ? 'success' : 'secondary'}>
                          {inv.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-1">
                          <Button size="sm" variant="ghost" onClick={() => void handleInspect(inv.id)} title="View Details">
                            <Eye className="h-4 w-4" />
                          </Button>
                          {!inv.shipmentFrozen && (
                            <Button
                              size="sm"
                              variant="ghost"
                              asChild
                              title={inv.status === 'Draft' ? 'Edit Draft in Wizard' : 'Edit Invoice in Wizard'}
                            >
                              <Link to={`/invoice-wizard?id=${inv.id}`}>
                                <Pencil
                                  className={cn(
                                    'h-4 w-4',
                                    inv.status === 'Draft' ? 'text-primary' : 'text-blue-600 dark:text-blue-400',
                                  )}
                                />
                              </Link>
                            </Button>
                          )}
                          {inv.status === 'Draft' && !inv.shipmentFrozen && matched && (
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => void handleQuickFinalize(inv)}
                              disabled={actionLoading === inv.id}
                              title="Finalize Invoice (Exact Cent Match)"
                            >
                              <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                            </Button>
                          )}
                          {canDelete && !inv.shipmentFrozen && (
                            <Button size="sm" variant="ghost" onClick={() => void handleDelete(inv)} title="Delete">
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>

          <div className="flex items-center justify-between text-sm text-muted-foreground pt-4">
            <span>
              Page {currentPage} of {totalPages}
            </span>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={currentPage <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={currentPage >= totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Invoice Detail Dialog with direct Line Item Edit, Remove, and Add actions */}
      <Dialog open={Boolean(detail)} onOpenChange={(o) => !o && setDetail(null)}>
        <DialogContent className="max-w-4xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <DialogTitle className="text-lg">
                Invoice {detail?.invoice.id} — {detail?.invoice.invoiceNumber}
              </DialogTitle>
              {detail?.invoice.shipmentFrozen && (
                <Badge variant="warning" className="flex items-center gap-1">
                  <Lock className="h-3 w-3" /> Shipment Frozen (Read-only)
                </Badge>
              )}
            </div>
            <DialogDescription>
              {detail?.invoice.supplierName} • Precision: {detail?.invoice.lineTotalDecimals}d /{' '}
              {detail?.invoice.invoiceTotalDecimals}d • Status: {detail?.invoice.status}
            </DialogDescription>
          </DialogHeader>
          {detail && (
            <div className="space-y-4">
              <div className="flex items-center justify-between border-b pb-2">
                <div>
                  <h4 className="text-sm font-semibold">
                    Line Items ({(detail.lines ?? []).length})
                  </h4>
                  <p className="text-xs text-muted-foreground">
                    Edit quantity/price or remove individual line items below.
                  </p>
                </div>
                {!detail.invoice.shipmentFrozen && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void handleOpenAddLine()}
                    className="h-8 gap-1.5 text-xs"
                    title="Add a new catalog item to this invoice"
                  >
                    <Plus className="h-3.5 w-3.5" />
                    Add Line Item
                  </Button>
                )}
              </div>

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Part No</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Qty</TableHead>
                    <TableHead>Unit Price</TableHead>
                    <TableHead>Line Total</TableHead>
                    <TableHead>BCD/SWS/IGST</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(detail.lines ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center text-muted-foreground py-6">
                        No line items on this invoice. Click &quot;Add Line Item&quot; to add one.
                      </TableCell>
                    </TableRow>
                  ) : (
                    (detail.lines ?? []).map((ln) => (
                      <TableRow key={ln.id}>
                        <TableCell className="font-mono text-xs font-semibold">{ln.partNumber}</TableCell>
                        <TableCell>{ln.itemDescription}</TableCell>
                        <TableCell className="font-mono text-xs">
                          {ln.quantity} {ln.unit}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {ln.unitPrice.toFixed(2)}
                          {ln.priceDiverged && (
                            <Badge variant="warning" className="ml-1 text-[10px]">
                              Master {ln.masterUnitPrice.toFixed(2)}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs font-medium">
                          {ln.lineTotal.toFixed(detail.invoice.lineTotalDecimals)}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {ln.dutyPercent}% / {ln.swsPercent}% / {ln.igstPercent}%
                        </TableCell>
                        <TableCell className="text-right">
                          {!detail.invoice.shipmentFrozen ? (
                            <div className="flex justify-end gap-1">
                              <Button
                                size="sm"
                                variant="ghost"
                                className="h-7 w-7 p-0"
                                onClick={() => handleOpenEditLine(ln)}
                                title="Edit line item"
                                aria-label={`Edit line item ${ln.partNumber}`}
                              >
                                <Pencil className="h-3.5 w-3.5 text-primary" />
                              </Button>
                              <Button
                                size="sm"
                                variant="ghost"
                                className="h-7 w-7 p-0"
                                onClick={() => setLineToDelete(ln)}
                                title="Remove line item"
                                aria-label={`Remove line item ${ln.partNumber}`}
                              >
                                <Trash2 className="h-3.5 w-3.5 text-destructive" />
                              </Button>
                            </div>
                          ) : (
                            <span className="text-[11px] text-muted-foreground flex items-center justify-end gap-1">
                              <Lock className="h-3 w-3" /> Locked
                            </span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>

              <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-3">
                <div className="text-xs text-muted-foreground">
                  Shipment Value: {detail.invoice.currency} {detail.invoice.shipmentInvoiceValue.toFixed(2)} • Line Total: {detail.invoice.currency} {detail.invoice.computedTotal.toFixed(detail.invoice.invoiceTotalDecimals)}
                  {Math.round(detail.invoice.computedTotal * 100) !==
                    Math.round(detail.invoice.shipmentInvoiceValue * 100) && (
                    <Badge variant="warning" className="ml-2">
                      Delta {(detail.invoice.computedTotal - detail.invoice.shipmentInvoiceValue).toFixed(2)}
                    </Badge>
                  )}
                </div>
                <div className="flex gap-2">
                  {!detail.invoice.shipmentFrozen && (
                    <Button variant="outline" size="sm" asChild>
                      <Link to={`/invoice-wizard?id=${detail.invoice.id}`}>
                        <Pencil className="mr-1.5 h-4 w-4" />
                        Open in Invoice Wizard
                      </Link>
                    </Button>
                  )}
                  {detail.invoice.status === 'Draft' && !detail.invoice.shipmentFrozen && (
                    <Button
                      size="sm"
                      disabled={
                        actionLoading === detail.invoice.id ||
                        Math.round(detail.invoice.computedTotal * 100) !==
                          Math.round(detail.invoice.shipmentInvoiceValue * 100)
                      }
                      onClick={async () => {
                        await handleQuickFinalize(detail.invoice);
                        setDetail(null);
                      }}
                    >
                      <CheckCircle2 className="mr-1.5 h-4 w-4" />
                      Finalize Invoice
                    </Button>
                  )}
                  <Button variant="outline" size="sm" onClick={() => setDetail(null)}>
                    Close
                  </Button>
                </div>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Edit Line Item Modal */}
      <Dialog open={Boolean(editingLine)} onOpenChange={(o) => !o && setEditingLine(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Pencil className="h-4 w-4 text-primary" />
              Edit Line Item: {editingLine?.partNumber}
            </DialogTitle>
            <DialogDescription>
              {editingLine?.itemDescription}
              {editingLine?.masterUnitPrice ? ` • Catalog Price: ${editingLine.masterUnitPrice.toFixed(2)}` : ''}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="edit-line-qty">Quantity *</Label>
                <Input
                  id="edit-line-qty"
                  type="number"
                  step="0.001"
                  min="0.001"
                  value={editQty}
                  onChange={(e) => setEditQty(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="edit-line-price">Unit Price ({detail?.invoice.currency}) *</Label>
                <Input
                  id="edit-line-price"
                  type="number"
                  step="0.0001"
                  min="0"
                  value={editUnitPrice}
                  onChange={(e) => setEditUnitPrice(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
            </div>

            <div className="grid grid-cols-3 gap-2">
              <div className="space-y-1.5">
                <Label htmlFor="edit-line-bcd" className="text-xs">BCD (%)</Label>
                <Input
                  id="edit-line-bcd"
                  type="number"
                  step="0.1"
                  value={editDutyPercent}
                  onChange={(e) => setEditDutyPercent(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="edit-line-sws" className="text-xs">SWS (%)</Label>
                <Input
                  id="edit-line-sws"
                  type="number"
                  step="0.1"
                  value={editSwsPercent}
                  onChange={(e) => setEditSwsPercent(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="edit-line-igst" className="text-xs">IGST (%)</Label>
                <Input
                  id="edit-line-igst"
                  type="number"
                  step="0.1"
                  value={editIgstPercent}
                  onChange={(e) => setEditIgstPercent(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
            </div>

            <div className="rounded-md border bg-muted/40 p-3 space-y-1 text-xs">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Computed Line Total:</span>
                <span className="font-mono font-semibold">
                  {detail?.invoice.currency}{' '}
                  {roundPrecision(editQty * editUnitPrice, detail?.invoice.lineTotalDecimals ?? 2).toFixed(
                    detail?.invoice.lineTotalDecimals ?? 2,
                  )}
                </span>
              </div>
              {detail && (
                <div className="flex justify-between border-t border-border/40 pt-1 text-[11px] text-muted-foreground">
                  <span>Shipment Invoice Value:</span>
                  <span className="font-mono">
                    {detail.invoice.currency} {detail.invoice.shipmentInvoiceValue.toFixed(2)}
                  </span>
                </div>
              )}
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button variant="outline" onClick={() => setEditingLine(null)} disabled={lineActionLoading}>
              Cancel
            </Button>
            <Button
              onClick={() => void handleSaveEditLine()}
              disabled={lineActionLoading || editQty <= 0 || editUnitPrice < 0}
            >
              {lineActionLoading ? 'Saving...' : 'Save Changes'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Remove Line Item Confirmation Modal */}
      <Dialog open={Boolean(lineToDelete)} onOpenChange={(o) => !o && setLineToDelete(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-destructive">
              <Trash2 className="h-4 w-4" />
              Remove Line Item
            </DialogTitle>
            <DialogDescription>
              Confirm removal of line item from commercial invoice {detail?.invoice.id}.
            </DialogDescription>
          </DialogHeader>

          {detail && (detail.lines ?? []).length <= 1 ? (
            <div className="space-y-3 py-2 text-sm">
              <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-amber-700 dark:text-amber-400">
                <p className="font-semibold">Cannot remove the only line item</p>
                <p className="mt-1 text-xs">
                  A commercial invoice must have at least one line item. If you wish to delete the entire invoice, please use the Delete Invoice action from the main invoice registry.
                </p>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setLineToDelete(null)}>
                  Close
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <div className="space-y-3 py-2 text-sm">
              <p>
                Are you sure you want to remove line item{' '}
                <strong className="font-mono">{lineToDelete?.partNumber}</strong> (
                {lineToDelete?.itemDescription})?
              </p>
              <div className="rounded-md border bg-muted/40 p-2 text-xs space-y-1">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Quantity:</span>
                  <span className="font-mono">
                    {lineToDelete?.quantity} {lineToDelete?.unit}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Line Total to be removed:</span>
                  <span className="font-mono font-medium text-destructive">
                    - {detail?.invoice.currency}{' '}
                    {lineToDelete?.lineTotal.toFixed(detail?.invoice.lineTotalDecimals ?? 2)}
                  </span>
                </div>
              </div>
              <DialogFooter className="gap-2 sm:gap-0">
                <Button variant="outline" onClick={() => setLineToDelete(null)} disabled={lineActionLoading}>
                  Cancel
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => void handleConfirmRemoveLine()}
                  disabled={lineActionLoading}
                >
                  {lineActionLoading ? 'Removing...' : 'Remove Line Item'}
                </Button>
              </DialogFooter>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Add Line Item Modal */}
      <Dialog open={addLineOpen} onOpenChange={(o) => !o && setAddLineOpen(false)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Plus className="h-4 w-4 text-primary" />
              Add Line Item to Invoice {detail?.invoice.id}
            </DialogTitle>
            <DialogDescription>
              Select a catalog item and enter quantity and pricing details.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <Label htmlFor="add-line-search">Select Catalog Item *</Label>
                <span className="text-[11px] text-muted-foreground">
                  {detail?.invoice.supplierName || 'All Suppliers'}
                </span>
              </div>
              <div className="relative">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground pointer-events-none" />
                <Input
                  id="add-line-search"
                  placeholder="Search part number or description..."
                  value={addSearch}
                  onChange={(e) => setAddSearch(e.target.value)}
                  className="pl-8 text-xs"
                />
              </div>
              <div className="max-h-40 overflow-y-auto rounded-md border p-1 space-y-1">
                {catalogItems
                  .filter((it) => {
                    if (!addSearch.trim()) return true;
                    const q = addSearch.toLowerCase();
                    return (
                      (it.partNumber || '').toLowerCase().includes(q) ||
                      (it.itemDescription || '').toLowerCase().includes(q) ||
                      (it.hsnCode || '').toLowerCase().includes(q)
                    );
                  })
                  .map((it) => {
                    const isSelected = addItemId === it.id;
                    return (
                      <div
                        key={it.id}
                        onClick={() => handleSelectAddItem(it.id)}
                        className={cn(
                          'flex items-center justify-between p-2 rounded cursor-pointer text-xs transition-colors',
                          isSelected
                            ? 'bg-primary/10 border border-primary/40 font-medium'
                            : 'hover:bg-accent/40',
                        )}
                      >
                        <div className="min-w-0 pr-2">
                          <div className="font-mono font-semibold text-primary">{it.partNumber}</div>
                          <div className="text-[11px] text-muted-foreground truncate">{it.itemDescription}</div>
                        </div>
                        <div className="text-right shrink-0">
                          <div>
                            {it.currency} {it.unitPrice}
                          </div>
                          {it.hsnCode && (
                            <div className="text-[10px] text-muted-foreground">HSN: {it.hsnCode}</div>
                          )}
                        </div>
                      </div>
                    );
                  })}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="add-line-qty">Quantity *</Label>
                <Input
                  id="add-line-qty"
                  type="number"
                  step="0.001"
                  min="0.001"
                  value={addQty}
                  onChange={(e) => setAddQty(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="add-line-price">Unit Price ({detail?.invoice.currency}) *</Label>
                <Input
                  id="add-line-price"
                  type="number"
                  step="0.0001"
                  min="0"
                  value={addUnitPrice}
                  onChange={(e) => setAddUnitPrice(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
            </div>

            <div className="grid grid-cols-3 gap-2">
              <div className="space-y-1.5">
                <Label htmlFor="add-line-bcd" className="text-xs">BCD (%)</Label>
                <Input
                  id="add-line-bcd"
                  type="number"
                  step="0.1"
                  value={addDutyPercent}
                  onChange={(e) => setAddDutyPercent(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="add-line-sws" className="text-xs">SWS (%)</Label>
                <Input
                  id="add-line-sws"
                  type="number"
                  step="0.1"
                  value={addSwsPercent}
                  onChange={(e) => setAddSwsPercent(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="add-line-igst" className="text-xs">IGST (%)</Label>
                <Input
                  id="add-line-igst"
                  type="number"
                  step="0.1"
                  value={addIgstPercent}
                  onChange={(e) => setAddIgstPercent(Number.parseFloat(e.target.value) || 0)}
                />
              </div>
            </div>

            <div className="rounded-md border bg-muted/40 p-2.5 text-xs flex justify-between">
              <span className="text-muted-foreground">Line Total:</span>
              <span className="font-mono font-semibold">
                {detail?.invoice.currency}{' '}
                {roundPrecision(addQty * addUnitPrice, detail?.invoice.lineTotalDecimals ?? 2).toFixed(
                  detail?.invoice.lineTotalDecimals ?? 2,
                )}
              </span>
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button variant="outline" onClick={() => setAddLineOpen(false)} disabled={lineActionLoading}>
              Cancel
            </Button>
            <Button
              onClick={() => void handleSaveAddLine()}
              disabled={lineActionLoading || !addItemId || addQty <= 0 || addUnitPrice < 0}
            >
              {lineActionLoading ? 'Adding...' : 'Add Line Item'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
