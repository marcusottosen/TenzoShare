import React, { useState } from 'react';
import { isDarkMode, setDarkMode } from '../branding';

function ComingSoonBadge() {
  return (
    <span style={{
      fontSize: 10,
      fontWeight: 600,
      letterSpacing: '0.04em',
      textTransform: 'uppercase',
      background: 'var(--color-border)',
      color: 'var(--color-text-muted)',
      borderRadius: 4,
      padding: '2px 6px',
      marginLeft: 8,
      verticalAlign: 'middle',
    }}>
      Coming soon
    </span>
  );
}

function SettingRow({ label, description, children }: { label: string; description?: string; children: React.ReactNode }) {
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '14px 0',
      borderBottom: '1px solid var(--color-border)',
      gap: 16,
    }}>
      <div style={{ flex: 1 }}>
        <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-primary)' }}>{label}</div>
        {description && <div style={{ fontSize: 12, color: 'var(--color-text-muted)', marginTop: 2 }}>{description}</div>}
      </div>
      <div>{children}</div>
    </div>
  );
}

export default function SettingsPage() {
  const [darkMode, setDarkModeState] = useState(() => isDarkMode());

  function handleDarkModeToggle(e: React.ChangeEvent<HTMLInputElement>) {
    const enabled = e.target.checked;
    setDarkModeState(enabled);
    setDarkMode(enabled);
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="page-subtitle">Application preferences</p>
        </div>
      </div>

      {/* Appearance */}
      <div className="card" style={{ marginBottom: 20 }}>
        <div className="card-header" style={{ marginBottom: 4 }}>
          <h2 className="card-title">Appearance</h2>
        </div>
        <SettingRow
          label="Dark mode"
          description="Switch the interface to a dark colour scheme."
        >
          <label className="toggle-switch">
            <input
              type="checkbox"
              checked={darkMode}
              onChange={handleDarkModeToggle}
            />
            <span className="toggle-track" />
          </label>
        </SettingRow>
      </div>

      {/* Notifications */}
      <div className="card">
        <div className="card-header" style={{ marginBottom: 4 }}>
          <h2 className="card-title">Notifications <ComingSoonBadge /></h2>
        </div>
        <SettingRow
          label="Download alerts"
          description="Receive an email when someone downloads one of your transfers."
        >
          <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'not-allowed', opacity: 0.45 }}>
            <input type="checkbox" disabled style={{ width: 16, height: 16 }} />
            <span style={{ fontSize: 13 }}>Notify me</span>
          </label>
        </SettingRow>
        <SettingRow
          label="Transfer expiry reminders"
          description="Get a heads-up before a transfer link expires."
        >
          <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'not-allowed', opacity: 0.45 }}>
            <input type="checkbox" disabled style={{ width: 16, height: 16 }} />
            <span style={{ fontSize: 13 }}>Notify me</span>
          </label>
        </SettingRow>
        <SettingRow
          label="File request submissions"
          description="Email me when someone submits files to one of your requests."
        >
          <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'not-allowed', opacity: 0.45 }}>
            <input type="checkbox" disabled style={{ width: 16, height: 16 }} />
            <span style={{ fontSize: 13 }}>Notify me</span>
          </label>
        </SettingRow>
      </div>
    </div>
  );
}
