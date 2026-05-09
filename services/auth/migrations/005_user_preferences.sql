-- Per-user date/time format overrides; NULL = use system default from platform_settings.
ALTER TABLE auth.users
    ADD COLUMN IF NOT EXISTS date_format TEXT CHECK (date_format IN ('ISO', 'EU', 'US', 'DE', 'LONG')),
    ADD COLUMN IF NOT EXISTS time_format TEXT CHECK (time_format IN ('12h', '24h')),
    ADD COLUMN IF NOT EXISTS timezone    TEXT;
