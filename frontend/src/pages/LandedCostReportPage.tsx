import * as React from 'react';
import { useNavigate } from 'react-router-dom';
import { FileDown, FileSpreadsheet, PackageSearch, Search } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
import { downloadTextFile } from '@/lib/download';
import { exportLandedCostPdf } from '@/lib/expense-export';
import {
  reportsService,
  type LandedCostReport,
} from '@/services/expense-service';
import { supplierService, type Supplier } from '@/services/master-service';

export function LandedCostReportPage() {
  const navigate = useNavigate();
  const [startDate, setStartDate] = React.useState('');
  const [endDate, setEndDate] = React.useState('');
  const [supplierId, setSupplierId] = React.useState('');
  const [invoiceNo, setInvoiceNo] = React.useState('');
  const [partNo, setPartNo] = React.useState('');
  const [search, setSearch] = React.useState('');

  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);
  const [report, setReport] = React.useState<LandedCostReport | null>(null);

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
    try {
      const rep = await reportsService.getLandedCostReport(currentFilter);
      setReport(rep);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
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
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <PackageSearch className="h-6 w-6 text-primary" />
            Landed Cost Report (Excl. IGST)
          </h1>
          <p className="text-sm text-muted-foreground">
            Per-unit landed cost from report_view: (Assessable + BCD + SWS + BOE Interest + Proportional Basic Expenses) / Qty.
          </p>
        </div>
        {report && (
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => void handleExportCsv()}>
              <FileSpreadsheet className="h-4 w-4" />
              Export 15-Col CSV
            </Button>
            <Button variant="outline" onClick={() => exportLandedCostPdf(report)}>
              <FileDown className="h-4 w-4" />
              Print Summary PDF
            </Button>
          </div>
        )}
      </div>

      <Card>
        <CardContent className="grid gap-3 pt-6 md:grid-cols-6">
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
        <div className="grid gap-4 md:grid-cols-7">
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
                <TableHead className="text-right">Assessable</TableHead>
                <TableHead className="text-right">BCD</TableHead>
                <TableHead className="text-right">SWS</TableHead>
                <TableHead className="text-right">IGST</TableHead>
                <TableHead className="text-right">BOE Interest</TableHead>
                <TableHead className="text-right">Expenses (Basic)</TableHead>
                <TableHead className="text-right">LDC / Qty</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(report?.rows ?? []).length === 0 ? (
                <TableRow>
                  <TableCell colSpan={15} className="text-center text-muted-foreground">
                    No finalized BOE &amp; invoice records matched filters.
                  </TableCell>
                </TableRow>
              ) : (
                report!.rows.map((r, idx) => (
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
                    <TableCell className="text-right font-mono">{r.qty}</TableCell>
                    <TableCell className="text-right font-mono">{r.unitPrice.toFixed(2)}</TableCell>
                    <TableCell className="text-right font-mono">{r.assessableValue.toFixed(2)}</TableCell>
                    <TableCell className="text-right font-mono">{r.bcdAmount.toFixed(2)}</TableCell>
                    <TableCell className="text-right font-mono">{r.swsAmount.toFixed(2)}</TableCell>
                    <TableCell className="text-right font-mono text-muted-foreground">
                      {r.igstAmount.toFixed(2)}
                    </TableCell>
                    <TableCell className="text-right font-mono">{(r.boeInterest || 0).toFixed(2)}</TableCell>
                    <TableCell className="text-right font-mono">{r.expensesTotal.toFixed(2)}</TableCell>
                    <TableCell className="text-right font-mono font-semibold text-primary">
                      {r.ldcPerQty.toFixed(4)}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
