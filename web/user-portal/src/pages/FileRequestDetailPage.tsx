import React, { useEffect, useRef, useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router';
import { fmt } from '../utils/dateFormat';
import {
  getFileRequest,
  deactivateFileRequest,
  updateRequestRecipients,
  resendRequestInvite,
  type FileRequest,
  type Submission,
} from '../api/requests';
import { presignFile } from '../api/files';

// ── Helpers ───────────────────────────────────────────────────────────────────

function buildRequestUrl(slug: string) {
  const base = (import.meta.env.VITE_REQUEST_UI_URL as string | undefined)?.replace(/\/$/, '')
    ?? `${window.location.protocol}//${window.location.hostname}:3002`;
  return `${base}/r/${slug}`;
}

function fmtBytes(b: number) {
  if (b === 0) return '0 B';
  const k = 1024;
  const u = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(b) / Math.log(k));
  return `${(b / Math.pow(k, i)).toFixed(1)} ${u[i]}`;
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
  ta.focus(); ta.select();
  try { if (document.execCommand('copy')) onSuccess(); } finally { document.body.removeChild(ta); }
}

// ── Icon components ───────────────────────────────────────────────────────────

function IconDownload() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="7 10 12 15 17 10" />
      <line x1="12" y1="15" x2="12" y2="3" />
    </svg>
  );
}

function IconFile() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <polyline points="14 2 14 8 20 8" />
    </svg>
  );
}

// ── Submission row ────────────────────────────────────────────────────────────

