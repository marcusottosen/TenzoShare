import { request } from './client';

export interface AdminUser {
  id: string;
  email: string;
  role: string;
  is_active: boolean;
  email_verified: boolean;
  mfa_enabled: boolean;
  failed_login_attempts: number;
  locked_until: string | null;
  last_login_at: string | null;
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

export async function resetUserPassword(id: string): Promise<{ temp_password: string }> {
  return request<{ temp_password: string }>(`/admin/users/${id}/reset-password`, { method: 'POST' });
}

export async function setUserPassword(id: string, password: string): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>(`/admin/users/${id}/set-password`, {
    method: 'POST',
    body: JSON.stringify({ password }),
  });
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
  total_size_bytes: number;
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

export interface StorageUserUsage {
  user_id: string;
  email: string;
  file_count: number;
  total_bytes: number;
}

export interface StorageUsageListResponse {
  usage: StorageUserUsage[];
  total: number;
  limit: number;
  offset: number;
}

export async function listStorageUsage(params: {
  limit?: number;
  offset?: number;
  sort_by?: string;
  sort_dir?: string;
} = {}): Promise<StorageUsageListResponse> {
  const qs = new URLSearchParams();
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.offset) qs.set('offset', String(params.offset));
  if (params.sort_by) qs.set('sort_by', params.sort_by);
  if (params.sort_dir) qs.set('sort_dir', params.sort_dir);
  return request<StorageUsageListResponse>(`/admin/storage/usage?${qs.toString()}`);
}

export interface StorageConfig {
  quota_enabled: boolean;
  quota_bytes_per_user: number;
  max_upload_size_bytes: number;
  retention_enabled: boolean;
  retention_days: number;
  orphan_retention_days: number;
  /** When true, plain-HTTP uploads are accepted (dev/test only). */
  test_mode: boolean;
  updated_at: string;
  updated_by: string;
}

export async function getStorageConfig(): Promise<StorageConfig> {
  return request<StorageConfig>('/admin/storage/config');
}

export async function updateStorageConfig(body: {
  quota_enabled?: boolean;
  quota_bytes_per_user?: number;
  max_upload_size_bytes?: number;
  retention_enabled?: boolean;
  retention_days?: number;
  orphan_retention_days?: number;
  test_mode?: boolean;
}): Promise<StorageConfig> {
  return request<StorageConfig>('/admin/storage/config', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export interface UserQuota {
  has_override: boolean;
  quota_bytes?: number;
  updated_at?: string;
  updated_by?: string;
}

export async function getUserQuota(userId: string): Promise<UserQuota> {
  return request<UserQuota>(`/admin/users/${userId}/quota`);
}

export async function setUserQuota(userId: string, quotaBytes: number | null): Promise<{ ok: boolean; has_override: boolean; quota_bytes?: number }> {
  return request<{ ok: boolean; has_override: boolean; quota_bytes?: number }>(`/admin/users/${userId}/quota`, {
    method: 'PUT',
    body: JSON.stringify({ quota_bytes: quotaBytes }),
  });
}

export interface QuotaOverride {
  user_id: string;
  quota_bytes: number;
  updated_at: string;
  updated_by: string;
}

export async function listUserQuotas(): Promise<{ overrides: QuotaOverride[] }> {
  return request<{ overrides: QuotaOverride[] }>('/admin/quotas');
}

export interface AdminFileRow {
  id: string;
  owner_id: string;
  owner_email: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
  share_count: number;
  active_shares: number;
  last_share_expires_at: string | null;
  eligible_purge: boolean;
}

export interface AdminFilesResponse {
  files: AdminFileRow[];
  total: number;
  limit: number;
  offset: number;
}

export async function listStorageFiles(params: {
  limit?: number;
  offset?: number;
  sort_by?: string;
  sort_dir?: string;
  filter?: 'all' | 'orphan' | 'eligible';
} = {}): Promise<AdminFilesResponse> {
  const qs = new URLSearchParams();
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.offset) qs.set('offset', String(params.offset));
  if (params.sort_by) qs.set('sort_by', params.sort_by);
  if (params.sort_dir) qs.set('sort_dir', params.sort_dir);
  if (params.filter) qs.set('filter', params.filter);
  return request<AdminFilesResponse>(`/admin/storage/files?${qs.toString()}`);
}

export async function adminDeleteFile(id: string): Promise<void> {
  await request<void>(`/admin/storage/files/${id}`, { method: 'DELETE' });
}

export interface PurgeResult {
  deleted: number;
  freed_bytes: number;
  capped?: boolean;
  cap?: number;
}

export async function triggerPurge(): Promise<PurgeResult> {
  return request<PurgeResult>('/admin/storage/purge', { method: 'POST' });
}

export interface PurgeLogEntry {
  file_id: string;
  owner_id: string;
  email: string;
  filename: string;
  size_bytes: number;
  reason: string;
  purged_by: string;
  purged_at: string;
}

export interface PurgeLogResponse {
  entries: PurgeLogEntry[];
  total: number;
  limit: number;
  offset: number;
}

export async function getPurgeLog(params: { limit?: number; offset?: number } = {}): Promise<PurgeLogResponse> {
  const qs = new URLSearchParams();
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.offset) qs.set('offset', String(params.offset));
  return request<PurgeLogResponse>(`/admin/storage/purge-log?${qs.toString()}`);
}

