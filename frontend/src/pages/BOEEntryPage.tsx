import * as React from 'react';
import {
  AlertTriangle,
  Calculator,
  ChevronDown,
  ChevronRight,
  FileDown,
  FileSpreadsheet,
  Info,
  RefreshCw,
  Save,
  Search,
  ShieldCheck,
  Sparkles,
  Upload,
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
import { Textarea } from '@/components/ui/textarea';
import { exportBOEExcel, exportBOEPdf } from '@/lib/boe-export';
import { useSearchParams } from 'react-router-dom';
import { QuickOptionDialog } from '@/components/master/QuickOptionDialog';
import { LinkedBoeSelector } from '@/components/boe/LinkedBoeSelector';
import { IcegateIngestionDialog, type ParsedIcegateBoe } from '@/components/boe/IcegateIngestionDialog';
import { optionsService, type LookupOption } from '@/services/master-service';
import { cbicRatesService } from '@/services/cbic-rates-service';
import {
  boeEntryService,
  boeService,
  boeSummaryService,
  type BOECalcParams,
  type BOECalculationResult,
  type BOEDetail,
} from '@/services/boe-service';
import { shipmentService, type Shipment } from '@/services/shipment-invoice-service';

const INCOTERMS = [
  'EXW',
  'FCA',
  'FAS',
  'FOB',
  'CFR',
  'CIF',
  'CPT',
  'CIP',
  'DAP',
  'DPU',
  'DDP',
] as const;

const TRADE_AGREEMENTS = [
  'India-Japan CEPA (Comprehensive Economic Partnership Agreement)',
  'India-Korea CEPA (Comprehensive Economic Partnership Agreement)',
  'India-ASEAN AIFTA (Free Trade Agreement)',
  'India-UAE CEPA (Comprehensive Economic Partnership Agreement)',
  'India-Australia ECTA (Economic Cooperation and Trade Agreement)',
  'India-Singapore CECA (Comprehensive Economic Cooperation Agreement)',
  'SAFTA (South Asian Free Trade Area)',
  'Other Preferential Trade Agreement',
] as const;

export function BOEEntryPage() {
  const [searchParams] = useSearchParams();
  const urlShipmentId = searchParams.get('shipmentId') || '';
  const urlCalcId = searchParams.get('calcId') || '';

  const [shipments, setShipments] = React.useState<Shipment[]>([]);
  const [boes, setBoes] = React.useState<BOEDetail[]>([]);
  const [allBoes, setAllBoes] = React.useState<BOEDetail[]>([]);
  const [selectedShipmentId, setSelectedShipmentId] = React.useState(urlShipmentId);
  const [params, setParams] = React.useState<BOECalcParams | null>(null);
  const [result, setResult] = React.useState<BOECalculationResult | null>(null);

  const [overrideOpen, setOverrideOpen] = React.useState(false);
  const [overrideCsv, setOverrideCsv] = React.useState('');
  const [saving, setSaving] = React.useState(false);
  const [showOriginBreakdown, setShowOriginBreakdown] = React.useState(false);
  const [icegateOpen, setIcegateOpen] = React.useState(false);
  const [itemSearch, setItemSearch] = React.useState('');
  const [insuranceRateOptions, setInsuranceRateOptions] = React.useState<LookupOption[]>([
    { value: '0.015', label: '0.015% (Open Cover Policy)' },
    { value: '1.125', label: '1.125% (Statutory Deemed Rule 10(2))' },
  ]);
  const [optionEditor, setOptionEditor] = React.useState<{ table: string; title: string } | null>(null);

  const loadInsuranceRates = React.useCallback(async () => {
    try {
      const opts = await optionsService.list('insurance_rates');
      if (opts && opts.length > 0) {
        setInsuranceRateOptions(opts);
      }
    } catch {
      // Keep canonical fallback
    }
  }, []);

  React.useEffect(() => {
    void loadInsuranceRates();
  }, [loadInsuranceRates]);

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
    void loadData(urlShipmentId);
  }, [loadData, urlShipmentId]);

  React.useEffect(() => {
    if (!selectedShipmentId && !urlCalcId) {
      setParams(null);
      setResult(null);
      return;
    }
    void (async () => {
      try {
        let ctx;
        if (urlCalcId) {
          ctx = await boeEntryService.loadCalculationContext(urlCalcId);
          if (ctx.params?.shipmentId && ctx.params.shipmentId !== selectedShipmentId) {
            setSelectedShipmentId(ctx.params.shipmentId);
          }
        } else {
          ctx = await boeEntryService.loadShipmentContext(selectedShipmentId);
        }
        setParams(ctx.params);
        setResult(ctx.result);
      } catch (err) {
        setParams(null);
        setResult(null);
        toast.error(err instanceof Error ? err.message : String(err));
      }
    })();
  }, [selectedShipmentId, urlCalcId]);

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
    const paid = Math.round(linkedBoe.dutyPaid);
    const gross = Math.round(result.customsDutyTotal);
    const cash = Math.round(result.cashCustomsDutyTotal ?? result.customsDutyTotal);
    return paid !== gross && paid !== cash;
  }, [linkedBoe, result]);

  const syncCbicRate = React.useCallback(
    async (targetDate?: string, silent = false) => {
      if (!params || !params.invoiceCurrency) return;
      const curr = params.invoiceCurrency.trim().toUpperCase();
      const dateToUse = targetDate || linkedBoe?.beDate || new Date().toISOString().slice(0, 10);
      try {
        const rate = await cbicRatesService.getEffectiveRate(curr, dateToUse);
        if (rate && rate.exchangeRate > 0) {
          const next = {
            ...params,
            exchangeRate: rate.exchangeRate,
            cbicNotificationNumber: rate.notificationNumber,
          };
          await recompute(next);
          if (!silent) {
            toast.success(
              `Auto-populated statutory CBIC rate ₹${rate.exchangeRate.toFixed(4)} (${rate.notificationNumber}) for ${curr} on ${dateToUse}`,
            );
          }
        }
      } catch (err) {
        if (!silent) {
          toast.error(
            `No statutory CBIC rate for ${curr} on ${dateToUse}: ${err instanceof Error ? err.message : String(err)}`,
          );
        }
      }
    },
    [params, linkedBoe, recompute],
  );

  React.useEffect(() => {
    setItemSearch('');
  }, [selectedShipmentId, urlCalcId]);

  const lineItemsWithIndex = React.useMemo(() => {
    return (result?.items ?? []).map((item, originalIndex) => ({
      item,
      originalIndex,
    }));
  }, [result?.items]);

  const filteredLineItems = React.useMemo(() => {
    const q = itemSearch.trim().toLowerCase();
    if (!q) return lineItemsWithIndex;
    return lineItemsWithIndex.filter(
      ({ item }) =>
        (item.partNumber && item.partNumber.toLowerCase().includes(q)) ||
        (item.description && item.description.toLowerCase().includes(q)) ||
        (item.hsnCode && item.hsnCode.toLowerCase().includes(q)),
    );
  }, [lineItemsWithIndex, itemSearch]);

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

  const incotermCode = (params?.incoterm || '').toUpperCase();
  const cifLocked = incotermCode === 'CIF';
  const freightIncluded = Boolean(cifLocked || result?.incotermRule?.freightIncluded);
  const insuranceIncluded = Boolean(cifLocked || result?.incotermRule?.insuranceIncluded);

  const handleIcegateApply = (parsed: ParsedIcegateBoe) => {
    if (params) {
      const updated: BOECalcParams = {
        ...params,
        exchangeRate: parsed.exchangeRate > 0 ? parsed.exchangeRate : params.exchangeRate,
      };
      void recompute(updated);
      toast.success(
        `Applied ICEGATE fields: Exchange Rate ₹${parsed.exchangeRate.toFixed(2)}, Extracted Total Duty: ₹${parsed.customsDutyTotal.toLocaleString()}`,
      );
    } else {
      toast.info(
        `Parsed ICEGATE BE #${parsed.beNumber} with ₹${parsed.customsDutyTotal.toLocaleString()} duty. Select an active shipment to apply calculation.`,
      );
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">BOE Duty Calculation Engine</h1>
          <p className="text-sm text-muted-foreground">
            Statutory 11-Incoterm Valuation (Rule 10), Standard / CEPA / RoDTEP E-Scrip customs duties in INR.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            className="border-primary/40 text-primary hover:bg-primary/10"
            onClick={() => setIcegateOpen(true)}
            title="Paste or upload ICEGATE text dump to extract duty heads"
          >
            <Sparkles className="h-4 w-4 mr-1 text-amber-500" />
            ICEGATE Parser
          </Button>
          {params && result && (
            <>
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
            </>
          )}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Calculator className="h-4 w-4 text-primary" />
            Shipment &amp; Valuation Parameters
          </CardTitle>
          <CardDescription>
            Customs Valuation (Determination of Value of Imported Goods) Rules, 2007 under Section 14.
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

            <LinkedBoeSelector
              value={params?.boeId || ''}
              onChange={async (newBoeId) => {
                if (!params) return;
                const next = { ...params, boeId: newBoeId };
                const chosenBoe = displayBoes.find((b) => b.id === newBoeId);
                if (chosenBoe?.beDate && params.invoiceCurrency) {
                  try {
                    const rate = await cbicRatesService.getEffectiveRate(
                      params.invoiceCurrency,
                      chosenBoe.beDate,
                    );
                    if (rate && rate.exchangeRate > 0) {
                      next.exchangeRate = rate.exchangeRate;
                      next.cbicNotificationNumber = rate.notificationNumber;
                      toast.success(
                        `Auto-populated statutory CBIC rate ₹${rate.exchangeRate.toFixed(4)} (${rate.notificationNumber}) for BOE Date ${chosenBoe.beDate}`,
                      );
                    }
                  } catch {
                    // Retain current rate if not found
                  }
                }
                void recompute(next);
              }}
              boes={displayBoes}
              expectedDuty={result?.customsDutyTotal}
              cashDuty={result?.cashCustomsDutyTotal}
              disabled={!params}
            />

            <div className="flex flex-wrap items-end gap-2 pb-1">
              {cifLocked && <Badge variant="warning">CIF Zero-Locked</Badge>}
              {!cifLocked && incotermCode && (
                <Badge variant="outline">Incoterm: {incotermCode}</Badge>
              )}
              {result?.payloadWarning && <Badge variant="destructive">Large Payload (&gt;768 KB)</Badge>}
            </div>
          </div>

          {params && (
            <>
              {/* Core 5-input valuation grid */}
              <div className="grid gap-4 md:grid-cols-5">
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="exRate">Exchange Rate (INR per {params.invoiceCurrency})</Label>
                    <div className="flex items-center gap-1">
                      {params.cbicNotificationNumber && (
                        <Badge
                          variant="outline"
                          className="text-[9px] px-1 py-0 text-primary border-primary/30 font-mono"
                          title={`Statutory CBIC Notification: ${params.cbicNotificationNumber}`}
                        >
                          CBIC Statutory
                        </Badge>
                      )}
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-5 px-1 text-[10px] text-muted-foreground hover:text-primary"
                        onClick={() => void syncCbicRate(linkedBoe?.beDate)}
                        title={`Sync CBIC statutory exchange rate for ${params.invoiceCurrency} on BOE date (${linkedBoe?.beDate || 'today'})`}
                      >
                        <RefreshCw className="h-3 w-3 mr-0.5" />
                        Sync CBIC
                      </Button>
                    </div>
                  </div>
                  <Input
                    id="exRate"
                    type="number"
                    step="0.0001"
                    min="0.0001"
                    className="font-mono text-sm"
                    value={params.exchangeRate}
                    onChange={(e) =>
                      void recompute({ ...params, exchangeRate: Number.parseFloat(e.target.value) || 0 })
                    }
                  />
                  <div className="flex items-center justify-between text-[10px] text-muted-foreground">
                    {params.cbicNotificationNumber ? (
                      <span className="truncate" title={params.cbicNotificationNumber}>
                        Notif: {params.cbicNotificationNumber}
                      </span>
                    ) : (
                      <span>Manual override / Default rate</span>
                    )}
                    {linkedBoe?.beDate && (
                      <span className="font-mono text-primary/80">BOE: {linkedBoe.beDate}</span>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="freightCost">Freight (INR)</Label>
                    {freightIncluded && <span className="text-[10px] text-muted-foreground">(Included)</span>}
                  </div>
                  <Input
                    id="freightCost"
                    type="number"
                    step="0.01"
                    disabled={freightIncluded}
                    value={params.freightCost}
                    onChange={(e) =>
                      void recompute({ ...params, freightCost: Number.parseFloat(e.target.value) || 0 })
                    }
                  />
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="exwCost">EXW (INR)</Label>
                    {cifLocked && <span className="text-[10px] text-muted-foreground">(Locked)</span>}
                  </div>
                  <Input
                    id="exwCost"
                    type="number"
                    step="0.01"
                    disabled={cifLocked}
                    value={params.exwCost}
                    onChange={(e) => {
                      const val = Number.parseFloat(e.target.value) || 0;
                      void recompute({ ...params, exwCost: val });
                    }}
                  />
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="insuranceRate">Insurance (%)</Label>
                    {insuranceIncluded && <span className="text-[10px] text-muted-foreground">(Included)</span>}
                  </div>
                  <Input
                    id="insuranceRate"
                    type="number"
                    step="0.001"
                    disabled={insuranceIncluded || params.insuranceMode === 'actual'}
                    value={params.insuranceRate}
                    onChange={(e) =>
                      void recompute({
                        ...params,
                        insuranceRate: Number.parseFloat(e.target.value) || 0,
                        insuranceMode: 'rate',
                      })
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

              {/* Incoterm & Statutory Insurance Rules Bar */}
              <div className="grid gap-4 rounded-lg border border-border/40 bg-muted/20 p-3 md:grid-cols-3">
                <div className="space-y-1">
                  <Label className="text-xs font-semibold">Incoterm (Incoterms 2020)</Label>
                  <Select
                    value={params.incoterm || 'CIF'}
                    onValueChange={(val) => void recompute({ ...params, incoterm: val })}
                  >
                    <SelectTrigger className="h-8 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {INCOTERMS.map((term) => (
                        <SelectItem key={term} value={term} className="text-xs">
                          {term}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs font-semibold">Insurance Basis (Rule 10(2))</Label>
                    <button
                      type="button"
                      onClick={() => setOptionEditor({ table: 'insurance_rates', title: 'Manage Insurance Rates' })}
                      className="text-[11px] font-medium text-primary hover:underline"
                    >
                      + Manage Rates
                    </button>
                  </div>
                  <Select
                    value={
                      insuranceIncluded
                        ? 'included'
                        : params.insuranceMode === 'actual'
                        ? 'actual'
                        : String(params.insuranceRate ?? '0.015')
                    }
                    disabled={insuranceIncluded}
                    onValueChange={(val) => {
                      if (val === 'included') {
                        void recompute({ ...params, insuranceMode: 'included', insuranceRate: 0 });
                      } else if (val === 'actual') {
                        void recompute({ ...params, insuranceMode: 'actual' });
                      } else {
                        const numRate = Number.parseFloat(val) || 0.015;
                        void recompute({ ...params, insuranceMode: 'rate', insuranceRate: numRate });
                      }
                    }}
                  >
                    <SelectTrigger className="h-8 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="included" disabled={!insuranceIncluded} className="text-xs">
                        Included in Invoice ({params.incoterm})
                      </SelectItem>
                      {insuranceRateOptions.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value} className="text-xs">
                          {opt.label}
                        </SelectItem>
                      ))}
                      {!insuranceRateOptions.some((opt) => opt.value === String(params.insuranceRate)) &&
                        params.insuranceRate > 0 &&
                        params.insuranceMode !== 'actual' && (
                          <SelectItem value={String(params.insuranceRate)} className="text-xs">
                            {params.insuranceRate}% (Custom Rate)
                          </SelectItem>
                        )}
                      <SelectItem value="actual" className="text-xs">
                        Actual Marine Premium (INR)
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                {params.insuranceMode === 'actual' && !insuranceIncluded ? (
                  <div className="space-y-1">
                    <Label htmlFor="actualInsurance" className="text-xs font-semibold">
                      Actual Policy Premium (INR)
                    </Label>
                    <Input
                      id="actualInsurance"
                      type="number"
                      step="0.01"
                      className="h-8 text-xs"
                      placeholder="e.g. 3500"
                      value={params.actualInsurance || 0}
                      onChange={(e) =>
                        void recompute({
                          ...params,
                          actualInsurance: Number.parseFloat(e.target.value) || 0,
                        })
                      }
                    />
                  </div>
                ) : (
                  <div className="flex items-center pt-5 text-xs text-muted-foreground">
                    <Info className="mr-1.5 h-3.5 w-3.5 shrink-0" />
                    {result?.incotermRule?.description || 'Statutory Customs Valuation Rules apply.'}
                  </div>
                )}
              </div>

              {/* Origin & Rule 10 Additions Section */}
              <div className="rounded-lg border border-border/40 p-3">
                <div
                  className="flex cursor-pointer items-center justify-between"
                  onClick={() => setShowOriginBreakdown((prev) => !prev)}
                >
                  <div className="flex items-center gap-2 text-xs font-semibold">
                    {showOriginBreakdown ? (
                      <ChevronDown className="h-4 w-4" />
                    ) : (
                      <ChevronRight className="h-4 w-4" />
                    )}
                    Origin &amp; Rule 10(1) Additions (Inland Freight, Handling, Packing, Assists)
                  </div>
                  <div className="text-xs text-muted-foreground">
                    Total Origin: INR {(result?.totalOriginCharges ?? params.exwCost).toLocaleString()}
                  </div>
                </div>

                {showOriginBreakdown && (
                  <div className="mt-3 grid gap-3 border-t border-border/30 pt-3 md:grid-cols-5">
                    <div className="space-y-1">
                      <Label htmlFor="originFreight" className="text-[11px]">
                        Origin Inland Freight (INR)
                      </Label>
                      <Input
                        id="originFreight"
                        type="number"
                        step="0.01"
                        className="h-8 text-xs"
                        disabled={cifLocked}
                        value={params.originInlandFreight || 0}
                        onChange={(e) => {
                          const val = Number.parseFloat(e.target.value) || 0;
                          const total =
                            val +
                            (params.exportHandling || 0) +
                            (params.loadingCharges || 0) +
                            (params.packingCharges || 0) +
                            (params.otherRule10Additions || 0);
                          void recompute({ ...params, originInlandFreight: val, exwCost: total });
                        }}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="exportHandling" className="text-[11px]">
                        Export Handling (INR)
                      </Label>
                      <Input
                        id="exportHandling"
                        type="number"
                        step="0.01"
                        className="h-8 text-xs"
                        disabled={cifLocked}
                        value={params.exportHandling || 0}
                        onChange={(e) => {
                          const val = Number.parseFloat(e.target.value) || 0;
                          const total =
                            (params.originInlandFreight || 0) +
                            val +
                            (params.loadingCharges || 0) +
                            (params.packingCharges || 0) +
                            (params.otherRule10Additions || 0);
                          void recompute({ ...params, exportHandling: val, exwCost: total });
                        }}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="loadingCharges" className="text-[11px]">
                        Loading Charges (INR)
                      </Label>
                      <Input
                        id="loadingCharges"
                        type="number"
                        step="0.01"
                        className="h-8 text-xs"
                        disabled={cifLocked}
                        value={params.loadingCharges || 0}
                        onChange={(e) => {
                          const val = Number.parseFloat(e.target.value) || 0;
                          const total =
                            (params.originInlandFreight || 0) +
                            (params.exportHandling || 0) +
                            val +
                            (params.packingCharges || 0) +
                            (params.otherRule10Additions || 0);
                          void recompute({ ...params, loadingCharges: val, exwCost: total });
                        }}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="packingCharges" className="text-[11px]">
                        Packing Charges (INR)
                      </Label>
                      <Input
                        id="packingCharges"
                        type="number"
                        step="0.01"
                        className="h-8 text-xs"
                        disabled={cifLocked}
                        value={params.packingCharges || 0}
                        onChange={(e) => {
                          const val = Number.parseFloat(e.target.value) || 0;
                          const total =
                            (params.originInlandFreight || 0) +
                            (params.exportHandling || 0) +
                            (params.loadingCharges || 0) +
                            val +
                            (params.otherRule10Additions || 0);
                          void recompute({ ...params, packingCharges: val, exwCost: total });
                        }}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="otherRule10" className="text-[11px]">
                        Other Rule 10 Additions (INR)
                      </Label>
                      <Input
                        id="otherRule10"
                        type="number"
                        step="0.01"
                        className="h-8 text-xs"
                        value={params.otherRule10Additions || 0}
                        onChange={(e) => {
                          const val = Number.parseFloat(e.target.value) || 0;
                          const total =
                            (params.originInlandFreight || 0) +
                            (params.exportHandling || 0) +
                            (params.loadingCharges || 0) +
                            (params.packingCharges || 0) +
                            val;
                          void recompute({ ...params, otherRule10Additions: val, exwCost: total });
                        }}
                      />
                    </div>
                  </div>
                )}
              </div>

              {/* RoDTEP Scrip Debit Section */}
              <div className="rounded-lg border border-border/40 bg-muted/10 p-3">
                <div className="grid gap-3 md:grid-cols-3">
                  <div className="space-y-1">
                    <Label htmlFor="rodtepScrip" className="text-xs font-semibold">
                      ICEGATE RoDTEP E-Scrip Balance (INR)
                    </Label>
                    <Input
                      id="rodtepScrip"
                      type="number"
                      step="1"
                      className="h-8 text-xs"
                      placeholder="0"
                      value={params.rodtepScripBalance || 0}
                      onChange={(e) =>
                        void recompute({
                          ...params,
                          rodtepScripBalance: Number.parseFloat(e.target.value) || 0,
                        })
                      }
                    />
                  </div>
                  <div className="col-span-2 flex items-center text-xs text-muted-foreground">
                    {result && (result.rodtepScripUtilized ?? 0) > 0 ? (
                      <div className="space-y-0.5">
                        <div className="font-semibold text-emerald-500">
                          RoDTEP E-Scrip Debited: INR {(result.rodtepScripUtilized ?? 0).toLocaleString()} against
                          BCD Liability (Cash BCD: INR {(result.bcdCashTotal ?? result.bcdTotal).toLocaleString()}).
                        </div>
                        <div className="text-[11px]">
                          SWS (INR {result.swsTotal.toLocaleString()}) and IGST (INR{' '}
                          {result.igstTotal.toLocaleString()}) are payable in cash via duty challan.
                        </div>
                      </div>
                    ) : (
                      <div className="text-[11px]">
                        Transferable RoDTEP electronic duty-credit scrips debit Basic Customs Duty (BCD) on
                        ICEGATE. SWS and IGST must be paid in cash.
                      </div>
                    )}
                  </div>
                </div>
              </div>

              {/* CEPA Preferential Eligibility Details (if any CEPA line exists) */}
              {params.lineItems?.some((it) => it.calculationMethod === 'CEPA') && (
                <div className="rounded-lg border border-blue-500/30 bg-blue-500/5 p-3">
                  <div className="flex items-center justify-between pb-2">
                    <div className="flex items-center gap-1.5 text-xs font-semibold text-blue-400">
                      <ShieldCheck className="h-4 w-4" />
                      CEPA Preferential Duty Eligibility &amp; Certificate of Origin
                    </div>
                    {params.certificateOfOriginNo ? (
                      <Badge variant="outline" className="border-emerald-500 text-emerald-400">
                        CEPA Documented &amp; Eligible
                      </Badge>
                    ) : (
                      <Badge variant="warning">COO Number Required</Badge>
                    )}
                  </div>
                  <div className="grid gap-3 md:grid-cols-3">
                    <div className="space-y-1">
                      <Label className="text-[11px]">Trade Agreement</Label>
                      <Select
                        value={params.tradeAgreement || TRADE_AGREEMENTS[0]}
                        onValueChange={(val) => void recompute({ ...params, tradeAgreement: val })}
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {TRADE_AGREEMENTS.map((ta) => (
                            <SelectItem key={ta} value={ta} className="text-xs">
                              {ta}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="cooNo" className="text-[11px]">
                        Certificate of Origin (COO) Number *
                      </Label>
                      <Input
                        id="cooNo"
                        className="h-8 text-xs"
                        placeholder="e.g. 260827-01-COO-JPN"
                        value={params.certificateOfOriginNo || ''}
                        onChange={(e) => void recompute({ ...params, certificateOfOriginNo: e.target.value })}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="cooDate" className="text-[11px]">
                        COO Issuance Date
                      </Label>
                      <Input
                        id="cooDate"
                        type="date"
                        className="h-8 text-xs"
                        value={params.certificateOfOriginDate || ''}
                        onChange={(e) =>
                          void recompute({ ...params, certificateOfOriginDate: e.target.value })
                        }
                      />
                    </div>
                  </div>
                </div>
              )}
            </>
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
              Save blocked: Linked BOE duty paid (INR {Math.round(linkedBoe.dutyPaid).toLocaleString()}) does
              not match computed Customs Duty Total (INR {Math.round(result.customsDutyTotal).toLocaleString()}).
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
                  {result.totalAssessableValue.toLocaleString(undefined, {
                    minimumFractionDigits: 1,
                    maximumFractionDigits: 1,
                  })}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>BCD Total (INR)</CardDescription>
                <CardTitle className="text-xl">
                  {result.bcdTotal.toLocaleString(undefined, {
                    minimumFractionDigits: 0,
                    maximumFractionDigits: 0,
                  })}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>SWS Total (INR)</CardDescription>
                <CardTitle className="text-xl">
                  {result.swsTotal.toLocaleString(undefined, {
                    minimumFractionDigits: 0,
                    maximumFractionDigits: 0,
                  })}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>IGST Total (INR)</CardDescription>
                <CardTitle className="text-xl">
                  {result.igstTotal.toLocaleString(undefined, {
                    minimumFractionDigits: 0,
                    maximumFractionDigits: 0,
                  })}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card className="border-primary/40">
              <CardHeader className="pb-2">
                <CardDescription>Customs Duty Grand Total</CardDescription>
                <CardTitle className="text-xl text-primary">
                  INR{' '}
                  {result.customsDutyTotal.toLocaleString(undefined, {
                    minimumFractionDigits: 0,
                    maximumFractionDigits: 0,
                  })}
                </CardTitle>
              </CardHeader>
            </Card>
          </div>

          {(result.rodtepScripUtilized ?? 0) > 0 && (
            <div className="grid gap-4 md:grid-cols-2">
              <Card className="border-emerald-500/40 bg-emerald-500/5">
                <CardHeader className="pb-2">
                  <CardDescription className="text-emerald-400">
                    RoDTEP Electronic Scrip Debited (from BCD)
                  </CardDescription>
                  <CardTitle className="text-xl text-emerald-400">
                    - INR{' '}
                    {(result.rodtepScripUtilized ?? 0).toLocaleString(undefined, {
                      minimumFractionDigits: 0,
                      maximumFractionDigits: 0,
                    })}
                  </CardTitle>
                </CardHeader>
              </Card>
              <Card className="border-primary/60 bg-primary/5">
                <CardHeader className="pb-2">
                  <CardDescription>Net Cash Challan Payable (Duty Paid)</CardDescription>
                  <CardTitle className="text-xl text-primary">
                    INR{' '}
                    {(result.cashCustomsDutyTotal ?? result.customsDutyTotal).toLocaleString(undefined, {
                      minimumFractionDigits: 0,
                      maximumFractionDigits: 0,
                    })}
                  </CardTitle>
                </CardHeader>
              </Card>
            </div>
          )}

          <Card>
            <CardHeader className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 pb-3">
              <div>
                <CardTitle className="text-base flex items-center gap-2">
                  <span>Line-by-Line Duty Breakdown ({result.items?.length ?? 0})</span>
                  {itemSearch.trim() && (
                    <Badge variant="secondary" className="font-normal text-xs">
                      {filteredLineItems.length} matching
                    </Badge>
                  )}
                </CardTitle>
                <CardDescription>
                  Customs tariff assessable valuation, BCD/SWS overrides, and item duty calculations.
                </CardDescription>
              </div>
              <div className="flex items-center gap-2">
                <div className="relative w-full sm:w-72">
                  <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
                  <Input
                    className="pl-8 pr-8 h-9 text-xs"
                    placeholder="Search part no or description..."
                    value={itemSearch}
                    onChange={(e) => setItemSearch(e.target.value)}
                    aria-label="Search line items by part no or description"
                  />
                  {itemSearch && (
                    <button
                      type="button"
                      onClick={() => setItemSearch('')}
                      className="absolute top-2.5 right-2.5 text-muted-foreground hover:text-foreground focus:outline-none"
                      title="Clear search"
                      aria-label="Clear search"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>
              </div>
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
                  {filteredLineItems.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={9} className="h-28 text-center text-muted-foreground">
                        {itemSearch.trim() ? (
                          <div className="flex flex-col items-center justify-center gap-1">
                            <p>No line items matching &ldquo;{itemSearch.trim()}&rdquo;</p>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-xs h-7 text-primary"
                              onClick={() => setItemSearch('')}
                            >
                              Clear filter
                            </Button>
                          </div>
                        ) : (
                          'No line items found.'
                        )}
                      </TableCell>
                    </TableRow>
                  ) : (
                    filteredLineItems.map(({ item: ln, originalIndex }) => (
                      <TableRow key={ln.itemId || `${ln.partNumber}-${originalIndex}`}>
                        <TableCell>
                          <div className="font-mono text-xs font-semibold">{ln.partNumber}</div>
                          <div className="text-xs text-muted-foreground">{ln.description}</div>
                        </TableCell>
                        <TableCell className="w-36">
                          <Select
                            value={ln.calculationMethod}
                            onValueChange={(method) => {
                              const updated = (params.lineItems ?? []).map((it, i) =>
                                i === originalIndex ? { ...it, calculationMethod: method } : it,
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
                            value={params.lineItems?.[originalIndex]?.boeBcdRate ?? ln.boeBcdRate}
                            onChange={(e) => {
                              const rate = Number.parseFloat(e.target.value) || 0;
                              const updated = (params.lineItems ?? []).map((it, i) =>
                                i === originalIndex ? { ...it, boeBcdRate: rate } : it,
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
                            value={params.lineItems?.[originalIndex]?.boeSwsRate ?? ln.boeSwsRate}
                            onChange={(e) => {
                              const rate = Number.parseFloat(e.target.value) || 0;
                              const updated = (params.lineItems ?? []).map((it, i) =>
                                i === originalIndex ? { ...it, boeSwsRate: rate } : it,
                              );
                              void recompute({ ...params, lineItems: updated });
                            }}
                          />
                        </TableCell>
                        <TableCell className="font-mono text-xs">{ln.boeIgstRate}%</TableCell>
                        <TableCell className="font-mono text-xs">
                          {ln.bcdValue.toFixed(1)} / {ln.swsValue.toFixed(1)} / {ln.igstValue.toFixed(1)}
                          {(ln.rodtepUtilized ?? 0) > 0 && (
                            <div className="text-[10px] text-emerald-400">
                              Scrip: -{ln.rodtepUtilized.toFixed(1)}
                            </div>
                          )}
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs font-semibold">
                          {ln.totalLineDuty.toFixed(1)}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
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
              Required headers: partNo, calculationMethod, boeBcdRate, boeSwsRate, boeIgstRate. Rows with IGST
              mismatch are skipped cleanly.
            </DialogDescription>
          </DialogHeader>
          <Textarea
            rows={6}
            className="font-mono text-xs"
            placeholder={
              'partNo,calculationMethod,boeBcdRate,boeSwsRate,boeIgstRate\nPN-100,CEPA,2.5,10,18'
            }
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

      {optionEditor && (
        <QuickOptionDialog
          open={Boolean(optionEditor)}
          onOpenChange={(o) => !o && setOptionEditor(null)}
          table={optionEditor.table}
          title={optionEditor.title}
          options={insuranceRateOptions}
          onUpdated={loadInsuranceRates}
        />
      )}

      <IcegateIngestionDialog
        open={icegateOpen}
        onOpenChange={setIcegateOpen}
        onApply={handleIcegateApply}
      />
    </div>
  );
}
