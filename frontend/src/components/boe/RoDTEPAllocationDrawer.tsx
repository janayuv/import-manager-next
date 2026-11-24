import * as React from 'react';
import { Award, CheckCircle2, Clock, FileCheck, RefreshCw, XCircle } from 'lucide-react';
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
import { rodtepService } from '@/services/rodtep-service';
import type {
  BOEEvalContext,
  RoDTEPAllocation,
  RoDTEPAllocationConfirmInput,
  RoDTEPAllocationItemInput,
  RoDTEPScrip,
} from '@/types/rodtep';

interface RoDTEPAllocationDrawerProps {
  boeId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
}

export function RoDTEPAllocationDrawer({
  boeId,
  open,
  onOpenChange,
  onSuccess,
}: RoDTEPAllocationDrawerProps) {
  const [loading, setLoading] = React.useState(false);
  const [evalCtx, setEvalCtx] = React.useState<BOEEvalContext | null>(null);
  const [portFilter, setPortFilter] = React.useState<string>('ALL');

  // Manual Selection state (strictly NO FIFO): map of scripId -> rupee amount input string
  const [selectedScrips, setSelectedScrips] = React.useState<Record<string, string>>({});
  const [allocationRemarks, setAllocationRemarks] = React.useState('');
  const [submittingAlloc, setSubmittingAlloc] = React.useState(false);

  // Confirmation Dialog State
  const [confirmTarget, setConfirmTarget] = React.useState<RoDTEPAllocation | null>(null);
  const [confirmAmountINR, setConfirmAmountINR] = React.useState('');
  const [confirmIcegateRef, setConfirmIcegateRef] = React.useState('');
  const [confirmChallanNo, setConfirmChallanNo] = React.useState('');
  const [confirmDebitDate, setConfirmDebitDate] = React.useState(
    new Date().toISOString().split('T')[0]
  );
  const [confirmDocRef, setConfirmDocRef] = React.useState('');
  const [confirmRemarks, setConfirmRemarks] = React.useState('');
  const [submittingConfirm, setSubmittingConfirm] = React.useState(false);

  // Closeout Dialog State
  const [closeTarget, setCloseTarget] = React.useState<RoDTEPAllocation | null>(null);
  const [closeReason, setCloseReason] = React.useState('');
  const [submittingClose, setSubmittingClose] = React.useState(false);

  const loadData = React.useCallback(async () => {
    if (!boeId) return;
    setLoading(true);
    try {
      const data = await rodtepService.getBOEEvalContext(boeId);
      setEvalCtx(data);
      // Reset selections
      setSelectedScrips({});
      setAllocationRemarks('');
    } catch (err: any) {
      toast.error('Failed to load BOE duty evaluation: ' + (err?.message || err));
    } finally {
      setLoading(false);
    }
  }, [boeId]);

  React.useEffect(() => {
    if (open) {
      loadData();
    }
  }, [open, loadData]);

  // Filter available scrips by port
  const filteredScrips = React.useMemo(() => {
    if (!evalCtx?.availableScrips) return [];
    if (portFilter === 'ALL') return evalCtx.availableScrips;
    return evalCtx.availableScrips.filter((s) => s.scripPortCode === portFilter);
  }, [evalCtx, portFilter]);

  // Calculate total requested allocation in Paise
  const totalAllocRequestedPaise = React.useMemo(() => {
    let sum = 0;
    for (const [_, valStr] of Object.entries(selectedScrips)) {
      const amt = parseFloat(valStr) || 0;
      if (amt > 0) {
        sum += Math.round(amt * 100);
      }
    }
    return sum;
  }, [selectedScrips]);

  const totalAllocRequestedINR = totalAllocRequestedPaise / 100.0;

  // Validation
  const eligibleUnpaidPaise = evalCtx?.eligibleUnpaidBcdPaise || 0;
  const eligibleUnpaidINR = evalCtx?.eligibleUnpaidBcdInr || 0;
  const isOverBOECap = totalAllocRequestedPaise > eligibleUnpaidPaise;

  // Check individual scrip caps
  const scripExceedsMap = React.useMemo(() => {
    const exceeds: Record<string, boolean> = {};
    if (!evalCtx?.availableScrips) return exceeds;
    for (const scrip of evalCtx.availableScrips) {
      const valStr = selectedScrips[scrip.id];
      if (valStr) {
        const valPaise = Math.round((parseFloat(valStr) || 0) * 100);
        if (valPaise > scrip.netAvailablePaise) {
          exceeds[scrip.id] = true;
        }
      }
    }
    return exceeds;
  }, [evalCtx, selectedScrips]);

  const hasAnyScripExceeded = Object.values(scripExceedsMap).some(Boolean);

  const handleSelectScrip = (scrip: RoDTEPScrip, checked: boolean) => {
    setSelectedScrips((prev) => {
      const next = { ...prev };
      if (!checked) {
        delete next[scrip.id];
      } else {
        // Default to either net available or remaining eligible BOE liability, whichever is smaller
        const currentSum = Object.entries(next).reduce(
          (acc, [id, val]) => (id === scrip.id ? acc : acc + Math.round((parseFloat(val) || 0) * 100)),
          0
        );
        const remainingToAllocate = eligibleUnpaidPaise - currentSum;
        const defaultPaise = Math.max(0, Math.min(scrip.netAvailablePaise, remainingToAllocate));
        next[scrip.id] = (defaultPaise / 100.0).toFixed(2);
      }
      return next;
    });
  };

  const handleScripAmountChange = (scripId: string, value: string) => {
    setSelectedScrips((prev) => ({
      ...prev,
      [scripId]: value,
    }));
  };

  const handleCreateAllocation = async () => {
    if (!boeId) return;
    const items: RoDTEPAllocationItemInput[] = [];
    for (const [scripId, valStr] of Object.entries(selectedScrips)) {
      const val = parseFloat(valStr) || 0;
      if (val > 0) {
        items.push({
          scripId,
          amountPaise: Math.round(val * 100),
        });
      }
    }

    if (items.length === 0) {
      toast.error('Please select at least one scrip and enter a positive allocation amount.');
      return;
    }

    if (isOverBOECap) {
      toast.error(
        `Total requested allocation (₹${totalAllocRequestedINR.toFixed(2)}) exceeds eligible unpaid BCD liability (₹${eligibleUnpaidINR.toFixed(2)}).`
      );
      return;
    }

    if (hasAnyScripExceeded) {
      toast.error('One or more selected scrips exceed their available balances.');
      return;
    }

    setSubmittingAlloc(true);
    try {
      await rodtepService.allocateScrips(boeId, items, allocationRemarks);
      toast.success(
        `Successfully reserved ₹${totalAllocRequestedINR.toFixed(2)} across ${items.length} scrip(s).`
      );
      await loadData();
      onSuccess?.();
    } catch (err: any) {
      toast.error('Allocation failed: ' + (err?.message || err));
    } finally {
      setSubmittingAlloc(false);
    }
  };

  const handleOpenConfirmDialog = (alloc: RoDTEPAllocation) => {
    setConfirmTarget(alloc);
    setConfirmAmountINR(alloc.outstandingInr.toFixed(2));
    setConfirmIcegateRef('');
    setConfirmChallanNo((evalCtx?.boeDetail as any)?.challanNumber || '');
    setConfirmDebitDate(new Date().toISOString().split('T')[0]);
    setConfirmDocRef('');
    setConfirmRemarks('');
  };

  const handleSubmitConfirmDebit = async () => {
    if (!confirmTarget) return;
    const amt = parseFloat(confirmAmountINR) || 0;
    if (amt <= 0) {
      toast.error('Please enter a valid debit amount.');
      return;
    }
    const amtPaise = Math.round(amt * 100);
    if (amtPaise > confirmTarget.outstandingPaise) {
      toast.error(
        `Confirmed amount exceeds outstanding allocation balance (₹${confirmTarget.outstandingInr.toFixed(2)}).`
      );
      return;
    }

    if (!confirmIcegateRef.trim() && !confirmDocRef.trim()) {
      toast.error('Either ICEGATE Transaction Reference or Supporting Document Reference is required.');
      return;
    }

    setSubmittingConfirm(true);
    try {
      const input: RoDTEPAllocationConfirmInput = {
        allocationId: confirmTarget.id,
        amountPaise: amtPaise,
        icegateTransactionRef: confirmIcegateRef.trim(),
        challanNumber: confirmChallanNo.trim(),
        debitDate: confirmDebitDate || '',
        supportingDocRef: confirmDocRef.trim(),
        remarks: confirmRemarks.trim(),
      };
      await rodtepService.confirmActualDebit(input);
      toast.success(`Confirmed ICEGATE debit of ₹${amt.toFixed(2)} recorded in statutory ledger.`);
      setConfirmTarget(null);
      await loadData();
      onSuccess?.();
    } catch (err: any) {
      toast.error('Debit confirmation failed: ' + (err?.message || err));
    } finally {
      setSubmittingConfirm(false);
    }
  };

  const handleOpenCloseDialog = (alloc: RoDTEPAllocation) => {
    setCloseTarget(alloc);
    setCloseReason('Customs duty settled via cash challan');
  };

  const handleSubmitCloseAllocation = async () => {
    if (!closeTarget) return;
    setSubmittingClose(true);
    try {
      await rodtepService.closeAllocation(closeTarget.id, closeReason);
      toast.success(`Released reservation of ₹${closeTarget.outstandingInr.toFixed(2)} back to scrip.`);
      setCloseTarget(null);
      await loadData();
      onSuccess?.();
    } catch (err: any) {
      toast.error('Release failed: ' + (err?.message || err));
    } finally {
      setSubmittingClose(false);
    }
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-w-5xl max-h-[92vh] flex flex-col p-6 overflow-hidden">
          <DialogHeader className="border-b pb-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="p-2 rounded-lg bg-primary/10 text-primary">
                  <Award className="h-6 w-6" />
                </div>
                <div>
                  <DialogTitle className="text-xl font-bold flex items-center gap-2">
                    RoDTEP Duty Settlement Drawer
                    {evalCtx && (
                      <Badge variant="outline" className="font-mono text-xs">
                        BE #{evalCtx.boeDetail.beNumber} ({evalCtx.boeDetail.location})
                      </Badge>
                    )}
                  </DialogTitle>
                  <DialogDescription>
                    Manual multi-scrip duty credit drawdown against Basic Customs Duty (BCD) liability (Section 51B, Customs Act 1962).
                  </DialogDescription>
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={loadData}
                disabled={loading}
                className="gap-1.5"
              >
                <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
                Refresh
              </Button>
            </div>
          </DialogHeader>

          <div className="flex-1 overflow-y-auto space-y-6 py-4 pr-1">
            {/* 1. Live Customs Duty Liability Summary Cards */}
            {evalCtx && (
              <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                <Card className="bg-muted/40">
                  <CardHeader className="p-3 pb-1">
                    <CardDescription className="text-xs font-semibold text-muted-foreground uppercase">
                      Assessed BCD
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-3 pt-0">
                    <div className="text-lg font-bold">
                      ₹{evalCtx.assessedBcdInr.toLocaleString('en-IN', { minimumFractionDigits: 2 })}
                    </div>
                    <span className="text-[11px] text-muted-foreground">Statutory BCD Total</span>
                  </CardContent>
                </Card>

                <Card className="bg-muted/40">
                  <CardHeader className="p-3 pb-1">
                    <CardDescription className="text-xs font-semibold text-muted-foreground uppercase">
                      Cash BCD Paid
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-3 pt-0">
                    <div className="text-lg font-bold text-slate-700 dark:text-slate-300">
                      ₹{(evalCtx.cashBcdPaidPaise / 100.0).toLocaleString('en-IN', { minimumFractionDigits: 2 })}
                    </div>
                    <span className="text-[11px] text-muted-foreground">Challan Duty Paid</span>
                  </CardContent>
                </Card>

                <Card className="bg-muted/40">
                  <CardHeader className="p-3 pb-1">
                    <CardDescription className="text-xs font-semibold text-muted-foreground uppercase">
                      Confirmed RoDTEP
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-3 pt-0">
                    <div className="text-lg font-bold text-emerald-600">
                      ₹{(evalCtx.netConfirmedRodtepPaise / 100.0).toLocaleString('en-IN', { minimumFractionDigits: 2 })}
                    </div>
                    <span className="text-[11px] text-muted-foreground">Statutory Debits</span>
                  </CardContent>
                </Card>

                <Card className="bg-muted/40">
                  <CardHeader className="p-3 pb-1">
                    <CardDescription className="text-xs font-semibold text-muted-foreground uppercase">
                      Reserved (Pending)
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-3 pt-0">
                    <div className="text-lg font-bold text-amber-600">
                      ₹{(evalCtx.activeReservedRodtepPaise / 100.0).toLocaleString('en-IN', { minimumFractionDigits: 2 })}
                    </div>
                    <span className="text-[11px] text-muted-foreground">Stage 1 Allocations</span>
                  </CardContent>
                </Card>

                <Card className={eligibleUnpaidPaise > 0 ? 'bg-primary/5 border-primary/40' : 'bg-muted/40'}>
                  <CardHeader className="p-3 pb-1">
                    <CardDescription className="text-xs font-semibold text-primary uppercase">
                      Eligible Unpaid BCD
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-3 pt-0">
                    <div className="text-lg font-black text-primary">
                      ₹{eligibleUnpaidINR.toLocaleString('en-IN', { minimumFractionDigits: 2 })}
                    </div>
                    <span className="text-[11px] text-muted-foreground">Maximum Drawdown Cap</span>
                  </CardContent>
                </Card>
              </div>
            )}

            {/* If zero unpaid BCD, show informational banner */}
            {eligibleUnpaidPaise === 0 && (
              <div className="flex items-center gap-3 p-4 rounded-lg bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800 text-emerald-800 dark:text-emerald-200">
                <CheckCircle2 className="h-5 w-5 flex-shrink-0 text-emerald-600 dark:text-emerald-400" />
                <div className="text-sm">
                  <span className="font-semibold">Statutory BCD Liability Satisfied:</span> No unpaid Basic Customs Duty liability remains for this Bill of Entry. Duty has been satisfied in full via confirmed RoDTEP debits or cash payment.
                </div>
              </div>
            )}

            {/* 2. Manual Scrip Drawdown Section (Only if Eligible Unpaid BCD > 0) */}
            {eligibleUnpaidPaise > 0 && (
              <Card>
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <div>
                      <CardTitle className="text-base font-semibold flex items-center gap-2">
                        Manual Scrip Selection & Drawdown
                        <Badge variant="secondary" className="text-[10px]">
                          STRICTLY MANUAL &bull; NO FIFO
                        </Badge>
                      </CardTitle>
                      <CardDescription className="text-xs">
                        Select specific scrips and enter exact drawdown amounts in Rupees.
                      </CardDescription>
                    </div>

                    <div className="flex items-center gap-2">
                      <Label className="text-xs text-muted-foreground">Filter Port:</Label>
                      <Select value={portFilter} onValueChange={setPortFilter}>
                        <SelectTrigger className="w-[140px] h-8 text-xs">
                          <SelectValue placeholder="All Ports" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="ALL">All Ports</SelectItem>
                          <SelectItem value="INMAA1">INMAA1 (Sea)</SelectItem>
                          <SelectItem value="INMAA4">INMAA4 (Air)</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-2">
                  {filteredScrips.length === 0 ? (
                    <div className="py-6 text-center text-sm text-muted-foreground">
                      No active scrips with available balance found for port filter: {portFilter}
                    </div>
                  ) : (
                    <div className="border rounded-lg overflow-hidden">
                      <Table className="text-xs">
                        <TableHeader>
                          <TableRow className="bg-muted/50">
                            <TableHead className="w-10 text-center">Select</TableHead>
                            <TableHead>Scrip Number</TableHead>
                            <TableHead>Reg Port</TableHead>
                            <TableHead>Expiry Date</TableHead>
                            <TableHead className="text-right">Confirmed Bal</TableHead>
                            <TableHead className="text-right">Reserved</TableHead>
                            <TableHead className="text-right font-semibold">Net Available</TableHead>
                            <TableHead className="w-36 text-right font-semibold">Drawdown (₹)</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {filteredScrips.map((scrip) => {
                            const isSelected = !!selectedScrips[scrip.id];
                            const exceeds = !!scripExceedsMap[scrip.id];
                            return (
                              <TableRow
                                key={scrip.id}
                                className={isSelected ? 'bg-primary/5' : undefined}
                              >
                                <TableCell className="text-center">
                                  <input
                                    type="checkbox"
                                    checked={isSelected}
                                    onChange={(e) => handleSelectScrip(scrip, e.target.checked)}
                                    className="h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary cursor-pointer"
                                  />
                                </TableCell>
                                <TableCell className="font-mono font-medium">
                                  {scrip.scripNumber}
                                </TableCell>
                                <TableCell>
                                  <Badge variant="outline" className="text-[10px]">
                                    {scrip.scripPortCode}
                                  </Badge>
                                </TableCell>
                                <TableCell>
                                  <div className="flex items-center gap-1.5">
                                    <span>{scrip.expiryDate}</span>
                                    {scrip.daysToExpiry < 90 && (
                                      <Badge variant="destructive" className="text-[9px] px-1 py-0">
                                        {scrip.daysToExpiry}d
                                      </Badge>
                                    )}
                                  </div>
                                </TableCell>
                                <TableCell className="text-right">
                                  ₹{scrip.confirmedBalInr.toFixed(2)}
                                </TableCell>
                                <TableCell className="text-right text-muted-foreground">
                                  ₹{scrip.pendingAllocInr.toFixed(2)}
                                </TableCell>
                                <TableCell className="text-right font-semibold text-emerald-600">
                                  ₹{scrip.netAvailableInr.toFixed(2)}
                                </TableCell>
                                <TableCell className="text-right">
                                  <div className="flex flex-col items-end gap-1">
                                    <Input
                                      type="number"
                                      step="0.01"
                                      min="0"
                                      max={scrip.netAvailableInr}
                                      disabled={!isSelected}
                                      value={selectedScrips[scrip.id] || ''}
                                      onChange={(e) =>
                                        handleScripAmountChange(scrip.id, e.target.value)
                                      }
                                      placeholder="0.00"
                                      className={`h-7 w-28 text-right font-mono text-xs ${
                                        exceeds ? 'border-destructive focus-visible:ring-destructive' : ''
                                      }`}
                                    />
                                    {exceeds && (
                                      <span className="text-[10px] text-destructive leading-tight">
                                        Exceeds scrip balance
                                      </span>
                                    )}
                                  </div>
                                </TableCell>
                              </TableRow>
                            );
                          })}
                        </TableBody>
                      </Table>
                    </div>
                  )}

                  {/* Drawdown Summary & Action */}
                  <div className="mt-4 pt-4 border-t flex flex-col md:flex-row items-center justify-between gap-4">
                    <div className="flex items-center gap-4 text-xs">
                      <div>
                        <span className="text-muted-foreground">Total Drawdown: </span>
                        <span
                          className={`font-mono text-sm font-bold ${
                            isOverBOECap ? 'text-destructive' : 'text-primary'
                          }`}
                        >
                          ₹{totalAllocRequestedINR.toFixed(2)}
                        </span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Eligible BCD Cap: </span>
                        <span className="font-mono text-sm font-semibold">
                          ₹{eligibleUnpaidINR.toFixed(2)}
                        </span>
                      </div>
                      {isOverBOECap && (
                        <Badge variant="destructive" className="gap-1 text-[10px]">
                          <XCircle className="h-3 w-3" />
                          Exceeds Eligible BCD by ₹{(totalAllocRequestedINR - eligibleUnpaidINR).toFixed(2)}
                        </Badge>
                      )}
                    </div>

                    <div className="flex items-center gap-3 w-full md:w-auto">
                      <Input
                        placeholder="Optional allocation remarks..."
                        value={allocationRemarks}
                        onChange={(e) => setAllocationRemarks(e.target.value)}
                        className="h-8 text-xs w-60"
                      />
                      <Button
                        size="sm"
                        onClick={handleCreateAllocation}
                        disabled={
                          submittingAlloc ||
                          totalAllocRequestedPaise === 0 ||
                          isOverBOECap ||
                          hasAnyScripExceeded
                        }
                        className="gap-1.5 whitespace-nowrap"
                      >
                        <FileCheck className="h-4 w-4" />
                        {submittingAlloc ? 'Reserving...' : 'Reserve RoDTEP Balance'}
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )}

            {/* 3. Existing Allocations for this BOE */}
            <Card>
              <CardHeader className="p-4 pb-2">
                <CardTitle className="text-base font-semibold flex items-center justify-between">
                  <span>Current BOE Scrip Allocations & Confirmations</span>
                  {evalCtx && (
                    <span className="text-xs font-normal text-muted-foreground">
                      {(evalCtx.activeAllocations ?? []).length} allocation record(s)
                    </span>
                  )}
                </CardTitle>
              </CardHeader>
              <CardContent className="p-4 pt-2">
                {!evalCtx || (evalCtx.activeAllocations ?? []).length === 0 ? (
                  <div className="py-6 text-center text-sm text-muted-foreground">
                    No scrip allocations have been created for this Bill of Entry yet.
                  </div>
                ) : (
                  <div className="border rounded-lg overflow-hidden">
                    <Table className="text-xs">
                      <TableHeader>
                        <TableRow className="bg-muted/50">
                          <TableHead>Allocation Ref</TableHead>
                          <TableHead>Scrip #</TableHead>
                          <TableHead>Port</TableHead>
                          <TableHead className="text-right">Allocated</TableHead>
                          <TableHead className="text-right">Confirmed</TableHead>
                          <TableHead className="text-right font-semibold">Outstanding</TableHead>
                          <TableHead className="text-center">Status</TableHead>
                          <TableHead className="text-right">Actions</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {(evalCtx.activeAllocations ?? []).map((alloc) => {
                          const canConfirm =
                            alloc.status === 'ALLOCATED' ||
                            alloc.status === 'PARTIALLY_CONFIRMED';
                          const canClose =
                            (alloc.status === 'ALLOCATED' ||
                              alloc.status === 'PARTIALLY_CONFIRMED') &&
                            alloc.outstandingPaise > 0;

                          return (
                            <TableRow key={alloc.id}>
                              <TableCell className="font-mono font-medium">
                                {alloc.allocationRef}
                              </TableCell>
                              <TableCell className="font-mono">
                                {alloc.scripNumber}
                              </TableCell>
                              <TableCell>
                                <Badge variant="outline" className="text-[10px]">
                                  {alloc.scripPortCode}
                                </Badge>
                              </TableCell>
                              <TableCell className="text-right">
                                ₹{alloc.allocatedInr.toFixed(2)}
                              </TableCell>
                              <TableCell className="text-right text-emerald-600 font-semibold">
                                ₹{alloc.confirmedInr.toFixed(2)}
                              </TableCell>
                              <TableCell className="text-right font-mono font-semibold">
                                ₹{alloc.outstandingInr.toFixed(2)}
                              </TableCell>
                              <TableCell className="text-center">
                                <Badge
                                  variant={
                                    alloc.status === 'CONFIRMED'
                                      ? 'default'
                                      : alloc.status === 'PARTIALLY_CONFIRMED'
                                      ? 'secondary'
                                      : alloc.status === 'ALLOCATED'
                                      ? 'outline'
                                      : 'destructive'
                                  }
                                  className="text-[10px]"
                                >
                                  {alloc.status}
                                </Badge>
                              </TableCell>
                              <TableCell className="text-right">
                                <div className="flex items-center justify-end gap-1.5">
                                  {canConfirm && (
                                    <Button
                                      size="sm"
                                      variant="default"
                                      onClick={() => handleOpenConfirmDialog(alloc)}
                                      className="h-7 px-2 text-xs gap-1"
                                    >
                                      <CheckCircle2 className="h-3 w-3" />
                                      Confirm Debit
                                    </Button>
                                  )}
                                  {canClose && (
                                    <Button
                                      size="sm"
                                      variant="ghost"
                                      onClick={() => handleOpenCloseDialog(alloc)}
                                      className="h-7 px-2 text-xs text-muted-foreground hover:text-destructive"
                                    >
                                      Release
                                    </Button>
                                  )}
                                </div>
                              </TableCell>
                            </TableRow>
                          );
                        })}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          <DialogFooter className="border-t pt-3">
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              Close Drawer
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Stage 2 Confirmation Dialog */}
      <Dialog open={!!confirmTarget} onOpenChange={(o) => !o && setConfirmTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base flex items-center gap-2">
              <CheckCircle2 className="h-5 w-5 text-emerald-600" />
              Confirm ICEGATE Actual Scrip Debit
            </DialogTitle>
            <DialogDescription className="text-xs">
              Record customs challan and ICEGATE transaction proof to update the statutory ledger.
            </DialogDescription>
          </DialogHeader>

          {confirmTarget && (
            <div className="space-y-4 py-2 text-xs">
              <div className="p-3 rounded-lg bg-muted/50 space-y-1 font-mono text-[11px]">
                <div>Allocation: {confirmTarget.allocationRef}</div>
                <div>Scrip: {confirmTarget.scripNumber} ({confirmTarget.scripPortCode})</div>
                <div>Outstanding Reserved: ₹{confirmTarget.outstandingInr.toFixed(2)}</div>
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Actual Debit Amount (₹) *</Label>
                <Input
                  type="number"
                  step="0.01"
                  min="0.01"
                  max={confirmTarget.outstandingInr}
                  value={confirmAmountINR}
                  onChange={(e) => setConfirmAmountINR(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">ICEGATE Transaction Reference *</Label>
                <Input
                  placeholder="e.g. ICE20261124-778899"
                  value={confirmIcegateRef}
                  onChange={(e) => setConfirmIcegateRef(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label className="text-xs">Customs Challan Number</Label>
                  <Input
                    placeholder="e.g. CH-20261120-1"
                    value={confirmChallanNo}
                    onChange={(e) => setConfirmChallanNo(e.target.value)}
                    className="font-mono text-xs"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs">Debit Date *</Label>
                  <Input
                    type="date"
                    value={confirmDebitDate}
                    onChange={(e) => setConfirmDebitDate(e.target.value)}
                    className="text-xs"
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Supporting Document / Receipt Ref</Label>
                <Input
                  placeholder="e.g. ICEGATE_ECL_CHALLAN_RECEIPT.PDF"
                  value={confirmDocRef}
                  onChange={(e) => setConfirmDocRef(e.target.value)}
                  className="text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Remarks</Label>
                <Input
                  placeholder="Optional confirmation remarks"
                  value={confirmRemarks}
                  onChange={(e) => setConfirmRemarks(e.target.value)}
                  className="text-xs"
                />
              </div>
            </div>
          )}

          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setConfirmTarget(null)}
              disabled={submittingConfirm}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={handleSubmitConfirmDebit}
              disabled={submittingConfirm}
              className="gap-1.5"
            >
              <CheckCircle2 className="h-4 w-4" />
              {submittingConfirm ? 'Confirming...' : 'Confirm Statutory Debit'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Closeout / Release Dialog */}
      <Dialog open={!!closeTarget} onOpenChange={(o) => !o && setCloseTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base flex items-center gap-2">
              <Clock className="h-5 w-5 text-amber-600" />
              Release Reserved Scrip Balance
            </DialogTitle>
            <DialogDescription className="text-xs">
              Release unconfirmed balance of ₹{closeTarget?.outstandingInr.toFixed(2)} back to the scrip.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 py-2 text-xs">
            <div className="space-y-1.5">
              <Label className="text-xs">Reason for Release *</Label>
              <Input
                value={closeReason}
                onChange={(e) => setCloseReason(e.target.value)}
                placeholder="e.g. Customs duty satisfied in cash, shipment cleared"
                className="text-xs"
              />
            </div>
          </div>

          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCloseTarget(null)}
              disabled={submittingClose}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={handleSubmitCloseAllocation}
              disabled={submittingClose || !closeReason.trim()}
              className="gap-1.5"
            >
              {submittingClose ? 'Releasing...' : 'Confirm Release'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
