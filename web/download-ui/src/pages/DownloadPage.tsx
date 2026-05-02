import { useState, useEffect, useCallback } from 'react';
import type { TransferPublic, FileInfo } from '../types';
import { fetchTransfer, fetchDownloadUrl, TransferApiError } from '../api/transfer';

// ─── Icon components ───────────────────────────────────────────────────────

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

function IconEye() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  );
}

function IconViewOnly() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
      <circle cx="12" cy="12" r="3" />
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

function IconClose() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="18" y1="6" x2="6" y2="18" />
      <line x1="6" y1="6" x2="18" y2="18" />
    </svg>
  );
}

// ─── File type icon helper ─────────────────────────────────────────────────

function canPreview(contentType: string): boolean {
  const t = contentType.split(';')[0].trim().toLowerCase();
  return (
    t.startsWith('image/') ||
    t.startsWith('text/') ||
    t.startsWith('audio/') ||
    t.startsWith('video/') ||
    t === 'application/pdf' ||
    t === 'application/json' ||
    t === 'application/javascript' ||
    t === 'application/xml'
  );
}

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
// Viewer modal state
// ---------------------------------------------------------------------------

type ViewerModal = {
  viewUrl: string;      // URL used for display (inline=1 appended for proxy URLs)
  downloadUrl: string;  // Original URL used for download
  filename: string;
  contentType: string;
  sizeBytes: number;
  viewOnly: boolean;
};

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
  const [viewer, setViewer] = useState<ViewerModal | null>(null);

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

  // Core action: either opens the in-page viewer modal or triggers a browser download.
  const handleAction = useCallback(
    async (fileId: string, action: 'download' | 'preview') => {
      if (!slug) return;
      const password = view.kind === 'ready' ? view.password : undefined;
      const isViewOnly = view.kind === 'ready' && view.transfer.view_only;
      const file =
        view.kind === 'ready' && view.transfer.files
          ? view.transfer.files.find((f) => f.id === fileId) ?? null
          : null;
      const shouldPreview = action === 'preview' || isViewOnly;
      setDownloading((d) => ({ ...d, [fileId]: true }));
      setDownloadErrors((d) => { const n = { ...d }; delete n[fileId]; return n; });
      try {
        const result = await fetchDownloadUrl(slug, fileId, password);
        const effectivelyViewOnly = isViewOnly || !!result.view_only;
        if (shouldPreview || effectivelyViewOnly) {
          const downloadUrl = result.url;
          let viewUrl = result.url;
          // For our own proxy URLs, append inline=1 so storage serves Content-Disposition: inline.
          if (viewUrl.startsWith('/') && !viewUrl.includes('inline=1')) {
            viewUrl += (viewUrl.includes('?') ? '&' : '?') + 'inline=1';
          }
          setViewer({
            viewUrl,
            downloadUrl,
            filename: file?.filename ?? fileId,
            contentType: file?.content_type ?? '',
            sizeBytes: file?.size_bytes ?? 0,
            viewOnly: effectivelyViewOnly,
          });
        } else {
          const a = document.createElement('a');
          a.href = result.url;
          a.rel = 'noopener noreferrer';
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
        }
      } catch (err: unknown) {
        const message =
          err instanceof TransferApiError ? err.message : 'Failed to open file.';
        setDownloadErrors((d) => ({ ...d, [fileId]: message }));
      } finally {
        setDownloading((d) => ({ ...d, [fileId]: false }));
      }
    },
    [slug, view],
  );

  const handleDownload = useCallback((fileId: string) => handleAction(fileId, 'download'), [handleAction]);
  const handlePreview  = useCallback((fileId: string) => handleAction(fileId, 'preview'),  [handleAction]);

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
  const viewOnly = !!transfer.view_only;
  const accessesLeft = transfer.max_downloads > 0
    ? Math.max(0, transfer.max_downloads - transfer.download_count)
    : null;

  // Build the list of file entries, preferring rich `files` array when available,
  // falling back to bare file_ids.
  const fileEntries: Array<FileInfo | { id: string; filename?: undefined }> =
    transfer.files && transfer.files.length > 0
      ? transfer.files
      : transfer.file_ids.map((id) => ({ id }));

  return (
    <>
    <Layout>
      {transfer.is_revoked && (
        <div className="revoked-banner" style={{ marginBottom: 20 }}>
          ⚠️ This transfer has been revoked by the sender.
        </div>
      )}

      {/* View-only notice */}
      {viewOnly && (
        <div style={{
          display: 'flex',
          alignItems: 'flex-start',
          gap: 12,
          padding: '12px 16px',
          marginBottom: 16,
          background: 'rgba(99,102,241,0.07)',
          border: '1px solid rgba(99,102,241,0.3)',
          borderRadius: 10,
          fontSize: 13,
          color: '#3730a3',
          lineHeight: 1.5,
        }}>
          <span style={{ marginTop: 1, flexShrink: 0 }}><IconViewOnly /></span>
          <div>
            <strong>View only</strong> — these files are provided for reading only.
            Downloading or saving is not permitted per the sender's settings.
          </div>
        </div>
      )}

      <h2 className="tenzo-title">{transfer.name || 'Files ready'}</h2>
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
        {accessesLeft !== null && (
          <span className={`chip ${accessesLeft > 0 ? 'chip-teal' : ''}`}>
            {accessesLeft} {viewOnly ? 'view' : 'download'}{accessesLeft !== 1 ? 's' : ''} remaining
          </span>
        )}
        <span className="chip">
          {fileEntries.length} file{fileEntries.length !== 1 ? 's' : ''}
        </span>
        {transfer.total_size_bytes > 0 && (
          <span className="chip">{fmtBytes(transfer.total_size_bytes)}</span>
        )}
        {viewOnly && (
          <span className="chip" style={{ background: 'rgba(99,102,241,0.1)', color: '#3730a3', border: '1px solid rgba(99,102,241,0.25)' }}>
            👁 View only
          </span>
        )}
      </div>

      <hr className="tenzo-divider" />

      <ul className="file-list">
        {fileEntries.map((f) => {
          const fid = f.id;
          const isRich = 'filename' in f && f.filename !== undefined;
          const deleteReason = isRich ? (f as FileInfo).delete_reason : '';
          const isDeleted = !!deleteReason;
          const isDisabled = !!downloading[fid] || transfer.is_revoked || accessesLeft === 0 || isDeleted;
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
              <div style={{ display: 'flex', gap: 6 }}>
                {/* Preview button for non-view-only previewable file types */}
                {!viewOnly && !isDeleted && isRich && canPreview((f as FileInfo).content_type) && (
                  <button
                    className="btn btn-secondary btn-sm"
                    onClick={() => handlePreview(fid)}
                    disabled={isDisabled}
                    title="Preview file"
                    aria-label="Preview file"
                  >
                    <IconEye />
                  </button>
                )}
                <button
                  className={`btn ${isDeleted ? 'btn-secondary' : 'btn-primary'} btn-sm`}
                  onClick={() => !isDeleted && handleDownload(fid)}
                  disabled={isDisabled}
                  title={isDeleted ? deletedTitle : (viewOnly ? 'View file (view only mode)' : 'Download file')}
                >
                  {downloading[fid] ? (
                    <>
                      <span className="spinner" style={{ width: 12, height: 12, borderWidth: 1.5 }} />
                      {viewOnly ? 'Opening…' : 'Preparing…'}
                    </>
                  ) : isDeleted ? (
                    'Unavailable'
                  ) : viewOnly ? (
                    <>
                      <IconEye />
                      View
                    </>
                  ) : (
                    <>
                      <IconDownload />
                      Download
                    </>
                  )}
                </button>
              </div>
            </li>
          );
        })}
      </ul>

      <TenzoFooter viewOnly={viewOnly} />
    </Layout>
    {viewer && (
      <FileViewerModal viewer={viewer} onClose={() => setViewer(null)} />
    )}
  </>
  );
}

