import {
  Calculator,
  Command,
  HelpCircle,
  Lock,
} from 'lucide-react';
import { cn } from '@/lib/utils';

interface AppStatusBarProps {
  onOpenCommandPalette: () => void;
  onOpenShortcuts: () => void;
  onOpenSimulator: () => void;
  className?: string;
}

export function AppStatusBar({
  onOpenCommandPalette,
  onOpenShortcuts,
  onOpenSimulator,
  className,
}: AppStatusBarProps) {
  return (
    <footer
      role="contentinfo"
      aria-label="Desktop status bar"
      className={cn(
        'flex h-7 shrink-0 items-center justify-between border-t border-border bg-card/95 px-3 text-[11px] text-muted-foreground select-none backdrop-blur-xs',
        className,
      )}
    >
      <div className="flex items-center gap-3">
        <div
          className="flex items-center gap-1.5 font-medium text-foreground hover:text-primary transition-colors cursor-default"
          title="SQLCipher AES-256 GCM authenticated full-database encryption active"
        >
          <Lock className="h-3 w-3 text-emerald-600 dark:text-emerald-400" />
          <span>SQLCipher AES-256</span>
        </div>

        <span className="text-border">|</span>

        <span className="font-mono text-muted-foreground" title="Current Indian Financial Year">
          FY 2026-27
        </span>

        <span className="hidden sm:inline text-border">|</span>

        <span className="hidden sm:inline text-muted-foreground" title="Application Runtime Architecture">
          Wails v3 Desktop
        </span>
      </div>

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={onOpenSimulator}
          className="hidden md:inline-flex items-center gap-1 hover:text-foreground transition-colors cursor-pointer"
          title="Open Landed Cost 'What-If' Sensitivity Simulator"
        >
          <Calculator className="h-3 w-3 text-primary" />
          <span>Duty Simulator</span>
        </button>

        <span className="hidden md:inline text-border">|</span>

        <button
          type="button"
          onClick={onOpenCommandPalette}
          className="inline-flex items-center gap-1 hover:text-foreground transition-colors cursor-pointer"
          title="Open Command Palette (Ctrl+K)"
        >
          <Command className="h-3 w-3" />
          <span>Commands (Ctrl+K)</span>
        </button>

        <span className="text-border">|</span>

        <button
          type="button"
          onClick={onOpenShortcuts}
          className="inline-flex items-center gap-1 hover:text-foreground transition-colors cursor-pointer"
          title="Open Keyboard Shortcuts (?)"
          aria-label="Keyboard Shortcuts"
        >
          <HelpCircle className="h-3 w-3" />
          <span>Shortcuts (?)</span>
        </button>
      </div>
    </footer>
  );
}
