import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ContainerMovementDialog } from './ContainerMovementDialog';
import { shipmentService, type Shipment, type ShipmentContainer } from '@/services/shipment-invoice-service';

vi.mock('@/services/shipment-invoice-service', () => ({
  shipmentService: {
    listContainers: vi.fn(),
    createContainer: vi.fn(),
    deleteContainer: vi.fn(),
    processContainerMovement: vi.fn(),
    update: vi.fn(),
    freeze: vi.fn(),
    unfreeze: vi.fn(),
  },
}));

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

describe('ContainerMovementDialog (Staged Split Delivery & Warehouse Tracking)', () => {
  const mockShipment: Shipment = {
    id: 'SHP-001',
    supplierId: 'SUP-001',
    supplierName: 'Global Shipping Ltd',
    invoiceNumber: 'INV-3X40',
    invoiceDate: '2026-11-01',
    goodsCategory: 'Machinery',
    invoiceValue: 90000,
    invoiceCurrency: 'USD',
    incoterm: 'CIF',
    shipmentMode: 'Sea',
    shipmentType: 'FCL',
    blAwbNumber: 'BL-999',
    blAwbDate: '2026-11-01',
    vesselName: 'Ever Given',
    containerNumber: 'CONT-1, CONT-2, CONT-3',
    grossWeightKg: 75000,
    etd: '2026-10-15',
    eta: '2026-11-01',
    status: 'customs-clearance',
    dateOfDelivery: '',
    isFrozen: false,
    createdAt: '2026-11-01T10:00:00Z',
  };

  const mockContainers: ShipmentContainer[] = [
    {
      id: 'C-1',
      shipmentId: 'SHP-001',
      containerNumber: 'MSCU-40-001',
      containerSizeType: '40FT',
      sealNumber: 'SEAL-01',
      routeType: 'DIRECT',
      warehouseName: '',
      gateInDate: '',
      gateOutDate: '',
      freeDaysAllowed: 3,
      storageRatePerDay: 500,
      deliveryStatus: 'PENDING',
      dateOfDelivery: '',
      transporterName: '',
      vehicleNumber: '',
      lrNumber: '',
      emptyReturnDate: '',
      remarks: '',
      dwellDays: 0,
      billableDays: 0,
      storageCost: 0,
      portDemurrageFreeDays: 3,
      portDemurrageRatePerDay: 500,
      portDemurrageAmount: 0,
      cfsGroundRentFreeDays: 3,
      cfsGroundRentRatePerDay: 500,
      cfsGroundRentAmount: 0,
      shippingLineDetentionFreeDays: 14,
      shippingLineDetentionRatePerDay: 1000,
      shippingLineDetentionAmount: 0,
      warehouseStorageDays: 0,
      warehouseStorageRatePerDay: 500,
      warehouseStorageAmount: 0,
      createdAt: '2026-11-01 10:00:00',
      updatedAt: '2026-11-01 10:00:00',
    },
    {
      id: 'C-2',
      shipmentId: 'SHP-001',
      containerNumber: 'MSCU-40-002',
      containerSizeType: '40FT',
      sealNumber: 'SEAL-02',
      routeType: 'WAREHOUSE',
      warehouseName: 'Central CFS Warehouse',
      gateInDate: '2026-11-05',
      gateOutDate: '',
      freeDaysAllowed: 3,
      storageRatePerDay: 500,
      deliveryStatus: 'IN_WAREHOUSE',
      dateOfDelivery: '',
      transporterName: 'CFS Shuttle',
      vehicleNumber: 'KA-01-WH-1111',
      lrNumber: '',
      emptyReturnDate: '',
      remarks: '',
      dwellDays: 11,
      billableDays: 8,
      storageCost: 4000,
      portDemurrageFreeDays: 3,
      portDemurrageRatePerDay: 500,
      portDemurrageAmount: 0,
      cfsGroundRentFreeDays: 3,
      cfsGroundRentRatePerDay: 500,
      cfsGroundRentAmount: 4000,
      shippingLineDetentionFreeDays: 14,
      shippingLineDetentionRatePerDay: 1000,
      shippingLineDetentionAmount: 0,
      warehouseStorageDays: 11,
      warehouseStorageRatePerDay: 500,
      warehouseStorageAmount: 4000,
      createdAt: '2026-11-01 10:00:00',
      updatedAt: '2026-11-05 10:00:00',
    },
    {
      id: 'C-3',
      shipmentId: 'SHP-001',
      containerNumber: 'MSCU-40-003',
      containerSizeType: '40FT',
      sealNumber: 'SEAL-03',
      routeType: 'WAREHOUSE',
      warehouseName: 'Central CFS Warehouse',
      gateInDate: '2026-11-05',
      gateOutDate: '2026-11-15',
      freeDaysAllowed: 3,
      storageRatePerDay: 500,
      deliveryStatus: 'DELIVERED',
      dateOfDelivery: '2026-11-15',
      transporterName: 'Express Logistics',
      vehicleNumber: 'KA-01-TR-9999',
      lrNumber: 'LR-12345',
      emptyReturnDate: '2026-11-16',
      remarks: 'Delivered from warehouse',
      dwellDays: 10,
      billableDays: 7,
      storageCost: 3500,
      portDemurrageFreeDays: 3,
      portDemurrageRatePerDay: 500,
      portDemurrageAmount: 0,
      cfsGroundRentFreeDays: 3,
      cfsGroundRentRatePerDay: 500,
      cfsGroundRentAmount: 3500,
      shippingLineDetentionFreeDays: 14,
      shippingLineDetentionRatePerDay: 1000,
      shippingLineDetentionAmount: 0,
      warehouseStorageDays: 10,
      warehouseStorageRatePerDay: 500,
      warehouseStorageAmount: 3500,
      createdAt: '2026-11-01 10:00:00',
      updatedAt: '2026-11-15 10:00:00',
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(shipmentService.listContainers).mockResolvedValue(mockContainers);
  });

  it('renders containers list with sizes, warehouse names, and dwell days', async () => {
    render(
      <ContainerMovementDialog
        open={true}
        onOpenChange={vi.fn()}
        shipment={mockShipment}
      />
    );

    expect(screen.getByText('Staged Container & Delivery Manager')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('MSCU-40-001')).toBeInTheDocument();
      expect(screen.getByText('MSCU-40-002')).toBeInTheDocument();
      expect(screen.getByText('MSCU-40-003')).toBeInTheDocument();
    });

    // Check dwell days display
    expect(screen.getByText(/11 days/)).toBeInTheDocument();
    expect(screen.getByText(/8 billable/)).toBeInTheDocument();
    expect(screen.getAllByText(/Central CFS Warehouse/).length).toBeGreaterThan(0);
  });

  it('submits Direct Delivery for selected container', async () => {
    vi.mocked(shipmentService.processContainerMovement).mockResolvedValue();

    render(
      <ContainerMovementDialog
        open={true}
        onOpenChange={vi.fn()}
        shipment={mockShipment}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('MSCU-40-001')).toBeInTheDocument();
    });

    // Select container 1
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[0]!); // First container checkbox

    // Click confirm direct delivery button
    const submitBtn = screen.getByRole('button', { name: /Confirm Direct Delivery/i });
    expect(submitBtn).not.toBeDisabled();
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(shipmentService.processContainerMovement).toHaveBeenCalledWith(
        expect.objectContaining({
          actionType: 'DIRECT_DELIVER',
          containerIds: ['C-1'],
        })
      );
    });
  });

  it('switches to Warehouse Dispatch tab and dispatches stored container', async () => {
    vi.mocked(shipmentService.processContainerMovement).mockResolvedValue();

    render(
      <ContainerMovementDialog
        open={true}
        onOpenChange={vi.fn()}
        shipment={mockShipment}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('MSCU-40-002')).toBeInTheDocument();
    });

    // Switch to Warehouse Dispatch tab
    const dispatchTab = screen.getByRole('tab', { name: /Warehouse Dispatch/i });
    fireEvent.pointerDown(dispatchTab);
    fireEvent.click(dispatchTab);

    // Select container 2 (which is in warehouse)
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]!); // Second container

    // Check submit button
    const submitBtn = screen.getByRole('button', { name: /Confirm/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(shipmentService.processContainerMovement).toHaveBeenCalledWith(
        expect.objectContaining({
          actionType: 'DISPATCH_FROM_WAREHOUSE',
          containerIds: ['C-2'],
        })
      );
    });
  });

  it('supports direct delivery for Air shipments without containers', async () => {
    const airShipment: Shipment = {
      ...mockShipment,
      id: 'SHP-033',
      shipmentMode: 'Air',
      blAwbNumber: 'AWB-778899',
      containerNumber: '',
    };

    vi.mocked(shipmentService.listContainers).mockResolvedValue([]);
    vi.mocked(shipmentService.update).mockResolvedValue({
      ...airShipment,
      status: 'delivered',
      dateOfDelivery: '2026-11-20',
    });

    render(
      <ContainerMovementDialog
        open={true}
        onOpenChange={vi.fn()}
        shipment={airShipment}
      />
    );

    // Shows Air shipment banner and title
    expect(screen.getByText('Air Cargo Delivery & Shipment Manager')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText(/Air Shipment — No Ocean Containers Required/i)).toBeInTheDocument();
    });

    // Confirm button is NOT disabled despite 0 containers
    const submitBtn = screen.getByRole('button', { name: /Confirm Direct Delivery/i });
    expect(submitBtn).not.toBeDisabled();

    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(shipmentService.update).toHaveBeenCalledWith(
        'SHP-033',
        expect.objectContaining({
          status: 'delivered',
          dateOfDelivery: expect.any(String),
        })
      );
    });
  });

  it('allows freezing and unfreezing the shipment from the dialog header', async () => {
    vi.mocked(shipmentService.freeze).mockResolvedValue({
      ...mockShipment,
      isFrozen: true,
    });
    vi.mocked(shipmentService.unfreeze).mockResolvedValue({
      ...mockShipment,
      isFrozen: false,
    });

    const { rerender } = render(
      <ContainerMovementDialog
        open={true}
        onOpenChange={vi.fn()}
        shipment={mockShipment}
      />
    );

    // Click Freeze button
    const freezeBtn = screen.getByRole('button', { name: /^Freeze$/i });
    fireEvent.click(freezeBtn);

    await waitFor(() => {
      expect(shipmentService.freeze).toHaveBeenCalledWith('SHP-001');
    });

    // Rerender with frozen shipment
    rerender(
      <ContainerMovementDialog
        open={true}
        onOpenChange={vi.fn()}
        shipment={{ ...mockShipment, isFrozen: true }}
      />
    );

    const unfreezeBtn = screen.getByRole('button', { name: /Unfreeze/i });
    expect(unfreezeBtn).toBeInTheDocument();
    fireEvent.click(unfreezeBtn);

    await waitFor(() => {
      expect(shipmentService.unfreeze).toHaveBeenCalledWith('SHP-001');
    });
  });
});
