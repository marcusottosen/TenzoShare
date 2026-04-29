import React, { useEffect, useState } from 'react';
import { getStorageConfig, updateStorageConfig, type StorageConfig } from '../api/admin';

// ── Unit helpers ─────────────────────────────────────────────────────────────

const GiB = 1024 * 1024 * 1024;
const MiB = 1024 * 1024;

function bytesToUnit(bytes: number): { value: number; unit: 'MB' | 'GB' } {
  if (bytes === 0) return { value: 0, unit: 'GB' };
  if (bytes >= GiB && bytes % GiB === 0) return { value: bytes / GiB, unit: 'GB' };
  return { value: bytes / MiB, unit: 'MB' };
}

function unitToBytes(value: number, unit: 'MB' | 'GB'): number {
  return unit === 'GB' ? Math.round(value * GiB) : Math.round(value * MiB);
}

function fmtBytes(b: number): string {
  if (!b) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(b) / Math.log(k));
  return `${(b / Math.pow(k, i)).toFixed(i <= 1 ? 0 : 1)} ${sizes[i]}`;
}

// ── Byte input component ─────────────────────────────────────────────────────

interface ByteInputProps {
  label: string;
  hint: string;
  bytes: number;
  zeroLabel?: string;
  onChange: (bytes: number) => void;
  disabled?: boolean;
}

function ByteInput({ label, hint, bytes, zeroLabel = 'Unlimited (0)', onChange, disabled }: ByteInputProps) {
  const initial = bytesToUnit(bytes);
  const [value, setValue] = useState<string>(initial.value === 0 ? '' : String(initial.value));
  const [unit, setUnit] = useState<'MB' | 'GB'>(initial.unit);

  useEffect(() => {
    const u = bytesToUnit(bytes);
    setValue(u.value === 0 ? '' : String(u.value));
    setUnit(u.unit);
  }, [bytes]);

  function handleChange(raw: string) {
    setValue(raw);
    const n = parseFloat(raw);
    if (!raw || isNaN(n) || n < 0) onChange(0);
    else onChange(unitToBytes(n, unit));
  }

  function handleUnit(u: 'MB' | 'GB') {
    setUnit(u);
    const n = parseFloat(value);
    if (!isNaN(n) && n > 0) onChange(unitToBytes(n, u));
  }

  return (
    <div className="form-group">
      <label>{label}</label>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <input
          type="number"
          min={0}
          step={unit === 'GB' ? 1 : 100}
          placeholder={zeroLabel}
          value={value}
          disabled={disabled}
          onChange={(e) => handleChange(e.target.value)}
          style={{ width: 120 }}
        />
        <div style={{ display: 'flex', gap: 4 }}>
          {(['MB', 'GB'] as const).map((u) => (
            <button key={u} type="button"
              className={`btn btn-sm ${unit === u ? 'btn-primary' : 'btn-secondary'}`}
              disabled={disabled} onClick={() => handleUnit(u)}>{u}</button>
          ))}
        </div>
        {bytes > 0 && <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>= {fmtBytes(bytes)}</span>}
        {bytes === 0 && <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>{zeroLabel}</span>}
      </div>
      <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>{hint}</p>
    </div>
  );
}

// ── Days input component ─────────────────────────────────────────────────────

