import * as React from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from '@tanstack/react-table';
import { Download, Pencil, Plus, Search, Settings2, Trash2, Upload, X } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { CsvImportDialog } from '@/components/master/CsvImportDialog';
import { QuickOptionDialog } from '@/components/master/QuickOptionDialog';
import { useAuth } from '@/contexts/AuthContext';
import { downloadTextFile } from '@/lib/download';
import {
  itemService,
  optionsService,
  supplierService,
  type Item,
  type ItemInput,
  type LookupOption,
  type Supplier,
} from '@/services/master-service';

const itemSchema = z.object({
  partNumber: z.string().trim().min(1, 'Part number is required'),
  itemDescription: z.string().trim().min(1, 'Description is required'),
  unit: z.string().trim().min(1, 'Unit is required'),
  currency: z.string().trim().min(1, 'Currency is required'),
  unitPrice: z.coerce.number().min(0, 'Unit price cannot be negative'),
  hsnCode: z.string(),
  supplierId: z.string(),
  countryOfOrigin: z.string(),
  bcd: z.string(),
  sws: z.string(),
  igst: z.string(),
  technicalWriteUp: z.string(),
  category: z.string(),
  endUse: z.string(),
  netWeightKg: z.coerce.number().min(0),
  purchaseUom: z.string(),
  grossWeightPerUomKg: z.coerce.number().min(0),
  photoPath: z.string(),
  isActive: z.boolean(),
});

type ItemFormValues = z.infer<typeof itemSchema>;

const EMPTY_ITEM: ItemFormValues = {
  partNumber: '',
  itemDescription: '',
  unit: 'PCS',
  currency: 'USD',
  unitPrice: 0,
  hsnCode: '',
  supplierId: '',
  countryOfOrigin: '',
  bcd: '',
  sws: '',
  igst: '',
  technicalWriteUp: '',
  category: '',
  endUse: '',
  netWeightKg: 0,
  purchaseUom: '',
  grossWeightPerUomKg: 0,
  photoPath: '',
  isActive: true,
};

