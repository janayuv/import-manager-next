import ExcelJS from 'exceljs';
import type { PartPeriodAnalysisResult } from '@/services/expense-service';

// Color Palette matching the Professional Template
const COLORS = {
  TITLE_NAVY: 'FF17365D',       // #17365D - Title Banner Fill
  SECTION_STEEL: 'FF1F4E78',     // #1F4E78 - Section Header Fill & Profile Text
  TABLE_TEAL: 'FF0F6B78',        // #0F6B78 - Table Column Headers Fill
  ICE_BLUE: 'FFEAF2F8',          // #EAF2F8 - Label Cell Background
  ZEBRA_ROW: 'FFF5F8FB',         // #F5F8FB - Even Row Striping Fill
  BORDER_LIGHT: 'FFD9E1F2',      // #D9E1F2 - Cell Border Line
  TEXT_MUTED: 'FF666666',        // #666666 - Subtitles & notes
  TEXT_DARK: 'FF222222',         // #222222 - Primary data text
  WHITE: 'FFFFFFFF',             // #FFFFFF - White text & fill
};

const BORDER_THIN: Partial<ExcelJS.Borders> = {
  top: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
  bottom: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
  left: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
  right: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
};

const BORDER_BOTTOM_ONLY: Partial<ExcelJS.Borders> = {
  bottom: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
};

function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function formatDateDisplay(dStr?: string): string {
  if (!dStr) return '';
  const d = new Date(dStr);
  if (isNaN(d.getTime())) return dStr;
  const day = String(d.getDate()).padStart(2, '0');
  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  const mon = months[d.getMonth()];
  const yr = d.getFullYear();
  return `${day}-${mon}-${yr}`;
}

function formatPeriodSubtitle(startDate?: string, endDate?: string): string {
  if (!startDate && !endDate) return 'All Recorded Periods';
  const s = formatDateDisplay(startDate) || 'Start';
  const e = formatDateDisplay(endDate) || 'Current';
  return `${s} to ${e}`;
}

function styleTitleBanner(
  ws: ExcelJS.Worksheet,
  title: string,
  subtitle: string,
  colCount: number,
) {
  const colLetter = ws.getColumn(colCount).letter;

  // Row 1: Main Title
  ws.mergeCells(`A1:${colLetter}1`);
  const r1 = ws.getRow(1);
  r1.height = 34;
  const c1 = r1.getCell(1);
  c1.value = title;
  c1.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.TITLE_NAVY } };
  c1.font = { name: 'Aptos Display', size: 18, bold: true, color: { argb: COLORS.WHITE } };
  c1.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Row 2: Subtitle
  ws.mergeCells(`A2:${colLetter}2`);
  const r2 = ws.getRow(2);
  r2.height = 22;
  const c2 = r2.getCell(1);
  c2.value = subtitle;
  c2.font = { name: 'Aptos', size: 10, italic: true, color: { argb: COLORS.TEXT_MUTED } };
  c2.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Row 3: Blank separator
  ws.getRow(3).height = 12;
}

function styleTableHeaders(
  ws: ExcelJS.Worksheet,
  rowNumber: number,
  headers: string[],
) {
  const row = ws.getRow(rowNumber);
  row.height = 34;
  headers.forEach((h, idx) => {
    const cell = row.getCell(idx + 1);
    cell.value = h;
    cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.TABLE_TEAL } };
    cell.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.WHITE } };
    cell.alignment = { vertical: 'middle', horizontal: 'center', wrapText: true };
    cell.border = {
      top: { style: 'thin', color: { argb: 'FF0A4C55' } },
      bottom: { style: 'medium', color: { argb: 'FF0A4C55' } },
      left: { style: 'thin', color: { argb: 'FF0A4C55' } },
      right: { style: 'thin', color: { argb: 'FF0A4C55' } },
    };
  });
}

