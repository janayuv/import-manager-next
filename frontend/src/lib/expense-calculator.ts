import type {
  ExpenseInvoiceCalculation,
  ExpenseLineInput,
  ExpenseType,
} from '@/services/expense-service';

type ExpenseLineCalculation = ExpenseInvoiceCalculation['lines'][number];

export const BASIS_POINTS_SCALE = 10000;
export const MAX_ROUND_OFF_PAISE = 10000; // ±₹100

/**
 * Symmetric whole-rupee rounding in integer paise, matching backend money.RoundNearestRupee.
 */
export function roundNearestRupee(paise: number): number {
  if (!Number.isFinite(paise) || paise === 0) return 0;
  const sign = paise < 0 ? -1 : 1;
  const val = Math.abs(Math.trunc(paise));
  return sign * Math.floor((val + 50) / 100) * 100;
}

export function extractStateCodeFromGstin(gstin?: string): string {
  const clean = (gstin || '').trim().toUpperCase();
  return /^[0-9]{2}/.test(clean) ? clean.slice(0, 2) : '';
}

export function detectSupplyType(
  companyGstin?: string,
  companyState?: string,
  providerGstin?: string,
  providerState?: string,
): 'intra' | 'inter' | 'unknown' {
  const compCode = extractStateCodeFromGstin(companyGstin);
  const provCode = extractStateCodeFromGstin(providerGstin);
  if (compCode && provCode) {
    return compCode === provCode ? 'intra' : 'inter';
  }
  const compState = (companyState || '').trim().toLowerCase();
  const provState = (providerState || '').trim().toLowerCase();
  if (compState && provState) {
    return compState === provState ? 'intra' : 'inter';
  }
  return 'unknown';
}

export function adaptExpenseTypeRatesForSupply(
  et: ExpenseType,
  supplyType: 'intra' | 'inter' | 'unknown',
): { cgstBp: number; sgstBp: number; igstBp: number; tdsBp: number } {
  let cgstBp = et.defaultCgstRateBp || 0;
  let sgstBp = et.defaultSgstRateBp || 0;
  let igstBp = et.defaultIgstRateBp || 0;
  const tdsBp = et.defaultTdsRateBp || 0;

  if (supplyType === 'inter' && (cgstBp > 0 || sgstBp > 0) && igstBp === 0) {
    igstBp = cgstBp + sgstBp;
    cgstBp = 0;
    sgstBp = 0;
  } else if (supplyType === 'intra' && igstBp > 0 && cgstBp === 0 && sgstBp === 0) {
    const half = Math.floor(igstBp / 2);
    cgstBp = half;
    sgstBp = igstBp - half;
    igstBp = 0;
  }

  return { cgstBp, sgstBp, igstBp, tdsBp };
}

export function validateLineTaxRules(line: ExpenseLineInput): string | null {
  if (!line.expenseTypeId) return 'Expense type is required';
  if (line.amountPaise <= 0) return 'Basic amount must be positive';
  if (line.tdsRateBp < 0 || line.tdsRateBp > 10000) return 'TDS% must be between 0% and 100%';

  if (line.isManualGst) {
    if (line.cgstPaise < 0 || line.sgstPaise < 0 || line.igstPaise < 0) {
      return 'Manual GST amounts cannot be negative';
    }
    if ((line.cgstPaise > 0 || line.sgstPaise > 0) && line.igstPaise > 0) {
      return 'Cannot mix CGST/SGST and IGST on the same line';
    }
  } else {
    if (line.cgstRateBp < 0 || line.sgstRateBp < 0 || line.igstRateBp < 0) {
      return 'Tax rates cannot be negative';
    }
    if (line.cgstRateBp > 10000 || line.sgstRateBp > 10000 || line.igstRateBp > 10000) {
      return 'Tax rates cannot exceed 100%';
    }
    if ((line.cgstRateBp > 0 || line.sgstRateBp > 0) && line.igstRateBp > 0) {
      return 'Cannot mix CGST/SGST and IGST on the same line';
    }
    if (line.cgstRateBp !== line.sgstRateBp) {
      return 'CGST% and SGST% must match in Auto mode';
    }
  }
  return null;
}

