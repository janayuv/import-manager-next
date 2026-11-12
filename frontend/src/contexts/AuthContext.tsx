import * as React from 'react';
import {
  securityService,
  type AuthSetupStatus,
  type SessionInfo,
} from '@/services/security-service';

interface AuthContextValue {
  loading: boolean;
  setupStatus: AuthSetupStatus | null;
  session: SessionInfo | null;
  permissions: string[];
  hasPermission: (perm: string) => boolean;
  refreshAuth: () => Promise<void>;
  login: (username: string, password: string, rememberMe: boolean) => Promise<SessionInfo>;
  completeSetup: (username: string, password: string) => Promise<SessionInfo>;
  logout: () => Promise<void>;
}

const AuthContext = React.createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [loading, setLoading] = React.useState(true);
  const [setupStatus, setSetupStatus] = React.useState<AuthSetupStatus | null>(null);
  const [session, setSession] = React.useState<SessionInfo | null>(null);
  const [permissions, setPermissions] = React.useState<string[]>([]);

  const refreshAuth = React.useCallback(async () => {
    setLoading(true);
    try {
      const status = await securityService.getAuthSetupStatus();
      setSetupStatus(status);
      if (status.needsSetup) {
        setSession(null);
        setPermissions([]);
        return;
      }
      try {
        const current = await securityService.getMySession();
        setSession(current);
        setPermissions(current.permissions ?? []);
      } catch {
        setSession(null);
        setPermissions([]);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refreshAuth();
  }, [refreshAuth]);

  const login = React.useCallback(
    async (username: string, password: string, rememberMe: boolean): Promise<SessionInfo> => {
      const nextSession = await securityService.authenticate(username, password, rememberMe);
      setSession(nextSession);
      setPermissions(nextSession.permissions ?? []);
      setSetupStatus((prev) =>
        prev ? { ...prev, needsSetup: false, adminUsername: nextSession.username } : prev,
      );
      return nextSession;
    },
    [],
  );

  const completeSetup = React.useCallback(
    async (username: string, password: string): Promise<SessionInfo> => {
      const nextSession = await securityService.completeAdminSetup(username, password);
      setSession(nextSession);
      setPermissions(nextSession.permissions ?? []);
      setSetupStatus({
        needsSetup: false,
        adminUsername: nextSession.username,
        recoveryMode: setupStatus?.recoveryMode ?? false,
      });
      return nextSession;
    },
    [setupStatus?.recoveryMode],
  );

  const logout = React.useCallback(async () => {
    await securityService.logout();
    setSession(null);
    setPermissions([]);
  }, []);

  const hasPermission = React.useCallback(
    (perm: string) => permissions.includes(perm),
    [permissions],
  );

  const value = React.useMemo<AuthContextValue>(
    () => ({
      loading,
      setupStatus,
      session,
      permissions,
      hasPermission,
      refreshAuth,
      login,
      completeSetup,
      logout,
    }),
    [loading, setupStatus, session, permissions, hasPermission, refreshAuth, login, completeSetup, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = React.useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used inside AuthProvider');
  }
  return ctx;
}
