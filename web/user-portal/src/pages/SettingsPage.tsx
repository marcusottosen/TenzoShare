import React, { useState, useEffect } from 'react';
import { isDarkMode, setDarkMode } from '../branding';
import { getPlatformConfig } from '../api/platform';
import { updatePreferences, getMe, getNotificationPrefs, updateNotificationPrefs, type NotificationPrefs } from '../api/auth';
import { updateAutoSaveContacts } from '../api/contacts';
import {
  setActivePrefs, getActivePrefs, COMMON_TIMEZONES,
  type DateFormat, type TimeFormat,
} from '../utils/dateFormat';

const DATE_FORMAT_OPTIONS: { value: DateFormat; label: string; example: string }[] = [
  { value: 'EU',   label: 'European (DD/MM/YYYY)',      example: '09/05/2026' },
  { value: 'US',   label: 'US (MM/DD/YYYY)',            example: '05/09/2026' },
  { value: 'ISO',  label: 'ISO 8601 (YYYY-MM-DD)',      example: '2026-05-09' },
  { value: 'DE',   label: 'Dot-separated (DD.MM.YYYY)', example: '09.05.2026' },
  { value: 'LONG', label: 'Long (D MMM YYYY)',           example: '9 May 2026' },
];

function ComingSoonBadge() {
  return (
    <span style={{
      fontSize: 10, fontWeight: 600, letterSpacing: '0.04em', textTransform: 'uppercase',
      background: 'var(--color-border)', color: 'var(--color-text-muted)',
      borderRadius: 4, padding: '2px 6px', marginLeft: 8, verticalAlign: 'middle',
    }}>
      Coming soon
    </span>
  );
}

function SettingRow({ label, description, children }: { label: string; description?: string; children: React.ReactNode }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '14px 0', borderBottom: '1px solid var(--color-border)', gap: 16,
    }}>
      <div style={{ flex: 1 }}>
        <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-primary)' }}>{label}</div>
        {description && <div style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 2 }}>{description}</div>}
      </div>
      <div>{children}</div>
    </div>
  );
}

