import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from 'react';
import { getMe, type MeResponse } from '../api/auth';
import { getPlatformConfig, type PlatformConfig } from '../api/platform';
import { clearTokens } from '../api/client';
import { setActivePrefs, DEFAULT_PREFS, type DateFormat, type TimeFormat } from '../utils/dateFormat';

interface AuthState {
  user: MeResponse | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  setUser: (u: MeResponse | null) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MeResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem('admin_access_token');
    if (!token) { setIsLoading(false); return; }
    Promise.all([getMe(), getPlatformConfig().catch((): PlatformConfig => ({
        date_format: DEFAULT_PREFS.dateFormat,
        time_format: DEFAULT_PREFS.timeFormat,
        timezone: DEFAULT_PREFS.timezone,
      }))])
      .then(([me, sys]) => {
        if (me.role !== 'admin') {
          clearTokens();
          setUser(null);
        } else {
          setUser(me);
          setActivePrefs({
            dateFormat: ((me.date_format ?? sys.date_format) as DateFormat) || DEFAULT_PREFS.dateFormat,
            timeFormat: ((me.time_format ?? sys.time_format) as TimeFormat) || DEFAULT_PREFS.timeFormat,
            timezone: me.timezone ?? sys.timezone ?? DEFAULT_PREFS.timezone,
          });
        }
      })
      .catch(() => clearTokens())
      .finally(() => setIsLoading(false));
  }, []);

  const logout = useCallback(() => {
    clearTokens();
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, isLoading, setUser, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider');
  return ctx;
}
