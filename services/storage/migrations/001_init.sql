-- Storage service initial schema

CREATE TABLE IF NOT EXISTS storage.files (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id     UUID        NOT NULL,           -- auth.users.id (no FK across services)
    object_key   TEXT        NOT NULL,           -- S3/MinIO key (e.g. "uploads/<uuid>/<filename>")
    filename     TEXT        NOT NULL,           -- original filename
    content_type TEXT        NOT NULL DEFAULT 'application/octet-stream',
    size_bytes   BIGINT      NOT NULL DEFAULT 0,
    is_encrypted BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,                   -- soft delete

    CONSTRAINT files_object_key_unique UNIQUE (object_key)
);

CREATE INDEX IF NOT EXISTS files_owner_idx      ON storage.files (owner_id);
CREATE INDEX IF NOT EXISTS files_deleted_idx    ON storage.files (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS files_created_idx    ON storage.files (created_at DESC);
