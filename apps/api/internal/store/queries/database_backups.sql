-- name: InsertDatabaseBackup :one
INSERT INTO database_backups (database_id, backup_config_id, target_database)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateDatabaseBackup :exec
UPDATE database_backups
SET finished_at = $2,
    status      = $3,
    local_path  = $4,
    remote_key  = $5,
    size_bytes  = $6,
    error       = $7,
    log         = $8
WHERE id = $1;

-- name: GetDatabaseBackup :one
SELECT * FROM database_backups
WHERE id = $1;

-- name: ListDatabaseBackups :many
SELECT * FROM database_backups
WHERE database_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListDatabaseBackupsByConfig :many
SELECT * FROM database_backups
WHERE backup_config_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListProjectBackupActivity :many
-- Recent backup runs across a project's databases AND application volumes,
-- newest first, for the project Backups-tab activity feed. resource_id is the
-- deep-link target: the database id on the database side, the owning
-- application id (not the volume id) on the volume side.
SELECT b.id,
       'database'::text  AS kind,
       b.started_at,
       b.finished_at,
       b.status,
       b.remote_key,
       b.size_bytes,
       b.error,
       b.log              AS log,
       b.backup_config_id,
       d.id               AS resource_id,
       d.name             AS resource_name,
       NULL::text         AS app_name
FROM database_backups b
JOIN databases d ON d.id = b.database_id
WHERE d.project_id = $1

UNION ALL

SELECT vb.id,
       'volume'::text    AS kind,
       vb.started_at,
       vb.finished_at,
       vb.status,
       vb.remote_key,
       vb.size_bytes,
       vb.error,
       COALESCE(vb.log, '') AS log,
       vb.backup_config_id,
       a.id               AS resource_id,
       v.name             AS resource_name,
       a.name             AS app_name
FROM application_volume_backups vb
JOIN application_volumes v ON v.id = vb.application_volume_id
JOIN applications a ON a.id = v.application_id
WHERE a.project_id = $1

ORDER BY started_at DESC
LIMIT $2;

-- name: DeleteDatabaseBackup :exec
DELETE FROM database_backups WHERE id = $1;

-- name: GetLastSucceededDatabaseBackup :one
SELECT * FROM database_backups
WHERE database_id = $1 AND status = 'succeeded'
ORDER BY finished_at DESC
LIMIT 1;

-- name: InsertDatabaseRestore :one
INSERT INTO database_restores (database_id, backup_id)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateDatabaseRestore :exec
UPDATE database_restores
SET finished_at = $2,
    status      = $3,
    error       = $4,
    log         = $5
WHERE id = $1;

-- name: ListDatabaseRestores :many
SELECT * FROM database_restores
WHERE database_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: SetDatabaseBackupLog :exec
UPDATE database_backups SET log = $2 WHERE id = $1;

-- name: SetDatabaseRestoreLog :exec
UPDATE database_restores SET log = $2 WHERE id = $1;

-- name: InsertBackupLocation :one
-- Records where a backup copy was written. Written at upload time, alongside
-- the legacy remote_key/local_path columns (reads move off those in 0.1.x).
INSERT INTO backup_locations (
    database_backup_id, volume_backup_id, destination_id, remote_key, local_path
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListLocationsForDatabaseBackup :many
SELECT * FROM backup_locations
WHERE database_backup_id = $1
ORDER BY uploaded_at;

-- name: ListLocationsForVolumeBackup :many
SELECT * FROM backup_locations
WHERE volume_backup_id = $1
ORDER BY uploaded_at;

-- name: CountLocationsByDestination :one
SELECT COUNT(*) FROM backup_locations WHERE destination_id = $1;

-- name: CountDatabaseBackupsWithArtifacts :one
-- Backups of a database that still have a file somewhere, for the delete
-- confirmation. Counts the artifact, not the row.
SELECT COUNT(*) FROM database_backups
WHERE database_id = $1
  AND (remote_key IS NOT NULL OR local_path IS NOT NULL);

-- name: ListDestinationNamesForDatabaseBackups :many
-- Distinct destinations holding recorded copies of a database's backups, so the
-- delete dialog can name where the data goes.
SELECT DISTINCT d.name
FROM backup_locations l
JOIN database_backups b ON b.id = l.database_backup_id
JOIN backup_destinations d ON d.id = l.destination_id
WHERE b.database_id = $1
ORDER BY d.name;

-- name: DeleteBackupLocation :exec
DELETE FROM backup_locations WHERE id = $1;
