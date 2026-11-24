import * as React from 'react';
import {
  AlertCircle,
  CheckCircle2,
  Download,
  FileSpreadsheet,
  Info,
  RefreshCw,
  Upload,
} from 'lucide-react';
import Papa from 'papaparse';
import { toast } from 'sonner';
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
import { formatINR } from '@/lib/format';
import { rodtepService } from '@/services/rodtep-service';
import type { RoDTEPScripInput } from '@/types/rodtep';

interface RoDTEPImportModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  existingScripNumbers: Set<string>;
  onSuccess: () => void;
}

interface ParsedScripRow {
  scripNumber: string;
  portCode: 'INMAA1' | 'INMAA4';
  issueDate: string;
  expiryDate: string;
  originalValueInr: number;
  openingBalanceInr: number;
  preErpUtilizedInr: number;
  remarks: string;
  evidenceRef: string;
  status: 'valid' | 'duplicate' | 'invalid';
  errorMessage?: string;
}

// ---------------------------------------------------------------------------
// Helpers for robust parsing
// ---------------------------------------------------------------------------

function parseDateCell(val: any): string | null {
  if (val === null || val === undefined || val === '') return null;

  if (val instanceof Date && !isNaN(val.getTime())) {
    const y = val.getFullYear();
    const m = String(val.getMonth() + 1).padStart(2, '0');
    const d = String(val.getDate()).padStart(2, '0');
    return `${y}-${m}-${d}`;
  }

  if (typeof val === 'number') {
    // Excel serial date to JS Date (base: Dec 30 1899)
    if (val > 10000 && val < 90000) {
      const utcDays = Math.floor(val - 25569);
      const date = new Date(utcDays * 86400 * 1000);
      if (!isNaN(date.getTime())) {
        const y = date.getUTCFullYear();
        const m = String(date.getUTCMonth() + 1).padStart(2, '0');
        const d = String(date.getUTCDate()).padStart(2, '0');
        return `${y}-${m}-${d}`;
      }
    }
  }

  if (typeof val === 'object' && val !== null) {
    if (val.text) return parseDateCell(val.text);
    if (val.result) return parseDateCell(val.result);
  }

  const str = String(val).trim();
  if (!str) return null;

  // YYYY-MM-DD
  const isoMatch = str.match(/^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})/);
  if (isoMatch && isoMatch[1] && isoMatch[2] && isoMatch[3]) {
    const y = isoMatch[1];
    const m = isoMatch[2].padStart(2, '0');
    const d = isoMatch[3].padStart(2, '0');
    return `${y}-${m}-${d}`;
  }

  // DD-MMM-YYYY or DD MMM YYYY (e.g. 15-Apr-2025, 23-April-2026)
  const monthMap: Record<string, string> = {
    jan: '01', feb: '02', mar: '03', apr: '04', may: '05', jun: '06',
    jul: '07', aug: '08', sep: '09', oct: '10', nov: '11', dec: '12',
  };
  const dmyTextMatch = str.match(/^(\d{1,2})[-/\s]([A-Za-z]{3,9})[-/\s](\d{2,4})/);
  if (dmyTextMatch && dmyTextMatch[1] && dmyTextMatch[2] && dmyTextMatch[3]) {
    const d = dmyTextMatch[1].padStart(2, '0');
    const mStr = dmyTextMatch[2].toLowerCase().substring(0, 3);
    let y = dmyTextMatch[3];
    if (y.length === 2) y = '20' + y;
    const m = monthMap[mStr];
    if (m) return `${y}-${m}-${d}`;
  }

  // DD/MM/YYYY or DD-MM-YYYY
  const dmyMatch = str.match(/^(\d{1,2})[-/.](\d{1,2})[-/.](\d{2,4})/);
  if (dmyMatch && dmyMatch[1] && dmyMatch[2] && dmyMatch[3]) {
    const d = dmyMatch[1].padStart(2, '0');
    const m = dmyMatch[2].padStart(2, '0');
    let y = dmyMatch[3];
    if (y.length === 2) y = '20' + y;
    return `${y}-${m}-${d}`;
  }

  // Standard Date parsing fallback
  const fallback = new Date(str);
  if (!isNaN(fallback.getTime())) {
    const y = fallback.getFullYear();
    const m = String(fallback.getMonth() + 1).padStart(2, '0');
    const d = String(fallback.getDate()).padStart(2, '0');
    return `${y}-${m}-${d}`;
  }

  return null;
}

