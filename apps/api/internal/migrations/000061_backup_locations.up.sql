-- Record where each backup was actually written.
--
-- Until now a backup row stored its object key but not its destination, and
-- restore resolved the destination by following backup_config_id to whatever
-- that config points at *now*. The config is editable and deletable, so the
-- pointer moves out from under backups already written: repoint a config and
-- every prior backup resolves to a bucket its objects were never in. It fails
-- at restore time, which is the one moment it cannot be discovered safely.
--
-- A join table rather than a destination_id column: 0.1.x's backup artifact
-- model needs one row per copy of an artifact, and migrations here are
-- forward-only, so a column added now could never be retired and would sit
-- unread forever.
CREATE TABLE backup_locations (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_backup_id UUID REFERENCES database_backups(id) ON DELETE CASCADE,
    volume_backup_id   UUID REFERENCES application_volume_backups(id) ON DELETE CASCADE,
    -- RESTRICT: a destination holding recorded backups can no longer be
    -- deleted. Stricter than before, and the point of the table.
    destination_id     UUID NOT NULL REFERENCES backup_destinations(id) ON DELETE RESTRICT,
    remote_key         TEXT,
    local_path         TEXT,
    uploaded_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT one_parent CHECK (num_nonnulls(database_backup_id, volume_backup_id) = 1),
    CONSTRAINT has_a_file CHECK (num_nonnulls(remote_key, local_path) >= 1)
);

CREATE INDEX idx_backup_locations_destination ON backup_locations(destination_id);

-- One recorded copy per (artifact, destination): a destination cannot hold two
-- recorded copies of the same backup.
CREATE UNIQUE INDEX idx_backup_locations_db_dest
    ON backup_locations(database_backup_id, destination_id)
    WHERE database_backup_id IS NOT NULL;
CREATE UNIQUE INDEX idx_backup_locations_vol_dest
    ON backup_locations(volume_backup_id, destination_id)
    WHERE volume_backup_id IS NOT NULL;

-- Backfill freezes the guess the code is already making, so it cannot make any
-- existing backup worse. From here the pointer stops moving.
--
-- The INNER JOIN is load-bearing. It excludes rows whose backup_config_id is
-- NULL — ad-hoc runs against the global env-var S3 target (which can never have
-- a backup_destinations row, since project_id is NOT NULL and the global target
-- belongs to no project) and backups orphaned by a deleted config (ON DELETE
-- SET NULL). Their destination is genuinely unknowable, so they get no row:
-- absence means "Belune does not know where this lives" and resolution falls
-- back to the existing config-then-global path, exactly as today.
INSERT INTO backup_locations (database_backup_id, destination_id, remote_key, local_path, uploaded_at)
SELECT b.id, c.destination_id, b.remote_key, b.local_path, COALESCE(b.finished_at, b.started_at)
FROM   database_backups b
JOIN   database_backup_configs c ON c.id = b.backup_config_id
WHERE  b.remote_key IS NOT NULL OR b.local_path IS NOT NULL;

INSERT INTO backup_locations (volume_backup_id, destination_id, remote_key, local_path, uploaded_at)
SELECT vb.id, c.destination_id, vb.remote_key, vb.local_path, COALESCE(vb.finished_at, vb.started_at)
FROM   application_volume_backups vb
JOIN   application_volume_backup_configs c ON c.id = vb.backup_config_id
WHERE  vb.remote_key IS NOT NULL OR vb.local_path IS NOT NULL;
