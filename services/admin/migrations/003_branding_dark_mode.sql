-- Add optional dark-mode colour overrides to branding settings.
-- All columns are nullable; NULL means "use the built-in dark theme default".
ALTER TABLE admin_svc.branding_settings
  ADD COLUMN IF NOT EXISTS dm_primary_color   VARCHAR(7),
  ADD COLUMN IF NOT EXISTS dm_secondary_color VARCHAR(7),
  ADD COLUMN IF NOT EXISTS dm_page_bg_color   VARCHAR(7),
  ADD COLUMN IF NOT EXISTS dm_surface_color   VARCHAR(7),
  ADD COLUMN IF NOT EXISTS dm_text_color      VARCHAR(7);
