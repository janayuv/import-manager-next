import * as React from 'react';
import { RefreshCw, ShieldCheck } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
import {
  fetchBootstrapInfo,
  listSchemaMigrations,
  type BootstrapInfoDto,
  type MigrationRecord,
} from '@/services/app-service';

export function WorkspaceOverviewPage() {
  const [info, setInfo] = React.useState<BootstrapInfoDto | null>(null);
  const [migrations, setMigrations] = React.useState<MigrationRecord[]>([]);
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [pageSize, setPageSize] = React.useState(5);
  const [currentPage, setCurrentPage] = React.useState(1);

  React.useEffect(() => {
    setCurrentPage(1);
  }, [pageSize]);

  const totalPages = Math.max(1, Math.ceil(migrations.length / pageSize));
  const safePage = Math.min(currentPage, totalPages);
  const pagedMigrations = React.useMemo(() => {
    const start = (safePage - 1) * pageSize;
    return migrations.slice(start, start + pageSize);
  }, [migrations, safePage, pageSize]);

  const load = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [boot, migs] = await Promise.all([fetchBootstrapInfo(), listSchemaMigrations()]);
      setInfo(boot);
      setMigrations(migs);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-6">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Import Manager — Next Generation ERP</h1>
          <p className="text-sm text-muted-foreground">
            Wails 3 + Go + SQLCipher Desktop Workspace Diagnostics
          </p>
        </div>
        <Button variant="outline" onClick={() => void load()} disabled={loading}>
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          Refresh Status
        </Button>
      </header>

      {error && (
        <Card className="border-destructive">
          <CardHeader>
            <CardTitle className="text-destructive">Backend Connection Error</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
        </Card>
      )}

      {info && (
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Desktop Runtime</CardTitle>
                <Badge variant="success">
                  <ShieldCheck className="mr-1 h-3 w-3" />
                  Wails 3
                </Badge>
              </div>
              <CardDescription>Go backend &amp; Windows AppData paths</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Application</span>
                <span className="font-medium">
                  {info.appName} v{info.appVersion}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Go Runtime</span>
                <span className="font-mono">{info.goVersion}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Platform</span>
                <span className="font-mono">
                  {info.os}/{info.arch}
                </span>
              </div>
              <div className="pt-2 text-xs text-muted-foreground">
                DB Path: <span className="font-mono break-all">{info.paths.databasePath}</span>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>SQLCipher Database</CardTitle>
                <Badge variant={info.database.encrypted ? 'success' : 'warning'}>
                  {info.database.encrypted ? 'Encrypted' : 'Plaintext'}
                </Badge>
              </div>
              <CardDescription>At-rest encryption and integrity pragmas</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Cipher Engine</span>
                <span className="font-mono">SQLCipher {info.database.cipherVersion}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Journal Mode</span>
                <span className="font-mono uppercase">{info.database.journalMode}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Foreign Keys</span>
                <Badge variant={info.database.foreignKeysOn ? 'success' : 'destructive'}>
                  {info.database.foreignKeysOn ? 'ENFORCED' : 'OFF'}
                </Badge>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Integrity Check</span>
                <span className="font-mono uppercase">{info.database.integrityCheck}</span>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Applied Schema Migrations</CardTitle>
          <CardDescription>
            SHA-256 verified schema migrations managed by the Go backend migration runner.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Version</TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Checksum</TableHead>
                <TableHead>Executed At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {migrations.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    No migration history loaded.
                  </TableCell>
                </TableRow>
              ) : (
                pagedMigrations.map((m) => (
                  <TableRow key={m.version}>
                    <TableCell className="font-mono">#{m.version}</TableCell>
                    <TableCell className="font-medium">{m.name}</TableCell>
                    <TableCell>
                      <Badge variant={m.status === 'APPLIED' || m.status === 'SUCCESS' ? 'success' : 'destructive'}>
                        {m.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">{m.checksum.slice(0, 16)}...</TableCell>
                    <TableCell className="font-mono text-xs">{m.executedAt}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>

          {/* Pagination Controls */}
          <div className="flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-muted-foreground pt-4 border-t">
            <div className="flex items-center gap-2">
              <span>Rows per page:</span>
              <Select
                value={String(pageSize)}
                onValueChange={(val) => {
                  setPageSize(Number(val));
                  setCurrentPage(1);
                }}
              >
                <SelectTrigger className="h-8 w-[72px] text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="5">5</SelectItem>
                  <SelectItem value="10">10</SelectItem>
                  <SelectItem value="20">20</SelectItem>
                  <SelectItem value="50">50</SelectItem>
                </SelectContent>
              </Select>
              <span>
                Showing {migrations.length === 0 ? 0 : (safePage - 1) * pageSize + 1} to{' '}
                {Math.min(safePage * pageSize, migrations.length)} of {migrations.length} migrations
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={safePage <= 1}
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
              >
                Previous
              </Button>
              <span className="text-xs font-medium">
                Page {safePage} of {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={safePage >= totalPages}
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
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
