-- Add reminder_sent_at column to track whether an expiry reminder email has been sent.
ALTER TABLE transfer.transfers
    ADD COLUMN IF NOT EXISTS reminder_sent_at TIMESTAMPTZ;
