import type { ExpenseReportResult, LandedCostReport } from '@/services/expense-service';

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

export async function exportExpenseReportExcel(report: ExpenseReportResult): Promise<void> {
  const { default: ExcelJS } = await import('exceljs');
  const wb = new ExcelJS.Workbook();
  wb.creator = 'Import Manager Next';
  wb.created = new Date();

  const summary = wb.addWorksheet('Summary');
  summary.columns = [
    { header: 'Metric', key: 'metric', width: 30 },
    { header: 'Value (INR)', key: 'value', width: 22 },
  ];
  summary.addRows([
    { metric: 'Report View', value: report.viewMode },
    { metric: 'Total Lines', value: report.totalLines },
    { metric: 'Total Base Amount (INR)', value: report.totalBasePaise / 100 },
    { metric: 'Total GST Taxes (INR)', value: report.totalTaxPaise / 100 },
    { metric: 'Total TDS Deducted (INR)', value: report.totalTdsPaise / 100 },
    { metric: 'Total Gross Billed (INR)', value: report.totalGrossPaise / 100 },
    { metric: 'Total Net Payable (INR)', value: report.totalNetPaise / 100 },
  ]);

  if (report.groupedRows.length > 0) {
    const grp = wb.addWorksheet('Grouped Breakdown');
    grp.columns = [
      { header: 'Group', key: 'groupLabel', width: 32 },
      { header: 'Lines', key: 'lineCount', width: 10 },
      { header: 'Basic (INR)', key: 'amount', width: 16 },
      { header: 'CGST (INR)', key: 'cgst', width: 14 },
      { header: 'SGST (INR)', key: 'sgst', width: 14 },
      { header: 'IGST (INR)', key: 'igst', width: 14 },
      { header: 'TDS (INR)', key: 'tds', width: 14 },
      { header: 'Gross Total (INR)', key: 'total', width: 16 },
      { header: 'Net Payable (INR)', key: 'net', width: 16 },
    ];
    for (const r of report.groupedRows) {
      grp.addRow({
        groupLabel: r.groupLabel,
        lineCount: r.lineCount,
        amount: r.amountPaise / 100,
        cgst: r.cgstPaise / 100,
        sgst: r.sgstPaise / 100,
        igst: r.igstPaise / 100,
        tds: r.tdsPaise / 100,
        total: r.totalPaise / 100,
        net: r.netPaise / 100,
      });
    }
  }

  const detail = wb.addWorksheet('Detailed Lines');
  detail.columns = [
    { header: 'Date', key: 'invoiceDate', width: 14 },
    { header: 'Invoice No', key: 'invoiceNo', width: 18 },
    { header: 'Shipment Inv', key: 'shipmentInvoiceNo', width: 18 },
    { header: 'Service Provider', key: 'serviceProviderName', width: 24 },
    { header: 'Expense Type', key: 'expenseTypeName', width: 22 },
    { header: 'Basic (INR)', key: 'amount', width: 14 },
    { header: 'GST (INR)', key: 'gst', width: 14 },
    { header: 'TDS (INR)', key: 'tds', width: 14 },
    { header: 'Total (INR)', key: 'total', width: 14 },
    { header: 'Net Payable (INR)', key: 'net', width: 16 },
    { header: 'Remarks', key: 'remarks', width: 28 },
  ];
  for (const r of report.detailedRows) {
    detail.addRow({
      invoiceDate: r.invoiceDate,
      invoiceNo: r.invoiceNo,
      shipmentInvoiceNo: r.shipmentInvoiceNo,
      serviceProviderName: r.serviceProviderName,
      expenseTypeName: r.expenseTypeName,
      amount: r.amountPaise / 100,
      gst: (r.cgstPaise + r.sgstPaise + r.igstPaise) / 100,
      tds: r.tdsPaise / 100,
      total: r.totalPaise / 100,
      net: r.netPaise / 100,
      remarks: r.remarks,
    });
  }

  const buf = await wb.xlsx.writeBuffer();
  const blob = new Blob([buf], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
  triggerDownload(blob, `Expense_Report_${report.viewMode}.xlsx`);
}

export async function exportLandedCostPdf(report: LandedCostReport, companyHeader?: string): Promise<void> {
  const [{ jsPDF }, { default: autoTable }] = await Promise.all([
    import('jspdf'),
    import('jspdf-autotable'),
  ]);
  const doc = new jsPDF({ orientation: 'landscape', unit: 'pt', format: 'a4' });
  doc.setFontSize(13);
  const title = companyHeader
    ? `${companyHeader} — Landed Cost Summary (Excl. IGST)`
    : 'Import Manager Next Generation — Landed Cost Summary (Excl. IGST)';
  doc.text(title, 32, 34);
  doc.setFontSize(9);
  doc.text(
    `Rows: ${report.summary.totalRows} | Material: INR ${(report.summary.totalMaterialValue || 0).toFixed(2)} | Assessable: INR ${report.summary.totalAssessableValue.toFixed(2)} | ` +
      `Basic Duties: INR ${report.summary.totalBasicDuties.toFixed(2)} | BOE Interest: INR ${(report.summary.totalBoeInterest || 0).toFixed(2)} | Basic Expenses: INR ${report.summary.totalBasicExpenses.toFixed(2)} | ` +
      `Total LDC (Excl. IGST): INR ${report.summary.totalLandedCostExIgst.toFixed(2)}`,
    32,
    52,
  );

  autoTable(doc, {
    startY: 66,
    styles: { fontSize: 7.5 },
    head: [[
      'Supplier',
      'Inv No',
      'Date',
      'Part No',
      'Unit',
      'Qty',
      'Material Val',
      'Assessable',
      'BCD',
      'SWS',
      'IGST',
      'BOE Interest',
      'Expenses (Basic)',
      'LDC/Qty',
    ]],
    body: report.rows.map((r) => [
      r.supplier,
      r.invoiceNo,
      r.invoiceDate,
      r.partNo,
      r.unit,
      r.qty.toFixed(2),
      (r.materialValue > 0 ? r.materialValue : r.assessableValue).toFixed(2),
      r.assessableValue.toFixed(2),
      r.bcdAmount.toFixed(2),
      r.swsAmount.toFixed(2),
      r.igstAmount.toFixed(2),
      (r.boeInterest || 0).toFixed(2),
      r.expensesTotal.toFixed(2),
      r.ldcPerQty.toFixed(4),
    ]),
  });

  doc.save('Landed_Cost_Report.pdf');
}
