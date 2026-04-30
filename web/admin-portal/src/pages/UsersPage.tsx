import React, { useState, useEffect, useCallback } from 'react';
import {
  listUsers, createUser, updateUser, deleteUser, unlockUser, verifyUserEmail,
  resetUserPassword, setUserPassword,
  listStorageUsage,
  type AdminUser, type StorageUserUsage,
} from '../api/admin';
import { useSortState } from '../hooks/useSort';
import { SortHeader } from '../components/SortHeader';

type UserSortKey = 'email' | 'role' | 'is_active' | 'created_at' | 'last_login_at';

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

// ── User Detail Modal ─────────────────────────────────────────────────────────

type PwPanel = null | 'reset' | 'set';

function UserDetailModal({
  user: initial,
  storage,
  onClose,
  onUpdated,
  onDeleted,
  flash,
}: {
  user: AdminUser;
  storage: StorageUserUsage | undefined;
  onClose: () => void;
  onUpdated: (u: AdminUser) => void;
  onDeleted: (id: string) => void;
  flash: (msg: string) => void;
}) {
  const [user, setUser] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState('');
  const [pwPanel, setPwPanel] = useState<PwPanel>(null);
  // set-pw fields
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');
  // reset-pw result
  const [tempPw, setTempPw] = useState('');
  const [copied, setCopied] = useState(false);
  // delete confirm
  const [confirmDelete, setConfirmDelete] = useState(false);

  function act<T>(fn: () => Promise<T>, onOk: (v: T) => void) {
    setBusy(true);
    setActionError('');
    fn().then(onOk).catch((e: any) => setActionError(e.message)).finally(() => setBusy(false));
  }

  function patch(changes: Partial<AdminUser>) {
    const next = { ...user, ...changes };
    setUser(next);
    onUpdated(next);
  }

  function Row({ label, children }: { label: string; children: React.ReactNode }) {
    return (
      <div style={{ display: 'flex', gap: 12, padding: '9px 0', borderBottom: '1px solid var(--color-border)', alignItems: 'flex-start' }}>
        <span style={{ minWidth: 130, fontSize: 12, color: 'var(--color-text-muted)', paddingTop: 2 }}>{label}</span>
        <span style={{ flex: 1, fontSize: 13 }}>{children}</span>
      </div>
    );
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" style={{ maxWidth: 560, width: '95vw' }} onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span style={{ fontWeight: 700 }}>{user.email}</span>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>

        <div className="modal-body" style={{ padding: '0 24px 8px' }}>
          {actionError && <div className="alert alert-error" style={{ marginBottom: 12 }}>{actionError}</div>}

          {/* ── Info ── */}
          <Row label="User ID">
            <span className="mono" style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>{user.id}</span>
          </Row>
          <Row label="Role">
            <select
              value={user.role}
              disabled={busy}
              onChange={(e) => {
                const newRole = e.target.value;
                act(() => updateUser(user.id, { role: newRole }), () => {
                  patch({ role: newRole });
                  flash(`Role changed to ${newRole}`);
                });
              }}
              style={{ fontSize: 13, padding: '3px 8px' }}
            >
              <option value="user">user</option>
              <option value="admin">admin</option>
            </select>
          </Row>
          <Row label="Account status">
            <span className={`badge ${user.is_active ? 'badge-green' : 'badge-red'}`} style={{ marginRight: 8 }}>
              {user.is_active ? 'Active' : 'Disabled'}
            </span>
            <button
              className={`btn btn-sm ${user.is_active ? 'btn-secondary' : 'btn-primary'}`}
              disabled={busy}
              onClick={() => act(
                () => updateUser(user.id, { is_active: !user.is_active }),
                () => { patch({ is_active: !user.is_active }); flash(`Account ${user.is_active ? 'disabled' : 'enabled'}`); }
              )}
            >
              {user.is_active ? 'Disable' : 'Enable'}
            </button>
          </Row>
          <Row label="Email verified">
            <span className={`badge ${user.email_verified ? 'badge-green' : 'badge-gray'}`} style={{ marginRight: 8 }}>
              {user.email_verified ? 'Verified' : 'Unverified'}
            </span>
            {!user.email_verified && (
              <button
                className="btn btn-sm btn-secondary"
                disabled={busy}
                onClick={() => act(() => verifyUserEmail(user.id), () => { patch({ email_verified: true }); flash('Email verified'); })}
              >
                Force verify
              </button>
            )}
          </Row>
          <Row label="Login attempts">
            {user.failed_login_attempts > 0
              ? <span className="badge badge-orange">{user.failed_login_attempts} failed</span>
              : <span style={{ color: 'var(--color-text-muted)' }}>0</span>}
            {isLocked(user) && (
              <>
                <span className="badge badge-red" style={{ marginLeft: 8 }}>Locked</span>
                <button
                  className="btn btn-sm btn-secondary"
                  disabled={busy}
                  style={{ marginLeft: 8 }}
                  onClick={() => act(() => unlockUser(user.id), () => { patch({ failed_login_attempts: 0, locked_until: null }); flash('Account unlocked'); })}
                >
                  Unlock
                </button>
              </>
            )}
          </Row>
          <Row label="Storage used">
            {storage
              ? <span title={`${storage.file_count} file${storage.file_count !== 1 ? 's' : ''}`}>{fmtBytes(storage.total_bytes)}</span>
              : <span style={{ color: 'var(--color-text-muted)' }}>—</span>}
          </Row>
          <Row label="Joined">{new Date(user.created_at).toLocaleString()}</Row>
          <Row label="Last login">
            {user.last_login_at ? new Date(user.last_login_at).toLocaleString() : <span style={{ color: 'var(--color-text-muted)' }}>Never</span>}
          </Row>

          {/* ── Password section ── */}
          <div style={{ marginTop: 18, marginBottom: 4 }}>
            <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Password</span>
          </div>
          <div style={{ display: 'flex', gap: 8, marginBottom: pwPanel ? 12 : 0 }}>
            <button className={`btn btn-sm ${pwPanel === 'reset' ? 'btn-primary' : 'btn-secondary'}`} onClick={() => { setPwPanel(pwPanel === 'reset' ? null : 'reset'); setTempPw(''); setActionError(''); }} disabled={busy}>
              Reset (generate temp)
            </button>
            <button className={`btn btn-sm ${pwPanel === 'set' ? 'btn-primary' : 'btn-secondary'}`} onClick={() => { setPwPanel(pwPanel === 'set' ? null : 'set'); setNewPw(''); setConfirmPw(''); setActionError(''); }} disabled={busy}>
              Set password
            </button>
          </div>

          {pwPanel === 'reset' && (
            <div style={{ background: 'var(--color-nav-active)', border: '1px solid var(--color-border)', borderRadius: 8, padding: '14px 16px', marginBottom: 8 }}>
              {!tempPw ? (
                <>
                  <p className="text-sm" style={{ margin: '0 0 10px', color: 'var(--color-text-muted)' }}>
                    A random temporary password will be generated and shown once. The current password is replaced immediately.
                  </p>
                  <button className="btn btn-sm btn-primary" disabled={busy} onClick={() =>
                    act(() => resetUserPassword(user.id), (res) => { setTempPw(res.temp_password); flash('Password reset'); })
                  }>
                    {busy ? 'Generating…' : 'Generate'}
                  </button>
                </>
              ) : (
                <>
                  <p className="text-sm" style={{ margin: '0 0 8px', color: 'var(--color-text-muted)' }}>Temporary password (shown once):</p>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <code style={{ flex: 1, background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: 6, padding: '7px 12px', fontSize: 14, fontFamily: 'monospace', letterSpacing: '0.05em', userSelect: 'all' }}>
                      {tempPw}
                    </code>
                    <button className="btn btn-sm btn-secondary" onClick={() => {
                      navigator.clipboard.writeText(tempPw).then(() => { setCopied(true); setTimeout(() => setCopied(false), 1500); });
                    }}>
                      {copied ? 'Copied!' : 'Copy'}
                    </button>
                  </div>
                  <p className="text-sm" style={{ marginTop: 8, color: '#92400e', background: '#fef3c7', border: '1px solid #fbbf24', borderRadius: 5, padding: '5px 10px', display: 'inline-block' }}>⚠️ Not stored — copy before closing this panel.</p>
                </>
              )}
            </div>
          )}

          {pwPanel === 'set' && (
            <form style={{ background: 'var(--color-nav-active)', border: '1px solid var(--color-border)', borderRadius: 8, padding: '14px 16px', marginBottom: 8 }}
              onSubmit={(e) => {
                e.preventDefault();
                if (newPw !== confirmPw) { setActionError('Passwords do not match'); return; }
                act(() => setUserPassword(user.id, newPw), () => { setPwPanel(null); setNewPw(''); setConfirmPw(''); flash('Password updated'); });
              }}
            >
              <div className="form-group" style={{ margin: '0 0 10px' }}>
                <label>New password</label>
                <input type="password" value={newPw} onChange={(e) => setNewPw(e.target.value)} required minLength={8} placeholder="Min 8 characters" autoFocus />
              </div>
              <div className="form-group" style={{ margin: '0 0 12px' }}>
                <label>Confirm</label>
                <input type="password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} required placeholder="Repeat password" />
              </div>
              <button className="btn btn-sm btn-primary" type="submit" disabled={busy}>{busy ? 'Saving…' : 'Set password'}</button>
            </form>
          )}

          {/* ── Danger zone ── */}
          <div style={{ marginTop: 18, marginBottom: 8 }}>
            <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Danger zone</span>
          </div>
          {!confirmDelete ? (
            <button className="btn btn-sm btn-danger" onClick={() => setConfirmDelete(true)} disabled={busy}>Delete account</button>
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, background: 'var(--color-nav-active)', border: '1px solid var(--color-error-border)', borderRadius: 8, padding: '10px 14px' }}>
              <span className="text-sm" style={{ flex: 1 }}>Permanently delete <strong>{user.email}</strong>? This cannot be undone.</span>
              <button className="btn btn-sm btn-danger" disabled={busy} onClick={() =>
                act(() => deleteUser(user.id), () => { onDeleted(user.id); onClose(); flash('User deleted'); })
              }>
                {busy ? 'Deleting…' : 'Confirm delete'}
              </button>
              <button className="btn btn-sm btn-secondary" onClick={() => setConfirmDelete(false)}>Cancel</button>
            </div>
          )}
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Close</button>
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
  const [showCreate, setShowCreate] = useState(false);
  const [viewTarget, setViewTarget] = useState<AdminUser | null>(null);
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

  function handleCreated(user: AdminUser) {
    setShowCreate(false);
    setTotal((t) => t + 1);
    setUsers((prev) => [user, ...prev]);
    flash(`User ${user.email} created`);
  }

  function handleDeleted(id: string) {
    setUsers((prev) => prev.filter((u) => u.id !== id));
    setTotal((t) => t - 1);
  }

  function handleUpdated(updated: AdminUser) {
    setUsers((prev) => prev.map((u) => u.id === updated.id ? updated : u));
    setViewTarget(updated);
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div className="page">
      {showCreate && (
        <CreateUserModal onClose={() => setShowCreate(false)} onCreated={handleCreated} />
      )}
      {viewTarget && (
        <UserDetailModal
          user={viewTarget}
          storage={storageMap.get(viewTarget.id)}
          onClose={() => setViewTarget(null)}
          onUpdated={handleUpdated}
          onDeleted={(id) => { handleDeleted(id); setViewTarget(null); flash('User deleted'); }}
          flash={flash}
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
                <th>Storage</th>
                <SortHeader label="Joined" sortKey="created_at" sort={sort} />
                <SortHeader label="Last Login" sortKey="last_login_at" sort={sort} />
                <th></th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} style={{ cursor: 'default' }}>
                  <td>
                    <div style={{ fontWeight: 500 }}>{u.email}</div>
                    <div className="text-sm mono" style={{ color: '#aaa' }}>{u.id}</div>
                  </td>
                  <td>
                    <span className={`badge ${u.role === 'admin' ? 'badge-orange' : 'badge-gray'}`}>{u.role}</span>
                  </td>
                  <td>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                      <span className={`badge ${u.is_active ? 'badge-green' : 'badge-red'}`} style={{ width: 'fit-content' }}>
                        {u.is_active ? 'Active' : 'Disabled'}
                      </span>
                      <span className={`badge ${u.email_verified ? 'badge-green' : 'badge-gray'}`} style={{ width: 'fit-content' }}>
                        {u.email_verified ? '✓ Email' : 'Unverified'}
                      </span>
                      {isLocked(u) && <span className="badge badge-red" style={{ width: 'fit-content' }}>Locked</span>}
                    </div>
                  </td>
                  <td className="text-sm">
                    {(() => {
                      const s = storageMap.get(u.id);
                      if (!s) return <span style={{ color: '#aaa' }}>—</span>;
                      return <span title={`${s.file_count} file${s.file_count !== 1 ? 's' : ''}`}>{fmtBytes(s.total_bytes)}</span>;
                    })()}
                  </td>
                  <td className="text-sm">{fmt(u.created_at)}</td>
                  <td className="text-sm">
                    {u.last_login_at
                      ? <span title={new Date(u.last_login_at).toLocaleString()}>{fmt(u.last_login_at)}</span>
                      : <span style={{ color: '#aaa' }}>Never</span>}
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <button className="btn btn-sm btn-secondary" onClick={() => setViewTarget(u)}>
                      View
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

