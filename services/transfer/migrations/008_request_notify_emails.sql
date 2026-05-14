-- Add notify_emails to file_requests so owners can send the upload link directly.
ALTER TABLE transfer.file_requests
    ADD COLUMN IF NOT EXISTS notify_emails TEXT NOT NULL DEFAULT '';
