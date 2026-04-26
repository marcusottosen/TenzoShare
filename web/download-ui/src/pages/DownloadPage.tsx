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
  const [downloading, setDownloading] = useState<Record<string, boolean>>({});

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
      setView({ kind: 'loading' });
      fetchTransfer(view.slug, passwordInput)
        .then((t) => setView({ kind: 'ready', transfer: t, password: passwordInput }))
        .catch((err: unknown) => {
          const message =
            err instanceof TransferApiError ? err.message : 'Incorrect password.';
          setView({ kind: 'password', slug: view.slug });
          // show inline error without unmounting the form
          alert(message);
        });
    },
    [view, passwordInput],
  );

  // Trigger download for a single file.
  const handleDownload = useCallback(
    async (fileId: string, fileName: string) => {
      if (!slug) return;
      const password = view.kind === 'ready' ? view.password : undefined;
      setDownloading((d) => ({ ...d, [fileId]: true }));
      try {
        const { url } = await fetchDownloadUrl(slug, fileId, password);
        // Trigger browser download via a transient <a> — works cross-browser
        // without popups being blocked (it's inside a user-gesture handler).
        const a = document.createElement('a');
        a.href = url;
        a.download = fileName;
        a.rel = 'noopener noreferrer';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
      } catch (err: unknown) {
        const message =
          err instanceof TransferApiError ? err.message : 'Download failed.';
        alert(message);
      } finally {
        setDownloading((d) => ({ ...d, [fileId]: false }));
      }
    },
    [slug, view],
  );

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  if (view.kind === 'loading') {
    return <Layout><p style={styles.muted}>Loading…</p></Layout>;
  }

  if (view.kind === 'error') {
    return (
      <Layout>
        <div style={styles.errorBox}>
          <strong>{errorTitle(view.status)}</strong>
          <p style={{ margin: '8px 0 0' }}>{view.message}</p>
        </div>
      </Layout>
    );
  }

  if (view.kind === 'password') {
    return (
      <Layout>
        <h2 style={styles.heading}>Password required</h2>
        <p style={styles.muted}>This transfer is password-protected.</p>
        <form onSubmit={handlePasswordSubmit} style={styles.form}>
          <input
            type="password"
            placeholder="Enter password"
            value={passwordInput}
            onChange={(e) => setPasswordInput(e.target.value)}
            autoFocus
            required
            style={styles.input}
          />
          <button type="submit" style={styles.btnPrimary}>
            Unlock
          </button>
        </form>
      </Layout>
    );
  }

  // view.kind === 'ready'
  const { transfer } = view;

  return (
    <Layout>
      <h2 style={styles.heading}>{transfer.name || 'Files ready to download'}</h2>
      {transfer.description && (
        <p style={{ ...styles.muted, marginBottom: 20 }}>{transfer.description}</p>
      )}

      {/* Transfer metadata */}
      <dl style={styles.meta}>
        <dt style={styles.dt}>Expires</dt>
        <dd style={styles.dd}>{formatDate(transfer.expires_at)}</dd>
        {transfer.max_downloads > 0 && (
          <>
            <dt style={styles.dt}>Downloads left</dt>
            <dd style={styles.dd}>
              {Math.max(0, transfer.max_downloads - transfer.download_count)}
              {' / '}
              {transfer.max_downloads}
            </dd>
          </>
        )}
      </dl>

      {/* File list */}
      <ul style={styles.fileList}>
        {transfer.file_ids.map((fid) => (
          <li key={fid} style={styles.fileItem}>
            <span style={styles.fileId}>{fid}</span>
            <button
              onClick={() => handleDownload(fid, fid)}
              disabled={!!downloading[fid]}
              style={downloading[fid] ? styles.btnDisabled : styles.btnPrimary}
            >
              {downloading[fid] ? 'Preparing…' : 'Download'}
            </button>
          </li>
        ))}
      </ul>

      {transfer.is_revoked && (
        <div style={styles.errorBox}>This transfer has been revoked by the sender.</div>
      )}
    </Layout>
  );
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div style={styles.page}>
      <header style={styles.header}>
        <span style={styles.logo}>TenzoShare</span>
      </header>
      <main style={styles.main}>{children}</main>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function errorTitle(status?: number): string {
  if (status === 404) return 'Transfer not found';
  if (status === 403) return 'Access denied';
  if (status === 401) return 'Authentication required';
  return 'Something went wrong';
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

// ---------------------------------------------------------------------------
// Inline styles — intentionally plain so there are zero build-time deps.
// Replace with any CSS solution you prefer.
// ---------------------------------------------------------------------------

const styles = {
  page: {
    minHeight: '100vh',
    fontFamily: 'system-ui, sans-serif',
    background: '#f5f5f5',
    color: '#111',
  } as React.CSSProperties,

  header: {
    background: '#0f172a',
    padding: '12px 24px',
  } as React.CSSProperties,

  logo: {
    color: '#fff',
    fontWeight: 700,
    fontSize: '1.1rem',
    letterSpacing: '-0.01em',
  } as React.CSSProperties,

  main: {
    maxWidth: 560,
    margin: '48px auto',
    padding: '0 16px',
  } as React.CSSProperties,

  heading: {
    fontSize: '1.4rem',
    fontWeight: 600,
    margin: '0 0 16px',
  } as React.CSSProperties,

  muted: {
    color: '#666',
    margin: 0,
  } as React.CSSProperties,

  meta: {
    display: 'grid',
    gridTemplateColumns: 'max-content 1fr',
    gap: '4px 16px',
    margin: '0 0 24px',
    fontSize: '0.9rem',
  } as React.CSSProperties,

  dt: {
    color: '#555',
    fontWeight: 500,
  } as React.CSSProperties,

  dd: {
    margin: 0,
  } as React.CSSProperties,

  form: {
    display: 'flex',
    gap: 8,
    marginTop: 16,
  } as React.CSSProperties,

  input: {
    flex: 1,
    padding: '8px 12px',
    border: '1px solid #ccc',
    borderRadius: 6,
    fontSize: '1rem',
  } as React.CSSProperties,

  fileList: {
    listStyle: 'none',
    margin: 0,
    padding: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  } as React.CSSProperties,

  fileItem: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    background: '#fff',
    border: '1px solid #e2e8f0',
    borderRadius: 8,
    padding: '12px 16px',
  } as React.CSSProperties,

  fileId: {
    fontFamily: 'monospace',
    fontSize: '0.8rem',
    color: '#444',
    wordBreak: 'break-all',
    flex: 1,
    marginRight: 12,
  } as React.CSSProperties,

  btnPrimary: {
    padding: '8px 18px',
    background: '#0f172a',
    color: '#fff',
    border: 'none',
    borderRadius: 6,
    cursor: 'pointer',
    fontWeight: 500,
    fontSize: '0.9rem',
    whiteSpace: 'nowrap',
  } as React.CSSProperties,

  btnDisabled: {
    padding: '8px 18px',
    background: '#94a3b8',
    color: '#fff',
    border: 'none',
    borderRadius: 6,
    cursor: 'not-allowed',
    fontWeight: 500,
    fontSize: '0.9rem',
    whiteSpace: 'nowrap',
  } as React.CSSProperties,

  errorBox: {
    background: '#fef2f2',
    border: '1px solid #fca5a5',
    borderRadius: 8,
    padding: '16px',
    color: '#991b1b',
    fontSize: '0.95rem',
  } as React.CSSProperties,
} as const;
