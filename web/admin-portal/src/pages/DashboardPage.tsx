import React, { useEffect, useRef, useState } from 'react';
import { getStats, getSystemHealth, type SystemStats, type ServiceHealth } from '../api/admin';

const AUTO_REFRESH_SECS = 30;

function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

function latencyClass(ms: number): string {
  if (ms < 100) return 'badge-green';
  if (ms < 500) return 'badge-orange';
  return 'badge-red';
}

export default function DashboardPage() {
  const [health, setHealth] = useState<ServiceHealth[]>([]);
  const [stats, setStats] = useState<SystemStats | null>(null);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState('');
  const [countdown, setCountdown] = useState(AUTO_REFRESH_SECS);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  async function runChecks() {
    setChecking(true);
    setError('');
    setCountdown(AUTO_REFRESH_SECS);
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

  // Auto-refresh every AUTO_REFRESH_SECS seconds with countdown
  useEffect(() => {
    runChecks();
  }, []);

  useEffect(() => {
    timerRef.current = setInterval(() => {
      setCountdown((c) => {
        if (c <= 1) {
          runChecks();
          return AUTO_REFRESH_SECS;
        }
        return c - 1;
      });
    }, 1000);
    return () => { if (timerRef.current) clearInterval(timerRef.current); };
  }, []);

  const upCount = health.filter((h) => h.status === 'up').length;
  const totalCount = health.length;
  const allUp = totalCount > 0 && upCount === totalCount;

  return (
    <div className="page">
      <div className="row mb-16" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <h1 className="page-title" style={{ margin: 0 }}>Dashboard</h1>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          {lastChecked && (
            <span className="text-sm">
              Updated {lastChecked.toLocaleTimeString()} · refreshing in {countdown}s
            </span>
          )}
          <button className="btn btn-secondary btn-sm" onClick={runChecks} disabled={checking}>
            {checking ? 'Checking…' : '↻ Refresh'}
          </button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {/* Stats row */}
      {stats && (
        <div className="stat-grid mb-16">
          <div className="stat-card">
            <div className="stat-label">Total Users</div>
            <div className="stat-value">{stats.total_users}</div>
            {stats.new_users_30d > 0 && (
              <div className="stat-trend">+{stats.new_users_30d} this month</div>
            )}
          </div>
          <div className="stat-card">
            <div className="stat-label">Active Transfers</div>
            <div className="stat-value">{stats.total_transfers}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Files Stored</div>
            <div className="stat-value">{stats.total_files}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Storage Used</div>
            <div className="stat-value">{formatBytes(stats.total_storage_bytes)}</div>
          </div>
        </div>
      )}

      {/* Health summary banner */}
      <div className={`health-summary mb-16 ${allUp ? 'health-summary-ok' : 'health-summary-warn'}`}>
        <span className="health-summary-dot">{allUp ? '●' : '●'}</span>
        <span>
          {totalCount === 0
            ? 'Checking services…'
            : allUp
            ? `All ${totalCount} services operational`
            : `${upCount} / ${totalCount} services up — ${totalCount - upCount} issue(s)`}
        </span>
      </div>

      {/* Service health cards */}
      <div className="health-grid">
        {checking && health.length === 0
          ? Array.from({ length: 7 }).map((_, i) => (
              <div key={i} className="health-card">
                <div className="health-card-name">…</div>
                <span className="badge badge-gray">Checking…</span>
              </div>
            ))
          : health.map((svc) => (
              <div key={svc.name} className={`health-card ${svc.status === 'down' ? 'health-card-down' : ''}`}>
                <div className="health-card-name">{svc.name}</div>
                {svc.status === 'up' ? (
                  <div style={{ display: 'flex', gap: 4, alignItems: 'center', flexWrap: 'wrap' }}>
                    <span className="badge badge-green">UP</span>
                    {svc.latency_ms > 0 && (
                      <span className={`badge ${latencyClass(svc.latency_ms)}`} style={{ fontSize: 10 }}>
                        {svc.latency_ms}ms
                      </span>
                    )}
                  </div>
                ) : (
                  <span className="badge badge-red">DOWN</span>
                )}
              </div>
            ))}
      </div>
    </div>
  );
}

