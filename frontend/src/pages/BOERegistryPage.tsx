import * as React from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from 'react-router-dom';
import { Award, Calculator, Eye, Pencil, Plus, Search, Trash2, Upload } from 'lucide-react';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { CsvImportDialog } from '@/components/master/CsvImportDialog';
import { ViewReconciledBoeDialog } from '@/components/boe/ViewReconciledBoeDialog';
import { EditReconciledBoeDialog } from '@/components/boe/EditReconciledBoeDialog';
import {
  DeleteReconciledBoeDialog,
  type DeleteReconciledBoeTarget,
} from '@/components/boe/DeleteReconciledBoeDialog';
import { RoDTEPAllocationDrawer } from '@/components/boe/RoDTEPAllocationDrawer';
import { useAuth } from '@/contexts/AuthContext';
import { useModuleSettings } from '@/contexts/ThemeContext';
import {
  boeService,
  boeSummaryService,
  type BOECalculationSummary,
  type BOEDetail,
  type BOEDetailInput,
} from '@/services/boe-service';

const RECON_STATUSES = [
  'Awaiting BOE Data',
  'Discrepancy Found',
  'Reconciled',
  'Investigation',
  'Closed',
];

const boeSchema = z.object({
  beNumber: z.string().trim().min(1, 'BE Number is required'),
  beDate: z.string().trim().min(1, 'BE Date is required'),
  location: z.string().trim().min(1, 'Location is required'),
  totalAssessmentValue: z.coerce.number().min(0),
  dutyAmount: z.coerce.number().min(0),
  paymentDate: z.string(),
  dutyPaid: z.coerce.number().min(0),
  challanNumber: z.string(),
  refId: z.string(),
  transactionId: z.string(),
});

type BOEFormValues = z.infer<typeof boeSchema>;

const EMPTY_BOE: BOEFormValues = {
  beNumber: '',
  beDate: new Date().toISOString().slice(0, 10),
  location: 'INMAA4',
  totalAssessmentValue: 0,
  dutyAmount: 0,
  paymentDate: '',
  dutyPaid: 0,
  challanNumber: '',
  refId: '',
  transactionId: '',
};

