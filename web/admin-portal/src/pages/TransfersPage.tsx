import React, { useEffect, useState, useCallback } from 'react';
import { listTransfers, type AdminTransfer } from '../api/admin';

const PAGE_SIZE = 50;

function fmt(date: string) {
  return new Date(date).toLocaleString();
}

function StatusBadge({ status }: { status: AdminTransfer['status'] }) {
  const cls =
    status === 'active' ? 'badge badge-green' :
    status === 'expired' ? 'badge badge-gray' :
    'badge badge-red';
  return <span className={cls}>{status}</span>;
}

export default function TransfersPage() {
  const [transfers, setTransfers] = useState<AdminTransfer[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [status, setStatus] = useState<'all' | 'active' | 'expired' | 'revoked'>('all');
  const [page, setPage] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await listTransfers({ limit: PAGE_SIZE, offset: page * PAGE_SIZE, status });
      setTransfers(res.transfers ?? []);
      setTotal(res.total ?? 0);
    } catch (err: unknown) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, [status, page]);

  useEffect(() => { load(); }, [load]);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Transfers</h1>
        <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>{total} total</span>
      </div>

      <div className="card">
        <div className="row" style={{ gap: 8, marginBottom: 16, alignItems: 'center' }}>
          <label className="text-sm font-medium" style={{ marginRight: 4 }}>Status:</label>
          {(['all', 'active', 'expired', 'revoked'] as const).map((s) => (
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
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>
        ) : transfers.length === 0 ? (
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>No transfers found.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Owner</th>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Downloads</th>
                  <th>Expires</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {transfers.map((t) => (
                  <tr key={t.id}>
                    <td className="text-sm">{t.owner_email}</td>
                    <td>
                      <span title={t.id} style={{ cursor: 'default' }}>{t.name}</span>
                    </td>
                    <td><StatusBadge status={t.status} /></td>
                    <td className="text-sm">
                      {t.download_count}{t.max_downloads != null ? ` / ${t.max_downloads}` : ''}
                    </td>
                    <td className="text-sm">{t.expires_at ? fmt(t.expires_at) : '—'}</td>
                    <td className="text-sm">{fmt(t.created_at)}</td>
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
              Previous
            </button>
            <span className="text-sm">
              Page {page + 1} of {totalPages}
            </span>
            <button
              className="btn btn-secondary btn-sm"
              disabled={page + 1 >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
