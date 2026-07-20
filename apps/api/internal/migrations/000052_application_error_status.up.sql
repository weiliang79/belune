-- applications.status was the only status column without a CHECK constraint —
-- type, build_type, and deployments.triggered_by all have one — so a typo in a
-- new code path would have been stored silently.
--
-- 'error' has been a defined constant since the beginning but was never written
-- anywhere: a failed deploy left the app on whatever it was before, so a
-- first-time app that failed to build still read "Inactive" and the failure was
-- visible only on the deployment record. It is now written when a deploy fails
-- terminally, and when a container exits non-zero (matching how databases
-- already distinguish a crash from a clean stop).

ALTER TABLE applications
    ADD CONSTRAINT applications_status_check
    CHECK (status IN ('inactive', 'running', 'stopped', 'error'));
