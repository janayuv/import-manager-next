import * as React from 'react';
import { useAuth } from '@/contexts/AuthContext';
import {
  applyThemeProfile,
  settingsService,
  type AppSettingsProfile,
} from '@/services/dashboard-settings-service';

export type ColorMode = 'light' | 'system' | 'dark';

const STORAGE_KEY = 'im_theme_profile';

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

interface ThemeContextValue {
  theme: string;
  mode: ColorMode;
  setMode: (mode: ColorMode) => Promise<void>;
  syncTheme: (next: { theme?: string; mode?: string }) => void;
}

const ThemeContext = React.createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const { session, hasPermission } = useAuth();
  const initial = React.useMemo(() => readCachedTheme(), []);
  const [theme, setTheme] = React.useState<string>(initial.theme);
  const [mode, setModeState] = React.useState<ColorMode>(initial.mode);
  const profileRef = React.useRef<AppSettingsProfile | null>(null);

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
      .then((profile) => {
        if (!active || !profile) return;
        profileRef.current = profile;
        setTheme(profile.theme || 'default');
        setModeState(normalizeMode(profile.mode));
      })
      .catch(() => {
        // Keep cached theme preference if profile load fails.
      });
    return () => {
      active = false;
    };
  }, [session]);

  const syncTheme = React.useCallback((next: { theme?: string; mode?: string }) => {
    if (next.theme) {
      setTheme(next.theme === 'steel' ? 'steel' : 'default');
    }
    if (next.mode) {
      setModeState(normalizeMode(next.mode));
    }
  }, []);

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
        }
      } catch {
        // Keep local toggle active even if backend save is unavailable.
      }
    },
    [session, hasPermission, theme],
  );

  const value = React.useMemo<ThemeContextValue>(
    () => ({ theme, mode, setMode, syncTheme }),
    [theme, mode, setMode, syncTheme],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useOptionalTheme(): ThemeContextValue | null {
  return React.useContext(ThemeContext);
}
