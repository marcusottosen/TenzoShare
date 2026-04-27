import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';
import { listTransfers, revokeTransfer, type Transfer } from '../api/transfers';
import {
  listFileRequests,
  deactivateFileRequest,
  getFileRequest,
  type FileRequest,
  type Submission,
} from '../api/requests';

// ── Clipboard ─────────────────────────────────────────────────────────────────
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
  ta.focus(); ta.select();
  try { if (document.execCommand('copy')) onSuccess(); } finally { document.body.removeChild(ta); }
}

function buildRequestUrl(slug: string) {
  const base = (import.meta.env.VITE_REQUEST_UI_URL as string | undefined)?.replace(/\/$/, '')
    ?? `${window.location.protocol}//${window.location.hostname}:3002`;
  return `${base}/r/${slug}`;
}
function buildTransferUrl(slug: string) {
  const base = (import.meta.env.VITE_DOWNLOAD_UI_URL as string | undefined)?.replace(/\/$/, '')
    ?? `${window.location.protocol}//${window.location.hostname}:3003`;
  return `${base}/t/${slug}`;
}
function fmt(d: string) { return new Date(d).toLocaleString(); }
function fmtDate(d: string) { return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }); }
function fmtBytes(b: number) {
  if (b === 0) return '0 B';
  const k = 1024; const u = ['B','KB','MB','GB']; const i = Math.floor(Math.log(b)/Math.log(k));
  return `${(b / Math.pow(k,i)).toFixed(1)} ${u[i]}`;
}

// ── Icons ─────────────────────────────────────────────────────────────────────
function IconLink() {
  return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>;
}
function IconBan() {
  return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>;
}
function IconChevron({ open }: { open: boolean }) {
  return <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ transform: open ? 'rotate(90deg)' : 'none', transition: 'transform 0.15s' }}><polyline points="9 18 15 12 9 6"/></svg>;
}
function IconShare() {
  return <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/></svg>;
}
function IconInbox() {
  return <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/></svg>;
}

// ── Badges ────────────────────────────────────────────────────────────────────
function TransferBadge({ t }: { t: Transfer }) {
  if (t.is_revoked || t.status === 'revoked') return <span className="badge badge-red">Revoked</span>;
  if (t.status === 'exhausted' || (t.max_downloads > 0 && t.download_count >= t.max_downloads)) return <span className="badge badge-yellow">Exhausted</span>;
  if (t.expires_at && new Date(t.expires_at) < new Date()) return <span className="badge badge-gray">Expired</span>;
  return <span className="badge badge-green">Active</span>;
}
function RequestBadge({ r }: { r: FileRequest }) {
  if (!r.is_active) return <span className="badge badge-red">Closed</span>;
  if (r.is_expired) return <span className="badge badge-gray">Expired</span>;
  return <span className="badge badge-green">Active</span>;
}

