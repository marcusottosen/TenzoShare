import React, { useState } from 'react';
import { useNavigate } from 'react-router';
import { createFileRequest } from '../api/requests';

function isValidEmail(e: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e.trim());
}

export default function RequestsPage() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [allowedTypes, setAllowedTypes] = useState('');
  const [maxSizeMB, setMaxSizeMB] = useState('');
  const [expiresInHrs, setExpiresInHrs] = useState('72');
  const [recipientEmails, setRecipientEmails] = useState<string[]>([]);
  const [emailInput, setEmailInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  function commitEmailInput() {
    const val = emailInput.trim().replace(/,+$/, '');
    if (!val) return;
    const parts = val.split(',').map((s) => s.trim()).filter(Boolean);
    const valid: string[] = [];
    const invalid: string[] = [];
    for (const p of parts) {
      if (isValidEmail(p) && !recipientEmails.includes(p)) valid.push(p);
      else if (!isValidEmail(p)) invalid.push(p);
    }
    if (invalid.length) {
      setError(`Invalid email(s): ${invalid.join(', ')}`);
      return;
    }
    setRecipientEmails((prev) => [...prev, ...valid]);
    setEmailInput('');
  }

  function removeEmail(email: string) {
    setRecipientEmails((prev) => prev.filter((e) => e !== email));
  }

  function handleEmailKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' || e.key === ',' || e.key === 'Tab') {
      e.preventDefault();
      commitEmailInput();
    } else if (e.key === 'Backspace' && emailInput === '' && recipientEmails.length > 0) {
      setRecipientEmails((prev) => prev.slice(0, -1));
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    // Commit any partially typed email before submitting
    if (emailInput.trim()) commitEmailInput();
    if (!name.trim()) { setError('Name is required.'); return; }
    const hrs = parseInt(expiresInHrs, 10);
    if (!hrs || hrs < 1) { setError('Expiry must be at least 1 hour.'); return; }
    setError('');
    setLoading(true);
    try {
      const req = await createFileRequest({
        name: name.trim(),
        description: description.trim() || undefined,
        allowed_types: allowedTypes.trim() || undefined,
        max_size_mb: maxSizeMB ? parseInt(maxSizeMB, 10) : undefined,
        expires_in_hours: hrs,
        recipient_emails: recipientEmails.length > 0 ? recipientEmails : undefined,
      });
      navigate(`/requests/${req.id}`);
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

          <div className="form-group">
            <label>Send link to (optional)</label>
            <div className="email-chips-field">
              {recipientEmails.map((email) => (
                <span key={email} className="email-chip">
                  {email}
                  <button
                    type="button"
                    className="email-chip-remove"
                    onClick={() => removeEmail(email)}
                    aria-label={`Remove ${email}`}
                  >
                    ×
                  </button>
                </span>
              ))}
              <input
                type="text"
                className="email-chip-input"
                value={emailInput}
                onChange={(e) => setEmailInput(e.target.value)}
                onKeyDown={handleEmailKeyDown}
                onBlur={commitEmailInput}
                placeholder={recipientEmails.length === 0 ? 'recipient@example.com' : ''}
              />
            </div>
            <small style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
              Press Enter, comma, or Tab to add. Recipients will receive the upload link via email.
            </small>
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
