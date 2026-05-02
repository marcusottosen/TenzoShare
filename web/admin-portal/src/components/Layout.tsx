import React, { useState, useEffect } from 'react';
import { NavLink, useNavigate, Outlet, useLocation } from 'react-router';
import { useAuth } from '../stores/auth';
import { logout as apiLogout } from '../api/auth';

/* ── Icons ──────────────────────────────────────────────────── */
function IconDashboard() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/>
      <rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/>
    </svg>
  );
}
function IconUsers() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
      <circle cx="9" cy="7" r="4"/>
      <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
      <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
    </svg>
  );
}
function IconTransfer() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/>
      <polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/>
    </svg>
  );
}
function IconAudit() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
      <polyline points="14 2 14 8 20 8"/>
      <line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>
      <polyline points="10 9 9 9 8 9"/>
    </svg>
  );
}
function IconKey() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/>
    </svg>
  );
}
function IconLogOut() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
      <polyline points="16 17 21 12 16 7"/>
      <line x1="21" y1="12" x2="9" y2="12"/>
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
function IconStorage() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <ellipse cx="12" cy="5" rx="9" ry="3"/>
      <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
      <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
    </svg>
  );
}
function IconClock() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10"/>
      <polyline points="12 6 12 12 16 14"/>
    </svg>
  );
}
function IconLock() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
      <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
    </svg>
  );
}
function IconPalette() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="13.5" cy="6.5" r=".5" fill="currentColor"/>
      <circle cx="17.5" cy="10.5" r=".5" fill="currentColor"/>
      <circle cx="8.5" cy="7.5" r=".5" fill="currentColor"/>
      <circle cx="6.5" cy="12.5" r=".5" fill="currentColor"/>
      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10c.28 0 .5-.22.5-.5 0-.16-.08-.28-.14-.35-.41-.46-.63-1.05-.63-1.65 0-1.38 1.12-2.5 2.5-2.5H16c2.76 0 5-2.24 5-5 0-4.42-4.03-8-9-8z"/>
    </svg>
  );
}

const PAGE_TITLES: Record<string, string> = {
  '/': 'System Overview',
  '/users': 'User Management',
  '/transfers': 'Transfers',
  '/audit': 'Audit Logs',
  '/audit/settings': 'Log Retention',
  '/security': 'Security Settings',
  '/apikeys': 'API Keys',
  '/storage': 'Storage Settings',
  '/storage/files': 'Storage Files',
  '/storage/insights': 'Storage Insights',
  '/branding': 'Branding',
};

function getInitials(email?: string): string {
  if (!email) return 'A';
  const parts = email.split('@')[0].split(/[._-]/);
  return parts.length > 1
    ? (parts[0][0] + parts[1][0]).toUpperCase()
    : email.slice(0, 2).toUpperCase();
}

function getDisplayName(email?: string): string {
  if (!email) return 'Admin';
  const local = email.split('@')[0];
  const parts = local.split(/[._-]/);
  return parts.map((p) => p.charAt(0).toUpperCase() + p.slice(1)).join(' ');
}

function IconMenu() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/>
    </svg>
  );
}

export default function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => { setSidebarOpen(false); }, [location.pathname]);

  const pageTitle = PAGE_TITLES[location.pathname] ?? 'Admin';

  async function handleLogout() {
    try { await apiLogout(); } catch { /* ignore */ }
    logout();
    navigate('/login');
  }

  const initials = getInitials(user?.email);
  const displayName = getDisplayName(user?.email);

  return (
    <div className="app-shell">
      {/* ── Sidebar overlay (mobile) ─────────────────────── */}
      {sidebarOpen && <div className="sidebar-overlay" onClick={() => setSidebarOpen(false)} />}

      {/* ── Dark navy sidebar ────────────────────────────────── */}
      <nav className={sidebarOpen ? 'sidebar open' : 'sidebar'}>
        <div className="sidebar-header">
          <div className="sidebar-logo">
            <img src="/logo.png" alt="TenzoShare" style={{ width: 32, height: 32, objectFit: 'contain' }} />
          </div>
          <div>
            <div className="sidebar-title">TenzoAdmin</div>
            <div className="sidebar-subtitle">Admin Portal</div>
          </div>
        </div>

        <div className="sidebar-nav">
          <div className="sidebar-section-label">Overview</div>
          <NavLink to="/" end className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconDashboard /> System Overview
          </NavLink>

          <div className="sidebar-section-label">Management</div>
          <NavLink to="/users" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconUsers /> User Management
          </NavLink>
          <NavLink to="/transfers" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconTransfer /> Transfers
          </NavLink>
          <NavLink to="/apikeys" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconKey /> API Keys
          </NavLink>

          <div className="sidebar-section-label">Security</div>
          <NavLink to="/audit" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconAudit /> Audit Logs
          </NavLink>
          <NavLink to="/audit/settings" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'} style={{ paddingLeft: 28 }}>
            <IconClock /> Log Retention
          </NavLink>
          <NavLink to="/security" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'} style={{ paddingLeft: 28 }}>
            <IconLock /> Lockout Policy
          </NavLink>

          <div className="sidebar-section-label">Configuration</div>
          <NavLink to="/branding" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconPalette /> Branding
          </NavLink>
          <NavLink to="/storage" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
            <IconStorage /> Storage Settings
          </NavLink>
          <NavLink to="/storage/files" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'} style={{ paddingLeft: 28 }}>
            <IconStorage /> File Management
          </NavLink>
          <NavLink to="/storage/insights" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'} style={{ paddingLeft: 28 }}>
            <IconStorage /> Storage Insights
          </NavLink>
        </div>

        <div className="sidebar-footer">
          <div className="sidebar-user-row">
            <div className="sidebar-avatar">{initials}</div>
            <div className="sidebar-user-info">
              <div className="sidebar-user-name">{displayName}</div>
              <div className="sidebar-user-role">{user?.role ?? 'admin'}</div>
            </div>
          </div>
          <button className="sidebar-logout-btn" onClick={handleLogout}>
            <IconLogOut /> Sign out
          </button>
        </div>
      </nav>

      {/* ── Navbar ──────────────────────────────────────────── */}
      <header className="navbar">
        <button className="hamburger-btn" onClick={() => setSidebarOpen(s => !s)} aria-label="Toggle menu">
          <IconMenu />
        </button>
        <div className="navbar-breadcrumb">{pageTitle}</div>

        <div className="navbar-avatar">{initials}</div>
      </header>

      {/* ── Main content ────────────────────────────────────── */}
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
