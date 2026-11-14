import * as React from 'react';
import { Building2, Pencil, Plus, Search, Trash2, X } from 'lucide-react';
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
import {
  expenseService,
  type ServiceProvider,
  type ServiceProviderInput,
} from '@/services/expense-service';

interface ManageServiceProvidersDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  providers: ServiceProvider[];
  onUpdated: () => Promise<void>;
  onSelect?: (providerId: string) => void;
  selectedId?: string;
}

export function ManageServiceProvidersDialog({
  open,
  onOpenChange,
  providers,
  onUpdated,
  onSelect,
  selectedId,
}: ManageServiceProvidersDialogProps) {
  const [search, setSearch] = React.useState('');
  const [editingId, setEditingId] = React.useState<string | null>(null);
  const [name, setName] = React.useState('');
  const [gstin, setGstin] = React.useState('');
  const [state, setState] = React.useState('');
  const [contactPerson, setContactPerson] = React.useState('');
  const [contactEmail, setContactEmail] = React.useState('');
  const [contactPhone, setContactPhone] = React.useState('');
  const [saving, setSaving] = React.useState(false);
  const [deletingId, setDeletingId] = React.useState<string | null>(null);

  const resetForm = () => {
    setEditingId(null);
    setName('');
    setGstin('');
    setState('');
    setContactPerson('');
    setContactEmail('');
    setContactPhone('');
  };

  React.useEffect(() => {
    if (!open) {
      resetForm();
      setSearch('');
    }
  }, [open]);

  const handleEdit = (p: ServiceProvider) => {
    setEditingId(p.id);
    setName(p.name);
    setGstin(p.gstin || '');
    setState(p.state || '');
    setContactPerson(p.contactPerson || '');
    setContactEmail(p.contactEmail || '');
    setContactPhone(p.contactPhone || '');
  };

  const handleCancelEdit = () => {
    resetForm();
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      toast.error('Service provider name is required');
      return;
    }

    const payload: ServiceProviderInput = {
      name: trimmedName,
      gstin: gstin.trim().toUpperCase(),
      state: state.trim(),
      contactPerson: contactPerson.trim(),
      contactEmail: contactEmail.trim(),
      contactPhone: contactPhone.trim(),
    };

    setSaving(true);
    try {
      if (editingId) {
        await expenseService.updateServiceProvider(editingId, payload);
        toast.success(`Updated service provider "${trimmedName}"`);
        resetForm();
        await onUpdated();
      } else {
        const created = await expenseService.createServiceProvider(payload);
        toast.success(`Added service provider "${created.name}"`);
        resetForm();
        await onUpdated();
        if (onSelect && created.id) {
          onSelect(created.id);
        }
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (p: ServiceProvider) => {
    setDeletingId(p.id);
    try {
      await expenseService.deleteServiceProvider(p.id);
      toast.success(`Removed service provider "${p.name}"`);
      if (editingId === p.id) {
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
    if (!q) return providers;
    return providers.filter(
      (p) =>
        p.name.toLowerCase().includes(q) ||
        p.gstin?.toLowerCase().includes(q) ||
        p.state?.toLowerCase().includes(q) ||
        p.contactPerson?.toLowerCase().includes(q) ||
        p.contactEmail?.toLowerCase().includes(q) ||
        p.contactPhone?.toLowerCase().includes(q)
    );
  }, [providers, search]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto z-[60]">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <Building2 className="h-5 w-5 text-primary" />
            <DialogTitle>Manage Service Providers</DialogTitle>
          </div>
          <DialogDescription>
            Add, update, or remove CHA agents, freight forwarders, and shipping vendors.
          </DialogDescription>
        </DialogHeader>

        {/* Create / Edit Form */}
        <form
          onSubmit={(e) => void handleSave(e)}
          className="rounded-lg border border-border bg-muted/30 p-3 space-y-3"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {editingId ? 'Edit Service Provider' : 'Add New Service Provider'}
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

          <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <div className="space-y-1 sm:col-span-2">
              <Label htmlFor="sp-manage-name" className="text-xs">
                Provider Name *
              </Label>
              <Input
                id="sp-manage-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. DHL Global Forwarding / Apollo Logistics"
                className="h-8 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="sp-manage-gstin" className="text-xs">
                GSTIN
              </Label>
              <Input
                id="sp-manage-gstin"
                value={gstin}
                onChange={(e) => setGstin(e.target.value)}
                placeholder="15-digit GSTIN"
                className="h-8 text-xs font-mono uppercase"
                maxLength={15}
              />
            </div>
          </div>

          <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <div className="space-y-1">
              <Label htmlFor="sp-manage-state" className="text-xs">
                State
              </Label>
              <Input
                id="sp-manage-state"
                value={state}
                onChange={(e) => setState(e.target.value)}
                placeholder="e.g. Maharashtra"
                className="h-8 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="sp-manage-contact" className="text-xs">
                Contact Person
              </Label>
              <Input
                id="sp-manage-contact"
                value={contactPerson}
                onChange={(e) => setContactPerson(e.target.value)}
                placeholder="Contact name"
                className="h-8 text-xs"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="sp-manage-phone" className="text-xs">
                Phone
              </Label>
              <Input
                id="sp-manage-phone"
                value={contactPhone}
                onChange={(e) => setContactPhone(e.target.value)}
                placeholder="Phone number"
                className="h-8 text-xs"
              />
            </div>
          </div>

          <div className="flex items-center justify-between pt-1">
            <div className="flex-1 mr-2">
              <Input
                id="sp-manage-email"
                value={contactEmail}
                onChange={(e) => setContactEmail(e.target.value)}
                placeholder="Contact Email (optional)"
                type="email"
                className="h-8 text-xs"
              />
            </div>
            <Button type="submit" size="sm" disabled={saving} className="h-8 gap-1 text-xs">
              {editingId ? (
                <>
                  <Pencil className="h-3.5 w-3.5" />
                  Update Provider
                </>
              ) : (
                <>
                  <Plus className="h-3.5 w-3.5" />
                  Add Provider
                </>
              )}
            </Button>
          </div>
        </form>

        {/* Existing Providers List */}
        <div className="space-y-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-semibold text-muted-foreground">
              Configured Providers ({providers.length})
            </span>
            <div className="relative w-56">
              <Search className="absolute top-2 left-2 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Filter providers..."
                className="h-7 pl-7 text-xs"
              />
            </div>
          </div>

          <div className="max-h-60 space-y-1.5 overflow-y-auto rounded-md border border-border p-2">
            {filtered.length === 0 ? (
              <p className="py-6 text-center text-xs text-muted-foreground">
                {search ? 'No matching service providers found.' : 'No service providers registered yet.'}
              </p>
            ) : (
              filtered.map((p) => {
                const isCurrent = selectedId === p.id;
                return (
                  <div
                    key={p.id}
                    className={`flex items-center justify-between rounded-md border p-2 transition-colors ${
                      isCurrent
                        ? 'border-primary/50 bg-primary/5'
                        : 'border-border/60 hover:bg-muted/40'
                    }`}
                  >
                    <div className="min-w-0 flex-1 space-y-0.5 pr-2">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-xs truncate">{p.name}</span>
                        {p.gstin && (
                          <Badge variant="outline" className="text-[10px] font-mono py-0 px-1">
                            {p.gstin}
                          </Badge>
                        )}
                        {p.state && (
                          <span className="text-[11px] text-muted-foreground">({p.state})</span>
                        )}
                      </div>
                      {(p.contactPerson || p.contactEmail || p.contactPhone) && (
                        <div className="text-[11px] text-muted-foreground truncate">
                          {[p.contactPerson, p.contactPhone, p.contactEmail]
                            .filter(Boolean)
                            .join(' · ')}
                        </div>
                      )}
                    </div>

                    <div className="flex items-center gap-1 shrink-0">
                      {onSelect && (
                        <Button
                          type="button"
                          size="sm"
                          variant={isCurrent ? 'default' : 'outline'}
                          className="h-7 text-xs px-2"
                          onClick={() => {
                            onSelect(p.id);
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
                        title={`Edit ${p.name}`}
                        onClick={() => handleEdit(p)}
                      >
                        <Pencil className="h-3.5 w-3.5 text-muted-foreground hover:text-foreground" />
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="h-7 w-7 p-0 text-destructive hover:bg-destructive/10"
                        title={`Delete ${p.name}`}
                        disabled={deletingId === p.id}
                        onClick={() => void handleDelete(p)}
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
