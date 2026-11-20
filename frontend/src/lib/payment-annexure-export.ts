import ExcelJS from 'exceljs';
import { jsPDF } from 'jspdf';
import autoTable from 'jspdf-autotable';
import type { PaymentAnnexure } from '@/services/payment-po-service';

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

export async function exportPaymentAnnexureExcel(annexure: PaymentAnnexure): Promise<void> {
  const wb = new ExcelJS.Workbook();
  wb.creator = 'Import Manager Next';
  wb.created = new Date();

  const ws = wb.addWorksheet('Payment Annexure');

  ws.columns = [
    { header: 'S.No', key: 'slNo', width: 8 },
    { header: 'Doc Type', key: 'docType', width: 16 },
    { header: 'Doc Number', key: 'docNum', width: 20 },
    { header: 'Doc Date', key: 'docDate', width: 14 },
    { header: `Gross (${annexure.currency})`, key: 'gross', width: 18 },
    { header: `Adv Adjusted (${annexure.currency})`, key: 'advAdj', width: 18 },
    { header: `Deductions (${annexure.currency})`, key: 'deduct', width: 16 },
    { header: `Net Payable (${annexure.currency})`, key: 'net', width: 18 },
    { header: 'Terms / Milestone', key: 'terms', width: 28 },
    { header: 'Remarks', key: 'remarks', width: 25 },
  ];

  // Title & Metadata
  ws.spliceRows(1, 0,
    ['PAYMENT ANNEXURE / REMITTANCE ADVICE'],
    [`Annexure Ref: ${annexure.annexureNumber}`, '', '', `Payment Date: ${annexure.paymentDate}`],
    [`Payment Voucher: ${annexure.paymentVoucherNumber}`, '', '', `Payment Mode: ${annexure.paymentMode}`],
    [`Remitting Bank: ${annexure.remitterBank}`, '', '', `Bank Ref / UTR: ${annexure.referenceNumber}`],
    [`Remitting Account: ${annexure.remitterAccount}`, '', '', `Currency: ${annexure.currency}`],
    [''],
    ['BENEFICIARY DETAILS'],
    [`Supplier: ${annexure.supplierName}`, '', '', `Country: ${annexure.supplierCountry}`],
    [`Beneficiary Name: ${annexure.beneficiaryName}`, '', '', `Beneficiary Bank: ${annexure.beneficiaryBank}`],
    [`Account / IBAN: ${annexure.beneficiaryAccount || annexure.beneficiaryIban}`, '', '', `SWIFT / BIC: ${annexure.beneficiarySwift}`],
    ['']
  );

  // Table rows
  for (const item of annexure.items) {
    ws.addRow({
      slNo: item.slNo,
      docType: item.documentType,
      docNum: item.documentNumber,
      docDate: item.documentDate,
      gross: item.grossAmount,
      advAdj: item.advanceAdjusted,
      deduct: item.deductions,
      net: item.netAmount,
      terms: item.paymentTermMilestone,
      remarks: item.remarks,
    });
  }

  // Totals Row
  ws.addRow({
    slNo: '',
    docType: '',
    docNum: 'GRAND TOTAL',
    docDate: '',
    gross: annexure.totalGrossAmount,
    advAdj: annexure.totalAdvanceAdjusted,
    deduct: annexure.totalDeductions,
    net: annexure.totalNetAmount,
    terms: '',
    remarks: '',
  });

  ws.addRow({});
  ws.addRow({ docNum: 'Amount in Words:', gross: annexure.netAmountInWords });
  ws.addRow({});
  ws.addRow({
    slNo: 'Prepared By:',
    docType: annexure.preparedBy || 'Accounts',
    docDate: 'Checked By:',
    gross: annexure.checkedBy || 'Finance Manager',
    advAdj: 'Approved By:',
    deduct: annexure.approvedBy || 'Authorized Signatory',
  });

  const buffer = await wb.xlsx.writeBuffer();
  triggerDownload(
    new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
    `${annexure.annexureNumber}.xlsx`
  );
}

