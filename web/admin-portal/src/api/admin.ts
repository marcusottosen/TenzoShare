import { request } from './client';

export interface AdminUser {
  id: string;
  email: string;
  role: string;
  is_active: boolean;
  email_verified: boolean;
  failed_login_attempts: number;
  locked_until: string | null;
  created_at: string;
  updated_at: string;
}

export interface UsersListResponse {
  users: AdminUser[];
  total: number;
  limit: number;
  offset: number;
}

export interface SystemStats {
  total_users: number;
  new_users_30d: number;
  total_transfers: number;
  total_files: number;
  total_storage_bytes: number;
}

export interface ServiceHealth {
  name: string;
  status: 'up' | 'down';
  latency_ms: number;
}

export interface SystemHealthResponse {
  services: ServiceHealth[];
}

export async function listUsers(params: {
  limit?: number;
  offset?: number;
  search?: string;
  role?: string;
} = {}): Promise<UsersListResponse> {
  const p = new URLSearchParams();
  if (params.limit) p.set('limit', String(params.limit));
  if (params.offset) p.set('offset', String(params.offset));
  if (params.search) p.set('search', params.search);
  if (params.role) p.set('role', params.role);
  return request<UsersListResponse>(`/admin/users?${p}`);
}

export async function createUser(body: { email: string; password: string; role: string }): Promise<AdminUser> {
  return request<AdminUser>('/admin/users', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function updateUser(id: string, body: { role?: string; is_active?: boolean }) {
  return request<{ ok: boolean }>(`/admin/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

export async function deleteUser(id: string) {
  return request<{ ok: boolean }>(`/admin/users/${id}`, { method: 'DELETE' });
}

export async function unlockUser(id: string) {
  return request<{ ok: boolean }>(`/admin/users/${id}/unlock`, { method: 'POST' });
}

export async function verifyUserEmail(id: string) {
  return request<{ ok: boolean }>(`/admin/users/${id}/verify`, { method: 'POST' });
}

export async function getStats(): Promise<SystemStats> {
  return request<SystemStats>('/admin/stats');
}

export async function getSystemHealth(): Promise<SystemHealthResponse> {
  return request<SystemHealthResponse>('/admin/system/health');
}
