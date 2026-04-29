-- Add test_mode flag to storage settings.
-- When true, plain-HTTP uploads are accepted (development / test environments).
-- When false (default / production), the storage service rejects uploads that
-- arrive over plain HTTP and requires HTTPS end-to-end.

ALTER TABLE storage.storage_settings
    ADD COLUMN IF NOT EXISTS test_mode BOOLEAN NOT NULL DEFAULT false;
