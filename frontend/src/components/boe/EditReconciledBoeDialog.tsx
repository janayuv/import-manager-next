import * as React from 'react';
import { Calculator, ExternalLink, Save, ShieldAlert, ShieldCheck } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
  boeEntryService,
  boeService,
  boeSummaryService,
  type BOECalcParams,
  type BOECalculationDetail,
  type BOECalculationResult,
  type BOEDetail,
} from '@/services/boe-service';

const RECON_STATUSES = [
  'Awaiting BOE Data',
  'Discrepancy Found',
  'Reconciled',
  'Investigation',
  'Closed',
];

interface EditReconciledBoeDialogProps {
  calcId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved: () => void;
}

export function EditReconciledBoeDialog({
  calcId,
  open,
  onOpenChange,
  onSaved,
}: EditReconciledBoeDialogProps) {
  const navigate = useNavigate();
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [detail, setDetail] = React.useState<BOECalculationDetail | null>(null);
  const [availableBoes, setAvailableBoes] = React.useState<BOEDetail[]>([]);

  // Editable form state
  const [selectedBoeId, setSelectedBoeId] = React.useState('');
  const [status, setStatus] = React.useState('Reconciled');
  const [exchangeRate, setExchangeRate] = React.useState(83.5);
  const [freightCost, setFreightCost] = React.useState(0);
  const [insuranceMode, setInsuranceMode] = React.useState('deemed_1.125');
  const [insuranceRate, setInsuranceRate] = React.useState(1.125);
  const [actualInsurance, setActualInsurance] = React.useState(0);
  const [rodtepBalance, setRodtepBalance] = React.useState(0);
  const [interest, setInterest] = React.useState(0);

  // Live preview result
  const [previewResult, setPreviewResult] = React.useState<BOECalculationResult | null>(null);

  React.useEffect(() => {
    if (!open || !calcId) {
      setDetail(null);
      setPreviewResult(null);
      return;
    }
    let isMounted = true;
    setLoading(true);

    Promise.all([boeSummaryService.getById(calcId), boeService.list('')])
      .then(([data, allBoes]) => {
        if (!isMounted) return;
        setDetail(data);
        setAvailableBoes(allBoes ?? []);

        // Initialize form fields
        setSelectedBoeId(data.boeId || 'none');
        setStatus(data.status || 'Reconciled');
        setExchangeRate(data.params.exchangeRate || 83.5);
        setFreightCost(data.params.freightCost || 0);
        setInsuranceMode(data.params.insuranceMode || 'deemed_1.125');
        setInsuranceRate(data.params.insuranceRate || 1.125);
        setActualInsurance(data.params.actualInsurance || 0);
        setRodtepBalance(data.params.rodtepScripBalance || 0);
        setInterest(data.params.interest || 0);
        setPreviewResult(data.result);
      })
      .catch((err) => {
        if (!isMounted) return;
        toast.error(err instanceof Error ? err.message : String(err));
        onOpenChange(false);
      })
      .finally(() => {
        if (isMounted) setLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [calcId, open, onOpenChange]);

  // Recalculate preview on parameter changes
  React.useEffect(() => {
    if (!detail) return;
    const boeId = selectedBoeId === 'none' ? '' : selectedBoeId;
    const nextParams: BOECalcParams = {
      ...detail.params,
      id: detail.id,
      shipmentId: detail.shipmentId,
      boeId: boeId,
      status: status,
      exchangeRate: exchangeRate,
      freightCost: freightCost,
      insuranceMode: insuranceMode,
      insuranceRate: insuranceRate,
      actualInsurance: actualInsurance,
      rodtepScripBalance: rodtepBalance,
      interest: interest,
    };

    boeEntryService
      .previewCalculation(nextParams)
      .then((res) => {
        setPreviewResult(res);
      })
      .catch(() => {
        // keep prior preview
      });
  }, [
    detail,
    selectedBoeId,
    status,
    exchangeRate,
    freightCost,
    insuranceMode,
    insuranceRate,
    actualInsurance,
    rodtepBalance,
    interest,
  ]);

  const linkedBoe = React.useMemo(() => {
    if (!selectedBoeId || selectedBoeId === 'none') return null;
    return availableBoes.find((b) => b.id === selectedBoeId) ?? null;
  }, [selectedBoeId, availableBoes]);

  const dutyPaid = linkedBoe?.dutyPaid ?? (detail?.boeId === selectedBoeId ? detail.dutyPaid : 0);
  const computedDuty = previewResult?.customsDutyTotal ?? detail?.result.customsDutyTotal ?? 0;
  const cashDuty = previewResult?.cashCustomsDutyTotal ?? detail?.result.cashCustomsDutyTotal ?? 0;
  const isDutyMatched =
    Math.round(dutyPaid) === Math.round(computedDuty) || Math.round(dutyPaid) === Math.round(cashDuty);

  const handleSave = async () => {
    if (!detail) return;
    setSaving(true);
    const boeId = selectedBoeId === 'none' ? '' : selectedBoeId;

    const payload: BOECalcParams = {
      ...detail.params,
      id: detail.id,
      shipmentId: detail.shipmentId,
      boeId: boeId,
      status: status,
      exchangeRate: exchangeRate,
      freightCost: freightCost,
      insuranceMode: insuranceMode,
      insuranceRate: insuranceRate,
      actualInsurance: actualInsurance,
      rodtepScripBalance: rodtepBalance,
      interest: interest,
      lineItems: detail.params.lineItems,
    };

    try {
      await boeEntryService.saveCalculation(payload);
      if (status !== payload.status) {
        await boeSummaryService.updateStatus(detail.id, status);
      }
      toast.success(`Updated BOE calculation for ${detail.invoiceNumber}`);
      onOpenChange(false);
      onSaved();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleOpenInCalculator = () => {
    if (!detail) return;
    onOpenChange(false);
    navigate(`/boe-entry?shipmentId=${detail.shipmentId}&calcId=${detail.id}`);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center justify-between pr-6">
            <div>
              <DialogTitle className="text-xl">
                Edit Reconciled BOE {detail ? `· ${detail.invoiceNumber}` : ''}
              </DialogTitle>
              <DialogDescription>
                {detail
                  ? `Shipment ${detail.shipmentId} · ${detail.supplierName}`
                  : 'Loading calculation...'}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        {loading ? (
          <div className="py-12 text-center text-sm text-muted-foreground">
            Loading BOE calculation parameters...
          </div>
        ) : !detail ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            No calculation data found.
          </div>
        ) : (
          <div className="space-y-6">
            {/* Real-time comparison banner */}
            <div
              className={`rounded-lg border p-3 text-xs flex items-center justify-between gap-3 ${
                isDutyMatched && selectedBoeId !== 'none'
                  ? 'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200'
                  : 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200'
              }`}
            >
              <div className="flex items-center gap-2">
                {isDutyMatched && selectedBoeId !== 'none' ? (
                  <ShieldCheck className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                ) : (
                  <ShieldAlert className="h-4 w-4 text-amber-600 dark:text-amber-400" />
                )}
                <div>
                  <span className="font-semibold">
                    Computed Duty: ₹{computedDuty.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                  </span>
                  {' · '}
                  <span>
                    Challan Duty Paid:{' '}
                    {selectedBoeId !== 'none'
                      ? `₹${dutyPaid.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`
                      : 'None (Unlinked)'}
                  </span>
                </div>
              </div>
              <Badge
                variant={isDutyMatched && selectedBoeId !== 'none' ? 'success' : 'warning'}
                className="text-[11px]"
              >
                {isDutyMatched && selectedBoeId !== 'none' ? 'Balanced (Matched)' : 'Variance'}
              </Badge>
            </div>

            {/* Form Fields Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {/* Linked BOE Challan */}
              <div className="space-y-1.5 sm:col-span-2">
                <Label htmlFor="linkedBoe">Linked ICEGATE BOE Challan</Label>
                <Select value={selectedBoeId} onValueChange={setSelectedBoeId}>
                  <SelectTrigger id="linkedBoe" className="w-full">
                    <SelectValue placeholder="Select linked ICEGATE BOE" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">-- Unlinked (No BOE record) --</SelectItem>
                    {detail.boeId && (
                      <SelectItem value={detail.boeId}>
                        Current: {detail.beNumber} ({detail.beDate}) · Port: {detail.location} · Duty Paid: ₹{detail.dutyPaid.toLocaleString()}
                      </SelectItem>
                    )}
                    {availableBoes
                      .filter((b) => b.id !== detail.boeId)
                      .map((b) => (
                        <SelectItem key={b.id} value={b.id}>
                          {b.beNumber} ({b.beDate}) · Port: {b.location} · Duty Paid: ₹{b.dutyPaid.toLocaleString()}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-muted-foreground">
                  The ICEGATE Bill of Entry document with which duty amounts are reconciled.
                </p>
              </div>

              {/* Status */}
              <div className="space-y-1.5">
                <Label htmlFor="reconStatus">Reconciliation Status</Label>
                <Select value={status} onValueChange={setStatus}>
                  <SelectTrigger id="reconStatus">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {RECON_STATUSES.map((st) => (
                      <SelectItem key={st} value={st}>
                        {st}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Exchange Rate */}
              <div className="space-y-1.5">
                <Label htmlFor="exchangeRate">Customs Exchange Rate (INR)</Label>
                <Input
                  id="exchangeRate"
                  type="number"
                  step="0.01"
                  min="0"
                  value={exchangeRate}
                  onChange={(e) => setExchangeRate(parseFloat(e.target.value) || 0)}
                />
              </div>

              {/* Freight Cost */}
              <div className="space-y-1.5">
                <Label htmlFor="freightCost">Freight Cost (INR)</Label>
                <Input
                  id="freightCost"
                  type="number"
                  step="1"
                  min="0"
                  value={freightCost}
                  onChange={(e) => setFreightCost(parseFloat(e.target.value) || 0)}
                />
              </div>

              {/* Insurance Mode */}
              <div className="space-y-1.5">
                <Label htmlFor="insuranceMode">Insurance Valuation Basis</Label>
                <Select value={insuranceMode} onValueChange={setInsuranceMode}>
                  <SelectTrigger id="insuranceMode">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="deemed_1.125">1.125% Statutory Deemed Rule 10(2)</SelectItem>
                    <SelectItem value="rate">Open Cover Rate Policy (%)</SelectItem>
                    <SelectItem value="actual">Actual Marine Policy Premium (INR)</SelectItem>
                    <SelectItem value="included">Included in Incoterm (CIF/CIP)</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {/* Insurance Value */}
              {insuranceMode === 'actual' ? (
                <div className="space-y-1.5">
                  <Label htmlFor="actualInsurance">Actual Insurance Premium (INR)</Label>
                  <Input
                    id="actualInsurance"
                    type="number"
                    step="1"
                    min="0"
                    value={actualInsurance}
                    onChange={(e) => setActualInsurance(parseFloat(e.target.value) || 0)}
                  />
                </div>
              ) : insuranceMode !== 'included' ? (
                <div className="space-y-1.5">
                  <Label htmlFor="insuranceRate">Insurance Rate (%)</Label>
                  <Input
                    id="insuranceRate"
                    type="number"
                    step="0.001"
                    min="0"
                    value={insuranceRate}
                    onChange={(e) => setInsuranceRate(parseFloat(e.target.value) || 0)}
                  />
                </div>
              ) : null}

              {/* RoDTEP Balance */}
              <div className="space-y-1.5">
                <Label htmlFor="rodtep">RoDTEP e-Scrip Balance (INR)</Label>
                <Input
                  id="rodtep"
                  type="number"
                  step="1"
                  min="0"
                  value={rodtepBalance}
                  onChange={(e) => setRodtepBalance(parseFloat(e.target.value) || 0)}
                />
              </div>

              {/* Interest */}
              <div className="space-y-1.5">
                <Label htmlFor="interest">Customs Interest / Penalties (INR)</Label>
                <Input
                  id="interest"
                  type="number"
                  step="1"
                  min="0"
                  value={interest}
                  onChange={(e) => setInterest(parseFloat(e.target.value) || 0)}
                />
              </div>
            </div>

            {/* Quick Link to full calculation */}
            <div className="flex items-center justify-between rounded-lg border border-dashed p-3 bg-muted/20">
              <div className="text-xs text-muted-foreground">
                Need to modify individual part HSN codes, BCD/SWS overrides, or line item quantities?
              </div>
              <Button variant="outline" size="sm" onClick={handleOpenInCalculator}>
                <Calculator className="h-4 w-4" />
                Open Full Calculator
                <ExternalLink className="h-3 w-3 ml-1" />
              </Button>
            </div>
          </div>
        )}

        <DialogFooter className="flex items-center justify-between pt-4">
          <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            disabled={loading || saving || !detail}
            onClick={handleSave}
          >
            <Save className="h-4 w-4" />
            {saving ? 'Saving...' : 'Save Reconciled BOE'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
