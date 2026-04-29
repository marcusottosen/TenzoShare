-- Audit service — severity level for each log entry.
-- Values: 'info' | 'warning' | 'error'
-- Derived at insert time by the consumer based on action name + success flag.

ALTER TABLE audit.audit_logs
    ADD COLUMN IF NOT EXISTS severity TEXT NOT NULL DEFAULT 'info';

-- Backfill existing rows:
--   success=false + _failed/_error     → error
--   success=false OR destructive action → warning
--   everything else                    → info
UPDATE audit.audit_logs
SET severity = CASE
    WHEN success = false
         AND (action LIKE '%_failed' OR action LIKE '%_error')
    THEN 'error'
    WHEN success = false
         OR action LIKE '%_deleted'
         OR action LIKE '%_purged'
         OR action LIKE '%_purge'
         OR action LIKE '%_revoked'
         OR action LIKE '%_terminated'
    THEN 'warning'
    ELSE 'info'
END;

CREATE INDEX IF NOT EXISTS audit_logs_severity_idx ON audit.audit_logs (severity);
