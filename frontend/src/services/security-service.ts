import * as SecurityBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/security/service.js';
import type {
  AuthSetupStatus,
  LockoutStatus,
  RoleMatrixRow,
  SecurityDashboardMetrics,
  SecuritySettings,
  SecurityTrendPoint,
  SessionInfo,
  UserRoleRow,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/services/security/models.js';
import type { UserActivityEntry } from '../../bindings/github.com/janayuv/import-manager-next/backend/services/audit/models.js';

export type {
  AuthSetupStatus,
  LockoutStatus,
  RoleMatrixRow,
  SecurityDashboardMetrics,
  SecuritySettings,
  SecurityTrendPoint,
  SessionInfo,
  UserRoleRow,
};

export const securityService = {
  getAuthSetupStatus(): Promise<AuthSetupStatus> {
    return SecurityBindings.GetAuthSetupStatus();
  },
  completeAdminSetup(username: string, password: string): Promise<SessionInfo> {
    return SecurityBindings.CompleteAdminSetup(username, password);
  },
  authenticate(username: string, password: string, rememberMe: boolean): Promise<SessionInfo> {
    return SecurityBindings.Authenticate(username, password, rememberMe);
  },
  getMySession(): Promise<SessionInfo> {
    return SecurityBindings.GetMySession();
  },
  getMyPermissions(): Promise<string[]> {
    return SecurityBindings.GetMyPermissions().then((items) => items ?? []);
  },
  logout(): Promise<void> {
    return SecurityBindings.Logout();
  },
  changePassword(currentPassword: string, newPassword: string): Promise<void> {
    return SecurityBindings.ChangePassword(currentPassword, newPassword);
  },
  getSecuritySettings(): Promise<SecuritySettings> {
    return SecurityBindings.GetSecuritySettings();
  },
  updateSecuritySettings(next: SecuritySettings): Promise<SecuritySettings> {
    return SecurityBindings.UpdateSecuritySettings(next);
  },
  getLockoutStatus(userId: string): Promise<LockoutStatus> {
    return SecurityBindings.GetLockoutStatus(userId);
  },
  resetAccountLockout(userId: string): Promise<void> {
    return SecurityBindings.ResetAccountLockout(userId);
  },
  getActiveSessions(): Promise<SessionInfo[]> {
    return SecurityBindings.GetActiveSessions().then((items) => items ?? []);
  },
  terminateSessionForAdmin(sessionId: string): Promise<void> {
    return SecurityBindings.TerminateSessionForAdmin(sessionId);
  },
  getRolePermissionsMatrix(): Promise<RoleMatrixRow[]> {
    return SecurityBindings.GetRolePermissionsMatrix().then((items) => items ?? []);
  },
  listUserRoles(): Promise<UserRoleRow[]> {
    return SecurityBindings.ListUserRoles().then((items) => items ?? []);
  },
  assignUserRole(userId: string, role: string): Promise<void> {
    return SecurityBindings.AssignUserRole(userId, role);
  },
  bootstrapFirstAdminIfEligible(userId: string): Promise<boolean> {
    return SecurityBindings.BootstrapFirstAdminIfEligible(userId);
  },
  getSecurityDashboardMetrics(): Promise<SecurityDashboardMetrics> {
    return SecurityBindings.GetSecurityDashboardMetrics();
  },
  getSecurityTrendSeries(days: number): Promise<SecurityTrendPoint[]> {
    return SecurityBindings.GetSecurityTrendSeries(days).then((items) => items ?? []);
  },
  getRecentAuthEvents(limit: number): Promise<UserActivityEntry[]> {
    return SecurityBindings.GetRecentAuthEvents(limit).then((items) => items ?? []);
  },
  isRecoveryModeActive(): Promise<boolean> {
    return SecurityBindings.IsRecoveryModeActive();
  },
  recoveryClearLockout(userId: string): Promise<void> {
    return SecurityBindings.RecoveryClearLockout(userId);
  },
  recoveryResetSecurityPolicy(): Promise<void> {
    return SecurityBindings.RecoveryResetSecurityPolicy();
  },
  recoverySetAdminPassword(newPassword: string): Promise<void> {
    return SecurityBindings.RecoverySetAdminPassword(newPassword);
  },
};
