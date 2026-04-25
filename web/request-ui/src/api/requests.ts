import type { FileRequestPublic, Submission } from '../types';
import { RequestApiError } from '../types';

const API_BASE = '/api/v1';

async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: `HTTP ${res.status}` }));
    const msg = body?.error?.message ?? body?.message ?? `HTTP ${res.status}`;
    throw new RequestApiError(res.status, msg);
  }
  return res.json() as Promise<T>;
}

/** Fetch public metadata for a file request by its slug. */
export function fetchRequest(slug: string): Promise<FileRequestPublic> {
  return apiGet<FileRequestPublic>(`/r/${slug}`);
}

/** Upload a single file to a file request.
 *  Returns the created submission record.
 *  Calls onProgress(0–100) as the upload progresses.
 */
export function uploadFile(
  slug: string,
  file: File,
  submitterName: string,
  message: string,
  onProgress: (pct: number) => void,
): Promise<Submission> {
  return new Promise((resolve, reject) => {
    const fd = new FormData();
    fd.append('file', file);
    if (submitterName.trim()) fd.append('submitter_name', submitterName.trim());
    if (message.trim()) fd.append('message', message.trim());

    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${API_BASE}/r/${slug}/upload`);

    if (xhr.upload) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
      };
    }

    xhr.onload = () => {
      if (xhr.status === 200 || xhr.status === 201) {
        try {
          resolve(JSON.parse(xhr.responseText) as Submission);
        } catch {
          reject(new RequestApiError(xhr.status, 'Invalid server response'));
        }
      } else {
        let msg = `HTTP ${xhr.status}`;
        try {
          const body = JSON.parse(xhr.responseText);
          msg = body?.error?.message ?? body?.message ?? msg;
        } catch {/* ignore */}
        reject(new RequestApiError(xhr.status, msg));
      }
    };

    xhr.onerror = () => reject(new RequestApiError(0, 'Network error'));
    xhr.send(fd);
  });
}
