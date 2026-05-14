import React, { useEffect, useRef, useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router';
import { fmt } from '../utils/dateFormat';
import { getTransfer, revokeTransfer, updateTransferRecipients, resendTransferEmail, type Transfer } from '../api/transfers';
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



function fmtBytes(b: number): string {
  if (b === 0) return '0 B';
  const k = 1024;
  const u = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(b) / Math.log(k));
  return `${(b / Math.pow(k, i)).toFixed(1)} ${u[i]}`;
}

export default function TransferDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [transfer, setTransfer] = useState<Transfer | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState(false);

  // Recipients state
  const [recipients, setRecipients] = useState<string[]>([]);
  const [addingEmail, setAddingEmail] = useState('');
  const [recipientError, setRecipientError] = useState('');
  const [savingRecipients, setSavingRecipients] = useState(false);
  const [resending, setResending] = useState(false);
  const [resendSuccess, setResendSuccess] = useState(false);
  const resendTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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
      .then((t) => {
        setTransfer(t);
        setRecipients(
          t.recipient_emails && t.recipient_emails.length > 0
            ? t.recipient_emails
            : t.recipient_email
              ? t.recipient_email.split(',').map((e) => e.trim()).filter(Boolean)
              : []
        );
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  async function handleRevoke() {
    if (!transfer) return;
    if (!confirm('Revoke this transfer? Recipients will no longer be able to download.')) return;
    setRevoking(true);
    try {
      await revokeTransfer(transfer.id);
      setTransfer({ ...transfer, is_revoked: true, status: 'revoked' });
    } catch (err: any) {
      alert(err.message);
    } finally {
      setRevoking(false);
    }
  }

  function addEmailToList() {
    const email = addingEmail.trim().toLowerCase();
    if (!email) return;
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setRecipientError('Invalid email address');
      return;
    }
    if (recipients.includes(email)) {
      setRecipientError('Already in the list');
      return;
    }
    if (recipients.length >= 20) {
      setRecipientError('Maximum 20 recipients');
      return;
    }
    setRecipientError('');
    setRecipients((prev) => [...prev, email]);
    setAddingEmail('');
  }

  function removeEmail(email: string) {
    setRecipients((prev) => prev.filter((e) => e !== email));
  }

  async function handleSaveRecipients() {
    if (!transfer) return;
    setSavingRecipients(true);
    setRecipientError('');
    try {
      // Detect newly added emails so we can notify them.
      const newlyAdded = recipients.filter((e) => !savedRecipients.includes(e));
      const updated = await updateTransferRecipients(transfer.id, recipients);
      setTransfer(updated);
      // Resend the notification if any new recipients were added.
      if (newlyAdded.length > 0) {
        try { await resendTransferEmail(transfer.id); } catch { /* best-effort */ }
        setResendSuccess(true);
        if (resendTimerRef.current) clearTimeout(resendTimerRef.current);
        resendTimerRef.current = setTimeout(() => setResendSuccess(false), 3000);
      }
    } catch (err: any) {
      setRecipientError(err.message ?? 'Failed to update recipients');
    } finally {
      setSavingRecipients(false);
    }
  }

  async function handleResend() {
    if (!transfer) return;
    setResending(true);
    try {
      await resendTransferEmail(transfer.id);
      setResendSuccess(true);
      if (resendTimerRef.current) clearTimeout(resendTimerRef.current);
      resendTimerRef.current = setTimeout(() => setResendSuccess(false), 3000);
    } catch (err: any) {
      alert(err.message ?? 'Failed to resend notification');
    } finally {
      setResending(false);
    }
  }

  if (loading) return <div className="empty-state">Loading…</div>;
  if (error) return <div className="alert alert-error">{error}</div>;
  if (!transfer) return null;

  const publicUrl = buildDownloadUrl(transfer.slug);
  const isActive = !transfer.is_revoked && transfer.status !== 'revoked'
    && transfer.status !== 'expired'
    && !(transfer.expires_at && new Date(transfer.expires_at) < new Date());

  const savedRecipients = (
    transfer.recipient_emails && transfer.recipient_emails.length > 0
      ? transfer.recipient_emails
      : transfer.recipient_email
        ? transfer.recipient_email.split(',').map((e) => e.trim()).filter(Boolean)
        : []
  );
  const recipientsChanged = JSON.stringify([...recipients].sort()) !== JSON.stringify([...savedRecipients].sort());
  const newlyAdded = recipients.filter((e) => !savedRecipients.includes(e));

  function statusBadge() {
    if (transfer!.is_revoked || transfer!.status === 'revoked')
      return <span className="badge badge-red">Revoked</span>;
    if (transfer!.status === 'exhausted' || (transfer!.max_downloads > 0 && transfer!.download_count >= transfer!.max_downloads))
      return <span className="badge badge-yellow">Exhausted</span>;
    if (transfer!.expires_at && new Date(transfer!.expires_at) < new Date())
      return <span className="badge badge-gray">Expired</span>;
    return <span className="badge badge-green">Active</span>;
  }

  return (
    <div className="page">

      {/* ── Page header ── */}
      <div className="page-header">
        <div>
          <Link to="/" className="text-link text-sm">← All transfers</Link>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 6 }}>
            <h1 className="page-title">{transfer.name || 'Transfer details'}</h1>
            {statusBadge()}
          </div>
          {transfer.description && (
            <p style={{ margin: '4px 0 0', color: 'var(--color-text-muted)', fontSize: 14 }}>{transfer.description}</p>
          )}
        </div>
        {!transfer.is_revoked && isActive && (
          <button className="btn btn-danger" onClick={handleRevoke} disabled={revoking}>
            {revoking ? 'Revoking…' : 'Revoke transfer'}
          </button>
        )}
      </div>

      {/* ── Stat strip ── */}
      <div className="stat-cards" style={{ marginBottom: 16 }}>
        {[
          { label: transfer.view_only ? 'Views' : 'Downloads', value: transfer.max_downloads > 0 ? `${transfer.download_count} / ${transfer.max_downloads}` : String(transfer.download_count) },
          { label: 'Files', value: String(transfer.file_count ?? transfer.file_ids?.length ?? '—') },
          { label: 'Total size', value: transfer.total_size_bytes != null ? fmtBytes(transfer.total_size_bytes) : '—' },
          { label: 'Expires', value: transfer.expires_at ? fmt(transfer.expires_at) : 'Never' },
        ].map(({ label, value }) => (
          <div key={label} className="stat-card" style={{ display: 'block' }}>
            <div className="stat-card-label">{label}</div>
            <div className="stat-card-value" style={{ fontSize: 22 }}>{value}</div>
          </div>
        ))}
      </div>

      {/* ── Details ── */}
      <div className="card">
        <div className="card-header">
          <span className="card-title">Details</span>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2px 32px' }}>
          {([
            ['Mode', transfer.view_only ? '👁 View only (no download)' : 'Standard — download allowed'],
            ['Password', transfer.has_password ? '🔒 Protected' : 'None'],
            ['Created', fmt(transfer.created_at)],
            ['Expires', transfer.expires_at ? fmt(transfer.expires_at) : 'Never'],
          ] as [string, string][]).map(([label, value]) => (
            <div key={label} style={{ display: 'flex', flexDirection: 'column', padding: '10px 0', borderBottom: '1px solid var(--color-border)' }}>
              <span style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.07em', color: 'var(--color-text-muted)', marginBottom: 3 }}>{label}</span>
              <span style={{ fontSize: 14, color: 'var(--color-text-primary)' }}>{value}</span>
            </div>
          ))}
        </div>

        {/* Share link */}
        <div style={{ marginTop: 20 }}>
          <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.07em', color: 'var(--color-text-muted)', marginBottom: 8 }}>Share link</div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', background: 'var(--color-bg)', border: '1px solid var(--color-border)', borderRadius: 8, padding: '10px 14px' }}>
            <a href={publicUrl} target="_blank" rel="noreferrer" className="text-link"
              style={{ fontSize: 13, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {publicUrl}
            </a>
            <button type="button" className="btn btn-secondary btn-sm" onClick={handleCopy} style={{ flexShrink: 0 }}>
              {copied ? '✓ Copied' : 'Copy link'}
            </button>
          </div>
        </div>
      </div>

      {/* ── Recipients ── */}
      <div className="card">
        <div className="card-header">
          <span className="card-title">
            Recipients
            {recipients.length > 0 && (
              <span className="badge badge-gray" style={{ marginLeft: 8, fontWeight: 500, fontSize: 11, verticalAlign: 'middle' }}>
                {recipients.length}
              </span>
            )}
          </span>
        </div>

        {/* Recipient chips */}
        {recipients.length > 0 ? (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 16 }}>
            {recipients.map((email) => (
              <span key={email} style={{
                display: 'inline-flex', alignItems: 'center', gap: 6,
                background: 'var(--color-bg)', border: '1px solid var(--color-border)',
                borderRadius: 20, padding: '5px 8px 5px 14px', fontSize: 13,
              }}>
                {email}
                <button
                  type="button"
                  onClick={() => removeEmail(email)}
                  style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '0 2px', lineHeight: 1, color: 'var(--color-text-muted)', fontSize: 16, display: 'flex', alignItems: 'center', borderRadius: 3 }}
                  title={`Remove ${email}`}
                >
                  ×
                </button>
              </span>
            ))}
          </div>
        ) : (
          <p style={{ margin: '0 0 16px', fontSize: 14, color: 'var(--color-text-muted)' }}>
            No recipients set — anyone with the link can access this transfer.
          </p>
        )}

        {/* Add email row */}
        <div>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              type="email"
              value={addingEmail}
              onChange={(e) => { setAddingEmail(e.target.value); setRecipientError(''); }}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addEmailToList(); } }}
              placeholder="Enter an email address…"
              style={{ flex: 1 }}
            />
            <button type="button" className="btn btn-secondary" onClick={addEmailToList}>
              Add
            </button>
          </div>
          {recipientError && (
            <p style={{ fontSize: 12, color: 'var(--color-error-text)', margin: '6px 0 0' }}>{recipientError}</p>
          )}
        </div>

        {/* Pending changes banner */}
        {recipientsChanged && (
          <div style={{
            marginTop: 16, padding: '14px 16px',
            background: 'var(--color-info-bg)', border: '1px solid var(--color-info-border)',
            borderRadius: 8,
          }}>
            {newlyAdded.length > 0 ? (
              <p style={{ margin: '0 0 10px', fontSize: 13, color: 'var(--color-info-text)' }}>
                <strong>{newlyAdded.length} new recipient{newlyAdded.length > 1 ? 's' : ''}</strong> will receive the file notification email when you apply these changes.
              </p>
            ) : (
              <p style={{ margin: '0 0 10px', fontSize: 13, color: 'var(--color-info-text)' }}>You have unsaved changes to the recipient list.</p>
            )}
            <div style={{ display: 'flex', gap: 8 }}>
              <button type="button" className="btn btn-primary btn-sm" onClick={handleSaveRecipients} disabled={savingRecipients}>
                {savingRecipients ? 'Updating…' : newlyAdded.length > 0 ? 'Update recipients & notify' : 'Update recipients'}
              </button>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => { setRecipients(savedRecipients); setRecipientError(''); }}
                disabled={savingRecipients}
              >
                Discard
              </button>
            </div>
          </div>
        )}

        {/* Resend section — only when no pending changes and there are recipients */}
        {isActive && recipients.length > 0 && !recipientsChanged && (
          <div style={{ marginTop: 20, paddingTop: 16, borderTop: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16 }}>
            <div>
              <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)', marginBottom: 2 }}>Resend notification email</div>
              <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>Re-send the download link to all current recipients.</div>
              {resendSuccess && <div style={{ fontSize: 12, color: 'var(--color-success)', marginTop: 4 }}>✓ Email queued for delivery.</div>}
            </div>
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={handleResend}
              disabled={resending}
              style={{ flexShrink: 0 }}
            >
              {resending ? 'Sending…' : 'Resend email'}
            </button>
          </div>
        )}
      </div>

      {/* ── Files ── */}
      {transfer.file_ids && transfer.file_ids.length > 0 && (
        <div className="card">
          <div className="card-header">
            <span className="card-title">Files ({transfer.file_ids.length})</span>
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
