import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  buildPartPeriodAnalysisWorkbook,
  exportPartPeriodAnalysisExcel,
} from '@/lib/part-analysis-export';
import type { PartPeriodAnalysisResult } from '@/services/expense-service';

describe('exportPartPeriodAnalysisExcel and buildPartPeriodAnalysisWorkbook', () => {
  let createdBlob: Blob | null = null;
  let clickedFilename = '';

  beforeEach(() => {
    createdBlob = null;
    clickedFilename = '';

    window.URL.createObjectURL = vi.fn().mockImplementation((blob: Blob) => {
      createdBlob = blob;
      return 'blob:mock-url';
    });
    window.URL.revokeObjectURL = vi.fn();

    // Prevent navigation error in jsdom
    HTMLAnchorElement.prototype.click = vi.fn();

    const originalAppendChild = document.body.appendChild.bind(document.body);
    vi.spyOn(document.body, 'appendChild').mockImplementation((node) => {
      if (node instanceof HTMLAnchorElement) {
        clickedFilename = node.download;
      }
      return originalAppendChild(node);
    });
  });

  const sampleData: PartPeriodAnalysisResult = {
    startDate: '2026-08-18',
    endDate: '2026-11-16',
    dateBasis: 'invoice_date',
    targetItem: {
      id: 'ITM-548143',
      partNumber: '548143',
      itemDescription: 'Acoustic Assy - AIR (Ai3 K1.0/1.2)',
      unit: 'PCS',
      currency: 'KRW',
      unitPrice: 3576,
      hsnCode: '87089900',
      supplierId: 'SUP-KWC',
      supplierName: 'Korea Wecosta Co Ltd',
      isActive: true,
      countryOfOrigin: 'S.Korea',
      bcd: '15',
      sws: '10',
      igst: '18',
      category: 'RM',
      endUse: 'Automotive Air Intake',
      netWeightKg: 0.85,
    },
    summary: {
      totalShipments: 1,
      totalInvoices: 1,
      totalBoes: 1,
      totalQuantity: 14400,
      unit: 'PCS',
      totalForexValue: 51494400,
      currency: 'KRW',
      totalAssessableValueInr: 4035619.8,
      totalBcdAmount: 201781,
      totalSwsAmount: 20178.1,
      totalIgstAmount: 766364.2,
      totalBoeInterest: 0,
      totalCustomsDutyInr: 988323.3,
      totalDutyPaidInr: 988323,
      totalAllocatedExpensesInr: 0,
      totalAllocatedExpensesWithGstInr: 0,
      totalLandedCostExIgst: 4257578.9,
      avgLandedCostPerUnit: 295.67,
      minLandedCostPerUnit: 295.67,
      maxLandedCostPerUnit: 295.67,
      firstImportDate: '2026-08-24',
      latestImportDate: '2026-08-24',
    },
    shipments: [
      {
        shipmentId: 'SHP-003',
        invoiceNumber: 'KWC-260824-001',
        invoiceDate: '2026-08-24',
        supplierName: 'Korea Wecosta Co Ltd',
        incoterm: 'EXW',
        shipmentMode: 'FCL',
        blAwbNumber: 'MVMX230207',
        blAwbDate: '2026-08-29',
        vesselName: '',
        containerNumber: 'KOCU4546737',
        status: 'ready-dly',
        quantity: 14400,
        unit: 'PCS',
        unitPrice: 3576,
        currency: 'KRW',
        lineTotal: 51494400,
        priceDiverged: false,
      },
    ],
    boeLines: [
      {
        shipmentId: 'SHP-003',
        invoiceNumber: 'KWC-260824-001',
        boeCalcId: 'CALC-003',
        boeId: 'BOE-003',
        beNumber: '3897994',
        beDate: '2026-09-20',
        location: 'INMAA1',
        status: 'Reconciled',
        exchangeRate: 0.0715,
        quantity: 14400,
        unit: 'PCS',
        itemForexValue: 51494400,
        itemValueInr: 3681849.6,
        assessableValue: 4035619.8,
        bcdRate: 15,
        bcdAmount: 201781,
        swsRate: 10,
        swsAmount: 20178.1,
        igstRate: 18,
        igstAmount: 766364.2,
        boeInterest: 0,
        totalDuty: 988323.3,
        dutyPaid: 988323,
        challanNumber: '2063268006',
        paymentDate: '2026-09-21',
      },
    ],
    expenses: [],
    trends: [
      {
        date: '2026-08-24',
        shipmentId: 'SHP-003',
        invoiceNumber: 'KWC-260824-001',
        quantity: 14400,
        unitPriceForex: 3576,
        currency: 'KRW',
        assessableValuePerUnit: 280.25,
        dutyPerUnit: 15.41,
        expensePerUnit: 0,
        landedCostPerUnit: 295.67,
      },
    ],
  } as unknown as PartPeriodAnalysisResult;

  it('builds professional multi-sheet Excel workbook matching template specification', async () => {
    const wb = await buildPartPeriodAnalysisWorkbook(sampleData);

    // 1. Verify sheet names
    const sheetNames = wb.worksheets.map((w) => w.name);
    expect(sheetNames).toEqual([
      'Executive Summary',
      'Shipments',
      'BOE Details',
      'Expenses',
      'Cost Trends',
      'Report Metadata',
    ]);

    // 2. Verify Executive Summary
    const wsSummary = wb.getWorksheet('Executive Summary')!;
    expect(wsSummary.getCell('A1').value).toBe('PART / ITEM PERIOD ANALYSIS');
    expect(wsSummary.getCell('A1').fill).toMatchObject({
      type: 'pattern',
      pattern: 'solid',
      fgColor: { argb: 'FF17365D' },
    });
    expect(wsSummary.getCell('A2').value).toBe(
      'Professional reporting workbook | Source period: 18-Aug-2026 to 16-Nov-2026',
    );
    expect(wsSummary.getCell('A4').value).toBe('REPORT CONTROL & ITEM MASTER PROFILE');
    expect(wsSummary.getCell('A5').value).toBe('Part Number');
    expect(wsSummary.getCell('B5').value).toBe('548143');
    expect(wsSummary.getCell('C5').value).toBe('Description');
    expect(wsSummary.getCell('D5').value).toBe('Acoustic Assy - AIR (Ai3 K1.0/1.2)');

    expect(wsSummary.getCell('A15').value).toBe('EXECUTIVE SUMMARY — PERIOD TOTALS');
    expect(wsSummary.getCell('A16').value).toBe('Total Shipments');
    expect(wsSummary.getCell('B16').value).toBe('1');
    expect(wsSummary.getCell('A24').value).toBe('REPORT NOTES');
    expect(wsSummary.getCell('A25').value).toContain('All source fields are retained');

    // 3. Verify Shipments worksheet
    const wsShipments = wb.getWorksheet('Shipments')!;
    expect(wsShipments.getCell('A1').value).toBe('RELATED SHIPMENTS & INVOICES');
    expect(wsShipments.getCell('A4').value).toBe('Shipment ID');
    expect(wsShipments.getCell('Q4').value).toBe('Price Diverged');
    expect(wsShipments.getCell('A5').value).toBe('SHP-003');
    expect(wsShipments.getCell('B5').value).toBe('KWC-260824-001');

    // 4. Verify BOE Details worksheet
    const wsBOE = wb.getWorksheet('BOE Details')!;
    expect(wsBOE.getCell('A1').value).toBe('BILL OF ENTRY — CUSTOMS VALUATION & DUTY');
    expect(wsBOE.getCell('A4').value).toBe('Shipment ID');
    expect(wsBOE.getCell('C5').value).toBe('3897994');
    expect(wsBOE.getCell('K5').value).toBe('201781.00');

    // 5. Verify Expenses worksheet
    const wsExpenses = wb.getWorksheet('Expenses')!;
    expect(wsExpenses.getCell('A1').value).toBe('ALLOCATED SHIPMENT EXPENSES');
    expect(wsExpenses.getCell('A2').value).toContain('No data rows were present');
    expect(wsExpenses.getCell('A4').value).toBe('Shipment ID');

    // 6. Verify Cost Trends worksheet
    const wsTrends = wb.getWorksheet('Cost Trends')!;
    expect(wsTrends.getCell('A1').value).toBe('CHRONOLOGICAL COST TRENDS');
    expect(wsTrends.getCell('A4').value).toBe('Date');
    expect(wsTrends.getCell('A5').value).toBe('2026-08-24');
    expect(wsTrends.getCell('J5').value).toBe('295.67');

    // 7. Verify Report Metadata worksheet
    const wsMeta = wb.getWorksheet('Report Metadata')!;
    expect(wsMeta.getCell('A1').value).toBe('REPORT METADATA & FIELD INVENTORY');
    expect(wsMeta.getCell('A4').value).toBe('Section');
    expect(wsMeta.getCell('B4').value).toBe('Field Count');
    expect(wsMeta.getCell('A5').value).toBe('Item Master Profile');
    expect(wsMeta.getCell('B5').value).toBe(13);
  });

  it('triggers browser file download with expected file name', async () => {
    await exportPartPeriodAnalysisExcel(sampleData);

    expect(createdBlob).not.toBeNull();
    expect(clickedFilename).toBe('548143_Period_Analysis_2026-08-18_to_2026-11-16.xlsx');
  });
});
