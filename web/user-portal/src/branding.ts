/**
 * Branding loader — fetches theme config from the admin service and applies
 * CSS variables to the document root. Call once before React mounts.
 */

let _logoUrl: string = '/logo.png';
let _appName: string = 'TenzoShare';

// Inline var maps — tracked so toggling dark mode can override branding inline styles.
let _lightVars: Record<string, string> = {};
let _darkVars: Record<string, string> = {};

// Built-in dark defaults for every var that branding may set inline.
const DARK_DEFAULTS: Record<string, string> = {
  '--color-primary':          '#0F172A',
  '--color-sidebar':          '#0F172A',
  '--color-secondary':        '#0D9488',
  '--color-secondary-hover':  '#0F766E',
  '--color-teal':             '#0D9488',
  '--color-success':          '#0D9488',
  '--color-page-bg':          '#0F172A',
  '--color-surface':          '#1E293B',
  '--color-text-primary':     '#E2E8F0',
  '--color-brand-text':       '#E2E8F0',
};

export function getLogoUrl(): string { return _logoUrl; }
export function getAppName(): string { return _appName; }

/** Apply dark mode attribute immediately (before paint) to prevent a flash. */
export function initDarkMode(): void {
  if (localStorage.getItem('tenzo-dark-mode') === 'true') {
    document.documentElement.setAttribute('data-theme', 'dark');
  }
}

export function isDarkMode(): boolean {
  return document.documentElement.getAttribute('data-theme') === 'dark';
}

/** Toggle dark mode, applying the right inline vars to override branding inline styles. */
export function setDarkMode(enabled: boolean): void {
  const root = document.documentElement;
  if (enabled) {
    localStorage.setItem('tenzo-dark-mode', 'true');
    root.setAttribute('data-theme', 'dark');
    for (const [k, v] of Object.entries(_darkVars)) {
      root.style.setProperty(k, v);
    }
  } else {
    localStorage.setItem('tenzo-dark-mode', 'false');
    root.removeAttribute('data-theme');
    for (const [k, v] of Object.entries(_lightVars)) {
      root.style.setProperty(k, v);
    }
  }
}

export async function loadBranding(): Promise<void> {
  try {
    const res = await fetch('/api/v1/branding');
    if (!res.ok) return;
    const data: {
      primary_color?: string;
      secondary_color?: string;
      page_bg_color?: string;
      surface_color?: string;
      text_color?: string;
      border_radius?: number;
      app_name?: string;
      custom_css?: string | null;
      logo_data_url?: string | null;
      dm_primary_color?: string | null;
      dm_secondary_color?: string | null;
      dm_page_bg_color?: string | null;
      dm_surface_color?: string | null;
      dm_text_color?: string | null;
    } = await res.json();

    // Build light var map
    function setLight(key: string, value: string) {
      _lightVars[key] = value;
    }

    if (data.primary_color) {
      setLight('--color-primary', data.primary_color);
      setLight('--color-sidebar', data.primary_color);
    }
    if (data.secondary_color) {
      setLight('--color-secondary', data.secondary_color);
      setLight('--color-secondary-hover', darken(data.secondary_color, 0.08));
      setLight('--color-teal', darken(data.secondary_color, 0.12));
      setLight('--color-success', darken(data.secondary_color, 0.12));
    }
    if (data.page_bg_color)  setLight('--color-page-bg', data.page_bg_color);
    if (data.surface_color)  setLight('--color-surface', data.surface_color);
    if (data.text_color) {
      setLight('--color-text-primary', data.text_color);
      setLight('--color-brand-text', data.text_color);
    }
    if (data.border_radius !== undefined && data.border_radius !== null) {
      const r = data.border_radius;
      setLight('--radius-sm', `${r}px`);
      setLight('--radius-md', `${r + 2}px`);
      setLight('--radius-lg', `${r + 6}px`);
    }

    // Build dark var map (start from built-in defaults, apply admin overrides)
    _darkVars = { ...DARK_DEFAULTS };
    if (data.dm_primary_color) {
      _darkVars['--color-primary'] = data.dm_primary_color;
      _darkVars['--color-sidebar'] = data.dm_primary_color;
    }
    if (data.dm_secondary_color) {
      _darkVars['--color-secondary'] = data.dm_secondary_color;
      _darkVars['--color-secondary-hover'] = darken(data.dm_secondary_color, 0.08);
    }
    if (data.dm_page_bg_color)  _darkVars['--color-page-bg']     = data.dm_page_bg_color;
    if (data.dm_surface_color)  _darkVars['--color-surface']      = data.dm_surface_color;
    if (data.dm_text_color) {
      _darkVars['--color-text-primary'] = data.dm_text_color;
      _darkVars['--color-brand-text']   = data.dm_text_color;
    }
    // Carry border-radius into dark mode too
    if (_lightVars['--radius-sm']) {
      _darkVars['--radius-sm'] = _lightVars['--radius-sm'];
      _darkVars['--radius-md'] = _lightVars['--radius-md'];
      _darkVars['--radius-lg'] = _lightVars['--radius-lg'];
    }

    // Apply the correct set inline (dark or light)
    const varsToApply = isDarkMode() ? _darkVars : _lightVars;
    const root = document.documentElement;
    for (const [k, v] of Object.entries(varsToApply)) {
      root.style.setProperty(k, v);
    }

    if (data.app_name) {
      _appName = data.app_name;
      document.title = document.title.replace('TenzoShare', data.app_name);
    }
    if (data.custom_css) {
      const style = document.createElement('style');
      style.id = 'branding-custom-css';
      style.textContent = data.custom_css;
      document.head.appendChild(style);
    }
    if (data.logo_data_url) {
      _logoUrl = data.logo_data_url;
    }
  } catch {
    // Silently use defaults if branding endpoint is unreachable.
  }
}

/** Darken a hex color by a fraction (0–1). */
function darken(hex: string, amount: number): string {
  const n = parseInt(hex.replace('#', ''), 16);
  const r = Math.max(0, Math.round(((n >> 16) & 0xff) * (1 - amount)));
  const g = Math.max(0, Math.round(((n >> 8) & 0xff) * (1 - amount)));
  const b = Math.max(0, Math.round((n & 0xff) * (1 - amount)));
  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`;
}
