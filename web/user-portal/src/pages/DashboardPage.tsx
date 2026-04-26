import React, { useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { listTransfers, revokeTransfer, type Transfer } from '../api/transfers';
import { useAuth } from '../stores/auth';
import { pendingUploadStore } from '../stores/pendingUpload';

// ─── Helpers ────────────────────────────────────────────────────
function getFirstName(email?: string): string {
  if (!email) return 'there';
  const local = email.split('@')[0];
  const parts = local.split(/[._-]/);
  const name = parts[0];
  return name.charAt(0).toUpperCase() + name.slice(1);
}

function getGreeting(): string {
  const h = new Date().getHours();
  if (h < 12) return 'Good morning';
  if (h < 17) return 'Good afternoon';
  return 'Good evening';
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins} min${mins > 1 ? 's' : ''} ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs} hour${hrs > 1 ? 's' : ''} ago`;
  const days = Math.floor(hrs / 24);
  if (days === 1) return 'Yesterday';
  return `${days} days ago`;
}

function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

// ─── SVG Icons ──────────────────────────────────────────────────
function IconCloudUpload() {
  return (
    <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="16 16 12 12 8 16"/><line x1="12" y1="12" x2="12" y2="21"/>
      <path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/>
    </svg>
  );
}
function IconShield() {
  return (
    <svg width="48" height="48" viewBox="0 0 24 24" fill="currentColor" stroke="none" opacity="0.15">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
    </svg>
  );
}
function IconLock() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
      <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
    </svg>
  );
}
function IconKey() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/>
    </svg>
  );
}
function IconCheck() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12"/>
    </svg>
  );
}
function IconDoc() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
      <polyline points="14 2 14 8 20 8"/>
    </svg>
  );
}
function IconDots() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="5" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="12" cy="19" r="1"/>
    </svg>
  );
}
function IconHDD() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
      <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
    </svg>
  );
}

// ─── Mock data ───────────────────────────────────────────────────
const MOCK_STORAGE = { usedGB: 42.8, totalGB: 100 };

// ─── Activity badge ──────────────────────────────────────────────
function ActivityBadge({ type }: { type: string }) {
  if (type === 'secure') return <span className="act-badge act-badge-secure">SECURE</span>;
  if (type === 'shared') return <span className="act-badge act-badge-shared">SHARED</span>;
  return <span className="act-badge act-badge-modified">MODIFIED</span>;
}

// ─── Build activity items from real transfers ────────────────────
function buildActivity(transfers: Transfer[]) {
  const items = transfers.slice(0, 5).map((t) => ({
    id: t.id,
    name: t.name || t.slug,
    sub: t.recipient_email ? `Shared with ${t.recipient_email}` : 'Created by you',
    time: timeAgo(t.created_at),
    type: t.is_revoked ? 'modified' : 'shared',
  }));
  // Pad with mock if needed
  if (items.length === 0) {
    return [
      { id: 'm1', name: 'Q4_Performance_Review.pdf', sub: 'Uploaded by you • 2.4 MB', time: '12 mins ago', type: 'secure' },
      { id: 'm2', name: 'Project_Alpha_Assets.zip', sub: 'Shared with Engineering Team', time: '2 hours ago', type: 'shared' },
      { id: 'm3', name: 'Meeting_Minutes_v2.docx', sub: 'Renamed from Meeting_Minutes.docx', time: 'Yesterday', type: 'modified' },
    ];
  }
  return items;
}

// ─── Component ──────────────────────────────────────────────────
export default function DashboardPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    listTransfers()
      .then((res) => setTransfers(res.transfers ?? []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const firstName = getFirstName(user?.email);
  const storagePct = Math.round((MOCK_STORAGE.usedGB / MOCK_STORAGE.totalGB) * 100);
  const activity = buildActivity(transfers);

  function handleFilesPicked(e: React.ChangeEvent<HTMLInputElement>) {
    const picked = Array.from(e.target.files ?? []);
    if (picked.length === 0) return;
    pendingUploadStore.set(picked);
    navigate('/transfers/new');
  }

  return (
    <div className="page page-wide">
      <input type="file" multiple ref={fileInputRef} style={{ display: 'none' }} onChange={handleFilesPicked} />

      {/* ── Greeting ─────────────────────────────────────────── */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.02em', marginBottom: 4 }}>
          {getGreeting()}, {firstName}
        </h1>
        <p style={{ fontSize: 14, color: 'var(--color-text-muted)' }}>
          Securely manage and share your enterprise assets.
        </p>
      </div>

      {/* ── Row 1: Upload zone + Widgets ─────────────────────── */}
      <div className="dash-row" style={{ marginBottom: 20 }}>

        {/* Upload zone */}
        <div className="upload-zone-card">
          <div className="upload-zone-icon">
            <IconCloudUpload />
          </div>
          <h2 style={{ fontSize: 17, fontWeight: 600, color: 'var(--color-text-primary)', margin: '12px 0 6px' }}>
            Secure Upload
          </h2>
          <p style={{ fontSize: 13, color: 'var(--color-text-muted)', maxWidth: 340, textAlign: 'center', lineHeight: 1.5 }}>
            Drag and drop your files here or click to browse. Files are automatically encrypted before storage.
          </p>
          <div style={{ display: 'flex', gap: 10, marginTop: 20, flexWrap: 'wrap', justifyContent: 'center' }}>
            <button className="btn btn-dark btn-lg" onClick={() => fileInputRef.current?.click()}>
              + Select Files
            </button>
            <button className="btn btn-secondary-outline btn-lg" onClick={() => navigate('/files')}>
              Browse Files
            </button>
          </div>
          <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
            <span className="enc-badge"><IconLock /> AES-256 ENCRYPTED</span>
            <span className="enc-badge"><IconCheck /> AUDITED PATH</span>
          </div>
        </div>

        {/* Widgets column */}
        <div className="dash-widgets">
          {/* Storage Used */}
          <div className="card" style={{ marginBottom: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
              <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                Storage Used
              </div>
              <span style={{ color: 'var(--color-text-muted)' }}><IconHDD /></span>
            </div>
            <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.03em', marginBottom: 10 }}>
              {MOCK_STORAGE.usedGB} GB
            </div>
            <div style={{ background: 'var(--color-border)', borderRadius: 4, height: 6, marginBottom: 8, overflow: 'hidden' }}>
              <div style={{ width: `${storagePct}%`, height: '100%', background: 'var(--color-secondary)', borderRadius: 4, transition: 'width 0.6s ease' }} />
            </div>
            <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
              {storagePct}% of {MOCK_STORAGE.totalGB}GB total storage used
            </div>
          </div>

          {/* Security Score */}
          <div className="card widget-dark">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
              <div>
                <div style={{ fontSize: 15, fontWeight: 600, color: '#fff', marginBottom: 4 }}>Security Score</div>
                <div style={{ fontSize: 12, color: 'rgba(255,255,255,0.55)', marginBottom: 14 }}>Your account is fully protected.</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontSize: 28, fontWeight: 700, color: '#fff', letterSpacing: '-0.03em' }}>100%</span>
                  <span style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: 22, height: 22, background: '#22C55E', borderRadius: '50%', color: '#fff', flexShrink: 0 }}>
                    <IconCheck />
                  </span>
                </div>
              </div>
              <div style={{ color: '#fff', opacity: 0.15, marginTop: -8, marginRight: -8, flexShrink: 0 }}>
                <IconShield />
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* ── Recent Activity ───────────────────────────────────── */}
      <div className="card" style={{ marginBottom: 16 }}>
        <div className="card-header">
          <h2 className="card-title">Recent Activity</h2>
          <Link to="/" className="text-link text-sm">View All Logs</Link>
        </div>

        {loading ? (
          <div className="empty-state" style={{ padding: '20px 0' }}>Loading…</div>
        ) : (
          <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
            {activity.map((a, i) => (
              <li key={a.id} style={{
                display: 'flex', alignItems: 'center', gap: 12, padding: '12px 0',
                borderBottom: i < activity.length - 1 ? '1px solid var(--color-border)' : 'none',
              }}>
                <div style={{
                  width: 36, height: 36, borderRadius: 8, flexShrink: 0,
                  background: 'var(--color-nav-active)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  color: 'var(--color-text-muted)',
                }}>
                  <IconDoc />
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {a.name}
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 1 }}>{a.sub}</div>
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-muted)', whiteSpace: 'nowrap', flexShrink: 0, marginRight: 8 }}>{a.time}</div>
                <ActivityBadge type={a.type} />
                <button style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-muted)', padding: '4px', borderRadius: 4, flexShrink: 0 }}>
                  <IconDots />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* ── Bottom row: Tips + Folders ────────────────────────── */}
      <div className="dash-row" style={{ alignItems: 'flex-start' }}>

        {/* Sharing Tips */}
        <div style={{ flex: '0 0 280px', padding: '4px 0' }}>
          <h2 className="card-title">Sharing Tips</h2>
          <div className="tip-item">
            <div className="tip-icon"><IconLock /></div>
            <div>
              <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)', marginBottom: 2 }}>Self-Destructing Links</div>
              <div style={{ fontSize: 12, color: 'var(--color-text-muted)', lineHeight: 1.4 }}>Set links to expire after a single view or specific time.</div>
            </div>
          </div>
          <div className="tip-item" style={{ marginTop: 12 }}>
            <div className="tip-icon"><IconKey /></div>
            <div>
              <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)', marginBottom: 2 }}>Password Protection</div>
              <div style={{ fontSize: 12, color: 'var(--color-text-muted)', lineHeight: 1.4 }}>Add an extra layer of security to public share links.</div>
            </div>
          </div>
        </div>

      </div>

    </div>
  );
}
