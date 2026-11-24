import * as React from 'react';
import {
  Award,
  CheckCircle2,
  FileSpreadsheet,
  History,
  Plus,
  RefreshCw,
  RotateCcw,
  Scale,
  Search,
  ShieldCheck,
  Trash2,
} from 'lucide-react';
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
import { RoDTEPImportModal } from '@/components/boe/RoDTEPImportModal';
import { rodtepService } from '@/services/rodtep-service';
import type {
  RoDTEPAuditLog,
  RoDTEPBOEDebitSummary,
  RoDTEPDebitLedger,
  RoDTEPPortSummary,
  RoDTEPReconciliation,
  RoDTEPReversalInput,
  RoDTEPScrip,
  RoDTEPScripInput,
} from '@/types/rodtep';

export function RoDTEPScripsPage() {
  const [activeTab, setActiveTab] = React.useState('register');
  const [loading, setLoading] = React.useState(false);

  // Summaries & Scrips state
  const [portSummaries, setPortSummaries] = React.useState<RoDTEPPortSummary[]>([]);
  const [scrips, setScrips] = React.useState<RoDTEPScrip[]>([]);
  const [boeDebits, setBoeDebits] = React.useState<RoDTEPBOEDebitSummary[]>([]);

  // Filter state
  const [selectedPort, setSelectedPort] = React.useState<string>('ALL');
  const [selectedStatus, setSelectedStatus] = React.useState<string>('ALL');
  const [searchQuery, setSearchQuery] = React.useState('');

  // Modals state
  const [isNewScripOpen, setIsNewScripOpen] = React.useState(false);
  const [newScrip, setNewScrip] = React.useState<Partial<RoDTEPScripInput>>({
    scripPortCode: 'INMAA1',
    acquisitionType: 'DIRECT_GENERATION',
    issueDate: new Date().toISOString().split('T')[0],
  });
  const [newOriginalINR, setNewOriginalINR] = React.useState('');
  const [newOpeningINR, setNewOpeningINR] = React.useState('');
  const [submittingScrip, setSubmittingScrip] = React.useState(false);

  // Opening Verification Dialog State
  const [verifyTarget, setVerifyTarget] = React.useState<RoDTEPScrip | null>(null);
  const [verifyEvidenceRef, setVerifyEvidenceRef] = React.useState('');
  const [submittingVerify, setSubmittingVerify] = React.useState(false);

  // Audit Logs Modal State
  const [auditTarget, setAuditTarget] = React.useState<RoDTEPScrip | null>(null);
  const [auditLogs, setAuditLogs] = React.useState<RoDTEPAuditLog[]>([]);
  const [loadingAudit, setLoadingAudit] = React.useState(false);

  // Reversal Dialog State
  const [reversalTarget, setReversalTarget] = React.useState<RoDTEPDebitLedger | null>(null);
  const [reversalAmountINR, setReversalAmountINR] = React.useState('');
  const [reversalReason, setReversalReason] = React.useState('');
  const [reversalDocRef, setReversalDocRef] = React.useState('');
  const [submittingReversal, setSubmittingReversal] = React.useState(false);

  // Reconciliation Dialog & History State
  const [reconScripId, setReconScripId] = React.useState('');
  const [reconDate, setReconDate] = React.useState(new Date().toISOString().split('T')[0]);
  const [reconPeriodEnd, setReconPeriodEnd] = React.useState(new Date().toISOString().split('T')[0]);
  const [reconIcegateBalanceINR, setReconIcegateBalanceINR] = React.useState('');
  const [reconReason, setReconReason] = React.useState('');
  const [reconDocPath, setReconDocPath] = React.useState('');
  const [reconciliationList, setReconciliationList] = React.useState<RoDTEPReconciliation[]>([]);
  const [submittingRecon, setSubmittingRecon] = React.useState(false);

  // Load all core data
  const loadData = React.useCallback(async () => {
    setLoading(true);
    try {
      const [summaries, scripList, boeList] = await Promise.all([
        rodtepService.getPortSummary(),
        rodtepService.listScrips(selectedPort, selectedStatus, searchQuery),
        rodtepService.getBOEDebitReport(),
      ]);
      setPortSummaries(summaries);
      setScrips(scripList);
      setBoeDebits(boeList);

      // If a scrip was selected in reconciliation tab, load its history
      if (reconScripId) {
        const recs = await rodtepService.listReconciliations(reconScripId);
        setReconciliationList(recs);
      }
    } catch (err: any) {
      toast.error('Failed to load RoDTEP records: ' + (err?.message || err));
    } finally {
      setLoading(false);
    }
  }, [selectedPort, selectedStatus, searchQuery, reconScripId]);

  React.useEffect(() => {
    loadData();
  }, [loadData]);

  // Excel / CSV Import Modal State
  const [isImportModalOpen, setIsImportModalOpen] = React.useState(false);

  const existingScripNumbers = React.useMemo(() => {
    return new Set((scrips || []).map((s) => s.scripNumber));
  }, [scrips]);

  // Create Scrip
  const handleCreateScrip = async () => {
    const scripNum = newScrip.scripNumber?.trim();
    if (!scripNum) {
      toast.error('Scrip Number is required.');
      return;
    }
    const orig = parseFloat(newOriginalINR) || 0;
    if (orig <= 0) {
      toast.error('Original Value must be greater than zero.');
      return;
    }
    const openVal = newOpeningINR ? parseFloat(newOpeningINR) || 0 : orig;
    if (openVal < 0 || openVal > orig) {
      toast.error('Opening Balance must be between 0 and Original Value.');
      return;
    }

    setSubmittingScrip(true);
    try {
      const input: RoDTEPScripInput = {
        scripNumber: scripNum,
        scripPortCode: (newScrip.scripPortCode as 'INMAA1' | 'INMAA4') || 'INMAA1',
        acquisitionType: newScrip.acquisitionType || 'DIRECT_GENERATION',
        issueDate: newScrip.issueDate || '',
        expiryDate: newScrip.expiryDate,
        originalValuePaise: Math.round(orig * 100),
        openingBalancePaise: Math.round(openVal * 100),
        icegateUserId: newScrip.icegateUserId,
        registrationDetails: newScrip.registrationDetails,
        documentRef: newScrip.documentRef,
        remarks: newScrip.remarks,
      };
      await rodtepService.createScrip(input);
      toast.success(`RoDTEP Scrip ${scripNum} created successfully.`);
      setIsNewScripOpen(false);
      setNewScrip({
        scripPortCode: 'INMAA1',
        acquisitionType: 'DIRECT_GENERATION',
        issueDate: new Date().toISOString().split('T')[0],
      });
      setNewOriginalINR('');
      setNewOpeningINR('');
      await loadData();
    } catch (err: any) {
      toast.error('Failed to create scrip: ' + (err?.message || err));
    } finally {
      setSubmittingScrip(false);
    }
  };

  // Verify Opening Balance
  const handleVerifyOpening = async () => {
    if (!verifyTarget) return;
    if (!verifyEvidenceRef.trim()) {
      toast.error('Please enter the statement or ICEGATE document reference.');
      return;
    }
    setSubmittingVerify(true);
    try {
      await rodtepService.verifyOpeningBalance(verifyTarget.id, verifyEvidenceRef.trim());
      toast.success(`Opening balance verified for scrip ${verifyTarget.scripNumber}.`);
      setVerifyTarget(null);
      setVerifyEvidenceRef('');
      await loadData();
    } catch (err: any) {
      toast.error('Verification failed: ' + (err?.message || err));
    } finally {
      setSubmittingVerify(false);
    }
  };

  // View Audit Logs
  const handleOpenAuditLogs = async (scrip: RoDTEPScrip) => {
    setAuditTarget(scrip);
    setLoadingAudit(true);
    try {
      const logs = await rodtepService.listAuditLogs(scrip.id);
      setAuditLogs(logs);
    } catch (err: any) {
      toast.error('Failed to load audit logs: ' + (err?.message || err));
    } finally {
      setLoadingAudit(false);
    }
  };

  // Submit Reversal
  const handleSubmitReversal = async () => {
    if (!reversalTarget) return;
    const amt = parseFloat(reversalAmountINR) || 0;
    if (amt <= 0) {
      toast.error('Reversal amount must be greater than zero.');
      return;
    }
    if (!reversalReason.trim()) {
      toast.error('Reversal reason is required.');
      return;
    }

    setSubmittingReversal(true);
    try {
      const input: RoDTEPReversalInput = {
        debitLedgerID: reversalTarget.id,
        amountPaise: Math.round(amt * 100),
        reason: reversalReason.trim(),
        docRef: reversalDocRef.trim(),
      };
      await rodtepService.reverseDebit(input);
      toast.success(`Statutory debit reversal of ₹${amt.toFixed(2)} recorded successfully.`);
      setReversalTarget(null);
      await loadData();
    } catch (err: any) {
      toast.error('Reversal failed: ' + (err?.message || err));
    } finally {
      setSubmittingReversal(false);
    }
  };

  // Submit Reconciliation
  const handleSubmitReconciliation = async () => {
    if (!reconScripId) {
      toast.error('Please select a scrip for reconciliation.');
      return;
    }
    const stmtVal = parseFloat(reconIcegateBalanceINR);
    if (isNaN(stmtVal) || stmtVal < 0) {
      toast.error('Please enter a valid ICEGATE statement balance.');
      return;
    }

    setSubmittingRecon(true);
    try {
      await rodtepService.saveReconciliation({
        scripId: reconScripId,
        reconciliationDate: reconDate || '',
        statementPeriodEnd: reconPeriodEnd || '',
        icegateStatementBalancePaise: Math.round(stmtVal * 100),
        discrepancyReason: reconReason.trim(),
        supportingStatementPath: reconDocPath.trim(),
      });
      toast.success('Reconciliation record logged successfully.');
      const recs = await rodtepService.listReconciliations(reconScripId);
      setReconciliationList(recs);
      setReconIcegateBalanceINR('');
      setReconReason('');
      setReconDocPath('');
      await loadData();
    } catch (err: any) {
      toast.error('Reconciliation failed: ' + (err?.message || err));
    } finally {
      setSubmittingRecon(false);
    }
  };

  // Collect all ledger items across all BOEs for the Ledger tab
  const allLedgerItems = React.useMemo(() => {
    const list: RoDTEPDebitLedger[] = [];
    for (const b of boeDebits || []) {
      if (b && b.confirmedDebits) {
        list.push(...b.confirmedDebits);
      }
    }
    return list.sort((a, b) => (b.createdAt || '').localeCompare(a.createdAt || ''));
  }, [boeDebits]);

  // Selected scrip for reconciliation live discrepancy calculation
  const targetReconScrip = React.useMemo(() => {
    return (scrips || []).find((s) => s.id === reconScripId) || null;
  }, [scrips, reconScripId]);

  const liveReconDiscrepancyINR = React.useMemo(() => {
    if (!targetReconScrip || reconIcegateBalanceINR === '') return null;
    const stmt = parseFloat(reconIcegateBalanceINR) || 0;
    return stmt - (targetReconScrip.confirmedBalInr ?? 0);
  }, [targetReconScrip, reconIcegateBalanceINR]);

  return (
    <div className="w-full space-y-6">
      {/* 1. Header Banner */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-2 border-b">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-primary/10 text-primary">
            <Award className="h-7 w-7" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">RoDTEP Scrip Management</h1>
            <p className="text-xs text-muted-foreground">
              Electronic Duty Credit Ledger sub-ledger &bull; Port-wise reference tracking &bull; Two-stage manual BOE drawdown &bull; Append-only audit trail
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={loadData}
            disabled={loading}
            className="gap-1.5 text-xs"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsImportModalOpen(true)}
            className="gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 border-emerald-500/30 hover:bg-emerald-500/10"
          >
            <FileSpreadsheet className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
            Import Scrips (Excel / CSV)
          </Button>

          <Button
            size="sm"
            onClick={() => setIsNewScripOpen(true)}
            className="gap-1.5 text-xs"
          >
            <Plus className="h-4 w-4" />
            Add New Scrip
          </Button>
        </div>
      </div>

      {/* 2. Portfolio Summary Cards (Combined, INMAA1 Sea, INMAA4 Air) */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {(portSummaries || []).map((ps) => {
          const isCombined = ps.portCode === 'COMBINED';
          const confBal = ps.confirmedBalanceInr ?? 0;
          const pendAlloc = ps.pendingAllocatedInr ?? 0;
          const netAvail = ps.netAvailableInr ?? 0;
          return (
            <Card
              key={ps.portCode}
              className={`border ${
                isCombined
                  ? 'bg-gradient-to-br from-primary/5 via-background to-primary/10 border-primary/30'
                  : 'bg-card'
              }`}
            >
              <CardHeader className="p-4 pb-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Badge
                      variant={isCombined ? 'default' : 'outline'}
                      className="text-xs font-semibold"
                    >
                      {ps.portCode}
                    </Badge>
                    <CardTitle className="text-sm font-bold">{ps.portName}</CardTitle>
                  </div>
                  <span className="text-xs text-muted-foreground font-mono">
                    {ps.totalScripsCount ?? 0} scrips
                  </span>
                </div>
              </CardHeader>
              <CardContent className="p-4 pt-1 space-y-2">
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <div>
                    <span className="text-muted-foreground block text-[11px]">Confirmed Balance</span>
                    <span className="font-bold text-sm">
                      ₹{confBal.toLocaleString('en-IN', { minimumFractionDigits: 2 })}
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block text-[11px]">Pending Reserved</span>
                    <span className="font-semibold text-amber-600">
                      ₹{pendAlloc.toLocaleString('en-IN', { minimumFractionDigits: 2 })}
                    </span>
                  </div>
                </div>

                <div className="pt-2 border-t flex items-center justify-between text-xs">
                  <span className="text-muted-foreground">Net Available for BOE:</span>
                  <span className="font-mono font-bold text-emerald-600 text-sm">
                    ₹{netAvail.toLocaleString('en-IN', { minimumFractionDigits: 2 })}
                  </span>
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>

      {/* 3. Navigation Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
        <TabsList className="inline-flex w-auto">
          <TabsTrigger value="register" className="text-xs gap-1.5">
            <Award className="h-4 w-4" />
            Scrip Register
          </TabsTrigger>
          <TabsTrigger value="boe-debits" className="text-xs gap-1.5">
            <FileSpreadsheet className="h-4 w-4" />
            BOE-Wise Debits
          </TabsTrigger>
          <TabsTrigger value="ledger" className="text-xs gap-1.5">
            <History className="h-4 w-4" />
            Utilization Ledger
          </TabsTrigger>
          <TabsTrigger value="reconciliation" className="text-xs gap-1.5">
            <Scale className="h-4 w-4" />
            ICEGATE Reconciliation
          </TabsTrigger>
        </TabsList>

        {/* TAB 1: SCRIP REGISTER */}
        <TabsContent value="register" className="space-y-4">
          {/* Filters Bar */}
          <Card className="p-4">
            <div className="flex flex-col md:flex-row items-center justify-between gap-3">
              <div className="flex items-center gap-3 w-full md:w-auto">
                <div className="relative flex-1 md:w-64">
                  <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                  <Input
                    placeholder="Search scrip number, remarks..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="pl-9 h-9 text-xs"
                  />
                </div>

                <Select value={selectedPort} onValueChange={setSelectedPort}>
                  <SelectTrigger className="w-[140px] h-9 text-xs">
                    <SelectValue placeholder="Port Filter" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ALL">All Ports</SelectItem>
                    <SelectItem value="INMAA1">INMAA1 (Sea)</SelectItem>
                    <SelectItem value="INMAA4">INMAA4 (Air)</SelectItem>
                  </SelectContent>
                </Select>

                <Select value={selectedStatus} onValueChange={setSelectedStatus}>
                  <SelectTrigger className="w-[140px] h-9 text-xs">
                    <SelectValue placeholder="Status Filter" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ALL">All Statuses</SelectItem>
                    <SelectItem value="ACTIVE">Active</SelectItem>
                    <SelectItem value="EXHAUSTED">Exhausted</SelectItem>
                    <SelectItem value="EXPIRED">Expired</SelectItem>
                    <SelectItem value="SUSPENDED">Suspended</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <span className="text-xs text-muted-foreground whitespace-nowrap">
                Showing {(scrips || []).length} scrips
              </span>
            </div>
          </Card>

          {/* Scrip Table */}
          <Card>
            <Table className="text-xs">
              <TableHeader>
                <TableRow className="bg-muted/50">
                  <TableHead>Scrip Number</TableHead>
                  <TableHead>Port</TableHead>
                  <TableHead>Acquisition</TableHead>
                  <TableHead>Issue Date</TableHead>
                  <TableHead>Expiry Date</TableHead>
                  <TableHead className="text-right">Original Value</TableHead>
                  <TableHead className="text-right">Confirmed Bal</TableHead>
                  <TableHead className="text-right">Reserved</TableHead>
                  <TableHead className="text-right font-semibold">Net Available</TableHead>
                  <TableHead className="text-center">Status</TableHead>
                  <TableHead className="text-center">Verification</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(!scrips || scrips.length === 0) ? (
                  <TableRow>
                    <TableCell colSpan={12} className="py-8 text-center text-muted-foreground">
                      No RoDTEP scrips found matching the selected criteria.
                    </TableCell>
                  </TableRow>
                ) : (
                  (scrips || []).map((s) => (
                    <TableRow key={s.id}>
                      <TableCell className="font-mono font-medium">{s.scripNumber}</TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-[10px]">
                          {s.scripPortCode}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <span className="text-muted-foreground text-[11px]">
                          {s.acquisitionType === 'OPENING_BALANCE' ? 'Opening' : 'Direct'}
                        </span>
                      </TableCell>
                      <TableCell>{s.issueDate}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1.5">
                          <span>{s.expiryDate}</span>
                          {(s.daysToExpiry ?? 0) < 90 && (s.daysToExpiry ?? 0) > 0 && (
                            <Badge variant="destructive" className="text-[9px] px-1 py-0">
                              {s.daysToExpiry}d
                            </Badge>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-right">₹{(s.originalValueInr ?? 0).toFixed(2)}</TableCell>
                      <TableCell className="text-right font-medium">
                        ₹{(s.confirmedBalInr ?? 0).toFixed(2)}
                      </TableCell>
                      <TableCell className="text-right text-amber-600">
                        ₹{(s.pendingAllocInr ?? 0).toFixed(2)}
                      </TableCell>
                      <TableCell className="text-right font-mono font-bold text-emerald-600">
                        ₹{(s.netAvailableInr ?? 0).toFixed(2)}
                      </TableCell>
                      <TableCell className="text-center">
                        <Badge
                          variant={
                            s.status === 'ACTIVE'
                              ? 'default'
                              : s.status === 'EXHAUSTED'
                              ? 'secondary'
                              : 'destructive'
                          }
                          className="text-[10px]"
                        >
                          {s.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-center">
                        {s.openingVerifiedAt ? (
                          <Badge variant="outline" className="text-[10px] text-emerald-600 border-emerald-300 gap-1">
                            <ShieldCheck className="h-3 w-3" />
                            Verified
                          </Badge>
                        ) : (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => {
                              setVerifyTarget(s);
                              setVerifyEvidenceRef(s.openingEvidenceRef || '');
                            }}
                            className="h-6 px-1.5 text-[10px] text-amber-600 hover:text-amber-700"
                          >
                            Verify Opening
                          </Button>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleOpenAuditLogs(s)}
                            className="h-7 w-7 p-0"
                            title="View Audit Trail"
                          >
                            <History className="h-3.5 w-3.5 text-muted-foreground" />
                          </Button>
                          {s.confirmedUtilizedPaise === 0 && s.pendingAllocatedPaise === 0 && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={async () => {
                                if (confirm(`Soft-delete scrip ${s.scripNumber}?`)) {
                                  try {
                                    await rodtepService.deleteScrip(s.id);
                                    toast.success('Scrip deleted.');
                                    await loadData();
                                  } catch (err: any) {
                                    toast.error('Delete failed: ' + (err?.message || err));
                                  }
                                }
                              }}
                              className="h-7 w-7 p-0 text-destructive hover:bg-destructive/10"
                              title="Delete Scrip"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </Card>
        </TabsContent>

        {/* TAB 2: BOE-WISE DEBITS */}
        <TabsContent value="boe-debits" className="space-y-4">
          <Card>
            <CardHeader className="p-4 pb-2">
              <CardTitle className="text-base font-semibold">
                Bill of Entry RoDTEP Duty Settlement Ledger
              </CardTitle>
              <CardDescription className="text-xs">
                Shows all Bills of Entry with active RoDTEP allocations, confirmed customs challan debits, and remaining unpaid BCD.
              </CardDescription>
            </CardHeader>
            <CardContent className="p-4 pt-2">
              {(!boeDebits || boeDebits.length === 0) ? (
                <div className="py-8 text-center text-sm text-muted-foreground">
                  No Bills of Entry have active RoDTEP duty reservations or confirmed debits yet.
                </div>
              ) : (
                <div className="border rounded-lg overflow-hidden">
                  <Table className="text-xs">
                    <TableHeader>
                      <TableRow className="bg-muted/50">
                        <TableHead>BE Number</TableHead>
                        <TableHead>BE Date</TableHead>
                        <TableHead>Port of Import</TableHead>
                        <TableHead className="text-right">Assessed BCD</TableHead>
                        <TableHead className="text-right">Cash BCD Paid</TableHead>
                        <TableHead className="text-right text-emerald-600 font-semibold">
                          Confirmed RoDTEP
                        </TableHead>
                        <TableHead className="text-right text-amber-600">
                          Reserved (Stage 1)
                        </TableHead>
                        <TableHead className="text-right font-bold text-primary">
                          Eligible Unpaid BCD
                        </TableHead>
                        <TableHead className="text-center">Allocations</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(boeDebits || []).map((b) => (
                        <TableRow key={b.boeId}>
                          <TableCell className="font-mono font-medium">{b.beNumber}</TableCell>
                          <TableCell>{b.beDate}</TableCell>
                          <TableCell>
                            <Badge variant="outline" className="text-[10px]">
                              {b.portOfImport}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right">₹{(b.assessedBcdInr ?? 0).toFixed(2)}</TableCell>
                          <TableCell className="text-right text-muted-foreground">
                            ₹{((b.cashBcdPaidPaise ?? 0) / 100.0).toFixed(2)}
                          </TableCell>
                          <TableCell className="text-right text-emerald-600 font-semibold">
                            ₹{(b.netConfirmedInr ?? 0).toFixed(2)}
                          </TableCell>
                          <TableCell className="text-right text-amber-600">
                            ₹{((b.activeReservedPaise ?? 0) / 100.0).toFixed(2)}
                          </TableCell>
                          <TableCell className="text-right font-mono font-bold text-primary">
                            ₹{(b.eligibleUnpaidInr ?? 0).toFixed(2)}
                          </TableCell>
                          <TableCell className="text-center">
                            <Badge variant="secondary" className="text-[10px]">
                              {b.allocations?.length || 0} active
                            </Badge>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* TAB 3: MASTER UTILIZATION LEDGER */}
        <TabsContent value="ledger" className="space-y-4">
          <Card>
            <CardHeader className="p-4 pb-2">
              <CardTitle className="text-base font-semibold">
                Chronological Statutory Scrip Debit Ledger
              </CardTitle>
              <CardDescription className="text-xs">
                Append-only statutory transactions recorded against customs challans and ICEGATE reference receipts.
              </CardDescription>
            </CardHeader>
            <CardContent className="p-4 pt-2">
              {allLedgerItems.length === 0 ? (
                <div className="py-8 text-center text-sm text-muted-foreground">
                  No confirmed customs debits have been recorded in the statutory ledger yet.
                </div>
              ) : (
                <div className="border rounded-lg overflow-hidden">
                  <Table className="text-xs">
                    <TableHeader>
                      <TableRow className="bg-muted/50">
                        <TableHead>Tx Ref</TableHead>
                        <TableHead>Scrip #</TableHead>
                        <TableHead>Port</TableHead>
                        <TableHead>BOE #</TableHead>
                        <TableHead>Type</TableHead>
                        <TableHead>Debit Date</TableHead>
                        <TableHead>ICEGATE Tx Ref</TableHead>
                        <TableHead>Challan #</TableHead>
                        <TableHead className="text-right font-semibold">Amount (₹)</TableHead>
                        <TableHead className="text-right">Remaining Bal (₹)</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {allLedgerItems.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell className="font-mono text-[11px] font-medium">
                            {item.transactionRef}
                          </TableCell>
                          <TableCell className="font-mono">{item.scripNumber}</TableCell>
                          <TableCell>
                            <Badge variant="outline" className="text-[10px]">
                              {item.scripPortCode}
                            </Badge>
                          </TableCell>
                          <TableCell className="font-mono">{item.boeNumber}</TableCell>
                          <TableCell>
                            <Badge
                              variant={item.txType === 'DEBIT_CONFIRMED' ? 'default' : 'secondary'}
                              className="text-[10px]"
                            >
                              {item.txType === 'DEBIT_CONFIRMED' ? 'Confirmed' : 'Reversal'}
                            </Badge>
                          </TableCell>
                          <TableCell>{item.debitDate}</TableCell>
                          <TableCell className="font-mono text-[11px]">
                            {item.icegateTransactionRef || '—'}
                          </TableCell>
                          <TableCell className="font-mono text-[11px]">
                            {item.challanNumber || '—'}
                          </TableCell>
                          <TableCell
                            className={`text-right font-mono font-bold ${
                              item.txType === 'DEBIT_CONFIRMED'
                                ? 'text-emerald-600'
                                : 'text-amber-600'
                            }`}
                          >
                            {item.txType === 'DEBIT_REVERSAL' ? '-' : '+'}₹{(item.amountInr ?? 0).toFixed(2)}
                          </TableCell>
                          <TableCell className="text-right font-mono">
                            ₹{(item.remainingInr ?? 0).toFixed(2)}
                          </TableCell>
                          <TableCell className="text-right">
                            {item.txType === 'DEBIT_CONFIRMED' && (
                              <Button
                                size="sm"
                                variant="ghost"
                                onClick={() => {
                                  setReversalTarget(item);
                                  setReversalAmountINR((item.amountInr ?? 0).toFixed(2));
                                  setReversalReason('');
                                  setReversalDocRef('');
                                }}
                                className="h-6 px-1.5 text-xs text-amber-600 hover:text-amber-700"
                              >
                                Reverse
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* TAB 4: ICEGATE MONTHLY RECONCILIATION */}
        <TabsContent value="reconciliation" className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {/* Reconciliation Logging Form */}
            <Card className="md:col-span-1">
              <CardHeader className="p-4 pb-2">
                <CardTitle className="text-base font-semibold flex items-center gap-2">
                  <Scale className="h-5 w-5 text-primary" />
                  Log Statement Reconciliation
                </CardTitle>
                <CardDescription className="text-xs">
                  Compare ERP confirmed balance against ICEGATE ECL statement balance.
                </CardDescription>
              </CardHeader>
              <CardContent className="p-4 pt-2 space-y-3 text-xs">
                <div className="space-y-1.5">
                  <Label className="text-xs">Target RoDTEP Scrip *</Label>
                  <Select value={reconScripId} onValueChange={setReconScripId}>
                    <SelectTrigger className="h-8 text-xs">
                      <SelectValue placeholder="Select scrip..." />
                    </SelectTrigger>
                    <SelectContent>
                      {(scrips || []).map((s) => (
                        <SelectItem key={s.id} value={s.id}>
                          {s.scripNumber} ({s.scripPortCode}) - Bal: ₹{(s.confirmedBalInr ?? 0).toFixed(2)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                {targetReconScrip && (
                  <div className="p-2.5 rounded bg-muted/50 space-y-1 text-[11px] font-mono">
                    <div>ERP Confirmed Balance: ₹{(targetReconScrip.confirmedBalInr ?? 0).toFixed(2)}</div>
                    <div>ERP Reserved Balance: ₹{(targetReconScrip.pendingAllocInr ?? 0).toFixed(2)}</div>
                  </div>
                )}

                <div className="grid grid-cols-2 gap-2">
                  <div className="space-y-1.5">
                    <Label className="text-xs">Reconciliation Date *</Label>
                    <Input
                      type="date"
                      value={reconDate}
                      onChange={(e) => setReconDate(e.target.value)}
                      className="h-8 text-xs"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Period End Date *</Label>
                    <Input
                      type="date"
                      value={reconPeriodEnd}
                      onChange={(e) => setReconPeriodEnd(e.target.value)}
                      className="h-8 text-xs"
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs">ICEGATE Statement Balance (₹) *</Label>
                  <Input
                    type="number"
                    step="0.01"
                    min="0"
                    placeholder="0.00"
                    value={reconIcegateBalanceINR}
                    onChange={(e) => setReconIcegateBalanceINR(e.target.value)}
                    className="h-8 font-mono text-xs"
                  />
                </div>

                {liveReconDiscrepancyINR !== null && (
                  <div
                    className={`p-2 rounded text-xs flex items-center justify-between font-mono ${
                      Math.abs(liveReconDiscrepancyINR) < 0.01
                        ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300'
                        : 'bg-amber-50 text-amber-800 dark:bg-amber-950/40 dark:text-amber-300'
                    }`}
                  >
                    <span>Discrepancy:</span>
                    <span className="font-bold">
                      {liveReconDiscrepancyINR === 0
                        ? '₹0.00 (Matched)'
                        : `₹${liveReconDiscrepancyINR.toFixed(2)}`}
                    </span>
                  </div>
                )}

                <div className="space-y-1.5">
                  <Label className="text-xs">Discrepancy Reason</Label>
                  <Input
                    placeholder="e.g. Unconfirmed debit pending customs response"
                    value={reconReason}
                    onChange={(e) => setReconReason(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs">Statement File / Reference</Label>
                  <Input
                    placeholder="e.g. ECL_STMT_OCT_2026.PDF"
                    value={reconDocPath}
                    onChange={(e) => setReconDocPath(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>

                <Button
                  size="sm"
                  onClick={handleSubmitReconciliation}
                  disabled={submittingRecon || !reconScripId || !reconIcegateBalanceINR}
                  className="w-full gap-1.5 mt-2"
                >
                  <CheckCircle2 className="h-4 w-4" />
                  {submittingRecon ? 'Saving...' : 'Save Reconciliation Record'}
                </Button>
              </CardContent>
            </Card>

            {/* Reconciliation Runs History */}
            <Card className="md:col-span-2">
              <CardHeader className="p-4 pb-2">
                <CardTitle className="text-base font-semibold">
                  Statement Reconciliation History
                </CardTitle>
                <CardDescription className="text-xs">
                  Historical statement reconciliation runs for the selected scrip.
                </CardDescription>
              </CardHeader>
              <CardContent className="p-4 pt-2">
                {!reconScripId ? (
                  <div className="py-8 text-center text-sm text-muted-foreground">
                    Select a RoDTEP scrip on the left to view historical reconciliation runs.
                  </div>
                ) : reconciliationList.length === 0 ? (
                  <div className="py-8 text-center text-sm text-muted-foreground">
                    No reconciliation runs logged yet for this scrip.
                  </div>
                ) : (
                  <div className="border rounded-lg overflow-hidden">
                    <Table className="text-xs">
                      <TableHeader>
                        <TableRow className="bg-muted/50">
                          <TableHead>Recon Date</TableHead>
                          <TableHead>Period End</TableHead>
                          <TableHead className="text-right">ICEGATE Bal</TableHead>
                          <TableHead className="text-right">ERP Bal</TableHead>
                          <TableHead className="text-right">Discrepancy</TableHead>
                          <TableHead>Reason</TableHead>
                          <TableHead>Reconciled By</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {reconciliationList.map((rec) => (
                          <TableRow key={rec.id}>
                            <TableCell>{rec.reconciliationDate}</TableCell>
                            <TableCell>{rec.statementPeriodEnd}</TableCell>
                            <TableCell className="text-right font-mono">
                              ₹{rec.icegateBalanceInr.toFixed(2)}
                            </TableCell>
                            <TableCell className="text-right font-mono">
                              ₹{rec.erpBalanceInr.toFixed(2)}
                            </TableCell>
                            <TableCell className="text-right font-mono font-bold">
                              {rec.discrepancyPaise === 0 ? (
                                <span className="text-emerald-600">₹0.00</span>
                              ) : (
                                <span className="text-destructive">
                                  ₹{rec.discrepancyInr.toFixed(2)}
                                </span>
                              )}
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {rec.discrepancyReason || '—'}
                            </TableCell>
                            <TableCell>{rec.reconciledBy}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>

      {/* MODAL: ADD NEW SCRIP */}
      <Dialog open={isNewScripOpen} onOpenChange={setIsNewScripOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base flex items-center gap-2">
              <Award className="h-5 w-5 text-primary" />
              Add RoDTEP Duty Credit Scrip
            </DialogTitle>
            <DialogDescription className="text-xs">
              Record a newly generated scrip or enter existing opening balance credit.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 py-2 text-xs">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">Scrip Number *</Label>
                <Input
                  placeholder="e.g. 2607054331"
                  value={newScrip.scripNumber || ''}
                  onChange={(e) => setNewScrip({ ...newScrip, scripNumber: e.target.value })}
                  className="font-mono text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Registration Port *</Label>
                <Select
                  value={newScrip.scripPortCode || 'INMAA1'}
                  onValueChange={(val: 'INMAA1' | 'INMAA4') =>
                    setNewScrip({ ...newScrip, scripPortCode: val })
                  }
                >
                  <SelectTrigger className="h-9 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="INMAA1">INMAA1 (Chennai Sea)</SelectItem>
                    <SelectItem value="INMAA4">INMAA4 (Chennai Air)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">Acquisition Type</Label>
                <Select
                  value={newScrip.acquisitionType || 'DIRECT_GENERATION'}
                  onValueChange={(val: any) =>
                    setNewScrip({ ...newScrip, acquisitionType: val })
                  }
                >
                  <SelectTrigger className="h-9 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="DIRECT_GENERATION">Direct Generation</SelectItem>
                    <SelectItem value="OPENING_BALANCE">Opening Balance</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Issue Date *</Label>
                <Input
                  type="date"
                  value={newScrip.issueDate || ''}
                  onChange={(e) => setNewScrip({ ...newScrip, issueDate: e.target.value })}
                  className="text-xs"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">Original Value (₹) *</Label>
                <Input
                  type="number"
                  step="0.01"
                  min="0"
                  placeholder="0.00"
                  value={newOriginalINR}
                  onChange={(e) => setNewOriginalINR(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Opening Balance (₹)</Label>
                <Input
                  type="number"
                  step="0.01"
                  min="0"
                  placeholder="Defaults to original"
                  value={newOpeningINR}
                  onChange={(e) => setNewOpeningINR(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">Statutory Expiry Date (Optional)</Label>
              <Input
                type="date"
                placeholder="Defaults to +24 months"
                value={newScrip.expiryDate || ''}
                onChange={(e) => setNewScrip({ ...newScrip, expiryDate: e.target.value })}
                className="text-xs"
              />
              <span className="text-[10px] text-muted-foreground">
                Leave empty to automatically calculate 24-month validity per Notification 79/2022-Customs.
              </span>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">Document Reference / Statement</Label>
              <Input
                placeholder="e.g. ICEGATE_ECL_STMT_OCT_2026.PDF"
                value={newScrip.documentRef || ''}
                onChange={(e) => setNewScrip({ ...newScrip, documentRef: e.target.value })}
                className="text-xs"
              />
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">Remarks</Label>
              <Input
                placeholder="Optional remarks"
                value={newScrip.remarks || ''}
                onChange={(e) => setNewScrip({ ...newScrip, remarks: e.target.value })}
                className="text-xs"
              />
            </div>
          </div>

          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsNewScripOpen(false)}
              disabled={submittingScrip}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={handleCreateScrip}
              disabled={submittingScrip}
              className="gap-1.5"
            >
              <Plus className="h-4 w-4" />
              {submittingScrip ? 'Saving...' : 'Save RoDTEP Scrip'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* MODAL: VERIFY OPENING BALANCE */}
      <Dialog open={!!verifyTarget} onOpenChange={(o) => !o && setVerifyTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base flex items-center gap-2">
              <ShieldCheck className="h-5 w-5 text-emerald-600" />
              Verify Opening Balance
            </DialogTitle>
            <DialogDescription className="text-xs">
              Confirm that scrip #{verifyTarget?.scripNumber} opening balance of ₹
              {verifyTarget?.confirmedBalInr.toFixed(2)} matches the official ICEGATE statement.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 py-2 text-xs">
            <div className="space-y-1.5">
              <Label className="text-xs">Supporting Evidence / Statement Document *</Label>
              <Input
                placeholder="e.g. ICEGATE_ECL_STMT_20261026.PDF"
                value={verifyEvidenceRef}
                onChange={(e) => setVerifyEvidenceRef(e.target.value)}
                className="text-xs"
              />
            </div>
          </div>

          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setVerifyTarget(null)}
              disabled={submittingVerify}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={handleVerifyOpening}
              disabled={submittingVerify || !verifyEvidenceRef.trim()}
              className="gap-1.5"
            >
              <ShieldCheck className="h-4 w-4" />
              {submittingVerify ? 'Verifying...' : 'Confirm Verification'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* MODAL: VIEW AUDIT LOGS */}
      <Dialog open={!!auditTarget} onOpenChange={(o) => !o && setAuditTarget(null)}>
        <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col p-6">
          <DialogHeader>
            <DialogTitle className="text-base flex items-center gap-2">
              <History className="h-5 w-5 text-primary" />
              Statutory Audit Trail — Scrip #{auditTarget?.scripNumber}
            </DialogTitle>
            <DialogDescription className="text-xs">
              Append-only audit log tracking all allocations, confirmations, reversals, and status transitions.
            </DialogDescription>
          </DialogHeader>

          <div className="flex-1 overflow-y-auto py-2">
            {loadingAudit ? (
              <div className="py-8 text-center text-xs text-muted-foreground">Loading audit records...</div>
            ) : auditLogs.length === 0 ? (
              <div className="py-8 text-center text-xs text-muted-foreground">No audit entries found.</div>
            ) : (
              <div className="space-y-3">
                {auditLogs.map((log) => (
                  <div key={log.id} className="p-3 rounded-lg border bg-muted/30 text-xs space-y-1">
                    <div className="flex items-center justify-between">
                      <Badge variant="outline" className="text-[10px] font-semibold">
                        {log.eventType}
                      </Badge>
                      <span className="text-[11px] text-muted-foreground">{log.performedAt}</span>
                    </div>
                    <div className="font-medium text-foreground">{log.reason}</div>
                    <div className="text-[11px] text-muted-foreground">
                      Performed by: <span className="font-semibold">{log.performedBy}</span>
                      {log.amountPaise ? ` • Amount: ₹${(log.amountPaise / 100.0).toFixed(2)}` : ''}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setAuditTarget(null)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* MODAL: REVERSE DEBIT */}
      <Dialog open={!!reversalTarget} onOpenChange={(o) => !o && setReversalTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base flex items-center gap-2">
              <RotateCcw className="h-5 w-5 text-amber-600" />
              Reverse Confirmed Customs Debit
            </DialogTitle>
            <DialogDescription className="text-xs">
              Record an append-only reversal re-crediting scrip balance (e.g. customs reassessment, amendment).
            </DialogDescription>
          </DialogHeader>

          {reversalTarget && (
            <div className="space-y-3 py-2 text-xs">
              <div className="p-2.5 rounded bg-muted/50 font-mono text-[11px] space-y-1">
                <div>Original Tx: {reversalTarget.transactionRef}</div>
                <div>Scrip: {reversalTarget.scripNumber}</div>
                <div>Original Amount: ₹{reversalTarget.amountInr.toFixed(2)}</div>
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Reversal Amount (₹) *</Label>
                <Input
                  type="number"
                  step="0.01"
                  min="0.01"
                  max={reversalTarget.amountInr}
                  value={reversalAmountINR}
                  onChange={(e) => setReversalAmountINR(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Reason for Reversal *</Label>
                <Input
                  placeholder="e.g. Section 149 customs reassessment reduction"
                  value={reversalReason}
                  onChange={(e) => setReversalReason(e.target.value)}
                  className="text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs">Customs Amendment / Document Ref</Label>
                <Input
                  placeholder="e.g. CUSTOMS_REASSESSMENT_ORDER_12.PDF"
                  value={reversalDocRef}
                  onChange={(e) => setReversalDocRef(e.target.value)}
                  className="text-xs"
                />
              </div>
            </div>
          )}

          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setReversalTarget(null)}
              disabled={submittingReversal}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={handleSubmitReversal}
              disabled={submittingReversal || !reversalReason.trim()}
              className="gap-1.5"
            >
              <RotateCcw className="h-4 w-4" />
              {submittingReversal ? 'Processing...' : 'Confirm Reversal'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 8. Generic Excel / CSV Scrip Import Modal */}
      <RoDTEPImportModal
        open={isImportModalOpen}
        onOpenChange={setIsImportModalOpen}
        existingScripNumbers={existingScripNumbers}
        onSuccess={loadData}
      />
    </div>
  );
}
