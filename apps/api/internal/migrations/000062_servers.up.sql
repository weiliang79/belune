-- Groundwork for multi-server: a row per host Belune can place workloads on,
-- and a project's placement on one of them. Nothing reads either yet — the
-- agent and the placement rules land in a later release. This ships now so the
-- backup artifact rework can record which host holds a local copy, and so the
-- backfill happens while every install still has exactly one host.
--
-- There is deliberately NO host, port, ssh_user or ssh_key column. The agent
-- always dials OUT to the control plane, so the control plane never needs an
-- address to reach a server; that is the structural difference from PaaSes
-- that SSH in, and the reason nothing here should ever grow an ip_address.
CREATE TABLE servers (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name     TEXT NOT NULL,

    -- The control plane's own host. Exactly one row, enforced below. It runs no
    -- separate agent (the container runtime is in-process), cannot be forgotten,
    -- and moves version-wise with the control plane.
    is_local BOOLEAN NOT NULL DEFAULT false,

    -- Operator INTENT, never connectivity. Whether a server is reachable right
    -- now is derived from last_seen_at at read time.
    lifecycle TEXT NOT NULL DEFAULT 'pending'
              CHECK (lifecycle IN ('pending', 'active', 'draining', 'revoked')),

    -- What OTHER servers dial for peer-to-peer data traffic (the ambassador).
    -- NOT how the control plane reaches this host: see the note above.
    advertise_address TEXT,

    -- Observed facts, refreshed by the agent when it connects. Every state the
    -- UI shows is computed from these and never stored: online/offline from
    -- last_seen_at, degraded from agent_protocol_version against the range this
    -- control plane supports, update-available from agent_version. A stored
    -- flag would go stale the moment the control plane upgrades and changes
    -- which protocol versions it accepts.
    last_seen_at           TIMESTAMPTZ,
    agent_version          TEXT,
    agent_protocol_version INT,
    arch                   TEXT,   -- amd64/arm64: decides which agent binary to serve
    os                     TEXT,
    docker_version         TEXT,
    cpu_cores              INT,
    memory_total_bytes     BIGINT,
    clock_skew_seconds     INT,    -- posture: mTLS cert validation is time-dependent

    enrolled_at TIMESTAMPTZ,
    -- Forgetting a server is revocation, not deletion: the row stays so that
    -- re-enrolment can tell "never seen" apart from "deliberately
    -- decommissioned". Deleting the row destroys the only evidence.
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial, so a name is not burned forever by a revoked row. Revocation keeps
-- the row rather than deleting it, so an unconditional unique index would mean
-- rebuilding a decommissioned host and re-enrolling it under its own name fails
-- — the operator would have to invent a new one, permanently.
CREATE UNIQUE INDEX idx_servers_name      ON servers(name) WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX idx_servers_one_local ON servers(is_local) WHERE is_local;

-- The local host is a real row from day one rather than an absent-means-local
-- sentinel, so there is no dual-mode code path and projects.server_id can be
-- NOT NULL. Its id is generated, not a well-known constant, so no magic value
-- leaks into code or tests; callers look it up by is_local.
INSERT INTO servers (name, is_local, lifecycle, enrolled_at)
VALUES ('local', true, 'active', NOW());

-- Placement is per-project, not per-resource: a project's Docker bridge network
-- is single-host, so an app and its database cannot be split across servers.
--
-- ON DELETE RESTRICT is safe here, but it was checked rather than copied: the
-- v0.1.3 backup_locations trap was a RESTRICT on a table reachable by two
-- parallel cascade paths, where one path tripped on rows the other was about to
-- delete. servers has no parent FK, so nothing cascades into it, and this
-- clause blocks deleting a *server* that still holds projects (drain first) —
-- it never blocks deleting a project or the user who owns it. Re-run that
-- reasoning for every future server_id column instead of copying this line.
ALTER TABLE projects ADD COLUMN server_id UUID REFERENCES servers(id) ON DELETE RESTRICT;
UPDATE projects SET server_id = (SELECT id FROM servers WHERE is_local);
ALTER TABLE projects ALTER COLUMN server_id SET NOT NULL;

-- Postgres indexes the referenced key, never the referencing column, so without
-- this every DELETE FROM servers sequentially scans to enforce the RESTRICT —
-- and "what is placed on this server" is the query this column exists for.
CREATE INDEX idx_projects_server ON projects(server_id);

-- Inert until the agent exists, but certain in shape, and adding it here means
-- the backup artifact rework needs no second migration against this table.
-- NULL is a real meaning, not a sentinel: the copy is not host-bound (it lives
-- in a remote bucket). Only rows with a local_path are on a specific host.
ALTER TABLE backup_locations ADD COLUMN server_id UUID REFERENCES servers(id);
UPDATE backup_locations SET server_id = (SELECT id FROM servers WHERE is_local)
 WHERE local_path IS NOT NULL;

-- Same reason as above, and it matters more here: this table gains a row per
-- copy per backup run, so it is the one that keeps growing.
CREATE INDEX idx_backup_locations_server ON backup_locations(server_id);
