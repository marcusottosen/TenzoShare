/**
 * TenzoShare Public Transfer API client.
 *
 * This module is intentionally dependency-free (no React, no third-party
 * libraries). It only uses the browser Fetch API.  Anyone building their own
 * download page can copy this file and use it as-is, or just read it as an
 * executable specification of the two public endpoints.
 *
 * API base is resolved from the environment so the same build works behind
 * nginx in Docker (relative "/api") and locally via Vite's dev-server proxy.
 *
 * ─── Endpoints ────────────────────────────────────────────────────────────────
 *
 *  POST /api/v1/t/:slug
 *    Returns transfer metadata.  Pass `password` in the JSON body only when
 *    `has_password` is true; the first call (without a body) tells you whether
 *    one is needed.
 *    ← { transfer: TransferPublic }
 *
 *  POST /api/v1/t/:slug/files/:fileId/download
 *    Returns a short-lived presigned URL for the requested file.
 *    The transfer validation (slug, password, expiry) is repeated server-side.
 *    ← { url: string, expires_in: number }
 *
 * ──────────────────────────────────────────────────────────────────────────────
 */

import type { TransferPublic, FileDownloadUrl, ApiErrorPayload } from '../types';

const API_BASE = '/api/v1';

// ---------------------------------------------------------------------------
// Error class
// ---------------------------------------------------------------------------

export class TransferApiError extends Error {
  readonly status: number;
  constructor(payload: ApiErrorPayload) {
    super(payload.message);
    this.name = 'TransferApiError';
    this.status = payload.status;
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.ok) return res.json() as Promise<T>;

  let message = `HTTP ${res.status}`;
  const status = res.status;
  try {
    const body = (await res.json()) as {
      error?: string | { message?: string; code?: string };
      message?: string;
    };
    // Backend returns {"error":{"code":"...","message":"..."}} or {"error":"string"} or {"message":"string"}
    if (typeof body.error === 'string') {
      message = body.error;
    } else if (body.error && typeof body.error === 'object' && body.error.message) {
      message = body.error.message;
    } else if (typeof body.message === 'string') {
      message = body.message;
    }
  } catch {
    /* ignore parse error — use default message */
  }
  throw new TransferApiError({ message, status });
}

// ---------------------------------------------------------------------------
// Public API functions
// ---------------------------------------------------------------------------

/**
 * Fetch the public metadata for a transfer.
 *
 * @param slug     The short token from the share URL (e.g. "AbCdEfGh1234").
 * @param password Optional password.  Omit on first call — the response
 *                 `has_password` field tells you whether one is required.
 *
 * @throws {TransferApiError} status 401 if a password is required but not
 *   provided, 403 if expired/revoked/download-limit-reached, 404 if not found.
 */
export async function fetchTransfer(
  slug: string,
  password?: string,
): Promise<TransferPublic> {
  const res = await fetch(`${API_BASE}/t/${encodeURIComponent(slug)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(password ? { password } : {}),
  });
  const body = await handleResponse<{ transfer: TransferPublic }>(res);
  return body.transfer;
}

/**
 * Obtain a presigned download URL for a single file in a transfer.
 *
 * The server re-validates the slug and password on every call, so the URL is
 * only issued for transfers that are still valid at request time.
 *
 * @param slug     Transfer slug.
 * @param fileId   UUID of the file to download (from `TransferPublic.file_ids`).
 * @param password Transfer password, if the transfer is password-protected.
 *
 * @throws {TransferApiError} same conditions as `fetchTransfer`, plus 404 if
 *   the file does not belong to this transfer.
 */
export async function fetchDownloadUrl(
  slug: string,
  fileId: string,
  password?: string,
): Promise<FileDownloadUrl> {
  const res = await fetch(
    `${API_BASE}/t/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}/download`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(password ? { password } : {}),
    },
  );
  return handleResponse<FileDownloadUrl>(res);
}
