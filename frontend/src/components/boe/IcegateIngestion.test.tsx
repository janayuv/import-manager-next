import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { IcegateIngestionDialog } from './IcegateIngestionDialog';

describe('Phase 2: ICEGATE Bill of Entry Auto-Parser', () => {
  const onOpenChange = vi.fn();
  const onApply = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders parser dialog with textarea and load sample button', () => {
    render(
      <IcegateIngestionDialog
        open={true}
        onOpenChange={onOpenChange}
        onApply={onApply}
      />,
    );

    expect(screen.getByText('ICEGATE Bill of Entry Auto-Parser')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Load Sample ICEGATE Dump/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Apply Extracted Fields to Form/i })).toBeDisabled();
  });

  it('loads sample ICEGATE dump and extracts BE number, AV, BCD, SWS, IGST and Challan', () => {
    render(
      <IcegateIngestionDialog
        open={true}
        onOpenChange={onOpenChange}
        onApply={onApply}
      />,
    );

    const loadSampleBtn = screen.getByRole('button', { name: /Load Sample ICEGATE Dump/i });
    fireEvent.click(loadSampleBtn);

    expect(screen.getByText('Successfully Extracted ICEGATE Records')).toBeInTheDocument();
    expect(screen.getByText('7894512')).toBeInTheDocument(); // BE NO
    expect(screen.getByText('INNSA1')).toBeInTheDocument(); // Port
    expect(screen.getByRole('button', { name: /Apply Extracted Fields to Form/i })).not.toBeDisabled();

    // Click Apply
    const applyBtn = screen.getByRole('button', { name: /Apply Extracted Fields to Form/i });
    fireEvent.click(applyBtn);

    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({
        beNumber: '7894512',
        portCode: 'INNSA1',
        assessableValue: 3000150,
        bcdAmount: 225011,
        swsAmount: 22501,
        igstAmount: 584579,
        customsDutyTotal: 832091,
      }),
    );
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
