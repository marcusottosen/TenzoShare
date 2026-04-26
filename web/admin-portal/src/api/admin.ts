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

export interface DayStat { day: string; count: number; }
export interface StorageDayStat { day: string; bytes: number; }
export interface TransferBreakdown { active: number; exhausted: number; expired: number; revoked: number; }

export interface SystemStats {
  total_users: number;
  new_users_30d: number;
  total_transfers: number;
  total_files: number;
  total_storage_bytes: number;
  transfers_per_day: DayStat[];
  users_per_day: DayStat[];
  storage_per_day: StorageDayStat[];
  transfer_breakdown: TransferBreakdown;
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
  sort_by?: string;
  sort_dir?: string;
} = {}): Promise<UsersListResponse> {
  const p = new URLSearchParams();
  if (params.limit) p.set('limit', String(params.limit));
  if (params.offset) p.set('offset', String(params.offset));
  if (params.search) p.set('search', params.search);
  if (params.role) p.set('role', params.role);
  if (params.sort_by) p.set('sort_by', params.sort_by);
  if (params.sort_dir) p.set('sort_dir', params.sort_dir);
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

export interface AdminTransfer {
  id: string;
  owner_email: string;
  name: string;
  description: string;
  recipient_email: string;
  slug: string;
  is_revoked: boolean;
  has_password: boolean;
  expires_at: string | null;
  download_count: number;
  max_downloads: number | null;
  file_count: number;
  created_at: string;
  status: 'active' | 'exhausted' | 'expired' | 'revoked';
}

export interface TransferFile {
  file_id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
}

export interface AdminTransferDetail extends AdminTransfer {
  files: TransferFile[];
}

export interface TransfersListResponse {
  transfers: AdminTransfer[];
  total: number;
}

export async function listTransfers(params?: {
  limit?: number;
  offset?: number;
  status?: 'all' | 'active' | 'exhausted' | 'expired' | 'revoked';
  sort_by?: string;
  sort_dir?: string;
}): Promise<TransfersListResponse> {
  const qs = new URLSearchParams();
  if (params?.limit) qs.set('limit', String(params.limit));
  if (params?.offset) qs.set('offset', String(params.offset));
  if (params?.status) qs.set('status', params.status);
  if (params?.sort_by) qs.set('sort_by', params.sort_by);
  if (params?.sort_dir) qs.set('sort_dir', params.sort_dir);
  return request<TransfersListResponse>(`/admin/transfers?${qs.toString()}`);
}

export async function getTransfer(id: string): Promise<AdminTransferDetail> {
  return request<AdminTransferDetail>(`/admin/transfers/${id}`);
}

export async function revokeTransfer(id: string): Promise<void> {
  await request<{ ok: boolean }>(`/admin/transfers/${id}/revoke`, { method: 'POST' });
}
