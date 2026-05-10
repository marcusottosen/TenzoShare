import React, { useState, useEffect } from 'react';
import { useAuth } from '../stores/auth';
import { getMe, changePassword } from '../api/auth';

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="card" style={{ marginBottom: 24 }}>
      <div style={{ marginBottom: 20 }}>
        <h2 style={{ fontSize: 16, fontWeight: 600, color: 'var(--color-text-primary)', margin: 0 }}>{title}</h2>
      </div>
      {children}
    </div>
  );
}

function FieldRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center',
      padding: '12px 0', borderBottom: '1px solid var(--color-border)',
      gap: 16,
    }}>
      <div style={{ width: 160, fontSize: 13, color: 'var(--color-text-muted)', flexShrink: 0 }}>{label}</div>
      <div style={{ fontSize: 14, color: 'var(--color-text-primary)', fontWeight: 500 }}>{value}</div>
    </div>
  );
}

export default function AccountPage() {
  const { user, setUser } = useAuth();

  // Profile
  const [profile, setProfile] = useState<{ email: string; role: string; created_at?: string } | null>(null);

  // Password change
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');
  const [pwSaving, setPwSaving] = useState(false);
  const [pwMsg, setPwMsg] = useState<{ ok: boolean; text: string } | null>(null);

  useEffect(() => {
    getMe().then((me) => {
      setProfile({ email: me.email, role: me.role, created_at: me.created_at });
      setUser(me);
    }).catch(() => {});
  }, []);

  async function handlePasswordChange(e: React.FormEvent) {
    e.preventDefault();
    setPwMsg(null);
    if (newPw !== confirmPw) {
      setPwMsg({ ok: false, text: 'New passwords do not match.' });
      return;
    }
    if (newPw.length < 8) {
      setPwMsg({ ok: false, text: 'Password must be at least 8 characters.' });
      return;
    }
    setPwSaving(true);
    try {
      await changePassword(currentPw, newPw);
      setPwMsg({ ok: true, text: 'Password changed successfully.' });
      setCurrentPw(''); setNewPw(''); setConfirmPw('');
    } catch (err: any) {
      setPwMsg({ ok: false, text: err.message ?? 'Failed to change password.' });
    } finally {
      setPwSaving(false);
    }
  }

  const email = profile?.email ?? user?.email ?? '';
  const role = profile?.role ?? user?.role ?? 'admin';
  const memberSince = profile?.created_at
    ? new Date(profile.created_at).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })
    : '—';

  function getInitials(e: string) {
    const parts = e.split('@')[0].split(/[._-]/);
    return parts.length > 1
      ? (parts[0][0] + parts[1][0]).toUpperCase()
      : e.slice(0, 2).toUpperCase();
  }

  return (
    <div style={{ maxWidth: 700 }}>

      {/* Profile card */}
      <Section title="Profile">
        <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 24 }}>
          <div style={{
            width: 56, height: 56, borderRadius: '50%',
            background: 'var(--color-secondary)', color: '#fff',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 20, fontWeight: 700, flexShrink: 0,
          }}>
            {getInitials(email)}
          </div>
          <div>
            <div style={{ fontSize: 17, fontWeight: 600, color: 'var(--color-text-primary)' }}>{email}</div>
            <div style={{ fontSize: 13, color: 'var(--color-text-muted)', marginTop: 2, textTransform: 'capitalize' }}>{role}</div>
          </div>
        </div>
        <FieldRow label="Email" value={email} />
        <FieldRow label="Role" value={
          <span className="badge badge-blue" style={{ textTransform: 'capitalize' }}>{role}</span>
        } />
        <FieldRow label="Member since" value={memberSince} />
      </Section>

      {/* Change password */}
      <Section title="Change Password">
        <form onSubmit={handlePasswordChange}>
          <div className="form-group">
            <label>Current Password</label>
            <input
              type="password"
              className="form-control"
              value={currentPw}
              onChange={(e) => setCurrentPw(e.target.value)}
              required
              autoComplete="current-password"
            />
          </div>
          <div className="form-group">
            <label>New Password</label>
            <input
              type="password"
              className="form-control"
              value={newPw}
              onChange={(e) => setNewPw(e.target.value)}
              required
              minLength={8}
              autoComplete="new-password"
            />
          </div>
          <div className="form-group">
            <label>Confirm New Password</label>
            <input
              type="password"
              className="form-control"
              value={confirmPw}
              onChange={(e) => setConfirmPw(e.target.value)}
              required
              autoComplete="new-password"
            />
          </div>
          {pwMsg && (
            <div className={`alert ${pwMsg.ok ? 'alert-success' : 'alert-error'}`} style={{ marginBottom: 12 }}>
              {pwMsg.text}
            </div>
          )}
          <button className="btn btn-primary" type="submit" disabled={pwSaving}>
            {pwSaving ? 'Saving…' : 'Update Password'}
          </button>
        </form>
      </Section>
    </div>
  );
}
