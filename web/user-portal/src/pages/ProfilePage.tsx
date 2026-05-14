import React, { useEffect, useState } from 'react';
import { useLocation } from 'react-router';
import QRCode from 'qrcode';
import { fmt } from '../utils/dateFormat';
import {
  getMe,
  setupMFA,
  verifyMFA,
  disableMFA,
  changePassword,
  storeTokens,
  type MeResponse,
} from '../api/auth';
import { useAuth } from '../stores/auth';

export default function ProfilePage() {
  const location = useLocation();
  const { setUser } = useAuth();
  const [profile, setProfile] = useState<MeResponse | null>(null);
  const [profileLoading, setProfileLoading] = useState(true);

  // MFA state
  const [mfaSetupData, setMfaSetupData] = useState<{ secret: string; provisioning_uri: string } | null>(null);
  const [qrDataUrl, setQrDataUrl] = useState('');
  const [otpCode, setOtpCode] = useState('');
  const [setupLoading, setSetupLoading] = useState(false);
  const [verifyLoading, setVerifyLoading] = useState(false);
  const [mfaError, setMfaError] = useState('');
  const [mfaSuccess, setMfaSuccess] = useState('');
  // Disable MFA state
  const [showDisablePanel, setShowDisablePanel] = useState(false);
  const [disableOtp, setDisableOtp] = useState('');
  const [disableLoading, setDisableLoading] = useState(false);

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

  // If navigated here with mfaSetupRequired=true (from login), auto-start setup.
  useEffect(() => {
    if ((location.state as any)?.mfaSetupRequired && profile && !profile.mfa_enabled) {
      handleSetupMFA();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profile]);

  // Generate QR code locally whenever setup data changes
  useEffect(() => {
    if (mfaSetupData?.provisioning_uri) {
      QRCode.toDataURL(mfaSetupData.provisioning_uri, { width: 200, margin: 2 })
        .then(setQrDataUrl)
        .catch(() => setQrDataUrl(''));
    } else {
      setQrDataUrl('');
    }
  }, [mfaSetupData]);

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
      const result = await verifyMFA(otpCode);
      // verifyMFA now returns full tokens — store them so the session becomes
      // fully authenticated (replacing the setup-only token if present).
      if (result.access_token && result.refresh_token) {
        storeTokens(result as any);
        const me = await getMe();
        setUser(me);
        setProfile(me);
      } else {
        setProfile((p) => p ? { ...p, mfa_enabled: true } : p);
      }
      setMfaSuccess('MFA enabled successfully!');
      setMfaSetupData(null);
      setOtpCode('');
    } catch (err: unknown) {
      setMfaError((err as Error).message);
    } finally {
      setVerifyLoading(false);
    }
  }

  async function handleDisableMFA(e: React.FormEvent) {
    e.preventDefault();
    setMfaError('');
    setDisableLoading(true);
    try {
      await disableMFA(disableOtp);
      setMfaSuccess('MFA has been disabled.');
      setShowDisablePanel(false);
      setDisableOtp('');
      setProfile((p) => p ? { ...p, mfa_enabled: false } : p);
    } catch (err: unknown) {
      setMfaError((err as Error).message);
    } finally {
      setDisableLoading(false);
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
          <h1 className="page-title">Profile</h1>
          <p className="page-subtitle">Manage your account info, password and two-factor authentication</p>
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

        {/* MFA already enabled — show disable option */}
        {!mfaSetupData && profile?.mfa_enabled && (
          <div>
            <p className="text-sm mb-16">
              MFA is currently <strong>enabled</strong> on your account. You will need your authenticator app at every login.
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
                <p className="text-sm mb-16" style={{ color: 'var(--color-text-muted)' }}>
                  Enter a code from your authenticator app to confirm you want to disable MFA.
                </p>
                <div className="form-group">
                  <label>Authenticator code</label>
                  <input
                    type="text"
                    value={disableOtp}
                    onChange={(e) => setDisableOtp(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    maxLength={6}
                    placeholder="000000"
                    autoFocus
                    style={{ width: 160 }}
                  />
                </div>
                <div style={{ display: 'flex', gap: 8 }}>
                  <button
                    className="btn btn-danger"
                    type="submit"
                    disabled={disableLoading || disableOtp.length !== 6}
                  >
                    {disableLoading ? 'Disabling…' : 'Confirm disable'}
                  </button>
                  <button
                    type="button"
                    className="btn btn-secondary"
                    onClick={() => { setShowDisablePanel(false); setDisableOtp(''); setMfaError(''); }}
                  >
                    Cancel
                  </button>
                </div>
              </form>
            )}
          </div>
        )}

        {/* MFA not enabled — show setup flow */}
        {!mfaSetupData && !profile?.mfa_enabled && (
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
        )}

        {/* QR setup flow */}
        {mfaSetupData && (
          <div>
            <p className="text-sm mb-16">
              Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.),
              then enter the 6-digit code to confirm.
            </p>
            <div className="card" style={{ display: 'inline-block', marginBottom: 16 }}>
              {qrDataUrl
                ? <img src={qrDataUrl} alt="MFA QR code" width={200} height={200} />
                : <div style={{ width: 200, height: 200, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-text-muted)', fontSize: 13 }}>Generating…</div>
              }
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
