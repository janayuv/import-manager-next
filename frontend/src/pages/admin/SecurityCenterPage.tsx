import * as React from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { RefreshCw, Shield, Unlock, UserCheck } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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
import { useAuth } from '@/contexts/AuthContext';
import {
  securityService,
  type LockoutStatus,
  type SecurityDashboardMetrics,
  type SecuritySettings,
  type SecurityTrendPoint,
  type SessionInfo,
} from '@/services/security-service';
import type { UserActivityEntry } from '@/services/audit-service';

const policySchema = z.object({
  idleTimeoutMinutes: z.coerce.number().int().min(1).max(1440),
  lockoutThreshold: z.coerce.number().int().min(1).max(20),
  lockoutWindowMinutes: z.coerce.number().int().min(1).max(1440),
  lockoutDurationMinutes: z.coerce.number().int().min(1).max(1440),
  passwordHistoryDepth: z.coerce.number().int().min(1).max(24),
});

const changePasswordSchema = z
  .object({
    currentPassword: z.string().min(1, 'Current password is required'),
    newPassword: z
      .string()
      .min(8, 'New password must be at least 8 characters')
      .regex(/[A-Z]/, 'Include at least one uppercase letter')
      .regex(/[a-z]/, 'Include at least one lowercase letter')
      .regex(/[0-9]/, 'Include at least one digit')
      .regex(/[^A-Za-z0-9]/, 'Include at least one special character'),
    confirmPassword: z.string(),
  })
  .refine((v) => v.newPassword === v.confirmPassword, {
    message: 'New passwords do not match',
    path: ['confirmPassword'],
  });

type PolicyFormValues = z.infer<typeof policySchema>;
type ChangePasswordValues = z.infer<typeof changePasswordSchema>;

