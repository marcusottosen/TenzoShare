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
  role: string;
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

export async function logout() {
  return request<void>('/auth/logout', { method: 'POST' });
}

export function storeTokens(res: TokenResponse) {
  setTokens(res.access_token, res.refresh_token);
}
