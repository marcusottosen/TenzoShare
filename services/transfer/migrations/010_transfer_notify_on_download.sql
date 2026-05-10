ALTER TABLE transfer.transfers
    ADD COLUMN IF NOT EXISTS notify_on_download BOOLEAN NOT NULL DEFAULT TRUE;
