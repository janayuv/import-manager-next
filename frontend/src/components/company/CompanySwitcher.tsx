import * as React from 'react';
import { Building2, Check, ChevronDown, Plus, Settings2 } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useCompany } from '@/contexts/CompanyContext';
import { CreateCompanyModal } from '@/components/company/CreateCompanyModal';
import { CompanyProfileModal } from '@/components/company/CompanyProfileModal';
import { cn } from '@/lib/utils';

export function CompanySwitcher() {
  const { currentCompany, companies, switchCompany, loading } = useCompany();
  const [open, setOpen] = React.useState(false);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [profileOpen, setProfileOpen] = React.useState(false);

  if (loading || !currentCompany) {
    return (
      <div className="flex h-8 items-center gap-2 rounded-lg border border-border bg-background/50 px-2.5 text-xs text-muted-foreground animate-pulse">
        <Building2 className="h-3.5 w-3.5" />
        <span className="hidden sm:inline">Loading company...</span>
      </div>
    );
  }

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            className="h-8 gap-2 border-border/80 bg-background/90 px-2.5 text-xs font-medium hover:bg-accent hover:text-accent-foreground shadow-2xs"
            title={`Active Company: ${currentCompany.name} (${currentCompany.shortCode})`}
            aria-label="Switch Company"
          >
            <div className="flex h-5 w-5 shrink-0 items-center justify-center rounded bg-primary/10 text-primary">
              <Building2 className="h-3 w-3" />
            </div>
            <span className="max-w-[140px] truncate text-xs font-semibold sm:max-w-[180px]">
              {currentCompany.name}
            </span>
            <Badge
              variant="secondary"
              className="px-1.5 py-0 text-[10px] font-mono uppercase tracking-wider"
            >
              {currentCompany.shortCode}
            </Badge>
            <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground opacity-70" />
          </Button>
        </PopoverTrigger>

        <PopoverContent align="start" className="w-80 p-0 shadow-lg">
          <div className="flex items-center justify-between border-b border-border bg-muted/40 px-3 py-2.5">
            <div>
              <p className="text-xs font-semibold text-foreground">Select Company</p>
              <p className="text-[11px] text-muted-foreground">
                {companies.length} corporate {companies.length === 1 ? 'entity' : 'entities'} registered
              </p>
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground"
              onClick={() => {
                setOpen(false);
                setProfileOpen(true);
              }}
              title="Edit Active Company Profile"
            >
              <Settings2 className="h-3.5 w-3.5" />
            </Button>
          </div>

          <div className="max-h-60 overflow-y-auto p-1.5 space-y-1">
            {companies.map((comp) => {
              const isActive = comp.id === currentCompany.id;
              return (
                <button
                  key={comp.id}
                  type="button"
                  onClick={() => {
                    setOpen(false);
                    if (!isActive) {
                      void switchCompany(comp.id);
                    }
                  }}
                  className={cn(
                    'flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-2 text-left text-xs transition-colors',
                    isActive
                      ? 'bg-primary/10 text-primary font-medium'
                      : 'hover:bg-muted text-foreground',
                  )}
                >
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <span className="truncate font-semibold">{comp.name}</span>
                      <Badge variant="outline" className="px-1 py-0 text-[9px] font-mono shrink-0">
                        {comp.shortCode}
                      </Badge>
                    </div>
                    {(comp.iecCode || comp.gstin) && (
                      <p className="truncate text-[10px] text-muted-foreground mt-0.5">
                        {comp.iecCode ? `IEC: ${comp.iecCode}` : ''}
                        {comp.iecCode && comp.gstin ? ' • ' : ''}
                        {comp.gstin ? `GST: ${comp.gstin}` : ''}
                      </p>
                    )}
                  </div>
                  {isActive && <Check className="h-4 w-4 shrink-0 text-primary" />}
                </button>
              );
            })}
          </div>

          <div className="border-t border-border p-1.5 bg-muted/20 flex items-center justify-between gap-1">
            <Button
              variant="ghost"
              size="sm"
              className="h-8 flex-1 justify-start gap-1.5 text-xs text-muted-foreground hover:text-foreground"
              onClick={() => {
                setOpen(false);
                setCreateOpen(true);
              }}
            >
              <Plus className="h-3.5 w-3.5 text-primary" />
              <span>New Company</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-8 text-xs text-muted-foreground hover:text-foreground"
              onClick={() => {
                setOpen(false);
                setProfileOpen(true);
              }}
            >
              Edit Profile
            </Button>
          </div>
        </PopoverContent>
      </Popover>

      <CreateCompanyModal open={createOpen} onOpenChange={setCreateOpen} />
      <CompanyProfileModal open={profileOpen} onOpenChange={setProfileOpen} />
    </>
  );
}
