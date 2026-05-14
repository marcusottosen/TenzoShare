-- Add configurable portal and download base URLs to the platform settings row.
-- These are used by the notification service to build email links.
ALTER TABLE admin_svc.platform_settings
  ADD COLUMN IF NOT EXISTS portal_url   TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS download_url TEXT NOT NULL DEFAULT '';
