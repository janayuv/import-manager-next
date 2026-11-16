import { Keyboard } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

interface ShortcutGroup {
  title: string;
  items: Array<{ key: string; description: string }>;
}

const SHORTCUT_GROUPS: ShortcutGroup[] = [
  {
    title: 'Global & Desktop Controls',
    items: [
      { key: 'Ctrl + K', description: 'Open spotlight Command Palette' },
      { key: '? or Ctrl + /', description: 'Open this Keyboard Shortcuts sheet' },
      { key: 'Ctrl + B', description: 'Toggle / collapse sidebar navigation' },
      { key: 'Esc', description: 'Close active modal, drawer, or dialog' },
    ],
  },
  {
    title: 'Fast Navigation (Alt + Key)',
    items: [
      { key: 'Alt + 1 / Alt + D', description: 'Jump to Executive Dashboard' },
      { key: 'Alt + 2 / Alt + S', description: 'Jump to Shipments Registry' },
      { key: 'Alt + 3 / Alt + I', description: 'Jump to Commercial Invoices' },
      { key: 'Alt + 4 / Alt + W', description: 'Jump to Commercial Invoice Wizard' },
      { key: 'Alt + 5 / Alt + B', description: 'Jump to BOE Registry / Duty Entry' },
      { key: 'Alt + 6 / Alt + E', description: 'Jump to Import Expenses Ledger' },
      { key: 'Alt + 7 / Alt + L', description: 'Jump to Landed Cost Report' },
      { key: 'Alt + 8 / Alt + P', description: 'Jump to Part / Item Period Analysis' },
    ],
  },
  {
    title: 'Customs, Entry & Table Controls',
    items: [
      { key: 'Tab / Shift + Tab', description: 'Traverse cells and inputs in multi-line grids' },
      { key: 'Ctrl + Enter', description: 'Submit active form or finalize calculation' },
      { key: '↑ / ↓', description: 'Navigate items in Command Palette & dropdowns' },
      { key: 'Enter', description: 'Confirm selection or open highlighted record' },
    ],
  },
];

interface KeyboardShortcutsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function KeyboardShortcutsDialog({
  open,
  onOpenChange,
}: KeyboardShortcutsDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10 text-primary">
              <Keyboard className="h-4 w-4" />
            </div>
            <div>
              <DialogTitle>Keyboard Shortcuts</DialogTitle>
              <DialogDescription>
                Power-user shortcuts for rapid navigation and data entry
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {SHORTCUT_GROUPS.map((group) => (
            <div key={group.title} className="space-y-2">
              <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {group.title}
              </h4>
              <div className="divide-y divide-border/40 rounded-lg border border-border bg-card">
                {group.items.map((item) => (
                  <div
                    key={item.key}
                    className="flex items-center justify-between px-3 py-2 text-sm"
                  >
                    <span className="text-muted-foreground">{item.description}</span>
                    <kbd className="inline-flex h-6 items-center rounded border border-border bg-muted/80 px-2 font-mono text-xs font-medium text-foreground shadow-2xs">
                      {item.key}
                    </kbd>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div className="rounded-md bg-muted/50 p-2.5 text-center text-xs text-muted-foreground">
          Tip: You can press <kbd className="font-mono">Ctrl+K</kbd> at any time to jump to any page or trigger quick actions.
        </div>
      </DialogContent>
    </Dialog>
  );
}
