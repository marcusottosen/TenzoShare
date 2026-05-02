-- Branding settings (singleton row) for user-facing sites
CREATE TABLE IF NOT EXISTS admin_svc.branding_settings (
    id              INT         PRIMARY KEY DEFAULT 1,
    primary_color   VARCHAR(7)  NOT NULL DEFAULT '#1E293B',
    secondary_color VARCHAR(7)  NOT NULL DEFAULT '#0D9488',
    logo_data_url   TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT branding_settings_singleton CHECK (id = 1)
);
INSERT INTO admin_svc.branding_settings (id) VALUES (1) ON CONFLICT DO NOTHING;
