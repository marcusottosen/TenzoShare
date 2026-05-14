import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router';
import { login, storeTokens, getMe, resendVerificationEmail } from '../api/auth';
import { useAuth } from '../stores/auth';
import { getLogoUrl } from '../branding';
import { setTokens } from '../api/client';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [emailNotVerified, setEmailNotVerified] = useState(false);
  const [resendStatus, setResendStatus] = useState<'idle' | 'sending' | 'sent'>('idle');
  const { setUser } = useAuth();
  const navigate = useNavigate();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setEmailNotVerified(false);
    setResendStatus('idle');
    setLoading(true);
    try {
      const result = await login(email, password);
      if (result.mfa_required && result.user_id) {
        navigate('/login/mfa', { state: { userId: result.user_id } });
        return;
      }
      if (result.mfa_setup_required && result.access_token) {
        // Store the setup-only access token (no refresh token — intentional).
        // This token only unlocks /mfa/setup and /mfa/verify.
        setTokens(result.access_token, '');
        navigate('/profile', { state: { mfaSetupRequired: true } });
        return;
      }
      if (result.access_token && result.refresh_token) {
        storeTokens({ ...result } as any);
        const me = await getMe();
        setUser(me);
        navigate('/');
      }
    } catch (err: any) {
      if (err?.status === 403 && err?.message === 'email_not_verified') {
        setEmailNotVerified(true);
      } else {
        setError(err.message ?? 'Login failed');
      }
    } finally {
      setLoading(false);
    }
  }

  async function handleResend() {
    setResendStatus('sending');
    try {
      await resendVerificationEmail(email);
      setResendStatus('sent');
    } catch {
      setResendStatus('idle');
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-box">
        <div className="auth-logo">
          <div className="auth-logo-icon"><img src={getLogoUrl()} alt="TenzoShare" style={{ width: 28, height: 28, objectFit: 'contain' }} /></div>
          <span className="auth-logo-name">TenzoShare</span>
        </div>
        <h1>Welcome back</h1>
        <p className="auth-sub">Sign in to your account to continue</p>

        {error && <div className="alert alert-error">{error}</div>}

        {emailNotVerified && (
          <div className="alert alert-warning" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <span>Your email address has not been verified. Please check your inbox for a verification link.</span>
            <button
              type="button"
              className="btn btn-sm btn-outline"
              onClick={handleResend}
              disabled={resendStatus === 'sending' || resendStatus === 'sent'}
              style={{ alignSelf: 'flex-start' }}
            >
              {resendStatus === 'sending' ? 'Sending…' : resendStatus === 'sent' ? '✓ Verification email sent' : 'Resend verification email'}
            </button>
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Email address</label>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com" required autoFocus />
          </div>
          <div className="form-group">
            <div className="form-label-row">
              <label style={{ margin: 0 }}>Password</label>
              <Link to="/forgot-password" className="text-link" style={{ fontSize: 12 }}>Forgot password?</Link>
            </div>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="••••••••" required />
          </div>
          <button className="btn btn-primary btn-full btn-lg" type="submit" disabled={loading} style={{ marginTop: 8 }}>
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>

        <p style={{ textAlign: 'center', marginTop: 20, fontSize: 14, color: 'var(--color-text-muted)' }}>
          Don't have an account?{' '}
          <Link to="/register" className="text-link">Create one</Link>
        </p>
      </div>
    </div>
  );
}
