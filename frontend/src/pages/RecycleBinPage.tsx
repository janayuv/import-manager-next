import { useEffect, useState } from 'react';
import {
  AlertTriangle,
  Check,
  Copy,
  RefreshCw,
  RotateCcw,
  Trash2,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  recycleBinService,
  type DependencyCount,
  type MissingParentGroup,
  type RecycleBinListResponse,
} from '@/services/data-ops-service';

const PAGE_SIZE = 50;

export function RecycleBinPage() {
  const [tables, setTables] = useState<string[]>([]);
  const [selectedTable, setSelectedTable] = useState<string>('all');
  const [search, setSearch] = useState<string>('');
  const [page, setPage] = useState<number>(1);
  const [data, setData] = useState<RecycleBinListResponse | null>(null);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [working, setWorking] = useState<boolean>(false);
  const [retentionDays, setRetentionDays] = useState<string>('30');
  const [missingParents, setMissingParents] = useState<MissingParentGroup[] | null>(null);
  const [depBlockers, setDepBlockers] = useState<DependencyCount[] | null>(null);
  const [blockerMessage, setBlockerMessage] = useState<string>('');

  const load = async (tbl = selectedTable, q = search, pg = page) => {
    setLoading(true);
    try {
      const [tblList, resp] = await Promise.all([
        recycleBinService.getSoftDeleteTables(),
        recycleBinService.getDeletedRecords(tbl, q, pg, PAGE_SIZE),
      ]);
      setTables(tblList ?? []);
      setData(resp);
      setSelectedIds([]);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load recycle bin');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
    );
  };

  const resolveSingleTargetTable = (): { table: string; ids: string[] } | null => {
    if (!data || selectedIds.length === 0) return null;
    if (selectedTable !== 'all') {
      return { table: selectedTable, ids: selectedIds };
    }
    const chosenRows = data.rows.filter((r) => selectedIds.includes(r.recordId));
    const distinctTables = Array.from(new Set(chosenRows.map((r) => r.tableName)));
    const firstTable = distinctTables[0];
    if (distinctTables.length !== 1 || !firstTable) {
      toast.error('Please select records from a single table filter when restoring or purging in bulk.');
      return null;
    }
    return { table: firstTable, ids: selectedIds };
  };

  const handleRestore = async (tableName: string, ids: string[]) => {
    setWorking(true);
    setMissingParents(null);
    setDepBlockers(null);
    setBlockerMessage('');
    try {
      const res = await recycleBinService.restoreRecords(tableName, ids);
      if (res.blocked && res.errorCode === 'missing_parent') {
        setMissingParents(res.missingParents ?? []);
        setBlockerMessage(res.message);
        toast.error('Restore blocked: parent record(s) must be restored first.');
        return;
      }
      toast.success(res.message);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Restore failed');
    } finally {
      setWorking(false);
    }
  };

  const handlePurge = async (tableName: string, ids: string[]) => {
    setWorking(true);
    setMissingParents(null);
    setDepBlockers(null);
    setBlockerMessage('');
    try {
      const res = await recycleBinService.permanentlyDeleteRecords(tableName, ids);
      if (res.blocked && res.errorCode === 'DEPENDENCY') {
        setDepBlockers(res.dependencies ?? []);
        setBlockerMessage(res.message);
        toast.error('Permanent purge blocked due to active/linked child dependencies.');
        return;
      }
      toast.success(res.message);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Purge failed');
    } finally {
      setWorking(false);
    }
  };

  const handlePurgeExpired = async () => {
    const days = Number(retentionDays) || 30;
    setWorking(true);
    try {
      const res = await recycleBinService.purgeExpired(days);
      toast.success(res.message);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Retention purge failed');
    } finally {
      setWorking(false);
    }
  };

  const copyMissingParents = async () => {
    if (!missingParents) return;
    const text = missingParents
      .map((g) => `${g.tableName}: ${g.parentIds.join(', ')}`)
      .join('\n');
    try {
      await navigator.clipboard.writeText(text);
      toast.success('Copied missing parent IDs to clipboard');
    } catch {
      toast.info(text);
    }
  };

  const rows = data?.rows ?? [];
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE));

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Recycle Bin &amp; Soft-Delete Recovery</h1>
          <p className="text-sm text-muted-foreground">
            Recover soft-deleted ERP records across all 11 core tables with parent integrity &amp; dependency checks.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Input
            aria-label="Retention Days"
            className="w-24"
            value={retentionDays}
            onChange={(e) => setRetentionDays(e.target.value)}
            placeholder="Days"
          />
          <Button variant="outline" disabled={working} onClick={handlePurgeExpired}>
            <Trash2 className="mr-2 h-4 w-4" />
            Purge Expired (&gt;{retentionDays}d)
          </Button>
          <Button variant="outline" disabled={loading} onClick={() => load()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Refresh
          </Button>
        </div>
      </div>

      {data?.countMismatch && (
        <div className="flex items-center gap-2 rounded-md border border-amber-500/50 bg-amber-50 p-3 text-sm text-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
          <AlertTriangle className="h-4 w-4 shrink-0" />
          <span>
            Count mismatch detected: aggregate ({data.aggregateDeleted}) vs per-table sum ({data.tableSumDeleted}).
          </span>
        </div>
      )}

      {(missingParents || depBlockers) && (
        <Card className="border-destructive/60 bg-destructive/5">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center justify-between text-base text-destructive">
              <span>Operation Blocked ({missingParents ? 'Missing Parent Records' : 'DEPENDENCY Blocker'})</span>
              {missingParents && (
                <Button size="sm" variant="outline" onClick={copyMissingParents}>
                  <Copy className="mr-1.5 h-3.5 w-3.5" />
                  Copy Missing IDs
                </Button>
              )}
            </CardTitle>
            <CardDescription>{blockerMessage}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            {missingParents?.map((grp) => (
              <div key={grp.tableName} className="rounded border bg-background p-2">
                <span className="font-medium">{grp.tableName}:</span>{' '}
                <code className="text-xs">{grp.parentIds.join(', ')}</code>
              </div>
            ))}
            {depBlockers?.map((dep) => (
              <div key={`${dep.childTable}.${dep.foreignKey}`} className="rounded border bg-background p-2">
                <span className="font-medium">{dep.childTable}</span>.{dep.foreignKey}: {dep.totalCount} referencing row(s)
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-2">
              <Select
                value={selectedTable}
                onValueChange={(v) => {
                  setSelectedTable(v);
                  setPage(1);
                  void load(v, search, 1);
                }}
              >
                <SelectTrigger className="w-56" aria-label="Filter by Table">
                  <SelectValue placeholder="All 11 Tables" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">
                    All Tables ({data?.aggregateDeleted ?? 0})
                  </SelectItem>
                  {tables.map((t) => (
                    <SelectItem key={t} value={t}>
                      {t} ({data?.perTableCounts?.[t] ?? 0})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Input
                className="w-64"
                placeholder="Search ID, name, or invoice..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    setPage(1);
                    void load(selectedTable, search, 1);
                  }
                }}
              />
              <Button variant="secondary" onClick={() => { setPage(1); void load(selectedTable, search, 1); }}>
                Search
              </Button>
            </div>

            {selectedIds.length > 0 && (
              <div className="flex items-center gap-2">
                <Badge variant="secondary">{selectedIds.length} selected</Badge>
                <Button
                  size="sm"
                  disabled={working}
                  onClick={() => {
                    const target = resolveSingleTargetTable();
                    if (target) void handleRestore(target.table, target.ids);
                  }}
                >
                  <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                  Restore Selected
                </Button>
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={working}
                  onClick={() => {
                    const target = resolveSingleTargetTable();
                    if (target) void handlePurge(target.table, target.ids);
                  }}
                >
                  <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                  Purge Selected
                </Button>
              </div>
            )}
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-left">
                <tr>
                  <th className="w-10 p-2.5">Select</th>
                  <th className="p-2.5">Table</th>
                  <th className="p-2.5">Record</th>
                  <th className="p-2.5">Deleted At / By</th>
                  <th className="p-2.5 text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {rows.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="p-6 text-center text-muted-foreground">
                      {loading ? 'Loading recycle bin...' : 'No soft-deleted records match your filter.'}
                    </td>
                  </tr>
                ) : (
                  rows.map((r) => {
                    const checked = selectedIds.includes(r.recordId);
                    return (
                      <tr key={`${r.tableName}:${r.recordId}`} className="border-t">
                        <td className="p-2.5">
                          <input
                            type="checkbox"
                            aria-label={`Select ${r.recordId}`}
                            checked={checked}
                            onChange={() => toggleSelect(r.recordId)}
                          />
                        </td>
                        <td className="p-2.5">
                          <Badge variant="outline">{r.tableName}</Badge>
                        </td>
                        <td className="p-2.5">
                          <div className="font-medium">{r.label || r.recordId}</div>
                          <div className="text-xs text-muted-foreground">
                            {r.recordId} · {r.summary}
                          </div>
                        </td>
                        <td className="p-2.5 text-xs text-muted-foreground">
                          <div>{r.deletedAt}</div>
                          <div>by {r.deletedBy || 'system'}</div>
                        </td>
                        <td className="p-2.5 text-right">
                          <div className="flex justify-end gap-1.5">
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={working}
                              onClick={() => void handleRestore(r.tableName, [r.recordId])}
                            >
                              <Check className="mr-1 h-3.5 w-3.5" />
                              Restore
                            </Button>
                            <Button
                              size="sm"
                              variant="destructive"
                              disabled={working}
                              onClick={() => void handlePurge(r.tableName, [r.recordId])}
                            >
                              Purge
                            </Button>
                          </div>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>

          <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
            <span>
              Page {page} of {totalPages} ({data?.total ?? 0} total deleted rows)
            </span>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                disabled={page <= 1}
                onClick={() => {
                  const next = page - 1;
                  setPage(next);
                  void load(selectedTable, search, next);
                }}
              >
                Previous
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={page >= totalPages}
                onClick={() => {
                  const next = page + 1;
                  setPage(next);
                  void load(selectedTable, search, next);
                }}
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
