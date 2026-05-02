import React, { useEffect, useRef, useState } from 'react';
import { getBranding, updateBranding, type BrandingConfig } from '../api/admin';

const DEFAULTS = { primary_color: '#1E293B', secondary_color: '#0D9488' };

// ── Page ─────────────────────────────────────────────────────────────────────

export default function BrandingPage() {
  const [config, setConfig] = useState<BrandingConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // form state
  const [primaryColor, setPrimaryColor] = useState(DEFAULTS.primary_color);
  const [secondaryColor, setSecondaryColor] = useState(DEFAULTS.secondary_color);
  const [logoPreview, setLogoPreview] = useState<string | null>(null);
  const [logoChanged, setLogoChanged] = useState(false);
  const [clearLogo, setClearLogo] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setLoading(true);
    getBranding()
      .then((c) => {
        setConfig(c);
        setPrimaryColor(c.primary_color);
        setSecondaryColor(c.secondary_color);
        setLogoPreview(c.logo_data_url ?? null);
      })
      .catch(() => setError('Failed to load branding settings.'))
      .finally(() => setLoading(false));
  }, []);

  function handleLogoFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > 512 * 1024) {
      setError('Logo file must be under 512 KB.');
      e.target.value = '';
      return;
    }
    const reader = new FileReader();
    reader.onload = (ev) => {
      setLogoPreview(ev.target?.result as string);
      setLogoChanged(true);
      setClearLogo(false);
    };
    reader.readAsDataURL(file);
  }

  function handleClearLogo() {
    setLogoPreview(null);
    setLogoChanged(true);
    setClearLogo(true);
    if (fileRef.current) fileRef.current.value = '';
  }

  function handleResetColors() {
    setPrimaryColor(DEFAULTS.primary_color);
    setSecondaryColor(DEFAULTS.secondary_color);
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const body: Parameters<typeof updateBranding>[0] = {
        primary_color: primaryColor,
        secondary_color: secondaryColor,
      };
      if (clearLogo) {
        body.clear_logo = true;
      } else if (logoChanged && logoPreview) {
        body.logo_data_url = logoPreview;
      }
      const updated = await updateBranding(body);
      setConfig(updated);
      setLogoChanged(false);
      setClearLogo(false);
      setSuccess('Branding saved. Changes will appear on user-facing sites after page reload.');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to save branding.');
    } finally {
      setSaving(false);
    }
  }

  const currentLogo = logoPreview ?? '/logo.png';

  return (
    <div className="page-content" style={{ maxWidth: 720 }}>
      <div className="page-header" style={{ marginBottom: 24 }}>
        <h1 className="page-title">Branding</h1>
        <p className="page-subtitle" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
          Customize the colors and logo shown on the User Portal, Download pages, and Upload Request pages.
          The Admin Portal itself is not affected.
        </p>
      </div>

      {loading && <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>}

      {!loading && (
        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          {/* Colors */}
          <div className="card" style={{ padding: 24 }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 20 }}>Colors</h2>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
              {/* Primary */}
              <div className="form-group">
                <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>
                  Primary color
                </label>
                <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 10 }}>
                  Sidebar / header background in the User Portal.
                </p>
                <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                  <input
                    type="color"
                    value={primaryColor}
                    onChange={(e) => setPrimaryColor(e.target.value)}
                    style={{ width: 48, height: 36, padding: 2, cursor: 'pointer', borderRadius: 6, border: '1px solid var(--color-border)' }}
                  />
                  <input
                    type="text"
                    maxLength={7}
                    value={primaryColor}
                    onChange={(e) => {
                      const v = e.target.value;
                      if (/^#[0-9A-Fa-f]{0,6}$/.test(v)) setPrimaryColor(v);
                    }}
                    style={{ width: 100, fontFamily: 'monospace' }}
                  />
                </div>
                <div style={{ marginTop: 10, width: 80, height: 32, borderRadius: 6, background: primaryColor, border: '1px solid var(--color-border)' }} />
              </div>

              {/* Secondary */}
              <div className="form-group">
                <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>
                  Accent color
                </label>
                <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 10 }}>
                  Buttons, active states, and highlights across all user-facing sites.
                </p>
                <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                  <input
                    type="color"
                    value={secondaryColor}
                    onChange={(e) => setSecondaryColor(e.target.value)}
                    style={{ width: 48, height: 36, padding: 2, cursor: 'pointer', borderRadius: 6, border: '1px solid var(--color-border)' }}
                  />
                  <input
                    type="text"
                    maxLength={7}
                    value={secondaryColor}
                    onChange={(e) => {
                      const v = e.target.value;
                      if (/^#[0-9A-Fa-f]{0,6}$/.test(v)) setSecondaryColor(v);
                    }}
                    style={{ width: 100, fontFamily: 'monospace' }}
                  />
                </div>
                <div style={{ marginTop: 10, width: 80, height: 32, borderRadius: 6, background: secondaryColor, border: '1px solid var(--color-border)' }} />
              </div>
            </div>

            <button
              type="button"
              className="btn btn-secondary btn-sm"
              style={{ marginTop: 16 }}
              onClick={handleResetColors}
              disabled={saving}
            >
              Reset to defaults
            </button>
          </div>

          {/* Logo */}
          <div className="card" style={{ padding: 24 }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 8 }}>Logo</h2>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 20 }}>
              Replaces the default logo on all user-facing sites. PNG, JPG or SVG, max 512 KB.
              If cleared, the default logo will be used.
            </p>

            <div style={{ display: 'flex', gap: 24, alignItems: 'flex-start', flexWrap: 'wrap' }}>
              {/* Preview */}
              <div style={{
                width: 120, height: 120, borderRadius: 12,
                border: '1px solid var(--color-border)',
                background: primaryColor,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                overflow: 'hidden',
              }}>
                <img
                  src={currentLogo}
                  alt="Logo preview"
                  style={{ maxWidth: 80, maxHeight: 80, objectFit: 'contain' }}
                />
              </div>

              {/* Controls */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => fileRef.current?.click()}
                  disabled={saving}
                >
                  Upload new logo
                </button>
                <input
                  ref={fileRef}
                  type="file"
                  accept="image/png,image/jpeg,image/svg+xml,image/webp"
                  style={{ display: 'none' }}
                  onChange={handleLogoFile}
                />
                {(logoPreview && (logoPreview !== (config?.logo_data_url ?? null) || config?.logo_data_url)) && (
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    style={{ color: 'var(--color-danger)' }}
                    onClick={handleClearLogo}
                    disabled={saving}
                  >
                    Remove custom logo
                  </button>
                )}
                {logoChanged && (
                  <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                    {clearLogo ? 'Logo will be cleared on save.' : 'New logo selected — save to apply.'}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Actions */}
          {error && (
            <div className="alert alert-error" style={{ margin: 0 }}>{error}</div>
          )}
          {success && (
            <div className="alert alert-success" style={{ margin: 0 }}>{success}</div>
          )}

          <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save branding'}
            </button>
            {config?.updated_at && (
              <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Last updated: {new Date(config.updated_at).toLocaleString()}
              </span>
            )}
          </div>
        </form>
      )}
    </div>
  );
}
