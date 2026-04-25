-- Audit service initial schema
-- Stores an append-only log of all system events received via NATS AUDIT.* stream.

CREATE TABLE IF NOT EXISTS audit.audit_logs (
    id          UUID        NOT NULL DEFAULT gen_random_uuid(),
    source      TEXT        NOT NULL,   -- e.g. "auth", "transfer", "storage"
    action      TEXT        NOT NULL,   -- e.g. "login", "register", "transfer.created"
    user_id     UUID,                   -- NULL for anonymous / system events
    client_ip   TEXT,
    subject     TEXT        NOT NULL,   -- NATS subject that delivered this event
    payload     JSONB       NOT NULL,   -- raw event payload
    success     BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)        -- partition key must be in PK
) PARTITION BY RANGE (created_at);

-- Create initial monthly partitions (current month + next month)
DO $$
DECLARE
    start_date DATE := date_trunc('month', now());
    end_date   DATE := start_date + INTERVAL '1 month';
    next_end   DATE := end_date   + INTERVAL '1 month';
    part_name  TEXT;
BEGIN
    part_name := 'audit_logs_' || to_char(start_date, 'YYYY_MM');
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'audit' AND c.relname = part_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE audit.%I PARTITION OF audit.audit_logs FOR VALUES FROM (%L) TO (%L)',
            part_name, start_date, end_date
        );
    END IF;

    part_name := 'audit_logs_' || to_char(end_date, 'YYYY_MM');
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'audit' AND c.relname = part_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE audit.%I PARTITION OF audit.audit_logs FOR VALUES FROM (%L) TO (%L)',
            part_name, end_date, next_end
        );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS audit_logs_user_idx    ON audit.audit_logs (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS audit_logs_action_idx  ON audit.audit_logs (action);
CREATE INDEX IF NOT EXISTS audit_logs_source_idx  ON audit.audit_logs (source);
CREATE INDEX IF NOT EXISTS audit_logs_created_idx ON audit.audit_logs (created_at DESC);
