/**
 * TenzoShare Public Download API — type contract.
 *
 * These types describe every field returned by the public transfer endpoints.
 * Anyone replacing this UI only needs to implement calls to these two endpoints:
 *
 *   GET /api/v1/t/:slug[?password=...]
 *   GET /api/v1/t/:slug/files/:fileId/download[?password=...]
 *
 * No authentication token is required.
 */

/** A transfer as returned by the public access endpoint. */
export interface TransferPublic {
  /** UUID of the transfer record. */
  id: string;
  /** Short URL token that identifies this transfer (appears in the share link). */
  slug: string;
  /** Human-readable name given by the sender. */
  name: string;
  /** Optional longer note from the sender. */
  description?: string;
  /** Whether the transfer requires a password to access. */
  has_password: boolean;
  /** IDs of the files included in this transfer. */
  file_ids: string[];
  /** Max allowed total downloads across all files. 0 = unlimited. */
  max_downloads: number;
  /** Number of times this transfer has already been accessed. */
  download_count: number;
  /** Whether the owner has revoked the transfer. */
  is_revoked: boolean;
  /** ISO-8601 expiry timestamp. */
  expires_at: string;
  /** Email of the intended recipient, if restricted to one address. */
  recipient_email?: string;
  /** ISO-8601 creation timestamp. */
  created_at: string;
}

/** Returned by the file download URL endpoint. */
export interface FileDownloadUrl {
  /**
   * Pre-signed URL pointing directly to the file in object storage.
   * Valid for `expires_in` seconds — open it with window.open() or
   * trigger via an <a href="…" download> element.
   */
  url: string;
  /** Seconds until the presigned URL expires (typically 900 = 15 min). */
  expires_in: number;
}

/** Structured error returned when the backend rejects a request. */
export interface ApiErrorPayload {
  /** Human-readable description of what went wrong. */
  message: string;
  /** HTTP status code (same as the response status). */
  status: number;
}
