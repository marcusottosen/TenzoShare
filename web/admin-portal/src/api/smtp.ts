import { request } from './client';

export interface SmtpSettings {
  host: string;
  port: string;
  username: string;
  password_set: boolean;
  from: string;
  use_tls: boolean;
  updated_at: string;
}

export interface SmtpSettingsUpdate {
  host?: string;
  port?: string;
  username?: string;
  /** Empty string clears the stored password; omit to leave unchanged. */
  password?: string;
  from?: string;
  use_tls?: boolean;
}

export interface SmtpTestPayload {
  host?: string;
  port?: string;
  username?: string;
  password?: string;
  from?: string;
  use_tls?: boolean;
}

export interface SmtpTestResult {
  ok: boolean;
  error?: string;
}

export async function getSmtpSettings(): Promise<SmtpSettings> {
  return request<SmtpSettings>('/admin/settings/smtp');
}

export async function updateSmtpSettings(settings: SmtpSettingsUpdate): Promise<SmtpSettings> {
  return request<SmtpSettings>('/admin/settings/smtp', {
    method: 'PUT',
    body: JSON.stringify(settings),
  });
}

/** Send a test email. Provide fields to test settings before saving; omit all to test the stored config. */
export async function testSmtpSettings(payload?: SmtpTestPayload): Promise<SmtpTestResult> {
  return request<SmtpTestResult>('/admin/settings/smtp/test', {
    method: 'POST',
    body: JSON.stringify(payload ?? {}),
  });
}
