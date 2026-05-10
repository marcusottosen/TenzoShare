-- Migration 009: per-type fully custom HTML email templates
-- Empty string = use the standard branded template; non-empty = use this verbatim.

ALTER TABLE admin_svc.branding_settings
    ADD COLUMN IF NOT EXISTS email_custom_transfer_received      TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_custom_password_reset         TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_custom_email_verification     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_custom_download_notification  TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_custom_expiry_reminder        TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_custom_transfer_revoked       TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS email_custom_request_submission     TEXT NOT NULL DEFAULT '';
