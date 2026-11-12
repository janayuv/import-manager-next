import * as React from 'react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { BarChart3, FileDown, FileSpreadsheet, Search } from 'lucide-react';
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
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { downloadTextFile } from '@/lib/download';
import { exportExpenseReportExcel } from '@/lib/expense-export';
import {
  expenseReportsService,
  expenseService,
  formatPaiseINR,
  type ExpenseReportFilter,
  type ExpenseReportResult,
  type ExpenseType,
  type ServiceProvider,
} from '@/services/expense-service';

const REPORT_VIEWS: Array<{ id: ExpenseReportFilter['viewMode']; label: string }> = [
  { id: 'detailed', label: 'Detailed Lines' },
  { id: 'summary-by-type', label: 'By Expense Type' },
  { id: 'summary-by-provider', label: 'By Provider' },
  { id: 'summary-by-shipment', label: 'By Shipment' },
  { id: 'summary-by-month', label: 'By Month' },
];

export function ExpenseReportsPage() {
  const [viewMode, setViewMode] = React.useState<string>('detailed');
  const [startDate, setStartDate] = React.useState('');
  const [endDate, setEndDate] = React.useState('');
  const [serviceProviderId, setServiceProviderId] = React.useState('');
  const [expenseTypeId, setExpenseTypeId] = React.useState('');
  const [minAmountInr, setMinAmountInr] = React.useState('');
  const [maxAmountInr, setMaxAmountInr] = React.useState('');
  const [search, setSearch] = React.useState('');

  const [providers, setProviders] = React.useState<ServiceProvider[]>([]);
  const [types, setTypes] = React.useState<ExpenseType[]>([]);
  const [report, setReport] = React.useState<ExpenseReportResult | null>(null);

  React.useEffect(() => {
    void Promise.all([
      expenseService.listServiceProviders(''),
      expenseService.listExpenseTypes(false),
    ])
      .then(([sps, ets]) => {
        setProviders(sps);
        setTypes(ets);
      })
      .catch((err) => toast.error(String(err)));
  }, []);

  const loadReport = React.useCallback(async () => {
    try {
      const minPaise = minAmountInr ? Math.round(Number.parseFloat(minAmountInr) * 100) : 0;
      const maxPaise = maxAmountInr ? Math.round(Number.parseFloat(maxAmountInr) * 100) : 0;
      const data = await expenseReportsService.getReport({
        viewMode,
        startDate,
        endDate,
        shipmentId: '',
        serviceProviderId,
        expenseTypeId,
        minAmountPaise: Number.isFinite(minPaise) ? minPaise : 0,
        maxAmountPaise: Number.isFinite(maxPaise) ? maxPaise : 0,
        search,
      });
      setReport(data);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  }, [viewMode, startDate, endDate, serviceProviderId, expenseTypeId, minAmountInr, maxAmountInr, search]);

  React.useEffect(() => {
    void loadReport();
  }, [loadReport]);

  const exportCsv = () => {
    if (!report) return;
    const header =
      viewMode === 'detailed'
        ? 'Date,Invoice No,Shipment,Service Provider,Expense Type,Basic (INR),GST (INR),TDS (INR),Gross (INR),Net (INR)'
        : 'Group,Lines,Basic (INR),CGST (INR),SGST (INR),IGST (INR),TDS (INR),Gross (INR),Net (INR)';
    const rows =
      viewMode === 'detailed'
        ? report.detailedRows.map((r) =>
            [
              r.invoiceDate,
              r.invoiceNo,
              r.shipmentInvoiceNo,
              `"${r.serviceProviderName.replaceAll('"', '""')}"`,
              `"${r.expenseTypeName.replaceAll('"', '""')}"`,
              (r.amountPaise / 100).toFixed(2),
              ((r.cgstPaise + r.sgstPaise + r.igstPaise) / 100).toFixed(2),
              (r.tdsPaise / 100).toFixed(2),
              (r.totalPaise / 100).toFixed(2),
              (r.netPaise / 100).toFixed(2),
            ].join(','),
          )
        : report.groupedRows.map((g) =>
            [
              `"${g.groupLabel.replaceAll('"', '""')}"`,
              g.lineCount,
              (g.amountPaise / 100).toFixed(2),
              (g.cgstPaise / 100).toFixed(2),
              (g.sgstPaise / 100).toFixed(2),
              (g.igstPaise / 100).toFixed(2),
              (g.tdsPaise / 100).toFixed(2),
              (g.totalPaise / 100).toFixed(2),
              (g.netPaise / 100).toFixed(2),
            ].join(','),
          );
    downloadTextFile(`expense_report_${viewMode}.csv`, [header, ...rows].join('\r\n'));
  };

  const chartData = React.useMemo(
    () =>
      (report?.groupedRows ?? []).map((g) => ({
        name: g.groupLabel,
        basic: g.amountPaise / 100,
        net: g.netPaise / 100,
      })),
    [report],
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Expense Reports &amp; Analytics</h1>
          <p className="text-sm text-muted-foreground">
            5 analytical views powered by integer-paise expense accounting.
          </p>
        </div>
        {report && (
          <div className="flex gap-2">
            <Button variant="outline" onClick={exportCsv}>
              <FileDown className="h-4 w-4" />
              Export CSV
            </Button>
            <Button onClick={() => void exportExpenseReportExcel(report)}>
              <FileSpreadsheet className="h-4 w-4" />
              Export Excel (.xlsx)
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
            <Label>Service Provider</Label>
            <Select
              value={serviceProviderId || 'all'}
              onValueChange={(v) => setServiceProviderId(v === 'all' ? '' : v)}
            >
              <SelectTrigger>
                <SelectValue placeholder="All" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Providers</SelectItem>
                {providers.map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Expense Type</Label>
            <Select
              value={expenseTypeId || 'all'}
              onValueChange={(v) => setExpenseTypeId(v === 'all' ? '' : v)}
            >
              <SelectTrigger>
                <SelectValue placeholder="All" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Types</SelectItem>
                {types.map((t) => (
                  <SelectItem key={t.id} value={t.id}>
                    {t.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Min Basic (INR)</Label>
            <Input
              type="number"
              placeholder="0"
              value={minAmountInr}
              onChange={(e) => setMinAmountInr(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label>Max Basic (INR)</Label>
            <Input
              type="number"
              placeholder="Any"
              value={maxAmountInr}
              onChange={(e) => setMaxAmountInr(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label>Search</Label>
            <div className="relative">
              <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-8"
                placeholder="Invoice, remarks..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {report && (
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Total Basic Spend</CardDescription>
              <CardTitle className="text-xl">INR {formatPaiseINR(report.totalBasePaise)}</CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Total GST Taxes</CardDescription>
              <CardTitle className="text-xl">INR {formatPaiseINR(report.totalTaxPaise)}</CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Total TDS Deducted</CardDescription>
              <CardTitle className="text-xl">INR {formatPaiseINR(report.totalTdsPaise)}</CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>Total Net Payable ({report.totalLines} lines)</CardDescription>
              <CardTitle className="text-xl text-primary">
                INR {formatPaiseINR(report.totalNetPaise)}
              </CardTitle>
            </CardHeader>
          </Card>
        </div>
      )}

      <Tabs value={viewMode} onValueChange={setViewMode}>
        <TabsList className="flex-wrap">
          {REPORT_VIEWS.map((v) => (
            <TabsTrigger key={v.id} value={v.id}>
              {v.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      {viewMode !== 'detailed' && chartData.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <BarChart3 className="h-4 w-4 text-primary" />
              Spend Distribution ({viewMode})
            </CardTitle>
          </CardHeader>
          <CardContent className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Bar dataKey="basic" name="Basic (INR)" fill="#2563eb" radius={[4, 4, 0, 0]} />
                <Bar dataKey="net" name="Net Payable (INR)" fill="#0f766e" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="pt-6">
          {viewMode === 'detailed' ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Date</TableHead>
                  <TableHead>Invoice No</TableHead>
                  <TableHead>Shipment</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead>Expense Type</TableHead>
                  <TableHead className="text-right">Basic</TableHead>
                  <TableHead className="text-right">GST</TableHead>
                  <TableHead className="text-right">TDS</TableHead>
                  <TableHead className="text-right">Net Payable</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(report?.detailedRows ?? []).length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="text-center text-muted-foreground">
                      No matching expense lines found.
                    </TableCell>
                  </TableRow>
                ) : (
                  report!.detailedRows.map((r) => (
                    <TableRow key={r.expenseId}>
                      <TableCell className="font-mono text-xs">{r.invoiceDate}</TableCell>
                      <TableCell className="font-medium">{r.invoiceNo}</TableCell>
                      <TableCell className="font-mono text-xs">{r.shipmentInvoiceNo}</TableCell>
                      <TableCell>{r.serviceProviderName}</TableCell>
                      <TableCell>{r.expenseTypeName}</TableCell>
                      <TableCell className="text-right font-mono">
                        {formatPaiseINR(r.amountPaise)}
                      </TableCell>
                      <TableCell className="text-right font-mono">
                        {formatPaiseINR(r.cgstPaise + r.sgstPaise + r.igstPaise)}
                      </TableCell>
                      <TableCell className="text-right font-mono">
                        {formatPaiseINR(r.tdsPaise)}
                      </TableCell>
                      <TableCell className="text-right font-mono font-semibold">
                        {formatPaiseINR(r.netPaise)}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Group</TableHead>
                  <TableHead className="text-right">Lines</TableHead>
                  <TableHead className="text-right">Basic (INR)</TableHead>
                  <TableHead className="text-right">CGST</TableHead>
                  <TableHead className="text-right">SGST</TableHead>
                  <TableHead className="text-right">IGST</TableHead>
                  <TableHead className="text-right">TDS</TableHead>
                  <TableHead className="text-right">Net Payable (INR)</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(report?.groupedRows ?? []).length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={8} className="text-center text-muted-foreground">
                      No grouped expense records found.
                    </TableCell>
                  </TableRow>
                ) : (
                  report!.groupedRows.map((g) => (
                    <TableRow key={g.groupKey}>
                      <TableCell className="font-medium">{g.groupLabel}</TableCell>
                      <TableCell className="text-right">{g.lineCount}</TableCell>
                      <TableCell className="text-right font-mono">
                        {formatPaiseINR(g.amountPaise)}
                      </TableCell>
                      <TableCell className="text-right font-mono">
                        {formatPaiseINR(g.cgstPaise)}
                      </TableCell>
                      <TableCell className="text-right font-mono">
                        {formatPaiseINR(g.sgstPaise)}
                      </TableCell>
                      <TableCell className="text-right font-mono">
                        {formatPaiseINR(g.igstPaise)}
                      </TableCell>
                      <TableCell className="text-right font-mono">
                        {formatPaiseINR(g.tdsPaise)}
                      </TableCell>
                      <TableCell className="text-right font-mono font-semibold">
                        {formatPaiseINR(g.netPaise)}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
