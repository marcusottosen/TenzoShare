import React, { useEffect, useState, useRef } from 'react';
import { useNavigate } from 'react-router';
import { listFiles, uploadFile, type FileRecord, type UploadProgress } from '../api/files';
import { createTransfer } from '../api/transfers';
import { pendingUploadStore } from '../stores/pendingUpload';
import { pendingFileStore } from '../stores/pendingFileStore';

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

const EXPIRY_OPTIONS: { label: string; hours: number }[] = [
  { label: '1 hour',  hours: 1 },
  { label: '6 hours', hours: 6 },
  { label: '1 day',   hours: 24 },
  { label: '3 days',  hours: 72 },
  { label: '7 days',  hours: 168 },
  { label: '14 days', hours: 336 },
  { label: '30 days', hours: 720 },
  { label: '60 days', hours: 1440 },
  { label: '90 days', hours: 2160 },
];

/** A file that has been staged for this transfer (already uploaded or reused). */
interface StagedFile {
  id: string;
  filename: string;
  size_bytes: number;
}

export default function NewTransferPage() {
  const navigate = useNavigate();

  // Transfer metadata
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [recipientEmail, setRecipientEmail] = useState('');
  const [password, setPassword] = useState('');
  const [maxDownloads, setMaxDownloads] = useState(0);
  const [viewOnly, setViewOnly] = useState(false);
  const [expiresInHours, setExpiresInHours] = useState(168);

  // Files staged for this transfer
  const [staged, setStaged] = useState<StagedFile[]>([]);

  // Upload state
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<UploadProgress | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Library (reuse) panel
  const [showLibrary, setShowLibrary] = useState(false);
  const [library, setLibrary] = useState<FileRecord[]>([]);
  const [loadingLibrary, setLoadingLibrary] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  // Load library lazily when the panel is opened
  useEffect(() => {
    if (!showLibrary || library.length > 0) return;
    setLoadingLibrary(true);
    listFiles()
      .then((res) => setLibrary(res.files ?? []))
      .catch(() => {})
      .finally(() => setLoadingLibrary(false));
  }, [showLibrary]);

  // Consume any files forwarded from the dashboard upload zone
  useEffect(() => {
    const pending = pendingUploadStore.get();
    if (pending.length === 0) return;
    pendingUploadStore.clear();
    // Synthesize a fake ChangeEvent-like call by uploading directly
    (async () => {
      for (const file of pending) {
        setUploading(true);
        setUploadProgress(null);
        try {
          const record = await uploadFile(file, (p) => setUploadProgress(p));
          setStaged((prev) =>
            prev.some((s) => s.id === record.id)
              ? prev
              : [...prev, { id: record.id, filename: record.filename, size_bytes: record.size_bytes }],
          );
          setLibrary((prev) =>
            prev.some((f) => f.id === record.id) ? prev : [record, ...prev],
          );
        } catch (err: any) {
          setError(`Upload failed for "${file.name}": ${err.message}`);
        } finally {
          setUploading(false);
          setUploadProgress(null);
        }
      }
    })();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Consume existing file records forwarded from My Files share button
  useEffect(() => {
    const preStaged = pendingFileStore.get();
    if (preStaged.length === 0) return;
    pendingFileStore.clear();
    setStaged((prev) => {
      const next = [...prev];
      for (const f of preStaged) {
        if (!next.some((s) => s.id === f.id)) {
          next.push({ id: f.id, filename: f.filename, size_bytes: f.size_bytes });
        }
      }
      return next;
    });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function uploadFiles(files: File[]) {
    if (files.length === 0) return;
    for (const file of files) {
      setUploading(true);
      setUploadProgress(null);
      try {
        const record = await uploadFile(file, (p) => setUploadProgress(p));
        setStaged((prev) =>
          prev.some((s) => s.id === record.id)
            ? prev
            : [...prev, { id: record.id, filename: record.filename, size_bytes: record.size_bytes }],
        );
        setLibrary((prev) =>
          prev.some((f) => f.id === record.id) ? prev : [record, ...prev],
        );
      } catch (err: any) {
        setError(`Upload failed for "${file.name}": ${err.message}`);
      } finally {
        setUploading(false);
        setUploadProgress(null);
      }
    }
  }

  async function handlePickFiles(e: React.ChangeEvent<HTMLInputElement>) {
    const picked = Array.from(e.target.files ?? []);
    if (fileInputRef.current) fileInputRef.current.value = '';
    await uploadFiles(picked);
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragOver(false);
    if (uploading) return;
    const dropped = Array.from(e.dataTransfer.files);
    uploadFiles(dropped);
  }

  function removeStaged(id: string) {
    setStaged((prev) => prev.filter((f) => f.id !== id));
  }

  function addFromLibrary(f: FileRecord) {
    setStaged((prev) =>
      prev.some((s) => s.id === f.id)
        ? prev
        : [...prev, { id: f.id, filename: f.filename, size_bytes: f.size_bytes }],
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) { setError('Transfer name is required.'); return; }
    if (staged.length === 0) { setError('Add at least one file.'); return; }
    setError('');
    setSubmitting(true);
    try {
      const t = await createTransfer({
        name: name.trim(),
        description: description.trim() || undefined,
        file_ids: staged.map((f) => f.id),
        recipient_email: recipientEmail || undefined,
        password: password || undefined,
        max_downloads: maxDownloads || undefined,
        view_only: viewOnly || undefined,
        expires_in_hours: expiresInHours,
      });
      navigate(`/transfers/${t.id}`);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  const pct = uploadProgress
    ? Math.round((uploadProgress.loaded / uploadProgress.total) * 100)
    : 0;

  // IDs already staged — used to grey out library items
  const stagedIds = new Set(staged.map((f) => f.id));

  return (
    <div className="page page-wide" style={{ maxWidth: 860 }}>
      <div className="page-header">
        <div>
          <h1 className="page-title">New Transfer</h1>
          <p className="page-subtitle">Upload files and create a shareable link</p>
        </div>
      </div>
      {error && <div className="alert alert-error">{error}</div>}

      <form onSubmit={handleSubmit}>

        {/* ── Transfer details ─────────────────────────────────── */}
        <div className="card">
          <div className="card-header"><h2 className="card-title">Details</h2></div>
          <div className="form-group">
            <label>Name <span style={{ color: 'var(--color-danger)' }}>*</span></label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Q1 Reports, Project Assets"
              maxLength={200}
              required
            />
          </div>
          <div className="form-group">
            <label>
              Description{' '}
              <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>(optional)</span>
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Brief note for the recipient"
              maxLength={1000}
              rows={2}
              style={{ resize: 'vertical' }}
            />
          </div>
        </div>

        {/* ── Files ────────────────────────────────────────────── */}
        <div className="card">
          <div className="card-header"><h2 className="card-title">Files</h2></div>

          {/* Hidden file input */}
          <input
            type="file"
            multiple
            ref={fileInputRef}
            style={{ display: 'none' }}
            onChange={handlePickFiles}
          />

          {/* Upload drop-zone */}
          <div
            onDragOver={(e) => { e.preventDefault(); if (!uploading) setDragOver(true); }}
            onDragEnter={(e) => { e.preventDefault(); if (!uploading) setDragOver(true); }}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleDrop}
            style={{
              border: `2px dashed ${dragOver ? 'var(--color-primary)' : 'var(--color-border)'}`,
              borderRadius: 8,
              padding: '32px 16px',
              textAlign: 'center',
              background: dragOver ? 'var(--color-primary-light, rgba(99,102,241,0.05))' : 'var(--color-bg)',
              transition: 'border-color 0.15s, background 0.15s',
              marginBottom: 12,
            }}
          >
            {uploading ? (
              <>
                <div className="text-sm" style={{ marginBottom: 8 }}>
                  Uploading… {pct}%
                </div>
                {uploadProgress && (
                  <div className="progress-bar-wrap">
                    <div className="progress-bar-fill" style={{ width: `${pct}%` }} />
                  </div>
                )}
              </>
            ) : (
              <>
                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-muted)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ marginBottom: 8 }}>
                  <polyline points="16 16 12 12 8 16"/>
                  <line x1="12" y1="12" x2="12" y2="21"/>
                  <path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/>
                </svg>
                <div className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 2 }}>
                  {dragOver ? 'Drop files here' : 'Drag & drop files here'}
                </div>
              </>
            )}
          </div>

          {/* Click-to-select button */}
          {!uploading && (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => fileInputRef.current?.click()}
              style={{ width: '100%', marginBottom: staged.length > 0 ? 16 : 0 }}
            >
              Click to select files
            </button>
          )}

          {/* Staged file list */}
          {staged.length > 0 && (
            <div className="table-wrap" style={{ marginBottom: 12 }}>
              <table>
                <thead>
                  <tr>
                    <th>Filename</th>
                    <th>Size</th>
                    <th style={{ width: 40 }}></th>
                  </tr>
                </thead>
                <tbody>
                  {staged.map((f) => (
                    <tr key={f.id}>
                      <td>{f.filename}</td>
                      <td className="text-sm">{fmtBytes(f.size_bytes)}</td>
                      <td>
                        <button
                          type="button"
                          className="btn btn-secondary btn-sm"
                          onClick={() => removeStaged(f.id)}
                          title="Remove"
                          style={{ padding: '2px 8px' }}
                        >
                          ✕
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Library reuse toggle */}
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={() => setShowLibrary((v) => !v)}
            style={{ marginTop: 4 }}
          >
            {showLibrary ? 'Hide library' : 'Reuse a previously uploaded file ↓'}
          </button>

          {showLibrary && (
            <div style={{ marginTop: 12 }}>
              {loadingLibrary ? (
                <div className="text-sm">Loading…</div>
              ) : library.length === 0 ? (
                <div className="text-sm">No files in library yet.</div>
              ) : (
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Filename</th>
                        <th>Size</th>
                        <th style={{ width: 72 }}></th>
                      </tr>
                    </thead>
                    <tbody>
                      {library.map((f) => (
                        <tr key={f.id} style={{ opacity: stagedIds.has(f.id) ? 0.4 : 1 }}>
                          <td>{f.filename}</td>
                          <td className="text-sm">{fmtBytes(f.size_bytes)}</td>
                          <td>
                            <button
                              type="button"
                              className="btn btn-secondary btn-sm"
                              onClick={() => addFromLibrary(f)}
                              disabled={stagedIds.has(f.id)}
                            >
                              {stagedIds.has(f.id) ? 'Added' : 'Add'}
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </div>

        {/* ── Options ──────────────────────────────────────────── */}
        <div className="card">
          <div className="card-header"><h2 className="card-title">Options</h2></div>
          <div className="row">
            <div className="col">
              <div className="form-group">
                <label>
                  Recipient email{' '}
                  <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>(optional)</span>
                </label>
                <input
                  type="email"
                  value={recipientEmail}
                  onChange={(e) => setRecipientEmail(e.target.value)}
                  placeholder="recipient@example.com"
                />
              </div>
            </div>
            <div className="col">
              <div className="form-group">
                <label>
                  Password{' '}
                  <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>(optional)</span>
                </label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Leave blank for no password"
                />
              </div>
            </div>
          </div>
          <div className="row">
            <div className="col">
              <div className="form-group">
                <label>Expires</label>
                <select
                  value={expiresInHours}
                  onChange={(e) => setExpiresInHours(Number(e.target.value))}
                >
                  {EXPIRY_OPTIONS.map((opt) => (
                    <option key={opt.hours} value={opt.hours}>{opt.label}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="col">
              <div className="form-group">
                <label>
                  {viewOnly ? 'Max views' : 'Max downloads'}{' '}
                  <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>(0 = unlimited)</span>
                </label>
                <input
                  type="number"
                  min={0}
                  value={maxDownloads}
                  onChange={(e) => setMaxDownloads(parseInt(e.target.value) || 0)}
                />
              </div>
            </div>
          </div>

          {/* View-only toggle */}
          <div style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: 12,
            padding: '14px 16px',
            background: 'var(--color-input-bg)',
            border: `1px solid ${viewOnly ? 'var(--color-secondary)' : 'var(--color-border)'}`,
            borderRadius: 8,
            marginTop: 4,
            transition: 'all 0.15s',
          }}>
            <input
              id="view-only"
              type="checkbox"
              checked={viewOnly}
              onChange={(e) => setViewOnly(e.target.checked)}
              style={{ marginTop: 2, flexShrink: 0, cursor: 'pointer', accentColor: 'var(--color-primary)' }}
            />
            <div style={{ flex: 1 }}>
              <label htmlFor="view-only" style={{ fontWeight: 600, fontSize: 13, cursor: 'pointer', display: 'block', marginBottom: 2 }}>
                View only — no download
              </label>
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', margin: 0, lineHeight: 1.5 }}>
                Recipients can open and read files in the browser but will not see a download button.
                The server enforces this by serving files inline.{' '}
                <em>Note: determined users may still save via browser tools — this is a workflow and compliance aid, not DRM.</em>
              </p>
            </div>
          </div>
        </div>

        <button
          className="btn btn-primary"
          type="submit"
          disabled={submitting || staged.length === 0}
        >
          {submitting ? 'Creating…' : `Create transfer${staged.length > 0 ? ` (${staged.length} file${staged.length > 1 ? 's' : ''})` : ''}`}
        </button>

      </form>
    </div>
  );
}

