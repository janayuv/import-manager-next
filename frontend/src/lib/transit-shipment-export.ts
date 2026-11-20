import type { TransitShipmentReport, TransitShipmentItem } from '@/services/shipment-invoice-service';
import { downloadTextFile } from '@/lib/download';

type ExcelJSWorkbook = import('exceljs').Workbook;
type ExcelJSBorders = import('exceljs').Borders;

// Professional styling palette
const COLORS = {
  TITLE_NAVY: 'FF17365D',     // #17365D - Title Banner Fill
  SECTION_STEEL: 'FF1F4E78',   // #1F4E78 - Section Header Fill
  TABLE_TEAL: 'FF0F6B78',      // #0F6B78 - Table Column Headers Fill
  LIGHT_BLUE: 'FFEAF2F8',      // #EAF2F8 - Light Blue editable cell background (Template rule 1)
  ZEBRA_ROW: 'FFF5F8FB',       // #F5F8FB - Alternating row striping
  BORDER_LIGHT: 'FFD9E1F2',    // #D9E1F2 - Cell border line
  TEXT_MUTED: 'FF666666',      // #666666 - Subtitles & notes
  TEXT_DARK: 'FF222222',       // #222222 - Primary data text
  WHITE: 'FFFFFFFF',           // #FFFFFF - White text & fill
  // Status Colors
  STATUS_TRANSIT_BG: 'FFDBEAFE', // #DBEAFE
  STATUS_TRANSIT_FG: 'FF1E40AF', // #1E40AF
  STATUS_ARRIVED_BG: 'FFDCFCE7', // #DCFCE7
  STATUS_ARRIVED_FG: 'FF166534', // #166534
  STATUS_DELAYED_BG: 'FFFFE4E6', // #FFE4E6
  STATUS_DELAYED_FG: 'FF9F1239', // #9F1239
  STATUS_PENDING_BG: 'FFF1F5F9', // #F1F5F9
  STATUS_PENDING_FG: 'FF475569', // #475569
};

const BORDER_THIN: Partial<ExcelJSBorders> = {
  top: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
  bottom: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
  left: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
  right: { style: 'thin', color: { argb: COLORS.BORDER_LIGHT } },
};

import { formatTransitDateDisplay } from './format';
export { formatTransitDateDisplay };

export function parseDateToExcel(dStr?: string): Date | string {
  if (!dStr) return '';
  const d = new Date(dStr);
  if (isNaN(d.getTime())) return dStr;
  return d;
}

/**
 * Builds the professional Transit Shipment Excel Workbook matching the attached reference template.
 * Includes "Shipment Tracker" sheet (preformatted up to capacity) and "Dashboard" summary sheet.
 */