export interface ContentTypeStat {
  content_type: string;
  count: number;
  size_bytes: number;
}

export interface PurgeReasonStat {
  reason: string;
  count: number;
  freed_bytes: number;
}

export interface PurgeDayStat {
  day: string;
  count: number;
  freed_bytes: number;
}

export interface StorageInsights {
  total_files: number;
  total_storage_bytes: number;
  deleted_files: number;
  purged_files: number;
  freed_bytes: number;
  unique_owners: number;
  content_type_breakdown: ContentTypeStat[];
  purge_reason_breakdown: PurgeReasonStat[];
  purge_per_day: PurgeDayStat[];
  storage_per_day: StorageDayStat[];
}

export async function getStorageInsights(): Promise<StorageInsights> {
  return request<StorageInsights>('/admin/storage/insights');
}

// ── Audit log retention ───────────────────────────────────────────────────────

export interface AuditConfig {
  retention_enabled: boolean;
  retention_days: number;
  updated_at: string;
  updated_by: string;
}

export interface AuditStats {
  total_entries: number;
  oldest_entry: string | null;
  newest_entry: string | null;
  by_source: { source: string; count: number }[];
}

export interface AuditPurgeResult {
  deleted: number;
  retention_days?: number;
  message?: string;
}

export async function getAuditConfig(): Promise<AuditConfig> {
  return request<AuditConfig>('/admin/audit/config');
}

