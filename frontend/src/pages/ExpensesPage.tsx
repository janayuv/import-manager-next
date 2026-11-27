import * as React from 'react';
import {
  AlertTriangle,
  ArrowLeft,
  ChevronDown,
  ChevronRight,
  ChevronsDownUp,
  Copy,
  FileSpreadsheet,
  History,
  Layers,
  Pencil,
  Plus,
  Receipt,
  RefreshCw,
  RotateCcw,
  Search,
  Settings2,
  Trash2,
  Upload,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchableCombobox } from '@/components/ui/searchable-combobox';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ManageExpenseTypesDialog } from '@/components/master/ManageExpenseTypesDialog';
import { ManageServiceProvidersDialog } from '@/components/master/ManageServiceProvidersDialog';
import { ExpenseCatalogAndLedgerTabs } from '@/components/expenses/ExpenseCatalogAndLedgerTabs';
import { ExpenseBulkImportDialog, ExpensePasteLinesDialog } from '@/components/expenses/ExpenseBulkAndPasteDialogs';
import {
  analyzeDraftLines,
  draftLinesToInputs,
  ExpenseLinesGrid,
  makeEmptyDraftLine,
  makeLineKey,
  type DraftLine,
} from '@/components/expenses/ExpenseLinesGrid';
import { useAuth } from '@/contexts/AuthContext';
import { useCompany } from '@/contexts/CompanyContext';
import { useModuleSettings } from '@/contexts/ThemeContext';
import { downloadTextFile } from '@/lib/download';
import {
  adaptExpenseTypeRatesForSupply,
  calculateInvoiceLocal,
  detectSupplyType,
  MAX_ROUND_OFF_PAISE,
  parsePastedExpenseLines,
} from '@/lib/expense-calculator';
import {
  expenseService,
  formatPaiseINR,
  type ExpenseAllocationRecord,
  type ExpenseInvoice,
  type ExpenseInvoiceCalculation,
  type ExpenseType,
  type ExpenseTypeInput,
  type ServiceProvider,
  type ServiceProviderInput,
} from '@/services/expense-service';
import { shipmentService, type Shipment } from '@/services/shipment-invoice-service';

const EXPENSE_DRAFT_STORAGE_KEY = 'im_next_expense_invoice_draft_v1';

interface BatchBill {
  key: string;
  serviceProviderId: string;
  invoiceNo: string;
  invoiceDate: string;
  isPaidByClearingAgent: boolean;
  clearingAgentId: string;
  chaReimburseInvoiceNo: string;
  chaReimburseInvoiceDate: string;
  remarks: string;
  roundOffInr: string;
  idempotencyKey: string;
  lines: DraftLine[];
  collapsed: boolean;
}

interface StoredExpenseDraft {
  savedAt: number;
  shipmentId: string;
  serviceProviderId: string;
  invoiceNo: string;
  invoiceDate: string;
  isPaidByClearingAgent: boolean;
  clearingAgentId: string;
  chaReimburseInvoiceNo: string;
  chaReimburseInvoiceDate: string;
  remarks: string;
  roundOffInr: string;
  draftLines: DraftLine[];
}

function makeIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `exp-${crypto.randomUUID()}`;
  }
  return `exp-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

const emptyLine = makeEmptyDraftLine;
const toLineInputs = draftLinesToInputs;

export function ExpensesPage() {
  const { hasPermission } = useAuth();
  const { currentCompany } = useCompany();
  const canWrite = hasPermission('data.write');
  const canDelete = hasPermission('data.delete');

  const { itemsPerPage: expensePageSize } = useModuleSettings('expenses');
  const [invoices, setInvoices] = React.useState<ExpenseInvoice[]>([]);
  const [invoicePage, setInvoicePage] = React.useState(1);
  const [itemizedPage, setItemizedPage] = React.useState(1);
  const [providers, setProviders] = React.useState<ServiceProvider[]>([]);
  const [types, setTypes] = React.useState<ExpenseType[]>([]);
  const [shipments, setShipments] = React.useState<Shipment[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [search, setSearch] = React.useState('');
  const [shipmentFilter, setShipmentFilter] = React.useState('');

  // Multi-line Expense Invoice dialog state
  const [invOpen, setInvOpen] = React.useState(false);
  const [entryMode, setEntryMode] = React.useState<'single' | 'batch'>('single');
  const [editingInv, setEditingInv] = React.useState<ExpenseInvoice | null>(null);
  const [shipmentId, setShipmentId] = React.useState('');
  const [serviceProviderId, setServiceProviderId] = React.useState('');
  const [invoiceNo, setInvoiceNo] = React.useState('');
  const [invoiceDate, setInvoiceDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [isPaidByClearingAgent, setIsPaidByClearingAgent] = React.useState(false);
  const [clearingAgentId, setClearingAgentId] = React.useState('');
  const [chaReimburseInvoiceNo, setChaReimburseInvoiceNo] = React.useState('');
  const [chaReimburseInvoiceDate, setChaReimburseInvoiceDate] = React.useState('');
  const [remarks, setRemarks] = React.useState('');
  const [idempotencyKey, setIdempotencyKey] = React.useState('');
  const [draftLines, setDraftLines] = React.useState<DraftLine[]>([]);
  const [backendPreview, setBackendPreview] = React.useState<ExpenseInvoiceCalculation | null>(null);
  const [roundOffInr, setRoundOffInr] = React.useState('0');
  const [savingInv, setSavingInv] = React.useState(false);
  const [pasteOpen, setPasteOpen] = React.useState(false);
  const [pasteText, setPasteText] = React.useState('');
  const [pasteTargetBillKey, setPasteTargetBillKey] = React.useState<string | null>(null);
  const [recoverableDraft, setRecoverableDraft] = React.useState<StoredExpenseDraft | null>(null);
  const previewSeqRef = React.useRef(0);
  const previewTimerRef = React.useRef<number | null>(null);
  const lineAmountRefs = React.useRef<Record<string, HTMLInputElement | null>>({});
  const pendingFocusKeyRef = React.useRef<string | null>(null);

  // Multi-provider shipment batch state
  const [batchBills, setBatchBills] = React.useState<BatchBill[]>([]);
  const [sharedClearingAgentId, setSharedClearingAgentId] = React.useState('');
  const [sharedChaReimburseInvoiceNo, setSharedChaReimburseInvoiceNo] = React.useState('');
  const [sharedChaReimburseInvoiceDate, setSharedChaReimburseInvoiceDate] = React.useState('');
  const [targetBatchBillKey, setTargetBatchBillKey] = React.useState<string | null>(null);

  // Bulk import dialog
  const [bulkOpen, setBulkOpen] = React.useState(false);
  const [bulkShipmentId, setBulkShipmentId] = React.useState('');

  // Service Provider dialog
  const [spOpen, setSpOpen] = React.useState(false);
  const [editingSp, setEditingSp] = React.useState<ServiceProvider | null>(null);
  const [spForm, setSpForm] = React.useState<ServiceProviderInput>({
    name: '',
    gstin: '',
    state: '',
    contactPerson: '',
    contactEmail: '',
    contactPhone: '',
  });

  // Expense Type dialog
  const [etOpen, setEtOpen] = React.useState(false);
  const [editingEt, setEditingEt] = React.useState<ExpenseType | null>(null);
  const [etForm, setEtForm] = React.useState<ExpenseTypeInput>({
    name: '',
    defaultCgstRateBp: 0,
    defaultSgstRateBp: 0,
    defaultIgstRateBp: 0,
    defaultTdsRateBp: 0,
    isActive: true,
  });

  // Dedicated Manage Dialogs
  const [manageSpOpen, setManageSpOpen] = React.useState(false);
  const [manageEtOpen, setManageEtOpen] = React.useState(false);
  const [targetLineIdx, setTargetLineIdx] = React.useState<number | null>(null);

  // Tab state
  const [activeTab, setActiveTab] = React.useState('invoices');

  // Expandable invoice rows in Ledger
  const [expandedInvoices, setExpandedInvoices] = React.useState<Set<string>>(new Set());

  // Itemized Ledger Filter states
  const [itemizedSearch, setItemizedSearch] = React.useState('');
  const [itemizedTypeFilter, setItemizedTypeFilter] = React.useState('all');
  const [itemizedProviderFilter, setItemizedProviderFilter] = React.useState('all');
  const [itemizedShipmentFilter, setItemizedShipmentFilter] = React.useState('all');

  // Allocation Audit Ledger state
  const [ledgerRecords, setLedgerRecords] = React.useState<ExpenseAllocationRecord[]>([]);
  const [ledgerLoading, setLedgerLoading] = React.useState(false);
  const [ledgerShipmentFilter, setLedgerShipmentFilter] = React.useState('');

  const loadLedger = React.useCallback(async () => {
    setLedgerLoading(true);
    try {
      const records = await expenseService.listAllocationHistory({
        shipmentId: ledgerShipmentFilter || undefined,
      });
      setLedgerRecords(records ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLedgerLoading(false);
    }
  }, [ledgerShipmentFilter]);

  React.useEffect(() => {
    if (activeTab === 'ledger') {
      void loadLedger();
    }
  }, [activeTab, loadLedger]);

  const loadAll = React.useCallback(async () => {
    setLoading(true);
    try {
      const [invs, sps, ets, shPage] = await Promise.all([
        expenseService.listExpenseInvoices(shipmentFilter, search),
        expenseService.listServiceProviders(''),
        expenseService.listExpenseTypes(false),
        shipmentService.list({
          search: '',
          supplierId: '',
          status: '',
          frozenOnly: false,
          sortBy: 'invoiceDate',
          sortDir: 'desc',
          page: 1,
          pageSize: 500,
        }),
      ]);
      setInvoices(invs);
      setProviders(sps);
      setTypes(ets);
      setShipments(shPage.items ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [search, shipmentFilter]);

  React.useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const activeTypes = React.useMemo(
    () => types.filter((t) => t.isActive),
    [types],
  );

  const selectedProvider = React.useMemo(
    () => providers.find((p) => p.id === serviceProviderId) ?? null,
    [providers, serviceProviderId],
  );

  const supplyType = React.useMemo(
    () =>
      detectSupplyType(
        currentCompany?.gstin,
        currentCompany?.state,
        selectedProvider?.gstin,
        selectedProvider?.state,
      ),
    [currentCompany?.gstin, currentCompany?.state, selectedProvider?.gstin, selectedProvider?.state],
  );

  const scheduleBackendPreview = React.useCallback((lines: DraftLine[]) => {
    if (previewTimerRef.current !== null) {
      window.clearTimeout(previewTimerRef.current);
    }
    const inputs = toLineInputs(lines).filter((l) => l.expenseTypeId && l.amountPaise > 0);
    if (inputs.length === 0) {
      setBackendPreview(null);
      return;
    }
    const seq = ++previewSeqRef.current;
    previewTimerRef.current = window.setTimeout(() => {
      void (async () => {
        try {
          const p = await expenseService.previewCalculation(inputs);
          if (seq === previewSeqRef.current) {
            setBackendPreview(p);
          }
        } catch {
          if (seq === previewSeqRef.current) {
            setBackendPreview(null);
          }
        }
      })();
    }, 60);
  }, []);

  React.useEffect(() => {
    return () => {
      if (previewTimerRef.current !== null) {
        window.clearTimeout(previewTimerRef.current);
      }
    };
  }, []);

  const roundOffPaiseVal = React.useMemo(
    () => Math.round(Number.parseFloat(roundOffInr) || 0) * 100,
    [roundOffInr],
  );

  const localPreview = React.useMemo(() => {
    const validInputs = toLineInputs(draftLines).filter((l) => l.expenseTypeId && l.amountPaise > 0);
    if (validInputs.length === 0) return null;
    return calculateInvoiceLocal(validInputs, 0);
  }, [draftLines]);

  const preview = backendPreview ?? localPreview;

  const draftAnalysis = React.useMemo(() => analyzeDraftLines(draftLines), [draftLines]);
  const lineValidationErrors = draftAnalysis.lineErrors;
  const hasLineValidationErrors = draftAnalysis.hasLineErrors;
  const hasZeroOrEmptyLine = draftAnalysis.hasZeroOrEmptyLine;
  const hasDuplicateType = draftAnalysis.hasDuplicateType;

  const roundOffOutOfRange = Math.abs(roundOffPaiseVal) > MAX_ROUND_OFF_PAISE;
  const negativeNetPayable = Boolean(
    preview && preview.netAmountPaise + roundOffPaiseVal < 0,
  );
  const sameClearingAgentAsProvider =
    isPaidByClearingAgent &&
    Boolean(clearingAgentId) &&
    clearingAgentId === serviceProviderId;

  const shipmentExistingInvoices = React.useMemo(() => {
    if (!shipmentId) return [];
    return invoices.filter((inv) => inv.shipmentId === shipmentId);
  }, [invoices, shipmentId]);

  const batchSummary = React.useMemo(() => {
    let basePaise = 0;
    let gstPaise = 0;
    let tdsPaise = 0;
    let netPaise = 0;
    let directNetPaise = 0;
    let directCount = 0;
    let chaNetPaise = 0;
    let chaCount = 0;
    let hasAnyErrors = false;
    const providerInvKeys = new Map<string, number>();
    const intraBatchDupKeys = new Set<string>();

    batchBills.forEach((bill, idx) => {
      const spInvKey = `${bill.serviceProviderId}|${bill.invoiceNo.trim().toLowerCase()}`;
      if (bill.serviceProviderId && bill.invoiceNo.trim()) {
        if (providerInvKeys.has(spInvKey)) {
          intraBatchDupKeys.add(spInvKey);
        } else {
          providerInvKeys.set(spInvKey, idx);
        }
      }

      const billAnalysis = analyzeDraftLines(bill.lines);
      const rOff = Math.round(Number.parseFloat(bill.roundOffInr) || 0) * 100;
      const calc = calculateInvoiceLocal(billAnalysis.inputs, rOff);

      basePaise += calc.baseAmountPaise;
      gstPaise += calc.totalCgstPaise + calc.totalSgstPaise + calc.totalIgstPaise;
      tdsPaise += calc.totalTdsPaise;
      netPaise += calc.netAmountPaise;

      if (bill.isPaidByClearingAgent) {
        chaNetPaise += calc.netAmountPaise;
        chaCount += 1;
      } else {
        directNetPaise += calc.netAmountPaise;
        directCount += 1;
      }

      const billInvalid =
        !bill.serviceProviderId ||
        !bill.invoiceNo.trim() ||
        !bill.invoiceDate ||
        billAnalysis.hasDuplicateType ||
        billAnalysis.hasLineErrors ||
        billAnalysis.hasZeroOrEmptyLine ||
        Math.abs(rOff) > MAX_ROUND_OFF_PAISE ||
        calc.totalAmountPaise <= 0 ||
        calc.netAmountPaise < 0 ||
        (bill.isPaidByClearingAgent &&
          (!bill.clearingAgentId || bill.clearingAgentId === bill.serviceProviderId));

      if (billInvalid) {
        hasAnyErrors = true;
      }
    });

    if (intraBatchDupKeys.size > 0) {
      hasAnyErrors = true;
    }

    return {
      basePaise,
      gstPaise,
      tdsPaise,
      netPaise,
      directNetPaise,
      directCount,
      chaNetPaise,
      chaCount,
      hasAnyErrors,
      intraBatchDupKeys,
    };
  }, [batchBills]);

  // Focus newly added row's amount field automatically for rapid entry
  React.useEffect(() => {
    const key = pendingFocusKeyRef.current;
    if (key && lineAmountRefs.current[key]) {
      pendingFocusKeyRef.current = null;
      lineAmountRefs.current[key]?.focus();
      lineAmountRefs.current[key]?.select();
    }
  }, [draftLines]);

  // Persist unsaved new-invoice draft in localStorage
  React.useEffect(() => {
    if (!invOpen || editingInv) return;
    const hasContent =
      invoiceNo.trim().length > 0 ||
      remarks.trim().length > 0 ||
      draftLines.some((l) => (Number.parseFloat(l.amountInr) || 0) > 0 || l.remarks.trim().length > 0);
    if (!hasContent) return;
    try {
      const payload: StoredExpenseDraft = {
        savedAt: Date.now(),
        shipmentId,
        serviceProviderId,
        invoiceNo,
        invoiceDate,
        isPaidByClearingAgent,
        clearingAgentId,
        chaReimburseInvoiceNo,
        chaReimburseInvoiceDate,
        remarks,
        roundOffInr,
        draftLines,
      };
      localStorage.setItem(EXPENSE_DRAFT_STORAGE_KEY, JSON.stringify(payload));
    } catch {
      // Ignore storage quota errors
    }
  }, [
    invOpen,
    editingInv,
    shipmentId,
    serviceProviderId,
    invoiceNo,
    invoiceDate,
    isPaidByClearingAgent,
    clearingAgentId,
    chaReimburseInvoiceNo,
    chaReimburseInvoiceDate,
    remarks,
    roundOffInr,
    draftLines,
  ]);

  const matchingExistingInvoice = React.useMemo(() => {
    if (editingInv) return null;
    const cleanNo = invoiceNo.trim().toLowerCase();
    if (!cleanNo || !serviceProviderId) return null;
    return invoices.find(
      (inv) =>
        inv.serviceProviderId === serviceProviderId &&
        inv.invoiceNo.trim().toLowerCase() === cleanNo
    );
  }, [editingInv, invoiceNo, serviceProviderId, invoices]);

  interface DisplayExpenseInvoice extends ExpenseInvoice {
    isVirtualReimbursement?: boolean;
    parentInvoice?: ExpenseInvoice;
    directNetPayablePaise: number;
  }

  const displayInvoices = React.useMemo(() => {
    const list: DisplayExpenseInvoice[] = [];
    for (const inv of invoices) {
      if (inv.isPaidByClearingAgent && inv.clearingAgentId) {
        // 1. Original CFS / Warehouse invoice:
        // Direct payable to this provider is ₹0 because CHA settled it upfront (Tally Agst Ref Debit)
        list.push({
          ...inv,
          directNetPayablePaise: 0,
        });

        // 2. Transferred CHA Reimbursement bill under Clearing Agent:
        // (Mirrors Tally Voucher No. 630 Credit to CHA under New Ref 439A / 438A)
        const reimbNo = inv.chaReimburseInvoiceNo?.trim() || `REIMB-${inv.invoiceNo}`;
        list.push({
          ...inv,
          id: `reimb-${inv.id}`,
          isVirtualReimbursement: true,
          parentInvoice: inv,
          serviceProviderId: inv.clearingAgentId,
          serviceProviderName: inv.clearingAgentName || 'Clearing Agent',
          invoiceNo: reimbNo,
          invoiceDate: inv.chaReimburseInvoiceDate || inv.invoiceDate,
          baseAmountPaise: inv.netAmountPaise,
          totalCgstPaise: 0,
          totalSgstPaise: 0,
          totalIgstPaise: 0,
          totalTdsPaise: 0,
          totalAmountPaise: inv.netAmountPaise,
          netAmountPaise: inv.netAmountPaise,
          directNetPayablePaise: inv.netAmountPaise,
          isPaidByClearingAgent: false,
          remarks: `CHA Reimbursement (Agst ${inv.serviceProviderName} Ref: ${inv.invoiceNo})`,
        });
      } else {
        list.push({
          ...inv,
          directNetPayablePaise: inv.netAmountPaise,
        });
      }
    }
    return list;
  }, [invoices]);

  const toggleInvoiceExpand = (id: string) => {
    setExpandedInvoices((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const allInvoicesExpanded =
    displayInvoices.length > 0 && displayInvoices.every((inv) => expandedInvoices.has(inv.id));

  const toggleAllInvoicesExpand = () => {
    if (allInvoicesExpanded) {
      setExpandedInvoices(new Set());
    } else {
      setExpandedInvoices(new Set(displayInvoices.map((inv) => inv.id)));
    }
  };

  const allItemizedLines = React.useMemo(() => {
    const list: Array<{
      lineId: string;
      invoiceId: string;
      invoiceNo: string;
      invoiceDate: string;
      shipmentId: string;
      shipmentInvoiceNo: string;
      serviceProviderId: string;
      serviceProviderName: string;
      isPaidByClearingAgent: boolean;
      clearingAgentId: string;
      clearingAgentName: string;
      chaReimburseInvoiceNo: string;
      chaReimburseInvoiceDate: string;
      expenseTypeId: string;
      expenseTypeName: string;
      amountPaise: number;
      isManualGst: boolean;
      cgstRateBp: number;
      sgstRateBp: number;
      igstRateBp: number;
      tdsRateBp: number;
      cgstPaise: number;
      sgstPaise: number;
      igstPaise: number;
      tdsPaise: number;
      totalPaise: number;
      netPaise: number;
      remarks: string;
      parentInvoice: ExpenseInvoice;
    }> = [];

    for (const inv of invoices) {
      if (Array.isArray(inv.lines) && inv.lines.length > 0) {
        for (const line of inv.lines) {
          list.push({
            lineId: line.id || `${inv.id}-${line.expenseTypeId}`,
            invoiceId: inv.id,
            invoiceNo: inv.invoiceNo,
            invoiceDate: inv.invoiceDate,
            shipmentId: inv.shipmentId,
            shipmentInvoiceNo: inv.shipmentInvoiceNo,
            serviceProviderId: inv.serviceProviderId,
            serviceProviderName: inv.serviceProviderName,
            isPaidByClearingAgent: Boolean(inv.isPaidByClearingAgent),
            clearingAgentId: inv.clearingAgentId || '',
            clearingAgentName: inv.clearingAgentName || '',
            chaReimburseInvoiceNo: inv.chaReimburseInvoiceNo || '',
            chaReimburseInvoiceDate: inv.chaReimburseInvoiceDate || '',
            expenseTypeId: line.expenseTypeId,
            expenseTypeName: line.expenseTypeName || 'Expense Item',
            amountPaise: line.amountPaise,
            isManualGst: Boolean(line.isManualGst),
            cgstRateBp: line.cgstRateBp,
            sgstRateBp: line.sgstRateBp,
            igstRateBp: line.igstRateBp,
            tdsRateBp: line.tdsRateBp,
            cgstPaise: line.cgstPaise,
            sgstPaise: line.sgstPaise,
            igstPaise: line.igstPaise,
            tdsPaise: line.tdsPaise,
            totalPaise: line.totalPaise,
            netPaise: line.netPaise,
            remarks: line.remarks || '',
            parentInvoice: inv,
          });
        }
      }
    }
    return list;
  }, [invoices]);

  const filteredItemizedLines = React.useMemo(() => {
    return allItemizedLines.filter((l) => {
      if (itemizedTypeFilter !== 'all' && l.expenseTypeId !== itemizedTypeFilter) {
        return false;
      }
      if (
        itemizedProviderFilter !== 'all' &&
        l.serviceProviderId !== itemizedProviderFilter &&
        (!l.isPaidByClearingAgent || l.clearingAgentId !== itemizedProviderFilter)
      ) {
        return false;
      }
      if (itemizedShipmentFilter !== 'all' && l.shipmentId !== itemizedShipmentFilter) {
        return false;
      }
      if (itemizedSearch.trim()) {
        const query = itemizedSearch.toLowerCase().trim();
        const matches =
          l.invoiceNo.toLowerCase().includes(query) ||
          l.serviceProviderName.toLowerCase().includes(query) ||
          l.expenseTypeName.toLowerCase().includes(query) ||
          l.shipmentInvoiceNo.toLowerCase().includes(query) ||
          l.clearingAgentName.toLowerCase().includes(query) ||
          l.chaReimburseInvoiceNo.toLowerCase().includes(query) ||
          l.remarks.toLowerCase().includes(query);
        if (!matches) return false;
      }
      return true;
    });
  }, [allItemizedLines, itemizedTypeFilter, itemizedProviderFilter, itemizedShipmentFilter, itemizedSearch]);

  const itemizedSummary = React.useMemo(() => {
    let basic = 0;
    let cgst = 0;
    let sgst = 0;
    let igst = 0;
    let total = 0;
    let tds = 0;
    let net = 0;
    for (const l of filteredItemizedLines) {
      basic += l.amountPaise;
      cgst += l.cgstPaise;
      sgst += l.sgstPaise;
      igst += l.igstPaise;
      total += l.totalPaise || (l.amountPaise + l.cgstPaise + l.sgstPaise + l.igstPaise);
      tds += l.tdsPaise;
      net += l.netPaise;
    }
    return {
      count: filteredItemizedLines.length,
      basicPaise: basic,
      gstPaise: cgst + sgst + igst,
      totalPaise: total,
      tdsPaise: tds,
      netPaise: net,
    };
  }, [filteredItemizedLines]);

  React.useEffect(() => {
    setInvoicePage(1);
  }, [expensePageSize, search, shipmentFilter]);

  React.useEffect(() => {
    setItemizedPage(1);
  }, [expensePageSize, itemizedSearch, itemizedTypeFilter, itemizedProviderFilter, itemizedShipmentFilter]);

  const totalInvoicePages = Math.max(1, Math.ceil(displayInvoices.length / expensePageSize));
  const currentInvoicePage = Math.min(invoicePage, totalInvoicePages);
  const pagedInvoices = React.useMemo(() => {
    const start = (currentInvoicePage - 1) * expensePageSize;
    return displayInvoices.slice(start, start + expensePageSize);
  }, [displayInvoices, currentInvoicePage, expensePageSize]);

  const clearingAgentSummaries = React.useMemo(() => {
    const chaMap = new Map<
      string,
      {
        agentId: string;
        agentName: string;
        directCount: number;
        directNetPaise: number;
        reimbCount: number;
        reimbNetPaise: number;
        totalNetPaise: number;
        refList: string[];
      }
    >();

    for (const item of displayInvoices) {
      if (item.isVirtualReimbursement) {
        const agentId = item.serviceProviderId;
        const agentName = item.serviceProviderName;
        const cur = chaMap.get(agentId) || {
          agentId,
          agentName,
          directCount: 0,
          directNetPaise: 0,
          reimbCount: 0,
          reimbNetPaise: 0,
          totalNetPaise: 0,
          refList: [],
        };
        cur.reimbCount += 1;
        cur.reimbNetPaise += item.directNetPayablePaise;
        cur.totalNetPaise += item.directNetPayablePaise;
        if (item.invoiceNo && !cur.refList.includes(item.invoiceNo)) {
          cur.refList.push(item.invoiceNo);
        }
        chaMap.set(agentId, cur);
      } else if (!item.isPaidByClearingAgent && item.directNetPayablePaise > 0) {
        const isAgentForAny = invoices.some((i) => i.clearingAgentId === item.serviceProviderId);
        if (isAgentForAny) {
          const agentId = item.serviceProviderId;
          const agentName = item.serviceProviderName;
          const cur = chaMap.get(agentId) || {
            agentId,
            agentName,
            directCount: 0,
            directNetPaise: 0,
            reimbCount: 0,
            reimbNetPaise: 0,
            totalNetPaise: 0,
            refList: [],
          };
          cur.directCount += 1;
          cur.directNetPaise += item.directNetPayablePaise;
          cur.totalNetPaise += item.directNetPayablePaise;
          chaMap.set(agentId, cur);
        }
      }
    }
    return Array.from(chaMap.values()).filter((cha) => cha.totalNetPaise > 0);
  }, [displayInvoices, invoices]);

  const totalItemizedPages = Math.max(1, Math.ceil(filteredItemizedLines.length / expensePageSize));
  const currentItemizedPage = Math.min(itemizedPage, totalItemizedPages);
  const pagedItemizedLines = React.useMemo(() => {
    const start = (currentItemizedPage - 1) * expensePageSize;
    return filteredItemizedLines.slice(start, start + expensePageSize);
  }, [filteredItemizedLines, currentItemizedPage, expensePageSize]);

  const exportItemizedCsv = () => {
    const headers = [
      'Invoice Date',
      'Invoice No',
      'Service Provider',
      'Shipment Invoice',
      'Expense Type',
      'Basic (INR)',
      'CGST %',
      'CGST (INR)',
      'SGST %',
      'SGST (INR)',
      'IGST %',
      'IGST (INR)',
      'Invoice Total (INR)',
      'TDS %',
      'TDS (INR)',
      'Net Payable (INR)',
      'Remarks',
    ];
    const rows = filteredItemizedLines.map((l) => [
      l.invoiceDate,
      `"${l.invoiceNo.replaceAll('"', '""')}"`,
      `"${l.serviceProviderName.replaceAll('"', '""')}"`,
      `"${l.shipmentInvoiceNo.replaceAll('"', '""')}"`,
      `"${l.expenseTypeName.replaceAll('"', '""')}"`,
      Math.round(l.amountPaise / 100).toString(),
      (l.cgstRateBp / 100).toFixed(2),
      Math.round(l.cgstPaise / 100).toString(),
      (l.sgstRateBp / 100).toFixed(2),
      Math.round(l.sgstPaise / 100).toString(),
      (l.igstRateBp / 100).toFixed(2),
      Math.round(l.igstPaise / 100).toString(),
      Math.round((l.totalPaise || (l.amountPaise + l.cgstPaise + l.sgstPaise + l.igstPaise)) / 100).toString(),
      (l.tdsRateBp / 100).toFixed(2),
      Math.round(l.tdsPaise / 100).toString(),
      Math.round(l.netPaise / 100).toString(),
      `"${(l.remarks || '').replaceAll('"', '""')}"`,
    ]);
    downloadTextFile(
      `expenses_itemized_ledger_${new Date().toISOString().slice(0, 10)}.csv`,
      [headers.join(','), ...rows.map((r) => r.join(','))].join('\r\n'),
    );
  };

  const pickNextUnusedType = React.useCallback(
    (currentLines: DraftLine[]) => {
      const pool = activeTypes.length > 0 ? activeTypes : types;
      const usedIds = new Set(currentLines.map((l) => l.expenseTypeId).filter(Boolean));
      return pool.find((t) => !usedIds.has(t.id)) ?? pool[0];
    },
    [activeTypes, types],
  );

  const openCreateInvoice = () => {
    setEditingInv(null);
    const activeSh = shipments.find((s) => !s.isFrozen);
    setShipmentId((prev) =>
      prev && shipments.some((s) => s.id === prev && !s.isFrozen) ? prev : (activeSh?.id ?? ''),
    );
    const defaultProv = providers[0];
    setServiceProviderId(defaultProv?.id ?? '');
    setInvoiceNo('');
    setInvoiceDate(new Date().toISOString().slice(0, 10));
    setIsPaidByClearingAgent(false);
    setClearingAgentId('');
    setChaReimburseInvoiceNo('');
    setChaReimburseInvoiceDate('');
    setRemarks('');
    setRoundOffInr('0');
    setIdempotencyKey(makeIdempotencyKey());
    const pool = activeTypes.length > 0 ? activeTypes : types;
    const defaultSupply = detectSupplyType(
      currentCompany?.gstin,
      currentCompany?.state,
      defaultProv?.gstin,
      defaultProv?.state,
    );
    const initial = [emptyLine(pool[0], false, defaultSupply)];
    setEntryMode('single');
    setBatchBills([]);
    setSharedClearingAgentId('');
    setSharedChaReimburseInvoiceNo('');
    setSharedChaReimburseInvoiceDate('');
    setTargetBatchBillKey(null);
    setPasteTargetBillKey(null);
    setDraftLines(initial);
    setBackendPreview(null);

    try {
      const raw = localStorage.getItem(EXPENSE_DRAFT_STORAGE_KEY);
      if (raw) {
        const parsed = JSON.parse(raw) as StoredExpenseDraft;
        if (parsed && Array.isArray(parsed.draftLines) && parsed.draftLines.length > 0) {
          setRecoverableDraft(parsed);
        } else {
          setRecoverableDraft(null);
        }
      } else {
        setRecoverableDraft(null);
      }
    } catch {
      setRecoverableDraft(null);
    }

    setInvOpen(true);
  };

  const openEditInvoice = (inv: ExpenseInvoice) => {
    setEditingInv(inv);
    setEntryMode('single');
    setBatchBills([]);
    setTargetBatchBillKey(null);
    setPasteTargetBillKey(null);
    setRecoverableDraft(null);
    setShipmentId(inv.shipmentId);
    setServiceProviderId(inv.serviceProviderId);
    setInvoiceNo(inv.invoiceNo);
    setInvoiceDate(inv.invoiceDate);
    setIsPaidByClearingAgent(Boolean(inv.isPaidByClearingAgent));
    setClearingAgentId(inv.clearingAgentId || '');
    setChaReimburseInvoiceNo(inv.chaReimburseInvoiceNo || '');
    setChaReimburseInvoiceDate(inv.chaReimburseInvoiceDate || '');
    setRemarks(inv.remarks);
    setRoundOffInr(Math.round((inv.roundOffPaise || 0) / 100).toString());
    setIdempotencyKey(inv.idempotencyKey);
    const lines: DraftLine[] = (inv.lines || []).map((l) => ({
      key: l.id || makeLineKey(),
      expenseTypeId: l.expenseTypeId,
      amountInr: Math.round(l.amountPaise / 100).toString(),
      isManualGst: Boolean(l.isManualGst),
      cgstPct: (l.cgstRateBp / 100).toFixed(2),
      sgstPct: (l.sgstRateBp / 100).toFixed(2),
      igstPct: (l.igstRateBp / 100).toFixed(2),
      manualCgstInr: Math.round(l.cgstPaise / 100).toString(),
      manualSgstInr: Math.round(l.sgstPaise / 100).toString(),
      manualIgstInr: Math.round(l.igstPaise / 100).toString(),
      tdsPct: (l.tdsRateBp / 100).toFixed(2),
      remarks: l.remarks,
    }));
    setDraftLines(lines);
    setBackendPreview(null);
    setInvOpen(true);
    scheduleBackendPreview(lines);
  };

  const getProviderSupplyType = React.useCallback(
    (provId: string) => {
      const prov = providers.find((p) => p.id === provId);
      return detectSupplyType(
        currentCompany?.gstin,
        currentCompany?.state,
        prov?.gstin,
        prov?.state,
      );
    },
    [providers, currentCompany?.gstin, currentCompany?.state],
  );

  const makeBlankBatchBill = React.useCallback(
    (existingBills: BatchBill[]): BatchBill => {
      const usedProviders = new Set(existingBills.map((b) => b.serviceProviderId).filter(Boolean));
      const nextProv = providers.find((p) => !usedProviders.has(p.id)) ?? providers[0];
      const provId = nextProv?.id ?? '';
      const provSupply = getProviderSupplyType(provId);
      const pool = activeTypes.length > 0 ? activeTypes : types;
      return {
        key: makeLineKey(),
        serviceProviderId: provId,
        invoiceNo: '',
        invoiceDate: new Date().toISOString().slice(0, 10),
        isPaidByClearingAgent: false,
        clearingAgentId: '',
        chaReimburseInvoiceNo: '',
        chaReimburseInvoiceDate: '',
        remarks: '',
        roundOffInr: '0',
        idempotencyKey: makeIdempotencyKey(),
        lines: [emptyLine(pool[0], false, provSupply)],
        collapsed: false,
      };
    },
    [providers, activeTypes, types, getProviderSupplyType],
  );

  const switchToBatchMode = (appendSecondBill = false) => {
    if (editingInv) return;
    if (entryMode === 'batch') {
      if (appendSecondBill) {
        setBatchBills((prev) => [...prev, makeBlankBatchBill(prev)]);
      }
      return;
    }

    const firstBill: BatchBill = {
      key: makeLineKey(),
      serviceProviderId,
      invoiceNo,
      invoiceDate,
      isPaidByClearingAgent,
      clearingAgentId,
      chaReimburseInvoiceNo,
      chaReimburseInvoiceDate,
      remarks,
      roundOffInr,
      idempotencyKey: idempotencyKey || makeIdempotencyKey(),
      lines: draftLines.length > 0 ? draftLines : [emptyLine(activeTypes[0] ?? types[0], false, supplyType)],
      collapsed: false,
    };

    if (isPaidByClearingAgent && clearingAgentId && !sharedClearingAgentId) {
      setSharedClearingAgentId(clearingAgentId);
      setSharedChaReimburseInvoiceNo(chaReimburseInvoiceNo);
      setSharedChaReimburseInvoiceDate(chaReimburseInvoiceDate);
    }

    const bills = appendSecondBill ? [firstBill, makeBlankBatchBill([firstBill])] : [firstBill];
    setBatchBills(bills);
    setEntryMode('batch');
  };

  const switchToSingleMode = () => {
    if (entryMode === 'single') return;
    const first = batchBills[0];
    if (first) {
      setServiceProviderId(first.serviceProviderId);
      setInvoiceNo(first.invoiceNo);
      setInvoiceDate(first.invoiceDate);
      setIsPaidByClearingAgent(first.isPaidByClearingAgent);
      setClearingAgentId(first.clearingAgentId);
      setChaReimburseInvoiceNo(first.chaReimburseInvoiceNo);
      setChaReimburseInvoiceDate(first.chaReimburseInvoiceDate);
      setRemarks(first.remarks);
      setRoundOffInr(first.roundOffInr);
      setIdempotencyKey(first.idempotencyKey);
      setDraftLines(first.lines);
      scheduleBackendPreview(first.lines);
    }
    setEntryMode('single');
  };

  const updateBatchBill = (billKey: string, patch: Partial<BatchBill>) => {
    setBatchBills((prev) => prev.map((b) => (b.key === billKey ? { ...b, ...patch } : b)));
  };

  const handleBatchProviderChange = (billKey: string, nextProvId: string) => {
    const nextSupply = getProviderSupplyType(nextProvId);
    setBatchBills((prev) =>
      prev.map((b) => {
        if (b.key !== billKey) return b;
        const nextClearingId =
          b.isPaidByClearingAgent && b.clearingAgentId === nextProvId ? '' : b.clearingAgentId;
        const nextLines =
          nextSupply === 'unknown'
            ? b.lines
            : b.lines.map((l) => {
                if (l.isManualGst || !l.expenseTypeId) return l;
                const et = types.find((t) => t.id === l.expenseTypeId);
                if (!et) return l;
                const adapted = adaptExpenseTypeRatesForSupply(et, nextSupply);
                return {
                  ...l,
                  cgstPct: (adapted.cgstBp / 100).toFixed(2),
                  sgstPct: (adapted.sgstBp / 100).toFixed(2),
                  igstPct: (adapted.igstBp / 100).toFixed(2),
                  tdsPct: (adapted.tdsBp / 100).toFixed(2),
                };
              });
        return {
          ...b,
          serviceProviderId: nextProvId,
          clearingAgentId: nextClearingId,
          lines: nextLines,
        };
      }),
    );
  };

  const addBatchBill = () => {
    setBatchBills((prev) => [...prev, makeBlankBatchBill(prev)]);
  };

  const duplicateBatchBill = (billKey: string) => {
    setBatchBills((prev) => {
      const idx = prev.findIndex((b) => b.key === billKey);
      if (idx === -1) return prev;
      const src = prev[idx]!;
      const copy: BatchBill = {
        ...src,
        key: makeLineKey(),
        invoiceNo: '',
        idempotencyKey: makeIdempotencyKey(),
        collapsed: false,
        lines: src.lines.map((l) => ({ ...l, key: makeLineKey() })),
      };
      return [...prev.slice(0, idx + 1), copy, ...prev.slice(idx + 1)];
    });
  };

  const removeBatchBill = (billKey: string) => {
    setBatchBills((prev) => (prev.length <= 1 ? prev : prev.filter((b) => b.key !== billKey)));
  };

  const handleBatchCombineDuplicates = async (billKey: string) => {
    const bill = batchBills.find((b) => b.key === billKey);
    if (!bill) return;
    try {
      const merged = await expenseService.mergeDuplicateLines(toLineInputs(bill.lines));
      const nextLines: DraftLine[] = merged.map((m) => ({
        key: makeLineKey(),
        expenseTypeId: m.expenseTypeId,
        amountInr: Math.round(m.amountPaise / 100).toString(),
        isManualGst: Boolean(m.isManualGst),
        cgstPct: (m.cgstRateBp / 100).toFixed(2),
        sgstPct: (m.sgstRateBp / 100).toFixed(2),
        igstPct: (m.igstRateBp / 100).toFixed(2),
        manualCgstInr: Math.round(m.cgstPaise / 100).toString(),
        manualSgstInr: Math.round(m.sgstPaise / 100).toString(),
        manualIgstInr: Math.round(m.igstPaise / 100).toString(),
        tdsPct: (m.tdsRateBp / 100).toFixed(2),
        remarks: m.remarks,
      }));
      updateBatchBill(billKey, { lines: nextLines });
      toast.success('Duplicate expense lines combined cleanly');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleCopyLastBillForBatch = (billKey: string) => {
    const bill = batchBills.find((b) => b.key === billKey);
    if (!bill?.serviceProviderId) return;
    const prevBill = invoices.find(
      (inv) => inv.serviceProviderId === bill.serviceProviderId && inv.lines?.length,
    );
    if (!prevBill || !prevBill.lines) {
      toast.info('No previous multi-line bill found for this provider.');
      return;
    }
    const nextLines: DraftLine[] = prevBill.lines.map((l) => ({
      key: makeLineKey(),
      expenseTypeId: l.expenseTypeId,
      amountInr: Math.round(l.amountPaise / 100).toString(),
      isManualGst: Boolean(l.isManualGst),
      cgstPct: (l.cgstRateBp / 100).toFixed(2),
      sgstPct: (l.sgstRateBp / 100).toFixed(2),
      igstPct: (l.igstRateBp / 100).toFixed(2),
      manualCgstInr: Math.round(l.cgstPaise / 100).toString(),
      manualSgstInr: Math.round(l.sgstPaise / 100).toString(),
      manualIgstInr: Math.round(l.igstPaise / 100).toString(),
      tdsPct: (l.tdsRateBp / 100).toFixed(2),
      remarks: l.remarks,
    }));
    updateBatchBill(billKey, { lines: nextLines });
    toast.success(`Loaded ${nextLines.length} line(s) from invoice ${prevBill.invoiceNo}`);
  };

  const applySharedChaToMarkedBills = (markAllEligible = false) => {
    if (!sharedClearingAgentId) {
      toast.error('Select a Shared Clearing Agent first.');
      return;
    }
    let updatedCount = 0;
    setBatchBills((prev) =>
      prev.map((b) => {
        if (b.serviceProviderId === sharedClearingAgentId) {
          return b;
        }
        if (markAllEligible || b.isPaidByClearingAgent) {
          updatedCount += 1;
          return {
            ...b,
            isPaidByClearingAgent: true,
            clearingAgentId: sharedClearingAgentId,
            chaReimburseInvoiceNo: sharedChaReimburseInvoiceNo,
            chaReimburseInvoiceDate: sharedChaReimburseInvoiceDate,
          };
        }
        return b;
      }),
    );
    if (updatedCount > 0) {
      toast.success(`Applied shared CHA reimbursement details to ${updatedCount} bill(s)`);
    } else {
      toast.info('No eligible third-party bills updated.');
    }
  };

  const handleSaveBatch = async () => {
    if (!shipmentId) {
      toast.error('Please select a shipment.');
      return;
    }
    if (batchSummary.hasAnyErrors || batchBills.length === 0) {
      toast.error('Please resolve highlighted bill errors before saving.');
      return;
    }
    setSavingInv(true);
    try {
      const reqs = batchBills.map((b) => {
        const roundOffVal = Math.round(Number.parseFloat(b.roundOffInr) || 0);
        return {
          id: '',
          shipmentId,
          serviceProviderId: b.serviceProviderId,
          invoiceNo: b.invoiceNo.trim(),
          invoiceDate: b.invoiceDate,
          currency: 'INR',
          isPaidByClearingAgent: b.isPaidByClearingAgent,
          clearingAgentId: b.isPaidByClearingAgent ? b.clearingAgentId : '',
          chaReimburseInvoiceNo: b.isPaidByClearingAgent ? b.chaReimburseInvoiceNo.trim() : '',
          chaReimburseInvoiceDate: b.isPaidByClearingAgent ? b.chaReimburseInvoiceDate : '',
          remarks: b.remarks.trim(),
          idempotencyKey: b.idempotencyKey,
          version: 1,
          mergeDuplicates: false,
          roundOffPaise: roundOffVal * 100,
          lines: toLineInputs(b.lines),
        };
      });
      await expenseService.createExpenseInvoicesBatch(reqs);
      toast.success(`Recorded ${reqs.length} expense invoice(s) atomically`);
      handleDiscardDraft();
      setInvOpen(false);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSavingInv(false);
    }
  };

  const handleProviderChange = (nextProvId: string) => {
    setServiceProviderId(nextProvId);
    if (isPaidByClearingAgent && clearingAgentId === nextProvId) {
      setClearingAgentId('');
    }
    if (editingInv) return;
    const nextProv = providers.find((p) => p.id === nextProvId);
    const nextSupply = detectSupplyType(
      currentCompany?.gstin,
      currentCompany?.state,
      nextProv?.gstin,
      nextProv?.state,
    );
    if (nextSupply === 'unknown') return;
    const nextLines = draftLines.map((l) => {
      if (l.isManualGst || !l.expenseTypeId) return l;
      const et = types.find((t) => t.id === l.expenseTypeId);
      if (!et) return l;
      const adapted = adaptExpenseTypeRatesForSupply(et, nextSupply);
      return {
        ...l,
        cgstPct: (adapted.cgstBp / 100).toFixed(2),
        sgstPct: (adapted.sgstBp / 100).toFixed(2),
        igstPct: (adapted.igstBp / 100).toFixed(2),
        tdsPct: (adapted.tdsBp / 100).toFixed(2),
      };
    });
    setDraftLines(nextLines);
    scheduleBackendPreview(nextLines);
  };

  const updateLine = (idx: number, patch: Partial<DraftLine>) => {
    const next = draftLines.map((l, i) => (i === idx ? { ...l, ...patch } : l));
    setDraftLines(next);
    scheduleBackendPreview(next);
  };

  const addDraftLine = (focusNew = false) => {
    setDraftLines((prev) => {
      const nextEt = pickNextUnusedType(prev);
      const created = emptyLine(
        nextEt,
        prev.some((l) => l.isManualGst),
        supplyType,
      );
      if (focusNew) {
        pendingFocusKeyRef.current = created.key;
      }
      const next = [...prev, created];
      scheduleBackendPreview(next);
      return next;
    });
  };

  const duplicateDraftLine = (idx: number) => {
    const src = draftLines[idx];
    if (!src) return;
    const nextEt = pickNextUnusedType(draftLines);
    const adapted = nextEt ? adaptExpenseTypeRatesForSupply(nextEt, supplyType) : null;
    const copy: DraftLine = {
      ...src,
      key: makeLineKey(),
      expenseTypeId: nextEt?.id ?? src.expenseTypeId,
      cgstPct: adapted ? (adapted.cgstBp / 100).toFixed(2) : src.cgstPct,
      sgstPct: adapted ? (adapted.sgstBp / 100).toFixed(2) : src.sgstPct,
      igstPct: adapted ? (adapted.igstBp / 100).toFixed(2) : src.igstPct,
      tdsPct: adapted ? (adapted.tdsBp / 100).toFixed(2) : src.tdsPct,
    };
    pendingFocusKeyRef.current = copy.key;
    const next = [...draftLines.slice(0, idx + 1), copy, ...draftLines.slice(idx + 1)];
    setDraftLines(next);
    scheduleBackendPreview(next);
  };

  const handleCopyLastBillForProvider = () => {
    if (!serviceProviderId) return;
    const prevBill = invoices.find((inv) => inv.serviceProviderId === serviceProviderId && inv.lines?.length);
    if (!prevBill || !prevBill.lines) {
      toast.info('No previous multi-line bill found for this provider.');
      return;
    }
    const next: DraftLine[] = prevBill.lines.map((l) => ({
      key: makeLineKey(),
      expenseTypeId: l.expenseTypeId,
      amountInr: Math.round(l.amountPaise / 100).toString(),
      isManualGst: Boolean(l.isManualGst),
      cgstPct: (l.cgstRateBp / 100).toFixed(2),
      sgstPct: (l.sgstRateBp / 100).toFixed(2),
      igstPct: (l.igstRateBp / 100).toFixed(2),
      manualCgstInr: Math.round(l.cgstPaise / 100).toString(),
      manualSgstInr: Math.round(l.sgstPaise / 100).toString(),
      manualIgstInr: Math.round(l.igstPaise / 100).toString(),
      tdsPct: (l.tdsRateBp / 100).toFixed(2),
      remarks: l.remarks,
    }));
    setDraftLines(next);
    scheduleBackendPreview(next);
    toast.success(`Loaded ${next.length} line(s) from invoice ${prevBill.invoiceNo}`);
  };

  const handleApplyPaste = () => {
    const pool = activeTypes.length > 0 ? activeTypes : types;
    const targetBill = pasteTargetBillKey
      ? batchBills.find((b) => b.key === pasteTargetBillKey)
      : null;
    const activeSupply = targetBill
      ? getProviderSupplyType(targetBill.serviceProviderId)
      : supplyType;

    const { rows, errors } = parsePastedExpenseLines(pasteText, pool, activeSupply);
    if (rows.length === 0) {
      toast.error(errors[0] || 'No valid expense lines found in pasted text');
      return;
    }
    const newLines: DraftLine[] = rows.map((r) => ({
      key: makeLineKey(),
      expenseTypeId: r.expenseType.id,
      amountInr: r.amountInr,
      isManualGst: false,
      cgstPct: r.cgstPct ?? '0.00',
      sgstPct: r.sgstPct ?? '0.00',
      igstPct: r.igstPct ?? '0.00',
      manualCgstInr: '0',
      manualSgstInr: '0',
      manualIgstInr: '0',
      tdsPct: r.tdsPct ?? '0.00',
      remarks: r.remarks,
    }));

    if (targetBill) {
      const hasExistingData = targetBill.lines.some(
        (l) => (Number.parseFloat(l.amountInr) || 0) > 0 || l.remarks.trim().length > 0,
      );
      const next = hasExistingData ? [...targetBill.lines, ...newLines] : newLines;
      updateBatchBill(targetBill.key, { lines: next });
    } else {
      const hasExistingData = draftLines.some(
        (l) => (Number.parseFloat(l.amountInr) || 0) > 0 || l.remarks.trim().length > 0,
      );
      const next = hasExistingData ? [...draftLines, ...newLines] : newLines;
      setDraftLines(next);
      scheduleBackendPreview(next);
    }

    setPasteOpen(false);
    setPasteText('');
    setPasteTargetBillKey(null);
    if (errors.length > 0) {
      toast.warning(`Added ${newLines.length} line(s); skipped ${errors.length}: ${errors[0]}`);
    } else {
      toast.success(`Added ${newLines.length} line(s) from clipboard`);
    }
  };

  const handleRestoreDraft = () => {
    if (!recoverableDraft) return;
    setShipmentId(recoverableDraft.shipmentId);
    setServiceProviderId(recoverableDraft.serviceProviderId);
    setInvoiceNo(recoverableDraft.invoiceNo);
    setInvoiceDate(recoverableDraft.invoiceDate);
    setIsPaidByClearingAgent(recoverableDraft.isPaidByClearingAgent);
    setClearingAgentId(recoverableDraft.clearingAgentId);
    setChaReimburseInvoiceNo(recoverableDraft.chaReimburseInvoiceNo);
    setChaReimburseInvoiceDate(recoverableDraft.chaReimburseInvoiceDate);
    setRemarks(recoverableDraft.remarks);
    setRoundOffInr(recoverableDraft.roundOffInr);
    const restored = recoverableDraft.draftLines.map((l) => ({ ...l, key: l.key || makeLineKey() }));
    setDraftLines(restored);
    setRecoverableDraft(null);
    scheduleBackendPreview(restored);
    toast.success('Restored unsaved draft');
  };

  const handleDiscardDraft = () => {
    setRecoverableDraft(null);
    try {
      localStorage.removeItem(EXPENSE_DRAFT_STORAGE_KEY);
    } catch {
      // Ignore
    }
  };

  const isEditorDirty = React.useMemo(() => {
    if (!invOpen) return false;
    if (editingInv) return true;
    if (entryMode === 'batch') {
      return batchBills.some(
        (b) =>
          b.invoiceNo.trim().length > 0 ||
          b.remarks.trim().length > 0 ||
          b.lines.some((l) => (Number.parseFloat(l.amountInr) || 0) > 0 || l.remarks.trim().length > 0),
      );
    }
    return (
      invoiceNo.trim().length > 0 ||
      remarks.trim().length > 0 ||
      draftLines.some((l) => (Number.parseFloat(l.amountInr) || 0) > 0 || l.remarks.trim().length > 0)
    );
  }, [invOpen, editingInv, entryMode, batchBills, invoiceNo, remarks, draftLines]);

  const requestCloseEditor = () => {
    if (!editingInv && isEditorDirty) {
      if (!window.confirm('Discard unsaved expense invoice entries and return to the ledger?')) {
        return;
      }
      handleDiscardDraft();
    }
    setInvOpen(false);
  };

  const handleCombineDuplicates = async () => {
    try {
      const merged = await expenseService.mergeDuplicateLines(toLineInputs(draftLines));
      const next: DraftLine[] = merged.map((m) => ({
        key: makeLineKey(),
        expenseTypeId: m.expenseTypeId,
        amountInr: Math.round(m.amountPaise / 100).toString(),
        isManualGst: Boolean(m.isManualGst),
        cgstPct: (m.cgstRateBp / 100).toFixed(2),
        sgstPct: (m.sgstRateBp / 100).toFixed(2),
        igstPct: (m.igstRateBp / 100).toFixed(2),
        manualCgstInr: Math.round(m.cgstPaise / 100).toString(),
        manualSgstInr: Math.round(m.sgstPaise / 100).toString(),
        manualIgstInr: Math.round(m.igstPaise / 100).toString(),
        tdsPct: (m.tdsRateBp / 100).toFixed(2),
        remarks: m.remarks,
      }));
      setDraftLines(next);
      scheduleBackendPreview(next);
      toast.success('Duplicate expense lines combined cleanly');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const canSubmitInvoice =
    !savingInv &&
    !hasDuplicateType &&
    !hasLineValidationErrors &&
    !hasZeroOrEmptyLine &&
    !roundOffOutOfRange &&
    !negativeNetPayable &&
    !sameClearingAgentAsProvider &&
    Boolean(shipmentId) &&
    Boolean(serviceProviderId) &&
    Boolean(invoiceNo.trim());

  const handleSaveInvoice = async (saveAndNew = false) => {
    if (isPaidByClearingAgent && !clearingAgentId) {
      toast.error('Please select the Clearing Agent who paid these charges.');
      return;
    }
    if (sameClearingAgentAsProvider) {
      toast.error('Clearing Agent cannot be the same as the original Service Provider.');
      return;
    }
    if (hasLineValidationErrors) {
      const firstErr = Object.values(lineValidationErrors)[0];
      toast.error(firstErr || 'Please resolve tax validation errors on lines.');
      return;
    }
    if (roundOffOutOfRange) {
      toast.error(`Round-off adjustment must be within ±₹${MAX_ROUND_OFF_PAISE / 100}.`);
      return;
    }
    setSavingInv(true);
    try {
      const roundOffVal = Math.round(Number.parseFloat(roundOffInr) || 0);
      const req = {
        id: editingInv?.id ?? '',
        shipmentId,
        serviceProviderId,
        invoiceNo: invoiceNo.trim(),
        invoiceDate,
        currency: 'INR',
        isPaidByClearingAgent,
        clearingAgentId: isPaidByClearingAgent ? clearingAgentId : '',
        chaReimburseInvoiceNo: isPaidByClearingAgent ? chaReimburseInvoiceNo.trim() : '',
        chaReimburseInvoiceDate: isPaidByClearingAgent ? chaReimburseInvoiceDate : '',
        remarks: remarks.trim(),
        idempotencyKey,
        version: editingInv?.version ?? 1,
        mergeDuplicates: false,
        roundOffPaise: roundOffVal * 100,
        lines: toLineInputs(draftLines),
      };
      if (editingInv) {
        await expenseService.updateExpenseInvoice(req);
        toast.success('Expense invoice updated');
      } else {
        await expenseService.createExpenseInvoice(req);
        toast.success('Expense invoice recorded');
        handleDiscardDraft();
      }
      if (saveAndNew && !editingInv) {
        setInvoiceNo('');
        setChaReimburseInvoiceNo('');
        setRemarks('');
        setRoundOffInr('0');
        setIdempotencyKey(makeIdempotencyKey());
        const pool = activeTypes.length > 0 ? activeTypes : types;
        setDraftLines([emptyLine(pool[0], false, supplyType)]);
        setBackendPreview(null);
        await loadAll();
      } else {
        setInvOpen(false);
        await loadAll();
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSavingInv(false);
    }
  };

  const handleUnlinkReimbursement = async (parentInv: ExpenseInvoice) => {
    if (!window.confirm(`Unlink CHA reimbursement bill ${parentInv.chaReimburseInvoiceNo || ''} from ${parentInv.serviceProviderName}? Direct liability will be restored to ${parentInv.serviceProviderName}.`)) {
      return;
    }
    try {
      const lineInputs = (parentInv.lines || []).map((l) => ({
        expenseTypeId: l.expenseTypeId,
        amountPaise: l.amountPaise,
        isManualGst: Boolean(l.isManualGst),
        cgstPaise: l.cgstPaise,
        sgstPaise: l.sgstPaise,
        igstPaise: l.igstPaise,
        cgstRateBp: l.cgstRateBp,
        sgstRateBp: l.sgstRateBp,
        igstRateBp: l.igstRateBp,
        tdsRateBp: l.tdsRateBp,
        remarks: l.remarks,
        roundOff: true,
      }));
      await expenseService.updateExpenseInvoice({
        id: parentInv.id,
        shipmentId: parentInv.shipmentId,
        serviceProviderId: parentInv.serviceProviderId,
        invoiceNo: parentInv.invoiceNo,
        invoiceDate: parentInv.invoiceDate,
        currency: parentInv.currency,
        isPaidByClearingAgent: false,
        clearingAgentId: '',
        chaReimburseInvoiceNo: '',
        chaReimburseInvoiceDate: '',
        remarks: parentInv.remarks,
        idempotencyKey: parentInv.idempotencyKey || `unlink-${Date.now()}`,
        version: parentInv.version,
        mergeDuplicates: false,
        roundOffPaise: parentInv.roundOffPaise ?? 0,
        lines: lineInputs,
      });
      toast.success('CHA reimbursement unlinked; direct liability restored');
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleSaveProvider = async () => {
    try {
      if (editingSp) {
        await expenseService.updateServiceProvider(editingSp.id, spForm);
        toast.success('Service provider updated');
      } else {
        await expenseService.createServiceProvider(spForm);
        toast.success('Service provider added');
      }
      setSpOpen(false);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleSaveType = async () => {
    try {
      if (editingEt) {
        await expenseService.updateExpenseType(editingEt.id, etForm);
        toast.success('Expense type updated');
      } else {
        await expenseService.createExpenseType(etForm);
        toast.success('Expense type added');
      }
      setEtOpen(false);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="space-y-6">
      {invOpen ? (
        <div
          className="space-y-5"
          onKeyDown={(e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
              e.preventDefault();
              if (entryMode === 'batch') {
                if (!savingInv && shipmentId && batchBills.length > 0 && !batchSummary.hasAnyErrors) {
                  void handleSaveBatch();
                }
              } else if (canSubmitInvoice) {
                void handleSaveInvoice(e.shiftKey && !editingInv);
              }
            } else if (entryMode === 'single' && e.altKey && e.key.toLowerCase() === 'n') {
              e.preventDefault();
              addDraftLine(true);
            }
          }}
        >
          {/* Top Navigation & Action Header */}
          <div className="flex flex-wrap items-center justify-between gap-3 border-b pb-4">
            <div className="flex items-center gap-3">
              <Button
                variant="outline"
                size="sm"
                onClick={requestCloseEditor}
                className="gap-1.5"
              >
                <ArrowLeft className="h-4 w-4" />
                <span>Back to Expense Ledger</span>
              </Button>
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <h1 className="text-2xl font-bold tracking-tight">
                    {editingInv
                      ? `Edit Expense Invoice (${editingInv.invoiceNo})`
                      : entryMode === 'batch'
                      ? 'Multi-Provider Shipment Expense Batch'
                      : 'New Multi-Line Expense Invoice'}
                  </h1>
                  {entryMode === 'single' && supplyType !== 'unknown' && (
                    <Badge
                      variant="outline"
                      className={
                        supplyType === 'inter'
                          ? 'border-blue-400/50 bg-blue-500/10 text-blue-700 dark:text-blue-300 text-[11px]'
                          : 'border-emerald-400/50 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 text-[11px]'
                      }
                    >
                      {supplyType === 'inter' ? 'Inter-State · IGST' : 'Intra-State · CGST + SGST'}
                    </Badge>
                  )}
                </div>
                <p className="text-xs text-muted-foreground">
                  {entryMode === 'batch'
                    ? 'Enter multiple service provider bills for one shipment with optional shared CHA reimbursement — saved atomically.'
                    : 'Whole-rupee integer-paise math · Tab across cells · Enter on last Remarks adds a line · Ctrl+Enter saves'}
                </p>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              {!editingInv && (
                <div className="inline-flex items-center rounded-md border bg-muted/40 p-0.5 text-xs">
                  <button
                    type="button"
                    className={`rounded px-2.5 py-1 font-medium transition-colors cursor-pointer ${
                      entryMode === 'single'
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                    onClick={switchToSingleMode}
                  >
                    Single Invoice
                  </button>
                  <button
                    type="button"
                    className={`rounded px-2.5 py-1 font-medium transition-colors cursor-pointer ${
                      entryMode === 'batch'
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                    onClick={() => switchToBatchMode(batchBills.length === 0)}
                  >
                    Multi-Provider Batch
                  </button>
                </div>
              )}

              <Button variant="outline" onClick={requestCloseEditor}>
                Cancel
              </Button>

              {entryMode === 'batch' ? (
                <Button
                  disabled={savingInv || !shipmentId || batchBills.length === 0 || batchSummary.hasAnyErrors}
                  onClick={() => void handleSaveBatch()}
                >
                  {savingInv ? 'Saving Batch...' : `Save All ${batchBills.length} Invoices`}
                </Button>
              ) : (
                <>
                  {!editingInv && (
                    <>
                      <Button
                        variant="outline"
                        type="button"
                        onClick={() => switchToBatchMode(true)}
                        title="Keep this shipment & add another service provider invoice in the same batch"
                      >
                        <Plus className="h-3.5 w-3.5 mr-1" />
                        Add Provider Bill
                      </Button>
                      <Button
                        variant="secondary"
                        disabled={!canSubmitInvoice}
                        onClick={() => void handleSaveInvoice(true)}
                        title="Save and start the next bill (Ctrl+Shift+Enter)"
                      >
                        {savingInv ? 'Saving...' : 'Save & New'}
                      </Button>
                    </>
                  )}
                  <Button
                    disabled={!canSubmitInvoice}
                    onClick={() => void handleSaveInvoice(false)}
                  >
                    {savingInv ? 'Saving...' : 'Save Expense Invoice'}
                  </Button>
                </>
              )}
            </div>
          </div>

          {recoverableDraft && !editingInv && entryMode === 'single' && (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-primary/40 bg-primary/5 px-4 py-2.5 text-xs">
              <span className="flex items-center gap-2 font-medium">
                <RotateCcw className="h-4 w-4 text-primary" />
                Unsaved draft found
                {recoverableDraft.invoiceNo ? ` (Invoice #${recoverableDraft.invoiceNo})` : ''} with{' '}
                {recoverableDraft.draftLines.length} line(s). Restore where you left off?
              </span>
              <div className="flex items-center gap-2">
                <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={handleDiscardDraft}>
                  Discard
                </Button>
                <Button size="sm" variant="outline" className="h-7 text-xs" onClick={handleRestoreDraft}>
                  Restore Draft
                </Button>
              </div>
            </div>
          )}

          {entryMode === 'batch' ? (
            <div className="space-y-5">
              {/* Shared Shipment + Shared CHA Reimbursement Header */}
              <Card>
                <CardHeader className="pb-3">
                  <CardTitle className="text-base font-semibold">
                    Shipment &amp; Shared CHA Reimbursement Settings
                  </CardTitle>
                  <CardDescription>
                    Choose the target shipment once, then add invoices from multiple service providers below.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid gap-4 md:grid-cols-2">
                    <div className="space-y-1.5">
                      <Label>Shipment (Shared for all bills in batch) *</Label>
                      <SearchableCombobox
                        value={shipmentId}
                        onValueChange={setShipmentId}
                        ariaLabel="Batch Shipment"
                        placeholder="Choose shipment"
                        searchPlaceholder="Search shipment invoice or supplier..."
                        options={shipments
                          .filter((s) => !s.isFrozen || s.id === shipmentId)
                          .map((s) => ({
                            value: s.id,
                            label: `${s.invoiceNumber} — ${s.supplierName || 'No Supplier'}`,
                            sublabel: `${s.invoiceDate || ''}${s.blAwbNumber ? ` · BL/AWB: ${s.blAwbNumber}` : ''}`,
                            keywords: `${s.invoiceNumber} ${s.supplierName} ${s.blAwbNumber || ''}`,
                          }))}
                      />
                    </div>

                    <div className="space-y-1.5">
                      <Label className="text-xs text-muted-foreground">
                        Already Recorded on This Shipment ({shipmentExistingInvoices.length})
                      </Label>
                      <div className="min-h-9 rounded-md border bg-muted/20 px-3 py-2 text-xs flex flex-wrap items-center gap-1.5">
                        {shipmentExistingInvoices.length === 0 ? (
                          <span className="text-muted-foreground">No expense invoices recorded yet for this shipment.</span>
                        ) : (
                          shipmentExistingInvoices.map((ex) => (
                            <Badge key={ex.id} variant="secondary" className="text-[11px] font-normal gap-1">
                              <span className="font-medium">{ex.serviceProviderName}</span>
                              <span>#{ex.invoiceNo}</span>
                              <span className="font-mono">({formatPaiseINR(ex.netAmountPaise)})</span>
                            </Badge>
                          ))
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Shared CHA Reimbursement Defaults */}
                  <div className="rounded-lg border p-3.5 bg-muted/20 space-y-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div>
                        <div className="flex items-center gap-2 text-sm font-semibold">
                          <span>Shared CHA Reimbursement Bill (Optional)</span>
                          <Badge variant="outline" className="text-[10px] font-normal">
                            Auto-fills bills marked Paid by CHA
                          </Badge>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          If one Clearing Agent settled multiple vendor bills (e.g., Liner + CFS) under a single reimbursement invoice, enter the shared CHA details here.
                        </p>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          type="button"
                          className="h-7 text-xs"
                          disabled={!sharedClearingAgentId}
                          onClick={() => applySharedChaToMarkedBills(false)}
                        >
                          Sync to Marked CHA Bills
                        </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          type="button"
                          className="h-7 text-xs"
                          disabled={!sharedClearingAgentId}
                          onClick={() => applySharedChaToMarkedBills(true)}
                        >
                          Apply CHA to All Other Bills
                        </Button>
                      </div>
                    </div>

                    <div className="grid gap-4 sm:grid-cols-3 pt-2 border-t">
                      <div className="space-y-1.5">
                        <Label>Shared Clearing Agent (Payable To)</Label>
                        <SearchableCombobox
                          value={sharedClearingAgentId}
                          onValueChange={setSharedClearingAgentId}
                          ariaLabel="Shared Clearing Agent"
                          placeholder="Select clearing agent"
                          searchPlaceholder="Search clearing agent..."
                          options={providers.map((p) => ({
                            value: p.id,
                            label: p.name,
                            sublabel: [p.gstin, p.state].filter(Boolean).join(' · ') || undefined,
                            keywords: `${p.name} ${p.gstin || ''} ${p.state || ''}`,
                          }))}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label htmlFor="sharedChaNo">Shared CHA Reimburse Invoice No</Label>
                        <Input
                          id="sharedChaNo"
                          maxLength={100}
                          placeholder="e.g. CHA-REIMB-2026-001"
                          className="font-mono text-sm"
                          value={sharedChaReimburseInvoiceNo}
                          onChange={(e) => setSharedChaReimburseInvoiceNo(e.target.value)}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label htmlFor="sharedChaDate">Shared CHA Reimburse Date</Label>
                        <Input
                          id="sharedChaDate"
                          type="date"
                          value={sharedChaReimburseInvoiceDate}
                          onChange={(e) => setSharedChaReimburseInvoiceDate(e.target.value)}
                        />
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>

              {/* Provider Invoice Cards */}
              {batchBills.map((bill, billIdx) => {
                const billProv = providers.find((p) => p.id === bill.serviceProviderId);
                const billSupply = getProviderSupplyType(bill.serviceProviderId);
                const billAnalysis = analyzeDraftLines(bill.lines);
                const billRoundOffPaise = Math.round(Number.parseFloat(bill.roundOffInr) || 0) * 100;
                const billCalc = calculateInvoiceLocal(billAnalysis.inputs, billRoundOffPaise);
                const billSpInvKey = `${bill.serviceProviderId}|${bill.invoiceNo.trim().toLowerCase()}`;
                const isIntraBatchDup = Boolean(
                  bill.serviceProviderId &&
                    bill.invoiceNo.trim() &&
                    batchSummary.intraBatchDupKeys.has(billSpInvKey),
                );
                const existingInDb =
                  bill.serviceProviderId && bill.invoiceNo.trim()
                    ? invoices.find(
                        (inv) =>
                          inv.serviceProviderId === bill.serviceProviderId &&
                          inv.invoiceNo.trim().toLowerCase() === bill.invoiceNo.trim().toLowerCase(),
                      )
                    : null;
                const billSameCha =
                  bill.isPaidByClearingAgent &&
                  Boolean(bill.clearingAgentId) &&
                  bill.clearingAgentId === bill.serviceProviderId;
                const billChaAgentName =
                  providers.find((p) => p.id === bill.clearingAgentId)?.name || '';

                return (
                  <Card
                    key={bill.key}
                    className={
                      isIntraBatchDup || billAnalysis.hasLineErrors || billSameCha
                        ? 'border-destructive/60'
                        : bill.isPaidByClearingAgent
                        ? 'border-purple-400/50'
                        : undefined
                    }
                  >
                    <CardHeader className="py-3 px-4 border-b bg-muted/20">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            className="h-7 w-7 p-0"
                            onClick={() => updateBatchBill(bill.key, { collapsed: !bill.collapsed })}
                            aria-label={bill.collapsed ? `Expand bill ${billIdx + 1}` : `Collapse bill ${billIdx + 1}`}
                          >
                            {bill.collapsed ? (
                              <ChevronRight className="h-4 w-4" />
                            ) : (
                              <ChevronDown className="h-4 w-4" />
                            )}
                          </Button>
                          <Badge variant="outline" className="font-mono text-xs">
                            Bill #{billIdx + 1}
                          </Badge>
                          <span className="font-semibold text-sm">
                            {billProv?.name || 'Select Service Provider'}
                          </span>
                          {bill.invoiceNo.trim() && (
                            <span className="font-mono text-xs text-muted-foreground">
                              #{bill.invoiceNo.trim()}
                            </span>
                          )}
                          {billSupply !== 'unknown' && (
                            <Badge variant="outline" className="text-[10px]">
                              {billSupply === 'inter' ? 'IGST' : 'CGST+SGST'}
                            </Badge>
                          )}
                          {bill.isPaidByClearingAgent && (
                            <Badge
                              variant="outline"
                              className="text-[10px] bg-purple-50 text-purple-700 border-purple-200 dark:bg-purple-950/50 dark:text-purple-300"
                            >
                              Paid by CHA{billChaAgentName ? `: ${billChaAgentName}` : ''}
                              {bill.chaReimburseInvoiceNo ? ` (Ref: ${bill.chaReimburseInvoiceNo})` : ''}
                            </Badge>
                          )}
                        </div>

                        <div className="flex items-center gap-2">
                          <span className="font-mono text-sm font-bold text-primary">
                            Net: INR {formatPaiseINR(billCalc.netAmountPaise)}
                          </span>
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            className="h-7 px-2 text-xs"
                            onClick={() => duplicateBatchBill(bill.key)}
                            title="Duplicate this provider bill"
                          >
                            <Copy className="h-3.5 w-3.5 mr-1" />
                            Duplicate
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                            disabled={batchBills.length <= 1}
                            onClick={() => removeBatchBill(bill.key)}
                            title="Remove bill from batch"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </div>
                    </CardHeader>

                    {!bill.collapsed && (
                      <CardContent className="p-4 space-y-4">
                        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                          <div className="space-y-1">
                            <div className="flex items-center justify-between">
                              <Label className="text-xs">Service Provider *</Label>
                              <button
                                type="button"
                                tabIndex={-1}
                                className="inline-flex items-center gap-1 text-[11px] text-primary hover:underline cursor-pointer"
                                onClick={() => {
                                  setTargetBatchBillKey(bill.key);
                                  setManageSpOpen(true);
                                }}
                              >
                                <Settings2 className="h-2.5 w-2.5" />
                                Manage
                              </button>
                            </div>
                            <SearchableCombobox
                              value={bill.serviceProviderId}
                              onValueChange={(v) => handleBatchProviderChange(bill.key, v)}
                              ariaLabel={`Bill ${billIdx + 1} Service Provider`}
                              placeholder="Choose provider"
                              searchPlaceholder="Search provider or GSTIN..."
                              options={providers.map((p) => ({
                                value: p.id,
                                label: p.name,
                                sublabel: [p.gstin, p.state].filter(Boolean).join(' · ') || undefined,
                                keywords: `${p.name} ${p.gstin || ''} ${p.state || ''}`,
                              }))}
                            />
                          </div>

                          <div className="space-y-1">
                            <Label className="text-xs">Invoice No *</Label>
                            <Input
                              className="h-9 text-xs font-mono"
                              maxLength={100}
                              aria-label={`Bill ${billIdx + 1} Invoice No`}
                              placeholder="e.g. INV-2026-001"
                              value={bill.invoiceNo}
                              onChange={(e) => updateBatchBill(bill.key, { invoiceNo: e.target.value })}
                            />
                          </div>

                          <div className="space-y-1">
                            <Label className="text-xs">Invoice Date *</Label>
                            <Input
                              className="h-9 text-xs"
                              type="date"
                              aria-label={`Bill ${billIdx + 1} Invoice Date`}
                              value={bill.invoiceDate}
                              onChange={(e) => updateBatchBill(bill.key, { invoiceDate: e.target.value })}
                            />
                          </div>

                          <div className="space-y-1">
                            <Label className="text-xs">Round Off (₹) &amp; Remarks</Label>
                            <div className="flex gap-2">
                              <Input
                                className="h-9 w-24 text-xs font-mono"
                                type="number"
                                step="1"
                                min={-MAX_ROUND_OFF_PAISE / 100}
                                max={MAX_ROUND_OFF_PAISE / 100}
                                aria-label={`Bill ${billIdx + 1} Round Off (INR)`}
                                value={bill.roundOffInr}
                                onChange={(e) => updateBatchBill(bill.key, { roundOffInr: e.target.value })}
                              />
                              <Input
                                className="h-9 flex-1 text-xs"
                                maxLength={1000}
                                placeholder="Bill remarks"
                                value={bill.remarks}
                                onChange={(e) => updateBatchBill(bill.key, { remarks: e.target.value })}
                              />
                            </div>
                          </div>
                        </div>

                        {/* Per-bill CHA Reimbursement toggle */}
                        <div className="rounded-md border bg-muted/20 p-3 space-y-2.5">
                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <div className="flex items-center gap-2">
                              <Switch
                                id={`cha-bill-${bill.key}`}
                                checked={bill.isPaidByClearingAgent}
                                onCheckedChange={(checked) => {
                                  if (checked) {
                                    const fallbackCa =
                                      bill.clearingAgentId ||
                                      (sharedClearingAgentId !== bill.serviceProviderId
                                        ? sharedClearingAgentId
                                        : '');
                                    updateBatchBill(bill.key, {
                                      isPaidByClearingAgent: true,
                                      clearingAgentId: fallbackCa,
                                      chaReimburseInvoiceNo:
                                        bill.chaReimburseInvoiceNo || sharedChaReimburseInvoiceNo,
                                      chaReimburseInvoiceDate:
                                        bill.chaReimburseInvoiceDate || sharedChaReimburseInvoiceDate,
                                    });
                                  } else {
                                    updateBatchBill(bill.key, { isPaidByClearingAgent: false });
                                  }
                                }}
                              />
                              <Label htmlFor={`cha-bill-${bill.key}`} className="text-xs font-semibold cursor-pointer">
                                Paid by Clearing Agent (Reimbursement)
                              </Label>
                            </div>
                            {billSameCha && (
                              <span className="text-[11px] font-medium text-destructive">
                                Clearing Agent cannot be the same as the Service Provider
                              </span>
                            )}
                          </div>

                          {bill.isPaidByClearingAgent && (
                            <div className="grid gap-3 sm:grid-cols-3 pt-2 border-t">
                              <div className="space-y-1">
                                <Label className="text-xs">Clearing Agent (Payable To) *</Label>
                                <SearchableCombobox
                                  value={bill.clearingAgentId}
                                  onValueChange={(v) => updateBatchBill(bill.key, { clearingAgentId: v })}
                                  ariaLabel={`Bill ${billIdx + 1} Clearing Agent`}
                                  placeholder="Select clearing agent"
                                  searchPlaceholder="Search clearing agent..."
                                  className="h-8 text-xs"
                                  options={providers
                                    .filter((p) => p.id !== bill.serviceProviderId)
                                    .map((p) => ({
                                      value: p.id,
                                      label: p.name,
                                      sublabel: [p.gstin, p.state].filter(Boolean).join(' · ') || undefined,
                                      keywords: `${p.name} ${p.gstin || ''} ${p.state || ''}`,
                                    }))}
                                />
                              </div>
                              <div className="space-y-1">
                                <Label className="text-xs">CHA Reimburse Invoice No</Label>
                                <Input
                                  className="h-8 text-xs font-mono"
                                  maxLength={100}
                                  aria-label={`Bill ${billIdx + 1} CHA Reimburse Invoice No`}
                                  placeholder="Shared or custom CHA Ref #"
                                  value={bill.chaReimburseInvoiceNo}
                                  onChange={(e) =>
                                    updateBatchBill(bill.key, { chaReimburseInvoiceNo: e.target.value })
                                  }
                                />
                              </div>
                              <div className="space-y-1">
                                <Label className="text-xs">CHA Reimburse Date</Label>
                                <Input
                                  className="h-8 text-xs"
                                  type="date"
                                  value={bill.chaReimburseInvoiceDate}
                                  onChange={(e) =>
                                    updateBatchBill(bill.key, { chaReimburseInvoiceDate: e.target.value })
                                  }
                                />
                              </div>
                            </div>
                          )}
                        </div>

                        {isIntraBatchDup && (
                          <div className="flex items-center gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-xs text-destructive font-medium">
                            <AlertTriangle className="h-4 w-4 shrink-0" />
                            <span>
                              Duplicate provider &amp; invoice number within this batch. Update one of the invoice numbers.
                            </span>
                          </div>
                        )}

                        {existingInDb && (
                          <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-blue-500/40 bg-blue-500/10 px-3 py-2 text-xs">
                            <span className="flex items-center gap-2 font-medium text-blue-900 dark:text-blue-200">
                              <AlertTriangle className="h-4 w-4 shrink-0 text-blue-600 dark:text-blue-400" />
                              Invoice &ldquo;{existingInDb.invoiceNo}&rdquo; already exists for {billProv?.name}.
                            </span>
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-xs"
                              onClick={() => openEditInvoice(existingInDb)}
                            >
                              Edit Existing #{existingInDb.invoiceNo}
                            </Button>
                          </div>
                        )}

                        <ExpenseLinesGrid
                          lines={bill.lines}
                          types={types}
                          supplyType={billSupply}
                          billLabelPrefix={`Bill ${billIdx + 1} `}
                          lineAmountRefs={lineAmountRefs}
                          onChangeLines={(nextLines) => updateBatchBill(bill.key, { lines: nextLines })}
                          onAddLine={() => {
                            const nextEt = pickNextUnusedType(bill.lines);
                            const created = emptyLine(
                              nextEt,
                              bill.lines.some((l) => l.isManualGst),
                              billSupply,
                            );
                            pendingFocusKeyRef.current = created.key;
                            updateBatchBill(bill.key, { lines: [...bill.lines, created] });
                          }}
                          onDuplicateLine={(lineIdx) => {
                            const src = bill.lines[lineIdx];
                            if (!src) return;
                            const nextEt = pickNextUnusedType(bill.lines);
                            const adapted = nextEt ? adaptExpenseTypeRatesForSupply(nextEt, billSupply) : null;
                            const copy: DraftLine = {
                              ...src,
                              key: makeLineKey(),
                              expenseTypeId: nextEt?.id ?? src.expenseTypeId,
                              cgstPct: adapted ? (adapted.cgstBp / 100).toFixed(2) : src.cgstPct,
                              sgstPct: adapted ? (adapted.sgstBp / 100).toFixed(2) : src.sgstPct,
                              igstPct: adapted ? (adapted.igstBp / 100).toFixed(2) : src.igstPct,
                              tdsPct: adapted ? (adapted.tdsBp / 100).toFixed(2) : src.tdsPct,
                            };
                            pendingFocusKeyRef.current = copy.key;
                            updateBatchBill(bill.key, {
                              lines: [
                                ...bill.lines.slice(0, lineIdx + 1),
                                copy,
                                ...bill.lines.slice(lineIdx + 1),
                              ],
                            });
                          }}
                          onCombineDuplicates={() => void handleBatchCombineDuplicates(bill.key)}
                          onCopyLastBill={
                            bill.serviceProviderId
                              ? () => handleCopyLastBillForBatch(bill.key)
                              : undefined
                          }
                          onOpenPaste={() => {
                            setPasteTargetBillKey(bill.key);
                            setPasteOpen(true);
                          }}
                          onOpenManageTypes={() => {
                            setTargetBatchBillKey(bill.key);
                            setTargetLineIdx(null);
                            setManageEtOpen(true);
                          }}
                        />

                        {/* Per-bill mini summary strip */}
                        <div className="flex flex-wrap items-center justify-end gap-4 rounded-md bg-muted/30 px-3 py-2 text-xs font-mono">
                          <span>Base: INR {formatPaiseINR(billCalc.baseAmountPaise)}</span>
                          <span>
                            GST: INR{' '}
                            {formatPaiseINR(
                              billCalc.totalCgstPaise + billCalc.totalSgstPaise + billCalc.totalIgstPaise,
                            )}
                          </span>
                          <span>TDS: INR {formatPaiseINR(billCalc.totalTdsPaise)}</span>
                          <span className="font-bold text-primary">
                            Net: INR {formatPaiseINR(billCalc.netAmountPaise)}
                          </span>
                        </div>
                      </CardContent>
                    )}
                  </Card>
                );
              })}

              <div className="flex justify-center">
                <Button type="button" variant="outline" className="gap-1.5" onClick={addBatchBill}>
                  <Plus className="h-4 w-4" />
                  Add Another Service Provider Invoice
                </Button>
              </div>

              {/* Grand Batch Summary Card */}
              <Card className="border-primary/30 bg-muted/20">
                <CardHeader className="py-3 px-6 border-b">
                  <CardTitle className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    Batch Summary ({batchBills.length} Provider Invoice{batchBills.length === 1 ? '' : 's'})
                  </CardTitle>
                </CardHeader>
                <CardContent className="p-5 space-y-3">
                  <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
                    <div className="rounded-lg border bg-card p-3">
                      <span className="text-xs text-muted-foreground">Total Base</span>
                      <div className="font-mono text-base font-bold mt-0.5">
                        INR {formatPaiseINR(batchSummary.basePaise)}
                      </div>
                    </div>
                    <div className="rounded-lg border bg-card p-3">
                      <span className="text-xs text-muted-foreground">Total GST</span>
                      <div className="font-mono text-base font-bold mt-0.5">
                        INR {formatPaiseINR(batchSummary.gstPaise)}
                      </div>
                    </div>
                    <div className="rounded-lg border bg-card p-3">
                      <span className="text-xs text-muted-foreground">Total TDS</span>
                      <div className="font-mono text-base font-bold mt-0.5">
                        INR {formatPaiseINR(batchSummary.tdsPaise)}
                      </div>
                    </div>
                    <div className="rounded-lg border bg-card p-3">
                      <span className="text-xs text-muted-foreground">Direct to Providers ({batchSummary.directCount})</span>
                      <div className="font-mono text-base font-bold mt-0.5">
                        INR {formatPaiseINR(batchSummary.directNetPaise)}
                      </div>
                    </div>
                    <div className="rounded-lg border bg-card p-3">
                      <span className="text-xs text-purple-700 dark:text-purple-300">CHA Reimbursable ({batchSummary.chaCount})</span>
                      <div className="font-mono text-base font-bold text-purple-700 dark:text-purple-300 mt-0.5">
                        INR {formatPaiseINR(batchSummary.chaNetPaise)}
                      </div>
                    </div>
                    <div className="rounded-lg border border-primary/40 bg-primary/5 p-3">
                      <span className="text-xs text-primary font-semibold">Combined Net</span>
                      <div className="font-mono text-lg font-extrabold text-primary mt-0.5">
                        INR {formatPaiseINR(batchSummary.netPaise)}
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>
          ) : (
            <>
              {/* Invoice Information Header Card */}
              <Card>
                <CardHeader className="pb-3">
                  <CardTitle className="text-base font-semibold">Invoice Information</CardTitle>
                  <CardDescription>Select the shipment and service provider, and enter invoice reference details.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                    <div className="space-y-1.5">
                      <Label>Shipment *</Label>
                      <SearchableCombobox
                        value={shipmentId}
                        onValueChange={setShipmentId}
                        ariaLabel="Shipment"
                        placeholder="Choose shipment"
                        searchPlaceholder="Search shipment invoice or supplier..."
                        options={shipments
                          .filter((s) => !s.isFrozen || s.id === shipmentId)
                          .map((s) => ({
                            value: s.id,
                            label: `${s.invoiceNumber} — ${s.supplierName || 'No Supplier'}`,
                            sublabel: `${s.invoiceDate || ''}${s.blAwbNumber ? ` · BL/AWB: ${s.blAwbNumber}` : ''}`,
                            keywords: `${s.invoiceNumber} ${s.supplierName} ${s.blAwbNumber || ''}`,
                          }))}
                      />
                    </div>

                    <div className="space-y-1.5">
                      <div className="flex items-center justify-between">
                        <Label>Service Provider *</Label>
                        <button
                          type="button"
                          tabIndex={-1}
                          className="inline-flex items-center gap-1 text-xs text-primary hover:underline cursor-pointer"
                          onClick={() => {
                            setTargetBatchBillKey(null);
                            setManageSpOpen(true);
                          }}
                        >
                          <Settings2 className="h-3 w-3" />
                          Manage
                        </button>
                      </div>
                      <SearchableCombobox
                        value={serviceProviderId}
                        onValueChange={handleProviderChange}
                        ariaLabel="Service Provider"
                        placeholder="Choose provider"
                        searchPlaceholder="Search provider or GSTIN..."
                        options={providers.map((p) => ({
                          value: p.id,
                          label: p.name,
                          sublabel: [p.gstin, p.state].filter(Boolean).join(' · ') || undefined,
                          keywords: `${p.name} ${p.gstin || ''} ${p.state || ''}`,
                        }))}
                      />
                    </div>

                    <div className="space-y-1.5">
                      <Label htmlFor="expInvNo">Invoice No *</Label>
                      <Input
                        id="expInvNo"
                        maxLength={100}
                        placeholder="e.g. INV-2026-001"
                        value={invoiceNo}
                        onChange={(e) => setInvoiceNo(e.target.value)}
                      />
                    </div>

                    <div className="space-y-1.5">
                      <Label htmlFor="expInvDate">Invoice Date *</Label>
                      <Input
                        id="expInvDate"
                        type="date"
                        value={invoiceDate}
                        onChange={(e) => setInvoiceDate(e.target.value)}
                      />
                    </div>
                  </div>

                  {/* Clearing Agent (CHA) Reimbursement Section */}
                  <div className="rounded-lg border p-3.5 bg-muted/20 space-y-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="space-y-0.5">
                        <div className="flex items-center gap-2">
                          <Label htmlFor="cha-toggle" className="text-sm font-semibold cursor-pointer">
                            Paid by Clearing Agent (Reimbursement)
                          </Label>
                          <Badge variant="outline" className="text-[10px] font-normal">
                            e.g. Liner &amp; CFS Charges
                          </Badge>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          Check this if charges were paid upfront by your clearing agent. The expense remains attributed to the original provider for GST credit, but payment liability is linked to the clearing agent.
                        </p>
                      </div>
                      <Switch
                        id="cha-toggle"
                        checked={isPaidByClearingAgent}
                        onCheckedChange={setIsPaidByClearingAgent}
                      />
                    </div>

                    {isPaidByClearingAgent && (
                      <div className="grid gap-4 sm:grid-cols-3 pt-3 border-t">
                        <div className="space-y-1.5">
                          <Label htmlFor="clearingAgentSelect">Clearing Agent (Payable To) *</Label>
                          <SearchableCombobox
                            triggerId="clearingAgentSelect"
                            value={clearingAgentId}
                            onValueChange={setClearingAgentId}
                            ariaLabel="Clearing Agent"
                            placeholder="Select clearing agent"
                            searchPlaceholder="Search clearing agent..."
                            options={providers
                              .filter((p) => p.id !== serviceProviderId)
                              .map((p) => ({
                                value: p.id,
                                label: p.name,
                                sublabel: [p.gstin, p.state].filter(Boolean).join(' · ') || undefined,
                                keywords: `${p.name} ${p.gstin || ''} ${p.state || ''}`,
                              }))}
                          />
                        </div>

                        <div className="space-y-1.5">
                          <Label htmlFor="chaReimbInvNo">CHA Reimburse Invoice No</Label>
                          <Input
                            id="chaReimbInvNo"
                            maxLength={100}
                            placeholder="e.g. CHA-REIMB-2026-001"
                            value={chaReimburseInvoiceNo}
                            onChange={(e) => setChaReimburseInvoiceNo(e.target.value)}
                            className="font-mono text-sm"
                          />
                        </div>

                        <div className="space-y-1.5">
                          <Label htmlFor="chaReimbInvDate">CHA Reimburse Date</Label>
                          <Input
                            id="chaReimbInvDate"
                            type="date"
                            value={chaReimburseInvoiceDate}
                            onChange={(e) => setChaReimburseInvoiceDate(e.target.value)}
                          />
                        </div>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              {matchingExistingInvoice && (
                <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-blue-500/40 bg-blue-500/10 px-4 py-3 text-xs">
                  <span className="flex items-center gap-2 font-medium text-blue-900 dark:text-blue-200">
                    <AlertTriangle className="h-4 w-4 shrink-0 text-blue-600 dark:text-blue-400" />
                    Invoice &ldquo;{matchingExistingInvoice.invoiceNo}&rdquo; already exists for this service provider ({matchingExistingInvoice.lines?.length ?? 0} line(s) recorded).
                    To add multiple services (transport, customs clearance, etc.) to this bill, edit the existing invoice.
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    className="border-blue-500/40 hover:bg-blue-500/20 text-xs shrink-0"
                    onClick={() => openEditInvoice(matchingExistingInvoice)}
                  >
                    <Pencil className="h-3.5 w-3.5 mr-1" />
                    Edit Existing Invoice #{matchingExistingInvoice.invoiceNo}
                  </Button>
                </div>
              )}

              {/* Expense Lines Compact Grid Card */}
              <Card>
                <CardHeader className="pb-3">
                  <CardTitle className="text-base font-semibold">Expense Lines ({draftLines.length})</CardTitle>
                </CardHeader>
                <CardContent>
                  <ExpenseLinesGrid
                    lines={draftLines}
                    types={types}
                    supplyType={supplyType}
                    lineAmountRefs={lineAmountRefs}
                    onChangeLines={(next) => {
                      setDraftLines(next);
                      scheduleBackendPreview(next);
                    }}
                    onAddLine={() => addDraftLine(true)}
                    onDuplicateLine={(idx) => duplicateDraftLine(idx)}
                    onCombineDuplicates={() => void handleCombineDuplicates()}
                    onCopyLastBill={!editingInv && serviceProviderId ? handleCopyLastBillForProvider : undefined}
                    onOpenPaste={() => {
                      setPasteTargetBillKey(null);
                      setPasteOpen(true);
                    }}
                    onOpenManageTypes={() => {
                      setTargetBatchBillKey(null);
                      setTargetLineIdx(null);
                      setManageEtOpen(true);
                    }}
                  />
                </CardContent>
              </Card>

              {/* Invoice Remarks Card */}
              <Card>
                <CardContent className="pt-5">
                  <div className="space-y-1.5">
                    <Label htmlFor="expInvRemarks" className="text-sm font-medium">Invoice Remarks</Label>
                    <Input
                      id="expInvRemarks"
                      maxLength={1000}
                      placeholder="Optional notes or reference instructions for this expense invoice"
                      value={remarks}
                      onChange={(e) => setRemarks(e.target.value)}
                    />
                  </div>
                </CardContent>
              </Card>

              {/* Calculation Summary Card */}
              {preview && (
                <Card className="border-primary/20 bg-muted/20">
                  <CardHeader className="py-3 px-6 border-b">
                    <CardTitle className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                      Computed Totals (Whole Rupee / Rounded)
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="p-6">
                    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4">
                      <div className="rounded-lg border bg-card p-4 shadow-xs">
                        <span className="text-xs text-muted-foreground font-medium">Base (Excl. GST)</span>
                        <div className="font-mono text-lg font-bold text-foreground mt-1">
                          INR {formatPaiseINR(preview.baseAmountPaise)}
                        </div>
                      </div>
                      <div className="rounded-lg border bg-card p-4 shadow-xs">
                        <div className="flex items-center justify-between">
                          <span className="text-xs text-muted-foreground font-medium">GST Total</span>
                          <span className="text-[10px] text-muted-foreground">(CGST+SGST+IGST)</span>
                        </div>
                        <div className="font-mono text-lg font-bold text-foreground mt-1">
                          INR {formatPaiseINR(preview.totalCgstPaise + preview.totalSgstPaise + preview.totalIgstPaise)}
                        </div>
                        <div className="text-[10px] text-muted-foreground mt-0.5">
                          CGST: {formatPaiseINR(preview.totalCgstPaise)} | SGST: {formatPaiseINR(preview.totalSgstPaise)} | IGST: {formatPaiseINR(preview.totalIgstPaise)}
                        </div>
                      </div>
                      <div className="rounded-lg border border-border/80 bg-muted/40 p-4 shadow-xs">
                        <span className="text-xs text-muted-foreground font-medium">Invoice Total</span>
                        <div className="font-mono text-lg font-bold text-foreground mt-1">
                          INR {formatPaiseINR(
                            preview.baseAmountPaise +
                              preview.totalCgstPaise +
                              preview.totalSgstPaise +
                              preview.totalIgstPaise +
                              roundOffPaiseVal
                          )}
                        </div>
                        <div className="text-[10px] text-muted-foreground mt-0.5">
                          Base + GST (w/o TDS)
                        </div>
                      </div>
                      <div className="rounded-lg border bg-card p-4 shadow-xs">
                        <span className="text-xs text-muted-foreground font-medium">TDS Deducted</span>
                        <div className="font-mono text-lg font-bold text-foreground mt-1">
                          INR {formatPaiseINR(preview.totalTdsPaise)}
                        </div>
                      </div>
                      <div className={`rounded-lg border bg-card p-4 shadow-xs ${roundOffOutOfRange ? 'border-destructive' : ''}`}>
                        <span className="text-xs text-muted-foreground font-medium">Round Off (₹)</span>
                        <div className="mt-1">
                          <Input
                            className="h-8 text-sm font-mono font-bold"
                            type="number"
                            step="1"
                            min={-MAX_ROUND_OFF_PAISE / 100}
                            max={MAX_ROUND_OFF_PAISE / 100}
                            placeholder="0"
                            aria-label="Round Off (INR)"
                            value={roundOffInr}
                            onChange={(e) => setRoundOffInr(e.target.value)}
                          />
                        </div>
                        <span className={`text-[10px] mt-0.5 block ${roundOffOutOfRange ? 'text-destructive font-medium' : 'text-muted-foreground'}`}>
                          {roundOffOutOfRange ? `Max ±₹${MAX_ROUND_OFF_PAISE / 100}` : `Whole rupee (±₹${MAX_ROUND_OFF_PAISE / 100})`}
                        </span>
                      </div>
                      <div className={`rounded-lg border p-4 shadow-xs ${negativeNetPayable ? 'border-destructive bg-destructive/10' : 'border-primary/40 bg-primary/5'}`}>
                        <span className={`text-xs font-semibold ${negativeNetPayable ? 'text-destructive' : 'text-primary'}`}>Net Payable</span>
                        <div className={`font-mono text-xl font-extrabold mt-1 ${negativeNetPayable ? 'text-destructive' : 'text-primary'}`}>
                          INR {formatPaiseINR(preview.netAmountPaise + roundOffPaiseVal)}
                        </div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )}
            </>
          )}

          {/* Bottom Action Footer */}
          <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-4">
            <Button variant="outline" onClick={requestCloseEditor}>
              Cancel
            </Button>
            <div className="flex items-center gap-2">
              <Button variant="outline" onClick={requestCloseEditor}>
                Back to List
              </Button>
              {entryMode === 'batch' ? (
                <Button
                  disabled={savingInv || !shipmentId || batchBills.length === 0 || batchSummary.hasAnyErrors}
                  onClick={() => void handleSaveBatch()}
                >
                  {savingInv ? 'Saving Batch...' : `Save All ${batchBills.length} Invoices`}
                </Button>
              ) : (
                <>
                  {!editingInv && (
                    <Button
                      variant="secondary"
                      disabled={!canSubmitInvoice}
                      onClick={() => void handleSaveInvoice(true)}
                    >
                      {savingInv ? 'Saving...' : 'Save & New'}
                    </Button>
                  )}
                  <Button
                    disabled={!canSubmitInvoice}
                    onClick={() => void handleSaveInvoice(false)}
                  >
                    {savingInv ? 'Saving...' : 'Save Expense Invoice'}
                  </Button>
                </>
              )}
            </div>
          </div>

          <ExpensePasteLinesDialog
            open={pasteOpen}
            onOpenChange={(open) => {
              setPasteOpen(open);
              if (!open) setPasteTargetBillKey(null);
            }}
            pasteText={pasteText}
            setPasteText={setPasteText}
            onApplyPaste={handleApplyPaste}
          />
        </div>
      ) : (
        <>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h1 className="text-2xl font-bold tracking-tight">Import Expenses Ledger</h1>
              <p className="text-sm text-muted-foreground">
                Exact integer-paise &amp; basis-point GST/TDS expense invoices, providers, and bulk import.
              </p>
            </div>
            {canWrite && (
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="outline"
                  onClick={() => {
                    const initialShipmentId =
                      (shipmentFilter && shipments.find((s) => s.id === shipmentFilter && !s.isFrozen)?.id) ||
                      '__MULTIPLE__';
                    setBulkShipmentId(initialShipmentId);
                    setBulkOpen(true);
                  }}
                >
                  <Upload className="h-4 w-4" />
                  Bulk CSV Import
                </Button>
                <Button onClick={openCreateInvoice}>
                  <Plus className="h-4 w-4" />
                  New Expense Invoice
                </Button>
              </div>
            )}
          </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="grid grid-cols-2 sm:grid-cols-5 w-full sm:w-auto h-auto">
          <TabsTrigger value="invoices" className="gap-2" onClick={() => setActiveTab('invoices')}>
            <Layers className="h-4 w-4" />
            <span>Expense Invoices ({displayInvoices.length})</span>
          </TabsTrigger>
          <TabsTrigger value="itemized" className="gap-2" onClick={() => setActiveTab('itemized')}>
            <Receipt className="h-4 w-4" />
            <span>Itemized Expense Details ({allItemizedLines.length})</span>
          </TabsTrigger>
          <TabsTrigger value="ledger" className="gap-2" onClick={() => setActiveTab('ledger')}>
            <History className="h-4 w-4" />
            <span>Allocation Ledger ({ledgerRecords.length})</span>
          </TabsTrigger>
          <TabsTrigger value="providers" onClick={() => setActiveTab('providers')}>Service Providers ({providers.length})</TabsTrigger>
          <TabsTrigger value="types" onClick={() => setActiveTab('types')}>Expense Types ({types.length})</TabsTrigger>
        </TabsList>

        <TabsContent value="invoices" className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-3">
              <div className="relative w-72">
                <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  className="pl-8"
                  placeholder="Search provider, invoice no..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <Select
                value={shipmentFilter || 'all'}
                onValueChange={(v) => setShipmentFilter(v === 'all' ? '' : v)}
              >
                <SelectTrigger className="w-64">
                  <SelectValue placeholder="All shipments" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Shipments</SelectItem>
                  {shipments.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.invoiceNumber} — {s.supplierName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {displayInvoices.length > 0 && (
              <Button
                variant="outline"
                size="sm"
                onClick={toggleAllInvoicesExpand}
                className="gap-1.5"
              >
                {allInvoicesExpanded ? (
                  <>
                    <ChevronsDownUp className="h-4 w-4" />
                    <span>Collapse All Details</span>
                  </>
                ) : (
                  <>
                    <ChevronDown className="h-4 w-4" />
                    <span>Expand All Details</span>
                  </>
                )}
              </Button>
            )}
          </div>

          {clearingAgentSummaries.length > 0 && (
            <div className="space-y-2">
              {clearingAgentSummaries.map((cha) => (
                <div
                  key={cha.agentId}
                  className="rounded-lg border border-purple-200/90 bg-purple-50/60 p-3.5 dark:border-purple-900/60 dark:bg-purple-950/30 flex flex-wrap items-center justify-between gap-3 shadow-xs"
                >
                  <div className="flex items-center gap-3">
                    <div className="h-9 w-9 rounded-full bg-purple-600 text-white flex items-center justify-center font-bold text-xs shadow-xs">
                      CHA
                    </div>
                    <div>
                      <div className="font-semibold text-sm flex items-center gap-2">
                        <span>{cha.agentName} Total Payable:</span>
                        <span className="font-mono text-base font-bold text-purple-700 dark:text-purple-300">
                          {formatPaiseINR(cha.totalNetPaise)}
                        </span>
                      </div>
                      <div className="text-xs text-muted-foreground flex flex-wrap items-center gap-x-2 pt-0.5">
                        <span>
                          Direct Services: {formatPaiseINR(cha.directNetPaise)} ({cha.directCount} invoice{cha.directCount === 1 ? '' : 's'})
                        </span>
                        <span>•</span>
                        <span>
                          Reimbursed CFS/Warehouse: {formatPaiseINR(cha.reimbNetPaise)} ({cha.refList.map((r) => `Ref: ${r}`).join(', ')})
                        </span>
                      </div>
                    </div>
                  </div>
                  <Badge variant="outline" className="border-purple-300 text-purple-700 bg-purple-100/60 dark:border-purple-700 dark:text-purple-300 dark:bg-purple-900/40 text-xs px-2.5 py-1 font-mono font-medium">
                    Tally Ledger Match: {formatPaiseINR(cha.totalNetPaise)}
                  </Badge>
                </div>
              ))}
            </div>
          )}

          <Card>
            <CardContent className="pt-6">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-10 text-center">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 w-6 p-0 hover:bg-muted"
                        onClick={toggleAllInvoicesExpand}
                        title={allInvoicesExpanded ? 'Collapse All' : 'Expand All'}
                      >
                        {allInvoicesExpanded ? (
                          <ChevronDown className="h-4 w-4 text-primary" />
                        ) : (
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
                        )}
                      </Button>
                    </TableHead>
                    <TableHead>Invoice No / Date</TableHead>
                    <TableHead>Service Provider</TableHead>
                    <TableHead>Shipment</TableHead>
                    <TableHead>Lines</TableHead>
                    <TableHead className="text-right">Basic (INR)</TableHead>
                    <TableHead className="text-right">GST (INR)</TableHead>
                    <TableHead className="text-right">Invoice Total (INR)</TableHead>
                    <TableHead className="text-right">TDS (INR)</TableHead>
                    <TableHead className="text-right">Net Payable (INR)</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={11} className="text-center text-muted-foreground py-8">
                        <div className="flex items-center justify-center gap-2">
                          <RefreshCw className="h-4 w-4 animate-spin text-primary" />
                          <span>Loading expense ledger...</span>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : displayInvoices.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={11} className="text-center text-muted-foreground py-8">
                        No expense invoices recorded yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    pagedInvoices.map((inv) => {
                      const isExpanded = expandedInvoices.has(inv.id);
                      const gstPaise =
                        inv.totalCgstPaise + inv.totalSgstPaise + inv.totalIgstPaise;
                      const lineCount = inv.lines?.length ?? 0;
                      return (
                        <React.Fragment key={inv.id}>
                          <TableRow className={isExpanded ? 'bg-muted/20 border-b-0' : undefined}>
                            <TableCell className="text-center p-1">
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 w-7 text-muted-foreground hover:text-foreground p-0"
                                aria-label={isExpanded ? `Collapse details for ${inv.invoiceNo}` : `Expand details for ${inv.invoiceNo}`}
                                onClick={() => toggleInvoiceExpand(inv.id)}
                              >
                                {isExpanded ? (
                                  <ChevronDown className="h-4 w-4 text-primary" />
                                ) : (
                                  <ChevronRight className="h-4 w-4" />
                                )}
                              </Button>
                            </TableCell>
                            <TableCell>
                              <div className="font-medium flex items-center gap-1.5">
                                <span>{inv.invoiceNo}</span>
                                {inv.isVirtualReimbursement && (
                                  <Badge variant="outline" className="text-[10px] bg-purple-50 text-purple-700 border-purple-200 dark:bg-purple-950/50 dark:text-purple-300">
                                    CHA Reimbursement Bill
                                  </Badge>
                                )}
                              </div>
                              <div className="text-xs text-muted-foreground">
                                {inv.invoiceDate} {inv.isVirtualReimbursement ? '· Reimbursement' : `· v${inv.version}`}
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="font-medium">{inv.serviceProviderName || '—'}</div>
                              {inv.isVirtualReimbursement && inv.parentInvoice ? (
                                <div className="mt-1">
                                  <Badge variant="outline" className="text-[10px] bg-slate-100 text-slate-700 border-slate-200 dark:bg-slate-800 dark:text-slate-300">
                                    Agst Ref: {inv.parentInvoice.invoiceNo} ({inv.parentInvoice.serviceProviderName})
                                  </Badge>
                                </div>
                              ) : inv.isPaidByClearingAgent ? (
                                <div className="mt-1">
                                  <Badge variant="outline" className="text-[10px] bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/50 dark:text-blue-300">
                                    Paid by CHA: {inv.clearingAgentName || 'Clearing Agent'}
                                    {inv.chaReimburseInvoiceNo ? ` (Ref: ${inv.chaReimburseInvoiceNo})` : ''}
                                  </Badge>
                                </div>
                              ) : null}
                            </TableCell>
                            <TableCell className="font-mono text-xs">{inv.shipmentInvoiceNo || '—'}</TableCell>
                            <TableCell>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 px-2 font-normal text-xs gap-1 border border-border/60 hover:bg-muted"
                                onClick={() => toggleInvoiceExpand(inv.id)}
                                title="Click to view detailed line items"
                              >
                                <Badge variant="secondary" className="px-1.5 py-0 text-xs">
                                  {lineCount}
                                </Badge>
                                <span>{lineCount === 1 ? 'line' : 'lines'}</span>
                                {isExpanded ? (
                                  <ChevronDown className="h-3 w-3 text-muted-foreground" />
                                ) : (
                                  <ChevronRight className="h-3 w-3 text-muted-foreground" />
                                )}
                              </Button>
                            </TableCell>
                            <TableCell className="text-right font-mono">
                              {formatPaiseINR(inv.baseAmountPaise)}
                            </TableCell>
                            <TableCell className="text-right font-mono">
                              {inv.isVirtualReimbursement ? (
                                <div>
                                  <span className="text-muted-foreground">₹0.00</span>
                                  <div className="text-[10px] text-muted-foreground">ITC on CFS</div>
                                </div>
                              ) : (
                                formatPaiseINR(gstPaise)
                              )}
                            </TableCell>
                            <TableCell className="text-right font-mono font-medium">
                              <div>
                                {formatPaiseINR(inv.totalAmountPaise || (inv.baseAmountPaise + gstPaise + (inv.roundOffPaise ?? 0)))}
                              </div>
                              <div className="text-[10px] text-muted-foreground font-normal">
                                (w/o TDS)
                              </div>
                            </TableCell>
                            <TableCell className="text-right font-mono text-amber-600 dark:text-amber-400">
                              {inv.isVirtualReimbursement ? (
                                <div>
                                  <span className="text-muted-foreground">0.00</span>
                                  <div className="text-[10px] text-muted-foreground">On CFS bill</div>
                                </div>
                              ) : inv.totalTdsPaise > 0 ? (
                                `-${formatPaiseINR(inv.totalTdsPaise)}`
                              ) : (
                                '0.00'
                              )}
                            </TableCell>
                            <TableCell className="text-right font-mono font-semibold">
                              {inv.isVirtualReimbursement ? (
                                <div>
                                  <div className="text-purple-600 dark:text-purple-400 font-bold">
                                    {formatPaiseINR(inv.directNetPayablePaise)}
                                  </div>
                                  <div className="text-[10px] text-muted-foreground font-normal">
                                    Payable to CHA
                                  </div>
                                </div>
                              ) : inv.isPaidByClearingAgent ? (
                                <div>
                                  <div className="text-emerald-600 dark:text-emerald-400 font-bold">
                                    ₹0.00
                                  </div>
                                  <div className="text-[10px] text-muted-foreground line-through">
                                    Orig: {formatPaiseINR(inv.netAmountPaise)}
                                  </div>
                                  <div className="text-[10px] text-muted-foreground font-normal">
                                    Transferred to {inv.clearingAgentName || 'CHA'}
                                    {inv.chaReimburseInvoiceNo ? ` (Ref: ${inv.chaReimburseInvoiceNo})` : ''}
                                  </div>
                                </div>
                              ) : (
                                <div>
                                  <div>{formatPaiseINR(inv.netAmountPaise)}</div>
                                  {Boolean(inv.roundOffPaise) && (
                                    <div className="text-[10px] text-muted-foreground font-normal">
                                      (Round off: {(inv.roundOffPaise ?? 0) > 0 ? '+' : ''}{formatPaiseINR(inv.roundOffPaise ?? 0)})
                                    </div>
                                  )}
                                </div>
                              )}
                            </TableCell>
                            <TableCell className="text-right">
                              <div className="flex justify-end gap-1">
                                {canWrite && (
                                  <Button
                                    size="sm"
                                    variant="ghost"
                                    aria-label={`Edit ${inv.invoiceNo}`}
                                    title={inv.isVirtualReimbursement ? `Edit parent invoice ${inv.parentInvoice?.invoiceNo}` : `Edit ${inv.invoiceNo}`}
                                    onClick={() => {
                                      if (inv.isVirtualReimbursement && inv.parentInvoice) {
                                        openEditInvoice(inv.parentInvoice);
                                      } else {
                                        openEditInvoice(inv);
                                      }
                                    }}
                                  >
                                    <Pencil className="h-4 w-4" />
                                  </Button>
                                )}
                                {canDelete && (
                                  <Button
                                    size="sm"
                                    variant="ghost"
                                    aria-label={`Delete ${inv.invoiceNo}`}
                                    title={inv.isVirtualReimbursement ? `Unlink reimbursement ref ${inv.invoiceNo}` : `Delete ${inv.invoiceNo}`}
                                    onClick={() => {
                                      if (inv.isVirtualReimbursement && inv.parentInvoice) {
                                        void handleUnlinkReimbursement(inv.parentInvoice);
                                      } else {
                                        void expenseService
                                          .deleteExpenseInvoice(inv.id)
                                          .then(loadAll)
                                          .then(() => toast.success('Expense invoice moved to Recycle Bin'))
                                          .catch((err) => toast.error(String(err)));
                                      }
                                    }}
                                  >
                                    <Trash2 className="h-4 w-4 text-destructive" />
                                  </Button>
                                )}
                              </div>
                            </TableCell>
                          </TableRow>

                          {/* Expanded Line Items Breakdown */}
                          {isExpanded && (
                            <TableRow className="bg-muted/10 hover:bg-muted/10 border-t-0">
                              <TableCell colSpan={11} className="p-0">
                                <div className="p-4 pl-10 pr-4 border-b border-border/60 bg-slate-950/30 dark:bg-slate-900/40 space-y-3">
                                  <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                                    <div className="flex items-center gap-2 font-medium text-foreground">
                                      <Receipt className="h-4 w-4 text-primary" />
                                      <span className="font-semibold">
                                        {inv.isVirtualReimbursement
                                          ? `Reimbursed CFS/Warehouse Details for ${inv.parentInvoice?.serviceProviderName} (${inv.parentInvoice?.invoiceNo})`
                                          : `Expense Line Details for Invoice ${inv.invoiceNo}`}
                                      </span>
                                      <Badge variant="outline" className="text-[11px]">
                                        {lineCount} line item{lineCount === 1 ? '' : 's'}
                                      </Badge>
                                      {Boolean(inv.roundOffPaise) && (
                                        <Badge variant="outline" className="text-[11px] text-muted-foreground">
                                          Round Off: {(inv.roundOffPaise ?? 0) > 0 ? '+' : ''}{formatPaiseINR(inv.roundOffPaise ?? 0)}
                                        </Badge>
                                      )}
                                      {inv.remarks && (
                                        <span className="text-muted-foreground italic">
                                          &ldquo;{inv.remarks}&rdquo;
                                        </span>
                                      )}
                                    </div>
                                    {canWrite && (
                                      <Button
                                        size="sm"
                                        variant="outline"
                                        className="h-7 text-xs gap-1.5"
                                        onClick={() => openEditInvoice(inv)}
                                      >
                                        <Pencil className="h-3 w-3" />
                                        Edit / Add Lines
                                      </Button>
                                    )}
                                  </div>

                                  {!inv.lines || inv.lines.length === 0 ? (
                                    <div className="p-4 text-xs text-muted-foreground bg-muted/20 rounded border border-dashed text-center">
                                      No individual line items found for this invoice.
                                    </div>
                                  ) : (
                                    <div className="rounded-md border border-border/70 overflow-hidden bg-background">
                                      <Table>
                                        <TableHeader className="bg-muted/50">
                                          <TableRow className="hover:bg-transparent text-xs">
                                            <TableHead className="w-10 text-center py-2">#</TableHead>
                                            <TableHead className="py-2">Expense Type</TableHead>
                                            <TableHead className="text-right py-2">Basic (INR)</TableHead>
                                            <TableHead className="text-right py-2">CGST</TableHead>
                                            <TableHead className="text-right py-2">SGST</TableHead>
                                            <TableHead className="text-right py-2">IGST</TableHead>
                                            <TableHead className="text-right py-2">Line Total (INR)</TableHead>
                                            <TableHead className="text-right py-2">TDS</TableHead>
                                            <TableHead className="text-right py-2 font-semibold">Net Payable (INR)</TableHead>
                                            <TableHead className="py-2">Remarks</TableHead>
                                          </TableRow>
                                        </TableHeader>
                                        <TableBody>
                                          {inv.lines.map((line, idx) => (
                                            <TableRow key={line.id || `line-${idx}`} className="text-xs hover:bg-muted/30">
                                              <TableCell className="text-center font-mono text-muted-foreground py-2">
                                                {idx + 1}
                                              </TableCell>
                                              <TableCell className="font-medium py-2">
                                                <Badge variant="secondary" className="font-medium">
                                                  {line.expenseTypeName || 'Expense'}
                                                </Badge>
                                              </TableCell>
                                              <TableCell className="text-right font-mono py-2 font-medium">
                                                {formatPaiseINR(line.amountPaise)}
                                              </TableCell>
                                              <TableCell className="text-right font-mono text-muted-foreground py-2">
                                                {line.cgstPaise > 0 ? (
                                                  <span>
                                                    {line.isManualGst ? (
                                                      <span className="text-[10px] text-amber-600 font-medium mr-1">Manual ·</span>
                                                    ) : line.cgstRateBp > 0 ? (
                                                      `${(line.cgstRateBp / 100).toFixed(1)}% · `
                                                    ) : null}
                                                    <span className="text-foreground">{formatPaiseINR(line.cgstPaise)}</span>
                                                  </span>
                                                ) : (
                                                  '—'
                                                )}
                                              </TableCell>
                                              <TableCell className="text-right font-mono text-muted-foreground py-2">
                                                {line.sgstPaise > 0 ? (
                                                  <span>
                                                    {line.isManualGst ? (
                                                      <span className="text-[10px] text-amber-600 font-medium mr-1">Manual ·</span>
                                                    ) : line.sgstRateBp > 0 ? (
                                                      `${(line.sgstRateBp / 100).toFixed(1)}% · `
                                                    ) : null}
                                                    <span className="text-foreground">{formatPaiseINR(line.sgstPaise)}</span>
                                                  </span>
                                                ) : (
                                                  '—'
                                                )}
                                              </TableCell>
                                              <TableCell className="text-right font-mono text-muted-foreground py-2">
                                                {line.igstPaise > 0 ? (
                                                  <span>
                                                    {line.isManualGst ? (
                                                      <span className="text-[10px] text-amber-600 font-medium mr-1">Manual ·</span>
                                                    ) : line.igstRateBp > 0 ? (
                                                      `${(line.igstRateBp / 100).toFixed(1)}% · `
                                                    ) : null}
                                                    <span className="text-foreground">{formatPaiseINR(line.igstPaise)}</span>
                                                  </span>
                                                ) : (
                                                  '—'
                                                )}
                                              </TableCell>
                                              <TableCell className="text-right font-mono py-2 font-medium">
                                                {formatPaiseINR(line.totalPaise || (line.amountPaise + line.cgstPaise + line.sgstPaise + line.igstPaise))}
                                              </TableCell>
                                              <TableCell className="text-right font-mono text-amber-500 py-2">
                                                {line.tdsRateBp > 0 ? (
                                                  <span>
                                                    -{(line.tdsRateBp / 100).toFixed(1)}% ·{' '}
                                                    <span>{formatPaiseINR(line.tdsPaise)}</span>
                                                  </span>
                                                ) : (
                                                  '—'
                                                )}
                                              </TableCell>
                                              <TableCell className="text-right font-mono font-semibold py-2">
                                                {formatPaiseINR(line.netPaise)}
                                              </TableCell>
                                              <TableCell className="text-muted-foreground max-w-xs truncate py-2" title={line.remarks}>
                                                {line.remarks || '—'}
                                              </TableCell>
                                            </TableRow>
                                          ))}
                                        </TableBody>
                                      </Table>
                                    </div>
                                  )}
                                </div>
                              </TableCell>
                            </TableRow>
                          )}
                        </React.Fragment>
                      );
                    })
                  )}
                </TableBody>
              </Table>

              <div className="flex items-center justify-between text-sm text-muted-foreground pt-4">
                <span>
                  Page {currentInvoicePage} of {totalInvoicePages}
                </span>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={currentInvoicePage <= 1}
                    onClick={() => setInvoicePage((p) => Math.max(1, p - 1))}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={currentInvoicePage >= totalInvoicePages}
                    onClick={() => setInvoicePage((p) => Math.min(totalInvoicePages, p + 1))}
                  >
                    Next
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="itemized" className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total Line Items</div>
              <div className="text-xl font-bold">{itemizedSummary.count}</div>
            </Card>
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total Basic (INR)</div>
              <div className="text-xl font-bold font-mono">{formatPaiseINR(itemizedSummary.basicPaise)}</div>
            </Card>
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total GST (INR)</div>
              <div className="text-xl font-bold font-mono text-blue-500">{formatPaiseINR(itemizedSummary.gstPaise)}</div>
            </Card>
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total Invoice Gross (INR)</div>
              <div className="text-xl font-bold font-mono text-foreground">{formatPaiseINR(itemizedSummary.totalPaise)}</div>
              <div className="text-[10px] text-muted-foreground">Basic + GST (w/o TDS)</div>
            </Card>
            <Card className="p-3">
              <div className="text-xs text-muted-foreground">Total Net Payable (INR)</div>
              <div className="text-xl font-bold font-mono text-green-500">{formatPaiseINR(itemizedSummary.netPaise)}</div>
            </Card>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative w-64">
                <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  className="pl-8 h-9 text-xs"
                  placeholder="Filter lines by invoice, type, remarks..."
                  value={itemizedSearch}
                  onChange={(e) => setItemizedSearch(e.target.value)}
                />
              </div>

              <Select
                value={itemizedTypeFilter}
                onValueChange={setItemizedTypeFilter}
              >
                <SelectTrigger className="w-48 h-9 text-xs">
                  <SelectValue placeholder="All Expense Types" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Expense Types</SelectItem>
                  {types.map((t) => (
                    <SelectItem key={t.id} value={t.id}>
                      {t.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              <Select
                value={itemizedProviderFilter}
                onValueChange={setItemizedProviderFilter}
              >
                <SelectTrigger className="w-48 h-9 text-xs">
                  <SelectValue placeholder="All Providers" />
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

              <Select
                value={itemizedShipmentFilter}
                onValueChange={setItemizedShipmentFilter}
              >
                <SelectTrigger className="w-52 h-9 text-xs">
                  <SelectValue placeholder="All Shipments" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Shipments</SelectItem>
                  {shipments.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.invoiceNumber} — {s.supplierName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={exportItemizedCsv}
              disabled={filteredItemizedLines.length === 0}
              className="h-9 gap-1.5"
            >
              <FileSpreadsheet className="h-4 w-4" />
              <span>Export CSV</span>
            </Button>
          </div>

          <Card>
            <CardContent className="pt-6">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Date / Invoice</TableHead>
                    <TableHead>Service Provider</TableHead>
                    <TableHead>Shipment</TableHead>
                    <TableHead>Expense Type</TableHead>
                    <TableHead className="text-right">Basic (INR)</TableHead>
                    <TableHead className="text-right">GST (INR)</TableHead>
                    <TableHead className="text-right">Invoice Total (INR)</TableHead>
                    <TableHead className="text-right">TDS (INR)</TableHead>
                    <TableHead className="text-right">Net Payable (INR)</TableHead>
                    <TableHead>Remarks</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredItemizedLines.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={11} className="text-center text-muted-foreground py-8">
                        No itemized expense lines match the current filters.
                      </TableCell>
                    </TableRow>
                  ) : (
                    pagedItemizedLines.map((line) => {
                      const gstPaise = line.cgstPaise + line.sgstPaise + line.igstPaise;
                      return (
                        <TableRow key={line.lineId}>
                          <TableCell>
                            <div className="font-medium">{line.invoiceNo}</div>
                            <div className="text-xs text-muted-foreground">{line.invoiceDate}</div>
                            {line.isPaidByClearingAgent && line.chaReimburseInvoiceNo && (
                              <div className="text-[10px] text-purple-600 dark:text-purple-400 font-medium">
                                CHA Bill: {line.chaReimburseInvoiceNo}
                              </div>
                            )}
                          </TableCell>
                          <TableCell>
                            <div>{line.serviceProviderName}</div>
                            {line.isPaidByClearingAgent && (
                              <div className="mt-0.5">
                                <Badge variant="outline" className="text-[10px] bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/50 dark:text-blue-300">
                                  Paid by CHA: {line.clearingAgentName || 'Clearing Agent'}
                                  {line.chaReimburseInvoiceNo ? ` (Ref: ${line.chaReimburseInvoiceNo})` : ''}
                                </Badge>
                              </div>
                            )}
                          </TableCell>
                          <TableCell className="font-mono text-xs">{line.shipmentInvoiceNo}</TableCell>
                          <TableCell>
                            <Badge variant="secondary" className="font-medium">
                              {line.expenseTypeName}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right font-mono">
                            {formatPaiseINR(line.amountPaise)}
                          </TableCell>
                          <TableCell className="text-right font-mono text-muted-foreground">
                            <div className="flex items-center justify-end gap-1">
                              {line.isManualGst && (
                                <Badge variant="outline" className="text-[10px] px-1 py-0 border-amber-300 text-amber-700 bg-amber-50 dark:bg-amber-950 dark:text-amber-200">
                                  Manual
                                </Badge>
                              )}
                              <span>{formatPaiseINR(gstPaise)}</span>
                            </div>
                          </TableCell>
                          <TableCell className="text-right font-mono font-medium">
                            <div>
                              {formatPaiseINR(line.totalPaise || (line.amountPaise + gstPaise))}
                            </div>
                            <div className="text-[10px] text-muted-foreground font-normal">
                              (w/o TDS)
                            </div>
                          </TableCell>
                          <TableCell className="text-right font-mono text-amber-500">
                            {line.tdsPaise > 0 ? `-${formatPaiseINR(line.tdsPaise)}` : '—'}
                          </TableCell>
                          <TableCell className="text-right font-mono font-semibold">
                            {formatPaiseINR(line.netPaise)}
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground max-w-xs truncate" title={line.remarks}>
                            {line.remarks || '—'}
                          </TableCell>
                          <TableCell className="text-right">
                            {canWrite && (
                              <Button
                                size="sm"
                                variant="ghost"
                                title="Edit parent invoice"
                                onClick={() => openEditInvoice(line.parentInvoice)}
                              >
                                <Pencil className="h-4 w-4" />
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>

              <div className="flex items-center justify-between text-sm text-muted-foreground pt-4">
                <span>
                  Page {currentItemizedPage} of {totalItemizedPages}
                </span>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={currentItemizedPage <= 1}
                    onClick={() => setItemizedPage((p) => Math.max(1, p - 1))}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={currentItemizedPage >= totalItemizedPages}
                    onClick={() => setItemizedPage((p) => Math.min(totalItemizedPages, p + 1))}
                  >
                    Next
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <ExpenseCatalogAndLedgerTabs
          canWrite={canWrite}
          canDelete={canDelete}
          providers={providers}
          types={types}
          shipments={shipments}
          ledgerRecords={ledgerRecords}
          ledgerLoading={ledgerLoading}
          ledgerShipmentFilter={ledgerShipmentFilter}
          setLedgerShipmentFilter={setLedgerShipmentFilter}
          loadLedger={loadLedger}
          loadAll={loadAll}
          setManageSpOpen={setManageSpOpen}
          setManageEtOpen={setManageEtOpen}
          setTargetLineIdx={setTargetLineIdx}
          spOpen={spOpen}
          setSpOpen={setSpOpen}
          editingSp={editingSp}
          setEditingSp={setEditingSp}
          spForm={spForm}
          setSpForm={setSpForm}
          handleSaveProvider={handleSaveProvider}
          etOpen={etOpen}
          setEtOpen={setEtOpen}
          editingEt={editingEt}
          setEditingEt={setEditingEt}
          etForm={etForm}
          setEtForm={setEtForm}
          handleSaveType={handleSaveType}
        />
      </Tabs>
    </>
  )}

      <ExpenseBulkImportDialog
        open={bulkOpen}
        onOpenChange={setBulkOpen}
        bulkShipmentId={bulkShipmentId}
        setBulkShipmentId={setBulkShipmentId}
        shipments={shipments}
        onCompleted={loadAll}
      />

      <ManageServiceProvidersDialog
        open={manageSpOpen}
        onOpenChange={(open) => {
          setManageSpOpen(open);
          if (!open) setTargetBatchBillKey(null);
        }}
        providers={providers}
        selectedId={
          targetBatchBillKey
            ? batchBills.find((b) => b.key === targetBatchBillKey)?.serviceProviderId
            : serviceProviderId
        }
        onUpdated={loadAll}
        onSelect={(id) => {
          if (targetBatchBillKey) {
            handleBatchProviderChange(targetBatchBillKey, id);
            setTargetBatchBillKey(null);
          } else {
            handleProviderChange(id);
          }
          toast.success('Selected service provider');
        }}
      />

      <ManageExpenseTypesDialog
        open={manageEtOpen}
        onOpenChange={setManageEtOpen}
        types={types}
        selectedId={
          targetLineIdx !== null && draftLines[targetLineIdx]
            ? draftLines[targetLineIdx].expenseTypeId
            : undefined
        }
        onUpdated={loadAll}
        onSelect={(et) => {
          if (targetLineIdx !== null && draftLines[targetLineIdx]) {
            const adapted = adaptExpenseTypeRatesForSupply(et, supplyType);
            updateLine(targetLineIdx, {
              expenseTypeId: et.id,
              cgstPct: (adapted.cgstBp / 100).toFixed(2),
              sgstPct: (adapted.sgstBp / 100).toFixed(2),
              igstPct: (adapted.igstBp / 100).toFixed(2),
              tdsPct: (adapted.tdsBp / 100).toFixed(2),
            });
            toast.success(`Selected expense type "${et.name}"`);
          }
        }}
      />
    </div>
  );
}
