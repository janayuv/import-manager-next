import * as React from 'react';
import { Check, KeyRound, Plus, RefreshCw } from 'lucide-react';
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
import {
  securityService,
  type RoleMatrixRow,
  type UserRoleRow,
} from '@/services/security-service';

const CANONICAL_ROLES = ['administrator', 'manager', 'operator', 'viewer'];

export function RolesPermissionsPage() {
  const [matrix, setMatrix] = React.useState<RoleMatrixRow[]>([]);
  const [userRoles, setUserRoles] = React.useState<UserRoleRow[]>([]);
  const [newUserId, setNewUserId] = React.useState('');
  const [newRole, setNewRole] = React.useState('administrator');
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const [m, u] = await Promise.all([
        securityService.getRolePermissionsMatrix(),
        securityService.listUserRoles(),
      ]);
      setMatrix(m);
      setUserRoles(u);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const allPermissions = React.useMemo(() => {
    const set = new Set<string>();
    for (const row of matrix) {
      for (const p of row.permissions ?? []) {
        set.add(p);
      }
    }
    return Array.from(set).sort();
  }, [matrix]);

  const handleAssignRole = async (userId: string, role: string) => {
    try {
      await securityService.assignUserRole(userId, role);
      toast.success(`Assigned ${role} to ${userId}`);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const handleAddUserRole = async (e: React.FormEvent) => {
    e.preventDefault();
    const uid = newUserId.trim();
    if (!uid) return;
    await handleAssignRole(uid, newRole);
    setNewUserId('');
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Roles &amp; Permissions</h1>
          <p className="text-sm text-muted-foreground">
            Backend-enforced RBAC policy matrix (4 canonical roles × {allPermissions.length} capabilities).
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
            <KeyRound className="h-4 w-4 text-primary" />
            User Role Assignments
          </CardTitle>
          <CardDescription>
            Every Wails service call verifies permissions against the active server-side session.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <form onSubmit={(e) => void handleAddUserRole(e)} className="flex flex-wrap items-end gap-3">
            <div className="w-56 space-y-1">
              <Label htmlFor="assign-user-id">User ID</Label>
              <Input
                id="assign-user-id"
                placeholder="e.g. admin"
                value={newUserId}
                onChange={(e) => setNewUserId(e.target.value)}
              />
            </div>
            <div className="w-48 space-y-1">
              <Label>Role</Label>
              <Select value={newRole} onValueChange={setNewRole}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {CANONICAL_ROLES.map((r) => (
                    <SelectItem key={r} value={r}>
                      {r}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button type="submit">
              <Plus className="h-4 w-4" />
              Upsert Assignment
            </Button>
          </form>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User ID</TableHead>
                <TableHead>Canonical Role</TableHead>
                <TableHead>Updated At</TableHead>
                <TableHead>Change Role</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {userRoles.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-muted-foreground">
                    No user role rows found.
                  </TableCell>
                </TableRow>
              ) : (
                userRoles.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className="font-medium">{row.userId}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{row.role}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">{row.updatedAt}</TableCell>
                    <TableCell className="w-52">
                      <Select
                        value={row.role}
                        onValueChange={(nextRole) => void handleAssignRole(row.userId, nextRole)}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {CANONICAL_ROLES.map((r) => (
                            <SelectItem key={r} value={r}>
                              {r}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Canonical Capability Matrix</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Permission</TableHead>
                {matrix.map((r) => (
                  <TableHead key={r.role} className="text-center">
                    {r.role}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {allPermissions.map((perm) => (
                <TableRow key={perm}>
                  <TableCell className="font-mono text-xs">{perm}</TableCell>
                  {matrix.map((r) => {
                    const granted = (r.permissions ?? []).includes(perm);
                    return (
                      <TableCell key={r.role} className="text-center">
                        {granted ? (
                          <Check className="mx-auto h-4 w-4 text-emerald-600" aria-label="Granted" />
                        ) : (
                          <span className="text-xs text-muted-foreground">-</span>
                        )}
                      </TableCell>
                    );
                  })}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
