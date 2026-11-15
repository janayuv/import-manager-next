import * as React from 'react';
import {
  AlertTriangle,
  ArrowUpDown,
  Calendar,
  Check,
  ChevronDown,
  ExternalLink,
  Link2,
  Link2Off,
  Search,
  Sparkles,
  X,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { cn } from '@/lib/utils';
import type { BOEDetail } from '@/services/boe-service';

export interface LinkedBoeSelectorProps {
  value: string;
  onChange: (boeId: string) => void;
  boes: BOEDetail[];
  expectedDuty?: number;
  cashDuty?: number;
  disabled?: boolean;
}

type FilterTab = 'all' | 'exact-match' | 'recent';
type SortOption = 'date-desc' | 'date-asc' | 'duty-desc' | 'duty-asc' | 'be-number';

export function LinkedBoeSelector({
  value,
  onChange,
  boes,
  expectedDuty,
  cashDuty,
  disabled = false,
}: LinkedBoeSelectorProps) {
  const [open, setOpen] = React.useState(false);
  const [search, setSearch] = React.useState('');
  const [filterTab, setFilterTab] = React.useState<FilterTab>('all');
  const [sortOption, setSortOption] = React.useState<SortOption>('date-desc');
  const [advancedModalOpen, setAdvancedModalOpen] = React.useState(false);
  const [modalSearch, setModalSearch] = React.useState('');

  const searchInputRef = React.useRef<HTMLInputElement | null>(null);

  const selectedBoe = React.useMemo(() => {
    if (!value || value === 'none') return null;
    return boes.find((b) => b.id === value) ?? null;
  }, [value, boes]);

  const isDutyMatch = React.useCallback(
    (b: BOEDetail) => {
      if (expectedDuty === undefined && cashDuty === undefined) return false;
      const paid = Math.round(b.dutyPaid);
      if (paid < 0) return false;
      const matchExpected =
        expectedDuty !== undefined && expectedDuty >= 0 && paid === Math.round(expectedDuty);
      const matchCash =
        cashDuty !== undefined && cashDuty >= 0 && paid === Math.round(cashDuty);
      return matchExpected || matchCash;
    },
    [expectedDuty, cashDuty],
  );

  const isRecent = React.useCallback((beDate: string) => {
    if (!beDate) return false;
    const d = new Date(beDate);
    if (Number.isNaN(d.getTime())) return false;
    const diffDays = (Date.now() - d.getTime()) / (1000 * 60 * 60 * 24);
    return diffDays >= -7 && diffDays <= 60;
  }, []);

  const exactMatchCount = React.useMemo(() => {
    return boes.filter(isDutyMatch).length;
  }, [boes, isDutyMatch]);

  const recentCount = React.useMemo(() => {
    return boes.filter((b) => isRecent(b.beDate)).length;
  }, [boes, isRecent]);

  const filteredBoes = React.useMemo(() => {
    const q = search.trim().toLowerCase();

    const matchesQuery = (b: BOEDetail) => {
      if (!q) return true;
      if (b.beNumber?.toLowerCase().includes(q)) return true;
      if (b.beDate?.toLowerCase().includes(q)) return true;
      if (b.location?.toLowerCase().includes(q)) return true;
      if (b.challanNumber?.toLowerCase().includes(q)) return true;
      const paidRound = Math.round(b.dutyPaid).toString();
      const paidFormatted = Math.round(b.dutyPaid).toLocaleString().toLowerCase();
      if (paidRound.includes(q) || paidFormatted.includes(q)) return true;
      return false;
    };

    const matchesFilterTab = (b: BOEDetail) => {
      if (filterTab === 'exact-match') return isDutyMatch(b);
      if (filterTab === 'recent') return isRecent(b.beDate);
      return true;
    };

    const list = boes.filter((b) => matchesQuery(b) && matchesFilterTab(b));

    return list.sort((a, b) => {
      if (sortOption === 'date-desc') {
        return (b.beDate || '').localeCompare(a.beDate || '');
      }
      if (sortOption === 'date-asc') {
        return (a.beDate || '').localeCompare(b.beDate || '');
      }
      if (sortOption === 'duty-desc') {
        return b.dutyPaid - a.dutyPaid;
      }
      if (sortOption === 'duty-asc') {
        return a.dutyPaid - b.dutyPaid;
      }
      if (sortOption === 'be-number') {
        return (a.beNumber || '').localeCompare(b.beNumber || '', undefined, { numeric: true });
      }
      return 0;
    });
  }, [boes, search, filterTab, sortOption, isDutyMatch, isRecent]);

  // Modal filtered list
  const modalFilteredBoes = React.useMemo(() => {
    const q = modalSearch.trim().toLowerCase();
    return boes
      .filter((b) => {
        if (!q) return true;
        if (b.beNumber?.toLowerCase().includes(q)) return true;
        if (b.beDate?.toLowerCase().includes(q)) return true;
        if (b.location?.toLowerCase().includes(q)) return true;
        if (b.challanNumber?.toLowerCase().includes(q)) return true;
        const paidRound = Math.round(b.dutyPaid).toString();
        const paidFormatted = Math.round(b.dutyPaid).toLocaleString().toLowerCase();
        return paidRound.includes(q) || paidFormatted.includes(q);
      })
      .sort((a, b) => (b.beDate || '').localeCompare(a.beDate || ''));
  }, [boes, modalSearch]);

  const selectedMatchesDuty = selectedBoe ? isDutyMatch(selectedBoe) : false;

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <label htmlFor="linkedBoeSelect" className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
          Linked ICEGATE BOE (for Duty Paid Check)
        </label>
        <button
          type="button"
          onClick={() => setAdvancedModalOpen(true)}
          className="text-[11px] text-primary hover:underline inline-flex items-center gap-1 font-medium"
          title="Open advanced BOE search dialog"
        >
          <Search className="h-3 w-3" />
          Advanced Search
        </button>
      </div>

      <Popover
        open={open}
        onOpenChange={(next) => {
          if (disabled) return;
          setOpen(next);
          if (next) {
            setTimeout(() => searchInputRef.current?.focus(), 50);
          } else {
            setSearch('');
          }
        }}
      >
        <PopoverTrigger asChild>
          <div
            id="linkedBoeSelect"
            role="combobox"
            aria-expanded={open}
            aria-haspopup="listbox"
            tabIndex={disabled ? -1 : 0}
            className={cn(
              'group flex h-9 w-full items-center justify-between rounded-md border border-input bg-card px-3 py-2 text-sm shadow-xs transition-colors cursor-pointer select-none',
              'focus:outline-none focus:ring-2 focus:ring-ring',
              disabled && 'cursor-not-allowed opacity-50',
            )}
            onClick={() => {
              if (!disabled) setOpen((prev) => !prev);
            }}
            onKeyDown={(e) => {
              if (disabled) return;
              if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') {
                e.preventDefault();
                setOpen(true);
              }
            }}
          >
            <div className="flex items-center gap-2 min-w-0 overflow-hidden pr-1">
              {selectedBoe ? (
                <>
                  <Link2 className="h-4 w-4 text-primary shrink-0" />
                  <span className="truncate font-medium text-foreground">
                    {selectedBoe.beNumber} ({selectedBoe.beDate}) — Paid: INR{' '}
                    {Math.round(selectedBoe.dutyPaid).toLocaleString()}
                  </span>
                  {expectedDuty !== undefined && (
                    selectedMatchesDuty ? (
                      <span className="shrink-0 inline-flex items-center gap-0.5 rounded bg-green-500/10 px-1.5 py-0.5 text-[10px] font-medium text-green-500 border border-green-500/20">
                        <Check className="h-3 w-3" /> Match
                      </span>
                    ) : (
                      <span className="shrink-0 inline-flex items-center gap-0.5 rounded bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-500 border border-amber-500/20">
                        <AlertTriangle className="h-3 w-3" /> Mismatch
                      </span>
                    )
                  )}
                </>
              ) : (
                <>
                  <Link2Off className="h-4 w-4 text-muted-foreground shrink-0" />
                  <span className="text-muted-foreground truncate">
                    Unlinked (Awaiting BOE Data)
                  </span>
                </>
              )}
            </div>

            <div className="flex items-center gap-1 shrink-0">
              {selectedBoe && !disabled && (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    onChange('');
                  }}
                  className="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
                  title="Unlink BOE"
                  aria-label="Unlink BOE"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
              <ChevronDown className="h-4 w-4 opacity-50 shrink-0" />
            </div>
          </div>
        </PopoverTrigger>

        <PopoverContent
          align="start"
          className="w-[420px] sm:w-[480px] p-3 space-y-2.5 max-w-[95vw]"
        >
          {/* Header & Quick stats */}
          <div className="flex items-center justify-between border-b border-border/40 pb-2">
            <div className="flex items-center gap-1.5">
              <Search className="h-3.5 w-3.5 text-primary" />
              <span className="text-xs font-semibold text-foreground">Find & Link ICEGATE BOE</span>
            </div>
            {exactMatchCount > 0 && (
              <span className="text-[11px] text-green-500 font-medium flex items-center gap-1 bg-green-500/10 px-1.5 py-0.5 rounded border border-green-500/20">
                <Sparkles className="h-3 w-3" /> {exactMatchCount} exact duty {exactMatchCount === 1 ? 'match' : 'matches'}
              </span>
            )}
          </div>

          {/* Search Bar */}
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground pointer-events-none" />
            <Input
              ref={searchInputRef}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search BE number, date (YYYY-MM-DD), duty INR, challan..."
              className="pl-8 pr-8 h-9 text-xs"
            />
            {search && (
              <button
                type="button"
                onClick={() => setSearch('')}
                className="absolute right-2.5 top-2.5 p-0.5 text-muted-foreground hover:text-foreground"
                title="Clear search input"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            )}
          </div>

          {/* Quick Filter Chips & Sort Controls */}
          <div className="flex flex-wrap items-center justify-between gap-1.5 text-xs">
            <div className="flex items-center gap-1 flex-wrap">
              <button
                type="button"
                onClick={() => setFilterTab('all')}
                className={cn(
                  'px-2 py-0.5 rounded text-[11px] font-medium border transition-colors',
                  filterTab === 'all'
                    ? 'bg-primary text-primary-foreground border-primary'
                    : 'bg-muted/50 text-muted-foreground hover:bg-muted border-border/50',
                )}
              >
                All ({boes.length})
              </button>
              {exactMatchCount > 0 && (
                <button
                  type="button"
                  onClick={() => setFilterTab(filterTab === 'exact-match' ? 'all' : 'exact-match')}
                  className={cn(
                    'px-2 py-0.5 rounded text-[11px] font-medium border flex items-center gap-1 transition-colors',
                    filterTab === 'exact-match'
                      ? 'bg-green-600 text-white border-green-600'
                      : 'bg-green-500/10 text-green-500 border-green-500/30 hover:bg-green-500/20',
                  )}
                  title="Show only BOEs whose duty paid matches computed duty"
                >
                  <Sparkles className="h-3 w-3" />
                  Exact Duty Match ({exactMatchCount})
                </button>
              )}
              {recentCount > 0 && (
                <button
                  type="button"
                  onClick={() => setFilterTab(filterTab === 'recent' ? 'all' : 'recent')}
                  className={cn(
                    'px-2 py-0.5 rounded text-[11px] font-medium border flex items-center gap-1 transition-colors',
                    filterTab === 'recent'
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'bg-muted/50 text-muted-foreground hover:bg-muted border-border/50',
                  )}
                >
                  <Calendar className="h-3 w-3" />
                  Recent ({recentCount})
                </button>
              )}
            </div>

            {/* Sort Dropdown */}
            <div className="flex items-center gap-1 text-[11px] text-muted-foreground ml-auto">
              <ArrowUpDown className="h-3 w-3" />
              <select
                value={sortOption}
                onChange={(e) => setSortOption(e.target.value as SortOption)}
                aria-label="Sort BOEs by"
                className="bg-card border border-border/60 rounded px-1.5 py-0.5 text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              >
                <option value="date-desc">Newest Date</option>
                <option value="date-asc">Oldest Date</option>
                <option value="duty-desc">Duty: High → Low</option>
                <option value="duty-asc">Duty: Low → High</option>
                <option value="be-number">BE Number</option>
              </select>
            </div>
          </div>

          {/* List Items */}
          <div className="max-h-64 overflow-y-auto space-y-1 pr-1" role="listbox">
            {/* Unlinked option */}
            {(!search || 'unlinked awaiting boe data'.includes(search.toLowerCase())) && (
              <div
                role="option"
                aria-selected={!selectedBoe}
                tabIndex={0}
                onClick={() => {
                  onChange('');
                  setOpen(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    onChange('');
                    setOpen(false);
                  }
                }}
                className={cn(
                  'flex items-center justify-between p-2 rounded-md cursor-pointer transition-colors text-xs border border-transparent',
                  !selectedBoe
                    ? 'bg-primary/10 font-medium text-foreground border-primary/20'
                    : 'hover:bg-accent/40 text-muted-foreground hover:text-foreground',
                )}
              >
                <div className="flex items-center gap-2">
                  <Link2Off className="h-4 w-4 text-muted-foreground shrink-0" />
                  <span>Unlinked (Awaiting BOE Data)</span>
                </div>
                {!selectedBoe && <Check className="h-3.5 w-3.5 text-primary shrink-0" />}
              </div>
            )}

            {filteredBoes.map((b) => {
              const isSelected = selectedBoe?.id === b.id;
              const dutyMatch = isDutyMatch(b);
              return (
                <div
                  key={b.id}
                  role="option"
                  aria-selected={isSelected}
                  tabIndex={0}
                  onClick={() => {
                    onChange(b.id);
                    setOpen(false);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      onChange(b.id);
                      setOpen(false);
                    }
                  }}
                  className={cn(
                    'flex flex-col gap-1 p-2 rounded-md cursor-pointer transition-colors text-xs border',
                    isSelected
                      ? 'bg-accent/80 font-medium border-primary/50 text-foreground'
                      : dutyMatch
                        ? 'bg-green-500/5 hover:bg-green-500/10 border-green-500/20 text-foreground'
                        : 'border-transparent hover:bg-accent/40 text-foreground',
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-1.5 flex-wrap">
                      <span className="font-semibold font-mono text-sm text-foreground">
                        {b.beNumber}
                      </span>
                      <span className="text-[11px] text-muted-foreground">({b.beDate})</span>
                      {b.location && (
                        <Badge variant="outline" className="text-[10px] px-1 py-0 h-4 border-border/50">
                          {b.location}
                        </Badge>
                      )}
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      {dutyMatch && (
                        <Badge
                          variant="outline"
                          className="bg-green-500/15 text-green-500 border-green-500/30 text-[10px] px-1.5 py-0 h-4 flex items-center gap-0.5"
                        >
                          <Sparkles className="h-2.5 w-2.5" />
                          Exact Duty Match
                        </Badge>
                      )}
                      {isSelected && <Check className="h-3.5 w-3.5 text-primary shrink-0" />}
                    </div>
                  </div>
                  <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                    <div>
                      Paid:{' '}
                      <span className="font-semibold text-foreground">
                        INR {Math.round(b.dutyPaid).toLocaleString()}
                      </span>
                    </div>
                    {b.challanNumber && (
                      <span>
                        Challan: <span className="font-mono">{b.challanNumber}</span>
                      </span>
                    )}
                  </div>
                </div>
              );
            })}

            {filteredBoes.length === 0 && (
              <div className="py-6 text-center text-xs text-muted-foreground space-y-2">
                <div>No BOEs match your search or filter.</div>
                {(search || filterTab !== 'all') && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      setSearch('');
                      setFilterTab('all');
                    }}
                    className="h-7 text-xs"
                  >
                    Clear Search & Filters
                  </Button>
                )}
              </div>
            )}
          </div>

          {/* Footer Info & Actions */}
          <div className="flex items-center justify-between pt-2 border-t border-border/40 text-[11px] text-muted-foreground">
            <span>
              Showing {filteredBoes.length} of {boes.length} BOEs
            </span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => {
                  setOpen(false);
                  setAdvancedModalOpen(true);
                }}
                className="text-primary hover:underline font-medium inline-flex items-center gap-0.5"
              >
                Table View <ExternalLink className="h-3 w-3" />
              </button>
            </div>
          </div>
        </PopoverContent>
      </Popover>

      {/* Advanced Full-Table Search Dialog */}
      <Dialog open={advancedModalOpen} onOpenChange={setAdvancedModalOpen}>
        <DialogContent className="max-w-4xl max-h-[85vh] flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center justify-between pr-6">
              <span>Find & Link ICEGATE BOE</span>
              {expectedDuty !== undefined && expectedDuty > 0 && (
                <Badge variant="outline" className="text-xs">
                  Expected Customs Duty: INR {Math.round(expectedDuty).toLocaleString()}
                </Badge>
              )}
            </DialogTitle>
            <DialogDescription>
              Search by BE number, date, duty paid, or challan number from the registry to link to this shipment.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 flex-1 overflow-hidden flex flex-col pt-1">
            <div className="relative">
              <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground pointer-events-none" />
              <Input
                value={modalSearch}
                onChange={(e) => setModalSearch(e.target.value)}
                placeholder="Quick filter across BE number, date, port, challan, or duty paid..."
                className="pl-9 pr-8"
                autoFocus
              />
              {modalSearch && (
                <button
                  type="button"
                  onClick={() => setModalSearch('')}
                  className="absolute right-3 top-2.5 text-muted-foreground hover:text-foreground"
                >
                  <X className="h-4 w-4" />
                </button>
              )}
            </div>

            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>Found {modalFilteredBoes.length} eligible BOEs</span>
              {value && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => {
                    onChange('');
                    setAdvancedModalOpen(false);
                  }}
                >
                  <Link2Off className="h-3.5 w-3.5 mr-1" />
                  Unlink Current BOE
                </Button>
              )}
            </div>

            <div className="flex-1 overflow-y-auto border border-border/50 rounded-md">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>BE Number</TableHead>
                    <TableHead>BE Date</TableHead>
                    <TableHead>Port</TableHead>
                    <TableHead className="text-right">Assessable Value (INR)</TableHead>
                    <TableHead className="text-right">Duty Paid (INR)</TableHead>
                    <TableHead>Challan / Payment</TableHead>
                    <TableHead className="text-right">Action</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {modalFilteredBoes.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                        No BOEs match your query &quot;{modalSearch}&quot;
                      </TableCell>
                    </TableRow>
                  ) : (
                    modalFilteredBoes.map((b) => {
                      const isSelected = selectedBoe?.id === b.id;
                      const dutyMatch = isDutyMatch(b);
                      return (
                        <TableRow
                          key={b.id}
                          className={cn(
                            isSelected && 'bg-primary/5 font-medium',
                            !isSelected && dutyMatch && 'bg-green-500/5',
                          )}
                        >
                          <TableCell className="font-mono font-semibold">
                            <div className="flex items-center gap-1.5">
                              {b.beNumber}
                              {dutyMatch && (
                                <Badge variant="outline" className="bg-green-500/10 text-green-500 border-green-500/30 text-[10px] px-1 py-0">
                                  Match
                                </Badge>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>{b.beDate}</TableCell>
                          <TableCell>{b.location || '—'}</TableCell>
                          <TableCell className="text-right font-mono">
                            {b.totalAssessmentValue > 0
                              ? b.totalAssessmentValue.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })
                              : '—'}
                          </TableCell>
                          <TableCell className="text-right font-mono font-semibold">
                            INR {Math.round(b.dutyPaid).toLocaleString()}
                          </TableCell>
                          <TableCell className="text-xs">
                            {b.challanNumber ? (
                              <div>
                                <span className="font-mono font-medium">{b.challanNumber}</span>
                                {b.paymentDate && <div className="text-muted-foreground">{b.paymentDate}</div>}
                              </div>
                            ) : (
                              '—'
                            )}
                          </TableCell>
                          <TableCell className="text-right">
                            {isSelected ? (
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                className="h-7 text-xs border-primary/50 text-primary"
                                onClick={() => {
                                  onChange('');
                                  setAdvancedModalOpen(false);
                                }}
                              >
                                <Check className="h-3 w-3 mr-1 text-primary" />
                                Linked (Unlink)
                              </Button>
                            ) : (
                              <Button
                                type="button"
                                size="sm"
                                variant={dutyMatch ? 'default' : 'secondary'}
                                className={cn(
                                  'h-7 text-xs',
                                  dutyMatch && 'bg-green-600 hover:bg-green-700 text-white',
                                )}
                                onClick={() => {
                                  onChange(b.id);
                                  setAdvancedModalOpen(false);
                                }}
                              >
                                Link BOE
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
