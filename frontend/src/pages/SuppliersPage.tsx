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
  Download,
  Eye,
  Pencil,
  Plus,
  Search,
  Settings2,
  Trash2,
  Upload,
  X,
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
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { CsvImportDialog } from '@/components/master/CsvImportDialog';
import { QuickOptionDialog } from '@/components/master/QuickOptionDialog';
import { useAuth } from '@/contexts/AuthContext';
import { downloadTextFile } from '@/lib/download';
import {
  optionsService,
  supplierService,
  type LookupOption,
  type Supplier,
  type SupplierHistorySummary,
  type SupplierInput,
} from '@/services/master-service';

const supplierSchema = z.object({
  supplierName: z.string().trim().min(1, 'Supplier name is required'),
  shortName: z.string(),
  country: z.string().trim().min(1, 'Country is required'),
  email: z
    .string()
    .trim()
    .refine((v) => v === '' || v.includes('@'), 'Enter a valid email address'),
  phone: z.string(),
  beneficiaryName: z.string(),
  bankName: z.string(),
  branch: z.string(),
  bankAddress: z.string(),
  accountNo: z.string(),
  iban: z.string(),
  swiftCode: z.string(),
  isActive: z.boolean(),
});

type SupplierFormValues = z.infer<typeof supplierSchema>;

const EMPTY_FORM: SupplierFormValues = {
  supplierName: '',
  shortName: '',
  country: '',
  email: '',
  phone: '',
  beneficiaryName: '',
  bankName: '',
  branch: '',
  bankAddress: '',
  accountNo: '',
  iban: '',
  swiftCode: '',
  isActive: true,
};

