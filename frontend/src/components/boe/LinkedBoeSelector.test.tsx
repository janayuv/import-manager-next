import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { LinkedBoeSelector } from './LinkedBoeSelector';
import type { BOEDetail } from '@/services/boe-service';

const MOCK_BOES: BOEDetail[] = [
  {
    id: 'boe-1',
    beNumber: '4747294',
    beDate: '2026-10-29',
    location: 'INMAA4',
    totalAssessmentValue: 5000000,
    dutyAmount: 1043252,
    paymentDate: '2026-10-29',
    dutyPaid: 1043252,
    challanNumber: 'CH-1001',
    refId: '',
    transactionId: '',
  },
  {
    id: 'boe-2',
    beNumber: '4725278',
    beDate: '2026-10-28',
    location: 'INMAA4',
    totalAssessmentValue: 100000,
    dutyAmount: 20322,
    paymentDate: '2026-10-28',
    dutyPaid: 20322,
    challanNumber: 'CH-1002',
    refId: '',
    transactionId: '',
  },
  {
    id: 'boe-3',
    beNumber: '4637210',
    beDate: '2026-10-24',
    location: 'INBLR4',
    totalAssessmentValue: 80000,
    dutyAmount: 10655,
    paymentDate: '2026-10-24',
    dutyPaid: 10655,
    challanNumber: 'CH-1003',
    refId: '',
    transactionId: '',
  },
];

describe('LinkedBoeSelector', () => {
  it('renders unlinked placeholder when value is empty', () => {
    const onChange = vi.fn();
    render(<LinkedBoeSelector value="" onChange={onChange} boes={MOCK_BOES} />);

    expect(screen.getByText('Unlinked (Awaiting BOE Data)')).toBeInTheDocument();
  });

  it('renders selected BOE details and detects duty match/mismatch', () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <LinkedBoeSelector
        value="boe-3"
        onChange={onChange}
        boes={MOCK_BOES}
        expectedDuty={10655}
      />
    );

    expect(screen.getByText(/4637210/)).toBeInTheDocument();
    expect(screen.getByText(/10,655/)).toBeInTheDocument();
    expect(screen.getByText('Match')).toBeInTheDocument();

    // With mismatched expected duty
    rerender(
      <LinkedBoeSelector
        value="boe-3"
        onChange={onChange}
        boes={MOCK_BOES}
        expectedDuty={99999}
      />
    );
    expect(screen.getByText('Mismatch')).toBeInTheDocument();

    // With ±1 rupee difference
    rerender(
      <LinkedBoeSelector
        value="boe-3"
        onChange={onChange}
        boes={MOCK_BOES}
        expectedDuty={10656}
      />
    );
    expect(screen.getByText('±₹1 Match')).toBeInTheDocument();
  });

  it('unlinks with one click on the trigger unlink button', () => {
    const onChange = vi.fn();
    render(
      <LinkedBoeSelector
        value="boe-3"
        onChange={onChange}
        boes={MOCK_BOES}
      />
    );

    const unlinkBtn = screen.getByRole('button', { name: /Unlink BOE/i });
    fireEvent.click(unlinkBtn);
    expect(onChange).toHaveBeenCalledWith('');
  });

  it('opens popover and filters by BE number search', () => {
    const onChange = vi.fn();
    render(
      <LinkedBoeSelector
        value=""
        onChange={onChange}
        boes={MOCK_BOES}
      />
    );

    // Open dropdown
    const trigger = screen.getByRole('combobox');
    fireEvent.click(trigger);

    // Filter input
    const searchInput = screen.getByPlaceholderText(/Search BE number/i);
    fireEvent.change(searchInput, { target: { value: '4725' } });

    // 4725278 should match, others should not
    expect(screen.getByText('4725278')).toBeInTheDocument();
    expect(screen.queryByText('4747294')).not.toBeInTheDocument();
    expect(screen.queryByText('4637210')).not.toBeInTheDocument();

    // Select the filtered item
    fireEvent.click(screen.getByText('4725278'));
    expect(onChange).toHaveBeenCalledWith('boe-2');
  });

  it('filters by Exact Duty Match chip', () => {
    const onChange = vi.fn();
    render(
      <LinkedBoeSelector
        value=""
        onChange={onChange}
        boes={MOCK_BOES}
        expectedDuty={10655}
      />
    );

    const trigger = screen.getByRole('combobox');
    fireEvent.click(trigger);

    // Click exact match filter chip
    const matchChip = screen.getByRole('button', { name: /Exact Duty Match/i });
    fireEvent.click(matchChip);

    // Only boe-3 has duty 10655
    expect(screen.getByText('4637210')).toBeInTheDocument();
    expect(screen.queryByText('4747294')).not.toBeInTheDocument();
    expect(screen.queryByText('4725278')).not.toBeInTheDocument();
  });

  it('opens Advanced Search table modal and selects BOE', () => {
    const onChange = vi.fn();
    render(
      <LinkedBoeSelector
        value=""
        onChange={onChange}
        boes={MOCK_BOES}
      />
    );

    const advSearchBtn = screen.getByRole('button', { name: /Advanced Search/i });
    fireEvent.click(advSearchBtn);

    expect(screen.getByText('Find & Link ICEGATE BOE')).toBeInTheDocument();
    expect(screen.getByText('Found 3 eligible BOEs')).toBeInTheDocument();

    const linkButtons = screen.getAllByRole('button', { name: /Link BOE/i });
    expect(linkButtons.length).toBe(3);
    expect(linkButtons[0]).toBeDefined();

    fireEvent.click(linkButtons[0]!);
    expect(onChange).toHaveBeenCalled();
  });
});
