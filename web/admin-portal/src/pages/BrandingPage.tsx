import React, { useEffect, useRef, useState } from 'react';
import { getBranding, updateBranding, type BrandingConfig } from '../api/admin';

const DEFAULTS = {
  primary_color: '#1E293B',
  secondary_color: '#0D9488',
  page_bg_color: '#F7F9FB',
  surface_color: '#FFFFFF',
  text_color: '#091426',
  border_radius: 6,
  app_name: 'TenzoShare',
};

// ── Helpers ───────────────────────────────────────────────────────────────────

function ColorPicker({
  label,
  description,
  value,
  onChange,
}: {
  label: string;
  description: string;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <div className="form-group">
      <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>{label}</label>
      <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 8 }}>{description}</p>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <input
          type="color"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          style={{ width: 44, height: 34, padding: 2, cursor: 'pointer', borderRadius: 6, border: '1px solid var(--color-border)' }}
        />
        <input
          type="text"
          maxLength={7}
          value={value}
          onChange={(e) => { if (/^#[0-9A-Fa-f]{0,6}$/.test(e.target.value)) onChange(e.target.value); }}
          style={{ width: 94, fontFamily: 'monospace' }}
        />
        <div style={{ width: 28, height: 28, borderRadius: 6, background: value, border: '1px solid var(--color-border)', flexShrink: 0 }} />
      </div>
    </div>
  );
}

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
  const [pageBgColor, setPageBgColor] = useState(DEFAULTS.page_bg_color);
  const [surfaceColor, setSurfaceColor] = useState(DEFAULTS.surface_color);
  const [textColor, setTextColor] = useState(DEFAULTS.text_color);
  const [borderRadius, setBorderRadius] = useState(DEFAULTS.border_radius);
  const [appName, setAppName] = useState(DEFAULTS.app_name);
  const [customCss, setCustomCss] = useState('');
  const [clearCustomCss, setClearCustomCss] = useState(false);
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
        setPageBgColor(c.page_bg_color);
        setSurfaceColor(c.surface_color);
        setTextColor(c.text_color);
        setBorderRadius(c.border_radius);
        setAppName(c.app_name);
        setCustomCss(c.custom_css ?? '');
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

  function handleResetDefaults() {
    setPrimaryColor(DEFAULTS.primary_color);
    setSecondaryColor(DEFAULTS.secondary_color);
    setPageBgColor(DEFAULTS.page_bg_color);
    setSurfaceColor(DEFAULTS.surface_color);
    setTextColor(DEFAULTS.text_color);
    setBorderRadius(DEFAULTS.border_radius);
    setAppName(DEFAULTS.app_name);
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
        page_bg_color: pageBgColor,
        surface_color: surfaceColor,
        text_color: textColor,
        border_radius: borderRadius,
        app_name: appName,
      };
      if (clearCustomCss) {
        body.clear_custom_css = true;
      } else if (customCss.trim()) {
        body.custom_css = customCss;
      }
      if (clearLogo) {
        body.clear_logo = true;
      } else if (logoChanged && logoPreview) {
        body.logo_data_url = logoPreview;
      }
      const updated = await updateBranding(body);
      setConfig(updated);
      setLogoChanged(false);
      setClearLogo(false);
      setClearCustomCss(false);
      setSuccess('Branding saved. Changes will appear on user-facing sites after page reload.');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to save branding.');
    } finally {
      setSaving(false);
    }
  }

  const currentLogo = logoPreview ?? '/logo.png';
  const radiusLabels: Record<number, string> = { 0: 'Sharp', 4: 'Slight', 6: 'Default', 10: 'Rounded', 16: 'Very rounded', 20: 'Pill' };

  return (
    <div className="page-content" style={{ maxWidth: 760 }}>
      <div className="page-header" style={{ marginBottom: 24 }}>
        <h1 className="page-title">Branding</h1>
        <p className="page-subtitle" style={{ marginTop: 4, color: 'var(--color-text-muted)' }}>
          Customize the appearance of the User Portal, Download pages, and Upload Request pages.
          The Admin Portal itself is not affected.
        </p>
      </div>

      {loading && <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>Loading…</p>}

      {!loading && (
        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>

          {/* ── Identity ── */}
          <div className="card" style={{ padding: 24 }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 20 }}>Identity</h2>
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>App name</label>
              <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 8 }}>
                Replaces "TenzoShare" in page titles and headings on user-facing sites.
              </p>
              <input
                type="text"
                value={appName}
                maxLength={100}
                onChange={(e) => setAppName(e.target.value)}
                placeholder="TenzoShare"
                style={{ maxWidth: 320 }}
              />
              {appName.trim() && appName.trim() !== 'TenzoShare' && (
                <p className="text-sm" style={{ marginTop: 8, color: 'var(--color-text-muted)', opacity: 0.75 }}>
                  A subtle "Powered by TenzoShare" attribution will appear on user-facing pages.
                </p>
              )}
            </div>
          </div>

          {/* ── Colors ── */}
          <div className="card" style={{ padding: 24 }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 20 }}>Colors</h2>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
              <ColorPicker
                label="Primary / sidebar color"
                description="Sidebar and header background in the User Portal."
                value={primaryColor}
                onChange={setPrimaryColor}
              />
              <ColorPicker
                label="Accent color"
                description="Buttons, active states, links and highlights."
                value={secondaryColor}
                onChange={setSecondaryColor}
              />
              <ColorPicker
                label="Page background"
                description="Background color of every page."
                value={pageBgColor}
                onChange={setPageBgColor}
              />
              <ColorPicker
                label="Surface / card color"
                description="Background of cards, modals, and form inputs."
                value={surfaceColor}
                onChange={setSurfaceColor}
              />
              <ColorPicker
                label="Primary text color"
                description="Main body text and headings."
                value={textColor}
                onChange={setTextColor}
              />
            </div>

            <button
              type="button"
              className="btn btn-secondary btn-sm"
              style={{ marginTop: 20 }}
              onClick={handleResetDefaults}
              disabled={saving}
            >
              Reset all to defaults
            </button>
          </div>

          {/* ── Shape ── */}
          <div className="card" style={{ padding: 24 }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 8 }}>Shape</h2>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 20 }}>
              Controls the roundness of buttons, cards, inputs, and other UI elements.
            </p>
            <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
              <span className="text-sm" style={{ color: 'var(--color-text-muted)', width: 40 }}>Sharp</span>
              <input
                type="range"
                min={0}
                max={20}
                step={1}
                value={borderRadius}
                onChange={(e) => setBorderRadius(Number(e.target.value))}
                style={{ flex: 1 }}
              />
              <span className="text-sm" style={{ color: 'var(--color-text-muted)', width: 40, textAlign: 'right' }}>Round</span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 16, flexWrap: 'wrap' }}>
              <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
                Base radius: <strong>{borderRadius}px</strong>
                {radiusLabels[borderRadius] ? ` — ${radiusLabels[borderRadius]}` : ''}
              </span>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <div style={{
                  width: 64, height: 28, background: secondaryColor, color: '#fff',
                  borderRadius: borderRadius, display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 12, fontWeight: 500,
                }}>Button</div>
                <div style={{
                  width: 56, height: 56, background: surfaceColor,
                  border: '1px solid #e2e8f0', borderRadius: borderRadius + 4,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 11, color: '#64748b',
                }}>Card</div>
              </div>
            </div>
          </div>

          {/* ── Logo ── */}
          <div className="card" style={{ padding: 24 }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 8 }}>Logo</h2>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 20 }}>
              Replaces the default logo on all user-facing sites. PNG, JPG or SVG, max 512 KB.
            </p>
            <div style={{ display: 'flex', gap: 24, alignItems: 'flex-start', flexWrap: 'wrap' }}>
              <div style={{
                width: 100, height: 100, borderRadius: 12,
                border: '1px solid var(--color-border)',
                background: primaryColor,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                overflow: 'hidden', flexShrink: 0,
              }}>
                <img src={currentLogo} alt="Logo preview" style={{ maxWidth: 68, maxHeight: 68, objectFit: 'contain' }} />
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                <button type="button" className="btn btn-secondary" onClick={() => fileRef.current?.click()} disabled={saving}>
                  Upload new logo
                </button>
                <input ref={fileRef} type="file" accept="image/png,image/jpeg,image/svg+xml,image/webp" style={{ display: 'none' }} onChange={handleLogoFile} />
                {(config?.logo_data_url || logoChanged) && (
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

          {/* ── Custom CSS ── */}
          <div className="card" style={{ padding: 24 }}>
            <h2 style={{ fontSize: 15, fontWeight: 600, marginBottom: 4 }}>Custom CSS</h2>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 4 }}>
              Injected as a <code>&lt;style&gt;</code> tag on every user-facing page (User Portal, Download pages, Upload Request pages).
              Use CSS variables or target class names directly.
            </p>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)', marginBottom: 16 }}>
              Key variables: <code>--color-primary</code> · <code>--color-secondary</code> · <code>--color-page-bg</code> · <code>--color-surface</code> · <code>--color-text-primary</code> · <code>--color-border</code> · <code>--radius-sm/md/lg</code> · <code>--font-sans</code>
            </p>
            <textarea
              rows={10}
              value={customCss}
              onChange={(e) => { setCustomCss(e.target.value); setClearCustomCss(false); }}
              placeholder={[
                '/* ─── Custom font ───────────────────────────────── */',
                "@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap');",
                ':root { --font-sans: "Inter", sans-serif; }',
                '',
                '/* ─── Sidebar gradient instead of flat color ────── */',
                '.sidebar { background: linear-gradient(160deg, #1E293B 0%, #0F172A 100%); }',
                '',
                '/* ─── Rounded pill buttons ──────────────────────── */',
                '.btn { border-radius: 999px; letter-spacing: 0.03em; }',
                '',
                '/* ─── Custom card shadow ────────────────────────── */',
                '.card, .tenzo-card { box-shadow: 0 4px 24px rgba(0,0,0,0.08); border: none; }',
                '',
                '/* ─── Hide the "Powered by TenzoShare" attribution ─ */',
                '/* .tenzo-footer span { display: none; } */',
              ].join('\n')}
              style={{ fontFamily: 'monospace', fontSize: 12.5, resize: 'vertical', lineHeight: 1.6 }}
            />
            {customCss && (
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                style={{ marginTop: 8, color: 'var(--color-danger)' }}
                onClick={() => { setCustomCss(''); setClearCustomCss(true); }}
                disabled={saving}
              >
                Clear custom CSS
              </button>
            )}
          </div>

          {/* ── Actions ── */}
          {error && <div className="alert alert-error" style={{ margin: 0 }}>{error}</div>}
          {success && <div className="alert alert-success" style={{ margin: 0 }}>{success}</div>}

          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
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

