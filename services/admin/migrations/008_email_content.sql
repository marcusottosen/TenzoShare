-- Extended email white-labeling: custom colors, reply-to, per-type
-- subject templates, and per-type CTA button text.
-- All new columns default to '' so existing installs keep working
-- without any admin action.
ALTER TABLE admin_svc.branding_settings
  -- Custom email colors (empty = fall back to built-in defaults)
  ADD COLUMN IF NOT EXISTS email_button_color      TEXT NOT NULL DEFAULT '',  -- CTA button bg; fallback: primary_color
  ADD COLUMN IF NOT EXISTS email_button_text_color TEXT NOT NULL DEFAULT '',  -- CTA button text; fallback: #ffffff
  ADD COLUMN IF NOT EXISTS email_body_bg_color     TEXT NOT NULL DEFAULT '',  -- outer wrapper bg; fallback: #f1f5f9
  ADD COLUMN IF NOT EXISTS email_card_bg_color     TEXT NOT NULL DEFAULT '',  -- info card bg; fallback: #f8fafc
  ADD COLUMN IF NOT EXISTS email_card_border_color TEXT NOT NULL DEFAULT '',  -- info card border; fallback: #e2e8f0
  ADD COLUMN IF NOT EXISTS email_heading_color     TEXT NOT NULL DEFAULT '',  -- heading text; fallback: #1e293b
  ADD COLUMN IF NOT EXISTS email_text_color        TEXT NOT NULL DEFAULT '',  -- body paragraph text; fallback: #475569
  ADD COLUMN IF NOT EXISTS email_reply_to          TEXT NOT NULL DEFAULT '',  -- Reply-To header address
  -- Per-type subject templates (empty = use built-in default)
  -- Supported placeholders: {{AppName}}, {{Title}}, {{RequestName}}
  ADD COLUMN IF NOT EXISTS email_subject_transfer_received      TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_subject_password_reset         TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_subject_email_verification     TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_subject_download_notification  TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_subject_expiry_reminder        TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_subject_transfer_revoked       TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_subject_request_submission     TEXT NOT NULL DEFAULT '',
  -- Per-type CTA button text (empty = use template default)
  ADD COLUMN IF NOT EXISTS email_cta_transfer_received      TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_cta_download_notification  TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_cta_password_reset         TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_cta_email_verification     TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_cta_expiry_reminder        TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_cta_request_submission     TEXT NOT NULL DEFAULT '';
