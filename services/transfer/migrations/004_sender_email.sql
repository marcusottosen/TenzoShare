-- 004_sender_email.sql
-- Store the sender's email on the transfer at creation time so it can be
-- surfaced to recipients on the public download page without a cross-service
-- DB join.

ALTER TABLE transfer.transfers
    ADD COLUMN IF NOT EXISTS sender_email TEXT NOT NULL DEFAULT '';
