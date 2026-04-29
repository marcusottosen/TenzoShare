import { useState, useEffect, useCallback } from 'react';
import type { TransferPublic, FileInfo } from '../types';
import { fetchTransfer, fetchDownloadUrl, TransferApiError } from '../api/transfer';

// ─── Icon components ───────────────────────────────────────────────────────

function IconBolt() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
    </svg>
  );
}

function IconFile() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <polyline points="14 2 14 8 20 8" />
    </svg>
  );
}

function IconImage() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
      <circle cx="8.5" cy="8.5" r="1.5" />
      <polyline points="21 15 16 10 5 21" />
    </svg>
  );
}

function IconVideo() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polygon points="23 7 16 12 23 17 23 7" />
      <rect x="1" y="5" width="15" height="14" rx="2" ry="2" />
    </svg>
  );
}

function IconAudio() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 18V5l12-2v13" />
      <circle cx="6" cy="18" r="3" />
      <circle cx="18" cy="16" r="3" />
    </svg>
  );
}

function IconArchive() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="21 8 21 21 3 21 3 8" />
      <rect x="1" y="3" width="22" height="5" />
      <line x1="10" y1="12" x2="14" y2="12" />
    </svg>
  );
}

function IconCode() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  );
}

function IconDownload() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="7 10 12 15 17 10" />
      <line x1="12" y1="15" x2="12" y2="3" />
    </svg>
  );
}

function IconLock() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
    </svg>
  );
}

function IconAlert() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  );
}

function IconShield() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  );
}

function IconTrash() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="3 6 5 6 21 6" />
      <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
      <path d="M10 11v6M14 11v6" />
      <path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" />
    </svg>
  );
}

// ─── File type icon helper ─────────────────────────────────────────────────

function fileTypeIcon(contentType: string) {
  if (contentType.startsWith('image/')) return <IconImage />;
  if (contentType.startsWith('video/')) return <IconVideo />;
  if (contentType.startsWith('audio/')) return <IconAudio />;
  if (contentType.includes('zip') || contentType.includes('tar') || contentType.includes('gzip') || contentType.includes('rar') || contentType.includes('7z')) return <IconArchive />;
  if (contentType.includes('javascript') || contentType.includes('typescript') || contentType.includes('json') || contentType.includes('xml') || contentType.includes('html') || contentType.includes('css')) return <IconCode />;
  return <IconFile />;
}

// ---------------------------------------------------------------------------
// Slug resolution
// ---------------------------------------------------------------------------

function resolveSlug(): string | null {
  const path = window.location.pathname;
  const pathMatch = path.match(/\/(?:t|download)\/([^/]+)/);
  if (pathMatch) return pathMatch[1];
  const param = new URLSearchParams(window.location.search).get('slug');
  if (param) return param;
  return null;
}

// ---------------------------------------------------------------------------
// View states
// ---------------------------------------------------------------------------

type View =
  | { kind: 'loading' }
  | { kind: 'error'; message: string; status?: number }
  | { kind: 'password'; slug: string }
  | { kind: 'ready'; transfer: TransferPublic; password?: string };

// ---------------------------------------------------------------------------
// DownloadPage
// ---------------------------------------------------------------------------

