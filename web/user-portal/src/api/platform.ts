import { request } from './client';
import type { DateFormat, TimeFormat } from '../utils/dateFormat';

export interface PlatformConfig {
  date_format: DateFormat;
  time_format: TimeFormat;
  timezone: string;
  updated_at?: string;
}

/** Public endpoint — no auth required. */
export async function getPlatformConfig(): Promise<PlatformConfig> {
  return request<PlatformConfig>('/platform/config');
}
