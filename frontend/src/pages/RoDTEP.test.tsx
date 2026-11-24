import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { RoDTEPScripsPage } from './RoDTEPScripsPage';
import { RoDTEPImportModal } from '@/components/boe/RoDTEPImportModal';

vi.mock('@/services/rodtep-service', () => ({
  rodtepService: {
    getPortSummary: vi.fn().mockResolvedValue([
      {
        portCode: 'COMBINED',
        portName: 'Combined (Pan-India RoDTEP Portfolio)',
        totalScripsCount: 0,
        activeScripsCount: 0,
        exhaustedScripsCount: 0,
        expiredScripsCount: 0,
        originalValuePaise: 0,
        confirmedUtilizedPaise: 0,
        confirmedBalancePaise: 0,
        pendingAllocatedPaise: 0,
        netAvailablePaise: 0,
        originalValueInr: 0,
        confirmedUtilizedInr: 0,
        confirmedBalanceInr: 0,
        pendingAllocatedInr: 0,
        netAvailableInr: 0,
      },
    ]),
    listScrips: vi.fn().mockResolvedValue([]),
    getBOEDebitReport: vi.fn().mockResolvedValue([]),
    listReconciliations: vi.fn().mockResolvedValue([]),
    ingestInitialScrips: vi.fn().mockResolvedValue(5),
  },
}));

describe('RoDTEPScripsPage', () => {
  it('renders cleanly and displays the new Import Scrips (Excel / CSV) button', async () => {
    render(
      <MemoryRouter>
        <RoDTEPScripsPage />
      </MemoryRouter>
    );

    expect(screen.getByText(/RoDTEP Scrip Management/i)).toBeInTheDocument();

    // Verify old button is gone
    expect(screen.queryByText(/Import Oct-26 Reference Scrips/i)).not.toBeInTheDocument();

    // Verify new Excel / CSV import button is present
    const importBtn = screen.getByRole('button', { name: /Import Scrips \(Excel \/ CSV\)/i });
    expect(importBtn).toBeInTheDocument();

    // Clicking it opens the import dialog
    fireEvent.click(importBtn);
    expect(screen.getByText('Import RoDTEP Scrips (Excel / CSV)')).toBeInTheDocument();
  });
});

describe('RoDTEPImportModal', () => {
  it('renders import modal with file upload zone and template buttons', () => {
    const onOpenChange = vi.fn();
    const onSuccess = vi.fn();

    render(
      <RoDTEPImportModal
        open={true}
        onOpenChange={onOpenChange}
        existingScripNumbers={new Set(['2621004567'])}
        onSuccess={onSuccess}
      />
    );

    expect(screen.getByText('Import RoDTEP Scrips (Excel / CSV)')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Excel Template/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /CSV Template/i })).toBeInTheDocument();
    expect(screen.getByText(/Click to browse or drag & drop your Excel/i)).toBeInTheDocument();
  });
});
