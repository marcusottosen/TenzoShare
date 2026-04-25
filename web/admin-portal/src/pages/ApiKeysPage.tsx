import React, { useEffect, useState } from 'react';
import {
  listAPIKeys,
  createAPIKey,
  deleteAPIKey,
  type APIKey,
} from '../api/auth';

function fmt(date: string) {
  return new Date(date).toLocaleString();
}

// Clipboard helper — works on plain HTTP (no HTTPS required)
function copyText(text: string, onSuccess: () => void) {
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

export default function ApiKeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [newName, setNewName] = useState('');
  const [creating, setCreating] = useState(false);
  const [newSecret, setNewSecret] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);

  useEffect(() => {
    listAPIKeys()
      .then((res) => setKeys(res.keys ?? []))
      .catch((err: unknown) => setError((err as Error).message))
      .finally(() => setLoading(false));
  }, []);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!newName.trim()) return;
    setCreating(true);
    setError('');
    setNewSecret(null);
    try {
      const key = await createAPIKey(newName.trim());
      setKeys((prev) => [key, ...prev]);
      setNewName('');
      if (key.key) setNewSecret(key.key);
    } catch (err: unknown) {
      setError((err as Error).message);
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id: string) {
    if (!confirm('Delete this API key? It will immediately stop working.')) return;
    setDeleting(id);
    try {
      await deleteAPIKey(id);
      setKeys((prev) => prev.filter((k) => k.id !== id));
    } catch (err: unknown) {
      alert((err as Error).message);
    } finally {
      setDeleting(null);
    }
  }

  function handleCopySecret() {
    if (!newSecret) return;
    copyText(newSecret, () => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">API Keys</h1>
        <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
          Personal access tokens for automation, CI/CD, and scripted admin tasks
        </span>
      </div>

      <div className="card">
        <div className="card-title">Create new key</div>
        {error && <div className="alert alert-error">{error}</div>}

        {newSecret && (
          <div className="alert alert-success" style={{ marginBottom: 16 }}>
            <strong>Key created.</strong> Copy it now — it won't be shown again.
            <div className="row" style={{ marginTop: 8, gap: 8, alignItems: 'center' }}>
              <code style={{ flex: 1, fontSize: 12, wordBreak: 'break-all', background: 'rgba(0,0,0,0.05)', padding: '4px 8px', borderRadius: 4 }}>
                {newSecret}
              </code>
              <button className="btn btn-secondary btn-sm" onClick={handleCopySecret}>
                {copied ? 'Copied!' : 'Copy'}
              </button>
              <button className="btn btn-secondary btn-sm" onClick={() => setNewSecret(null)}>
                Dismiss
              </button>
            </div>
          </div>
        )}

        <form onSubmit={handleCreate} className="row" style={{ gap: 8, alignItems: 'flex-end' }}>
          <div className="form-group" style={{ flex: 1, margin: 0 }}>
            <label>Key name</label>
            <input
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="e.g. Monitoring script, CI pipeline"
              maxLength={100}
            />
          </div>
          <button
            className="btn btn-primary"
            type="submit"
            disabled={creating || !newName.trim()}
            style={{ flexShrink: 0 }}
          >
            {creating ? 'Creating…' : 'Create key'}
          </button>
        </form>
      </div>

      <div className="card">
        <div className="card-title">Active keys</div>
        {loading ? (
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>
        ) : keys.length === 0 ? (
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>No API keys yet.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Prefix</th>
                  <th>Created</th>
                  <th>Expires</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {keys.map((k) => (
                  <tr key={k.id}>
                    <td>{k.name}</td>
                    <td><code className="text-sm">{k.key_prefix}…</code></td>
                    <td className="text-sm">{fmt(k.created_at)}</td>
                    <td className="text-sm">{k.expires_at ? fmt(k.expires_at) : '—'}</td>
                    <td>
                      <button
                        className="btn btn-danger btn-sm"
                        onClick={() => handleDelete(k.id)}
                        disabled={deleting === k.id}
                      >
                        {deleting === k.id ? '…' : 'Revoke'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