export function ItemMasterPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');
  const canDelete = hasPermission('data.delete');

  const [items, setItems] = React.useState<Item[]>([]);
  const [totalCount, setTotalCount] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const pageSize = 10;
  const [search, setSearch] = React.useState('');
  const [supplierFilter, setSupplierFilter] = React.useState('all');
  const [loading, setLoading] = React.useState(true);

  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);
  const [allOptions, setAllOptions] = React.useState<Record<string, LookupOption[]>>({});
  const [optionEditor, setOptionEditor] = React.useState<{ table: string; title: string } | null>(null);

  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Item | null>(null);
  const [dupWarning, setDupWarning] = React.useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<Item | null>(null);
  const [importOpen, setImportOpen] = React.useState(false);
  const formCardRef = React.useRef<HTMLDivElement | null>(null);

  const form = useForm<ItemFormValues>({
    resolver: zodResolver(itemSchema),
    defaultValues: EMPTY_ITEM,
  });

  const loadOptions = React.useCallback(async () => {
    try {
      const [optMap, supPage] = await Promise.all([
        optionsService.listAll(),
        supplierService.list({ search: '', status: 'active', sortBy: 'supplierName', sortDir: 'asc', page: 1, pageSize: 500 }),
      ]);
      setAllOptions(optMap);
      setSuppliers(supPage.items ?? []);
    } catch {
      // ignore if initial mount is unauthenticated
    }
  }, []);

  const loadItems = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await itemService.list({
        search,
        supplierId: supplierFilter === 'all' ? '' : supplierFilter,
        status: '',
        sortBy: 'partNumber',
        sortDir: 'asc',
        page,
        pageSize,
      });
      setItems(res.items ?? []);
      setTotalCount(res.totalCount);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search, supplierFilter, page]);

  React.useEffect(() => {
    void loadItems();
  }, [loadItems]);

  React.useEffect(() => {
    void loadOptions();
  }, [loadOptions]);

  const scrollToForm = () => {
    requestAnimationFrame(() => {
      formCardRef.current?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest' });
    });
  };

  const openCreate = () => {
    setEditing(null);
    setDupWarning(null);
    form.reset(EMPTY_ITEM);
    setFormOpen(true);
    scrollToForm();
  };

  const openEdit = (item: Item) => {
    setEditing(item);
    setDupWarning(null);
    form.reset({
      partNumber: item.partNumber,
      itemDescription: item.itemDescription,
      unit: item.unit,
      currency: item.currency,
      unitPrice: item.unitPrice,
      hsnCode: item.hsnCode,
      supplierId: item.supplierId,
      countryOfOrigin: item.countryOfOrigin,
      bcd: item.bcd,
      sws: item.sws,
      igst: item.igst,
      technicalWriteUp: item.technicalWriteUp,
      category: item.category,
      endUse: item.endUse,
      netWeightKg: item.netWeightKg,
      purchaseUom: item.purchaseUom,
      grossWeightPerUomKg: item.grossWeightPerUomKg,
      photoPath: item.photoPath,
      isActive: item.isActive,
    });
    setFormOpen(true);
    scrollToForm();
  };

  const checkPartNumberBlur = async () => {
    const pn = form.getValues('partNumber').trim();
    if (!pn) {
      setDupWarning(null);
      return;
    }
    try {
      const existing = await itemService.checkPartNumber(pn, editing?.id ?? '');
      setDupWarning(existing ? `Part number ${existing.partNumber} already exists (${existing.id})` : null);
    } catch {
      setDupWarning(null);
    }
  };

  const onSubmit = async (values: ItemFormValues) => {
    const payload: ItemInput = { ...values };
    try {
      if (editing) {
        await itemService.update(editing.id, payload);
        toast.success(`Updated item ${values.partNumber}`);
      } else {
        const created = await itemService.create(payload);
        toast.success(`Created item ${created.partNumber} (${created.id})`);
      }
      setFormOpen(false);
      await loadItems();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await itemService.delete(deleteTarget.id);
      toast.success(`Moved item ${deleteTarget.partNumber} to Recycle Bin`);
      setDeleteTarget(null);
      await loadItems();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleExport = async () => {
    try {
      const csv = await itemService.exportCsv();
      downloadTextFile(`items-20col-${new Date().toISOString().slice(0, 10)}.csv`, csv);
      toast.success('Exported 20-column Item Master CSV');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const columns = React.useMemo<ColumnDef<Item>[]>(
    () => [
      {
        accessorKey: 'partNumber',
        header: 'Part No',
        cell: ({ row }) => (
          <div>
            <div className="font-mono text-xs font-semibold">{row.original.partNumber}</div>
            <div className="text-[11px] text-muted-foreground">{row.original.id}</div>
          </div>
        ),
      },
      {
        accessorKey: 'itemDescription',
        header: 'Description',
        cell: ({ row }) => (
          <div>
            <div className="font-medium">{row.original.itemDescription}</div>
            <div className="text-xs text-muted-foreground">{row.original.supplierName || 'Unlinked'}</div>
          </div>
        ),
      },
      { accessorKey: 'hsnCode', header: 'HSN' },
      {
        accessorKey: 'unitPrice',
        header: 'Unit Price',
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {row.original.currency} {row.original.unitPrice.toFixed(2)} / {row.original.unit}
          </span>
        ),
      },
      {
        id: 'dutyRates',
        header: 'BCD / SWS / IGST',
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {row.original.bcd || '-'}% / {row.original.sws || '-'}% / {row.original.igst || '-'}%
          </span>
        ),
      },
      {
        accessorKey: 'isActive',
        header: 'Status',
        cell: ({ row }) => (
          <Badge variant={row.original.isActive ? 'success' : 'secondary'}>
            {row.original.isActive ? 'Active' : 'Inactive'}
          </Badge>
        ),
      },
      {
        id: 'actions',
        header: () => <div className="text-right">Actions</div>,
        cell: ({ row }) => (
          <div className="flex justify-end gap-1">
            {canWrite && (
              <Button
                size="sm"
                variant="ghost"
                aria-label={`Edit ${row.original.partNumber}`}
                onClick={() => openEdit(row.original)}
              >
                <Pencil className="h-4 w-4" />
              </Button>
            )}
            {canDelete && (
              <Button
                size="sm"
                variant="ghost"
                aria-label={`Delete ${row.original.partNumber}`}
                onClick={() => setDeleteTarget(row.original)}
              >
                <Trash2 className="h-4 w-4 text-destructive" />
              </Button>
            )}
          </div>
        ),
      },
    ],
    [canWrite, canDelete],
  );

  const table = useReactTable({
    data: items,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Item Master</h1>
          <p className="text-sm text-muted-foreground">
            20-column catalog, HSN duty baselines, lookup options, and bulk CSV import/export.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" onClick={() => void handleExport()}>
            <Download className="h-4 w-4" />
            Export CSV
          </Button>
          {canWrite && (
            <>
              <Button variant="outline" onClick={() => setImportOpen(true)}>
                <Upload className="h-4 w-4" />
                Import CSV
              </Button>
              <Button onClick={openCreate}>
                <Plus className="h-4 w-4" />
                Add Item
              </Button>
            </>
          )}
        </div>
      </div>

      {formOpen && (
        <div ref={formCardRef}>
          <Card className="border-primary/40">
            <CardHeader>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <CardTitle className="text-base">
                    {editing ? `Edit Item (${editing.id})` : 'Create Item Master Entry'}
                  </CardTitle>
                  <CardDescription>
                    All 20 item fields are preserved with duplicate part-number blocking.
                  </CardDescription>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
                  aria-label="Close item form"
                  onClick={() => setFormOpen(false)}
                >
                  <X className="h-4 w-4" />
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {dupWarning && (
                <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                  {dupWarning}
                </div>
              )}

              <form
                onSubmit={form.handleSubmit(onSubmit)}
                className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
              >
                <div className="space-y-1">
                  <Label htmlFor="partNumber">Part Number *</Label>
                  <Input
                    id="partNumber"
                    {...form.register('partNumber')}
                    onBlur={() => void checkPartNumberBlur()}
                  />
                  {form.formState.errors.partNumber && (
                    <p className="text-xs text-destructive">
                      {form.formState.errors.partNumber.message}
                    </p>
                  )}
                </div>

                <div className="space-y-1 lg:col-span-2">
                  <Label htmlFor="itemDescription">Item Description *</Label>
                  <Input id="itemDescription" {...form.register('itemDescription')} />
                  {form.formState.errors.itemDescription && (
                    <p className="text-xs text-destructive">
                      {form.formState.errors.itemDescription.message}
                    </p>
                  )}
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="unit">Unit *</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() => setOptionEditor({ table: 'units', title: 'Manage Units' })}
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input id="unit" list="item-units-list" {...form.register('unit')} />
                  <datalist id="item-units-list">
                    {(allOptions['units'] ?? []).map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label !== o.value ? o.label : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="currency">Currency *</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() =>
                        setOptionEditor({ table: 'currencies', title: 'Manage Currencies' })
                      }
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input id="currency" list="item-currencies-list" {...form.register('currency')} />
                  <datalist id="item-currencies-list">
                    {(allOptions['currencies'] ?? []).map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label !== o.value ? o.label : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="unitPrice">Unit Price *</Label>
                  <Input id="unitPrice" type="number" step="0.0001" {...form.register('unitPrice')} />
                </div>

                <div className="space-y-1">
                  <Label htmlFor="hsnCode">HSN Code</Label>
                  <Input id="hsnCode" {...form.register('hsnCode')} />
                </div>

                <div className="space-y-1">
                  <Label>Supplier</Label>
                  <Select
                    value={form.watch('supplierId') || 'none'}
                    onValueChange={(v) => form.setValue('supplierId', v === 'none' ? '' : v)}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select supplier" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">Unlinked</SelectItem>
                      {suppliers.map((s) => (
                        <SelectItem key={s.id} value={s.id}>
                          {s.supplierName}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="countryOfOrigin">Country of Origin</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() =>
                        setOptionEditor({ table: 'countries', title: 'Manage Countries' })
                      }
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input
                    id="countryOfOrigin"
                    list="item-countries-list"
                    {...form.register('countryOfOrigin')}
                  />
                  <datalist id="item-countries-list">
                    {(allOptions['countries'] ?? []).map((o) => (
                      <option key={o.value} value={o.label}>
                        {o.label !== o.value ? o.value : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="bcd">BCD %</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() =>
                        setOptionEditor({ table: 'bcd_rates', title: 'Manage BCD Rates' })
                      }
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input id="bcd" list="item-bcd-list" {...form.register('bcd')} />
                  <datalist id="item-bcd-list">
                    {(allOptions['bcd_rates'] ?? []).map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label !== o.value ? o.label : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="sws">SWS %</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() =>
                        setOptionEditor({ table: 'sws_rates', title: 'Manage SWS Rates' })
                      }
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input id="sws" list="item-sws-list" {...form.register('sws')} />
                  <datalist id="item-sws-list">
                    {(allOptions['sws_rates'] ?? []).map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label !== o.value ? o.label : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="igst">IGST %</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() =>
                        setOptionEditor({ table: 'igst_rates', title: 'Manage IGST Rates' })
                      }
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input id="igst" list="item-igst-list" {...form.register('igst')} />
                  <datalist id="item-igst-list">
                    {(allOptions['igst_rates'] ?? []).map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label !== o.value ? o.label : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="category">Category</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() =>
                        setOptionEditor({ table: 'categories', title: 'Manage Categories' })
                      }
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input id="category" list="item-cat-list" {...form.register('category')} />
                  <datalist id="item-cat-list">
                    {(allOptions['categories'] ?? []).map((o) => (
                      <option key={o.value} value={o.label}>
                        {o.label !== o.value ? o.value : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="endUse">End Use</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() =>
                        setOptionEditor({ table: 'end_uses', title: 'Manage End Uses' })
                      }
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input id="endUse" list="item-enduse-list" {...form.register('endUse')} />
                  <datalist id="item-enduse-list">
                    {(allOptions['end_uses'] ?? []).map((o) => (
                      <option key={o.value} value={o.label}>
                        {o.label !== o.value ? o.value : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="purchaseUom">Purchase UOM</Label>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                      onClick={() =>
                        setOptionEditor({ table: 'purchase_uoms', title: 'Manage Purchase UOMs' })
                      }
                    >
                      <Settings2 className="h-3 w-3" />
                      Manage
                    </button>
                  </div>
                  <Input id="purchaseUom" list="item-puom-list" {...form.register('purchaseUom')} />
                  <datalist id="item-puom-list">
                    {(allOptions['purchase_uoms'] ?? []).map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label !== o.value ? o.label : undefined}
                      </option>
                    ))}
                  </datalist>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="netWeightKg">Net Weight (kg)</Label>
                  <Input id="netWeightKg" type="number" step="0.001" {...form.register('netWeightKg')} />
                </div>

                <div className="space-y-1">
                  <Label htmlFor="grossWeightPerUomKg">Gross Wt / UOM (kg)</Label>
                  <Input
                    id="grossWeightPerUomKg"
                    type="number"
                    step="0.001"
                    {...form.register('grossWeightPerUomKg')}
                  />
                </div>

                <div className="flex items-center justify-between pt-6">
                  <Label htmlFor="item-active">Active Item</Label>
                  <Switch
                    id="item-active"
                    checked={form.watch('isActive')}
                    onCheckedChange={(v) => form.setValue('isActive', v)}
                  />
                </div>

                <div className="col-span-full space-y-1">
                  <Label htmlFor="technicalWriteUp">Technical Write-Up</Label>
                  <Textarea id="technicalWriteUp" rows={2} {...form.register('technicalWriteUp')} />
                </div>

                <div className="col-span-full flex justify-end gap-2 border-t border-border pt-4">
                  <Button type="button" variant="outline" onClick={() => setFormOpen(false)}>
                    Cancel
                  </Button>
                  <Button type="submit" disabled={form.formState.isSubmitting}>
                    {form.formState.isSubmitting
                      ? 'Saving...'
                      : editing
                        ? 'Save Changes'
                        : 'Create Item'}
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      )}

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base">Catalog Items ({totalCount})</CardTitle>
              <CardDescription>Search by part number, description, HSN, or category.</CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative w-64">
                <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  className="pl-8"
                  placeholder="Search items..."
                  value={search}
                  onChange={(e) => {
                    setSearch(e.target.value);
                    setPage(1);
                  }}
                />
              </div>
              <Select
                value={supplierFilter}
                onValueChange={(v) => {
                  setSupplierFilter(v);
                  setPage(1);
                }}
              >
                <SelectTrigger className="w-52">
                  <SelectValue placeholder="All Suppliers" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Suppliers</SelectItem>
                  {suppliers.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.supplierName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id}>
                  {hg.headers.map((header) => (
                    <TableHead key={header.id}>
                      {flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={columns.length} className="text-center text-muted-foreground">
                    Loading item catalog...
                  </TableCell>
                </TableRow>
              ) : table.getRowModel().rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length} className="text-center text-muted-foreground">
                    No items found.
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>

          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <span>
              Page {page} of {totalPages}
            </span>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Dialog open={Boolean(deleteTarget)} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Soft Delete Item</DialogTitle>
            <DialogDescription>
              Move <strong>{deleteTarget?.partNumber}</strong> to the Recycle Bin?
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={() => void handleConfirmDelete()}>
              Move to Recycle Bin
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <CsvImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        title="Import Item Master CSV"
        description="Required: partNumber, itemDescription, unit, currency, unitPrice. Automatically resolves supplierName to supplierId."
        templateFilename="items-template.csv"
        onDownloadTemplate={() => itemService.downloadTemplateCsv()}
        onImport={(content) => itemService.importCsv(content)}
        onCompleted={loadItems}
      />

      {optionEditor && (
        <QuickOptionDialog
          open={Boolean(optionEditor)}
          onOpenChange={(o) => !o && setOptionEditor(null)}
          table={optionEditor.table}
          title={optionEditor.title}
          options={allOptions[optionEditor.table] ?? []}
          onUpdated={loadOptions}
        />
      )}
    </div>
  );
}
