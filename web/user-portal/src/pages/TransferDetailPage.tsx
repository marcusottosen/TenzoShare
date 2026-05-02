import React, { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router';
import { getTransfer, revokeTransfer, type Transfer } from '../api/transfers';
import { getFile, presignFile, type FileRecord } from '../api/files';
import { getToken } from '../api/client';
import { isPreviewable, IconEye, FilePreviewModal } from '../components/FilePreviewModal';

/**
 * Build the public recipient link for a transfer slug.
 *
 * Resolution order:
 *  1. VITE_DOWNLOAD_UI_URL env var — set this in production to your
 *     download-ui container's public base URL (e.g. https://share.example.com).
 *  2. Dev fallback — same host as the user-portal but on port 3003
 *     (matches the download-ui Vite dev server).
 */
function buildDownloadUrl(slug: string): string {
  const base =
    (import.meta.env.VITE_DOWNLOAD_UI_URL as string | undefined)?.replace(/\/$/, '') ??
    `${window.location.protocol}//${window.location.hostname}:3003`;
  return `${base}/t/${slug}`;
}

function copyToClipboard(text: string, onSuccess: () => void) {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(onSuccess).catch(() => execCopy(text, onSuccess));
  } else {
    execCopy(text, onSuccess);
  }
}
function execCopy(text: string, onSuccess: () => void) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.cssText = 'position:fixed;opacity:0;top:0;left:0;width:1px;height:1px';
  document.body.appendChild(ta);
  ta.focus();
  ta.select();
  try { if (document.execCommand('copy')) onSuccess(); }
  finally { document.body.removeChild(ta); }
}

function fileTypeInfo(ct: string): { icon: string; color: string } {
  if (ct === 'application/pdf')                                           return { icon: '📄', color: '#3B82F6' };
  if (ct.startsWith('image/'))                                            return { icon: '🖼', color: '#8B5CF6' };
  if (ct.startsWith('audio/'))                                            return { icon: '🎵', color: '#F59E0B' };
  if (ct.includes('zip') || ct.includes('compressed'))                    return { icon: '📦', color: '#F97316' };
  if (ct.includes('spreadsheet') || ct.includes('excel') || ct.includes('csv')) return { icon: '📊', color: '#10B981' };
  if (ct.includes('word') || ct.includes('document'))                    return { icon: '📝', color: '#3B82F6' };
  if (ct.startsWith('text/'))                                             return { icon: '📃', color: '#64748B' };
  return { icon: '📎', color: '#64748B' };
}

