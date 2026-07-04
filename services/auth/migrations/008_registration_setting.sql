-- Add registration_enabled flag to auth_settings singleton
ALTER TABLE auth.auth_settings
    ADD COLUMN IF NOT EXISTS registration_enabled BOOLEAN NOT NULL DEFAULT true;
