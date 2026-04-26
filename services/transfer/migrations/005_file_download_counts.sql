-- Per-file download tracking for enforcing download limits per individual file.
--
-- The previous approach tracked a single global download_count on the transfers
-- table and checked against (max_downloads × file_count). This allowed a recipient
-- to exhaust the quota by downloading the same file multiple times, locking other
-- files. This table replaces that enforcement with per-file tracking.
--
-- Enforcement: before issuing a presigned URL for file F in transfer T, the service
-- atomically executes an INSERT … ON CONFLICT DO UPDATE … WHERE count < max_downloads.
-- If the WHERE clause is not satisfied no row is returned and the download is denied.
-- This is race-safe under concurrent requests.
--
-- The global download_count on transfer.transfers is kept as an informational total
-- (incremented on each actual file download) but is no longer used for limit enforcement.

CREATE TABLE IF NOT EXISTS transfer.file_download_counts (
    transfer_id        UUID        NOT NULL REFERENCES transfer.transfers(id) ON DELETE CASCADE,
    file_id            UUID        NOT NULL,
    count              INTEGER     NOT NULL DEFAULT 1 CHECK (count >= 1),
    last_downloaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (transfer_id, file_id)
);

CREATE INDEX IF NOT EXISTS idx_file_dl_counts_transfer ON transfer.file_download_counts(transfer_id);
