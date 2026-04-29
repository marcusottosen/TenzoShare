-- Storage service: file retention policy fields and purge log

-- Add retention controls to the singleton settings row
ALTER TABLE storage.storage_settings
    ADD COLUMN IF NOT EXISTS retention_enabled          BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS retention_days             INTEGER     NOT NULL DEFAULT 30,   -- days after last share expires
    ADD COLUMN IF NOT EXISTS orphan_retention_days      INTEGER     NOT NULL DEFAULT 90;   -- days for files with no shares

-- Track what the cleanup worker actually deleted (for audit trail)
CREATE TABLE IF NOT EXISTS storage.file_purge_log (
    id              BIGSERIAL   PRIMARY KEY,
    file_id         UUID        NOT NULL,
    owner_id        UUID        NOT NULL,
    filename        TEXT        NOT NULL,
    size_bytes      BIGINT      NOT NULL DEFAULT 0,
    reason          TEXT        NOT NULL,  -- 'retention_expired' | 'orphan_expired' | 'admin_purge'
    purged_by       TEXT        NOT NULL DEFAULT 'system',
    purged_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_purge_log_purged_at ON storage.file_purge_log(purged_at DESC);
CREATE INDEX IF NOT EXISTS idx_purge_log_owner_id  ON storage.file_purge_log(owner_id);
