import { request } from './client';

export interface Transfer {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  slug: string;
  status?: 'active' | 'exhausted' | 'expired' | 'revoked';
  recipient_email?: string;
  max_downloads: number;
  download_count: number;
  is_revoked: boolean;
  has_password: boolean;
  view_only: boolean;
  expires_at?: string;
  created_at: string;
  file_ids?: string[];
  file_count?: number;
  total_size_bytes?: number;
}

export interface TransferListResponse {
  transfers: Transfer[];
  limit: number;
  offset: number;
}

export interface CreateTransferParams {
  name: string;
  description?: string;
  file_ids: string[];
  recipient_email?: string;
  password?: string;
  max_downloads?: number;
  view_only?: boolean;
  /** Required: 1–2160 hours (max 90 days). */
  expires_in_hours: number;
}

export async function listTransfers(limit = 50, offset = 0): Promise<TransferListResponse> {
  return request<TransferListResponse>(`/transfers?limit=${limit}&offset=${offset}`);
}

export async function getTransfer(id: string): Promise<Transfer> {
  return request<Transfer>(`/transfers/${id}`);
}

export async function createTransfer(params: CreateTransferParams): Promise<Transfer> {
  return request<Transfer>('/transfers', {
    method: 'POST',
    body: JSON.stringify(params),
  });
}

export async function revokeTransfer(id: string): Promise<void> {
  return request<void>(`/transfers/${id}`, { method: 'DELETE' });
}
