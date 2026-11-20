import * as React from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  FileDown,
  FileSpreadsheet,
  Pencil,
  Search,
  ShieldCheck,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
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
import { exportBOEExcel, exportBOEPdf } from '@/lib/boe-export';
import {
  boeSummaryService,
  type BOECalculationDetail,
} from '@/services/boe-service';

interface ViewReconciledBoeDialogProps {
  calcId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onEdit?: (calcId: string) => void;
  canEdit?: boolean;
}

export function ViewReconciledBoeDialog({
  calcId,
  open,
  onOpenChange,
  onEdit,
  canEdit = true,
}: ViewReconciledBoeDialogProps) {
  const [loading, setLoading] = React.useState(false);
  const [detail, setDetail] = React.useState<BOECalculationDetail | null>(null);
  const [itemSearch, setItemSearch] = React.useState('');

  React.useEffect(() => {
    setItemSearch('');
  }, [open, calcId]);

  const filteredItems = React.useMemo(() => {
    const list = detail?.result.items ?? [];
    const q = itemSearch.trim().toLowerCase();
    if (!q) return list;
    return list.filter(
      (item) =>
        (item.partNumber && item.partNumber.toLowerCase().includes(q)) ||
        (item.description && item.description.toLowerCase().includes(q)) ||
        (item.hsnCode && item.hsnCode.toLowerCase().includes(q)),
    );
  }, [detail?.result.items, itemSearch]);

  React.useEffect(() => {
    if (!open || !calcId) {
      setDetail(null);
      return;
    }
    let isMounted = true;
    setLoading(true);
    boeSummaryService
      .getById(calcId)
      .then((data) => {
        if (isMounted) {
          setDetail(data);
        }
      })
      .catch((err) => {
        if (isMounted) {
          toast.error(err instanceof Error ? err.message : String(err));
          onOpenChange(false);
        }
      })
      .finally(() => {
        if (isMounted) setLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [calcId, open, onOpenChange]);

  const handleExportExcel = async () => {
    if (!detail) return;
    try {
      await exportBOEExcel(detail.params, detail.result);
      toast.success('Exported BOE calculation to Excel');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleExportPdf = async () => {
    if (!detail) return;
    try {
      await exportBOEPdf(detail.params, detail.result);
      toast.success('Generated BOE calculation PDF');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex flex-wrap items-center justify-between gap-2 pr-6">
            <div>
              <DialogTitle className="text-xl">
                Reconciled BOE Dossier {detail ? `· ${detail.invoiceNumber}` : ''}
              </DialogTitle>
              <DialogDescription>
                {detail
                  ? `Shipment ${detail.shipmentId} · Calculation ID: ${detail.id} · Created ${detail.createdAt}`
                  : 'Loading BOE details...'}
              </DialogDescription>
            </div>
            {detail && (
              <div className="flex items-center gap-2">
                <Badge
                  variant={
                    detail.status === 'Reconciled'
                      ? 'success'
                      : detail.status === 'Discrepancy Found'
                      ? 'destructive'
                      : 'warning'
                  }
                  className="px-2.5 py-1 text-xs"
                >
                  {detail.status}
                </Badge>
              </div>
            )}
          </div>
        </DialogHeader>

        {loading ? (
          <div className="py-12 text-center text-sm text-muted-foreground">
            Loading reconciled BOE dossier and customs line items...
          </div>
        ) : !detail ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            No calculation data found.
          </div>
        ) : (
          <div className="space-y-6">
            {/* KPI Reconciliation Metric Cards */}
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <Card>
                <CardContent className="p-3">
                  <div className="text-xs font-medium text-muted-foreground">Total Assessable Value</div>
                  <div className="mt-1 font-mono text-base font-semibold">
                    ₹{detail.result.totalAssessableValue.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-3">
                  <div className="text-xs font-medium text-muted-foreground">Customs Duty (Calc)</div>
                  <div className="mt-1 font-mono text-base font-semibold text-primary">
                    ₹{detail.result.customsDutyTotal.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-3">
                  <div className="text-xs font-medium text-muted-foreground">Challan Duty Paid</div>
                  <div className="mt-1 font-mono text-base font-semibold">
                    ₹{detail.dutyPaid.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-3">
                  <div className="text-xs font-medium text-muted-foreground">Reconciliation Variance</div>
                  <div className="mt-1 flex items-center gap-1 font-mono text-base font-semibold">
                    {detail.reconciliation.isReconciled ? (
                      <>
                        <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                        <span className="text-emerald-600 dark:text-emerald-400">
                          {Math.abs(detail.reconciliation.dutyDifference) === 1
                            ? `±₹${Math.abs(detail.reconciliation.dutyDifference)} (Matched)`
                            : '₹0 (Matched)'}
                        </span>
                      </>
                    ) : (
                      <>
                        <AlertTriangle className="h-4 w-4 text-amber-500" />
                        <span className="text-amber-600 dark:text-amber-400">
                          ₹{Math.abs(detail.reconciliation.dutyDifference).toLocaleString()}
                        </span>
                      </>
                    )}
                  </div>
                </CardContent>
              </Card>
            </div>

            {/* Reconciliation Audit Banner */}
            <div
              className={`rounded-lg border p-3 text-sm flex items-start gap-2.5 ${
                detail.reconciliation.isReconciled
                  ? 'border-emerald-200 bg-emerald-50/50 text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200'
                  : 'border-amber-200 bg-amber-50/50 text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200'
              }`}
            >
              {detail.reconciliation.isReconciled ? (
                <ShieldCheck className="h-5 w-5 shrink-0 text-emerald-600 dark:text-emerald-400" />
              ) : (
                <AlertTriangle className="h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400" />
              )}
              <div className="space-y-0.5">
                <div className="font-semibold">
                  {detail.reconciliation.isReconciled
                    ? '100% Financial Reconciliation Verified'
                    : 'Reconciliation Variance Detected'}
                </div>
                <div className="text-xs opacity-90">
                  {detail.reconciliation.isReconciled
                    ? `ICEGATE Challan duty_paid (₹${detail.dutyPaid.toLocaleString()}) matches customs assessed duty liability (₹${detail.result.customsDutyTotal.toLocaleString()})${Math.abs(detail.reconciliation.dutyDifference) === 1 ? ' within ±₹1 statutory round-off tolerance' : ''}.`
                    : `Discrepancy of ₹${detail.reconciliation.dutyDifference.toLocaleString()} between challan duty paid (₹${detail.dutyPaid.toLocaleString()}) and assessed calculation (₹${detail.result.customsDutyTotal.toLocaleString()}).`}
                </div>
              </div>
            </div>

            {/* Shipment and ICEGATE Details */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 rounded-lg border p-4 bg-muted/20 text-xs">
              <div className="space-y-2">
                <div className="font-semibold text-sm text-foreground">Shipment & Valuation Parameters</div>
                <div className="grid grid-cols-2 gap-1.5 text-muted-foreground">
                  <span>Supplier:</span>
                  <span className="font-medium text-foreground">{detail.supplierName}</span>

                  <span>Invoice No:</span>
                  <span className="font-medium text-foreground">{detail.invoiceNumber}</span>

                  <span>Currency & Incoterm:</span>
                  <span className="font-medium text-foreground">
                    {detail.params.invoiceCurrency || 'USD'} · {detail.params.incoterm || 'CIF'}
                  </span>

                  <span>Exchange Rate:</span>
                  <span className="font-mono text-foreground font-medium">₹{detail.params.exchangeRate}</span>

                  <span>Freight (INR):</span>
                  <span className="font-mono text-foreground font-medium">
                    ₹{detail.params.freightCost?.toLocaleString() ?? '0'}
                  </span>

                  <span>Insurance Mode:</span>
                  <span className="font-medium text-foreground">
                    {detail.params.insuranceMode === 'included'
                      ? 'Included in Incoterm'
                      : detail.params.insuranceMode === 'actual'
                      ? `Actual (₹${detail.params.actualInsurance?.toLocaleString() ?? 0})`
                      : `Deemed Rate (${detail.params.insuranceRate}%)`}
                  </span>

                  <span>RoDTEP Balance:</span>
                  <span className="font-mono text-foreground font-medium">
                    ₹{detail.params.rodtepScripBalance?.toLocaleString() ?? '0'}
                  </span>
                </div>
              </div>

              <div className="space-y-2">
                <div className="font-semibold text-sm text-foreground">Linked ICEGATE BOE Challan</div>
                <div className="grid grid-cols-2 gap-1.5 text-muted-foreground">
                  <span>BE Number:</span>
                  <span className="font-mono font-medium text-foreground">
                    {detail.beNumber || 'Not Linked'}
                  </span>

                  <span>BE Date:</span>
                  <span className="font-medium text-foreground">{detail.beDate || '—'}</span>

                  <span>Customs Port / Location:</span>
                  <span className="font-medium text-foreground">{detail.location || '—'}</span>

                  <span>Challan Number:</span>
                  <span className="font-mono font-medium text-foreground">
                    {detail.challanNumber || '—'}
                  </span>

                  <span>Challan Payment Date:</span>
                  <span className="font-medium text-foreground">{detail.paymentDate || '—'}</span>

                  <span>ICEGATE Ref ID:</span>
                  <span className="font-mono text-[11px] text-foreground truncate">
                    {detail.refId || '—'}
                  </span>

                  <span>Transaction ID:</span>
                  <span className="font-mono text-[11px] text-foreground truncate">
                    {detail.transactionId || '—'}
                  </span>
                </div>
              </div>
            </div>

            {/* Line Items Table */}
            <div className="space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <h4 className="font-semibold text-sm">
                    Calculated Line Items ({detail.result.items?.length ?? 0})
                  </h4>
                  {itemSearch.trim() && (
                    <Badge variant="secondary" className="font-normal text-xs">
                      {filteredItems.length} matching
                    </Badge>
                  )}
                </div>
                <div className="relative w-full sm:w-64">
                  <Search className="absolute top-2.5 left-2.5 h-3.5 w-3.5 text-muted-foreground" />
                  <Input
                    className="pl-8 pr-7 h-8 text-xs"
                    placeholder="Search part no, description..."
                    value={itemSearch}
                    onChange={(e) => setItemSearch(e.target.value)}
                    aria-label="Search line items by part no or description"
                  />
                  {itemSearch && (
                    <button
                      type="button"
                      onClick={() => setItemSearch('')}
                      className="absolute top-2 right-2 text-muted-foreground hover:text-foreground focus:outline-none"
                      title="Clear search"
                      aria-label="Clear search"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  )}
                </div>
              </div>
              <div className="rounded-md border overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Part No</TableHead>
                      <TableHead>Description</TableHead>
                      <TableHead>HSN</TableHead>
                      <TableHead className="text-right">Qty</TableHead>
                      <TableHead className="text-right">Unit Price</TableHead>
                      <TableHead className="text-right">Assessable (INR)</TableHead>
                      <TableHead className="text-right">BCD %</TableHead>
                      <TableHead className="text-right">BCD (INR)</TableHead>
                      <TableHead className="text-right">SWS (INR)</TableHead>
                      <TableHead className="text-right">IGST (INR)</TableHead>
                      <TableHead className="text-right">Total Duty</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredItems.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={11} className="h-20 text-center text-muted-foreground">
                          {itemSearch.trim() ? (
                            <span>No line items matching &ldquo;{itemSearch.trim()}&rdquo;</span>
                          ) : (
                            <span>No item records.</span>
                          )}
                        </TableCell>
                      </TableRow>
                    ) : (
                      filteredItems.map((item, idx) => (
                        <TableRow key={item.itemId || idx} className="text-xs">
                          <TableCell className="font-medium font-mono">{item.partNumber}</TableCell>
                          <TableCell className="max-w-48 truncate" title={item.description}>
                            {item.description}
                          </TableCell>
                          <TableCell className="font-mono text-muted-foreground">{item.hsnCode}</TableCell>
                          <TableCell className="text-right font-mono">{item.quantity} {item.unit}</TableCell>
                          <TableCell className="text-right font-mono">
                            {item.unitPrice.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                          </TableCell>
                          <TableCell className="text-right font-mono font-medium">
                            {item.assessableValue.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                          </TableCell>
                          <TableCell className="text-right font-mono">{item.boeBcdRate}%</TableCell>
                          <TableCell className="text-right font-mono">
                            {item.bcdValue.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                          </TableCell>
                          <TableCell className="text-right font-mono">
                            {item.swsValue.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                          </TableCell>
                          <TableCell className="text-right font-mono">
                            {item.igstValue.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                          </TableCell>
                          <TableCell className="text-right font-mono font-semibold text-primary">
                            {item.totalLineDuty.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </div>

            {/* Duty Liability Summary Totals */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs rounded-lg border p-3 bg-muted/10">
              <div>
                <span className="text-muted-foreground">Gross BCD Total:</span>
                <div className="font-mono font-medium text-foreground">
                  ₹{detail.result.grossBcdTotal?.toLocaleString() ?? detail.result.bcdTotal?.toLocaleString() ?? 0}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">RoDTEP Scrip Debited:</span>
                <div className="font-mono font-medium text-foreground">
                  ₹{detail.result.rodtepScripUtilized?.toLocaleString() ?? 0}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">BCD Cash Payable:</span>
                <div className="font-mono font-medium text-foreground">
                  ₹{detail.result.bcdCashTotal?.toLocaleString() ?? detail.result.bcdTotal?.toLocaleString() ?? 0}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">SWS Total (10%):</span>
                <div className="font-mono font-medium text-foreground">
                  ₹{detail.result.swsTotal?.toLocaleString() ?? 0}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">IGST Total:</span>
                <div className="font-mono font-medium text-foreground">
                  ₹{detail.result.igstTotal?.toLocaleString() ?? 0}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Interest:</span>
                <div className="font-mono font-medium text-foreground">
                  ₹{detail.result.interest?.toLocaleString() ?? 0}
                </div>
              </div>
              {Boolean(detail.result.roundOffAdjustment) && (
                <div>
                  <span className="text-muted-foreground">Round-off:</span>
                  <div className="font-mono font-medium text-foreground">
                    {(detail.result.roundOffAdjustment || 0) > 0
                      ? `+₹${detail.result.roundOffAdjustment}`
                      : `-₹${Math.abs(detail.result.roundOffAdjustment || 0)}`}
                  </div>
                </div>
              )}
              <div>
                <span className="text-muted-foreground">Assessed Liability:</span>
                <div className="font-mono font-semibold text-primary">
                  ₹{detail.result.customsDutyTotal?.toLocaleString() ?? 0}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">Net Cash Challan:</span>
                <div className="font-mono font-semibold text-foreground">
                  ₹{detail.result.cashCustomsDutyTotal?.toLocaleString() ?? detail.result.customsDutyTotal?.toLocaleString() ?? 0}
                </div>
              </div>
            </div>
          </div>
        )}

        <DialogFooter className="flex flex-wrap items-center justify-between gap-2 pt-4">
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={!detail}
              onClick={handleExportExcel}
            >
              <FileSpreadsheet className="h-4 w-4" />
              Export Excel
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={!detail}
              onClick={handleExportPdf}
            >
              <FileDown className="h-4 w-4" />
              Export PDF
            </Button>
          </div>

          <div className="flex items-center gap-2">
            {canEdit && onEdit && detail && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  onOpenChange(false);
                  onEdit(detail.id);
                }}
              >
                <Pencil className="h-4 w-4" />
                Edit BOE
              </Button>
            )}
            <Button variant="default" size="sm" onClick={() => onOpenChange(false)}>
              Close
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
