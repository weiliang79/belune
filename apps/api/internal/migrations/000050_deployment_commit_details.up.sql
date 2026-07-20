-- Push webhooks from GitHub, Gitea, and GitLab all carry the commit's message
-- and author alongside the SHA, but we only ever kept the SHA — so the
-- deployments list could show "abc1234" and nothing else. These columns keep
-- the provenance the provider already handed us.
--
-- Display-only, and deliberately nullable: manual, reload, rebuild, template,
-- and deploy-hook deployments have no upstream commit to describe. The message
-- is capped in application code (1000 chars) rather than by a column width, so
-- a long commit body is truncated with an ellipsis instead of erroring the
-- insert and failing an otherwise-fine deploy.

ALTER TABLE deployments
    ADD COLUMN commit_message TEXT,
    ADD COLUMN commit_author TEXT;
