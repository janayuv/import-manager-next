import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ShipmentLifecycleStepper } from './ShipmentLifecycleStepper';

describe('Phase 1: Shipment Lifecycle Stepper', () => {
  it('renders all 5 stages in full mode with active stage indicator', () => {
    render(<ShipmentLifecycleStepper currentStatus="customs-clearance" isFrozen={false} />);

    expect(screen.getByText('Shipment Pipeline Status')).toBeInTheDocument();
    expect(screen.getByText('Docs Received')).toBeInTheDocument();
    expect(screen.getByText('In Transit')).toBeInTheDocument();
    expect(screen.getAllByText('Customs Clearance').length).toBeGreaterThan(0);
    expect(screen.getByText('Ready for Delivery')).toBeInTheDocument();
    expect(screen.getByText('Delivered & Frozen')).toBeInTheDocument();
    expect(screen.getByText('Stage 3 of 5')).toBeInTheDocument();
  });

  it('renders frozen ledger indicator when shipment is frozen', () => {
    render(<ShipmentLifecycleStepper currentStatus="delivered" isFrozen={true} />);

    expect(screen.getByText('Ledger Record Frozen')).toBeInTheDocument();
  });

  it('renders compact mode with small badges', () => {
    render(<ShipmentLifecycleStepper currentStatus="in-transit" compact={true} />);

    expect(screen.getByText('In Transit')).toBeInTheDocument();
    expect(screen.queryByText('Shipment Pipeline Status')).not.toBeInTheDocument();
  });
});
