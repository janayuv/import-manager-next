import * as React from 'react';
import { AlertTriangle, Calculator, FileDown, FileSpreadsheet, Save, Upload } from 'lucide-react';
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
import { Textarea } from '@/components/ui/textarea';
import { exportBOEExcel, exportBOEPdf } from '@/lib/boe-export';
import {
  boeEntryService,
  boeService,
  boeSummaryService,
  type BOECalcParams,
  type BOECalculationResult,
  type BOEDetail,
} from '@/services/boe-service';
import { shipmentService, type Shipment } from '@/services/shipment-invoice-service';

export function BOEEntryPage() {
  const [shipments, setShipments] = React.useState<Shipment[]>([]);
  const [boes, setBoes] = React.useState<BOEDetail[]>([]);
  const [allBoes, setAllBoes] = React.useState<BOEDetail[]>([]);
  const [selectedShipmentId, setSelectedShipmentId] = React.useState('');
  const [params, setParams] = React.useState<BOECalcParams | null>(null);
  const [result, setResult] = React.useState<BOECalculationResult | null>(null);

  const [overrideOpen, setOverrideOpen] = React.useState(false);
  const [overrideCsv, setOverrideCsv] = React.useState('');
  const [saving, setSaving] = React.useState(false);

  const loadData = React.useCallback(async (keepSelectedShipmentId?: string) => {
    try {
      const [shPage, boeList, summaries] = await Promise.all([
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
        boeService.list(''),
        boeSummaryService.list('', '').catch(() => []),
      ]);

      const sumList = summaries ?? [];
      const finalizedShipmentIds = new Set(
        sumList
          .filter((s) => s.status === 'Reconciled' || s.status === 'Closed' || s.status === 'Finalized')
          .map((s) => s.shipmentId),
      );
      const linkedBoeIds = new Set(
        sumList
          .filter((s) => s.boeId && (s.status === 'Reconciled' || s.status === 'Closed' || s.status === 'Finalized'))
          .map((s) => s.boeId),
      );

      const terminalStatuses = new Set(['delivered', 'cancelled', 'closed', 'completed', 'finalized']);

      const eligibleShipments = (shPage.items ?? []).filter((sh) => {
        if (keepSelectedShipmentId && sh.id === keepSelectedShipmentId) return true;
        if (finalizedShipmentIds.has(sh.id)) return false;
        if (terminalStatuses.has((sh.status || '').toLowerCase())) return false;
        return true;
      });

      const eligibleBoes = (boeList ?? []).filter((b) => {
        return !linkedBoeIds.has(b.id);
      });

      setShipments(eligibleShipments);
      setBoes(eligibleBoes);
      setAllBoes(boeList ?? []);

      setSelectedShipmentId((prev) => {
        if (keepSelectedShipmentId && eligibleShipments.some((s) => s.id === keepSelectedShipmentId)) {
          return keepSelectedShipmentId;
        }
        if (prev && eligibleShipments.some((s) => s.id === prev)) {
          return prev;
        }
        return eligibleShipments[0]?.id ?? '';
      });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  }, []);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  React.useEffect(() => {
    if (!selectedShipmentId) {
      setParams(null);
      setResult(null);
      return;
    }
    void (async () => {
      try {
        const ctx = await boeEntryService.loadShipmentContext(selectedShipmentId);
        setParams(ctx.params);
        setResult(ctx.result);
      } catch (err) {
        setParams(null);
        setResult(null);
        toast.error(err instanceof Error ? err.message : String(err));
      }
    })();
  }, [selectedShipmentId]);

  const recompute = React.useCallback(async (nextParams: BOECalcParams) => {
    setParams(nextParams);
    try {
      const nextRes = await boeEntryService.previewCalculation(nextParams);
      setResult(nextRes);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  }, []);

  const displayBoes = React.useMemo(() => {
    if (params?.boeId && !boes.some((b) => b.id === params.boeId)) {
      const current = allBoes.find((b) => b.id === params.boeId);
      if (current) return [current, ...boes];
    }
    return boes;
  }, [boes, params?.boeId, allBoes]);

  const linkedBoe = React.useMemo(
    () => displayBoes.find((b) => b.id === params?.boeId) ?? null,
    [displayBoes, params?.boeId],
  );

  const dutyPaidMismatch = React.useMemo(() => {
    if (!linkedBoe || !result) return false;
    return Math.round(linkedBoe.dutyPaid) !== Math.round(result.customsDutyTotal);
  }, [linkedBoe, result]);

  const handleApplyOverride = async () => {
    if (!params || !overrideCsv.trim()) return;
    try {
      const res = await boeEntryService.parseOverrideCsv(overrideCsv, params.lineItems ?? []);
      const next = { ...params, lineItems: res.items ?? [] };
      await recompute(next);
      setOverrideOpen(false);
      setOverrideCsv('');
      if (res.skippedCount > 0) {
        toast.warning(
          `Applied ${res.appliedCount} overrides; skipped ${res.skippedCount} (${res.warnings?.[0]?.message ?? ''})`,
        );
      } else {
        toast.success(`Applied ${res.appliedCount} rate overrides`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleSave = async () => {
    if (!params) return;
    setSaving(true);
    try {
      const saved = await boeEntryService.saveCalculation(params);
      toast.success(`Saved BOE calculation (${saved.status})`);
      await loadData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const cifLocked = params?.incoterm?.toUpperCase() === 'CIF';

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">BOE Duty Calculation Engine</h1>
          <p className="text-sm text-muted-foreground">
            Interactive Standard / CEPA / RoDTEP customs duty calculator (all duty figures in INR).
          </p>
        </div>
        {params && result && (
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={() => setOverrideOpen(true)}>
              <Upload className="h-4 w-4" />
              Import Override CSV
            </Button>
            <Button variant="outline" onClick={() => void exportBOEExcel(params, result)}>
              <FileSpreadsheet className="h-4 w-4" />
              Excel (.xlsx)
            </Button>
            <Button variant="outline" onClick={() => exportBOEPdf(params, result)}>
              <FileDown className="h-4 w-4" />
              ICEGATE PDF
            </Button>
            <Button
              disabled={saving || result.hasBlockingBcdOverride || dutyPaidMismatch}
              onClick={() => void handleSave()}
            >
              <Save className="h-4 w-4" />
              {saving ? 'Saving...' : 'Save Calculation'}
            </Button>
          </div>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Calculator className="h-4 w-4 text-primary" />
            Shipment &amp; Valuation Parameters
          </CardTitle>
          <CardDescription>
            Freight &amp; EXW inputs are entered in INR. Incoterm CIF automatically locks Freight/EXW/Insurance to 0.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="space-y-1">
              <Label>Shipment *</Label>
              <Select value={selectedShipmentId} onValueChange={setSelectedShipmentId}>
                <SelectTrigger>
                  <SelectValue placeholder={shipments.length === 0 ? 'No pending shipments' : 'Choose shipment'} />
                </SelectTrigger>
                <SelectContent>
                  {shipments.length === 0 ? (
                    <div className="p-2 text-xs text-muted-foreground">No pending shipments to calculate</div>
                  ) : (
                    shipments.map((sh) => (
                      <SelectItem key={sh.id} value={sh.id}>
                        {sh.invoiceNumber} — {sh.supplierName} ({sh.incoterm})
                      </SelectItem>
                    ))
                  )}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label>Linked ICEGATE BOE (for Duty Paid Check)</Label>
              <Select
                value={params?.boeId || 'none'}
                onValueChange={(v) =>
                  params && void recompute({ ...params, boeId: v === 'none' ? '' : v })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">Unlinked (Awaiting BOE Data)</SelectItem>
                  {displayBoes.map((b) => (
                    <SelectItem key={b.id} value={b.id}>
                      {b.beNumber} ({b.beDate}) — Paid: INR {Math.round(b.dutyPaid).toLocaleString()}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-end gap-2 pb-1">
              {cifLocked && <Badge variant="warning">CIF Zero-Locked</Badge>}
              {result?.payloadWarning && <Badge variant="destructive">Large Payload (&gt;768 KB)</Badge>}
            </div>
          </div>

          {params && (
            <div className="grid gap-4 md:grid-cols-5">
              <div className="space-y-1">
                <Label htmlFor="exRate">Exchange Rate (INR per {params.invoiceCurrency})</Label>
                <Input
                  id="exRate"
                  type="number"
                  step="0.01"
                  value={params.exchangeRate}
                  onChange={(e) =>
                    void recompute({ ...params, exchangeRate: Number.parseFloat(e.target.value) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="freightCost">Freight (INR)</Label>
                <Input
                  id="freightCost"
                  type="number"
                  step="0.01"
                  disabled={cifLocked}
                  value={params.freightCost}
                  onChange={(e) =>
                    void recompute({ ...params, freightCost: Number.parseFloat(e.target.value) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="exwCost">EXW (INR)</Label>
                <Input
                  id="exwCost"
                  type="number"
                  step="0.01"
                  disabled={cifLocked}
                  value={params.exwCost}
                  onChange={(e) =>
                    void recompute({ ...params, exwCost: Number.parseFloat(e.target.value) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="insuranceRate">Insurance (%)</Label>
                <Input
                  id="insuranceRate"
                  type="number"
                  step="0.001"
                  disabled={cifLocked}
                  value={params.insuranceRate}
                  onChange={(e) =>
                    void recompute({ ...params, insuranceRate: Number.parseFloat(e.target.value) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="interest">Interest (INR)</Label>
                <Input
                  id="interest"
                  type="number"
                  step="0.01"
                  value={params.interest}
                  onChange={(e) =>
                    void recompute({ ...params, interest: Number.parseFloat(e.target.value) || 0 })
                  }
                />
              </div>
            </div>
          )}

          {result?.hasBlockingBcdOverride && (
            <div className="flex items-center gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-xs text-destructive">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              Save blocked: One or more line items have BOE BCD rate higher than Item Master Actual BCD rate.
            </div>
          )}

          {dutyPaidMismatch && linkedBoe && result && (
            <div className="flex items-center gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-xs text-destructive">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              Save blocked: Linked BOE duty paid (INR {Math.round(linkedBoe.dutyPaid).toLocaleString()}) does not match computed Customs Duty Total (INR {Math.round(result.customsDutyTotal).toLocaleString()}).
            </div>
          )}
        </CardContent>
      </Card>

      {result && params && (
        <>
          <div className="grid gap-4 md:grid-cols-5">
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>Assessable Value (INR)</CardDescription>
                <CardTitle className="text-xl">
                  {result.totalAssessableValue.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>BCD Total (INR)</CardDescription>
                <CardTitle className="text-xl">
                  {result.bcdTotal.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>SWS Total (INR)</CardDescription>
                <CardTitle className="text-xl">
                  {result.swsTotal.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>IGST Total (INR)</CardDescription>
                <CardTitle className="text-xl">
                  {result.igstTotal.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card className="border-primary/40">
              <CardHeader className="pb-2">
                <CardDescription>Customs Duty Grand Total</CardDescription>
                <CardTitle className="text-xl text-primary">
                  INR {result.customsDutyTotal.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                </CardTitle>
              </CardHeader>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Line-by-Line Duty Breakdown ({result.items?.length ?? 0})</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Part No</TableHead>
                    <TableHead>Method</TableHead>
                    <TableHead>Forex</TableHead>
                    <TableHead>Assessable (INR)</TableHead>
                    <TableHead>BOE BCD %</TableHead>
                    <TableHead>BOE SWS %</TableHead>
                    <TableHead>IGST % (Readonly)</TableHead>
                    <TableHead>BCD / SWS / IGST (INR)</TableHead>
                    <TableHead className="text-right">Line Duty (INR)</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(result.items ?? []).map((ln, idx) => (
                    <TableRow key={ln.itemId + idx}>
                      <TableCell>
                        <div className="font-mono text-xs font-semibold">{ln.partNumber}</div>
                        <div className="text-xs text-muted-foreground">{ln.description}</div>
                      </TableCell>
                      <TableCell className="w-36">
                        <Select
                          value={ln.calculationMethod}
                          onValueChange={(method) => {
                            const updated = (params.lineItems ?? []).map((it, i) =>
                              i === idx ? { ...it, calculationMethod: method } : it,
                            );
                            void recompute({ ...params, lineItems: updated });
                          }}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="Standard">Standard</SelectItem>
                            <SelectItem value="CEPA">CEPA</SelectItem>
                            <SelectItem value="Rodtep">Rodtep</SelectItem>
                          </SelectContent>
                        </Select>
                      </TableCell>
                      <TableCell className="font-mono text-xs">{ln.itemForexValue.toFixed(2)}</TableCell>
                      <TableCell className="font-mono text-xs">{ln.assessableValue.toFixed(1)}</TableCell>
                      <TableCell className="w-28">
                        <Input
                          type="number"
                          step="0.01"
                          value={params.lineItems?.[idx]?.boeBcdRate ?? ln.boeBcdRate}
                          onChange={(e) => {
                            const rate = Number.parseFloat(e.target.value) || 0;
                            const updated = (params.lineItems ?? []).map((it, i) =>
                              i === idx ? { ...it, boeBcdRate: rate } : it,
                            );
                            void recompute({ ...params, lineItems: updated });
                          }}
                        />
                        {ln.bcdDiscrepancy && (
                          <Badge variant="destructive" className="mt-1">
                            Actual {ln.actualBcdRate}%
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="w-24">
                        <Input
                          type="number"
                          step="0.01"
                          value={params.lineItems?.[idx]?.boeSwsRate ?? ln.boeSwsRate}
                          onChange={(e) => {
                            const rate = Number.parseFloat(e.target.value) || 0;
                            const updated = (params.lineItems ?? []).map((it, i) =>
                              i === idx ? { ...it, boeSwsRate: rate } : it,
                            );
                            void recompute({ ...params, lineItems: updated });
                          }}
                        />
                      </TableCell>
                      <TableCell className="font-mono text-xs">{ln.boeIgstRate}%</TableCell>
                      <TableCell className="font-mono text-xs">
                        {ln.bcdValue.toFixed(1)} / {ln.swsValue.toFixed(1)} / {ln.igstValue.toFixed(1)}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs font-semibold">
                        {ln.totalLineDuty.toFixed(1)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </>
      )}

      <Dialog open={overrideOpen} onOpenChange={setOverrideOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>Import BOE Rate Override CSV</DialogTitle>
            <DialogDescription>
              Required headers: partNo, calculationMethod, boeBcdRate, boeSwsRate, boeIgstRate. Rows with IGST mismatch are skipped cleanly.
            </DialogDescription>
          </DialogHeader>
          <Textarea
            rows={6}
            className="font-mono text-xs"
            placeholder={'partNo,calculationMethod,boeBcdRate,boeSwsRate,boeIgstRate\nPN-100,CEPA,2.5,10,18'}
            value={overrideCsv}
            onChange={(e) => setOverrideCsv(e.target.value)}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setOverrideOpen(false)}>
              Cancel
            </Button>
            <Button onClick={() => void handleApplyOverride()}>Apply Overrides</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
