import React, { useState } from 'react';
import { Link, useSearchParams } from 'react-router';
import { requestPasswordReset, confirmPasswordReset } from '../api/auth';

function IconTenz() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
    </svg>
  );
}

export default function ForgotPasswordPage() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') ?? '';

  const [email, setEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleRequest(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const res = await requestPasswordReset(email);
      setMessage(res.message);
    } catch (err: any) {
      setError(err.message ?? 'Failed');
    } finally {
      setLoading(false);
    }
  }

  async function handleConfirm(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const res = await confirmPasswordReset(token, newPassword);
      setMessage(res.message + ' You can now sign in.');
    } catch (err: any) {
      setError(err.message ?? 'Failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-box">
        <div className="auth-logo">
          <div className="auth-logo-icon"><IconTenz /></div>
          <span className="auth-logo-name">TenzoShare</span>
        </div>

        {token ? (
          <>
            <h1>Set new password</h1>
            <p className="auth-sub">Choose a strong password for your account.</p>
            {error && <div className="alert alert-error">{error}</div>}
            {message ? (
              <div className="alert alert-success">{message} <Link to="/login" className="text-link">Sign in</Link></div>
            ) : (
              <form onSubmit={handleConfirm}>
                <div className="form-group">
                  <label>New password <span style={{ fontSize: 12, color: 'var(--color-text-muted)', fontWeight: 400 }}>(min 8 chars)</span></label>
                  <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="••••••••" minLength={8} required autoFocus />
                </div>
                <button className="btn btn-primary btn-full btn-lg" type="submit" disabled={loading} style={{ marginTop: 8 }}>
                  {loading ? 'Saving…' : 'Set password'}
                </button>
              </form>
            )}
          </>
        ) : (
          <>
            <h1>Reset password</h1>
            <p className="auth-sub">Enter your email and we'll send you a reset link.</p>
            {error && <div className="alert alert-error">{error}</div>}
            {message ? (
              <div className="alert alert-success">{message}</div>
            ) : (
              <form onSubmit={handleRequest}>
                <div className="form-group">
                  <label>Email address</label>
                  <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com" required autoFocus />
                </div>
                <button className="btn btn-primary btn-full btn-lg" type="submit" disabled={loading} style={{ marginTop: 8 }}>
                  {loading ? 'Sending…' : 'Send reset link'}
                </button>
              </form>
            )}
          </>
        )}

        <p style={{ textAlign: 'center', marginTop: 20, fontSize: 14, color: 'var(--color-text-muted)' }}>
          <Link to="/login" className="text-link">← Back to sign in</Link>
        </p>
      </div>
    </div>
  );
}
