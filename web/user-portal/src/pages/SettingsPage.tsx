import React, { useEffect, useState } from 'react';
import {
  getMe,
  setupMFA,
  verifyMFA,
  changePassword,
  type MeResponse,
} from '../api/auth';

function fmt(date: string) {
  return new Date(date).toLocaleString();
}

export default function SettingsPage() {
  const [profile, setProfile] = useState<MeResponse | null>(null);
  const [profileLoading, setProfileLoading] = useState(true);

  // MFA state
  const [mfaSetupData, setMfaSetupData] = useState<{ secret: string; provisioning_uri: string } | null>(null);
  const [otpCode, setOtpCode] = useState('');
  const [setupLoading, setSetupLoading] = useState(false);
  const [verifyLoading, setVerifyLoading] = useState(false);
  const [mfaError, setMfaError] = useState('');
  const [mfaSuccess, setMfaSuccess] = useState('');

  // Password change state
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [pwLoading, setPwLoading] = useState(false);
  const [pwError, setPwError] = useState('');
  const [pwSuccess, setPwSuccess] = useState('');

  useEffect(() => {
    getMe().then(setProfile).catch(() => null).finally(() => setProfileLoading(false));
  }, []);

  // --- MFA ---
  async function handleSetupMFA() {
    setMfaError('');
    setMfaSuccess('');
    setSetupLoading(true);
    try {
      const data = await setupMFA();
      setMfaSetupData(data);
    } catch (err: unknown) {
      setMfaError((err as Error).message);
    } finally {
      setSetupLoading(false);
    }
  }

  async function handleVerifyMFA(e: React.FormEvent) {
    e.preventDefault();
    setMfaError('');
    setVerifyLoading(true);
    try {
      await verifyMFA(otpCode);
      setMfaSuccess('MFA enabled successfully!');
      setMfaSetupData(null);
      setOtpCode('');
      setProfile((p) => p ? { ...p, mfa_enabled: true } : p);
    } catch (err: unknown) {
      setMfaError((err as Error).message);
    } finally {
      setVerifyLoading(false);
    }
  }

  // --- Password change ---
  async function handleChangePassword(e: React.FormEvent) {
    e.preventDefault();
    if (newPassword !== confirmPassword) {
      setPwError('New passwords do not match.');
      return;
    }
    if (newPassword.length < 8) {
      setPwError('New password must be at least 8 characters.');
      return;
    }
    setPwError('');
    setPwSuccess('');
    setPwLoading(true);
    try {
      await changePassword(currentPassword, newPassword);
      setPwSuccess('Password changed. Please log in again with your new password.');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err: unknown) {
      setPwError((err as Error).message);
    } finally {
      setPwLoading(false);
    }
  }



  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="page-subtitle">Manage your account and preferences</p>
        </div>
      </div>

      {/* Profile */}
      <div className="card">
        <div className="card-header"><h2 className="card-title">Account</h2></div>
        {profileLoading ? (
          <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>
        ) : profile ? (
          <table style={{ width: 'auto', border: 'none' }}>
            <tbody>
              {profile.email && (
                <tr>
                  <td style={{ paddingLeft: 0, fontWeight: 500, width: 140, paddingBottom: 8 }}>Email</td>
                  <td style={{ paddingLeft: 0, paddingBottom: 8 }}>{profile.email}</td>
                </tr>
              )}
              <tr>
                <td style={{ paddingLeft: 0, fontWeight: 500, paddingBottom: 8 }}>Role</td>
                <td style={{ paddingLeft: 0, paddingBottom: 8 }}>
                  <span className="badge badge-gray">{profile.role}</span>
                </td>
              </tr>
              {profile.email_verified !== undefined && (
                <tr>
                  <td style={{ paddingLeft: 0, fontWeight: 500, paddingBottom: 8 }}>Email verified</td>
                  <td style={{ paddingLeft: 0, paddingBottom: 8 }}>
                    {profile.email_verified ? (
                      <span className="badge badge-green">Yes</span>
                    ) : (
                      <span className="badge badge-red">No</span>
                    )}
                  </td>
                </tr>
              )}
              {profile.mfa_enabled !== undefined && (
                <tr>
                  <td style={{ paddingLeft: 0, fontWeight: 500, paddingBottom: 8 }}>2FA (TOTP)</td>
                  <td style={{ paddingLeft: 0, paddingBottom: 8 }}>
                    {profile.mfa_enabled ? (
                      <span className="badge badge-green">Enabled</span>
                    ) : (
                      <span className="badge badge-gray">Disabled</span>
                    )}
                  </td>
                </tr>
              )}
              {profile.created_at && (
                <tr>
                  <td style={{ paddingLeft: 0, fontWeight: 500, paddingBottom: 8 }}>Member since</td>
                  <td style={{ paddingLeft: 0, paddingBottom: 8 }}>{fmt(profile.created_at)}</td>
                </tr>
              )}
              <tr>
                <td style={{ paddingLeft: 0, fontWeight: 500 }}>User ID</td>
                <td style={{ paddingLeft: 0, fontFamily: 'monospace', fontSize: 13 }}>
                  {profile.id ?? profile.user_id}
                </td>
              </tr>
            </tbody>
          </table>
        ) : null}
      </div>

      {/* Change password */}
      <div className="card">
        <div className="card-header"><h2 className="card-title">Change password</h2></div>
        {pwError && <div className="alert alert-error">{pwError}</div>}
        {pwSuccess && <div className="alert alert-success">{pwSuccess}</div>}
        <form onSubmit={handleChangePassword}>
          <div className="form-group">
            <label>Current password</label>
            <input
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
              autoComplete="current-password"
            />
          </div>
          <div className="form-group">
            <label>New password</label>
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
              minLength={8}
              autoComplete="new-password"
            />
          </div>
          <div className="form-group">
            <label>Confirm new password</label>
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              autoComplete="new-password"
            />
          </div>
          <button className="btn btn-primary" type="submit" disabled={pwLoading}>
            {pwLoading ? 'Saving…' : 'Change password'}
          </button>
        </form>
      </div>

      {/* Two-Factor Authentication */}
      <div className="card">
        <div className="card-header"><h2 className="card-title">Two-Factor Authentication</h2></div>
        {mfaError && <div className="alert alert-error">{mfaError}</div>}
        {mfaSuccess && <div className="alert alert-success">{mfaSuccess}</div>}
        {!mfaSetupData ? (
          <div>
            <p className="text-sm mb-16">
              Enable TOTP MFA to require an authenticator app code at login.
            </p>
            <button
              className="btn btn-primary"
              onClick={handleSetupMFA}
              disabled={setupLoading}
            >
              {setupLoading ? 'Loading…' : 'Set up MFA'}
            </button>
          </div>
        ) : (
          <div>
            <p className="text-sm mb-16">
              Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.),
              then enter the 6-digit code to confirm.
            </p>
            <div className="card" style={{ display: 'inline-block', marginBottom: 16 }}>
              <img
                src={`https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=${encodeURIComponent(mfaSetupData.provisioning_uri)}`}
                alt="MFA QR code"
                width={180}
                height={180}
              />
            </div>
            <div className="form-group">
              <label>Secret key (manual entry)</label>
              <input
                type="text"
                value={mfaSetupData.secret}
                readOnly
                style={{ fontFamily: 'monospace', background: '#f5f5f5' }}
              />
            </div>
            <form onSubmit={handleVerifyMFA}>
              <div className="form-group">
                <label>Enter 6-digit code to confirm</label>
                <input
                  type="text"
                  value={otpCode}
                  onChange={(e) => setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  maxLength={6}
                  placeholder="000000"
                  autoFocus
                  style={{ width: 160 }}
                />
              </div>
              <button className="btn btn-primary" type="submit" disabled={verifyLoading || otpCode.length !== 6}>
                {verifyLoading ? 'Verifying…' : 'Enable MFA'}
              </button>
              <button
                type="button"
                className="btn btn-secondary"
                style={{ marginLeft: 8 }}
                onClick={() => setMfaSetupData(null)}
              >
                Cancel
              </button>
            </form>
          </div>
        )}
      </div>

    </div>
  );
}
