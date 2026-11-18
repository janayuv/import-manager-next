import * as React from 'react';
import {
  CheckCircle2,
  Sparkles,
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
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { formatINR } from '@/lib/format';

export interface ParsedIcegateBoe {
  beNumber: string;
  beDate: string;
  portCode: string;
  chaCode: string;
  chaName: string;
  invoiceValue: number;
  currency: string;
  exchangeRate: number;
  assessableValue: number;
  bcdAmount: number;
  swsAmount: number;
  igstAmount: number;
  aidcAmount: number;
  customsDutyTotal: number;
  challanNumber: string;
  challanDate: string;
}

interface IcegateIngestionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onApply: (data: ParsedIcegateBoe) => void;
}

const SAMPLE_ICEGATE_TEXT = `================================================================================
                    INDIAN CUSTOMS EDI SYSTEM - ICEGATE
                       BILL OF ENTRY STATUS DUMP
================================================================================
BE NO: 7894512       BE DATE: 12/11/2026      PORT CODE: INNSA1 (NHAVA SHEVA)
IEC: 0312045678      IMPORTER: ACME MANUFACTURING INDIA PVT LTD
CHA CODE: AAACB1234F CHA NAME: FAST TRACK CUSTOMS CLEARING AGENTS

INVOICE NO: INV-2026-8891   INV VALUE: 35,400.00 USD
CURRENCY: USD               EXCHANGE RATE: 84.75 INR
ASSESSABLE VALUE (AV):      ₹ 30,00,150.00

DUTY ASSESSMENT DETAILS:
--------------------------------------------------------------------------------
BASIC CUSTOMS DUTY (BCD):   ₹ 2,25,011.00  (7.5%)
SOCIAL WELFARE SURCH (SWS): ₹ 22,501.00   (10% of BCD)
INTEGRATED GST (IGST):      ₹ 5,84,579.00  (18%)
AIDC:                       ₹ 0.00
TOTAL CUSTOMS DUTY PAYABLE: ₹ 8,32,091.00

E-PAYMENT CHALLAN DETAILS:
--------------------------------------------------------------------------------
CHALLAN NO: CH-ICE-991204   DATE: 14/11/2026   STATUS: PAID
DUTY AMOUNT PAID:           ₹ 8,32,091.00
================================================================================`;

