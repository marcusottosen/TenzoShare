import React, { useEffect, useState, useCallback } from 'react';
import { useLocation } from 'react-router';
import { fmt, fmtDate } from '../utils/dateFormat';
import {
  listTransfers, getTransfer, revokeTransfer,
  type AdminTransfer, type AdminTransferDetail, type TransferFile,
} from '../api/admin';
import { useSortState } from '../hooks/useSort';
import { SortHeader } from '../components/SortHeader';

type TransferSortKey = 'owner_email' | 'name' | 'status' | 'file_count' | 'total_size_bytes' | 'download_count' | 'expires_at' | 'created_at';

const PAGE_SIZE = 50;

// fmt / fmtDate imported from utils/dateFormat

function fmtBytes(bytes: number): string {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

function StatusBadge({ status }: { status: AdminTransfer['status'] }) {
  const cls =
    status === 'active' ? 'badge badge-green' :
    status === 'exhausted' ? 'badge badge-orange' :
    status === 'expired' ? 'badge badge-gray' :
    'badge badge-red';
  return <span className={cls}>{status}</span>;
}

function FileIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, color: 'var(--color-text-muted)' }}>
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
      <polyline points="14 2 14 8 20 8"/>
    </svg>
  );
}

// ── Transfer Detail Modal ────────────────────────────────────────────────────

interface DetailModalProps {
  transfer: AdminTransfer;
  onClose: () => void;
  onRevoked: (id: string) => void;
}

