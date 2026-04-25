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
        <NavLink to="/" end className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          Transfers
        </NavLink>
        <NavLink to="/transfers/new" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          New Transfer
        </NavLink>
        <NavLink to="/requests" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          File Requests
        </NavLink>
        <NavLink to="/settings" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
          Settings
        </NavLink>
        <div className="sidebar-spacer" />
        <div className="sidebar-user">{user?.email ?? user?.user_id?.slice(0, 8) + '…'}</div>
        <button className="btn-link" onClick={handleLogout}>Logout</button>
      </nav>
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
