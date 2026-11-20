import * as React from 'react';
import {
  Banknote,
  CreditCard,
  DollarSign,
  Download,
  FileCheck2,
  FileSpreadsheet,
  FileText,
  Plus,
  RefreshCw,
  Search,
  Sparkles,
  Wallet,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
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
import { useAuth } from '@/contexts/AuthContext';
import { exportPaymentAnnexureExcel, exportPaymentAnnexurePdf } from '@/lib/payment-annexure-export';
import { supplierService, type Supplier } from '@/services/master-service';
import {
  paymentPOService,
  type InvoicePaymentCandidate,
  type Payment,
  type PaymentAllocationInput,
  type PaymentAnnexure,
  type PaymentInput,
  type PurchaseOrder,
} from '@/services/payment-po-service';

export function PaymentsPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('data.write');

  const [payments, setPayments] = React.useState<Payment[]>([]);
  const [suppliers, setSuppliers] = React.useState<Supplier[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [search, setSearch] = React.useState('');
  const [supplierFilter, setSupplierFilter] = React.useState('all');
  const [typeFilter, setTypeFilter] = React.useState('all');
  const page = 1;
  const [totalCount, setTotalCount] = React.useState(0);
  const pageSize = 15;

  // Modals
  const [createModalOpen, setCreateModalOpen] = React.useState(false);
  const [annexureModalOpen, setAnnexureModalOpen] = React.useState(false);
  const [currentAnnexure, setCurrentAnnexure] = React.useState<PaymentAnnexure | null>(null);
  const [submitting, setSubmitting] = React.useState(false);

  // Active creation workflow tab: 'multi_invoice' | 'single_invoice' | 'po_advance'
  const [paymentWorkflow, setPaymentWorkflow] = React.useState<'multi_invoice' | 'single_invoice' | 'po_advance'>('multi_invoice');

  // Common payment header fields
  const [paymentDate, setPaymentDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [selectedSupplierId, setSelectedSupplierId] = React.useState('');
  const [paymentMode, setPaymentMode] = React.useState('WIRE_TRANSFER');
  const [currency, setCurrency] = React.useState('USD');
  const [bankName] = React.useState('State Bank of India');
  const [bankAccountNo] = React.useState('30018899221');
  const [referenceNumber, setReferenceNumber] = React.useState('');
  const [paymentNotes, setPaymentNotes] = React.useState('');

  // Workflow 1: Multi-Invoice Fund Availability state
  const [availableFunds, setAvailableFunds] = React.useState<number>(50000);
  const [candidateInvoices, setCandidateInvoices] = React.useState<InvoicePaymentCandidate[]>([]);
  const [invoiceAllocations, setInvoiceAllocations] = React.useState<Record<string, number>>({});
  const [selectedInvoices, setSelectedInvoices] = React.useState<Record<string, boolean>>({});

  // Workflow 2: Single Invoice state
  const [singleInvoiceId, setSingleInvoiceId] = React.useState('');
  const [singleGross, setSingleGross] = React.useState<number>(0);
  const [singleAdvAdj, setSingleAdvAdj] = React.useState<number>(0);
  const [singleDeduct, setSingleDeduct] = React.useState<number>(0);

  // Workflow 3: PO Advance state
  const [supplierPOs, setSupplierPOs] = React.useState<PurchaseOrder[]>([]);
  const [selectedPOId, setSelectedPOId] = React.useState('');
  const [selectedMilestoneId, setSelectedMilestoneId] = React.useState('');
  const [advanceAmount, setAdvanceAmount] = React.useState<number>(0);

  const loadPayments = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await paymentPOService.listPayments({
        search: search.trim(),
        supplierId: supplierFilter === 'all' ? '' : supplierFilter,
        paymentType: typeFilter === 'all' ? '' : typeFilter,
        page,
        pageSize,
      });
      setPayments(res.items || []);
      setTotalCount(res.totalCount || 0);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search, supplierFilter, typeFilter, page]);

  React.useEffect(() => {
    loadPayments();
  }, [loadPayments]);

  React.useEffect(() => {
    supplierService.list({ page: 1, pageSize: 200, search: '', status: '', sortBy: '', sortDir: '' }).then((res) => {
      setSuppliers(res.items || []);
    }).catch(() => {});
  }, []);

  // When selected supplier changes in payment modal, load candidates & POs
  React.useEffect(() => {
    if (!selectedSupplierId) {
      setCandidateInvoices([]);
      setSupplierPOs([]);
      return;
    }

    // Load unpaid invoices for this supplier
    paymentPOService.listInvoiceCandidates(selectedSupplierId).then((candidates) => {
      setCandidateInvoices(candidates || []);
      // Reset allocations
      setInvoiceAllocations({});
      setSelectedInvoices({});
      if (candidates && candidates.length > 0 && candidates[0]?.currency) {
        setCurrency(candidates[0].currency);
      }
    }).catch(() => {});

    // Load POs for this supplier with advance balance
    paymentPOService.listPurchaseOrders({ supplierId: selectedSupplierId, page: 1, pageSize: 100 }).then((res) => {
      setSupplierPOs(res.items || []);
    }).catch(() => {});
  }, [selectedSupplierId]);

  const handleOpenCreateModal = () => {
    setSelectedSupplierId(suppliers[0]?.id || '');
    setPaymentDate(new Date().toISOString().slice(0, 10));
    setPaymentMode('WIRE_TRANSFER');
    setReferenceNumber(`UTR-${Date.now().toString().slice(-8)}`);
    setPaymentNotes('');
    setAvailableFunds(50000);
    setCreateModalOpen(true);
  };

  // Auto-allocate available funds across invoices by due date (FIFO)
  const handleAutoAllocate = () => {
    let budget = Number(availableFunds) || 0;
    const newAlloc: Record<string, number> = {};
    const newSel: Record<string, boolean> = {};

    // Invoices are already sorted chronologically (oldest / overdue first)
    for (const inv of candidateInvoices) {
      if (budget <= 0) break;
      const need = inv.pendingBalance;
      const allocate = Math.min(budget, need);
      if (allocate > 0) {
        newAlloc[inv.invoiceId] = Math.round(allocate * 100) / 100;
        newSel[inv.invoiceId] = true;
        budget -= allocate;
      }
    }

    setInvoiceAllocations(newAlloc);
    setSelectedInvoices(newSel);
    toast.success(`Allocated ${(availableFunds - budget).toFixed(2)} across selected invoices as per terms`);
  };

  const totalAllocatedBatch = React.useMemo(() => {
    return Object.entries(invoiceAllocations).reduce((acc, [invId, amount]) => {
      return selectedInvoices[invId] ? acc + (Number(amount) || 0) : acc;
    }, 0);
  }, [invoiceAllocations, selectedInvoices]);

  const remainingBudget = (Number(availableFunds) || 0) - totalAllocatedBatch;

  // Handle PO selection for Advance payment
  const handleSelectPO = (poId: string) => {
    setSelectedPOId(poId);
    const po = supplierPOs.find((p) => p.id === poId);
    if (!po) return;
    setCurrency(po.currency || 'USD');

    // Default to the first unpaid advance milestone
    const pendingMs = po.milestones?.find((m) => m.stageType === 'ADVANCE' && m.status !== 'PAID') || po.milestones?.[0];
    if (pendingMs) {
      setSelectedMilestoneId(pendingMs.id);
      setAdvanceAmount(pendingMs.balanceAmount || pendingMs.amount);
    } else {
      setSelectedMilestoneId('');
      setAdvanceAmount(po.advanceBalance || 0);
    }
  };

  const handleSubmitPayment = async () => {
    if (!selectedSupplierId) {
      toast.error('Please select a supplier');
      return;
    }

    setSubmitting(true);
    try {
      let allocations: PaymentAllocationInput[] = [];
      let totalAmount = 0;
      let payType = 'MULTI_INVOICE_BATCH';

      if (paymentWorkflow === 'multi_invoice') {
        payType = 'MULTI_INVOICE_BATCH';
        for (const inv of candidateInvoices) {
          const allocAmt = invoiceAllocations[inv.invoiceId];
          if (selectedInvoices[inv.invoiceId] && allocAmt && allocAmt > 0) {
            allocations.push({
              invoiceId: inv.invoiceId,
              shipmentId: inv.shipmentId,
              grossAmount: allocAmt,
              advanceAdjusted: 0,
              deductions: 0,
              notes: `Batch payment as per ${inv.paymentTerm}`,
            });
            totalAmount += allocAmt;
          }
        }
        if (allocations.length === 0) {
          toast.error('Please select at least one invoice and allocate funds');
          setSubmitting(false);
          return;
        }
      } else if (paymentWorkflow === 'single_invoice') {
        payType = 'SINGLE_INVOICE';
        const inv = candidateInvoices.find((x) => x.invoiceId === singleInvoiceId);
        if (!inv) {
          toast.error('Please select an invoice');
          setSubmitting(false);
          return;
        }
        const netAmt = Math.max(0, singleGross - singleAdvAdj - singleDeduct);
        allocations.push({
          invoiceId: inv.invoiceId,
          shipmentId: inv.shipmentId,
          grossAmount: singleGross,
          advanceAdjusted: singleAdvAdj,
          deductions: singleDeduct,
          notes: `Single invoice payment`,
        });
        totalAmount = netAmt;
      } else if (paymentWorkflow === 'po_advance') {
        payType = 'PO_ADVANCE';
        if (!selectedPOId) {
          toast.error('Please select a purchase order');
          setSubmitting(false);
          return;
        }
        if (advanceAmount <= 0) {
          toast.error('Advance amount must be greater than zero');
          setSubmitting(false);
          return;
        }
        allocations.push({
          poId: selectedPOId,
          poMilestoneId: selectedMilestoneId,
          grossAmount: advanceAmount,
          advanceAdjusted: 0,
          deductions: 0,
          notes: `Advance payment for PO`,
        });
        totalAmount = advanceAmount;
      }

      const payload: PaymentInput = {
        paymentDate,
        supplierId: selectedSupplierId,
        paymentType: payType,
        paymentMode,
        currency,
        exchangeRate: 1.0,
        totalAmount,
        bankName,
        bankAccountNo,
        referenceNumber,
        notes: paymentNotes,
        allocations,
      };

      const created = await paymentPOService.createPayment(payload);
      toast.success(`Payment voucher ${created.voucherNumber} created successfully`);
      setCreateModalOpen(false);
      loadPayments();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const handleOpenAnnexure = async (paymentId: string) => {
    try {
      const annx = await paymentPOService.generatePaymentAnnexure(paymentId, 'Acme Global Imports Ltd.');
      setCurrentAnnexure(annx);
      setAnnexureModalOpen(true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="space-y-6 p-6">
      {/* Header */}
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Payments & Annexure Generator</h1>
          <p className="text-sm text-muted-foreground">
            Execute batch multi-invoice payments based on fund availability, pay individual invoices, disburse PO advances, and create audit-ready payment annexures.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={loadPayments}>
            <RefreshCw className="mr-2 h-4 w-4" /> Refresh
          </Button>
          {canWrite && (
            <Button size="sm" onClick={handleOpenCreateModal}>
              <Plus className="mr-2 h-4 w-4" /> Process Payment
            </Button>
          )}
        </div>
      </div>

      {/* Overview Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card className="border-border/50">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Payment Vouchers</CardTitle>
            <ReceiptIcon className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalCount}</div>
            <p className="text-xs text-muted-foreground">Processed payment vouchers</p>
          </CardContent>
        </Card>

        <Card className="border-border/50">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">PO Advances Paid</CardTitle>
            <Banknote className="h-4 w-4 text-blue-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">
              {payments.filter((p) => p.paymentType === 'PO_ADVANCE').length}
            </div>
            <p className="text-xs text-muted-foreground">Staged advance disbursements</p>
          </CardContent>
        </Card>

        <Card className="border-border/50">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Multi-Invoice Batches</CardTitle>
            <FileSpreadsheet className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">
              {payments.filter((p) => p.paymentType === 'MULTI_INVOICE_BATCH').length}
            </div>
            <p className="text-xs text-muted-foreground">Fund availability allocations</p>
          </CardContent>
        </Card>

        <Card className="border-border/50">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-xs font-semibold uppercase text-muted-foreground">Total Disbursed</CardTitle>
            <DollarSign className="h-4 w-4 text-emerald-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-emerald-600">
              {payments.reduce((acc, p) => acc + (p.totalAmount || 0), 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
            </div>
            <p className="text-xs text-muted-foreground">Total remittance across currencies</p>
          </CardContent>
        </Card>
      </div>

      {/* Filter Toolbar */}
      <Card>
        <CardContent className="p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex flex-1 flex-wrap items-center gap-2">
              <div className="relative min-w-[240px] flex-1 sm:max-w-xs">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search voucher, ref#, bank, notes..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="pl-9 text-xs"
                />
              </div>

              <Select value={supplierFilter} onValueChange={setSupplierFilter}>
                <SelectTrigger className="w-[180px] text-xs">
                  <SelectValue placeholder="All Suppliers" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Suppliers</SelectItem>
                  {suppliers.map((s) => (
                    <SelectItem key={s.id} value={s.id}>{s.supplierName}</SelectItem>
                  ))}
                </SelectContent>
              </Select>

              <Select value={typeFilter} onValueChange={setTypeFilter}>
                <SelectTrigger className="w-[170px] text-xs">
                  <SelectValue placeholder="All Payment Types" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Types</SelectItem>
                  <SelectItem value="PO_ADVANCE">PO Advance</SelectItem>
                  <SelectItem value="SINGLE_INVOICE">Single Invoice</SelectItem>
                  <SelectItem value="MULTI_INVOICE_BATCH">Multi-Invoice Batch</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Payments Table */}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[140px]">Voucher No.</TableHead>
                <TableHead>Payment Date</TableHead>
                <TableHead>Supplier</TableHead>
                <TableHead>Payment Type</TableHead>
                <TableHead>Mode / Ref</TableHead>
                <TableHead>Remitting Bank</TableHead>
                <TableHead className="text-right">Total Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={9} className="h-32 text-center text-sm text-muted-foreground">
                    Loading Payments...
                  </TableCell>
                </TableRow>
              ) : payments.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={9} className="h-32 text-center text-sm text-muted-foreground">
                    No payment records found matching your filters.
                  </TableCell>
                </TableRow>
              ) : (
                payments.map((pay) => (
                  <TableRow key={pay.id} className="hover:bg-muted/40">
                    <TableCell className="font-semibold text-primary">{pay.voucherNumber}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{pay.paymentDate}</TableCell>
                    <TableCell className="font-medium">{pay.supplierName}</TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={
                          pay.paymentType === 'PO_ADVANCE'
                            ? 'border-blue-500 text-blue-700 bg-blue-50'
                            : pay.paymentType === 'MULTI_INVOICE_BATCH'
                            ? 'border-purple-500 text-purple-700 bg-purple-50'
                            : 'border-emerald-500 text-emerald-700 bg-emerald-50'
                        }
                      >
                        {pay.paymentType.replace(/_/g, ' ')}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs">
                      <div className="font-medium">{pay.paymentMode.replace(/_/g, ' ')}</div>
                      <div className="text-[11px] text-muted-foreground font-mono">{pay.referenceNumber || 'No Ref'}</div>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">{pay.bankName || 'Treasury'}</TableCell>
                    <TableCell className="text-right font-bold text-emerald-700">
                      {pay.currency} {pay.totalAmount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </TableCell>
                    <TableCell>
                      <Badge className="bg-emerald-600/15 text-emerald-700">Processed</Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-8 gap-1 text-xs"
                        onClick={() => handleOpenAnnexure(pay.id)}
                      >
                        <FileText className="h-3.5 w-3.5 text-primary" /> Annexure
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Process Payment Modal */}
      <Dialog open={createModalOpen} onOpenChange={setCreateModalOpen}>
        <DialogContent className="max-w-4xl max-h-[92vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Process Supplier Payment & Remittance</DialogTitle>
            <DialogDescription>
              Select payment workflow: Multi-Invoice Batch (with fund availability), Individual Invoice, or PO Advance milestone.
            </DialogDescription>
          </DialogHeader>

          {/* Workflow Tabs */}
          <Tabs
            value={paymentWorkflow}
            onValueChange={(val: any) => setPaymentWorkflow(val)}
            className="w-full"
          >
            <TabsList className="grid grid-cols-3 w-full">
              <TabsTrigger value="multi_invoice" className="text-xs gap-1.5">
                <FileSpreadsheet className="h-3.5 w-3.5" /> Multi-Invoice (Fund Budget)
              </TabsTrigger>
              <TabsTrigger value="single_invoice" className="text-xs gap-1.5">
                <CreditCard className="h-3.5 w-3.5" /> Individual Invoice
              </TabsTrigger>
              <TabsTrigger value="po_advance" className="text-xs gap-1.5">
                <Banknote className="h-3.5 w-3.5" /> PO Advance Payment
              </TabsTrigger>
            </TabsList>

            {/* Common Header Details */}
            <div className="grid grid-cols-1 md:grid-cols-4 gap-3 py-3 border-b mt-2">
              <div>
                <Label className="text-xs">Supplier *</Label>
                <Select value={selectedSupplierId} onValueChange={setSelectedSupplierId}>
                  <SelectTrigger className="text-xs mt-1">
                    <SelectValue placeholder="Select supplier" />
                  </SelectTrigger>
                  <SelectContent>
                    {suppliers.map((s) => (
                      <SelectItem key={s.id} value={s.id}>{s.supplierName}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div>
                <Label className="text-xs">Payment Date *</Label>
                <Input type="date" value={paymentDate} onChange={(e) => setPaymentDate(e.target.value)} className="text-xs mt-1" />
              </div>

              <div>
                <Label className="text-xs">Payment Mode *</Label>
                <Select value={paymentMode} onValueChange={setPaymentMode}>
                  <SelectTrigger className="text-xs mt-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="WIRE_TRANSFER">Wire Transfer / TT</SelectItem>
                    <SelectItem value="RTGS">RTGS</SelectItem>
                    <SelectItem value="NEFT">NEFT</SelectItem>
                    <SelectItem value="LC">Letter of Credit (LC)</SelectItem>
                    <SelectItem value="CHEQUE">Cheque</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div>
                <Label className="text-xs">Bank UTR / Ref Number *</Label>
                <Input
                  value={referenceNumber}
                  onChange={(e) => setReferenceNumber(e.target.value)}
                  placeholder="e.g. UTR-202611-988"
                  className="text-xs mt-1 font-mono"
                />
              </div>
            </div>

            {/* TAB 1: Multi-Invoice Batch Payment based on Fund Availability */}
            <TabsContent value="multi_invoice" className="space-y-4 pt-2">
              <div className="rounded-lg border bg-gradient-to-r from-blue-50/50 to-indigo-50/30 p-4 border-blue-200">
                <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
                  <div>
                    <h4 className="text-xs font-bold text-blue-900 flex items-center gap-1.5">
                      <Wallet className="h-4 w-4 text-blue-600" /> Fund Availability Budget Engine
                    </h4>
                    <p className="text-[11px] text-blue-700 mt-0.5">
                      Specify available treasury funds. The system will distribute funds across unpaid invoices according to terms and due dates.
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <div className="w-36">
                      <Label className="text-[10px] text-blue-900 font-semibold">Available Fund ({currency})</Label>
                      <Input
                        type="number"
                        value={availableFunds}
                        onChange={(e) => setAvailableFunds(Number(e.target.value))}
                        className="h-8 text-xs font-bold bg-white"
                      />
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      onClick={handleAutoAllocate}
                      className="h-8 text-xs bg-blue-600 hover:bg-blue-700 text-white mt-3"
                    >
                      <Sparkles className="mr-1 h-3.5 w-3.5" /> Auto-Allocate (FIFO)
                    </Button>
                  </div>
                </div>

                {/* Utilization Meter */}
                <div className="mt-3 flex items-center justify-between text-xs font-medium pt-2 border-t border-blue-200/60">
                  <div className="flex gap-4">
                    <span>Budget: <strong>{currency} {availableFunds.toLocaleString(undefined, { minimumFractionDigits: 2 })}</strong></span>
                    <span className="text-emerald-700">Allocated: <strong>{currency} {totalAllocatedBatch.toLocaleString(undefined, { minimumFractionDigits: 2 })}</strong></span>
                    <span className={remainingBudget < 0 ? 'text-destructive font-bold' : 'text-blue-700'}>
                      Remaining: <strong>{currency} {remainingBudget.toLocaleString(undefined, { minimumFractionDigits: 2 })}</strong>
                    </span>
                  </div>
                </div>
              </div>

              {/* Invoices List */}
              <div className="border rounded-md overflow-x-auto">
                <Table>
                  <TableHeader className="bg-muted/50 text-[11px]">
                    <TableRow>
                      <TableHead className="w-[40px]"></TableHead>
                      <TableHead>Invoice #</TableHead>
                      <TableHead>Date</TableHead>
                      <TableHead>Payment Term</TableHead>
                      <TableHead>Due Date</TableHead>
                      <TableHead className="text-right">Gross Amount</TableHead>
                      <TableHead className="text-right">Already Paid</TableHead>
                      <TableHead className="text-right">Pending Balance</TableHead>
                      <TableHead className="w-[140px] text-right">Fund Allocated ({currency})</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {candidateInvoices.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={9} className="h-24 text-center text-xs text-muted-foreground">
                          No outstanding invoices found for this supplier.
                        </TableCell>
                      </TableRow>
                    ) : (
                      candidateInvoices.map((inv) => {
                        const isSelected = !!selectedInvoices[inv.invoiceId];
                        const allocAmt = invoiceAllocations[inv.invoiceId] || 0;
                        return (
                          <TableRow key={inv.invoiceId} className={isSelected ? 'bg-primary/5' : ''}>
                            <TableCell className="p-2 text-center">
                              <input
                                type="checkbox"
                                checked={isSelected}
                                onChange={(e) => {
                                  const checked = e.target.checked;
                                  setSelectedInvoices((prev) => ({ ...prev, [inv.invoiceId]: checked }));
                                  if (checked && !invoiceAllocations[inv.invoiceId]) {
                                    setInvoiceAllocations((prev) => ({ ...prev, [inv.invoiceId]: inv.pendingBalance }));
                                  }
                                }}
                                className="h-4 w-4 rounded border-gray-300 text-primary"
                              />
                            </TableCell>
                            <TableCell className="font-semibold text-primary text-xs">{inv.invoiceNumber}</TableCell>
                            <TableCell className="text-xs text-muted-foreground">{inv.invoiceDate}</TableCell>
                            <TableCell><Badge variant="outline" className="text-[11px]">{inv.paymentTerm}</Badge></TableCell>
                            <TableCell className="text-xs">
                              {inv.dueDate}
                              {inv.isOverdue && (
                                <Badge className="ml-1.5 bg-red-100 text-red-800 text-[10px]">
                                  {inv.daysOverdue}d overdue
                                </Badge>
                              )}
                            </TableCell>
                            <TableCell className="text-right text-xs font-medium">
                              {inv.currency} {inv.grossAmount.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                            </TableCell>
                            <TableCell className="text-right text-xs text-muted-foreground">
                              {inv.alreadyPaid > 0 ? `${inv.currency} ${inv.alreadyPaid.toFixed(2)}` : '-'}
                            </TableCell>
                            <TableCell className="text-right text-xs font-semibold text-foreground">
                              {inv.currency} {inv.pendingBalance.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                            </TableCell>
                            <TableCell className="p-1.5 text-right">
                              <Input
                                type="number"
                                step="0.01"
                                min="0"
                                max={inv.pendingBalance}
                                value={allocAmt || ''}
                                placeholder="0.00"
                                onChange={(e) => {
                                  const val = Number(e.target.value) || 0;
                                  setInvoiceAllocations((prev) => ({ ...prev, [inv.invoiceId]: val }));
                                  if (val > 0) {
                                    setSelectedInvoices((prev) => ({ ...prev, [inv.invoiceId]: true }));
                                  }
                                }}
                                className="h-7 text-xs text-right font-medium"
                              />
                            </TableCell>
                          </TableRow>
                        );
                      })
                    )}
                  </TableBody>
                </Table>
              </div>
            </TabsContent>

            {/* TAB 2: Individual Invoice Payment */}
            <TabsContent value="single_invoice" className="space-y-4 pt-2">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-xs">Select Commercial Invoice *</Label>
                  <Select
                    value={singleInvoiceId}
                    onValueChange={(val) => {
                      setSingleInvoiceId(val);
                      const inv = candidateInvoices.find((x) => x.invoiceId === val);
                      if (inv) {
                        setSingleGross(inv.pendingBalance);
                        setSingleAdvAdj(Math.min(inv.pendingBalance, inv.availablePoAdvance || 0));
                        setSingleDeduct(0);
                      }
                    }}
                  >
                    <SelectTrigger className="text-xs mt-1">
                      <SelectValue placeholder="Choose invoice to pay..." />
                    </SelectTrigger>
                    <SelectContent>
                      {candidateInvoices.map((inv) => (
                        <SelectItem key={inv.invoiceId} value={inv.invoiceId}>
                          {inv.invoiceNumber} - Balance: {inv.currency} {inv.pendingBalance.toFixed(2)} (Due: {inv.dueDate})
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div>
                  <Label className="text-xs">Gross Payable Amount ({currency}) *</Label>
                  <Input
                    type="number"
                    step="0.01"
                    value={singleGross}
                    onChange={(e) => setSingleGross(Number(e.target.value))}
                    className="text-xs mt-1"
                  />
                </div>

                <div>
                  <Label className="text-xs">Advance Adjusted from PO ({currency})</Label>
                  <Input
                    type="number"
                    step="0.01"
                    value={singleAdvAdj}
                    onChange={(e) => setSingleAdvAdj(Number(e.target.value))}
                    placeholder="Deduct from prior advance"
                    className="text-xs mt-1"
                  />
                  <span className="text-[10px] text-muted-foreground">Adjust from advance paid earlier on this order</span>
                </div>

                <div>
                  <Label className="text-xs">Statutory Deductions / TDS ({currency})</Label>
                  <Input
                    type="number"
                    step="0.01"
                    value={singleDeduct}
                    onChange={(e) => setSingleDeduct(Number(e.target.value))}
                    placeholder="TDS or bank charge deductions"
                    className="text-xs mt-1"
                  />
                </div>
              </div>

              <div className="rounded-lg bg-muted/40 p-3 flex justify-between items-center text-sm font-semibold">
                <span>Net Remittance Payable:</span>
                <span className="text-base text-emerald-700 font-bold">
                  {currency} {Math.max(0, singleGross - singleAdvAdj - singleDeduct).toFixed(2)}
                </span>
              </div>
            </TabsContent>

            {/* TAB 3: PO Advance Payment */}
            <TabsContent value="po_advance" className="space-y-4 pt-2">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-xs">Select Purchase Order *</Label>
                  <Select value={selectedPOId} onValueChange={handleSelectPO}>
                    <SelectTrigger className="text-xs mt-1">
                      <SelectValue placeholder="Choose Purchase Order..." />
                    </SelectTrigger>
                    <SelectContent>
                      {supplierPOs.map((po) => (
                        <SelectItem key={po.id} value={po.id}>
                          {po.poNumber} - Total: {po.currency} {po.totalAmount.toFixed(2)} | Adv Bal: {po.advanceBalance.toFixed(2)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div>
                  <Label className="text-xs">PO Milestone Stage</Label>
                  <Select value={selectedMilestoneId} onValueChange={setSelectedMilestoneId}>
                    <SelectTrigger className="text-xs mt-1">
                      <SelectValue placeholder="Select milestone stage" />
                    </SelectTrigger>
                    <SelectContent>
                      {supplierPOs
                        .find((p) => p.id === selectedPOId)
                        ?.milestones?.map((ms) => (
                          <SelectItem key={ms.id} value={ms.id}>
                            {ms.milestoneName} ({ms.percentage}%) - Due: {currency} {ms.balanceAmount.toFixed(2)}
                          </SelectItem>
                        ))}
                    </SelectContent>
                  </Select>
                </div>

                <div>
                  <Label className="text-xs">Advance Amount to Disburse ({currency}) *</Label>
                  <Input
                    type="number"
                    step="0.01"
                    value={advanceAmount}
                    onChange={(e) => setAdvanceAmount(Number(e.target.value))}
                    className="text-xs mt-1 font-bold text-primary"
                  />
                </div>
              </div>
            </TabsContent>
          </Tabs>

          <DialogFooter className="mt-4">
            <Button variant="outline" onClick={() => setCreateModalOpen(false)}>Cancel</Button>
            <Button onClick={handleSubmitPayment} disabled={submitting}>
              {submitting ? 'Processing Payment...' : 'Confirm & Process Payment'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Payment Annexure View & Export Dialog */}
      {currentAnnexure && (
        <Dialog open={annexureModalOpen} onOpenChange={setAnnexureModalOpen}>
          <DialogContent className="max-w-4xl max-h-[92vh] overflow-y-auto">
            <DialogHeader>
              <div className="flex items-center justify-between">
                <div>
                  <DialogTitle className="text-lg flex items-center gap-2">
                    <FileCheck2 className="h-5 w-5 text-emerald-600" />
                    Payment Annexure & Remittance Advice
                  </DialogTitle>
                  <DialogDescription>
                    Annexure Ref: {currentAnnexure.annexureNumber} | Voucher: {currentAnnexure.paymentVoucherNumber}
                  </DialogDescription>
                </div>
                <div className="flex gap-2">
                  <Button size="sm" variant="outline" onClick={() => exportPaymentAnnexureExcel(currentAnnexure)}>
                    <FileSpreadsheet className="mr-1.5 h-3.5 w-3.5 text-emerald-600" /> Excel
                  </Button>
                  <Button size="sm" onClick={() => exportPaymentAnnexurePdf(currentAnnexure)}>
                    <Download className="mr-1.5 h-3.5 w-3.5" /> PDF
                  </Button>
                </div>
              </div>
            </DialogHeader>

            <div className="space-y-4 py-2 text-xs">
              {/* Remitter & Beneficiary Cards */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="rounded-lg border p-3 bg-muted/30">
                  <span className="font-bold text-[11px] text-muted-foreground uppercase">Remitter Details</span>
                  <div className="mt-1 font-semibold text-sm">{currentAnnexure.remitterCompany}</div>
                  <div className="text-muted-foreground">Bank: {currentAnnexure.remitterBank}</div>
                  <div className="text-muted-foreground">Account: {currentAnnexure.remitterAccount}</div>
                  <div className="text-muted-foreground font-mono">Bank Ref / UTR: {currentAnnexure.referenceNumber}</div>
                </div>

                <div className="rounded-lg border p-3 bg-muted/30">
                  <span className="font-bold text-[11px] text-muted-foreground uppercase">Beneficiary Details</span>
                  <div className="mt-1 font-semibold text-sm">{currentAnnexure.beneficiaryName || currentAnnexure.supplierName}</div>
                  <div className="text-muted-foreground">Country: {currentAnnexure.supplierCountry}</div>
                  <div className="text-muted-foreground">Bank: {currentAnnexure.beneficiaryBank || 'N/A'}</div>
                  <div className="text-muted-foreground font-mono">Account / IBAN: {currentAnnexure.beneficiaryAccount || currentAnnexure.beneficiaryIban || 'N/A'}</div>
                  <div className="text-muted-foreground font-mono">SWIFT: {currentAnnexure.beneficiarySwift || 'N/A'}</div>
                </div>
              </div>

              {/* Items Table */}
              <div className="border rounded-md overflow-x-auto">
                <Table>
                  <TableHeader className="bg-muted/50 text-[11px]">
                    <TableRow>
                      <TableHead className="w-[30px]">#</TableHead>
                      <TableHead>Doc Type</TableHead>
                      <TableHead>Doc Number</TableHead>
                      <TableHead>Date</TableHead>
                      <TableHead className="text-right">Gross ({currentAnnexure.currency})</TableHead>
                      <TableHead className="text-right">Adv. Adjusted</TableHead>
                      <TableHead className="text-right">Deductions</TableHead>
                      <TableHead className="text-right font-bold">Net Remittance</TableHead>
                      <TableHead>Terms / Milestone</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {currentAnnexure.items.map((it) => (
                      <TableRow key={it.slNo}>
                        <TableCell className="font-medium">{it.slNo}</TableCell>
                        <TableCell><Badge variant="outline">{it.documentType}</Badge></TableCell>
                        <TableCell className="font-semibold text-primary">{it.documentNumber}</TableCell>
                        <TableCell className="text-muted-foreground">{it.documentDate}</TableCell>
                        <TableCell className="text-right">{it.grossAmount.toFixed(2)}</TableCell>
                        <TableCell className="text-right text-orange-600">
                          {it.advanceAdjusted > 0 ? `-${it.advanceAdjusted.toFixed(2)}` : '0.00'}
                        </TableCell>
                        <TableCell className="text-right text-red-600">
                          {it.deductions > 0 ? `-${it.deductions.toFixed(2)}` : '0.00'}
                        </TableCell>
                        <TableCell className="text-right font-bold text-emerald-700">{it.netAmount.toFixed(2)}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">{it.paymentTermMilestone}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {/* Summary & Amount in Words */}
              <div className="rounded-lg bg-emerald-50/60 border border-emerald-200 p-3 space-y-1">
                <div className="flex justify-between text-sm font-bold text-emerald-950">
                  <span>Total Net Remittance:</span>
                  <span>{currentAnnexure.currency} {currentAnnexure.totalNetAmount.toLocaleString(undefined, { minimumFractionDigits: 2 })}</span>
                </div>
                <div className="text-xs text-emerald-900 font-medium">
                  <strong>Amount in Words:</strong> {currentAnnexure.netAmountInWords}
                </div>
              </div>

              {/* Signatures */}
              <div className="grid grid-cols-3 gap-6 pt-4 border-t text-center text-xs text-muted-foreground">
                <div className="border-t pt-1">Prepared By ({currentAnnexure.preparedBy || 'Accounts Executive'})</div>
                <div className="border-t pt-1">Checked By ({currentAnnexure.checkedBy || 'Finance Manager'})</div>
                <div className="border-t pt-1 font-semibold text-foreground">Approved By ({currentAnnexure.approvedBy || 'Authorized Signatory'})</div>
              </div>
            </div>

            <DialogFooter>
              <Button onClick={() => setAnnexureModalOpen(false)}>Close</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

function ReceiptIcon(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M4 2v20l2-1 2 1 2-1 2 1 2-1 2 1 2-1 2 1V2l-2 1-2-1-2 1-2-1-2 1-2-1-2 1Z" />
      <path d="M16 8h-6a2 2 0 1 0 0 4h4a2 2 0 1 1 0 4H8" />
      <path d="M12 17.5v-11" />
    </svg>
  );
}
