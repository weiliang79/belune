-- Rework token pinning from one project to a set of projects.
--
-- Pinning has never been creatable in any released version — CreateAPIToken
-- never accepted project_id until the PR that ships alongside this migration
-- — so no pinned token exists in any install anywhere. That makes this the
-- only window where the pin's shape is free to change; once a real token can
-- carry one, reshaping it means either a live-data migration (forbidden for
-- v0.1.x, since update.sh runs migrations against real installs) or two
-- parallel mechanisms forever. This is that reshape, done now while it costs
-- nothing.
--
-- pinned is a property of the TOKEN, not inferred from row count. The
-- obvious alternative — zero rows in the join table means unpinned — has a
-- silent privilege-escalation path: today project_id is ON DELETE CASCADE
-- on api_tokens, so deleting the pinned project deletes the token with it.
-- With a join table alone, deleting a project only removes that one pin row;
-- deleting the LAST pin would leave a "pinned" token with zero rows, and
-- under "zero rows = unpinned" that reads as full reach across every project
-- its owner can see. Deleting a project would silently WIDEN a credential,
-- and nobody would ever notice. pinned=true with zero matching rows must
-- instead reach NOTHING — an empty intersection, not a wildcard — so pinning
-- has to be recorded explicitly rather than derived.
ALTER TABLE api_tokens ADD COLUMN pinned BOOLEAN NOT NULL DEFAULT false;

-- No backfill: every existing token is unpinned, which pinned DEFAULT false
-- already expresses correctly for every row written before this column
-- existed.

-- api_tokens.project_id is superseded by api_token_projects below and is now
-- vestigial. It ships in migration 000066 (v0.1.6) and a forward-only line
-- cannot drop it, so it stays forever unread and unwritten. Do not wire it
-- back up.
COMMENT ON COLUMN api_tokens.project_id IS
    'Superseded by api_token_projects + api_tokens.pinned (migration 000067). Never read, never written.';

CREATE TABLE api_token_projects (
    token_id   UUID NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    PRIMARY KEY (token_id, project_id)
);

CREATE INDEX idx_api_token_projects_token ON api_token_projects(token_id);
-- Postgres does not index a foreign key's referencing column automatically —
-- only the PK/unique side gets one for free. Without this, deleting a
-- project (the ON DELETE CASCADE side) sequentially scans this table to find
-- rows to remove, a cost that grows with the whole table rather than with
-- the deleted project's own pins.
CREATE INDEX idx_api_token_projects_project ON api_token_projects(project_id);
