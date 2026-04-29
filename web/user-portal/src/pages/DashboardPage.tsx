import React, { useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { listTransfers, type Transfer } from '../api/transfers';
import { listFileRequests, type FileRequest } from '../api/requests';
import { getMyUsage, type StorageUsage } from '../api/files';
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
function IconHDD() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
      <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
    </svg>
  );
}

// ─── Activity helpers ────────────────────────────────────────────
function transferStatus(t: Transfer): 'active' | 'revoked' | 'exhausted' | 'expired' {
  if (t.is_revoked) return 'revoked';
  if (t.status === 'exhausted' || (t.max_downloads > 0 && t.download_count >= t.max_downloads)) return 'exhausted';
  if (t.expires_at && new Date(t.expires_at) < new Date()) return 'expired';
  return 'active';
}

const STATUS_META: Record<string, { label: string; dotColor: string; iconBg: string; iconColor: string }> = {
  active:   { label: 'Active',    dotColor: '#22C55E', iconBg: '#F0FDF4', iconColor: '#16A34A' },
  revoked:  { label: 'Revoked',   dotColor: '#EF4444', iconBg: '#FEF2F2', iconColor: '#DC2626' },
  exhausted:{ label: 'Exhausted', dotColor: '#F59E0B', iconBg: '#FFFBEB', iconColor: '#D97706' },
  expired:  { label: 'Expired',   dotColor: '#94A3B8', iconBg: '#F8FAFC', iconColor: '#64748B' },
};

function IconArrowRight() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/>
    </svg>
  );
}
function IconCopy() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
    </svg>
  );
}
function IconShare2() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/>
      <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
    </svg>
  );
}
function IconInbox() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/>
      <path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>
    </svg>
  );
}

type ActivityItem = { kind: 'transfer'; data: Transfer } | { kind: 'request'; data: FileRequest };

