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
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

function fmt(date: string) {
  return new Date(date).toLocaleString();
}

export default function FilesPage() {
  const [files, setFiles] = useState<FileRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState<UploadProgress | null>(null);
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

  return (
    <div className="page">
      <div className="row mb-16" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <h1 className="page-title" style={{ margin: 0 }}>Files</h1>
        <div>
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
            {uploading ? `Uploading ${pct}%…` : 'Upload file'}
          </button>
        </div>
      </div>

      {uploading && progress && (
        <div className="mb-16">
          <div className="text-sm">{fmtBytes(progress.loaded)} / {fmtBytes(progress.total)}</div>
          <div className="progress-bar-wrap">
            <div className="progress-bar-fill" style={{ width: `${pct}%` }} />
          </div>
        </div>
      )}

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <div className="empty-state">Loading…</div>
      ) : files.length === 0 ? (
        <div className="empty-state">No files yet. Upload one above.</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Filename</th>
                <th>Type</th>
                <th>Size</th>
                <th>Uploaded</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {files.map((f) => (
                <tr key={f.id}>
                  <td>{f.filename}</td>
                  <td className="text-sm">{f.content_type}</td>
                  <td className="text-sm">{fmtBytes(f.size_bytes)}</td>
                  <td className="text-sm">{fmt(f.created_at)}</td>
                  <td>
                    <button
                      className="btn btn-secondary btn-sm"
                      onClick={() => handleDownload(f.id)}
                      style={{ marginRight: 4 }}
                    >
                      Download
                    </button>
                    <button
                      className="btn btn-danger btn-sm"
                      onClick={() => handleDelete(f.id, f.filename)}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