function SubmissionRow({ sub }: { sub: Submission }) {
  const [downloading, setDownloading] = useState(false);

  async function handleDownload() {
    if (downloading) return;
    setDownloading(true);
    try {
      const { url } = await presignFile(sub.file_id);
      const a = document.createElement('a');
      a.href = url;
      a.download = sub.filename;
      a.rel = 'noopener';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    } catch (err: any) {
      alert(err.message ?? 'Download failed');
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 12,
      padding: '10px 0', borderBottom: '1px solid var(--color-border)',
    }}>
      <div style={{ color: 'var(--color-text-muted)', flexShrink: 0 }}><IconFile /></div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {sub.filename}
        </div>
        <div style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 2 }}>
          {fmtBytes(sub.size_bytes)}
          {sub.submitter_name && ` · ${sub.submitter_name}`}
          {` · ${fmt(sub.submitted_at)}`}
        </div>
        {sub.message && (
          <div style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 3, fontStyle: 'italic' }}>
            "{sub.message}"
          </div>
        )}
      </div>
      <button
        type="button"
        className="btn btn-secondary btn-sm"
        onClick={handleDownload}
        disabled={downloading}
        style={{ flexShrink: 0, display: 'flex', alignItems: 'center', gap: 5 }}
      >
        <IconDownload />
        {downloading ? 'Downloading…' : 'Download'}
      </button>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function FileRequestDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [request, setRequest] = useState<FileRequest | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deactivating, setDeactivating] = useState(false);
  const [copied, setCopied] = useState(false);
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Recipients management state
  const [recipients, setRecipients] = useState<string[]>([]);
  const [addingEmail, setAddingEmail] = useState('');
  const [recipientError, setRecipientError] = useState('');
  const [savingRecipients, setSavingRecipients] = useState(false);
  const [resending, setResending] = useState(false);
  const [resendSuccess, setResendSuccess] = useState(false);
  const resendTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    getFileRequest(id)
      .then((r) => {
        setRequest(r);
        setRecipients(
          r.recipient_emails && r.recipient_emails.length > 0 ? r.recipient_emails : []
        );
      })
      .catch((err: any) => setError(err.message ?? 'Failed to load request'))
      .finally(() => setLoading(false));

    return () => {
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
      if (resendTimerRef.current) clearTimeout(resendTimerRef.current);
    };
  }, [id]);

  async function handleDeactivate() {
    if (!request) return;
    if (!confirm('Close this request? Guests will no longer be able to upload.')) return;
    setDeactivating(true);
    try {
      await deactivateFileRequest(request.id);
      setRequest({ ...request, is_active: false });
    } catch (err: any) {
      alert(err.message ?? 'Failed to close request');
    } finally {
      setDeactivating(false);
    }
  }

  function handleCopy() {
    if (!request) return;
    copyToClipboard(buildRequestUrl(request.slug), () => {
      setCopied(true);
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
      copyTimerRef.current = setTimeout(() => setCopied(false), 2000);
    });
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
    if (!request) return;
    setSavingRecipients(true);
    setRecipientError('');
    try {
      const savedEmails = request.recipient_emails ?? [];
      const newlyAdded = recipients.filter((e) => !savedEmails.includes(e));
      const updated = await updateRequestRecipients(request.id, recipients);
      setRequest(updated);
      setRecipients(updated.recipient_emails ?? []);
      if (newlyAdded.length > 0) {
        try { await resendRequestInvite(request.id); } catch { /* best-effort */ }
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
    if (!request) return;
    setResending(true);
    try {
      await resendRequestInvite(request.id);
      setResendSuccess(true);
      if (resendTimerRef.current) clearTimeout(resendTimerRef.current);
      resendTimerRef.current = setTimeout(() => setResendSuccess(false), 3000);
    } catch (err: any) {
      alert(err.message ?? 'Failed to resend invite');
    } finally {
      setResending(false);
    }
  }

  if (loading) return <div className="empty-state">Loading…</div>;
  if (error) return <div className="alert alert-error">{error}</div>;
  if (!request) return null;

  const isExpired = request.is_expired || (!request.is_active && !request.is_expired)
    ? false : !request.is_active ? false : new Date(request.expires_at) < new Date();
  const isClosed = !request.is_active;
  const isActive = request.is_active && !isExpired;

  const savedRecipients = request.recipient_emails ?? [];
  const recipientsChanged = JSON.stringify([...recipients].sort()) !== JSON.stringify([...savedRecipients].sort());
  const newlyAdded = recipients.filter((e) => !savedRecipients.includes(e));

  function statusBadge() {
    if (isClosed && !isExpired) return <span className="badge badge-red">Closed</span>;
    if (request!.is_expired || new Date(request!.expires_at) < new Date())
      return <span className="badge badge-gray">Expired</span>;
    return <span className="badge badge-green">Active</span>;
  }

  const publicUrl = buildRequestUrl(request.slug);
  const submissions = request.submissions ?? [];

  return (
    <div className="page">

      {/* ── Page header ── */}
      <div className="page-header">
        <div>
          <Link to="/shares?tab=requests" className="text-link text-sm">← File Requests</Link>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 6 }}>
            <h1 className="page-title">{request.name}</h1>
            {statusBadge()}
          </div>
          {request.description && (
            <p style={{ margin: '4px 0 0', color: 'var(--color-text-muted)', fontSize: 14 }}>
              {request.description}
            </p>
          )}
        </div>
        {isActive && (
          <button className="btn btn-danger" onClick={handleDeactivate} disabled={deactivating}>
            {deactivating ? 'Closing…' : 'Close request'}
          </button>
        )}
      </div>

      {/* ── Stat strip ── */}
      <div className="stat-cards" style={{ marginBottom: 16 }}>
        {[
          { label: 'Files received', value: String(request.submission_count ?? submissions.length) },
          { label: 'Expires', value: fmt(request.expires_at) },
          { label: 'Max file size', value: request.max_size_mb > 0 ? `${request.max_size_mb} MB` : 'No limit' },
          { label: 'Max files', value: request.max_files > 0 ? String(request.max_files) : 'No limit' },
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
            ['Created', fmt(request.created_at)],
            ['Expires', fmt(request.expires_at)],
            ['Allowed types', request.allowed_types || 'All types allowed'],
            ['Status', isClosed ? 'Closed' : request.is_expired || new Date(request.expires_at) < new Date() ? 'Expired' : 'Active'],
          ] as [string, string][]).map(([label, value]) => (
            <div key={label} style={{ display: 'flex', flexDirection: 'column', padding: '10px 0', borderBottom: '1px solid var(--color-border)' }}>
              <span style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.07em', color: 'var(--color-text-muted)', marginBottom: 3 }}>{label}</span>
              <span style={{ fontSize: 14, color: 'var(--color-text-primary)' }}>{value}</span>
            </div>
          ))}
        </div>

        {/* Upload link */}
        <div style={{ marginTop: 20 }}>
          <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.07em', color: 'var(--color-text-muted)', marginBottom: 8 }}>
            Upload link
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', background: 'var(--color-bg)', border: '1px solid var(--color-border)', borderRadius: 8, padding: '10px 14px' }}>
            <a
              href={publicUrl}
              target="_blank"
              rel="noreferrer"
              className="text-link"
              style={{ fontSize: 13, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
            >
              {publicUrl}
            </a>
            <button type="button" className="btn btn-secondary btn-sm" onClick={handleCopy} style={{ flexShrink: 0 }}>
              {copied ? '✓ Copied' : 'Copy link'}
            </button>
          </div>
          {isActive && (
            <p style={{ margin: '8px 0 0', fontSize: 12, color: 'var(--color-text-muted)' }}>
              Share this link with anyone you want to receive files from — no account required.
            </p>
          )}
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

        {/* Current recipient chips */}
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
            No recipients set — share the upload link manually.
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
                <strong>{newlyAdded.length} new recipient{newlyAdded.length > 1 ? 's' : ''}</strong> will receive the upload link via email when you apply these changes.
              </p>
            ) : (
              <p style={{ margin: '0 0 10px', fontSize: 13, color: 'var(--color-info-text)' }}>You have unsaved changes to the recipient list.</p>
            )}
            <div style={{ display: 'flex', gap: 8 }}>
              <button type="button" className="btn btn-primary btn-sm" onClick={handleSaveRecipients} disabled={savingRecipients}>
                {savingRecipients ? 'Updating…' : newlyAdded.length > 0 ? 'Update & send invites' : 'Update recipients'}
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
              <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-primary)', marginBottom: 2 }}>Resend invite email</div>
              <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>Re-send the upload link to all current recipients.</div>
              {resendSuccess && <div style={{ fontSize: 12, color: 'var(--color-success)', marginTop: 4 }}>✓ Email queued for delivery.</div>}
            </div>
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={handleResend}
              disabled={resending}
              style={{ flexShrink: 0 }}
            >
              {resending ? 'Sending…' : 'Resend invite'}
            </button>
          </div>
        )}
      </div>

      {/* ── Submissions ── */}
      <div className="card">
        <div className="card-header">
          <span className="card-title">
            Received files
            {submissions.length > 0 && (
              <span className="badge badge-blue" style={{ marginLeft: 8, verticalAlign: 'middle' }}>
                {submissions.length}
              </span>
            )}
          </span>
        </div>

        {submissions.length === 0 ? (
          <p style={{ margin: 0, fontSize: 14, color: 'var(--color-text-muted)' }}>
            {isActive
              ? 'No files received yet. Share the upload link to start collecting files.'
              : 'No files were received for this request.'}
          </p>
        ) : (
          <div>
            {submissions.map((sub) => (
              <SubmissionRow key={sub.id} sub={sub} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
