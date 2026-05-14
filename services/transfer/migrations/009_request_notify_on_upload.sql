ALTER TABLE transfer.file_requests
    ADD COLUMN IF NOT EXISTS notify_on_upload BOOLEAN NOT NULL DEFAULT TRUE;