// ─── Component ──────────────────────────────────────────────────
export default function DashboardPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [requests, setRequests] = useState<FileRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [storageUsage, setStorageUsage] = useState<StorageUsage | null>(null);

  useEffect(() => {
    listTransfers()
      .then((res) => setTransfers(res.transfers ?? []))
      .catch(() => {})
      .finally(() => setLoading(false));
    getMyUsage()
      .then(setStorageUsage)
      .catch(() => {});
    listFileRequests()
      .then((res) => setRequests(res.requests ?? []))
      .catch(() => {});
  }, []);

  const [copiedId, setCopiedId] = useState<string | null>(null);

  const firstName = getFirstName(user?.email);
  const usedBytes = storageUsage?.total_bytes ?? 0;
  const recentActivity: ActivityItem[] = [
    ...transfers.map(t => ({ kind: 'transfer' as const, data: t })),
    ...requests.map(r => ({ kind: 'request' as const, data: r })),
  ].sort((a, b) => new Date(b.data.created_at).getTime() - new Date(a.data.created_at).getTime()).slice(0, 6);

  function handleCopyLink(t: Transfer, e: React.MouseEvent) {
    e.stopPropagation();
    const base = (import.meta.env.VITE_DOWNLOAD_UI_URL as string | undefined)?.replace(/\/$/, '') ??
      `${window.location.protocol}//${window.location.hostname}:3003`;
    const url = `${base}/t/${t.slug}`;
    const copy = (text: string) => {
      if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).catch(() => fallbackCopy(text));
      } else { fallbackCopy(text); }
    };
    const fallbackCopy = (text: string) => {
      const ta = document.createElement('textarea');
      ta.value = text; ta.style.cssText = 'position:fixed;opacity:0';
      document.body.appendChild(ta); ta.focus(); ta.select();
      try { document.execCommand('copy'); } finally { document.body.removeChild(ta); }
    };
    copy(url);
    setCopiedId(t.id);
    setTimeout(() => setCopiedId(null), 2000);
  }

  function handleCopyRequestLink(r: FileRequest, e: React.MouseEvent) {
    e.stopPropagation();
    const base = (import.meta.env.VITE_REQUEST_UI_URL as string | undefined)?.replace(/\/$/, '') ??
      `${window.location.protocol}//${window.location.hostname}:3004`;
    const url = `${base}/r/${r.slug}`;
    const fallbackCopy = (text: string) => {
      const ta = document.createElement('textarea');
      ta.value = text; ta.style.cssText = 'position:fixed;opacity:0';
      document.body.appendChild(ta); ta.focus(); ta.select();
      try { document.execCommand('copy'); } finally { document.body.removeChild(ta); }
    };
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(url).catch(() => fallbackCopy(url));
    } else { fallbackCopy(url); }
    setCopiedId(r.id);
    setTimeout(() => setCopiedId(null), 2000);
  }

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
            {storageUsage === null ? (
              <>
                <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.03em', marginBottom: 10 }}>—</div>
                <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>Loading…</div>
              </>
            ) : storageUsage.quota_enabled && storageUsage.quota_bytes_per_user > 0 ? (
              /* Quota mode: show progress bar */
              <>
                <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.02em', marginBottom: 4 }}>
                  {fmtBytes(usedBytes)}
                  <span style={{ fontSize: 13, fontWeight: 400, color: 'var(--color-text-muted)', marginLeft: 4 }}>
                    of {fmtBytes(storageUsage.quota_bytes_per_user)}
                  </span>
                </div>
                {/* Progress bar */}
                {(() => {
                  const pct = Math.min(100, Math.round((usedBytes / storageUsage.quota_bytes_per_user) * 100));
                  const barColor = pct >= 90 ? '#EF4444' : pct >= 70 ? '#F59E0B' : 'var(--color-primary)';
                  return (
                    <div style={{ marginBottom: 6 }}>
                      <div style={{ height: 6, background: 'var(--color-border)', borderRadius: 99, overflow: 'hidden' }}>
                        <div style={{ height: '100%', width: `${pct}%`, background: barColor, borderRadius: 99, transition: 'width 0.4s ease' }} />
                      </div>
                    </div>
                  );
                })()}
                <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
                  {storageUsage.file_count} file{storageUsage.file_count !== 1 ? 's' : ''} &middot; {Math.min(100, Math.round((usedBytes / storageUsage.quota_bytes_per_user) * 100))}% used
                </div>
              </>
            ) : (
              /* Unmetered mode */
              <>
                <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.03em', marginBottom: 10 }}>
                  {fmtBytes(usedBytes)}
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
                  {storageUsage.file_count} file{storageUsage.file_count !== 1 ? 's' : ''} stored &middot; Unlimited
                </div>
              </>
            )}
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
          <Link to="/shares" className="text-link text-sm" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            View all <IconArrowRight />
          </Link>
        </div>

        {loading ? (
          <div className="empty-state" style={{ padding: '20px 0' }}>Loading…</div>
        ) : recentActivity.length === 0 ? (
          <div style={{ padding: '24px 0', textAlign: 'center' }}>
            <div style={{ fontSize: 32, marginBottom: 8 }}>📭</div>
            <div style={{ fontSize: 13, color: 'var(--color-text-muted)', marginBottom: 12 }}>No recent activity</div>
            <button className="btn btn-primary btn-sm" onClick={() => navigate('/transfers/new')}>+ New Transfer</button>
          </div>
        ) : (
          <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
            {recentActivity.map((item, i) => {
              const isTransfer = item.kind === 'transfer';
              const t = isTransfer ? (item.data as Transfer) : null;
              const r = !isTransfer ? (item.data as FileRequest) : null;
              const statusKey = isTransfer && t ? transferStatus(t) : (r?.is_active && !r?.is_expired) ? 'active' : 'expired';
              const meta = STATUS_META[statusKey];
              const isActive = statusKey === 'active';
              const id = isTransfer ? t!.id : r!.id;
              const isCopied = copiedId === id;
              const countDisplay = isTransfer && t && t.download_count > 0
                ? `↓ ${t.download_count}`
                : !isTransfer && (r?.submission_count ?? 0) > 0
                ? `↑ ${r!.submission_count}`
                : '';
              const createdAt = isTransfer ? t!.created_at : r!.created_at;
              return (
                <li
                  key={id}
                  className="act-row"
                  onClick={() => isTransfer && t ? navigate(`/transfers/${t.id}`) : navigate('/shares?tab=requests')}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 12, padding: '11px 8px',
                    borderBottom: i < recentActivity.length - 1 ? '1px solid var(--color-border)' : 'none',
                    borderRadius: 8,
                    cursor: 'pointer',
                    transition: 'background 0.12s',
                    margin: '0 -8px',
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--color-nav-active)')}
                  onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                >
                  {/* Status dot + icon */}
                  <div style={{
                    width: 36, height: 36, borderRadius: 8, flexShrink: 0,
                    background: meta.iconBg,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    color: meta.iconColor,
                    position: 'relative',
                  }}>
                    {isTransfer ? <IconShare2 /> : <IconInbox />}
                    <span style={{
                      position: 'absolute', top: -2, right: -2,
                      width: 9, height: 9, borderRadius: '50%',
                      background: meta.dotColor,
                      border: '2px solid var(--color-surface)',
                    }} />
                  </div>

                  {/* Name + meta */}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'flex', alignItems: 'center', gap: 6 }}>
                      {isTransfer ? (t!.name || t!.slug) : r!.name}
                      {!isTransfer && (r!.submission_count ?? 0) > 0 && (
                        <span style={{
                          fontSize: 10, fontWeight: 700, letterSpacing: '0.04em',
                          padding: '1px 6px', borderRadius: 99,
                          background: '#ECFDF5', color: '#059669',
                          border: '1px solid #6EE7B7',
                          whiteSpace: 'nowrap', flexShrink: 0,
                        }}>
                          {r!.submission_count} FILE{r!.submission_count !== 1 ? 'S' : ''} RECEIVED
                        </span>
                      )}
                    </div>
                    <div style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 1, display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap' }}>
                      {isTransfer ? (
                        <>
                          {t!.recipient_email ? <span>→ {t!.recipient_email}</span> : <span>Public link</span>}
                          {t!.file_count != null && <><span style={{ opacity: 0.4 }}>·</span><span>{t!.file_count} file{t!.file_count !== 1 ? 's' : ''}</span></>}
                          {t!.total_size_bytes != null && <><span style={{ opacity: 0.4 }}>·</span><span>{fmtBytes(t!.total_size_bytes)}</span></>}
                        </>
                      ) : (
                        <>
                          <span>File request</span>
                          {(r!.submission_count ?? 0) === 0 && <><span style={{ opacity: 0.4 }}>·</span><span>No submissions yet</span></>}
                        </>
                      )}
                    </div>
                  </div>

                  {/* Right: fixed columns — count | time | badge | copy */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
                    <div style={{ width: 40, textAlign: 'right', fontSize: 11, color: 'var(--color-text-muted)', fontVariantNumeric: 'tabular-nums' }}>
                      {countDisplay}
                    </div>
                    <div style={{ width: 72, textAlign: 'right', fontSize: 11, color: 'var(--color-text-muted)', whiteSpace: 'nowrap' }}>
                      {timeAgo(createdAt)}
                    </div>
                    <div style={{ width: 76, display: 'flex', justifyContent: 'flex-end' }}>
                      <span style={{
                        fontSize: 10, fontWeight: 600, letterSpacing: '0.05em', textTransform: 'uppercase',
                        padding: '2px 7px', borderRadius: 99,
                        background: meta.iconBg, color: meta.iconColor,
                        border: `1px solid ${meta.dotColor}30`,
                        whiteSpace: 'nowrap',
                      }}>
                        {meta.label}
                      </span>
                    </div>
                    <div style={{ width: 30, display: 'flex', justifyContent: 'center' }}>
                      {isActive && (
                        <button
                          className="act-copy-btn"
                          title={isCopied ? 'Copied!' : 'Copy link'}
                          onClick={(e) => {
                            if (isTransfer && t) handleCopyLink(t, e);
                            else if (r) handleCopyRequestLink(r, e);
                          }}
                        >
                          {isCopied ? '✓' : <IconCopy />}
                        </button>
                      )}
                    </div>
                  </div>
                </li>
              );
            })}
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