function computeStatutoryExpiry(issueDateStr: string): string {
  try {
    const parts = issueDateStr.split('-').map(Number);
    const y = parts[0];
    const m = parts[1];
    const d = parts[2];
    if (y === undefined || m === undefined || d === undefined || isNaN(y) || isNaN(m) || isNaN(d)) {
      return '';
    }
    const date = new Date(y, m - 1, d);
    date.setFullYear(date.getFullYear() + 2);
    const expY = date.getFullYear();
    const expM = String(date.getMonth() + 1).padStart(2, '0');
    const expD = String(date.getDate()).padStart(2, '0');
    return `${expY}-${expM}-${expD}`;
  } catch {
    return '';
  }
}

function parseAmount(val: any): number {
  if (val === null || val === undefined || val === '') return 0;
  if (typeof val === 'number') return isNaN(val) ? 0 : val;
  if (typeof val === 'object') {
    if (val.result !== undefined) return parseAmount(val.result);
    if (val.text !== undefined) return parseAmount(val.text);
  }
  const clean = String(val).replace(/[^\d.-]/g, '');
  const num = parseFloat(clean);
  return isNaN(num) ? 0 : num;
}

function normalizePort(val: any, defaultPort: 'INMAA1' | 'INMAA4'): 'INMAA1' | 'INMAA4' {
  const str = String(val || '').trim().toUpperCase();
  if (str.includes('INMAA4') || str.includes('MAA4') || str.includes('AIR')) {
    return 'INMAA4';
  }
  if (str.includes('INMAA1') || str.includes('MAA1') || str.includes('SEA')) {
    return 'INMAA1';
  }
  return defaultPort;
}

// ---------------------------------------------------------------------------
// Main Component
// ---------------------------------------------------------------------------

