import * as React from 'react';
import {
  AlertCircle,
  Building2,
  CheckCircle2,
  Clock,
  PackageCheck,
  Plus,
  Ship,
  Trash2,
  Truck,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
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
  ContainerMovementAction,
  Shipment,
  ShipmentContainer,
  ShipmentContainerInput,
  shipmentService,
} from '@/services/shipment-invoice-service';
import { toast } from 'sonner';

interface ContainerMovementDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  shipment: Shipment | null;
  onSuccess?: () => void;
}

export function ContainerMovementDialog({
  open,
  onOpenChange,
  shipment,
  onSuccess,
}: ContainerMovementDialogProps) {
  const [containers, setContainers] = React.useState<ShipmentContainer[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [selectedIds, setSelectedIds] = React.useState<string[]>([]);
  const [actionTab, setActionTab] = React.useState<'direct' | 'to-wh' | 'from-wh'>('direct');
  const [submitting, setSubmitting] = React.useState(false);

  // Direct Delivery Form State
  const [directDate, setDirectDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [directVehicle, setDirectVehicle] = React.useState('');
  const [directTransporter, setDirectTransporter] = React.useState('');
  const [directLR, setDirectLR] = React.useState('');
  const [directRemarks, setDirectRemarks] = React.useState('');

  // Move to Warehouse Form State
  const [whName, setWhName] = React.useState('Main CFS Warehouse');
  const [whInDate, setWhInDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [whVehicle, setWhVehicle] = React.useState('');
  const [whTransporter, setWhTransporter] = React.useState('');
  const [whRemarks, setWhRemarks] = React.useState('');

  // Warehouse Dispatch Form State
  const [whOutDate, setWhOutDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [whDeliveryDate, setWhDeliveryDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [whOutVehicle, setWhOutVehicle] = React.useState('');
  const [whOutTransporter, setWhOutTransporter] = React.useState('');
  const [whOutLR, setWhOutLR] = React.useState('');
  const [whOutRemarks, setWhOutRemarks] = React.useState('');

  // Add Container Inline State
  const [showAddContainer, setShowAddContainer] = React.useState(false);
  const [newContainerNum, setNewContainerNum] = React.useState('');
  const [newContainerSize, setNewContainerSize] = React.useState('40FT');
  const [newSealNumber, setNewSealNumber] = React.useState('');
  const [newPortDemurrageFreeDays, setNewPortDemurrageFreeDays] = React.useState('3');
  const [newPortDemurrageRate, setNewPortDemurrageRate] = React.useState('500');
  const [newCfsGroundRentFreeDays, setNewCfsGroundRentFreeDays] = React.useState('3');
  const [newCfsGroundRentRate, setNewCfsGroundRentRate] = React.useState('300');
  const [newDetentionFreeDays, setNewDetentionFreeDays] = React.useState('14');
  const [newDetentionRate, setNewDetentionRate] = React.useState('1000');
  const [newWhStorageRate, setNewWhStorageRate] = React.useState('500');

  const loadContainers = React.useCallback(async () => {
    if (!shipment?.id) return;
    setLoading(true);
    try {
      const list = await shipmentService.listContainers(shipment.id);
      setContainers(list ?? []);
      // Reset selections
      setSelectedIds([]);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [shipment?.id]);

  React.useEffect(() => {
    if (open && shipment?.id) {
      void loadContainers();
    }
  }, [open, shipment?.id, loadContainers]);

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]
    );
  };

  const selectAllEligible = () => {
    if (actionTab === 'from-wh') {
      const eligible = containers.filter((c) => c.deliveryStatus === 'IN_WAREHOUSE').map((c) => c.id);
      setSelectedIds(eligible);
    } else {
      const eligible = containers.filter((c) => c.deliveryStatus !== 'DELIVERED').map((c) => c.id);
      setSelectedIds(eligible);
    }
  };

  const handleAddContainer = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!shipment?.id) return;
    const num = newContainerNum.trim().toUpperCase();
    if (!num) {
      toast.error('Container number is required');
      return;
    }
    try {
      const portFree = parseInt(newPortDemurrageFreeDays, 10) || 0;
      const portRate = parseFloat(newPortDemurrageRate) || 0;
      const cfsFree = parseInt(newCfsGroundRentFreeDays, 10) || 0;
      const cfsRate = parseFloat(newCfsGroundRentRate) || 0;
      const detentionFree = parseInt(newDetentionFreeDays, 10) || 14;
      const detentionRate = parseFloat(newDetentionRate) || 0;
      const whRate = parseFloat(newWhStorageRate) || 0;

      const payload: ShipmentContainerInput = {
        shipmentId: shipment.id,
        containerNumber: num,
        containerSizeType: newContainerSize,
        sealNumber: newSealNumber.trim(),
        routeType: 'DIRECT',
        freeDaysAllowed: portFree,
        storageRatePerDay: portRate,
        portDemurrageFreeDays: portFree,
        portDemurrageRatePerDay: portRate,
        portDemurrageAmount: 0,
        cfsGroundRentFreeDays: cfsFree,
        cfsGroundRentRatePerDay: cfsRate,
        cfsGroundRentAmount: 0,
        shippingLineDetentionFreeDays: detentionFree,
        shippingLineDetentionRatePerDay: detentionRate,
        shippingLineDetentionAmount: 0,
        warehouseStorageDays: 0,
        warehouseStorageRatePerDay: whRate,
        warehouseStorageAmount: 0,
        deliveryStatus: 'PENDING',
        dateOfDelivery: '',
        warehouseName: '',
        gateInDate: '',
        gateOutDate: '',
        transporterName: '',
        vehicleNumber: '',
        lrNumber: '',
        emptyReturnDate: '',
        remarks: '',
      };
      await shipmentService.createContainer(payload);
      toast.success(`Container ${num} added successfully`);
      setNewContainerNum('');
      setNewSealNumber('');
      setShowAddContainer(false);
      await loadContainers();
      onSuccess?.();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleDeleteContainer = async (c: ShipmentContainer) => {
    if (!confirm(`Are you sure you want to remove container ${c.containerNumber}?`)) return;
    try {
      await shipmentService.deleteContainer(c.id);
      toast.success(`Container ${c.containerNumber} removed`);
      await loadContainers();
      onSuccess?.();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleExecuteAction = async () => {
    if (!shipment?.id) return;
    if (selectedIds.length === 0) {
      toast.error('Please select at least one container to apply this action');
      return;
    }

    setSubmitting(true);
    try {
      let action: ContainerMovementAction;

      if (actionTab === 'direct') {
        action = {
          shipmentId: shipment.id,
          containerIds: selectedIds,
          actionType: 'DIRECT_DELIVER',
          dateOfDelivery: directDate,
          vehicleNumber: directVehicle.trim(),
          transporterName: directTransporter.trim(),
          lrNumber: directLR.trim(),
          remarks: directRemarks.trim(),
        };
      } else if (actionTab === 'to-wh') {
        action = {
          shipmentId: shipment.id,
          containerIds: selectedIds,
          actionType: 'MOVE_TO_WAREHOUSE',
          warehouseName: whName.trim() || 'Main CFS Warehouse',
          gateInDate: whInDate,
          vehicleNumber: whVehicle.trim(),
          transporterName: whTransporter.trim(),
          remarks: whRemarks.trim(),
        };
      } else {
        action = {
          shipmentId: shipment.id,
          containerIds: selectedIds,
          actionType: 'DISPATCH_FROM_WAREHOUSE',
          gateOutDate: whOutDate,
          dateOfDelivery: whDeliveryDate || whOutDate,
          vehicleNumber: whOutVehicle.trim(),
          transporterName: whOutTransporter.trim(),
          lrNumber: whOutLR.trim(),
          remarks: whOutRemarks.trim(),
        };
      }

      await shipmentService.processContainerMovement(action);
      toast.success(`Processed movement for ${selectedIds.length} container(s)`);
      await loadContainers();
      onSuccess?.();
      // If all delivered or finished, can close dialog or stay to view
      if (actionTab === 'direct' || actionTab === 'from-wh') {
        const refreshed = await shipmentService.listContainers(shipment.id);
        const pending = refreshed.filter((c) => c.deliveryStatus !== 'DELIVERED');
        if (pending.length === 0) {
          onOpenChange(false);
        }
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const deliveredCount = containers.filter((c) => c.deliveryStatus === 'DELIVERED').length;
  const inWhCount = containers.filter((c) => c.deliveryStatus === 'IN_WAREHOUSE').length;
  const pendingCount = containers.filter((c) => c.deliveryStatus === 'PENDING').length;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center justify-between">
            <div>
              <DialogTitle className="flex items-center gap-2 text-lg">
                <Truck className="h-5 w-5 text-primary" />
                Staged Container & Delivery Manager
              </DialogTitle>
              <DialogDescription>
                Shipment {shipment?.id} • Invoice {shipment?.invoiceNumber} ({shipment?.supplierName})
              </DialogDescription>
            </div>
            <div className="flex gap-2">
              <Badge variant="outline" className="text-xs">
                {containers.length} Total Containers
              </Badge>
              {deliveredCount > 0 && (
                <Badge variant="success" className="text-xs">
                  {deliveredCount} Delivered
                </Badge>
              )}
              {inWhCount > 0 && (
                <Badge variant="secondary" className="text-xs bg-purple-100 text-purple-800 dark:bg-purple-950 dark:text-purple-300">
                  {inWhCount} in Warehouse
                </Badge>
              )}
              {pendingCount > 0 && (
                <Badge variant="outline" className="text-xs">
                  {pendingCount} Pending
                </Badge>
              )}
            </div>
          </div>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Container List Table */}
          <div className="rounded-md border border-border">
            <div className="flex items-center justify-between p-2.5 bg-muted/40 border-b border-border">
              <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Container Inventory ({containers.length})
              </span>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-7 text-xs"
                  onClick={selectAllEligible}
                >
                  Select Active ({actionTab === 'from-wh' ? inWhCount : containers.length - deliveredCount})
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="h-7 text-xs"
                  onClick={() => setShowAddContainer(!showAddContainer)}
                >
                  <Plus className="h-3.5 w-3.5 mr-1" />
                  {showAddContainer ? 'Cancel' : 'Add Container'}
                </Button>
              </div>
            </div>

            {showAddContainer && (
              <form onSubmit={handleAddContainer} className="p-3 bg-muted/20 border-b border-border space-y-3">
                <div className="grid grid-cols-1 sm:grid-cols-4 gap-2">
                  <div>
                    <Label htmlFor="newContNum" className="text-xs">Container No *</Label>
                    <Input
                      id="newContNum"
                      placeholder="e.g. MSCU-9988776"
                      className="h-8 text-xs font-mono"
                      value={newContainerNum}
                      onChange={(e) => setNewContainerNum(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="newContSize" className="text-xs">Size / Type</Label>
                    <Select value={newContainerSize} onValueChange={setNewContainerSize}>
                      <SelectTrigger id="newContSize" className="h-8 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="40FT">40 FT High Cube</SelectItem>
                        <SelectItem value="20FT">20 FT Standard</SelectItem>
                        <SelectItem value="45FT">45 FT High Cube</SelectItem>
                        <SelectItem value="LCL">LCL Cargo Lot</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div>
                    <Label htmlFor="newSeal" className="text-xs">Seal Number</Label>
                    <Input
                      id="newSeal"
                      placeholder="Optional seal"
                      className="h-8 text-xs font-mono"
                      value={newSealNumber}
                      onChange={(e) => setNewSealNumber(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="newWhRate" className="text-xs">WH Storage Rate (₹/d)</Label>
                    <Input
                      id="newWhRate"
                      type="number"
                      min="0"
                      className="h-8 text-xs"
                      value={newWhStorageRate}
                      onChange={(e) => setNewWhStorageRate(e.target.value)}
                    />
                  </div>
                </div>

                <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 pt-1 border-t border-border/50">
                  <div className="rounded p-2 bg-background border border-border/60 space-y-1">
                    <span className="text-[11px] font-semibold text-muted-foreground">Port Demurrage</span>
                    <div className="flex gap-1.5">
                      <div className="w-1/2">
                        <Label className="text-[10px]">Free Days</Label>
                        <Input
                          type="number"
                          min="0"
                          className="h-7 text-xs"
                          value={newPortDemurrageFreeDays}
                          onChange={(e) => setNewPortDemurrageFreeDays(e.target.value)}
                        />
                      </div>
                      <div className="w-1/2">
                        <Label className="text-[10px]">Rate (₹/d)</Label>
                        <Input
                          type="number"
                          min="0"
                          className="h-7 text-xs"
                          value={newPortDemurrageRate}
                          onChange={(e) => setNewPortDemurrageRate(e.target.value)}
                        />
                      </div>
                    </div>
                  </div>

                  <div className="rounded p-2 bg-background border border-border/60 space-y-1">
                    <span className="text-[11px] font-semibold text-muted-foreground">Line Detention</span>
                    <div className="flex gap-1.5">
                      <div className="w-1/2">
                        <Label className="text-[10px]">Free Days</Label>
                        <Input
                          type="number"
                          min="0"
                          className="h-7 text-xs"
                          value={newDetentionFreeDays}
                          onChange={(e) => setNewDetentionFreeDays(e.target.value)}
                        />
                      </div>
                      <div className="w-1/2">
                        <Label className="text-[10px]">Rate (₹/d)</Label>
                        <Input
                          type="number"
                          min="0"
                          className="h-7 text-xs"
                          value={newDetentionRate}
                          onChange={(e) => setNewDetentionRate(e.target.value)}
                        />
                      </div>
                    </div>
                  </div>

                  <div className="rounded p-2 bg-background border border-border/60 space-y-1">
                    <span className="text-[11px] font-semibold text-muted-foreground">CFS Ground Rent</span>
                    <div className="flex gap-1.5">
                      <div className="w-1/2">
                        <Label className="text-[10px]">Free Days</Label>
                        <Input
                          type="number"
                          min="0"
                          className="h-7 text-xs"
                          value={newCfsGroundRentFreeDays}
                          onChange={(e) => setNewCfsGroundRentFreeDays(e.target.value)}
                        />
                      </div>
                      <div className="w-1/2">
                        <Label className="text-[10px]">Rate (₹/d)</Label>
                        <Input
                          type="number"
                          min="0"
                          className="h-7 text-xs"
                          value={newCfsGroundRentRate}
                          onChange={(e) => setNewCfsGroundRentRate(e.target.value)}
                        />
                      </div>
                    </div>
                  </div>
                </div>

                <div className="flex justify-end gap-2">
                  <Button type="button" size="sm" variant="ghost" onClick={() => setShowAddContainer(false)}>
                    Cancel
                  </Button>
                  <Button type="submit" size="sm">
                    Save Container
                  </Button>
                </div>
              </form>
            )}

            <Table>
              <TableHeader>
                <TableRow className="text-xs">
                  <TableHead className="w-10 text-center">Select</TableHead>
                  <TableHead>Container No</TableHead>
                  <TableHead>Size</TableHead>
                  <TableHead>Route / Location</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Warehouse Dwell</TableHead>
                  <TableHead>Demurrage &amp; Detention</TableHead>
                  <TableHead>Delivery Date</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={9} className="text-center py-4 text-xs text-muted-foreground">
                      Loading containers...
                    </TableCell>
                  </TableRow>
                ) : containers.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="text-center py-4 text-xs text-muted-foreground">
                      No containers recorded yet. Click &quot;Add Container&quot; above.
                    </TableCell>
                  </TableRow>
                ) : (
                  containers.map((c) => {
                    const isSelected = selectedIds.includes(c.id);
                    const isDelivered = c.deliveryStatus === 'DELIVERED';
                    const isInWh = c.deliveryStatus === 'IN_WAREHOUSE';

                    return (
                      <TableRow
                        key={c.id}
                        className={isSelected ? 'bg-primary/5 dark:bg-primary/10' : undefined}
                      >
                        <TableCell className="text-center">
                          <input
                            type="checkbox"
                            checked={isSelected}
                            onChange={() => toggleSelect(c.id)}
                            className="rounded border-border"
                          />
                        </TableCell>
                        <TableCell className="font-mono text-xs font-medium">
                          {c.containerNumber}
                        </TableCell>
                        <TableCell className="text-xs">
                          <Badge variant="outline" className="text-[10px]">
                            {c.containerSizeType}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs">
                          {c.routeType === 'WAREHOUSE' || c.warehouseName ? (
                            <div className="flex items-center gap-1 text-purple-700 dark:text-purple-400">
                              <Building2 className="h-3 w-3" />
                              <span>{c.warehouseName || 'Warehouse'}</span>
                            </div>
                          ) : (
                            <div className="flex items-center gap-1 text-muted-foreground">
                              <Ship className="h-3 w-3" />
                              <span>Direct Port to Factory</span>
                            </div>
                          )}
                        </TableCell>
                        <TableCell className="text-xs">
                          {isDelivered ? (
                            <Badge variant="success" className="gap-1 text-[10px]">
                              <CheckCircle2 className="h-2.5 w-2.5" /> Delivered
                            </Badge>
                          ) : isInWh ? (
                            <Badge variant="secondary" className="gap-1 text-[10px] bg-purple-100 text-purple-800 dark:bg-purple-950 dark:text-purple-300">
                              <Clock className="h-2.5 w-2.5" /> In Warehouse
                            </Badge>
                          ) : (
                            <Badge variant="outline" className="text-[10px]">
                              Pending
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-xs">
                          {c.gateInDate ? (
                            <div>
                              <div className="font-medium text-xs">
                                {c.dwellDays} {c.dwellDays === 1 ? 'day' : 'days'}
                                {c.billableDays > 0 && (
                                  <span className="text-destructive font-semibold ml-1">
                                    ({c.billableDays} billable)
                                  </span>
                                )}
                              </div>
                              <span className="text-[10px] text-muted-foreground">
                                In: {c.gateInDate} {c.gateOutDate ? `• Out: ${c.gateOutDate}` : ''}
                              </span>
                            </div>
                          ) : (
                            <span className="text-muted-foreground text-[11px]">—</span>
                          )}
                        </TableCell>
                        <TableCell className="text-xs">
                          <div className="space-y-0.5 text-[11px]">
                            <div className="flex justify-between gap-1 text-muted-foreground">
                              <span>Port:</span>
                              <span className={c.portDemurrageAmount > 0 ? 'text-destructive font-semibold' : ''}>
                                ₹{(c.portDemurrageAmount || 0).toLocaleString()}
                              </span>
                            </div>
                            <div className="flex justify-between gap-1 text-muted-foreground">
                              <span>Detention:</span>
                              <span className={c.shippingLineDetentionAmount > 0 ? 'text-destructive font-semibold' : ''}>
                                ₹{(c.shippingLineDetentionAmount || 0).toLocaleString()}
                              </span>
                            </div>
                            {(c.cfsGroundRentAmount > 0 || c.warehouseStorageAmount > 0) && (
                              <div className="flex justify-between gap-1 text-[10px] text-muted-foreground">
                                <span>CFS/WH:</span>
                                <span>
                                  ₹{((c.cfsGroundRentAmount || 0) + (c.warehouseStorageAmount || c.storageCost || 0)).toLocaleString()}
                                </span>
                              </div>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="text-xs">
                          {c.dateOfDelivery ? (
                            <span className="text-emerald-600 dark:text-emerald-400 font-medium">
                              {c.dateOfDelivery}
                            </span>
                          ) : (
                            <span className="text-muted-foreground text-[11px]">—</span>
                          )}
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                            onClick={() => void handleDeleteContainer(c)}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </div>

          {/* Action Tabs for Staged Delivery Operations */}
          <div className="rounded-lg border border-border p-3 bg-card shadow-xs">
            <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">
              Apply Operation on Selected Containers ({selectedIds.length} chosen)
            </h4>

            <Tabs value={actionTab} onValueChange={(v) => setActionTab(v as typeof actionTab)}>
              <TabsList className="grid grid-cols-3 mb-3">
                <TabsTrigger value="direct" onClick={() => setActionTab('direct')} className="gap-1.5 text-xs">
                  <PackageCheck className="h-3.5 w-3.5 text-emerald-600" />
                  1. Direct Delivery
                </TabsTrigger>
                <TabsTrigger value="to-wh" onClick={() => setActionTab('to-wh')} className="gap-1.5 text-xs">
                  <Building2 className="h-3.5 w-3.5 text-purple-600" />
                  2. Move to Warehouse
                </TabsTrigger>
                <TabsTrigger value="from-wh" onClick={() => setActionTab('from-wh')} className="gap-1.5 text-xs">
                  <Truck className="h-3.5 w-3.5 text-blue-600" />
                  3. Warehouse Dispatch
                </TabsTrigger>
              </TabsList>

              {/* Mode 1: Direct Delivery */}
              <TabsContent value="direct" className="space-y-3 mt-0">
                <div className="p-2.5 rounded bg-emerald-50/50 dark:bg-emerald-950/20 border border-emerald-200 dark:border-emerald-900/40 text-xs text-emerald-800 dark:text-emerald-300">
                  Bypasses warehouse storage and delivers selected container(s) straight to factory gate.
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div>
                    <Label htmlFor="directDate" className="text-xs">Factory Delivery Date *</Label>
                    <Input
                      id="directDate"
                      type="date"
                      className="h-8 text-xs"
                      value={directDate}
                      onChange={(e) => setDirectDate(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="directVehicle" className="text-xs">Trailer / Vehicle No</Label>
                    <Input
                      id="directVehicle"
                      placeholder="e.g. KA-01-AB-1234"
                      className="h-8 text-xs font-mono"
                      value={directVehicle}
                      onChange={(e) => setDirectVehicle(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="directTransporter" className="text-xs">Transporter Name</Label>
                    <Input
                      id="directTransporter"
                      placeholder="e.g. Safe Logistics"
                      className="h-8 text-xs"
                      value={directTransporter}
                      onChange={(e) => setDirectTransporter(e.target.value)}
                    />
                  </div>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor="directLR" className="text-xs">LR / Bilty Number</Label>
                    <Input
                      id="directLR"
                      placeholder="e.g. LR-90812"
                      className="h-8 text-xs"
                      value={directLR}
                      onChange={(e) => setDirectLR(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="directRemarks" className="text-xs">Remarks</Label>
                    <Input
                      id="directRemarks"
                      placeholder="e.g. Direct delivery container 1"
                      className="h-8 text-xs"
                      value={directRemarks}
                      onChange={(e) => setDirectRemarks(e.target.value)}
                    />
                  </div>
                </div>
              </TabsContent>

              {/* Mode 2: Move to Warehouse */}
              <TabsContent value="to-wh" className="space-y-3 mt-0">
                <div className="p-2.5 rounded bg-purple-50/50 dark:bg-purple-950/20 border border-purple-200 dark:border-purple-900/40 text-xs text-purple-800 dark:text-purple-300">
                  Moves container(s) to a buffer or 3PL warehouse to avoid container detention charges. Begins tracking storage dwell days.
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
                  <div>
                    <Label htmlFor="whNameInput" className="text-xs">Warehouse Facility Name *</Label>
                    <Input
                      id="whNameInput"
                      placeholder="e.g. Central Bonded CFS / 3PL Warehouse A"
                      className="h-8 text-xs"
                      value={whName}
                      onChange={(e) => setWhName(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="whInDate" className="text-xs">Gate-In Date *</Label>
                    <Input
                      id="whInDate"
                      type="date"
                      className="h-8 text-xs"
                      value={whInDate}
                      onChange={(e) => setWhInDate(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="whVehicle" className="text-xs">Shuttle Vehicle No</Label>
                    <Input
                      id="whVehicle"
                      placeholder="e.g. MH-04-WH-8888"
                      className="h-8 text-xs font-mono"
                      value={whVehicle}
                      onChange={(e) => setWhVehicle(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="whTransporter" className="text-xs">Transporter</Label>
                    <Input
                      id="whTransporter"
                      placeholder="e.g. CFS Shuttle"
                      className="h-8 text-xs"
                      value={whTransporter}
                      onChange={(e) => setWhTransporter(e.target.value)}
                    />
                  </div>
                </div>
                <div>
                  <Label htmlFor="whRemarks" className="text-xs">Storage Notes</Label>
                  <Input
                    id="whRemarks"
                    placeholder="e.g. Stored due to plant unloading bay congestion"
                    className="h-8 text-xs"
                    value={whRemarks}
                    onChange={(e) => setWhRemarks(e.target.value)}
                  />
                </div>
              </TabsContent>

              {/* Mode 3: Warehouse Dispatch */}
              <TabsContent value="from-wh" className="space-y-3 mt-0">
                <div className="p-2.5 rounded bg-blue-50/50 dark:bg-blue-950/20 border border-blue-200 dark:border-blue-900/40 text-xs text-blue-800 dark:text-blue-300">
                  Dispatches container(s) from warehouse to factory. Finalizes warehouse storage dwell days and calculates billable days.
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
                  <div>
                    <Label htmlFor="whOutDate" className="text-xs">Warehouse Gate-Out Date *</Label>
                    <Input
                      id="whOutDate"
                      type="date"
                      className="h-8 text-xs"
                      value={whOutDate}
                      onChange={(e) => {
                        setWhOutDate(e.target.value);
                        if (!whDeliveryDate) setWhDeliveryDate(e.target.value);
                      }}
                    />
                  </div>
                  <div>
                    <Label htmlFor="whDeliveryDate" className="text-xs">Factory Gate Delivery Date</Label>
                    <Input
                      id="whDeliveryDate"
                      type="date"
                      className="h-8 text-xs"
                      value={whDeliveryDate}
                      onChange={(e) => setWhDeliveryDate(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="whOutVehicle" className="text-xs">Dispatch Truck / Trailer No</Label>
                    <Input
                      id="whOutVehicle"
                      placeholder="e.g. DL-01-TR-5555"
                      className="h-8 text-xs font-mono"
                      value={whOutVehicle}
                      onChange={(e) => setWhOutVehicle(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="whOutTransporter" className="text-xs">Transporter</Label>
                    <Input
                      id="whOutTransporter"
                      placeholder="e.g. Main Transport"
                      className="h-8 text-xs"
                      value={whOutTransporter}
                      onChange={(e) => setWhOutTransporter(e.target.value)}
                    />
                  </div>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor="whOutLR" className="text-xs">Gate Pass / LR No</Label>
                    <Input
                      id="whOutLR"
                      placeholder="e.g. GP-88219"
                      className="h-8 text-xs"
                      value={whOutLR}
                      onChange={(e) => setWhOutLR(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="whOutRemarks" className="text-xs">Dispatch Notes</Label>
                    <Input
                      id="whOutRemarks"
                      placeholder="e.g. Stage 2 delivery from warehouse"
                      className="h-8 text-xs"
                      value={whOutRemarks}
                      onChange={(e) => setWhOutRemarks(e.target.value)}
                    />
                  </div>
                </div>
              </TabsContent>
            </Tabs>
          </div>
        </div>

        <DialogFooter className="flex items-center justify-between sm:justify-between border-t border-border pt-3">
          <div className="text-xs text-muted-foreground">
            {selectedIds.length === 0 ? (
              <span className="flex items-center gap-1">
                <AlertCircle className="h-3.5 w-3.5 text-amber-500" />
                Select at least 1 container above to apply action
              </span>
            ) : (
              <span className="font-medium text-foreground">
                Ready to update {selectedIds.length} container(s)
              </span>
            )}
          </div>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
              Close
            </Button>
            <Button
              size="sm"
              disabled={selectedIds.length === 0 || submitting}
              onClick={() => void handleExecuteAction()}
            >
              {submitting ? 'Updating...' : `Confirm ${actionTab === 'direct' ? 'Direct Delivery' : actionTab === 'to-wh' ? 'Move to Warehouse' : 'Dispatch & Deliver'}`}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
