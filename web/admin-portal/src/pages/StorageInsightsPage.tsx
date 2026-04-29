import React, { useCallback, useEffect, useState } from 'react';
import {
  AreaChart, Area, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts';
import {
  getStorageInsights, listStorageUsage,
  type StorageInsights, type StorageUserUsage,
} from '../api/admin';

// ── Shared chart theme ────────────────────────────────────────────────────────

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

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtBytes(b: number): string {
  if (!b) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(b) / Math.log(k));
  return `${(b / Math.pow(k, i)).toFixed(i <= 1 ? 0 : 1)} ${sizes[i]}`;
}

function fmtNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function mimeCategory(ct: string): string {
  if (!ct) return 'Other';
  const [major, minor = ''] = ct.split('/');
  if (major === 'image') return 'Images';
  if (major === 'video') return 'Videos';
  if (major === 'audio') return 'Audio';
  if (major === 'text') return 'Documents';
  if (minor === 'pdf') return 'PDFs';
  if (['zip', 'x-zip-compressed', 'x-rar-compressed', 'x-7z-compressed', 'gzip', 'x-tar', 'x-bzip2'].includes(minor)) return 'Archives';
  if (minor.startsWith('vnd.openxmlformats') || minor.startsWith('vnd.ms-') || minor === 'msword' || minor === 'vnd.oasis') return 'Documents';
  if (minor === 'json' || minor === 'xml') return 'Data';
  return 'Other';
}

const MIME_COLORS: Record<string, string> = {
  Images: '#60A5FA',
  Videos: '#F59E0B',
  Audio: '#34D399',
  PDFs: '#F87171',
  Archives: '#A78BFA',
  Documents: '#FB923C',
  Data: '#22D3EE',
  Other: '#94A3B8',
};

const PURGE_REASON_LABELS: Record<string, string> = {
  admin_purge: 'Admin Deletion',
  retention_expired: 'Retention Expired',
  orphan_expired: 'Orphan Expired',
  owner_deleted: 'Owner Deleted',
};

const PURGE_REASON_COLORS: Record<string, string> = {
  admin_purge: '#F87171',
  retention_expired: '#F59E0B',
  orphan_expired: '#94A3B8',
  owner_deleted: '#60A5FA',
};

// ── Sub-components ────────────────────────────────────────────────────────────

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

// ── KPI stat card ─────────────────────────────────────────────────────────────

function StatCard({
  label, value, sub, color = 'teal', icon,
}: {
  label: string;
  value: string;
  sub?: string;
  color?: 'teal' | 'purple' | 'blue' | 'amber' | 'red' | 'green';
  icon: React.ReactNode;
}) {
  return (
    <div className="stat-card">
      <div className={`stat-card-icon ${color}`}>{icon}</div>
      <div className="stat-card-body">
        <div className="stat-label">{label}</div>
        <div className="stat-value">{value}</div>
        {sub && <div className="stat-trend">{sub}</div>}
      </div>
    </div>
  );
}

// ── Storage Growth Area Chart ────────────────────────────────────────────────

function StorageGrowthChart({ data }: { data: { day: string; bytes: number }[] }) {
  if (data.length === 0) return <EmptyChart message="No uploads in the last 30 days" />;
  const chartData = data.map((d) => ({ day: d.day, mb: +(d.bytes / (1024 * 1024)).toFixed(2) }));
  return (
    <ResponsiveContainer width="100%" height={180}>
      <AreaChart data={chartData} margin={{ top: 4, right: 4, left: -10, bottom: 0 }}>
        <defs>
          <linearGradient id="storageGrowthGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="#C084FC" stopOpacity={0.3} />
            <stop offset="95%" stopColor="#C084FC" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_COLOR} vertical={false} />
        <XAxis dataKey="day" tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
        <YAxis tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} unit=" MB" />
        <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v) => [`${v} MB`, 'Uploaded']} />
        <Area type="monotone" dataKey="mb" stroke="#C084FC" strokeWidth={2} fill="url(#storageGrowthGrad)" dot={false} activeDot={{ r: 4, fill: '#C084FC' }} />
      </AreaChart>
    </ResponsiveContainer>
  );
}

