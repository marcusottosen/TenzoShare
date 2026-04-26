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
      <div className="page-header">
        <div>
          <h1 className="page-title">System Overview</h1>
          <p className="page-subtitle">Infrastructure health and platform statistics</p>
        </div>
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
            <div className="stat-card-icon blue">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/>
                <path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>
              </svg>
            </div>
            <div className="stat-card-body">
              <div className="stat-label">Total Users</div>
              <div className="stat-value">{stats.total_users}</div>
              {stats.new_users_30d > 0 && (
                <div className="stat-trend">+{stats.new_users_30d} this month</div>
              )}
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-card-icon teal">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/>
                <polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/>
              </svg>
            </div>
            <div className="stat-card-body">
              <div className="stat-label">Active Transfers</div>
              <div className="stat-value">{stats.total_transfers}</div>
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-card-icon purple">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
            </div>
            <div className="stat-card-body">
              <div className="stat-label">Files Stored</div>
              <div className="stat-value">{stats.total_files}</div>
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-card-icon amber">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <ellipse cx="12" cy="5" rx="9" ry="3"/>
                <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
                <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
              </svg>
            </div>
            <div className="stat-card-body">
              <div className="stat-label">Storage Used</div>
              <div className="stat-value">{formatBytes(stats.total_storage_bytes)}</div>
            </div>
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

