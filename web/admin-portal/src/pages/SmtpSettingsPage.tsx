import React, { useEffect, useState } from 'react';
import {
  getSmtpSettings,
  updateSmtpSettings,
  testSmtpSettings,
  type SmtpSettings,
  type SmtpTestResult,
} from '../api/smtp';

// ── Icons ─────────────────────────────────────────────────────────────────────

function IconMail() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
      <polyline points="22,6 12,13 2,6"/>
    </svg>
  );
}

function IconCheck() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12"/>
    </svg>
  );
}

function IconX() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
    </svg>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function SmtpSettingsPage() {
  const [settings, setSettings] = useState<SmtpSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [testResult, setTestResult] = useState<SmtpTestResult | null>(null);

  // form fields
  const [host, setHost] = useState('');
  const [port, setPort] = useState('1025');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [passwordChanged, setPasswordChanged] = useState(false);
  const [from, setFrom] = useState('');
  const [useTLS, setUseTLS] = useState(false);

  useEffect(() => {
    setLoading(true);
    getSmtpSettings()
      .then((s) => {
        setSettings(s);
        setHost(s.host);
        setPort(s.port || '1025');
        setUsername(s.username);
        setFrom(s.from);
        setUseTLS(s.use_tls);
        setLoadError(null);
      })
      .catch(() => setLoadError('Failed to load SMTP settings.'))
      .finally(() => setLoading(false));
  }, []);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setSaveError(null);
    setSaveSuccess(false);
    setTestResult(null);
    try {
      const payload: Parameters<typeof updateSmtpSettings>[0] = {
        host,
        port,
        username,
        from,
        use_tls: useTLS,
      };
      if (passwordChanged) {
        payload.password = password;
      }
      const updated = await updateSmtpSettings(payload);
      setSettings(updated);
      setPasswordChanged(false);
      setPassword('');
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 4000);
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save settings.');
    } finally {
      setSaving(false);
    }
  }

  async function handleTest() {
    setTesting(true);
    setTestResult(null);
    setSaveError(null);
    try {
      // Send current form values so the admin can test unsaved changes too.
      const payload: Parameters<typeof testSmtpSettings>[0] = {
        host,
        port,
        username,
        from,
        use_tls: useTLS,
      };
      if (passwordChanged && password) {
        payload.password = password;
      }
      const result = await testSmtpSettings(payload);
      setTestResult(result);
    } catch (err: unknown) {
      setTestResult({ ok: false, error: err instanceof Error ? err.message : 'Request failed.' });
    } finally {
      setTesting(false);
    }
  }

  if (loading) {
    return (
      <div className="page">
        <div className="page-header"><h1 className="page-title">Email / SMTP</h1></div>
        <div className="card"><p style={{ color: 'var(--color-text-muted)', fontSize: 14 }}>Loading…</p></div>
      </div>
    );
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Email / SMTP</h1>
          <p className="page-subtitle">Configure outbound email delivery. Changes take effect immediately without restart.</p>
        </div>
      </div>

      {loadError && (
        <div className="alert alert-error" style={{ marginBottom: 16 }}>{loadError}</div>
      )}
      {saveError && (
        <div className="alert alert-error" style={{ marginBottom: 16 }}>{saveError}</div>
      )}
      {saveSuccess && (
        <div className="alert alert-success" style={{ marginBottom: 16 }}>SMTP settings saved and applied to the notification service.</div>
      )}

      <form onSubmit={handleSave}>
        {/* ── Connection ── */}
        <div className="card" style={{ marginBottom: 20, padding: 24 }}>
          <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 4 }}>SMTP Server</h2>
          <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginBottom: 20 }}>
            Connection details for your mail transfer agent. Dev default: <code>mailhog:1025</code>.
          </p>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: 12, marginBottom: 16 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>
                SMTP Host
              </label>
              <input
                type="text"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="mailhog or smtp.example.com"
                disabled={saving}
                style={{ width: '100%' }}
              />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>
                Port
              </label>
              <input
                type="text"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                placeholder="1025"
                disabled={saving}
                style={{ width: 90 }}
              />
            </div>
          </div>

          {/* TLS toggle */}
          <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer', userSelect: 'none' }}>
            <div
              role="checkbox"
              aria-checked={useTLS}
              tabIndex={0}
              onClick={() => !saving && setUseTLS((v) => !v)}
              onKeyDown={(e) => e.key === ' ' && !saving && setUseTLS((v) => !v)}
              style={{
                width: 40,
                height: 22,
                borderRadius: 11,
                background: useTLS ? 'var(--color-primary)' : 'var(--color-border)',
                position: 'relative',
                cursor: saving ? 'not-allowed' : 'pointer',
                transition: 'background 0.2s',
                flexShrink: 0,
              }}
            >
              <div style={{
                position: 'absolute',
                top: 3,
                left: useTLS ? 21 : 3,
                width: 16,
                height: 16,
                borderRadius: '50%',
                background: '#fff',
                transition: 'left 0.2s',
                boxShadow: '0 1px 3px rgba(0,0,0,0.3)',
              }} />
            </div>
            <span style={{ fontSize: 14 }}>Use TLS (implicit, port 465)</span>
            <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
              — leave off for STARTTLS/plain (port 587/1025)
            </span>
          </label>
        </div>

        {/* ── Authentication ── */}
        <div className="card" style={{ marginBottom: 20, padding: 24 }}>
          <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 4 }}>Authentication</h2>
          <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginBottom: 20 }}>
            Leave blank for unauthenticated relays (MailHog, internal MTA).
          </p>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>
                Username
              </label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="user@example.com"
                autoComplete="username"
                disabled={saving}
                style={{ width: '100%' }}
              />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>
                Password
                {settings?.password_set && !passwordChanged && (
                  <span style={{
                    marginLeft: 8,
                    fontSize: 11,
                    color: 'var(--color-success, #16a34a)',
                    fontWeight: 400,
                  }}>
                    ✓ stored
                  </span>
                )}
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => { setPassword(e.target.value); setPasswordChanged(true); }}
                placeholder={settings?.password_set && !passwordChanged ? '••••••••' : 'password'}
                autoComplete="new-password"
                disabled={saving}
                style={{ width: '100%' }}
              />
              {passwordChanged && (
                <p style={{ fontSize: 11, color: 'var(--color-text-muted)', marginTop: 4 }}>
                  Leave blank to clear the stored password.
                </p>
              )}
            </div>
          </div>
        </div>

        {/* ── Sender address ── */}
        <div className="card" style={{ marginBottom: 20, padding: 24 }}>
          <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 4 }}>Sender Address</h2>
          <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginBottom: 16 }}>
            The <em>From</em> address used in all outbound emails.
          </p>
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>
              From address
            </label>
            <input
              type="email"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              placeholder="noreply@example.com"
              disabled={saving}
              style={{ width: '100%', maxWidth: 400 }}
            />
          </div>
        </div>

        {/* ── Actions ── */}
        <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <button type="submit" className="btn btn-primary" disabled={saving || testing}>
            {saving ? 'Saving…' : 'Save settings'}
          </button>

          <button
            type="button"
            className="btn btn-secondary"
            onClick={handleTest}
            disabled={saving || testing || !host || !port}
            title={!host || !port ? 'Host and port are required to send a test email' : ''}
          >
            <IconMail />
            {testing ? 'Sending…' : 'Send test email'}
          </button>

          {testResult && (
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              fontSize: 13,
              padding: '6px 12px',
              borderRadius: 6,
              background: testResult.ok
                ? 'rgba(22,163,74,0.1)'
                : 'rgba(239,68,68,0.1)',
              color: testResult.ok ? 'var(--color-success, #16a34a)' : '#ef4444',
              border: `1px solid ${testResult.ok ? 'rgba(22,163,74,0.3)' : 'rgba(239,68,68,0.3)'}`,
            }}>
              {testResult.ok ? <IconCheck /> : <IconX />}
              {testResult.ok
                ? 'Test email sent to your account address'
                : (testResult.error ?? 'Failed to send test email')}
            </div>
          )}
        </div>
      </form>

      {/* ── Info card ── */}
      <div style={{
        marginTop: 32,
        padding: '14px 18px',
        borderRadius: 8,
        background: 'rgba(99,102,241,0.07)',
        border: '1px solid rgba(99,102,241,0.2)',
        fontSize: 13,
        color: 'var(--color-text-muted)',
        lineHeight: 1.6,
      }}>
        <strong style={{ color: 'var(--color-text)' }}>How config reload works:</strong>{' '}
        Saving publishes a <code>CONFIG.smtp</code> NATS message. The notification service
        receives it instantly and rebuilds its SMTP sender — no restart needed.
        Env vars (<code>SMTP_HOST</code>, etc.) remain the bootstrap defaults used before
        any setting is saved here.
      </div>
    </div>
  );
}
