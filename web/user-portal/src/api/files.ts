import { request, getToken } from './client';

export interface FileRecord {
  id: string;
  owner_id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
  // Share / retention info (populated by the storage service when retention is configured)
  share_count?: number;
  active_shares?: number;
  last_share_expires_at?: string | null;
  /** ISO date after which this file becomes eligible for automatic deletion. Null = protected. */
  auto_delete_at?: string | null;
}

export interface FileListResponse {
  files: FileRecord[];
  limit: number;
  offset: number;
}

export interface StorageUsage {
  user_id: string;
  file_count: number;
  total_bytes: number;
  quota_enabled: boolean;
  quota_bytes_per_user: number;
  /** Maximum allowed file size in bytes (0 = no limit). */
  max_upload_size_bytes: number;
}

export async function getMyUsage(): Promise<StorageUsage> {
  return request<StorageUsage>('/files/usage');
}

export async function listFiles(limit = 50, offset = 0): Promise<FileListResponse> {
  return request<FileListResponse>(`/files?limit=${limit}&offset=${offset}`);
}

export async function getFile(id: string): Promise<FileRecord> {
  return request<FileRecord>(`/files/${id}`);
}

export async function deleteFile(id: string): Promise<void> {
  return request<void>(`/files/${id}`, { method: 'DELETE' });
}

export async function presignFile(id: string): Promise<{ url: string; expires_in: number }> {
  return request<{ url: string; expires_in: number }>(`/files/${id}/presign`);
}

export interface UploadProgress {
  loaded: number;
  total: number;
}

export async function uploadFile(
  file: File,
  onProgress?: (p: UploadProgress) => void,
): Promise<FileRecord> {
  const formData = new FormData();
  formData.append('file', file);

  const token = getToken();
  const headers: Record<string, string> = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;

  // Use XMLHttpRequest for progress tracking
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/v1/files');
    Object.entries(headers).forEach(([k, v]) => xhr.setRequestHeader(k, v));

    if (onProgress && xhr.upload) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress({ loaded: e.loaded, total: e.total });
      };
    }

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText));
        } catch {
          reject(new Error('Invalid response'));
        }
      } else {
        // Try to parse a structured JSON error from the Go backend first.
        // If the response is not JSON (e.g. an nginx HTML error page), fall
        // back to a human-readable message based on the status code.
        try {
          const err = JSON.parse(xhr.responseText);
          reject(new Error(err.error?.message ?? err.message ?? `HTTP ${xhr.status}`));
        } catch {
          if (xhr.status === 413) {
            reject(new Error(
              'File too large — the server rejected this upload. ' +
              'Check the maximum upload size in your storage settings.',
            ));
          } else {
            reject(new Error(`Upload failed (HTTP ${xhr.status})`));
          }
        }
      }
    };
    xhr.onerror = () => reject(new Error('Network error'));
    xhr.send(formData);
  });
}


