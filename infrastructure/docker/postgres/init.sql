-- =============================================================================
-- TenzoShare — PostgreSQL Initialization
-- Runs ONCE when the container data directory is first created (empty volume).
-- On subsequent starts this file is ignored by the postgres Docker entrypoint.
--
-- Purpose: create extensions, schemas, and all tables so that a fresh
-- `docker compose up` produces a fully functional database without waiting
-- for services to apply migrations.  All statements are idempotent.
--
-- Each Go service ALSO embeds its own migration SQL files and applies them
-- via database.RunMigrations() at startup (stored in <schema>.schema_migrations).
-- That mechanism handles incremental upgrades on existing installs.
-- =============================================================================

-- ── Extensions ───────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ── Schemas ───────────────────────────────────────────────────────────────────
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS transfer;
CREATE SCHEMA IF NOT EXISTS storage;
CREATE SCHEMA IF NOT EXISTS audit;
CREATE SCHEMA IF NOT EXISTS admin_svc;

DO $$
DECLARE
  app_user TEXT := current_user;
BEGIN
  EXECUTE format('GRANT ALL ON SCHEMA auth TO %I',      app_user);
  EXECUTE format('GRANT ALL ON SCHEMA transfer TO %I',  app_user);
  EXECUTE format('GRANT ALL ON SCHEMA storage TO %I',   app_user);
  EXECUTE format('GRANT ALL ON SCHEMA audit TO %I',     app_user);
  EXECUTE format('GRANT ALL ON SCHEMA admin_svc TO %I', app_user);
END
$$;

-- =============================================================================
-- AUTH SERVICE
-- =============================================================================

CREATE TABLE IF NOT EXISTS auth.schema_migrations (
    name       TEXT        PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth.users (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email                  TEXT        NOT NULL,
    password_hash          TEXT        NOT NULL,
    role                   TEXT        NOT NULL DEFAULT 'user',
    is_active              BOOLEAN     NOT NULL DEFAULT true,
    email_verified         BOOLEAN     NOT NULL DEFAULT false,
    failed_login_attempts  INT         NOT NULL DEFAULT 0,
    locked_until           TIMESTAMPTZ,
    last_login_at          TIMESTAMPTZ,
    date_format            TEXT        CHECK (date_format IN ('ISO', 'EU', 'US', 'DE', 'LONG')),
    time_format            TEXT        CHECK (time_format IN ('12h', '24h')),
    timezone               TEXT,
    notifications_opt_out  BOOL        NOT NULL DEFAULT false,
    notification_prefs     JSONB       NOT NULL DEFAULT '{}',
    auto_save_contacts     BOOLEAN     NOT NULL DEFAULT true,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_role_check   CHECK (role IN ('user', 'admin'))
);
CREATE INDEX IF NOT EXISTS users_email_idx ON auth.users (email);

CREATE TABLE IF NOT EXISTS auth.refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT refresh_tokens_hash_unique UNIQUE (token_hash)
);
CREATE INDEX IF NOT EXISTS refresh_tokens_user_idx    ON auth.refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS refresh_tokens_hash_idx    ON auth.refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS refresh_tokens_expires_idx ON auth.refresh_tokens (expires_at);

CREATE TABLE IF NOT EXISTS auth.mfa_secrets (
    user_id    UUID    PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    secret     TEXT    NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth.password_reset_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT prt_hash_unique UNIQUE (token_hash)
);
CREATE INDEX IF NOT EXISTS prt_user_idx ON auth.password_reset_tokens (user_id);
CREATE INDEX IF NOT EXISTS prt_hash_idx ON auth.password_reset_tokens (token_hash);

CREATE OR REPLACE FUNCTION auth.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$;
DROP TRIGGER IF EXISTS users_updated_at ON auth.users;
CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON auth.users
    FOR EACH ROW EXECUTE FUNCTION auth.set_updated_at();

