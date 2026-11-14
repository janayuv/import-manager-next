import * as React from 'react';
import { Pencil, Plus, Receipt, Search, Trash2, X } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
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
import { Switch } from '@/components/ui/switch';
import {
  expenseService,
  type ExpenseType,
  type ExpenseTypeInput,
} from '@/services/expense-service';

interface ManageExpenseTypesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  types: ExpenseType[];
  onUpdated: () => Promise<void>;
  onSelect?: (type: ExpenseType) => void;
  selectedId?: string;
}

export function ManageExpenseTypesDialog({
  open,
  onOpenChange,
  types,
  onUpdated,
  onSelect,
  selectedId,
}: ManageExpenseTypesDialogProps) {
  const [search, setSearch] = React.useState('');
  const [editingId, setEditingId] = React.useState<string | null>(null);
  const [name, setName] = React.useState('');
  const [cgstPct, setCgstPct] = React.useState('9');
  const [sgstPct, setSgstPct] = React.useState('9');
  const [igstPct, setIgstPct] = React.useState('0');
  const [tdsPct, setTdsPct] = React.useState('2');
  const [isActive, setIsActive] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [deletingId, setDeletingId] = React.useState<string | null>(null);

  const resetForm = () => {
    setEditingId(null);
    setName('');
    setCgstPct('9');
    setSgstPct('9');
    setIgstPct('0');
    setTdsPct('2');
    setIsActive(true);
  };

  React.useEffect(() => {
    if (!open) {
      resetForm();
      setSearch('');
    }
  }, [open]);

  const handleEdit = (t: ExpenseType) => {
    setEditingId(t.id);
    setName(t.name);
    setCgstPct((t.defaultCgstRateBp / 100).toString());
    setSgstPct((t.defaultSgstRateBp / 100).toString());
    setIgstPct((t.defaultIgstRateBp / 100).toString());
    setTdsPct((t.defaultTdsRateBp / 100).toString());
    setIsActive(t.isActive);
  };

  const handleCancelEdit = () => {
    resetForm();
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      toast.error('Expense type name is required');
      return;
    }

    const payload: ExpenseTypeInput = {
      name: trimmedName,
      defaultCgstRateBp: Math.round((Number.parseFloat(cgstPct) || 0) * 100),
      defaultSgstRateBp: Math.round((Number.parseFloat(sgstPct) || 0) * 100),
      defaultIgstRateBp: Math.round((Number.parseFloat(igstPct) || 0) * 100),
      defaultTdsRateBp: Math.round((Number.parseFloat(tdsPct) || 0) * 100),
      isActive,
    };

    setSaving(true);
    try {
      if (editingId) {
        await expenseService.updateExpenseType(editingId, payload);
        toast.success(`Updated expense type "${trimmedName}"`);
        resetForm();
        await onUpdated();
      } else {
        const created = await expenseService.createExpenseType(payload);
        toast.success(`Added expense type "${created.name}"`);
        resetForm();
        await onUpdated();
        if (onSelect && created.id) {
          onSelect(created);
        }
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (t: ExpenseType) => {
    setDeletingId(t.id);
    try {
      await expenseService.deleteExpenseType(t.id);
      toast.success(`Removed expense type "${t.name}"`);
      if (editingId === t.id) {
        resetForm();
      }
      await onUpdated();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setDeletingId(null);
    }
  };

  const filtered = React.useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return types;
    return types.filter((t) => t.name.toLowerCase().includes(q));
  }, [types, search]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto z-[60]">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <Receipt className="h-5 w-5 text-primary" />
            <DialogTitle>Manage Expense Types</DialogTitle>
          </div>
          <DialogDescription>
            Configure expense categories and default statutory tax rates (CGST, SGST, IGST, TDS).
          </DialogDescription>
        </DialogHeader>

        {/* Create / Edit Form */}
        <form
          onSubmit={(e) => void handleSave(e)}
          className="rounded-lg border border-border bg-muted/30 p-3 space-y-3"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {editingId ? 'Edit Expense Type' : 'Add New Expense Type'}
            </span>
            {editingId && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-6 text-xs text-muted-foreground hover:text-foreground"
                onClick={handleCancelEdit}
              >
                <X className="h-3 w-3 mr-1" />
                Cancel Edit
              </Button>
            )}
          </div>

          <div className="space-y-1">
            <Label htmlFor="et-manage-name" className="text-xs">
              Expense Type Name *
            </Label>
            <Input
              id="et-manage-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Ocean Freight / Customs Clearance / DO Charges / Terminal Handling"
              className="h-8 text-xs"
            />
          </div>

          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <div className="space-y-1">
              <Label htmlFor="et-manage-cgst" className="text-xs">
                CGST (%)
              </Label>
              <Input
                id="et-manage-cgst"
                type="number"
                step="0.01"
                min="0"
                max="100"
                value={cgstPct}
                onChange={(e) => setCgstPct(e.target.value)}
                placeholder="9"
                className="h-8 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="et-manage-sgst" className="text-xs">
                SGST (%)
              </Label>
              <Input
                id="et-manage-sgst"
                type="number"
                step="0.01"
                min="0"
                max="100"
                value={sgstPct}
                onChange={(e) => setSgstPct(e.target.value)}
                placeholder="9"
                className="h-8 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="et-manage-igst" className="text-xs">
                IGST (%)
              </Label>
              <Input
                id="et-manage-igst"
                type="number"
                step="0.01"
                min="0"
                max="100"
                value={igstPct}
                onChange={(e) => setIgstPct(e.target.value)}
                placeholder="0"
                className="h-8 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="et-manage-tds" className="text-xs">
                TDS (%)
              </Label>
              <Input
                id="et-manage-tds"
                type="number"
                step="0.01"
                min="0"
                max="100"
                value={tdsPct}
                onChange={(e) => setTdsPct(e.target.value)}
                placeholder="2"
                className="h-8 text-xs"
              />
            </div>
          </div>

          <div className="flex items-center justify-between pt-1">
            <div className="flex items-center gap-2">
              <Switch
                id="et-manage-active"
                checked={isActive}
                onCheckedChange={setIsActive}
              />
              <Label htmlFor="et-manage-active" className="text-xs cursor-pointer">
                Active for Invoicing
              </Label>
            </div>

            <Button type="submit" size="sm" disabled={saving} className="h-8 gap-1 text-xs">
              {editingId ? (
                <>
                  <Pencil className="h-3.5 w-3.5" />
                  Update Expense Type
                </>
              ) : (
                <>
                  <Plus className="h-3.5 w-3.5" />
                  Add Expense Type
                </>
              )}
            </Button>
          </div>
        </form>

        {/* Existing Expense Types List */}
        <div className="space-y-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-semibold text-muted-foreground">
              Configured Expense Types ({types.length})
            </span>
            <div className="relative w-56">
              <Search className="absolute top-2 left-2 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Filter types..."
                className="h-7 pl-7 text-xs"
              />
            </div>
          </div>

          <div className="max-h-60 space-y-1.5 overflow-y-auto rounded-md border border-border p-2">
            {filtered.length === 0 ? (
              <p className="py-6 text-center text-xs text-muted-foreground">
                {search ? 'No matching expense types found.' : 'No expense types configured yet.'}
              </p>
            ) : (
              filtered.map((t) => {
                const isCurrent = selectedId === t.id;
                const cgstStr = (t.defaultCgstRateBp / 100).toFixed(1).replace(/\.0$/, '');
                const sgstStr = (t.defaultSgstRateBp / 100).toFixed(1).replace(/\.0$/, '');
                const igstStr = (t.defaultIgstRateBp / 100).toFixed(1).replace(/\.0$/, '');
                const tdsStr = (t.defaultTdsRateBp / 100).toFixed(1).replace(/\.0$/, '');

                return (
                  <div
                    key={t.id}
                    className={`flex items-center justify-between rounded-md border p-2 transition-colors ${
                      isCurrent
                        ? 'border-primary/50 bg-primary/5'
                        : 'border-border/60 hover:bg-muted/40'
                    }`}
                  >
                    <div className="min-w-0 flex-1 space-y-1 pr-2">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-xs truncate">{t.name}</span>
                        <Badge
                          variant={t.isActive ? 'outline' : 'secondary'}
                          className={`text-[10px] py-0 px-1 ${
                            t.isActive ? 'text-emerald-600 dark:text-emerald-400 border-emerald-500/30' : ''
                          }`}
                        >
                          {t.isActive ? 'Active' : 'Inactive'}
                        </Badge>
                      </div>
                      <div className="flex flex-wrap items-center gap-1.5 text-[10px] text-muted-foreground font-mono">
                        <span className="bg-muted px-1 py-0.5 rounded">CGST: {cgstStr}%</span>
                        <span className="bg-muted px-1 py-0.5 rounded">SGST: {sgstStr}%</span>
                        {t.defaultIgstRateBp > 0 && (
                          <span className="bg-muted px-1 py-0.5 rounded">IGST: {igstStr}%</span>
                        )}
                        <span className="bg-muted px-1 py-0.5 rounded">TDS: {tdsStr}%</span>
                      </div>
                    </div>

                    <div className="flex items-center gap-1 shrink-0">
                      {onSelect && (
                        <Button
                          type="button"
                          size="sm"
                          variant={isCurrent ? 'default' : 'outline'}
                          className="h-7 text-xs px-2"
                          onClick={() => {
                            onSelect(t);
                            onOpenChange(false);
                          }}
                        >
                          {isCurrent ? 'Selected' : 'Select'}
                        </Button>
                      )}
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="h-7 w-7 p-0"
                        title={`Edit ${t.name}`}
                        onClick={() => handleEdit(t)}
                      >
                        <Pencil className="h-3.5 w-3.5 text-muted-foreground hover:text-foreground" />
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="h-7 w-7 p-0 text-destructive hover:bg-destructive/10"
                        title={`Delete ${t.name}`}
                        disabled={deletingId === t.id}
                        onClick={() => void handleDelete(t)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
