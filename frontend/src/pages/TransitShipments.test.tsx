import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { TransitShipmentsPage, computeTransitStatusClient } from '@/pages/TransitShipmentsPage';
import {
  shipmentService,
  type TransitShipmentReport,
} from '@/services/shipment-invoice-service';
import * as exportLib from '@/lib/transit-shipment-export';

vi.mock('@/services/security-service', () => ({
  securityService: {
    getAuthSetupStatus: vi.fn().mockResolvedValue({ needsSetup: false }),
    getMySession: vi.fn().mockResolvedValue({
      userId: 'usr-admin',
      username: 'admin',
      role: 'ADMIN',
      permissions: ['data.read', 'data.write', 'shipment.write'],
      expiresAt: '2099-01-01T00:00:00Z',
    }),
    logout: vi.fn(),
  },
}));

vi.mock('@/services/master-service', () => ({
  supplierService: {
    list: vi.fn().mockResolvedValue({
      items: [
        { id: 'SUP-01', supplierName: 'Apex Logistics Ltd', country: 'Singapore', isActive: true },
        { id: 'SUP-02', supplierName: 'Nippon Steel Corp', country: 'Japan', isActive: true },
      ],
      totalCount: 2,
      page: 1,
      pageSize: 500,
    }),
  },
}));

vi.mock('@/services/shipment-invoice-service', () => ({
  shipmentService: {
    list: vi.fn(),
    getById: vi.fn(),
    update: vi.fn(),
    getTransitShipmentReport: vi.fn(),
    exportTransitShipmentsCsv: vi.fn(),
  },
}));

const mockReport: TransitShipmentReport = {
  summary: {
    totalShipments: 4,
    inTransit: 1,
    arrived: 1,
    delayed: 1,
    pending: 1,
    averagePlannedTransit: 19.3,
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
      invoiceNumber: 'INV-2026-001',
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
      invoiceNumber: 'INV-2026-002',
      invoiceDate: '2026-10-10',
      vesselVoyage: 'MAERSK HONAM',
      etd: '2026-10-12',
      eta: '2026-10-27',
      transitDays: 15,
      status: 'Delayed',
      remarks: 'Delay at transshipment hub',
    },
    {
      sNo: 3,
      id: 'SHP-003',
      supplierId: 'SUP-01',
      supplierName: 'Apex Logistics Ltd',
      invoiceNumber: 'INV-2026-003',
      invoiceDate: '2026-10-05',
      vesselVoyage: 'CMA CGM MARCO POLO',
      etd: '2026-10-06',
      eta: '2026-10-29',
      transitDays: 23,
      status: 'Arrived',
      dateOfDelivery: '2026-10-30',
      remarks: 'Delivered to factory',
    },
    {
      sNo: 4,
      id: 'SHP-004',
      supplierId: 'SUP-02',
      supplierName: 'Nippon Steel Corp',
      invoiceNumber: 'INV-2026-004',
      invoiceDate: '2026-11-15',
      vesselVoyage: '',
      etd: '2026-11-18',
      eta: '',
      transitDays: null,
      status: 'Pending',
      remarks: 'Vessel nomination awaited',
    },
  ],
};

describe('computeTransitStatusClient unit logic', () => {
  const refDate = new Date('2026-11-18T12:00:00Z');

  it('determines In Transit for future ETA and no ATA', () => {
    const res = computeTransitStatusClient('2026-11-10', '2026-11-25', '', 'in-transit', refDate);
    expect(res.status).toBe('In Transit');
    expect(res.transitDays).toBe(15);
  });

  it('determines Delayed for past ETA without ATA', () => {
    const res = computeTransitStatusClient('2026-11-01', '2026-11-15', '', 'in-transit', refDate);
    expect(res.status).toBe('Delayed');
    expect(res.transitDays).toBe(14);
  });

  it('determines Arrived when ATA / dateOfDelivery is recorded', () => {
    const res = computeTransitStatusClient('2026-11-01', '2026-11-15', '2026-11-16', 'in-transit', refDate);
    expect(res.status).toBe('Arrived');
    expect(res.transitDays).toBe(14);
  });

  it('determines Pending when ETA is missing', () => {
    const res = computeTransitStatusClient('2026-11-10', '', '', 'docs-rcvd', refDate);
    expect(res.status).toBe('Pending');
    expect(res.transitDays).toBeUndefined();
  });
});