CREATE TABLE IF NOT EXISTS auth.api_keys (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    key_hash   TEXT        NOT NULL,
    key_prefix TEXT        NOT NULL,
    last_used  TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT api_keys_hash_unique UNIQUE (key_hash)
);
CREATE INDEX IF NOT EXISTS api_keys_user_idx ON auth.api_keys (user_id);
CREATE INDEX IF NOT EXISTS api_keys_hash_idx ON auth.api_keys (key_hash);

CREATE TABLE IF NOT EXISTS auth.auth_settings (
    id                           INT         PRIMARY KEY DEFAULT 1,
    max_failed_attempts          INT         NOT NULL DEFAULT 10,
    lockout_duration_minutes     INT         NOT NULL DEFAULT 15,
    require_mfa                  BOOLEAN     NOT NULL DEFAULT false,
    require_email_verification   BOOLEAN     NOT NULL DEFAULT false,
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT auth_settings_singleton CHECK (id = 1)
);
INSERT INTO auth.auth_settings (id, max_failed_attempts, lockout_duration_minutes, require_mfa, require_email_verification)
VALUES (1, 10, 15, false, false) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS auth.email_verifications (
    token      TEXT        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ev_user_idx ON auth.email_verifications (user_id);

CREATE TABLE IF NOT EXISTS auth.contacts (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    email      TEXT        NOT NULL,
    name       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, email)
);
CREATE INDEX IF NOT EXISTS idx_contacts_user_id ON auth.contacts(user_id);

-- Mark all auth migrations applied so service startup skips re-running them.
INSERT INTO auth.schema_migrations (name) VALUES
  ('001_init.sql'), ('002_phase1.sql'), ('003_last_login.sql'), ('004_auth_settings.sql'), ('005_user_preferences.sql'), ('006_mfa_settings.sql'), ('007_email_verifications.sql'), ('008_notification_prefs.sql'), ('009_contacts.sql')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- TRANSFER SERVICE
-- =============================================================================

