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
}

export interface MeResponse {
  user_id: string;
  email: string;
  role: string;
  created_at?: string;
  mfa_enabled?: boolean;
  // per-user format prefs (null = use system default)
  date_format?: string | null;
  time_format?: string | null;
  timezone?: string | null;
}

export async function login(email: string, password: string): Promise<LoginResult> {
  return request<LoginResult>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function getMe(): Promise<MeResponse> {
  return request<MeResponse>('/auth/me');
}

export async function loginMFA(userId: string, otpCode: string): Promise<TokenResponse> {
  return request<TokenResponse>('/auth/login/mfa', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId, otp_code: otpCode }),
  });
}

export async function logout() {
  return request<void>('/auth/logout', { method: 'POST' });
}

export function storeTokens(res: TokenResponse) {
  setTokens(res.access_token, res.refresh_token);
}

export interface APIKey {
  id: string;
  name: string;
  key_prefix: string;
  created_at: string;
  expires_at: string | null;
  key?: string; // only present on creation
}

export interface APIKeysListResponse {
  keys: APIKey[];
}

export async function listAPIKeys(): Promise<APIKeysListResponse> {
  return request<APIKeysListResponse>('/users/apikeys');
}

export async function createAPIKey(name: string): Promise<APIKey> {
  return request<APIKey>('/users/apikeys', {
    method: 'POST',
    body: JSON.stringify({ name }),
  });
}

export async function deleteAPIKey(id: string): Promise<void> {
  return request<void>(`/users/apikeys/${id}`, { method: 'DELETE' });
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  return request<void>('/users/me', {
    method: 'PATCH',
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
}

export async function setupMFA(): Promise<{ secret: string; provisioning_uri: string }> {
  return request<{ secret: string; provisioning_uri: string }>('/auth/mfa/setup', {
    method: 'POST',
  });
}

export interface VerifyMFAResult {
  mfa_enabled: boolean;
  access_token?: string;
  refresh_token?: string;
  expires_in?: number;
  token_type?: string;
}

export async function verifyMFA(otpCode: string): Promise<VerifyMFAResult> {
  return request<VerifyMFAResult>('/auth/mfa/verify', {
    method: 'POST',
    body: JSON.stringify({ otp_code: otpCode }),
  });
}

export async function disableMFA(otpCode: string): Promise<{ mfa_enabled: boolean }> {
  return request<{ mfa_enabled: boolean }>('/auth/mfa/disable', {
    method: 'POST',
    body: JSON.stringify({ otp_code: otpCode }),
  });
}
