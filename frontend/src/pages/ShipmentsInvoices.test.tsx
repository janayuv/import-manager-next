import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { InvoicesPage } from '@/pages/InvoicesPage';
import { InvoiceWizardPage } from '@/pages/InvoiceWizardPage';
import { ShipmentsPage } from '@/pages/ShipmentsPage';
import { itemService, optionsService, supplierService } from '@/services/master-service';
import { securityService } from '@/services/security-service';
import {
  invoiceService,
  invoiceWizardService,
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

  it('filters Add Catalog Item in Invoice Wizard to shipment supplier by default and supports toggling', async () => {
    vi.mocked(invoiceService.listActiveShipments).mockResolvedValue([
      {
        id: 'SHP-001',
        supplierId: 'SUP-001',
        supplierName: 'DY ELACEN CO LTD',
        invoiceNumber: 'INV-100',
        invoiceDate: '2026-11-01',
        goodsCategory: 'Gaskets',
        invoiceValue: 500,
        invoiceCurrency: 'USD',
        incoterm: 'CIF',
        shipmentMode: 'Sea',
        shipmentType: 'FCL',
        blAwbNumber: '',
        blAwbDate: '',
        vesselName: '',
        containerNumber: '',
        grossWeightKg: 10,
        etd: '',
        eta: '',
        status: 'in-transit',
        dateOfDelivery: '',
        isFrozen: false,
        createdAt: '',
      },
    ]);
    vi.mocked(itemService.list).mockResolvedValue({
      items: [
        {
          id: 'ITM-DY-1',
          partNumber: 'DY-557144',
          itemDescription: 'AIM Gasket ETC',
          unit: 'PCS',
          currency: 'USD',
          unitPrice: 10,
          hsnCode: '84849000',
          supplierId: 'SUP-001',
          supplierName: 'DY ELACEN CO LTD',
          isActive: true,
          countryOfOrigin: 'Vietnam',
          bcd: '10',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: '',
          endUse: '',
          netWeightKg: 0.1,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 0.1,
          photoPath: '',
        },
        {
          id: 'ITM-WEC-1',
          partNumber: 'WEC-548143',
          itemDescription: 'Acoustic Assy - AIR',
          unit: 'PCS',
          currency: 'KRW',
          unitPrice: 3576,
          hsnCode: '85182900',
          supplierId: 'SUP-002',
          supplierName: 'Wecosta Co Ltd',
          isActive: true,
          countryOfOrigin: 'Korea',
          bcd: '10',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: '',
          endUse: '',
          netWeightKg: 0.2,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 0.2,
          photoPath: '',
        },
      ],
      totalCount: 2,
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

    // In Step 2, "Supplier items only" switch should be rendered and checked by default
    const supplierOnlySwitch = screen.getByRole('switch', { name: /Supplier items only/i });
    expect(supplierOnlySwitch).toBeChecked();

    // Default selected item in the dropdown should be the DY ELACEN item, not Wecosta
    expect(screen.getByText(/DY-557144/i)).toBeInTheDocument();
    expect(screen.queryByText(/WEC-548143/i)).not.toBeInTheDocument();

    // Toggle off "Supplier items only" to show all items
    fireEvent.click(supplierOnlySwitch);
    expect(supplierOnlySwitch).not.toBeChecked();

    // Toggle back on to re-filter
    fireEvent.click(supplierOnlySwitch);
    expect(supplierOnlySwitch).toBeChecked();
    expect(screen.queryByText(/WEC-548143/i)).not.toBeInTheDocument();
  });

  it('allows searching and filtering catalog items by part number or description in Invoice Wizard', async () => {
    vi.mocked(invoiceService.listActiveShipments).mockResolvedValue([
      {
        id: 'SHP-001',
        supplierId: 'SUP-001',
        supplierName: 'INZI Controls Co Limited',
        invoiceNumber: 'ICK-ICI261027NON',
        invoiceDate: '2026-11-01',
        goodsCategory: 'Electronics',
        invoiceValue: 29008.80,
        invoiceCurrency: 'USD',
        incoterm: 'CIF',
        shipmentMode: 'Sea',
        shipmentType: 'FCL',
        blAwbNumber: '',
        blAwbDate: '',
        vesselName: '',
        containerNumber: '',
        grossWeightKg: 100,
        etd: '',
        eta: '',
        status: 'in-transit',
        dateOfDelivery: '',
        isFrozen: false,
        createdAt: '',
      },
    ]);
    vi.mocked(itemService.list).mockResolvedValue({
      items: [
        {
          id: 'ITM-1',
          partNumber: '10101462',
          itemDescription: 'DUAL S/S(3PIN)_39220-38030',
          unit: 'PCS',
          currency: 'USD',
          unitPrice: 1.337,
          hsnCode: '84849000',
          supplierId: 'SUP-001',
          supplierName: 'INZI Controls Co Limited',
          isActive: true,
          countryOfOrigin: 'Korea',
          bcd: '7.5',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: '',
          endUse: '',
          netWeightKg: 0.1,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 0.1,
          photoPath: '',
        },
        {
          id: 'ITM-2',
          partNumber: '40189050',
          itemDescription: 'HOSE ASSY INTAKE',
          unit: 'PCS',
          currency: 'USD',
          unitPrice: 9.0,
          hsnCode: '40093100',
          supplierId: 'SUP-001',
          supplierName: 'INZI Controls Co Limited',
          isActive: true,
          countryOfOrigin: 'Korea',
          bcd: '10',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: '',
          endUse: '',
          netWeightKg: 0.5,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 0.5,
          photoPath: '',
        },
        {
          id: 'ITM-3',
          partNumber: '50490871',
          itemDescription: 'Gasket : TMM Housing(kappa MV)',
          unit: 'PCS',
          currency: 'USD',
          unitPrice: 0.46,
          hsnCode: '84841090',
          supplierId: 'SUP-001',
          supplierName: 'INZI Controls Co Limited',
          isActive: true,
          countryOfOrigin: 'Korea',
          bcd: '10',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: '',
          endUse: '',
          netWeightKg: 0.05,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 0.05,
          photoPath: '',
        },
      ],
      totalCount: 3,
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

    // Open combobox
    const comboboxTrigger = screen.getByRole('combobox', { name: /Add Catalog Item/i });
    expect(comboboxTrigger).toBeInTheDocument();
    fireEvent.click(comboboxTrigger);

    // Search input should appear
    const searchInput = screen.getByPlaceholderText(/Type part number, description, or HSN to search/i);
    expect(searchInput).toBeInTheDocument();

    // Type "HOSE" to search
    fireEvent.change(searchInput, { target: { value: 'HOSE' } });

    // 40189050 should be visible in the dropdown listbox, others filtered out
    const listbox = screen.getByRole('listbox');
    expect(within(listbox).getByText('40189050')).toBeInTheDocument();
    expect(within(listbox).queryByText('10101462')).not.toBeInTheDocument();
    expect(within(listbox).queryByText('50490871')).not.toBeInTheDocument();

    // Select the filtered item
    fireEvent.click(within(listbox).getByText('40189050'));

    // Selected item is now 40189050 in trigger
    expect(screen.getByText(/40189050/)).toBeInTheDocument();

    // Click Add Line and verify it was added to table
    fireEvent.click(screen.getByRole('button', { name: /Add Line/i }));
    expect(screen.getAllByText('40189050').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('HOSE ASSY INTAKE').length).toBeGreaterThanOrEqual(1);
  });

  it('supports direct quick add and clearing search in catalog item combobox', async () => {
    vi.mocked(invoiceService.listActiveShipments).mockResolvedValue([
      {
        id: 'SHP-001',
        supplierId: 'SUP-001',
        supplierName: 'INZI Controls Co Limited',
        invoiceNumber: 'ICK-ICI261027NON',
        invoiceDate: '2026-11-01',
        goodsCategory: 'Electronics',
        invoiceValue: 29008.80,
        invoiceCurrency: 'USD',
        incoterm: 'CIF',
        shipmentMode: 'Sea',
        shipmentType: 'FCL',
        blAwbNumber: '',
        blAwbDate: '',
        vesselName: '',
        containerNumber: '',
        grossWeightKg: 100,
        etd: '',
        eta: '',
        status: 'in-transit',
        dateOfDelivery: '',
        isFrozen: false,
        createdAt: '',
      },
    ]);
    vi.mocked(itemService.list).mockResolvedValue({
      items: [
        {
          id: 'ITM-1',
          partNumber: '10101462',
          itemDescription: 'DUAL S/S(3PIN)_39220-38030',
          unit: 'PCS',
          currency: 'USD',
          unitPrice: 1.337,
          hsnCode: '84849000',
          supplierId: 'SUP-001',
          supplierName: 'INZI Controls Co Limited',
          isActive: true,
          countryOfOrigin: 'Korea',
          bcd: '7.5',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: '',
          endUse: '',
          netWeightKg: 0.1,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 0.1,
          photoPath: '',
        },
        {
          id: 'ITM-2',
          partNumber: '50490871',
          itemDescription: 'Gasket : TMM Housing(kappa MV)',
          unit: 'PCS',
          currency: 'USD',
          unitPrice: 0.46,
          hsnCode: '84841090',
          supplierId: 'SUP-001',
          supplierName: 'INZI Controls Co Limited',
          isActive: true,
          countryOfOrigin: 'Korea',
          bcd: '10',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: '',
          endUse: '',
          netWeightKg: 0.05,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 0.05,
          photoPath: '',
        },
      ],
      totalCount: 2,
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

    // Open combobox
    fireEvent.click(screen.getByRole('combobox', { name: /Add Catalog Item/i }));

    const searchInput = screen.getByPlaceholderText(/Type part number, description, or HSN to search/i);
    fireEvent.change(searchInput, { target: { value: 'Gasket' } });

    // Click direct Add button on Gasket item
    const quickAddBtn = screen.getByRole('button', { name: /Add 50490871 directly to lines/i });
    fireEvent.click(quickAddBtn);

    // Verify Gasket line was added directly to the table
    expect(screen.getAllByText('50490871').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Gasket : TMM Housing(kappa MV)').length).toBeGreaterThanOrEqual(1);
  });

  it('renders InvoicesPage with Edit Draft and Quick Finalize actions', async () => {
    vi.mocked(invoiceService.list).mockResolvedValue([
      {
        id: 'INV-002',
        shipmentId: 'SHP-002',
        supplierId: 'SUP-001',
        supplierName: 'DY ELACEN CO LTD - VIETNAM',
        invoiceNumber: 'DEVP/INZI260827-01',
        invoiceDate: '2026-08-27',
        currency: 'USD',
        shipmentInvoiceValue: 47379.40,
        computedTotal: 47379.40,
        lineCount: 6,
        status: 'Draft',
        lineTotalDecimals: 2,
        invoiceTotalDecimals: 2,
        shipmentFrozen: false,
      },
    ]);
    vi.mocked(invoiceService.getDetail).mockResolvedValue({
      invoice: {
        id: 'INV-002',
        shipmentId: 'SHP-002',
        supplierId: 'SUP-001',
        supplierName: 'DY ELACEN CO LTD - VIETNAM',
        invoiceNumber: 'DEVP/INZI260827-01',
        invoiceDate: '2026-08-27',
        currency: 'USD',
        shipmentInvoiceValue: 47379.40,
        computedTotal: 47379.40,
        lineCount: 1,
        status: 'Draft',
        lineTotalDecimals: 2,
        invoiceTotalDecimals: 2,
        shipmentFrozen: false,
      },
      lines: [
        {
          id: 'IL-1',
          invoiceId: 'INV-002',
          itemId: 'ITM-001',
          partNumber: 'PN-1',
          itemDescription: 'Control Unit',
          unit: 'PCS',
          hsnCode: '85371000',
          quantity: 473.794,
          unitPrice: 100,
          masterUnitPrice: 100,
          lineTotal: 47379.40,
          dutyPercent: 7.5,
          swsPercent: 10,
          igstPercent: 18,
          priceDiverged: false,
        },
      ],
    });
    vi.mocked(invoiceWizardService.saveInvoice).mockResolvedValue({
      invoice: {
        id: 'INV-002',
        shipmentId: 'SHP-002',
        supplierId: 'SUP-001',
        supplierName: 'DY ELACEN CO LTD - VIETNAM',
        invoiceNumber: 'DEVP/INZI260827-01',
        invoiceDate: '2026-08-27',
        currency: 'USD',
        shipmentInvoiceValue: 47379.40,
        computedTotal: 47379.40,
        lineCount: 1,
        status: 'Finalized',
        lineTotalDecimals: 2,
        invoiceTotalDecimals: 2,
        shipmentFrozen: false,
      },
      lines: [],
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <InvoicesPage />
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('INV-002')).toBeInTheDocument();
      expect(screen.getByText('Draft')).toBeInTheDocument();
    });

    // Check Edit Draft button exists with correct link
    const editLink = screen.getByTitle('Edit Draft in Wizard');
    expect(editLink).toBeInTheDocument();
    expect(editLink.closest('a')).toHaveAttribute('href', '/invoice-wizard?id=INV-002');

    // Check Quick Finalize button exists and triggers saveInvoice with finalize: true
    const finalizeBtn = screen.getByTitle('Finalize Invoice (Exact Cent Match)');
    expect(finalizeBtn).toBeInTheDocument();
    fireEvent.click(finalizeBtn);

    await waitFor(() => {
      expect(invoiceWizardService.saveInvoice).toHaveBeenCalledWith(
        expect.objectContaining({
          invoiceId: 'INV-002',
          finalize: true,
          shipmentId: 'SHP-002',
        }),
      );
    });
  });

  it('loads draft invoice into InvoiceWizardPage when ?id= query param is provided', async () => {
    vi.mocked(invoiceService.listActiveShipments).mockResolvedValue([]);
    vi.mocked(invoiceService.list).mockResolvedValue([]);
    vi.mocked(invoiceService.getDetail).mockResolvedValue({
      invoice: {
        id: 'INV-002',
        shipmentId: 'SHP-002',
        supplierId: 'SUP-001',
        supplierName: 'DY ELACEN CO LTD - VIETNAM',
        invoiceNumber: 'DEVP/INZI260827-01',
        invoiceDate: '2026-08-27',
        currency: 'USD',
        shipmentInvoiceValue: 200,
        computedTotal: 200,
        lineCount: 1,
        status: 'Draft',
        lineTotalDecimals: 2,
        invoiceTotalDecimals: 2,
        shipmentFrozen: false,
      },
      lines: [
        {
          id: 'IL-1',
          invoiceId: 'INV-002',
          itemId: 'ITM-001',
          partNumber: 'PN-1',
          itemDescription: 'Control Unit',
          unit: 'PCS',
          hsnCode: '85371000',
          quantity: 2,
          unitPrice: 100,
          masterUnitPrice: 100,
          lineTotal: 200,
          dutyPercent: 7.5,
          swsPercent: 10,
          igstPercent: 18,
          priceDiverged: false,
        },
      ],
    });
    vi.mocked(invoiceWizardService.saveInvoice).mockResolvedValue({
      invoice: {
        id: 'INV-002',
        shipmentId: 'SHP-002',
        supplierId: 'SUP-001',
        supplierName: 'DY ELACEN CO LTD - VIETNAM',
        invoiceNumber: 'DEVP/INZI260827-01',
        invoiceDate: '2026-08-27',
        currency: 'USD',
        shipmentInvoiceValue: 200,
        computedTotal: 200,
        lineCount: 1,
        status: 'Finalized',
        lineTotalDecimals: 2,
        invoiceTotalDecimals: 2,
        shipmentFrozen: false,
      },
      lines: [],
    });

    render(
      <MemoryRouter initialEntries={['/invoice-wizard?id=INV-002']}>
        <AuthProvider>
          <InvoiceWizardPage />
        </AuthProvider>
      </MemoryRouter>,
    );

    // Should load the draft and automatically jump to Step 2 (lines)
    await waitFor(() => {
      expect(screen.getByText(/Editing Draft: INV-002/i)).toBeInTheDocument();
      expect(screen.getByText('Control Unit')).toBeInTheDocument();
    });

    // Navigate to Step 3
    fireEvent.click(screen.getByRole('button', { name: /Review & Save/i }));

    await waitFor(() => {
      expect(screen.getByText(/Exact Cent Match/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Finalize Invoice/i })).not.toBeDisabled();
      expect(screen.getByRole('button', { name: /Update Draft/i })).not.toBeDisabled();
    });

    // Finalize invoice
    fireEvent.click(screen.getByRole('button', { name: /Finalize Invoice/i }));

    await waitFor(() => {
      expect(invoiceWizardService.saveInvoice).toHaveBeenCalledWith(
        expect.objectContaining({
          invoiceId: 'INV-002',
          finalize: true,
          shipmentId: 'SHP-002',
        }),
      );
    });
  });

  it('excludes shipments with finalized invoices from the Commercial Invoice Wizard active shipment list', async () => {
    vi.mocked(invoiceService.listActiveShipments).mockResolvedValue([
      {
        id: 'SHP-FINAL',
        supplierId: 'SUP-001',
        supplierName: 'DY ELACEN CO LTD - VIETNAM',
        invoiceNumber: 'DEVP/INZI260827-01',
        invoiceDate: '2026-08-27',
        goodsCategory: 'General',
        invoiceValue: 47379.4,
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
        status: 'in-transit',
        dateOfDelivery: '',
        isFrozen: false,
        createdAt: '',
      },
      {
        id: 'SHP-PENDING',
        supplierId: 'SUP-002',
        supplierName: 'Pending Supplier',
        invoiceNumber: 'INV-PENDING',
        invoiceDate: '2026-11-01',
        goodsCategory: 'General',
        invoiceValue: 1000,
        invoiceCurrency: 'USD',
        incoterm: 'FOB',
        shipmentMode: 'Air',
        shipmentType: 'LCL',
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

    vi.mocked(invoiceService.list).mockImplementation(async (_search, status) => {
      if (status === 'Draft') return [];
      return [
        {
          id: 'INV-FINAL',
          shipmentId: 'SHP-FINAL',
          supplierId: 'SUP-001',
          supplierName: 'DY ELACEN CO LTD - VIETNAM',
          invoiceNumber: 'DEVP/INZI260827-01',
          invoiceDate: '2026-08-27',
          currency: 'USD',
          shipmentInvoiceValue: 47379.4,
          computedTotal: 47379.4,
          lineCount: 6,
          status: 'Finalized',
          lineTotalDecimals: 2,
          invoiceTotalDecimals: 2,
          shipmentFrozen: false,
        },
      ];
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <InvoiceWizardPage />
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText(/INV-PENDING/i)).toBeInTheDocument();
      expect(screen.queryByText(/DEVP\/INZI260827-01/i)).not.toBeInTheDocument();
    });
  });
});
