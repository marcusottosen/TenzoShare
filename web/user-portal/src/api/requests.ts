import { request } from './client';

export interface FileRequest {
  id: string;
  slug: string;
  name: string;
  description: string;
  allowed_types: string;
  max_size_mb: number;
  max_files: number;
  recipient_emails?: string[];
  expires_at: string;
  is_active: boolean;
  is_expired: boolean;
  created_at: string;
  submission_count?: number;
  submissions?: Submission[];
}

export interface Submission {
  id: string;
  file_id: string;
  filename: string;
  size_bytes: number;
  submitter_name: string;
  message: string;
  submitted_at: string;
}

export interface CreateFileRequestParams {
  name: string;
  description?: string;
  allowed_types?: string;
  max_size_mb?: number;
  max_files?: number;
  expires_in_hours: number;
  recipient_emails?: string[];
}

export async function listFileRequests(): Promise<{ requests: FileRequest[] }> {
  return request<{ requests: FileRequest[] }>('/requests');
}

export async function getFileRequest(id: string): Promise<FileRequest> {
  return request<FileRequest>(`/requests/${id}`);
}

export async function createFileRequest(params: CreateFileRequestParams): Promise<FileRequest> {
  return request<FileRequest>('/requests', {
    method: 'POST',
    body: JSON.stringify(params),
  });
}

export async function deactivateFileRequest(id: string): Promise<void> {
  return request<void>(`/requests/${id}`, { method: 'DELETE' });
}

export async function updateRequestRecipients(id: string, emails: string[]): Promise<FileRequest> {
  return request<FileRequest>(`/requests/${id}/recipients`, {
    method: 'PATCH',
    body: JSON.stringify({ emails }),
  });
}

export async function resendRequestInvite(id: string): Promise<void> {
  return request<void>(`/requests/${id}/resend`, { method: 'POST' });
}
