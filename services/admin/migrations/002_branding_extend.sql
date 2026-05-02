-- Extend branding_settings with additional customization options.
ALTER TABLE admin_svc.branding_settings
  ADD COLUMN IF NOT EXISTS page_bg_color  VARCHAR(7)   NOT NULL DEFAULT '#F7F9FB',
  ADD COLUMN IF NOT EXISTS surface_color  VARCHAR(7)   NOT NULL DEFAULT '#FFFFFF',
  ADD COLUMN IF NOT EXISTS text_color     VARCHAR(7)   NOT NULL DEFAULT '#091426',
  ADD COLUMN IF NOT EXISTS border_radius  SMALLINT     NOT NULL DEFAULT 6,
  ADD COLUMN IF NOT EXISTS app_name       VARCHAR(100) NOT NULL DEFAULT 'TenzoShare',
  ADD COLUMN IF NOT EXISTS custom_css     TEXT;
