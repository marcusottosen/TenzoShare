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
const API_BASE = '/api/v1';
// ---------------------------------------------------------------------------
// Error class
// ---------------------------------------------------------------------------
export class TransferApiError extends Error {
    constructor(payload) {
        super(payload.message);
        Object.defineProperty(this, "status", {
            enumerable: true,
            configurable: true,
            writable: true,
            value: void 0
        });
        this.name = 'TransferApiError';
        this.status = payload.status;
    }
}
// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
async function handleResponse(res) {
    if (res.ok)
        return res.json();
    let message = `HTTP ${res.status}`;
    const status = res.status;
    try {
        const body = (await res.json());
        // Backend returns {"error":{"code":"...","message":"..."}} or {"error":"string"} or {"message":"string"}
        if (typeof body.error === 'string') {
            message = body.error;
        }
        else if (body.error && typeof body.error === 'object' && body.error.message) {
            message = body.error.message;
        }
        else if (typeof body.message === 'string') {
            message = body.message;
        }
    }
    catch {
        /* ignore parse error — use default message */
    }
    throw new TransferApiError({ message, status });
}
function buildURL(path, params) {
    const url = new URL(path, window.location.origin);
    if (params) {
        for (const [k, v] of Object.entries(params)) {
            if (v)
                url.searchParams.set(k, v);
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
 * @param slug     The short token from the share URL (e.g. "AbCdEfGh1234").
 * @param password Optional password.  Omit on first call — the response
 *                 `has_password` field tells you whether one is required.
 *
 * @throws {TransferApiError} status 401 if a password is required but not
 *   provided, 403 if expired/revoked/download-limit-reached, 404 if not found.
 */
export async function fetchTransfer(slug, password) {
    const params = password ? { password } : undefined;
    const url = buildURL(`${API_BASE}/t/${encodeURIComponent(slug)}`, params);
    const res = await fetch(url);
    const body = await handleResponse(res);
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
export async function fetchDownloadUrl(slug, fileId, password) {
    const params = password ? { password } : undefined;
    const url = buildURL(`${API_BASE}/t/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}/download`, params);
    const res = await fetch(url);
    return handleResponse(res);
}
