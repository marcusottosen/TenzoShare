import React, { useEffect, useState } from 'react';
import { Link } from 'react-router';
import { listTransfers, revokeTransfer, type Transfer } from '../api/transfers';

function statusBadge(t: Transfer) {
  if (t.is_revoked) return <span className="badge badge-red">Revoked</span>;
  if (t.expires_at && new Date(t.expires_at) < new Date())
    return <span className="badge badge-gray">Expired</span>;
  return <span className="badge badge-green">Active</span>;
}

function fmtDate(date: string) {
  return new Date(date).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

function fmtTime(date: string) {
  return new Date(date).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

// Mock activity feed (backend doesn't have this yet)
const MOCK_ACTIVITY = [
  { id: 1, type: 'download', text: 'someone@example.com downloaded "Q4 Report.pdf"', time: '2 minutes ago' },
  { id: 2, type: 'transfer', text: 'New transfer created — "Design Assets.zip"', time: '1 hour ago' },
  { id: 3, type: 'request', text: 'File request completed by client@acme.com', time: '3 hours ago' },
  { id: 4, type: 'download', text: 'partner@example.com downloaded "Contract.docx"', time: 'Yesterday' },
];

function IconUpload() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="16 16 12 12 8 16"/><line x1="12" y1="12" x2="12" y2="21"/>
      <path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/>
    </svg>
  );
}
function IconShare() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/>
      <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
    </svg>
  );
}
function IconInbox() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/>
      <path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>
    </svg>
  );
}
function IconDownload() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="8 17 12 21 16 17"/><line x1="12" y1="12" x2="12" y2="21"/>
      <path d="M20.88 18.09A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.09"/>
    </svg>
  );
}

export default function DashboardPage() {
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [revoking, setRevoking] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const res = await listTransfers();
      setTransfers(res.transfers ?? []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function handleRevoke(id: string) {
    if (!confirm('Revoke this transfer? Recipients will no longer be able to download.')) return;
    setRevoking(id);
    try {
      await revokeTransfer(id);
      await load();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setRevoking(null);
    }
  }

  const activeCount = transfers.filter((t) => !t.is_revoked && (!t.expires_at || new Date(t.expires_at) > new Date())).length;
  const totalDownloads = transfers.reduce((sum, t) => sum + (t.download_count ?? 0), 0);

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Dashboard</h1>
          <p className="page-subtitle">Overview of your transfers and activity</p>
        </div>
        <Link to="/transfers/new" className="btn btn-primary">
          <IconUpload /> New transfer
        </Link>
      </div>

      {/* Stat cards */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-card-icon teal"><IconShare /></div>
          <div className="stat-card-body">
            <div className="stat-card-value">{loading ? '—' : transfers.length}</div>
            <div className="stat-card-label">Total transfers</div>
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-icon green"><IconShare /></div>
          <div className="stat-card-body">
            <div className="stat-card-value">{loading ? '—' : activeCount}</div>
            <div className="stat-card-label">Active transfers</div>
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-icon blue"><IconDownload /></div>
          <div className="stat-card-body">
            <div className="stat-card-value">{loading ? '—' : totalDownloads}</div>
            <div className="stat-card-label">Total downloads</div>
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-icon orange"><IconInbox /></div>
          <div className="stat-card-body">
            <div className="stat-card-value">—</div>
            <div className="stat-card-label">Pending requests</div>
          </div>
        </div>
      </div>

      <div className="dashboard-grid">
        {/* Recent Transfers */}
        <div className="card" style={{ flex: '1 1 60%', minWidth: 0 }}>
          <div className="card-header">
            <h2 className="card-title">Recent transfers</h2>
            <Link to="/" className="text-link text-sm">View all</Link>
          </div>

          {error && <div className="alert alert-error">{error}</div>}

          {loading ? (
            <div className="empty-state">Loading…</div>
          ) : transfers.length === 0 ? (
            <div className="empty-state">No transfers yet. <Link to="/transfers/new" className="text-link">Create one</Link>.</div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Status</th>
                    <th>Name</th>
                    <th>Recipient</th>
                    <th>Downloads</th>
                    <th>Created</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {transfers.slice(0, 8).map((t) => (
                    <tr key={t.id}>
                      <td>{statusBadge(t)}</td>
                      <td>
                        <Link to={`/transfers/${t.id}`} className="text-link">
                          {t.name || <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>{t.slug}</span>}
                        </Link>
                      </td>
                      <td className="text-sm">{t.recipient_email ?? <span style={{ color: 'var(--color-text-muted)' }}>—</span>}</td>
                      <td className="text-sm">
                        {t.download_count}
                        {t.max_downloads > 0 && ` / ${t.max_downloads}`}
                      </td>
                      <td className="text-sm">{fmtDate(t.created_at)}</td>
                      <td>
                        {!t.is_revoked && (
                          <button
                            className="btn btn-danger btn-sm"
                            onClick={() => handleRevoke(t.id)}
                            disabled={revoking === t.id}
                          >
                            Revoke
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Activity Feed (mock) */}
        <div className="card" style={{ flex: '1 1 36%', minWidth: 240 }}>
          <div className="card-header">
            <h2 className="card-title">Recent activity</h2>
          </div>
          <ul className="activity-list">
            {MOCK_ACTIVITY.map((a) => (
              <li key={a.id} className="activity-item">
                <div className="activity-dot" />
                <div className="activity-body">
                  <div className="activity-text">{a.text}</div>
                  <div className="activity-time">{a.time}</div>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
