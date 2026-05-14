import React, { useEffect, useState } from 'react';
import { listContacts, upsertContact, updateContact, deleteContact, type Contact } from '../api/contacts';

export default function ContactsPage() {
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Add form state
  const [addEmail, setAddEmail] = useState('');
  const [addName, setAddName] = useState('');
  const [addError, setAddError] = useState('');
  const [adding, setAdding] = useState(false);

  // Inline edit state: contactId → draft name
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setLoading(true);
    listContacts()
      .then(setContacts)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    const email = addEmail.trim();
    if (!email) return;
    setAddError('');
    setAdding(true);
    try {
      const ct = await upsertContact(email, addName.trim());
      setContacts((prev) => {
        const idx = prev.findIndex((c) => c.id === ct.id);
        if (idx >= 0) {
          const next = [...prev];
          next[idx] = ct;
          return next;
        }
        return [...prev, ct].sort((a, b) => a.email.localeCompare(b.email));
      });
      setAddEmail('');
      setAddName('');
    } catch (e: unknown) {
      setAddError((e as Error).message ?? 'Failed to add contact.');
    } finally {
      setAdding(false);
    }
  }

  async function handleSaveName(id: string) {
    setSaving(true);
    try {
      const ct = await updateContact(id, editName);
      setContacts((prev) => prev.map((c) => (c.id === id ? ct : c)));
      setEditingId(null);
    } catch (e: unknown) {
      setError((e as Error).message ?? 'Failed to update contact.');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteContact(id);
      setContacts((prev) => prev.filter((c) => c.id !== id));
    } catch (e: unknown) {
      setError((e as Error).message ?? 'Failed to delete contact.');
    }
  }

  return (
    <div className="page" style={{ maxWidth: 680 }}>
      <div className="page-header" style={{ marginBottom: 24 }}>
        <div>
          <h1 className="page-title">Contacts</h1>
          <p className="page-subtitle">Saved recipients for quick email lookup when sharing files.</p>
        </div>
      </div>

      {error && <div className="alert alert-error" style={{ marginBottom: 16 }}>{error}</div>}

      {/* ── Add contact form ─────────────────────────────── */}
      <div className="card" style={{ marginBottom: 20 }}>
        <div className="card-header"><h2 className="card-title">Add contact</h2></div>
        {addError && <div className="alert alert-error" style={{ marginBottom: 12 }}>{addError}</div>}
        <form onSubmit={handleAdd} style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'flex-end' }}>
          <div className="form-group" style={{ flex: '1 1 200px', marginBottom: 0 }}>
            <label>Email *</label>
            <input
              type="email"
              value={addEmail}
              onChange={(e) => setAddEmail(e.target.value)}
              placeholder="colleague@example.com"
              required
              maxLength={254}
            />
          </div>
          <div className="form-group" style={{ flex: '1 1 160px', marginBottom: 0 }}>
            <label>Name <span style={{ color: 'var(--color-text-muted)', fontWeight: 400 }}>(optional)</span></label>
            <input
              type="text"
              value={addName}
              onChange={(e) => setAddName(e.target.value)}
              placeholder="Alice Smith"
              maxLength={200}
            />
          </div>
          <button type="submit" className="btn btn-primary" disabled={adding} style={{ marginBottom: 1 }}>
            {adding ? 'Adding…' : 'Add'}
          </button>
        </form>
      </div>

      {/* ── Contact list ─────────────────────────────────── */}
      <div className="card">
        <div className="card-header"><h2 className="card-title">Your contacts</h2></div>
        {loading ? (
          <p style={{ fontSize: 13, color: 'var(--color-text-muted)' }}>Loading…</p>
        ) : contacts.length === 0 ? (
          <p style={{ fontSize: 13, color: 'var(--color-text-muted)' }}>
            No contacts yet. Add one above, or emails will be saved automatically when you create file shares or requests.
          </p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Name</th>
                  <th style={{ width: 120 }}></th>
                </tr>
              </thead>
              <tbody>
                {contacts.map((ct) => (
                  <tr key={ct.id}>
                    <td style={{ fontSize: 13 }}>{ct.email}</td>
                    <td>
                      {editingId === ct.id ? (
                        <input
                          autoFocus
                          type="text"
                          value={editName}
                          onChange={(e) => setEditName(e.target.value)}
                          maxLength={200}
                          style={{ fontSize: 13, padding: '2px 6px' }}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') { e.preventDefault(); handleSaveName(ct.id); }
                            if (e.key === 'Escape') setEditingId(null);
                          }}
                        />
                      ) : (
                        <span style={{ fontSize: 13, color: ct.name ? 'inherit' : 'var(--color-text-muted)' }}>
                          {ct.name || '—'}
                        </span>
                      )}
                    </td>
                    <td style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                      {editingId === ct.id ? (
                        <>
                          <button
                            type="button"
                            className="btn btn-primary btn-sm"
                            disabled={saving}
                            onClick={() => handleSaveName(ct.id)}
                          >
                            Save
                          </button>
                          <button
                            type="button"
                            className="btn btn-secondary btn-sm"
                            onClick={() => setEditingId(null)}
                          >
                            Cancel
                          </button>
                        </>
                      ) : (
                        <>
                          <button
                            type="button"
                            className="btn btn-secondary btn-sm"
                            onClick={() => { setEditingId(ct.id); setEditName(ct.name); }}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            className="btn btn-secondary btn-sm"
                            style={{ color: 'var(--color-danger, #dc2626)' }}
                            onClick={() => handleDelete(ct.id)}
                          >
                            Delete
                          </button>
                        </>
                      )}
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
