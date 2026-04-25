import { useState, useEffect, useCallback, useRef } from 'react';
import type { FileRequestPublic } from '../types';
import { RequestApiError } from '../types';
import { fetchRequest, uploadFile } from '../api/requests';

// ---------------------------------------------------------------------------
// Slug resolution — extract from /r/<slug>
// ---------------------------------------------------------------------------
function resolveSlug(): string | null {
  const m = window.location.pathname.match(/\/r\/([^/]+)/);
  return m ? m[1] : null;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${units[i]}`;
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

// ---------------------------------------------------------------------------
// File upload state machine
// ---------------------------------------------------------------------------
type FileStatus = 'pending' | 'uploading' | 'done' | 'error';

interface FileEntry {
  id: string;
  file: File;
  status: FileStatus;
  progress: number; // 0–100
  error?: string;
}

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------
const styles = {
  page: {
    minHeight: '100vh',
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    justifyContent: 'flex-start',
    padding: '48px 16px',
    background: '#f8f9fa',
  },
  card: {
    background: '#fff',
    borderRadius: 12,
    boxShadow: '0 2px 12px rgba(0,0,0,0.08)',
    padding: 32,
    width: '100%',
    maxWidth: 560,
  },
  logo: {
    fontSize: 13,
    fontWeight: 600,
    letterSpacing: '0.08em',
    color: '#6c757d',
    textTransform: 'uppercase' as const,
    marginBottom: 24,
  },
  heading: {
    fontSize: 22,
    fontWeight: 700,
    margin: '0 0 8px',
    color: '#111',
  },
  description: {
    fontSize: 15,
    color: '#555',
    margin: '0 0 20px',
    lineHeight: 1.5,
  },
  metaRow: {
    display: 'flex',
    flexWrap: 'wrap' as const,
    gap: 8,
    marginBottom: 24,
  },
  chip: {
    background: '#f1f3f5',
    borderRadius: 20,
    padding: '3px 10px',
    fontSize: 12,
    color: '#495057',
  },
  dropZone: (active: boolean) => ({
    border: `2px dashed ${active ? '#4263eb' : '#ced4da'}`,
    borderRadius: 8,
    padding: '32px 16px',
    textAlign: 'center' as const,
    cursor: 'pointer',
    transition: 'border-color 0.15s',
    background: active ? '#edf2ff' : '#fafafa',
    marginBottom: 16,
  }),
  dropText: {
    fontSize: 14,
    color: '#868e96',
    margin: 0,
  },
  dropHint: {
    fontSize: 12,
    color: '#adb5bd',
    marginTop: 4,
  },
  fileList: {
    listStyle: 'none',
    padding: 0,
    margin: '0 0 16px',
  },
  fileItem: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    padding: '8px 10px',
    borderRadius: 6,
    background: '#f8f9fa',
    marginBottom: 6,
    fontSize: 13,
  },
  fileName: {
    flex: 1,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
    color: '#212529',
  },
  fileSize: {
    color: '#868e96',
    flexShrink: 0,
  },
  progressBar: (pct: number, status: FileStatus) => ({
    height: 3,
    background: status === 'error' ? '#fa5252' : status === 'done' ? '#40c057' : '#4263eb',
    borderRadius: 2,
    width: `${pct}%`,
    transition: 'width 0.2s',
    marginTop: 4,
  }),
  removeBtn: {
    background: 'none',
    border: 'none',
    cursor: 'pointer',
    color: '#adb5bd',
    fontSize: 16,
    padding: '0 2px',
    lineHeight: 1,
    flexShrink: 0,
  },
  field: {
    marginBottom: 14,
  },
  label: {
    display: 'block',
    fontSize: 13,
    fontWeight: 500,
    color: '#495057',
    marginBottom: 4,
  },
  input: {
    width: '100%',
    border: '1px solid #ced4da',
    borderRadius: 6,
    padding: '8px 10px',
    fontSize: 14,
    fontFamily: 'inherit',
    outline: 'none',
    boxSizing: 'border-box' as const,
  },
  btnPrimary: (disabled: boolean) => ({
    display: 'block',
    width: '100%',
    padding: '11px 0',
    background: disabled ? '#a5b4fc' : '#4263eb',
    color: '#fff',
    border: 'none',
    borderRadius: 7,
    fontSize: 15,
    fontWeight: 600,
    cursor: disabled ? 'not-allowed' : 'pointer',
    marginTop: 8,
  }),
  alert: (type: 'error' | 'success') => ({
    padding: '10px 14px',
    borderRadius: 6,
    fontSize: 14,
    marginBottom: 16,
    background: type === 'error' ? '#fff5f5' : '#ebfbee',
    color: type === 'error' ? '#c92a2a' : '#2b8a3e',
    border: `1px solid ${type === 'error' ? '#ffc9c9' : '#b2f2bb'}`,
  }),
  muted: {
    color: '#868e96',
    fontSize: 14,
    margin: 0,
  },
  expiredCard: {
    textAlign: 'center' as const,
    padding: '40px 24px',
  },
  expiredIcon: {
    fontSize: 40,
    marginBottom: 12,
  },
};

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------
export default function RequestPage() {
  const slug = resolveSlug();

  // View state
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

  // Fetch request metadata on mount
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

  // Add files helper
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

  // Drag & drop handlers
  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      if (e.dataTransfer.files.length > 0) addFiles(e.dataTransfer.files);
    },
    [addFiles],
  );

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  };

  const handleDragLeave = () => setDragOver(false);

  const handleFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) addFiles(e.target.files);
    e.target.value = '';
  };

  const removeFile = (id: string) => {
    setFiles((prev) => prev.filter((f) => f.id !== id));
  };

  // Submit handler — uploads files one at a time
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

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  if (view.kind === 'loading') {
    return (
      <div style={styles.page}>
        <div style={styles.card}>
          <div style={styles.logo}>TenzoShare</div>
          <p style={styles.muted}>Loading…</p>
        </div>
      </div>
    );
  }

  if (view.kind === 'error') {
    return (
      <div style={styles.page}>
        <div style={styles.card}>
          <div style={styles.logo}>TenzoShare</div>
          <div style={styles.alert('error')}>
            {view.status === 404
              ? 'This file request does not exist.'
              : view.message}
          </div>
        </div>
      </div>
    );
  }

  if (view.kind === 'closed') {
    return (
      <div style={styles.page}>
        <div style={styles.card}>
          <div style={styles.logo}>TenzoShare</div>
          <div style={styles.expiredCard}>
            <div style={styles.expiredIcon}>{view.reason === 'expired' ? '⌛' : '🔒'}</div>
            <h2 style={{ ...styles.heading, marginBottom: 8 }}>
              {view.reason === 'expired' ? 'This request has expired' : 'This request is closed'}
            </h2>
            <p style={styles.muted}>
              {view.reason === 'expired'
                ? 'The upload deadline has passed. Please contact the requester for a new link.'
                : 'This request has been closed by the requester.'}
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (view.kind === 'success') {
    return (
      <div style={styles.page}>
        <div style={styles.card}>
          <div style={styles.logo}>TenzoShare</div>
          <div style={{ textAlign: 'center', padding: '32px 0' }}>
            <div style={{ fontSize: 48, marginBottom: 16 }}>✅</div>
            <h2 style={{ ...styles.heading, marginBottom: 8 }}>Files submitted!</h2>
            <p style={styles.muted}>
              Your files have been uploaded successfully. The requester has been notified.
            </p>
          </div>
        </div>
      </div>
    );
  }

  const req = view.request;
  const hasFiles = files.length > 0;
  const allDone = hasFiles && files.every((f) => f.status === 'done');
  const isUploading = files.some((f) => f.status === 'uploading');
  const canSubmit = hasFiles && !submitting && !allDone && !isUploading;

  return (
    <div style={styles.page}>
      <div style={styles.card}>
        <div style={styles.logo}>TenzoShare</div>

        <h1 style={styles.heading}>{req.name}</h1>
        {req.description && <p style={styles.description}>{req.description}</p>}

        <div style={styles.metaRow}>
          <span style={styles.chip}>⏰ Expires {fmtDate(req.expires_at)}</span>
          {req.max_size_mb > 0 && (
            <span style={styles.chip}>📦 Max {req.max_size_mb} MB per file</span>
          )}
          {req.max_files > 0 && (
            <span style={styles.chip}>📁 Max {req.max_files} file{req.max_files !== 1 ? 's' : ''}</span>
          )}
          {req.allowed_types && (
            <span style={styles.chip}>🗂 {req.allowed_types}</span>
          )}
        </div>

        {/* Drop zone */}
        <div
          style={styles.dropZone(dragOver)}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onClick={() => fileInputRef.current?.click()}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => e.key === 'Enter' && fileInputRef.current?.click()}
        >
          <p style={styles.dropText}>Drag & drop files here, or click to browse</p>
          <p style={styles.dropHint}>
            {req.max_size_mb > 0 ? `Up to ${req.max_size_mb} MB per file` : 'No size limit'}
          </p>
        </div>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          style={{ display: 'none' }}
          onChange={handleFileInput}
        />

        {/* File list */}
        {files.length > 0 && (
          <ul style={styles.fileList}>
            {files.map((entry) => (
              <li key={entry.id} style={styles.fileItem}>
                <span style={styles.fileName}>{entry.file.name}</span>
                <span style={styles.fileSize}>{fmtBytes(entry.file.size)}</span>
                {entry.status === 'done' && <span style={{ color: '#40c057' }}>✓</span>}
                {entry.status === 'error' && (
                  <span style={{ color: '#fa5252' }} title={entry.error}>✗</span>
                )}
                {entry.status === 'pending' && (
                  <button
                    style={styles.removeBtn}
                    onClick={() => removeFile(entry.id)}
                    title="Remove"
                    type="button"
                  >
                    ×
                  </button>
                )}
                {(entry.status === 'uploading' || entry.status === 'done' || entry.status === 'error') && (
                  <div style={{ width: '100%', marginTop: 4 }}>
                    <div style={styles.progressBar(entry.progress, entry.status)} />
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}

        {/* Optional fields */}
        <div style={styles.field}>
          <label style={styles.label} htmlFor="submitter-name">
            Your name <span style={{ color: '#adb5bd', fontWeight: 400 }}>(optional)</span>
          </label>
          <input
            id="submitter-name"
            type="text"
            style={styles.input}
            value={submitterName}
            onChange={(e) => setSubmitterName(e.target.value)}
            placeholder="e.g. Jane Smith"
            maxLength={100}
          />
        </div>

        <div style={styles.field}>
          <label style={styles.label} htmlFor="message">
            Message <span style={{ color: '#adb5bd', fontWeight: 400 }}>(optional)</span>
          </label>
          <textarea
            id="message"
            style={{ ...styles.input, resize: 'vertical', minHeight: 72 }}
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder="Add a note for the requester…"
            maxLength={500}
          />
        </div>

        <button
          type="button"
          style={styles.btnPrimary(!canSubmit)}
          onClick={handleSubmit}
          disabled={!canSubmit}
        >
          {isUploading
            ? 'Uploading…'
            : allDone
            ? 'All files uploaded ✓'
            : `Submit ${files.length > 0 ? `${files.filter((f) => f.status !== 'done').length} file${files.filter((f) => f.status !== 'done').length !== 1 ? 's' : ''}` : 'files'}`}
        </button>
      </div>
    </div>
  );
}
