import * as React from 'react';
import { Download, Upload } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/textarea';
import { downloadTextFile } from '@/lib/download';
import type { CSVImportResult } from '@/services/master-service';

interface CsvImportDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  templateFilename: string;
  onDownloadTemplate: () => Promise<string>;
  onImport: (csvContent: string) => Promise<CSVImportResult>;
  onCompleted: () => Promise<void>;
  children?: React.ReactNode;
}

export function CsvImportDialog({
  open,
  onOpenChange,
  title,
  description,
  templateFilename,
  onDownloadTemplate,
  onImport,
  onCompleted,
  children,
}: CsvImportDialogProps) {
  const [csvText, setCsvText] = React.useState('');
  const [submitting, setSubmitting] = React.useState(false);
  const [result, setResult] = React.useState<CSVImportResult | null>(null);
  const fileRef = React.useRef<HTMLInputElement | null>(null);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > 10 * 1024 * 1024) {
      toast.error('CSV file exceeds 10 MB limit');
      return;
    }
    const text = await file.text();
    setCsvText(text);
    setResult(null);
  };

  const handleDownloadTemplate = async () => {
    try {
      const content = await onDownloadTemplate();
      downloadTextFile(templateFilename, content);
      toast.success('CSV template downloaded');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleRunImport = async () => {
    if (!csvText.trim()) {
      toast.error('Select a CSV file or paste CSV rows first');
      return;
    }
    setSubmitting(true);
    try {
      const res = await onImport(csvText);
      setResult(res);
      await onCompleted();
      toast.success(`Imported ${res.importedRows} rows (${res.skippedRows} skipped)`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          {children}
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" variant="outline" size="sm" onClick={() => void handleDownloadTemplate()}>
              <Download className="h-4 w-4" />
              Download Template
            </Button>
            <input
              ref={fileRef}
              type="file"
              accept=".csv,text/csv"
              className="hidden"
              onChange={(e) => void handleFileChange(e)}
            />
            <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()}>
              <Upload className="h-4 w-4" />
              Choose CSV File
            </Button>
          </div>

          <Textarea
            rows={7}
            placeholder="Or paste RFC 4180 CSV content including header row..."
            value={csvText}
            onChange={(e) => setCsvText(e.target.value)}
            className="font-mono text-xs"
          />

          {result && (
            <div className="space-y-2 rounded-md border border-border bg-muted/40 p-3 text-xs">
              <div className="font-medium">
                Total: {result.totalRows} • Imported: {result.importedRows} • Skipped: {result.skippedRows}
              </div>
              {result.errors && result.errors.length > 0 && (
                <div className="max-h-28 space-y-1 overflow-y-auto text-destructive">
                  {result.errors.map((e, i) => (
                    <div key={`${e.row}-${i}`}>
                      Row {e.row}: {e.message}
                    </div>
                  ))}
                </div>
              )}
              {result.warnings && result.warnings.length > 0 && (
                <div className="max-h-24 space-y-1 overflow-y-auto text-amber-700 dark:text-amber-300">
                  {result.warnings.map((w, i) => (
                    <div key={`${w.row}-${i}`}>
                      Warning (Row {w.row}): {w.message}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Close
          </Button>
          <Button type="button" onClick={() => void handleRunImport()} disabled={submitting}>
            {submitting ? 'Importing...' : 'Run Import'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
