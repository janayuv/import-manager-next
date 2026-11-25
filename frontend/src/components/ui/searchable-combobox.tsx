import * as React from 'react';
import { Check, ChevronsUpDown, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

export interface ComboboxOption {
  value: string;
  label: string;
  sublabel?: string;
  keywords?: string;
  disabled?: boolean;
}

interface SearchableComboboxProps {
  value: string;
  onValueChange: (value: string) => void;
  options: ComboboxOption[];
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  className?: string;
  triggerId?: string;
  ariaLabel?: string;
  disabled?: boolean;
}

export function SearchableCombobox({
  value,
  onValueChange,
  options,
  placeholder = 'Select option...',
  searchPlaceholder = 'Type to search...',
  emptyText = 'No matches found.',
  className,
  triggerId,
  ariaLabel,
  disabled,
}: SearchableComboboxProps) {
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState('');
  const [activeIndex, setActiveIndex] = React.useState(0);
  const inputRef = React.useRef<HTMLInputElement>(null);

  const selected = React.useMemo(
    () => options.find((o) => o.value === value),
    [options, value],
  );

  const filtered = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return options;
    return options.filter((o) => {
      const hay = `${o.label} ${o.sublabel ?? ''} ${o.keywords ?? ''}`.toLowerCase();
      return hay.includes(q);
    });
  }, [options, query]);

  React.useEffect(() => {
    setActiveIndex(0);
  }, [query, open]);

  React.useEffect(() => {
    if (open) {
      const id = window.setTimeout(() => {
        inputRef.current?.focus();
      }, 10);
      return () => window.clearTimeout(id);
    } else {
      setQuery('');
    }
  }, [open]);

  const handleSelect = (opt: ComboboxOption) => {
    if (opt.disabled) return;
    onValueChange(opt.value);
    setOpen(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex((prev) => Math.min(filtered.length - 1, prev + 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex((prev) => Math.max(0, prev - 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const target = filtered[activeIndex];
      if (target && !target.disabled) {
        handleSelect(target);
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      setOpen(false);
    }
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          id={triggerId}
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          aria-label={ariaLabel}
          disabled={disabled}
          className={cn(
            'h-9 w-full justify-between px-3 font-normal text-left shadow-xs',
            !selected && 'text-muted-foreground',
            className,
          )}
        >
          <span className="truncate">{selected ? selected.label : placeholder}</span>
          <ChevronsUpDown className="ml-2 h-3.5 w-3.5 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] min-w-[240px] p-0" align="start">
        <div className="flex items-center border-b px-2.5 py-1.5">
          <Search className="mr-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <Input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={searchPlaceholder}
            className="h-7 border-0 bg-transparent px-0 text-xs shadow-none focus-visible:ring-0"
          />
        </div>
        <div className="max-h-60 overflow-y-auto p-1">
          {filtered.length === 0 ? (
            <div className="py-4 text-center text-xs text-muted-foreground">{emptyText}</div>
          ) : (
            filtered.map((opt, idx) => {
              const isSelected = opt.value === value;
              const isHighlighted = idx === activeIndex;
              return (
                <button
                  key={opt.value}
                  type="button"
                  disabled={opt.disabled}
                  onMouseEnter={() => setActiveIndex(idx)}
                  onClick={() => handleSelect(opt)}
                  className={cn(
                    'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-xs outline-none transition-colors cursor-pointer',
                    isHighlighted && 'bg-accent text-accent-foreground',
                    isSelected && 'font-medium',
                    opt.disabled && 'pointer-events-none opacity-50',
                  )}
                >
                  <Check
                    className={cn('h-3.5 w-3.5 shrink-0', isSelected ? 'opacity-100 text-primary' : 'opacity-0')}
                  />
                  <div className="min-w-0 flex-1">
                    <div className="truncate">{opt.label}</div>
                    {opt.sublabel && (
                      <div className="truncate text-[10px] text-muted-foreground">{opt.sublabel}</div>
                    )}
                  </div>
                </button>
              );
            })
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
