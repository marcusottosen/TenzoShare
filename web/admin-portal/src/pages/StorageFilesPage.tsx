import React, { useCallback, useEffect, useState } from 'react';
import {
  adminDeleteFile,
  getPurgeLog,
  listStorageFiles,
  triggerPurge,
  type AdminFileRow,
  type PurgeLogEntry,
} from '../api/admin';

// ── Helpers ──────────────────────────────────────────────────────────────────

function fmtBytes(b: number): string {
  if (!b) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(b) / Math.log(k));
  return `${(b / Math.pow(k, i)).toFixed(i <= 1 ? 0 : 1)} ${sizes[i]}`;
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

type Filter = 'all' | 'orphan' | 'eligible';
type SortBy = 'created_at' | 'size_bytes' | 'filename' | 'owner' | 'shares';
type SortDir = 'asc' | 'desc';

// ── Status badge ─────────────────────────────────────────────────────────────

function StatusBadge({ row }: { row: AdminFileRow }) {
  if (row.eligible_purge) {
    return (
      <span style={{ padding: '2px 8px', borderRadius: 12, fontSize: 12, background: '#fef3c7', color: '#92400e', fontWeight: 600 }}>
        Eligible for purge
      </span>
    );
  }
  if (row.share_count === 0) {
    return (
      <span style={{ padding: '2px 8px', borderRadius: 12, fontSize: 12, background: '#e0e7ff', color: '#3730a3', fontWeight: 600 }}>
        Orphan
      </span>
    );
  }
  if (row.active_shares > 0) {
    return (
      <span style={{ padding: '2px 8px', borderRadius: 12, fontSize: 12, background: '#d1fae5', color: '#065f46', fontWeight: 600 }}>
        Active
      </span>
    );
  }
  return (
    <span style={{ padding: '2px 8px', borderRadius: 12, fontSize: 12, background: '#f3f4f6', color: '#6b7280', fontWeight: 600 }}>
      Expired
    </span>
  );
}

// ── Files table ──────────────────────────────────────────────────────────────

