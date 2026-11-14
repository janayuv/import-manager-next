import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { InvoiceWizardPage } from '@/pages/InvoiceWizardPage';
import { ShipmentsPage } from '@/pages/ShipmentsPage';
import { itemService, optionsService, supplierService } from '@/services/master-service';
import { securityService } from '@/services/security-service';
import {
  invoiceService,
  shipmentService,
} from '@/services/shipment-invoice-service';

vi.mock('@/services/security-service', () => ({
  securityService: {
    getAuthSetupStatus: vi.fn(),
    getMySession: vi.fn(),
    logout: vi.fn(),
  },
}));

vi.mock('@/services/master-service', () => ({
  optionsService: {
    listAll: vi.fn().mockResolvedValue({ currencies: [], incoterms: [] }),
  },
  supplierService: {
    list: vi.fn().mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 }),
  },
  itemService: {
    list: vi.fn().mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 }),
  },
}));

vi.mock('@/services/shipment-invoice-service', () => ({
  shipmentService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    freeze: vi.fn(),
    unfreeze: vi.fn(),
    delete: vi.fn(),
    getDetail: vi.fn(),
    checkReadyForDelivery: vi.fn(),
    downloadTemplateCsv: vi.fn(),
    importCsv: vi.fn(),
  },
  invoiceService: {
    list: vi.fn(),
    getDetail: vi.fn(),
    delete: vi.fn(),
    listActiveShipments: vi.fn(),
  },
  invoiceWizardService: {
    parseMultiLinePaste: vi.fn(),
    saveInvoice: vi.fn(),
  },
}));

describe('Stage 5 Shipments & Invoice Wizard UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.mocked(securityService.getAuthSetupStatus).mockResolvedValue({
      needsSetup: false,
      adminUsername: 'admin',
      recoveryMode: false,
    });
    vi.mocked(securityService.getMySession).mockResolvedValue({
      sessionId: 's1',
      userId: 'admin',
      username: 'admin',
      role: 'administrator',
      permissions: ['data.read', 'data.write', 'data.delete', 'role.write'],
      rememberMe: false,
      createdAt: '',
      lastActive: '',
      expiresAt: '',
    });
    vi.mocked(optionsService.listAll).mockResolvedValue({});
    vi.mocked(supplierService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 });
  });

  it('renders shipments table and triggers 7-day ready-dly sweep', async () => {
    vi.mocked(shipmentService.list).mockResolvedValue({
      items: [
        {
          id: 'SHP-001',
          supplierId: 'Sup-001',
          supplierName: 'Acme Corp',
          invoiceNumber: 'CI-500',
          invoiceDate: '2026-11-01',
          goodsCategory: 'Electronics',
          invoiceValue: 1000,
          invoiceCurrency: 'USD',
          incoterm: 'CIF',
          shipmentMode: 'Sea',
          shipmentType: 'FCL',
          blAwbNumber: 'BL-1',
          blAwbDate: '2026-11-02',
          vesselName: 'Horizon',
          containerNumber: 'CONT1',
          grossWeightKg: 500,
          etd: '',
          eta: '',
          status: 'customs-clearance',
          dateOfDelivery: '',
          isFrozen: false,
          createdAt: '',
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 10,
    });
    vi.mocked(shipmentService.checkReadyForDelivery).mockResolvedValue(1);

    render(
      <MemoryRouter>
        <AuthProvider>
          <ShipmentsPage />
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('CI-500')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /New Shipment/i }));
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Create Shipment' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    // Verify dropdown lists for Currency, Incoterm, Category, Mode, and Type are rendered as comboboxes
    expect(screen.getByRole('combobox', { name: 'Currency' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Incoterm' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Goods Category' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Shipment Mode' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Shipment Type' })).toBeInTheDocument();

    const manageButtons = screen.getAllByRole('button', { name: /^Manage$/i });
    expect(manageButtons.length).toBe(5);
    fireEvent.click(manageButtons[0]!);
    await waitFor(() => {
      expect(screen.getByText('Manage Currencies')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('e.g. USD')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('e.g. US Dollar')).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Cancel/i }));
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: 'Create Shipment' })).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Check Ready-Dly/i }));
    await waitFor(() => {
      expect(shipmentService.checkReadyForDelivery).toHaveBeenCalled();
    });
  });

  it('walks through 3-step Invoice Wizard and disables Finalize when line sum mismatches', async () => {
    vi.mocked(invoiceService.listActiveShipments).mockResolvedValue([
      {
        id: 'SHP-001',
        supplierId: 'Sup-001',
        supplierName: 'Acme Corp',
        invoiceNumber: 'CI-500',
        invoiceDate: '2026-11-01',
        goodsCategory: 'Electronics',
        invoiceValue: 200,
        invoiceCurrency: 'USD',
        incoterm: 'CIF',
        shipmentMode: 'Sea',
        shipmentType: 'FCL',
        blAwbNumber: '',
        blAwbDate: '',
        vesselName: '',
        containerNumber: '',
        grossWeightKg: 0,
        etd: '',
        eta: '',
        status: 'docs-rcvd',
        dateOfDelivery: '',
        isFrozen: false,
        createdAt: '',
      },
    ]);
    vi.mocked(itemService.list).mockResolvedValue({
      items: [
        {
          id: 'ITM-001',
          partNumber: 'PN-1',
          itemDescription: 'Control Unit',
          unit: 'PCS',
          currency: 'USD',
          unitPrice: 100,
          hsnCode: '85371000',
          supplierId: 'Sup-001',
          isActive: true,
          countryOfOrigin: 'Germany',
          bcd: '7.5',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: '',
          endUse: '',
          netWeightKg: 1,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 1,
          photoPath: '',
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 500,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <InvoiceWizardPage />
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Continue to Line Items/i })).not.toBeDisabled();
    });

    fireEvent.click(screen.getByRole('button', { name: /Continue to Line Items/i }));
    fireEvent.click(screen.getByRole('button', { name: /Add Line/i }));
    fireEvent.click(screen.getByRole('button', { name: /Review & Save/i }));

    await waitFor(() => {
      expect(screen.getByText(/Delta -100\.00 \(Draft Only\)/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Finalize Invoice/i })).toBeDisabled();
      expect(screen.getByRole('button', { name: /Save Draft/i })).not.toBeDisabled();
    });
  });
});
