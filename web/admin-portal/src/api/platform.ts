import { request } from './client';
import type { DateFormat, TimeFormat } from '../utils/dateFormat';

export interface PlatformConfig {
  date_format: DateFormat;
  time_format: TimeFormat;
  timezone: string;
  portal_url: string;
  download_url: string;
  updated_at?: string;
}

/** Public endpoint — no auth required. */
export async function getPlatformConfig(): Promise<PlatformConfig> {
  return request<PlatformConfig>('/admin/platform/config');
}

/** Admin-only: update system-wide date/time defaults. */
export async function updatePlatformConfig(cfg: Partial<Omit<PlatformConfig, 'updated_at'>>): Promise<PlatformConfig> {
  return request<PlatformConfig>('/admin/platform/config', {
    method: 'PUT',
    body: JSON.stringify(cfg),
  });
}
