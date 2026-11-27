import { History, Pencil, Plus, RefreshCw, Settings2, ShieldAlert, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
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
import { TabsContent } from '@/components/ui/tabs';
import {
  expenseService,
  formatPaiseINR,
  type ExpenseAllocationRecord,
  type ExpenseType,
  type ExpenseTypeInput,
  type ServiceProvider,
  type ServiceProviderInput,
} from '@/services/expense-service';
import type { Shipment } from '@/services/shipment-invoice-service';

interface ExpenseCatalogAndLedgerTabsProps {
  canWrite: boolean;
  canDelete: boolean;
  providers: ServiceProvider[];
  types: ExpenseType[];
  shipments: Shipment[];
  ledgerRecords: ExpenseAllocationRecord[];
  ledgerLoading: boolean;
  ledgerShipmentFilter: string;
  setLedgerShipmentFilter: (val: string) => void;
  loadLedger: () => Promise<void>;
  loadAll: () => Promise<void>;
  setManageSpOpen: (open: boolean) => void;
  setManageEtOpen: (open: boolean) => void;
  setTargetLineIdx: (idx: number | null) => void;
  spOpen: boolean;
  setSpOpen: (open: boolean) => void;
  editingSp: ServiceProvider | null;
  setEditingSp: (sp: ServiceProvider | null) => void;
  spForm: ServiceProviderInput;
  setSpForm: (form: ServiceProviderInput) => void;
  handleSaveProvider: () => Promise<void>;
  etOpen: boolean;
  setEtOpen: (open: boolean) => void;
  editingEt: ExpenseType | null;
  setEditingEt: (et: ExpenseType | null) => void;
  etForm: ExpenseTypeInput;
  setEtForm: (form: ExpenseTypeInput) => void;
  handleSaveType: () => Promise<void>;
}

export function ExpenseCatalogAndLedgerTabs({
  canWrite,
  canDelete,
  providers,
  types,
  shipments,
  ledgerRecords,
  ledgerLoading,
  ledgerShipmentFilter,
  setLedgerShipmentFilter,
  loadLedger,
  loadAll,
  setManageSpOpen,
  setManageEtOpen,
  setTargetLineIdx,
  spOpen,
  setSpOpen,
  editingSp,
  setEditingSp,
  spForm,
  setSpForm,
  handleSaveProvider,
  etOpen,
  setEtOpen,
  editingEt,
  setEditingEt,
  etForm,
  setEtForm,
  handleSaveType,
}: ExpenseCatalogAndLedgerTabsProps) {
  return (
    <>
      <TabsContent value="providers">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle className="text-base">Service Providers (CHA / Forwarders)</CardTitle>
              <CardDescription>Manage GSTIN and contact details for expense vendors.</CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Button size="sm" variant="outline" onClick={() => setManageSpOpen(true)}>
                <Settings2 className="h-4 w-4 mr-1" />
                Manage Providers
              </Button>
              {canWrite && (
                <Button
                  size="sm"
                  onClick={() => {
                    setEditingSp(null);
                    setSpForm({
                      name: '',
                      gstin: '',
                      state: '',
                      contactPerson: '',
                      contactEmail: '',
                      contactPhone: '',
                    });
                    setSpOpen(true);
                  }}
                >
                  <Plus className="h-4 w-4 mr-1" />
                  Add Provider
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Provider Name</TableHead>
                  <TableHead>GSTIN</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead>Contact</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {providers.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell className="font-mono text-xs">{p.gstin || '—'}</TableCell>
                    <TableCell>{p.state || '—'}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {p.contactPerson || p.contactEmail || p.contactPhone || '—'}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        {canWrite && (
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => {
                              setEditingSp(p);
                              setSpForm({
                                name: p.name,
                                gstin: p.gstin,
                                state: p.state,
                                contactPerson: p.contactPerson,
                                contactEmail: p.contactEmail,
                                contactPhone: p.contactPhone,
                              });
                              setSpOpen(true);
                            }}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                        )}
                        {canDelete && (
                          <Button
                            size="sm"
                            variant="ghost"
                            aria-label={`Delete ${p.name}`}
                            onClick={() =>
                              void expenseService
                                .deleteServiceProvider(p.id)
                                .then(loadAll)
                                .then(() => toast.success(`Service provider "${p.name}" removed`))
                                .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
                            }
                          >
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="types">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle className="text-base">Expense Types &amp; Statutory Tax Defaults</CardTitle>
              <CardDescription>Default CGST/SGST/IGST/TDS rates stored in basis points (100 bp = 1%).</CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  setTargetLineIdx(null);
                  setManageEtOpen(true);
                }}
              >
                <Settings2 className="h-4 w-4 mr-1" />
                Manage Types
              </Button>
              {canWrite && (
                <Button
                  size="sm"
                  onClick={() => {
                    setEditingEt(null);
                    setEtForm({
                      name: '',
                      defaultCgstRateBp: 900,
                      defaultSgstRateBp: 900,
                      defaultIgstRateBp: 0,
                      defaultTdsRateBp: 200,
                      isActive: true,
                    });
                    setEtOpen(true);
                  }}
                >
                  <Plus className="h-4 w-4 mr-1" />
                  Add Expense Type
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>CGST</TableHead>
                  <TableHead>SGST</TableHead>
                  <TableHead>IGST</TableHead>
                  <TableHead>TDS</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {types.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell className="font-medium">{t.name}</TableCell>
                    <TableCell className="font-mono text-xs">{(t.defaultCgstRateBp / 100).toFixed(2)}%</TableCell>
                    <TableCell className="font-mono text-xs">{(t.defaultSgstRateBp / 100).toFixed(2)}%</TableCell>
                    <TableCell className="font-mono text-xs">{(t.defaultIgstRateBp / 100).toFixed(2)}%</TableCell>
                    <TableCell className="font-mono text-xs">{(t.defaultTdsRateBp / 100).toFixed(2)}%</TableCell>
                    <TableCell>
                      <Badge variant={t.isActive ? 'success' : 'secondary'}>
                        {t.isActive ? 'Active' : 'Inactive'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        {canWrite && (
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => {
                              setEditingEt(t);
                              setEtForm({
                                name: t.name,
                                defaultCgstRateBp: t.defaultCgstRateBp,
                                defaultSgstRateBp: t.defaultSgstRateBp,
                                defaultIgstRateBp: t.defaultIgstRateBp,
                                defaultTdsRateBp: t.defaultTdsRateBp,
                                isActive: t.isActive,
                              });
                              setEtOpen(true);
                            }}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                        )}
                        {canDelete && (
                          <Button
                            size="sm"
                            variant="ghost"
                            aria-label={`Delete ${t.name}`}
                            onClick={() =>
                              void expenseService
                                .deleteExpenseType(t.id)
                                .then(loadAll)
                                .then(() => toast.success(`Expense type "${t.name}" removed`))
                                .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
                            }
                          >
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="ledger" className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-3">
            <div className="w-56">
              <Select value={ledgerShipmentFilter} onValueChange={setLedgerShipmentFilter}>
                <SelectTrigger className="h-9 text-xs">
                  <SelectValue placeholder="All Shipments" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">All Shipments</SelectItem>
                  {shipments.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.id} ({s.invoiceNumber})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button size="sm" variant="outline" onClick={() => void loadLedger()} disabled={ledgerLoading}>
              <RefreshCw className={`h-4 w-4 mr-1 ${ledgerLoading ? 'animate-spin' : ''}`} />
              Refresh Ledger
            </Button>
          </div>
          <div className="text-xs text-muted-foreground">
            {ledgerRecords.length} immutable allocation records
          </div>
        </div>

        <div className="rounded-lg border border-primary/20 bg-primary/5 p-4 text-xs space-y-1">
          <div className="flex items-center gap-2 font-semibold text-primary">
            <ShieldAlert className="h-4 w-4" />
            <span>Immutable Statutory Landed Cost Allocation Ledger</span>
          </div>
          <p className="text-muted-foreground">
            Records are appended on every expense invoice creation, modification, and landed cost computation.
            Every line item allocation is protected by database triggers against tampering or deletion.
          </p>
        </div>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <History className="h-4 w-4" />
              Historical Expense Allocations
            </CardTitle>
            <CardDescription>
              Auditable pro-rata distribution of commercial invoice costs, duty, freight, insurance, and CHA expenses across line items.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow className="text-xs">
                  <TableHead>Timestamp</TableHead>
                  <TableHead>Shipment / BOE</TableHead>
                  <TableHead>Reason</TableHead>
                  <TableHead>Expense Category</TableHead>
                  <TableHead>Method</TableHead>
                  <TableHead>Part / Line Item</TableHead>
                  <TableHead className="text-right">Allocation %</TableHead>
                  <TableHead className="text-right">Allocated Amount</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {ledgerLoading ? (
                  <TableRow>
                    <TableCell colSpan={8} className="text-center py-6 text-xs text-muted-foreground">
                      Loading allocation history...
                    </TableCell>
                  </TableRow>
                ) : ledgerRecords.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={8} className="text-center py-6 text-xs text-muted-foreground">
                      No expense allocation records found. Add an expense invoice to a shipment to generate allocations.
                    </TableCell>
                  </TableRow>
                ) : (
                  ledgerRecords.map((rec) => (
                    <TableRow key={rec.id} className="text-xs">
                      <TableCell className="font-mono text-[11px] text-muted-foreground">
                        {rec.calculatedAt}
                      </TableCell>
                      <TableCell>
                        <div className="font-medium">{rec.shipmentId}</div>
                        {rec.boeId && (
                          <div className="text-[10px] text-muted-foreground">BOE: {rec.boeId}</div>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-[10px] font-mono">
                          {rec.recalculationReason}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-medium">{rec.expenseCategory}</TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="text-[10px]">
                          {rec.allocationMethod}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-[11px]">
                        {rec.partNumber || rec.itemId || '—'}
                      </TableCell>
                      <TableCell className="text-right font-mono">
                        {rec.allocationPercentage ? `${rec.allocationPercentage.toFixed(2)}%` : '—'}
                      </TableCell>
                      <TableCell className="text-right font-mono font-semibold text-primary">
                        ₹{formatPaiseINR(rec.allocatedAmountPaise)}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </TabsContent>

      {/* Service Provider Dialog */}
      <Dialog open={spOpen} onOpenChange={setSpOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingSp ? 'Edit Service Provider' : 'Add Service Provider'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="spName">Name *</Label>
              <Input
                id="spName"
                value={spForm.name}
                onChange={(e) => setSpForm({ ...spForm, name: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label htmlFor="spGstin">GSTIN</Label>
                <Input
                  id="spGstin"
                  value={spForm.gstin}
                  onChange={(e) => setSpForm({ ...spForm, gstin: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="spState">State</Label>
                <Input
                  id="spState"
                  value={spForm.state}
                  onChange={(e) => setSpForm({ ...spForm, state: e.target.value })}
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label htmlFor="spEmail">Contact Email</Label>
                <Input
                  id="spEmail"
                  value={spForm.contactEmail}
                  onChange={(e) => setSpForm({ ...spForm, contactEmail: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="spPhone">Phone</Label>
                <Input
                  id="spPhone"
                  value={spForm.contactPhone}
                  onChange={(e) => setSpForm({ ...spForm, contactPhone: e.target.value })}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSpOpen(false)}>Cancel</Button>
            <Button onClick={() => void handleSaveProvider()}>Save Provider</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Expense Type Dialog */}
      <Dialog open={etOpen} onOpenChange={setEtOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingEt ? 'Edit Expense Type' : 'Add Expense Type'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="etName">Name *</Label>
              <Input
                id="etName"
                value={etForm.name}
                onChange={(e) => setEtForm({ ...etForm, name: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Default CGST (bp)</Label>
                <Input
                  type="number"
                  value={etForm.defaultCgstRateBp}
                  onChange={(e) =>
                    setEtForm({ ...etForm, defaultCgstRateBp: Number.parseInt(e.target.value, 10) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label>Default SGST (bp)</Label>
                <Input
                  type="number"
                  value={etForm.defaultSgstRateBp}
                  onChange={(e) =>
                    setEtForm({ ...etForm, defaultSgstRateBp: Number.parseInt(e.target.value, 10) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label>Default IGST (bp)</Label>
                <Input
                  type="number"
                  value={etForm.defaultIgstRateBp}
                  onChange={(e) =>
                    setEtForm({ ...etForm, defaultIgstRateBp: Number.parseInt(e.target.value, 10) || 0 })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label>Default TDS (bp)</Label>
                <Input
                  type="number"
                  value={etForm.defaultTdsRateBp}
                  onChange={(e) =>
                    setEtForm({ ...etForm, defaultTdsRateBp: Number.parseInt(e.target.value, 10) || 0 })
                  }
                />
              </div>
            </div>
            <div className="flex items-center gap-2 pt-1">
              <Switch
                checked={etForm.isActive}
                onCheckedChange={(v) => setEtForm({ ...etForm, isActive: v })}
              />
              <Label>Active</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEtOpen(false)}>Cancel</Button>
            <Button onClick={() => void handleSaveType()}>Save Expense Type</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
