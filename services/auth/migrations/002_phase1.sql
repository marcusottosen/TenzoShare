-- Auth service — Phase 1 schema additions
-- Idempotent: all statements use IF NOT EXISTS / DO NOTHING.

-- ── Login security columns ────────────────────────────────────────────────────
-- Track consecutive failures for account lockout.
ALTER TABLE auth.users
    ADD COLUMN IF NOT EXISTS failed_login_attempts INT         NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS locked_until          TIMESTAMPTZ;

-- ── API keys ──────────────────────────────────────────────────────────────────
-- Personal access tokens for programmatic / CLI use.
CREATE TABLE IF NOT EXISTS auth.api_keys (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,                 -- human-readable label
    key_hash    TEXT        NOT NULL,                 -- SHA-256 of the raw key (never stored raw)
    key_prefix  TEXT        NOT NULL,                 -- first 8 chars of raw key, shown in UI
    last_used   TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ,                          -- NULL = never expires
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT api_keys_hash_unique UNIQUE (key_hash)
);

CREATE INDEX IF NOT EXISTS api_keys_user_idx ON auth.api_keys (user_id);
CREATE INDEX IF NOT EXISTS api_keys_hash_idx ON auth.api_keys (key_hash);
