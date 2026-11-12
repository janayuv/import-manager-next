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
import {
  CheckCircle2,
  Eye,
  Lock,
  Pencil,
  Plus,
  Search,
  Trash2,
  Unlock,
  Upload,
} from 'lucide-react';
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { CsvImportDialog } from '@/components/master/CsvImportDialog';
import { useAuth } from '@/contexts/AuthContext';
import {
  optionsService,
  supplierService,
  type LookupOption,
  type Supplier,
} from '@/services/master-service';
import {
  shipmentService,
  type Shipment,
  type ShipmentDetail,
  type ShipmentInput,
} from '@/services/shipment-invoice-service';

const shipmentSchema = z.object({
  supplierId: z.string().min(1, 'Supplier is required'),
  invoiceNumber: z.string().trim().min(1, 'Invoice number is required'),
  invoiceDate: z.string().trim().min(1, 'Invoice date is required'),
  goodsCategory: z.string().trim().min(1, 'Category is required'),
  invoiceValue: z.coerce.number().min(0, 'Value cannot be negative'),
  invoiceCurrency: z.string().trim().min(1, 'Currency is required'),
  incoterm: z.string().trim().min(1, 'Incoterm is required'),
  shipmentMode: z.string(),
  shipmentType: z.string(),
  blAwbNumber: z.string(),
  blAwbDate: z.string(),
  vesselName: z.string(),
  containerNumber: z.string(),
  grossWeightKg: z.coerce.number().min(0),
  etd: z.string(),
  eta: z.string(),
  status: z.string().min(1),
  dateOfDelivery: z.string(),
});

type ShipmentFormValues = z.infer<typeof shipmentSchema>;

const EMPTY_SHIPMENT: ShipmentFormValues = {
  supplierId: '',
  invoiceNumber: '',
  invoiceDate: new Date().toISOString().slice(0, 10),
  goodsCategory: 'General',
  invoiceValue: 0,
  invoiceCurrency: 'USD',
  incoterm: 'CIF',
  shipmentMode: 'Sea',
  shipmentType: 'FCL',
  blAwbNumber: '',
  blAwbDate: '',
  vesselName: '',
  containerNumber: '',
  grossWeightKg: 0,
  etd: '',
  eta: '',
  status: 'docs-rcvd',
  dateOfDelivery: '',
};

const DEFAULT_STATUSES = [
  'docs-rcvd',
  'in-transit',
  'customs-clearance',
  'ready-dly',
  'delivered',
  'cancelled',
  'finalized',
  'closed',
  'completed',
];

