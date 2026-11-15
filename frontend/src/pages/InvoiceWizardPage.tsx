import * as React from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { AlertTriangle, CheckCircle2, ClipboardPaste, Pencil, Plus, Save, Trash2, X } from 'lucide-react';
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
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { itemService, type Item } from '@/services/master-service';
import {
  invoiceService,
  invoiceWizardService,
  type InvoiceSummary,
  type Shipment,
} from '@/services/shipment-invoice-service';

interface WizardLine {
  key: string;
  itemId: string;
  partNumber: string;
  itemDescription: string;
  unit: string;
  quantity: number;
  unitPrice: number;
  masterUnitPrice: number;
  dutyPercent: number;
  swsPercent: number;
  igstPercent: number;
}

interface StoredDraft {
  id: string;
  shipmentId: string;
  label: string;
  savedAt: number;
  lineTotalDecimals: number;
  invoiceTotalDecimals: number;
  lines: WizardLine[];
}

const DRAFT_STORAGE_KEY = 'im_next_invoice_wizard_drafts_v1';
const MAX_DRAFTS = 20;
const DRAFT_TTL_MS = 30 * 24 * 60 * 60 * 1000;

function roundPrecision(val: number, decimals: number): number {
  if (decimals <= 0) return Math.round(val);
  const f = Math.pow(10, decimals);
  return Math.round(val * f) / f;
}

