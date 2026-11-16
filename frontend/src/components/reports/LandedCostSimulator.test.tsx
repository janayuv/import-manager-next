import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LandedCostSimulatorDialog } from './LandedCostSimulatorDialog';

vi.mock('@/lib/download', () => ({
  downloadTextFile: vi.fn(),
}));

describe('Phase 2: Landed Cost "What-If" Sensitivity Simulator', () => {
  const onOpenChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders simulator modal with baseline and simulated KPI values', () => {
    render(
      <LandedCostSimulatorDialog
        open={true}
        onOpenChange={onOpenChange}
        initialPartNo="PN-TEST-99"
        initialUnitPrice={10}
        initialCurrency="USD"
      />,
    );

    expect(screen.getByText(/Landed Cost "What-If" Sensitivity Simulator/i)).toBeInTheDocument();
    expect(screen.getByText('Baseline Unit Landed Cost')).toBeInTheDocument();
    expect(screen.getByText('Simulated Unit Cost')).toBeInTheDocument();
    expect(screen.getByText('Sensitivity Adjusters (Simulate Market Fluctuations)')).toBeInTheDocument();
  });

  it('adjusts simulation parameters when sliders change', () => {
    render(
      <LandedCostSimulatorDialog
        open={true}
        onOpenChange={onOpenChange}
        initialPartNo="PN-TEST-99"
        initialUnitPrice={10}
      />,
    );

    const fxSlider = screen.getByLabelText('Exchange Rate Delta');
    fireEvent.change(fxSlider, { target: { value: '5' } });

    expect(screen.getByText('+₹5.00')).toBeInTheDocument();
  });

  it('triggers CSV export of sensitivity summary', async () => {
    const { downloadTextFile } = await import('@/lib/download');

    render(
      <LandedCostSimulatorDialog
        open={true}
        onOpenChange={onOpenChange}
        initialPartNo="PN-TEST-99"
      />,
    );

    const exportBtn = screen.getByRole('button', { name: /Export CSV/i });
    fireEvent.click(exportBtn);

    expect(downloadTextFile).toHaveBeenCalledWith(
      expect.stringContaining('landed_cost_simulation_PN-TEST-99.csv'),
      expect.stringContaining('Landed Cost "What-If" Sensitivity Simulation Summary'),
    );
  });
});