export async function buildTransitShipmentWorkbook(
  report: TransitShipmentReport,
  options?: { capacity?: number },
): Promise<ExcelJSWorkbook> {
  const { default: ExcelJS } = await import('exceljs');
  const wb = new ExcelJS.Workbook();
  wb.creator = 'Import Manager';
  wb.created = new Date();

  const capacity = options?.capacity ?? Math.max(200, report.items.length); // Preformatted capacity (default 200 as per template Note 8)

  // ==========================================
  // SHEET 1: Shipment Tracker
  // ==========================================
  const wsTracker = wb.addWorksheet('Shipment Tracker', {
    views: [{ showGridLines: true }],
  });

  wsTracker.columns = [
    { key: 'sNo', width: 8 },
    { key: 'supplier', width: 30 },
    { key: 'invoiceNo', width: 20 },
    { key: 'invoiceDate', width: 16 },
    { key: 'vesselVoyage', width: 24 },
    { key: 'etd', width: 16 },
    { key: 'eta', width: 16 },
    { key: 'transitDays', width: 22 },
    { key: 'status', width: 18 },
    { key: 'remarks', width: 28 },
  ];

  // Row 1: Title Banner
  wsTracker.mergeCells('A1:J1');
  const r1 = wsTracker.getRow(1);
  r1.height = 36;
  const c1 = r1.getCell(1);
  c1.value = 'IMPORT SHIPMENT TRANSIT DETAILS';
  c1.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.TITLE_NAVY } };
  c1.font = { name: 'Calibri', size: 16, bold: true, color: { argb: COLORS.WHITE } };
  c1.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Row 2: Blank separator
  wsTracker.getRow(2).height = 10;

  // Row 3: Table Column Headers
  const headers = [
    'S.No.',
    'Shipper / Supplier',
    'Invoice No.',
    'Invoice Date',
    'Vessel / Voyage',
    'ETD',
    'ETA',
    'Transit Period (Days)',
    'Shipment Status',
    'Remarks',
  ];
  const r3 = wsTracker.getRow(3);
  r3.height = 32;
  headers.forEach((h, idx) => {
    const cell = r3.getCell(idx + 1);
    cell.value = h;
    cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.TABLE_TEAL } };
    cell.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.WHITE } };
    cell.alignment = { vertical: 'middle', horizontal: 'center', wrapText: true };
    cell.border = BORDER_THIN;
  });

  // Populate data rows + preformatted capacity rows
  for (let i = 0; i < capacity; i++) {
    const rowNum = 4 + i;
    const row = wsTracker.getRow(rowNum);
    row.height = 24;

    const item: TransitShipmentItem | undefined = report.items[i];
    const isEven = i % 2 === 1;
    const baseFill = isEven ? COLORS.ZEBRA_ROW : COLORS.WHITE;

    // S.No.
    const cellA = row.getCell(1);
    cellA.value = i + 1;
    cellA.alignment = { vertical: 'middle', horizontal: 'center' };
    cellA.border = BORDER_THIN;
    cellA.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: baseFill } };

    // Shipper / Supplier (Light Blue cell background as per Rule 1)
    const cellB = row.getCell(2);
    cellB.value = item ? item.supplierName : '';
    cellB.alignment = { vertical: 'middle', horizontal: 'left' };
    cellB.border = BORDER_THIN;
    cellB.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.LIGHT_BLUE } };

    // Invoice No. (Light Blue)
    const cellC = row.getCell(3);
    cellC.value = item ? item.invoiceNumber : '';
    cellC.alignment = { vertical: 'middle', horizontal: 'center' };
    cellC.border = BORDER_THIN;
    cellC.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.LIGHT_BLUE } };

    // Invoice Date (Light Blue)
    const cellD = row.getCell(4);
    if (item && item.invoiceDate) {
      const parsed = parseDateToExcel(item.invoiceDate);
      cellD.value = parsed;
      if (parsed instanceof Date) {
        cellD.numFmt = 'dd-mmm-yyyy';
      }
    } else {
      cellD.value = '';
    }
    cellD.alignment = { vertical: 'middle', horizontal: 'center' };
    cellD.border = BORDER_THIN;
    cellD.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.LIGHT_BLUE } };

    // Vessel / Voyage (Light Blue)
    const cellE = row.getCell(5);
    cellE.value = item ? item.vesselVoyage : '';
    cellE.alignment = { vertical: 'middle', horizontal: 'left' };
    cellE.border = BORDER_THIN;
    cellE.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.LIGHT_BLUE } };

    // ETD (Light Blue)
    const cellF = row.getCell(6);
    if (item && item.etd) {
      const parsed = parseDateToExcel(item.etd);
      cellF.value = parsed;
      if (parsed instanceof Date) {
        cellF.numFmt = 'dd-mmm-yyyy';
      }
    } else {
      cellF.value = '';
    }
    cellF.alignment = { vertical: 'middle', horizontal: 'center' };
    cellF.border = BORDER_THIN;
    cellF.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.LIGHT_BLUE } };

    // ETA (Light Blue)
    const cellG = row.getCell(7);
    if (item && item.eta) {
      const parsed = parseDateToExcel(item.eta);
      cellG.value = parsed;
      if (parsed instanceof Date) {
        cellG.numFmt = 'dd-mmm-yyyy';
      }
    } else {
      cellG.value = '';
    }
    cellG.alignment = { vertical: 'middle', horizontal: 'center' };
    cellG.border = BORDER_THIN;
    cellG.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.LIGHT_BLUE } };

    // Transit Period (Days) - Excel formula G - F (ETA - ETD)
    const cellH = row.getCell(8);
    cellH.value = {
      formula: `IF(AND(ISNUMBER(F${rowNum}),ISNUMBER(G${rowNum})), G${rowNum}-F${rowNum}, IF(AND(F${rowNum}<>"",G${rowNum}<>""), G${rowNum}-F${rowNum}, ""))`,
      result: item?.transitDays ?? undefined,
    };
    cellH.alignment = { vertical: 'middle', horizontal: 'center' };
    cellH.border = BORDER_THIN;
    cellH.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: baseFill } };

    // Shipment Status
    const cellI = row.getCell(9);
    cellI.value = item ? item.status : '';
    cellI.alignment = { vertical: 'middle', horizontal: 'center' };
    cellI.font = { name: 'Calibri', size: 10, bold: true };
    cellI.border = BORDER_THIN;

    if (item) {
      if (item.status === 'In Transit') {
        cellI.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.STATUS_TRANSIT_BG } };
        cellI.font = { name: 'Calibri', size: 10, bold: true, color: { argb: COLORS.STATUS_TRANSIT_FG } };
      } else if (item.status === 'Arrived') {
        cellI.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.STATUS_ARRIVED_BG } };
        cellI.font = { name: 'Calibri', size: 10, bold: true, color: { argb: COLORS.STATUS_ARRIVED_FG } };
      } else if (item.status === 'Delayed') {
        cellI.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.STATUS_DELAYED_BG } };
        cellI.font = { name: 'Calibri', size: 10, bold: true, color: { argb: COLORS.STATUS_DELAYED_FG } };
      } else {
        cellI.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.STATUS_PENDING_BG } };
        cellI.font = { name: 'Calibri', size: 10, bold: true, color: { argb: COLORS.STATUS_PENDING_FG } };
      }
    } else {
      cellI.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: baseFill } };
    }

    // Remarks
    const cellJ = row.getCell(10);
    cellJ.value = item ? item.remarks : '';
    cellJ.alignment = { vertical: 'middle', horizontal: 'left' };
    cellJ.border = BORDER_THIN;
    cellJ.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: baseFill } };
  }

  // ==========================================
  // SHEET 2: Dashboard (Overview & How-To)
  // ==========================================
  const wsDash = wb.addWorksheet('Dashboard', {
    views: [{ showGridLines: true }],
  });

  wsDash.columns = [
    { width: 22 },
    { width: 16 },
    { width: 22 },
    { width: 16 },
    { width: 26 },
    { width: 16 },
  ];

  // Row 1: Dashboard Title Banner
  wsDash.mergeCells('A1:F1');
  const dashR1 = wsDash.getRow(1);
  dashR1.height = 36;
  const dashC1 = dashR1.getCell(1);
  dashC1.value = 'IMPORT SHIPMENT OVERVIEW';
  dashC1.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.TITLE_NAVY } };
  dashC1.font = { name: 'Calibri', size: 16, bold: true, color: { argb: COLORS.WHITE } };
  dashC1.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Row 2: Subtitle
  wsDash.mergeCells('A2:F2');
  const dashR2 = wsDash.getRow(2);
  dashR2.height = 20;
  const dashC2 = dashR2.getCell(1);
  dashC2.value = 'Live summary based on entries in the Shipment Tracker sheet';
  dashC2.font = { name: 'Calibri', size: 10, italic: true, color: { argb: COLORS.TEXT_MUTED } };
  dashC2.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Row 3: Blank
  wsDash.getRow(3).height = 12;

  // Row 4 & 5: KPI Card Set 1 (Total Shipments, In Transit, Arrived)
  wsDash.mergeCells('A4:B4');
  wsDash.mergeCells('C4:D4');
  wsDash.mergeCells('E4:F4');

  const r4 = wsDash.getRow(4);
  r4.height = 24;

  const cTotalH = r4.getCell(1);
  cTotalH.value = 'Total Shipments';
  cTotalH.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.LIGHT_BLUE } };
  cTotalH.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.SECTION_STEEL } };
  cTotalH.alignment = { vertical: 'middle', horizontal: 'center' };
  cTotalH.border = BORDER_THIN;

  const cInTransitH = r4.getCell(3);
  cInTransitH.value = 'In Transit';
  cInTransitH.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.STATUS_TRANSIT_BG } };
  cInTransitH.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.STATUS_TRANSIT_FG } };
  cInTransitH.alignment = { vertical: 'middle', horizontal: 'center' };
  cInTransitH.border = BORDER_THIN;

  const cArrivedH = r4.getCell(5);
  cArrivedH.value = 'Arrived';
  cArrivedH.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.STATUS_ARRIVED_BG } };
  cArrivedH.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.STATUS_ARRIVED_FG } };
  cArrivedH.alignment = { vertical: 'middle', horizontal: 'center' };
  cArrivedH.border = BORDER_THIN;

  wsDash.mergeCells('A5:B5');
  wsDash.mergeCells('C5:D5');
  wsDash.mergeCells('E5:F5');

  const r5 = wsDash.getRow(5);
  r5.height = 36;

  const cTotalVal = r5.getCell(1);
  cTotalVal.value = {
    formula: `COUNTA('Shipment Tracker'!B4:B${capacity + 3})`,
    result: report.summary.totalShipments,
  };
  cTotalVal.font = { name: 'Calibri', size: 18, bold: true, color: { argb: COLORS.TEXT_DARK } };
  cTotalVal.alignment = { vertical: 'middle', horizontal: 'center' };
  cTotalVal.border = BORDER_THIN;

  const cInTransitVal = r5.getCell(3);
  cInTransitVal.value = {
    formula: `COUNTIF('Shipment Tracker'!I4:I${capacity + 3}, "In Transit")`,
    result: report.summary.inTransit,
  };
  cInTransitVal.font = { name: 'Calibri', size: 18, bold: true, color: { argb: COLORS.STATUS_TRANSIT_FG } };
  cInTransitVal.alignment = { vertical: 'middle', horizontal: 'center' };
  cInTransitVal.border = BORDER_THIN;

  const cArrivedVal = r5.getCell(5);
  cArrivedVal.value = {
    formula: `COUNTIF('Shipment Tracker'!I4:I${capacity + 3}, "Arrived")`,
    result: report.summary.arrived,
  };
  cArrivedVal.font = { name: 'Calibri', size: 18, bold: true, color: { argb: COLORS.STATUS_ARRIVED_FG } };
  cArrivedVal.alignment = { vertical: 'middle', horizontal: 'center' };
  cArrivedVal.border = BORDER_THIN;

  // Row 6: Blank
  wsDash.getRow(6).height = 12;

  // Row 7 & 8: KPI Card Set 2 (Delayed, Pending, Average Planned Transit)
  wsDash.mergeCells('A7:B7');
  wsDash.mergeCells('C7:D7');
  wsDash.mergeCells('E7:F7');

  const r7 = wsDash.getRow(7);
  r7.height = 24;

  const cDelayedH = r7.getCell(1);
  cDelayedH.value = 'Delayed';
  cDelayedH.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.STATUS_DELAYED_BG } };
  cDelayedH.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.STATUS_DELAYED_FG } };
  cDelayedH.alignment = { vertical: 'middle', horizontal: 'center' };
  cDelayedH.border = BORDER_THIN;

  const cPendingH = r7.getCell(3);
  cPendingH.value = 'Pending';
  cPendingH.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.STATUS_PENDING_BG } };
  cPendingH.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.STATUS_PENDING_FG } };
  cPendingH.alignment = { vertical: 'middle', horizontal: 'center' };
  cPendingH.border = BORDER_THIN;

  const cAvgTransitH = r7.getCell(5);
  cAvgTransitH.value = 'Average Planned Transit (Days)';
  cAvgTransitH.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.LIGHT_BLUE } };
  cAvgTransitH.font = { name: 'Calibri', size: 11, bold: true, color: { argb: COLORS.SECTION_STEEL } };
  cAvgTransitH.alignment = { vertical: 'middle', horizontal: 'center' };
  cAvgTransitH.border = BORDER_THIN;

  wsDash.mergeCells('A8:B8');
  wsDash.mergeCells('C8:D8');
  wsDash.mergeCells('E8:F8');

  const r8 = wsDash.getRow(8);
  r8.height = 36;

  const cDelayedVal = r8.getCell(1);
  cDelayedVal.value = {
    formula: `COUNTIF('Shipment Tracker'!I4:I${capacity + 3}, "Delayed")`,
    result: report.summary.delayed,
  };
  cDelayedVal.font = { name: 'Calibri', size: 18, bold: true, color: { argb: COLORS.STATUS_DELAYED_FG } };
  cDelayedVal.alignment = { vertical: 'middle', horizontal: 'center' };
  cDelayedVal.border = BORDER_THIN;

  const cPendingVal = r8.getCell(3);
  cPendingVal.value = {
    formula: `COUNTIF('Shipment Tracker'!I4:I${capacity + 3}, "Pending")`,
    result: report.summary.pending,
  };
  cPendingVal.font = { name: 'Calibri', size: 18, bold: true, color: { argb: COLORS.STATUS_PENDING_FG } };
  cPendingVal.alignment = { vertical: 'middle', horizontal: 'center' };
  cPendingVal.border = BORDER_THIN;

  const cAvgTransitVal = r8.getCell(5);
  cAvgTransitVal.value = {
    formula: `IFERROR(ROUND(AVERAGE('Shipment Tracker'!H4:H${capacity + 3}),1), 0)`,
    result: report.summary.averagePlannedTransit,
  };
  cAvgTransitVal.font = { name: 'Calibri', size: 18, bold: true, color: { argb: COLORS.TEXT_DARK } };
  cAvgTransitVal.alignment = { vertical: 'middle', horizontal: 'center' };
  cAvgTransitVal.border = BORDER_THIN;

  // Row 9-10: Blank
  wsDash.getRow(9).height = 12;
  wsDash.getRow(10).height = 12;

  // Row 11: Section Title: Shipment Status Breakdown
  wsDash.mergeCells('A11:C11');
  const r11 = wsDash.getRow(11);
  r11.height = 26;
  const c11 = r11.getCell(1);
  c11.value = 'Shipment Status Breakdown';
  c11.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.SECTION_STEEL } };
  c11.font = { name: 'Calibri', size: 12, bold: true, color: { argb: COLORS.WHITE } };
  c11.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  // Row 12: Breakdown Table Header
  const r12 = wsDash.getRow(12);
  r12.height = 24;
  ['Status', 'Count', '% Share'].forEach((h, idx) => {
    const cell = r12.getCell(idx + 1);
    cell.value = h;
    cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.TABLE_TEAL } };
    cell.font = { name: 'Calibri', size: 10, bold: true, color: { argb: COLORS.WHITE } };
    cell.alignment = { vertical: 'middle', horizontal: 'center' };
    cell.border = BORDER_THIN;
  });

  const breakdownRows = [
    { label: 'In Transit', count: report.summary.inTransit, bg: COLORS.STATUS_TRANSIT_BG, fg: COLORS.STATUS_TRANSIT_FG },
    { label: 'Arrived', count: report.summary.arrived, bg: COLORS.STATUS_ARRIVED_BG, fg: COLORS.STATUS_ARRIVED_FG },
    { label: 'Delayed', count: report.summary.delayed, bg: COLORS.STATUS_DELAYED_BG, fg: COLORS.STATUS_DELAYED_FG },
    { label: 'Pending', count: report.summary.pending, bg: COLORS.STATUS_PENDING_BG, fg: COLORS.STATUS_PENDING_FG },
  ];

  breakdownRows.forEach((bRow, idx) => {
    const rowNum = 13 + idx;
    const row = wsDash.getRow(rowNum);
    row.height = 22;

    const cellStatus = row.getCell(1);
    cellStatus.value = bRow.label;
    cellStatus.font = { name: 'Calibri', size: 10, bold: true, color: { argb: bRow.fg } };
    cellStatus.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: bRow.bg } };
    cellStatus.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };
    cellStatus.border = BORDER_THIN;

    const cellCount = row.getCell(2);
    cellCount.value = {
      formula: `COUNTIF('Shipment Tracker'!I4:I${capacity + 3}, "${bRow.label}")`,
      result: bRow.count,
    };
    cellCount.alignment = { vertical: 'middle', horizontal: 'center' };
    cellCount.border = BORDER_THIN;

    const cellPct = row.getCell(3);
    cellPct.value = {
      formula: `IF(A5>0, B${rowNum}/A5, 0)`,
      result: report.summary.totalShipments > 0 ? bRow.count / report.summary.totalShipments : 0,
    };
    cellPct.numFmt = '0.0%';
    cellPct.alignment = { vertical: 'middle', horizontal: 'center' };
    cellPct.border = BORDER_THIN;
  });

  // Row 17-18: Blank
  wsDash.getRow(17).height = 12;
  wsDash.getRow(18).height = 12;

  // Row 19: HOW TO USE THIS TEMPLATE
  wsDash.mergeCells('A19:F19');
  const r19 = wsDash.getRow(19);
  r19.height = 26;
  const c19 = r19.getCell(1);
  c19.value = 'HOW TO USE THIS TEMPLATE';
  c19.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLORS.SECTION_STEEL } };
  c19.font = { name: 'Calibri', size: 12, bold: true, color: { argb: COLORS.WHITE } };
  c19.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 };

  const guidelines = [
    { num: '1. Enter shipment details', text: 'Use the light-blue cells in the Shipment Tracker sheet. One row represents one import shipment / invoice.' },
    { num: '2. Required fields', text: 'Enter at least the Shipper / Supplier, Invoice No., ETD and ETA to calculate the planned transit period.' },
    { num: '3. Transit Period', text: 'Transit Period (Days) is calculated automatically as ETA minus ETD.' },
    { num: '4. Actual Transit', text: 'Enter ATD (Actual Time of Departure) and ATA (Actual Time of Arrival). Actual Transit (Days) is calculated automatically.' },
    { num: '5. Shipment Status', text: 'Status is automatic: Pending (ETA missing), In Transit (ETA today or later and no ATA), Delayed (ETA passed and no ATA), Arrived (ATA entered).' },
    { num: '6. Dashboard', text: 'The Dashboard updates automatically based on the Shipment Tracker entries.' },
    { num: '7. Date format', text: 'Enter real Excel dates. The workbook displays dates as dd-mmm-yyyy.' },
    { num: '8. Capacity', text: 'The template is preformatted for 200 shipment entries. Extend the formulas/table if you need more rows.' },
    { num: 'Note', text: 'Transit period is calendar days (not working days).' },
  ];

  guidelines.forEach((g, idx) => {
    const rowNum = 20 + idx;
    const row = wsDash.getRow(rowNum);
    row.height = 22;

    const cellA = row.getCell(1);
    cellA.value = g.num;
    cellA.font = { name: 'Calibri', size: 9, bold: true, color: { argb: COLORS.SECTION_STEEL } };
    cellA.alignment = { vertical: 'middle', horizontal: 'left' };

    wsDash.mergeCells(`B${rowNum}:F${rowNum}`);
    const cellB = row.getCell(2);
    cellB.value = g.text;
    cellB.font = { name: 'Calibri', size: 9, italic: g.num === 'Note', color: { argb: COLORS.TEXT_DARK } };
    cellB.alignment = { vertical: 'middle', horizontal: 'left' };
  });

  return wb;
}

