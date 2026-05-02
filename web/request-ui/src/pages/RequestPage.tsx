import { useState, useEffect, useCallback, useRef } from 'react';
import type { FileRequestPublic } from '../types';
import { RequestApiError } from '../types';
import { fetchRequest, uploadFile } from '../api/requests';

// ─── Slug resolution ───────────────────────────────────────────────────────
function resolveSlug(): string | null {
  const m = window.location.pathname.match(/\/r\/([^/]+)/);
  return m ? m[1] : null;
}

// ─── Helpers ───────────────────────────────────────────────────────────────
function fmtBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${units[i]}`;
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

// ─── File upload state ─────────────────────────────────────────────────────
type FileStatus = 'pending' | 'uploading' | 'done' | 'error';

interface FileEntry {
  id: string;
  file: File;
  status: FileStatus;
  progress: number;
  error?: string;
}

// ─── Icon components ───────────────────────────────────────────────────────

function IconUpload() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="16 16 12 12 8 16" />
      <line x1="12" y1="12" x2="12" y2="21" />
      <path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3" />
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

function IconCheck() {
  return (
    <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
      <polyline points="22 4 12 14.01 9 11.01" />
    </svg>
  );
}

function IconClock() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <polyline points="12 6 12 12 16 14" />
    </svg>
  );
}

function IconLock() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
    </svg>
  );
}

function IconAlert() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  );
}

function IconClose() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="18" y1="6" x2="6" y2="18" />
      <line x1="6" y1="6" x2="18" y2="18" />
    </svg>
  );
}

// ─── Layout ────────────────────────────────────────────────────────────────

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="tenzo-page">
      <div className="tenzo-card">
        <div className="tenzo-brand">
          <div className="tenzo-brand-icon">
            <img src="/logo.png" alt="TenzoShare" style={{ width: 32, height: 32, objectFit: 'contain' }} />
          </div>
          <span className="tenzo-brand-name">TenzoShare</span>
        </div>
        {children}
      </div>
    </div>
  );
}

// ─── Footer ────────────────────────────────────────────────────────────────

function TenzoFooter() {
  return (
    <div className="tenzo-footer">
      <img src="/logo.png" alt="" style={{ width: 14, height: 14, objectFit: 'contain' }} />
      Files are encrypted and delivered securely via TenzoShare
    </div>
  );
}

// ─── Main component ────────────────────────────────────────────────────────

export default function RequestPage() {
  const slug = resolveSlug();

  type View =
    | { kind: 'loading' }
    | { kind: 'error'; message: string; status?: number }
    | { kind: 'closed'; reason: 'expired' | 'inactive' }
    | { kind: 'open'; request: FileRequestPublic }
    | { kind: 'success' };

  const [view, setView] = useState<View>({ kind: 'loading' });
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [submitterName, setSubmitterName] = useState('');
  const [message, setMessage] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!slug) {
      setView({ kind: 'error', message: 'No request link found. Please check the URL.' });
      return;
    }
    fetchRequest(slug)
      .then((req) => {
        if (req.is_expired || !req.is_active) {
          setView({ kind: 'closed', reason: req.is_active ? 'expired' : 'inactive' });
        } else {
          setView({ kind: 'open', request: req });
        }
      })
      .catch((err: unknown) => {
        const msg = err instanceof RequestApiError ? err.message : 'Failed to load request.';
        const status = err instanceof RequestApiError ? err.status : undefined;
        setView({ kind: 'error', message: msg, status });
      });
  }, [slug]);

  const addFiles = useCallback((incoming: FileList | File[]) => {
    const arr = Array.from(incoming);
    const entries: FileEntry[] = arr.map((f) => ({
      id: `${f.name}-${f.size}-${Date.now()}-${Math.random()}`,
      file: f,
      status: 'pending',
      progress: 0,
    }));
    setFiles((prev) => [...prev, ...entries]);
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      if (e.dataTransfer.files.length > 0) addFiles(e.dataTransfer.files);
    },
    [addFiles],
  );

  const handleDragOver = (e: React.DragEvent) => { e.preventDefault(); setDragOver(true); };
  const handleDragLeave = () => setDragOver(false);

  const handleFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) addFiles(e.target.files);
    e.target.value = '';
  };

  const removeFile = (id: string) => {
    setFiles((prev) => prev.filter((f) => f.id !== id));
  };

  const handleSubmit = async () => {
    if (!slug || files.length === 0) return;
    setSubmitting(true);

    const pending = files.filter((f) => f.status === 'pending' || f.status === 'error');
    let allOk = true;

    for (const entry of pending) {
      setFiles((prev) =>
        prev.map((f) => (f.id === entry.id ? { ...f, status: 'uploading', progress: 0 } : f)),
      );
      try {
        await uploadFile(slug, entry.file, submitterName, message, (pct) => {
          setFiles((prev) =>
            prev.map((f) => (f.id === entry.id ? { ...f, progress: pct } : f)),
          );
        });
        setFiles((prev) =>
          prev.map((f) => (f.id === entry.id ? { ...f, status: 'done', progress: 100 } : f)),
        );
      } catch (err: unknown) {
        const errMsg = err instanceof RequestApiError ? err.message : 'Upload failed';
        setFiles((prev) =>
          prev.map((f) => (f.id === entry.id ? { ...f, status: 'error', error: errMsg } : f)),
        );
        allOk = false;
      }
    }

    setSubmitting(false);
    if (allOk) setView({ kind: 'success' });
  };

  // ─── Render ──────────────────────────────────────────────────────────────

  if (view.kind === 'loading') {
    return (
      <Layout>
        <div className="state-center">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, justifyContent: 'center' }}>
            <span className="spinner" />
            <span className="tenzo-muted">Loading request…</span>
          </div>
        </div>
      </Layout>
    );
  }

  if (view.kind === 'error') {
    return (
      <Layout>
        <div className="state-center">
          <div className="state-icon state-icon-error">
            <IconAlert />
          </div>
          <h2 className="tenzo-title">
            {view.status === 404 ? 'Request not found' : 'Something went wrong'}
          </h2>
          <p className="tenzo-subtitle">{view.message}</p>
        </div>
      </Layout>
    );
  }

  if (view.kind === 'closed') {
    const isExpired = view.reason === 'expired';
    return (
      <Layout>
        <div className="state-center">
          <div className={`state-icon ${isExpired ? 'state-icon-warn' : 'state-icon-error'}`}>
            {isExpired ? <IconClock /> : <IconLock />}
          </div>
          <h2 className="tenzo-title">
            {isExpired ? 'This request has expired' : 'This request is closed'}
          </h2>
          <p className="tenzo-subtitle">
            {isExpired
              ? 'The upload deadline has passed. Please contact the requester for a new link.'
              : 'This request has been closed by the requester.'}
          </p>
        </div>
        <TenzoFooter />
      </Layout>
    );
  }

  if (view.kind === 'success') {
    return (
      <Layout>
        <div className="state-center">
          <div className="state-icon state-icon-teal">
            <IconCheck />
          </div>
          <h2 className="tenzo-title">Files submitted!</h2>
          <p className="tenzo-subtitle">
            Your files have been uploaded successfully. The requester has been notified.
          </p>
        </div>
        <TenzoFooter />
      </Layout>
    );
  }

  const req = view.request;
  const hasFiles = files.length > 0;
  const allDone = hasFiles && files.every((f) => f.status === 'done');
  const isUploading = files.some((f) => f.status === 'uploading');
  const pendingCount = files.filter((f) => f.status !== 'done').length;
  const canSubmit = hasFiles && !submitting && !allDone && !isUploading;

  return (
    <Layout>
      <h2 className="tenzo-title">{req.name}</h2>
      {req.description && (
        <p className="tenzo-subtitle">{req.description}</p>
      )}

      {/* Metadata chips */}
      <div className="chips-row">
        <span className="chip">Expires {fmtDate(req.expires_at)}</span>
        {req.max_size_mb > 0 && (
          <span className="chip">Max {req.max_size_mb} MB per file</span>
        )}
        {req.max_files > 0 && (
          <span className="chip">Up to {req.max_files} file{req.max_files !== 1 ? 's' : ''}</span>
        )}
        {req.allowed_types && (
          <span className="chip">{req.allowed_types}</span>
        )}
      </div>

      {/* Drop zone */}
      <div
        className={`drop-zone${dragOver ? ' active' : ''}`}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onClick={() => fileInputRef.current?.click()}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => e.key === 'Enter' && fileInputRef.current?.click()}
        aria-label="Upload files"
      >
        <div className="drop-zone-icon">
          <IconUpload />
        </div>
        <p className="drop-zone-text">
          {dragOver ? 'Drop files here' : 'Drag & drop files here, or click to browse'}
        </p>
        <p className="drop-zone-hint">
          {req.max_size_mb > 0 ? `Up to ${req.max_size_mb} MB per file` : 'No file size limit'}
        </p>
      </div>
      <input
        ref={fileInputRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={handleFileInput}
        aria-hidden="true"
      />

      {/* File list */}
      {files.length > 0 && (
        <ul className="file-list">
          {files.map((entry) => (
            <li key={entry.id} className="file-item">
              <div className="file-item-row">
                <div className="file-icon">
                  <IconFile />
                </div>
                <span className="file-name">{entry.file.name}</span>
                <span className="file-size">{fmtBytes(entry.file.size)}</span>
                {entry.status === 'done' && (
                  <span className="file-status-done" aria-label="Uploaded">✓</span>
                )}
                {entry.status === 'uploading' && (
                  <span className="spinner" style={{ width: 14, height: 14, borderWidth: 1.5 }} />
                )}
                {entry.status === 'pending' && (
                  <button
                    className="remove-btn"
                    onClick={() => removeFile(entry.id)}
                    title="Remove file"
                    type="button"
                    aria-label={`Remove ${entry.file.name}`}
                  >
                    <IconClose />
                  </button>
                )}
              </div>
              {entry.status === 'error' && entry.error && (
                <div className="file-status-error">{entry.error}</div>
              )}
              {(entry.status === 'uploading' || entry.status === 'done' || entry.status === 'error') && (
                <div className="progress-bar-wrap">
                  <div
                    className={`progress-bar progress-bar-${entry.status}`}
                    style={{ width: `${entry.progress}%` }}
                    role="progressbar"
                    aria-valuenow={entry.progress}
                    aria-valuemin={0}
                    aria-valuemax={100}
                  />
                </div>
              )}
            </li>
          ))}
        </ul>
      )}

      <hr className="tenzo-divider" />

      {/* Optional fields */}
      <div className="form-group">
        <label htmlFor="submitter-name">
          Your name <span className="form-label-optional">(optional)</span>
        </label>
        <input
          id="submitter-name"
          type="text"
          className="tenzo-input"
          value={submitterName}
          onChange={(e) => setSubmitterName(e.target.value)}
          placeholder="e.g. Jane Smith"
          maxLength={100}
        />
      </div>

      <div className="form-group">
        <label htmlFor="req-message">
          Message <span className="form-label-optional">(optional)</span>
        </label>
        <textarea
          id="req-message"
          className="tenzo-input"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Add a note for the requester…"
          maxLength={500}
          rows={3}
        />
      </div>

      <button
        type="button"
        className="btn btn-primary btn-full btn-lg"
        style={{ marginTop: 8 }}
        onClick={handleSubmit}
        disabled={!canSubmit}
      >
        {isUploading ? (
          <>
            <span className="spinner" style={{ width: 14, height: 14, borderWidth: 2, borderColor: 'rgba(255,255,255,0.3)', borderTopColor: '#fff' }} />
            Uploading…
          </>
        ) : allDone ? (
          'All files uploaded ✓'
        ) : (
          `Submit ${pendingCount > 0 ? `${pendingCount} file${pendingCount !== 1 ? 's' : ''}` : 'files'}`
        )}
      </button>

      <TenzoFooter />
    </Layout>
  );
}


