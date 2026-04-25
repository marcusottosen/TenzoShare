import React, { useState } from 'react';
import { Link, useSearchParams } from 'react-router';
import { requestPasswordReset, confirmPasswordReset } from '../api/auth';

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
        {token ? (
          <>
            <h1>Set new password</h1>
            {error && <div className="alert alert-error">{error}</div>}
            {message ? (
              <div className="alert alert-success">{message} <Link to="/login" className="text-link">Sign in</Link></div>
            ) : (
              <form onSubmit={handleConfirm}>
                <div className="form-group">
                  <label>New password (min 8 chars)</label>
                  <input
                    type="password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    minLength={8}
                    required
                    autoFocus
                  />
                </div>
                <button className="btn btn-primary btn-full" type="submit" disabled={loading}>
                  {loading ? 'Saving…' : 'Set password'}
                </button>
              </form>
            )}
          </>
        ) : (
          <>
            <h1>Reset password</h1>
            {error && <div className="alert alert-error">{error}</div>}
            {message ? (
              <div className="alert alert-success">{message}</div>
            ) : (
              <form onSubmit={handleRequest}>
                <div className="form-group">
                  <label>Email address</label>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                    autoFocus
                  />
                </div>
                <button className="btn btn-primary btn-full" type="submit" disabled={loading}>
                  {loading ? 'Sending…' : 'Send reset link'}
                </button>
              </form>
            )}
          </>
        )}
        <div className="mt-16 text-sm">
          <Link to="/login" className="text-link">Back to login</Link>
        </div>
      </div>
    </div>
  );
}
