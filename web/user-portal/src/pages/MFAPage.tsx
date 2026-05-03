import React, { useState } from 'react';
import { useNavigate, useLocation, Link } from 'react-router';
import { loginMFA, storeTokens, getMe } from '../api/auth';
import { useAuth } from '../stores/auth';
import { getLogoUrl } from '../branding';

export default function MFAPage() {
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const { setUser } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const userId = (location.state as any)?.userId as string | undefined;

  if (!userId) {
    return (
      <div className="auth-page">
        <div className="auth-box">
          <div className="alert alert-error">No MFA session. Please <Link to="/login" className="text-link">sign in again</Link>.</div>
        </div>
      </div>
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const tokens = await loginMFA(userId!, code);
      storeTokens(tokens);
      const me = await getMe();
      setUser(me);
      navigate('/');
    } catch (err: any) {
      setError(err.message ?? 'Invalid code');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-box">
        <div className="auth-logo">
          <div className="auth-logo-icon"><img src={getLogoUrl()} alt="TenzoShare" style={{ width: 28, height: 28, objectFit: 'contain' }} /></div>
          <span className="auth-logo-name">TenzoShare</span>
        </div>
        <h1>Two-factor authentication</h1>
        <p className="auth-sub">Enter the 6-digit code from your authenticator app.</p>

        {error && <div className="alert alert-error">{error}</div>}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Authenticator code</label>
            <input
              type="text"
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              placeholder="000000"
              maxLength={6}
              required
              autoFocus
              style={{ letterSpacing: '0.3em', fontSize: 22, textAlign: 'center' }}
            />
          </div>
          <button className="btn btn-primary btn-full btn-lg" type="submit" disabled={loading || code.length !== 6} style={{ marginTop: 8 }}>
            {loading ? 'Verifying…' : 'Verify'}
          </button>
        </form>

        <p style={{ textAlign: 'center', marginTop: 20, fontSize: 14, color: 'var(--color-text-muted)' }}>
          <Link to="/login" className="text-link">← Back to sign in</Link>
        </p>
      </div>
    </div>
  );
}
