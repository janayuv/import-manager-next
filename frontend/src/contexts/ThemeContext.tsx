import * as React from 'react';
import { useAuth } from '@/contexts/AuthContext';
import {
  applyThemeProfile,
  settingsService,
  type AppSettingsProfile,
  type ModuleDisplaySettings,
} from '@/services/dashboard-settings-service';

export type ColorMode = 'light' | 'system' | 'dark';

const STORAGE_KEY = 'im_theme_profile';
const SETTINGS_STORAGE_KEY = 'im_settings_profile';

export const DEFAULT_MODULE_SETTINGS: ModuleDisplaySettings = {
  visibleFields: [],
  itemsPerPage: 10,
  decimalPrecision: 2,
  dateFormat: 'DD/MM/YYYY',
  fieldCases: {},
};

export function getDefaultModuleSettings(moduleKey: string): ModuleDisplaySettings {
  return {
    ...DEFAULT_MODULE_SETTINGS,
    fieldCases: moduleKey === 'shipment' ? { shipmentType: 'uppercase' } : {},
  };
}

interface StoredTheme {
  theme: string;
  mode: ColorMode;
}

function normalizeMode(value: string | undefined): ColorMode {
  if (value === 'light' || value === 'dark' || value === 'system') {
    return value;
  }
  return 'system';
}

function readCachedTheme(): StoredTheme {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { theme: 'default', mode: 'system' };
    const parsed = JSON.parse(raw) as Partial<StoredTheme>;
    return {
      theme: parsed.theme === 'steel' ? 'steel' : 'default',
      mode: normalizeMode(parsed.mode),
    };
  } catch {
    return { theme: 'default', mode: 'system' };
  }
}

function writeCachedTheme(value: StoredTheme): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
  } catch {
    // Ignore storage quota errors in restricted WebViews.
  }
}

export function readCachedProfile(): AppSettingsProfile | null {
  try {
    const raw = window.localStorage.getItem(SETTINGS_STORAGE_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as AppSettingsProfile;
  } catch {
    return null;
  }
}

export function writeCachedProfile(profile: AppSettingsProfile): void {
  try {
    window.localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(profile));
  } catch {
    // Ignore storage quota errors in restricted WebViews.
  }
}

interface ThemeContextValue {
  theme: string;
  mode: ColorMode;
  profile: AppSettingsProfile | null;
  setMode: (mode: ColorMode) => Promise<void>;
  syncTheme: (next: Partial<AppSettingsProfile> | AppSettingsProfile) => void;
  getModuleSettings: (moduleKey: string) => ModuleDisplaySettings;
}

const ThemeContext = React.createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const { session, hasPermission } = useAuth();
  const initial = React.useMemo(() => readCachedTheme(), []);
  const [theme, setTheme] = React.useState<string>(initial.theme);
  const [mode, setModeState] = React.useState<ColorMode>(initial.mode);
  const [profile, setProfileState] = React.useState<AppSettingsProfile | null>(() => readCachedProfile());
  const profileRef = React.useRef<AppSettingsProfile | null>(profile);

  React.useEffect(() => {
    applyThemeProfile({ theme, mode });
    writeCachedTheme({ theme, mode });
  }, [theme, mode]);

  React.useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return;
    }
    const mql = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => {
      if (mode === 'system') {
        applyThemeProfile({ theme, mode });
      }
    };
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, [theme, mode]);

  React.useEffect(() => {
    if (!session) return;
    let active = true;
    Promise.resolve(settingsService.getProfile())
      .then((p) => {
        if (!active || !p) return;
        profileRef.current = p;
        setProfileState(p);
        writeCachedProfile(p);
        setTheme(p.theme || 'default');
        setModeState(normalizeMode(p.mode));
      })
      .catch(() => {
        // Keep cached theme preference if profile load fails.
      });
    return () => {
      active = false;
    };
  }, [session]);

  const syncTheme = React.useCallback((next: Partial<AppSettingsProfile> | AppSettingsProfile) => {
    if (next.theme) {
      setTheme(next.theme === 'steel' ? 'steel' : 'default');
    }
    if (next.mode) {
      setModeState(normalizeMode(next.mode));
    }
    setProfileState((prev) => {
      const merged = { ...(prev ?? {}), ...next } as AppSettingsProfile;
      profileRef.current = merged;
      writeCachedProfile(merged);
      return merged;
    });
  }, []);

  const getModuleSettings = React.useCallback(
    (moduleKey: string): ModuleDisplaySettings => {
      return profile?.modules?.[moduleKey] ?? getDefaultModuleSettings(moduleKey);
    },
    [profile],
  );

  const setMode = React.useCallback(
    async (nextMode: ColorMode) => {
      setModeState(nextMode);
      if (!session || !hasPermission('data.write')) {
        return;
      }
      try {
        const baseProfile = profileRef.current ?? (await settingsService.getProfile());
        if (!baseProfile) return;
        const saved = await settingsService.saveProfile({
          ...baseProfile,
          theme: theme || baseProfile.theme,
          mode: nextMode,
        });
        if (saved) {
          profileRef.current = saved;
          setProfileState(saved);
          writeCachedProfile(saved);
        }
      } catch {
        // Keep local toggle active even if backend save is unavailable.
      }
    },
    [session, hasPermission, theme],
  );

  const value = React.useMemo<ThemeContextValue>(
    () => ({ theme, mode, profile, setMode, syncTheme, getModuleSettings }),
    [theme, mode, profile, setMode, syncTheme, getModuleSettings],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useOptionalTheme(): ThemeContextValue | null {
  return React.useContext(ThemeContext);
}

export function useModuleSettings(moduleKey: string) {
  const themeCtx = useOptionalTheme();
  const [fallbackProfile, setFallbackProfile] = React.useState<AppSettingsProfile | null>(() => {
    return themeCtx?.profile ?? readCachedProfile();
  });

  React.useEffect(() => {
    if (themeCtx?.profile) return;
    let active = true;
    Promise.resolve(settingsService.getProfile())
      .then((p) => {
        if (!active || !p) return;
        setFallbackProfile(p);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [themeCtx?.profile]);

  const activeProfile = themeCtx?.profile ?? fallbackProfile;
  const settings = React.useMemo(() => {
    return activeProfile?.modules?.[moduleKey] ?? getDefaultModuleSettings(moduleKey);
  }, [activeProfile, moduleKey]);

  const itemsPerPage = Math.max(5, Math.min(200, settings.itemsPerPage || 10));

  return {
    settings: { ...settings, itemsPerPage },
    itemsPerPage,
    profile: activeProfile,
  };
}
