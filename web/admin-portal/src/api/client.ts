const API_BASE = '/api/v1';

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

export function setTokens(access: string, refresh: string): void {
  localStorage.setItem('admin_access_token', access);
  localStorage.setItem('admin_refresh_token', refresh);
}

export function clearTokens(): void {
  localStorage.removeItem('admin_access_token');
  localStorage.removeItem('admin_refresh_token');
}

export function getToken(): string | null {
  return localStorage.getItem('admin_access_token');
}

let refreshPromise: Promise<string | null> | null = null;

async function doRefresh(): Promise<string | null> {
  const refresh = localStorage.getItem('admin_refresh_token');
  if (!refresh) return null;
  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refresh }),
    });
    if (!res.ok) { clearTokens(); return null; }
    const data = await res.json();
    setTokens(data.access_token, data.refresh_token);
    return data.access_token;
  } catch {
    clearTokens();
    return null;
  }
}

async function refreshToken(): Promise<string | null> {
  if (!refreshPromise) {
    refreshPromise = doRefresh().finally(() => { refreshPromise = null; });
  }
  return refreshPromise;
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  let res = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (res.status === 401 && token) {
    const newToken = await refreshToken();
    if (newToken) {
      headers['Authorization'] = `Bearer ${newToken}`;
      res = await fetch(`${API_BASE}${path}`, { ...options, headers });
    } else {
      throw new ApiError(401, 'Session expired. Please log in again.');
    }
  }

  if (res.status === 204) return undefined as T;
  const body = await res.json().catch(() => ({ message: `HTTP ${res.status}` }));
  if (!res.ok) throw new ApiError(res.status, body.message ?? `HTTP ${res.status}`);
  return body as T;
}
