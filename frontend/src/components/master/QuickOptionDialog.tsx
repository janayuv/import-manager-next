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

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    const v = value.trim() || label.trim();
    const l = label.trim() || value.trim();
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
          <DialogDescription>
            Manage standardized values in lookup table <code className="font-mono text-xs">{table}</code>.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={(e) => void handleAdd(e)} className="grid grid-cols-5 items-end gap-2">
          <div className="col-span-2 space-y-1">
            <Label htmlFor={`opt-val-${table}`}>Value</Label>
            <Input
              id={`opt-val-${table}`}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder="e.g. PCS"
            />
          </div>
          <div className="col-span-2 space-y-1">
            <Label htmlFor={`opt-lbl-${table}`}>Label</Label>
            <Input
              id={`opt-lbl-${table}`}
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="Pieces"
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
