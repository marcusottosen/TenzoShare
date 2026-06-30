import React, { useState, useMemo, useEffect } from 'react';
import type { Contact } from '../api/contacts';

interface ContactPickerModalProps {
  contacts: Contact[];
  onSelect: (email: string) => void;
  onClose: () => void;
  excludeEmails?: string[];
}

export function ContactPickerModal({ contacts, onSelect, onClose, excludeEmails = [] }: ContactPickerModalProps) {
  const [search, setSearch] = useState('');

  const filtered = useMemo(() => {
    const q = search.toLowerCase().trim();
    if (!q) return contacts.filter(c => !excludeEmails.includes(c.email));
    return contacts.filter(c => 
      !excludeEmails.includes(c.email) && (
        c.email.toLowerCase().includes(q) ||
        (c.name && c.name.toLowerCase().includes(q))
      )
    );
  }, [contacts, search, excludeEmails]);

  // Auto-focus search input
  useEffect(() => {
    const input = document.getElementById('contact-picker-search') as HTMLInputElement;
    if (input) input.focus();
  }, []);

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [onClose]);

  return (
    <div
      className="modal-overlay"
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
        padding: 16,
      }}
    >
      <div
        className="modal-content"
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'var(--color-card-bg, #fff)',
          borderRadius: 12,
          boxShadow: '0 20px 60px rgba(0,0,0,0.3)',
          width: '100%',
          maxWidth: 500,
          maxHeight: '80vh',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          padding: '20px 24px',
          borderBottom: '1px solid var(--color-border)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}>
          <div>
            <h3 style={{ margin: 0, fontSize: 18, fontWeight: 600, color: 'var(--color-text-primary)' }}>
              Select Contact
            </h3>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--color-text-muted)' }}>
              Choose from {contacts.length} saved contact{contacts.length !== 1 ? 's' : ''}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="btn btn-secondary btn-sm"
            style={{
              width: 32,
              height: 32,
              padding: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              borderRadius: '50%',
            }}
            aria-label="Close"
          >
            ✕
          </button>
        </div>

        {/* Search */}
        <div style={{ padding: '16px 24px', borderBottom: '1px solid var(--color-border)' }}>
          <div style={{ position: 'relative' }}>
            <span style={{
              position: 'absolute',
              left: 12,
              top: '50%',
              transform: 'translateY(-50%)',
              color: 'var(--color-text-muted)',
              pointerEvents: 'none',
            }}>
              🔍
            </span>
            <input
              id="contact-picker-search"
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by name or email..."
              style={{
                width: '100%',
                paddingLeft: 40,
              }}
            />
          </div>
        </div>

        {/* Contact list */}
        <div style={{
          flex: 1,
          overflowY: 'auto',
          padding: '8px 0',
        }}>
          {filtered.length === 0 ? (
            <div style={{
              padding: '32px 24px',
              textAlign: 'center',
              color: 'var(--color-text-muted)',
              fontSize: 14,
            }}>
              {search ? 'No contacts match your search' : 'No contacts saved yet'}
            </div>
          ) : (
            filtered.map((contact) => (
              <button
                key={contact.id}
                type="button"
                onClick={() => {
                  onSelect(contact.email);
                  onClose();
                }}
                style={{
                  width: '100%',
                  padding: '12px 24px',
                  border: 'none',
                  background: 'transparent',
                  textAlign: 'left',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                  transition: 'background 0.15s',
                }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-nav-active)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                }}
              >
                <div style={{
                  width: 40,
                  height: 40,
                  borderRadius: '50%',
                  background: 'var(--color-primary-light, rgba(99,102,241,0.1))',
                  color: 'var(--color-primary)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 16,
                  fontWeight: 600,
                  flexShrink: 0,
                }}>
                  {(contact.name || contact.email)[0].toUpperCase()}
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{
                    fontSize: 14,
                    fontWeight: 500,
                    color: 'var(--color-text-primary)',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}>
                    {contact.name || contact.email}
                  </div>
                  {contact.name && (
                    <div style={{
                      fontSize: 12,
                      color: 'var(--color-text-muted)',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      marginTop: 2,
                    }}>
                      {contact.email}
                    </div>
                  )}
                </div>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--color-text-muted)', flexShrink: 0 }}>
                  <polyline points="9 18 15 12 9 6"/>
                </svg>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