CREATE TABLE IF NOT EXISTS transfer.schema_migrations (
    name       TEXT        PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transfer.transfers (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID        NOT NULL,
    recipient_email TEXT,
    slug            TEXT        NOT NULL UNIQUE,
    password_hash   TEXT,
    max_downloads   INT         NOT NULL DEFAULT 0,
    download_count  INT         NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    is_revoked      BOOLEAN     NOT NULL DEFAULT false,
    name            TEXT        NOT NULL DEFAULT '',
    description     TEXT        NOT NULL DEFAULT '',
    sender_email    TEXT        NOT NULL DEFAULT '',
    view_only          BOOLEAN     NOT NULL DEFAULT false,
    reminder_sent_at   TIMESTAMPTZ,
    notify_on_download BOOLEAN     NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_transfers_owner   ON transfer.transfers (owner_id);
CREATE INDEX IF NOT EXISTS idx_transfers_slug    ON transfer.transfers (slug);
CREATE INDEX IF NOT EXISTS idx_transfers_expires ON transfer.transfers (expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS transfer.transfer_files (
    transfer_id UUID NOT NULL REFERENCES transfer.transfers(id) ON DELETE CASCADE,
    file_id     UUID NOT NULL,
    PRIMARY KEY (transfer_id, file_id)
);

CREATE TABLE IF NOT EXISTS transfer.file_requests (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      UUID        NOT NULL,
    slug          TEXT        NOT NULL UNIQUE,
    name          TEXT        NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    allowed_types TEXT        NOT NULL DEFAULT '',
    max_size_mb   INT         NOT NULL DEFAULT 0,
    max_files     INT         NOT NULL DEFAULT 0,
    notify_emails    TEXT        NOT NULL DEFAULT '',
    notify_on_upload BOOLEAN     NOT NULL DEFAULT true,
    expires_at       TIMESTAMPTZ NOT NULL,
    is_active        BOOLEAN     NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_file_requests_owner ON transfer.file_requests (owner_id);
CREATE INDEX IF NOT EXISTS idx_file_requests_slug  ON transfer.file_requests (slug);

CREATE TABLE IF NOT EXISTS transfer.request_submissions (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id     UUID        NOT NULL REFERENCES transfer.file_requests(id) ON DELETE CASCADE,
    file_id        TEXT        NOT NULL,
    filename       TEXT        NOT NULL,
    size_bytes     BIGINT      NOT NULL DEFAULT 0,
    submitter_name TEXT        NOT NULL DEFAULT '',
    message        TEXT        NOT NULL DEFAULT '',
    submitter_ip   TEXT        NOT NULL DEFAULT '',
    submitted_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_request_submissions ON transfer.request_submissions (request_id);

CREATE TABLE IF NOT EXISTS transfer.file_download_counts (
    transfer_id        UUID        NOT NULL REFERENCES transfer.transfers(id) ON DELETE CASCADE,
    file_id            UUID        NOT NULL,
    count              INTEGER     NOT NULL DEFAULT 1 CHECK (count >= 1),
    last_downloaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (transfer_id, file_id)
);
CREATE INDEX IF NOT EXISTS idx_file_dl_counts_transfer ON transfer.file_download_counts (transfer_id);

INSERT INTO transfer.schema_migrations (name) VALUES
  ('001_init.sql'), ('002_add_name_description.sql'), ('003_file_requests.sql'),
  ('004_sender_email.sql'), ('005_file_download_counts.sql'), ('006_view_only.sql'),
  ('007_reminder_sent_at.sql'), ('008_request_notify_emails.sql'),
  ('009_request_notify_on_upload.sql'), ('010_transfer_notify_on_download.sql')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- STORAGE SERVICE
-- =============================================================================

CREATE TABLE IF NOT EXISTS storage.schema_migrations (
    name       TEXT        PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS storage.files (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      UUID        NOT NULL,
    object_key    TEXT        NOT NULL,
    filename      TEXT        NOT NULL,
    content_type  TEXT        NOT NULL DEFAULT 'application/octet-stream',
    size_bytes    BIGINT      NOT NULL DEFAULT 0,
    is_encrypted  BOOLEAN     NOT NULL DEFAULT true,
    encryption_iv BYTEA,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    CONSTRAINT files_object_key_unique UNIQUE (object_key)
);
CREATE INDEX IF NOT EXISTS files_owner_idx   ON storage.files (owner_id);
CREATE INDEX IF NOT EXISTS files_deleted_idx ON storage.files (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS files_created_idx ON storage.files (created_at DESC);

CREATE TABLE IF NOT EXISTS storage.storage_settings (
    id                    INT         PRIMARY KEY DEFAULT 1,
    quota_enabled         BOOLEAN     NOT NULL DEFAULT false,
    quota_bytes_per_user  BIGINT      NOT NULL DEFAULT 10737418240,
    max_upload_size_bytes BIGINT      NOT NULL DEFAULT 0,
    retention_enabled     BOOLEAN     NOT NULL DEFAULT false,
    retention_days        INTEGER     NOT NULL DEFAULT 30,
    orphan_retention_days INTEGER     NOT NULL DEFAULT 90,
    test_mode             BOOLEAN     NOT NULL DEFAULT false,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by            TEXT        NOT NULL DEFAULT 'system',
    CONSTRAINT storage_settings_singleton CHECK (id = 1)
);
INSERT INTO storage.storage_settings (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS storage.file_purge_log (
    id         BIGSERIAL   PRIMARY KEY,
    file_id    UUID        NOT NULL,
    owner_id   UUID        NOT NULL,
    filename   TEXT        NOT NULL,
    size_bytes BIGINT      NOT NULL DEFAULT 0,
    reason     TEXT        NOT NULL,
    purged_by  TEXT        NOT NULL DEFAULT 'system',
    purged_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_purge_log_purged_at ON storage.file_purge_log (purged_at DESC);
CREATE INDEX IF NOT EXISTS idx_purge_log_owner_id  ON storage.file_purge_log (owner_id);

INSERT INTO storage.schema_migrations (name) VALUES
  ('001_init.sql'), ('002_add_encryption_iv.sql'), ('003_storage_settings.sql'),
  ('004_file_retention.sql'), ('005_test_mode.sql')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- AUDIT SERVICE
-- =============================================================================

CREATE TABLE IF NOT EXISTS audit.schema_migrations (
    name       TEXT        PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit.audit_logs (
    id         UUID        NOT NULL DEFAULT gen_random_uuid(),
    source     TEXT        NOT NULL,
    action     TEXT        NOT NULL,
    user_id    UUID,
    client_ip  TEXT,
    subject    TEXT        NOT NULL,
    payload    JSONB       NOT NULL,
    success    BOOLEAN     NOT NULL DEFAULT true,
    severity   TEXT        NOT NULL DEFAULT 'info',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

DO $$
DECLARE
    start_date DATE := date_trunc('month', now());
    end_date   DATE := start_date + INTERVAL '1 month';
    next_end   DATE := end_date   + INTERVAL '1 month';
    part_name  TEXT;
BEGIN
    part_name := 'audit_logs_' || to_char(start_date, 'YYYY_MM');
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'audit' AND c.relname = part_name
    ) THEN
        EXECUTE format('CREATE TABLE audit.%I PARTITION OF audit.audit_logs FOR VALUES FROM (%L) TO (%L)',
            part_name, start_date, end_date);
    END IF;

    part_name := 'audit_logs_' || to_char(end_date, 'YYYY_MM');
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'audit' AND c.relname = part_name
    ) THEN
        EXECUTE format('CREATE TABLE audit.%I PARTITION OF audit.audit_logs FOR VALUES FROM (%L) TO (%L)',
            part_name, end_date, next_end);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS audit_logs_user_idx     ON audit.audit_logs (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS audit_logs_action_idx   ON audit.audit_logs (action);
CREATE INDEX IF NOT EXISTS audit_logs_source_idx   ON audit.audit_logs (source);
CREATE INDEX IF NOT EXISTS audit_logs_created_idx  ON audit.audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS audit_logs_severity_idx ON audit.audit_logs (severity);

CREATE TABLE IF NOT EXISTS audit.audit_settings (
    id                INT         PRIMARY KEY DEFAULT 1,
    retention_enabled BOOLEAN     NOT NULL DEFAULT true,
    retention_days    INT         NOT NULL DEFAULT 365,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        TEXT        NOT NULL DEFAULT 'system',
    CONSTRAINT audit_settings_singleton CHECK (id = 1)
);
INSERT INTO audit.audit_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

INSERT INTO audit.schema_migrations (name) VALUES
  ('001_init.sql'), ('002_audit_settings.sql'), ('003_audit_severity.sql')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- ADMIN SERVICE
-- =============================================================================

CREATE TABLE IF NOT EXISTS admin_svc.schema_migrations (
    name       TEXT        PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_svc.branding_settings (
    id                 INT          PRIMARY KEY DEFAULT 1,
    primary_color      VARCHAR(7)   NOT NULL DEFAULT '#1E293B',
    secondary_color    VARCHAR(7)   NOT NULL DEFAULT '#0D9488',
    logo_data_url      TEXT,
    page_bg_color      VARCHAR(7)   NOT NULL DEFAULT '#F7F9FB',
    surface_color      VARCHAR(7)   NOT NULL DEFAULT '#FFFFFF',
    text_color         VARCHAR(7)   NOT NULL DEFAULT '#091426',
    border_radius      SMALLINT     NOT NULL DEFAULT 6,
    app_name           VARCHAR(100) NOT NULL DEFAULT 'TenzoShare',
    custom_css         TEXT,
    dm_primary_color   VARCHAR(7),
    dm_secondary_color VARCHAR(7),
    dm_page_bg_color   VARCHAR(7),
    dm_surface_color   VARCHAR(7),
    dm_text_color      VARCHAR(7),
    email_sender_name    TEXT        NOT NULL DEFAULT '',
    email_support_email  TEXT        NOT NULL DEFAULT '',
    email_footer_text    TEXT        NOT NULL DEFAULT '',
    email_subject_prefix TEXT        NOT NULL DEFAULT '',
    email_header_link    TEXT        NOT NULL DEFAULT '',
    email_reply_to               TEXT NOT NULL DEFAULT '',
    email_button_color           TEXT NOT NULL DEFAULT '',
    email_button_text_color      TEXT NOT NULL DEFAULT '',
    email_body_bg_color          TEXT NOT NULL DEFAULT '',
    email_card_bg_color          TEXT NOT NULL DEFAULT '',
    email_card_border_color      TEXT NOT NULL DEFAULT '',
    email_heading_color          TEXT NOT NULL DEFAULT '',
    email_text_color             TEXT NOT NULL DEFAULT '',
    email_subject_transfer_received     TEXT NOT NULL DEFAULT '',
    email_subject_password_reset        TEXT NOT NULL DEFAULT '',
    email_subject_email_verification    TEXT NOT NULL DEFAULT '',
    email_subject_download_notification TEXT NOT NULL DEFAULT '',
    email_subject_expiry_reminder       TEXT NOT NULL DEFAULT '',
    email_subject_transfer_revoked      TEXT NOT NULL DEFAULT '',
    email_subject_request_submission    TEXT NOT NULL DEFAULT '',
    email_cta_transfer_received     TEXT NOT NULL DEFAULT '',
    email_cta_download_notification  TEXT NOT NULL DEFAULT '',
    email_cta_password_reset         TEXT NOT NULL DEFAULT '',
    email_cta_email_verification     TEXT NOT NULL DEFAULT '',
    email_cta_expiry_reminder        TEXT NOT NULL DEFAULT '',
    email_cta_request_submission     TEXT NOT NULL DEFAULT '',
    email_custom_transfer_received      TEXT NOT NULL DEFAULT '',
    email_custom_password_reset         TEXT NOT NULL DEFAULT '',
    email_custom_email_verification     TEXT NOT NULL DEFAULT '',
    email_custom_download_notification  TEXT NOT NULL DEFAULT '',
    email_custom_expiry_reminder        TEXT NOT NULL DEFAULT '',
    email_custom_transfer_revoked       TEXT NOT NULL DEFAULT '',
    email_custom_request_submission     TEXT NOT NULL DEFAULT '',
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT branding_settings_singleton CHECK (id = 1)
);
INSERT INTO admin_svc.branding_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS admin_svc.platform_settings (
    id                      INT         PRIMARY KEY DEFAULT 1,
    date_format             TEXT        NOT NULL DEFAULT 'EU',
    time_format             TEXT        NOT NULL DEFAULT '24h',
    timezone                TEXT        NOT NULL DEFAULT 'UTC',
    portal_url              TEXT        NOT NULL DEFAULT '',
    download_url            TEXT        NOT NULL DEFAULT '',
    link_protection_policy  TEXT        NOT NULL DEFAULT 'none',
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT platform_settings_singleton   CHECK (id = 1),
    CONSTRAINT platform_settings_date_format CHECK (date_format IN ('ISO', 'EU', 'US', 'DE', 'LONG')),
    CONSTRAINT platform_settings_time_format CHECK (time_format IN ('12h', '24h')),
    CONSTRAINT platform_settings_link_policy CHECK (link_protection_policy IN ('none', 'password', 'email', 'either'))
);
INSERT INTO admin_svc.platform_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS admin_svc.smtp_settings (
    id                INT     PRIMARY KEY DEFAULT 1,
    smtp_host         TEXT    NOT NULL DEFAULT '',
    smtp_port         TEXT    NOT NULL DEFAULT '1025',
    smtp_username     TEXT    NOT NULL DEFAULT '',
    smtp_password_enc TEXT             DEFAULT NULL,
    smtp_from         TEXT    NOT NULL DEFAULT '',
    smtp_use_tls      BOOL    NOT NULL DEFAULT false,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT smtp_settings_singleton CHECK (id = 1)
);
INSERT INTO admin_svc.smtp_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

INSERT INTO admin_svc.schema_migrations (name) VALUES
  ('001_branding_settings.sql'), ('002_branding_extend.sql'), ('003_branding_dark_mode.sql'), ('004_platform_settings.sql'), ('005_smtp_settings.sql'), ('006_platform_urls.sql'), ('007_link_protection.sql'), ('007_email_branding.sql'), ('008_email_content.sql'), ('009_custom_email_templates.sql')
ON CONFLICT DO NOTHING;
