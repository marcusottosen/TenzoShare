import React, { useEffect, useState, useRef } from 'react';
import { useNavigate } from 'react-router';
import {
  listFiles,
  uploadFile,
  deleteFile,
  presignFile,
  getMyUsage,
  type FileRecord,
  type UploadProgress,
  type StorageUsage,
} from '../api/files';
import { getToken } from '../api/client';
import { listTransfers, type Transfer } from '../api/transfers';
import { pendingFileStore } from '../stores/pendingFileStore';

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins} min${mins !== 1 ? 's' : ''} ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs} hour${hrs !== 1 ? 's' : ''} ago`;
  const days = Math.floor(hrs / 24);
  if (days === 1) return 'Yesterday';
  if (days < 7) return `${days} days ago`;
  return new Date(dateStr).toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

function fileTypeInfo(contentType: string): { icon: string; color: string } {
  if (contentType === 'application/pdf') return { icon: '📄', color: '#3B82F6' };
  if (contentType.startsWith('image/')) return { icon: '🖼', color: '#8B5CF6' };
  if (contentType.startsWith('video/')) return { icon: '🎬', color: '#EC4899' };
  if (contentType.startsWith('audio/')) return { icon: '🎵', color: '#F59E0B' };
  if (contentType.includes('zip') || contentType.includes('compressed')) return { icon: '📦', color: '#F97316' };
  if (contentType.includes('spreadsheet') || contentType.includes('excel') || contentType.includes('csv')) return { icon: '📊', color: '#10B981' };
  if (contentType.includes('word') || contentType.includes('document')) return { icon: '📝', color: '#3B82F6' };
  if (contentType.includes('shell') || contentType.includes('script')) return { icon: '⚡', color: '#F59E0B' };
  return { icon: '📎', color: '#64748B' };
}

function IconUpload() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="16 16 12 12 8 16"/><line x1="12" y1="12" x2="12" y2="21"/>
      <path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/>
    </svg>
  );
}
function IconSearch() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
    </svg>
  );
}
function IconShare() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/>
      <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
    </svg>
  );
}
function IconDownload() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="8 17 12 21 16 17"/><line x1="12" y1="12" x2="12" y2="21"/>
      <path d="M20.88 18.09A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.09"/>
    </svg>
  );
}
function IconTrash() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
    </svg>
  );
}
function IconEye() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>
    </svg>
  );
}

function isPreviewable(contentType: string): boolean {
  return (
    contentType.startsWith('image/') ||
    contentType.startsWith('audio/') ||
    contentType === 'application/pdf' ||
    contentType.startsWith('text/') ||
    contentType === 'application/json'
  );
}

function FilePreviewModal({ file, onClose }: { file: FileRecord; onClose: () => void }) {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [textContent, setTextContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');

  useEffect(() => {
    let objectUrl: string | null = null;
    (async () => {
      try {
        const token = getToken();
        const resp = await fetch(`/api/v1/files/${file.id}/download?inline=1`, {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        });
        if (!resp.ok) throw new Error(`Server returned ${resp.status}`);
        if (file.content_type.startsWith('text/') || file.content_type === 'application/json') {
          setTextContent(await resp.text());
        } else {
          const blob = await resp.blob();
          objectUrl = URL.createObjectURL(blob);
          setPreviewUrl(objectUrl);
        }
      } catch (e: any) {
        setErr(e.message);
      } finally {
        setLoading(false);
      }
    })();
    return () => { if (objectUrl) URL.revokeObjectURL(objectUrl); };
  }, [file.id, file.content_type]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [onClose]);

  const { icon } = fileTypeInfo(file.content_type);

  return (
    <div className="preview-backdrop" onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="preview-modal">
        {/* Header */}
        <div className="preview-modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0, flex: 1 }}>
            <span style={{ fontSize: 18, flexShrink: 0 }}>{icon}</span>
            <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{file.filename}</span>
            <span style={{ fontSize: 12, color: 'var(--color-text-muted)', flexShrink: 0 }}>{fmtBytes(file.size_bytes)}</span>
          </div>
          <button className="preview-close-btn" onClick={onClose} title="Close (Esc)">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        {/* Body */}
        <div className="preview-modal-body">
          {loading && (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '40vh', gap: 12, color: 'var(--color-text-muted)' }}>
              <IconEye />
              <span className="text-sm">Loading preview…</span>
            </div>
          )}
          {err && <div style={{ padding: 24 }}><div className="alert alert-error">{err}</div></div>}
          {!loading && !err && (
            <div className="preview-content">
              {file.content_type.startsWith('image/') && previewUrl && (
                <img src={previewUrl} alt={file.filename} style={{ maxWidth: '100%', maxHeight: '72vh', objectFit: 'contain', borderRadius: 4 }} />
              )}
              {file.content_type.startsWith('audio/') && previewUrl && (
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, width: '100%' }}>
                  <span style={{ fontSize: 48 }}>🎵</span>
                  <audio controls src={previewUrl} style={{ width: '100%', maxWidth: 480 }} />
                </div>
              )}
              {file.content_type === 'application/pdf' && previewUrl && (
                <iframe src={previewUrl} title={file.filename} style={{ width: '100%', height: '72vh', border: 'none', borderRadius: 4 }} />
              )}
              {(file.content_type.startsWith('text/') || file.content_type === 'application/json') && textContent !== null && (
                <pre className="preview-text">{textContent}</pre>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

type FileSortKey = 'name' | 'size' | 'modified';
function SortArrow({ col, sortKey, sortDir }: { col: FileSortKey; sortKey: FileSortKey; sortDir: 'asc' | 'desc' }) {
  if (col !== sortKey) return <span className="sort-arrow" style={{ opacity: 0.3 }}>{'↕'}</span>;
  return <span className="sort-arrow">{sortDir === 'asc' ? '↑' : '↓'}</span>;
}

function IconLock() {
  return (
    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
      <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
    </svg>
  );
}
function IconStorage() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
      <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
    </svg>
  );
}
function IconShares() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/>
      <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
    </svg>
  );
}
function IconDownloads() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
      <polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>
    </svg>
  );
}

export default function FilesPage() {
  const navigate = useNavigate();
  const [files, setFiles] = useState<FileRecord[]>([]);
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [storageUsage, setStorageUsage] = useState<StorageUsage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState<UploadProgress | null>(null);
  const [search, setSearch] = useState('');
  const [previewTarget, setPreviewTarget] = useState<FileRecord | null>(null);
  const [sortKey, setSortKey] = useState<FileSortKey>('modified');
  const [sortDir, setSortDir] = useState<'asc'|'desc'>('desc');
  const fileInputRef = useRef<HTMLInputElement>(null);

  function toggleSort(key: FileSortKey) {
    if (sortKey === key) setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    else { setSortKey(key); setSortDir('asc'); }
  }

  async function load() {
    setLoading(true);
    setError('');
    try {
      const [fr, tr, usage] = await Promise.all([listFiles(), listTransfers(), getMyUsage()]);
      setFiles(fr.files ?? []);
      setTransfers(tr.transfers ?? []);
      setStorageUsage(usage);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    setProgress(null);
    try {
      await uploadFile(file, (p) => setProgress(p));
      await load();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setUploading(false);
      setProgress(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  }

  async function handleDelete(id: string, filename: string) {
    if (!confirm(`Delete "${filename}"? This cannot be undone.`)) return;
    try {
      await deleteFile(id);
      await load();
    } catch (err: any) {
      alert(err.message);
    }
  }

  async function handleDownload(id: string) {
    try {
      const { url } = await presignFile(id);
      window.open(url, '_blank');
    } catch (err: any) {
      alert(err.message);
    }
  }

  const pct = progress ? Math.round((progress.loaded / progress.total) * 100) : 0;
  const usedBytes = storageUsage?.total_bytes ?? 0;
  const quotaEnabled = storageUsage?.quota_enabled ?? false;
  const quotaBytes = storageUsage?.quota_bytes_per_user ?? 0;
  const storagePct = quotaEnabled && quotaBytes > 0 ? Math.min(100, Math.round((usedBytes / quotaBytes) * 100)) : 0;
  const filtered = files.filter((f) => f.filename.toLowerCase().includes(search.toLowerCase()));
  const sorted = [...filtered].sort((a, b) => {
    let cmp = 0;
    if (sortKey === 'name') cmp = a.filename.localeCompare(b.filename);
    else if (sortKey === 'size') cmp = a.size_bytes - b.size_bytes;
    else cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
    return sortDir === 'asc' ? cmp : -cmp;
  });

  // Stats from real data
  const activeShares = transfers.filter((t) => !t.is_revoked && (!t.expires_at || new Date(t.expires_at) > new Date())).length;
  const totalDownloads = transfers.reduce((sum, t) => sum + (t.download_count ?? 0), 0);

  return (
    <div className="page page-wide">
      {/* ── Header ───────────────────────────────────────── */}
      <div className="page-header">
        <div>
          <h1 className="page-title">My Files</h1>
          <p className="page-subtitle">{loading ? '' : `${files.length} file${files.length !== 1 ? 's' : ''} stored`}</p>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <input type="file" ref={fileInputRef} style={{ display: 'none' }} onChange={handleUpload} />
          <button className="btn btn-primary" onClick={() => fileInputRef.current?.click()} disabled={uploading}>
            <IconUpload /> {uploading ? `Uploading ${pct}%…` : 'Upload file'}
          </button>
        </div>
      </div>

      {uploading && progress && (
        <div className="card" style={{ marginBottom: 16, padding: '12px 16px' }}>
          <div className="text-sm" style={{ marginBottom: 6, color: 'var(--color-text-secondary)' }}>
            Uploading… {fmtBytes(progress.loaded)} / {fmtBytes(progress.total)}
          </div>
          <div className="progress-bar-wrap"><div className="progress-bar-fill" style={{ width: `${pct}%` }} /></div>
        </div>
      )}
      {error && <div className="alert alert-error">{error}</div>}

      {/* ── Stats row ─────────────────────────────────────── */}
      <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr', gap: 14, marginBottom: 20 }}>
        {/* Storage card */}
        <div className="card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
            <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
              Storage Capacity
            </div>
            <span style={{ color: 'var(--color-text-muted)' }}><IconStorage /></span>
          </div>
          {storageUsage === null ? (
            <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.03em', marginBottom: 10 }}>—</div>
          ) : quotaEnabled && quotaBytes > 0 ? (
            <>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, marginBottom: 10 }}>
                <span style={{ fontSize: 22, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.02em' }}>
                  {fmtBytes(usedBytes)}
                </span>
                <span style={{ fontSize: 13, color: 'var(--color-text-muted)' }}>/ {fmtBytes(quotaBytes)}</span>
              </div>
              <div style={{ background: 'var(--color-border)', borderRadius: 4, height: 6, marginBottom: 8, overflow: 'hidden' }}>
                <div style={{ width: `${storagePct}%`, height: '100%', background: storagePct >= 90 ? '#EF4444' : storagePct >= 70 ? '#F59E0B' : 'var(--color-secondary)', borderRadius: 4, transition: 'width 0.4s ease' }} />
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span className="enc-badge"><IconLock /> Encrypted Storage</span>
                <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>{storagePct}% utilized</span>
              </div>
            </>
          ) : (
            <>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, marginBottom: 10 }}>
                <span style={{ fontSize: 28, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.03em' }}>
                  {fmtBytes(usedBytes)}
                </span>
                <span style={{ fontSize: 13, color: 'var(--color-text-muted)' }}>unlimited</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span className="enc-badge"><IconLock /> Encrypted Storage</span>
                <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>No quota</span>
              </div>
            </>
          )}
        </div>

        {/* Active Shares */}
        <div className="card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between', cursor: 'pointer' }}
          onClick={() => navigate('/shares')} title="View active shares">
          <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 10 }}>
            Active Shares
          </div>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: 32, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.03em' }}>
              {loading ? '…' : activeShares}
            </span>
            <span style={{ color: 'var(--color-secondary)', opacity: 0.7 }}><IconShares /></span>
          </div>
        </div>

        {/* Recent Downloads */}
        <div className="card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between', cursor: 'pointer' }}
          onClick={() => navigate('/shares')} title="View transfers with downloads">
          <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 10 }}>
            Total Downloads
          </div>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: 32, fontWeight: 700, color: 'var(--color-text-primary)', letterSpacing: '-0.03em' }}>
              {loading ? '…' : totalDownloads}
            </span>
            <span style={{ color: 'var(--color-text-muted)', opacity: 0.7 }}><IconDownloads /></span>
          </div>
        </div>
      </div>

      {/* ── File Directory ────────────────────────────────── */}
      <div className="card" style={{ padding: 0, overflow: 'hidden', marginBottom: 20 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 20px', borderBottom: '1px solid var(--color-border)' }}>
          <span style={{ fontWeight: 600, fontSize: 15, color: 'var(--color-text-primary)' }}>File Directory</span>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, background: 'var(--color-input-bg)', border: '1px solid var(--color-border)', borderRadius: 6, padding: '0 10px', height: 32 }}>
              <IconSearch />
              <input
                type="text"
                placeholder="Search files…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                style={{ border: 'none', background: 'transparent', outline: 'none', fontSize: 13, fontFamily: 'var(--font-sans)', color: 'var(--color-text-primary)', width: 160 }}
              />
            </div>
            <button className="btn btn-dark btn-sm" onClick={() => fileInputRef.current?.click()}>
              + New File
            </button>
          </div>
        </div>

        {loading ? (
          <div className="empty-state" style={{ padding: '32px 0' }}>Loading…</div>
        ) : files.length === 0 ? (
          <div className="empty-state" style={{ padding: '40px 0' }}>
            <div style={{ fontSize: 36, marginBottom: 10 }}>📂</div>
            <strong>No files yet</strong>
            <p style={{ color: 'var(--color-text-muted)', marginTop: 4, fontSize: 14 }}>Upload your first file to get started</p>
            <button className="btn btn-primary" style={{ marginTop: 14 }} onClick={() => fileInputRef.current?.click()}>
              <IconUpload /> Upload file
            </button>
          </div>
        ) : (
          <>
            {/* Table header */}
            <div style={{
              display: 'grid', gridTemplateColumns: '2.5fr 100px 130px 140px 120px',
              padding: '8px 20px', borderBottom: '1px solid var(--color-border)',
              fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)',
              textTransform: 'uppercase', letterSpacing: '0.06em',
            }}>
              <span className="sort-th" onClick={() => toggleSort('name')} style={{ display: 'inline-flex', alignItems: 'center', gap: 3 }}>
                Name <SortArrow col="name" sortKey={sortKey} sortDir={sortDir} />
              </span>
              <span className="sort-th" onClick={() => toggleSort('size')} style={{ display: 'inline-flex', alignItems: 'center', gap: 3 }}>
                Size <SortArrow col="size" sortKey={sortKey} sortDir={sortDir} />
              </span>
              <span>Security</span>
              <span className="sort-th" onClick={() => toggleSort('modified')} style={{ display: 'inline-flex', alignItems: 'center', gap: 3 }}>
                Uploaded <SortArrow col="modified" sortKey={sortKey} sortDir={sortDir} />
              </span>
              <span style={{ textAlign: 'right' }}>Actions</span>
            </div>

            {filtered.length === 0 ? (
              <div className="empty-state" style={{ padding: '24px 0' }}>No files matching "{search}"</div>
            ) : (
              sorted.map((f) => {
                const { icon, color } = fileTypeInfo(f.content_type);
                return (
                  <div key={f.id} className="files-row" onDoubleClick={() => isPreviewable(f.content_type) ? setPreviewTarget(f) : undefined}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
                      <div style={{
                        width: 36, height: 36, borderRadius: 8, flexShrink: 0,
                        background: color + '18',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontSize: 16,
                      }}>
                        {icon}
                      </div>
                      <div style={{ minWidth: 0 }}>
                        <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {f.filename}
                        </div>
                        <div style={{ fontSize: 11, color: 'var(--color-text-muted)', marginTop: 2 }}>{f.content_type}</div>
                      </div>
                    </div>
                    <span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>{fmtBytes(f.size_bytes)}</span>
                    <span className="aes-badge"><IconLock /> AES-256</span>
                    <span style={{ fontSize: 12, color: 'var(--color-text-muted)', whiteSpace: 'nowrap' }} title={new Date(f.created_at).toLocaleString()}>{timeAgo(f.created_at)}</span>
                    <div style={{ display: 'flex', gap: 4, justifyContent: 'flex-end' }}>
                      {isPreviewable(f.content_type) && (
                        <button className="files-icon-btn" title="Preview" onClick={() => setPreviewTarget(f)}>
                          <IconEye />
                        </button>
                      )}
                      <button className="files-icon-btn" title="Share" onClick={() => {
                        pendingFileStore.set([{ id: f.id, filename: f.filename, size_bytes: f.size_bytes }]);
                        navigate('/transfers/new');
                      }}>
                        <IconShare />
                      </button>
                      <button className="files-icon-btn" title="Download" onClick={() => handleDownload(f.id)}>
                        <IconDownload />
                      </button>
                      <button className="files-icon-btn files-icon-btn-danger" title="Delete" onClick={() => handleDelete(f.id, f.filename)}>
                        <IconTrash />
                      </button>
                    </div>
                  </div>
                );
              })
            )}

            {/* Footer */}
            <div style={{ padding: '10px 20px', borderTop: '1px solid var(--color-border)', fontSize: 12, color: 'var(--color-text-muted)' }}>
              Showing {sorted.length} of {files.length} file{files.length !== 1 ? 's' : ''}
            </div>
          </>
        )}
      </div>

      {/* ── File Preview Modal ─────────────────────────────── */}
      {previewTarget && (
        <FilePreviewModal file={previewTarget} onClose={() => setPreviewTarget(null)} />
      )}

      {/* ── Zero-Knowledge Sharing banner ─────────────────── */}
      <div className="zk-banner">
        <div>
          <h3 style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-white)', marginBottom: 6 }}>
            Zero-Knowledge Sharing
          </h3>
          <p style={{ fontSize: 13, color: 'rgba(255,255,255,0.65)', maxWidth: 480, lineHeight: 1.55 }}>
            Experience the security of end-to-end encrypted sharing. Your files are encrypted before they leave your device.
          </p>
        </div>
        <button className="btn" style={{ background: 'var(--color-secondary)', color: 'var(--color-white)', flexShrink: 0 }}>
          Learn More
        </button>
      </div>
    </div>
  );
}
