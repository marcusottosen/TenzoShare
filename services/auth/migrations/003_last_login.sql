-- Auth service — add last_login_at tracking
-- Idempotent: uses IF NOT EXISTS.

ALTER TABLE auth.users
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
