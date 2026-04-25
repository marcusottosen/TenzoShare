import React, { useEffect, useState } from 'react';
import { Link } from 'react-router';
import { listTransfers, revokeTransfer, type Transfer } from '../api/transfers';

function statusBadge(t: Transfer) {
  if (t.is_revoked) return <span className="badge badge-red">Revoked</span>;
  if (t.expires_at && new Date(t.expires_at) < new Date())
    return <span className="badge badge-gray">Expired</span>;
  return <span className="badge badge-green">Active</span>;
}

function fmt(date: string) {
  return new Date(date).toLocaleString();
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

  return (
    <div className="page">
      <div className="row mb-16" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <h1 className="page-title" style={{ margin: 0 }}>Transfers</h1>
        <Link to="/transfers/new" className="btn btn-primary">+ New transfer</Link>
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
                <th>Expires</th>
                <th>Created</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {transfers.map((t) => (
                <tr key={t.id}>
                  <td>{statusBadge(t)}</td>
                  <td>
                    <Link to={`/transfers/${t.id}`} className="text-link">
                      {t.name || <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>{t.slug}</span>}
                    </Link>
                  </td>
                  <td>{t.recipient_email ?? <span className="text-sm">—</span>}</td>
                  <td className="text-sm">
                    {t.download_count}
                    {t.max_downloads > 0 && ` / ${t.max_downloads}`}
                  </td>
                  <td className="text-sm">{t.expires_at ? fmt(t.expires_at) : '—'}</td>
                  <td className="text-sm">{fmt(t.created_at)}</td>
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
  );
}