// ─── Layout ────────────────────────────────────────────────────────────────

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="tenzo-page">
      <div className="tenzo-card">
        <div className="tenzo-brand">
          <div className="tenzo-brand-icon">
            <img src="/logo.png" alt="TenzoShare" style={{ width: 32, height: 32, objectFit: 'contain' }} />
          </div>
          <span className="tenzo-brand-name">TenzoShare</span>
        </div>
        {children}
      </div>
    </div>
  );
}

// ─── File Viewer Modal ─────────────────────────────────────────────────────

function FileViewerModal({
  viewer,
  onClose,
}: {
  viewer: ViewerModal;
  onClose: () => void;
}) {
  const { viewUrl, downloadUrl, filename, contentType, sizeBytes, viewOnly } = viewer;
  const t = contentType.split(';')[0].trim().toLowerCase();
  const isImage = t.startsWith('image/');
  const isPDF = t === 'application/pdf';
  const isAudio = t.startsWith('audio/');
  const isVideo = t.startsWith('video/');
  const isText =
    t.startsWith('text/') ||
    t === 'application/json' ||
    t === 'application/javascript' ||
    t === 'application/xml';

  const TEXT_SIZE_LIMIT = 500 * 1024; // 500 KB
  const [textContent, setTextContent] = useState<string | null>(null);
  const [textLoading, setTextLoading] = useState(false);
  const [textError, setTextError] = useState<string | null>(null);

  useEffect(() => {
    if (!isText) return;
    if (sizeBytes > TEXT_SIZE_LIMIT) {
      setTextError(`File is too large to preview inline (${fmtBytes(sizeBytes)}). Download it to view.`);
      return;
    }
    setTextLoading(true);
    fetch(viewUrl)
      .then((r) => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.text(); })
      .then((text) => { setTextContent(text); setTextLoading(false); })
      .catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : 'Unknown error';
        setTextError(`Could not load file content: ${msg}`);
        setTextLoading(false);
      });
  }, [viewUrl, isText, sizeBytes]);

  // ESC to close
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const handleModalDownload = () => {
    const a = document.createElement('a');
    a.href = downloadUrl;
    a.download = filename;
    a.rel = 'noopener noreferrer';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  const isUnsupported = !isImage && !isPDF && !isText && !isAudio && !isVideo;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={`Viewing ${filename}`}
      style={{
        position: 'fixed', inset: 0, zIndex: 1000,
        background: 'rgba(0,0,0,0.72)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        padding: 16,
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div
        style={{
          background: '#fff',
          borderRadius: 12,
          boxShadow: '0 24px 64px rgba(0,0,0,0.45)',
          display: 'flex',
          flexDirection: 'column',
          width: '100%',
          maxWidth: isPDF ? 960 : isImage ? 860 : isVideo ? 860 : 720,
          maxHeight: '92vh',
          overflow: 'hidden',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', gap: 10,
          padding: '12px 14px',
          borderBottom: '1px solid #f0f0f0',
          flexShrink: 0,
        }}>
          <span style={{
            flex: 1, fontSize: 13, fontWeight: 600, color: '#111827',
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}>
            {filename}
          </span>
          {viewOnly && (
            <span style={{
              fontSize: 10, fontWeight: 700, letterSpacing: '0.06em',
              background: 'rgba(99,102,241,0.1)', color: '#3730a3',
              border: '1px solid rgba(99,102,241,0.2)',
              borderRadius: 5, padding: '2px 8px', flexShrink: 0,
            }}>
              VIEW ONLY
            </span>
          )}
          <button
            onClick={onClose}
            aria-label="Close viewer"
            style={{
              background: 'none', border: 'none', cursor: 'pointer',
              padding: '4px 6px', borderRadius: 6,
              color: '#6b7280', display: 'flex', alignItems: 'center',
              flexShrink: 0,
            }}
          >
            <IconClose />
          </button>
        </div>

        {/* Content */}
        <div style={{ flex: 1, minHeight: 0, overflow: 'hidden', position: 'relative' }}>
          {isImage && (
            <div style={{
              width: '100%', height: '100%', minHeight: 300,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              padding: 16, boxSizing: 'border-box',
              background: '#f9fafb', overflow: 'auto',
            }}>
              <img
                src={viewUrl}
                alt={filename}
                style={{ maxWidth: '100%', maxHeight: '75vh', objectFit: 'contain', borderRadius: 4 }}
              />
            </div>
          )}

          {isPDF && (
            <iframe
              src={viewUrl}
              title={filename}
              style={{ width: '100%', height: '76vh', border: 'none', display: 'block' }}
            />
          )}

          {isText && (
            <div style={{ overflow: 'auto', maxHeight: '75vh' }}>
              {textLoading && (
                <div style={{ padding: 32, textAlign: 'center', color: '#6b7280', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10 }}>
                  <span className="spinner" /> Loading…
                </div>
              )}
              {textError && (
                <div style={{ padding: 24, color: '#b91c1c', fontSize: 13 }}>{textError}</div>
              )}
              {textContent !== null && (
                <pre style={{
                  margin: 0, padding: '16px 20px',
                  fontSize: 12.5,
                  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                  whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                  color: '#1f2937', lineHeight: 1.65,
                  background: '#f8fafc', minHeight: '30vh',
                }}>
                  {textContent}
                </pre>
              )}
            </div>
          )}

          {isAudio && (
            <div style={{ padding: 40, display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
              <audio controls src={viewUrl} style={{ width: '100%', maxWidth: 520 }} />
            </div>
          )}

          {isVideo && (
            <div style={{ background: '#000', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <video
                controls
                src={viewUrl}
                style={{ maxWidth: '100%', maxHeight: '75vh', display: 'block' }}
              />
            </div>
          )}

          {isUnsupported && (
            <div style={{ padding: 48, textAlign: 'center', color: '#6b7280' }}>
              <p style={{ marginBottom: 12 }}>Preview is not available for this file type.</p>
              {!viewOnly && (
                <button onClick={handleModalDownload} className="btn btn-primary btn-sm">
                  <IconDownload /> Download to view
                </button>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div style={{
          display: 'flex', justifyContent: 'flex-end', gap: 8,
          padding: '10px 14px',
          borderTop: '1px solid #f0f0f0',
          flexShrink: 0,
        }}>
          {!viewOnly && (
            <button onClick={handleModalDownload} className="btn btn-secondary btn-sm">
              <IconDownload /> Download
            </button>
          )}
          <button onClick={onClose} className="btn btn-primary btn-sm">
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

// ─── Footer ────────────────────────────────────────────────────────────────

function TenzoFooter({ viewOnly }: { viewOnly?: boolean }) {
  return (
    <div className="tenzo-footer">
      <img src="/logo.png" alt="" style={{ width: 14, height: 14, objectFit: 'contain' }} />
      {viewOnly
        ? 'Files are encrypted and served securely in view-only mode via TenzoShare'
        : 'Files are encrypted and served securely via TenzoShare'}
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

