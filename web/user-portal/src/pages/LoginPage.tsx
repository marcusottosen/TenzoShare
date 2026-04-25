import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router';
import { login, storeTokens } from '../api/auth';
import { useAuth } from '../stores/auth';
import { getMe } from '../api/auth';

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
      if (result.mfa_required && result.user_id) {
        navigate('/login/mfa', { state: { userId: result.user_id } });
        return;
      }
      if (result.access_token && result.refresh_token) {
        storeTokens({ ...result } as any);
        const me = await getMe();
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
        <h1>Sign in to TenzoShare</h1>
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
        <div className="mt-16 text-sm">
          <Link to="/forgot-password" className="text-link">Forgot password?</Link>
          {' · '}
          <Link to="/register" className="text-link">Create account</Link>
        </div>
      </div>
    </div>
  );
}
