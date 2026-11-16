import * as React from 'react';
import {
  Download,
  FileText,
  Maximize2,
  Minimize2,
  RotateCw,
  ZoomIn,
  ZoomOut,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { downloadTextFile } from '@/lib/download';
import { cn } from '@/lib/utils';

export interface DocumentPreviewFile {
  name: string;
  type?: string;
  size?: number | string;
  url?: string;
  content?: string;
  category?: string;
}

interface DocumentPreviewDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  document: DocumentPreviewFile | null;
}

export function DocumentPreviewDialog({
  open,
  onOpenChange,
  document,
}: DocumentPreviewDialogProps) {
  const [zoom, setZoom] = React.useState(100);
  const [rotation, setRotation] = React.useState(0);
  const [isMaximized, setIsMaximized] = React.useState(false);

  React.useEffect(() => {
    if (open) {
      setZoom(100);
      setRotation(0);
      setIsMaximized(false);
    }
  }, [open, document]);

  if (!document) return null;

  const handleZoomIn = () => setZoom((prev) => Math.min(prev + 25, 200));
  const handleZoomOut = () => setZoom((prev) => Math.max(prev - 25, 50));
  const handleRotate = () => setRotation((prev) => (prev + 90) % 360);

  const handleDownload = () => {
    if (document.content) {
      downloadTextFile(document.name, document.content);
    } else if (document.url) {
      const a = window.document.createElement('a');
      a.href = document.url;
      a.download = document.name;
      a.click();
    }
  };

  const isImage =
    document.type?.startsWith('image/') ||
    /\.(jpg|jpeg|png|webp|gif|svg)$/i.test(document.name);

  const isPdf =
    document.type === 'application/pdf' || /\.pdf$/i.test(document.name);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn(
          'transition-all duration-200 flex flex-col',
          isMaximized
            ? 'fixed inset-2 max-w-none w-[calc(100vw-16px)] h-[calc(100vh-16px)] rounded-xl'
            : 'max-w-4xl h-[85vh]',
        )}
      >
        <DialogHeader className="shrink-0 border-b border-border pb-3 pr-8">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-2 min-w-0">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <FileText className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <DialogTitle className="truncate text-base">{document.name}</DialogTitle>
                <DialogDescription className="sr-only">
                  In-app vault document preview for {document.name}
                </DialogDescription>
                <div className="flex items-center gap-2 text-xs text-muted-foreground mt-0.5">
                  {document.category && <Badge variant="outline">{document.category}</Badge>}
                  {document.size && <span>{String(document.size)}</span>}
                  <span>In-App Vault Viewer</span>
                </div>
              </div>
            </div>

            {/* Viewer Controls */}
            <div className="flex items-center gap-1.5 shrink-0">
              <Button
                variant="outline"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={handleZoomOut}
                title="Zoom Out"
              >
                <ZoomOut className="h-3.5 w-3.5" />
              </Button>
              <span className="font-mono text-xs w-11 text-center">{zoom}%</span>
              <Button
                variant="outline"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={handleZoomIn}
                title="Zoom In"
              >
                <ZoomIn className="h-3.5 w-3.5" />
              </Button>

              <div className="h-4 w-px bg-border mx-1" />

              <Button
                variant="outline"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={handleRotate}
                title="Rotate 90° Clockwise"
              >
                <RotateCw className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={() => setIsMaximized(!isMaximized)}
                title={isMaximized ? 'Restore Size' : 'Maximize Window'}
              >
                {isMaximized ? (
                  <Minimize2 className="h-3.5 w-3.5" />
                ) : (
                  <Maximize2 className="h-3.5 w-3.5" />
                )}
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="h-7 px-2 text-xs gap-1"
                onClick={handleDownload}
                title="Download file to local machine"
              >
                <Download className="h-3.5 w-3.5" /> Download
              </Button>
            </div>
          </div>
        </DialogHeader>

        {/* Document Content Viewport */}
        <div className="flex-1 overflow-auto bg-muted/30 p-4 flex items-center justify-center rounded-b-lg">
          {isImage && document.url ? (
            <div
              className="transition-transform duration-150"
              style={{
                transform: `scale(${zoom / 100}) rotate(${rotation}deg)`,
              }}
            >
              <img
                src={document.url}
                alt={document.name}
                className="max-w-full max-h-[65vh] object-contain rounded shadow-md"
              />
            </div>
          ) : isPdf && document.url ? (
            <iframe
              src={document.url}
              title={document.name}
              className="w-full h-full min-h-[500px] rounded border border-border bg-white shadow-xs"
            />
          ) : document.content ? (
            <div
              className="w-full h-full overflow-auto rounded bg-card p-4 border border-border font-mono text-xs whitespace-pre leading-relaxed"
              style={{ fontSize: `${(zoom / 100) * 12}px` }}
            >
              {document.content}
            </div>
          ) : (
            <div className="text-center p-8 text-muted-foreground space-y-2">
              <FileText className="h-10 w-10 mx-auto text-muted-foreground/60" />
              <p className="text-sm font-medium">Document Preview Ready</p>
              <p className="text-xs">
                Binary preview stream active for {document.name}.
              </p>
              <Button size="sm" variant="outline" onClick={handleDownload}>
                <Download className="h-3.5 w-3.5 mr-1" /> Download Copy
              </Button>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
