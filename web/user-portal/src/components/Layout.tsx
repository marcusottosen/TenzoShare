import React from 'react';
import { NavLink, useNavigate, Outlet, useLocation } from 'react-router';
import { useAuth } from '../stores/auth';
import { logout as apiLogout } from '../api/auth';

function IconGrid() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/>
      <rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/>
    </svg>
  );
}
function IconUpload() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="16 16 12 12 8 16"/><line x1="12" y1="12" x2="12" y2="21"/>
      <path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/>
    </svg>
  );
}
function IconFolder() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
    </svg>
  );
}
function IconShare() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/>
      <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
    </svg>
  );
}
function IconInbox() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/>
      <path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>
    </svg>
  );
}
function IconSettings() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3"/>
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
    </svg>
  );
}
function IconLogOut() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/>
    </svg>
  );
}
function IconTenz() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
    </svg>
  );
}

const PAGE_TITLES: Record<string, string> = {
  '/': 'Dashboard',
  '/files': 'My Files',
  '/transfers/new': 'Upload',
  '/requests': 'File Requests',
  '/shares': 'Shares & Requests',
  '/settings': 'Settings',
  '/profile': 'Profile',
};

function getInitials(email?: string): string {
  if (!email) return '?';
  const parts = email.split('@')[0].split(/[._-]/);
  return parts.length > 1
    ? (parts[0][0] + parts[1][0]).toUpperCase()
    : email.slice(0, 2).toUpperCase();
}

function getDisplayName(email?: string): string {
  if (!email) return 'User';
  const local = email.split('@')[0];
  const parts = local.split(/[._-]/);
  return parts.map((p) => p.charAt(0).toUpperCase() + p.slice(1)).join(' ');
}

export default function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();


  const pageTitle = PAGE_TITLES[location.pathname]
    ?? (location.pathname.startsWith('/transfers/') ? 'Transfer Details' : 'TenzoShare');

  async function handleLogout() {
    try { await apiLogout(); } catch { /* ignore */ }
    logout();
    navigate('/login');
  }

  const initials = getInitials(user?.email);
  const displayName = getDisplayName(user?.email);

  return (
    <div className="app-shell">
      {/* ── Sidebar ─────────────────────────────────────────── */}
      <nav className="sidebar">
        <div className="sidebar-header">
          <div className="sidebar-logo">
            <IconTenz />
          </div>
          <div>
            <div className="sidebar-title">TenzoShare</div>
            <div className="sidebar-subtitle">User Portal</div>
          </div>
        </div>

        <div className="sidebar-nav">
          <div className="sidebar-section-label">Overview</div>
          <NavLink to="/" end className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconGrid /> Dashboard
          </NavLink>

          <div className="sidebar-section-label">Transfer</div>
          <NavLink to="/transfers/new" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconUpload /> Upload
          </NavLink>
          <NavLink to="/requests" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconInbox /> File Requests
          </NavLink>

          <div className="sidebar-section-label">Library</div>
          <NavLink to="/files" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconFolder /> My Files
          </NavLink>
          <NavLink to="/shares" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconShare /> Shares &amp; Requests
          </NavLink>

          <div className="sidebar-section-label">Account</div>
          <NavLink to="/settings" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconSettings /> Settings
          </NavLink>
        </div>

        <div className="sidebar-footer">
          <div className="sidebar-user-row" onClick={() => navigate('/profile')} style={{ cursor: 'pointer' }}>
            <div className="sidebar-avatar">{initials}</div>
            <div className="sidebar-user-info">
              <div className="sidebar-user-name">{displayName}</div>
              <div className="sidebar-user-role">{user?.email ?? ''}</div>
            </div>
          </div>
          <button className="sidebar-logout-btn" onClick={handleLogout}>
            <IconLogOut /> Sign out
          </button>
        </div>
      </nav>

      {/* ── Navbar ──────────────────────────────────────────── */}
      <header className="navbar">
        <div className="navbar-breadcrumb">{pageTitle}</div>

        <div className="navbar-avatar" onClick={() => navigate('/profile')} style={{ cursor: 'pointer' }}>{initials}</div>
      </header>

      {/* ── Main content ────────────────────────────────────── */}
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
