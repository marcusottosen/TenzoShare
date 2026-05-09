-- Add per-user notification opt-out flag and fine-grained notification preferences.
ALTER TABLE auth.users
    ADD COLUMN IF NOT EXISTS notifications_opt_out BOOL    NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS notification_prefs    JSONB   NOT NULL DEFAULT '{}';
