import React, { useState, useEffect, useCallback } from 'react';
import { fmt } from '../utils/dateFormat';
import {
  listUsers, createUser, updateUser, deleteUser, unlockUser, verifyUserEmail,
  resetUserPassword, setUserPassword, resetUserMFA,
  listStorageUsage, getUserQuota, setUserQuota, listUserQuotas,
  type AdminUser, type StorageUserUsage, type UserQuota, type QuotaOverride,
} from '../api/admin';
import { useSortState } from '../hooks/useSort';
import { SortHeader } from '../components/SortHeader';

type UserSortKey = 'email' | 'role' | 'is_active' | 'created_at' | 'last_login_at';

const PAGE_SIZE = 50;

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
  // per-user quota override
  const [quota, setQuota] = useState<UserQuota | null>(null);
  const [editingQuota, setEditingQuota] = useState(false);
  const [quotaInput, setQuotaInput] = useState('');
  const [quotaBusy, setQuotaBusy] = useState(false);

  useEffect(() => {
    getUserQuota(user.id).then(setQuota).catch(() => setQuota({ has_override: false }));
  }, [user.id]);

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
          <Row label="Storage quota">
            {quota === null ? (
              <span style={{ color: 'var(--color-text-muted)' }}>Loading…</span>
            ) : editingQuota ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                  <input
                    type="number"
                    min="0"
                    step="0.1"
                    value={quotaInput}
                    onChange={(e) => setQuotaInput(e.target.value)}
                    placeholder="e.g. 5"
                    style={{ width: 90, fontSize: 13, padding: '3px 8px' }}
                    autoFocus
                  />
                  <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>GB</span>
                  <button
                    className="btn btn-sm btn-primary"
                    disabled={quotaBusy || quotaInput === ''}
                    onClick={async () => {
                      const gb = parseFloat(quotaInput);
                      if (isNaN(gb) || gb <= 0) return;
                      setQuotaBusy(true);
                      try {
                        const res = await setUserQuota(user.id, Math.round(gb * 1024 * 1024 * 1024));
                        setQuota({ has_override: res.has_override, quota_bytes: res.quota_bytes });
                        setEditingQuota(false);
                        flash('Custom quota saved');
                      } catch (e: any) {
                        setActionError(e.message);
                      } finally {
                        setQuotaBusy(false);
                      }
                    }}
                  >
                    Save
                  </button>
                  <button
                    className="btn btn-sm btn-secondary"
                    disabled={quotaBusy}
                    onClick={() => setEditingQuota(false)}
                  >
                    Cancel
                  </button>
                </div>
                {quota.has_override && (
                  <button
                    className="btn btn-sm btn-secondary"
                    disabled={quotaBusy}
                    style={{ alignSelf: 'flex-start', color: 'var(--color-danger, #ef4444)' }}
                    onClick={async () => {
                      setQuotaBusy(true);
                      try {
                        await setUserQuota(user.id, null);
                        setQuota({ has_override: false });
                        setEditingQuota(false);
                        flash('Quota reset to global default');
                      } catch (e: any) {
                        setActionError(e.message);
                      } finally {
                        setQuotaBusy(false);
                      }
                    }}
                  >
                    Reset to global default
                  </button>
                )}
              </div>
            ) : (
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                {quota.has_override && quota.quota_bytes !== undefined
                  ? <span className="badge badge-orange">Custom: {fmtBytes(quota.quota_bytes)}</span>
                  : <span style={{ color: 'var(--color-text-muted)', fontSize: 12 }}>Global default</span>}
                <button
                  className="btn btn-sm btn-secondary"
                  onClick={() => {
                    setQuotaInput(quota.has_override && quota.quota_bytes !== undefined
                      ? (quota.quota_bytes / (1024 * 1024 * 1024)).toFixed(2)
                      : '');
                    setEditingQuota(true);
                  }}
                >
                  Edit
                </button>
              </div>
            )}
          </Row>
          <Row label="Joined">{fmt(user.created_at)}</Row>
          <Row label="Last login">
            {user.last_login_at ? fmt(user.last_login_at) : <span style={{ color: 'var(--color-text-muted)' }}>Never</span>}
          </Row>
          <Row label="MFA (TOTP)">
            <span className={`badge ${user.mfa_enabled ? 'badge-green' : 'badge-gray'}`} style={{ marginRight: 8 }}>
              {user.mfa_enabled ? 'Enabled' : 'Disabled'}
            </span>
            {user.mfa_enabled && (
              <button
                className="btn btn-sm btn-secondary"
                disabled={busy}
                onClick={() => act(
                  () => resetUserMFA(user.id),
                  () => { patch({ mfa_enabled: false }); flash('MFA reset'); }
                )}
              >
                Reset MFA
              </button>
            )}
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

// ── Column picker ────────────────────────────────────────────────────────────