export default function StorageFilesPage() {
  const PAGE_SIZE = 50;

  const [files, setFiles] = useState<AdminFileRow[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [filter, setFilter] = useState<Filter>('all');
  const [sortBy, setSortBy] = useState<SortBy>('created_at');
  const [sortDir, setSortDir] = useState<SortDir>('desc');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [deleteConfirm, setDeleteConfirm] = useState<AdminFileRow | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [purgeConfirm, setPurgeConfirm] = useState(false);
  const [purging, setPurging] = useState(false);
  const [purgeResult, setPurgeResult] = useState<{ deleted: number; freed_bytes: number; capped?: boolean; cap?: number } | null>(null);
  const [purgeError, setPurgeError] = useState('');

  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' | 'warning' } | null>(null);

  // Auto-dismiss toast after 5 seconds
  React.useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 5000);
    return () => clearTimeout(t);
  }, [toast]);

  const [logEntries, setLogEntries] = useState<PurgeLogEntry[]>([]);
  const [logTotal, setLogTotal] = useState(0);
  const [logLoading, setLogLoading] = useState(true);

  const loadFiles = useCallback(() => {
    setLoading(true);
    setError('');
    listStorageFiles({ limit: PAGE_SIZE, offset, filter, sort_by: sortBy, sort_dir: sortDir })
      .then((r) => { setFiles(r.files ?? []); setTotal(r.total); })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [offset, filter, sortBy, sortDir]);

  const loadLog = useCallback(() => {
    setLogLoading(true);
    getPurgeLog({ limit: 50 })
      .then((r) => { setLogEntries(r.entries ?? []); setLogTotal(r.total); })
      .catch(() => {/* silent */})
      .finally(() => setLogLoading(false));
  }, []);

  useEffect(() => { loadFiles(); }, [loadFiles]);
  useEffect(() => { loadLog(); }, [loadLog]);

  function handleSort(col: SortBy) {
    if (sortBy === col) setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    else { setSortBy(col); setSortDir('desc'); }
    setOffset(0);
  }

  function SortIcon({ col }: { col: SortBy }) {
    if (sortBy !== col) return <span style={{ color: '#9ca3af', marginLeft: 4 }}>↕</span>;
    return <span style={{ marginLeft: 4 }}>{sortDir === 'asc' ? '↑' : '↓'}</span>;
  }

  async function confirmDelete() {
    if (!deleteConfirm) return;
    setDeleting(true);
    try {
      await adminDeleteFile(deleteConfirm.id);
      const name = deleteConfirm.filename;
      setDeleteConfirm(null);
      loadFiles();
      loadLog();
      setToast({ message: `"${name}" has been deleted.`, type: 'success' });
    } catch (e: unknown) {
      setError((e as Error).message);
      setDeleteConfirm(null);
    } finally {
      setDeleting(false);
    }
  }

  async function handlePurge() {
    setPurging(true);
    setPurgeResult(null);
    setPurgeError('');
    setPurgeConfirm(false);
    try {
      const r = await triggerPurge();
      setPurgeResult(r);
      loadFiles();
      loadLog();
      if (r.capped) {
        setToast({
          message: `Purge complete: ${r.deleted} files deleted, ${fmtBytes(r.freed_bytes)} freed. ⚠ Result was capped at ${r.cap} files — run again to continue.`,
          type: 'warning',
        });
      } else {
        setToast({
          message: `Purge complete: ${r.deleted} file${r.deleted !== 1 ? 's' : ''} deleted, ${fmtBytes(r.freed_bytes)} freed.`,
          type: 'success',
        });
      }
    } catch (e: unknown) {
      const msg = (e as Error).message;
      setPurgeError(msg);
      setToast({ message: `Purge failed: ${msg}`, type: 'error' });
    } finally {
      setPurging(false);
    }
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  return (
    <div className="page">
      {/* ── Header ─────────────────────────────────────────────────────── */}
      <div className="page-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 className="page-title">Storage Files</h1>
          <p className="page-subtitle">Manage all files stored in the system</p>
        </div>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
          <button className="btn btn-secondary" onClick={() => setPurgeConfirm(true)} disabled={purging}>
            {purging ? 'Purging…' : 'Run Purge Now'}
          </button>
        </div>
      </div>

      {/* ── Filters ────────────────────────────────────────────────────── */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        {(['all', 'orphan', 'eligible'] as Filter[]).map((f) => (
          <button key={f} type="button"
            className={`btn btn-sm ${filter === f ? 'btn-primary' : 'btn-secondary'}`}
            onClick={() => { setFilter(f); setOffset(0); }}>
            {f === 'all' ? 'All Files' : f === 'orphan' ? 'Never Shared' : 'Eligible for Purge'}
          </button>
        ))}
        <span className="text-sm" style={{ marginLeft: 'auto', alignSelf: 'center', color: 'var(--color-text-muted)' }}>
          {total} file{total !== 1 ? 's' : ''}
        </span>
      </div>

      {/* ── Files table ────────────────────────────────────────────────── */}
      {error && <div className="alert alert-error" style={{ marginBottom: 12 }}>{error}</div>}

      <div className="card" style={{ padding: 0, overflow: 'hidden', marginBottom: 20 }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--color-border)', background: 'var(--color-nav-active)' }}>
              <th style={thStyle} onClick={() => handleSort('filename')}>
                File <SortIcon col="filename" />
              </th>
              <th style={thStyle} onClick={() => handleSort('owner')}>
                Owner <SortIcon col="owner" />
              </th>
              <th style={{ ...thStyle, textAlign: 'right' }} onClick={() => handleSort('size_bytes')}>
                Size <SortIcon col="size_bytes" />
              </th>
              <th style={{ ...thStyle, textAlign: 'right' }} onClick={() => handleSort('shares')}>
                Shares <SortIcon col="shares" />
              </th>
              <th style={thStyle} onClick={() => handleSort('created_at')}>
                Uploaded <SortIcon col="created_at" />
              </th>
              <th style={thStyle}>Expires</th>
              <th style={thStyle}>Status</th>
              <th style={{ ...thStyle, textAlign: 'center' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={8} style={{ padding: 24, textAlign: 'center', color: 'var(--color-text-muted)' }}>Loading…</td></tr>
            ) : files.length === 0 ? (
              <tr><td colSpan={8} style={{ padding: 24, textAlign: 'center', color: 'var(--color-text-muted)' }}>No files found</td></tr>
            ) : files.map((f) => (
              <tr key={f.id} style={{ borderBottom: '1px solid var(--color-border)' }}
                onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--color-nav-active)')}
                onMouseLeave={(e) => (e.currentTarget.style.background = '')}>
                <td style={tdStyle}>
                  <div style={{ fontWeight: 500, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={f.filename}>
                    {f.filename}
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--color-text-muted)', fontFamily: 'monospace' }}>
                    {f.id.substring(0, 8)}…
                  </div>
                </td>
                <td style={tdStyle}>
                  <div style={{ maxWidth: 160, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={f.owner_email}>
                    {f.owner_email || <span style={{ color: 'var(--color-text-muted)' }}>—</span>}
                  </div>
                </td>
                <td style={{ ...tdStyle, textAlign: 'right' }}>{fmtBytes(f.size_bytes)}</td>
                <td style={{ ...tdStyle, textAlign: 'right' }}>
                  {f.share_count > 0 ? (
                    <span title={`${f.active_shares} active, ${f.share_count - f.active_shares} expired/revoked`}>
                      {f.share_count} <span style={{ color: 'var(--color-text-muted)', fontSize: 11 }}>({f.active_shares} active)</span>
                    </span>
                  ) : '—'}
                </td>
                <td style={tdStyle}>{fmtDate(f.created_at)}</td>
                <td style={tdStyle}>
                  {f.last_share_expires_at
                    ? fmtDate(f.last_share_expires_at)
                    : <span style={{ color: 'var(--color-text-muted)' }}>—</span>}
                </td>
                <td style={tdStyle}><StatusBadge row={f} /></td>
                <td style={{ ...tdStyle, textAlign: 'center' }}>
                  <button className="btn btn-sm btn-danger" onClick={() => setDeleteConfirm(f)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        {/* Pagination */}
        {totalPages > 1 && (
          <div style={{ display: 'flex', justifyContent: 'center', gap: 8, padding: 12, borderTop: '1px solid var(--color-border)' }}>
            <button className="btn btn-sm btn-secondary" disabled={currentPage === 1} onClick={() => setOffset(0)}>«</button>
            <button className="btn btn-sm btn-secondary" disabled={currentPage === 1} onClick={() => setOffset(o => Math.max(0, o - PAGE_SIZE))}>‹ Prev</button>
            <span className="text-sm" style={{ alignSelf: 'center', color: 'var(--color-text-muted)' }}>
              Page {currentPage} of {totalPages}
            </span>
            <button className="btn btn-sm btn-secondary" disabled={currentPage === totalPages} onClick={() => setOffset(o => o + PAGE_SIZE)}>Next ›</button>
            <button className="btn btn-sm btn-secondary" disabled={currentPage === totalPages} onClick={() => setOffset((totalPages - 1) * PAGE_SIZE)}>»</button>
          </div>
        )}
      </div>

      {/* ── Purge Log ──────────────────────────────────────────────────── */}
      <div className="card" style={{ marginBottom: 20 }}>
        <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <h2 style={{ fontSize: 15, fontWeight: 600, margin: 0, color: 'var(--color-text-primary)' }}>
              Purge Log
            </h2>
            <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
              Audit trail of all deleted files ({logTotal} total entries)
            </p>
          </div>
        </div>
        {logLoading ? (
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>
        ) : logEntries.length === 0 ? (
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>No purge events recorded yet.</p>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--color-border)', background: 'var(--color-nav-active)' }}>
                <th style={thStyle}>File</th>
                <th style={thStyle}>Owner</th>
                <th style={{ ...thStyle, textAlign: 'right' }}>Size</th>
                <th style={thStyle}>Reason</th>
                <th style={thStyle}>Purged By</th>
                <th style={thStyle}>Purged At</th>
              </tr>
            </thead>
            <tbody>
              {logEntries.map((e) => (
                <tr key={`${e.file_id}-${e.purged_at}`} style={{ borderBottom: '1px solid var(--color-border)' }}>
                  <td style={tdStyle}>
                    <div style={{ fontWeight: 500 }}>{e.filename}</div>
                    <div style={{ fontSize: 11, color: 'var(--color-text-muted)', fontFamily: 'monospace' }}>{e.file_id.substring(0, 8)}…</div>
                  </td>
                  <td style={tdStyle}>{e.email}</td>
                  <td style={{ ...tdStyle, textAlign: 'right' }}>{fmtBytes(e.size_bytes)}</td>
                  <td style={tdStyle}>
                    <span style={reasonStyle(e.reason)}>{reasonLabel(e.reason)}</span>
                  </td>
                  <td style={tdStyle}>{e.purged_by}</td>
                  <td style={tdStyle}>{fmtDate(e.purged_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* ── Delete confirm modal ────────────────────────────────────────── */}
      {deleteConfirm && (
        <div style={overlayStyle}>
          <div style={modalStyle}>
            <h3 style={{ marginTop: 0, fontSize: 16, fontWeight: 600 }}>Delete File?</h3>
            <p style={{ fontSize: 14, color: 'var(--color-text-muted)', margin: '8px 0 20px' }}>
              <strong>{deleteConfirm.filename}</strong> ({fmtBytes(deleteConfirm.size_bytes)}) will be permanently
              removed from storage. This cannot be undone.
              {deleteConfirm.active_shares > 0 && (
                <span style={{ display: 'block', marginTop: 8, color: '#dc2626', fontWeight: 600 }}>
                  Warning: This file has {deleteConfirm.active_shares} active share{deleteConfirm.active_shares !== 1 ? 's' : ''}.
                  Recipients will no longer be able to download it.
                </span>
              )}
            </p>
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button className="btn btn-secondary" onClick={() => setDeleteConfirm(null)} disabled={deleting}>Cancel</button>
              <button className="btn btn-danger" onClick={confirmDelete} disabled={deleting}>
                {deleting ? 'Deleting…' : 'Delete Permanently'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Purge confirm modal ─────────────────────────────────────────── */}
      {purgeConfirm && (
        <div style={overlayStyle}>
          <div style={modalStyle}>
            <h3 style={{ marginTop: 0, fontSize: 16, fontWeight: 600 }}>Run Purge Now?</h3>
            <p style={{ fontSize: 14, color: 'var(--color-text-muted)', margin: '8px 0 8px' }}>
              This will immediately soft-delete all files that are eligible for retention removal:
            </p>
            <ul style={{ fontSize: 13, color: 'var(--color-text-muted)', margin: '0 0 12px 20px', lineHeight: '1.7' }}>
              <li>Files whose last share expired more than <strong>retention_days</strong> ago</li>
              <li>Orphan files (never shared) older than <strong>orphan_retention_days</strong></li>
            </ul>
            <p style={{ fontSize: 13, color: '#92400e', background: '#fffbeb', border: '1px solid #fde68a', borderRadius: 6, padding: '8px 12px', margin: '0 0 20px' }}>
              Up to <strong>500 files</strong> will be processed per run. The cleanup worker will remove
              the actual objects from storage within the next hour.
            </p>
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button className="btn btn-secondary" onClick={() => setPurgeConfirm(false)} disabled={purging}>Cancel</button>
              <button className="btn btn-danger" onClick={handlePurge} disabled={purging}>
                {purging ? 'Purging…' : 'Confirm — Run Purge'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Toast notification ──────────────────────────────────────────── */}
      {toast && (
        <div style={{
          position: 'fixed',
          bottom: 28,
          right: 28,
          zIndex: 2000,
          maxWidth: 420,
          padding: '14px 18px',
          borderRadius: 10,
          boxShadow: '0 8px 24px rgba(0,0,0,0.18)',
          display: 'flex',
          alignItems: 'flex-start',
          gap: 12,
          background: toast.type === 'success' ? '#ecfdf5' : toast.type === 'error' ? '#fef2f2' : '#fffbeb',
          border: `1px solid ${toast.type === 'success' ? '#a7f3d0' : toast.type === 'error' ? '#fecaca' : '#fde68a'}`,
          color: toast.type === 'success' ? '#065f46' : toast.type === 'error' ? '#991b1b' : '#92400e',
          fontSize: 13,
          fontWeight: 500,
        }}>
          <span style={{ flexShrink: 0, marginTop: 1 }}>
            {toast.type === 'success' ? '✓' : toast.type === 'error' ? '✕' : '⚠'}
          </span>
          <span style={{ flex: 1 }}>{toast.message}</span>
          <button onClick={() => setToast(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontSize: 16, lineHeight: 1, opacity: 0.6, flexShrink: 0, marginTop: -1 }}>×</button>
        </div>
      )}
    </div>
  );
}

// ── Styles ────────────────────────────────────────────────────────────────────

const thStyle: React.CSSProperties = {
  padding: '10px 14px',
  textAlign: 'left',
  fontSize: 12,
  fontWeight: 600,
  color: 'var(--color-text-muted)',
  cursor: 'pointer',
  userSelect: 'none',
  whiteSpace: 'nowrap',
};

const tdStyle: React.CSSProperties = {
  padding: '10px 14px',
  verticalAlign: 'middle',
};

const overlayStyle: React.CSSProperties = {
  position: 'fixed',
  inset: 0,
  background: 'rgba(0,0,0,0.4)',
  zIndex: 1000,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const modalStyle: React.CSSProperties = {
  background: 'var(--color-surface)',
  borderRadius: 12,
  padding: 24,
  maxWidth: 480,
  width: '90%',
  boxShadow: '0 20px 40px rgba(0,0,0,0.3)',
};

function reasonLabel(r: string): string {
  if (r === 'retention_expired') return 'Share expired';
  if (r === 'orphan_expired') return 'Orphan';
  if (r === 'admin_purge') return 'Admin purge';
  return r;
}

function reasonStyle(r: string): React.CSSProperties {
  const base: React.CSSProperties = { padding: '2px 8px', borderRadius: 12, fontSize: 12, fontWeight: 600 };
  if (r === 'admin_purge') return { ...base, background: '#fee2e2', color: '#991b1b' };
  if (r === 'orphan_expired') return { ...base, background: '#e0e7ff', color: '#3730a3' };
  return { ...base, background: '#fef3c7', color: '#92400e' };
}
