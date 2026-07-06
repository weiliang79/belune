-- Reload/Rebuild deployment triggers + image digest pinning.
--
-- 1. Allow 'reload' (recreate from current image, skip build) and 'rebuild'
--    (rebuild the currently-deployed commit) as deployment trigger sources.
-- 2. Pin managed-database images to a resolved @sha256 digest so recreates
--    (external-access toggle, restart, reconfigure) reuse the exact image
--    instead of silently following a mutable tag (e.g. postgres:18 -> 18.1).
--    NULL means "not yet pinned"; an explicit Upgrade clears + re-pins it.

ALTER TABLE deployments DROP CONSTRAINT deployments_triggered_by_check;
ALTER TABLE deployments ADD CONSTRAINT deployments_triggered_by_check
    CHECK (triggered_by IN ('push', 'manual', 'api', 'rollback', 'reload', 'rebuild'));

ALTER TABLE databases ADD COLUMN image_digest TEXT;
