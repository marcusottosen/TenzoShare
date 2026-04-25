import React, { useState } from 'react';
import { useNavigate } from 'react-router';
import { login, storeTokens, getMe } from '../api/auth';
import { useAuth } from '../stores/auth';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const { setUser } = useAuth();
  const navigate = useNavigate();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const result = await login(email, password);
      if (result.mfa_required) {
        setError('MFA is required. Use the user portal to complete MFA login.');
        return;
      }
      if (result.access_token && result.refresh_token) {
        storeTokens({ ...result } as any);
        const me = await getMe();
        if (me.role !== 'admin') {
          setError('Access denied. You need admin role to use this portal.');
          return;
        }
        setUser(me);
        navigate('/');
      }
    } catch (err: any) {
      setError(err.message ?? 'Login failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-box">
        <h1>TenzoShare Admin</h1>
        <p className="subtitle">Admin access only</p>
        {error && <div className="alert alert-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Email</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
            />
          </div>
          <div className="form-group">
            <label>Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          <button className="btn btn-primary btn-full" type="submit" disabled={loading}>
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  );
}
