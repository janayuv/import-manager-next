import ExcelJS from 'exceljs';
import { jsPDF } from 'jspdf';
import autoTable from 'jspdf-autotable';
import type { BOECalcParams, BOECalculationResult } from '@/services/boe-service';

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

export async function exportBOEExcel(
  params: BOECalcParams,
  result: BOECalculationResult,
): Promise<void> {
  const wb = new ExcelJS.Workbook();
  wb.creator = 'Import Manager Next';
  wb.created = new Date();

  const summary = wb.addWorksheet('Summary');
  summary.columns = [
    { header: 'Metric', key: 'metric', width: 34 },
    { header: 'Value', key: 'value', width: 28 },
  ];
  summary.addRows([
    { metric: 'Supplier Name', value: params.supplierName },
    { metric: 'Invoice Number', value: params.invoiceNumber },
    { metric: 'Shipment ID', value: params.shipmentId },
    { metric: 'Invoice Currency', value: params.invoiceCurrency || 'USD' },
    { metric: 'Total Invoice Value (Forex)', value: result.totalInvoiceValue },
    { metric: 'Total Invoice Value (INR)', value: result.totalValueInr },
    { metric: 'Total Assessable Value (INR)', value: result.totalAssessableValue },
    { metric: 'BCD Total (INR)', value: result.bcdTotal },
    { metric: 'SWS Total (INR)', value: result.swsTotal },
    { metric: 'IGST Total (INR)', value: result.igstTotal },
    { metric: 'Interest (INR)', value: result.interest },
    { metric: 'Customs Duty Grand Total (INR)', value: result.customsDutyTotal },
  ]);

  const itemsSheet = wb.addWorksheet('Item Duties');
  itemsSheet.columns = [
    { header: 'Part No', key: 'partNumber', width: 18 },
    { header: 'Description', key: 'description', width: 32 },
    { header: 'HSN', key: 'hsnCode', width: 14 },
    { header: 'Method', key: 'calculationMethod', width: 12 },
    { header: 'Qty', key: 'quantity', width: 10 },
    { header: `Unit Price (${params.invoiceCurrency || 'USD'})`, key: 'unitPrice', width: 16 },
    { header: 'Forex Value', key: 'itemForexValue', width: 14 },
    { header: 'INR Value', key: 'itemValueInr', width: 14 },
    { header: 'Assessable Value (INR)', key: 'assessableValue', width: 20 },
    { header: 'BOE BCD %', key: 'boeBcdRate', width: 12 },
    { header: 'BOE SWS %', key: 'boeSwsRate', width: 12 },
    { header: 'BOE IGST %', key: 'boeIgstRate', width: 12 },
    { header: 'BCD (INR)', key: 'bcdValue', width: 14 },
    { header: 'SWS (INR)', key: 'swsValue', width: 14 },
    { header: 'IGST (INR)', key: 'igstValue', width: 14 },
    { header: 'Line Duty Total (INR)', key: 'totalLineDuty', width: 18 },
  ];
  itemsSheet.addRows(result.items ?? []);

  const inputSheet = wb.addWorksheet('Input Parameters');
  inputSheet.columns = [
    { header: 'Parameter', key: 'param', width: 30 },
    { header: 'Value', key: 'value', width: 24 },
  ];
  inputSheet.addRows([
    { param: 'Incoterm', value: params.incoterm },
    { param: 'CIF Zero Lock Active', value: result.cifZeroLocked ? 'YES' : 'NO' },
    { param: 'Exchange Rate (INR)', value: params.exchangeRate },
    { param: 'Freight Cost (INR)', value: params.freightCost },
    { param: 'EXW Cost (INR)', value: params.exwCost },
    { param: 'Insurance Rate (%)', value: params.insuranceRate },
    { param: 'Interest (INR)', value: params.interest },
    { param: 'Reconciliation Status', value: params.status },
  ]);

  const buffer = await wb.xlsx.writeBuffer();
  const safeInv = (params.invoiceNumber || params.shipmentId || 'calc').replace(/[^a-zA-Z0-9_-]/g, '_');
  const today = new Date().toISOString().slice(0, 10);
  triggerDownload(
    new Blob([buffer], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    }),
    `BOE-CALC-${safeInv}-${today}.xlsx`,
  );
}

export function exportBOEPdf(params: BOECalcParams, result: BOECalculationResult): void {
  const doc = new jsPDF({ orientation: 'landscape', unit: 'mm', format: 'a4' });
  doc.setFontSize(14);
  doc.text('ICEGATE Customs Valuation & Duty Worksheet (INR)', 14, 14);
  doc.setFontSize(9);
  doc.text(
    `Supplier: ${params.supplierName} | Invoice: ${params.invoiceNumber} (${params.invoiceCurrency}) | Incoterm: ${params.incoterm} | ExRate: ${params.exchangeRate}`,
    14,
    21,
  );

  autoTable(doc, {
    startY: 26,
    head: [[
      'Part No',
      'HSN',
      'Method',
      'Qty',
      'Forex',
      'Assessable (INR)',
      'BCD % / Val',
      'SWS % / Val',
      'IGST % / Val',
      'Line Duty (INR)',
    ]],
    body: (result.items ?? []).map((ln) => [
      ln.partNumber,
      ln.hsnCode || '-',
      ln.calculationMethod,
      ln.quantity.toFixed(2),
      ln.itemForexValue.toFixed(2),
      ln.assessableValue.toFixed(1),
      `${ln.boeBcdRate}% / ${ln.bcdValue.toFixed(1)}`,
      `${ln.boeSwsRate}% / ${ln.swsValue.toFixed(1)}`,
      `${ln.boeIgstRate}% / ${ln.igstValue.toFixed(1)}`,
      ln.totalLineDuty.toFixed(1),
    ]),
    styles: { fontSize: 8 },
  });

  const finalY = (doc as unknown as { lastAutoTable?: { finalY?: number } }).lastAutoTable?.finalY ?? 120;
  doc.setFontSize(10);
  doc.text(
    `Totals (INR): Assessable ${result.totalAssessableValue.toFixed(1)} | BCD ${result.bcdTotal} | SWS ${result.swsTotal} | IGST ${result.igstTotal} | Interest ${result.interest} | Customs Duty Total: ${result.customsDutyTotal}`,
    14,
    finalY + 10,
  );

  const safeInv = (params.invoiceNumber || params.shipmentId || 'calc').replace(/[^a-zA-Z0-9_-]/g, '_');
  const today = new Date().toISOString().slice(0, 10);
  doc.save(`BOE-CALC-${safeInv}-${today}.pdf`);
}
