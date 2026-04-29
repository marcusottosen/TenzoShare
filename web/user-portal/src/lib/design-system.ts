/**
 * TenzoShare Design System
 * Single source of truth for brand colors, typography, spacing, and component tokens.
 * Derived from Figma exports (UIImages/TenzoShare - User Portal.svg + My Files.svg)
 */

// ─── Brand Colors ─────────────────────────────────────────────────────────────

export const colors = {
  /** Deep navy — primary brand, sidebar text, headings */
  primary: '#1E293B',
  /** Teal — CTAs, active states, accent icons, progress bars */
  secondary: '#0D9488',
  /** Confirmed teal from SVG icon fills */
  teal: '#006A61',
  /** Slate — muted text, inactive icons */
  tertiary: '#64748B',
  /** Near-white — page background */
  neutral: '#F8FAFC',

  // Extended palette
  /** Dark brand text (#0F172A confirms from SVG "TenzoShare" logo text) */
  brandText: '#0F172A',
  /** Page/canvas background */
  pageBg: '#F7F9FB',
  /** Card/surface background */
  surface: '#FFFFFF',
  /** Primary borders */
  border: '#E2E8F0',
  /** Secondary borders (cards in SVG) */
  borderCard: '#C5C6CD',
  /** Active nav item background pill */
  navActive: '#F1F5F9',
  /** Input/search background */
  inputBg: '#F1F5F9',

  // Text hierarchy
  textPrimary: '#091426',
  textSecondary: '#45474C',
  textMuted: '#64748B',
  textPlaceholder: '#6B7280',

  // Status
  success: '#006A61',
  successBg: '#ECFDF5',
  danger: '#DC2626',
  dangerBg: '#FEF2F2',
  warning: '#D97706',
  warningBg: '#FFFBEB',
  info: '#2563EB',
  infoBg: '#EFF6FF',
} as const;

// ─── Typography ───────────────────────────────────────────────────────────────

export const typography = {
  fontFamily: "'Inter', system-ui, -apple-system, sans-serif",
  sizes: {
    xs: '11px',
    sm: '12px',
    base: '14px',
    md: '15px',
    lg: '16px',
    xl: '18px',
    '2xl': '20px',
    '3xl': '24px',
    '4xl': '32px',
  },
  weights: {
    regular: 400,
    medium: 500,
    semibold: 600,
    bold: 700,
  },
} as const;

// ─── Layout ───────────────────────────────────────────────────────────────────

export const layout = {
  /** Sidebar width (confirmed from SVG: sidebar occupies 0..260px) */
  sidebarWidth: 260,
  /** Navbar height (confirmed from SVG: content starts at y=64) */
  navbarHeight: 64,
  /** Page content padding */
  contentPadding: 24,
} as const;

// ─── Spacing ──────────────────────────────────────────────────────────────────

export const spacing = {
  1: '4px',
  2: '8px',
  3: '12px',
  4: '16px',
  5: '20px',
  6: '24px',
  8: '32px',
  10: '40px',
  12: '48px',
} as const;

// ─── Border Radius ────────────────────────────────────────────────────────────

export const radii = {
  sm: '4px',
  md: '6px',
  lg: '8px',
  xl: '12px',
  '2xl': '16px',
  full: '9999px',
  /** Active nav pill (227x36 rounded-6 from SVG) */
  navPill: '6px',
  /** Card border radius from SVG: rx="7.5" */
  card: '8px',
  /** Avatar: 31x31, rx=11.5 from SVG */
  avatar: '12px',
} as const;

// ─── Shadows ──────────────────────────────────────────────────────────────────

export const shadows = {
  /** Subtle card shadow */
  card: '0 1px 3px 0 rgba(0,0,0,0.06), 0 1px 2px -1px rgba(0,0,0,0.06)',
  /** Elevated dropdown/modal shadow */
  elevated: '0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1)',
  /** Navbar blur shadow */
  navbar: '0 1px 0 0 #E2E8F0',
} as const;

// ─── Component tokens ─────────────────────────────────────────────────────────

export const components = {
  sidebar: {
    width: `${layout.sidebarWidth}px`,
    bg: colors.surface,
    borderRight: `1px solid ${colors.border}`,
    brandColor: colors.brandText,
    brandSize: '16px',
    brandWeight: 600,
    navItemColor: colors.textMuted,
    navItemActiveColor: colors.brandText,
    navItemActiveBg: colors.navActive,
    navItemPadding: '0 12px',
    navItemHeight: '36px',
    navItemBorderRadius: radii.navPill,
    iconSize: '16px',
  },
  navbar: {
    height: `${layout.navbarHeight}px`,
    bg: 'rgba(255,255,255,0.8)',
    backdropBlur: '6px',
    borderBottom: `1px solid ${colors.border}`,
    iconColor: colors.textMuted,
    searchBg: colors.inputBg,
    searchBorderRadius: '4px',
  },
  card: {
    bg: colors.surface,
    border: `1px solid ${colors.borderCard}`,
    borderRadius: radii.card,
    padding: '24px',
    shadow: shadows.card,
  },
  button: {
    primary: {
      bg: colors.secondary,
      color: '#FFFFFF',
      hoverBg: '#0F766E',
      borderRadius: radii.md,
    },
    secondary: {
      bg: colors.surface,
      color: colors.textPrimary,
      border: `1px solid ${colors.border}`,
      borderRadius: radii.md,
    },
    danger: {
      bg: '#DC2626',
      color: '#FFFFFF',
      borderRadius: radii.md,
    },
  },
  badge: {
    active: { bg: '#ECFDF5', color: '#065F46', border: '#A7F3D0' },
    expired: { bg: '#F1F5F9', color: '#475569', border: '#CBD5E1' },
    revoked: { bg: '#FEF2F2', color: '#991B1B', border: '#FECACA' },
    pending: { bg: '#FFFBEB', color: '#92400E', border: '#FDE68A' },
  },
} as const;
