-- Change tracking, so the UI can say "your saved config is not what is running"
-- instead of the seven hardcoded "takes effect on the next deploy" strings that
-- were shown unconditionally, whether or not anything had actually changed.
--
-- Two marker columns, because they clear asymmetrically. Needing a redeploy
-- implies needing a reload, so the severity is ordered rather than categorical;
-- what forces two fields is that a *reload* must clear one and not the other:
--
--   config_changed_at  env vars, volumes, file mounts, CPU/memory limits,
--                      runtime profile, container port, health-check config.
--                      A reload recreates the container from the image that is
--                      already there, which is enough to apply all of these.
--
--   source_changed_at  source_image, dockerfile_path, build_type_override,
--                      builder_image, branch, git credentials. Only a real
--                      build or pull applies these, so a reload must NOT clear
--                      it. Rollback is the proof: change source_image, then
--                      roll back, and you still need a deploy to get the new
--                      image.
--
-- Timestamps rather than booleans: a config edit made *during* a running deploy
-- would be wrongly cleared by a boolean. The clearing queries only clear a
-- marker older than the deployment's started_at, so a mid-deploy edit survives.
--
-- These cannot be derived from updated_at: UpdateApplicationStatus bumps it, so
-- a plain start/stop would falsely flag the application as changed.

ALTER TABLE applications
    ADD COLUMN config_changed_at TIMESTAMPTZ,
    ADD COLUMN source_changed_at TIMESTAMPTZ,
    -- Distinct from last_activity_at, which is NOT NULL DEFAULT NOW() (set at
    -- creation, for preview GC) and so cannot answer "has this ever deployed?".
    -- The markers are suppressed until this is set — otherwise a fresh app
    -- reads "needs redeploy" from birth, which is exactly the false positive
    -- that trains people to ignore the indicator.
    ADD COLUMN last_deployed_at TIMESTAMPTZ;
