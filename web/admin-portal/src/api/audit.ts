import { request } from './client';

export interface AuditEvent {
  id: string;
  user_id?: string;
  source: string;
  action: string;
  resource_type?: string;
  resource_id?: string;
  metadata?: Record<string, unknown>;
  ip_address?: string;
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
}

export async function listAuditEvents(filters: AuditFilters = {}): Promise<AuditListResponse> {
  const params = new URLSearchParams();
  if (filters.user_id) params.set('user_id', filters.user_id);
  if (filters.source) params.set('source', filters.source);
  if (filters.action) params.set('action', filters.action);
  if (filters.start) params.set('start', filters.start);
  if (filters.end) params.set('end', filters.end);
  params.set('limit', String(filters.limit ?? 50));
  params.set('offset', String(filters.offset ?? 0));

  return request<AuditListResponse>(`/audit/events?${params.toString()}`);
}