export function SuppliersPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');
  const canDelete = hasPermission('data.delete');

  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);
  const [totalCount, setTotalCount] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const pageSize = 10;
  const [search, setSearch] = React.useState('');
  const [statusFilter, setStatusFilter] = React.useState('all');
  const [sortBy, setSortBy] = React.useState('supplierName');
  const [sortDir, setSortDir] = React.useState<'asc' | 'desc'>('asc');
  const [loading, setLoading] = React.useState(true);

  const [countries, setCountries] = React.useState<LookupOption[]>([]);
  const [countryDialogOpen, setCountryDialogOpen] = React.useState(false);

  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Supplier | null>(null);
  const [duplicateWarning, setDuplicateWarning] = React.useState<string | null>(null);
  const formCardRef = React.useRef<HTMLDivElement | null>(null);

  const [historyOpen, setHistoryOpen] = React.useState(false);
  const [historySupplier, setHistorySupplier] = React.useState<Supplier | null>(null);
  const [history, setHistory] = React.useState<SupplierHistorySummary | null>(null);

  const [deleteTarget, setDeleteTarget] = React.useState<Supplier | null>(null);
  const [importOpen, setImportOpen] = React.useState(false);

  const form = useForm<SupplierFormValues>({
    resolver: zodResolver(supplierSchema),
    defaultValues: EMPTY_FORM,
  });

  const loadCountries = React.useCallback(async () => {
    try {
      const list = await optionsService.list('countries');
      setCountries(list);
    } catch {
      // ignore if unauthorized
    }
  }, []);

  const loadSuppliers = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await supplierService.list({
        search,
        status: statusFilter === 'all' ? '' : statusFilter,
        sortBy,
        sortDir,
        page,
        pageSize,
      });
      setSuppliers(res.items ?? []);
      setTotalCount(res.totalCount);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter, sortBy, sortDir, page]);

  React.useEffect(() => {
    void loadSuppliers();
  }, [loadSuppliers]);

  React.useEffect(() => {
    void loadCountries();
  }, [loadCountries]);

  const scrollToForm = () => {
    requestAnimationFrame(() => {
      formCardRef.current?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest' });
    });
  };

  const openCreate = () => {
    setEditing(null);
    setDuplicateWarning(null);
    form.reset(EMPTY_FORM);
    setFormOpen(true);
    scrollToForm();
  };

  const openEdit = (sup: Supplier) => {
    setEditing(sup);
    setDuplicateWarning(null);
    form.reset({
      supplierName: sup.supplierName,
      shortName: sup.shortName,
      country: sup.country,
      email: sup.email,
      phone: sup.phone,
      beneficiaryName: sup.beneficiaryName,
      bankName: sup.bankName,
      branch: sup.branch,
      bankAddress: sup.bankAddress,
      accountNo: sup.accountNo,
      iban: sup.iban,
      swiftCode: sup.swiftCode,
      isActive: sup.isActive,
    });
    setFormOpen(true);
    scrollToForm();
  };

  const checkDupBlur = async () => {
    const name = form.getValues('supplierName').trim();
    const email = form.getValues('email').trim();
    if (!name && !email) {
      setDuplicateWarning(null);
      return;
    }
    try {
      const dup = await supplierService.checkDuplicate(name, email, editing?.id ?? '');
      setDuplicateWarning(
        dup ? `Matches existing supplier ${dup.supplierName} (${dup.id})` : null,
      );
    } catch {
      setDuplicateWarning(null);
    }
  };

  const onSubmit = async (values: SupplierFormValues) => {
    const payload: SupplierInput = { ...values };
    try {
      if (editing) {
        await supplierService.update(editing.id, payload);
        toast.success(`Updated supplier ${values.supplierName}`);
      } else {
        const created = await supplierService.create(payload);
        toast.success(`Created supplier ${created.supplierName} (${created.id})`);
      }
      setFormOpen(false);
      await loadSuppliers();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleOpenHistory = async (sup: Supplier) => {
    setHistorySupplier(sup);
    setHistory(null);
    setHistoryOpen(true);
    try {
      const data = await supplierService.getHistory(sup.id);
      setHistory(data);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await supplierService.delete(deleteTarget.id);
      toast.success(`Moved supplier ${deleteTarget.supplierName} to Recycle Bin`);
      setDeleteTarget(null);
      await loadSuppliers();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleExport = async () => {
    try {
      const csv = await supplierService.exportCsv();
      downloadTextFile(`suppliers-${new Date().toISOString().slice(0, 10)}.csv`, csv);
      toast.success('Exported suppliers CSV');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const columns = React.useMemo<ColumnDef<Supplier>[]>(
    () => [
      {
        accessorKey: 'id',
        header: 'ID',
        cell: ({ row }) => <span className="font-mono text-xs">{row.original.id}</span>,
      },
      {
        accessorKey: 'supplierName',
        header: 'Supplier Name',
        cell: ({ row }) => (
          <div>
            <div className="font-medium">{row.original.supplierName}</div>
            {row.original.shortName && (
              <div className="text-xs text-muted-foreground">{row.original.shortName}</div>
            )}
          </div>
        ),
      },
      { accessorKey: 'country', header: 'Country' },
      { accessorKey: 'email', header: 'Email' },
      { accessorKey: 'phone', header: 'Phone' },
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
            <Button
              size="sm"
              variant="ghost"
              title="View History"
              aria-label={`View ${row.original.supplierName}`}
              onClick={() => void handleOpenHistory(row.original)}
            >
              <Eye className="h-4 w-4" />
            </Button>
            {canWrite && (
              <Button
                size="sm"
                variant="ghost"
                title="Edit Supplier"
                aria-label={`Edit ${row.original.supplierName}`}
                onClick={() => openEdit(row.original)}
              >
                <Pencil className="h-4 w-4" />
              </Button>
            )}
            {canDelete && (
              <Button
                size="sm"
                variant="ghost"
                title="Delete Supplier"
                aria-label={`Delete ${row.original.supplierName}`}
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
    data: suppliers,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Supplier Management</h1>
          <p className="text-sm text-muted-foreground">
            Manage overseas vendors, banking details, country lookups, and shipment history.
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
                Add Supplier
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
                  {editing ? `Edit Supplier (${editing.id})` : 'Add New Supplier'}
                </CardTitle>
                <CardDescription>
                  Required: Supplier Name and Country. Banking fields are persisted for remittance exports.
                </CardDescription>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
                aria-label="Close supplier form"
                onClick={() => setFormOpen(false)}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {duplicateWarning && (
              <div className="rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-800 dark:text-amber-200">
                {duplicateWarning}
              </div>
            )}

            <form
              onSubmit={form.handleSubmit(onSubmit)}
              className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
            >
              <div className="space-y-1">
                <Label htmlFor="supplierName">Supplier Name *</Label>
                <Input
                  id="supplierName"
                  {...form.register('supplierName')}
                  onBlur={() => void checkDupBlur()}
                />
                {form.formState.errors.supplierName && (
                  <p className="text-xs text-destructive">
                    {form.formState.errors.supplierName.message}
                  </p>
                )}
              </div>

              <div className="space-y-1">
                <Label htmlFor="shortName">Short Name</Label>
                <Input id="shortName" {...form.register('shortName')} />
              </div>

              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <Label htmlFor="country">Country *</Label>
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                    onClick={() => setCountryDialogOpen(true)}
                  >
                    <Settings2 className="h-3 w-3" />
                    Manage
                  </button>
                </div>
                <Input
                  id="country"
                  list="supplier-country-list"
                  {...form.register('country')}
                  placeholder="Select or type country"
                />
                <datalist id="supplier-country-list">
                  {countries.map((c) => (
                    <option key={c.value} value={c.label}>
                      {c.label !== c.value ? c.value : undefined}
                    </option>
                  ))}
                </datalist>
                {form.formState.errors.country && (
                  <p className="text-xs text-destructive">{form.formState.errors.country.message}</p>
                )}
              </div>

              <div className="space-y-1">
                <Label htmlFor="email">Email</Label>
                <Input id="email" {...form.register('email')} onBlur={() => void checkDupBlur()} />
                {form.formState.errors.email && (
                  <p className="text-xs text-destructive">{form.formState.errors.email.message}</p>
                )}
              </div>

              <div className="space-y-1">
                <Label htmlFor="phone">Phone</Label>
                <Input id="phone" {...form.register('phone')} />
              </div>

              <div className="space-y-1">
                <Label htmlFor="beneficiaryName">Beneficiary Name</Label>
                <Input id="beneficiaryName" {...form.register('beneficiaryName')} />
              </div>

              <div className="space-y-1">
                <Label htmlFor="bankName">Bank Name</Label>
                <Input id="bankName" {...form.register('bankName')} />
              </div>

              <div className="space-y-1">
                <Label htmlFor="branch">Branch</Label>
                <Input id="branch" {...form.register('branch')} />
              </div>

              <div className="space-y-1">
                <Label htmlFor="accountNo">Account No</Label>
                <Input id="accountNo" {...form.register('accountNo')} />
              </div>

              <div className="space-y-1 md:col-span-2">
                <Label htmlFor="bankAddress">Bank Address</Label>
                <Input id="bankAddress" {...form.register('bankAddress')} />
              </div>

              <div className="space-y-1">
                <Label htmlFor="iban">IBAN</Label>
                <Input id="iban" {...form.register('iban')} />
              </div>

              <div className="space-y-1">
                <Label htmlFor="swiftCode">SWIFT Code</Label>
                <Input id="swiftCode" {...form.register('swiftCode')} />
              </div>

              <div className="flex items-center justify-between pt-6">
                <Label htmlFor="supplier-active">Active Supplier</Label>
                <Switch
                  id="supplier-active"
                  checked={form.watch('isActive')}
                  onCheckedChange={(v) => form.setValue('isActive', v)}
                />
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
                      : 'Create Supplier'}
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
              <CardTitle className="text-base">Suppliers ({totalCount})</CardTitle>
              <CardDescription>Search by ID, supplier name, country, or email.</CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative w-64">
                <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  className="pl-8"
                  placeholder="Search suppliers..."
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
                <SelectTrigger className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Status</SelectItem>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="inactive">Inactive</SelectItem>
                </SelectContent>
              </Select>
              <Select
                value={`${sortBy}:${sortDir}`}
                onValueChange={(val) => {
                  const [col, dir] = val.split(':');
                  if (col) setSortBy(col);
                  setSortDir(dir === 'desc' ? 'desc' : 'asc');
                }}
              >
                <SelectTrigger className="w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="supplierName:asc">Name (A-Z)</SelectItem>
                  <SelectItem value="supplierName:desc">Name (Z-A)</SelectItem>
                  <SelectItem value="country:asc">Country (A-Z)</SelectItem>
                  <SelectItem value="id:asc">ID (Asc)</SelectItem>
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
                    Loading suppliers...
                  </TableCell>
                </TableRow>
              ) : table.getRowModel().rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length} className="text-center text-muted-foreground">
                    No suppliers match your search filters.
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

      <Dialog open={historyOpen} onOpenChange={setHistoryOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {historySupplier?.supplierName} ({historySupplier?.id})
            </DialogTitle>
            <DialogDescription>
              Country: {historySupplier?.country} • Bank: {historySupplier?.bankName || '-'}
            </DialogDescription>
          </DialogHeader>

          {history && (
            <div className="space-y-4">
              <div className="grid grid-cols-3 gap-3">
                <div className="rounded-md border border-border p-3">
                  <div className="text-xs text-muted-foreground">Shipments</div>
                  <div className="text-lg font-bold">{history.shipmentCount}</div>
                </div>
                <div className="rounded-md border border-border p-3">
                  <div className="text-xs text-muted-foreground">Invoices</div>
                  <div className="text-lg font-bold">{history.invoiceCount}</div>
                </div>
                <div className="rounded-md border border-border p-3">
                  <div className="text-xs text-muted-foreground">Total Value</div>
                  <div className="text-lg font-bold">{history.totalValue.toLocaleString()}</div>
                </div>
              </div>

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Shipment</TableHead>
                    <TableHead>Invoice No</TableHead>
                    <TableHead>Date</TableHead>
                    <TableHead>Value</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(history.recentRecords ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="text-center text-muted-foreground">
                        No shipment history recorded for this supplier.
                      </TableCell>
                    </TableRow>
                  ) : (
                    (history.recentRecords ?? []).map((rec) => (
                      <TableRow key={rec.shipmentId}>
                        <TableCell className="font-mono text-xs">{rec.shipmentId}</TableCell>
                        <TableCell>{rec.invoiceNumber}</TableCell>
                        <TableCell>{rec.invoiceDate}</TableCell>
                        <TableCell>
                          {rec.currency} {rec.invoiceValue.toLocaleString()}
                        </TableCell>
                        <TableCell>{rec.status || '-'}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(deleteTarget)} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Soft Delete Supplier</DialogTitle>
            <DialogDescription>
              Move <strong>{deleteTarget?.supplierName}</strong> to the Recycle Bin? You can restore it
              later if no permanent purge is performed.
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
        title="Import Suppliers CSV"
        description="Supports RFC 4180 CSV with quoted commas. Required columns: supplierName, country."
        templateFilename="suppliers-template.csv"
        onDownloadTemplate={() => supplierService.downloadTemplateCsv()}
        onImport={(content) => supplierService.importCsv(content)}
        onCompleted={loadSuppliers}
      />

      <QuickOptionDialog
        open={countryDialogOpen}
        onOpenChange={setCountryDialogOpen}
        table="countries"
        title="Manage Countries"
        options={countries}
        onUpdated={loadCountries}
      />
    </div>
  );
}
