-- Allow 'template' as a deployment trigger. The template:finalize worker creates
-- the first deploy of each app instantiated from a catalog template; without
-- this the insert fails the triggered_by check constraint and the app never
-- auto-deploys. Mirrors the 000033 extension pattern.
ALTER TABLE deployments DROP CONSTRAINT deployments_triggered_by_check;
ALTER TABLE deployments ADD CONSTRAINT deployments_triggered_by_check
    CHECK (triggered_by IN ('push', 'manual', 'api', 'rollback', 'reload', 'rebuild', 'template'));
