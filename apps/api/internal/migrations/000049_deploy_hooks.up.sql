-- Deploy hooks: a per-application token that turns POST /api/webhooks/deploy/{token}
-- into a deploy trigger. Git apps already auto-deploy from push webhooks (matched by
-- payload repo URL); image apps had no automatic path at all, so CI that pushes a new
-- image had no way to tell us about it. The hook covers both types — the caller decides
-- when to fire, so there is deliberately no branch logic here.
--
-- Two columns for one secret, each serving a different access pattern:
--   hash      — SHA-256 of the token, looked up on every trigger request. Indexed so
--               the public endpoint is an O(1) probe and never needs to scan/decrypt.
--   encrypted — keyring-encrypted copy so the UI can reveal the token on demand
--               (the file-mount pattern) instead of a show-once-then-lost flow.
-- Both NULL = hook disabled, which is the default for every existing application.

ALTER TABLE applications
    ADD COLUMN deploy_hook_token_hash BYTEA,
    ADD COLUMN deploy_hook_token_encrypted BYTEA;

-- Unique: a token identifies exactly one application. Partial so the (many) rows with
-- the hook disabled do not collide with each other on NULL.
CREATE UNIQUE INDEX idx_applications_deploy_hook_token_hash
    ON applications (deploy_hook_token_hash)
    WHERE deploy_hook_token_hash IS NOT NULL;

-- Hook-triggered deployments get their own triggered_by value so the deployments
-- list distinguishes them from a manual click or a git push. Without this the
-- insert fails the check constraint and the trigger endpoint can never deploy.
ALTER TABLE deployments DROP CONSTRAINT deployments_triggered_by_check;
ALTER TABLE deployments ADD CONSTRAINT deployments_triggered_by_check
    CHECK (triggered_by IN ('push', 'manual', 'api', 'rollback', 'reload', 'rebuild', 'template', 'hook'));