// ── SubmissionList ────────────────────────────────────────────────────────────
function SubmissionList({ subs }: { subs: Submission[] }) {
  if (subs.length === 0) return <p style={{ color: 'var(--color-text-muted)', fontSize: 13 }}>No files submitted yet.</p>;
  return (
    <div className="table-wrap" style={{ marginTop: 10 }}>
      <table>
        <thead><tr><th>Filename</th><th>Size</th><th>From</th><th>Submitted</th></tr></thead>
        <tbody>
          {subs.map((s) => (
            <tr key={s.id}>
              <td>{s.filename}</td>
              <td>{fmtBytes(s.size_bytes)}</td>
              <td>{s.submitter_name || <span style={{ color: 'var(--color-text-muted)' }}>—</span>}</td>
              <td>{fmt(s.submitted_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ── My Shares tab ─────────────────────────────────────────────────────────────
type ShareSortKey = 'name' | 'recipient' | 'downloads' | 'size' | 'expires';

function SortArrow({ col, sortKey, sortDir }: { col: ShareSortKey; sortKey: ShareSortKey; sortDir: 'asc'|'desc' }) {
  if (col !== sortKey) return <span className="sort-arrow" style={{ opacity: 0.3 }}>{'\u2195'}</span>;
  return <span className="sort-arrow">{sortDir === 'asc' ? '\u2191' : '\u2193'}</span>;
}

function MySharesTab() {
  const navigate = useNavigate();
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [revoking, setRevoking] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<ShareSortKey>('expires');
  const [sortDir, setSortDir] = useState<'asc'|'desc'>('desc');

  function toggleSort(key: ShareSortKey) {
    if (sortKey === key) setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    else { setSortKey(key); setSortDir('asc'); }
  }

  useEffect(() => {
    setLoading(true);
    listTransfers()
      .then((res) => setTransfers(res.transfers ?? []))
      .catch((err: unknown) => setError((err as Error).message))
      .finally(() => setLoading(false));
  }, []);

  async function handleRevoke(id: string) {
    if (!confirm('Revoke this share? Recipients will no longer be able to download.')) return;
    setRevoking(id);
    try {
      await revokeTransfer(id);
      setTransfers((prev) => prev.map((t) => (t.id === id ? { ...t, is_revoked: true } : t)));
    } catch (err: unknown) { alert((err as Error).message); }
    finally { setRevoking(null); }
  }

  function handleCopy(slug: string) {
    copyToClipboard(buildTransferUrl(slug), () => { setCopied(slug); setTimeout(() => setCopied(null), 2000); });
  }

  if (loading) return <div className="empty-state">Loading…</div>;
  if (error) return <div className="alert alert-error">{error}</div>;
  if (transfers.length === 0) {
    return (
      <div className="empty-state" style={{ padding: '48px 0' }}>
        <div style={{ fontSize: 36, marginBottom: 12 }}>📤</div>
        <strong style={{ fontSize: 15 }}>No shares yet</strong>
        <p style={{ color: 'var(--color-text-muted)', marginTop: 6, fontSize: 13 }}>
          Go to <a href="/transfers/new" className="text-link">New Transfer</a> to share files with anyone.
        </p>
      </div>
    );
  }

  const sorted = [...transfers].sort((a, b) => {
    let cmp = 0;
    if (sortKey === 'name') cmp = (a.name || '').localeCompare(b.name || '');
    else if (sortKey === 'recipient') cmp = (a.recipient_email || '').localeCompare(b.recipient_email || '');
    else if (sortKey === 'downloads') cmp = (a.download_count ?? 0) - (b.download_count ?? 0);
    else if (sortKey === 'size') cmp = (a.total_size_bytes ?? 0) - (b.total_size_bytes ?? 0);
    else cmp = new Date(a.expires_at ?? 0).getTime() - new Date(b.expires_at ?? 0).getTime();
    return sortDir === 'asc' ? cmp : -cmp;
  });

  return (
    <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
      <div className="table-wrap" style={{ border: 'none', borderRadius: 0 }}>
        <table>
          <thead>
            <tr>
              <th className="sort-th" onClick={() => toggleSort('name')}>
                Name <SortArrow col="name" sortKey={sortKey} sortDir={sortDir} />
              </th>
              <th>Status</th>
              <th className="sort-th" onClick={() => toggleSort('recipient')}>
                Recipient <SortArrow col="recipient" sortKey={sortKey} sortDir={sortDir} />
              </th>
              <th className="sort-th" onClick={() => toggleSort('downloads')}>
                Downloads <SortArrow col="downloads" sortKey={sortKey} sortDir={sortDir} />
              </th>
              <th className="sort-th" onClick={() => toggleSort('size')}>
                Size <SortArrow col="size" sortKey={sortKey} sortDir={sortDir} />
              </th>
              <th className="sort-th" onClick={() => toggleSort('expires')}>
                Expires <SortArrow col="expires" sortKey={sortKey} sortDir={sortDir} />
              </th>
              <th style={{ textAlign: 'right' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((t) => (
              <tr
                key={t.id}
                onDoubleClick={() => navigate(`/transfers/${t.id}`)}
                style={{ cursor: 'pointer' }}
              >
                <td style={{ maxWidth: 260 }}>
                  <div style={{ fontWeight: 500, fontSize: 13, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.name}</div>
                  {t.description && <div style={{ fontSize: 11, color: 'var(--color-text-muted)', marginTop: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.description}</div>}
                </td>
                <td><TransferBadge t={t} /></td>
                <td style={{ fontSize: 13, color: 'var(--color-text-muted)', maxWidth: 160, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {t.recipient_email ?? <em>anyone</em>}
                </td>
                <td style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>
                  {t.download_count ?? 0}{t.max_downloads ? ` / ${t.max_downloads}` : ''}
                </td>
                <td style={{ fontSize: 13, color: 'var(--color-text-muted)', whiteSpace: 'nowrap' }}>
                  {t.total_size_bytes != null ? fmtBytes(t.total_size_bytes) : '—'}
                </td>
                <td style={{ fontSize: 12, color: 'var(--color-text-muted)', whiteSpace: 'nowrap' }}>
                  {t.expires_at ? fmtDate(t.expires_at) : '—'}
                </td>
                <td>
                  <div style={{ display: 'flex', gap: 5, justifyContent: 'flex-end' }}>
                    <button className="btn btn-secondary btn-sm" onClick={() => navigate(`/transfers/${t.id}`)}>View</button>
                    <button className="btn btn-secondary btn-sm" onClick={() => handleCopy(t.slug)}>
                      <IconLink /> {copied === t.slug ? 'Copied!' : 'Copy link'}
                    </button>
                    {!t.is_revoked && (
                      <button className="btn btn-danger btn-sm" onClick={() => handleRevoke(t.id)} disabled={revoking === t.id}>
                        <IconBan /> {revoking === t.id ? '…' : 'Revoke'}
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div style={{ padding: '10px 20px', fontSize: 12, color: 'var(--color-text-muted)', borderTop: '1px solid var(--color-border)' }}>
        {sorted.length} transfer{sorted.length !== 1 ? 's' : ''} total
      </div>
    </div>
  );
}

// ── File Requests tab ─────────────────────────────────────────────────────────
function FileRequestsTab() {
  const navigate = useNavigate();
  const [requests, setRequests] = useState<FileRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [expandedSubs, setExpandedSubs] = useState<Submission[] | null>(null);
  const [loadingSubs, setLoadingSubs] = useState(false);
  const [deactivating, setDeactivating] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    listFileRequests()
      .then((res) => setRequests(res.requests ?? []))
      .catch((err: unknown) => setError((err as Error).message))
      .finally(() => setLoading(false));
  }, []);

  async function handleDeactivate(id: string) {
    if (!confirm('Close this request? Guests will no longer be able to upload.')) return;
    setDeactivating(id);
    try {
      await deactivateFileRequest(id);
      setRequests((prev) => prev.map((r) => (r.id === id ? { ...r, is_active: false } : r)));
    } catch (err: unknown) { alert((err as Error).message); }
    finally { setDeactivating(null); }
  }

  async function handleExpand(id: string) {
    if (expandedId === id) { setExpandedId(null); setExpandedSubs(null); return; }
    setExpandedId(id); setExpandedSubs(null); setLoadingSubs(true);
    try {
      const req = await getFileRequest(id);
      setExpandedSubs(req.submissions ?? []);
    } catch { setExpandedSubs([]); } finally { setLoadingSubs(false); }
  }

  function handleCopy(slug: string) {
    copyToClipboard(buildRequestUrl(slug), () => { setCopied(slug); setTimeout(() => setCopied(null), 2000); });
  }

  return (
    <>
      {error && <div className="alert alert-error">{error}</div>}
      {loading ? (
        <div className="empty-state">Loading…</div>
      ) : requests.length === 0 ? (
        <div className="empty-state" style={{ padding: '48px 0' }}>
          <div style={{ fontSize: 36, marginBottom: 12 }}>📬</div>
          <strong style={{ fontSize: 15 }}>No file requests yet</strong>
          <p style={{ color: 'var(--color-text-muted)', marginTop: 6, fontSize: 13 }}>
            Create a request to let anyone upload files directly to you.
          </p>
          <button className="btn btn-primary" style={{ marginTop: 14 }} onClick={() => navigate('/requests')}>
            + New request
          </button>
        </div>
      ) : (
        requests.map((r) => {
          const url = buildRequestUrl(r.slug);
          const isExpanded = expandedId === r.id;
          return (
            <div key={r.id} className="request-card">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12, flexWrap: 'wrap' }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                    <RequestBadge r={r} />
                    <span style={{ fontWeight: 600, fontSize: 14, color: 'var(--color-text-primary)' }}>{r.name}</span>
                    {(r.submission_count ?? 0) > 0 ? (
                      <span className="badge badge-blue">📥 {r.submission_count} file{r.submission_count !== 1 ? 's' : ''} received</span>
                    ) : (
                      <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>No submissions</span>
                    )}
                  </div>
                  {r.description && (
                    <p style={{ margin: '0 0 6px', fontSize: 13, color: 'var(--color-text-muted)' }}>{r.description}</p>
                  )}
                  <p style={{ margin: 0, fontSize: 12, color: 'var(--color-text-muted)' }}>
                    Expires {fmt(r.expires_at)}
                    {r.max_size_mb > 0 && ` · Max ${r.max_size_mb} MB`}
                    {r.max_files > 0 && ` · Max ${r.max_files} files`}
                  </p>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 10, flexWrap: 'wrap' }}>
                    <a href={url} target="_blank" rel="noreferrer" className="text-link"
                      style={{ fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 300 }}>
                      {url}
                    </a>
                    <button className="btn btn-secondary btn-sm" onClick={() => handleCopy(r.slug)}>
                      {copied === r.slug ? 'Copied!' : 'Copy link'}
                    </button>
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 6, flexShrink: 0 }}>
                  <button className="btn btn-secondary btn-sm" onClick={() => handleExpand(r.id)}>
                    <IconChevron open={isExpanded} /> {isExpanded ? 'Hide' : 'Submissions'}
                  </button>
                  {r.is_active && (
                    <button className="btn btn-danger btn-sm" onClick={() => handleDeactivate(r.id)} disabled={deactivating === r.id}>
                      {deactivating === r.id ? 'Closing…' : 'Close'}
                    </button>
                  )}
                </div>
              </div>
              {isExpanded && (
                <div style={{ marginTop: 14, paddingTop: 14, borderTop: '1px solid var(--color-border)' }}>
                  {loadingSubs ? <p style={{ fontSize: 13, color: 'var(--color-text-muted)' }}>Loading…</p> : <SubmissionList subs={expandedSubs ?? []} />}
                </div>
              )}
            </div>
          );
        })
      )}
    </>
  );
}

// ── Main ──────────────────────────────────────────────────────────────────────
type Tab = 'shares' | 'requests';

export default function SharesPage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>('shares');

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Shares &amp; Requests</h1>
          <p className="page-subtitle">Manage outgoing transfers and incoming file requests</p>
        </div>
        {tab === 'shares'
          ? <button className="btn btn-primary" onClick={() => navigate('/transfers/new')}>+ New Transfer</button>
          : <button className="btn btn-primary" onClick={() => navigate('/requests')}>+ New Request</button>
        }
      </div>

      <div className="tab-bar">
        <button className={`tab-btn${tab === 'shares' ? ' active' : ''}`} onClick={() => setTab('shares')}>
          <IconShare /> My Shares
        </button>
        <button className={`tab-btn${tab === 'requests' ? ' active' : ''}`} onClick={() => setTab('requests')}>
          <IconInbox /> File Requests
        </button>
      </div>

      {tab === 'shares' ? <MySharesTab /> : <FileRequestsTab />}
    </div>
  );
}
