import React, { useState, useEffect, useCallback } from 'react';
import {
  listUsers, createUser, updateUser, deleteUser, unlockUser, verifyUserEmail,
  listStorageUsage,
  type AdminUser, type StorageUserUsage,
} from '../api/admin';
import { useSortState } from '../hooks/useSort';
import { SortHeader } from '../components/SortHeader';

type UserSortKey = 'email' | 'role' | 'is_active' | 'created_at';

const PAGE_SIZE = 50;

function fmt(date: string) {
  return new Date(date).toLocaleDateString();
}

function fmtBytes(n: number): string {
  if (n === 0) return '0 B';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function isLocked(user: AdminUser) {
  return !!(user.locked_until && new Date(user.locked_until) > new Date());
}

// ── Create User Modal ────────────────────────────────────────────────────────

interface CreateModalProps {
  onClose: () => void;
  onCreated: (user: AdminUser) => void;
}

function CreateUserModal({ onClose, onCreated }: CreateModalProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState('user');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError('');
    try {
      const user = await createUser({ email, password, role });
      onCreated(user);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span>Create User</span>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}
            <div className="form-group">
              <label>Email</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoFocus
                placeholder="user@example.com"
              />
            </div>
            <div className="form-group">
              <label>Password</label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
                placeholder="Min 8 characters"
              />
            </div>
            <div className="form-group">
              <label>Role</label>
              <select value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="user">user</option>
                <option value="admin">admin</option>
              </select>
            </div>
          </div>
          <div className="modal-footer">
            <button type="button" className="btn btn-secondary" onClick={onClose} disabled={saving}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Creating…' : 'Create User'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Delete Confirm Modal ─────────────────────────────────────────────────────

interface DeleteModalProps {
  user: AdminUser;
  onClose: () => void;
  onDeleted: (id: string) => void;
}

function DeleteUserModal({ user, onClose, onDeleted }: DeleteModalProps) {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  async function handleDelete() {
    setSaving(true);
    setError('');
    try {
      await deleteUser(user.id);
      onDeleted(user.id);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" style={{ maxWidth: 440 }} onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span>Delete User</span>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          {error && <div className="alert alert-error">{error}</div>}
          <p>Permanently delete <strong>{user.email}</strong>?</p>
          <p className="text-sm" style={{ marginTop: 6, color: '#888' }}>
            This removes the account and all associated tokens. Transfers and files are preserved.
          </p>
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose} disabled={saving}>Cancel</button>
          <button className="btn btn-danger" onClick={handleDelete} disabled={saving}>
            {saving ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Users Page ────────────────────────────────────────────────────────────────

export default function UsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState('');
  const [roleFilter, setRoleFilter] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<AdminUser | null>(null);
  const [storageMap, setStorageMap] = useState<Map<string, StorageUserUsage>>(new Map());
  const sort = useSortState<UserSortKey>('created_at', 'desc', () => setPage(0));

  const load = useCallback(async (offset = 0) => {
    setLoading(true);
    setError('');
    try {
      const [usersRes, storageRes] = await Promise.all([
        listUsers({
          limit: PAGE_SIZE,
          offset,
          search: search.trim() || undefined,
          role: roleFilter || undefined,
          sort_by: sort.sortKey,
          sort_dir: sort.sortDir,
        }),
        listStorageUsage({ limit: 200 }),
      ]);
      setUsers(usersRes.users ?? []);
      setTotal(usersRes.total ?? 0);
      const map = new Map<string, StorageUserUsage>();
      for (const u of storageRes.usage ?? []) map.set(u.user_id, u);
      setStorageMap(map);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [search, roleFilter, sort.sortKey, sort.sortDir]);

  useEffect(() => {
    load(page * PAGE_SIZE);
  }, [page, load]);

  function flash(msg: string) {
    setSuccess(msg);
    setTimeout(() => setSuccess(''), 3000);
  }

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
      flash(`${user.email} promoted to ${newRole}`);
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
      flash(`${user.email} ${user.is_active ? 'disabled' : 'enabled'}`);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  async function handleUnlock(user: AdminUser) {
    setSaving(true);
    try {
      await unlockUser(user.id);
      setUsers((prev) => prev.map((u) =>
        u.id === user.id ? { ...u, failed_login_attempts: 0, locked_until: null } : u));
      flash(`${user.email} unlocked`);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  async function handleVerify(user: AdminUser) {
    setSaving(true);
    try {
      await verifyUserEmail(user.id);
      setUsers((prev) => prev.map((u) =>
        u.id === user.id ? { ...u, email_verified: true } : u));
      flash(`${user.email} email verified`);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  function handleCreated(user: AdminUser) {
    setShowCreate(false);
    setTotal((t) => t + 1);
    setUsers((prev) => [user, ...prev]);
    flash(`User ${user.email} created`);
  }

  function handleDeleted(id: string) {
    setDeleteTarget(null);
    setUsers((prev) => prev.filter((u) => u.id !== id));
    setTotal((t) => t - 1);
    flash('User deleted');
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div className="page">
      {showCreate && (
        <CreateUserModal onClose={() => setShowCreate(false)} onCreated={handleCreated} />
      )}
      {deleteTarget && (
        <DeleteUserModal
          user={deleteTarget}
          onClose={() => setDeleteTarget(null)}
          onDeleted={handleDeleted}
        />
      )}

      <div className="page-header">
        <div>
          <h1 className="page-title">User Management</h1>
          <p className="page-subtitle">Manage accounts, roles, and access controls</p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
          + Add User
        </button>
      </div>

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
            <button className="btn btn-primary" type="submit" disabled={loading}>Search</button>
          </div>
        </div>
      </form>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}

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
                <SortHeader label="User" sortKey="email" sort={sort} />
                <SortHeader label="Role" sortKey="role" sort={sort} />
                <SortHeader label="Status" sortKey="is_active" sort={sort} />
                <th>Email</th>
                <th>Logins</th>
                <th>Storage</th>
                <SortHeader label="Joined" sortKey="created_at" sort={sort} />
                <th>Last Login</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id}>
                  <td>
                    <div style={{ fontWeight: 500 }}>{u.email}</div>
                    <div className="text-sm mono" style={{ color: '#aaa' }}>{u.id.slice(0, 8)}…</div>
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
                        title="Click to change role"
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
                      {u.email_verified ? 'Verified' : 'Unverified'}
                    </span>
                  </td>
                  <td className="text-sm">
                    {u.failed_login_attempts > 0 ? (
                      <span className="badge badge-orange">{u.failed_login_attempts} failed</span>
                    ) : (
                      <span style={{ color: '#aaa' }}>—</span>
                    )}
                    {isLocked(u) && (
                      <span className="badge badge-red" style={{ marginLeft: 4 }}>Locked</span>
                    )}
                  </td>
                  <td className="text-sm">
                    {(() => {
                      const s = storageMap.get(u.id);
                      if (!s) return <span style={{ color: '#aaa' }}>—</span>;
                      return (
                        <span title={`${s.file_count} file${s.file_count !== 1 ? 's' : ''}`}>
                          {fmtBytes(s.total_bytes)}
                        </span>
                      );
                    })()}
                  </td>
                  <td className="text-sm">{fmt(u.created_at)}</td>
                  <td className="text-sm">
                    {u.last_login_at
                      ? <span title={new Date(u.last_login_at).toLocaleString()}>{fmt(u.last_login_at)}</span>
                      : <span style={{ color: '#aaa' }}>Never</span>}
                  </td>
                  <td>
                    <div className="action-group">
                      <button
                        className={`btn btn-sm ${u.is_active ? 'btn-secondary' : 'btn-primary'}`}
                        disabled={saving}
                        onClick={() => handleToggleActive(u)}
                        title={u.is_active ? 'Disable account' : 'Enable account'}
                      >
                        {u.is_active ? 'Disable' : 'Enable'}
                      </button>
                      {isLocked(u) && (
                        <button
                          className="btn btn-sm btn-secondary"
                          disabled={saving}
                          onClick={() => handleUnlock(u)}
                          title="Clear lockout"
                        >
                          Unlock
                        </button>
                      )}
                      {!u.email_verified && (
                        <button
                          className="btn btn-sm btn-secondary"
                          disabled={saving}
                          onClick={() => handleVerify(u)}
                          title="Force verify email"
                        >
                          Verify
                        </button>
                      )}
                      <button
                        className="btn btn-sm btn-danger"
                        disabled={saving}
                        onClick={() => setDeleteTarget(u)}
                        title="Delete user"
                      >
                        Delete
                      </button>
                    </div>
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