export function SecurityCenterPage() {
  const { session } = useAuth();
  const [metrics, setMetrics] = React.useState<SecurityDashboardMetrics | null>(null);
  const [trends, setTrends] = React.useState<SecurityTrendPoint[]>([]);
  const [sessions, setSessions] = React.useState<SessionInfo[]>([]);
  const [lockout, setLockout] = React.useState<LockoutStatus | null>(null);
  const [events, setEvents] = React.useState<UserActivityEntry[]>([]);
  const [eventsPageSize, setEventsPageSize] = React.useState(5);
  const [eventsCurrentPage, setEventsCurrentPage] = React.useState(1);
  const [policyVersion, setPolicyVersion] = React.useState<number>(1);
  const [loading, setLoading] = React.useState(true);

  React.useEffect(() => {
    setEventsCurrentPage(1);
  }, [eventsPageSize]);

  const totalEventsPages = Math.max(1, Math.ceil(events.length / eventsPageSize));
  const safeEventsPage = Math.min(eventsCurrentPage, totalEventsPages);
  const pagedEvents = React.useMemo(() => {
    const start = (safeEventsPage - 1) * eventsPageSize;
    return events.slice(start, start + eventsPageSize);
  }, [events, safeEventsPage, eventsPageSize]);

  const policyForm = useForm<PolicyFormValues>({
    resolver: zodResolver(policySchema),
    defaultValues: {
      idleTimeoutMinutes: 30,
      lockoutThreshold: 5,
      lockoutWindowMinutes: 15,
      lockoutDurationMinutes: 15,
      passwordHistoryDepth: 5,
    },
  });

  const pwForm = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { currentPassword: '', newPassword: '', confirmPassword: '' },
  });

  const loadAll = React.useCallback(async () => {
    setLoading(true);
    try {
      const targetUser = session?.username ?? 'admin';
      const [m, t, s, activeSess, lockStatus, authEvents] = await Promise.all([
        securityService.getSecurityDashboardMetrics(),
        securityService.getSecurityTrendSeries(14),
        securityService.getSecuritySettings(),
        securityService.getActiveSessions(),
        securityService.getLockoutStatus(targetUser),
        securityService.getRecentAuthEvents(50),
      ]);
      setMetrics(m);
      setTrends(t);
      setSessions(activeSess);
      setLockout(lockStatus);
      setEvents(authEvents);
      setPolicyVersion(s.policyVersion);
      policyForm.reset({
        idleTimeoutMinutes: s.idleTimeoutMinutes,
        lockoutThreshold: s.lockoutThreshold,
        lockoutWindowMinutes: s.lockoutWindowMinutes,
        lockoutDurationMinutes: s.lockoutDurationMinutes,
        passwordHistoryDepth: s.passwordHistoryDepth,
      });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [session?.username, policyForm]);

  React.useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const onSavePolicy = async (values: PolicyFormValues) => {
    try {
      const payload: SecuritySettings = {
        ...values,
        policyVersion,
      };
      const updated = await securityService.updateSecuritySettings(payload);
      setPolicyVersion(updated.policyVersion);
      toast.success(`Security policy updated (v${updated.policyVersion})`);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const onChangePassword = async (values: ChangePasswordValues) => {
    try {
      await securityService.changePassword(values.currentPassword, values.newPassword);
      pwForm.reset();
      toast.success('Administrator password updated');
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleResetLockout = async (userId: string) => {
    try {
      await securityService.resetAccountLockout(userId);
      toast.success(`Cleared lockout for ${userId}`);
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleTerminateSession = async (sessionId: string) => {
    try {
      await securityService.terminateSessionForAdmin(sessionId);
      toast.success('Session terminated');
      await loadAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Security Center</h1>
          <p className="text-sm text-muted-foreground">
            Authentication telemetry, Argon2id credentials, lockout controls, and active desktop sessions.
          </p>
        </div>
        <Button variant="outline" onClick={() => void loadAll()} disabled={loading}>
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-5">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Sessions</CardDescription>
            <CardTitle className="text-2xl">{metrics?.activeSessions ?? 0}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Locked Accounts</CardDescription>
            <CardTitle className="text-2xl">{metrics?.lockedAccounts ?? 0}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Failed Logins (24h)</CardDescription>
            <CardTitle className="text-2xl">{metrics?.failedLogins24h ?? 0}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Users With Roles</CardDescription>
            <CardTitle className="text-2xl">{metrics?.totalUsersWithRoles ?? 0}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Policy Version</CardDescription>
            <CardTitle className="text-2xl">v{metrics?.policyVersion ?? policyVersion}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Shield className="h-4 w-4 text-primary" />
              Authentication &amp; Lockout Trend (14 Days)
            </CardTitle>
          </CardHeader>
          <CardContent className="h-64">
            {trends.length === 0 ? (
              <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                No authentication events recorded in the last 14 days.
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={trends}>
                  <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                  <XAxis dataKey="date" tick={{ fontSize: 11 }} />
                  <YAxis allowDecimals={false} tick={{ fontSize: 11 }} />
                  <Tooltip />
                  <Bar dataKey="authEvents" name="Auth Events" fill="#2563eb" radius={[4, 4, 0, 0]} />
                  <Bar dataKey="failedAttempts" name="Failed Attempts" fill="#dc2626" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Security &amp; Lockout Policy</CardTitle>
            <CardDescription>
              Updates increment security policy version ({policyVersion}) and write to the audit trail.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={policyForm.handleSubmit(onSavePolicy)} className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label htmlFor="idleTimeoutMinutes">Idle Timeout (mins)</Label>
                <Input id="idleTimeoutMinutes" type="number" {...policyForm.register('idleTimeoutMinutes')} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="lockoutThreshold">Lockout Threshold</Label>
                <Input id="lockoutThreshold" type="number" {...policyForm.register('lockoutThreshold')} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="lockoutWindowMinutes">Attempt Window (mins)</Label>
                <Input id="lockoutWindowMinutes" type="number" {...policyForm.register('lockoutWindowMinutes')} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="lockoutDurationMinutes">Lockout Duration (mins)</Label>
                <Input id="lockoutDurationMinutes" type="number" {...policyForm.register('lockoutDurationMinutes')} />
              </div>
              <div className="col-span-2 space-y-1">
                <Label htmlFor="passwordHistoryDepth">Password History Depth (1-24)</Label>
                <Input id="passwordHistoryDepth" type="number" {...policyForm.register('passwordHistoryDepth')} />
              </div>
              <div className="col-span-2 flex justify-end">
                <Button type="submit" disabled={policyForm.formState.isSubmitting}>
                  Save Security Policy
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Change Administrator Password</CardTitle>
            <CardDescription>
              Enforces complexity rules and prevents reuse of recent historical passwords.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={pwForm.handleSubmit(onChangePassword)} className="space-y-3">
              <div className="space-y-1">
                <Label htmlFor="currentPassword">Current Password</Label>
                <Input id="currentPassword" type="password" {...pwForm.register('currentPassword')} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="newPassword">New Password</Label>
                <Input id="newPassword" type="password" {...pwForm.register('newPassword')} />
                {pwForm.formState.errors.newPassword && (
                  <p className="text-xs text-destructive">{pwForm.formState.errors.newPassword.message}</p>
                )}
              </div>
              <div className="space-y-1">
                <Label htmlFor="confirmPassword">Confirm New Password</Label>
                <Input id="confirmPassword" type="password" {...pwForm.register('confirmPassword')} />
                {pwForm.formState.errors.confirmPassword && (
                  <p className="text-xs text-destructive">{pwForm.formState.errors.confirmPassword.message}</p>
                )}
              </div>
              <Button type="submit" disabled={pwForm.formState.isSubmitting}>
                Update Password
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center justify-between text-base">
              <span>Account Lockout Status</span>
              {lockout && (
                <Badge variant={lockout.isLocked ? 'destructive' : 'success'}>
                  {lockout.isLocked ? `Locked (${lockout.remainingSecs}s)` : 'Unlocked'}
                </Badge>
              )}
            </CardTitle>
            <CardDescription>
              User: {lockout?.userId ?? session?.username ?? 'admin'} • Failures in window: {lockout?.failureCount ?? 0} / {lockout?.threshold ?? 5}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {lockout?.isLocked && (
              <Button
                variant="outline"
                onClick={() => void handleResetLockout(lockout.userId)}
              >
                <Unlock className="h-4 w-4" />
                Reset Account Lockout
              </Button>
            )}
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                Active Desktop Sessions
              </p>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>User</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Expires</TableHead>
                    <TableHead className="text-right">Action</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sessions.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className="text-center text-muted-foreground">
                        No active sessions
                      </TableCell>
                    </TableRow>
                  ) : (
                    sessions.map((s) => (
                      <TableRow key={s.sessionId}>
                        <TableCell className="font-medium">
                          <span className="inline-flex items-center gap-1.5">
                            <UserCheck className="h-3.5 w-3.5 text-emerald-600" />
                            {s.username}
                          </span>
                        </TableCell>
                        <TableCell>{s.role}</TableCell>
                        <TableCell className="font-mono text-xs">{s.expiresAt.slice(0, 19)}</TableCell>
                        <TableCell className="text-right">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => void handleTerminateSession(s.sessionId)}
                          >
                            Revoke
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Recent Security &amp; Authentication Events</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Timestamp</TableHead>
                <TableHead>Severity</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>User</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {events.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    No authentication activity logged yet.
                  </TableCell>
                </TableRow>
              ) : (
                pagedEvents.map((ev) => (
                  <TableRow key={ev.id}>
                    <TableCell className="font-mono text-xs">{ev.timestamp}</TableCell>
                    <TableCell>
                      <Badge variant={ev.severity === 'CRITICAL' || ev.severity === 'WARNING' ? 'warning' : 'outline'}>
                        {ev.severity}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">{ev.actionName}</TableCell>
                    <TableCell>{ev.userId}</TableCell>
                    <TableCell>{ev.status}</TableCell>
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
                value={String(eventsPageSize)}
                onValueChange={(val) => {
                  setEventsPageSize(Number(val));
                  setEventsCurrentPage(1);
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
                Showing {events.length === 0 ? 0 : (safeEventsPage - 1) * eventsPageSize + 1} to{' '}
                {Math.min(safeEventsPage * eventsPageSize, events.length)} of {events.length} events
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={safeEventsPage <= 1}
                onClick={() => setEventsCurrentPage((p) => Math.max(1, p - 1))}
              >
                Previous
              </Button>
              <span className="text-xs font-medium">
                Page {safeEventsPage} of {totalEventsPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={safeEventsPage >= totalEventsPages}
                onClick={() => setEventsCurrentPage((p) => Math.min(totalEventsPages, p + 1))}
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
