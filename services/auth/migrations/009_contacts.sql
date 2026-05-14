-- Migration 009: per-user contacts list + auto_save_contacts preference
-- Idempotent — safe to run multiple times.

CREATE TABLE IF NOT EXISTS auth.contacts (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    email      TEXT        NOT NULL,
    name       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, email)
);

CREATE INDEX IF NOT EXISTS idx_contacts_user_id ON auth.contacts(user_id);

ALTER TABLE auth.users
    ADD COLUMN IF NOT EXISTS auto_save_contacts BOOLEAN NOT NULL DEFAULT TRUE;
