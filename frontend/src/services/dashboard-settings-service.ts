import * as DashboardBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/dashboard/service.js';
import * as SettingsBindings from '../../bindings/github.com/janayuv/import-manager-next/backend/services/settings/service.js';
import type {
  AppSettingsProfile,
  DashboardActivityItem,
  DashboardAlertItem,
  DashboardFilter,
  DashboardKPIs,
  DashboardMonthlyTrend,
  DashboardOverview,
  DashboardStatusCount,
  DashboardSupplierSummary,
  ModuleDisplaySettings,
} from '../../bindings/github.com/janayuv/import-manager-next/backend/models/models.js';

export type {
  AppSettingsProfile,
  DashboardActivityItem,
  DashboardAlertItem,
  DashboardFilter,
  DashboardKPIs,
  DashboardMonthlyTrend,
  DashboardOverview,
  DashboardStatusCount,
  DashboardSupplierSummary,
  ModuleDisplaySettings,
};

export const dashboardService = {
  getOverview(filter: DashboardFilter): Promise<DashboardOverview> {
    return DashboardBindings.GetOverview(filter);
  },
  invalidateCache(): Promise<void> {
    return DashboardBindings.InvalidateCache();
  },
  rebuildDailySnapshot(): Promise<string> {
    return DashboardBindings.RebuildDailySnapshot();
  },
};

export const settingsService = {
  getProfile(): Promise<AppSettingsProfile> {
    return SettingsBindings.GetProfile();
  },
  saveProfile(profile: AppSettingsProfile): Promise<AppSettingsProfile> {
    return SettingsBindings.SaveProfile(profile);
  },
  resetModuleDefaults(moduleKey: string): Promise<AppSettingsProfile> {
    return SettingsBindings.ResetModuleDefaults(moduleKey);
  },
};

export function applyThemeProfile(profile: Pick<AppSettingsProfile, 'theme' | 'mode'>): void {
  const root = document.documentElement;
  root.setAttribute('data-theme', profile.theme || 'default');
  const prefersDark =
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-color-scheme: dark)').matches;
  const dark = profile.mode === 'dark' || (profile.mode === 'system' && prefersDark);
  root.classList.toggle('dark', dark);
}
