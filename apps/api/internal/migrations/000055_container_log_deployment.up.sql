-- Tag each container log line with the deployment (image generation) that
-- produced its container, so the viewer can separate one run's logs from the
-- next across redeploy / rebuild / rollback. Nullable: database and system
-- (Caddy) logs have no deployment, and all pre-existing rows keep NULL and fall
-- into an "earlier" bucket in the UI.
ALTER TABLE container_logs
    ADD COLUMN deployment_id UUID;

-- Supports the session picker's per-deployment filter as well as the default
-- per-source, time-ordered read.
CREATE INDEX idx_container_logs_source_deployment
    ON container_logs (source_type, source_id, deployment_id, recorded_at DESC);
