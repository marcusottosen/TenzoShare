import React, { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router';
import {
  AreaChart, Area, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts';
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

// ── Shared chart theme ───────────────────────────────────────────────────────

const CHART_BG = 'transparent';
const AXIS_COLOR = '#64748B';
const GRID_COLOR = 'rgba(148,163,184,0.1)';

const TOOLTIP_STYLE = {
  background: '#1E293B',
  border: '1px solid rgba(148,163,184,0.2)',
  borderRadius: 6,
  color: '#F1F5F9',
  fontSize: 12,
  padding: '6px 10px',
};

function ChartCard({ title, subtitle, children }: { title: string; subtitle?: string; children: React.ReactNode }) {
  return (
    <div className="card" style={{ padding: '20px 20px 12px' }}>
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-primary)' }}>{title}</div>
        {subtitle && <div style={{ fontSize: 11, color: 'var(--color-text-muted)', marginTop: 2 }}>{subtitle}</div>}
      </div>
      {children}
    </div>
  );
}

function EmptyChart({ message }: { message: string }) {
  return (
    <div style={{ height: 160, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>{message}</span>
    </div>
  );
}

// ── Transfer Status Donut ────────────────────────────────────────────────────

const STATUS_COLORS = { active: '#2DD4BF', exhausted: '#F59E0B', expired: '#94A3B8', revoked: '#F87171' };

function TransferStatusChart({ stats }: { stats: SystemStats }) {
  const { active, exhausted, expired, revoked } = stats.transfer_breakdown ?? { active: 0, exhausted: 0, expired: 0, revoked: 0 };
  const data = [
    { name: 'Active', value: active, color: STATUS_COLORS.active },
    { name: 'Exhausted', value: exhausted, color: STATUS_COLORS.exhausted },
    { name: 'Expired', value: expired, color: STATUS_COLORS.expired },
    { name: 'Revoked', value: revoked, color: STATUS_COLORS.revoked },
  ].filter((d) => d.value > 0);

  const total = active + exhausted + expired + revoked;

  if (total === 0) return <EmptyChart message="No transfer data yet" />;

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
      <ResponsiveContainer width={140} height={140}>
        <PieChart>
          <Pie data={data} cx="50%" cy="50%" innerRadius={42} outerRadius={62} dataKey="value" strokeWidth={0} isAnimationActive={true} animationBegin={0} animationDuration={600} animationEasing="ease-out">
            {data.map((entry, i) => (
              <Cell key={i} fill={entry.color} />
            ))}
          </Pie>
          <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v) => [v, '']} />
        </PieChart>
      </ResponsiveContainer>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {data.map((d) => (
          <div key={d.name} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: d.color, flexShrink: 0 }} />
            <span style={{ fontSize: 12, color: 'var(--color-text-secondary)', minWidth: 56 }}>{d.name}</span>
            <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-primary)' }}>
              {d.value}
              <span style={{ fontWeight: 400, color: 'var(--color-text-muted)', marginLeft: 4 }}>
                ({Math.round((d.value / total) * 100)}%)
              </span>
            </span>
          </div>
        ))}
        <div style={{ marginTop: 4, fontSize: 11, color: 'var(--color-text-muted)' }}>
          {total} total
        </div>
      </div>
    </div>
  );
}

// ── Transfers Per Day Area Chart ─────────────────────────────────────────────

function TransfersAreaChart({ stats }: { stats: SystemStats }) {
  const data = stats.transfers_per_day ?? [];
  if (data.length === 0) return <EmptyChart message="No transfers in the last 14 days" />;
  return (
    <ResponsiveContainer width="100%" height={160}>
      <AreaChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
        <defs>
          <linearGradient id="transferGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="#2DD4BF" stopOpacity={0.25} />
            <stop offset="95%" stopColor="#2DD4BF" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_COLOR} vertical={false} />
        <XAxis dataKey="day" tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} />
        <YAxis allowDecimals={false} tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} />
        <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v) => [v, 'Transfers']} />
        <Area type="monotone" dataKey="count" stroke="#2DD4BF" strokeWidth={2} fill="url(#transferGrad)" dot={false} activeDot={{ r: 4, fill: '#2DD4BF' }} />
      </AreaChart>
    </ResponsiveContainer>
  );
}

// ── New Users Per Day Bar Chart ──────────────────────────────────────────────

function UsersBarChart({ stats }: { stats: SystemStats }) {
  const data = stats.users_per_day ?? [];
  if (data.length === 0) return <EmptyChart message="No new users in the last 14 days" />;
  return (
    <ResponsiveContainer width="100%" height={160}>
      <BarChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_COLOR} vertical={false} />
        <XAxis dataKey="day" tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} />
        <YAxis allowDecimals={false} tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} />
        <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v) => [v, 'New Users']} />
        <Bar dataKey="count" fill="#818CF8" radius={[3, 3, 0, 0]} maxBarSize={28} />
      </BarChart>
    </ResponsiveContainer>
  );
}

// ── Storage Added Per Day Area Chart ────────────────────────────────────────

function StorageAreaChart({ stats }: { stats: SystemStats }) {
  const data = (stats.storage_per_day ?? []).map((d) => ({
    day: d.day,
    mb: +(d.bytes / (1024 * 1024)).toFixed(2),
  }));
  if (data.length === 0) return <EmptyChart message="No file uploads in the last 14 days" />;
  return (
    <ResponsiveContainer width="100%" height={160}>
      <AreaChart data={data} margin={{ top: 4, right: 4, left: -10, bottom: 0 }}>
        <defs>
          <linearGradient id="storageGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="#C084FC" stopOpacity={0.25} />
            <stop offset="95%" stopColor="#C084FC" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_COLOR} vertical={false} />
        <XAxis dataKey="day" tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} />
        <YAxis tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} unit=" MB" />
        <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v) => [`${v} MB`, 'Uploaded']} />
        <Area type="monotone" dataKey="mb" stroke="#C084FC" strokeWidth={2} fill="url(#storageGrad)" dot={false} activeDot={{ r: 4, fill: '#C084FC' }} />
      </AreaChart>
    </ResponsiveContainer>
  );
}

// ── Main Page ────────────────────────────────────────────────────────────────

export default function DashboardPage() {
  const navigate = useNavigate();
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
    } catch (err: unknown) {
      setError((err as Error).message);
    } finally {
      setChecking(false);
    }
  }

  useEffect(() => { runChecks(); }, []);

  useEffect(() => {
    timerRef.current = setInterval(() => {
      setCountdown((c) => {
        if (c <= 1) { runChecks(); return AUTO_REFRESH_SECS; }
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

      {/* Stat cards */}
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
          <div className="stat-card" style={{ cursor: 'pointer' }} onClick={() => navigate('/transfers?status=active')}>
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

      {/* Charts row */}
      {stats && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: 16, marginBottom: 16 }}>
          <ChartCard title="Transfers Created" subtitle="Last 14 days">
            <TransfersAreaChart stats={stats} />
          </ChartCard>
          <ChartCard title="New Registrations" subtitle="Last 14 days">
            <UsersBarChart stats={stats} />
          </ChartCard>
          <ChartCard title="Transfer Status" subtitle="All time breakdown">
            <TransferStatusChart stats={stats} />
          </ChartCard>
          <ChartCard title="Storage Uploaded" subtitle="Last 14 days (MB)">
            <StorageAreaChart stats={stats} />
          </ChartCard>
        </div>
      )}

      {/* Health summary */}
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

