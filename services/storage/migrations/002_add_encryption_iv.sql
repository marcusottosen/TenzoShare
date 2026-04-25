-- Add encryption_iv column to store the AES-256-GCM nonce alongside each file.
-- The nonce is 12 bytes (96 bits) and is unique per file.
ALTER TABLE storage.files ADD COLUMN IF NOT EXISTS encryption_iv BYTEA;
