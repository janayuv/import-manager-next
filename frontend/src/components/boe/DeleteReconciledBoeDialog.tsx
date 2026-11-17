import * as React from 'react';
import { AlertTriangle, Trash2 } from 'lucide-react';
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
import { boeSummaryService } from '@/services/boe-service';

export interface DeleteReconciledBoeTarget {
  id: string;
  invoiceNumber: string;
  shipmentId: string;
  beNumber?: string;
  dutyPaid?: number;
}

interface DeleteReconciledBoeDialogProps {
  target: DeleteReconciledBoeTarget | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
}

export function DeleteReconciledBoeDialog({
  target,
  open,
  onOpenChange,
  onDeleted,
}: DeleteReconciledBoeDialogProps) {
  const [deleting, setDeleting] = React.useState(false);

  const handleDelete = async () => {
    if (!target) return;
    setDeleting(true);
    try {
      await boeSummaryService.delete(target.id);
      toast.success(
        `Moved BOE calculation for ${target.invoiceNumber} to Recycle Bin`
      );
      onOpenChange(false);
      onDeleted();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <div className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="h-5 w-5" />
            <DialogTitle>Delete BOE Calculation?</DialogTitle>
          </div>
          <DialogDescription className="pt-2 text-sm text-foreground">
            Are you sure you want to delete this reconciled BOE calculation?
          </DialogDescription>
        </DialogHeader>

        {target && (
          <div className="rounded-lg border p-3 bg-muted/30 text-xs space-y-1.5">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Invoice Number:</span>
              <span className="font-semibold">{target.invoiceNumber}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Shipment ID:</span>
              <span className="font-mono">{target.shipmentId}</span>
            </div>
            {target.beNumber && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Linked BE:</span>
                <span className="font-mono">{target.beNumber}</span>
              </div>
            )}
            <div className="flex justify-between">
              <span className="text-muted-foreground">Calculation ID:</span>
              <span className="font-mono text-muted-foreground">{target.id}</span>
            </div>
          </div>
        )}

        <p className="text-xs text-muted-foreground">
          This record will be moved to the <strong>Recycle Bin</strong> and can be
          restored by administrators if needed.
        </p>

        <DialogFooter className="flex items-center justify-between pt-2">
          <Button
            variant="ghost"
            size="sm"
            disabled={deleting}
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            size="sm"
            disabled={deleting || !target}
            onClick={handleDelete}
          >
            <Trash2 className="h-4 w-4" />
            {deleting ? 'Deleting...' : 'Move to Recycle Bin'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