// ── Purge Activity Bar Chart ──────────────────────────────────────────────────

function PurgeActivityChart({ data }: { data: { day: string; count: number; freed_bytes: number }[] }) {
  if (data.length === 0) return <EmptyChart message="No purge activity in the last 30 days" />;
  const chartData = data.map((d) => ({ day: d.day, count: d.count, mb: +(d.freed_bytes / (1024 * 1024)).toFixed(2) }));
  return (
    <ResponsiveContainer width="100%" height={180}>
      <BarChart data={chartData} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_COLOR} vertical={false} />
        <XAxis dataKey="day" tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
        <YAxis allowDecimals={false} tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} />
        <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v, name) => [name === 'count' ? v : `${v} MB`, name === 'count' ? 'Files Purged' : 'Freed']} />
        <Bar dataKey="count" fill="#F87171" radius={[3, 3, 0, 0]} maxBarSize={24} />
      </BarChart>
    </ResponsiveContainer>
  );
}

// ── Top Users Horizontal Bar Chart ───────────────────────────────────────────

function TopUsersChart({ users }: { users: StorageUserUsage[] }) {
  if (users.length === 0) return <EmptyChart message="No user storage data" />;
  const top10 = users.slice(0, 10);
  const chartData = top10.map((u) => ({
    email: u.email.length > 24 ? u.email.slice(0, 22) + '…' : u.email,
    mb: +(u.total_bytes / (1024 * 1024)).toFixed(1),
    files: u.file_count,
  })).reverse(); // reverse so largest is at top
  return (
    <ResponsiveContainer width="100%" height={Math.max(160, chartData.length * 28)}>
      <BarChart data={chartData} layout="vertical" margin={{ top: 4, right: 40, left: 4, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_COLOR} horizontal={false} />
        <XAxis type="number" tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} unit=" MB" />
        <YAxis type="category" dataKey="email" tick={{ fontSize: 10, fill: AXIS_COLOR }} axisLine={false} tickLine={false} width={120} />
        <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v, name) => [name === 'mb' ? `${v} MB` : v, name === 'mb' ? 'Storage' : 'Files']} />
        <Bar dataKey="mb" fill="#2DD4BF" radius={[0, 3, 3, 0]} maxBarSize={18} />
      </BarChart>
    </ResponsiveContainer>
  );
}

// ── File Type Donut ───────────────────────────────────────────────────────────

type CategoryStat = { name: string; count: number; size_bytes: number; color: string };

function FileTypeChart({ data }: { data: { content_type: string; count: number; size_bytes: number }[] }) {
  // Aggregate by category
  const agg: Record<string, CategoryStat> = {};
  for (const d of data) {
    const cat = mimeCategory(d.content_type);
    if (!agg[cat]) agg[cat] = { name: cat, count: 0, size_bytes: 0, color: MIME_COLORS[cat] ?? '#94A3B8' };
    agg[cat].count += d.count;
    agg[cat].size_bytes += d.size_bytes;
  }
  const chartData = Object.values(agg).sort((a, b) => b.size_bytes - a.size_bytes);
  const total = chartData.reduce((s, d) => s + d.count, 0);
  if (chartData.length === 0) return <EmptyChart message="No file type data" />;
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
      <ResponsiveContainer width={130} height={130}>
        <PieChart>
          <Pie data={chartData} dataKey="count" cx="50%" cy="50%" innerRadius={38} outerRadius={58} strokeWidth={0} isAnimationActive animationDuration={600}>
            {chartData.map((d, i) => <Cell key={i} fill={d.color} />)}
          </Pie>
          <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v, _n, p) => [`${v} files · ${fmtBytes(p.payload.size_bytes)}`, p.payload.name]} />
        </PieChart>
      </ResponsiveContainer>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, flex: 1 }}>
        {chartData.map((d) => (
          <div key={d.name} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: d.color, flexShrink: 0 }} />
            <span style={{ fontSize: 11, color: 'var(--color-text-secondary)', flex: 1 }}>{d.name}</span>
            <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-primary)' }}>
              {d.count}
              <span style={{ fontWeight: 400, color: 'var(--color-text-muted)', marginLeft: 4 }}>
                ({Math.round((d.count / total) * 100)}%)
              </span>
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Purge Reason Donut ────────────────────────────────────────────────────────

