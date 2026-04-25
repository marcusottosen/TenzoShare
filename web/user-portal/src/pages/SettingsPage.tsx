import React, { useState } from 'react';
import { setupMFA, verifyMFA } from '../api/auth';

export default function SettingsPage() {
  const [mfaSetupData, setMfaSetupData] = useState<{
    secret: string;
    provisioning_uri: string;
  } | null>(null);
  const [otpCode, setOtpCode] = useState('');
  const [setupLoading, setSetupLoading] = useState(false);
  const [verifyLoading, setVerifyLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  async function handleSetupMFA() {
    setError('');
    setSetupLoading(true);
    try {
      const data = await setupMFA();
      setMfaSetupData(data);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSetupLoading(false);
    }
  }

  async function handleVerifyMFA(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setVerifyLoading(true);
    try {
      await verifyMFA(otpCode);
      setSuccess('MFA enabled successfully!');
      setMfaSetupData(null);
      setOtpCode('');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setVerifyLoading(false);
    }
  }

  return (
    <div className="page">
      <h1 className="page-title">Settings</h1>

      <div className="card">
        <div className="card-title">Two-Factor Authentication (TOTP)</div>

        {error && <div className="alert alert-error">{error}</div>}
        {success && <div className="alert alert-success">{success}</div>}

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
              {/* Render QR code as an image using Google Charts API (local dev only) */}
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
              <button
                className="btn btn-primary"
                type="submit"
                disabled={verifyLoading || otpCode.length !== 6}
              >
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
