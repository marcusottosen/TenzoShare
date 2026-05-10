import React, { useState, useEffect } from 'react';
import QRCode from 'qrcode';
import { useAuth } from '../stores/auth';
import { getMe, changePassword, setupMFA, verifyMFA, disableMFA } from '../api/auth';

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
  const [profile, setProfile] = useState<{ email: string; role: string; created_at?: string; mfa_enabled?: boolean } | null>(null);

  // Password change
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');
  const [pwSaving, setPwSaving] = useState(false);
  const [pwMsg, setPwMsg] = useState<{ ok: boolean; text: string } | null>(null);

  // MFA
  const [mfaSetupData, setMfaSetupData] = useState<{ secret: string; provisioning_uri: string } | null>(null);
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null);
  const [otpCode, setOtpCode] = useState('');
  const [disableOtp, setDisableOtp] = useState('');
  const [showDisablePanel, setShowDisablePanel] = useState(false);
  const [setupLoading, setSetupLoading] = useState(false);
  const [verifyLoading, setVerifyLoading] = useState(false);
  const [disableLoading, setDisableLoading] = useState(false);
  const [mfaError, setMfaError] = useState('');
  const [mfaSuccess, setMfaSuccess] = useState('');

  useEffect(() => {
    getMe().then((me) => {
      setProfile({ email: me.email, role: me.role, created_at: me.created_at, mfa_enabled: me.mfa_enabled });
      setUser(me);
    }).catch(() => {});
  }, []);

  async function handleSetupMFA() {
    setSetupLoading(true);
    setMfaError('');
    setMfaSuccess('');
    try {
      const data = await setupMFA();
      setMfaSetupData(data);
      QRCode.toDataURL(data.provisioning_uri).then(setQrDataUrl).catch(() => {});
    } catch (err: any) {
      setMfaError(err.message ?? 'Failed to start MFA setup.');
    } finally {
      setSetupLoading(false);
    }
  }

  async function handleVerifyMFA(e: React.FormEvent) {
    e.preventDefault();
    setVerifyLoading(true);
    setMfaError('');
    try {
      await verifyMFA(otpCode);
      setProfile((p) => p ? { ...p, mfa_enabled: true } : p);
      setMfaSetupData(null);
      setQrDataUrl(null);
      setOtpCode('');
      setMfaSuccess('MFA has been enabled on your account.');
    } catch (err: any) {
      setMfaError(err.message ?? 'Invalid code. Please try again.');
    } finally {
      setVerifyLoading(false);
    }
  }

  async function handleDisableMFA(e: React.FormEvent) {
    e.preventDefault();
    setDisableLoading(true);
    setMfaError('');
    try {
      await disableMFA(disableOtp);
      setProfile((p) => p ? { ...p, mfa_enabled: false } : p);
      setShowDisablePanel(false);
      setDisableOtp('');
      setMfaSuccess('MFA has been disabled.');
    } catch (err: any) {
      setMfaError(err.message ?? 'Invalid code. Please try again.');
    } finally {
      setDisableLoading(false);
    }
  }

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

      {/* Two-Factor Authentication */}
      <Section title="Two-Factor Authentication">
        {mfaError && <div className="alert alert-error" style={{ marginBottom: 12 }}>{mfaError}</div>}
        {mfaSuccess && <div className="alert alert-success" style={{ marginBottom: 12 }}>{mfaSuccess}</div>}

        {/* MFA enabled — show disable option */}
        {!mfaSetupData && profile?.mfa_enabled && (
          <div>
            <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginBottom: 16 }}>
              MFA is currently <strong style={{ color: 'var(--color-text-primary)' }}>enabled</strong> on your account.
              You will need your authenticator app at every login.
            </p>
            {!showDisablePanel ? (
              <button
                className="btn btn-secondary"
                onClick={() => { setShowDisablePanel(true); setMfaError(''); setMfaSuccess(''); }}
              >
                Disable MFA
              </button>
            ) : (
              <form onSubmit={handleDisableMFA} style={{ maxWidth: 320 }}>
                <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginBottom: 12 }}>
                  Enter a code from your authenticator app to confirm.
                </p>
                <div className="form-group">
                  <label>Authenticator code</label>
                  <input
                    type="text"
                    className="form-control"
                    value={disableOtp}
                    onChange={(e) => setDisableOtp(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    maxLength={6}
                    placeholder="000000"
                    autoFocus
                    style={{ width: 160 }}
                  />
                </div>
                <div style={{ display: 'flex', gap: 8 }}>
                  <button className="btn btn-danger" type="submit" disabled={disableLoading || disableOtp.length !== 6}>
                    {disableLoading ? 'Disabling…' : 'Confirm disable'}
                  </button>
                  <button type="button" className="btn btn-secondary" onClick={() => { setShowDisablePanel(false); setDisableOtp(''); setMfaError(''); }}>
                    Cancel
                  </button>
                </div>
              </form>
            )}
          </div>
        )}

        {/* MFA not enabled — show setup button */}
        {!mfaSetupData && !profile?.mfa_enabled && (
          <div>
            <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginBottom: 16 }}>
              Enable TOTP two-factor authentication to protect your admin account with an authenticator app.
            </p>
            <button className="btn btn-primary" onClick={handleSetupMFA} disabled={setupLoading}>
              {setupLoading ? 'Loading…' : 'Set up MFA'}
            </button>
          </div>
        )}

        {/* QR setup flow */}
        {mfaSetupData && (
          <div>
            <p style={{ fontSize: 13, color: 'var(--color-text-muted)', marginBottom: 16 }}>
              Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.),
              then enter the 6-digit code to confirm.
            </p>
            <div style={{ marginBottom: 16 }}>
              {qrDataUrl
                ? <img src={qrDataUrl} alt="MFA QR code" width={200} height={200} style={{ display: 'block', borderRadius: 8, border: '1px solid var(--color-border)' }} />
                : <div style={{ width: 200, height: 200, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-text-muted)', fontSize: 13, border: '1px solid var(--color-border)', borderRadius: 8 }}>Generating…</div>
              }
            </div>
            <div className="form-group">
              <label>Secret key (manual entry)</label>
              <input
                type="text"
                className="form-control"
                value={mfaSetupData.secret}
                readOnly
                style={{ fontFamily: 'monospace', maxWidth: 320 }}
              />
            </div>
            <form onSubmit={handleVerifyMFA} style={{ maxWidth: 320 }}>
              <div className="form-group">
                <label>Authenticator code</label>
                <input
                  type="text"
                  className="form-control"
                  value={otpCode}
                  onChange={(e) => setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  maxLength={6}
                  placeholder="000000"
                  autoFocus
                />
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <button className="btn btn-primary" type="submit" disabled={verifyLoading || otpCode.length !== 6}>
                  {verifyLoading ? 'Verifying…' : 'Enable MFA'}
                </button>
                <button type="button" className="btn btn-secondary" onClick={() => { setMfaSetupData(null); setQrDataUrl(null); setOtpCode(''); setMfaError(''); }}>
                  Cancel
                </button>
              </div>
            </form>
          </div>
        )}
      </Section>
    </div>
  );
}