function DetailModal({ transfer: initialTransfer, onClose, onRevoked }: DetailModalProps) {
  const [detail, setDetail] = useState<AdminTransferDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [revoking, setRevoking] = useState(false);
  const [revokeError, setRevokeError] = useState('');
  const [transfer, setTransfer] = useState(initialTransfer);

  useEffect(() => {
    getTransfer(transfer.id)
      .then(setDetail)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [transfer.id]);

  async function handleRevoke() {
    if (!confirm(`Revoke transfer "${transfer.name}"? Recipients will immediately lose access.`)) return;
    setRevoking(true);
    setRevokeError('');
    try {
      await revokeTransfer(transfer.id);
      setTransfer((t) => ({ ...t, status: 'revoked', is_revoked: true }));
      if (detail) setDetail({ ...detail, status: 'revoked', is_revoked: true });
      onRevoked(transfer.id);
    } catch (e: unknown) {
      setRevokeError((e as Error).message);
    } finally {
      setRevoking(false);
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal"
        style={{ maxWidth: 600 }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {transfer.name}
            </span>
            <StatusBadge status={transfer.status} />
          </div>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>

        <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          {revokeError && <div className="alert alert-error">{revokeError}</div>}

          {/* Meta grid */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px 24px' }}>
            <MetaItem label="Owner" value={transfer.owner_email} />
            <MetaItem
              label="Shared to"
              value={transfer.recipient_email || <em style={{ color: 'var(--color-text-muted)' }}>Public link</em>}
            />
            <MetaItem
              label="Password"
              value={transfer.has_password
                ? <span className="badge badge-yellow">Protected</span>
                : <span className="badge badge-gray">None</span>}
            />
            <MetaItem
              label="Downloads"
              value={`${transfer.download_count}${transfer.max_downloads != null ? ` / ${transfer.max_downloads}` : ' (unlimited)'}`}
            />
            <MetaItem
              label="Expires"
              value={transfer.expires_at ? fmtDate(transfer.expires_at) : '—'}
            />
            <MetaItem label="Created" value={fmtDate(transfer.created_at)} />
            <MetaItem
              label="Share slug"
              value={<span className="mono" style={{ fontSize: 12 }}>{transfer.slug}</span>}
            />
            <MetaItem label="Files" value={`${transfer.file_count}`} />
            <MetaItem label="Total size" value={fmtBytes(transfer.total_size_bytes ?? 0)} />
          </div>

          {transfer.description && (
            <div>
              <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 6 }}>Description</div>
              <p style={{ fontSize: 13, color: 'var(--color-text-secondary)', lineHeight: 1.5 }}>{transfer.description}</p>
            </div>
          )}

          {/* Files section */}
          <div>
            <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 8 }}>
              Files
            </div>
            {loading ? (
              <p className="text-sm">Loading files…</p>
            ) : error ? (
              <div className="alert alert-error">{error}</div>
            ) : !detail?.files.length ? (
              <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>No files attached.</p>
            ) : (
              <div style={{ border: '1px solid var(--color-border)', borderRadius: 6, overflow: 'hidden' }}>
                {detail.files.map((f, i) => (
                  <FileRow key={f.file_id} file={f} last={i === detail.files.length - 1} />
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Close</button>
          {transfer.status === 'active' && (
            <button
              className="btn btn-danger"
              onClick={handleRevoke}
              disabled={revoking}
            >
              {revoking ? 'Revoking…' : 'Revoke Transfer'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function MetaItem({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 3 }}>{label}</div>
      <div style={{ fontSize: 13, color: 'var(--color-text-primary)' }}>{value}</div>
    </div>
  );
}

function FileRow({ file, last }: { file: TransferFile; last: boolean }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 10, padding: '9px 14px',
      borderBottom: last ? 'none' : '1px solid var(--color-border)',
      background: 'var(--color-surface)',
    }}>
      <FileIcon />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 13, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'var(--color-text-primary)' }}>
          {file.filename}
        </div>
        <div style={{ fontSize: 11, color: 'var(--color-text-muted)', marginTop: 1 }}>{file.content_type}</div>
      </div>
      <div style={{ fontSize: 12, color: 'var(--color-text-muted)', whiteSpace: 'nowrap' }}>
        {fmtBytes(file.size_bytes)}
      </div>
    </div>
  );
}

// ── Main Page ────────────────────────────────────────────────────────────────

const VALID_STATUSES = ['all', 'active', 'exhausted', 'expired', 'revoked'] as const;
type StatusFilter = typeof VALID_STATUSES[number];

function getInitialStatus(search: string): StatusFilter {
  const param = new URLSearchParams(search).get('status');
  return (VALID_STATUSES as readonly string[]).includes(param ?? '') ? (param as StatusFilter) : 'all';
}

export default function TransfersPage() {
  const location = useLocation();
  const [transfers, setTransfers] = useState<AdminTransfer[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [status, setStatus] = useState<StatusFilter>(() => getInitialStatus(location.search));
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<AdminTransfer | null>(null);
  const sort = useSortState<TransferSortKey>('created_at', 'desc', () => setPage(0));

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await listTransfers({
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
        status,
        sort_by: sort.sortKey,
        sort_dir: sort.sortDir,
      });
      setTransfers(res.transfers ?? []);
      setTotal(res.total ?? 0);
    } catch (err: unknown) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, [status, page, sort.sortKey, sort.sortDir]);

  useEffect(() => { load(); }, [load]);

  function handleRevoked(id: string) {
    setTransfers((prev) =>
      prev.map((t) => t.id === id ? { ...t, status: 'revoked', is_revoked: true } : t),
    );
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="page">
      {selected && (
        <DetailModal
          transfer={selected}
          onClose={() => setSelected(null)}
          onRevoked={handleRevoked}
        />
      )}

      <div className="page-header">
        <div>
          <h1 className="page-title">Transfers</h1>
          <p className="page-subtitle">All transfers across users — {total} total</p>
        </div>
      </div>

      <div className="card">
        {/* Status filter */}
        <div style={{ display: 'flex', gap: 6, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
          <span className="text-sm" style={{ marginRight: 4 }}>Status:</span>
          {VALID_STATUSES.map((s) => (
            <button
              key={s}
              className={`btn btn-sm ${status === s ? 'btn-primary' : 'btn-secondary'}`}
              onClick={() => { setStatus(s); setPage(0); }}
            >
              {s.charAt(0).toUpperCase() + s.slice(1)}
            </button>
          ))}
        </div>

        {error && <div className="alert alert-error">{error}</div>}

        {loading ? (
          <p className="text-sm" style={{ color: 'var(--color-text-muted)', padding: '20px 0' }}>Loading…</p>
        ) : transfers.length === 0 ? (
          <div className="empty-state">No transfers found.</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <SortHeader label="Owner" sortKey="owner_email" sort={sort} />
                  <SortHeader label="Transfer Name" sortKey="name" sort={sort} />
                  <th>Shared To</th>
                  <SortHeader label="Status" sortKey="status" sort={sort} />
                  <SortHeader label="Files" sortKey="file_count" sort={sort} />
                  <SortHeader label="Size" sortKey="total_size_bytes" sort={sort} />
                  <SortHeader label="Downloads" sortKey="download_count" sort={sort} />
                  <th>Password</th>
                  <SortHeader label="Expires" sortKey="expires_at" sort={sort} />
                  <SortHeader label="Created" sortKey="created_at" sort={sort} />
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {transfers.map((t) => (
                  <tr key={t.id} style={{ cursor: 'pointer' }} onClick={() => setSelected(t)}>
                    <td>
                      <span style={{ fontSize: 13 }}>{t.owner_email}</span>
                    </td>
                    <td>
                      <div style={{ fontWeight: 500, color: 'var(--color-text-primary)', fontSize: 13 }}>{t.name || '—'}</div>
                      {t.description && (
                        <div className="text-sm" style={{ marginTop: 2, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {t.description}
                        </div>
                      )}
                    </td>
                    <td>
                      {t.recipient_email
                        ? <span style={{ fontSize: 13 }}>{t.recipient_email}</span>
                        : <span className="badge badge-gray">Public link</span>}
                    </td>
                    <td><StatusBadge status={t.status} /></td>
                    <td>
                      <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 13 }}>
                        <FileIcon />
                        {t.file_count}
                      </span>
                    </td>
                    <td className="text-sm" style={{ whiteSpace: 'nowrap' }}>
                      {fmtBytes(t.total_size_bytes ?? 0)}
                    </td>
                    <td className="text-sm">
                      {t.download_count}
                      {t.max_downloads != null ? ` / ${t.max_downloads}` : ''}
                    </td>
                    <td>
                      {t.has_password
                        ? <span className="badge badge-yellow">Yes</span>
                        : <span style={{ color: 'var(--color-text-muted)', fontSize: 12 }}>—</span>}
                    </td>
                    <td className="text-sm">
                      {t.expires_at ? fmtDate(t.expires_at) : '—'}
                    </td>
                    <td className="text-sm">{fmtDate(t.created_at)}</td>
                    <td onClick={(e) => e.stopPropagation()}>
                      <div className="action-group">
                        <button
                          className="btn btn-sm btn-secondary"
                          onClick={() => setSelected(t)}
                        >
                          Details
                        </button>
                        {t.status === 'active' && (
                          <RevokeButton id={t.id} onRevoked={handleRevoked} />
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {totalPages > 1 && (
          <div className="row" style={{ marginTop: 16, gap: 8, alignItems: 'center' }}>
            <button
              className="btn btn-secondary btn-sm"
              disabled={page === 0}
              onClick={() => setPage((p) => p - 1)}
            >
              ← Previous
            </button>
            <span className="text-sm">
              Page {page + 1} of {totalPages}
            </span>
            <button
              className="btn btn-secondary btn-sm"
              disabled={page + 1 >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              Next →
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Inline Revoke Button ─────────────────────────────────────────────────────

function RevokeButton({ id, onRevoked }: { id: string; onRevoked: (id: string) => void }) {
  const [loading, setLoading] = useState(false);

  async function handleClick(e: React.MouseEvent) {
    e.stopPropagation();
    if (!confirm('Revoke this transfer? Recipients will immediately lose access.')) return;
    setLoading(true);
    try {
      await revokeTransfer(id);
      onRevoked(id);
    } finally {
      setLoading(false);
    }
  }

  return (
    <button className="btn btn-sm btn-danger" onClick={handleClick} disabled={loading}>
      {loading ? '…' : 'Revoke'}
    </button>
  );
}


