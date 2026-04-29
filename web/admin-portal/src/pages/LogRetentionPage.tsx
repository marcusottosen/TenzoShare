import React, { useEffect, useState } from 'react';
import {
  getAuditConfig,
  updateAuditConfig,
  getAuditStats,
  triggerAuditPurge,
  type AuditConfig,
  type AuditStats,
} from '../api/admin';

// ── Days input (local copy matching StorageSettingsPage pattern) ──────────────

function DaysInput({
  label, hint, days, onChange, disabled, min = 1,
}: {
  label: string; hint: string; days: number;
  onChange: (d: number) => void; disabled?: boolean; min?: number;
}) {
  const presets = [
    { label: '90 d', days: 90, note: '≈ 3 months' },
    { label: '1 yr',  days: 365, note: 'SOC 2 / PCI-DSS minimum' },
    { label: '2 yr',  days: 730, note: '' },
    { label: '6 yr',  days: 2190, note: 'HIPAA minimum' },
  ];

  return (
    <div className="form-group">
      <label>{label}</label>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
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
        <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
          {days === 365 ? '(≈ 1 year)' : days === 730 ? '(≈ 2 years)' : days === 2190 ? '(≈ 6 years)' : days === 90 ? '(≈ 3 months)' : ''}
        </span>
      </div>
      <div style={{ display: 'flex', gap: 6, marginTop: 8, flexWrap: 'wrap' }}>
        {presets.map((p) => (
          <button
            key={p.days}
            type="button"
            disabled={disabled}
            className={`btn btn-sm ${days === p.days ? 'btn-primary' : 'btn-secondary'}`}
            onClick={() => onChange(p.days)}
            title={p.note}
          >
            {p.label}
          </button>
        ))}
      </div>
      <p className="text-sm" style={{ marginTop: 6, color: 'var(--color-text-muted)' }}>{hint}</p>
    </div>
  );
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtNumber(n: number): string {
  return n.toLocaleString();
}

function fmtDate(iso: string | null): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleString();
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function LogRetentionPage() {
  const [config, setConfig] = useState<AuditConfig | null>(null);
  const [stats, setStats] = useState<AuditStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [purging, setPurging] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [purgeResult, setPurgeResult] = useState<string>('');
  const [confirmPurge, setConfirmPurge] = useState(false);

  const [retentionEnabled, setRetentionEnabled] = useState(true);
  const [retentionDays, setRetentionDays] = useState(365);

  useEffect(() => {
    Promise.all([getAuditConfig(), getAuditStats()])
      .then(([cfg, st]) => {
        setConfig(cfg);
        setStats(st);
        setRetentionEnabled(cfg.retention_enabled);
        setRetentionDays(cfg.retention_days || 365);
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
      const updated = await updateAuditConfig({
        retention_enabled: retentionEnabled,
        retention_days: retentionDays,
      });
      setConfig(updated);
      setSuccess('Log retention settings saved.');
    } catch (e: unknown) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  async function handlePurge() {
    setConfirmPurge(false);
    setPurging(true);
    setPurgeResult('');
    setError('');
    try {
      const result = await triggerAuditPurge();
      if (result.message) {
        setPurgeResult(result.message);
      } else {
        setPurgeResult(
          `Purge complete — ${fmtNumber(result.deleted)} log ${result.deleted === 1 ? 'entry' : 'entries'} deleted (older than ${result.retention_days ?? retentionDays} days).`,
        );
      }
      // Refresh stats after purge
      getAuditStats().then(setStats).catch(() => {});
    } catch (e: unknown) {
      setError((e as Error).message);
    } finally {
      setPurging(false);
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Log Retention</h1>
          <p className="page-subtitle">Audit log lifecycle policy — control how long activity records are kept</p>
        </div>
      </div>

      {loading ? (
        <div className="card">
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>
        </div>
      ) : (
        <>
          {/* ── Compliance reference ─────────────────────────────────────────── */}
          <div className="card" style={{ marginBottom: 20, borderLeft: '3px solid var(--color-primary)' }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, margin: '0 0 10px', color: 'var(--color-text-primary)' }}>
              Compliance Reference
            </h2>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ textAlign: 'left', color: 'var(--color-text-muted)', borderBottom: '1px solid var(--color-border)' }}>
                  <th style={{ padding: '4px 12px 4px 0' }}>Framework</th>
                  <th style={{ padding: '4px 12px 4px 0' }}>Minimum retention</th>
                  <th style={{ padding: '4px 0' }}>Notes</th>
                </tr>
              </thead>
              <tbody>
                {[
                  { fw: 'GDPR', min: 'Organisation-defined', note: 'Must justify retention period; include in privacy notice' },
                  { fw: 'SOC 2 Type II', min: '12 months (365 days)', note: 'Recommended default; 3 months online + rest archived' },
                  { fw: 'PCI-DSS v4', min: '12 months (365 days)', note: '3 months must be immediately available' },
                  { fw: 'HIPAA', min: '6 years (2 190 days)', note: 'From date of creation or last effective date' },
                  { fw: 'NIS2', min: 'Organisation-defined', note: '"Appropriate" period; document your policy' },
                  { fw: 'ISO 27001', min: 'Organisation-defined', note: 'Define retention in information security policy' },
                ].map(({ fw, min, note }) => (
                  <tr key={fw} style={{ borderBottom: '1px solid var(--color-border)' }}>
                    <td style={{ padding: '6px 12px 6px 0', fontWeight: 500 }}>{fw}</td>
                    <td style={{ padding: '6px 12px 6px 0' }}>{min}</td>
                    <td style={{ padding: '6px 0', color: 'var(--color-text-muted)' }}>{note}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <form onSubmit={handleSave}>
            {/* ── Automatic log purging ──────────────────────────────────────── */}
            <div className="card" style={{ marginBottom: 20 }}>
              <div style={{ marginBottom: 16 }}>
                <h2 style={{ fontSize: 15, fontWeight: 600, margin: 0, color: 'var(--color-text-primary)' }}>
                  Automatic Log Purging
                </h2>
                <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                  When enabled, audit log entries older than the configured period are automatically
                  deleted daily. When disabled, logs accumulate indefinitely — ensure you have
                  adequate storage for your expected log volume.
                </p>
              </div>

              <div className="form-group">
                <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
                  <input
                    type="checkbox"
                    checked={retentionEnabled}
                    onChange={(e) => setRetentionEnabled(e.target.checked)}
                    style={{ width: 16, height: 16, cursor: 'pointer' }}
                  />
                  <span style={{ fontWeight: 500 }}>Enable automatic log deletion</span>
                </label>
                <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                  When disabled, audit logs are never automatically deleted (manual purge only).
                  Recommended for regulated industries where audit trails must be preserved.
                </p>
              </div>

              <DaysInput
                label="Retention period"
                hint="Audit log entries older than this will be deleted during the nightly purge cycle. Default: 365 days (SOC 2 / PCI-DSS baseline)."
                days={retentionDays}
                onChange={setRetentionDays}
                disabled={!retentionEnabled}
                min={1}
              />
            </div>

            {/* ── Error / success feedback ───────────────────────────────────── */}
            {error && (
              <div className="alert alert-error" style={{ marginBottom: 16 }}>{error}</div>
            )}
            {success && (
              <div className="alert alert-success" style={{ marginBottom: 16 }}>{success}</div>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 24 }}>
              <button type="submit" className="btn btn-primary" disabled={saving}>
                {saving ? 'Saving…' : 'Save settings'}
              </button>
            </div>
          </form>

          {/* ── Current log statistics ─────────────────────────────────────── */}
          {stats && (
            <div className="card" style={{ marginBottom: 20 }}>
              <div style={{ marginBottom: 16 }}>
                <h2 style={{ fontSize: 15, fontWeight: 600, margin: 0, color: 'var(--color-text-primary)' }}>
                  Current Log Statistics
                </h2>
                <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                  Live snapshot of records currently in the audit log table.
                </p>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 16, marginBottom: 16 }}>
                <div>
                  <p className="text-sm" style={{ color: 'var(--color-text-muted)', margin: '0 0 2px' }}>Total entries</p>
                  <p style={{ fontSize: 22, fontWeight: 700, margin: 0 }}>{fmtNumber(stats.total_entries)}</p>
                </div>
                <div>
                  <p className="text-sm" style={{ color: 'var(--color-text-muted)', margin: '0 0 2px' }}>Oldest entry</p>
                  <p style={{ fontSize: 13, fontWeight: 500, margin: 0 }}>{fmtDate(stats.oldest_entry)}</p>
                </div>
                <div>
                  <p className="text-sm" style={{ color: 'var(--color-text-muted)', margin: '0 0 2px' }}>Most recent</p>
                  <p style={{ fontSize: 13, fontWeight: 500, margin: 0 }}>{fmtDate(stats.newest_entry)}</p>
                </div>
              </div>
              {stats.by_source && stats.by_source.length > 0 && (
                <div>
                  <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 8 }}>By source service</p>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                    {stats.by_source.map(({ source, count }) => (
                      <span key={source} style={{
                        background: 'var(--color-bg-secondary)',
                        border: '1px solid var(--color-border)',
                        borderRadius: 6,
                        padding: '3px 10px',
                        fontSize: 13,
                      }}>
                        <strong>{source}</strong>&nbsp;
                        <span style={{ color: 'var(--color-text-muted)' }}>{fmtNumber(count)}</span>
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* ── Manual purge ──────────────────────────────────────────────────── */}
          <div className="card" style={{ marginBottom: 20 }}>
            <div style={{ marginBottom: 16 }}>
              <h2 style={{ fontSize: 15, fontWeight: 600, margin: 0, color: 'var(--color-text-primary)' }}>
                Manual Purge
              </h2>
              <p className="text-sm" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
                Immediately delete all audit log entries older than the current retention period.
                This uses the saved retention setting — save your changes before triggering a purge.
                {!retentionEnabled && ' Automatic purging is currently disabled — manual purge is also disabled.'}
              </p>
            </div>

            {purgeResult && (
              <div className="alert alert-success" style={{ marginBottom: 12 }}>{purgeResult}</div>
            )}

            {!confirmPurge ? (
              <button
                type="button"
                className="btn btn-danger"
                disabled={purging || !config?.retention_enabled}
                onClick={() => setConfirmPurge(true)}
              >
                Purge old logs now
              </button>
            ) : (
              <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
                <p className="text-sm" style={{ margin: 0, color: 'var(--color-text-primary)', fontWeight: 500 }}>
                  This will permanently delete log entries older than {config?.retention_days ?? retentionDays} days. Confirm?
                </p>
                <button type="button" className="btn btn-danger" onClick={handlePurge} disabled={purging}>
                  {purging ? 'Purging…' : 'Yes, delete old logs'}
                </button>
                <button type="button" className="btn btn-secondary" onClick={() => setConfirmPurge(false)}>
                  Cancel
                </button>
              </div>
            )}

            {config && (
              <p className="text-sm" style={{ marginTop: 12, color: 'var(--color-text-muted)' }}>
                Last updated: {fmtDate(config.updated_at)}
                {config.updated_by ? ` by ${config.updated_by}` : ''}
              </p>
            )}
          </div>
        </>
      )}
    </div>
  );
}
