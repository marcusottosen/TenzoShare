import React, { useEffect, useState, useRef } from 'react';
import { useNavigate } from 'react-router';
import { listFiles, uploadFile, type FileRecord, type UploadProgress } from '../api/files';
import { createTransfer } from '../api/transfers';

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
  const [expiresInHours, setExpiresInHours] = useState(168);

  // Files staged for this transfer
  const [staged, setStaged] = useState<StagedFile[]>([]);

  // Upload state
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<UploadProgress | null>(null);
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

  async function handlePickFiles(e: React.ChangeEvent<HTMLInputElement>) {
    const picked = Array.from(e.target.files ?? []);
    if (picked.length === 0) return;
    if (fileInputRef.current) fileInputRef.current.value = '';

    for (const file of picked) {
      setUploading(true);
      setUploadProgress(null);
      try {
        const record = await uploadFile(file, (p) => setUploadProgress(p));
        setStaged((prev) =>
          prev.some((s) => s.id === record.id)
            ? prev
            : [...prev, { id: record.id, filename: record.filename, size_bytes: record.size_bytes }],
        );
        // Keep library in sync so newly uploaded files show up if panel is open
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
    <div className="page">
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

          {/* Upload drop-zone */}
          <div
            style={{
              border: '2px dashed var(--color-border)',
              borderRadius: 8,
              padding: '20px 16px',
              textAlign: 'center',
              cursor: uploading ? 'wait' : 'pointer',
              background: 'var(--color-bg)',
              marginBottom: staged.length > 0 ? 16 : 0,
            }}
            onClick={() => !uploading && fileInputRef.current?.click()}
          >
            <input
              type="file"
              multiple
              ref={fileInputRef}
              style={{ display: 'none' }}
              onChange={handlePickFiles}
            />
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
              <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Click to select files
              </span>
            )}
          </div>

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
                  Max downloads{' '}
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