export default function DownloadPage() {
  const slug = resolveSlug();
  const [view, setView] = useState<View>({ kind: 'loading' });
  const [passwordInput, setPasswordInput] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [downloading, setDownloading] = useState<Record<string, boolean>>({});
  const [downloadErrors, setDownloadErrors] = useState<Record<string, string>>({});

  // Initial fetch — no password yet.
  useEffect(() => {
    if (!slug) {
      setView({ kind: 'error', message: 'No transfer link found. Check your URL.' });
      return;
    }
    fetchTransfer(slug)
      .then((t) => setView({ kind: 'ready', transfer: t }))
      .catch((err: unknown) => {
        if (err instanceof TransferApiError && err.status === 401) {
          setView({ kind: 'password', slug });
        } else if (err instanceof TransferApiError) {
          setView({ kind: 'error', message: err.message, status: err.status });
        } else {
          setView({ kind: 'error', message: 'Failed to load transfer.' });
        }
      });
  }, [slug]);

  // Submit password form.
  const handlePasswordSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      if (view.kind !== 'password') return;
      setPasswordError('');
      setView({ kind: 'loading' });
      fetchTransfer(view.slug, passwordInput)
        .then((t) => setView({ kind: 'ready', transfer: t, password: passwordInput }))
        .catch((err: unknown) => {
          const message =
            err instanceof TransferApiError ? err.message : 'Incorrect password.';
          setView({ kind: 'password', slug: (view as { kind: 'password'; slug: string }).slug });
          setPasswordError(message);
        });
    },
    [view, passwordInput],
  );

  // Trigger download for a single file.
  const handleDownload = useCallback(
    async (fileId: string) => {
      if (!slug) return;
      const password = view.kind === 'ready' ? view.password : undefined;
      setDownloading((d) => ({ ...d, [fileId]: true }));
      setDownloadErrors((d) => { const n = { ...d }; delete n[fileId]; return n; });
      try {
        const { url } = await fetchDownloadUrl(slug, fileId, password);
        const a = document.createElement('a');
        a.href = url;
        a.rel = 'noopener noreferrer';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
      } catch (err: unknown) {
        const message =
          err instanceof TransferApiError ? err.message : 'Download failed.';
        setDownloadErrors((d) => ({ ...d, [fileId]: message }));
      } finally {
        setDownloading((d) => ({ ...d, [fileId]: false }));
      }
    },
    [slug, view],
  );

  // ─── Render ────────────────────────────────────────────────────────────────

  if (view.kind === 'loading') {
    return (
      <Layout>
        <div className="state-center">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, justifyContent: 'center' }}>
            <span className="spinner" />
            <span className="tenzo-muted">Loading transfer…</span>
          </div>
        </div>
      </Layout>
    );
  }

  if (view.kind === 'error') {
    return (
      <Layout>
        <div className="state-center">
          <div className="state-icon state-icon-error">
            <IconAlert />
          </div>
          <h2 className="tenzo-title">{errorTitle(view.status)}</h2>
          <p className="tenzo-subtitle">{view.message}</p>
        </div>
      </Layout>
    );
  }

  if (view.kind === 'password') {
    return (
      <Layout>
        <div className="state-icon state-icon-warn" style={{ margin: '0 auto 20px' }}>
          <IconLock />
        </div>
        <h2 className="tenzo-title" style={{ textAlign: 'center' }}>Password required</h2>
        <p className="tenzo-subtitle" style={{ textAlign: 'center' }}>
          This transfer is password-protected. Enter the password to access the files.
        </p>
        {passwordError && (
          <div className="alert alert-error">{passwordError}</div>
        )}
        <form onSubmit={handlePasswordSubmit}>
          <div className="form-group">
            <label htmlFor="dl-password">Password</label>
            <input
              id="dl-password"
              type="password"
              className="tenzo-input"
              placeholder="Enter password"
              value={passwordInput}
              onChange={(e) => setPasswordInput(e.target.value)}
              autoFocus
              required
            />
          </div>
          <button type="submit" className="btn btn-primary btn-full btn-lg" style={{ marginTop: 8 }}>
            Unlock transfer
          </button>
        </form>
      </Layout>
    );
  }

  // view.kind === 'ready'
  const { transfer } = view;
  const downloadsLeft = transfer.max_downloads > 0
    ? Math.max(0, transfer.max_downloads - transfer.download_count)
    : null;

  // Build the list of file entries, preferring rich `files` array when available,
  // falling back to bare file_ids.
  const fileEntries: Array<FileInfo | { id: string; filename?: undefined }> =
    transfer.files && transfer.files.length > 0
      ? transfer.files
      : transfer.file_ids.map((id) => ({ id }));

  return (
    <Layout>
      {transfer.is_revoked && (
        <div className="revoked-banner" style={{ marginBottom: 20 }}>
          ⚠️ This transfer has been revoked by the sender.
        </div>
      )}

      <h2 className="tenzo-title">{transfer.name || 'Files ready to download'}</h2>
      {transfer.sender_email && (
        <p className="tenzo-subtitle">
          Shared by <strong>{transfer.sender_email}</strong>
        </p>
      )}
      {transfer.description && (
        <p className="tenzo-subtitle">{transfer.description}</p>
      )}

      {/* Metadata chips */}
      <div className="chips-row">
        {transfer.expires_at && (
          <span className="chip">Expires {formatDate(transfer.expires_at)}</span>
        )}
        {downloadsLeft !== null && (
          <span className={`chip ${downloadsLeft > 0 ? 'chip-teal' : ''}`}>
            {downloadsLeft} download{downloadsLeft !== 1 ? 's' : ''} remaining
          </span>
        )}
        <span className="chip">
          {fileEntries.length} file{fileEntries.length !== 1 ? 's' : ''}
        </span>
        {transfer.total_size_bytes > 0 && (
          <span className="chip">{fmtBytes(transfer.total_size_bytes)}</span>
        )}
      </div>

      <hr className="tenzo-divider" />

      {/* File list */}
      <ul className="file-list">
        {fileEntries.map((f) => {
          const fid = f.id;
          const isRich = 'filename' in f && f.filename !== undefined;
          const deleteReason = isRich ? (f as FileInfo).delete_reason : '';
          const isDeleted = !!deleteReason;
          const isDisabled = !!downloading[fid] || transfer.is_revoked || downloadsLeft === 0 || isDeleted;
          const downloadError = downloadErrors[fid];
          const isDeletedError = downloadError &&
            (downloadError.toLowerCase().includes('no longer available') ||
             downloadError.toLowerCase().includes('deleted') ||
             downloadError.toLowerCase().includes('not found'));
          const deletedLabel = deleteReasonLabel(deleteReason);
          const deletedTitle = deleteReasonTitle(deleteReason);

          return (
            <li
              key={fid}
              className="file-item"
              style={isDeleted ? { opacity: 0.6, background: '#fafafa' } : undefined}
            >
              <div
                className="file-icon"
                style={isDeleted ? { background: '#f3f4f6', border: '1px solid #e5e7eb', color: '#9ca3af' } : undefined}
              >
                {isDeleted ? <IconTrash /> : (isRich ? fileTypeIcon((f as FileInfo).content_type) : <IconFile />)}
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div className="file-name" style={isDeleted ? { color: 'var(--color-text-muted)', textDecoration: 'line-through' } : undefined}>
                  {isRich ? (f as FileInfo).filename : fid}
                </div>
                {isRich && !isDeleted && (
                  <div className="file-meta">
                    {fmtBytes((f as FileInfo).size_bytes)}
                    <span style={{ margin: '0 4px', opacity: 0.4 }}>·</span>
                    {friendlyMime((f as FileInfo).content_type)}
                  </div>
                )}
                {isDeleted && (
                  <div className="file-meta" style={{ color: 'var(--color-warn-text)', fontStyle: 'italic' }}>
                    {deletedLabel}
                  </div>
                )}
                {!isDeleted && downloadError && (
                  <div style={{ fontSize: 11, color: 'var(--color-error-text)', marginTop: 2 }}>
                    {isDeletedError
                      ? 'This file has been deleted and is no longer available.'
                      : downloadError}
                  </div>
                )}
              </div>
              <button
                className={`btn ${isDeleted ? 'btn-secondary' : 'btn-primary'} btn-sm`}
                onClick={() => !isDeleted && handleDownload(fid)}
                disabled={isDisabled}
                title={isDeleted ? deletedTitle : undefined}
              >
                {downloading[fid] ? (
                  <>
                    <span className="spinner" style={{ width: 12, height: 12, borderWidth: 1.5 }} />
                    Preparing…
                  </>
                ) : isDeleted ? (
                  'Unavailable'
                ) : (
                  <>
                    <IconDownload />
                    Download
                  </>
                )}
              </button>
            </li>
          );
        })}
      </ul>

      <TenzoFooter />
    </Layout>
  );
}

