-- Per-user storage quota overrides.
-- When a row exists for a user_id the value in quota_bytes overrides the
-- global quota_bytes_per_user setting in storage.storage_settings.
-- A missing row means "use the global default".
CREATE TABLE IF NOT EXISTS admin_svc.user_quotas (
    user_id     UUID        PRIMARY KEY,
    quota_bytes BIGINT      NOT NULL CHECK (quota_bytes > 0),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  TEXT        NOT NULL DEFAULT ''
);