function styleDataRow(
  row: ExcelJS.Row,
  colCount: number,
  isEven: boolean,
  alignments?: ('left' | 'center' | 'right')[],
) {
  row.height = 21;
  for (let c = 1; c <= colCount; c++) {
    const cell = row.getCell(c);
    cell.font = { name: 'Aptos', size: 10, color: { argb: COLORS.TEXT_DARK } };
    cell.border = BORDER_THIN;
    if (isEven) {
      cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.ZEBRA_ROW } };
    }
    const align = alignments?.[c - 1] || 'left';
    cell.alignment = { vertical: 'middle', horizontal: align };
  }
}

export async function buildPartPeriodAnalysisWorkbook(
  data: PartPeriodAnalysisResult,
): Promise<ExcelJS.Workbook> {
  const wb = new ExcelJS.Workbook();
  wb.creator = 'Import Manager Next';
  wb.lastModifiedBy = 'Import Manager Next';
  wb.created = new Date();
  wb.modified = new Date();

  const item = data.targetItem || {
    partNumber: '',
    itemDescription: '',
    unit: 'PCS',
    currency: 'USD',
    unitPrice: 0,
    hsnCode: '',
    supplierName: '',
    countryOfOrigin: '',
    category: '',
    isActive: true,
    bcd: '',
    sws: '',
    igst: '',
  };
  const sum = data.summary || {
    totalShipments: 0,
    totalInvoices: 0,
    totalBoes: 0,
    totalQuantity: 0,
    totalForexValue: 0,
    totalAssessableValueInr: 0,
    totalBcdAmount: 0,
    totalSwsAmount: 0,
    totalIgstAmount: 0,
    totalBoeInterest: 0,
    totalCustomsDutyInr: 0,
    totalDutyPaidInr: 0,
    totalAllocatedExpensesInr: 0,
    totalAllocatedExpensesWithGstInr: 0,
    totalLandedCostExIgst: 0,
    avgLandedCostPerUnit: 0,
    minLandedCostPerUnit: 0,
    maxLandedCostPerUnit: 0,
    currency: item.currency || 'USD',
    unit: item.unit || 'PCS',
  };

  const periodStr = formatPeriodSubtitle(data.startDate, data.endDate);
  const dateRangeExact = data.startDate && data.endDate ? `${data.startDate} to ${data.endDate}` : 'All Recorded Dates';

  // =========================================================================
  // SHEET 1: Executive Summary
  // =========================================================================
  const wsSummary = wb.addWorksheet('Executive Summary', {
    views: [{ state: 'frozen', xSplit: 0, ySplit: 4, topLeftCell: 'A14', showGridLines: false, zoomScale: 90 }],
  });
  wsSummary.columns = [
    { width: 29 },
    { width: 22 },
    { width: 31 },
    { width: 22 },
    { width: 30 },
    { width: 22 },
  ];

  styleTitleBanner(
    wsSummary,
    'PART / ITEM PERIOD ANALYSIS',
    `Professional reporting workbook | Source period: ${periodStr}`,
    6,
  );

  // Section 1: Report Control & Item Master Profile Header
  wsSummary.mergeCells('A4:F4');
  const rSec1 = wsSummary.getRow(4);
  rSec1.height = 23.1;
  const cSec1 = rSec1.getCell(1);
  cSec1.value = 'REPORT CONTROL & ITEM MASTER PROFILE';
  cSec1.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.SECTION_STEEL } };
  cSec1.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.WHITE } };
  cSec1.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Profile Rows 5 to 12
  const profileRows: [string, any, string, any][] = [
    ['Part Number', item.partNumber || '-', 'Description', item.itemDescription || '-'],
    ['HSN Code', item.hsnCode || '-', 'Unit', item.unit || '-'],
    ['Currency', item.currency || '-', 'Master Unit Price', (item.unitPrice || 0).toFixed(2)],
    ['Supplier', item.supplierName || '-', 'Country of Origin', item.countryOfOrigin || '-'],
    ['Category', item.category || '-', 'Status', item.isActive ? 'Active' : 'Inactive'],
    ['BCD %', item.bcd || '0', 'SWS %', item.sws || '0'],
    ['IGST %', item.igst || '0', 'Period Basis', data.dateBasis || 'invoice_date'],
    ['Generated At', new Date().toLocaleString(), 'Date Range', dateRangeExact],
  ];

  profileRows.forEach(([lbl1, val1, lbl2, val2], idx) => {
    const rowNum = 5 + idx;
    const row = wsSummary.getRow(rowNum);
    row.height = 20;

    // Col A: Label 1
    const ca = row.getCell(1);
    ca.value = lbl1;
    ca.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.ICE_BLUE } };
    ca.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.SECTION_STEEL } };
    ca.border = BORDER_BOTTOM_ONLY;
    ca.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

    // Col B: Val 1
    const cb = row.getCell(2);
    cb.value = val1;
    cb.font = { name: 'Calibri', size: 11, color: { argb: COLORS.TEXT_DARK } };
    cb.border = BORDER_BOTTOM_ONLY;
    cb.alignment = { vertical: 'middle', horizontal: 'left', wrapText: true };

    // Col C: Label 2
    const cc = row.getCell(3);
    cc.value = lbl2;
    cc.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.ICE_BLUE } };
    cc.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.SECTION_STEEL } };
    cc.border = BORDER_BOTTOM_ONLY;
    cc.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

    // Col D to F: Merged Val 2
    wsSummary.mergeCells(`D${rowNum}:F${rowNum}`);
    const cd = row.getCell(4);
    cd.value = val2;
    cd.font = { name: 'Calibri', size: 11, color: { argb: COLORS.TEXT_DARK } };
    cd.border = BORDER_BOTTOM_ONLY;
    cd.alignment = { vertical: 'middle', horizontal: 'left', wrapText: true };

    row.getCell(5).border = BORDER_BOTTOM_ONLY;
    row.getCell(6).border = BORDER_BOTTOM_ONLY;
  });

  // Empty separator rows
  wsSummary.getRow(13).height = 14;
  wsSummary.getRow(14).height = 14;

  // Section 2: Executive Summary Period Totals Header
  wsSummary.mergeCells('A15:F15');
  const rSec2 = wsSummary.getRow(15);
  rSec2.height = 23.1;
  const cSec2 = rSec2.getCell(1);
  cSec2.value = 'EXECUTIVE SUMMARY — PERIOD TOTALS';
  cSec2.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.SECTION_STEEL } };
  cSec2.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.WHITE } };
  cSec2.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Totals Grid Rows 16 to 21
  const kpiGrid: [string, any, string, any, string, any][] = [
    [
      'Total Shipments', String(sum.totalShipments || 0),
      'Total Invoices', String(sum.totalInvoices || 0),
      'Total BOEs', String(sum.totalBoes || 0),
    ],
    [
      'Total Quantity', `${(sum.totalQuantity || 0).toFixed(2)} ${sum.unit || 'PCS'}`,
      'Total Forex Value', `${(sum.totalForexValue || 0).toFixed(2)} ${sum.currency || 'USD'}`,
      'Total Assessable Value (INR)', (sum.totalAssessableValueInr || 0).toFixed(2),
    ],
    [
      'Basic Customs Duty (BCD)', (sum.totalBcdAmount || 0).toFixed(2),
      'Social Welfare Surcharge (SWS)', (sum.totalSwsAmount || 0).toFixed(2),
      'Integrated GST (IGST)', (sum.totalIgstAmount || 0).toFixed(2),
    ],
    [
      'BOE Interest Allocated', (sum.totalBoeInterest || 0).toFixed(2),
      'Total Customs Duties', (sum.totalCustomsDutyInr || 0).toFixed(2),
      'Total Duty Paid', (sum.totalDutyPaidInr || 0).toFixed(2),
    ],
    [
      'Allocated Basic Expenses', (sum.totalAllocatedExpensesInr || 0).toFixed(2),
      'Allocated Expenses (incl. GST)', (sum.totalAllocatedExpensesWithGstInr || 0).toFixed(2),
      'Total Landed Cost (excl. IGST)', (sum.totalLandedCostExIgst || 0).toFixed(2),
    ],
    [
      'Weighted Avg Landed Cost / Unit', (sum.avgLandedCostPerUnit || 0).toFixed(2),
      'Min Landed Cost / Unit', (sum.minLandedCostPerUnit || 0).toFixed(2),
      'Max Landed Cost / Unit', (sum.maxLandedCostPerUnit || 0).toFixed(2),
    ],
  ];

  kpiGrid.forEach(([l1, v1, l2, v2, l3, v3], idx) => {
    const rowNum = 16 + idx;
    const row = wsSummary.getRow(rowNum);
    row.height = 20;

    const pairs = [
      { colLabel: 1, colVal: 2, lbl: l1, val: v1 },
      { colLabel: 3, colVal: 4, lbl: l2, val: v2 },
      { colLabel: 5, colVal: 6, lbl: l3, val: v3 },
    ];

    pairs.forEach(p => {
      const cLabel = row.getCell(p.colLabel);
      cLabel.value = p.lbl;
      cLabel.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.ICE_BLUE } };
      cLabel.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.SECTION_STEEL } };
      cLabel.border = BORDER_BOTTOM_ONLY;
      cLabel.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

      const cVal = row.getCell(p.colVal);
      cVal.value = p.val;
      cVal.font = { name: 'Calibri', size: 11, color: { argb: COLORS.TEXT_DARK } };
      cVal.border = BORDER_BOTTOM_ONLY;
      cVal.alignment = { vertical: 'middle', horizontal: 'right', indent: 1 };
    });
  });

  // Empty separator rows
  wsSummary.getRow(22).height = 14;
  wsSummary.getRow(23).height = 14;

  // Section 3: Report Notes Header
  wsSummary.mergeCells('A24:F24');
  const rSec3 = wsSummary.getRow(24);
  rSec3.height = 23.1;
  const cSec3 = rSec3.getCell(1);
  cSec3.value = 'REPORT NOTES';
  cSec3.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.SECTION_STEEL } };
  cSec3.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.WHITE } };
  cSec3.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Notes Box (Merged A25:F26)
  wsSummary.mergeCells('A25:F26');
  const rNotes = wsSummary.getRow(25);
  rNotes.height = 36;
  const cNotes = rNotes.getCell(1);
  const expenseCount = data.expenses?.length || 0;
  const expenseNoteText = expenseCount === 0
    ? 'The source report contains no allocated shipment expense rows; the expense sheet is included with all original column headers and is ready for data entry.'
    : `Includes ${expenseCount} allocated shipment expense row(s) apportioned proportionally by part assessable value.`;
  cNotes.value = `All source fields are retained in the dedicated detail sheets. ${expenseNoteText} Values are carried over as reported and have not been recalculated.`;
  cNotes.font = { name: 'Aptos', size: 10, color: { argb: COLORS.TEXT_DARK } };
  cNotes.alignment = { vertical: 'top', horizontal: 'left', wrapText: true };
  cNotes.border = BORDER_THIN;

  // =========================================================================
  // SHEET 2: Shipments
  // =========================================================================
  const shipmentHeaders = [
    'Shipment ID', 'Invoice Number', 'Invoice Date', 'Supplier', 'Incoterm', 'Mode',
    'BL/AWB Number', 'BL Date', 'Vessel Name', 'Container No', 'Status',
    'Quantity', 'Unit', 'Invoiced Unit Price', 'Currency', 'Line Total', 'Price Diverged',
  ];
  const shipmentWidths = [14, 16, 14, 22, 14, 14, 15, 14, 14, 14, 14, 14, 14, 21, 14, 14, 16];
  const wsShipments = wb.addWorksheet('Shipments', {
    views: [{ state: 'frozen', xSplit: 0, ySplit: 4, topLeftCell: 'A5', showGridLines: true, zoomScale: 90 }],
  });
  wsShipments.columns = shipmentWidths.map(w => ({ width: w }));

  styleTitleBanner(
    wsShipments,
    'RELATED SHIPMENTS & INVOICES',
    'Shipment and supplier invoice register | All source columns retained',
    shipmentHeaders.length,
  );
  styleTableHeaders(wsShipments, 4, shipmentHeaders);

  const shipmentsAlignments: ('left' | 'center' | 'right')[] = [
    'left', 'left', 'center', 'left', 'center', 'center',
    'left', 'center', 'left', 'left', 'center',
    'right', 'center', 'right', 'center', 'right', 'center',
  ];

  (data.shipments || []).forEach((sh, idx) => {
    const rowNum = 5 + idx;
    const row = wsShipments.getRow(rowNum);
    row.values = [
      sh.shipmentId || '',
      sh.invoiceNumber || '',
      sh.invoiceDate || '',
      sh.supplierName || '',
      sh.incoterm || '',
      sh.shipmentMode || '',
      sh.blAwbNumber || '',
      sh.blAwbDate || '',
      sh.vesselName || null,
      sh.containerNumber || null,
      sh.status || '',
      (sh.quantity || 0).toFixed(2),
      sh.unit || 'PCS',
      (sh.unitPrice || 0).toFixed(4),
      sh.currency || 'USD',
      (sh.lineTotal || 0).toFixed(2),
      String(Boolean(sh.priceDiverged)),
    ];
    styleDataRow(row, shipmentHeaders.length, idx % 2 === 0, shipmentsAlignments);
  });

  // =========================================================================
  // SHEET 3: BOE Details
  // =========================================================================
  const boeHeaders = [
    'Shipment ID', 'Invoice Number', 'BE Number', 'BE Date', 'Location', 'Status',
    'Exchange Rate', 'Quantity', 'Assessable Value (INR)', 'BCD Rate', 'BCD Amount',
    'SWS Rate', 'SWS Amount', 'IGST Rate', 'IGST Amount', 'BOE Interest',
    'Total Duty', 'Duty Paid', 'Challan Number', 'Payment Date',
  ];
  const boeWidths = [14, 16, 14, 14, 14, 14, 15, 14, 24, 14, 14, 14, 14, 14, 14, 14, 14, 14, 16, 14];
  const wsBOE = wb.addWorksheet('BOE Details', {
    views: [{ state: 'frozen', xSplit: 0, ySplit: 4, topLeftCell: 'A5', showGridLines: true, zoomScale: 90 }],
  });
  wsBOE.columns = boeWidths.map(w => ({ width: w }));

  styleTitleBanner(
    wsBOE,
    'BILL OF ENTRY — CUSTOMS VALUATION & DUTY',
    'BOE assessment, duty calculation and payment details | All source columns retained',
    boeHeaders.length,
  );
  styleTableHeaders(wsBOE, 4, boeHeaders);

  const boeAlignments: ('left' | 'center' | 'right')[] = [
    'left', 'left', 'left', 'center', 'center', 'center',
    'right', 'right', 'right', 'right', 'right',
    'right', 'right', 'right', 'right', 'right',
    'right', 'right', 'left', 'center',
  ];

  (data.boeLines || []).forEach((b, idx) => {
    const rowNum = 5 + idx;
    const row = wsBOE.getRow(rowNum);
    row.values = [
      b.shipmentId || '',
      b.invoiceNumber || '',
      b.beNumber || '',
      b.beDate || '',
      b.location || '',
      b.status || '',
      (b.exchangeRate || 0).toFixed(4),
      (b.quantity || 0).toFixed(2),
      (b.assessableValue || 0).toFixed(2),
      `${(b.bcdRate || 0).toFixed(2)}%`,
      (b.bcdAmount || 0).toFixed(2),
      `${(b.swsRate || 0).toFixed(2)}%`,
      (b.swsAmount || 0).toFixed(2),
      `${(b.igstRate || 0).toFixed(2)}%`,
      (b.igstAmount || 0).toFixed(2),
      (b.boeInterest || 0).toFixed(2),
      (b.totalDuty || 0).toFixed(2),
      (b.dutyPaid || 0).toFixed(2),
      b.challanNumber || '',
      b.paymentDate || '',
    ];
    styleDataRow(row, boeHeaders.length, idx % 2 === 0, boeAlignments);
  });

  // =========================================================================
  // SHEET 4: Expenses
  // =========================================================================
  const expenseHeaders = [
    'Shipment ID', 'Shipment Invoice', 'Service Provider', 'Expense Type',
    'Expense Invoice No', 'Expense Invoice Date', 'Shipment Expense Basic', 'Shipment Expense Total',
    'Allocation Ratio', 'Allocated Basic (INR)', 'Allocated GST (INR)', 'Allocated Total (INR)', 'Remarks',
  ];
  const expenseWidths = [14, 18, 18, 14, 20, 22, 24, 24, 18, 23, 21, 23, 14];
  const wsExpenses = wb.addWorksheet('Expenses', {
    views: [{ state: 'frozen', xSplit: 0, ySplit: 4, topLeftCell: 'A5', showGridLines: true, zoomScale: 90 }],
  });
  wsExpenses.columns = expenseWidths.map(w => ({ width: w }));

  const expenseSubtitle = (data.expenses && data.expenses.length > 0)
    ? 'Shipment expense allocation register | All source columns retained'
    : 'Shipment expense allocation register | No data rows were present in the source export';

  styleTitleBanner(
    wsExpenses,
    'ALLOCATED SHIPMENT EXPENSES',
    expenseSubtitle,
    expenseHeaders.length,
  );
  styleTableHeaders(wsExpenses, 4, expenseHeaders);

  const expenseAlignments: ('left' | 'center' | 'right')[] = [
    'left', 'left', 'left', 'left',
    'left', 'center', 'right', 'right',
    'right', 'right', 'right', 'right', 'left',
  ];

  (data.expenses || []).forEach((ex, idx) => {
    const rowNum = 5 + idx;
    const row = wsExpenses.getRow(rowNum);
    row.values = [
      ex.shipmentId || '',
      ex.shipmentInvoiceNumber || '',
      ex.serviceProviderName || '',
      ex.expenseTypeName || '',
      ex.expenseInvoiceNo || '',
      ex.expenseInvoiceDate || '',
      (ex.shipmentExpenseBasicInr || 0).toFixed(2),
      (ex.shipmentExpenseTotalInr || 0).toFixed(2),
      (ex.allocationRatio || 0).toFixed(4),
      (ex.allocatedBasicInr || 0).toFixed(2),
      (ex.allocatedGstInr || 0).toFixed(2),
      (ex.allocatedTotalInr || 0).toFixed(2),
      ex.remarks || '',
    ];
    styleDataRow(row, expenseHeaders.length, idx % 2 === 0, expenseAlignments);
  });

  // =========================================================================
  // SHEET 5: Cost Trends
  // =========================================================================
  const trendHeaders = [
    'Date', 'Shipment ID', 'Invoice Number', 'Quantity', 'Invoiced Unit Price', 'Currency',
    'Assessable / Unit', 'Duty / Unit', 'Expense / Unit', 'Total Landed Cost / Unit',
  ];
  const trendWidths = [14, 14, 16, 14, 21, 14, 19, 14, 16, 26];
  const wsTrends = wb.addWorksheet('Cost Trends', {
    views: [{ state: 'frozen', xSplit: 0, ySplit: 4, topLeftCell: 'A5', showGridLines: true, zoomScale: 90 }],
  });
  wsTrends.columns = trendWidths.map(w => ({ width: w }));

  styleTitleBanner(
    wsTrends,
    'CHRONOLOGICAL COST TRENDS',
    'Chronological landed-cost movement by shipment and invoice | All source columns retained',
    trendHeaders.length,
  );
  styleTableHeaders(wsTrends, 4, trendHeaders);

  const trendAlignments: ('left' | 'center' | 'right')[] = [
    'center', 'left', 'left', 'right', 'right', 'center',
    'right', 'right', 'right', 'right',
  ];

  (data.trends || []).forEach((tr, idx) => {
    const rowNum = 5 + idx;
    const row = wsTrends.getRow(rowNum);
    row.values = [
      tr.date || '',
      tr.shipmentId || '',
      tr.invoiceNumber || '',
      (tr.quantity || 0).toFixed(2),
      (tr.unitPriceForex || 0).toFixed(4),
      tr.currency || 'USD',
      (tr.assessableValuePerUnit || 0).toFixed(2),
      (tr.dutyPerUnit || 0).toFixed(2),
      (tr.expensePerUnit || 0).toFixed(2),
      (tr.landedCostPerUnit || 0).toFixed(2),
    ];
    styleDataRow(row, trendHeaders.length, idx % 2 === 0, trendAlignments);
  });

  // =========================================================================
  // SHEET 6: Report Metadata
  // =========================================================================
  const metaHeaders = ['Section', 'Field Count', 'Source Data Rows', 'Status'];
  const metaWidths = [38, 18, 22, 55];
  const wsMeta = wb.addWorksheet('Report Metadata', {
    views: [{ state: 'frozen', xSplit: 0, ySplit: 4, topLeftCell: 'A5', showGridLines: true, zoomScale: 90 }],
  });
  wsMeta.columns = metaWidths.map(w => ({ width: w }));

  styleTitleBanner(
    wsMeta,
    'REPORT METADATA & FIELD INVENTORY',
    'Source export details and field completeness reference',
    metaHeaders.length,
  );
  styleTableHeaders(wsMeta, 4, metaHeaders);

  const metadataRows: [string, number, number, string][] = [
    ['Item Master Profile', 13, 1, 'All fields retained'],
    ['Executive Summary', 18, 1, 'All metrics retained'],
    ['Related Shipments & Invoices', 17, data.shipments?.length || 0, 'All columns and rows retained'],
    ['BOE Customs Valuation & Duty', 20, data.boeLines?.length || 0, 'All columns and rows retained'],
    [
      'Allocated Shipment Expenses',
      13,
      data.expenses?.length || 0,
      (data.expenses?.length || 0) === 0 ? 'Headers retained; source has no expense rows' : 'All columns and rows retained',
    ],
    ['Chronological Cost Trends', 10, data.trends?.length || 0, 'All columns and rows retained'],
  ];

  metadataRows.forEach((r, idx) => {
    const rowNum = 5 + idx;
    const row = wsMeta.getRow(rowNum);
    row.values = [r[0], r[1], r[2], r[3]];
    row.height = 20;
    for (let c = 1; c <= 4; c++) {
      const cell = row.getCell(c);
      cell.font = { name: 'Calibri', size: 11, color: { argb: COLORS.TEXT_DARK } };
      cell.border = BORDER_THIN;
      if (idx % 2 === 0) {
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.ZEBRA_ROW } };
      }
      cell.alignment = {
        vertical: 'middle',
        horizontal: c === 2 || c === 3 ? 'center' : 'left',
      };
    }
  });

  return wb;
}

export async function exportPartPeriodAnalysisExcel(
  data: PartPeriodAnalysisResult,
): Promise<void> {
  const wb = await buildPartPeriodAnalysisWorkbook(data);
  const buffer = await wb.xlsx.writeBuffer();
  const safePart = (data.targetItem?.partNumber || 'part').replace(/[^a-zA-Z0-9_-]/g, '_');
  const safePeriod = (data.startDate && data.endDate) ? `${data.startDate}_to_${data.endDate}` : 'all_time';
  const filename = `${safePart}_Period_Analysis_${safePeriod}.xlsx`;

  const blob = new Blob([buffer], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
  triggerDownload(blob, filename);
}
