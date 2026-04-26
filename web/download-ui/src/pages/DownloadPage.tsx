import { useState, useEffect, useCallback } from 'react';
import type { TransferPublic } from '../types';
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

// ---------------------------------------------------------------------------
// Slug resolution
// ---------------------------------------------------------------------------

/**
 * The share link can be in any of these forms:
 *   https://your-host/t/AbCdEfGh1234
 *   https://your-host/download/AbCdEfGh1234
 *   https://your-host/?slug=AbCdEfGh1234
 *
 * We try each in order so that deployments can choose their own URL structure
 * by routing requests to this SPA.
 */
function resolveSlug(): string | null {
  const path = window.location.pathname;
  // /t/<slug> or /download/<slug>
  const pathMatch = path.match(/\/(?:t|download)\/([^/]+)/);
  if (pathMatch) return pathMatch[1];
  // ?slug=<slug>
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
        a.download = fileId;
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
        <span className="chip">Expires {formatDate(transfer.expires_at)}</span>
        {downloadsLeft !== null && (
          <span className={`chip ${downloadsLeft > 0 ? 'chip-teal' : ''}`}>
            {downloadsLeft} download{downloadsLeft !== 1 ? 's' : ''} remaining
          </span>
        )}
        {transfer.file_ids.length > 0 && (
          <span className="chip">
            {transfer.file_ids.length} file{transfer.file_ids.length !== 1 ? 's' : ''}
          </span>
        )}
      </div>

      <hr className="tenzo-divider" />

      {/* File list */}
      <ul className="file-list">
        {transfer.file_ids.map((fid) => (
          <li key={fid} className="file-item">
            <div className="file-icon">
              <IconFile />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="file-name">{fid}</div>
              {downloadErrors[fid] && (
                <div style={{ fontSize: 11, color: 'var(--color-error-text)', marginTop: 2 }}>
                  {downloadErrors[fid]}
                </div>
              )}
            </div>
            <button
              className="btn btn-primary btn-sm"
              onClick={() => handleDownload(fid)}
              disabled={!!downloading[fid] || transfer.is_revoked || downloadsLeft === 0}
            >
              {downloading[fid] ? (
                <>
                  <span className="spinner" style={{ width: 12, height: 12, borderWidth: 1.5 }} />
                  Preparing…
                </>
              ) : (
                <>
                  <IconDownload />
                  Download
                </>
              )}
            </button>
          </li>
        ))}
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

