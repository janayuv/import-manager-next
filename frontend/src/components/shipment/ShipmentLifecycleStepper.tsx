import * as React from 'react';
import {
  CheckCircle2,
  FileCheck2,
  Landmark,
  Lock,
  PackageCheck,
  Ship,
  Truck,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

export type ShipmentStatus =
  | 'docs-rcvd'
  | 'in-transit'
  | 'customs-clearance'
  | 'ready-dly'
  | 'delivered';

interface StepDef {
  key: ShipmentStatus;
  label: string;
  description: string;
  icon: React.ComponentType<{ className?: string }>;
}

const STEPS: StepDef[] = [
  {
    key: 'docs-rcvd',
    label: 'Docs Received',
    description: 'Invoice & BL registered',
    icon: FileCheck2,
  },
  {
    key: 'in-transit',
    label: 'In Transit',
    description: 'Commercial invoice finalized',
    icon: Ship,
  },
  {
    key: 'customs-clearance',
    label: 'Customs Clearance',
    description: 'BOE duty assessed & challan',
    icon: Landmark,
  },
  {
    key: 'ready-dly',
    label: 'Ready for Delivery',
    description: '7-day BOE window passed',
    icon: Truck,
  },
  {
    key: 'delivered',
    label: 'Delivered & Frozen',
    description: 'Factory gate delivery complete',
    icon: PackageCheck,
  },
];

const ORDER: Record<ShipmentStatus, number> = {
  'docs-rcvd': 0,
  'in-transit': 1,
  'customs-clearance': 2,
  'ready-dly': 3,
  'delivered': 4,
};

interface ShipmentLifecycleStepperProps {
  currentStatus: ShipmentStatus | string;
  isFrozen?: boolean;
  className?: string;
  compact?: boolean;
  onStepClick?: (step: ShipmentStatus) => void;
}

export function ShipmentLifecycleStepper({
  currentStatus,
  isFrozen = false,
  className,
  compact = false,
  onStepClick,
}: ShipmentLifecycleStepperProps) {
  let currentIdx = 0;
  let customBadge: string | null = null;
  if (currentStatus === 'partially-delivered') {
    currentIdx = 3;
    customBadge = 'Partially Delivered';
  } else if (currentStatus === 'in-warehouse') {
    currentIdx = 3;
    customBadge = 'In Warehouse Buffer';
  } else if ((currentStatus as ShipmentStatus) in ORDER) {
    currentIdx = ORDER[currentStatus as ShipmentStatus] ?? 0;
  }

  if (compact) {
    return (
      <div className={cn('flex items-center gap-1.5 overflow-x-auto py-1', className)}>
        {STEPS.map((step, idx) => {
          const isDone = idx < currentIdx || (idx === currentIdx && isFrozen);
          const isCurrent = idx === currentIdx && !isFrozen;
          const Icon = step.icon;

          return (
            <React.Fragment key={step.key}>
              <div
                onClick={() => onStepClick?.(step.key)}
                className={cn(
                  'flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium transition-colors',
                  onStepClick && 'cursor-pointer hover:opacity-80',
                  isCurrent && 'bg-primary text-primary-foreground font-semibold shadow-xs',
                  isDone && !isCurrent && 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400',
                  !isDone && !isCurrent && 'bg-muted/70 text-muted-foreground',
                )}
                title={`${step.label}: ${step.description}`}
              >
                {isDone && !isCurrent ? (
                  <CheckCircle2 className="h-3 w-3 shrink-0" />
                ) : (
                  <Icon className="h-3 w-3 shrink-0" />
                )}
                <span className="truncate">{step.label}</span>
              </div>
              {idx < STEPS.length - 1 && (
                <div
                  className={cn(
                    'h-0.5 w-3 shrink-0',
                    idx < currentIdx ? 'bg-emerald-500' : 'bg-border',
                  )}
                />
              )}
            </React.Fragment>
          );
        })}
        {customBadge && (
          <Badge variant="secondary" className="ml-1 text-[10px] bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300">
            {customBadge}
          </Badge>
        )}
        {isFrozen && (
          <Badge variant="outline" className="ml-1 gap-1 border-muted-foreground/40 text-[10px]">
            <Lock className="h-2.5 w-2.5" /> Frozen
          </Badge>
        )}
      </div>
    );
  }

  return (
    <div className={cn('rounded-lg border border-border bg-card p-4 shadow-2xs', className)}>
      <div className="flex flex-wrap items-center justify-between gap-2 pb-3 mb-3 border-b border-border">
        <div>
          <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Shipment Pipeline Status
          </span>
          <h4 className="text-sm font-semibold tracking-tight">
            Lifecycle Progress: <span className="text-primary">{customBadge ? `${STEPS[currentIdx]?.label} (${customBadge})` : STEPS[currentIdx]?.label}</span>
          </h4>
        </div>
        <div className="flex items-center gap-2">
          {isFrozen ? (
            <Badge variant="secondary" className="gap-1 bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-200">
              <Lock className="h-3 w-3" /> Ledger Record Frozen
            </Badge>
          ) : (
            <Badge variant="default" className="text-xs">
              Stage {currentIdx + 1} of 5
            </Badge>
          )}
        </div>
      </div>

      <div className="relative grid grid-cols-1 sm:grid-cols-5 gap-3">
        {STEPS.map((step, idx) => {
          const isDone = idx < currentIdx || (idx === currentIdx && isFrozen);
          const isCurrent = idx === currentIdx && !isFrozen;
          const Icon = step.icon;

          return (
            <div
              key={step.key}
              onClick={() => onStepClick?.(step.key)}
              className={cn(
                'relative flex flex-col justify-between rounded-md border p-3 transition-all',
                onStepClick && 'cursor-pointer hover:border-primary/60',
                isCurrent &&
                  'border-primary bg-primary/5 shadow-xs ring-1 ring-primary/30',
                isDone && !isCurrent &&
                  'border-emerald-300/70 bg-emerald-50/50 dark:border-emerald-800/60 dark:bg-emerald-950/20',
                !isDone && !isCurrent &&
                  'border-border/60 bg-muted/20 opacity-70',
              )}
            >
              <div className="flex items-center justify-between gap-1 mb-1.5">
                <div
                  className={cn(
                    'flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold',
                    isCurrent && 'bg-primary text-primary-foreground',
                    isDone && !isCurrent && 'bg-emerald-600 text-white',
                    !isDone && !isCurrent && 'bg-muted text-muted-foreground',
                  )}
                >
                  {isDone && !isCurrent ? <CheckCircle2 className="h-3.5 w-3.5" /> : idx + 1}
                </div>
                <Icon
                  className={cn(
                    'h-4 w-4',
                    isCurrent && 'text-primary',
                    isDone && !isCurrent && 'text-emerald-600 dark:text-emerald-400',
                    !isDone && !isCurrent && 'text-muted-foreground/60',
                  )}
                />
              </div>

              <div>
                <p
                  className={cn(
                    'text-xs font-medium leading-snug',
                    isCurrent && 'font-bold text-foreground',
                    isDone && !isCurrent && 'text-emerald-900 dark:text-emerald-200',
                    !isDone && !isCurrent && 'text-muted-foreground',
                  )}
                >
                  {step.label}
                </p>
                <p className="mt-0.5 text-[11px] text-muted-foreground line-clamp-2">
                  {step.description}
                </p>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
