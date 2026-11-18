import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  buildTransitShipmentWorkbook,
  generateTransitShipmentTemplateCSV,
  generateTransitShipmentTabularCSV,
  exportTransitShipmentExcel,
  exportTransitShipmentTemplateCSV,
  exportTransitShipmentTabularCSV,
  formatTransitDateDisplay,
} from '@/lib/transit-shipment-export';
import type { TransitShipmentReport } from '@/services/shipment-invoice-service';

describe('transit-shipment-export suite', () => {
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

    HTMLAnchorElement.prototype.click = vi.fn();

    const originalAppendChild = document.body.appendChild.bind(document.body);
    vi.spyOn(document.body, 'appendChild').mockImplementation((node) => {
      if (node instanceof HTMLAnchorElement) {
        clickedFilename = node.download;
      }
      return originalAppendChild(node);
    });
  });

  const sampleReport: TransitShipmentReport = {
    summary: {
      totalShipments: 4,
      inTransit: 1,
      arrived: 1,
      delayed: 1,
      pending: 1,
      averagePlannedTransit: 20.0,
    },
    breakdown: [
      { status: 'In Transit', count: 1, pct: 25.0 },
      { status: 'Arrived', count: 1, pct: 25.0 },
      { status: 'Delayed', count: 1, pct: 25.0 },
      { status: 'Pending', count: 1, pct: 25.0 },
    ],
    items: [
      {
        sNo: 1,
        id: 'SHP-001',
        supplierId: 'SUP-01',
        supplierName: 'Apex Logistics Ltd',
        invoiceNumber: 'INV-101',
        invoiceDate: '2026-11-01',
        vesselVoyage: 'EVER GIVEN V.102',
        etd: '2026-11-05',
        eta: '2026-11-25',
        transitDays: 20,
        status: 'In Transit',
        remarks: 'Container: MSCU1234567',
      },
      {
        sNo: 2,
        id: 'SHP-002',
        supplierId: 'SUP-02',
        supplierName: 'Nippon Steel Corp',
        invoiceNumber: 'INV-102',
        invoiceDate: '2026-10-10',
        vesselVoyage: 'MAERSK HONAM',
        etd: '2026-10-12',
        eta: '2026-10-27',
        transitDays: 15,
        status: 'Delayed',
        remarks: 'Delayed at transshipment port',
      },
      {
        sNo: 3,
        id: 'SHP-003',
        supplierId: 'SUP-03',
        supplierName: 'Korea Wecosta Co Ltd',
        invoiceNumber: 'INV-103',
        invoiceDate: '2026-10-05',
        vesselVoyage: 'CMA CGM MARCO POLO',
        etd: '2026-10-06',
        eta: '2026-10-31',
        transitDays: 25,
        status: 'Arrived',
        remarks: 'Delivered to warehouse',
        dateOfDelivery: '2026-11-01',
      },
      {
        sNo: 4,
        id: 'SHP-004',
        supplierId: 'SUP-01',
        supplierName: 'Apex Logistics Ltd',
        invoiceNumber: 'INV-104',
        invoiceDate: '2026-11-15',
        vesselVoyage: '',
        etd: '2026-11-16',
        eta: '',
        transitDays: undefined as unknown as number,
        status: 'Pending',
        remarks: 'Awaiting vessel allocation',
      },
    ],
  };

  it('formats dates into dd-mmm-yyyy format properly', () => {
    expect(formatTransitDateDisplay('2026-11-25')).toBe('25-Nov-2026');
    expect(formatTransitDateDisplay('')).toBe('');
  });

  it('builds an Excel workbook matching reference template specifications', async () => {
    const wb = await buildTransitShipmentWorkbook(sampleReport);

    const sheetNames = wb.worksheets.map((w) => w.name);
    expect(sheetNames).toEqual(['Shipment Tracker', 'Dashboard']);

    // 1. Verify "Shipment Tracker"
    const wsTracker = wb.getWorksheet('Shipment Tracker')!;
    expect(wsTracker.getCell('A1').value).toBe('IMPORT SHIPMENT TRANSIT DETAILS');
    expect(wsTracker.getCell('A1').fill).toMatchObject({
      type: 'pattern',
      pattern: 'solid',
      fgColor: { argb: 'FF17365D' },
    });
    expect(wsTracker.getCell('B3').value).toBe('Shipper / Supplier');
    expect(wsTracker.getCell('H3').value).toBe('Transit Period (Days)');
    expect(wsTracker.getCell('I3').value).toBe('Shipment Status');

    // Row 4: First item
    expect(wsTracker.getCell('A4').value).toBe(1);
    expect(wsTracker.getCell('B4').value).toBe('Apex Logistics Ltd');
    // Check light-blue background on user input cells (Rule 1)
    expect(wsTracker.getCell('B4').fill).toMatchObject({
      type: 'pattern',
      pattern: 'solid',
      fgColor: { argb: 'FFEAF2F8' },
    });
    expect(wsTracker.getCell('C4').value).toBe('INV-101');
    expect(wsTracker.getCell('I4').value).toBe('In Transit');

    // Verify preformatted capacity (200 rows: rows 4 to 203)
    expect(wsTracker.rowCount).toBeGreaterThanOrEqual(203);
    const row203 = wsTracker.getRow(203);
    expect(row203.getCell(1).value).toBe(200);

    // 2. Verify "Dashboard"
    const wsDash = wb.getWorksheet('Dashboard')!;
    expect(wsDash.getCell('A1').value).toBe('IMPORT SHIPMENT OVERVIEW');
    expect(wsDash.getCell('A2').value).toBe('Live summary based on entries in the Shipment Tracker sheet');
    expect(wsDash.getCell('A4').value).toBe('Total Shipments');
    expect(wsDash.getCell('C4').value).toBe('In Transit');
    expect(wsDash.getCell('E4').value).toBe('Arrived');
    expect(wsDash.getCell('A7').value).toBe('Delayed');
    expect(wsDash.getCell('C7').value).toBe('Pending');
    expect(wsDash.getCell('E7').value).toBe('Average Planned Transit (Days)');

    // Status Breakdown title
    expect(wsDash.getCell('A11').value).toBe('Shipment Status Breakdown');

    // How to use this template title
    expect(wsDash.getCell('A19').value).toBe('HOW TO USE THIS TEMPLATE');
    expect(wsDash.getCell('B20').value).toContain('Use the light-blue cells in the Shipment Tracker sheet');
  });

  it('generates the exact multi-section template CSV matching user attachment', () => {
    const csv = generateTransitShipmentTemplateCSV(sampleReport);

    expect(csv.startsWith('\uFEFF')).toBe(true);
    expect(csv).toContain('IMPORT SHIPMENT TRANSIT DETAILS,,,,,,,,,');
    expect(csv).toContain('S.No.,Shipper / Supplier,Invoice No.,Invoice Date,Vessel / Voyage,ETD,ETA,Transit Period (Days),Shipment Status,Remarks');
    expect(csv).toContain('1,Apex Logistics Ltd,INV-101,01-Nov-2026,EVER GIVEN V.102,05-Nov-2026,25-Nov-2026,20,In Transit,Container: MSCU1234567');
    expect(csv).toContain('IMPORT SHIPMENT OVERVIEW,,,,');
    expect(csv).toContain('Total Shipments,,In Transit,,Arrived');
    expect(csv).toContain('4,,1,,1');
    expect(csv).toContain('Delayed,,Pending,,Average Planned Transit (Days)');
    expect(csv).toContain('1,,1,,20.0');
    expect(csv).toContain('Shipment Status Breakdown,,,,');
    expect(csv).toContain('In Transit,1,,,');
    expect(csv).toContain('Arrived,1,,,');
    expect(csv).toContain('Delayed,1,,,');
    expect(csv).toContain('Pending,1,,,');
    expect(csv).toContain('HOW TO USE THIS TEMPLATE,');
    expect(csv).toContain('1. Enter shipment details,Use the light-blue cells in the Shipment Tracker sheet. One row represents one import shipment / invoice.');
    expect(csv).toContain('Note,Transit period is calendar days (not working days).');
  });

  it('generates clean tabular CSV for data ingestion', () => {
    const csv = generateTransitShipmentTabularCSV(sampleReport);
    expect(csv.startsWith('\uFEFF')).toBe(true);
    expect(csv).toContain('S.No.,Shipper / Supplier,Invoice No.,Invoice Date,Vessel / Voyage,ETD,ETA,Transit Period (Days),Shipment Status,Remarks');
    expect(csv).toContain('1,Apex Logistics Ltd,INV-101');
    expect(csv).not.toContain('HOW TO USE THIS TEMPLATE');
  });

  it('triggers browser file downloads with correct filenames', async () => {
    await exportTransitShipmentExcel(sampleReport, 'My_Transit_Report.xlsx');
    expect(createdBlob).not.toBeNull();
    expect(clickedFilename).toBe('My_Transit_Report.xlsx');

    exportTransitShipmentTemplateCSV(sampleReport, 'Template_Export.csv');
    expect(clickedFilename).toBe('Template_Export.csv');

    exportTransitShipmentTabularCSV(sampleReport, 'Tabular_Export.csv');
    expect(clickedFilename).toBe('Tabular_Export.csv');
  });
});
