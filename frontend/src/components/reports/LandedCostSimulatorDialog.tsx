import * as React from 'react';
import {
  ArrowDownRight,
  ArrowUpRight,
  Calculator,
  Download,
  RefreshCw,
  Sliders,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { downloadTextFile } from '@/lib/download';
import { formatINR } from '@/pages/PartPeriodAnalysisPage';

interface LandedCostSimulatorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialPartNo?: string;
  initialUnitPrice?: number;
  initialCurrency?: string;
}

export function LandedCostSimulatorDialog({
  open,
  onOpenChange,
  initialPartNo = 'PART-SIM-01',
  initialUnitPrice = 12.5,
  initialCurrency = 'USD',
}: LandedCostSimulatorDialogProps) {
  // Baseline Parameters
  const [partNo, setPartNo] = React.useState(initialPartNo);
  const [currency, setCurrency] = React.useState(initialCurrency);
  const [quantity, setQuantity] = React.useState(1000);
  const [unitPrice, setUnitPrice] = React.useState(initialUnitPrice);
  const [baselineFx, setBaselineFx] = React.useState(85.0);
  const [freightInr, setFreightInr] = React.useState(25000);
  const insuranceRatePct = 0.015;
  const [bcdRatePct, setBcdRatePct] = React.useState(7.5);
  const swsRatePct = 10.0;
  const igstRatePct = 18.0;
  const [basicExpensesInr, setBasicExpensesInr] = React.useState(15000);

  // Simulation Deltas
  const [fxDelta, setFxDelta] = React.useState(2.0); // +2.0 INR per foreign currency
  const [freightDeltaPct, setFreightDeltaPct] = React.useState(10); // +10%
  const [bcdDeltaPct, setBcdDeltaPct] = React.useState(0); // 0% tariff change
  const [expensesDeltaPct, setExpensesDeltaPct] = React.useState(5); // +5% handling

  // Reset deltas
  const handleReset = () => {
    setFxDelta(0);
    setFreightDeltaPct(0);
    setBcdDeltaPct(0);
    setExpensesDeltaPct(0);
  };

  // Calculations
  const calculateCosts = React.useCallback(
    (fx: number, freight: number, bcdPct: number, expenses: number) => {
      const invoiceValInr = quantity * unitPrice * fx;
      const insuranceInr = (invoiceValInr * insuranceRatePct) / 100;
      const assessableValue = invoiceValInr + freight + insuranceInr;

      const bcd = (assessableValue * bcdPct) / 100;
      const sws = (bcd * swsRatePct) / 100;
      const dutyBaseForIgst = assessableValue + bcd + sws;
      const igst = (dutyBaseForIgst * igstRatePct) / 100;
      const customsDutyTotal = bcd + sws + igst;

      // Note: In standard Indian customs accounting, IGST is creditable (ITC), so Landed Cost = AV + BCD + SWS + Allocated Basic Expenses
      const landedCostExIgst = assessableValue + bcd + sws + expenses;
      const landedCostPerQty = quantity > 0 ? landedCostExIgst / quantity : 0;

      return {
        invoiceValInr,
        freight,
        assessableValue,
        bcd,
        sws,
        igst,
        customsDutyTotal,
        expenses,
        landedCostExIgst,
        landedCostPerQty,
      };
    },
    [quantity, unitPrice, insuranceRatePct, swsRatePct, igstRatePct],
  );

  const baseline = React.useMemo(() => {
    return calculateCosts(baselineFx, freightInr, bcdRatePct, basicExpensesInr);
  }, [calculateCosts, baselineFx, freightInr, bcdRatePct, basicExpensesInr]);

  const simulatedFx = baselineFx + fxDelta;
  const simulatedFreight = freightInr * (1 + freightDeltaPct / 100);
  const simulatedBcdPct = Math.max(0, bcdRatePct + bcdDeltaPct);
  const simulatedExpenses = basicExpensesInr * (1 + expensesDeltaPct / 100);

  const simulated = React.useMemo(() => {
    return calculateCosts(
      simulatedFx,
      simulatedFreight,
      simulatedBcdPct,
      simulatedExpenses,
    );
  }, [
    calculateCosts,
    simulatedFx,
    simulatedFreight,
    simulatedBcdPct,
    simulatedExpenses,
  ]);

  const unitVariance = simulated.landedCostPerQty - baseline.landedCostPerQty;
  const unitVariancePct =
    baseline.landedCostPerQty > 0
      ? (unitVariance / baseline.landedCostPerQty) * 100
      : 0;
  const totalVariance = simulated.landedCostExIgst - baseline.landedCostExIgst;

  const handleExportCsv = () => {
    const lines = [
      'Landed Cost "What-If" Sensitivity Simulation Summary',
      `Generated At,${new Date().toISOString()}`,
      `Part Number,${partNo}`,
      `Quantity,${quantity}`,
      `Currency,${currency}`,
      `Unit Price,${unitPrice}`,
      '',
      'Metric,Baseline,Simulated,Variance (INR),Variance (%)',
      `Exchange Rate,${baselineFx.toFixed(2)},${simulatedFx.toFixed(2)},${fxDelta.toFixed(2)},${((fxDelta / baselineFx) * 100).toFixed(2)}%`,
      `Freight Cost,${baseline.freight.toFixed(2)},${simulated.freight.toFixed(2)},${(simulated.freight - baseline.freight).toFixed(2)},${freightDeltaPct}%`,
      `BCD Rate %,${bcdRatePct.toFixed(2)}%,${simulatedBcdPct.toFixed(2)}%,${bcdDeltaPct.toFixed(2)}%,-`,
      `Assessable Value,${baseline.assessableValue.toFixed(2)},${simulated.assessableValue.toFixed(2)},${(simulated.assessableValue - baseline.assessableValue).toFixed(2)},${(((simulated.assessableValue - baseline.assessableValue) / baseline.assessableValue) * 100).toFixed(2)}%`,
      `Basic Customs Duty (BCD),${baseline.bcd.toFixed(2)},${simulated.bcd.toFixed(2)},${(simulated.bcd - baseline.bcd).toFixed(2)},-`,
      `Total Landed Cost,${baseline.landedCostExIgst.toFixed(2)},${simulated.landedCostExIgst.toFixed(2)},${totalVariance.toFixed(2)},${unitVariancePct.toFixed(2)}%`,
      `Landed Cost Per Unit,${baseline.landedCostPerQty.toFixed(2)},${simulated.landedCostPerQty.toFixed(2)},${unitVariance.toFixed(2)},${unitVariancePct.toFixed(2)}%`,
    ];
    downloadTextFile(`landed_cost_simulation_${partNo}.csv`, lines.join('\n'));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center justify-between gap-3 pr-6">
            <div className="flex items-center gap-2">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <Calculator className="h-5 w-5" />
              </div>
              <div>
                <DialogTitle>Landed Cost "What-If" Sensitivity Simulator</DialogTitle>
                <DialogDescription>
                  Simulate procurement landed costs under fluctuating exchange rates, freight tariffs, and customs rates
                </DialogDescription>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={handleReset}
                title="Reset simulation parameters to baseline"
              >
                <RefreshCw className="h-3.5 w-3.5 mr-1" /> Reset
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={handleExportCsv}
                title="Export sensitivity summary to CSV"
              >
                <Download className="h-3.5 w-3.5 mr-1" /> Export CSV
              </Button>
            </div>
          </div>
        </DialogHeader>

        {/* Executive Comparison KPI Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-4 gap-3 py-2">
          <div className="rounded-lg border border-border bg-card p-3 shadow-2xs">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
              Baseline Unit Landed Cost
            </span>
            <div className="mt-1 font-mono text-xl font-bold">
              {formatINR(baseline.landedCostPerQty)}
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">At ₹{baselineFx.toFixed(2)} / {currency}</p>
          </div>

          <div className="rounded-lg border border-primary/40 bg-primary/5 p-3 shadow-2xs">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-primary">
              Simulated Unit Cost
            </span>
            <div className="mt-1 font-mono text-xl font-bold text-primary">
              {formatINR(simulated.landedCostPerQty)}
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">At ₹{simulatedFx.toFixed(2)} / {currency}</p>
          </div>

          <div
            className={`rounded-lg border p-3 shadow-2xs ${
              unitVariance > 0
                ? 'border-amber-300/80 bg-amber-50/50 dark:border-amber-800/60 dark:bg-amber-950/20'
                : 'border-emerald-300/80 bg-emerald-50/50 dark:border-emerald-800/60 dark:bg-emerald-950/20'
            }`}
          >
            <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
              Unit Variance Delta
            </span>
            <div className="mt-1 flex items-center gap-1 font-mono text-xl font-bold">
              {unitVariance > 0 ? (
                <ArrowUpRight className="h-5 w-5 text-amber-600 dark:text-amber-400" />
              ) : (
                <ArrowDownRight className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />
              )}
              <span>{formatINR(Math.abs(unitVariance))}</span>
            </div>
            <p
              className={`text-xs font-medium mt-0.5 ${
                unitVariance > 0 ? 'text-amber-700 dark:text-amber-400' : 'text-emerald-700 dark:text-emerald-400'
              }`}
            >
              {unitVariance > 0 ? '+' : ''}
              {unitVariancePct.toFixed(2)}% vs Baseline
            </p>
          </div>

          <div className="rounded-lg border border-border bg-card p-3 shadow-2xs">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
              Total Batch Impact ({quantity} qty)
            </span>
            <div className="mt-1 font-mono text-xl font-bold">
              {formatINR(totalVariance)}
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              Net lot expenditure impact
            </p>
          </div>
        </div>

        {/* Sensitivity Adjustment Sliders / Controls */}
        <div className="rounded-lg border border-border bg-muted/30 p-4 space-y-4">
          <div className="flex items-center gap-2">
            <Sliders className="h-4 w-4 text-primary" />
            <h4 className="text-xs font-semibold uppercase tracking-wider text-foreground">
              Sensitivity Adjusters (Simulate Market Fluctuations)
            </h4>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
            {/* FX Delta */}
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs font-medium">
                <Label htmlFor="fxDelta">Exchange Rate Delta</Label>
                <span className="font-mono text-primary font-bold">
                  {fxDelta >= 0 ? `+₹${fxDelta.toFixed(2)}` : `-₹${Math.abs(fxDelta).toFixed(2)}`}
                </span>
              </div>
              <input
                id="fxDelta"
                type="range"
                min="-10"
                max="15"
                step="0.25"
                value={fxDelta}
                onChange={(e) => setFxDelta(parseFloat(e.target.value))}
                className="w-full accent-primary cursor-pointer"
              />
              <div className="flex justify-between text-[10px] text-muted-foreground">
                <span>-₹10</span>
                <span>Sim: ₹{simulatedFx.toFixed(2)}</span>
                <span>+₹15</span>
              </div>
            </div>

            {/* Freight Delta % */}
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs font-medium">
                <Label htmlFor="freightDelta">Ocean / Air Freight</Label>
                <span className="font-mono text-primary font-bold">
                  {freightDeltaPct >= 0 ? `+${freightDeltaPct}%` : `${freightDeltaPct}%`}
                </span>
              </div>
              <input
                id="freightDelta"
                type="range"
                min="-50"
                max="100"
                step="5"
                value={freightDeltaPct}
                onChange={(e) => setFreightDeltaPct(parseFloat(e.target.value))}
                className="w-full accent-primary cursor-pointer"
              />
              <div className="flex justify-between text-[10px] text-muted-foreground">
                <span>-50%</span>
                <span>Sim: {formatINR(simulatedFreight)}</span>
                <span>+100%</span>
              </div>
            </div>

            {/* BCD Tariff Delta */}
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs font-medium">
                <Label htmlFor="bcdDelta">BCD Tariff Rate Delta</Label>
                <span className="font-mono text-primary font-bold">
                  {bcdDeltaPct >= 0 ? `+${bcdDeltaPct}%` : `${bcdDeltaPct}%`}
                </span>
              </div>
              <input
                id="bcdDelta"
                type="range"
                min="-7.5"
                max="15"
                step="0.5"
                value={bcdDeltaPct}
                onChange={(e) => setBcdDeltaPct(parseFloat(e.target.value))}
                className="w-full accent-primary cursor-pointer"
              />
              <div className="flex justify-between text-[10px] text-muted-foreground">
                <span>-7.5%</span>
                <span>Sim: {simulatedBcdPct.toFixed(1)}%</span>
                <span>+15%</span>
              </div>
            </div>

            {/* Expenses Delta % */}
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs font-medium">
                <Label htmlFor="expDelta">Handling &amp; CHA Fees</Label>
                <span className="font-mono text-primary font-bold">
                  {expensesDeltaPct >= 0 ? `+${expensesDeltaPct}%` : `${expensesDeltaPct}%`}
                </span>
              </div>
              <input
                id="expDelta"
                type="range"
                min="-30"
                max="50"
                step="5"
                value={expensesDeltaPct}
                onChange={(e) => setExpensesDeltaPct(parseFloat(e.target.value))}
                className="w-full accent-primary cursor-pointer"
              />
              <div className="flex justify-between text-[10px] text-muted-foreground">
                <span>-30%</span>
                <span>Sim: {formatINR(simulatedExpenses)}</span>
                <span>+50%</span>
              </div>
            </div>
          </div>
        </div>

        {/* Baseline Configuration Accordion/Inputs */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Baseline Order Parameters
            </h4>
            <span className="text-xs text-muted-foreground">Modify baseline values to match your quotation</span>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
            <div className="space-y-1">
              <Label htmlFor="partNo" className="text-xs">Part Number / SKU</Label>
              <Input
                id="partNo"
                value={partNo}
                onChange={(e) => setPartNo(e.target.value)}
                className="h-8 text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="quantity" className="text-xs">Batch Quantity</Label>
              <Input
                id="quantity"
                type="number"
                value={quantity}
                onChange={(e) => setQuantity(Math.max(1, parseInt(e.target.value) || 1))}
                className="h-8 text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="unitPrice" className="text-xs">Unit Price ({currency})</Label>
              <Input
                id="unitPrice"
                type="number"
                step="0.01"
                value={unitPrice}
                onChange={(e) => setUnitPrice(Math.max(0, parseFloat(e.target.value) || 0))}
                className="h-8 text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="baselineFx" className="text-xs">Base Exchange Rate (INR)</Label>
              <Input
                id="baselineFx"
                type="number"
                step="0.1"
                value={baselineFx}
                onChange={(e) => setBaselineFx(Math.max(1, parseFloat(e.target.value) || 1))}
                className="h-8 text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="currency" className="text-xs">Currency</Label>
              <Input
                id="currency"
                value={currency}
                onChange={(e) => setCurrency(e.target.value.toUpperCase())}
                className="h-8 text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="baseFreight" className="text-xs">Base Freight (INR)</Label>
              <Input
                id="baseFreight"
                type="number"
                value={freightInr}
                onChange={(e) => setFreightInr(Math.max(0, parseFloat(e.target.value) || 0))}
                className="h-8 text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="baseBcd" className="text-xs">Base BCD (%)</Label>
              <Input
                id="baseBcd"
                type="number"
                step="0.5"
                value={bcdRatePct}
                onChange={(e) => setBcdRatePct(Math.max(0, parseFloat(e.target.value) || 0))}
                className="h-8 text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="baseExp" className="text-xs">Base Expenses (INR)</Label>
              <Input
                id="baseExp"
                type="number"
                value={basicExpensesInr}
                onChange={(e) => setBasicExpensesInr(Math.max(0, parseFloat(e.target.value) || 0))}
                className="h-8 text-xs font-mono"
              />
            </div>
          </div>
        </div>

        {/* Detailed Breakdown Comparison Table */}
        <div className="rounded-lg border border-border overflow-hidden">
          <table className="w-full text-xs">
            <thead className="bg-muted/60 text-muted-foreground font-semibold border-b border-border">
              <tr>
                <th className="py-2 px-3 text-left">Cost Component</th>
                <th className="py-2 px-3 text-right">Baseline (₹)</th>
                <th className="py-2 px-3 text-right font-semibold text-foreground">Simulated (₹)</th>
                <th className="py-2 px-3 text-right">Variance Delta</th>
                <th className="py-2 px-3 text-right">Impact %</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border/40 font-mono">
              <tr>
                <td className="py-2 px-3 font-sans font-medium">Invoice Value (INR)</td>
                <td className="py-2 px-3 text-right">{formatINR(baseline.invoiceValInr)}</td>
                <td className="py-2 px-3 text-right font-bold text-foreground">{formatINR(simulated.invoiceValInr)}</td>
                <td className="py-2 px-3 text-right text-muted-foreground">
                  {formatINR(simulated.invoiceValInr - baseline.invoiceValInr)}
                </td>
                <td className="py-2 px-3 text-right">
                  {(((simulated.invoiceValInr - baseline.invoiceValInr) / baseline.invoiceValInr) * 100).toFixed(1)}%
                </td>
              </tr>
              <tr>
                <td className="py-2 px-3 font-sans font-medium">Ocean / Air Freight</td>
                <td className="py-2 px-3 text-right">{formatINR(baseline.freight)}</td>
                <td className="py-2 px-3 text-right font-bold text-foreground">{formatINR(simulated.freight)}</td>
                <td className="py-2 px-3 text-right text-muted-foreground">
                  {formatINR(simulated.freight - baseline.freight)}
                </td>
                <td className="py-2 px-3 text-right">{freightDeltaPct}%</td>
              </tr>
              <tr className="bg-muted/20 font-semibold">
                <td className="py-2 px-3 font-sans">Assessable Value (AV)</td>
                <td className="py-2 px-3 text-right">{formatINR(baseline.assessableValue)}</td>
                <td className="py-2 px-3 text-right text-foreground">{formatINR(simulated.assessableValue)}</td>
                <td className="py-2 px-3 text-right">
                  {formatINR(simulated.assessableValue - baseline.assessableValue)}
                </td>
                <td className="py-2 px-3 text-right">
                  {(((simulated.assessableValue - baseline.assessableValue) / baseline.assessableValue) * 100).toFixed(1)}%
                </td>
              </tr>
              <tr>
                <td className="py-2 px-3 font-sans font-medium">Basic Customs Duty (BCD)</td>
                <td className="py-2 px-3 text-right">{formatINR(baseline.bcd)}</td>
                <td className="py-2 px-3 text-right font-bold text-foreground">{formatINR(simulated.bcd)}</td>
                <td className="py-2 px-3 text-right text-muted-foreground">{formatINR(simulated.bcd - baseline.bcd)}</td>
                <td className="py-2 px-3 text-right">-</td>
              </tr>
              <tr>
                <td className="py-2 px-3 font-sans font-medium">Social Welfare Surcharge (SWS 10%)</td>
                <td className="py-2 px-3 text-right">{formatINR(baseline.sws)}</td>
                <td className="py-2 px-3 text-right font-bold text-foreground">{formatINR(simulated.sws)}</td>
                <td className="py-2 px-3 text-right text-muted-foreground">{formatINR(simulated.sws - baseline.sws)}</td>
                <td className="py-2 px-3 text-right">-</td>
              </tr>
              <tr>
                <td className="py-2 px-3 font-sans font-medium">Other Basic Handling &amp; CHA Fees</td>
                <td className="py-2 px-3 text-right">{formatINR(baseline.expenses)}</td>
                <td className="py-2 px-3 text-right font-bold text-foreground">{formatINR(simulated.expenses)}</td>
                <td className="py-2 px-3 text-right text-muted-foreground">{formatINR(simulated.expenses - baseline.expenses)}</td>
                <td className="py-2 px-3 text-right">{expensesDeltaPct}%</td>
              </tr>
              <tr className="bg-primary/5 font-bold text-sm border-t-2 border-primary/20">
                <td className="py-2.5 px-3 font-sans text-primary">Unit Landed Cost (Per Piece)</td>
                <td className="py-2.5 px-3 text-right text-foreground">{formatINR(baseline.landedCostPerQty)}</td>
                <td className="py-2.5 px-3 text-right text-primary">{formatINR(simulated.landedCostPerQty)}</td>
                <td className="py-2.5 px-3 text-right">
                  {unitVariance > 0 ? `+${formatINR(unitVariance)}` : formatINR(unitVariance)}
                </td>
                <td className="py-2.5 px-3 text-right">
                  {unitVariancePct > 0 ? `+${unitVariancePct.toFixed(2)}%` : `${unitVariancePct.toFixed(2)}%`}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </DialogContent>
    </Dialog>
  );
}
