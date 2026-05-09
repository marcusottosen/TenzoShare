import { request, setTokens } from './client';

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
}

export interface LoginResult {
  mfa_required?: boolean;
  user_id?: string;
  access_token?: string;
  refresh_token?: string;
  expires_in?: number;
  token_type?: string;
}

export interface MeResponse {
  user_id: string;
  id?: string;
  email?: string;
  role: string;
  is_active?: boolean;
  email_verified?: boolean;
  mfa_enabled?: boolean;
  created_at?: string;
  // per-user format prefs (null = use system default)
  date_format?: string | null;
  time_format?: string | null;
  timezone?: string | null;
}

export interface APIKey {
  id: string;
  name: string;
  key?: string; // only present on creation
  key_prefix: string;
  expires_at?: string;
  created_at: string;
}

export async function login(email: string, password: string): Promise<LoginResult> {
  return request<LoginResult>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function loginMFA(userId: string, otpCode: string): Promise<TokenResponse> {
  return request<TokenResponse>('/auth/login/mfa', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId, otp_code: otpCode }),
  });
}

export async function register(email: string, password: string) {
  return request<{ id: string; email: string; role: string }>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function logout() {
  return request<void>('/auth/logout', { method: 'POST' });
}

export async function getMe(): Promise<MeResponse> {
  return request<MeResponse>('/auth/me');
}

export async function changePassword(currentPassword: string, newPassword: string) {
  return request<void>('/users/me', {
    method: 'PATCH',
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
}

export async function setupMFA() {
  return request<{ secret: string; provisioning_uri: string }>('/auth/mfa/setup', {
    method: 'POST',
  });
}

export async function verifyMFA(otpCode: string) {
  return request<{ mfa_enabled: boolean }>('/auth/mfa/verify', {
    method: 'POST',
    body: JSON.stringify({ otp_code: otpCode }),
  });
}

export async function requestPasswordReset(email: string) {
  return request<{ message: string }>('/auth/password-reset/request', {
    method: 'POST',
    body: JSON.stringify({ email }),
  });
}

export async function confirmPasswordReset(token: string, newPassword: string) {
  return request<{ message: string }>('/auth/password-reset/confirm', {
    method: 'POST',
    body: JSON.stringify({ token, new_password: newPassword }),
  });
}

export async function listAPIKeys(): Promise<{ keys: APIKey[] }> {
  return request<{ keys: APIKey[] }>('/users/apikeys');
}

export async function createAPIKey(name: string, expiresAt?: string): Promise<APIKey> {
  return request<APIKey>('/users/apikeys', {
    method: 'POST',
    body: JSON.stringify({ name, expires_at: expiresAt }),
  });
}

export async function deleteAPIKey(id: string): Promise<void> {
  return request<void>(`/users/apikeys/${id}`, { method: 'DELETE' });
}

export function storeTokens(res: TokenResponse) {
  setTokens(res.access_token, res.refresh_token);
}

export interface DatePrefsPayload {
  date_format: string | null;
  time_format: string | null;
  timezone: string | null;
}

export async function updatePreferences(prefs: DatePrefsPayload): Promise<MeResponse> {
  return request<MeResponse>('/auth/me/preferences', {
    method: 'PATCH',
    body: JSON.stringify(prefs),
  });
}
