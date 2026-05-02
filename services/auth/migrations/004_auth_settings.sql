-- Auth settings singleton: configurable lockout policy
CREATE TABLE IF NOT EXISTS auth.auth_settings (
    id                      INT  PRIMARY KEY DEFAULT 1,
    max_failed_attempts     INT  NOT NULL DEFAULT 10,
    lockout_duration_minutes INT NOT NULL DEFAULT 15,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT auth_settings_singleton CHECK (id = 1)
);

-- Seed with defaults matching the previous hard-coded constants
INSERT INTO auth.auth_settings (id, max_failed_attempts, lockout_duration_minutes)
VALUES (1, 10, 15)
ON CONFLICT (id) DO NOTHING;
