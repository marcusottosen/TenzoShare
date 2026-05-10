import { request } from './client';
import type { DateFormat, TimeFormat } from '../utils/dateFormat';

export interface PlatformConfig {
  date_format: DateFormat;
  time_format: TimeFormat;
  timezone: string;
  link_protection_policy?: 'none' | 'password' | 'email' | 'either';
  updated_at?: string;
}

/** Public endpoint — no auth required. */
export async function getPlatformConfig(): Promise<PlatformConfig> {
  return request<PlatformConfig>('/platform/config');
}
