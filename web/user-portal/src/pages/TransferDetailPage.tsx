import React, { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router';
import { getTransfer, revokeTransfer, type Transfer } from '../api/transfers';
import { presignFile } from '../api/files';

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

  async function handleDownload(fileId: string) {
    try {
      const { url } = await presignFile(fileId);
      window.open(url, '_blank');
    } catch (err: any) {
      alert(err.message);
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
              <td style={{ paddingLeft: 0, fontWeight: 500 }}>Downloads</td>
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
          <div className="card-title">Files ({transfer.file_ids.length})</div>
          <div>
            {transfer.file_ids.map((fid) => (
              <div key={fid} className="row" style={{ marginBottom: 6, alignItems: 'center' }}>
                <span className="text-sm" style={{ flex: 1, fontFamily: 'monospace' }}>{fid}</span>
                <button
                  className="btn btn-secondary btn-sm"
                  onClick={() => handleDownload(fid)}
                >
                  Download
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
