-- SMTP delivery settings (singleton row, id = 1).
-- smtp_password_enc is AES-256-GCM encrypted (base64); NULL = no SMTP authentication.
-- Env vars are the bootstrap defaults; a stored row overrides env on config-reload.
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
