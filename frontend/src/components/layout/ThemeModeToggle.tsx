import { Monitor, Moon, Sun } from 'lucide-react';
import { useOptionalTheme, type ColorMode } from '@/contexts/ThemeContext';
import { cn } from '@/lib/utils';

const MODES: Array<{ value: ColorMode; label: string; icon: typeof Sun }> = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'system', label: 'System', icon: Monitor },
  { value: 'dark', label: 'Dark', icon: Moon },
];

export function ThemeModeToggle({
  compact = false,
  vertical = false,
}: {
  compact?: boolean;
  vertical?: boolean;
}) {
  const themeCtx = useOptionalTheme();
  if (!themeCtx) return null;

  return (
    <div
      role="group"
      aria-label="Color mode"
      className={cn(
        'inline-flex items-center rounded-md border border-border bg-muted/60 p-0.5',
        vertical ? 'w-full flex-col gap-0.5' : compact ? 'gap-0.5' : 'w-full gap-1',
      )}
    >
      {MODES.map((item) => {
        const Icon = item.icon;
        const active = themeCtx.mode === item.value;
        return (
          <button
            key={item.value}
            type="button"
            aria-label={`${item.label} mode`}
            aria-pressed={active}
            title={`${item.label} mode`}
            onClick={() => void themeCtx.setMode(item.value)}
            className={cn(
              'inline-flex items-center justify-center gap-1 rounded px-2 py-1 text-xs font-medium transition-colors',
              vertical ? 'w-full py-1.5' : !compact && 'flex-1',
              active
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            <Icon className="h-3.5 w-3.5 shrink-0" />
            {!compact && !vertical && <span>{item.label}</span>}
          </button>
        );
      })}
    </div>
  );
}
