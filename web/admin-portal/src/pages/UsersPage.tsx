import React, { useState, useEffect, useCallback } from 'react';
import { listUsers, updateUser, type AdminUser } from '../api/admin';

const PAGE_SIZE = 50;

function fmt(date: string) {
  return new Date(date).toLocaleDateString();
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '—';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

export default function UsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState('');
  const [roleFilter, setRoleFilter] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async (offset = 0) => {
    setLoading(true);
    setError('');
    try {
      const res = await listUsers({
        limit: PAGE_SIZE,
        offset,
        search: search.trim() || undefined,
        role: roleFilter || undefined,
      });
      setUsers(res.users ?? []);
      setTotal(res.total ?? 0);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [search, roleFilter]);

  useEffect(() => {
    load(page * PAGE_SIZE);
  }, [page, load]);

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    setPage(0);
    load(0);
  }

  async function handleRoleChange(user: AdminUser, newRole: string) {
    setSaving(true);
    try {
      await updateUser(user.id, { role: newRole });
      setUsers((prev) => prev.map((u) => u.id === user.id ? { ...u, role: newRole } : u));
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
      setEditing(null);
    }
  }

  async function handleToggleActive(user: AdminUser) {
    setSaving(true);
    try {
      await updateUser(user.id, { is_active: !user.is_active });
      setUsers((prev) => prev.map((u) => u.id === user.id ? { ...u, is_active: !u.is_active } : u));
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div className="page">
      <h1 className="page-title">Users</h1>

      <form onSubmit={handleSearch}>
        <div className="filter-bar mb-16">
          <div className="form-group">
            <label>Search</label>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Email address"
            />
          </div>
          <div className="form-group">
            <label>Role</label>
            <select value={roleFilter} onChange={(e) => setRoleFilter(e.target.value)}>
              <option value="">All</option>
              <option value="admin">admin</option>
              <option value="user">user</option>
            </select>
          </div>
          <div style={{ paddingTop: 18 }}>
            <button className="btn btn-primary" type="submit" disabled={loading}>
              Search
            </button>
          </div>
        </div>
      </form>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="text-sm mb-16">
        {loading ? 'Loading…' : `${total} user(s) total`}
        {totalPages > 1 && ` — page ${page + 1} of ${totalPages}`}
      </div>

      {!loading && users.length === 0 ? (
        <div className="empty-state">No users found.</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Email</th>
                <th>Role</th>
                <th>Status</th>
                <th>Verified</th>
                <th>Failed Logins</th>
                <th>Joined</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id}>
                  <td>
                    <div style={{ fontWeight: 500 }}>{u.email}</div>
                    <div className="text-sm mono">{u.id.slice(0, 8)}…</div>
                  </td>
                  <td>
                    {editing === u.id ? (
                      <select
                        defaultValue={u.role}
                        disabled={saving}
                        onChange={(e) => handleRoleChange(u, e.target.value)}
                        onBlur={() => setEditing(null)}
                        autoFocus
                        style={{ width: 90 }}
                      >
                        <option value="user">user</option>
                        <option value="admin">admin</option>
                      </select>
                    ) : (
                      <span
                        className={`badge ${u.role === 'admin' ? 'badge-orange' : 'badge-gray'}`}
                        style={{ cursor: 'pointer' }}
                        title="Click to edit role"
                        onClick={() => setEditing(u.id)}
                      >
                        {u.role}
                      </span>
                    )}
                  </td>
                  <td>
                    <span className={`badge ${u.is_active ? 'badge-green' : 'badge-red'}`}>
                      {u.is_active ? 'Active' : 'Disabled'}
                    </span>
                  </td>
                  <td>
                    <span className={`badge ${u.email_verified ? 'badge-green' : 'badge-gray'}`}>
                      {u.email_verified ? 'Yes' : 'No'}
                    </span>
                  </td>
                  <td className="text-sm">
                    {u.failed_login_attempts > 0 ? (
                      <span className="badge badge-orange">{u.failed_login_attempts}</span>
                    ) : '0'}
                    {u.locked_until && new Date(u.locked_until) > new Date() && (
                      <span className="badge badge-red" style={{ marginLeft: 4 }}>Locked</span>
                    )}
                  </td>
                  <td className="text-sm">{fmt(u.created_at)}</td>
                  <td>
                    <button
                      className={`btn btn-sm ${u.is_active ? 'btn-secondary' : 'btn-primary'}`}
                      disabled={saving}
                      onClick={() => handleToggleActive(u)}
                    >
                      {u.is_active ? 'Disable' : 'Enable'}
                    </button>
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
