-- Migration 011: per-recipient magic link tokens for transfers shared by email.
-- When a transfer is created with recipient emails, each email gets its own
-- unique 32-byte random token embedded in the email link (?rt=<token>).
-- Presenting the token grants access without a password; it ties identity
-- to the email address rather than a shared secret.
--
-- One token per (transfer_id, email) pair; regenerating replaces the old one.
-- Tokens expire when the transfer expires.

CREATE TABLE IF NOT EXISTS transfer.recipient_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    transfer_id UUID        NOT NULL REFERENCES transfer.transfers(id) ON DELETE CASCADE,
    email       TEXT        NOT NULL,
    token_hash  TEXT        NOT NULL UNIQUE, -- hex(SHA-256(raw token))
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT recipient_tokens_email_length CHECK (char_length(email) <= 320)
);

-- One active token per (transfer, email) — upsert replaces the token.
CREATE UNIQUE INDEX IF NOT EXISTS idx_recipient_tokens_transfer_email
    ON transfer.recipient_tokens(transfer_id, email);

-- For purging expired tokens.
CREATE INDEX IF NOT EXISTS idx_recipient_tokens_expires_at
    ON transfer.recipient_tokens(expires_at);
