import React, { useState } from 'react';
import { useNavigate, useLocation, Link } from 'react-router';
import { loginMFA, storeTokens, getMe } from '../api/auth';
import { useAuth } from '../stores/auth';

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
        <h1>Two-factor authentication</h1>
        <p className="text-sm mb-16">Enter the 6-digit code from your authenticator app.</p>
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
            />
          </div>
          <button className="btn btn-primary btn-full" type="submit" disabled={loading || code.length !== 6}>
            {loading ? 'Verifying…' : 'Verify'}
          </button>
        </form>
        <div className="mt-16 text-sm">
          <Link to="/login" className="text-link">Back to login</Link>
        </div>
      </div>
    </div>
  );
}