export default function SettingsPage() {
  const [darkMode, setDarkModeState] = useState(() => isDarkMode());

  // Date/time prefs
  const [sysDefault, setSysDefault] = useState<{ dateFormat: DateFormat; timeFormat: TimeFormat; timezone: string } | null>(null);
  const [dateFormat, setDateFormat] = useState<DateFormat | ''>('');
  const [timeFormat, setTimeFormat] = useState<TimeFormat | ''>('');
  const [timezone, setTimezone] = useState<string>('');
  const [savingPrefs, setSavingPrefs] = useState(false);
  const [prefsSaved, setPrefsSaved] = useState(false);
  const [prefsError, setPrefsError] = useState<string | null>(null);

  // Notification prefs
  const [notifPrefs, setNotifPrefs] = useState<NotificationPrefs | null>(null);
  const [savingNotif, setSavingNotif] = useState(false);
  const [notifSaved, setNotifSaved] = useState(false);
  const [notifError, setNotifError] = useState<string | null>(null);

  useEffect(() => {
    // Load system defaults, current user prefs, and notification prefs
    Promise.all([
      getPlatformConfig().catch(() => null),
      getMe().catch(() => null),
      getNotificationPrefs().catch(() => null),
    ]).then(([sys, me, notif]) => {
        if (sys) {
          setSysDefault({ dateFormat: sys.date_format, timeFormat: sys.time_format, timezone: sys.timezone });
        }
        // Empty string = "use system default"
        setDateFormat((me?.date_format as DateFormat) ?? '');
        setTimeFormat((me?.time_format as TimeFormat) ?? '');
        setTimezone(me?.timezone ?? '');
        if (notif) setNotifPrefs(notif);
      });
  }, []);

  function handleDarkModeToggle(e: React.ChangeEvent<HTMLInputElement>) {
    const enabled = e.target.checked;
    setDarkModeState(enabled);
    setDarkMode(enabled);
  }

  async function handleSavePrefs(e: React.FormEvent) {
    e.preventDefault();
    setSavingPrefs(true);
    setPrefsError(null);
    setPrefsSaved(false);
    try {
      const updated = await updatePreferences({
        date_format: dateFormat || null,
        time_format: timeFormat || null,
        timezone: timezone || null,
      });
      // Update active prefs to reflect immediately
      const active = getActivePrefs();
      setActivePrefs({
        dateFormat: ((updated.date_format ?? sysDefault?.dateFormat ?? active.dateFormat) as DateFormat),
        timeFormat: ((updated.time_format ?? sysDefault?.timeFormat ?? active.timeFormat) as TimeFormat),
        timezone: updated.timezone ?? sysDefault?.timezone ?? active.timezone,
      });
      setPrefsSaved(true);
      setTimeout(() => setPrefsSaved(false), 3000);
    } catch {
      setPrefsError('Failed to save preferences.');
    } finally {
      setSavingPrefs(false);
    }
  }

  const sysFallback = sysDefault ? `System default: ${sysDefault.dateFormat}` : '';

  async function handleSaveNotifPrefs(e: React.FormEvent) {
    e.preventDefault();
    if (!notifPrefs) return;
    setSavingNotif(true);
    setNotifError(null);
    setNotifSaved(false);
    try {
      const [updated] = await Promise.all([
        updateNotificationPrefs(notifPrefs),
        updateAutoSaveContacts(notifPrefs.auto_save_contacts),
      ]);
      setNotifPrefs({ ...updated, auto_save_contacts: notifPrefs.auto_save_contacts });
      setNotifSaved(true);
      setTimeout(() => setNotifSaved(false), 3000);
    } catch {
      setNotifError('Failed to save notification preferences.');
    } finally {
      setSavingNotif(false);
    }
  }

  function toggleNotifPref(key: keyof NotificationPrefs) {
    setNotifPrefs(prev => prev ? { ...prev, [key]: !prev[key] } : prev);
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="page-subtitle">Application preferences</p>
        </div>
      </div>

      {/* Appearance */}
      <div className="card" style={{ marginBottom: 20 }}>
        <div className="card-header" style={{ marginBottom: 4 }}>
          <h2 className="card-title">Appearance</h2>
        </div>
        <SettingRow label="Dark mode" description="Switch the interface to a dark colour scheme.">
          <label className="toggle-switch">
            <input type="checkbox" checked={darkMode} onChange={handleDarkModeToggle} />
            <span className="toggle-track" />
          </label>
        </SettingRow>
      </div>

      {/* Date & Time */}
      <div className="card" style={{ marginBottom: 20 }}>
        <div className="card-header" style={{ marginBottom: 4 }}>
          <h2 className="card-title">Date &amp; Time</h2>
          <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginTop: 2 }}>
            Override the system-wide defaults for your account. Leave a field blank to use the system default.
          </p>
        </div>

        {prefsError && (
          <div className="alert alert-error" style={{ marginBottom: 12 }}>{prefsError}</div>
        )}
        {prefsSaved && (
          <div className="alert alert-success" style={{ marginBottom: 12 }}>Preferences saved.</div>
        )}

        <form onSubmit={handleSavePrefs} style={{ marginTop: 8 }}>
          {/* ── Date format ── */}
          <div style={{ padding: '14px 0', borderBottom: '1px solid var(--color-border)' }}>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 10 }}>
              <span style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-primary)' }}>Date format</span>
              {sysFallback && <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>{sysFallback}</span>}
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              <label style={{
                display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
                padding: '6px 14px', borderRadius: 8, cursor: 'pointer',
                border: `1.5px solid ${dateFormat === '' ? 'var(--color-primary)' : 'var(--color-border)'}`,
                background: dateFormat === '' ? 'color-mix(in srgb, var(--color-primary) 10%, transparent)' : 'var(--color-bg)',
                transition: 'border-color 0.15s, background 0.15s',
              }}>
                <input type="radio" name="date_format" value="" checked={dateFormat === ''} onChange={() => setDateFormat('')} style={{ display: 'none' }} />
                <span style={{ fontSize: 13, fontWeight: 500, color: dateFormat === '' ? 'var(--color-primary)' : 'var(--color-text-muted)' }}>System default</span>
              </label>
              {DATE_FORMAT_OPTIONS.map((opt) => (
                <label key={opt.value} style={{
                  display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
                  padding: '6px 14px', borderRadius: 8, cursor: 'pointer',
                  border: `1.5px solid ${dateFormat === opt.value ? 'var(--color-primary)' : 'var(--color-border)'}`,
                  background: dateFormat === opt.value ? 'color-mix(in srgb, var(--color-primary) 10%, transparent)' : 'var(--color-bg)',
                  transition: 'border-color 0.15s, background 0.15s',
                }}>
                  <input type="radio" name="date_format" value={opt.value} checked={dateFormat === opt.value} onChange={() => setDateFormat(opt.value)} style={{ display: 'none' }} />
                  <code style={{ fontSize: 13, fontWeight: 600, letterSpacing: '0.01em', color: dateFormat === opt.value ? 'var(--color-primary)' : 'var(--color-text-primary)' }}>{opt.example}</code>
                  <span style={{ fontSize: 11, color: 'var(--color-text-muted)', marginTop: 2 }}>{opt.label.split(' ')[0]}</span>
                </label>
              ))}
            </div>
          </div>

          {/* ── Time format ── */}
          <SettingRow label="Time format">
            <div style={{ display: 'inline-flex', border: '1.5px solid var(--color-border)', borderRadius: 8, overflow: 'hidden' }}>
              <label style={{
                padding: '6px 16px', cursor: 'pointer', fontSize: 13, fontWeight: 500,
                borderRight: '1px solid var(--color-border)',
                background: timeFormat === '' ? 'var(--color-primary)' : 'transparent',
                color: timeFormat === '' ? '#fff' : 'var(--color-text-primary)',
                transition: 'background 0.15s, color 0.15s',
              }}>
                <input type="radio" name="time_format" value="" checked={timeFormat === ''} onChange={() => setTimeFormat('')} style={{ display: 'none' }} />
                System
              </label>
              <label style={{
                padding: '6px 16px', cursor: 'pointer', fontSize: 13, fontWeight: 500,
                borderRight: '1px solid var(--color-border)',
                background: timeFormat === '24h' ? 'var(--color-primary)' : 'transparent',
                color: timeFormat === '24h' ? '#fff' : 'var(--color-text-primary)',
                transition: 'background 0.15s, color 0.15s',
              }}>
                <input type="radio" name="time_format" value="24h" checked={timeFormat === '24h'} onChange={() => setTimeFormat('24h')} style={{ display: 'none' }} />
                14:30
              </label>
              <label style={{
                padding: '6px 16px', cursor: 'pointer', fontSize: 13, fontWeight: 500,
                background: timeFormat === '12h' ? 'var(--color-primary)' : 'transparent',
                color: timeFormat === '12h' ? '#fff' : 'var(--color-text-primary)',
                transition: 'background 0.15s, color 0.15s',
              }}>
                <input type="radio" name="time_format" value="12h" checked={timeFormat === '12h'} onChange={() => setTimeFormat('12h')} style={{ display: 'none' }} />
                2:30 PM
              </label>
            </div>
          </SettingRow>

          {/* ── Timezone ── */}
          <SettingRow label="Timezone" description="Used for display only — files are stored in UTC.">
            <select
              id="timezone"
              className="form-input"
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              style={{ minWidth: 220, maxWidth: 300 }}
            >
              <option value="">System default{sysDefault ? ` (${sysDefault.timezone})` : ''}</option>
              {COMMON_TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>{tz}</option>
              ))}
            </select>
          </SettingRow>

          <div style={{ paddingTop: 4 }}>
            <button type="submit" className="btn btn-primary" disabled={savingPrefs}>
              {savingPrefs ? 'Saving…' : 'Save preferences'}
            </button>
          </div>
        </form>
      </div>

      {/* Notifications */}
      <div className="card">
        <div className="card-header" style={{ marginBottom: 4 }}>
          <h2 className="card-title">Notifications</h2>
        </div>
        {notifPrefs === null ? (
          <p style={{ fontSize: 13, color: 'var(--color-text-muted)', padding: '12px 0' }}>Loading…</p>
        ) : (
          <form onSubmit={handleSaveNotifPrefs}>
            {notifError && (
              <div style={{ background: 'var(--color-error-bg, #fee2e2)', color: 'var(--color-error, #dc2626)', borderRadius: 6, padding: '8px 12px', fontSize: 13, marginBottom: 12 }}>
                {notifError}
              </div>
            )}
            {notifSaved && (
              <div style={{ background: 'var(--color-success-bg, #dcfce7)', color: 'var(--color-success, #16a34a)', borderRadius: 6, padding: '8px 12px', fontSize: 13, marginBottom: 12 }}>
                Notification preferences saved.
              </div>
            )}
            <SettingRow label="Unsubscribe from all emails" description="Opt out of all non-essential notification emails.">
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={notifPrefs.notifications_opt_out}
                  onChange={() => toggleNotifPref('notifications_opt_out')}
                  style={{ width: 16, height: 16 }}
                />
                <span style={{ fontSize: 13 }}>Unsubscribe</span>
              </label>
            </SettingRow>
            <SettingRow label="Transfer received" description="Email when someone sends you a transfer.">
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: notifPrefs.notifications_opt_out ? 'not-allowed' : 'pointer', opacity: notifPrefs.notifications_opt_out ? 0.45 : 1 }}>
                <input
                  type="checkbox"
                  checked={notifPrefs.transfer_received}
                  disabled={notifPrefs.notifications_opt_out}
                  onChange={() => toggleNotifPref('transfer_received')}
                  style={{ width: 16, height: 16 }}
                />
                <span style={{ fontSize: 13 }}>Notify me</span>
              </label>
            </SettingRow>
            <SettingRow label="Download alerts" description="Email when someone downloads one of your transfers.">
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: notifPrefs.notifications_opt_out ? 'not-allowed' : 'pointer', opacity: notifPrefs.notifications_opt_out ? 0.45 : 1 }}>
                <input
                  type="checkbox"
                  checked={notifPrefs.download_notification}
                  disabled={notifPrefs.notifications_opt_out}
                  onChange={() => toggleNotifPref('download_notification')}
                  style={{ width: 16, height: 16 }}
                />
                <span style={{ fontSize: 13 }}>Notify me</span>
              </label>
            </SettingRow>
            <SettingRow label="Expiry reminders" description="Email before a transfer link expires.">
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: notifPrefs.notifications_opt_out ? 'not-allowed' : 'pointer', opacity: notifPrefs.notifications_opt_out ? 0.45 : 1 }}>
                <input
                  type="checkbox"
                  checked={notifPrefs.expiry_reminders}
                  disabled={notifPrefs.notifications_opt_out}
                  onChange={() => toggleNotifPref('expiry_reminders')}
                  style={{ width: 16, height: 16 }}
                />
                <span style={{ fontSize: 13 }}>Notify me</span>
              </label>
            </SettingRow>
            <SettingRow label="Auto-save contacts" description="Automatically save recipient emails to your Contacts when creating file shares or requests.">
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={notifPrefs.auto_save_contacts}
                  onChange={() => toggleNotifPref('auto_save_contacts')}
                  style={{ width: 16, height: 16 }}
                />
                <span style={{ fontSize: 13 }}>Save automatically</span>
              </label>
            </SettingRow>
            <div style={{ marginTop: 16, display: 'flex', gap: 10 }}>
              <button type="submit" className="btn btn-primary" disabled={savingNotif}>
                {savingNotif ? 'Saving…' : 'Save'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
