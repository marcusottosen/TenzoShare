import React from 'react';

export default function SettingsPage() {
  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="page-subtitle">Application settings</p>
        </div>
      </div>

      <div className="card">
        <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
          No settings have been configured yet.
        </p>
      </div>
    </div>
  );
}
