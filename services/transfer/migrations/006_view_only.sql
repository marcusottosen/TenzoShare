-- 006_view_only.sql
-- Adds view-only mode to transfers.
--
-- A view-only transfer allows recipients to open and read files in the browser
-- but does not provide a save/download button. The server serves the file with
-- Content-Disposition: inline so the browser renders it in-page rather than
-- prompting a save-dialog.
--
-- max_downloads on a view-only transfer acts as "max views" — the semantics
-- (0 = unlimited, N = cap on accesses) remain identical.
--
-- Note: server-side enforcement (inline Content-Disposition) prevents casual
-- downloads. Determined users can still save via browser DevTools or screenshot.
-- This is intentional — view-only is a compliance and workflow aid, not DRM.

ALTER TABLE transfer.transfers
    ADD COLUMN IF NOT EXISTS view_only BOOLEAN NOT NULL DEFAULT false;
