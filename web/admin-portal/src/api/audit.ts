import { request } from './client';

export interface AuditEvent {
  id: string;
  source: string;
  action: string;
  user_id: string | null;
  client_ip: string | null;
  subject: string;
  payload: Record<string, unknown>;
  success: boolean;
  created_at: string;
}

export interface AuditListResponse {
  total: number;
  limit: number;
  offset: number;
  items: AuditEvent[];
}

export interface AuditFilters {
  user_id?: string;
  source?: string;
  action?: string;
  start?: string;
  end?: string;
  limit?: number;
  offset?: number;
  sort_by?: string;
  sort_dir?: string;
}

export async function listAuditEvents(filters: AuditFilters = {}): Promise<AuditListResponse> {
  const params = new URLSearchParams();
  if (filters.user_id) params.set('user_id', filters.user_id);
  if (filters.source) params.set('source', filters.source);
  if (filters.action) params.set('action', filters.action);
  if (filters.start) params.set('start', filters.start);
  if (filters.end) params.set('end', filters.end);
  if (filters.sort_by) params.set('sort_by', filters.sort_by);
  if (filters.sort_dir) params.set('sort_dir', filters.sort_dir);
  params.set('limit', String(filters.limit ?? 50));
  params.set('offset', String(filters.offset ?? 0));

  return request<AuditListResponse>(`/audit/events?${params.toString()}`);
}
