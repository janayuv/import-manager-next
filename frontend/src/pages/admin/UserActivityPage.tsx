import * as React from 'react';
import { Activity, RefreshCw } from 'lucide-react';
import { toast } from 'sonner';
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
import { auditService, type UserActivityEntry } from '@/services/audit-service';

export function UserActivityPage() {
  const [severity, setSeverity] = React.useState('ALL');
  const [entries, setEntries] = React.useState<UserActivityEntry[]>([]);
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const filter = severity === 'ALL' ? '' : severity;
      const rows = await auditService.queryUserActivityLogs(filter, 100);
      setEntries(rows);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [severity]);

  React.useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">User Activity Audit</h1>
          <p className="text-sm text-muted-foreground">
            Authentication, authorization, and operator actions tracked in user_activity_audit_logs.
          </p>
        </div>
        <Button variant="outline" onClick={() => void load()} disabled={loading}>
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Activity className="h-4 w-4 text-primary" />
            Activity Stream
          </CardTitle>
          <CardDescription>Filter by severity level (INFO, WARNING, SECURITY, CRITICAL).</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <div className="w-52">
              <Select value={severity} onValueChange={setSeverity}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ALL">All Severities</SelectItem>
                  <SelectItem value="INFO">INFO</SelectItem>
                  <SelectItem value="WARNING">WARNING</SelectItem>
                  <SelectItem value="SECURITY">SECURITY</SelectItem>
                  <SelectItem value="CRITICAL">CRITICAL</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Timestamp</TableHead>
                <TableHead>Severity</TableHead>
                <TableHead>User</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Entity</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="text-center text-muted-foreground">
                    No activity logs matching current severity filter.
                  </TableCell>
                </TableRow>
              ) : (
                entries.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell className="font-mono text-xs">{entry.timestamp}</TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          entry.severity === 'CRITICAL'
                            ? 'destructive'
                            : entry.severity === 'WARNING'
                              ? 'warning'
                              : 'outline'
                        }
                      >
                        {entry.severity}
                      </Badge>
                    </TableCell>
                    <TableCell>{entry.userId}</TableCell>
                    <TableCell className="font-mono text-xs">{entry.actionName}</TableCell>
                    <TableCell className="text-xs">
                      {entry.entityType}
                      {entry.entityId ? `:${entry.entityId}` : ''}
                    </TableCell>
                    <TableCell>{entry.status}</TableCell>
                    <TableCell className="max-w-xs truncate font-mono text-xs text-muted-foreground">
                      {entry.detailsJson}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
