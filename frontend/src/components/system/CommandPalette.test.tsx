import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { CommandPaletteDialog } from './CommandPaletteDialog';

describe('Phase 1: Command Palette (Ctrl+K)', () => {
  const onOpenChange = vi.fn();
  const onOpenSimulator = vi.fn();
  const onOpenIcegate = vi.fn();
  const onOpenShortcuts = vi.fn();
  const onOpenUpdates = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders command search and default command categories when open', () => {
    render(
      <MemoryRouter>
        <CommandPaletteDialog
          open={true}
          onOpenChange={onOpenChange}
          onOpenSimulator={onOpenSimulator}
          onOpenIcegate={onOpenIcegate}
          onOpenShortcuts={onOpenShortcuts}
          onOpenUpdates={onOpenUpdates}
        />
      </MemoryRouter>,
    );

    expect(screen.getByRole('dialog', { name: 'Command Palette' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/Type a command or search/i)).toBeInTheDocument();
    expect(screen.getByText('Navigation')).toBeInTheDocument();
    expect(screen.getByText('Executive Dashboard')).toBeInTheDocument();
    expect(screen.getByText('Shipments Registry')).toBeInTheDocument();
  });

  it('filters command list on user input', () => {
    render(
      <MemoryRouter>
        <CommandPaletteDialog
          open={true}
          onOpenChange={onOpenChange}
          onOpenSimulator={onOpenSimulator}
          onOpenIcegate={onOpenIcegate}
          onOpenShortcuts={onOpenShortcuts}
          onOpenUpdates={onOpenUpdates}
        />
      </MemoryRouter>,
    );

    const input = screen.getByPlaceholderText(/Type a command or search/i);
    fireEvent.change(input, { target: { value: 'simulator' } });

    expect(screen.getByText(/Landed Cost "What-If" Simulator/i)).toBeInTheDocument();
    expect(screen.queryByText('Shipments Registry')).not.toBeInTheDocument();
  });

  it('executes quick actions like simulator and closes palette', () => {
    render(
      <MemoryRouter>
        <CommandPaletteDialog
          open={true}
          onOpenChange={onOpenChange}
          onOpenSimulator={onOpenSimulator}
          onOpenIcegate={onOpenIcegate}
          onOpenShortcuts={onOpenShortcuts}
          onOpenUpdates={onOpenUpdates}
        />
      </MemoryRouter>,
    );

    const input = screen.getByPlaceholderText(/Type a command or search/i);
    fireEvent.change(input, { target: { value: 'simulator' } });

    const simItem = screen.getByText(/Landed Cost "What-If" Simulator/i);
    fireEvent.click(simItem);

    expect(onOpenSimulator).toHaveBeenCalledTimes(1);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