describe('TransitShipmentsPage Component UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(shipmentService.getTransitShipmentReport).mockResolvedValue(mockReport);
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [],
      totalCount: 0,
      page: 1,
      pageSize: 500,
    });
  });

  const renderComponent = () =>
    render(
      <MemoryRouter>
        <AuthProvider>
          <TransitShipmentsPage />
        </AuthProvider>
      </MemoryRouter>,
    );

  it('renders header, title, and all 6 KPI cards', async () => {
    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Transit Shipment Tracker')).toBeInTheDocument();
    });

    // Check KPI Overview Cards
    expect(screen.getByText('Total Shipments')).toBeInTheDocument();
    expect(screen.getAllByText('In Transit').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Arrived').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Delayed').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Pending').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Avg Planned Transit')).toBeInTheDocument();

    // Check KPI summary values
    expect(screen.getAllByText('4').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('19.3 d')).toBeInTheDocument();
  });

  it('renders the 10 template columns in the data table', async () => {
    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('INV-2026-001')).toBeInTheDocument();
    });

    // Check column headers
    expect(screen.getByText('S.No.')).toBeInTheDocument();
    expect(screen.getByText('Shipper / Supplier')).toBeInTheDocument();
    expect(screen.getByText('Invoice No.')).toBeInTheDocument();
    expect(screen.getByText('Invoice Date')).toBeInTheDocument();
    expect(screen.getByText('Vessel / Voyage')).toBeInTheDocument();
    expect(screen.getByText('ETD')).toBeInTheDocument();
    expect(screen.getByText('ETA')).toBeInTheDocument();
    expect(screen.getByText('Transit Period (Days)')).toBeInTheDocument();
    expect(screen.getByText('Shipment Status')).toBeInTheDocument();
    expect(screen.getByText('Remarks')).toBeInTheDocument();

    // Check row data
    expect(screen.getByText('EVER GIVEN V.102')).toBeInTheDocument();
    expect(screen.getByText('20 d')).toBeInTheDocument();
    expect(screen.getByText('Container: MSCU1234567')).toBeInTheDocument();
  });

  it('toggles the reference template instructions guide', async () => {
    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('How To Use This Template')).toBeInTheDocument();
    });

    const guideBtn = screen.getByRole('button', { name: /how to use this template/i });
    fireEvent.click(guideBtn);

    expect(
      screen.getByText(/HOW TO USE THIS TEMPLATE \(Reference Guide\)/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Use the light-blue cells in the Shipment Tracker sheet/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/not working days/i),
    ).toBeInTheDocument();

    // Hide guide
    fireEvent.click(screen.getByRole('button', { name: /hide template rules/i }));
    expect(
      screen.queryByText(/HOW TO USE THIS TEMPLATE \(Reference Guide\)/i),
    ).not.toBeInTheDocument();
  });

  it('triggers Excel and CSV exports when clicked', async () => {
    const excelSpy = vi.spyOn(exportLib, 'exportTransitShipmentExcel').mockResolvedValue();
    const csvTmplSpy = vi.spyOn(exportLib, 'exportTransitShipmentTemplateCSV').mockImplementation(() => {});
    const csvTabSpy = vi.spyOn(exportLib, 'exportTransitShipmentTabularCSV').mockImplementation(() => {});

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('INV-2026-001')).toBeInTheDocument();
    });

    // 1. Export Excel (.xlsx)
    const excelBtn = screen.getByRole('button', { name: /export excel \(\.xlsx\)/i });
    fireEvent.click(excelBtn);
    await waitFor(() => {
      expect(excelSpy).toHaveBeenCalled();
    });

    // 2. Export Template CSV
    const tmplBtn = screen.getByRole('button', { name: /template csv/i });
    fireEvent.click(tmplBtn);
    await waitFor(() => {
      expect(csvTmplSpy).toHaveBeenCalled();
    });

    // 3. Export Tabular CSV
    const tabBtn = screen.getByRole('button', { name: /tabular csv/i });
    fireEvent.click(tabBtn);
    expect(csvTabSpy).toHaveBeenCalled();
  });

  it('opens inspect & quick update dialog and shows live computed status', async () => {
    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('INV-2026-001')).toBeInTheDocument();
    });

    // Find and click the first Inspect button (actions column)
    const inspectButtons = screen.getAllByTitle('Inspect & Update Transit Dates');
    expect(inspectButtons.length).toBeGreaterThan(0);
    fireEvent.click(inspectButtons[0]!);

    await waitFor(() => {
      expect(screen.getByText('Transit Shipment Details')).toBeInTheDocument();
      expect(screen.getByText('Computed Status:')).toBeInTheDocument();
    });

    // Change ATA input and verify live status preview changes to Arrived
    const ataInput = screen.getByLabelText(/ATA \/ Date of Delivery/i);
    fireEvent.change(ataInput, { target: { value: '2026-11-20' } });

    expect(screen.getAllByText('Arrived').length).toBeGreaterThanOrEqual(1);
  });
});
