import React, { useEffect, useState } from 'react';
import { getPlatformConfig, updatePlatformConfig, type PlatformConfig } from '../api/platform';
import { setActivePrefs, COMMON_TIMEZONES, type DateFormat, type TimeFormat } from '../utils/dateFormat';

const DATE_FORMAT_OPTIONS: { value: DateFormat; label: string; example: string }[] = [
  { value: 'EU',   label: 'European (DD/MM/YYYY)',   example: '09/05/2026' },
  { value: 'US',   label: 'US (MM/DD/YYYY)',         example: '05/09/2026' },
  { value: 'ISO',  label: 'ISO 8601 (YYYY-MM-DD)',   example: '2026-05-09' },
  { value: 'DE',   label: 'Dot-separated (DD.MM.YYYY)', example: '09.05.2026' },
  { value: 'LONG', label: 'Long (D MMM YYYY)',        example: '9 May 2026' },
];

export default function GeneralSettingsPage() {
  const [config, setConfig] = useState<PlatformConfig | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // local form state
  const [dateFormat, setDateFormat] = useState<DateFormat>('EU');
  const [timeFormat, setTimeFormat] = useState<TimeFormat>('24h');
  const [timezone, setTimezone] = useState('UTC');
  const [portalUrl, setPortalUrl] = useState('');
  const [downloadUrl, setDownloadUrl] = useState('');
  const [linkPolicy, setLinkPolicy] = useState<'none' | 'password' | 'email' | 'either'>('none');

  useEffect(() => {
    getPlatformConfig()
      .then((c) => {
        setConfig(c);
        setDateFormat(c.date_format);
        setTimeFormat(c.time_format);
        setTimezone(c.timezone);
        setPortalUrl(c.portal_url ?? '');
        setDownloadUrl(c.download_url ?? '');
        setLinkPolicy(c.link_protection_policy ?? 'none');
      })
      .catch(() => setError('Failed to load platform settings.'));
  }, []);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const updated = await updatePlatformConfig({ date_format: dateFormat, time_format: timeFormat, timezone, portal_url: portalUrl, download_url: downloadUrl, link_protection_policy: linkPolicy });
      setConfig(updated);
      setActivePrefs({ dateFormat: updated.date_format, timeFormat: updated.time_format, timezone: updated.timezone });
      setPortalUrl(updated.portal_url ?? '');
      setDownloadUrl(updated.download_url ?? '');
      setLinkPolicy(updated.link_protection_policy ?? 'none');
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch {
      setError('Failed to save settings.');
    } finally {
      setSaving(false);
    }
  }

  if (!config) {
    return (
      <div className="page">
        <div className="page-header"><h1 className="page-title">General Settings</h1></div>
        {error
          ? <div className="alert alert-error">{error}</div>
          : <div className="card"><p style={{ color: 'var(--color-text-muted)', fontSize: 14 }}>Loading…</p></div>}
      </div>
    );
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">General Settings</h1>
          <p className="page-subtitle">System-wide defaults for date, time, timezone, and platform URLs.</p>
        </div>
      </div>

      {error && <div className="alert alert-error" style={{ marginBottom: 16 }}>{error}</div>}
      {saved && <div className="alert alert-success" style={{ marginBottom: 16 }}>Settings saved.</div>}

      <form onSubmit={handleSave}>
        <div className="card" style={{ marginBottom: 20 }}>
          <div className="card-header">
            <h2 className="card-title">Date &amp; Time Format</h2>
            <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginTop: 2 }}>
              Users can override these defaults in their personal settings.
            </p>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 20, marginTop: 16 }}>
            {/* Date format */}
            <div>
              <label className="form-label">Date format</label>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 6 }}>
                {DATE_FORMAT_OPTIONS.map((opt) => (
                  <label key={opt.value} style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
                    <input
                      type="radio"
                      name="date_format"
                      value={opt.value}
                      checked={dateFormat === opt.value}
                      onChange={() => setDateFormat(opt.value)}
                    />
                    <span style={{ fontSize: 14 }}>{opt.label}</span>
                    <code style={{
                      fontSize: 12,
                      background: 'var(--color-bg)',
                      border: '1px solid var(--color-border)',
                      borderRadius: 4,
                      padding: '1px 6px',
                      color: 'var(--color-text-muted)',
                    }}>{opt.example}</code>
                  </label>
                ))}
              </div>
            </div>

            {/* Time format */}
            <div>
              <label className="form-label">Time format</label>
              <div style={{ display: 'flex', gap: 20, marginTop: 6 }}>
                {([['24h', '14:30 (24-hour)'], ['12h', '2:30 PM (12-hour)']] as const).map(([val, label]) => (
                  <label key={val} style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
                    <input
                      type="radio"
                      name="time_format"
                      value={val}
                      checked={timeFormat === val}
                      onChange={() => setTimeFormat(val)}
                    />
                    <span style={{ fontSize: 14 }}>{label}</span>
                  </label>
                ))}
              </div>
            </div>

            {/* Timezone */}
            <div>
              <label className="form-label" htmlFor="timezone">Default timezone</label>
              <select
                id="timezone"
                className="form-input"
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                style={{ maxWidth: 360 }}
              >
                {COMMON_TIMEZONES.map((tz) => (
                  <option key={tz} value={tz}>{tz}</option>
                ))}
              </select>
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 4 }}>
                All timestamps will display in this timezone unless a user sets their own.
              </p>
            </div>
          </div>
        </div>

        <div className="card" style={{ marginTop: 20, marginBottom: 20 }}>
          <div className="card-header">
            <h2 className="card-title">Platform URLs</h2>
            <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginTop: 2 }}>
              Base URLs used when building links in outgoing emails. Leave blank to fall back to the server's configured defaults.
            </p>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, marginTop: 16 }}>
            {/* Portal URL */}
            <div>
              <label htmlFor="portal_url" style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 6, color: 'var(--color-text-primary)' }}>
                User Portal URL
              </label>
              <div style={{ position: 'relative' }}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"
                  style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', width: 15, height: 15, color: 'var(--color-text-muted)', pointerEvents: 'none' }}>
                  <circle cx="12" cy="12" r="10" /><line x1="2" y1="12" x2="22" y2="12" />
                  <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
                </svg>
                <input
                  id="portal_url"
                  type="text"
                  className="form-input"
                  value={portalUrl}
                  onChange={(e) => setPortalUrl(e.target.value)}
                  placeholder="https://app.example.com"
                  style={{ paddingLeft: 32, width: '100%' }}
                />
              </div>
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 6, lineHeight: 1.5 }}>
                Used in verification and password-reset email links.
              </p>
            </div>

            {/* Download URL */}
            <div>
              <label htmlFor="download_url" style={{ display: 'block', fontSize: 13, fontWeight: 500, marginBottom: 6, color: 'var(--color-text-primary)' }}>
                Download URL
              </label>
              <div style={{ position: 'relative' }}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"
                  style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', width: 15, height: 15, color: 'var(--color-text-muted)', pointerEvents: 'none' }}>
                  <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                  <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
                </svg>
                <input
                  id="download_url"
                  type="text"
                  className="form-input"
                  value={downloadUrl}
                  onChange={(e) => setDownloadUrl(e.target.value)}
                  placeholder="https://app.example.com"
                  style={{ paddingLeft: 32, width: '100%' }}
                />
              </div>
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 6, lineHeight: 1.5 }}>
                Used in transfer download links sent to recipients.
              </p>
            </div>
          </div>
        </div>

        {config.updated_at && (
          <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 12 }}>
            Last updated: {new Date(config.updated_at).toLocaleString()}
          </p>
        )}

        <div className="card" style={{ marginTop: 20, marginBottom: 20 }}>
          <div className="card-header">
            <h2 className="card-title">Transfer Link Protection</h2>
            <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginTop: 2 }}>
              Require all transfer links to be protected. Open (public) links are blocked when a policy is active.
            </p>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginTop: 16 }}>
            {([
              ['none',     'No requirement',          'Users may create unprotected links (default).'],
              ['password', 'Password required',       'Every transfer must have a password set before it can be created.'],
              ['email',    'Recipient email required', 'Every transfer must include at least one recipient email address.'],
              ['either',   'Password or email',       'Every transfer must have a password OR at least one recipient email.'],
            ] as const).map(([val, label, desc]) => (
              <label key={val} style={{ display: 'flex', alignItems: 'flex-start', gap: 10, cursor: 'pointer', padding: '10px 12px', borderRadius: 8, border: `1px solid ${linkPolicy === val ? 'var(--color-primary)' : 'var(--color-border)'}`, background: linkPolicy === val ? 'var(--color-primary-light, rgba(99,102,241,0.06))' : 'transparent' }}>
                <input
                  type="radio"
                  name="link_policy"
                  value={val}
                  checked={linkPolicy === val}
                  onChange={() => setLinkPolicy(val)}
                  style={{ marginTop: 2 }}
                />
                <div>
                  <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-primary)' }}>{label}</div>
                  <div style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 2 }}>{desc}</div>
                </div>
              </label>
            ))}
          </div>
        </div>

        <div style={{ display: 'flex', gap: 10, marginBottom: 8 }}>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      </form>
    </div>
  );
}
