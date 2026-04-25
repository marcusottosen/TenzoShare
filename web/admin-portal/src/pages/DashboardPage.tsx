import React, { useEffect, useState } from 'react';
import { getStats, getSystemHealth, type SystemStats, type ServiceHealth } from '../api/admin';

function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

export default function DashboardPage() {
  const [health, setHealth] = useState<ServiceHealth[]>([]);
  const [stats, setStats] = useState<SystemStats | null>(null);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState('');

  async function runChecks() {
    setChecking(true);
    setError('');
    try {
      const [healthRes, statsRes] = await Promise.all([getSystemHealth(), getStats()]);
      setHealth(healthRes.services ?? []);
      setStats(statsRes);
      setLastChecked(new Date());
    } catch (err: any) {
      setError(err.message);
    } finally {
      setChecking(false);
    }
  }

  useEffect(() => { runChecks(); }, []);

  const upCount = health.filter((h) => h.status === 'up').length;
  const totalCount = health.length;

  return (
    <div className="page">
      <div className="row mb-16" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <h1 className="page-title" style={{ margin: 0 }}>Dashboard</h1>
        <div>
          {lastChecked && (
            <span className="text-sm" style={{ marginRight: 12 }}>
              Last checked: {lastChecked.toLocaleTimeString()}
            </span>
          )}
          <button className="btn btn-secondary" onClick={runChecks} disabled={checking}>
            {checking ? 'Checking…' : 'Refresh'}
          </button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {/* Stats row */}
      {stats && (
        <div className="row mb-16" style={{ gap: 12 }}>
          {[
            { label: 'Users', value: stats.total_users },
            { label: 'Active Transfers', value: stats.total_transfers },
            { label: 'Files', value: stats.total_files },
            { label: 'Storage Used', value: formatBytes(stats.total_storage_bytes) },
          ].map(({ label, value }) => (
            <div key={label} className="card" style={{ flex: '1 1 160px', marginBottom: 0 }}>
              <div className="text-sm" style={{ marginBottom: 4 }}>{label}</div>
              <div style={{ fontSize: 28, fontWeight: 700 }}>{value}</div>
            </div>
          ))}
        </div>
      )}

      {/* Health */}
      <div className="card" style={{ marginBottom: 20 }}>
        <div style={{ fontSize: 20, fontWeight: 700, marginBottom: 8 }}>
          {totalCount > 0 ? `${upCount} / ${totalCount}` : '—'}
          <span style={{ fontSize: 14, fontWeight: 400, color: '#888', marginLeft: 8 }}>services up</span>
        </div>
        {upCount < totalCount && totalCount > 0 && (
          <div className="alert alert-warning mt-8">
            {totalCount - upCount} service(s) are down or unreachable.
          </div>
        )}
      </div>

      <div className="health-grid">
        {checking && health.length === 0
          ? Array.from({ length: 7 }).map((_, i) => (
              <div key={i} className="health-card">
                <div className="health-card-name">…</div>
                <span className="badge badge-gray">Checking…</span>
              </div>
            ))
          : health.map((svc) => (
              <div key={svc.name} className="health-card">
                <div className="health-card-name">{svc.name}</div>
                {svc.status === 'up' ? (
                  <span className="badge badge-green">
                    UP {svc.latency_ms > 0 && `(${svc.latency_ms}ms)`}
                  </span>
                ) : (
                  <span className="badge badge-red">DOWN</span>
                )}
              </div>
            ))}
      </div>
    </div>
  );
}

