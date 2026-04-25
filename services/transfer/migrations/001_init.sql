-- Transfer schema
-- A transfer links a set of uploaded files to a recipient (by email or link)
-- with optional password protection and expiry.
CREATE SCHEMA IF NOT EXISTS transfer;

CREATE TABLE IF NOT EXISTS transfer.transfers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID        NOT NULL,
    recipient_email TEXT,
    slug            TEXT        NOT NULL UNIQUE, -- short share link token
    password_hash   TEXT,                        -- NULL = no password required
    max_downloads   INT         NOT NULL DEFAULT 0,  -- 0 = unlimited
    download_count  INT         NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    is_revoked      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS transfer.transfer_files (
    transfer_id UUID NOT NULL REFERENCES transfer.transfers(id) ON DELETE CASCADE,
    file_id     UUID NOT NULL,
    PRIMARY KEY (transfer_id, file_id)
);

CREATE INDEX IF NOT EXISTS idx_transfers_owner    ON transfer.transfers(owner_id);
CREATE INDEX IF NOT EXISTS idx_transfers_slug     ON transfer.transfers(slug);
CREATE INDEX IF NOT EXISTS idx_transfers_expires  ON transfer.transfers(expires_at) WHERE expires_at IS NOT NULL;
