// dateFormat.ts — centralised date/time formatting for the user portal.
//
// Preferences are a module-level singleton, set once on app load from the /me
// response merged with the public /platform/config system defaults.  Components
// that render dates simply call fmt() / fmtDate() — no prop-drilling required.

export type DateFormat = 'ISO' | 'EU' | 'US' | 'DE' | 'LONG';
export type TimeFormat = '12h' | '24h';

export interface DatePrefs {
  dateFormat: DateFormat;
  timeFormat: TimeFormat;
  timezone: string;
}

export const DEFAULT_PREFS: DatePrefs = {
  dateFormat: 'EU',
  timeFormat: '24h',
  timezone: 'UTC',
};

let _active: DatePrefs = { ...DEFAULT_PREFS };

/** Called by AuthProvider after loading system defaults + user prefs. */
export function setActivePrefs(p: Partial<DatePrefs>): void {
  _active = { ..._active, ...p };
}

export function getActivePrefs(): DatePrefs {
  return _active;
}

// ── formatting ────────────────────────────────────────────────────────────────

function safeDate(iso: string | null | undefined): Date | null {
  if (!iso) return null;
  const d = new Date(iso);
  return isNaN(d.getTime()) ? null : d;
}

/** Format a date+time string using the active preferences. */
export function fmt(iso: string | null | undefined): string {
  const d = safeDate(iso);
  if (!d) return '—';
  const tz = _active.timezone;
  const hour12 = _active.timeFormat === '12h';
  return new Intl.DateTimeFormat('en', {
    ...dateParts(_active.dateFormat),
    hour: '2-digit',
    minute: '2-digit',
    hour12,
    timeZone: tz,
  }).format(d);
}

/** Format a date-only string using the active preferences. */
export function fmtDate(iso: string | null | undefined): string {
  const d = safeDate(iso);
  if (!d) return '—';
  return new Intl.DateTimeFormat('en', {
    ...dateParts(_active.dateFormat),
    timeZone: _active.timezone,
  }).format(d);
}

function dateParts(f: DateFormat): Intl.DateTimeFormatOptions {
  switch (f) {
    case 'ISO':
      return { year: 'numeric', month: '2-digit', day: '2-digit',
               // Intl doesn't support ISO order natively — format manually
      };
    case 'US':
      return { month: '2-digit', day: '2-digit', year: 'numeric' };
    case 'DE':
      return { day: '2-digit', month: '2-digit', year: 'numeric' };
    case 'LONG':
      return { day: 'numeric', month: 'short', year: 'numeric' };
    case 'EU':
    default:
      return { day: '2-digit', month: '2-digit', year: 'numeric' };
  }
}

/** List of common IANA timezones for picker dropdowns. */
export const COMMON_TIMEZONES = [
  'UTC',
  'Europe/London',
  'Europe/Oslo',
  'Europe/Berlin',
  'Europe/Paris',
  'Europe/Amsterdam',
  'Europe/Stockholm',
  'Europe/Helsinki',
  'Europe/Warsaw',
  'Europe/Rome',
  'Europe/Madrid',
  'Europe/Lisbon',
  'Europe/Athens',
  'Europe/Istanbul',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Anchorage',
  'America/Halifax',
  'America/Toronto',
  'America/Vancouver',
  'America/Sao_Paulo',
  'America/Argentina/Buenos_Aires',
  'America/Mexico_City',
  'Asia/Dubai',
  'Asia/Kolkata',
  'Asia/Singapore',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Asia/Seoul',
  'Asia/Bangkok',
  'Asia/Jakarta',
  'Asia/Riyadh',
  'Asia/Tehran',
  'Asia/Karachi',
  'Asia/Dhaka',
  'Asia/Colombo',
  'Australia/Sydney',
  'Australia/Melbourne',
  'Australia/Perth',
  'Pacific/Auckland',
  'Pacific/Honolulu',
  'Africa/Cairo',
  'Africa/Johannesburg',
  'Africa/Lagos',
  'Africa/Nairobi',
];
