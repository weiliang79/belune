-- Personal access tokens (0.1.x #1), plus the audit attribution they need.
--
-- token_hash is SHA-256, not encrypted: a 256-bit random secret needs no slow
-- hash, and a plain hash is indexable for lookup — unlike bcrypt, which is a
-- one-way compare, not something you can SELECT by. cmd/rewrap needs no new
-- target for this column; there is nothing here for key rotation to touch.
--
-- role_at_issue is the role the owner held when the token was created.
-- Capability only ever SHRINKS from there — the auth path takes
-- min(role_at_issue, the owner's role right now), so a token cannot silently
-- gain access when its owner is promoted later.
--
-- project_id is nullable: NULL means every project the owner can currently
-- reach, evaluated at use time. A token pinned to one project whose owner
-- later loses ACCESS to it (unshared, say) is simply an empty intersection at
-- enforcement time, not a special case.
--
-- ON DELETE CASCADE, deliberately, for when the project itself is deleted —
-- not RESTRICT (a forgotten CI token must never block deleting a project) and
-- not SET NULL (that would silently WIDEN the token from "this one project"
-- to "every project the owner can reach", the exact escalation this design
-- otherwise forbids). A token narrowed to a project that no longer exists has
-- nothing left to be useful for, so it goes with it.
CREATE TABLE api_tokens (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    token_hash    BYTEA NOT NULL UNIQUE,
    scopes        TEXT[] NOT NULL,
    project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
    role_at_issue TEXT NOT NULL,
    expires_at    TIMESTAMPTZ,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_project_id ON api_tokens(project_id) WHERE project_id IS NOT NULL;

-- Attribution must exist before the first token can be issued: a row written
-- before this column exists correctly has no token (it was a session), and a
-- row written after tokens exist but before this column landed would falsely
-- read as a human action forever — audit history cannot be corrected after
-- the fact.
--
-- Deliberately NO foreign key, matching resource_id (audit_logs is an
-- append-only log whose rows must survive their referent, never depend on
-- it, and AuditService.Log's own contract is that an entry must never
-- silently disappear). A real FK here is actively wrong, not just
-- unnecessary: audit writes are asynchronous, so "delete a project, which
-- CASCADEs away any token pinned to it, then audit the delete" would insert
-- a token_id that no longer exists by the time the write lands — an FK
-- violation on the single INSERT statement, which drops the ENTIRE row
-- (not just the attribution) with only a log warning. The same window hits
-- self-revocation (deleting the very token authenticating the request) once
-- that endpoint exists. A dangling token_id is fine: the row is looked up by
-- LEFT JOIN (ListAuditLogsFiltered) exactly like resource_id already is, so
-- a token that no longer exists just shows no token_name, same as a
-- long-deleted application shows no resource_name.
ALTER TABLE audit_logs ADD COLUMN token_id UUID;
CREATE INDEX idx_audit_logs_token_id ON audit_logs(token_id) WHERE token_id IS NOT NULL;
