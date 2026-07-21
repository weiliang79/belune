-- Make the container generation, not the deployment, the log session key.
--
-- deployment_id (000055) only works for applications: databases and system
-- containers have no deployment row, so their logs could never be separated into
-- sessions — yet databases genuinely get new container generations (a major
-- version upgrade provisions a new container, and again on rollback).
--
-- Every source type has a container, and Docker already gives each generation a
-- unique id, so container_id is the key that works for all of them.
-- deployment_id stays as enrichment: it is what lets an application's session be
-- labelled with its trigger and commit rather than just a time range.
ALTER TABLE container_logs
    ADD COLUMN container_id TEXT;

-- Replaces the deployment-keyed index: the session filter is now by
-- container_id, so keeping both would pay write cost on the busiest table for an
-- index nothing queries.
DROP INDEX IF EXISTS idx_container_logs_source_deployment;

CREATE INDEX idx_container_logs_source_container
    ON container_logs (source_type, source_id, container_id, recorded_at DESC);
