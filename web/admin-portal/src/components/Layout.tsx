import React from 'react';
import { NavLink, useNavigate, Outlet } from 'react-router';
import { useAuth } from '../stores/auth';
import { logout as apiLogout } from '../api/auth';

export default function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  async function handleLogout() {
    try { await apiLogout(); } catch { /* ignore */ }
    logout();
    navigate('/login');
  }

  return (
    <div className="app-shell">
      <nav className="sidebar">
        <div className="sidebar-title">TenzoShare</div>
        <div className="sidebar-subtitle">ADMIN</div>
        <NavLink to="/" end className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          Dashboard
        </NavLink>
        <NavLink to="/users" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          Users
        </NavLink>
        <NavLink to="/transfers" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          Transfers
        </NavLink>
        <NavLink to="/audit" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          Audit Logs
        </NavLink>
        <NavLink to="/apikeys" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          API Keys
        </NavLink>
        <div className="sidebar-spacer" />
        <div className="sidebar-user">{user?.user_id?.slice(0, 8)}… ({user?.role})</div>
        <button className="btn-link" onClick={handleLogout}>Logout</button>
      </nav>
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