export function calcLinePaiseLocal(line: ExpenseLineInput): ExpenseLineCalculation {
  const amt = Math.max(0, Math.trunc(line.amountPaise || 0));
  let cgst = 0;
  let sgst = 0;
  let igst = 0;
  let cgstRateBp = line.cgstRateBp || 0;
  let sgstRateBp = line.sgstRateBp || 0;
  let igstRateBp = line.igstRateBp || 0;

  if (line.isManualGst) {
    cgst = Math.max(0, Math.trunc(line.cgstPaise || 0));
    sgst = Math.max(0, Math.trunc(line.sgstPaise || 0));
    igst = Math.max(0, Math.trunc(line.igstPaise || 0));
    if (line.roundOff) {
      cgst = roundNearestRupee(cgst);
      sgst = roundNearestRupee(sgst);
      igst = roundNearestRupee(igst);
    }
    if (cgstRateBp === 0 && amt > 0 && cgst > 0) {
      cgstRateBp = Math.round((cgst * BASIS_POINTS_SCALE) / amt);
    }
    if (sgstRateBp === 0 && amt > 0 && sgst > 0) {
      sgstRateBp = Math.round((sgst * BASIS_POINTS_SCALE) / amt);
    }
    if (igstRateBp === 0 && amt > 0 && igst > 0) {
      igstRateBp = Math.round((igst * BASIS_POINTS_SCALE) / amt);
    }
  } else {
    const rawCgst = Math.trunc((amt * cgstRateBp) / BASIS_POINTS_SCALE);
    const rawSgst = Math.trunc((amt * sgstRateBp) / BASIS_POINTS_SCALE);
    const rawIgst = Math.trunc((amt * igstRateBp) / BASIS_POINTS_SCALE);
    if (line.roundOff) {
      cgst = roundNearestRupee(rawCgst);
      sgst = roundNearestRupee(rawSgst);
      igst = roundNearestRupee(rawIgst);
    } else {
      cgst = rawCgst;
      sgst = rawSgst;
      igst = rawIgst;
    }
  }

  const rawTds = Math.trunc((amt * (line.tdsRateBp || 0)) / BASIS_POINTS_SCALE);
  const tds = line.roundOff ? roundNearestRupee(rawTds) : rawTds;

  const totalPaise = amt + cgst + sgst + igst;
  const netPaise = totalPaise - tds;

  return {
    expenseTypeId: line.expenseTypeId,
    amountPaise: amt,
    isManualGst: Boolean(line.isManualGst),
    cgstRateBp,
    sgstRateBp,
    igstRateBp,
    tdsRateBp: line.tdsRateBp || 0,
    cgstPaise: cgst,
    sgstPaise: sgst,
    igstPaise: igst,
    tdsPaise: tds,
    totalPaise,
    netPaise,
    amountInr: amt / 100,
    totalInr: totalPaise / 100,
    netInr: netPaise / 100,
    remarks: (line.remarks || '').trim(),
  };
}

export function calculateInvoiceLocal(
  lines: ExpenseLineInput[],
  roundOffPaise = 0,
): ExpenseInvoiceCalculation {
  const seen = new Set<string>();
  let hasDuplicateType = false;
  const calcLines: ExpenseLineCalculation[] = [];

  let baseAmountPaise = 0;
  let totalCgstPaise = 0;
  let totalSgstPaise = 0;
  let totalIgstPaise = 0;
  let totalTdsPaise = 0;
  let totalAmountPaise = 0;
  let netAmountPaise = 0;

  for (const line of lines) {
    const key = (line.expenseTypeId || '').trim();
    if (key) {
      if (seen.has(key)) hasDuplicateType = true;
      seen.add(key);
    }
    const calc = calcLinePaiseLocal(line);
    calcLines.push(calc);
    baseAmountPaise += calc.amountPaise;
    totalCgstPaise += calc.cgstPaise;
    totalSgstPaise += calc.sgstPaise;
    totalIgstPaise += calc.igstPaise;
    totalTdsPaise += calc.tdsPaise;
    totalAmountPaise += calc.totalPaise;
    netAmountPaise += calc.netPaise;
  }

  return {
    lines: calcLines,
    baseAmountPaise,
    totalCgstPaise,
    totalSgstPaise,
    totalIgstPaise,
    totalTdsPaise,
    roundOffPaise,
    totalAmountPaise: totalAmountPaise + roundOffPaise,
    netAmountPaise: netAmountPaise + roundOffPaise,
    totalAmountInr: (totalAmountPaise + roundOffPaise) / 100,
    netAmountInr: (netAmountPaise + roundOffPaise) / 100,
    hasDuplicateType,
  };
}

