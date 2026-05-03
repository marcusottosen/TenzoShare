/**
 * Branding loader — fetches theme config from the admin service and applies
 * CSS variables to the document root. Call once before React mounts.
 */

let _logoUrl: string = '/logo.png';
let _appName: string = 'TenzoShare';

export function getLogoUrl(): string {
  return _logoUrl;
}

export function getAppName(): string {
  return _appName;
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
    } = await res.json();

    const root = document.documentElement;

    if (data.primary_color) {
      root.style.setProperty('--color-primary', data.primary_color);
      root.style.setProperty('--color-sidebar', data.primary_color);
    }
    if (data.secondary_color) {
      root.style.setProperty('--color-secondary', data.secondary_color);
      root.style.setProperty('--color-secondary-hover', darken(data.secondary_color, 0.08));
      root.style.setProperty('--color-teal', darken(data.secondary_color, 0.12));
      root.style.setProperty('--color-success', darken(data.secondary_color, 0.12));
    }
    if (data.page_bg_color) {
      root.style.setProperty('--color-page-bg', data.page_bg_color);
    }
    if (data.surface_color) {
      root.style.setProperty('--color-surface', data.surface_color);
    }
    if (data.text_color) {
      root.style.setProperty('--color-text-primary', data.text_color);
      root.style.setProperty('--color-brand-text', data.text_color);
    }
    if (data.border_radius !== undefined && data.border_radius !== null) {
      const r = data.border_radius;
      root.style.setProperty('--radius-sm', `${r}px`);
      root.style.setProperty('--radius-md', `${r + 2}px`);
      root.style.setProperty('--radius-lg', `${r + 6}px`);
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