type ColKey = 'role' | 'status' | 'mfa' | 'storage_used' | 'file_count' | 'failed_logins' | 'joined' | 'last_login' | 'storage_quota';

const ALL_COLUMNS: { key: ColKey; label: string; defaultOn: boolean }[] = [
  { key: 'role',          label: 'Role',          defaultOn: true  },
  { key: 'status',        label: 'Status',        defaultOn: true  },
  { key: 'mfa',           label: 'MFA',           defaultOn: true  },
  { key: 'storage_used',  label: 'Storage Used',  defaultOn: true  },
  { key: 'storage_quota', label: 'Storage Quota', defaultOn: true  },
  { key: 'file_count',    label: 'File Count',    defaultOn: false },
  { key: 'failed_logins', label: 'Failed Logins', defaultOn: false },
  { key: 'joined',        label: 'Joined',        defaultOn: true  },
  { key: 'last_login',    label: 'Last Login',    defaultOn: true  },
];

const COLS_LS_KEY = 'tenzo_users_columns';

function loadColPrefs(): Set<ColKey> {
  try {
    const raw = localStorage.getItem(COLS_LS_KEY);
    if (raw) {
      const arr = JSON.parse(raw) as string[];
      const valid = arr.filter((k): k is ColKey => ALL_COLUMNS.some(c => c.key === k));
      if (valid.length) return new Set(valid);
    }
  } catch { /* ignore */ }
  return new Set(ALL_COLUMNS.filter(c => c.defaultOn).map(c => c.key));
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
  const [quotaMap, setQuotaMap] = useState<Map<string, QuotaOverride>>(new Map());
  const sort = useSortState<UserSortKey>('created_at', 'desc', () => setPage(0));
  const [visibleCols, setVisibleCols] = useState<Set<ColKey>>(loadColPrefs);
  const [colPickerOpen, setColPickerOpen] = useState(false);
  const colPickerRef = React.useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!colPickerOpen) return;
    function handler(e: MouseEvent) {
      if (colPickerRef.current && !colPickerRef.current.contains(e.target as Node)) {
        setColPickerOpen(false);
      }
    }
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [colPickerOpen]);

  function toggleCol(key: ColKey) {
    setVisibleCols(prev => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      localStorage.setItem(COLS_LS_KEY, JSON.stringify([...next]));
      return next;
    });
  }

  const load = useCallback(async (offset = 0) => {
    setLoading(true);
    setError('');
    try {
      const [usersRes, storageRes, quotasRes] = await Promise.all([
        listUsers({
          limit: PAGE_SIZE,
          offset,
          search: search.trim() || undefined,
          role: roleFilter || undefined,
          sort_by: sort.sortKey,
          sort_dir: sort.sortDir,
        }),
        listStorageUsage({ limit: 200 }),
        listUserQuotas(),
      ]);
      setUsers(usersRes.users ?? []);
      setTotal(usersRes.total ?? 0);
      const map = new Map<string, StorageUserUsage>();
      for (const u of storageRes.usage ?? []) map.set(u.user_id, u);
      setStorageMap(map);
      const qmap = new Map<string, QuotaOverride>();
      for (const q of quotasRes.overrides ?? []) qmap.set(q.user_id, q);
      setQuotaMap(qmap);
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
  const vis = visibleCols;

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

      <div className="text-sm mb-16" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span>
          {loading ? 'Loading…' : `${total} user(s) total`}
          {totalPages > 1 && ` — page ${page + 1} of ${totalPages}`}
        </span>

        {/* ── Column picker ── */}
        <div ref={colPickerRef} style={{ position: 'relative' }}>
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => setColPickerOpen(o => !o)}
            style={{ display: 'flex', alignItems: 'center', gap: 5 }}
          >
            Columns
            <span style={{ fontSize: 10, opacity: 0.6 }}>{colPickerOpen ? '▲' : '▼'}</span>
          </button>
          {colPickerOpen && (
            <div style={{
              position: 'absolute', right: 0, top: 'calc(100% + 4px)', zIndex: 200,
              background: 'var(--color-surface)', border: '1px solid var(--color-border)',
              borderRadius: 8, padding: '6px 0', minWidth: 190,
              boxShadow: '0 4px 20px rgba(0,0,0,0.35)',
            }}>
              <div style={{ padding: '4px 14px 7px', fontSize: 11, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em', fontWeight: 600 }}>
                Visible columns
              </div>
              {ALL_COLUMNS.map(col => (
                <label
                  key={col.key}
                  style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '5px 14px', cursor: 'pointer', fontSize: 13, userSelect: 'none' }}
                >
                  <input
                    type="checkbox"
                    checked={vis.has(col.key)}
                    onChange={() => toggleCol(col.key)}
                    style={{ cursor: 'pointer', width: 14, height: 14 }}
                  />
                  {col.label}
                </label>
              ))}
              <div style={{ margin: '6px 14px 4px', borderTop: '1px solid var(--color-border)' }} />
              <div style={{ display: 'flex', gap: 6, padding: '2px 14px 4px' }}>
                <button
                  className="btn btn-secondary btn-sm"
                  style={{ fontSize: 11, padding: '2px 10px' }}
                  onClick={() => {
                    const all = new Set(ALL_COLUMNS.map(c => c.key));
                    setVisibleCols(all);
                    localStorage.setItem(COLS_LS_KEY, JSON.stringify([...all]));
                  }}
                >
                  Show all
                </button>
                <button
                  className="btn btn-secondary btn-sm"
                  style={{ fontSize: 11, padding: '2px 10px' }}
                  onClick={() => {
                    const defaults = new Set(ALL_COLUMNS.filter(c => c.defaultOn).map(c => c.key));
                    setVisibleCols(defaults);
                    localStorage.setItem(COLS_LS_KEY, JSON.stringify([...defaults]));
                  }}
                >
                  Reset
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {!loading && users.length === 0 ? (
        <div className="empty-state">No users found.</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <SortHeader label="User" sortKey="email" sort={sort} />
                {vis.has('role') && <SortHeader label="Role" sortKey="role" sort={sort} />}
                {vis.has('status') && <SortHeader label="Status" sortKey="is_active" sort={sort} />}
                {vis.has('mfa') && <th>MFA</th>}
                {vis.has('storage_used') && <th>Storage Used</th>}
                {vis.has('storage_quota') && <th>Quota</th>}
                {vis.has('file_count') && <th>Files</th>}
                {vis.has('failed_logins') && <th>Failed Logins</th>}
                {vis.has('joined') && <SortHeader label="Joined" sortKey="created_at" sort={sort} />}
                {vis.has('last_login') && <SortHeader label="Last Login" sortKey="last_login_at" sort={sort} />}
                <th></th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => {
                const s = storageMap.get(u.id);
                const q = quotaMap.get(u.id);
                return (
                  <tr key={u.id} style={{ cursor: 'default' }}>
                    <td>
                      <div style={{ fontWeight: 500 }}>{u.email}</div>
                      <div className="text-sm mono" style={{ color: '#aaa' }}>{u.id}</div>
                    </td>
                    {vis.has('role') && (
                      <td>
                        <span className={`badge ${u.role === 'admin' ? 'badge-orange' : 'badge-gray'}`}>{u.role}</span>
                      </td>
                    )}
                    {vis.has('status') && (
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
                    )}
                    {vis.has('mfa') && (
                      <td>
                        <span className={`badge ${u.mfa_enabled ? 'badge-green' : 'badge-gray'}`}>
                          {u.mfa_enabled ? 'On' : 'Off'}
                        </span>
                      </td>
                    )}
                    {vis.has('storage_used') && (
                      <td className="text-sm">
                        {s
                          ? <span title={`${s.file_count} file${s.file_count !== 1 ? 's' : ''}`}>{fmtBytes(s.total_bytes)}</span>
                          : <span style={{ color: '#aaa' }}>—</span>}
                      </td>
                    )}
                    {vis.has('storage_quota') && (
                      <td className="text-sm">
                        {q
                          ? <span className="badge badge-orange" title={`Set by ${q.updated_by}`}>Custom: {fmtBytes(q.quota_bytes)}</span>
                          : <span style={{ color: '#aaa' }}>Default</span>}
                      </td>
                    )}
                    {vis.has('file_count') && (
                      <td className="text-sm">
                        {s
                          ? `${s.file_count} file${s.file_count !== 1 ? 's' : ''}`
                          : <span style={{ color: '#aaa' }}>—</span>}
                      </td>
                    )}
                    {vis.has('failed_logins') && (
                      <td className="text-sm">
                        <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                          {u.failed_login_attempts > 0
                            ? <span className="badge badge-orange">{u.failed_login_attempts}</span>
                            : <span style={{ color: '#aaa' }}>0</span>}
                          {isLocked(u) && <span className="badge badge-red">Locked</span>}
                        </div>
                      </td>
                    )}
                    {vis.has('joined') && (
                      <td className="text-sm">{fmt(u.created_at)}</td>
                    )}
                    {vis.has('last_login') && (
                      <td className="text-sm">
                        {u.last_login_at
                          ? <span title={fmt(u.last_login_at)}>{fmt(u.last_login_at)}</span>
                          : <span style={{ color: '#aaa' }}>Never</span>}
                      </td>
                    )}
                    <td style={{ textAlign: 'right' }}>
                      <button className="btn btn-sm btn-secondary" onClick={() => setViewTarget(u)}>
                        View
                      </button>
                    </td>
                  </tr>
                );
              })}
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

