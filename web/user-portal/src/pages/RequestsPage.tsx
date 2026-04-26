import React, { useEffect, useState } from 'react';
import {
  listFileRequests,
  createFileRequest,
  deactivateFileRequest,
  getFileRequest,
  type FileRequest,
  type Submission,
} from '../api/requests';

// clipboard.writeText requires HTTPS or localhost. Use execCommand as fallback
// for plain-HTTP LAN access.
function copyToClipboard(text: string, onSuccess: () => void) {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(onSuccess).catch(() => execCopy(text, onSuccess));
  } else {
    execCopy(text, onSuccess);
  }
}

function execCopy(text: string, onSuccess: () => void) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.cssText = 'position:fixed;opacity:0;top:0;left:0;width:1px;height:1px';
  document.body.appendChild(ta);
  ta.focus();
  ta.select();
  try {
    if (document.execCommand('copy')) onSuccess();
  } finally {
    document.body.removeChild(ta);
  }
}

function buildRequestUrl(slug: string): string {
  const base =
    (import.meta.env.VITE_REQUEST_UI_URL as string | undefined)?.replace(/\/$/, '') ??
    `${window.location.protocol}//${window.location.hostname}:3002`;
  return `${base}/r/${slug}`;
}

function fmt(date: string) {
  return new Date(date).toLocaleString();
}

function fmtBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${units[i]}`;
}

function statusBadge(r: FileRequest) {
  if (!r.is_active) return <span className="badge badge-red">Closed</span>;
  if (r.is_expired) return <span className="badge badge-gray">Expired</span>;
  return <span className="badge badge-green">Active</span>;
}

// ---------------------------------------------------------------------------
// Create Request form (modal-style inline card)
// ---------------------------------------------------------------------------
interface CreateFormProps {
  onCreated: (r: FileRequest) => void;
  onCancel: () => void;
}

function CreateForm({ onCreated, onCancel }: CreateFormProps) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [allowedTypes, setAllowedTypes] = useState('');
  const [maxSizeMB, setMaxSizeMB] = useState('');
  const [expiresInHrs, setExpiresInHrs] = useState('72');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) { setError('Name is required.'); return; }
    const hrs = parseInt(expiresInHrs, 10);
    if (!hrs || hrs < 1) { setError('Expiry must be at least 1 hour.'); return; }

    setError('');
    setLoading(true);
    try {
      const req = await createFileRequest({
        name: name.trim(),
        description: description.trim() || undefined,
        allowed_types: allowedTypes.trim() || undefined,
        max_size_mb: maxSizeMB ? parseInt(maxSizeMB, 10) : undefined,
        expires_in_hours: hrs,
      });
      onCreated(req);
    } catch (err: unknown) {
      setError((err as Error).message ?? 'Failed to create request.');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="card" style={{ marginBottom: 24 }}>
      <div className="card-title">New file request</div>
      {error && <div className="alert alert-error">{error}</div>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Name *</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Project deliverables"
            maxLength={200}
            required
            autoFocus
          />
        </div>
        <div className="form-group">
          <label>Description</label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Instructions for the submitter (optional)"
            maxLength={1000}
            rows={2}
            style={{ resize: 'vertical' }}
          />
        </div>
        <div className="row" style={{ gap: 12 }}>
          <div className="form-group" style={{ flex: 1 }}>
            <label>Allowed types</label>
            <input
              type="text"
              value={allowedTypes}
              onChange={(e) => setAllowedTypes(e.target.value)}
              placeholder="e.g. image/,application/pdf"
            />
            <small className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
              Comma-separated MIME prefixes; leave blank for all types.
            </small>
          </div>
          <div className="form-group" style={{ width: 120 }}>
            <label>Max size (MB)</label>
            <input
              type="number"
              value={maxSizeMB}
              onChange={(e) => setMaxSizeMB(e.target.value)}
              placeholder="∞"
              min={1}
            />
          </div>
        </div>
        <div className="form-group">
          <label>Expires in (hours) *</label>
          <input
            type="number"
            value={expiresInHrs}
            onChange={(e) => setExpiresInHrs(e.target.value)}
            min={1}
            max={8760}
            required
          />
          <small className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
            1 day = 24 hrs · 1 week = 168 hrs · max 365 days = 8760 hrs
          </small>
        </div>
        <div className="row" style={{ gap: 8, justifyContent: 'flex-end' }}>
          <button type="button" className="btn btn-secondary" onClick={onCancel} disabled={loading}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={loading}>
            {loading ? 'Creating…' : 'Create request'}
          </button>
        </div>
      </form>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Submission detail panel
// ---------------------------------------------------------------------------
function SubmissionList({ subs }: { subs: Submission[] }) {
  if (subs.length === 0) {
    return <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>No files submitted yet.</p>;
  }
  return (
    <div className="table-wrap" style={{ marginTop: 8 }}>
      <table>
        <thead>
          <tr>
            <th>Filename</th>
            <th>Size</th>
            <th>From</th>
            <th>Submitted</th>
          </tr>
        </thead>
        <tbody>
          {subs.map((s) => (
            <tr key={s.id}>
              <td className="text-sm">{s.filename}</td>
              <td className="text-sm">{fmtBytes(s.size_bytes)}</td>
              <td className="text-sm">{s.submitter_name || <span style={{ color: 'var(--color-text-muted)' }}>—</span>}</td>
              <td className="text-sm">{fmt(s.submitted_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------
export default function RequestsPage() {
  const [requests, setRequests] = useState<FileRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [expandedSubs, setExpandedSubs] = useState<Submission[] | null>(null);
  const [loadingSubs, setLoadingSubs] = useState(false);
  const [deactivating, setDeactivating] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const res = await listFileRequests();
      setRequests(res.requests ?? []);
    } catch (err: unknown) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function handleDeactivate(id: string) {
    if (!confirm('Close this file request? Guests will no longer be able to upload to it.')) return;
    setDeactivating(id);
    try {
      await deactivateFileRequest(id);
      setRequests((prev) =>
        prev.map((r) => (r.id === id ? { ...r, is_active: false } : r)),
      );
    } catch (err: unknown) {
      alert((err as Error).message);
    } finally {
      setDeactivating(null);
    }
  }

  async function handleExpand(id: string) {
    if (expandedId === id) {
      setExpandedId(null);
      setExpandedSubs(null);
      return;
    }
    setExpandedId(id);
    setExpandedSubs(null);
    setLoadingSubs(true);
    try {
      const req = await getFileRequest(id);
      setExpandedSubs(req.submissions ?? []);
    } catch {
      setExpandedSubs([]);
    } finally {
      setLoadingSubs(false);
    }
  }

  function handleCopy(slug: string) {
    const url = buildRequestUrl(slug);
    copyToClipboard(url, () => {
      setCopied(slug);
      setTimeout(() => setCopied(null), 2000);
    });
  }

  function handleCreated(r: FileRequest) {
    setRequests((prev) => [r, ...prev]);
    setShowCreate(false);
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">File Requests</h1>
          <p className="page-subtitle">Request files from anyone — no account needed</p>
        </div>
        {!showCreate && (
          <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
            + New request
          </button>
        )}
      </div>

      {showCreate && (
        <CreateForm onCreated={handleCreated} onCancel={() => setShowCreate(false)} />
      )}

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <div className="empty-state">Loading…</div>
      ) : requests.length === 0 ? (
        <div className="empty-state">
          No file requests yet.{' '}
          <button
            className="text-link"
            style={{ background: 'none', border: 'none', padding: 0, cursor: 'pointer' }}
            onClick={() => setShowCreate(true)}
          >
            Create one
          </button>{' '}
          to let guests upload files to you.
        </div>
      ) : (
        <div>
          {requests.map((r) => {
            const url = buildRequestUrl(r.slug);
            const isExpanded = expandedId === r.id;
            return (
              <div key={r.id} className="card" style={{ marginBottom: 12 }}>
                <div className="row" style={{ justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 8 }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div className="row" style={{ alignItems: 'center', gap: 8, marginBottom: 4 }}>
                      {statusBadge(r)}
                      <span style={{ fontWeight: 600 }}>{r.name}</span>
                    </div>
                    {r.description && (
                      <p className="text-sm" style={{ margin: '0 0 4px', color: 'var(--color-text-muted)' }}>
                        {r.description}
                      </p>
                    )}
                    <p className="text-sm" style={{ margin: 0, color: 'var(--color-text-muted)' }}>
                      Expires {fmt(r.expires_at)}
                      {r.max_size_mb > 0 && ` · Max ${r.max_size_mb} MB`}
                      {r.max_files > 0 && ` · Max ${r.max_files} files`}
                    </p>
                    <div className="row" style={{ alignItems: 'center', gap: 6, marginTop: 8, flexWrap: 'wrap' }}>
                      <a
                        href={url}
                        target="_blank"
                        rel="noreferrer"
                        className="text-link text-sm"
                        style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 280 }}
                      >
                        {url}
                      </a>
                      <button
                        className="btn btn-secondary btn-sm"
                        onClick={() => handleCopy(r.slug)}
                      >
                        {copied === r.slug ? 'Copied!' : 'Copy link'}
                      </button>
                    </div>
                  </div>
                  <div className="row" style={{ gap: 6, flexShrink: 0 }}>
                    <button
                      className="btn btn-secondary btn-sm"
                      onClick={() => handleExpand(r.id)}
                    >
                      {isExpanded ? 'Hide submissions' : 'View submissions'}
                    </button>
                    {r.is_active && (
                      <button
                        className="btn btn-danger btn-sm"
                        onClick={() => handleDeactivate(r.id)}
                        disabled={deactivating === r.id}
                      >
                        {deactivating === r.id ? 'Closing…' : 'Close'}
                      </button>
                    )}
                  </div>
                </div>

                {isExpanded && (
                  <div style={{ marginTop: 16, borderTop: '1px solid var(--color-border)', paddingTop: 16 }}>
                    {loadingSubs ? (
                      <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading submissions…</p>
                    ) : (
                      <SubmissionList subs={expandedSubs ?? []} />
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
