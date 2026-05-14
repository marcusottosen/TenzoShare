-- Email white-labeling fields for outgoing email content.
-- These extend branding_settings so the notification service can
-- fetch everything it needs from a single /api/v1/branding call.
ALTER TABLE admin_svc.branding_settings
  ADD COLUMN IF NOT EXISTS email_sender_name    TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_support_email  TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_footer_text    TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_subject_prefix TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_header_link    TEXT NOT NULL DEFAULT '';
