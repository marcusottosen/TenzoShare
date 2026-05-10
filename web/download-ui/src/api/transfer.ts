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
 *  GET /api/v1/t/:slug[?password=<value>]
 *    Returns transfer metadata.  Pass `password` only when `has_password` is
 *    true; the first call (without a password) tells you whether one is needed.
 *    ← { transfer: TransferPublic }
 *
 *  GET /api/v1/t/:slug/files/:fileId/download[?password=<value>]
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

function buildURL(path: string, params?: Record<string, string>): string {
  const url = new URL(path, window.location.origin);
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v) url.searchParams.set(k, v);
    }
  }
  return url.toString();
}

// ---------------------------------------------------------------------------
// Public API functions
// ---------------------------------------------------------------------------

/**
 * Fetch the public metadata for a transfer.
 *
 * @param slug           The short token from the share URL (e.g. "AbCdEfGh1234").
 * @param password       Optional password. Omit on first call — the response
 *                       `has_password` field tells you whether one is required.
 * @param recipientToken Optional magic-link token from the email link (?rt=...).
 *                       When provided the server validates the token and bypasses
 *                       the password requirement.
 *
 * @throws {TransferApiError} status 401 if a password is required but not
 *   provided, 403 if expired/revoked/download-limit-reached, 404 if not found.
 */
export async function fetchTransfer(
  slug: string,
  password?: string,
  recipientToken?: string,
): Promise<TransferPublic> {
  const params: Record<string, string> = {};
  if (recipientToken) params['rt'] = recipientToken;
  else if (password) params['password'] = password;
  const url = buildURL(`${API_BASE}/t/${encodeURIComponent(slug)}`, Object.keys(params).length ? params : undefined);
  const res = await fetch(url);
  const body = await handleResponse<{ transfer: TransferPublic }>(res);
  return body.transfer;
}

/**
 * Obtain a presigned download URL for a single file in a transfer.
 *
 * The server re-validates the slug and password/token on every call.
 *
 * @param slug           Transfer slug.
 * @param fileId         UUID of the file to download.
 * @param password       Transfer password, if the transfer is password-protected.
 * @param recipientToken Magic-link token from the email link, if present.
 *
 * @throws {TransferApiError} same conditions as `fetchTransfer`, plus 404 if
 *   the file does not belong to this transfer.
 */
export async function fetchDownloadUrl(
  slug: string,
  fileId: string,
  password?: string,
  recipientToken?: string,
): Promise<FileDownloadUrl> {
  const params: Record<string, string> = {};
  if (recipientToken) params['rt'] = recipientToken;
  else if (password) params['password'] = password;
  const url = buildURL(
    `${API_BASE}/t/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}/download`,
    Object.keys(params).length ? params : undefined,
  );
  const res = await fetch(url);
  return handleResponse<FileDownloadUrl>(res);
}

/**
 * Request a new magic-link access email for an expired recipient link.
 * Always resolves (server never reveals whether the email is a recipient).
 */
export async function requestAccessLink(slug: string, email: string): Promise<void> {
  const url = buildURL(`${API_BASE}/t/${encodeURIComponent(slug)}/request-access`);
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  });
  await handleResponse<unknown>(res);
}
