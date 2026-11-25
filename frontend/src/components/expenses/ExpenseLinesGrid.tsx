import * as React from 'react';
import {
  AlertTriangle,
  ClipboardPaste,
  Copy,
  Merge,
  Plus,
  Settings2,
  ShieldAlert,
  Sparkles,
  Trash2,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { SearchableCombobox } from '@/components/ui/searchable-combobox';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  adaptExpenseTypeRatesForSupply,
  calculateInvoiceLocal,
  validateLineTaxRules,
} from '@/lib/expense-calculator';
import {
  formatPaiseINR,
  type ExpenseLineInput,
  type ExpenseType,
} from '@/services/expense-service';

export interface DraftLine {
  key: string;
  expenseTypeId: string;
  amountInr: string;
  isManualGst: boolean;
  cgstPct: string;
  sgstPct: string;
  igstPct: string;
  manualCgstInr: string;
  manualSgstInr: string;
  manualIgstInr: string;
  tdsPct: string;
  remarks: string;
}

export function makeLineKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `line-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

export function makeEmptyDraftLine(
  et?: ExpenseType,
  isManualGst = false,
  supplyType: 'intra' | 'inter' | 'unknown' = 'unknown',
): DraftLine {
  const adapted = et ? adaptExpenseTypeRatesForSupply(et, supplyType) : null;
  return {
    key: makeLineKey(),
    expenseTypeId: et?.id ?? '',
    amountInr: '0',
    isManualGst,
    cgstPct: adapted ? (adapted.cgstBp / 100).toFixed(2) : '0.00',
    sgstPct: adapted ? (adapted.sgstBp / 100).toFixed(2) : '0.00',
    igstPct: adapted ? (adapted.igstBp / 100).toFixed(2) : '0.00',
    manualCgstInr: '0',
    manualSgstInr: '0',
    manualIgstInr: '0',
    tdsPct: adapted ? (adapted.tdsBp / 100).toFixed(2) : '0.00',
    remarks: '',
  };
}

export function draftLinesToInputs(lines: DraftLine[]): ExpenseLineInput[] {
  return lines.map((l) => ({
    expenseTypeId: l.expenseTypeId,
    amountPaise: Math.round((Number.parseFloat(l.amountInr) || 0) * 100),
    isManualGst: l.isManualGst,
    cgstPaise: l.isManualGst ? Math.round((Number.parseFloat(l.manualCgstInr) || 0) * 100) : 0,
    sgstPaise: l.isManualGst ? Math.round((Number.parseFloat(l.manualSgstInr) || 0) * 100) : 0,
    igstPaise: l.isManualGst ? Math.round((Number.parseFloat(l.manualIgstInr) || 0) * 100) : 0,
    cgstRateBp: Math.round((Number.parseFloat(l.cgstPct) || 0) * 100),
    sgstRateBp: Math.round((Number.parseFloat(l.sgstPct) || 0) * 100),
    igstRateBp: Math.round((Number.parseFloat(l.igstPct) || 0) * 100),
    tdsRateBp: Math.round((Number.parseFloat(l.tdsPct) || 0) * 100),
    remarks: l.remarks,
    roundOff: true,
  }));
}

export function analyzeDraftLines(lines: DraftLine[]) {
  const inputs = draftLinesToInputs(lines);
  const lineErrors: Record<number, string> = {};
  const typeCounts = new Map<string, number>();

  inputs.forEach((inp, idx) => {
    if (inp.expenseTypeId) {
      typeCounts.set(inp.expenseTypeId, (typeCounts.get(inp.expenseTypeId) ?? 0) + 1);
    }
    if (inp.amountPaise > 0) {
      const msg = validateLineTaxRules(inp);
      if (msg) lineErrors[idx] = msg;
    } else if (
      (!inp.isManualGst && (inp.cgstRateBp > 0 || inp.sgstRateBp > 0) && inp.igstRateBp > 0) ||
      (inp.isManualGst && (inp.cgstPaise > 0 || inp.sgstPaise > 0) && inp.igstPaise > 0)
    ) {
      lineErrors[idx] = 'Cannot mix CGST/SGST and IGST on the same line';
    }
  });

  const duplicateTypeIds = new Set(
    Array.from(typeCounts.entries())
      .filter(([, cnt]) => cnt > 1)
      .map(([id]) => id),
  );
  const hasDuplicateType = duplicateTypeIds.size > 0;
  const hasLineErrors = Object.keys(lineErrors).length > 0;
  const hasZeroOrEmptyLine = lines.some(
    (l) => !l.expenseTypeId || Math.round((Number.parseFloat(l.amountInr) || 0) * 100) <= 0,
  );

  return {
    inputs,
    lineErrors,
    duplicateTypeIds,
    hasDuplicateType,
    hasLineErrors,
    hasZeroOrEmptyLine,
  };
}

interface ExpenseLinesGridProps {
  lines: DraftLine[];
  types: ExpenseType[];
  supplyType: 'intra' | 'inter' | 'unknown';
  onChangeLines: (next: DraftLine[]) => void;
  onAddLine: () => void;
  onDuplicateLine: (idx: number) => void;
  onCombineDuplicates?: () => void;
  onOpenPaste?: () => void;
  onCopyLastBill?: () => void;
  onOpenManageTypes?: () => void;
  lineAmountRefs?: React.MutableRefObject<Record<string, HTMLInputElement | null>>;
  billLabelPrefix?: string;
}

export function ExpenseLinesGrid({
  lines,
  types,
  supplyType,
  onChangeLines,
  onAddLine,
  onDuplicateLine,
  onCombineDuplicates,
  onOpenPaste,
  onCopyLastBill,
  onOpenManageTypes,
  lineAmountRefs,
  billLabelPrefix = '',
}: ExpenseLinesGridProps) {
  const analysis = React.useMemo(() => analyzeDraftLines(lines), [lines]);
  const anyManualGst = lines.some((l) => l.isManualGst);
  const allManualGst = lines.length > 0 && lines.every((l) => l.isManualGst);

  const updateLine = (idx: number, patch: Partial<DraftLine>) => {
    const next = lines.map((l, i) => (i === idx ? { ...l, ...patch } : l));
    onChangeLines(next);
  };

  return (
    <div className="space-y-3">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="inline-flex items-center rounded-md border bg-muted/40 p-0.5 text-xs">
          <button
            type="button"
            className={`rounded px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
              !anyManualGst
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            }`}
            onClick={() => {
              onChangeLines(lines.map((l) => ({ ...l, isManualGst: false })));
            }}
          >
            Automatic (% Rate)
          </button>
          <button
            type="button"
            className={`rounded px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
              allManualGst
                ? 'bg-amber-500 text-white shadow-sm dark:bg-amber-600'
                : anyManualGst
                ? 'bg-amber-200 text-amber-900 dark:bg-amber-900 dark:text-amber-100'
                : 'text-muted-foreground hover:text-foreground'
            }`}
            onClick={() => {
              const next = lines.map((l) => {
                const amt = Number.parseFloat(l.amountInr) || 0;
                const cPct = Number.parseFloat(l.cgstPct) || 0;
                const sPct = Number.parseFloat(l.sgstPct) || 0;
                const iPct = Number.parseFloat(l.igstPct) || 0;
                return {
                  ...l,
                  isManualGst: true,
                  manualCgstInr:
                    l.manualCgstInr && l.manualCgstInr !== '0' && l.manualCgstInr !== '0.00'
                      ? Math.round(Number.parseFloat(l.manualCgstInr)).toString()
                      : Math.round((amt * cPct) / 100).toString(),
                  manualSgstInr:
                    l.manualSgstInr && l.manualSgstInr !== '0' && l.manualSgstInr !== '0.00'
                      ? Math.round(Number.parseFloat(l.manualSgstInr)).toString()
                      : Math.round((amt * sPct) / 100).toString(),
                  manualIgstInr:
                    l.manualIgstInr && l.manualIgstInr !== '0' && l.manualIgstInr !== '0.00'
                      ? Math.round(Number.parseFloat(l.manualIgstInr)).toString()
                      : Math.round((amt * iPct) / 100).toString(),
                };
              });
              onChangeLines(next);
            }}
          >
            Manual GST Amount (₹)
          </button>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {onCopyLastBill && (
            <Button
              size="sm"
              variant="outline"
              type="button"
              className="gap-1.5 text-xs h-8"
              onClick={onCopyLastBill}
              title="Load expense lines from this provider's most recent bill"
            >
              <Sparkles className="h-3.5 w-3.5" />
              Load Last Bill
            </Button>
          )}
          {onOpenPaste && (
            <Button
              size="sm"
              variant="outline"
              type="button"
              className="gap-1.5 text-xs h-8"
              onClick={onOpenPaste}
              title="Paste rows from Excel or Tally"
            >
              <ClipboardPaste className="h-3.5 w-3.5" />
              Paste Lines
            </Button>
          )}
          {onOpenManageTypes && (
            <Button
              size="sm"
              variant="outline"
              type="button"
              className="gap-1.5 text-xs h-8"
              onClick={onOpenManageTypes}
            >
              <Settings2 className="h-3.5 w-3.5" />
              Manage Types
            </Button>
          )}
          <Button
            size="sm"
            variant="outline"
            type="button"
            className="gap-1.5 text-xs h-8"
            onClick={onAddLine}
          >
            <Plus className="h-3.5 w-3.5" />
            Add Line
          </Button>
        </div>
      </div>

      {anyManualGst && (
        <div className="rounded-md border border-amber-300 bg-amber-50/50 p-2.5 text-xs text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-200">
          <span className="font-semibold">Manual GST Mode active:</span> Enter exact GST rupee amounts (CGST/SGST or IGST) for lines where some values are exempt or not eligible for full GST.
        </div>
      )}

      {analysis.hasDuplicateType && (
        <div className="flex items-center justify-between rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs">
          <span className="flex items-center gap-2 font-medium text-amber-800 dark:text-amber-300">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            Duplicate expense type detected across lines. Combine matching lines before saving.
          </span>
          {onCombineDuplicates && (
            <Button size="sm" variant="outline" type="button" className="h-7 text-xs" onClick={onCombineDuplicates}>
              <Merge className="h-3.5 w-3.5 mr-1" />
              Combine Duplicate Lines
            </Button>
          )}
        </div>
      )}

      {analysis.hasLineErrors && (
        <div className="flex items-center gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-xs font-medium text-destructive">
          <ShieldAlert className="h-4 w-4 shrink-0" />
          <span>{Object.values(analysis.lineErrors)[0]}</span>
        </div>
      )}

      {/* Compact Grid */}
      <div className="overflow-x-auto rounded-md border">
        <Table>
          <TableHeader className="bg-muted/50">
            <TableRow className="text-xs hover:bg-transparent">
              <TableHead className="w-10 text-center px-2">#</TableHead>
              <TableHead className="min-w-[220px] px-2">Expense Type</TableHead>
              <TableHead className="w-32 px-2">Basic (INR)</TableHead>
              <TableHead className="w-24 px-2 text-center">
                {anyManualGst ? 'CGST (₹)' : 'CGST%'}
              </TableHead>
              <TableHead className="w-24 px-2 text-center">
                {anyManualGst ? 'SGST (₹)' : 'SGST%'}
              </TableHead>
              <TableHead className="w-24 px-2 text-center">
                {anyManualGst ? 'IGST (₹)' : 'IGST%'}
              </TableHead>
              <TableHead className="w-20 px-2 text-center">TDS%</TableHead>
              <TableHead className="w-28 px-2 text-right">Net (INR)</TableHead>
              <TableHead className="min-w-[130px] px-2">Remarks</TableHead>
              <TableHead className="w-20 px-2 text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {lines.map((line, idx) => {
              const isDup = Boolean(line.expenseTypeId && analysis.duplicateTypeIds.has(line.expenseTypeId));
              const lineErr = analysis.lineErrors[idx];
              const lineCalc = calculateInvoiceLocal(draftLinesToInputs([line]), 0);
              const typePool = types.filter((t) => t.isActive || t.id === line.expenseTypeId);

              return (
                <TableRow
                  key={line.key}
                  className={
                    lineErr
                      ? 'bg-destructive/5 hover:bg-destructive/10'
                      : isDup
                      ? 'bg-amber-500/5 hover:bg-amber-500/10'
                      : line.isManualGst
                      ? 'bg-amber-50/20 dark:bg-amber-950/10'
                      : undefined
                  }
                >
                  <TableCell className="text-center font-mono text-xs text-muted-foreground px-2 py-2 align-top pt-3">
                    {idx + 1}
                  </TableCell>
                  <TableCell className="px-2 py-2 align-top space-y-1">
                    <div className="flex items-center gap-1.5">
                      <div className="flex-1 min-w-0">
                        <SearchableCombobox
                          value={line.expenseTypeId}
                          ariaLabel={`${billLabelPrefix}Expense Type line ${idx + 1}`}
                          placeholder="Select type"
                          searchPlaceholder="Search expense type..."
                          className="h-8 text-xs"
                          options={typePool.map((t) => ({
                            value: t.id,
                            label: t.name,
                            sublabel:
                              t.defaultIgstRateBp > 0
                                ? `IGST ${(t.defaultIgstRateBp / 100).toFixed(0)}% · TDS ${(t.defaultTdsRateBp / 100).toFixed(0)}%`
                                : `CGST/SGST ${(t.defaultCgstRateBp / 100).toFixed(0)}%/${(t.defaultSgstRateBp / 100).toFixed(0)}% · TDS ${(t.defaultTdsRateBp / 100).toFixed(0)}%`,
                            keywords: t.name,
                          }))}
                          onValueChange={(v) => {
                            const et = types.find((t) => t.id === v);
                            const adapted = et ? adaptExpenseTypeRatesForSupply(et, supplyType) : null;
                            const cPct = adapted ? (adapted.cgstBp / 100).toFixed(2) : line.cgstPct;
                            const sPct = adapted ? (adapted.sgstBp / 100).toFixed(2) : line.sgstPct;
                            const iPct = adapted ? (adapted.igstBp / 100).toFixed(2) : line.igstPct;
                            const amt = Number.parseFloat(line.amountInr) || 0;
                            updateLine(idx, {
                              expenseTypeId: v,
                              cgstPct: cPct,
                              sgstPct: sPct,
                              igstPct: iPct,
                              manualCgstInr: line.isManualGst
                                ? Math.round((amt * Number.parseFloat(cPct)) / 100).toString()
                                : line.manualCgstInr,
                              manualSgstInr: line.isManualGst
                                ? Math.round((amt * Number.parseFloat(sPct)) / 100).toString()
                                : line.manualSgstInr,
                              manualIgstInr: line.isManualGst
                                ? Math.round((amt * Number.parseFloat(iPct)) / 100).toString()
                                : line.manualIgstInr,
                              tdsPct: adapted ? (adapted.tdsBp / 100).toFixed(2) : line.tdsPct,
                            });
                          }}
                        />
                      </div>
                      <button
                        type="button"
                        tabIndex={-1}
                        className={`shrink-0 text-[10px] px-1.5 py-1 rounded border transition-colors cursor-pointer font-medium ${
                          line.isManualGst
                            ? 'bg-amber-100 text-amber-800 border-amber-300 dark:bg-amber-950 dark:text-amber-200'
                            : 'bg-muted text-muted-foreground border-transparent hover:bg-muted/80'
                        }`}
                        title={
                          line.isManualGst
                            ? 'Manual GST amount mode active. Click to switch to Auto %'
                            : 'Auto percentage mode active. Click to enter manual GST amount'
                        }
                        onClick={() => {
                          const nextManual = !line.isManualGst;
                          let mCgst = line.manualCgstInr;
                          let mSgst = line.manualSgstInr;
                          let mIgst = line.manualIgstInr;
                          if (nextManual && (!mCgst || mCgst === '0' || mCgst === '0.00')) {
                            const amt = Number.parseFloat(line.amountInr) || 0;
                            const cPct = Number.parseFloat(line.cgstPct) || 0;
                            const sPct = Number.parseFloat(line.sgstPct) || 0;
                            const iPct = Number.parseFloat(line.igstPct) || 0;
                            mCgst = Math.round((amt * cPct) / 100).toString();
                            mSgst = Math.round((amt * sPct) / 100).toString();
                            mIgst = Math.round((amt * iPct) / 100).toString();
                          }
                          updateLine(idx, {
                            isManualGst: nextManual,
                            manualCgstInr: mCgst,
                            manualSgstInr: mSgst,
                            manualIgstInr: mIgst,
                          });
                        }}
                      >
                        {line.isManualGst ? 'Manual ₹' : 'Auto %'}
                      </button>
                    </div>
                    {lineErr && <div className="text-[11px] font-medium text-destructive">{lineErr}</div>}
                    {!lineErr && isDup && (
                      <div className="text-[11px] font-medium text-amber-700 dark:text-amber-400">
                        Duplicate expense type
                      </div>
                    )}
                  </TableCell>

                  <TableCell className="px-2 py-2 align-top">
                    <Input
                      ref={(el) => {
                        if (lineAmountRefs) {
                          lineAmountRefs.current[line.key] = el;
                        }
                      }}
                      className="h-8 text-xs font-mono text-right"
                      type="number"
                      min="0"
                      step="1"
                      aria-label={`${billLabelPrefix}Basic (INR)`}
                      value={line.amountInr}
                      onFocus={(e) => e.target.select()}
                      onChange={(e) => updateLine(idx, { amountInr: e.target.value })}
                    />
                  </TableCell>

                  {!line.isManualGst ? (
                    <>
                      <TableCell className="px-2 py-2 align-top">
                        <Input
                          className="h-8 text-xs font-mono text-center px-1.5"
                          type="number"
                          min="0"
                          max="100"
                          step="0.01"
                          aria-label={`${billLabelPrefix}CGST % line ${idx + 1}`}
                          value={line.cgstPct}
                          onFocus={(e) => e.target.select()}
                          onChange={(e) => updateLine(idx, { cgstPct: e.target.value })}
                        />
                      </TableCell>
                      <TableCell className="px-2 py-2 align-top">
                        <Input
                          className="h-8 text-xs font-mono text-center px-1.5"
                          type="number"
                          min="0"
                          max="100"
                          step="0.01"
                          aria-label={`${billLabelPrefix}SGST % line ${idx + 1}`}
                          value={line.sgstPct}
                          onFocus={(e) => e.target.select()}
                          onChange={(e) => updateLine(idx, { sgstPct: e.target.value })}
                        />
                      </TableCell>
                      <TableCell className="px-2 py-2 align-top">
                        <Input
                          className="h-8 text-xs font-mono text-center px-1.5"
                          type="number"
                          min="0"
                          max="100"
                          step="0.01"
                          aria-label={`${billLabelPrefix}IGST % line ${idx + 1}`}
                          value={line.igstPct}
                          onFocus={(e) => e.target.select()}
                          onChange={(e) => updateLine(idx, { igstPct: e.target.value })}
                        />
                      </TableCell>
                    </>
                  ) : (
                    <>
                      <TableCell className="px-2 py-2 align-top">
                        <Input
                          className="h-8 text-xs font-mono text-right border-amber-300 dark:border-amber-800 px-1.5"
                          type="number"
                          min="0"
                          step="1"
                          placeholder="0"
                          aria-label={`${billLabelPrefix}CGST Amount (INR)`}
                          value={line.manualCgstInr}
                          onFocus={(e) => e.target.select()}
                          onChange={(e) => updateLine(idx, { manualCgstInr: e.target.value })}
                        />
                      </TableCell>
                      <TableCell className="px-2 py-2 align-top space-y-0.5">
                        <Input
                          className="h-8 text-xs font-mono text-right border-amber-300 dark:border-amber-800 px-1.5"
                          type="number"
                          min="0"
                          step="1"
                          placeholder="0"
                          aria-label={`${billLabelPrefix}SGST Amount (INR)`}
                          value={line.manualSgstInr}
                          onFocus={(e) => e.target.select()}
                          onChange={(e) => updateLine(idx, { manualSgstInr: e.target.value })}
                        />
                        <button
                          type="button"
                          tabIndex={-1}
                          className="block w-full text-right text-[10px] text-primary hover:underline cursor-pointer font-medium"
                          title="Copy CGST to SGST"
                          onClick={() => updateLine(idx, { manualSgstInr: line.manualCgstInr })}
                        >
                          =CGST
                        </button>
                      </TableCell>
                      <TableCell className="px-2 py-2 align-top">
                        <Input
                          className="h-8 text-xs font-mono text-right border-amber-300 dark:border-amber-800 px-1.5"
                          type="number"
                          min="0"
                          step="1"
                          placeholder="0"
                          aria-label={`${billLabelPrefix}IGST Amount (INR)`}
                          value={line.manualIgstInr}
                          onFocus={(e) => e.target.select()}
                          onChange={(e) => updateLine(idx, { manualIgstInr: e.target.value })}
                        />
                      </TableCell>
                    </>
                  )}

                  <TableCell className="px-2 py-2 align-top">
                    <Input
                      className="h-8 text-xs font-mono text-center px-1.5"
                      type="number"
                      min="0"
                      max="100"
                      step="0.01"
                      aria-label={`${billLabelPrefix}TDS % line ${idx + 1}`}
                      value={line.tdsPct}
                      onFocus={(e) => e.target.select()}
                      onChange={(e) => updateLine(idx, { tdsPct: e.target.value })}
                    />
                  </TableCell>

                  <TableCell className="px-2 py-2 align-top text-right font-mono text-xs font-semibold pt-3">
                    {formatPaiseINR(lineCalc.netAmountPaise)}
                  </TableCell>

                  <TableCell className="px-2 py-2 align-top">
                    <Input
                      className="h-8 text-xs"
                      maxLength={250}
                      placeholder="Line remarks (Enter adds row)"
                      value={line.remarks}
                      onChange={(e) => updateLine(idx, { remarks: e.target.value })}
                      onKeyDown={(e) => {
                        if (
                          e.key === 'Enter' &&
                          !e.shiftKey &&
                          !e.ctrlKey &&
                          !e.metaKey &&
                          idx === lines.length - 1
                        ) {
                          e.preventDefault();
                          onAddLine();
                        }
                      }}
                    />
                  </TableCell>

                  <TableCell className="px-2 py-2 align-top text-right">
                    <div className="flex items-center justify-end gap-0.5">
                      <Button
                        size="sm"
                        variant="ghost"
                        type="button"
                        tabIndex={-1}
                        className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
                        title="Duplicate line"
                        onClick={() => onDuplicateLine(idx)}
                      >
                        <Copy className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        type="button"
                        tabIndex={-1}
                        className="h-8 w-8 p-0 text-muted-foreground hover:text-destructive"
                        disabled={lines.length === 1}
                        title="Delete line"
                        onClick={() => onChangeLines(lines.filter((_, i) => i !== idx))}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      <div className="flex justify-end">
        <Button
          size="sm"
          variant="outline"
          type="button"
          className="gap-1.5 text-xs"
          onClick={onAddLine}
        >
          <Plus className="h-3.5 w-3.5" />
          Add Another Line
        </Button>
      </div>
    </div>
  );
}
