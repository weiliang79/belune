-- Let a database's backups outlive the database.
--
-- Deleting a database destroys every backup ever taken of it, including copies
-- already uploaded to a remote destination. v0.1.3 made that consented rather
-- than silent (efe7a7b), which was the honest fix available at the time — but
-- consent to an irreversible mistake is still an irreversible mistake, and the
-- one moment you want yesterday's backup is right after deleting the database
-- by accident. From here the default reverses: backups are kept, and destroying
-- them is the explicit choice.
--
-- A kept backup needs a parent, because a row pointing at nothing is exactly
-- the unreachable, unprunable, still-billed artifact this release is trying to
-- eliminate. The tombstone is that parent: the minimum needed to identify what
-- the backup came from and to put it back.
CREATE TABLE database_tombstones (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    original_id UUID NOT NULL,
    -- Restore recreates the database under its ORIGINAL slug, because the slug
    -- is the container name and therefore the hostname applications connect to.
    -- Attaching a database does not inject connection env vars, so a restore
    -- under a new name leaves every dependent application pointing at a host
    -- that no longer exists.
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    version     TEXT,
    -- Same reasoning as the slug: the credentials have to come back unchanged
    -- or the applications reconnect to a database that rejects them.
    credentials_encrypted BYTEA,
    -- Everything below is what provisioning needs that the engine name does not
    -- imply. A "other"-type database carries its own image, port and data
    -- directory, and its own backup/restore commands; a tombstone holding only
    -- identity would recreate it as something that starts wrong or cannot be
    -- restored into at all. A tombstone is what the database WAS, not a label
    -- for it.
    image           TEXT,
    container_port  INTEGER,
    data_dir        TEXT,
    backup_mode     TEXT,
    backup_command  TEXT,
    restore_command TEXT,
    deleted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_database_tombstones_project ON database_tombstones(project_id);

-- A backup now hangs off exactly one of the two parents.
ALTER TABLE database_backups
    ALTER COLUMN database_id DROP NOT NULL,
    ADD COLUMN tombstone_id UUID REFERENCES database_tombstones(id) ON DELETE CASCADE,
    ADD CONSTRAINT one_parent CHECK (num_nonnulls(database_id, tombstone_id) = 1);

-- ⚠️ Not a restructuring of existing data, deliberately: DROP NOT NULL is a
-- catalogue change, the new column is NULL for every existing row, and the
-- re-added foreign key below validates without rewriting one. Every backup that
-- exists today keeps its database_id and reads exactly as it did before. This
-- is called out because forward-only migrations run against live installs on
-- upgrade, so "does this touch existing rows" is the first question a reviewer
-- should be able to answer without deriving it.

-- The live parent's FK stays ON DELETE CASCADE, and that was checked rather
-- than assumed. SET NULL is the tempting choice — let a backup survive its
-- database — but it is wrong here twice over.
--
-- It buys nothing: a surviving backup needs a tombstone_id, and only
-- application code can write one, so every path that wants survival has to
-- re-point its rows explicitly anyway. Both delete paths do exactly that.
--
-- And it breaks a cascade that has no application code in it at all. Deleting a
-- user cascades users → projects → databases entirely inside Postgres; with SET
-- NULL those backup rows are left holding neither parent, the one_parent CHECK
-- rejects them, and deleting the user fails outright. That is the v0.1.3
-- backup_locations trap wearing a different hat: an FK action that reads as
-- protective, on a table reachable by a cascade nobody was thinking about.
--
-- So CASCADE, with the CHECK as the real guarantee: no row can ever exist
-- holding neither parent, whichever direction it arrived from.

CREATE INDEX idx_database_backups_tombstone ON database_backups(tombstone_id)
    WHERE tombstone_id IS NOT NULL;
