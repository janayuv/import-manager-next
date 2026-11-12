import * as AuditBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/audit/service.js';
import type {
  AuditEntry,
  UserActivityEntry,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/services/audit/models.js';

export type { AuditEntry, UserActivityEntry };

export const auditService = {
  queryAuditLogs(tableName: string, action: string, limit: number): Promise<AuditEntry[]> {
    return AuditBindings.QueryAuditLogs(tableName, action, limit).then((items) => items ?? []);
  },
  queryUserActivityLogs(severity: string, limit: number): Promise<UserActivityEntry[]> {
    return AuditBindings.QueryUserActivityLogs(severity, limit).then((items) => items ?? []);
  },
};
