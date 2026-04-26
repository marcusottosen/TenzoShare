import React, { useState } from 'react';
import { useNavigate } from 'react-router';
import { createFileRequest } from '../api/requests';

export default function RequestsPage() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [allowedTypes, setAllowedTypes] = useState('');
  const [maxSizeMB, setMaxSizeMB] = useState('');
  const [expiresInHrs, setExpiresInHrs] = useState('72');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) { setError('Name is required.'); return; }
    const hrs = parseInt(expiresInHrs, 10);
    if (!hrs || hrs < 1) { setError('Expiry must be at least 1 hour.'); return; }
    setError('');
    setLoading(true);
    try {
      await createFileRequest({
        name: name.trim(),
        description: description.trim() || undefined,
        allowed_types: allowedTypes.trim() || undefined,
        max_size_mb: maxSizeMB ? parseInt(maxSizeMB, 10) : undefined,
        expires_in_hours: hrs,
      });
      navigate('/shares');
    } catch (err: unknown) {
      setError((err as Error).message ?? 'Failed to create request.');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="page" style={{ maxWidth: 700 }}>
      <div className="page-header" style={{ marginBottom: 28 }}>
        <div>
          <h1 className="page-title">New File Request</h1>
          <p className="page-subtitle">
            Create a link that lets anyone upload files directly to you — no account required.
          </p>
        </div>
      </div>

      <div className="card">
        {error && <div className="alert alert-error" style={{ marginBottom: 20 }}>{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Name *</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Project deliverables, Client assets"
              maxLength={200}
              required
              autoFocus
            />
          </div>

          <div className="form-group">
            <label>Description</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Instructions for the submitter (optional)"
              maxLength={1000}
              rows={3}
              style={{ resize: 'vertical' }}
            />
          </div>

          <div className="row" style={{ gap: 12 }}>
            <div className="form-group" style={{ flex: 1 }}>
              <label>Allowed file types</label>
              <input
                type="text"
                value={allowedTypes}
                onChange={(e) => setAllowedTypes(e.target.value)}
                placeholder="e.g. image/,application/pdf"
              />
              <small style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
                Comma-separated MIME prefixes. Leave blank to allow all types.
              </small>
            </div>
            <div className="form-group" style={{ width: 140 }}>
              <label>Max file size (MB)</label>
              <input
                type="number"
                value={maxSizeMB}
                onChange={(e) => setMaxSizeMB(e.target.value)}
                placeholder="No limit"
                min={1}
              />
            </div>
          </div>

          <div className="form-group">
            <label>Expires in (hours) *</label>
            <input
              type="number"
              value={expiresInHrs}
              onChange={(e) => setExpiresInHrs(e.target.value)}
              min={1}
              max={8760}
              required
            />
            <small style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
              1 day = 24 hrs · 1 week = 168 hrs · max 1 year = 8760 hrs
            </small>
          </div>

          <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end', marginTop: 8 }}>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => navigate('/shares')}
              disabled={loading}
            >
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? 'Creating…' : 'Create Request'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