export function InvoiceWizardPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const editIdParam = searchParams.get('id') || searchParams.get('invoiceId') || '';

  const [step, setStep] = React.useState<1 | 2 | 3>(1);
  const [editingInvoiceId, setEditingInvoiceId] = React.useState<string>(editIdParam);
  const [dbDrafts, setDbDrafts] = React.useState<InvoiceSummary[]>([]);

  const [shipments, setShipments] = React.useState<Shipment[]>([]);
  const [items, setItems] = React.useState<Item[]>([]);
  const [selectedShipmentId, setSelectedShipmentId] = React.useState('');
  const [lineTotalDecimals, setLineTotalDecimals] = React.useState<0 | 2>(2);
  const [invoiceTotalDecimals, setInvoiceTotalDecimals] = React.useState<0 | 2>(2);
  const [freezeOnFinalize, setFreezeOnFinalize] = React.useState(false);

  const [lines, setLines] = React.useState<WizardLine[]>([]);
  const [addItemId, setAddItemId] = React.useState('');
  const [supplierOnly, setSupplierOnly] = React.useState(true);

  const [pasteOpen, setPasteOpen] = React.useState(false);
  const [pasteText, setPasteText] = React.useState('');
  const [saving, setSaving] = React.useState(false);

  const [drafts, setDrafts] = React.useState<StoredDraft[]>(() => {
    try {
      const raw = localStorage.getItem(DRAFT_STORAGE_KEY);
      if (!raw) return [];
      const parsed = JSON.parse(raw) as StoredDraft[];
      const cutoff = Date.now() - DRAFT_TTL_MS;
      return parsed.filter((d) => d.savedAt >= cutoff).slice(0, MAX_DRAFTS);
    } catch {
      return [];
    }
  });

  const loadDraftForEdit = React.useCallback(async (invoiceId: string) => {
    try {
      const detail = await invoiceService.getDetail(invoiceId);
      if (!detail || !detail.invoice) return;
      setEditingInvoiceId(detail.invoice.id);
      setSelectedShipmentId(detail.invoice.shipmentId);
      setLineTotalDecimals(detail.invoice.lineTotalDecimals === 0 ? 0 : 2);
      setInvoiceTotalDecimals(detail.invoice.invoiceTotalDecimals === 0 ? 0 : 2);

      setShipments((prev) => {
        if (prev.some((s) => s.id === detail.invoice.shipmentId)) return prev;
        return [
          {
            id: detail.invoice.shipmentId,
            invoiceNumber: detail.invoice.invoiceNumber,
            invoiceDate: detail.invoice.invoiceDate,
            supplierName: detail.invoice.supplierName,
            supplierId: detail.invoice.supplierId,
            invoiceCurrency: detail.invoice.currency,
            invoiceValue: detail.invoice.shipmentInvoiceValue,
            isFrozen: detail.invoice.shipmentFrozen,
            status: 'in-transit',
          } as Shipment,
          ...prev,
        ];
      });

      const loadedLines: WizardLine[] = (detail.lines ?? []).map((ln) => ({
        key: `${ln.itemId}-${ln.id || Date.now()}`,
        itemId: ln.itemId,
        partNumber: ln.partNumber,
        itemDescription: ln.itemDescription,
        unit: ln.unit,
        quantity: ln.quantity,
        unitPrice: ln.unitPrice,
        masterUnitPrice: ln.masterUnitPrice,
        dutyPercent: ln.dutyPercent,
        swsPercent: ln.swsPercent,
        igstPercent: ln.igstPercent,
      }));
      setLines(loadedLines);
      setStep(2);
      toast.info(`Loaded draft invoice ${detail.invoice.id} for editing`);
    } catch (err) {
      toast.error(`Failed to load draft ${invoiceId}: ${err instanceof Error ? err.message : String(err)}`);
    }
  }, []);

  React.useEffect(() => {
    void (async () => {
      try {
        const [activeShips, catalog, draftList, allInvoices] = await Promise.all([
          invoiceService.listActiveShipments(),
          itemService.list({ search: '', supplierId: '', status: 'active', sortBy: 'partNumber', sortDir: 'asc', page: 1, pageSize: 500 }),
          Promise.resolve(invoiceService.list('', 'Draft')).catch(() => []),
          Promise.resolve(invoiceService.list('', '')).catch(() => []),
        ]);

        const sumList = Array.isArray(allInvoices) ? allInvoices : [];
        const finalizedShipmentIds = new Set(
          sumList
            .filter((inv) => (inv.status || '').toLowerCase() === 'finalized')
            .map((inv) => inv.shipmentId),
        );

        const eligibleShipments = (activeShips ?? []).filter((sh) => {
          if (editIdParam) return true;
          return !finalizedShipmentIds.has(sh.id);
        });

        setShipments(eligibleShipments);
        setItems(catalog?.items ?? []);
        setDbDrafts(Array.isArray(draftList) ? draftList : []);

        if (editIdParam) {
          await loadDraftForEdit(editIdParam);
        } else if (eligibleShipments?.[0] && !selectedShipmentId) {
          setSelectedShipmentId(eligibleShipments[0].id);
        } else if (!eligibleShipments.some((s) => s.id === selectedShipmentId)) {
          setSelectedShipmentId(eligibleShipments[0]?.id ?? '');
        }

        // Item selection is automatically synchronized with available catalog items for the selected shipment.
      } catch (err) {
        toast.error(err instanceof Error ? err.message : String(err));
      }
    })();
  }, [editIdParam, loadDraftForEdit]);

  const handleStartNew = () => {
    setEditingInvoiceId('');
    setLines([]);
    setStep(1);
    setSearchParams({});
    if (shipments[0]) {
      setSelectedShipmentId(shipments[0].id);
    }
    toast.info('Cleared draft; started new invoice');
  };

  const selectedShipment = React.useMemo(
    () => shipments.find((s) => s.id === selectedShipmentId) ?? null,
    [shipments, selectedShipmentId],
  );

  const availableCatalogItems = React.useMemo(() => {
    if (!supplierOnly || !selectedShipment) {
      return items;
    }
    const supId = selectedShipment.supplierId?.trim().toLowerCase();
    const supName = selectedShipment.supplierName?.trim().toLowerCase();
    if (!supId && !supName) {
      return items;
    }
    return items.filter((it) => {
      const itSupId = it.supplierId?.trim().toLowerCase();
      const itSupName = it.supplierName?.trim().toLowerCase();
      if (supId && itSupId && itSupId === supId) return true;
      if (supName && itSupName && (itSupName === supName || itSupName.includes(supName) || supName.includes(itSupName))) return true;
      return false;
    });
  }, [items, selectedShipment, supplierOnly]);

  React.useEffect(() => {
    if (availableCatalogItems.length > 0) {
      if (!addItemId || !availableCatalogItems.some((it) => it.id === addItemId)) {
        setAddItemId(availableCatalogItems[0]?.id ?? '');
      }
    } else {
      setAddItemId('');
    }
  }, [availableCatalogItems, addItemId]);

  const saveLocalDraft = React.useCallback(
    (nextLines: WizardLine[]) => {
      if (!selectedShipmentId || nextLines.length === 0) return;
      const sh = shipments.find((s) => s.id === selectedShipmentId);
      const entry: StoredDraft = {
        id: editingInvoiceId ? `draft-${editingInvoiceId}` : `draft-${selectedShipmentId}`,
        shipmentId: selectedShipmentId,
        label: editingInvoiceId
          ? `${editingInvoiceId} (${sh?.invoiceNumber || selectedShipmentId})`
          : sh
            ? `${sh.invoiceNumber} (${sh.id})`
            : selectedShipmentId,
        savedAt: Date.now(),
        lineTotalDecimals,
        invoiceTotalDecimals,
        lines: nextLines,
      };
      setDrafts((prev) => {
        const filtered = prev.filter((d) => d.id !== entry.id);
        const merged = [entry, ...filtered].slice(0, MAX_DRAFTS);
        localStorage.setItem(DRAFT_STORAGE_KEY, JSON.stringify(merged));
        return merged;
      });
    },
    [selectedShipmentId, shipments, lineTotalDecimals, invoiceTotalDecimals, editingInvoiceId],
  );

  const computedInvoiceTotal = React.useMemo(() => {
    const sum = lines.reduce(
      (acc, ln) => acc + roundPrecision(ln.quantity * ln.unitPrice, lineTotalDecimals),
      0,
    );
    return roundPrecision(sum, invoiceTotalDecimals);
  }, [lines, lineTotalDecimals, invoiceTotalDecimals]);

  const totalMatchesShipment = React.useMemo(() => {
    if (!selectedShipment) return false;
    return Math.round(computedInvoiceTotal * 100) === Math.round(selectedShipment.invoiceValue * 100);
  }, [computedInvoiceTotal, selectedShipment]);

  const handleAddSelectedItem = () => {
    const item = items.find((it) => it.id === addItemId);
    if (!item) return;
    const next: WizardLine = {
      key: `${item.id}-${Date.now()}`,
      itemId: item.id,
      partNumber: item.partNumber,
      itemDescription: item.itemDescription,
      unit: item.unit,
      quantity: 1,
      unitPrice: item.unitPrice,
      masterUnitPrice: item.unitPrice,
      dutyPercent: Number.parseFloat(item.bcd || '0') || 0,
      swsPercent: Number.parseFloat(item.sws || '0') || 0,
      igstPercent: Number.parseFloat(item.igst || '0') || 0,
    };
    const updated = [...lines, next];
    setLines(updated);
    saveLocalDraft(updated);
  };

  const handleMultiLinePaste = async () => {
    if (!pasteText.trim()) return;
    try {
      const parsed = await invoiceWizardService.parseMultiLinePaste(pasteText);
      const valid: WizardLine[] = [];
      const errors: string[] = [];
      for (const p of parsed) {
        if (!p.matched) {
          errors.push(`Row ${p.row}: ${p.error || 'unmatched item'}`);
          continue;
        }
        valid.push({
          key: `${p.itemId}-${p.row}-${Date.now()}`,
          itemId: p.itemId,
          partNumber: p.partNumber,
          itemDescription: p.itemDescription,
          unit: p.unit,
          quantity: p.quantity,
          unitPrice: p.unitPrice,
          masterUnitPrice: p.masterUnitPrice,
          dutyPercent: p.dutyPercent,
          swsPercent: p.swsPercent,
          igstPercent: p.igstPercent,
        });
      }
      const updated = [...lines, ...valid];
      setLines(updated);
      saveLocalDraft(updated);
      setPasteOpen(false);
      setPasteText('');
      if (errors.length > 0) {
        toast.warning(`Added ${valid.length} lines; ${errors.length} skipped (${errors[0]})`);
      } else {
        toast.success(`Added ${valid.length} line items from clipboard`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleSubmit = async (finalize: boolean) => {
    if (!selectedShipmentId) {
      toast.error('Select a shipment first');
      return;
    }
    if (lines.length === 0) {
      toast.error('Add at least one line item');
      return;
    }
    setSaving(true);
    try {
      const saved = await invoiceWizardService.saveInvoice({
        invoiceId: editingInvoiceId || undefined,
        shipmentId: selectedShipmentId,
        finalize,
        freezeShipment: finalize && freezeOnFinalize,
        lineTotalDecimals,
        invoiceTotalDecimals,
        lines: lines.map((l) => ({
          itemId: l.itemId,
          quantity: l.quantity,
          unitPrice: l.unitPrice,
          dutyPercent: l.dutyPercent,
          swsPercent: l.swsPercent,
          igstPercent: l.igstPercent,
        })),
      });
      // Remove completed local draft
      const remaining = drafts.filter(
        (d) => d.shipmentId !== selectedShipmentId && d.id !== `draft-${editingInvoiceId}`,
      );
      setDrafts(remaining);
      localStorage.setItem(DRAFT_STORAGE_KEY, JSON.stringify(remaining));

      toast.success(
        finalize
          ? `Invoice ${saved.invoice.id} finalized successfully`
          : editingInvoiceId
            ? `Invoice ${saved.invoice.id} draft updated`
            : `Invoice ${saved.invoice.id} saved as Draft`,
      );
      navigate('/invoices');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight">Commercial Invoice Wizard</h1>
            {editingInvoiceId && (
              <Badge variant="outline" className="border-amber-500/50 bg-amber-500/10 text-amber-600 dark:text-amber-400">
                Editing Draft: {editingInvoiceId}
              </Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            3-step line-item builder with clipboard multi-line paste and integer-cent finalize gate.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {editingInvoiceId && (
            <Button variant="ghost" size="sm" onClick={handleStartNew} title="Cancel editing and start a new invoice">
              <X className="mr-1 h-3.5 w-3.5" />
              New Invoice
            </Button>
          )}
          <Button variant={step === 1 ? 'default' : 'outline'} size="sm" onClick={() => setStep(1)}>
            1. Shipment &amp; Precision
          </Button>
          <Button
            variant={step === 2 ? 'default' : 'outline'}
            size="sm"
            disabled={!selectedShipment}
            onClick={() => setStep(2)}
          >
            2. Line Items ({lines.length})
          </Button>
          <Button
            variant={step === 3 ? 'default' : 'outline'}
            size="sm"
            disabled={!selectedShipment || lines.length === 0}
            onClick={() => setStep(3)}
          >
            3. Review &amp; Finalize
          </Button>
        </div>
      </div>

      {step === 1 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Step 1: Select Active Shipment &amp; Decimal Precision</CardTitle>
            <CardDescription>
              Only non-frozen, non-terminal shipments are eligible for invoice creation.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 md:grid-cols-3">
              <div className="space-y-1">
                <Label>Target Shipment *</Label>
                <Select value={selectedShipmentId} onValueChange={setSelectedShipmentId}>
                  <SelectTrigger>
                    <SelectValue placeholder={shipments.length === 0 ? 'No pending shipments eligible for invoicing' : 'Choose active shipment'} />
                  </SelectTrigger>
                  <SelectContent>
                    {shipments.map((sh) => (
                      <SelectItem key={sh.id} value={sh.id}>
                        {sh.invoiceNumber} — {sh.supplierName} ({sh.invoiceCurrency} {sh.invoiceValue})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label>Line Total Decimals</Label>
                <Select
                  value={String(lineTotalDecimals)}
                  onValueChange={(v) => setLineTotalDecimals(v === '0' ? 0 : 2)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="2">2 Decimals (0.01)</SelectItem>
                    <SelectItem value="0">0 Decimals (Rounded Integer)</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label>Invoice Total Decimals</Label>
                <Select
                  value={String(invoiceTotalDecimals)}
                  onValueChange={(v) => setInvoiceTotalDecimals(v === '0' ? 0 : 2)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="2">2 Decimals (0.01)</SelectItem>
                    <SelectItem value="0">0 Decimals (Rounded Integer)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {dbDrafts.length > 0 && (
              <div className="space-y-2 rounded-md border border-amber-500/30 bg-amber-500/5 p-3">
                <p className="text-xs font-semibold uppercase tracking-wide text-amber-600 dark:text-amber-400">
                  Saved Database Drafts ({dbDrafts.length}) — Click to Edit &amp; Finalize
                </p>
                <div className="flex flex-wrap gap-2">
                  {dbDrafts.map((d) => (
                    <Button
                      key={d.id}
                      type="button"
                      variant={editingInvoiceId === d.id ? 'default' : 'outline'}
                      size="sm"
                      className={editingInvoiceId === d.id ? '' : 'border-amber-500/40 hover:bg-amber-500/10'}
                      onClick={() => void loadDraftForEdit(d.id)}
                    >
                      <Pencil className="mr-1.5 h-3.5 w-3.5" />
                      {d.id}: {d.invoiceNumber} ({d.lineCount} lines • {d.currency}{' '}
                      {d.computedTotal.toFixed(d.invoiceTotalDecimals)})
                    </Button>
                  ))}
                </div>
              </div>
            )}

            {drafts.length > 0 && (
              <div className="space-y-2 rounded-md border border-border bg-muted/30 p-3">
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Saved Browser Drafts ({drafts.length} / {MAX_DRAFTS})
                </p>
                <div className="flex flex-wrap gap-2">
                  {drafts.map((d) => (
                    <Button
                      key={d.id}
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setSelectedShipmentId(d.shipmentId);
                        setLineTotalDecimals(d.lineTotalDecimals === 0 ? 0 : 2);
                        setInvoiceTotalDecimals(d.invoiceTotalDecimals === 0 ? 0 : 2);
                        setLines(d.lines);
                        setStep(2);
                        toast.info(`Restored draft for ${d.label}`);
                      }}
                    >
                      {d.label} ({d.lines.length} lines)
                    </Button>
                  ))}
                </div>
              </div>
            )}

            <div className="flex justify-end">
              <Button disabled={!selectedShipment} onClick={() => setStep(2)}>
                Continue to Line Items
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 2 && (
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <CardTitle className="text-base">
                  Step 2: Invoice Lines — {selectedShipment?.invoiceNumber}
                  {editingInvoiceId && (
                    <span className="ml-2 text-xs font-normal text-amber-600 dark:text-amber-400">
                      (Editing {editingInvoiceId})
                    </span>
                  )}
                </CardTitle>
                <CardDescription>
                  Target Shipment Total: {selectedShipment?.invoiceCurrency}{' '}
                  {selectedShipment?.invoiceValue.toFixed(2)}
                  {selectedShipment?.supplierName && ` • Supplier: ${selectedShipment.supplierName}`}
                  {' • '}
                  Current Sum: {computedInvoiceTotal.toFixed(invoiceTotalDecimals)}
                </CardDescription>
              </div>
              <Button variant="outline" size="sm" onClick={() => setPasteOpen(true)}>
                <ClipboardPaste className="h-4 w-4" />
                Multi-Line Paste
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap items-end gap-3">
              <div className="w-80 space-y-1">
                <div className="flex items-center justify-between">
                  <Label>Add Catalog Item</Label>
                  {selectedShipment?.supplierName && (
                    <span
                      className="text-[11px] text-muted-foreground truncate max-w-[140px]"
                      title={selectedShipment.supplierName}
                    >
                      {selectedShipment.supplierName}
                    </span>
                  )}
                </div>
                <Select
                  value={addItemId}
                  onValueChange={setAddItemId}
                  disabled={availableCatalogItems.length === 0}
                >
                  <SelectTrigger>
                    <SelectValue
                      placeholder={
                        availableCatalogItems.length === 0
                          ? 'No items for this supplier'
                          : 'Choose item'
                      }
                    />
                  </SelectTrigger>
                  <SelectContent>
                    {availableCatalogItems.map((it) => (
                      <SelectItem key={it.id} value={it.id}>
                        {it.partNumber} — {it.itemDescription} ({it.currency} {it.unitPrice})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button
                type="button"
                onClick={handleAddSelectedItem}
                disabled={!addItemId || availableCatalogItems.length === 0}
              >
                <Plus className="h-4 w-4" />
                Add Line
              </Button>
              <div className="flex items-center space-x-2 pb-2">
                <Switch
                  id="supplier-items-only"
                  checked={supplierOnly}
                  onCheckedChange={setSupplierOnly}
                />
                <Label
                  htmlFor="supplier-items-only"
                  className="text-xs cursor-pointer select-none font-medium text-muted-foreground hover:text-foreground"
                >
                  Supplier items only
                </Label>
              </div>
            </div>
            {supplierOnly && selectedShipment && availableCatalogItems.length === 0 && (
              <p className="text-xs text-amber-500">
                No catalog items found for {selectedShipment.supplierName || 'this supplier'}. Uncheck
                &quot;Supplier items only&quot; to pick from all items or add parts in Item Master.
              </p>
            )}

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Part No</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Qty</TableHead>
                  <TableHead>Unit Price</TableHead>
                  <TableHead>Line Total</TableHead>
                  <TableHead className="text-right">Remove</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {lines.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="text-center text-muted-foreground">
                      Add catalog items above or paste multi-line spreadsheet data.
                    </TableCell>
                  </TableRow>
                ) : (
                  lines.map((ln, idx) => {
                    const diverged =
                      ln.masterUnitPrice > 0 && Math.abs(ln.unitPrice - ln.masterUnitPrice) > 0.001;
                    const lineTotal = roundPrecision(ln.quantity * ln.unitPrice, lineTotalDecimals);
                    return (
                      <TableRow key={ln.key}>
                        <TableCell className="font-mono text-xs">{ln.partNumber}</TableCell>
                        <TableCell>{ln.itemDescription}</TableCell>
                        <TableCell className="w-28">
                          <Input
                            type="number"
                            step="0.001"
                            value={ln.quantity}
                            onChange={(e) => {
                              const val = Number.parseFloat(e.target.value) || 0;
                              const next = lines.map((x, i) => (i === idx ? { ...x, quantity: val } : x));
                              setLines(next);
                              saveLocalDraft(next);
                            }}
                          />
                        </TableCell>
                        <TableCell className="w-40">
                          <Input
                            type="number"
                            step="0.0001"
                            value={ln.unitPrice}
                            onChange={(e) => {
                              const val = Number.parseFloat(e.target.value) || 0;
                              const next = lines.map((x, i) => (i === idx ? { ...x, unitPrice: val } : x));
                              setLines(next);
                              saveLocalDraft(next);
                            }}
                          />
                          {diverged && (
                            <Badge variant="warning" className="mt-1">
                              Master: {ln.masterUnitPrice.toFixed(2)}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {lineTotal.toFixed(lineTotalDecimals)}
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => setLines((prev) => prev.filter((_, i) => i !== idx))}
                          >
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>

            <div className="flex justify-between">
              <Button variant="outline" onClick={() => setStep(1)}>
                Back
              </Button>
              <Button disabled={lines.length === 0} onClick={() => setStep(3)}>
                Review &amp; Save
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 3 && selectedShipment && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">
              Step 3: Total Reconciliation &amp; Finalization
              {editingInvoiceId && (
                <span className="ml-2 text-xs font-normal text-amber-600 dark:text-amber-400">
                  (Editing {editingInvoiceId})
                </span>
              )}
            </CardTitle>
            <CardDescription>
              Finalizing requires exact integer-cent agreement with shipment.invoice_value; Drafts bypass
              this check and transition shipment to in-transit.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 md:grid-cols-3">
              <div className="rounded-md border border-border p-3">
                <div className="text-xs text-muted-foreground">Shipment Invoice Value</div>
                <div className="text-lg font-bold">
                  {selectedShipment.invoiceCurrency} {selectedShipment.invoiceValue.toFixed(2)}
                </div>
              </div>
              <div className="rounded-md border border-border p-3">
                <div className="text-xs text-muted-foreground">Computed Line Sum</div>
                <div className="text-lg font-bold">
                  {selectedShipment.invoiceCurrency} {computedInvoiceTotal.toFixed(invoiceTotalDecimals)}
                </div>
              </div>
              <div className="rounded-md border border-border p-3">
                <div className="text-xs text-muted-foreground">Reconciliation Gate</div>
                <div className="mt-1 flex items-center gap-1.5 text-sm font-medium">
                  {totalMatchesShipment ? (
                    <>
                      <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                      <span className="text-emerald-700">Exact Cent Match</span>
                    </>
                  ) : (
                    <>
                      <AlertTriangle className="h-4 w-4 text-amber-600" />
                      <span className="text-amber-700">
                        Delta {(computedInvoiceTotal - selectedShipment.invoiceValue).toFixed(2)} (Draft Only)
                      </span>
                    </>
                  )}
                </div>
              </div>
            </div>

            <div className="flex items-center justify-between rounded-md border border-border px-4 py-3">
              <div>
                <Label htmlFor="freeze-after-finalize">Freeze Shipment Upon Finalization</Label>
                <p className="text-xs text-muted-foreground">
                  Applies a commercial lock to shipment {selectedShipment.id} immediately after Finalize.
                </p>
              </div>
              <Switch
                id="freeze-after-finalize"
                checked={freezeOnFinalize}
                onCheckedChange={setFreezeOnFinalize}
              />
            </div>

            <div className="flex flex-wrap justify-between gap-2">
              <Button variant="outline" onClick={() => setStep(2)}>
                Back to Lines
              </Button>
              <div className="flex gap-2">
                <Button variant="outline" disabled={saving} onClick={() => void handleSubmit(false)}>
                  <Save className="h-4 w-4" />
                  {editingInvoiceId ? 'Update Draft' : 'Save Draft'}
                </Button>
                <Button
                  disabled={saving || !totalMatchesShipment}
                  onClick={() => void handleSubmit(true)}
                >
                  <CheckCircle2 className="h-4 w-4" />
                  Finalize Invoice
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <Dialog open={pasteOpen} onOpenChange={setPasteOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>Multi-Line Clipboard Paste</DialogTitle>
            <DialogDescription>
              Paste rows separated by tab, comma, semicolon, or pipe (Format A: PartNo, Qty, Price or
              Format B: PartNo, Desc, Unit, Qty, Price).
            </DialogDescription>
          </DialogHeader>
          <Textarea
            rows={7}
            className="font-mono text-xs"
            placeholder={'OR-100\t4\t105.00\nPN-200\t10\t18.75'}
            value={pasteText}
            onChange={(e) => setPasteText(e.target.value)}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasteOpen(false)}>
              Cancel
            </Button>
            <Button onClick={() => void handleMultiLinePaste()}>Parse &amp; Append</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
