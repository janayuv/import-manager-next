import * as React from 'react';
import { useNavigate } from 'react-router-dom';
import { Calculator, FileDown, FileSpreadsheet, PackageSearch, Search } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useCompany } from '@/contexts/CompanyContext';
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
import { SkeletonTable } from '@/components/ui/skeleton';
import { LandedCostSimulatorDialog } from '@/components/reports/LandedCostSimulatorDialog';
import { downloadTextFile } from '@/lib/download';
import { exportLandedCostPdf } from '@/lib/expense-export';
import {
  reportsService,
  type LandedCostReport,
} from '@/services/expense-service';
import { supplierService, type Supplier } from '@/services/master-service';
import { useModuleSettings } from '@/contexts/ThemeContext';

export function LandedCostReportPage() {
  const navigate = useNavigate();
  const [startDate, setStartDate] = React.useState('');
  const [endDate, setEndDate] = React.useState('');
  const [supplierId, setSupplierId] = React.useState('');
  const [invoiceNo, setInvoiceNo] = React.useState('');
  const [partNo, setPartNo] = React.useState('');
  const [search, setSearch] = React.useState('');
  const [allocationMethod, setAllocationMethod] = React.useState<'av' | 'qty' | 'weight'>('av');
  const [simulatorOpen, setSimulatorOpen] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const { currentCompany } = useCompany();

  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);
  const [report, setReport] = React.useState<LandedCostReport | null>(null);

  const { itemsPerPage: pageSize } = useModuleSettings('landedCost');
  const [page, setPage] = React.useState(1);

  React.useEffect(() => {
    setPage(1);
  }, [pageSize, startDate, endDate, supplierId, invoiceNo, partNo, search]);

  const rows = report?.rows ?? [];
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pagedRows = React.useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return rows.slice(start, start + pageSize);
  }, [rows, currentPage, pageSize]);

  React.useEffect(() => {
    void supplierService
      .list({
        search: '',
        status: 'active',
        sortBy: 'supplierName',
        sortDir: 'asc',
        page: 1,
        pageSize: 500,
      })
      .then((res) => setSuppliers(res.items ?? []))
      .catch(() => {});
  }, []);

  const currentFilter = React.useMemo(
    () => ({ startDate, endDate, supplierId, invoiceNo, partNo, search }),
    [startDate, endDate, supplierId, invoiceNo, partNo, search],
  );

  const loadReport = React.useCallback(async () => {
    setLoading(true);
    try {
      const rep = await reportsService.getLandedCostReport(currentFilter);
      setReport(rep);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [currentFilter]);

  React.useEffect(() => {
    void loadReport();
  }, [loadReport]);

  const handleExportCsv = async () => {
    try {
      const csv = await reportsService.exportLandedCostCsv(currentFilter);
      downloadTextFile('landed_cost_report.csv', csv);
      toast.success('Exported 14-column Landed Cost CSV');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
              <PackageSearch className="h-6 w-6 text-primary" />
              Landed Cost Report (Excl. IGST)
            </h1>
            {currentCompany && (
              <Badge variant="outline" className="font-mono text-xs">
                {currentCompany.name} ({currentCompany.shortCode})
                {currentCompany.iecCode ? ` • IEC: ${currentCompany.iecCode}` : ''}
              </Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground mt-0.5">
            Per-unit landed cost from report_view: (Material Value + BCD + SWS + BOE Interest + Proportional Basic Expenses) / Qty.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            className="border-primary/40 text-primary hover:bg-primary/10"
            onClick={() => setSimulatorOpen(true)}
            title="Simulate landed costs under exchange rate or tariff shifts"
          >
            <Calculator className="h-4 w-4 mr-1.5" />
            What-If Simulator
          </Button>
          {report && (
            <>
              <Button variant="outline" onClick={() => void handleExportCsv()}>
                <FileSpreadsheet className="h-4 w-4 mr-1.5" />
                Export 16-Col CSV
              </Button>
              <Button
                variant="outline"
                onClick={() =>
                  void exportLandedCostPdf(
                    report,
                    currentCompany
                      ? `${currentCompany.name}${currentCompany.iecCode ? ` (IEC: ${currentCompany.iecCode})` : ''}`
                      : undefined,
                  )
                }
              >
                <FileDown className="h-4 w-4 mr-1.5" />
                Print Summary PDF
              </Button>
            </>
          )}
        </div>
      </div>

      <Card>
        <CardContent className="grid gap-3 pt-6 md:grid-cols-7">
          <div className="space-y-1">
            <Label>From Date</Label>
            <Input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label>To Date</Label>
            <Input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label>Supplier</Label>
            <Select
              value={supplierId || 'all'}
              onValueChange={(v) => setSupplierId(v === 'all' ? '' : v)}
            >
              <SelectTrigger>
                <SelectValue placeholder="All" />
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
          <div className="space-y-1">
            <Label>Allocation Method</Label>
            <Select
              value={allocationMethod}
              onValueChange={(v) => setAllocationMethod(v as 'av' | 'qty' | 'weight')}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="av">By Assessable Value (Std)</SelectItem>
                <SelectItem value="qty">By Batch Quantity</SelectItem>
                <SelectItem value="weight">By Unit Weight</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Invoice No</Label>
            <Input value={invoiceNo} onChange={(e) => setInvoiceNo(e.target.value)} placeholder="Filter invoice..." />
          </div>
          <div className="space-y-1">
            <Label>Part No</Label>
            <Input value={partNo} onChange={(e) => setPartNo(e.target.value)} placeholder="Filter part no..." />
          </div>
          <div className="space-y-1">
            <Label>Search</Label>
            <div className="relative">
              <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-8"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Any field..."
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {report && (
        <div className="grid gap-4 md:grid-cols-4 lg:grid-cols-8">
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Total Items / Qty</CardDescription>
              <CardTitle className="text-lg">
                {report.summary.totalRows} / {report.summary.totalQuantity.toLocaleString()}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Material Value (INR)</CardDescription>
              <CardTitle className="text-lg">
                {(report.summary.totalMaterialValue || 0).toLocaleString(undefined, { minimumFractionDigits: 2 })}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Assessable Value</CardDescription>
              <CardTitle className="text-lg">
                {report.summary.totalAssessableValue.toLocaleString(undefined, { minimumFractionDigits: 2 })}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Basic Duties (BCD+SWS)</CardDescription>
              <CardTitle className="text-lg">
                {report.summary.totalBasicDuties.toLocaleString(undefined, { minimumFractionDigits: 2 })}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>IGST (Input Credit)</CardDescription>
              <CardTitle className="text-lg text-muted-foreground">
                {report.summary.totalIgst.toLocaleString(undefined, { minimumFractionDigits: 2 })}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>BOE Interest</CardDescription>
              <CardTitle className="text-lg">
                {(report.summary.totalBoeInterest || 0).toLocaleString(undefined, { minimumFractionDigits: 2 })}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Allocated Expenses</CardDescription>
              <CardTitle className="text-lg">
                {report.summary.totalBasicExpenses.toLocaleString(undefined, { minimumFractionDigits: 2 })}
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Total Landed (Excl. IGST)</CardDescription>
              <CardTitle className="text-lg text-primary">
                {report.summary.totalLandedCostExIgst.toLocaleString(undefined, { minimumFractionDigits: 2 })}
              </CardTitle>
            </CardHeader>
          </Card>
        </div>
      )}

      {loading ? (
        <SkeletonTable rows={6} cols={6} />
      ) : (
        <Card>
          <CardContent className="pt-6 overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Supplier</TableHead>
                  <TableHead>Invoice No</TableHead>
                  <TableHead>Date</TableHead>
                  <TableHead>Part No</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Unit</TableHead>
                  <TableHead className="text-right">Qty</TableHead>
                  <TableHead className="text-right">Unit Price</TableHead>
                  <TableHead className="text-right">Material Value</TableHead>
                  <TableHead className="text-right">Assessable</TableHead>
                  <TableHead className="text-right">BCD</TableHead>
                  <TableHead className="text-right">SWS</TableHead>
                  <TableHead className="text-right">IGST</TableHead>
                  <TableHead className="text-right">BOE Interest</TableHead>
                  <TableHead className="text-right">
                    Expenses ({allocationMethod === 'qty' ? 'By Qty' : allocationMethod === 'weight' ? 'By Weight' : 'By AV'})
                  </TableHead>
                  <TableHead className="text-right">LDC / Qty</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(report?.rows ?? []).length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={16} className="text-center text-muted-foreground">
                      No finalized BOE &amp; invoice records matched filters.
                    </TableCell>
                  </TableRow>
                ) : (
                  pagedRows.map((r, idx) => {
                    const totalQty = report!.summary.totalQuantity || 1;
                    const dynamicExpenses =
                      allocationMethod === 'qty'
                        ? (r.qty / totalQty) * report!.summary.totalBasicExpenses
                        : r.expensesTotal;
                    const materialVal = r.materialValue > 0 ? r.materialValue : (r.assessableValue || 0);
                    const dynamicLdc =
                      r.qty > 0
                        ? (materialVal + r.bcdAmount + r.swsAmount + (r.boeInterest || 0) + dynamicExpenses) / r.qty
                        : r.ldcPerQty;

                    return (
                      <TableRow key={`${r.invoiceNo}-${r.partNo}-${idx}`}>
                        <TableCell>{r.supplier}</TableCell>
                        <TableCell className="font-mono text-xs">{r.invoiceNo}</TableCell>
                        <TableCell className="font-mono text-xs">{r.invoiceDate}</TableCell>
                        <TableCell>
                          <button
                            type="button"
                            className="font-mono font-semibold text-primary hover:underline text-left cursor-pointer"
                            title={`View Period Analysis for ${r.partNo}`}
                            onClick={() => navigate(`/part-analysis?partNo=${encodeURIComponent(r.partNo)}`)}
                          >
                            {r.partNo}
                          </button>
                        </TableCell>
                        <TableCell className="max-w-44 truncate">{r.description}</TableCell>
                        <TableCell>{r.unit}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{r.qty.toLocaleString()}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{r.unitPrice.toFixed(2)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{materialVal.toFixed(2)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{r.assessableValue.toFixed(2)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{r.bcdAmount.toFixed(2)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{r.swsAmount.toFixed(2)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums text-muted-foreground">
                          {r.igstAmount.toFixed(2)}
                        </TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{(r.boeInterest || 0).toFixed(2)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{dynamicExpenses.toFixed(2)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums font-semibold text-primary">
                          {dynamicLdc.toFixed(4)}
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>

            <div className="flex items-center justify-between text-sm text-muted-foreground pt-4">
              <span>
                Page {currentPage} of {totalPages}
              </span>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={currentPage <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={currentPage >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                >
                  Next
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <LandedCostSimulatorDialog open={simulatorOpen} onOpenChange={setSimulatorOpen} />
    </div>
  );
}