export async function updateAuditConfig(body: {
  retention_enabled?: boolean;
  retention_days?: number;
}): Promise<AuditConfig> {
  return request<AuditConfig>('/admin/audit/config', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export async function getAuditStats(): Promise<AuditStats> {
  return request<AuditStats>('/admin/audit/stats');
}

export async function triggerAuditPurge(): Promise<AuditPurgeResult> {
  return request<AuditPurgeResult>('/admin/audit/purge', { method: 'POST' });
}

// ── Auth lockout config ────────────────────────────────────────────────────────

export interface AuthLockoutConfig {
  max_failed_attempts: number;
  lockout_duration_minutes: number;
  require_mfa: boolean;
  updated_at: string;
}

export async function getAuthConfig(): Promise<AuthLockoutConfig> {
  return request<AuthLockoutConfig>('/admin/auth/config');
}

export async function updateAuthConfig(body: {
  max_failed_attempts?: number;
  lockout_duration_minutes?: number;
  require_mfa?: boolean;
}): Promise<AuthLockoutConfig> {
  return request<AuthLockoutConfig>('/admin/auth/config', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export async function resetUserMFA(userId: string): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>(`/admin/users/${userId}/mfa`, { method: 'DELETE' });
}

// ── Branding ──────────────────────────────────────────────────────────────────

export interface BrandingConfig {
  primary_color: string;
  secondary_color: string;
  page_bg_color: string;
  surface_color: string;
  text_color: string;
  border_radius: number;
  app_name: string;
  custom_css: string | null;
  logo_data_url: string | null;
  updated_at: string;
  // Dark-mode overrides (null = use built-in dark defaults)
  dm_primary_color: string | null;
  dm_secondary_color: string | null;
  dm_page_bg_color: string | null;
  dm_surface_color: string | null;
  dm_text_color: string | null;
  // Email white-label
  email_sender_name: string;
  email_support_email: string;
  email_footer_text: string;
  email_subject_prefix: string;
  email_header_link: string;
  // Email colors & reply-to (migration 008)
  email_reply_to: string;
  email_button_color: string;
  email_button_text_color: string;
  email_body_bg_color: string;
  email_card_bg_color: string;
  email_card_border_color: string;
  email_heading_color: string;
  email_text_color: string;
  // Per-type subjects
  subject_transfer_received: string;
  subject_password_reset: string;
  subject_email_verification: string;
  subject_download_notification: string;
  subject_expiry_reminder: string;
  subject_transfer_revoked: string;
  subject_request_submission: string;
  // Per-type CTA button text
  cta_transfer_received: string;
  cta_download_notification: string;
  cta_password_reset: string;
  cta_email_verification: string;
  cta_expiry_reminder: string;
  cta_request_submission: string;
  // Per-type fully custom HTML templates (empty = use standard branded template)
  custom_transfer_received: string;
  custom_password_reset: string;
  custom_email_verification: string;
  custom_download_notification: string;
  custom_expiry_reminder: string;
  custom_transfer_revoked: string;
  custom_request_submission: string;
}

export async function getBranding(): Promise<BrandingConfig> {
  return request<BrandingConfig>('/admin/branding');
}

export async function updateBranding(body: {
  primary_color?: string;
  secondary_color?: string;
  page_bg_color?: string;
  surface_color?: string;
  text_color?: string;
  border_radius?: number;
  app_name?: string;
  custom_css?: string;
  clear_custom_css?: boolean;
  logo_data_url?: string;
  clear_logo?: boolean;
  // Dark-mode overrides
  dm_primary_color?: string;
  dm_secondary_color?: string;
  dm_page_bg_color?: string;
  dm_surface_color?: string;
  dm_text_color?: string;
  clear_dark_mode?: boolean;
  // Email white-label
  email_sender_name?: string;
  email_support_email?: string;
  email_footer_text?: string;
  email_subject_prefix?: string;
  email_header_link?: string;
  // Email colors & reply-to (migration 008)
  email_reply_to?: string;
  email_button_color?: string;
  email_button_text_color?: string;
  email_body_bg_color?: string;
  email_card_bg_color?: string;
  email_card_border_color?: string;
  email_heading_color?: string;
  email_text_color?: string;
  // Per-type subjects
  subject_transfer_received?: string;
  subject_password_reset?: string;
  subject_email_verification?: string;
  subject_download_notification?: string;
  subject_expiry_reminder?: string;
  subject_transfer_revoked?: string;
  subject_request_submission?: string;
  // Per-type CTA button text
  cta_transfer_received?: string;
  cta_download_notification?: string;
  cta_password_reset?: string;
  cta_email_verification?: string;
  cta_expiry_reminder?: string;
  cta_request_submission?: string;
  // Per-type fully custom HTML templates
  custom_transfer_received?: string;
  custom_password_reset?: string;
  custom_email_verification?: string;
  custom_download_notification?: string;
  custom_expiry_reminder?: string;
  custom_transfer_revoked?: string;
  custom_request_submission?: string;
}): Promise<BrandingConfig> {
  return request<BrandingConfig>('/admin/branding', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

