import * as React from 'react';
import {
  AlertCircle,
  Calculator,
  CheckCircle2,
  FileText,
  FileUp,
  Filter,
  Landmark,
  Plus,
  RefreshCw,
  Search,
  ShieldAlert,
  Sparkles,
  Trash2,
  Upload,
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
import { Textarea } from '@/components/ui/textarea';
import {
  cbicRatesService,
  type CBICExchangeRate,
  type CBICExchangeRateInput,
} from '@/services/cbic-rates-service';
import {
  extractTextFromPdf,
  parseCBICNotificationText,
  type ExtractedCurrencyRate,
} from '@/lib/cbic-parser';
import { useAuth } from '@/contexts/AuthContext';
import { useModuleSettings } from '@/contexts/ThemeContext';

const TOP_CURRENCIES = ['USD', 'EUR', 'CNY', 'KRW', 'JPY', 'GBP', 'AED', 'SGD'];

export function CBICRatesPage() {
  const { hasPermission } = useAuth();
  const canManage = hasPermission('data.write');

  const { itemsPerPage: pageSizeSetting } = useModuleSettings('cbicRates');
  const pageSize = pageSizeSetting > 0 ? pageSizeSetting : 10;
  const [currentPage, setCurrentPage] = React.useState(1);

  const [rates, setRates] = React.useState<CBICExchangeRate[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [currencyFilter, setCurrencyFilter] = React.useState<string>('all');
  const [search, setSearch] = React.useState<string>('');

  // Add Single Dialog State
  const [addOpen, setAddOpen] = React.useState(false);
  const [submitting, setSubmitting] = React.useState(false);
  const [formCurrency, setFormCurrency] = React.useState('USD');
  const [formRate, setFormRate] = React.useState('84.50');
  const [formEffectiveFrom, setFormEffectiveFrom] = React.useState(
    new Date().toISOString().slice(0, 10),
  );
  const [formEffectiveTo, setFormEffectiveTo] = React.useState('');
  const [formNotif, setFormNotif] = React.useState('');
  const [formSourceDoc, setFormSourceDoc] = React.useState('');
  const [formStatus, setFormStatus] = React.useState('Verified');

  // PDF Upload / Ingestion Dialog State
  const [uploadOpen, setUploadOpen] = React.useState(false);
  const [uploadParsing, setUploadParsing] = React.useState(false);
  const [uploadSubmitting, setUploadSubmitting] = React.useState(false);
  const [previewRates, setPreviewRates] = React.useState<ExtractedCurrencyRate[]>([]);
  const [uploadNotifNumber, setUploadNotifNumber] = React.useState('');
  const [uploadEffectiveFrom, setUploadEffectiveFrom] = React.useState('');
  const [uploadStatus, setUploadStatus] = React.useState('Verified');
  const [uploadRawText, setUploadRawText] = React.useState('');
  const [uploadTab, setUploadTab] = React.useState<'rates' | 'text'>('rates');
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);

  // Lookup Tool State
  const [lookupCurrency, setLookupCurrency] = React.useState('USD');
  const [lookupDate, setLookupDate] = React.useState(new Date().toISOString().slice(0, 10));
  const [lookupResult, setLookupResult] = React.useState<CBICExchangeRate | null>(null);
  const [lookupLoading, setLookupLoading] = React.useState(false);

  // Delete State
  const [deleteTarget, setDeleteTarget] = React.useState<CBICExchangeRate | null>(null);

  const loadRates = React.useCallback(async () => {
    setLoading(true);
    try {
      const list = await cbicRatesService.list(currencyFilter === 'all' ? '' : currencyFilter);
      setRates(list ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [currencyFilter]);

  React.useEffect(() => {
    void loadRates();
  }, [loadRates]);

  React.useEffect(() => {
    setCurrentPage(1);
  }, [currencyFilter, search, pageSize]);

  const handleLookup = async () => {
    setLookupLoading(true);
    try {
      const rate = await cbicRatesService.getEffectiveRate(lookupCurrency, lookupDate);
      setLookupResult(rate);
      toast.success(
        `Resolved statutory rate for ${lookupCurrency} on ${lookupDate}: ₹${rate.exchangeRate < 1 ? rate.exchangeRate.toFixed(4) : rate.exchangeRate.toFixed(2)}`,
      );
    } catch (err) {
      setLookupResult(null);
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLookupLoading(false);
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    const curr = formCurrency.trim().toUpperCase();
    const rateVal = parseFloat(formRate);
    if (!curr || isNaN(rateVal) || rateVal <= 0) {
      toast.error('Valid currency code and positive exchange rate are required');
      return;
    }
    if (!formEffectiveFrom) {
      toast.error('Effective from date is required');
      return;
    }
    setSubmitting(true);
    try {
      const payload: CBICExchangeRateInput = {
        currencyCode: curr,
        exchangeRate: rateVal,
        effectiveFrom: formEffectiveFrom,
        effectiveTo: formEffectiveTo.trim() || undefined,
        notificationNumber: formNotif.trim() || `CBIC/${new Date().getFullYear()}`,
        sourceDocument: formSourceDoc.trim() || 'Gazette of India',
        verificationStatus: formStatus.toLowerCase() === 'pending' ? 'Pending' : 'Verified',
      };
      await cbicRatesService.create(payload);
      toast.success(`CBIC exchange rate notification for ${curr} saved successfully`);
      setAddOpen(false);
      await loadRates();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploadParsing(true);
    try {
      let text = '';
      if (file.name.toLowerCase().endsWith('.pdf') || file.type === 'application/pdf') {
        const buffer = await file.arrayBuffer();
        text = await extractTextFromPdf(buffer);
      } else {
        text = await file.text();
      }

      processExtractedText(text);
      setUploadTab('rates');
      setUploadOpen(true);
    } catch (err) {
      toast.error(`Failed to parse notification document: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setUploadParsing(false);
      if (e.target) e.target.value = '';
    }
  };

  const processExtractedText = (text: string) => {
    const parsed = parseCBICNotificationText(text);
    setPreviewRates(parsed.rates);
    setUploadNotifNumber(parsed.notificationNumber);
    setUploadEffectiveFrom(parsed.effectiveFrom);
    setUploadRawText(text);
    setUploadTab('rates');

    if (parsed.warnings && parsed.warnings.length > 0) {
      toast.warning(parsed.warnings[0]);
    } else {
      toast.success(
        `Extracted ${parsed.rates.length} currencies from ${parsed.notificationNumber} (w.e.f ${parsed.effectiveFrom})`,
      );
    }
  };

  const handleUpdatePreviewRate = (
    index: number,
    field: 'currencyUnit' | 'rateImport' | 'rateExport',
    val: number,
  ) => {
    setPreviewRates((prev) => {
      const copy = [...prev];
      const item = { ...copy[index]! };
      item[field] = val;
      const unit = field === 'currencyUnit' ? val : item.currencyUnit;
      const rateImport = field === 'rateImport' ? val : item.rateImport;
      item.effectiveRate = unit > 0 ? Number((rateImport / unit).toFixed(6)) : rateImport;
      copy[index] = item;
      return copy;
    });
  };

  const handleRemovePreviewRate = (index: number) => {
    setPreviewRates((prev) => prev.filter((_, i) => i !== index));
  };

  const handleBatchIngest = async () => {
    if (previewRates.length === 0) {
      toast.error('No currency rates to ingest');
      return;
    }
    if (!uploadNotifNumber.trim()) {
      toast.error('Notification number is required');
      return;
    }
    if (!uploadEffectiveFrom) {
      toast.error('Effective date is required');
      return;
    }

    setUploadSubmitting(true);
    try {
      const payloads: CBICExchangeRateInput[] = previewRates.map((r) => ({
        currencyCode: r.currencyCode,
        exchangeRate: r.effectiveRate > 0 ? r.effectiveRate : r.rateImport,
        effectiveFrom: uploadEffectiveFrom,
        notificationNumber: uploadNotifNumber.trim(),
        sourceDocument: `CBIC Gazette (Unit: ${r.currencyUnit}, Import: ₹${r.rateImport}, Export: ₹${r.rateExport})`,
        verificationStatus: uploadStatus.toLowerCase() === 'pending' ? 'Pending' : 'Verified',
        notes: `Statutory customs rate from ${uploadNotifNumber} w.e.f ${uploadEffectiveFrom}`,
      }));

      const res = await cbicRatesService.createBatch(payloads);
      if (res.errors.length > 0) {
        toast.warning(
          `Ingested ${res.created} rates; ${res.errors.length} failed (${res.errors[0]})`,
        );
      } else {
        toast.success(
          `Successfully ingested all ${res.created} statutory exchange rates for ${uploadNotifNumber}!`,
        );
      }
      setUploadOpen(false);
      await loadRates();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setUploadSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await cbicRatesService.delete(deleteTarget.id);
      toast.success(
        `CBIC rate entry for ${deleteTarget.currencyCode} (${deleteTarget.effectiveFrom}) removed`,
      );
      setDeleteTarget(null);
      await loadRates();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const filteredRates = React.useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return rates;
    return rates.filter(
      (r) =>
        r.currencyCode.toLowerCase().includes(q) ||
        r.notificationNumber.toLowerCase().includes(q) ||
        (r.sourceDocument || '').toLowerCase().includes(q) ||
        r.effectiveFrom.includes(q),
    );
  }, [rates, search]);

  const totalPages = Math.max(1, Math.ceil(filteredRates.length / pageSize));
  const safePage = Math.min(currentPage, totalPages);
  const pagedRates = React.useMemo(() => {
    const start = (safePage - 1) * pageSize;
    return filteredRates.slice(start, start + pageSize);
  }, [filteredRates, safePage, pageSize]);

  return (
    <div className="space-y-6">
      {/* Hidden PDF/Text File Input */}
      <input
        ref={fileInputRef}
        type="file"
        accept=".pdf,.txt"
        className="hidden"
        onChange={(e) => void handleFileChange(e)}
      />

      {/* Page Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <Landmark className="h-6 w-6 text-primary" />
            <h1 className="text-2xl font-bold tracking-tight">CBIC Customs Exchange Rate Master</h1>
            <Badge variant="outline" className="text-xs">
              v1.7.0 Statutory Master
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            Official exchange rates notified under Section 14 of the Customs Act, 1962. Automatically
            applied to BOE Duty Calculations.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => void loadRates()} disabled={loading}>
            <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>

          {canManage && (
            <>
              <Button
                variant="outline"
                size="sm"
                className="border-primary/40 text-primary hover:bg-primary/10"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploadParsing}
              >
                <FileUp className={`h-4 w-4 mr-1.5 ${uploadParsing ? 'animate-spin' : ''}`} />
                {uploadParsing ? 'Parsing PDF...' : 'Upload CBIC Notification PDF'}
              </Button>

              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setPreviewRates([]);
                  setUploadNotifNumber('');
                  setUploadEffectiveFrom(new Date().toISOString().slice(0, 10));
                  setUploadRawText('');
                  setUploadTab('text');
                  setUploadOpen(true);
                }}
              >
                <FileText className="h-4 w-4 mr-1" />
                Paste / Ingest Text
              </Button>

              <Button size="sm" onClick={() => setAddOpen(true)}>
                <Plus className="h-4 w-4 mr-1" />
                Add CBIC Notification
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Safety & Immutability Notice Banner */}
      <div className="rounded-lg border border-primary/20 bg-primary/5 p-4 text-xs space-y-1">
        <div className="flex items-center gap-2 font-semibold text-primary">
          <ShieldAlert className="h-4 w-4" />
          <span>Statutory Customs Rate Compliance &amp; Historical Snapshot Invariant</span>
        </div>
        <p className="text-muted-foreground">
          Assessed Bills of Entry snapshot and preserve the exact exchange rate applied during customs duty calculation.
          Updating or adding newer CBIC gazette notifications will <strong>never alter historical assessed BOEs</strong>.
        </p>
      </div>

      {/* Quick Rate Lookup Tool */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <Calculator className="h-4 w-4 text-primary" />
            Statutory Effective Rate Resolver
          </CardTitle>
          <CardDescription>
            Simulate or look up the legally applicable customs exchange rate for any currency on a specific BOE filing / valuation date.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-end gap-3">
            <div className="w-36">
              <Label className="text-xs">Currency</Label>
              <Select value={lookupCurrency} onValueChange={setLookupCurrency}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TOP_CURRENCIES.map((c) => (
                    <SelectItem key={c} value={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="w-44">
              <Label className="text-xs">Valuation Date</Label>
              <Input
                type="date"
                className="h-8 text-xs"
                value={lookupDate}
                onChange={(e) => setLookupDate(e.target.value)}
              />
            </div>
            <Button
              size="sm"
              variant="secondary"
              className="h-8 text-xs"
              onClick={() => void handleLookup()}
              disabled={lookupLoading}
            >
              Resolve Statutory Rate
            </Button>
            {lookupResult && (
              <div className="flex items-center gap-2 rounded-md border border-emerald-500/30 bg-emerald-500/10 px-3 py-1.5 text-xs text-emerald-800 dark:text-emerald-300">
                <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600" />
                <span>
                  <strong>1 {lookupResult.currencyCode} = ₹{lookupResult.exchangeRate < 1 ? lookupResult.exchangeRate.toFixed(4) : lookupResult.exchangeRate.toFixed(2)}</strong> (Notif: {lookupResult.notificationNumber}, Effective from: {lookupResult.effectiveFrom})
                </span>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Filter and Search Bar */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Filter className="h-4 w-4 text-muted-foreground" />
          <Select value={currencyFilter} onValueChange={setCurrencyFilter}>
            <SelectTrigger className="h-8 w-36 text-xs">
              <SelectValue placeholder="All Currencies" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Currencies</SelectItem>
              {TOP_CURRENCIES.map((c) => (
                <SelectItem key={c} value={c}>
                  {c}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="relative w-64">
          <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Search notification, currency or source..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-8 pl-8 text-xs"
          />
        </div>
      </div>

      {/* CBIC Exchange Rates Table */}
      <div className="rounded-md border border-border">
        <Table>
          <TableHeader>
            <TableRow className="text-xs">
              <TableHead>Currency</TableHead>
              <TableHead>Statutory Rate (INR)</TableHead>
              <TableHead>Effective From</TableHead>
              <TableHead>Effective To</TableHead>
              <TableHead>CBIC Notification Ref</TableHead>
              <TableHead>Source Document</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center py-6 text-xs text-muted-foreground">
                  Loading CBIC exchange rates...
                </TableCell>
              </TableRow>
            ) : filteredRates.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center py-6 text-xs text-muted-foreground">
                  No CBIC customs exchange rate notifications found.
                </TableCell>
              </TableRow>
            ) : (
              pagedRates.map((r) => (
                <TableRow key={r.id}>
                  <TableCell className="font-mono font-semibold text-xs">
                    <Badge variant="outline" className="font-mono">
                      {r.currencyCode}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono font-medium text-xs text-primary">
                    ₹{r.exchangeRate < 1 ? r.exchangeRate.toFixed(4) : r.exchangeRate.toFixed(2)}
                  </TableCell>
                  <TableCell className="text-xs">{r.effectiveFrom}</TableCell>
                  <TableCell className="text-xs">
                    {r.effectiveTo ? (
                      r.effectiveTo
                    ) : (
                      <Badge variant="secondary" className="text-[10px] bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                        Current Active
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-xs font-mono">{r.notificationNumber}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{r.sourceDocument}</TableCell>
                  <TableCell className="text-xs">
                    <Badge
                      variant={r.verificationStatus?.toLowerCase() === 'verified' ? 'success' : 'warning'}
                      className="text-[10px]"
                    >
                      {r.verificationStatus}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    {canManage && (
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                        onClick={() => setDeleteTarget(r)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination Controls */}
      <div className="flex items-center justify-between text-xs text-muted-foreground pt-2">
        <span>
          Showing {pagedRates.length} of {filteredRates.length} exchange rates (Page {safePage} of {totalPages})
        </span>
        <div className="flex gap-1">
          <Button
            variant="outline"
            size="sm"
            disabled={safePage <= 1}
            onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
          >
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={safePage >= totalPages}
            onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
          >
            Next
          </Button>
        </div>
      </div>

      {/* PDF Upload & Batch Ingestion Dialog */}
      <Dialog open={uploadOpen} onOpenChange={setUploadOpen}>
        <DialogContent className="max-w-4xl max-h-[90vh] flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FileUp className="h-5 w-5 text-primary" />
              CBIC Statutory Exchange Rate Notification Ingestion
            </DialogTitle>
            <DialogDescription>
              Upload official CBIC exchange rate gazette PDF or paste notification text. Review extracted currencies and ingest in one click.
            </DialogDescription>
          </DialogHeader>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-3 py-2 border-b border-border">
            <div>
              <Label className="text-xs font-semibold">Notification Number *</Label>
              <Input
                placeholder="e.g. Notification No. 28/2026"
                className="h-8 text-xs font-mono mt-1"
                value={uploadNotifNumber}
                onChange={(e) => setUploadNotifNumber(e.target.value)}
              />
            </div>
            <div>
              <Label className="text-xs font-semibold">Effective From (w.e.f) *</Label>
              <Input
                type="date"
                className="h-8 text-xs mt-1"
                value={uploadEffectiveFrom}
                onChange={(e) => setUploadEffectiveFrom(e.target.value)}
              />
            </div>
            <div>
              <Label className="text-xs font-semibold">Verification Status</Label>
              <Select value={uploadStatus} onValueChange={setUploadStatus}>
                <SelectTrigger className="h-8 text-xs mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Verified">VERIFIED (Gazette Confirmed)</SelectItem>
                  <SelectItem value="Pending">PENDING (Draft)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <Tabs
            value={uploadTab}
            onValueChange={(v) => setUploadTab(v as 'rates' | 'text')}
            className="flex-1 flex flex-col min-h-0"
          >
            <div className="flex items-center justify-between pt-2">
              <TabsList className="h-8">
                <TabsTrigger value="rates" className="text-xs">
                  Extracted Currencies ({previewRates.length})
                </TabsTrigger>
                <TabsTrigger value="text" className="text-xs">
                  Raw Document Text / Paste
                </TabsTrigger>
              </TabsList>

              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => fileInputRef.current?.click()}
                >
                  <Upload className="h-3.5 w-3.5 mr-1" />
                  Select Different File
                </Button>
              </div>
            </div>

            <TabsContent value="rates" className="flex-1 overflow-auto mt-2 border rounded-md">
              {previewRates.length === 0 ? (
                <div className="p-8 text-center text-muted-foreground text-xs space-y-2">
                  <AlertCircle className="h-8 w-8 mx-auto text-amber-500" />
                  <p>No currencies detected yet. Upload a CBIC PDF notification or paste document text in the raw text tab.</p>
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow className="text-[11px]">
                      <TableHead className="w-16">Code</TableHead>
                      <TableHead>Currency Name</TableHead>
                      <TableHead className="w-24">Unit</TableHead>
                      <TableHead className="w-28">Rate Import (INR)</TableHead>
                      <TableHead className="w-28">Rate Export (INR)</TableHead>
                      <TableHead className="w-32">Statutory Duty Rate</TableHead>
                      <TableHead className="w-12"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {previewRates.map((r, idx) => (
                      <TableRow key={r.currencyCode} className="text-xs">
                        <TableCell className="font-mono font-bold">{r.currencyCode}</TableCell>
                        <TableCell className="text-muted-foreground">{r.currencyName}</TableCell>
                        <TableCell>
                          <Input
                            type="number"
                            step="1"
                            min="1"
                            className="h-7 text-xs font-mono w-20"
                            value={r.currencyUnit}
                            onChange={(e) =>
                              handleUpdatePreviewRate(idx, 'currencyUnit', parseFloat(e.target.value) || 1)
                            }
                          />
                        </TableCell>
                        <TableCell>
                          <Input
                            type="number"
                            step="0.0001"
                            className="h-7 text-xs font-mono w-24"
                            value={r.rateImport}
                            onChange={(e) =>
                              handleUpdatePreviewRate(idx, 'rateImport', parseFloat(e.target.value) || 0)
                            }
                          />
                        </TableCell>
                        <TableCell>
                          <Input
                            type="number"
                            step="0.0001"
                            className="h-7 text-xs font-mono w-24"
                            value={r.rateExport}
                            onChange={(e) =>
                              handleUpdatePreviewRate(idx, 'rateExport', parseFloat(e.target.value) || 0)
                            }
                          />
                        </TableCell>
                        <TableCell className="font-mono font-semibold text-primary">
                          ₹{r.effectiveRate.toFixed(4)}
                          {r.currencyUnit > 1 && (
                            <span className="text-[10px] text-muted-foreground block font-normal">
                              ({r.rateImport} / {r.currencyUnit})
                            </span>
                          )}
                        </TableCell>
                        <TableCell>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                            onClick={() => handleRemovePreviewRate(idx)}
                            title="Remove row"
                          >
                            <Trash2 className="h-3 w-3" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </TabsContent>

            <TabsContent value="text" className="flex-1 flex flex-col gap-2 mt-2">
              <Textarea
                rows={10}
                placeholder="Paste CBIC Exchange Rate Notification text here..."
                className="font-mono text-xs flex-1 min-h-[220px]"
                value={uploadRawText}
                onChange={(e) => setUploadRawText(e.target.value)}
              />
              <div className="flex justify-between items-center">
                <span className="text-[11px] text-muted-foreground">
                  Paste content copied from gazette PDF, ICEGATE notification, or official release.
                </span>
                <Button
                  size="sm"
                  variant="secondary"
                  className="h-7 text-xs"
                  onClick={() => processExtractedText(uploadRawText)}
                >
                  <Sparkles className="h-3.5 w-3.5 mr-1 text-primary" />
                  Parse Text
                </Button>
              </div>
            </TabsContent>
          </Tabs>

          <DialogFooter className="pt-3 border-t border-border flex items-center justify-between">
            <span className="text-xs text-muted-foreground">
              {previewRates.length > 0
                ? `Ready to ingest ${previewRates.length} statutory rates into CBIC Master`
                : 'Upload document or paste text to proceed'}
            </span>
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={() => setUploadOpen(false)}>
                Cancel
              </Button>
              <Button
                type="button"
                disabled={previewRates.length === 0 || uploadSubmitting}
                onClick={() => void handleBatchIngest()}
              >
                <FileUp className="h-4 w-4 mr-1.5" />
                {uploadSubmitting ? 'Ingesting Rates...' : `Ingest All ${previewRates.length} Currencies`}
              </Button>
            </div>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add Single Rate Notification Dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="max-w-md">
          <form onSubmit={handleCreate}>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Landmark className="h-5 w-5 text-primary" />
                Add CBIC Rate Notification
              </DialogTitle>
              <DialogDescription>
                Register an individual customs exchange rate notification issued under Section 14.
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3 py-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label htmlFor="formCurr" className="text-xs">Currency Code *</Label>
                  <Input
                    id="formCurr"
                    placeholder="e.g. USD, EUR, CNY"
                    className="h-8 text-xs font-mono uppercase"
                    value={formCurrency}
                    onChange={(e) => setFormCurrency(e.target.value)}
                    required
                  />
                </div>
                <div>
                  <Label htmlFor="formRate" className="text-xs">Customs Rate (INR) *</Label>
                  <Input
                    id="formRate"
                    type="number"
                    step="0.0001"
                    min="0.0001"
                    placeholder="84.50"
                    className="h-8 text-xs font-mono"
                    value={formRate}
                    onChange={(e) => setFormRate(e.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label htmlFor="formEffFrom" className="text-xs">Effective From *</Label>
                  <Input
                    id="formEffFrom"
                    type="date"
                    className="h-8 text-xs"
                    value={formEffectiveFrom}
                    onChange={(e) => setFormEffectiveFrom(e.target.value)}
                    required
                  />
                </div>
                <div>
                  <Label htmlFor="formEffTo" className="text-xs">Effective To (Optional)</Label>
                  <Input
                    id="formEffTo"
                    type="date"
                    className="h-8 text-xs"
                    value={formEffectiveTo}
                    onChange={(e) => setFormEffectiveTo(e.target.value)}
                  />
                </div>
              </div>

              <div>
                <Label htmlFor="formNotif" className="text-xs">CBIC Notification Number *</Label>
                <Input
                  id="formNotif"
                  placeholder="e.g. No. 12/2026-Customs (N.T.)"
                  className="h-8 text-xs font-mono"
                  value={formNotif}
                  onChange={(e) => setFormNotif(e.target.value)}
                  required
                />
              </div>

              <div>
                <Label htmlFor="formSource" className="text-xs">Source Document / Gazette Ref</Label>
                <Input
                  id="formSource"
                  placeholder="e.g. Gazette of India Notification Extra Ordinary"
                  className="h-8 text-xs"
                  value={formSourceDoc}
                  onChange={(e) => setFormSourceDoc(e.target.value)}
                />
              </div>

              <div>
                <Label htmlFor="formStatus" className="text-xs">Verification Status</Label>
                <Select value={formStatus} onValueChange={setFormStatus}>
                  <SelectTrigger id="formStatus" className="h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="Verified">VERIFIED (Gazette Confirmed)</SelectItem>
                    <SelectItem value="Pending">PENDING (Draft Notification)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting ? 'Saving...' : 'Save Notification'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={Boolean(deleteTarget)} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Delete CBIC Exchange Rate?</DialogTitle>
            <DialogDescription>
              Are you sure you want to remove the rate notification for <strong>{deleteTarget?.currencyCode}</strong> effective {deleteTarget?.effectiveFrom} ({deleteTarget?.notificationNumber})?
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={() => void handleDelete()}>
              Delete Notification
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
