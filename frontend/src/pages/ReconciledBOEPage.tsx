import * as React from 'react';
import { Link } from 'react-router-dom';
import {
  Calculator,
  CheckCircle2,
  Download,
  Eye,
  FileSpreadsheet,
  Pencil,
  RefreshCw,
  Search,
  ShieldAlert,
  Trash2,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  ViewReconciledBoeDialog,
} from '@/components/boe/ViewReconciledBoeDialog';
import {
  EditReconciledBoeDialog,
} from '@/components/boe/EditReconciledBoeDialog';
import {
  DeleteReconciledBoeDialog,
  type DeleteReconciledBoeTarget,
} from '@/components/boe/DeleteReconciledBoeDialog';
import { useAuth } from '@/contexts/AuthContext';
import { useModuleSettings } from '@/contexts/ThemeContext';
import {
  boeSummaryService,
  type BOECalculationSummary,
} from '@/services/boe-service';

export function ReconciledBOEPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');
  const canDelete = hasPermission('data.delete');

  const { itemsPerPage: pageSizeSetting } = useModuleSettings('reconciledBoe');
  const pageSize = pageSizeSetting > 0 ? pageSizeSetting : 10;
  const [currentPage, setCurrentPage] = React.useState(1);

  const [loading, setLoading] = React.useState(true);
  const [summaries, setSummaries] = React.useState<BOECalculationSummary[]>([]);
  const [search, setSearch] = React.useState('');
  const [statusFilter, setStatusFilter] = React.useState<'reconciled' | 'all'>('reconciled');

  // Modal dialog states
  const [viewCalcId, setViewCalcId] = React.useState<string | null>(null);
  const [viewOpen, setViewOpen] = React.useState(false);

  const [editCalcId, setEditCalcId] = React.useState<string | null>(null);
  const [editOpen, setEditOpen] = React.useState(false);

  const [deleteTarget, setDeleteTarget] = React.useState<DeleteReconciledBoeTarget | null>(null);
  const [deleteOpen, setDeleteOpen] = React.useState(false);

  const loadData = React.useCallback(async () => {
    setLoading(true);
    try {
      const data = await boeSummaryService.list(search, '');
      setSummaries(data ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search]);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  React.useEffect(() => {
    setCurrentPage(1);
  }, [search, statusFilter, pageSize]);

  // Filtered rows
  const filteredSummaries = React.useMemo(() => {
    if (statusFilter === 'reconciled') {
      return summaries.filter((s) => s.status === 'Reconciled');
    }
    return summaries;
  }, [summaries, statusFilter]);

  // Aggregate KPI metrics
  const kpiMetrics = React.useMemo(() => {
    const reconciledList = summaries.filter((s) => s.status === 'Reconciled');
    const totalDuty = reconciledList.reduce((acc, row) => acc + (row.dutyPaid || row.customsDutyTotal || 0), 0);
    const totalAssessable = reconciledList.reduce((acc, row) => acc + (row.assessableTotal || 0), 0);
    const linkedChallans = reconciledList.filter((s) => !!s.beNumber).length;

    return {
      reconciledCount: reconciledList.length,
      totalDuty,
      totalAssessable,
      linkedChallans,
    };
  }, [summaries]);

  // Pagination
  const totalPages = Math.max(1, Math.ceil(filteredSummaries.length / pageSize));
  const safePage = Math.min(currentPage, totalPages);
  const pagedRows = React.useMemo(() => {
    const start = (safePage - 1) * pageSize;
    return filteredSummaries.slice(start, start + pageSize);
  }, [filteredSummaries, safePage, pageSize]);

  const handleOpenView = (calcId: string) => {
    setViewCalcId(calcId);
    setViewOpen(true);
  };

  const handleOpenEdit = (calcId: string) => {
    setEditCalcId(calcId);
    setEditOpen(true);
  };

  const handleOpenDelete = (row: BOECalculationSummary) => {
    setDeleteTarget({
      id: row.id,
      invoiceNumber: row.invoiceNumber,
      shipmentId: row.shipmentId,
      beNumber: row.beNumber,
      dutyPaid: row.dutyPaid,
    });
    setDeleteOpen(true);
  };

  const handleExportCSV = () => {
    if (filteredSummaries.length === 0) {
      toast.error('No rows to export');
      return;
    }
    const headers = [
      'Calculation ID',
      'Shipment ID',
      'Invoice Number',
      'Supplier',
      'Linked BE Number',
      'BE Date',
      'Status',
      'Assessable Total (INR)',
      'Customs Duty (INR)',
      'Duty Paid (INR)',
      'Variance (INR)',
    ];

    const rows = filteredSummaries.map((s) => {
      const variance = (s.dutyPaid || 0) - (s.customsDutyTotal || 0);
      return [
        s.id,
        s.shipmentId,
        s.invoiceNumber,
        `"${s.supplierName.replace(/"/g, '""')}"`,
        s.beNumber || 'Unlinked',
        s.beDate || '',
        s.status,
        s.assessableTotal,
        s.customsDutyTotal,
        s.dutyPaid,
        variance,
      ].join(',');
    });

    const csvContent = '\uFEFF' + [headers.join(','), ...rows].join('\r\n');
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `reconciled_boes_${new Date().toISOString().slice(0, 10)}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
    toast.success('Exported reconciled BOEs to CSV');
  };

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight">Reconciled Bill of Entry (BOE)</h1>
            <Badge variant="outline" className="text-xs">
              Module
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground">
            View, edit, audit, and manage customs-reconciled BOE calculations and linked ICEGATE challans.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => void loadData()}>
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>

          <Button variant="outline" size="sm" onClick={handleExportCSV}>
            <Download className="h-4 w-4" />
            Export CSV
          </Button>

          <Button variant="outline" size="sm" asChild>
            <Link to="/boe">
              <FileSpreadsheet className="h-4 w-4" />
              BOE Registry
            </Link>
          </Button>

          {canWrite && (
            <Button size="sm" asChild>
              <Link to="/boe-entry">
                <Calculator className="h-4 w-4" />
                Duty Calculator
              </Link>
            </Button>
          )}
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs font-medium">Reconciled BOEs</CardDescription>
            <CardTitle className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">
              {kpiMetrics.reconciledCount}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-[11px] text-muted-foreground pt-0">
            Total verified duty calculations
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs font-medium">Reconciled Duty (INR)</CardDescription>
            <CardTitle className="text-2xl font-bold text-foreground">
              ₹{kpiMetrics.totalDuty.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-[11px] text-muted-foreground pt-0">
            Customs duty paid & balanced
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs font-medium">Assessable Value (INR)</CardDescription>
            <CardTitle className="text-2xl font-bold text-foreground">
              ₹{kpiMetrics.totalAssessable.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-[11px] text-muted-foreground pt-0">
            Total customs valuation base
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs font-medium">Linked ICEGATE BOEs</CardDescription>
            <CardTitle className="text-2xl font-bold text-foreground">
              {kpiMetrics.linkedChallans}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-[11px] text-muted-foreground pt-0">
            Active ICEGATE challan linkages
          </CardContent>
        </Card>
      </div>

      {/* Filter and Search Bar */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative w-72">
            <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              className="pl-8"
              placeholder="Search invoice, BE no, supplier..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>

          <Tabs
            value={statusFilter}
            onValueChange={(v) => setStatusFilter(v as 'reconciled' | 'all')}
          >
            <TabsList>
              <TabsTrigger value="reconciled">
                Reconciled Only ({kpiMetrics.reconciledCount})
              </TabsTrigger>
              <TabsTrigger value="all">
                All Calculations ({summaries.length})
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </div>

      {/* Main Table */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">
            {statusFilter === 'reconciled' ? 'Reconciled Customs BOEs' : 'All BOE Duty Calculations'}
          </CardTitle>
          <CardDescription>
            Complete overview of customs duty calculations, verified ICEGATE challan duty payments, and actions.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Shipment / Invoice</TableHead>
                  <TableHead>Supplier</TableHead>
                  <TableHead>Linked BE Number</TableHead>
                  <TableHead className="text-right">Assessable (INR)</TableHead>
                  <TableHead className="text-right">Duty Calc (INR)</TableHead>
                  <TableHead className="text-right">Duty Paid (INR)</TableHead>
                  <TableHead className="text-center">Recon Variance</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                      Loading reconciled BOE calculations...
                    </TableCell>
                  </TableRow>
                ) : pagedRows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                      No {statusFilter === 'reconciled' ? 'reconciled' : ''} BOE calculations found.
                    </TableCell>
                  </TableRow>
                ) : (
                  pagedRows.map((row) => {
                    const variance = Math.round(row.dutyPaid || 0) - Math.round(row.customsDutyTotal || 0);
                    const isBalanced = Math.abs(variance) <= 1 && !!row.beNumber;

                    return (
                      <TableRow key={row.id}>
                        <TableCell>
                          <div className="font-semibold text-foreground">{row.invoiceNumber}</div>
                          <div className="font-mono text-xs text-muted-foreground">{row.shipmentId}</div>
                        </TableCell>

                        <TableCell className="max-w-40 truncate" title={row.supplierName}>
                          {row.supplierName}
                        </TableCell>

                        <TableCell>
                          {row.beNumber ? (
                            <div>
                              <span className="font-mono text-xs font-medium">{row.beNumber}</span>
                              <div className="text-[11px] text-muted-foreground">{row.beDate}</div>
                            </div>
                          ) : (
                            <Badge variant="warning" className="text-[11px]">
                              Unlinked
                            </Badge>
                          )}
                        </TableCell>

                        <TableCell className="text-right font-mono text-xs">
                          {row.assessableTotal?.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 }) ?? 0}
                        </TableCell>

                        <TableCell className="text-right font-mono text-xs font-semibold text-primary">
                          {row.customsDutyTotal?.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 }) ?? 0}
                        </TableCell>

                        <TableCell className="text-right font-mono text-xs font-medium">
                          {row.dutyPaid?.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 }) ?? 0}
                        </TableCell>

                        <TableCell className="text-center">
                          {isBalanced ? (
                            <Badge variant="success" className="gap-1 text-[11px]">
                              <CheckCircle2 className="h-3 w-3" />
                              ₹0 (Balanced)
                            </Badge>
                          ) : (
                            <Badge
                              variant={Math.abs(variance) > 0 ? 'warning' : 'outline'}
                              className="gap-1 text-[11px]"
                            >
                              <ShieldAlert className="h-3 w-3" />
                              ₹{Math.abs(variance).toLocaleString()}
                            </Badge>
                          )}
                        </TableCell>

                        <TableCell>
                          <Badge
                            variant={
                              row.status === 'Reconciled'
                                ? 'success'
                                : row.status === 'Discrepancy Found'
                                ? 'destructive'
                                : 'warning'
                            }
                            className="text-[11px]"
                          >
                            {row.status}
                          </Badge>
                        </TableCell>

                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-1">
                            {/* View Option */}
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 w-8 p-0"
                              title="View Reconciled BOE Dossier"
                              onClick={() => handleOpenView(row.id)}
                            >
                              <Eye className="h-4 w-4 text-muted-foreground hover:text-foreground" />
                              <span className="sr-only">View</span>
                            </Button>

                            {/* Edit Option */}
                            {canWrite && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-8 w-8 p-0"
                                title="Edit Reconciled BOE"
                                onClick={() => handleOpenEdit(row.id)}
                              >
                                <Pencil className="h-4 w-4 text-muted-foreground hover:text-foreground" />
                                <span className="sr-only">Edit</span>
                              </Button>
                            )}

                            {/* Delete Option */}
                            {canDelete && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-8 w-8 p-0 text-destructive hover:text-destructive"
                                title="Delete BOE Calculation"
                                onClick={() => handleOpenDelete(row)}
                              >
                                <Trash2 className="h-4 w-4" />
                                <span className="sr-only">Delete</span>
                              </Button>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </div>

          {/* Pagination Controls */}
          <div className="flex items-center justify-between text-sm text-muted-foreground pt-4">
            <span>
              Showing {pagedRows.length} of {filteredSummaries.length} calculations (Page {safePage} of {totalPages})
            </span>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={safePage <= 1}
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={safePage >= totalPages}
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* View Dialog Modal */}
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

      {/* Edit Dialog Modal */}
      <EditReconciledBoeDialog
        calcId={editCalcId}
        open={editOpen}
        onOpenChange={setEditOpen}
        onSaved={() => void loadData()}
      />

      {/* Delete Confirmation Dialog */}
      <DeleteReconciledBoeDialog
        target={deleteTarget}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={() => void loadData()}
      />
    </div>
  );
}
export default ReconciledBOEPage;
