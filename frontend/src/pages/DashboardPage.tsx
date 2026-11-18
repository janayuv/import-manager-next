import * as React from 'react';
import { Link } from 'react-router-dom';
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  Activity,
  AlertTriangle,
  DatabaseBackup,
  FileCheck2,
  Landmark,
  Package,
  RefreshCw,
  ShieldCheck,
  Ship,
  Users,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
import { SkeletonCard, SkeletonChart } from '@/components/ui/skeleton';
import { useAuth } from '@/contexts/AuthContext';
import {
  dashboardService,
  type DashboardOverview,
} from '@/services/dashboard-settings-service';

export function DashboardPage() {
  const { hasPermission } = useAuth();
  const canRebuild = hasPermission('data.write');
  const canViewAudit = hasPermission('admin.activity_log');

  const [financialYear, setFinancialYear] = React.useState('all');
  const [overview, setOverview] = React.useState<DashboardOverview | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  const load = React.useCallback(
    async (forceRefresh = false) => {
      setLoading(true);
      setError(null);
      try {
        const data = await dashboardService.getOverview({
          financialYear,
          startDate: '',
          endDate: '',
          forceRefresh,
        });
        setOverview(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    },
    [financialYear],
  );

  React.useEffect(() => {
    void load(false);
  }, [load]);

  const handleRebuildSnapshot = async () => {
    try {
      const snapDate = await dashboardService.rebuildDailySnapshot();
      toast.success(`Daily KPI snapshot saved (${snapDate})`);
      await load(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const k = overview?.kpis;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">ERP Executive Dashboard</h1>
          <p className="text-sm text-muted-foreground">
            Real-time operational analytics sourced directly from encrypted SQLite records.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Select value={financialYear} onValueChange={setFinancialYear}>
            <SelectTrigger className="w-40" aria-label="Financial Year">
              <SelectValue placeholder="All FYs" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Financial Years</SelectItem>
              {(overview?.availableFys ?? []).map((fy) => (
                <SelectItem key={fy} value={fy}>
                  FY {fy}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" onClick={() => void load(true)} disabled={loading}>
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh Live
          </Button>
          {canRebuild && (
            <Button variant="outline" onClick={() => void handleRebuildSnapshot()}>
              <DatabaseBackup className="h-4 w-4" />
              Rebuild Snapshot
            </Button>
          )}
        </div>
      </div>

      {error && (
        <Card className="border-destructive">
          <CardHeader>
            <CardTitle className="text-destructive">Dashboard Query Error</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
        </Card>
      )}

      {loading && !overview && (
        <div className="space-y-6">
          <div className="grid gap-4 md:grid-cols-4">
            <SkeletonCard />
            <SkeletonCard />
            <SkeletonCard />
            <SkeletonCard />
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <SkeletonChart />
            <SkeletonChart />
          </div>
        </div>
      )}

      {overview && k && (
        <>
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <Badge variant={overview.cacheHit ? 'secondary' : 'success'}>
              {overview.cacheHit ? '180s Cache Hit' : 'Live SQL Aggregation'}
            </Badge>
            <span>Window: {overview.financialYear === 'all' ? 'All Dates' : `FY ${overview.financialYear} (${overview.startDate} to ${overview.endDate})`}</span>
            {overview.lastSnapshotDate && <span>Last Snapshot: {overview.lastSnapshotDate}</span>}
          </div>

          {/* 8 Primary KPI Cards */}
          <div className="grid gap-4 md:grid-cols-4">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>Suppliers (Active / Total)</CardDescription>
                <Users className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {k.activeSuppliers} / {k.totalSuppliers}
                </div>
                <p className="text-xs text-muted-foreground">{k.totalInvoices} commercial invoices</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>Shipments (Pending / Completed)</CardDescription>
                <Ship className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {k.totalShipments}
                </div>
                <p className="text-xs text-muted-foreground">
                  {k.pendingShipments} pending · {k.completedShipments} completed · {k.overdueEtaCount} overdue ETA
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>BOE Reconciliation</CardDescription>
                <FileCheck2 className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {k.reconciledBoes} Reconciled
                </div>
                <p className="text-xs text-muted-foreground">
                  {k.pendingBoes} pending BOEs · {k.shipmentsMissingBoe} missing calc
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>Import Value &amp; Customs Duty</CardDescription>
                <Landmark className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  INR {k.totalImportValueInr.toLocaleString(undefined, { maximumFractionDigits: 0 })}
                </div>
                <p className="text-xs text-muted-foreground">
                  Customs Duty: INR {k.totalCustomsDutyInr.toLocaleString(undefined, { maximumFractionDigits: 0 })}
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>Import Expenses (Basic / Gross)</CardDescription>
                <Package className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-xl font-bold">
                  INR {k.totalBasicExpensesInr.toLocaleString(undefined, { maximumFractionDigits: 0 })}
                </div>
                <p className="text-xs text-muted-foreground">
                  Gross (incl. GST): INR {k.totalGrossExpensesInr.toLocaleString(undefined, { maximumFractionDigits: 0 })}
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>Workflow Health Score</CardDescription>
                <ShieldCheck className="h-4 w-4 text-primary" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-primary">
                  {k.workflowHealthScore.toFixed(1)} / 100
                </div>
                <p className="text-xs text-muted-foreground">
                  {k.openExceptionCount} exceptions · {k.slaBreachCount} SLA breaches
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>Missing Documentation</CardDescription>
                <AlertTriangle className="h-4 w-4 text-amber-500" />
              </CardHeader>
              <CardContent>
                <div className="text-xl font-bold">
                  {k.shipmentsMissingBoe + k.shipmentsMissingExpense}
                </div>
                <p className="text-xs text-muted-foreground">
                  {k.shipmentsMissingBoe} no BOE · {k.shipmentsMissingExpense} no Expenses
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>Active System Incidents</CardDescription>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{k.openIncidentCount}</div>
                <p className="text-xs text-muted-foreground">Monitored runtime &amp; job alerts</p>
              </CardContent>
            </Card>
          </div>

          {/* Operational Alerts */}
          {overview.alerts.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Alerts &amp; Exceptions</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-wrap gap-3">
                {overview.alerts.map((al) => (
                  <Link
                    key={al.kind}
                    to={al.navigationTarget}
                    className="flex items-center gap-2 rounded-md border px-3 py-2 text-xs hover:bg-accent"
                  >
                    <Badge variant={al.severity === 'critical' ? 'destructive' : 'warning'}>
                      {al.count}
                    </Badge>
                    <span className="font-medium">{al.title}</span>
                  </Link>
                ))}
              </CardContent>
            </Card>
          )}

          {/* Charts */}
          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Monthly Import &amp; Customs Duty Trends</CardTitle>
                <CardDescription>Assessable value, customs duty, and basic spend per month (INR)</CardDescription>
              </CardHeader>
              <CardContent className="h-64">
                {overview.monthlyTrends.length === 0 ? (
                  <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                    No shipment activity in selected period.
                  </div>
                ) : (
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={overview.monthlyTrends}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="period" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} />
                      <Tooltip />
                      <Area
                        type="monotone"
                        dataKey="importValueInr"
                        name="Assessable Value"
                        stroke="#2563eb"
                        fill="#2563eb"
                        fillOpacity={0.2}
                      />
                      <Area
                        type="monotone"
                        dataKey="customsDutyInr"
                        name="Customs Duty"
                        stroke="#d97706"
                        fill="#d97706"
                        fillOpacity={0.2}
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Top Suppliers by Assessable Import Value</CardTitle>
                <CardDescription>Cumulative customs valuation &amp; duty liability per supplier</CardDescription>
              </CardHeader>
              <CardContent className="h-64">
                {overview.supplierBreakdown.length === 0 ? (
                  <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                    No supplier valuation data recorded yet.
                  </div>
                ) : (
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={overview.supplierBreakdown}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="supplierName" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} />
                      <Tooltip />
                      <Bar dataKey="totalAssessableInr" name="Assessable INR" fill="#2563eb" radius={[4, 4, 0, 0]} />
                      <Bar dataKey="totalDutyInr" name="Duty INR" fill="#0f766e" radius={[4, 4, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                )}
              </CardContent>
            </Card>
          </div>

          {/* Shipment Statuses & Recent Activity */}
          <div className="grid gap-4 lg:grid-cols-3">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Shipment Statuses</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {overview.statusBreakdown.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No shipments recorded.</p>
                ) : (
                  overview.statusBreakdown.map((st) => (
                    <div key={st.status} className="flex items-center justify-between text-sm">
                      <span className="font-mono text-xs uppercase">{st.status}</span>
                      <Badge variant="outline">{st.count}</Badge>
                    </div>
                  ))
                )}
              </CardContent>
            </Card>

            <Card className="lg:col-span-2">
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-base">Recent ERP Activity</CardTitle>
                {canViewAudit && (
                  <Link
                    to="/admin/activity-log"
                    className="text-xs text-muted-foreground hover:text-primary transition-colors underline-offset-4 hover:underline"
                  >
                    View all
                  </Link>
                )}
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Action</TableHead>
                      <TableHead>Table / Record</TableHead>
                      <TableHead>User</TableHead>
                      <TableHead>Timestamp</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {overview.recentActivity.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={4} className="text-center text-muted-foreground">
                          No recent audit activity.
                        </TableCell>
                      </TableRow>
                    ) : (
                      overview.recentActivity.slice(0, 5).map((act) => (
                        <TableRow key={act.id}>
                          <TableCell>
                            <Badge variant="outline">{act.action}</Badge>
                          </TableCell>
                          <TableCell className="font-mono text-xs">
                            {act.tableName} ({act.rowId})
                          </TableCell>
                          <TableCell>{act.userId}</TableCell>
                          <TableCell className="font-mono text-xs">{act.createdAt}</TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        </>
      )}
    </div>
  );
}