function PurgeReasonChart({ data }: { data: { reason: string; count: number; freed_bytes: number }[] }) {
  const chartData = data.map((d) => ({
    ...d,
    label: PURGE_REASON_LABELS[d.reason] ?? d.reason,
    color: PURGE_REASON_COLORS[d.reason] ?? '#94A3B8',
  }));
  const total = chartData.reduce((s, d) => s + d.count, 0);
  if (chartData.length === 0) return <EmptyChart message="No purge events recorded" />;
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
      <ResponsiveContainer width={130} height={130}>
        <PieChart>
          <Pie data={chartData} dataKey="count" cx="50%" cy="50%" innerRadius={38} outerRadius={58} strokeWidth={0} isAnimationActive animationDuration={600}>
            {chartData.map((d, i) => <Cell key={i} fill={d.color} />)}
          </Pie>
          <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v, _n, p) => [`${v} files · ${fmtBytes(p.payload.freed_bytes)} freed`, p.payload.label]} />
        </PieChart>
      </ResponsiveContainer>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, flex: 1 }}>
        {chartData.map((d) => (
          <div key={d.reason} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: d.color, flexShrink: 0 }} />
            <span style={{ fontSize: 11, color: 'var(--color-text-secondary)', flex: 1 }}>{d.label}</span>
            <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-primary)' }}>
              {d.count}
              <span style={{ fontWeight: 400, color: 'var(--color-text-muted)', marginLeft: 4 }}>
                ({Math.round((d.count / total) * 100)}%)
              </span>
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── File Status Donut ─────────────────────────────────────────────────────────

function FileStatusChart({ active, deleted }: { active: number; deleted: number }) {
  const total = active + deleted;
  if (total === 0) return <EmptyChart message="No file data" />;
  const data = [
    { name: 'Active', value: active, color: '#2DD4BF' },
    { name: 'Deleted', value: deleted, color: '#F87171' },
  ].filter((d) => d.value > 0);
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
      <ResponsiveContainer width={130} height={130}>
        <PieChart>
          <Pie data={data} dataKey="value" cx="50%" cy="50%" innerRadius={38} outerRadius={58} strokeWidth={0} isAnimationActive animationDuration={600}>
            {data.map((d, i) => <Cell key={i} fill={d.color} />)}
          </Pie>
          <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v) => [v, '']} />
        </PieChart>
      </ResponsiveContainer>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {data.map((d) => (
          <div key={d.name} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: d.color, flexShrink: 0 }} />
            <span style={{ fontSize: 12, color: 'var(--color-text-secondary)', minWidth: 44 }}>{d.name}</span>
            <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-primary)' }}>
              {fmtNumber(d.value)}
              <span style={{ fontWeight: 400, color: 'var(--color-text-muted)', marginLeft: 4 }}>
                ({Math.round((d.value / total) * 100)}%)
              </span>
            </span>
          </div>
        ))}
        <div style={{ fontSize: 11, color: 'var(--color-text-muted)', marginTop: 2 }}>
          {fmtNumber(total)} total files
        </div>
      </div>
    </div>
  );
}

// ── Main Page ─────────────────────────────────────────────────────────────────

