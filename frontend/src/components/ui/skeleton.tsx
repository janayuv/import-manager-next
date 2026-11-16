import * as React from 'react';
import { cn } from '@/lib/utils';

export function Skeleton({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('animate-pulse rounded-md bg-muted/70', className)}
      {...props}
    />
  );
}

export function SkeletonCard({ className }: { className?: string }) {
  return (
    <div className={cn('rounded-lg border border-border bg-card p-5 space-y-3', className)}>
      <div className="flex items-center justify-between">
        <Skeleton className="h-4 w-28" />
        <Skeleton className="h-4 w-4 rounded-full" />
      </div>
      <Skeleton className="h-7 w-36" />
      <Skeleton className="h-3 w-48" />
    </div>
  );
}

export function SkeletonTable({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="w-full rounded-md border border-border bg-card p-4 space-y-3">
      <div className="flex items-center justify-between pb-2 border-b border-border">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-8 w-24 rounded-md" />
      </div>
      <div className="grid grid-cols-4 gap-4 py-2 border-b border-border/50">
        {Array.from({ length: cols }).map((_, i) => (
          <Skeleton key={i} className="h-4 w-full" />
        ))}
      </div>
      <div className="space-y-2 pt-1">
        {Array.from({ length: rows }).map((_, r) => (
          <div key={r} className="grid grid-cols-4 gap-4 py-2 border-b border-border/30 last:border-0">
            {Array.from({ length: cols }).map((_, c) => (
              <Skeleton key={c} className="h-4 w-5/6" />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

export function SkeletonChart({ className }: { className?: string }) {
  return (
    <div className={cn('rounded-lg border border-border bg-card p-5 space-y-4', className)}>
      <div className="space-y-2">
        <Skeleton className="h-5 w-48" />
        <Skeleton className="h-3 w-64" />
      </div>
      <div className="h-64 w-full flex items-end gap-3 pt-4">
        {Array.from({ length: 12 }).map((_, i) => (
          <Skeleton
            key={i}
            className="flex-1 rounded-t"
            style={{ height: `${20 + ((i * 17) % 75)}%` }}
          />
        ))}
      </div>
    </div>
  );
}
