import React, { useEffect, useState, useRef } from 'react';
import {
  listFiles,
  uploadFile,
  deleteFile,
  presignFile,
  type FileRecord,
  type UploadProgress,
} from '../api/files';

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function fmtDate(date: string) {
  return new Date(date).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

function fileIcon(contentType: string) {
  if (contentType.startsWith('image/')) return '🖼';
  if (contentType === 'application/pdf') return '📄';
  if (contentType.startsWith('video/')) return '🎬';
  if (contentType.startsWith('audio/')) return '🎵';
  if (contentType.includes('zip') || contentType.includes('tar') || contentType.includes('compressed')) return '📦';
  if (contentType.includes('spreadsheet') || contentType.includes('excel') || contentType.includes('csv')) return '📊';
  if (contentType.includes('word') || contentType.includes('document')) return '📝';
  return '📎';
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
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
    </svg>
  );
}

export default function FilesPage() {
  const [files, setFiles] = useState<FileRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState<UploadProgress | null>(null);
  const [search, setSearch] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const res = await listFiles();
      setFiles(res.files ?? []);
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
  const filtered = files.filter((f) => f.filename.toLowerCase().includes(search.toLowerCase()));

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">My Files</h1>
          <p className="page-subtitle">{loading ? '' : `${files.length} file${files.length !== 1 ? 's' : ''} stored`}</p>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <input
            type="file"
            ref={fileInputRef}
            style={{ display: 'none' }}
            onChange={handleUpload}
          />
          <button
            className="btn btn-primary"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
          >
            <IconUpload /> {uploading ? `Uploading ${pct}%…` : 'Upload file'}
          </button>
        </div>
      </div>

      {uploading && progress && (
        <div className="card" style={{ marginBottom: 16, padding: '12px 16px' }}>
          <div className="text-sm" style={{ marginBottom: 6, color: 'var(--color-text-secondary)' }}>
            Uploading… {fmtBytes(progress.loaded)} / {fmtBytes(progress.total)}
          </div>
          <div className="progress-bar-wrap">
            <div className="progress-bar-fill" style={{ width: `${pct}%` }} />
          </div>
        </div>
      )}

      {error && <div className="alert alert-error">{error}</div>}

      {/* Search bar */}
      {files.length > 0 && (
        <div className="files-search">
          <IconSearch />
          <input
            type="text"
            placeholder="Search files…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      )}

      {loading ? (
        <div className="empty-state">Loading…</div>
      ) : files.length === 0 ? (
        <div className="empty-state">
          <div style={{ fontSize: 40, marginBottom: 12 }}>📂</div>
          <strong>No files yet</strong>
          <p style={{ color: 'var(--color-text-muted)', marginTop: 4, fontSize: 14 }}>Upload a file to get started</p>
          <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={() => fileInputRef.current?.click()}>
            <IconUpload /> Upload file
          </button>
        </div>
      ) : filtered.length === 0 ? (
        <div className="empty-state">No files matching "{search}"</div>
      ) : (
        <div className="file-grid">
          {filtered.map((f) => (
            <div key={f.id} className="file-card">
              <div className="file-card-icon">{fileIcon(f.content_type)}</div>
              <div className="file-card-name" title={f.filename}>{f.filename}</div>
              <div className="file-card-meta">
                <span>{fmtBytes(f.size_bytes)}</span>
                <span>{fmtDate(f.created_at)}</span>
              </div>
              <div className="file-card-actions">
                <button className="btn btn-secondary btn-sm" onClick={() => handleDownload(f.id)}>
                  Download
                </button>
                <button className="btn btn-danger btn-sm" onClick={() => handleDelete(f.id, f.filename)}>
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
