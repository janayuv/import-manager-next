import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { CBICRatesPage } from '@/pages/CBICRatesPage';
import { cbicRatesService, type CBICExchangeRate } from '@/services/cbic-rates-service';

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

vi.mock('@/services/cbic-rates-service', () => ({
  cbicRatesService: {
    list: vi.fn(),
    getEffectiveRate: vi.fn(),
    create: vi.fn(),
    createBatch: vi.fn().mockResolvedValue({ created: 22, errors: [] }),
    delete: vi.fn(),
  },
}));

describe('CBIC Customs Exchange Rate Master (v1.7.0 Priority 3)', () => {
  const mockRates: CBICExchangeRate[] = [
    {
      id: 1,
      currencyCode: 'USD',
      exchangeRate: 84.5,
      effectiveFrom: '2026-11-01',
      effectiveTo: null,
      notificationNumber: 'No. 12/2026-Customs (N.T.)',
      sourceDocument: 'Gazette of India Extraordinary Part II',
      verificationStatus: 'VERIFIED',
      createdAt: '2026-11-01 10:00:00',
      createdBy: 'admin',
    },
    {
      id: 2,
      currencyCode: 'EUR',
      exchangeRate: 91.2,
      effectiveFrom: '2026-11-01',
      effectiveTo: null,
      notificationNumber: 'No. 12/2026-Customs (N.T.)',
      sourceDocument: 'Gazette of India Extraordinary Part II',
      verificationStatus: 'VERIFIED',
      createdAt: '2026-11-01 10:00:00',
      createdBy: 'admin',
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(cbicRatesService.list).mockResolvedValue(mockRates);
    vi.mocked(cbicRatesService.getEffectiveRate).mockResolvedValue(mockRates[0]!);
    vi.mocked(cbicRatesService.create).mockResolvedValue({
      id: 3,
      currencyCode: 'JPY',
      exchangeRate: 0.56,
      effectiveFrom: '2026-11-15',
      effectiveTo: null,
      notificationNumber: 'No. 14/2026-Customs (N.T.)',
      sourceDocument: 'Gazette of India',
      verificationStatus: 'VERIFIED',
      createdAt: '2026-11-15 10:00:00',
      createdBy: 'admin',
    });
  });

  const renderComponent = () =>
    render(
      <MemoryRouter>
        <AuthProvider>
          <CBICRatesPage />
        </AuthProvider>
      </MemoryRouter>,
    );

  it('renders CBIC Customs Exchange Rate Master with statutory notifications and exchange rates', async () => {
    renderComponent();

    expect(await screen.findByText('CBIC Customs Exchange Rate Master')).toBeInTheDocument();
    expect(screen.getAllByText('USD').length).toBeGreaterThan(0);
    expect(screen.getByText('EUR')).toBeInTheDocument();
    expect(screen.getByText('₹84.50')).toBeInTheDocument();
    expect(screen.getByText('₹91.20')).toBeInTheDocument();
    expect(screen.getAllByText('No. 12/2026-Customs (N.T.)').length).toBeGreaterThan(0);
  });

  it('resolves statutory rate using the Statutory Effective Rate Resolver tool', async () => {
    renderComponent();

    expect(await screen.findByText('Statutory Effective Rate Resolver')).toBeInTheDocument();

    const resolveBtn = screen.getByRole('button', { name: /Resolve Statutory Rate/i });
    fireEvent.click(resolveBtn);

    await waitFor(() => {
      expect(cbicRatesService.getEffectiveRate).toHaveBeenCalledWith('USD', expect.any(String));
    });

    expect(await screen.findByText(/1 USD = ₹84.50/i)).toBeInTheDocument();
  });

  it('opens Add CBIC Notification dialog and creates new verified rate', async () => {
    renderComponent();

    const addBtn = await screen.findByRole('button', { name: /Add CBIC Notification/i });
    fireEvent.click(addBtn);

    expect(await screen.findByText('Add CBIC Rate Notification')).toBeInTheDocument();

    const currInput = screen.getByLabelText(/Currency Code \*/i);
    fireEvent.change(currInput, { target: { value: 'JPY' } });

    const rateInput = screen.getByLabelText(/Customs Rate \(INR\) \*/i);
    fireEvent.change(rateInput, { target: { value: '0.56' } });

    const notifInput = screen.getByLabelText(/CBIC Notification Number \*/i);
    fireEvent.change(notifInput, { target: { value: 'No. 14/2026-Customs (N.T.)' } });

    const saveBtn = screen.getByRole('button', { name: /Save Notification/i });
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(cbicRatesService.create).toHaveBeenCalledWith(
        expect.objectContaining({
          currencyCode: 'JPY',
          exchangeRate: 0.56,
          notificationNumber: 'No. 14/2026-Customs (N.T.)',
          verificationStatus: 'Verified',
        }),
      );
    });
  });

  it('opens ingestion dialog, parses CBIC notification text, and triggers batch creation', async () => {
    renderComponent();

    const openIngestBtn = await screen.findByRole('button', { name: /Paste \/ Ingest Text/i });
    fireEvent.click(openIngestBtn);

    expect(await screen.findByText(/CBIC Statutory Exchange Rate Notification Ingestion/i)).toBeInTheDocument();

    const textarea = screen.getByPlaceholderText(/Paste CBIC Exchange Rate Notification text here/i);
    fireEvent.change(textarea, {
      target: {
        value: `
GOVERNMENT OF INDIA
MINISTRY OF FINANCE
DEPARTMENT OF REVENUE
CENTRAL BOARD OF INDIRECT TAXES AND CUSTOMS
Exchange Rate Notification No.: 28/2026
Exchange Rate Notification for foreign currency rates for Imports & Exports w.e.f 02-10-2026
Currency Code Currency Name Currency Unit Rate Import Rate Export
USD US Dollar 1.0 96.95 95.25
JPY Japanese Yen 100.0 61.6 59.75
EUR EURO 1.0 110.5 106.8
`,
      },
    });

    const parseBtn = screen.getByRole('button', { name: /Parse Text/i });
    fireEvent.click(parseBtn);

    expect(await screen.findByDisplayValue('96.95')).toBeInTheDocument();
    expect(screen.getByText(/₹0.6160/)).toBeInTheDocument();

    const ingestBtn = screen.getByRole('button', { name: /Ingest All/i });
    fireEvent.click(ingestBtn);

    await waitFor(() => {
      expect(cbicRatesService.createBatch).toHaveBeenCalledWith(
        expect.arrayContaining([
          expect.objectContaining({ currencyCode: 'USD', exchangeRate: 96.95, effectiveFrom: '2026-10-02' }),
          expect.objectContaining({ currencyCode: 'JPY', exchangeRate: 0.616, effectiveFrom: '2026-10-02' }),
        ]),
      );
    });
  });
});
