-- v0.0.9-alpha Phase 4: post-deploy health verification.
-- Each deployment carries the result of the most recent health probe so the
-- UI can render "passing", "failing", or "pending" badges without re-running
-- the check on the read path. Status values:
--   * NULL     — no probe ran (no health_check_path configured, or pre-Phase-4)
--   * passing  — final probe returned 2xx within the verify window
--   * failing  — final probe never returned 2xx within the verify window
--   * skipped  — health_check_path was empty, intentionally not probed
ALTER TABLE deployments
    ADD COLUMN health_status     TEXT,
    ADD COLUMN health_message    TEXT,
    ADD COLUMN health_checked_at TIMESTAMPTZ;
