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

  useEffect(() => {
    getPlatformConfig()
      .then((c) => {
        setConfig(c);
        setDateFormat(c.date_format);
        setTimeFormat(c.time_format);
        setTimezone(c.timezone);
      })
      .catch(() => setError('Failed to load platform settings.'));
  }, []);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const updated = await updatePlatformConfig({ date_format: dateFormat, time_format: timeFormat, timezone });
      setConfig(updated);
      setActivePrefs({ dateFormat: updated.date_format, timeFormat: updated.time_format, timezone: updated.timezone });
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
          <p className="page-subtitle">System-wide defaults for date, time, and timezone formatting.</p>
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

        <div style={{ display: 'flex', gap: 10 }}>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save changes'}
          </button>
        </div>

        {config.updated_at && (
          <p style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 12 }}>
            Last updated: {new Date(config.updated_at).toLocaleString()}
          </p>
        )}
      </form>
    </div>
  );
}