/**
 * Triggers Excel (.xlsx) file download using ExcelJS workbook.
 */
export async function exportTransitShipmentExcel(
  report: TransitShipmentReport,
  filename = 'Import_Shipment_Transit_Details.xlsx',
): Promise<void> {
  const wb = await buildTransitShipmentWorkbook(report);
  const buffer = await wb.xlsx.writeBuffer();
  const blob = new Blob([buffer], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

/**
 * Generates exact multi-section CSV string matching the attached template.
 */
export function generateTransitShipmentTemplateCSV(report: TransitShipmentReport): string {
  const lines: string[] = [];

  // 1. Header Section
  lines.push('IMPORT SHIPMENT TRANSIT DETAILS,,,,,,,,,');
  lines.push(',,,,,,,,,');
  lines.push('S.No.,Shipper / Supplier,Invoice No.,Invoice Date,Vessel / Voyage,ETD,ETA,Transit Period (Days),Shipment Status,Remarks');

  // 2. Data Rows
  report.items.forEach((item, idx) => {
    const tDays = item.transitDays !== undefined && item.transitDays !== null ? String(item.transitDays) : '';
    const row = [
      String(idx + 1),
      item.supplierName || '',
      item.invoiceNumber || '',
      formatTransitDateDisplay(item.invoiceDate),
      item.vesselVoyage || '',
      formatTransitDateDisplay(item.etd),
      formatTransitDateDisplay(item.eta),
      tDays,
      item.status || '',
      item.remarks || '',
    ];
    lines.push(
      row
        .map((val) => {
          if (val.includes(',') || val.includes('"') || val.includes('\n')) {
            return `"${val.replace(/"/g, '""')}"`;
          }
          return val;
        })
        .join(','),
    );
  });

  // 3. Overview Section
  lines.push('IMPORT SHIPMENT OVERVIEW,,,,');
  lines.push('Live summary based on entries in the Shipment Tracker sheet,,,,');
  lines.push(',,,,');
  lines.push('Total Shipments,,In Transit,,Arrived');
  lines.push(`${report.summary.totalShipments},,${report.summary.inTransit},,${report.summary.arrived}`);
  lines.push(',,,,');
  lines.push(',,,,');
  lines.push('Delayed,,Pending,,Average Planned Transit (Days)');
  lines.push(`${report.summary.delayed},,${report.summary.pending},,${report.summary.averagePlannedTransit.toFixed(1)}`);
  lines.push(',,,,');
  lines.push(',,,,');
  lines.push(',,,,');

  // 4. Status Breakdown
  lines.push('Shipment Status Breakdown,,,,');
  lines.push('Status,Count,,,');
  lines.push(`In Transit,${report.summary.inTransit},,,`);
  lines.push(`Arrived,${report.summary.arrived},,,`);
  lines.push(`Delayed,${report.summary.delayed},,,`);
  lines.push(`Pending,${report.summary.pending},,,`);

  // 5. How To Use This Template
  lines.push('HOW TO USE THIS TEMPLATE,');
  lines.push(',');
  lines.push('1. Enter shipment details,Use the light-blue cells in the Shipment Tracker sheet. One row represents one import shipment / invoice.');
  lines.push('2. Required fields,"Enter at least the Shipper / Supplier, Invoice No., ETD and ETA to calculate the planned transit period."');
  lines.push('3. Transit Period,Transit Period (Days) is calculated automatically as ETA minus ETD.');
  lines.push('4. Actual Transit,Enter ATD (Actual Time of Departure) and ATA (Actual Time of Arrival). Actual Transit (Days) is calculated automatically.');
  lines.push('5. Shipment Status,"Status is automatic: Pending (ETA missing), In Transit (ETA today or later and no ATA), Delayed (ETA passed and no ATA), Arrived (ATA entered)."');
  lines.push('6. Dashboard,The Dashboard updates automatically based on the Shipment Tracker entries.');
  lines.push('7. Date format,Enter real Excel dates. The workbook displays dates as dd-mmm-yyyy.');
  lines.push('8. Capacity,The template is preformatted for 200 shipment entries. Extend the formulas/table if you need more rows.');
  lines.push('Note,Transit period is calendar days (not working days).');

  return '\uFEFF' + lines.join('\r\n') + '\r\n';
}

/**
 * Triggers download of the multi-section template CSV file.
 */
export function exportTransitShipmentTemplateCSV(
  report: TransitShipmentReport,
  filename = 'Import_Shipment_Transit_Details_Template.csv',
): void {
  const content = generateTransitShipmentTemplateCSV(report);
  downloadTextFile(filename, content);
}

/**
 * Generates pure tabular CSV string suitable for standard table imports.
 */
export function generateTransitShipmentTabularCSV(report: TransitShipmentReport): string {
  const headers = [
    'S.No.',
    'Shipper / Supplier',
    'Invoice No.',
    'Invoice Date',
    'Vessel / Voyage',
    'ETD',
    'ETA',
    'Transit Period (Days)',
    'Shipment Status',
    'Remarks',
  ];
  const lines: string[] = [];
  lines.push(headers.join(','));

  report.items.forEach((item, idx) => {
    const tDays = item.transitDays !== undefined && item.transitDays !== null ? String(item.transitDays) : '';
    const row = [
      String(idx + 1),
      item.supplierName || '',
      item.invoiceNumber || '',
      formatTransitDateDisplay(item.invoiceDate),
      item.vesselVoyage || '',
      formatTransitDateDisplay(item.etd),
      formatTransitDateDisplay(item.eta),
      tDays,
      item.status || '',
      item.remarks || '',
    ];
    lines.push(
      row
        .map((val) => {
          if (val.includes(',') || val.includes('"') || val.includes('\n')) {
            return `"${val.replace(/"/g, '""')}"`;
          }
          return val;
        })
        .join(','),
    );
  });

  return '\uFEFF' + lines.join('\r\n') + '\r\n';
}

/**
 * Triggers download of the pure tabular CSV file.
 */
export function exportTransitShipmentTabularCSV(
  report: TransitShipmentReport,
  filename = 'Transit_Shipments_Table.csv',
): void {
  const content = generateTransitShipmentTabularCSV(report);
  downloadTextFile(filename, content);
}