export function RoDTEPImportModal({
  open,
  onOpenChange,
  existingScripNumbers,
  onSuccess,
}: RoDTEPImportModalProps) {
  const [file, setFile] = React.useState<File | null>(null);
  const [parsing, setParsing] = React.useState(false);
  const [submitting, setSubmitting] = React.useState(false);
  const [defaultPort, setDefaultPort] = React.useState<'INMAA1' | 'INMAA4'>('INMAA1');
  const [acquisitionType, setAcquisitionType] = React.useState<'OPENING_BALANCE' | 'DIRECT_GENERATION'>('OPENING_BALANCE');
  const [parsedRows, setParsedRows] = React.useState<ParsedScripRow[]>([]);
  const [filterView, setFilterView] = React.useState<'all' | 'valid' | 'duplicate' | 'invalid'>('all');
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);

  // Reset when dialog closes
  React.useEffect(() => {
    if (!open) {
      setFile(null);
      setParsedRows([]);
      setParsing(false);
      setSubmitting(false);
      setFilterView('all');
    }
  }, [open]);

  // Handle File Parse
  const processFile = async (selectedFile: File) => {
    setFile(selectedFile);
    setParsing(true);
    setParsedRows([]);

    try {
      const fileName = selectedFile.name.toLowerCase();
      let rows: ParsedScripRow[] = [];

      if (fileName.endsWith('.csv')) {
        rows = await parseCsvFile(selectedFile, defaultPort, existingScripNumbers);
      } else if (fileName.endsWith('.xlsx') || fileName.endsWith('.xls')) {
        rows = await parseExcelFile(selectedFile, defaultPort, existingScripNumbers);
      } else {
        toast.error('Unsupported file format. Please upload a .xlsx, .xls, or .csv file.');
        setFile(null);
        return;
      }

      setParsedRows(rows);
      if (rows.length === 0) {
        toast.warning('No scrip records found in the uploaded file. Please check table headers.');
      } else {
        const validCount = rows.filter((r) => r.status === 'valid').length;
        const dupCount = rows.filter((r) => r.status === 'duplicate').length;
        toast.info(`Parsed ${rows.length} rows (${validCount} ready to import, ${dupCount} duplicates/existing).`);
      }
    } catch (err: any) {
      console.error('File parsing error:', err);
      toast.error('Failed to parse file: ' + (err?.message || err));
      setParsedRows([]);
    } finally {
      setParsing(false);
    }
  };

  // CSV Parser
  const parseCsvFile = async (
    csvFile: File,
    fallbackPort: 'INMAA1' | 'INMAA4',
    existingSet: Set<string>,
  ): Promise<ParsedScripRow[]> => {
    return new Promise((resolve, reject) => {
      Papa.parse(csvFile, {
        header: true,
        skipEmptyLines: 'greedy',
        complete: (results) => {
          try {
            const data = results.data as Record<string, any>[];
            const rows: ParsedScripRow[] = [];

            data.forEach((row, idx) => {
              // Find keys
              const keys = Object.keys(row);
              if (keys.length === 0) return;

              const scripKey = keys.find((k) => /scrip.*(?:no|num|id)|license|script/i.test(k) || /^scrip$/i.test(k));
              const portKey = keys.find((k) => /port|location|customs/i.test(k));
              const issueDateKey = keys.find((k) => /issue.*date|date.*issue/i.test(k) || /^issue$/i.test(k));
              const expDateKey = keys.find((k) => /exp.*date|valid.*upto|expiry/i.test(k));
              const origValKey = keys.find((k) => /issued.*amount|original.*(?:value|amount)|face.*value|total.*amount/i.test(k));
              const unutilKey = keys.find((k) => /unutilized|balance|opening/i.test(k));
              const utilKey = keys.find((k) => /utilized.*amount/i.test(k) && !/unutilized|balance/i.test(k));
              const remarksKey = keys.find((k) => /remark|note|comment|source|evidence/i.test(k));

              const rawScripNum = scripKey ? String(row[scripKey] ?? '').trim() : '';
              if (!rawScripNum || rawScripNum.toLowerCase().includes('total')) {
                return; // Skip empty or summary rows
              }

              const port = portKey ? normalizePort(row[portKey], fallbackPort) : fallbackPort;
              const issueDate = issueDateKey ? parseDateCell(row[issueDateKey]) : null;
              let expiryDate = expDateKey ? parseDateCell(row[expDateKey]) : null;
              if (!expiryDate && issueDate) {
                expiryDate = computeStatutoryExpiry(issueDate);
              }

              const origInr = origValKey ? parseAmount(row[origValKey]) : 0;
              const openInr = unutilKey ? parseAmount(row[unutilKey]) : origInr;
              const preUtilInr = utilKey ? parseAmount(row[utilKey]) : Math.max(0, origInr - openInr);
              const remarks = remarksKey ? String(row[remarksKey] || '').trim() : 'CSV Batch Ingestion';

              // Validation
              let status: 'valid' | 'duplicate' | 'invalid' = 'valid';
              let errorMessage: string | undefined;

              if (existingSet.has(rawScripNum)) {
                status = 'duplicate';
                errorMessage = 'Already exists in database';
              } else if (!issueDate) {
                status = 'invalid';
                errorMessage = 'Missing or invalid issue date';
              } else if (origInr <= 0) {
                status = 'invalid';
                errorMessage = 'Original value must be greater than zero';
              } else if (openInr < 0 || openInr > origInr) {
                status = 'invalid';
                errorMessage = 'Opening balance must be between 0 and original value';
              }

              rows.push({
                scripNumber: rawScripNum,
                portCode: port,
                issueDate: issueDate || '',
                expiryDate: expiryDate || '',
                originalValueInr: origInr,
                openingBalanceInr: openInr,
                preErpUtilizedInr: preUtilInr,
                remarks,
                evidenceRef: `${csvFile.name} (Row ${idx + 2})`,
                status,
                errorMessage,
              });
            });

            resolve(rows);
          } catch (e) {
            reject(e);
          }
        },
        error: (err) => reject(err),
      });
    });
  };

  // Excel Parser (.xlsx, .xls)
  const parseExcelFile = async (
    excelFile: File,
    fallbackPort: 'INMAA1' | 'INMAA4',
    existingSet: Set<string>,
  ): Promise<ParsedScripRow[]> => {
    const { default: ExcelJS } = await import('exceljs');
    const workbook = new ExcelJS.Workbook();
    const arrayBuffer = await excelFile.arrayBuffer();
    await workbook.xlsx.load(arrayBuffer);

    const rows: ParsedScripRow[] = [];

    workbook.eachSheet((worksheet) => {
      const sheetName = worksheet.name || '';
      // Determine sheet port hint (e.g. 'INMAA1', 'INMAA4', 'Chennai Sea', 'Chennai Air')
      let sheetPort = fallbackPort;
      if (sheetName.toUpperCase().includes('INMAA4') || sheetName.toUpperCase().includes('AIR')) {
        sheetPort = 'INMAA4';
      } else if (sheetName.toUpperCase().includes('INMAA1') || sheetName.toUpperCase().includes('SEA')) {
        sheetPort = 'INMAA1';
      }

      // Step 1: Detect header row
      let headerRowIndex = -1;
      const colMap: {
        scrip?: number;
        port?: number;
        issueDate?: number;
        expDate?: number;
        origVal?: number;
        unutilVal?: number;
        utilVal?: number;
        remarks?: number;
      } = {};

      worksheet.eachRow((row, rowNumber) => {
        if (headerRowIndex !== -1) return;

        const cells: { col: number; text: string }[] = [];
        row.eachCell((cell, colNumber) => {
          const t = String(cell.text || cell.value || '').trim().toLowerCase();
          if (t) cells.push({ col: colNumber, text: t });
        });

        if (cells.length >= 3) {
          const hasScrip = cells.some((c) => /scrip|license|script/i.test(c.text));
          const hasDetail = cells.some((c) => /date|amount|value|balance|port|no/i.test(c.text));

          if (hasScrip && hasDetail) {
            headerRowIndex = rowNumber;
            // Map columns
            cells.forEach((c) => {
              if (/scrip.*(?:no|num|id)|license|script/i.test(c.text) || /^scrip$/i.test(c.text)) {
                colMap.scrip = c.col;
              } else if (/port|location|customs/i.test(c.text)) {
                colMap.port = c.col;
              } else if (/issue.*date|date.*issue/i.test(c.text) || /^issue$/i.test(c.text)) {
                colMap.issueDate = c.col;
              } else if (/exp.*date|valid.*upto|expiry/i.test(c.text)) {
                colMap.expDate = c.col;
              } else if (/issued.*amount|original.*(?:value|amount)|face.*value|total.*amount/i.test(c.text)) {
                colMap.origVal = c.col;
              } else if (/unutilized|balance|opening/i.test(c.text)) {
                colMap.unutilVal = c.col;
              } else if (/utilized.*amount/i.test(c.text) && !/unutilized|balance/i.test(c.text)) {
                colMap.utilVal = c.col;
              } else if (/remark|note|comment|source|evidence/i.test(c.text)) {
                colMap.remarks = c.col;
              }
            });
          }
        }
      });

      if (headerRowIndex === -1) {
        // Fallback: If no header found, skip sheet
        return;
      }

      // Step 2: Read data rows
      worksheet.eachRow((row, rowNumber) => {
        if (rowNumber <= headerRowIndex) return;

        const scripCell = colMap.scrip ? row.getCell(colMap.scrip) : row.getCell(1);
        const rawScripNum = String(scripCell.text || scripCell.value || '').trim();
        if (!rawScripNum || rawScripNum.toLowerCase().includes('total')) {
          return; // Skip empty or summary rows
        }

        const portCell = colMap.port ? row.getCell(colMap.port) : null;
        const port = portCell ? normalizePort(portCell.value, sheetPort) : sheetPort;

        const issueDateCell = colMap.issueDate ? row.getCell(colMap.issueDate) : null;
        const issueDate = issueDateCell ? parseDateCell(issueDateCell.value) : null;

        const expDateCell = colMap.expDate ? row.getCell(colMap.expDate) : null;
        let expiryDate = expDateCell ? parseDateCell(expDateCell.value) : null;
        if (!expiryDate && issueDate) {
          expiryDate = computeStatutoryExpiry(issueDate);
        }

        const origCell = colMap.origVal ? row.getCell(colMap.origVal) : null;
        const origInr = origCell ? parseAmount(origCell.value) : 0;

        const unutilCell = colMap.unutilVal ? row.getCell(colMap.unutilVal) : null;
        const openInr = unutilCell ? parseAmount(unutilCell.value) : origInr;

        const utilCell = colMap.utilVal ? row.getCell(colMap.utilVal) : null;
        const preUtilInr = utilCell ? parseAmount(utilCell.value) : Math.max(0, origInr - openInr);

        const remarksCell = colMap.remarks ? row.getCell(colMap.remarks) : null;
        const remarks = remarksCell ? String(remarksCell.text || remarksCell.value || '').trim() : `Opening balance (${sheetName})`;

        // Validation
        let status: 'valid' | 'duplicate' | 'invalid' = 'valid';
        let errorMessage: string | undefined;

        if (existingSet.has(rawScripNum)) {
          status = 'duplicate';
          errorMessage = 'Already exists in database';
        } else if (!issueDate) {
          status = 'invalid';
          errorMessage = 'Missing or invalid issue date';
        } else if (origInr <= 0) {
          status = 'invalid';
          errorMessage = 'Original value must be greater than zero';
        } else if (openInr < 0 || openInr > origInr) {
          status = 'invalid';
          errorMessage = 'Opening balance must be between 0 and original value';
        }

        rows.push({
          scripNumber: rawScripNum,
          portCode: port,
          issueDate: issueDate || '',
          expiryDate: expiryDate || '',
          originalValueInr: origInr,
          openingBalanceInr: openInr,
          preErpUtilizedInr: preUtilInr,
          remarks,
          evidenceRef: `${excelFile.name} (${sheetName})`,
          status,
          errorMessage,
        });
      });
    });

    return rows;
  };

  // Submit valid scrips to backend
  const handleConfirmImport = async () => {
    const validScrips = parsedRows.filter((r) => r.status === 'valid');
    if (validScrips.length === 0) {
      toast.error('No valid scrips available to import.');
      return;
    }

    setSubmitting(true);
    try {
      const inputs: RoDTEPScripInput[] = validScrips.map((r) => ({
        scripNumber: r.scripNumber,
        scripPortCode: r.portCode,
        acquisitionType,
        issueDate: r.issueDate,
        expiryDate: r.expiryDate || undefined,
        originalValuePaise: Math.round(r.originalValueInr * 100),
        openingBalancePaise: Math.round(r.openingBalanceInr * 100),
        preErpUtilizedPaise: Math.round(r.preErpUtilizedInr * 100),
        openingEvidenceRef: r.evidenceRef,
        remarks: r.remarks,
      }));

      const count = await rodtepService.ingestInitialScrips(inputs, 'CustomsAdministrator');
      const skipped = validScrips.length - count;

      if (count > 0) {
        toast.success(`Successfully imported ${count} RoDTEP scrips!${skipped > 0 ? ` (${skipped} duplicates skipped)` : ''}`);
        onSuccess();
        onOpenChange(false);
      } else {
        toast.info('All submitted scrips already exist in the database.');
      }
    } catch (err: any) {
      toast.error('Failed to import scrips: ' + (err?.message || err));
    } finally {
      setSubmitting(false);
    }
  };

  // Template Downloads
  const downloadExcelTemplate = async () => {
    try {
      const { default: ExcelJS } = await import('exceljs');
      const wb = new ExcelJS.Workbook();
      wb.creator = 'Import Manager ERP';
      wb.created = new Date();

      const sheets = [
        { name: 'INMAA1 (Chennai Sea)', port: 'INMAA1' },
        { name: 'INMAA4 (Chennai Air)', port: 'INMAA4' },
      ];

      sheets.forEach(({ name, port }) => {
        const ws = wb.addWorksheet(name);
        ws.columns = [
          { header: 'Scrip No.', key: 'scripNumber', width: 16 },
          { header: 'Port Code', key: 'portCode', width: 12 },
          { header: 'Scrip Issue Date', key: 'issueDate', width: 16 },
          { header: 'Scrip Exp Date', key: 'expiryDate', width: 16 },
          { header: 'Scrip Issued Amount (₹)', key: 'originalValue', width: 22 },
          { header: 'Utilized Amount (₹)', key: 'utilized', width: 18 },
          { header: 'Unutilized / Balance Amount (₹)', key: 'balance', width: 26 },
          { header: 'Remarks', key: 'remarks', width: 30 },
        ];

        // Header style
        ws.getRow(1).font = { bold: true, color: { argb: 'FFFFFFFF' } };
        ws.getRow(1).fill = {
          type: 'pattern',
          pattern: 'solid',
          fgColor: { argb: port === 'INMAA1' ? 'FF1E3A8A' : 'FF065F46' },
        };

        // Sample rows
        if (port === 'INMAA1') {
          ws.addRow({
            scripNumber: '2621004567',
            portCode: 'INMAA1',
            issueDate: '2026-05-10',
            expiryDate: '2028-05-10',
            originalValue: 50000.0,
            utilized: 0.0,
            balance: 50000.0,
            remarks: 'Chennai Sea Direct Generation',
          });
          ws.addRow({
            scripNumber: '2621004568',
            portCode: 'INMAA1',
            issueDate: '2026-05-15',
            expiryDate: '2028-05-15',
            originalValue: 125000.0,
            utilized: 25000.0,
            balance: 100000.0,
            remarks: 'Partially utilized scrip',
          });
        } else {
          ws.addRow({
            scripNumber: '2624009876',
            portCode: 'INMAA4',
            issueDate: '2026-06-15',
            expiryDate: '2028-06-15',
            originalValue: 75000.0,
            utilized: 0.0,
            balance: 75000.0,
            remarks: 'Chennai Air Opening Balance',
          });
        }
      });

      const buffer = await wb.xlsx.writeBuffer();
      const blob = new Blob([buffer], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'RoDTEP_Scrips_Import_Template.xlsx';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast.success('Downloaded Excel template.');
    } catch (err: any) {
      toast.error('Failed to generate template: ' + (err?.message || err));
    }
  };

  const downloadCsvTemplate = () => {
    const csvContent =
      'Scrip No.,Port Code,Scrip Issue Date,Scrip Exp Date,Scrip Issued Amount (₹),Utilized Amount (₹),Unutilized / Balance Amount (₹),Remarks\n' +
      '2621004567,INMAA1,2026-05-10,2028-05-10,50000.00,0.00,50000.00,Chennai Sea Port reference\n' +
      '2624009876,INMAA4,2026-06-15,2028-06-15,75000.00,0.00,75000.00,Chennai Air Cargo reference\n';

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'RoDTEP_Scrips_Import_Template.csv';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast.success('Downloaded CSV template.');
  };

  // Metrics
  const totalCount = parsedRows.length;
  const validCount = parsedRows.filter((r) => r.status === 'valid').length;
  const dupCount = parsedRows.filter((r) => r.status === 'duplicate').length;
  const invalidCount = parsedRows.filter((r) => r.status === 'invalid').length;
  const readyTotalInr = parsedRows
    .filter((r) => r.status === 'valid')
    .reduce((acc, r) => acc + r.openingBalanceInr, 0);

  const displayedRows = parsedRows.filter((r) => {
    if (filterView === 'all') return true;
    return r.status === filterView;
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] flex flex-col p-6 overflow-hidden">
        <DialogHeader className="shrink-0 pb-2">
          <div className="flex items-center justify-between">
            <div>
              <DialogTitle className="text-xl font-bold flex items-center gap-2">
                <FileSpreadsheet className="h-5 w-5 text-emerald-500" />
                Import RoDTEP Scrips (Excel / CSV)
              </DialogTitle>
              <DialogDescription className="text-xs text-muted-foreground mt-0.5">
                Batch ingest RoDTEP duty credit scrips from DGFT, ICEGATE, or custom spreadsheets.
              </DialogDescription>
            </div>
            <div className="flex items-center gap-1.5">
              <Button
                variant="outline"
                size="sm"
                onClick={downloadExcelTemplate}
                className="text-xs h-8 gap-1.5"
                title="Download formatted sample Excel sheet"
              >
                <Download className="h-3.5 w-3.5" />
                Excel Template
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={downloadCsvTemplate}
                className="text-xs h-8 gap-1.5"
                title="Download sample CSV file"
              >
                <Download className="h-3.5 w-3.5" />
                CSV Template
              </Button>
            </div>
          </div>
        </DialogHeader>

        {/* Configuration Row */}
        <div className="shrink-0 grid grid-cols-1 md:grid-cols-2 gap-3 py-2 border-y border-border/60 bg-muted/20 px-3 rounded-md">
          <div className="space-y-1">
            <Label className="text-xs font-medium">Default Port (when unspecified in file)</Label>
            <Select
              value={defaultPort}
              onValueChange={(val) => setDefaultPort(val as 'INMAA1' | 'INMAA4')}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="INMAA1">Chennai Sea Port (INMAA1)</SelectItem>
                <SelectItem value="INMAA4">Chennai Air Cargo (INMAA4)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <Label className="text-xs font-medium">Default Acquisition Type</Label>
            <Select
              value={acquisitionType}
              onValueChange={(val) => setAcquisitionType(val as 'OPENING_BALANCE' | 'DIRECT_GENERATION')}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="OPENING_BALANCE">Opening Balance (Legacy / Historical Reference)</SelectItem>
                <SelectItem value="DIRECT_GENERATION">Direct Generation (DGFT Export Ingestion)</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        {/* Upload Drop Zone */}
        <div className="shrink-0 py-3">
          <input
            type="file"
            ref={fileInputRef}
            className="hidden"
            accept=".xlsx, .xls, .csv"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) processFile(f);
            }}
          />

          <div
            onClick={() => fileInputRef.current?.click()}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              const f = e.dataTransfer.files?.[0];
              if (f) processFile(f);
            }}
            className="border-2 border-dashed border-border/80 hover:border-emerald-500/80 bg-muted/10 hover:bg-emerald-500/5 transition-colors rounded-lg p-4 text-center cursor-pointer flex flex-col items-center justify-center gap-2"
          >
            {parsing ? (
              <>
                <RefreshCw className="h-6 w-6 text-emerald-500 animate-spin" />
                <p className="text-xs font-medium">Reading and validating spreadsheet data...</p>
              </>
            ) : file ? (
              <div className="flex items-center gap-3">
                <FileSpreadsheet className="h-8 w-8 text-emerald-500" />
                <div className="text-left">
                  <p className="text-sm font-semibold text-foreground">{file.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {(file.size / 1024).toFixed(1)} KB &bull; Click to choose another file
                  </p>
                </div>
              </div>
            ) : (
              <>
                <Upload className="h-6 w-6 text-muted-foreground" />
                <div>
                  <p className="text-xs font-medium text-foreground">
                    Click to browse or drag & drop your Excel (.xlsx, .xls) or CSV file
                  </p>
                  <p className="text-[11px] text-muted-foreground mt-0.5">
                    Supports Annexure multi-sheet workbooks (e.g., INMAA1 / INMAA4) and ICEGATE export formats
                  </p>
                </div>
              </>
            )}
          </div>
        </div>

        {/* Parsed Summary KPIs */}
        {totalCount > 0 && (
          <div className="shrink-0 flex items-center justify-between bg-card border border-border rounded-lg p-2.5 mb-2">
            <div className="flex items-center gap-2">
              <Badge variant="outline" className="text-xs font-normal">
                Total Rows: <strong className="ml-1 text-foreground">{totalCount}</strong>
              </Badge>
              <Badge
                variant="outline"
                className="text-xs font-normal text-emerald-600 border-emerald-500/40 bg-emerald-500/10"
              >
                Ready: <strong className="ml-1">{validCount}</strong>
              </Badge>
              {dupCount > 0 && (
                <Badge
                  variant="outline"
                  className="text-xs font-normal text-amber-600 border-amber-500/40 bg-amber-500/10"
                >
                  Duplicates (Skip): <strong className="ml-1">{dupCount}</strong>
                </Badge>
              )}
              {invalidCount > 0 && (
                <Badge
                  variant="outline"
                  className="text-xs font-normal text-destructive border-destructive/40 bg-destructive/10"
                >
                  Invalid: <strong className="ml-1">{invalidCount}</strong>
                </Badge>
              )}
            </div>

            <div className="flex items-center gap-3">
              <span className="text-xs text-muted-foreground">
                Net Import Value:{' '}
                <strong className="text-emerald-500 font-mono text-sm">{formatINR(readyTotalInr)}</strong>
              </span>

              {/* View filter */}
              <div className="flex items-center gap-1 border border-border/80 rounded-md p-0.5 text-[11px]">
                <button
                  type="button"
                  onClick={() => setFilterView('all')}
                  className={`px-2 py-0.5 rounded ${filterView === 'all' ? 'bg-primary text-primary-foreground font-medium' : 'text-muted-foreground hover:text-foreground'}`}
                >
                  All ({totalCount})
                </button>
                <button
                  type="button"
                  onClick={() => setFilterView('valid')}
                  className={`px-2 py-0.5 rounded ${filterView === 'valid' ? 'bg-emerald-600 text-white font-medium' : 'text-muted-foreground hover:text-foreground'}`}
                >
                  Ready ({validCount})
                </button>
                {dupCount > 0 && (
                  <button
                    type="button"
                    onClick={() => setFilterView('duplicate')}
                    className={`px-2 py-0.5 rounded ${filterView === 'duplicate' ? 'bg-amber-600 text-white font-medium' : 'text-muted-foreground hover:text-foreground'}`}
                  >
                    Duplicates ({dupCount})
                  </button>
                )}
                {invalidCount > 0 && (
                  <button
                    type="button"
                    onClick={() => setFilterView('invalid')}
                    className={`px-2 py-0.5 rounded ${filterView === 'invalid' ? 'bg-destructive text-white font-medium' : 'text-muted-foreground hover:text-foreground'}`}
                  >
                    Errors ({invalidCount})
                  </button>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Preview Table */}
        <div className="flex-1 overflow-y-auto border border-border rounded-lg min-h-[220px]">
          {parsedRows.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center p-8 text-center text-muted-foreground">
              <FileSpreadsheet className="h-10 w-10 text-muted-foreground/40 mb-2" />
              <p className="text-sm font-medium">No scrip data loaded yet</p>
              <p className="text-xs text-muted-foreground max-w-sm mt-1">
                Upload a file or download our template above to preview scrip numbers, port codes, dates, and amounts before importing.
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader className="bg-muted/50 sticky top-0 z-10">
                <TableRow>
                  <TableHead className="text-xs py-2">Scrip Number</TableHead>
                  <TableHead className="text-xs py-2">Port</TableHead>
                  <TableHead className="text-xs py-2">Issue Date</TableHead>
                  <TableHead className="text-xs py-2">Expiry Date</TableHead>
                  <TableHead className="text-xs py-2 text-right">Original Value</TableHead>
                  <TableHead className="text-xs py-2 text-right">Opening Balance</TableHead>
                  <TableHead className="text-xs py-2 text-right">Pre-ERP Utilized</TableHead>
                  <TableHead className="text-xs py-2">Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {displayedRows.map((r, i) => (
                  <TableRow key={i} className="text-xs">
                    <TableCell className="font-mono font-medium">{r.scripNumber}</TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-[10px] font-mono">
                        {r.portCode}
                      </Badge>
                    </TableCell>
                    <TableCell>{r.issueDate || '—'}</TableCell>
                    <TableCell className="text-muted-foreground">{r.expiryDate || '—'}</TableCell>
                    <TableCell className="text-right font-mono">{formatINR(r.originalValueInr)}</TableCell>
                    <TableCell className="text-right font-mono font-semibold text-emerald-500">
                      {formatINR(r.openingBalanceInr)}
                    </TableCell>
                    <TableCell className="text-right font-mono text-muted-foreground">
                      {formatINR(r.preErpUtilizedInr)}
                    </TableCell>
                    <TableCell>
                      {r.status === 'valid' && (
                        <Badge variant="outline" className="text-[10px] text-emerald-600 border-emerald-500/40 bg-emerald-500/10">
                          <CheckCircle2 className="h-3 w-3 mr-1" />
                          Ready
                        </Badge>
                      )}
                      {r.status === 'duplicate' && (
                        <Badge variant="outline" className="text-[10px] text-amber-600 border-amber-500/40 bg-amber-500/10" title={r.errorMessage}>
                          <Info className="h-3 w-3 mr-1" />
                          Duplicate
                        </Badge>
                      )}
                      {r.status === 'invalid' && (
                        <Badge variant="outline" className="text-[10px] text-destructive border-destructive/40 bg-destructive/10" title={r.errorMessage}>
                          <AlertCircle className="h-3 w-3 mr-1" />
                          {r.errorMessage || 'Invalid'}
                        </Badge>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>

        <DialogFooter className="shrink-0 pt-3 border-t border-border flex items-center justify-between sm:justify-between">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onOpenChange(false)}
            disabled={submitting}
            className="text-xs"
          >
            Cancel
          </Button>

          <Button
            size="sm"
            onClick={handleConfirmImport}
            disabled={validCount === 0 || submitting}
            className="text-xs gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white"
          >
            {submitting ? (
              <>
                <RefreshCw className="h-4 w-4 animate-spin" />
                Importing...
              </>
            ) : (
              <>
                <Upload className="h-4 w-4" />
                Confirm Import ({validCount} Scrip{validCount === 1 ? '' : 's'})
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
