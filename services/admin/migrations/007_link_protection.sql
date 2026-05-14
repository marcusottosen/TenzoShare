-- Migration 007: link protection policy for transfers
-- Idempotent — safe to run multiple times.

ALTER TABLE admin_svc.platform_settings
    ADD COLUMN IF NOT EXISTS link_protection_policy TEXT NOT NULL DEFAULT 'none';

-- Add CHECK constraint idempotently
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'admin_svc.platform_settings'::regclass
          AND conname = 'platform_settings_link_policy'
    ) THEN
        ALTER TABLE admin_svc.platform_settings
            ADD CONSTRAINT platform_settings_link_policy
                CHECK (link_protection_policy IN ('none', 'password', 'email', 'either'));
    END IF;
END;
$$;

-- Valid values: none | password | email | either
-- none:     open links allowed (default)
-- password: transfer must have a password
-- email:    transfer must have at least one recipient email
-- either:   transfer must have a password OR at least one recipient email
