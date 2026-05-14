-- Add require_mfa flag to the auth_settings singleton.
-- When true, users without MFA configured are forced to set it up after login.
ALTER TABLE auth.auth_settings
    ADD COLUMN IF NOT EXISTS require_mfa BOOLEAN NOT NULL DEFAULT false;
