import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { KeyboardShortcutsDialog } from './KeyboardShortcutsDialog';

describe('Phase 1: Keyboard Shortcuts Cheat Sheet', () => {
  it('renders shortcut categories and keybindings', () => {
    const onOpenChange = vi.fn();
    render(<KeyboardShortcutsDialog open={true} onOpenChange={onOpenChange} />);

    expect(screen.getByText('Keyboard Shortcuts')).toBeInTheDocument();
    expect(screen.getByText('Global & Desktop Controls')).toBeInTheDocument();
    expect(screen.getByText('Fast Navigation (Alt + Key)')).toBeInTheDocument();
    expect(screen.getByText('Ctrl + K')).toBeInTheDocument();
    expect(screen.getByText('Open spotlight Command Palette')).toBeInTheDocument();
  });
});
