-- Add human-readable name and optional description to transfers.
-- Expiry is now required (enforced at the service layer, not here),
-- but no DB-level constraint is added to avoid breaking existing rows.
ALTER TABLE transfer.transfers
    ADD COLUMN IF NOT EXISTS name        TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
