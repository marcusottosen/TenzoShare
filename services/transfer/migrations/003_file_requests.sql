-- File request (dropbox) tables.
-- A file request is a public upload link that lets guests submit files to a registered user.

CREATE TABLE IF NOT EXISTS transfer.file_requests (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID        NOT NULL,
    slug            TEXT        NOT NULL UNIQUE,
    name            TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    allowed_types   TEXT        NOT NULL DEFAULT '',  -- comma-separated MIME prefixes, '' = all
    max_size_mb     INT         NOT NULL DEFAULT 0,   -- 0 = no limit
    max_files       INT         NOT NULL DEFAULT 0,   -- 0 = no limit
    expires_at      TIMESTAMPTZ NOT NULL,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS transfer.request_submissions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      UUID        NOT NULL REFERENCES transfer.file_requests(id) ON DELETE CASCADE,
    file_id         TEXT        NOT NULL,
    filename        TEXT        NOT NULL,
    size_bytes      BIGINT      NOT NULL DEFAULT 0,
    submitter_name  TEXT        NOT NULL DEFAULT '',
    message         TEXT        NOT NULL DEFAULT '',
    submitter_ip    TEXT        NOT NULL DEFAULT '',
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_file_requests_owner  ON transfer.file_requests(owner_id);
CREATE INDEX IF NOT EXISTS idx_file_requests_slug   ON transfer.file_requests(slug);
CREATE INDEX IF NOT EXISTS idx_request_submissions  ON transfer.request_submissions(request_id);
