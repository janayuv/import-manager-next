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
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
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
import {
  invoiceService,
  invoiceWizardService,
  type InvoiceDetail,
  type InvoiceSummary,
} from '@/services/shipment-invoice-service';

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
                          {inv.status === 'Draft' && !inv.shipmentFrozen && (
                            <Button size="sm" variant="ghost" asChild title="Edit Draft in Wizard">
                              <Link to={`/invoice-wizard?id=${inv.id}`}>
                                <Pencil className="h-4 w-4 text-primary" />
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

      <Dialog open={Boolean(detail)} onOpenChange={(o) => !o && setDetail(null)}>
        <DialogContent className="max-w-3xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              Invoice {detail?.invoice.id} — {detail?.invoice.invoiceNumber}
            </DialogTitle>
            <DialogDescription>
              {detail?.invoice.supplierName} • Precision: {detail?.invoice.lineTotalDecimals}d /{' '}
              {detail?.invoice.invoiceTotalDecimals}d • Status: {detail?.invoice.status}
            </DialogDescription>
          </DialogHeader>
          {detail && (
            <div className="space-y-4">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Part No</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Qty</TableHead>
                    <TableHead>Unit Price</TableHead>
                    <TableHead>Line Total</TableHead>
                    <TableHead>BCD/SWS/IGST</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(detail.lines ?? []).map((ln) => (
                    <TableRow key={ln.id}>
                      <TableCell className="font-mono text-xs">{ln.partNumber}</TableCell>
                      <TableCell>{ln.itemDescription}</TableCell>
                      <TableCell>{ln.quantity}</TableCell>
                      <TableCell className="font-mono text-xs">
                        {ln.unitPrice.toFixed(2)}
                        {ln.priceDiverged && (
                          <Badge variant="warning" className="ml-1">
                            Master {ln.masterUnitPrice.toFixed(2)}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="font-mono text-xs">{ln.lineTotal.toFixed(detail.invoice.lineTotalDecimals)}</TableCell>
                      <TableCell className="font-mono text-xs">
                        {ln.dutyPercent}% / {ln.swsPercent}% / {ln.igstPercent}%
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-3">
                <div className="text-xs text-muted-foreground">
                  Shipment Value: {detail.invoice.currency} {detail.invoice.shipmentInvoiceValue.toFixed(2)} • Line Total: {detail.invoice.currency} {detail.invoice.computedTotal.toFixed(detail.invoice.invoiceTotalDecimals)}
                </div>
                <div className="flex gap-2">
                  {detail.invoice.status === 'Draft' && !detail.invoice.shipmentFrozen && (
                    <>
                      <Button variant="outline" size="sm" asChild>
                        <Link to={`/invoice-wizard?id=${detail.invoice.id}`}>
                          <Pencil className="mr-1.5 h-4 w-4" />
                          Edit in Wizard
                        </Link>
                      </Button>
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
                    </>
                  )}
                </div>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
