-- Storage service: singleton settings row for admin-controlled storage policy

CREATE TABLE IF NOT EXISTS storage.storage_settings (
    id                    INT         PRIMARY KEY DEFAULT 1,
    quota_enabled         BOOLEAN     NOT NULL DEFAULT false,
    quota_bytes_per_user  BIGINT      NOT NULL DEFAULT 10737418240,  -- 10 GiB default
    max_upload_size_bytes BIGINT      NOT NULL DEFAULT 0,            -- 0 = unlimited
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by            TEXT        NOT NULL DEFAULT 'system',

    -- Enforce singleton: only row with id=1 is ever allowed
    CONSTRAINT storage_settings_singleton CHECK (id = 1)
);

-- Seed the one-and-only config row on first migration
INSERT INTO storage.storage_settings (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;
