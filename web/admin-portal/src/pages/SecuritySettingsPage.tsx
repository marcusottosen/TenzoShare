import React, { useEffect, useState } from 'react';
import {
  getAuthConfig,
  updateAuthConfig,
  type AuthLockoutConfig,
} from '../api/admin';

// ── Page ─────────────────────────────────────────────────────────────────────

export default function SecuritySettingsPage() {
  const [config, setConfig] = useState<AuthLockoutConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // form state
  const [maxAttempts, setMaxAttempts] = useState(10);
  const [lockoutMins, setLockoutMins] = useState(15);

  useEffect(() => {
    setLoading(true);
    getAuthConfig()
      .then((c) => {
        setConfig(c);
        setMaxAttempts(c.max_failed_attempts);
        setLockoutMins(c.lockout_duration_minutes);
      })
      .catch(() => setError('Failed to load lockout config.'))
      .finally(() => setLoading(false));
  }, []);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const updated = await updateAuthConfig({
        max_failed_attempts: maxAttempts,
        lockout_duration_minutes: lockoutMins,
      });
      setConfig(updated);
      setMaxAttempts(updated.max_failed_attempts);
      setLockoutMins(updated.lockout_duration_minutes);
      setSuccess('Lockout policy saved.');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to save.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="page-content" style={{ maxWidth: 640 }}>
      <div className="page-header" style={{ marginBottom: 24 }}>
        <h1 className="page-title">Security Settings</h1>
        <p className="page-subtitle" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
          Configure account lockout policy. Changes take effect within 60 seconds.
        </p>
      </div>

      {loading && <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>}

      {!loading && (
        <div className="card" style={{ padding: 24 }}>
          <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 20 }}>Account Lockout Policy</h2>

          <form onSubmit={handleSave}>
            {/* Max failed attempts */}
            <div className="form-group" style={{ marginBottom: 24 }}>
              <label htmlFor="maxAttempts" style={{ display: 'block', marginBottom: 6, fontWeight: 500 }}>
                Max failed login attempts
              </label>
              <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                <input
                  id="maxAttempts"
                  type="number"
                  min={1}
                  max={100}
                  step={1}
                  value={maxAttempts}
                  onChange={(e) => {
                    const n = parseInt(e.target.value, 10);
                    if (!isNaN(n) && n >= 1) setMaxAttempts(n);
                  }}
                  style={{ width: 100 }}
                  disabled={saving}
                />
                <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>attempts</span>
              </div>
              <div style={{ display: 'flex', gap: 6, marginTop: 8 }}>
                {[3, 5, 10, 20].map((n) => (
                  <button
                    key={n}
                    type="button"
                    className={`btn btn-sm ${maxAttempts === n ? 'btn-primary' : 'btn-secondary'}`}
                    onClick={() => setMaxAttempts(n)}
                    disabled={saving}
                  >
                    {n}
                  </button>
                ))}
              </div>
              <p className="text-sm" style={{ marginTop: 6, color: 'var(--color-text-muted)' }}>
                The account is locked after this many consecutive failed login attempts. Admins can
                reset this via the Unlock action in User Management.
              </p>
            </div>

            {/* Lockout duration */}
            <div className="form-group" style={{ marginBottom: 28 }}>
              <label htmlFor="lockoutMins" style={{ display: 'block', marginBottom: 6, fontWeight: 500 }}>
                Lockout duration
              </label>
              <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                <input
                  id="lockoutMins"
                  type="number"
                  min={1}
                  max={1440}
                  step={1}
                  value={lockoutMins}
                  onChange={(e) => {
                    const n = parseInt(e.target.value, 10);
                    if (!isNaN(n) && n >= 1) setLockoutMins(n);
                  }}
                  style={{ width: 100 }}
                  disabled={saving}
                />
                <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>minutes</span>
                <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                  {lockoutMins === 15
                    ? '(15 min)'
                    : lockoutMins === 30
                    ? '(30 min)'
                    : lockoutMins === 60
                    ? '(1 hour)'
                    : lockoutMins === 1440
                    ? '(24 hours)'
                    : ''}
                </span>
              </div>
              <div style={{ display: 'flex', gap: 6, marginTop: 8 }}>
                {[
                  { label: '15 min', value: 15 },
                  { label: '30 min', value: 30 },
                  { label: '1 hr',   value: 60 },
                  { label: '24 hr',  value: 1440 },
                ].map((p) => (
                  <button
                    key={p.value}
                    type="button"
                    className={`btn btn-sm ${lockoutMins === p.value ? 'btn-primary' : 'btn-secondary'}`}
                    onClick={() => setLockoutMins(p.value)}
                    disabled={saving}
                  >
                    {p.label}
                  </button>
                ))}
              </div>
              <p className="text-sm" style={{ marginTop: 6, color: 'var(--color-text-muted)' }}>
                How long the account stays locked before allowing new attempts. Admins can unlock
                immediately via User Management.
              </p>
            </div>

            {error && (
              <div className="alert alert-error" style={{ marginBottom: 16 }}>{error}</div>
            )}
            {success && (
              <div className="alert alert-success" style={{ marginBottom: 16 }}>{success}</div>
            )}

            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save Changes'}
            </button>
          </form>

          {config && (
            <p className="text-sm" style={{ marginTop: 16, color: 'var(--color-text-muted)' }}>
              Last updated: {config.updated_at ? new Date(config.updated_at).toLocaleString() : '—'}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