export interface ParsedPasteRow {
  expenseType: ExpenseType;
  amountInr: string;
  cgstPct?: string;
  sgstPct?: string;
  igstPct?: string;
  tdsPct?: string;
  remarks: string;
}

export function parsePastedExpenseLines(
  rawText: string,
  activeTypes: ExpenseType[],
  supplyType: 'intra' | 'inter' | 'unknown',
): { rows: ParsedPasteRow[]; errors: string[] } {
  const lines = rawText
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);

  const rows: ParsedPasteRow[] = [];
  const errors: string[] = [];

  lines.forEach((rawLine, i) => {
    const delimiter = rawLine.includes('\t') ? '\t' : ',';
    const cols = rawLine.split(delimiter).map((c) => c.trim().replace(/^"|"$/g, ''));
    if (cols.length < 2) {
      errors.push(`Row ${i + 1}: expected at least Expense Type and Amount`);
      return;
    }

    const typeQuery = cols[0]!.toLowerCase();
    // Skip header row if user pasted column titles
    if (i === 0 && (typeQuery === 'expense type' || typeQuery === 'expense_type' || typeQuery === 'particulars')) {
      return;
    }

    const matched =
      activeTypes.find((t) => t.name.toLowerCase() === typeQuery) ||
      activeTypes.find((t) => t.name.toLowerCase().includes(typeQuery));
    if (!matched) {
      errors.push(`Row ${i + 1}: unknown Expense Type "${cols[0]}"`);
      return;
    }

    const cleanAmt = Number.parseFloat((cols[1] || '').replace(/[₹,\s]/g, ''));
    if (!Number.isFinite(cleanAmt) || cleanAmt <= 0) {
      errors.push(`Row ${i + 1}: invalid positive amount "${cols[1]}"`);
      return;
    }

    const adapted = adaptExpenseTypeRatesForSupply(matched, supplyType);
    const parsePct = (val: string | undefined, fallbackBp: number) => {
      if (val === undefined || val === '') return (fallbackBp / 100).toFixed(2);
      const n = Number.parseFloat(val.replace(/%/g, ''));
      return Number.isFinite(n) && n >= 0 ? n.toFixed(2) : (fallbackBp / 100).toFixed(2);
    };

    // Supports:
    // 2 cols: Type, Amount
    // 3 cols: Type, Amount, Remarks
    // 6+ cols: Type, Amount, CGST%, SGST%, IGST%, TDS%, [Remarks]
    let cgstPct = (adapted.cgstBp / 100).toFixed(2);
    let sgstPct = (adapted.sgstBp / 100).toFixed(2);
    let igstPct = (adapted.igstBp / 100).toFixed(2);
    let tdsPct = (adapted.tdsBp / 100).toFixed(2);
    let remarks = '';

    if (cols.length === 3) {
      remarks = cols[2] || '';
    } else if (cols.length >= 4) {
      cgstPct = parsePct(cols[2], adapted.cgstBp);
      sgstPct = parsePct(cols[3], adapted.sgstBp);
      igstPct = parsePct(cols[4], adapted.igstBp);
      tdsPct = parsePct(cols[5], adapted.tdsBp);
      remarks = cols[6] || '';
    }

    rows.push({
      expenseType: matched,
      amountInr: Math.round(cleanAmt).toString(),
      cgstPct,
      sgstPct,
      igstPct,
      tdsPct,
      remarks,
    });
  });

  return { rows, errors };
}