function FileCard({ fileId }: { fileId: string }) {
  const [file, setFile] = useState<FileRecord | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [thumbUrl, setThumbUrl] = useState<string | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);

  useEffect(() => {
    getFile(fileId).then((f) => {
      setFile(f);
      if (f.content_type.startsWith('image/')) {
        const token = getToken();
        fetch(`/api/v1/files/${f.id}/download?inline=1`, {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        })
          .then((r) => r.blob())
          .then((b) => setThumbUrl(URL.createObjectURL(b)))
          .catch(() => null);
      }
    }).catch((err: unknown) => {
      const msg = err instanceof Error ? err.message : '';
      // 404 = file was deleted; show a clear message instead of "Loading..."
      if (msg.toLowerCase().includes('not found') || msg.includes('404')) {
        setFileError('deleted');
      } else {
        setFileError('unavailable');
      }
    });
    return () => { if (thumbUrl) URL.revokeObjectURL(thumbUrl); };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fileId]);

  async function handleDownload() {
    try {
      const { url } = await presignFile(fileId);
      window.open(url, '_blank');
    } catch (err: any) {
      const msg: string = err?.message ?? 'Download failed.';
      if (msg.toLowerCase().includes('no longer available') || msg.toLowerCase().includes('not found') || msg.toLowerCase().includes('deleted')) {
        alert('This file has been deleted by an administrator and is no longer available for download.');
      } else {
        alert(msg);
      }
    }
  }

  // File was deleted — show a clear placeholder card
  if (fileError) {
    return (
      <div style={{
        border: '1px solid var(--color-border)',
        borderRadius: 10,
        overflow: 'hidden',
        background: 'var(--color-bg)',
        display: 'flex',
        flexDirection: 'column',
        opacity: 0.7,
      }}>
        <div style={{ height: 120, background: '#f3f4f6', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <span style={{ fontSize: 36 }}>🗑</span>
        </div>
        <div style={{ padding: '10px 12px' }}>
          <div style={{ fontSize: 12, color: '#6b7280', fontStyle: 'italic' }}>
            {fileError === 'deleted' ? 'File removed by administrator' : 'File unavailable'}
          </div>
          <div style={{ fontSize: 11, color: '#9ca3af', fontFamily: 'monospace', marginTop: 4 }}>
            {fileId.substring(0, 8)}…
          </div>
        </div>
        <div style={{ padding: '0 12px 12px' }}>
          <button className="btn btn-secondary btn-sm" style={{ width: '100%' }} disabled>
            Unavailable
          </button>
        </div>
      </div>
    );
  }

  const { icon, color } = file ? fileTypeInfo(file.content_type) : { icon: '📎', color: '#64748B' };
  const canPreview = file ? isPreviewable(file.content_type) : false;

  return (
    <>
      {previewOpen && file && (
        <FilePreviewModal file={file} onClose={() => setPreviewOpen(false)} />
      )}
      <div
        style={{
          border: '1px solid var(--color-border)',
          borderRadius: 10,
          overflow: 'hidden',
          background: 'var(--color-bg)',
          display: 'flex',
          flexDirection: 'column',
          transition: 'box-shadow 0.15s',
          cursor: canPreview ? 'pointer' : 'default',
        }}
        onDoubleClick={() => { if (canPreview && file) setPreviewOpen(true); }}
        title={canPreview ? 'Double-click to preview' : undefined}
      >
        {/* Thumbnail / Icon area */}
        <div style={{
          height: 120,
          background: thumbUrl ? 'var(--color-border)' : color + '14',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          overflow: 'hidden',
          flexShrink: 0,
          position: 'relative',
        }}>
          {thumbUrl ? (
            <img src={thumbUrl} alt={file?.filename} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
          ) : (
            <span style={{ fontSize: 40 }}>{file ? icon : '…'}</span>
          )}
          {canPreview && (
            <button
              className="files-icon-btn"
              title="Preview"
              onClick={(e) => { e.stopPropagation(); if (file) setPreviewOpen(true); }}
              style={{ position: 'absolute', top: 6, right: 6, background: 'rgba(255,255,255,0.85)', borderRadius: 6 }}
            >
              <IconEye />
            </button>
          )}
        </div>

        {/* File info */}
        <div style={{ padding: '10px 12px', flex: 1 }}>
          {file ? (
            <>
              <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginBottom: 4 }} title={file.filename}>
                {file.filename}
              </div>
              <div style={{ display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap' }}>
                <span style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>{fmtBytes(file.size_bytes)}</span>
                <span style={{ fontSize: 11, color: 'var(--color-text-muted)', opacity: 0.5 }}>·</span>
                <span style={{ fontSize: 11, color: 'var(--color-text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 120 }}>{file.content_type}</span>
              </div>
            </>
          ) : (
            <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>Loading…</div>
          )}
        </div>

        {/* Download button */}
        <div style={{ padding: '0 12px 12px' }}>
          <button
            className="btn btn-secondary btn-sm"
            style={{ width: '100%' }}
            onClick={handleDownload}
            disabled={!file}
          >
            Download
          </button>
        </div>
      </div>
    </>
  );
}



function fmt(date: string) {
  return new Date(date).toLocaleString();
}

function fmtBytes(b: number): string {
  if (b === 0) return '0 B';
  const k = 1024;
  const u = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(b) / Math.log(k));
  return `${(b / Math.pow(k, i)).toFixed(1)} ${u[i]}`;
}

export default function TransferDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [transfer, setTransfer] = useState<Transfer | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState(false);

  function handleCopy() {
    if (!transfer) return;
    copyToClipboard(buildDownloadUrl(transfer.slug), () => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  useEffect(() => {
    if (!id) return;
    getTransfer(id)
      .then(setTransfer)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  async function handleRevoke() {
    if (!transfer) return;
    if (!confirm('Revoke this transfer? Recipients will no longer be able to download.')) return;
    setRevoking(true);
    try {
      await revokeTransfer(transfer.id);
      setTransfer({ ...transfer, is_revoked: true });
    } catch (err: any) {
      alert(err.message);
    } finally {
      setRevoking(false);
    }
  }

  if (loading) return <div className="empty-state">Loading…</div>;
  if (error) return <div className="alert alert-error">{error}</div>;
  if (!transfer) return null;

  const publicUrl = buildDownloadUrl(transfer.slug);

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <Link to="/" className="text-link text-sm">← Transfers</Link>
          <h1 className="page-title" style={{ marginTop: 4, marginBottom: 0 }}>Transfer details</h1>
        </div>
        {!transfer.is_revoked && (
          <button className="btn btn-danger" onClick={handleRevoke} disabled={revoking}>
            {revoking ? 'Revoking…' : 'Revoke transfer'}
          </button>
        )}
      </div>

      <div className="card">
        <div className="card-header"><h2 className="card-title">Info</h2></div>
        <table style={{ width: 'auto', border: 'none' }}>
          <tbody>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500, width: 160 }}>Status</td>
              <td style={{ paddingLeft: 0 }}>
                {transfer.is_revoked || transfer.status === 'revoked' ? (
                  <span className="badge badge-red">Revoked</span>
                ) : transfer.status === 'exhausted' || (transfer.max_downloads > 0 && transfer.download_count >= transfer.max_downloads) ? (
                  <span className="badge badge-yellow">Exhausted</span>
                ) : transfer.expires_at && new Date(transfer.expires_at) < new Date() ? (
                  <span className="badge badge-gray">Expired</span>
                ) : (
                  <span className="badge badge-green">Active</span>
                )}
              </td>
            </tr>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Name</td>
              <td style={{ paddingLeft: 0 }}>{transfer.name || '—'}</td>
            </tr>
            {transfer.description && (
              <tr>
                <td style={{ paddingLeft: 0, fontWeight: 500 }}>Description</td>
                <td style={{ paddingLeft: 0 }}>{transfer.description}</td>
              </tr>
            )}
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Public link</td>
              <td style={{ paddingLeft: 0 }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <a href={publicUrl} target="_blank" rel="noreferrer" className="text-link">
                    {publicUrl}
                  </a>
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    onClick={handleCopy}
                  >
                    {copied ? 'Copied!' : 'Copy'}
                  </button>
                </span>
              </td>
            </tr>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Recipient</td>
              <td style={{ paddingLeft: 0 }}>{transfer.recipient_email ?? '—'}</td>
            </tr>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Password</td>
              <td style={{ paddingLeft: 0 }}>{transfer.has_password ? 'Yes' : 'No'}</td>
            </tr>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Mode</td>
              <td style={{ paddingLeft: 0 }}>
                {transfer.view_only ? (
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5, fontSize: 12, fontWeight: 600, padding: '2px 8px', borderRadius: 4, background: 'rgba(99,102,241,0.1)', color: 'var(--color-primary)', border: '1px solid rgba(99,102,241,0.25)' }}>
                    👁 View only — recipients cannot download
                  </span>
                ) : (
                  <span style={{ fontSize: 13, color: 'var(--color-text-muted)' }}>Standard (download allowed)</span>
                )}
              </td>
            </tr>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>{transfer.view_only ? 'Views' : 'Downloads'}</td>
              <td style={{ paddingLeft: 0 }}>
                {transfer.download_count}
                {transfer.max_downloads > 0 && ` / ${transfer.max_downloads}`}
              </td>
            </tr>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Total size</td>
              <td style={{ paddingLeft: 0 }}>
                {transfer.total_size_bytes != null
                  ? `${fmtBytes(transfer.total_size_bytes)}${transfer.file_count != null ? ` (${transfer.file_count} file${transfer.file_count !== 1 ? 's' : ''})` : ''}`
                  : '—'}
              </td>
            </tr>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Expires</td>
              <td style={{ paddingLeft: 0 }}>{transfer.expires_at ? fmt(transfer.expires_at) : 'Never'}</td>
            </tr>
            <tr>
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Created</td>
              <td style={{ paddingLeft: 0 }}>{fmt(transfer.created_at)}</td>
            </tr>
          </tbody>
        </table>
      </div>

      {transfer.file_ids && transfer.file_ids.length > 0 && (
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">Files ({transfer.file_ids.length})</h2>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 12 }}>
            {transfer.file_ids.map((fid) => (
              <FileCard key={fid} fileId={fid} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
