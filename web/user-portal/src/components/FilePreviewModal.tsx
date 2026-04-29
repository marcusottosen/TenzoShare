import React, { useEffect, useState } from 'react';
import { getToken } from '../api/client';
import type { FileRecord } from '../api/files';

export function isPreviewable(contentType: string): boolean {
  return (
    contentType.startsWith('image/') ||
    contentType.startsWith('audio/') ||
    contentType === 'application/pdf' ||
    contentType.startsWith('text/') ||
    contentType === 'application/json'
  );
}

export function IconEye() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>
    </svg>
  );
}

function fileTypeIcon(contentType: string): string {
  if (contentType === 'application/pdf') return '📄';
  if (contentType.startsWith('image/')) return '🖼';
  if (contentType.startsWith('audio/')) return '🎵';
  if (contentType.startsWith('text/') || contentType === 'application/json') return '📃';
  return '📎';
}

function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export function FilePreviewModal({ file, onClose }: { file: FileRecord; onClose: () => void }) {
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

  const icon = fileTypeIcon(file.content_type);

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
