import * as React from 'react';
import {
  AlertTriangle,
  Building2,
  Clock,
  DollarSign,
  RefreshCw,
  Search,
  Truck,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  WarehouseDwellSummary,
  shipmentService,
} from '@/services/shipment-invoice-service';
import { toast } from 'sonner';

interface WarehouseDwellMonitorProps {
  onSelectShipment?: (shipmentId: string) => void;
}

export function WarehouseDwellMonitor({ onSelectShipment }: WarehouseDwellMonitorProps) {
  const [data, setData] = React.useState<WarehouseDwellSummary[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [search, setSearch] = React.useState('');
  const [statusFilter, setStatusFilter] = React.useState<'active' | 'all' | 'dispatched'>('active');

  const loadData = React.useCallback(async () => {
    setLoading(true);
    try {
      const rows = await shipmentService.getWarehouseDwellReport(statusFilter);
      setData(rows ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [statusFilter]);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  const filtered = React.useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return data;
    return data.filter(
      (item) =>
        item.containerNumber.toLowerCase().includes(q) ||
        item.shipmentId.toLowerCase().includes(q) ||
        item.supplierName.toLowerCase().includes(q) ||
        item.warehouseName.toLowerCase().includes(q) ||
        item.invoiceNumber.toLowerCase().includes(q)
    );
  }, [data, search]);

  // Aggregate Metrics
  const activeContainers = data.filter((d) => d.deliveryStatus === 'IN_WAREHOUSE');
  const longDwellCount = activeContainers.filter((d) => d.dwellDays > 7).length;
  const totalAccruedCost = data.reduce((acc, d) => acc + (d.estimatedCharges || 0), 0);

  const getDwellBadge = (dwellDays: number, isDelivered: boolean) => {
    if (isDelivered) {
      return (
        <Badge variant="outline" className="font-mono text-xs">
          {dwellDays} {dwellDays === 1 ? 'day' : 'days'}
        </Badge>
      );
    }
    if (dwellDays <= 5) {
      return (
        <Badge variant="success" className="gap-1 font-mono text-xs">
          <Clock className="h-3 w-3" /> {dwellDays} {dwellDays === 1 ? 'day' : 'days'}
        </Badge>
      );
    }
    if (dwellDays <= 14) {
      return (
        <Badge variant="warning" className="gap-1 font-mono text-xs">
          <Clock className="h-3 w-3" /> {dwellDays} days (Attention)
        </Badge>
      );
    }
    return (
      <Badge variant="destructive" className="gap-1 font-mono text-xs">
        <AlertTriangle className="h-3 w-3" /> {dwellDays} days (Overstay)
      </Badge>
    );
  };

  return (
    <div className="space-y-4">
      {/* KPI Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">
              Containers in Warehouse
            </CardTitle>
            <Building2 className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{activeContainers.length}</div>
            <p className="text-xs text-muted-foreground mt-1">
              Currently buffering across warehouses
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">
              Overstay Risk (&gt; 7 Days)
            </CardTitle>
            <AlertTriangle className="h-4 w-4 text-amber-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-amber-600 dark:text-amber-400">
              {longDwellCount}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Containers exceeding standard free buffer period
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">
              Est. Accrued Storage Charges
            </CardTitle>
            <DollarSign className="h-4 w-4 text-emerald-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              ₹{totalAccruedCost.toLocaleString('en-IN', { maximumFractionDigits: 2 })}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Billable storage days calculated
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Filter and Search Bar */}
      <div className="flex flex-col sm:flex-row items-center justify-between gap-2">
        <div className="relative w-full sm:w-80">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search container, shipment, warehouse..."
            className="pl-8 text-xs h-9"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>

        <div className="flex items-center gap-2 w-full sm:w-auto justify-end">
          <Select
            value={statusFilter}
            onValueChange={(v) => setStatusFilter(v as typeof statusFilter)}
          >
            <SelectTrigger className="h-9 text-xs w-[180px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="active">Currently Stored (Active)</SelectItem>
              <SelectItem value="dispatched">Dispatched / Delivered</SelectItem>
              <SelectItem value="all">All Warehouse Records</SelectItem>
            </SelectContent>
          </Select>

          <Button
            size="sm"
            variant="outline"
            className="h-9 text-xs"
            onClick={() => void loadData()}
            disabled={loading}
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      {/* Dwell Time Table */}
      <div className="rounded-md border border-border bg-card">
        <Table>
          <TableHeader>
            <TableRow className="text-xs">
              <TableHead>Container No</TableHead>
              <TableHead>Size</TableHead>
              <TableHead>Shipment / BL</TableHead>
              <TableHead>Supplier</TableHead>
              <TableHead>Warehouse</TableHead>
              <TableHead>Gate-In Date</TableHead>
              <TableHead>Gate-Out / Delivery</TableHead>
              <TableHead>Days in Warehouse</TableHead>
              <TableHead>Billable Storage</TableHead>
              <TableHead className="text-right">Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={10} className="text-center py-6 text-xs text-muted-foreground">
                  Loading warehouse storage data...
                </TableCell>
              </TableRow>
            ) : filtered.length === 0 ? (
              <TableRow>
                <TableCell colSpan={10} className="text-center py-6 text-xs text-muted-foreground">
                  No warehouse containers found matching the filter.
                </TableCell>
              </TableRow>
            ) : (
              filtered.map((item) => {
                const isDelivered = item.deliveryStatus === 'DELIVERED';
                return (
                  <TableRow key={item.containerId}>
                    <TableCell className="font-mono text-xs font-semibold">
                      {item.containerNumber}
                    </TableCell>
                    <TableCell className="text-xs">
                      <Badge variant="outline" className="text-[10px]">
                        {item.containerSizeType}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs">
                      <button
                        onClick={() => onSelectShipment?.(item.shipmentId)}
                        className="font-medium text-primary hover:underline text-left"
                      >
                        {item.shipmentId}
                      </button>
                      <div className="text-[10px] text-muted-foreground">
                        Inv: {item.invoiceNumber || '—'} {item.blAwbNumber ? `• BL: ${item.blAwbNumber}` : ''}
                      </div>
                    </TableCell>
                    <TableCell className="text-xs font-medium truncate max-w-[150px]">
                      {item.supplierName}
                    </TableCell>
                    <TableCell className="text-xs">
                      <div className="flex items-center gap-1 font-medium text-purple-700 dark:text-purple-400">
                        <Building2 className="h-3 w-3 shrink-0" />
                        <span className="truncate max-w-[140px]">{item.warehouseName || 'Warehouse'}</span>
                      </div>
                    </TableCell>
                    <TableCell className="text-xs font-mono">
                      {item.gateInDate}
                    </TableCell>
                    <TableCell className="text-xs font-mono">
                      {item.gateOutDate || item.dateOfDelivery ? (
                        <span className="text-emerald-600 dark:text-emerald-400 font-medium">
                          {item.gateOutDate || item.dateOfDelivery}
                        </span>
                      ) : (
                        <span className="text-muted-foreground italic">In Storage</span>
                      )}
                    </TableCell>
                    <TableCell className="text-xs">
                      {getDwellBadge(item.dwellDays, isDelivered)}
                    </TableCell>
                    <TableCell className="text-xs">
                      {item.billableDays > 0 ? (
                        <div>
                          <span className="font-medium text-destructive">
                            {item.billableDays}d @ ₹{item.storageRatePerDay}
                          </span>
                          <div className="text-[10px] font-semibold text-destructive">
                            ₹{item.estimatedCharges.toLocaleString('en-IN', { maximumFractionDigits: 2 })}
                          </div>
                        </div>
                      ) : (
                        <span className="text-[11px] text-emerald-600 font-medium">
                          Within Free Days ({item.freeDaysAllowed}d)
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 text-xs"
                        onClick={() => onSelectShipment?.(item.shipmentId)}
                      >
                        <Truck className="h-3.5 w-3.5 mr-1" />
                        Manage
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