function DaysInput({ label, hint, days, onChange, disabled, min = 1 }: {
  label: string; hint: string; days: number;
  onChange: (d: number) => void; disabled?: boolean; min?: number;
}) {
  return (
    <div className="form-group">
      <label>{label}</label>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <input
          type="number"
          min={min}
          step={1}
          value={days}
          disabled={disabled}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10);
            if (!isNaN(n) && n >= min) onChange(n);
          }}
          style={{ width: 100 }}
        />
        <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>days</span>
        {days === 30 && <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>(≈ 1 month)</span>}
        {days === 90 && <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>(≈ 3 months)</span>}
        {days === 365 && <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>(≈ 1 year)</span>}
      </div>
      <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>{hint}</p>
    </div>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function StorageSettingsPage() {
  const [config, setConfig] = useState<StorageConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const [quotaEnabled, setQuotaEnabled] = useState(false);
  const [quotaBytes, setQuotaBytes] = useState(10 * GiB);
  const [maxUploadBytes, setMaxUploadBytes] = useState(0);
  const [retentionEnabled, setRetentionEnabled] = useState(false);
  const [retentionDays, setRetentionDays] = useState(30);
  const [orphanDays, setOrphanDays] = useState(90);

  useEffect(() => {
    getStorageConfig()
      .then((cfg) => {
        setConfig(cfg);
        setQuotaEnabled(cfg.quota_enabled);
        setQuotaBytes(cfg.quota_bytes_per_user);
        setMaxUploadBytes(cfg.max_upload_size_bytes);
        setRetentionEnabled(cfg.retention_enabled);
        setRetentionDays(cfg.retention_days || 30);
        setOrphanDays(cfg.orphan_retention_days || 90);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      const updated = await updateStorageConfig({
        quota_enabled: quotaEnabled,
        quota_bytes_per_user: quotaBytes,
        max_upload_size_bytes: maxUploadBytes,
        retention_enabled: retentionEnabled,
        retention_days: retentionDays,
        orphan_retention_days: orphanDays,
      });
      setConfig(updated);
      setSuccess('Storage settings saved.');
    } catch (e: unknown) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Storage Settings</h1>
          <p className="page-subtitle">Global storage policy — applies to all users</p>
        </div>
      </div>

      {loading ? (
        <div className="card">
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>
        </div>
      ) : (
        <form onSubmit={handleSave}>
          {/* ── User quota ─────────────────────────────────────────────── */}
          <div className="card" style={{ marginBottom: 20 }}>
            <div style={{ marginBottom: 16 }}>
              <h2 style={{ fontSize: 15, fontWeight: 600, margin: 0, color: 'var(--color-text-primary)' }}>
                Per-User Storage Quota
              </h2>
              <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                When enabled, uploads that would push a user over their quota are rejected.
              </p>
            </div>
            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
                <input type="checkbox" checked={quotaEnabled} onChange={(e) => setQuotaEnabled(e.target.checked)}
                  style={{ width: 16, height: 16, cursor: 'pointer' }} />
                <span style={{ fontWeight: 500 }}>Enable quota enforcement</span>
              </label>
              <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                When disabled, users can upload without limit.
              </p>
            </div>
            <ByteInput label="Quota per user"
              hint="Maximum total storage each user may consume. Checked on every upload."
              bytes={quotaBytes} zeroLabel="No limit"
              onChange={setQuotaBytes} disabled={!quotaEnabled} />
            {quotaEnabled && quotaBytes === 0 && (
              <div className="alert alert-warning" style={{ marginTop: 8 }}>
                Quota enforcement is on but the limit is 0 — all uploads will be blocked.
              </div>
            )}
          </div>

          {/* ── Per-file size cap ───────────────────────────────────────── */}
          <div className="card" style={{ marginBottom: 20 }}>
            <div style={{ marginBottom: 16 }}>
              <h2 style={{ fontSize: 15, fontWeight: 600, margin: 0, color: 'var(--color-text-primary)' }}>
                Maximum Upload Size
              </h2>
              <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                Enforced per file, for every user. Set to 0 to allow any file size.
              </p>
            </div>
            <ByteInput label="Max file size"
              hint="A single upload larger than this value will be rejected at the storage service."
              bytes={maxUploadBytes} zeroLabel="Unlimited" onChange={setMaxUploadBytes} />
          </div>

          {/* ── File retention ──────────────────────────────────────────── */}
          <div className="card" style={{ marginBottom: 20 }}>
            <div style={{ marginBottom: 16 }}>
              <h2 style={{ fontSize: 15, fontWeight: 600, margin: 0, color: 'var(--color-text-primary)' }}>
                Automatic File Retention
              </h2>
              <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                The system runs a cleanup cycle every hour. Files are deleted when their retention
                period expires. A file shared multiple times stays alive until <em>all</em> its
                shares have expired.
              </p>
            </div>

            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
                <input type="checkbox" checked={retentionEnabled} onChange={(e) => setRetentionEnabled(e.target.checked)}
                  style={{ width: 16, height: 16, cursor: 'pointer' }} />
                <span style={{ fontWeight: 500 }}>Enable automatic file deletion</span>
              </label>
              <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                When disabled, files are never automatically deleted (manual deletion only).
              </p>
            </div>

            <DaysInput
              label="Shared file retention"
              hint="Delete files this many days after all their shares have expired or been revoked."
              days={retentionDays}
              onChange={setRetentionDays}
              disabled={!retentionEnabled}
            />

            <DaysInput
              label="Unshared file retention (orphan)"
              hint="Delete files that were never shared (no transfers) after this many days. Prevents storage filling up with forgotten uploads."
              days={orphanDays}
              onChange={setOrphanDays}
              disabled={!retentionEnabled}
            />

            {retentionEnabled && (
              <div style={{ marginTop: 12, padding: '10px 14px', background: 'var(--color-nav-active)', borderRadius: 8, fontSize: 13, color: 'var(--color-text-muted)', lineHeight: 1.6 }}>
                <strong style={{ color: 'var(--color-text-primary)' }}>How it works:</strong>{' '}
                Files that have been shared are deleted <strong>{retentionDays} day{retentionDays !== 1 ? 's' : ''}</strong> after the last
                share expires. Files that were never shared are deleted after{' '}
                <strong>{orphanDays} day{orphanDays !== 1 ? 's' : ''}</strong>. All deletions are logged in the Purge Log on the
                Storage Files page. You can also trigger a purge manually from there.
              </div>
            )}
          </div>

          {/* ── Actions ─────────────────────────────────────────────────── */}
          <div className="card">
            {error && <div className="alert alert-error" style={{ marginBottom: 12 }}>{error}</div>}
            {success && <div className="alert alert-success" style={{ marginBottom: 12 }}>{success}</div>}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <button type="submit" className="btn btn-primary" disabled={saving}>
                {saving ? 'Saving…' : 'Save Settings'}
              </button>
              {config && (
                <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                  Last updated {new Date(config.updated_at).toLocaleString()} by {config.updated_by}
                </span>
              )}
            </div>
          </div>
        </form>
      )}
    </div>
  );
}