export function ShipmentsPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');
  const canDelete = hasPermission('data.delete');
  const canUnfreeze = hasPermission('role.write');

  const [shipments, setShipments] = React.useState<Shipment[]>([]);
  const [totalCount, setTotalCount] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const pageSize = 10;
  const [search, setSearch] = React.useState('');
  const [statusFilter, setStatusFilter] = React.useState('all');
  const [loading, setLoading] = React.useState(true);

  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);
  const [opts, setOpts] = React.useState<Record<string, LookupOption[]>>({});

  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Shipment | null>(null);
  const [detail, setDetail] = React.useState<ShipmentDetail | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<Shipment | null>(null);
  const [importOpen, setImportOpen] = React.useState(false);

  const form = useForm<ShipmentFormValues>({
    resolver: zodResolver(shipmentSchema),
    defaultValues: EMPTY_SHIPMENT,
  });

  const loadLookups = React.useCallback(async () => {
    try {
      const [supPage, optMap] = await Promise.all([
        supplierService.list({ search: '', status: 'active', sortBy: 'supplierName', sortDir: 'asc', page: 1, pageSize: 500 }),
        optionsService.listAll(),
      ]);
      setSuppliers(supPage.items ?? []);
      setOpts(optMap);
    } catch {
      // ignore if initial mount is unauthorized
    }
  }, []);

  const loadShipments = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await shipmentService.list({
        search,
        supplierId: '',
        status: statusFilter === 'all' ? '' : statusFilter,
        frozenOnly: false,
        sortBy: 'invoiceDate',
        sortDir: 'desc',
        page,
        pageSize,
      });
      setShipments(res.items ?? []);
      setTotalCount(res.totalCount);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter, page]);

  React.useEffect(() => {
    void loadShipments();
  }, [loadShipments]);

  React.useEffect(() => {
    void loadLookups();
  }, [loadLookups]);

  const openCreate = () => {
    setEditing(null);
    form.reset({
      ...EMPTY_SHIPMENT,
      supplierId: suppliers[0]?.id ?? '',
    });
    setFormOpen(true);
  };

  const openEdit = (sh: Shipment) => {
    setEditing(sh);
    form.reset({
      supplierId: sh.supplierId,
      invoiceNumber: sh.invoiceNumber,
      invoiceDate: sh.invoiceDate,
      goodsCategory: sh.goodsCategory,
      invoiceValue: sh.invoiceValue,
      invoiceCurrency: sh.invoiceCurrency,
      incoterm: sh.incoterm,
      shipmentMode: sh.shipmentMode,
      shipmentType: sh.shipmentType,
      blAwbNumber: sh.blAwbNumber,
      blAwbDate: sh.blAwbDate,
      vesselName: sh.vesselName,
      containerNumber: sh.containerNumber,
      grossWeightKg: sh.grossWeightKg,
      etd: sh.etd,
      eta: sh.eta,
      status: sh.status,
      dateOfDelivery: sh.dateOfDelivery,
    });
    setFormOpen(true);
  };

  const onSubmit = async (values: ShipmentFormValues) => {
    const payload: ShipmentInput = { ...values };
    try {
      if (editing) {
        await shipmentService.update(editing.id, payload);
        toast.success(`Updated shipment ${editing.id}`);
      } else {
        const created = await shipmentService.create(payload);
        toast.success(`Created shipment ${created.id}`);
      }
      setFormOpen(false);
      await loadShipments();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleToggleFreeze = async (sh: Shipment) => {
    try {
      if (sh.isFrozen) {
        await shipmentService.unfreeze(sh.id);
        toast.success(`Unfroze shipment ${sh.id}`);
      } else {
        await shipmentService.freeze(sh.id);
        toast.success(`Froze shipment ${sh.id}`);
      }
      await loadShipments();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleReadySweep = async () => {
    try {
      const updated = await shipmentService.checkReadyForDelivery('');
      toast.success(`Ready-for-delivery check completed (${updated} updated)`);
      await loadShipments();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleInspect = async (sh: Shipment) => {
    try {
      const data = await shipmentService.getDetail(sh.id);
      setDetail(data);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await shipmentService.delete(deleteTarget.id);
      toast.success(`Moved shipment ${deleteTarget.id} to Recycle Bin`);
      setDeleteTarget(null);
      await loadShipments();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const columns = React.useMemo<ColumnDef<Shipment>[]>(
    () => [
      {
        accessorKey: 'id',
        header: 'Shipment',
        cell: ({ row }) => (
          <div className="flex items-center gap-1.5">
            <span className="font-mono text-xs font-semibold">{row.original.id}</span>
            {row.original.isFrozen && (
              <span title="Frozen">
                <Lock className="h-3.5 w-3.5 text-amber-600" />
              </span>
            )}
          </div>
        ),
      },
      { accessorKey: 'supplierName', header: 'Supplier' },
      {
        accessorKey: 'invoiceNumber',
        header: 'Invoice',
        cell: ({ row }) => (
          <div>
            <div className="font-medium">{row.original.invoiceNumber}</div>
            <div className="text-xs text-muted-foreground">{row.original.invoiceDate}</div>
          </div>
        ),
      },
      {
        accessorKey: 'invoiceValue',
        header: 'Value',
        cell: ({ row }) => (
          <span className="font-mono text-xs">
            {row.original.invoiceCurrency} {row.original.invoiceValue.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
          </span>
        ),
      },
      { accessorKey: 'incoterm', header: 'Incoterm' },
      {
        accessorKey: 'status',
        header: 'Status',
        cell: ({ row }) => (
          <Badge variant={row.original.status === 'delivered' ? 'success' : 'outline'}>
            {row.original.status}
          </Badge>
        ),
      },
      {
        id: 'actions',
        header: () => <div className="text-right">Actions</div>,
        cell: ({ row }) => {
          const sh = row.original;
          return (
            <div className="flex justify-end gap-1">
              <Button size="sm" variant="ghost" aria-label={`Inspect ${sh.id}`} onClick={() => void handleInspect(sh)}>
                <Eye className="h-4 w-4" />
              </Button>
              {canWrite && !sh.isFrozen && (
                <Button size="sm" variant="ghost" aria-label={`Edit ${sh.id}`} onClick={() => openEdit(sh)}>
                  <Pencil className="h-4 w-4" />
                </Button>
              )}
              {((!sh.isFrozen && canWrite) || (sh.isFrozen && canUnfreeze)) && (
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={sh.isFrozen ? `Unfreeze ${sh.id}` : `Freeze ${sh.id}`}
                  onClick={() => void handleToggleFreeze(sh)}
                >
                  {sh.isFrozen ? <Unlock className="h-4 w-4" /> : <Lock className="h-4 w-4" />}
                </Button>
              )}
              {canDelete && !sh.isFrozen && (
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={`Delete ${sh.id}`}
                  onClick={() => setDeleteTarget(sh)}
                >
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              )}
            </div>
          );
        },
      },
    ],
    [canWrite, canDelete, canUnfreeze],
  );

  const table = useReactTable({
    data: shipments,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Shipment Management</h1>
          <p className="text-sm text-muted-foreground">
            Track consignments, dates, freeze locks, BOE readiness automation, and batch CSV imports.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {canWrite && (
            <>
              <Button variant="outline" onClick={() => void handleReadySweep()}>
                <CheckCircle2 className="h-4 w-4" />
                Check Ready-Dly (7d)
              </Button>
              <Button variant="outline" onClick={() => setImportOpen(true)}>
                <Upload className="h-4 w-4" />
                Import CSV
              </Button>
              <Button onClick={openCreate}>
                <Plus className="h-4 w-4" />
                New Shipment
              </Button>
            </>
          )}
        </div>
      </div>

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base">Shipments ({totalCount})</CardTitle>
              <CardDescription>Search by ID, invoice number, supplier, BL/AWB, or vessel.</CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative w-64">
                <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  className="pl-8"
                  placeholder="Search shipments..."
                  value={search}
                  onChange={(e) => {
                    setSearch(e.target.value);
                    setPage(1);
                  }}
                />
              </div>
              <Select
                value={statusFilter}
                onValueChange={(v) => {
                  setStatusFilter(v);
                  setPage(1);
                }}
              >
                <SelectTrigger className="w-44">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Statuses</SelectItem>
                  {DEFAULT_STATUSES.map((st) => (
                    <SelectItem key={st} value={st}>
                      {st}
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
                    Loading shipments...
                  </TableCell>
                </TableRow>
              ) : table.getRowModel().rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length} className="text-center text-muted-foreground">
                    No shipments found.
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
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                Previous
              </Button>
              <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editing ? `Edit Shipment (${editing.id})` : 'Create Shipment'}</DialogTitle>
            <DialogDescription>Dates are normalized to YYYY-MM-DD.</DialogDescription>
          </DialogHeader>
          <form onSubmit={form.handleSubmit(onSubmit)} className="grid grid-cols-3 gap-4">
            <div className="space-y-1">
              <Label>Supplier *</Label>
              <Select
                value={form.watch('supplierId')}
                onValueChange={(v) => form.setValue('supplierId', v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select supplier" />
                </SelectTrigger>
                <SelectContent>
                  {suppliers.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.supplierName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label htmlFor="invoiceNumber">Invoice Number *</Label>
              <Input id="invoiceNumber" {...form.register('invoiceNumber')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="invoiceDate">Invoice Date *</Label>
              <Input id="invoiceDate" type="date" {...form.register('invoiceDate')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="invoiceValue">Invoice Value *</Label>
              <Input id="invoiceValue" type="number" step="0.01" {...form.register('invoiceValue')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="invoiceCurrency">Currency *</Label>
              <Input id="invoiceCurrency" list="shp-curr-list" {...form.register('invoiceCurrency')} />
              <datalist id="shp-curr-list">
                {(opts['currencies'] ?? []).map((o) => (
                  <option key={o.value} value={o.value} />
                ))}
              </datalist>
            </div>

            <div className="space-y-1">
              <Label htmlFor="incoterm">Incoterm *</Label>
              <Input id="incoterm" list="shp-inco-list" {...form.register('incoterm')} />
              <datalist id="shp-inco-list">
                {(opts['incoterms'] ?? []).map((o) => (
                  <option key={o.value} value={o.value} />
                ))}
              </datalist>
            </div>

            <div className="space-y-1">
              <Label htmlFor="goodsCategory">Category *</Label>
              <Input id="goodsCategory" {...form.register('goodsCategory')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="shipmentMode">Mode</Label>
              <Input id="shipmentMode" {...form.register('shipmentMode')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="shipmentType">Type</Label>
              <Input id="shipmentType" {...form.register('shipmentType')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="blAwbNumber">BL / AWB No</Label>
              <Input id="blAwbNumber" {...form.register('blAwbNumber')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="blAwbDate">BL / AWB Date</Label>
              <Input id="blAwbDate" type="date" {...form.register('blAwbDate')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="grossWeightKg">Gross Weight (kg)</Label>
              <Input id="grossWeightKg" type="number" step="0.01" {...form.register('grossWeightKg')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="vesselName">Vessel / Flight</Label>
              <Input id="vesselName" {...form.register('vesselName')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="containerNumber">Container No</Label>
              <Input id="containerNumber" {...form.register('containerNumber')} />
            </div>

            <div className="space-y-1">
              <Label>Status</Label>
              <Select value={form.watch('status')} onValueChange={(v) => form.setValue('status', v)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DEFAULT_STATUSES.map((st) => (
                    <SelectItem key={st} value={st}>
                      {st}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label htmlFor="etd">ETD</Label>
              <Input id="etd" type="date" {...form.register('etd')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="eta">ETA</Label>
              <Input id="eta" type="date" {...form.register('eta')} />
            </div>

            <div className="space-y-1">
              <Label htmlFor="dateOfDelivery">Delivery Date</Label>
              <Input id="dateOfDelivery" type="date" {...form.register('dateOfDelivery')} />
            </div>

            <DialogFooter className="col-span-3">
              <Button type="button" variant="outline" onClick={() => setFormOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting ? 'Saving...' : editing ? 'Save Shipment' : 'Create Shipment'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(detail)} onOpenChange={(o) => !o && setDetail(null)}>
        <DialogContent className="max-w-3xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              Shipment {detail?.shipment.id} — {detail?.shipment.invoiceNumber}
            </DialogTitle>
            <DialogDescription>
              {detail?.shipment.supplierName} • {detail?.shipment.invoiceCurrency}{' '}
              {detail?.shipment.invoiceValue.toLocaleString()} • Status: {detail?.shipment.status}
            </DialogDescription>
          </DialogHeader>
          {detail && (
            <div className="space-y-4 text-sm">
              <div>
                <h4 className="mb-1 font-semibold">Linked Commercial Invoices ({(detail.invoices ?? []).length})</h4>
                {(detail.invoices ?? []).length === 0 ? (
                  <p className="text-xs text-muted-foreground">No invoices saved yet.</p>
                ) : (
                  <ul className="space-y-1 text-xs">
                    {(detail.invoices ?? []).map((inv) => (
                      <li key={inv.id} className="flex justify-between rounded border border-border px-2 py-1">
                        <span>{inv.id} ({inv.status})</span>
                        <span>{inv.lineCount} lines • {inv.currency} {inv.computedTotal.toFixed(2)}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div>
                <h4 className="mb-1 font-semibold">Linked BOEs ({(detail.boes ?? []).length})</h4>
                {(detail.boes ?? []).length === 0 ? (
                  <p className="text-xs text-muted-foreground">No BOEs linked yet.</p>
                ) : (
                  <ul className="space-y-1 text-xs">
                    {(detail.boes ?? []).map((b) => (
                      <li key={b.id} className="flex justify-between rounded border border-border px-2 py-1">
                        <span>{b.beNumber} ({b.beDate})</span>
                        <span>Duty: {b.dutyAmount.toLocaleString()} • {b.reconStatus}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div>
                <h4 className="mb-1 font-semibold">Linked Expenses ({(detail.expenses ?? []).length})</h4>
                {(detail.expenses ?? []).length === 0 ? (
                  <p className="text-xs text-muted-foreground">No expenses recorded yet.</p>
                ) : (
                  <ul className="space-y-1 text-xs">
                    {(detail.expenses ?? []).map((ex) => (
                      <li key={ex.id} className="flex justify-between rounded border border-border px-2 py-1">
                        <span>{ex.expenseType}</span>
                        <span>Base: {ex.amount.toFixed(2)} • Total: {ex.totalAmount.toFixed(2)}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(deleteTarget)} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Soft Delete Shipment</DialogTitle>
            <DialogDescription>
              Move shipment <strong>{deleteTarget?.id}</strong> ({deleteTarget?.invoiceNumber}) to the Recycle Bin?
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
        title="Import Shipments CSV / TSV"
        description="Auto-detects comma, semicolon, or tab delimiters and normalizes ISO/DMY/Excel serial dates. Required: invoiceNumber, invoiceDate, invoiceValue."
        templateFilename="shipments-template.csv"
        onDownloadTemplate={() => shipmentService.downloadTemplateCsv()}
        onImport={(content) => shipmentService.importCsv('shipments-upload.csv', content)}
        onCompleted={loadShipments}
      />
    </div>
  );
}
