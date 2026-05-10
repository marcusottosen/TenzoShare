import React, { useState } from 'react';
import { useNavigate, useLocation, Link } from 'react-router';
import { loginMFA, storeTokens, getMe } from '../api/auth';
import { useAuth } from '../stores/auth';

export default function MFALoginPage() {
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
          <div className="alert alert-error">
            No MFA session found. Please <Link to="/login" className="text-link">sign in again</Link>.
          </div>
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
      if (me.role !== 'admin') {
        setError('Access denied. You need admin role to use this portal.');
        return;
      }
      setUser(me);
      navigate('/');
    } catch (err: any) {
      setError(err.message ?? 'Invalid code. Please try again.');
      setCode('');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-box">
        <div className="auth-logo">
          <div className="auth-logo-icon">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
            </svg>
          </div>
          <div>
            <div className="auth-logo-name">TenzoAdmin</div>
            <div className="auth-logo-sub">Admin Portal</div>
          </div>
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
          <button
            className="btn btn-primary btn-full"
            type="submit"
            disabled={loading || code.length !== 6}
            style={{ marginTop: 8 }}
          >
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