export default function StorageInsightsPage() {
  const [insights, setInsights] = useState<StorageInsights | null>(null);
  const [users, setUsers] = useState<StorageUserUsage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(() => {
    setLoading(true);
    setError('');
    Promise.all([
      getStorageInsights(),
      listStorageUsage({ limit: 10, sort_by: 'total_bytes', sort_dir: 'desc' }),
    ])
      .then(([ins, usr]) => {
        setInsights(ins);
        setUsers((usr.usage ?? []).filter((u) => u.total_bytes > 0));
      })
      .catch((e: unknown) => setError((e as Error).message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Storage Insights</h1>
          <p className="page-subtitle">Usage analytics, file statistics and purge history</p>
        </div>
        <button className="btn btn-secondary btn-sm" onClick={load} disabled={loading}>
          {loading ? 'Loading…' : '↻ Refresh'}
        </button>
      </div>

      {error && <div className="alert alert-error" style={{ marginBottom: 16 }}>{error}</div>}

      {/* ── KPI Cards ─────────────────────────────────────────────────── */}
      {insights && (
        <div className="stat-grid mb-16">
          <StatCard
            label="Active Files"
            value={fmtNumber(insights.total_files)}
            color="teal"
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
            }
          />
          <StatCard
            label="Storage Used"
            value={fmtBytes(insights.total_storage_bytes)}
            color="purple"
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <ellipse cx="12" cy="5" rx="9" ry="3"/>
                <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
                <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
              </svg>
            }
          />
          <StatCard
            label="Deleted Files"
            value={fmtNumber(insights.deleted_files)}
            sub="Soft-deleted, not purged"
            color="amber"
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="3 6 5 6 21 6"/>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
              </svg>
            }
          />
          <StatCard
            label="Purged Files"
            value={fmtNumber(insights.purged_files)}
            sub="Permanently removed"
            color="red"
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10"/>
                <line x1="15" y1="9" x2="9" y2="15"/>
                <line x1="9" y1="9" x2="15" y2="15"/>
              </svg>
            }
          />
          <StatCard
            label="Storage Freed"
            value={fmtBytes(insights.freed_bytes)}
            sub="From all purge operations"
            color="green"
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="23 6 13.5 15.5 8.5 10.5 1 18"/>
                <polyline points="17 6 23 6 23 12"/>
              </svg>
            }
          />
          <StatCard
            label="Unique Owners"
            value={fmtNumber(insights.unique_owners)}
            sub="Users with active files"
            color="blue"
            icon={
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
                <circle cx="9" cy="7" r="4"/>
                <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
                <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
              </svg>
            }
          />
        </div>
      )}

      {/* ── Row 1: Growth + Top Users ─────────────────────────────────── */}
      {insights && (
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 16 }}>
          <ChartCard title="Storage Growth" subtitle="Data uploaded per day — last 30 days">
            <StorageGrowthChart data={insights.storage_per_day} />
          </ChartCard>
          <ChartCard title="Top Users by Storage" subtitle="Users with the most active storage">
            <TopUsersChart users={users} />
          </ChartCard>
        </div>
      )}

      {/* ── Row 2: File types + File Status + Purge Reasons ──────────── */}
      {insights && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 16 }}>
          <ChartCard title="File Type Breakdown" subtitle="Active files by MIME category">
            <FileTypeChart data={insights.content_type_breakdown} />
          </ChartCard>
          <ChartCard title="File Status" subtitle="Active vs deleted across all time">
            <FileStatusChart active={insights.total_files} deleted={insights.deleted_files} />
          </ChartCard>
          <ChartCard title="Purge Reason Breakdown" subtitle="How files have been removed">
            <PurgeReasonChart data={insights.purge_reason_breakdown} />
          </ChartCard>
        </div>
      )}

      {/* ── Row 3: Purge Activity ──────────────────────────────────────── */}
      {insights && (
        <div style={{ marginBottom: 16 }}>
          <ChartCard title="Purge Activity" subtitle="Files purged per day — last 30 days">
            <PurgeActivityChart data={insights.purge_per_day} />
          </ChartCard>
        </div>
      )}

      {/* ── Loading skeleton ──────────────────────────────────────────── */}
      {loading && !insights && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: 16 }}>
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="card" style={{ height: 96, opacity: 0.4, background: 'var(--color-surface)' }} />
          ))}
        </div>
      )}
    </div>
  );
}