// ─── Layout ────────────────────────────────────────────────────────────────

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="tenzo-page">
      <div className="tenzo-card">
        <div className="tenzo-brand">
          <div className="tenzo-brand-icon">
            <IconBolt />
          </div>
          <span className="tenzo-brand-name">TenzoShare</span>
        </div>
        {children}
      </div>
    </div>
  );
}

// ─── Footer ────────────────────────────────────────────────────────────────

function TenzoFooter() {
  return (
    <div className="tenzo-footer">
      <IconShield />
      Files are encrypted and served securely via TenzoShare
    </div>
  );
}

// ─── Helpers ───────────────────────────────────────────────────────────────


// ─── Helpers ───────────────────────────────────────────────────────────────

function errorTitle(status?: number): string {
  if (status === 404) return 'Transfer not found';
  if (status === 403) return 'Access denied';
  if (status === 410) return 'Transfer expired';
  if (status === 401) return 'Authentication required';
  return 'Something went wrong';
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

function fmtBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

function friendlyMime(contentType: string): string {
  const t = contentType.split(';')[0].trim().toLowerCase();
  const map: Record<string, string> = {
    'application/pdf': 'PDF',
    'application/zip': 'ZIP',
    'application/x-zip-compressed': 'ZIP',
    'application/gzip': 'GZip',
    'application/x-tar': 'TAR',
    'application/x-7z-compressed': '7-Zip',
    'application/x-rar-compressed': 'RAR',
    'application/msword': 'Word',
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document': 'Word',
    'application/vnd.ms-excel': 'Excel',
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet': 'Excel',
    'application/vnd.ms-powerpoint': 'PowerPoint',
    'application/vnd.openxmlformats-officedocument.presentationml.presentation': 'PowerPoint',
    'text/plain': 'Text',
    'text/html': 'HTML',
    'text/css': 'CSS',
    'application/javascript': 'JavaScript',
    'application/json': 'JSON',
    'application/xml': 'XML',
    'image/jpeg': 'JPEG',
    'image/png': 'PNG',
    'image/gif': 'GIF',
    'image/webp': 'WebP',
    'image/svg+xml': 'SVG',
    'video/mp4': 'MP4',
    'video/webm': 'WebM',
    'audio/mpeg': 'MP3',
    'audio/wav': 'WAV',
    'audio/ogg': 'OGG',
  };
  if (map[t]) return map[t];
  // Fallback: strip the category prefix, uppercase the subtype
  const sub = t.split('/')[1] ?? t;
  return sub.replace(/^x-/, '').toUpperCase().slice(0, 12);
}

/** Human-readable label shown below a deleted file's name. */
function deleteReasonLabel(reason: string): string {
  switch (reason) {
    case 'owner_deleted':     return 'Removed by sender';
    case 'admin_purge':       return 'Removed by administrator';
    case 'retention_expired': return 'File expired (retention policy)';
    case 'orphan_expired':    return 'File expired';
    default:                  return reason ? 'No longer available' : '';
  }
}

/** Tooltip text for the disabled download button on a deleted file. */
function deleteReasonTitle(reason: string): string {
  switch (reason) {
    case 'owner_deleted':     return 'The sender deleted this file';
    case 'admin_purge':       return 'This file was removed by an administrator';
    case 'retention_expired': return 'This file was automatically removed after the retention period expired';
    case 'orphan_expired':    return 'This file was automatically removed';
    default:                  return 'This file is no longer available';
  }
}

