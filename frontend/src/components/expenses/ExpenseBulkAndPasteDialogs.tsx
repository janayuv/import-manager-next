import { toast } from 'sonner';
import { CsvImportDialog } from '@/components/master/CsvImportDialog';
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
import { Textarea } from '@/components/ui/textarea';
import { expenseService } from '@/services/expense-service';
import type { Shipment } from '@/services/shipment-invoice-service';

interface PasteLinesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  pasteText: string;
  setPasteText: (text: string) => void;
  onApplyPaste: () => void;
}

export function ExpensePasteLinesDialog({
  open,
  onOpenChange,
  pasteText,
  setPasteText,
  onApplyPaste,
}: PasteLinesDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Paste Expense Lines from Excel / Tally</DialogTitle>
          <DialogDescription>
            Paste tab- or comma-separated lines. Format: <span className="font-mono">Expense Type, Basic Amount, [Remarks]</span> or <span className="font-mono">Expense Type, Basic, CGST%, SGST%, IGST%, TDS%, [Remarks]</span>.
          </DialogDescription>
        </DialogHeader>
        <Textarea
          rows={7}
          className="font-mono text-xs"
          placeholder={"Customs Clearance\t15000\tBOE processing\nTransport Charges\t6250\tLocal cartage"}
          value={pasteText}
          onChange={(e) => setPasteText(e.target.value)}
        />
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={onApplyPaste} disabled={!pasteText.trim()}>
            Import Pasted Lines
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

interface BulkExpenseImportDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bulkShipmentId: string;
  setBulkShipmentId: (id: string) => void;
  shipments: Shipment[];
  onCompleted: () => Promise<void>;
}

export function ExpenseBulkImportDialog({
  open,
  onOpenChange,
  bulkShipmentId,
  setBulkShipmentId,
  shipments,
  onCompleted,
}: BulkExpenseImportDialogProps) {
  return (
    <CsvImportDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Bulk Expense CSV Import"
      description="Import expenses across multiple shipments via CSV or select a default target shipment."
      templateFilename="expenses_bulk_template.csv"
      onDownloadTemplate={() => expenseService.downloadBulkTemplateCsv()}
      onImport={(content) => {
        if (bulkShipmentId && bulkShipmentId !== '__MULTIPLE__') {
          const selectedShipment = shipments.find((s) => s.id === bulkShipmentId);
          if (selectedShipment?.isFrozen) {
            toast.error('The selected default shipment is frozen and cannot accept new expense entries.');
            return Promise.reject(new Error('The selected default shipment is frozen and cannot accept new expense entries.'));
          }
        }
        const targetId = bulkShipmentId === '__MULTIPLE__' ? '' : bulkShipmentId;
        return expenseService.importBulkCsv(targetId, content);
      }}
      onCompleted={onCompleted}
    >
      <div className="space-y-1.5 rounded-lg border border-border/80 bg-muted/40 p-3">
        <Label className="text-xs font-semibold flex items-center justify-between">
          <span>Target Shipment for Uploaded Expenses</span>
          {bulkShipmentId && bulkShipmentId !== '__MULTIPLE__' ? (
            <span className="text-[11px] font-normal text-muted-foreground">
              {shipments.find((s) => s.id === bulkShipmentId)?.isFrozen
                ? '⚠️ Selected shipment is frozen (Locked)'
                : '✓ Active shipment ready'}
            </span>
          ) : (
            <span className="text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
              ✓ Multi-Shipment Mode (Specified per CSV row)
            </span>
          )}
        </Label>
        <Select value={bulkShipmentId} onValueChange={setBulkShipmentId}>
          <SelectTrigger className="w-full bg-background h-9 text-xs">
            <SelectValue placeholder="Select target shipment or choose Multi-Shipment..." />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__MULTIPLE__">
              🌐 Multiple Shipments (Specified via &apos;Shipment Invoice No&apos; in CSV)
            </SelectItem>
            {shipments.map((s) => (
              <SelectItem key={s.id} value={s.id} disabled={s.isFrozen}>
                {s.invoiceNumber} — {s.supplierName || 'No Supplier'} {s.isFrozen ? '(Frozen - Locked)' : ''}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-[11px] text-muted-foreground">
          {bulkShipmentId === '__MULTIPLE__'
            ? 'Each row in the CSV file must specify its target shipment in the "Shipment Invoice No" column.'
            : 'Expenses without a "Shipment Invoice No" in the CSV will be assigned to this shipment. Rows specifying their own shipment will route to their designated shipment.'}
        </p>
      </div>
    </CsvImportDialog>
  );
}
