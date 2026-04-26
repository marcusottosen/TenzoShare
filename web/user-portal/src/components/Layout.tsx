import React, { useState } from 'react';
import { NavLink, useNavigate, Outlet, useLocation } from 'react-router';
import { useAuth } from '../stores/auth';
import { logout as apiLogout } from '../api/auth';

// ─── Lucide-style inline SVG icons ──────────────────────────────
function IconShare() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/>
      <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
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
function IconBell() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/>
    </svg>
  );
}
function IconSearch() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
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

// ─── Page title map ───────────────────────────────────────────────
const PAGE_TITLES: Record<string, string> = {
  '/': 'Dashboard',
  '/files': 'My Files',
  '/transfers/new': 'New Transfer',
  '/requests': 'Requests',
  '/settings': 'Settings',
};

function getInitials(email?: string): string {
  if (!email) return '?';
  const parts = email.split('@')[0].split(/[._-]/);
  return parts.length > 1
    ? (parts[0][0] + parts[1][0]).toUpperCase()
    : email.slice(0, 2).toUpperCase();
}

export default function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchQuery, setSearchQuery] = useState('');

  const pageTitle = PAGE_TITLES[location.pathname]
    ?? (location.pathname.startsWith('/transfers/') ? 'Transfer Details' : 'TenzoShare');

  async function handleLogout() {
    try { await apiLogout(); } catch { /* ignore */ }
    logout();
    navigate('/login');
  }

  const initials = getInitials(user?.email);

  return (
    <div className="app-shell">
      {/* ── Sidebar ─────────────────────────────────────────── */}
      <nav className="sidebar">
        <div className="sidebar-header">
          <div className="sidebar-logo">
            <IconTenz />
          </div>
          <span className="sidebar-title">TenzoShare</span>
        </div>

        <div className="sidebar-nav">
          <NavLink to="/" end className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconShare /> Transfers
          </NavLink>
          <NavLink to="/transfers/new" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconUpload /> Upload
          </NavLink>
          <NavLink to="/files" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconFolder /> My Files
          </NavLink>
          <NavLink to="/requests" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconInbox /> Requests
          </NavLink>

          <div className="sidebar-section-label">Account</div>
          <NavLink to="/settings" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconSettings /> Settings
          </NavLink>
        </div>

        <div className="sidebar-footer">
          <div className="sidebar-user-row">
            <div className="sidebar-avatar">{initials}</div>
            <div className="sidebar-user-info">
              <div className="sidebar-user-name">{user?.email ?? 'User'}</div>
              <div className="sidebar-user-role">Free plan</div>
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

        <div className="navbar-search">
          <IconSearch />
          <input
            type="text"
            placeholder="Search files…"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>

        <div className="navbar-actions">
          <button className="navbar-icon-btn" title="Notifications">
            <IconBell />
          </button>
          <button className="navbar-icon-btn" title="Settings" onClick={() => navigate('/settings')}>
            <IconSettings />
          </button>
          <div className="navbar-avatar" title={user?.email}>{initials}</div>
        </div>
      </header>

      {/* ── Main content ────────────────────────────────────── */}
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
