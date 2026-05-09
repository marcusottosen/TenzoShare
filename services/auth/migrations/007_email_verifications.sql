-- Add require_email_verification flag to the settings singleton
ALTER TABLE auth.auth_settings
    ADD COLUMN IF NOT EXISTS require_email_verification BOOLEAN NOT NULL DEFAULT false;

-- Verification tokens issued after registration (TTL 24 h)
CREATE TABLE IF NOT EXISTS auth.email_verifications (
    token      TEXT        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ev_user_idx ON auth.email_verifications (user_id);
