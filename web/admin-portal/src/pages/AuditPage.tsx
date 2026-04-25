import React, { useState, useEffect } from 'react';
import { listAuditEvents, type AuditEvent, type AuditFilters } from '../api/audit';

function fmt(date: string) {
  return new Date(date).toLocaleString();
}

const PAGE_SIZE = 50;

export default function AuditPage() {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [page, setPage] = useState(0);

  const [filterUserId, setFilterUserId] = useState('');
  const [filterSource, setFilterSource] = useState('');
  const [filterAction, setFilterAction] = useState('');
  const [filterStart, setFilterStart] = useState('');
  const [filterEnd, setFilterEnd] = useState('');

  async function load(offset = 0) {
    setLoading(true);
    setError('');
    const filters: AuditFilters = {
      limit: PAGE_SIZE,
      offset,
    };
    if (filterUserId.trim()) filters.user_id = filterUserId.trim();
    if (filterSource.trim()) filters.source = filterSource.trim();
    if (filterAction.trim()) filters.action = filterAction.trim();
    if (filterStart) filters.start = new Date(filterStart).toISOString();
    if (filterEnd) filters.end = new Date(filterEnd).toISOString();

    try {
      const res = await listAuditEvents(filters);
      setEvents(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load(page * PAGE_SIZE);
  }, [page]);

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    setPage(0);
    load(0);
  }

  function handleClear() {
    setFilterUserId('');
    setFilterSource('');
    setFilterAction('');
    setFilterStart('');
    setFilterEnd('');
    setPage(0);
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div className="page">
      <h1 className="page-title">Audit Logs</h1>

      <form onSubmit={handleSearch}>
        <div className="filter-bar mb-16">
          <div className="form-group">
            <label>User ID</label>
            <input
              type="text"
              value={filterUserId}
              onChange={(e) => setFilterUserId(e.target.value)}
              placeholder="UUID"
            />
          </div>
          <div className="form-group">
            <label>Source</label>
            <select value={filterSource} onChange={(e) => setFilterSource(e.target.value)}>
              <option value="">All</option>
              <option value="auth">auth</option>
              <option value="transfer">transfer</option>
              <option value="storage">storage</option>
              <option value="upload">upload</option>
              <option value="admin">admin</option>
            </select>
          </div>
          <div className="form-group">
            <label>Action</label>
            <input
              type="text"
              value={filterAction}
              onChange={(e) => setFilterAction(e.target.value)}
              placeholder="e.g. login"
            />
          </div>
          <div className="form-group">
            <label>Start</label>
            <input
              type="datetime-local"
              value={filterStart}
              onChange={(e) => setFilterStart(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label>End</label>
            <input
              type="datetime-local"
              value={filterEnd}
              onChange={(e) => setFilterEnd(e.target.value)}
            />
          </div>
          <div>
            <button className="btn btn-primary" type="submit" disabled={loading}>
              Search
            </button>
            <button
              className="btn btn-secondary"
              type="button"
              style={{ marginLeft: 6 }}
              onClick={handleClear}
            >
              Clear
            </button>
          </div>
        </div>
      </form>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="text-sm mb-16">
        {loading ? 'Loading…' : `${total} event(s) total`}
        {totalPages > 1 && ` — page ${page + 1} of ${totalPages}`}
      </div>

      {!loading && events.length === 0 ? (
        <div className="empty-state">No audit events found.</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Source</th>
                <th>Action</th>
                <th>User ID</th>
                <th>Resource</th>
                <th>IP</th>
                <th>Metadata</th>
              </tr>
            </thead>
            <tbody>
              {events.map((e) => (
                <tr key={e.id}>
                  <td className="text-sm mono">{fmt(e.created_at)}</td>
                  <td><span className="badge badge-blue">{e.source}</span></td>
                  <td><span className="badge badge-gray">{e.action}</span></td>
                  <td className="text-sm mono">
                    {e.user_id ? e.user_id.slice(0, 8) + '…' : '—'}
                  </td>
                  <td className="text-sm">
                    {e.resource_type && e.resource_id
                      ? `${e.resource_type}/${e.resource_id.slice(0, 8)}…`
                      : e.resource_type ?? '—'}
                  </td>
                  <td className="text-sm mono">{e.ip_address ?? '—'}</td>
                  <td className="text-sm">
                    {e.metadata && Object.keys(e.metadata).length > 0 ? (
                      <details>
                        <summary style={{ cursor: 'pointer' }}>view</summary>
                        <pre style={{ fontSize: 11, marginTop: 4, whiteSpace: 'pre-wrap' }}>
                          {JSON.stringify(e.metadata, null, 2)}
                        </pre>
                      </details>
                    ) : (
                      '—'
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {totalPages > 1 && (
        <div className="row mt-16" style={{ gap: 6 }}>
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => setPage((p) => p - 1)}
            disabled={page === 0 || loading}
          >
            ← Prev
          </button>
          <span className="text-sm" style={{ padding: '4px 8px' }}>
            {page + 1} / {totalPages}
          </span>
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => setPage((p) => p + 1)}
            disabled={page >= totalPages - 1 || loading}
          >
            Next →
          </button>
        </div>
      )}
    </div>
  );
}
