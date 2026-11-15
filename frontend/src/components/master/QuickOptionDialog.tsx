import * as React from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { optionsService, type LookupOption } from '@/services/master-service';

interface TableOptionMeta {
  valuePlaceholder: string;
  labelPlaceholder: string;
  helperText: string;
}

const OPTION_TABLE_META: Record<string, TableOptionMeta> = {
  countries: {
    valuePlaceholder: 'e.g. India',
    labelPlaceholder: 'e.g. India',
    helperText: 'Enter the country name (or code in Value and full country name in Label).',
  },
  currencies: {
    valuePlaceholder: 'e.g. USD',
    labelPlaceholder: 'e.g. US Dollar',
    helperText: 'Value should be the 3-letter currency code (e.g. USD, EUR, INR); Label is its name.',
  },
  incoterms: {
    valuePlaceholder: 'e.g. CIF',
    labelPlaceholder: 'e.g. Cost, Insurance & Freight',
    helperText: 'Value should be the trade term code (e.g. CIF, FOB, EXW); Label is its description.',
  },
  shipment_modes: {
    valuePlaceholder: 'e.g. Sea',
    labelPlaceholder: 'e.g. Sea Freight',
    helperText: 'Value is the mode (e.g. Sea, Air, Road); Label is the display name.',
  },
  shipment_types: {
    valuePlaceholder: 'e.g. FCL',
    labelPlaceholder: 'e.g. Full Container Load',
    helperText: 'Value is the cargo type code (e.g. FCL, LCL, Air Cargo); Label is its description.',
  },
  shipment_statuses: {
    valuePlaceholder: 'e.g. in-transit',
    labelPlaceholder: 'e.g. In Transit',
    helperText: 'Value is the workflow status key; Label is the human-readable status.',
  },
  categories: {
    valuePlaceholder: 'e.g. Electronics',
    labelPlaceholder: 'e.g. Electronics',
    helperText: 'Enter the goods or material category name for items and shipments.',
  },
  end_uses: {
    valuePlaceholder: 'e.g. Manufacturing',
    labelPlaceholder: 'e.g. Manufacturing',
    helperText: 'Enter the customs end-use classification (e.g. Manufacturing, Industrial, Trading).',
  },
  units: {
    valuePlaceholder: 'e.g. PCS',
    labelPlaceholder: 'e.g. Pieces',
    helperText: 'Value is the UQC/unit code (e.g. PCS, KGS, SET); Label is the full unit name.',
  },
  purchase_uoms: {
    valuePlaceholder: 'e.g. BOX',
    labelPlaceholder: 'e.g. Box',
    helperText: 'Value is the purchase UOM code (e.g. BOX, CTN, PCS); Label is the full name.',
  },
  bcd_rates: {
    valuePlaceholder: 'e.g. 7.5',
    labelPlaceholder: 'e.g. 7.5%',
    helperText: 'Value must be the numeric BCD % without "%" (e.g. 7.5); Label is the display text (e.g. 7.5%).',
  },
  sws_rates: {
    valuePlaceholder: 'e.g. 10',
    labelPlaceholder: 'e.g. 10%',
    helperText: 'Value must be the numeric SWS % without "%" (e.g. 10); Label is the display text (e.g. 10%).',
  },
  igst_rates: {
    valuePlaceholder: 'e.g. 18',
    labelPlaceholder: 'e.g. 18%',
    helperText: 'Value must be the numeric IGST % without "%" (e.g. 18); Label is the display text (e.g. 18%).',
  },
  insurance_rates: {
    valuePlaceholder: 'e.g. 0.015',
    labelPlaceholder: 'e.g. 0.015% (Open Cover Policy)',
    helperText: 'Value must be the numeric insurance percentage without "%" (e.g. 0.015 or 1.125); Label is the display text.',
  },
};

interface QuickOptionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  table: string;
  title: string;
  options: LookupOption[];
  onUpdated: () => Promise<void>;
}

export function QuickOptionDialog({
  open,
  onOpenChange,
  table,
  title,
  options,
  onUpdated,
}: QuickOptionDialogProps) {
  const [value, setValue] = React.useState('');
  const [label, setLabel] = React.useState('');
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    setValue('');
    setLabel('');
  }, [open, table]);

  const meta = OPTION_TABLE_META[table] ?? {
    valuePlaceholder: 'Enter value',
    labelPlaceholder: 'Enter label',
    helperText: `Manage standardized values in lookup table ${table}.`,
  };

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    const isRateTable = table.endsWith('_rates');
    const rawV = value.trim() || label.trim();
    const v = isRateTable ? rawV.replace(/%$/, '').trim() : rawV;
    const rawL = label.trim();
    const l = rawL || (isRateTable && v ? `${v}%` : v);
    if (!v) return;
    setSaving(true);
    try {
      await optionsService.upsert(table, v, l);
      setValue('');
      setLabel('');
      await onUpdated();
      toast.success(`Saved option "${l}"`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (optValue: string) => {
    try {
      await optionsService.delete(table, optValue);
      await onUpdated();
      toast.success('Option removed');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{meta.helperText}</DialogDescription>
        </DialogHeader>

        <form onSubmit={(e) => void handleAdd(e)} className="grid grid-cols-5 items-end gap-2">
          <div className="col-span-2 space-y-1">
            <Label htmlFor={`opt-val-${table}`}>Value</Label>
            <Input
              id={`opt-val-${table}`}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder={meta.valuePlaceholder}
            />
          </div>
          <div className="col-span-2 space-y-1">
            <Label htmlFor={`opt-lbl-${table}`}>Label</Label>
            <Input
              id={`opt-lbl-${table}`}
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder={meta.labelPlaceholder}
            />
          </div>
          <Button type="submit" disabled={saving}>
            <Plus className="h-4 w-4" />
            Add
          </Button>
        </form>

        <div className="max-h-56 space-y-1 overflow-y-auto rounded-md border border-border p-2">
          {options.length === 0 ? (
            <p className="py-4 text-center text-xs text-muted-foreground">No values configured yet.</p>
          ) : (
            options.map((opt) => (
              <div
                key={opt.value}
                className="flex items-center justify-between rounded px-2 py-1 text-sm hover:bg-muted/50"
              >
                <div>
                  <span className="font-medium">{opt.label}</span>
                  {opt.label !== opt.value && (
                    <span className="ml-2 font-mono text-xs text-muted-foreground">({opt.value})</span>
                  )}
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  onClick={() => void handleDelete(opt.value)}
                  aria-label={`Delete ${opt.label}`}
                >
                  <Trash2 className="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            ))
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
