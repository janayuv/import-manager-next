import * as AppBindings from '../../bindings/github.com/janayuv/import-manager-next/appservice.js';
import type { BootstrapInfo } from '../../bindings/github.com/janayuv/import-manager-next/models.js';
import type { HealthInfo } from '../../bindings/github.com/janayuv/import-manager-next/backend/database/connection/models.js';
import type { AppliedRecord } from '../../bindings/github.com/janayuv/import-manager-next/backend/database/migrations/models.js';

export type BootstrapInfoDto = BootstrapInfo;
export type MigrationRecord = AppliedRecord;
export type { HealthInfo };

export function fetchBootstrapInfo(): Promise<BootstrapInfo> {
  return AppBindings.GetBootstrapInfo();
}

export function checkDatabaseHealth(): Promise<HealthInfo> {
  return AppBindings.CheckDatabaseHealth();
}

export function listSchemaMigrations(): Promise<AppliedRecord[]> {
  return AppBindings.ListSchemaMigrations().then((items) => items ?? []);
}
