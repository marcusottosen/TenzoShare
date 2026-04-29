-- Audit service — log retention settings
-- Singleton row (id = 1) stores the global audit log retention policy.

CREATE TABLE IF NOT EXISTS audit.audit_settings (
    id                  INT         PRIMARY KEY DEFAULT 1,
    retention_enabled   BOOLEAN     NOT NULL DEFAULT true,
    retention_days      INT         NOT NULL DEFAULT 365,  -- 1 year default (SOC 2 / NIS2 baseline)
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          TEXT        NOT NULL DEFAULT 'system',
    CONSTRAINT audit_settings_singleton CHECK (id = 1)
);

-- Ensure the singleton row exists.
INSERT INTO audit.audit_settings (id) VALUES (1) ON CONFLICT DO NOTHING;
