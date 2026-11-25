import { describe, expect, it } from 'vitest';
import {
  adaptExpenseTypeRatesForSupply,
  calcLinePaiseLocal,
  calculateInvoiceLocal,
  detectSupplyType,
  parsePastedExpenseLines,
  roundNearestRupee,
  validateLineTaxRules,
} from '@/lib/expense-calculator';
import type { ExpenseType } from '@/services/expense-service';

const sampleTypes: ExpenseType[] = [
  {
    id: 'ET-1',
    name: 'Customs Clearance',
    defaultCgstRateBp: 900,
    defaultSgstRateBp: 900,
    defaultIgstRateBp: 0,
    defaultTdsRateBp: 200,
    isActive: true,
    createdAt: '',
  },
  {
    id: 'ET-2',
    name: 'Ocean Freight',
    defaultCgstRateBp: 0,
    defaultSgstRateBp: 0,
    defaultIgstRateBp: 1800,
    defaultTdsRateBp: 200,
    isActive: true,
    createdAt: '',
  },
];

describe('expense-calculator', () => {
  it('performs symmetric whole-rupee rounding matching backend money.RoundNearestRupee', () => {
    expect(roundNearestRupee(56250)).toBe(56300);
    expect(roundNearestRupee(30591)).toBe(30600);
    expect(roundNearestRupee(-140)).toBe(-100);
    expect(roundNearestRupee(-160)).toBe(-200);
  });

  it('computes exact integer paise totals locally without float drift', () => {
    const calc = calculateInvoiceLocal(
      [
        {
          expenseTypeId: 'ET-1',
          amountPaise: 625000,
          isManualGst: false,
          roundOff: true,
          cgstPaise: 0,
          sgstPaise: 0,
          igstPaise: 0,
          cgstRateBp: 900,
          sgstRateBp: 900,
          igstRateBp: 0,
          tdsRateBp: 0,
          remarks: 'Transport',
        },
        {
          expenseTypeId: 'ET-2',
          amountPaise: 339900,
          isManualGst: false,
          roundOff: true,
          cgstPaise: 0,
          sgstPaise: 0,
          igstPaise: 0,
          cgstRateBp: 900,
          sgstRateBp: 900,
          igstRateBp: 0,
          tdsRateBp: 0,
          remarks: 'Clearance',
        },
      ],
      -100,
    );
    expect(calc.baseAmountPaise).toBe(964900);
    expect(calc.totalCgstPaise).toBe(86900);
    expect(calc.totalSgstPaise).toBe(86900);
    expect(calc.netAmountPaise).toBe(1138600);
  });

  it('detects intra vs inter state from GSTIN/state and adapts tax defaults', () => {
    expect(detectSupplyType('33AAAAA0000A1Z5', 'Tamil Nadu', '33BBBBB1111B1Z6', 'TN')).toBe('intra');
    expect(detectSupplyType('33AAAAA0000A1Z5', 'Tamil Nadu', '27BBBBB1111B1Z6', 'MH')).toBe('inter');

    const adaptedInter = adaptExpenseTypeRatesForSupply(sampleTypes[0]!, 'inter');
    expect(adaptedInter).toEqual({ cgstBp: 0, sgstBp: 0, igstBp: 1800, tdsBp: 200 });

    const adaptedIntra = adaptExpenseTypeRatesForSupply(sampleTypes[1]!, 'intra');
    expect(adaptedIntra).toEqual({ cgstBp: 900, sgstBp: 900, igstBp: 0, tdsBp: 200 });
  });

  it('rejects mixing CGST/SGST and IGST on a single line', () => {
    const err = validateLineTaxRules({
      expenseTypeId: 'ET-1',
      amountPaise: 100000,
      isManualGst: false,
      roundOff: true,
      cgstPaise: 0,
      sgstPaise: 0,
      igstPaise: 0,
      cgstRateBp: 900,
      sgstRateBp: 900,
      igstRateBp: 1800,
      tdsRateBp: 0,
      remarks: '',
    });
    expect(err).toMatch(/Cannot mix CGST\/SGST and IGST/i);
  });

  it('parses pasted tab-separated Excel rows cleanly', () => {
    const { rows, errors } = parsePastedExpenseLines(
      'Expense Type\tAmount\tRemarks\nCustoms Clearance\t15000\tBOE pass\nOcean Freight\t25000\tSea',
      sampleTypes,
      'inter',
    );
    expect(errors).toHaveLength(0);
    expect(rows).toHaveLength(2);
    expect(rows[0]!.expenseType.id).toBe('ET-1');
    expect(rows[0]!.igstPct).toBe('18.00');
    expect(rows[1]!.amountInr).toBe('25000');
  });

  it('computes manual GST line totals accurately', () => {
    const line = calcLinePaiseLocal({
      expenseTypeId: 'ET-1',
      amountPaise: 5000000,
      isManualGst: true,
      roundOff: true,
      cgstPaise: 90000,
      sgstPaise: 90000,
      igstPaise: 0,
      cgstRateBp: 0,
      sgstRateBp: 0,
      igstRateBp: 0,
      tdsRateBp: 200,
      remarks: '',
    });
    expect(line.totalPaise).toBe(5180000);
    expect(line.netPaise).toBe(5080000);
  });
});
