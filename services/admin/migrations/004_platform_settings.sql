-- Platform-wide date/time formatting defaults (singleton row, id = 1).
CREATE TABLE IF NOT EXISTS admin_svc.platform_settings (
    id          INT         PRIMARY KEY DEFAULT 1,
    date_format TEXT        NOT NULL DEFAULT 'EU',
    time_format TEXT        NOT NULL DEFAULT '24h',
    timezone    TEXT        NOT NULL DEFAULT 'UTC',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT platform_settings_singleton   CHECK (id = 1),
    CONSTRAINT platform_settings_date_format CHECK (date_format IN ('ISO', 'EU', 'US', 'DE', 'LONG')),
    CONSTRAINT platform_settings_time_format CHECK (time_format IN ('12h', '24h'))
);
INSERT INTO admin_svc.platform_settings (id) VALUES (1) ON CONFLICT DO NOTHING;
