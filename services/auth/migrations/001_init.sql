-- Auth service initial schema
-- Run once on first startup (or via migration tool in Phase 2).

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ── Users ─────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS auth.users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL DEFAULT 'user',
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    email_verified BOOLEAN    NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_role_check   CHECK (role IN ('user', 'admin'))
);

CREATE INDEX IF NOT EXISTS users_email_idx ON auth.users (email);

-- ── Refresh tokens ────────────────────────────────────────────────────────────
-- We store a SHA-256 hash of the token, never the raw value.
CREATE TABLE IF NOT EXISTS auth.refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT refresh_tokens_hash_unique UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS refresh_tokens_user_idx     ON auth.refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS refresh_tokens_hash_idx     ON auth.refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS refresh_tokens_expires_idx  ON auth.refresh_tokens (expires_at);

-- ── MFA secrets ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS auth.mfa_secrets (
    user_id    UUID        PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    secret     TEXT        NOT NULL,     -- encrypted TOTP secret (AES-256-GCM)
    is_enabled BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Password reset tokens ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS auth.password_reset_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,              -- set when consumed; NULL = still valid
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT prt_hash_unique UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS prt_user_idx  ON auth.password_reset_tokens (user_id);
CREATE INDEX IF NOT EXISTS prt_hash_idx  ON auth.password_reset_tokens (token_hash);

-- ── updated_at trigger ────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION auth.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS users_updated_at ON auth.users;
CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON auth.users
    FOR EACH ROW EXECUTE FUNCTION auth.set_updated_at();