export function exportPaymentAnnexurePdf(annexure: PaymentAnnexure): void {
  const doc = new jsPDF({ orientation: 'landscape', unit: 'mm', format: 'a4' });

  // Header Title
  doc.setFontSize(16);
  doc.setTextColor(30, 41, 59);
  doc.text('PAYMENT ANNEXURE & REMITTANCE ADVICE', 14, 15);

  doc.setFontSize(9);
  doc.setTextColor(100, 116, 139);
  doc.text(`Generated on: ${annexure.generatedDate} | Import Manager ERP`, 14, 21);

  // Remitter & Payment Details Box
  doc.setFillColor(248, 250, 252);
  doc.roundedRect(14, 25, 130, 32, 2, 2, 'F');
  doc.setFontSize(10);
  doc.setTextColor(15, 23, 42);
  doc.text('REMITTANCE METADATA', 18, 30);
  doc.setFontSize(8.5);
  doc.setTextColor(71, 85, 105);
  doc.text(`Annexure No: ${annexure.annexureNumber}`, 18, 36);
  doc.text(`Voucher No: ${annexure.paymentVoucherNumber}`, 18, 41);
  doc.text(`Payment Date: ${annexure.paymentDate}`, 18, 46);
  doc.text(`Payment Mode: ${annexure.paymentMode}`, 18, 51);
  doc.text(`Remitter Bank: ${annexure.remitterBank || 'Corporate Treasury'}`, 80, 36);
  doc.text(`Account No: ${annexure.remitterAccount || 'Primary Account'}`, 80, 41);
  doc.text(`UTR / Reference: ${annexure.referenceNumber || 'N/A'}`, 80, 46);
  doc.text(`Currency: ${annexure.currency}`, 80, 51);

  // Beneficiary Box
  doc.setFillColor(248, 250, 252);
  doc.roundedRect(152, 25, 130, 32, 2, 2, 'F');
  doc.setFontSize(10);
  doc.setTextColor(15, 23, 42);
  doc.text('BENEFICIARY DETAILS', 156, 30);
  doc.setFontSize(8.5);
  doc.setTextColor(71, 85, 105);
  doc.text(`Supplier: ${annexure.supplierName}`, 156, 36);
  doc.text(`Country: ${annexure.supplierCountry}`, 156, 41);
  doc.text(`Beneficiary Name: ${annexure.beneficiaryName || annexure.supplierName}`, 156, 46);
  doc.text(`Bank: ${annexure.beneficiaryBank || 'N/A'}`, 215, 36);
  doc.text(`Account/IBAN: ${annexure.beneficiaryAccount || annexure.beneficiaryIban || 'N/A'}`, 215, 41);
  doc.text(`SWIFT / BIC: ${annexure.beneficiarySwift || 'N/A'}`, 215, 46);

  // Table
  const tableData = annexure.items.map((it) => [
    it.slNo,
    it.documentType,
    it.documentNumber,
    it.documentDate,
    it.grossAmount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
    it.advanceAdjusted.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
    it.deductions.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
    it.netAmount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
    it.paymentTermMilestone,
    it.remarks,
  ]);

  autoTable(doc, {
    startY: 62,
    head: [[
      '#',
      'Doc Type',
      'Doc Number',
      'Date',
      `Gross (${annexure.currency})`,
      `Adv Adj (${annexure.currency})`,
      `Deduct (${annexure.currency})`,
      `Net Remittance (${annexure.currency})`,
      'Terms / Milestone',
      'Remarks',
    ]],
    body: tableData,
    foot: [[
      '',
      '',
      'TOTAL',
      '',
      annexure.totalGrossAmount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
      annexure.totalAdvanceAdjusted.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
      annexure.totalDeductions.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
      annexure.totalNetAmount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
      '',
      '',
    ]],
    theme: 'grid',
    headStyles: { fillColor: [30, 41, 59], textColor: [255, 255, 255], fontSize: 8.5 },
    footStyles: { fillColor: [241, 245, 249], textColor: [15, 23, 42], fontStyle: 'bold', fontSize: 8.5 },
    styles: { fontSize: 8, cellPadding: 2.2 },
    columnStyles: {
      0: { cellWidth: 8 },
      1: { cellWidth: 24 },
      2: { cellWidth: 28 },
      3: { cellWidth: 20 },
      4: { halign: 'right', cellWidth: 24 },
      5: { halign: 'right', cellWidth: 24 },
      6: { halign: 'right', cellWidth: 22 },
      7: { halign: 'right', cellWidth: 28, fontStyle: 'bold' },
      8: { cellWidth: 50 },
      9: { cellWidth: 32 },
    },
  });

  const finalY = (doc as any).lastAutoTable.finalY + 8;

  // Amount in Words
  doc.setFontSize(9);
  doc.setTextColor(30, 41, 59);
  doc.text(`Amount in Words: ${annexure.netAmountInWords}`, 14, finalY);

  // Signatory Block
  const sigY = finalY + 18;
  doc.setDrawColor(203, 213, 225);
  doc.line(14, sigY, 65, sigY);
  doc.line(115, sigY, 175, sigY);
  doc.line(220, sigY, 280, sigY);

  doc.setFontSize(8);
  doc.setTextColor(100, 116, 139);
  doc.text(`Prepared By (${annexure.preparedBy || 'Accounts Executive'})`, 14, sigY + 4);
  doc.text(`Checked By (${annexure.checkedBy || 'Finance Manager'})`, 115, sigY + 4);
  doc.text(`Approved By (${annexure.approvedBy || 'Authorized Signatory'})`, 220, sigY + 4);

  doc.save(`${annexure.annexureNumber}.pdf`);
}
