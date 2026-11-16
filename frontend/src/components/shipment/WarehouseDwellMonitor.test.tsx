import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { WarehouseDwellMonitor } from './WarehouseDwellMonitor';
import { shipmentService, type WarehouseDwellSummary } from '@/services/shipment-invoice-service';

vi.mock('@/services/shipment-invoice-service', () => ({
  shipmentService: {
    getWarehouseDwellReport: vi.fn(),
  },
}));

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

describe('WarehouseDwellMonitor (Warehouse Dwell Time & Storage Reports)', () => {
  const mockDwellData: WarehouseDwellSummary[] = [
    {
      containerId: 'C-101',
      shipmentId: 'SHP-001',
      supplierName: 'Kobe Steel Ltd',
      invoiceNumber: 'INV-1001',
      blAwbNumber: 'BL-901',
      containerNumber: 'TGHU-40-1111',
      containerSizeType: '40FT',
      warehouseName: 'Central Bonded CFS',
      gateInDate: '2026-11-01',
      gateOutDate: '',
      deliveryStatus: 'IN_WAREHOUSE',
      dateOfDelivery: '',
      dwellDays: 15,
      freeDaysAllowed: 5,
      billableDays: 10,
      storageRatePerDay: 500,
      estimatedCharges: 5000,
    },
    {
      containerId: 'C-102',
      shipmentId: 'SHP-002',
      supplierName: 'Nippon Carbon',
      invoiceNumber: 'INV-2002',
      blAwbNumber: 'BL-902',
      containerNumber: 'MSCU-20-2222',
      containerSizeType: '20FT',
      warehouseName: 'Port CFS Depot',
      gateInDate: '2026-11-12',
      gateOutDate: '',
      deliveryStatus: 'IN_WAREHOUSE',
      dateOfDelivery: '',
      dwellDays: 4,
      freeDaysAllowed: 5,
      billableDays: 0,
      storageRatePerDay: 300,
      estimatedCharges: 0,
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(shipmentService.getWarehouseDwellReport).mockResolvedValue(mockDwellData);
  });

  it('renders KPI metrics and dwell time rows with color badges', async () => {
    render(<WarehouseDwellMonitor />);

    await waitFor(() => {
      expect(screen.getByText('TGHU-40-1111')).toBeInTheDocument();
      expect(screen.getByText('MSCU-20-2222')).toBeInTheDocument();
    });

    // Check KPIs
    expect(screen.getByText('Containers in Warehouse')).toBeInTheDocument();
    expect(screen.getByText('Overstay Risk (> 7 Days)')).toBeInTheDocument();
    expect(screen.getByText('Est. Accrued Storage Charges')).toBeInTheDocument();

    // Check dwell days badge text
    expect(screen.getByText(/15 days \(Overstay\)/)).toBeInTheDocument();
    expect(screen.getByText(/4 days/)).toBeInTheDocument();

    // Check storage cost
    expect(screen.getByText(/10d @ ₹500/)).toBeInTheDocument();
  });

  it('filters table rows based on search input', async () => {
    render(<WarehouseDwellMonitor />);

    await waitFor(() => {
      expect(screen.getByText('TGHU-40-1111')).toBeInTheDocument();
      expect(screen.getByText('MSCU-20-2222')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/Search container, shipment, warehouse/i);
    fireEvent.change(searchInput, { target: { value: 'Nippon' } });

    expect(screen.queryByText('TGHU-40-1111')).not.toBeInTheDocument();
    expect(screen.getByText('MSCU-20-2222')).toBeInTheDocument();
  });
});