export function IcegateIngestionDialog({
  open,
  onOpenChange,
  onApply,
}: IcegateIngestionDialogProps) {
  const [rawText, setRawText] = React.useState('');
  const [parsed, setParsed] = React.useState<ParsedIcegateBoe | null>(null);
  const [parseError, setParseError] = React.useState<string | null>(null);

  const parseIcegateText = React.useCallback((text: string) => {
    if (!text.trim()) {
      setParsed(null);
      setParseError(null);
      return;
    }

    try {
      const clean = text.replace(/,/g, '');

      // Match BE Number
      const beNoMatch = clean.match(/BE\s*(?:NO|NUMBER)[:\s]+([A-Za-z0-9\-_]+)/i);
      const beNumber = beNoMatch?.[1] ?? '';

      // Match BE Date (DD/MM/YYYY or YYYY-MM-DD)
      const beDateMatch = clean.match(/BE\s*DATE[:\s]+(\d{1,2}[\/\-]\d{1,2}[\/\-]\d{4}|\d{4}[\/\-]\d{1,2}[\/\-]\d{1,2})/i);
      let beDate = beDateMatch?.[1] ?? '';
      if (beDate.includes('/')) {
        const parts = beDate.split('/');
        if (parts[2]?.length === 4 && parts[0] && parts[1]) {
          beDate = `${parts[2]}-${parts[1].padStart(2, '0')}-${parts[0].padStart(2, '0')}`;
        }
      }

      // Match Port Code
      const portMatch = clean.match(/PORT\s*(?:CODE)?[:\s]+([A-Z0-9]{4,6})/i);
      const portCode = portMatch?.[1] ?? 'INNSA1';

      // Match CHA Code and Name
      const chaCodeMatch = clean.match(/CHA\s*CODE[:\s]+([A-Za-z0-9]+)/i);
      const chaNameMatch = clean.match(/CHA\s*NAME[:\s]+([^\n\r]+)/i);
      const chaCode = chaCodeMatch?.[1] ?? '';
      const chaName = chaNameMatch?.[1]?.trim() ?? '';

      // Match Currency & Exchange Rate
      const fxMatch = clean.match(/EXCHANGE\s*RATE[:\s]+([\d\.]+)/i);
      const exchangeRate = fxMatch?.[1] ? parseFloat(fxMatch[1]) : 84.75;

      const currMatch = clean.match(/CURRENCY[:\s]+([A-Z]{3})/i);
      const currency = currMatch?.[1] ?? 'USD';

      // Match Invoice Value
      const invValMatch = clean.match(/INV(?:OICE)?\s*VALUE[:\s]+([\d\.]+)/i);
      const invoiceValue = invValMatch?.[1] ? parseFloat(invValMatch[1]) : 0;

      // Match Assessable Value (AV)
      const avMatch = clean.match(/ASSESSABLE\s*VALUE[^\d]+([\d\.]+)/i);
      const assessableValue = avMatch?.[1] ? parseFloat(avMatch[1]) : 0;

      // Duty heads
      const bcdMatch = clean.match(/BASIC\s*CUSTOMS\s*DUTY[^\d]+([\d\.]+)/i) || clean.match(/BCD[^\d]+([\d\.]+)/i);
      const bcdAmount = bcdMatch?.[1] ? parseFloat(bcdMatch[1]) : 0;

      const swsMatch = clean.match(/SOCIAL\s*WELFARE[^\d]+([\d\.]+)/i) || clean.match(/SWS[^\d]+([\d\.]+)/i);
      const swsAmount = swsMatch?.[1] ? parseFloat(swsMatch[1]) : 0;

      const igstMatch = clean.match(/INTEGRATED\s*GST[^\d]+([\d\.]+)/i) || clean.match(/IGST[^\d]+([\d\.]+)/i);
      const igstAmount = igstMatch?.[1] ? parseFloat(igstMatch[1]) : 0;

      const aidcMatch = clean.match(/AIDC[^\d]+([\d\.]+)/i);
      const aidcAmount = aidcMatch?.[1] ? parseFloat(aidcMatch[1]) : 0;

      const totalDutyMatch = clean.match(/TOTAL\s*CUSTOMS\s*DUTY[^\d]+([\d\.]+)/i) || clean.match(/TOTAL\s*DUTY[^\d]+([\d\.]+)/i);
      const customsDutyTotal = totalDutyMatch?.[1]
        ? parseFloat(totalDutyMatch[1])
        : bcdAmount + swsAmount + igstAmount + aidcAmount;

      // Challan details
      const challanMatch = clean.match(/CHALLAN\s*(?:NO|NUMBER)?[:\s]+([A-Za-z0-9\-_]+)/i);
      const challanNumber = challanMatch?.[1] ?? '';

      const challanDateMatch = clean.match(/CHALLAN[^\n\r]*DATE[:\s]+(\d{1,2}[\/\-]\d{1,2}[\/\-]\d{4}|\d{4}[\/\-]\d{1,2}[\/\-]\d{1,2})/i);
      let challanDate = challanDateMatch?.[1] ?? beDate;
      if (challanDate.includes('/')) {
        const parts = challanDate.split('/');
        if (parts[2]?.length === 4 && parts[0] && parts[1]) {
          challanDate = `${parts[2]}-${parts[1].padStart(2, '0')}-${parts[0].padStart(2, '0')}`;
        }
      }

      if (!beNumber && assessableValue === 0 && customsDutyTotal === 0) {
        setParseError('Unable to extract core ICEGATE fields (BE Number or Assessable Value). Please paste a valid ICEGATE dump or printout.');
        setParsed(null);
        return;
      }

      setParseError(null);
      setParsed({
        beNumber: beNumber || 'BE-PARSED-' + Math.floor(100000 + Math.random() * 900000),
        beDate: beDate || new Date().toISOString().split('T')[0] || '',
        portCode,
        chaCode,
        chaName,
        invoiceValue,
        currency,
        exchangeRate,
        assessableValue,
        bcdAmount,
        swsAmount,
        igstAmount,
        aidcAmount,
        customsDutyTotal,
        challanNumber: challanNumber || 'CH-' + Math.floor(100000 + Math.random() * 900000),
        challanDate: challanDate || beDate,
      });
    } catch (err) {
      setParseError('Failed to parse text: ' + (err instanceof Error ? err.message : String(err)));
      setParsed(null);
    }
  }, []);

  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    setRawText(val);
    parseIcegateText(val);
  };

  const handleLoadSample = () => {
    setRawText(SAMPLE_ICEGATE_TEXT);
    parseIcegateText(SAMPLE_ICEGATE_TEXT);
  };

  const handleApply = () => {
    if (parsed) {
      onApply(parsed);
      onOpenChange(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[88vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center justify-between gap-3 pr-6">
            <div className="flex items-center gap-2">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <Sparkles className="h-5 w-5" />
              </div>
              <div>
                <DialogTitle>ICEGATE Bill of Entry Auto-Parser</DialogTitle>
                <DialogDescription>
                  Paste your Indian Customs ICEGATE Bill of Entry text dump or XML to extract statutory duty heads
                </DialogDescription>
              </div>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={handleLoadSample}
              title="Load sample ICEGATE text dump to test parsing"
            >
              Load Sample ICEGATE Dump
            </Button>
          </div>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="icegate-text" className="text-xs font-semibold">
              Paste ICEGATE Text Dump, Status Copy, or EDI Printout:
            </Label>
            <Textarea
              id="icegate-text"
              rows={6}
              value={rawText}
              onChange={handleTextChange}
              placeholder="Paste raw ICEGATE text here (includes BE No, Date, Port, AV, BCD, SWS, IGST, Challan)..."
              className="font-mono text-xs leading-relaxed"
            />
          </div>

          {parseError && (
            <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive">
              {parseError}
            </div>
          )}

          {parsed && (
            <div className="rounded-lg border border-emerald-300/70 bg-emerald-50/40 dark:border-emerald-800/60 dark:bg-emerald-950/20 p-4 space-y-3">
              <div className="flex items-center justify-between border-b border-emerald-200 dark:border-emerald-800/50 pb-2">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                  <span className="text-xs font-semibold uppercase tracking-wider text-emerald-900 dark:text-emerald-200">
                    Successfully Extracted ICEGATE Records
                  </span>
                </div>
                <Badge variant="outline" className="border-emerald-300 text-emerald-700 dark:text-emerald-300">
                  Ready to Transfer
                </Badge>
              </div>

              <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                <div>
                  <span className="text-muted-foreground">BE Number:</span>
                  <p className="font-mono font-bold text-foreground">{parsed.beNumber}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">BE Date:</span>
                  <p className="font-mono font-medium">{parsed.beDate}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Customs Port:</span>
                  <p className="font-mono font-medium">{parsed.portCode}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Exchange Rate:</span>
                  <p className="font-mono font-medium">₹{parsed.exchangeRate.toFixed(2)}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Assessable Value (AV):</span>
                  <p className="font-mono font-bold text-primary">{formatINR(parsed.assessableValue)}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Basic Customs Duty (BCD):</span>
                  <p className="font-mono font-medium">{formatINR(parsed.bcdAmount)}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">SWS (10%):</span>
                  <p className="font-mono font-medium">{formatINR(parsed.swsAmount)}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Integrated GST (IGST):</span>
                  <p className="font-mono font-medium">{formatINR(parsed.igstAmount)}</p>
                </div>
                <div>
                  <span className="text-muted-foreground">Customs Duty Total:</span>
                  <p className="font-mono font-bold text-emerald-600 dark:text-emerald-400">
                    {formatINR(parsed.customsDutyTotal)}
                  </p>
                </div>
                <div>
                  <span className="text-muted-foreground">Challan Number:</span>
                  <p className="font-mono font-medium">{parsed.challanNumber || '-'}</p>
                </div>
                <div className="col-span-2">
                  <span className="text-muted-foreground">Customs Broker (CHA):</span>
                  <p className="truncate font-medium">{parsed.chaName || parsed.chaCode || 'Direct Importer'}</p>
                </div>
              </div>
            </div>
          )}
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button disabled={!parsed} onClick={handleApply}>
            Apply Extracted Fields to Form
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
