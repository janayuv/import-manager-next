import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { ItemMasterPage } from '@/pages/ItemMasterPage';
import { SuppliersPage } from '@/pages/SuppliersPage';
import { itemService, optionsService, supplierService } from '@/services/master-service';
import { securityService } from '@/services/security-service';

vi.mock('@/services/security-service', () => ({
  securityService: {
    getAuthSetupStatus: vi.fn().mockResolvedValue({ needsSetup: false, adminUsername: 'admin', recoveryMode: false }),
    getMySession: vi.fn().mockResolvedValue({
      sessionId: 's1',
      userId: 'admin',
      username: 'admin',
      role: 'administrator',
      permissions: ['data.read', 'data.write', 'data.delete'],
      rememberMe: false,
      createdAt: '',
      lastActive: '',
      expiresAt: '',
    }),
    logout: vi.fn(),
  },
}));

vi.mock('@/services/master-service', () => ({
  optionsService: {
    list: vi.fn().mockResolvedValue([{ value: 'Germany', label: 'Germany' }]),
    listAll: vi.fn().mockResolvedValue({
      units: [{ value: 'PCS', label: 'Pieces' }],
      currencies: [{ value: 'USD', label: 'USD' }],
      bcd_rates: [{ value: '7.5', label: '7.5%' }],
      sws_rates: [{ value: '10', label: '10%' }],
      igst_rates: [{ value: '18', label: '18%' }],
      categories: [],
      end_uses: [],
    }),
    upsert: vi.fn(),
    delete: vi.fn(),
  },
  supplierService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    checkDuplicate: vi.fn().mockResolvedValue(null),
    getHistory: vi.fn(),
    exportCsv: vi.fn(),
    downloadTemplateCsv: vi.fn(),
    importCsv: vi.fn(),
  },
  itemService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    checkPartNumber: vi.fn().mockResolvedValue(null),
    exportCsv: vi.fn(),
    downloadTemplateCsv: vi.fn(),
    importCsv: vi.fn(),
  },
}));

describe('Stage 4 Master Data UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
      permissions: ['data.read', 'data.write', 'data.delete'],
      rememberMe: false,
      createdAt: '',
      lastActive: '',
      expiresAt: '',
    });
    vi.mocked(optionsService.list).mockResolvedValue([{ value: 'Germany', label: 'Germany' }]);
    vi.mocked(optionsService.listAll).mockResolvedValue({
      units: [{ value: 'PCS', label: 'Pieces' }],
      currencies: [
        { value: 'USD', label: 'US Dollar (USD)' },
        { value: 'KRW', label: 'Korean Won (KRW)' },
      ],
      countries: [{ value: 'Germany', label: 'Germany' }],
      bcd_rates: [{ value: '7.5', label: '7.5%' }],
      sws_rates: [{ value: '10', label: '10%' }],
      igst_rates: [{ value: '18', label: '18%' }],
      categories: [{ value: 'Electronics', label: 'Electronics' }],
      end_uses: [{ value: 'Industrial', label: 'Industrial' }],
      purchase_uoms: [{ value: 'PCS', label: 'Pieces' }],
    });
  });

  it('renders suppliers table and submits new supplier creation', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({
      items: [
        {
          id: 'Sup-001',
          supplierName: 'Bosch GmbH',
          shortName: 'BOSCH',
          country: 'Germany',
          email: 'de@bosch.example',
          phone: '+49-1234',
          beneficiaryName: '',
          bankName: '',
          branch: '',
          bankAddress: '',
          accountNo: '',
          iban: '',
          swiftCode: '',
          isActive: true,
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 10,
    });
    vi.mocked(supplierService.create).mockResolvedValue({
      id: 'Sup-002',
      supplierName: 'Kuka Robotics',
      shortName: 'KUKA',
      country: 'Germany',
      email: 'info@kuka.example',
      phone: '',
      beneficiaryName: '',
      bankName: '',
      branch: '',
      bankAddress: '',
      accountNo: '',
      iban: '',
      swiftCode: '',
      isActive: true,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <SuppliersPage />
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Bosch GmbH')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Add Supplier/i }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByText('Add New Supplier')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Manage/i }));
    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByText('Manage Countries')).toBeInTheDocument();
      expect(screen.getAllByPlaceholderText('e.g. India').length).toBeGreaterThan(0);
      expect(screen.queryByPlaceholderText('e.g. PCS')).not.toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Supplier Name/i), { target: { value: 'Kuka Robotics' } });
    fireEvent.change(screen.getByLabelText(/Country/i), { target: { value: 'Germany' } });
    fireEvent.click(screen.getByRole('button', { name: /Create Supplier/i }));

    await waitFor(() => {
      expect(supplierService.create).toHaveBeenCalledWith(
        expect.objectContaining({ supplierName: 'Kuka Robotics', country: 'Germany' }),
      );
    });
  });

  it('renders item master table cleanly', async () => {
    vi.mocked(supplierService.list).mockResolvedValue({ items: [], totalCount: 0, page: 1, pageSize: 500 });
    vi.mocked(itemService.list).mockResolvedValue({
      items: [
        {
          id: 'ITM-001',
          partNumber: 'PN-900',
          itemDescription: 'Servo Drive',
          unit: 'PCS',
          currency: 'USD',
          unitPrice: 250,
          hsnCode: '85044090',
          supplierId: 'Sup-001',
          supplierName: 'Bosch GmbH',
          isActive: true,
          countryOfOrigin: 'Germany',
          bcd: '7.5',
          sws: '10',
          igst: '18',
          technicalWriteUp: '',
          category: 'Electronics',
          endUse: 'Industrial',
          netWeightKg: 2.5,
          purchaseUom: 'PCS',
          grossWeightPerUomKg: 2.8,
          photoPath: '',
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 10,
    });

    render(
      <MemoryRouter>
        <AuthProvider>
          <ItemMasterPage />
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('PN-900')).toBeInTheDocument();
      expect(screen.getByText('Servo Drive')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Add Item/i }));
    await waitFor(() => {
      expect(screen.getByText('Create Item Master Entry')).toBeInTheDocument();
    });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /^Manage$/i }).length).toBe(9);
    expect(screen.getByRole('combobox', { name: /Currency/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Cancel/i }));
    await waitFor(() => {
      expect(screen.queryByText('Create Item Master Entry')).not.toBeInTheDocument();
    });
  });
});