export function BOERegistryPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');
  const canDelete = hasPermission('data.delete');

  const { itemsPerPage: boePageSize } = useModuleSettings('boe');
  const summaryPageSize = boePageSize;
  const [summaryPage, setSummaryPage] = React.useState(1);
  const [boePage, setBoePage] = React.useState(1);

  const [rodtepBoeId, setRodtepBoeId] = React.useState('');
  const [rodtepDrawerOpen, setRodtepDrawerOpen] = React.useState(false);

  const [details, setDetails] = React.useState<BOEDetail[]>([]);
  const [summaries, setSummaries] = React.useState<BOECalculationSummary[]>([]);
  const [search, setSearch] = React.useState('');
  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<BOEDetail | null>(null);
  const [importOpen, setImportOpen] = React.useState(false);

  // Calculation dialogs
  const [viewCalcId, setViewCalcId] = React.useState<string | null>(null);
  const [viewOpen, setViewOpen] = React.useState(false);
  const [editCalcId, setEditCalcId] = React.useState<string | null>(null);
  const [editOpen, setEditOpen] = React.useState(false);
  const [deleteCalcTarget, setDeleteCalcTarget] = React.useState<DeleteReconciledBoeTarget | null>(null);
  const [deleteCalcOpen, setDeleteCalcOpen] = React.useState(false);

  React.useEffect(() => {
    setSummaryPage(1);
  }, [summaryPageSize, search]);

  React.useEffect(() => {
    setBoePage(1);
  }, [boePageSize, search]);

  const totalSummaryPages = Math.max(1, Math.ceil(summaries.length / summaryPageSize));
  const currentSummaryPage = Math.min(summaryPage, totalSummaryPages);
  const pagedSummaries = React.useMemo(() => {
    const start = (currentSummaryPage - 1) * summaryPageSize;
    return summaries.slice(start, start + summaryPageSize);
  }, [summaries, currentSummaryPage, summaryPageSize]);

  const totalBoePages = Math.max(1, Math.ceil(details.length / boePageSize));
  const currentBoePage = Math.min(boePage, totalBoePages);
  const pagedDetails = React.useMemo(() => {
    const start = (currentBoePage - 1) * boePageSize;
    return details.slice(start, start + boePageSize);
  }, [details, currentBoePage, boePageSize]);

  const form = useForm<BOEFormValues>({
    resolver: zodResolver(boeSchema),
    defaultValues: EMPTY_BOE,
  });

  const loadAll = React.useCallback(async () => {
    try {
      const [d, s] = await Promise.all([boeService.list(search), boeSummaryService.list(search, '')]);
      setDetails(d);
      setSummaries(s);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  }, [search]);

  React.useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const openCreate = () => {
    setEditing(null);
    form.reset(EMPTY_BOE);
    setFormOpen(true);
  };

  const openEdit = (row: BOEDetail) => {
    setEditing(row);
    form.reset({ ...row });
    setFormOpen(true);
  };

  const onSubmit = async (values: BOEFormValues) => {
    const payload: BOEDetailInput = { ...values };
    try {
      if (editing) {
        await boeService.update(editing.id, payload);
        toast.success(`Updated BOE ${values.beNumber}`);
      } else {
        await boeService.create(payload);
        toast.success(`Registered BOE ${values.beNumber}`);
      }
      setFormOpen(false);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await boeService.delete(id);
      toast.success('Moved BOE detail to Recycle Bin');
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleStatusChange = async (calcId: string, nextStatus: string) => {
    try {
      await boeSummaryService.updateStatus(calcId, nextStatus);
      toast.success(`Status updated to ${nextStatus}`);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Bill of Entry (BOE) Registry</h1>
          <p className="text-sm text-muted-foreground">
            Manage ICEGATE BE challan details, customs duty calculations, and reconciliation status.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" asChild>
            <Link to="/boe-entry">
              <Calculator className="h-4 w-4" />
              Duty Calculator
            </Link>
          </Button>
          {canWrite && (
            <>
              <Button variant="outline" onClick={() => setImportOpen(true)}>
                <Upload className="h-4 w-4" />
                Import BOE CSV
              </Button>
              <Button onClick={openCreate}>
                <Plus className="h-4 w-4" />
                Register BOE
              </Button>
            </>
          )}
        </div>
      </div>

      <div className="relative w-72">
        <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          className="pl-8"
          placeholder="Search BE number, port, shipment..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      <Tabs defaultValue="calculations">
        <TabsList>
          <TabsTrigger value="calculations">Duty Calculations ({summaries.length})</TabsTrigger>
          <TabsTrigger value="registry">BE Challan Registry ({details.length})</TabsTrigger>
        </TabsList>

        <TabsContent value="calculations">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Saved BOE Duty Calculations</CardTitle>
              <CardDescription>
                Compares server-calculated customs duty against linked challan duty_paid.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Shipment / Inv</TableHead>
                    <TableHead>Supplier</TableHead>
                    <TableHead>Linked BE</TableHead>
                    <TableHead>Assessable (INR)</TableHead>
                    <TableHead>Duty Calc (INR)</TableHead>
                    <TableHead>Duty Paid (INR)</TableHead>
                    <TableHead>Recon Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pagedSummaries.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={8} className="text-center text-muted-foreground">
                        No BOE calculations saved yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    pagedSummaries.map((row) => (
                      <TableRow key={row.id}>
                        <TableCell>
                          <div className="font-medium">{row.invoiceNumber}</div>
                          <div className="font-mono text-xs text-muted-foreground">{row.shipmentId}</div>
                        </TableCell>
                        <TableCell>{row.supplierName}</TableCell>
                        <TableCell>
                          {row.beNumber ? (
                            <span className="font-mono text-xs">
                              {row.beNumber} ({row.beDate})
                            </span>
                          ) : (
                            <Badge variant="warning">Unlinked</Badge>
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {row.assessableTotal.toLocaleString()}
                        </TableCell>
                        <TableCell className="font-mono text-xs font-semibold">
                          {row.customsDutyTotal.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {row.dutyPaid.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                        </TableCell>
                        <TableCell className="w-48">
                          <Select
                            value={row.status}
                            disabled={!canWrite}
                            onValueChange={(v) => void handleStatusChange(row.id, v)}
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {RECON_STATUSES.map((st) => (
                                <SelectItem key={st} value={st}>
                                  {st}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 w-8 p-0"
                              title="View Reconciled BOE Dossier"
                              onClick={() => {
                                setViewCalcId(row.id);
                                setViewOpen(true);
                              }}
                            >
                              <Eye className="h-4 w-4 text-muted-foreground hover:text-foreground" />
                              <span className="sr-only">View</span>
                            </Button>
                            {canWrite && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-8 w-8 p-0"
                                title="Edit Reconciled BOE"
                                onClick={() => {
                                  setEditCalcId(row.id);
                                  setEditOpen(true);
                                }}
                              >
                                <Pencil className="h-4 w-4 text-muted-foreground hover:text-foreground" />
                                <span className="sr-only">Edit</span>
                              </Button>
                            )}
                            {canDelete && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-8 w-8 p-0 text-destructive hover:text-destructive"
                                title="Delete BOE Calculation"
                                onClick={() => {
                                  setDeleteCalcTarget({
                                    id: row.id,
                                    invoiceNumber: row.invoiceNumber,
                                    shipmentId: row.shipmentId,
                                    beNumber: row.beNumber,
                                    dutyPaid: row.dutyPaid,
                                  });
                                  setDeleteCalcOpen(true);
                                }}
                              >
                                <Trash2 className="h-4 w-4" />
                                <span className="sr-only">Delete</span>
                              </Button>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>

              <div className="flex items-center justify-between text-sm text-muted-foreground pt-4">
                <span>
                  Page {currentSummaryPage} of {totalSummaryPages}
                </span>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={currentSummaryPage <= 1}
                    onClick={() => setSummaryPage((p) => Math.max(1, p - 1))}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={currentSummaryPage >= totalSummaryPages}
                    onClick={() => setSummaryPage((p) => Math.min(totalSummaryPages, p + 1))}
                  >
                    Next
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="registry">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">ICEGATE Bill of Entry Records</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>BE Number</TableHead>
                    <TableHead>BE Date</TableHead>
                    <TableHead>Port</TableHead>
                    <TableHead>Assessable (INR)</TableHead>
                    <TableHead>Duty Amount</TableHead>
                    <TableHead>Duty Paid</TableHead>
                    <TableHead>Challan</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pagedDetails.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={8} className="text-center text-muted-foreground">
                        No BOE records found.
                      </TableCell>
                    </TableRow>
                  ) : (
                    pagedDetails.map((b) => (
                      <TableRow key={b.id}>
                        <TableCell className="font-mono text-xs font-semibold">{b.beNumber}</TableCell>
                        <TableCell>{b.beDate}</TableCell>
                        <TableCell>{b.location}</TableCell>
                        <TableCell className="font-mono text-xs">
                          {b.totalAssessmentValue.toLocaleString()}
                        </TableCell>
                        <TableCell className="font-mono text-xs">{b.dutyAmount.toLocaleString()}</TableCell>
                        <TableCell className="font-mono text-xs">{b.dutyPaid.toLocaleString()}</TableCell>
                        <TableCell className="font-mono text-xs">{b.challanNumber || '-'}</TableCell>
                        <TableCell className="text-right">
                          <div className="flex justify-end gap-1">
                            <Button
                              size="sm"
                              variant="ghost"
                              title="RoDTEP Duty Settlement"
                              onClick={() => {
                                setRodtepBoeId(b.id);
                                setRodtepDrawerOpen(true);
                              }}
                              className="text-primary hover:text-primary/80"
                            >
                              <Award className="h-4 w-4" />
                            </Button>
                            {canWrite && (
                              <Button size="sm" variant="ghost" onClick={() => openEdit(b)}>
                                <Pencil className="h-4 w-4" />
                              </Button>
                            )}
                            {canDelete && (
                              <Button size="sm" variant="ghost" onClick={() => void handleDelete(b.id)}>
                                <Trash2 className="h-4 w-4 text-destructive" />
                              </Button>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>

              <div className="flex items-center justify-between text-sm text-muted-foreground pt-4">
                <span>
                  Page {currentBoePage} of {totalBoePages}
                </span>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={currentBoePage <= 1}
                    onClick={() => setBoePage((p) => Math.max(1, p - 1))}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={currentBoePage >= totalBoePages}
                    onClick={() => setBoePage((p) => Math.min(totalBoePages, p + 1))}
                  >
                    Next
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{editing ? `Edit BOE ${editing.beNumber}` : 'Register Bill of Entry'}</DialogTitle>
            <DialogDescription>Required: BE Number and BE Date.</DialogDescription>
          </DialogHeader>
          <form onSubmit={form.handleSubmit(onSubmit)} className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <Label htmlFor="beNumber">BE Number *</Label>
              <Input id="beNumber" {...form.register('beNumber')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="beDate">BE Date *</Label>
              <Input id="beDate" type="date" {...form.register('beDate')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="location">Port Code *</Label>
              <Input id="location" {...form.register('location')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="totalAssessmentValue">Assessable Value (INR)</Label>
              <Input id="totalAssessmentValue" type="number" step="0.01" {...form.register('totalAssessmentValue')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="dutyAmount">Duty Amount (INR)</Label>
              <Input id="dutyAmount" type="number" step="0.01" {...form.register('dutyAmount')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="dutyPaid">Duty Paid (INR)</Label>
              <Input id="dutyPaid" type="number" step="0.01" {...form.register('dutyPaid')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="paymentDate">Payment Date</Label>
              <Input id="paymentDate" type="date" {...form.register('paymentDate')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="challanNumber">Challan Number</Label>
              <Input id="challanNumber" {...form.register('challanNumber')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="refId">Ref ID</Label>
              <Input id="refId" {...form.register('refId')} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="transactionId">Transaction ID</Label>
              <Input id="transactionId" {...form.register('transactionId')} />
            </div>
            <DialogFooter className="col-span-2">
              <Button type="button" variant="outline" onClick={() => setFormOpen(false)}>
                Cancel
              </Button>
              <Button type="submit">{editing ? 'Save BOE' : 'Register BOE'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <CsvImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        title="Import BOE Registry CSV"
        description="Required columns: beNumber, beDate. Duplicates on (beNumber, beDate) are skipped."
        templateFilename="boe-template.csv"
        onDownloadTemplate={() => boeService.downloadTemplateCsv()}
        onImport={(content) => boeService.importCsv(content)}
        onCompleted={loadAll}
      />

      {/* Reconciled BOE View Dialog */}
      <ViewReconciledBoeDialog
        calcId={viewCalcId}
        open={viewOpen}
        onOpenChange={setViewOpen}
        canEdit={canWrite}
        onEdit={(id) => {
          setEditCalcId(id);
          setEditOpen(true);
        }}
      />

      {/* Reconciled BOE Edit Dialog */}
      <EditReconciledBoeDialog
        calcId={editCalcId}
        open={editOpen}
        onOpenChange={setEditOpen}
        onSaved={() => void loadAll()}
      />

      {/* Reconciled BOE Delete Dialog */}
      <DeleteReconciledBoeDialog
        target={deleteCalcTarget}
        open={deleteCalcOpen}
        onOpenChange={setDeleteCalcOpen}
        onDeleted={() => void loadAll()}
      />
      {/* RoDTEP Duty Settlement Drawer */}
      <RoDTEPAllocationDrawer
        boeId={rodtepBoeId}
        open={rodtepDrawerOpen}
        onOpenChange={setRodtepDrawerOpen}
        onSuccess={() => void loadAll()}
      />
    </div>
  );
}
