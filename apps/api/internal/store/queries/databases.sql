-- name: ListDatabasesByProject :many
SELECT * FROM databases WHERE project_id = $1 ORDER BY created_at DESC;

-- name: GetDatabase :one
SELECT * FROM databases WHERE id = $1;

-- name: CreateDatabase :one
INSERT INTO databases (
    project_id, type, name, slug, version, status, internal_host, internal_port,
    credentials_encrypted, image, container_port, data_dir, backup_mode,
    backup_command, restore_command
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: UpdateDatabaseStatus :one
UPDATE databases SET status = $2 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseAfterProvision :one
UPDATE databases SET status = $2, internal_host = $3, internal_port = $4, host_port = $5 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseResources :one
UPDATE databases SET cpu_limit = $2, memory_limit = $3 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseVersion :one
UPDATE databases SET version = $2 WHERE id = $1 RETURNING *;

-- name: UpdateDatabaseImageDigest :exec
-- Pins (or clears, with NULL) the resolved @sha256 image digest so recreates
-- reuse the exact image. Upgrade clears it so the target tag is re-pinned.
UPDATE databases SET image_digest = $2 WHERE id = $1;

-- name: ListDatabasesByStatus :many
SELECT * FROM databases WHERE status = $1;

-- name: ListAllDatabases :many
SELECT * FROM databases;

-- name: DeleteDatabase :exec
DELETE FROM databases WHERE id = $1;

-- name: UpdateDatabaseSlug :exec
UPDATE databases SET slug = $2 WHERE id = $1;

-- name: CountDatabases :one
SELECT count(*) FROM databases;

-- name: GetDatabaseOwnerUserID :one
SELECT p.user_id FROM databases d
JOIN projects p ON p.id = d.project_id
WHERE d.id = $1;

-- name: UpdateDatabaseSource :exec
UPDATE databases SET source_kind = $2, source_ref = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetServerIDForDatabase :one
-- Placement lookup, sibling of GetServerIDForApplication. Databases are fetched
-- as a bare row everywhere, so their host is resolved from the project here.
SELECT p.server_id FROM databases d
JOIN projects p ON p.id = d.project_id
WHERE d.id = $1;

-- name: ListAllDatabasesWithServerID :many
-- Placement rides along so a host-by-host sweep can tell which databases belong
-- on which server without a lookup per row.
SELECT d.*, p.server_id FROM databases d
JOIN projects p ON p.id = d.project_id;

-- name: CreateDatabaseTombstone :one
-- Records what a deleted database was, so its backups keep a parent and a
-- replacement can be recreated identically.
INSERT INTO database_tombstones (
    project_id, original_id, slug, name, type, version, credentials_encrypted,
    image, container_port, data_dir, backup_mode, backup_command, restore_command,
    cpu_limit, memory_limit, image_digest
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetDatabaseTombstone :one
SELECT * FROM database_tombstones WHERE id = $1;

-- name: ListDatabaseTombstonesByProject :many
SELECT * FROM database_tombstones WHERE project_id = $1 ORDER BY deleted_at DESC;

-- name: DeleteDatabaseTombstone :exec
DELETE FROM database_tombstones WHERE id = $1;

-- name: ReparentDatabaseBackupsToTombstone :exec
-- Moves a database's backups onto its tombstone. Both columns are written in
-- one statement because the one_parent CHECK forbids a row holding both, so
-- there is no intermediate state where this could be split in two.
UPDATE database_backups
SET    database_id = NULL, tombstone_id = $2
WHERE  database_id = $1;

-- name: DeleteDatabaseBackupsForDatabase :exec
-- Removes the rows outright, for the purge path. Object deletion stays
-- best-effort, but the rows must go deterministically: a leftover row pointing
-- at a database about to disappear trips the one_parent CHECK and aborts the
-- delete.
DELETE FROM database_backups WHERE database_id = $1;

-- name: CountOrphanedBackupsByProject :one
SELECT count(*) FROM database_backups b
JOIN database_tombstones t ON t.id = b.tombstone_id
WHERE t.project_id = $1;

-- name: ReclaimBackupsFromTombstone :exec
-- The inverse of ReparentDatabaseBackupsToTombstone: hands a tombstone's
-- backups back to the database recreated from it. One statement for the same
-- reason — the one_parent CHECK forbids a row holding both.
UPDATE database_backups
SET    tombstone_id = NULL, database_id = $2
WHERE  tombstone_id = $1;

-- name: ListOrphanedBackupsByProject :many
-- Backups whose database is gone, with enough of the tombstone to say what they
-- came from. Project-scoped because the tombstone is: the project is the access
-- boundary, so an orphaned backup has no owner above it.
SELECT b.*, t.slug AS database_slug, t.name AS database_name,
       t.type AS database_type, t.deleted_at AS database_deleted_at
FROM   database_backups b
JOIN   database_tombstones t ON t.id = b.tombstone_id
WHERE  t.project_id = $1
ORDER  BY b.started_at DESC;

-- name: CountBackupsForTombstone :one
SELECT count(*) FROM database_backups WHERE tombstone_id = $1;

-- name: ListExpiredOrphanedBackups :many
-- Orphaned backups whose keeping period is over. The clock runs from when the
-- database was deleted, not from when the backup was taken: keeping is a
-- decision made at deletion time, so a two-year-old backup of a database
-- deleted yesterday has just been kept on purpose and is not expired.
SELECT b.* FROM database_backups b
JOIN   database_tombstones t ON t.id = b.tombstone_id
WHERE  t.deleted_at < $1
LIMIT  $2;

-- name: DeleteEmptyDatabaseTombstones :exec
-- A tombstone exists to give backups a parent. One with none left describes
-- nothing, so it goes rather than accumulating forever.
DELETE FROM database_tombstones t
WHERE NOT EXISTS (SELECT 1 FROM database_backups b WHERE b.tombstone_id = t.id);

-- name: CountDatabaseBackups :one
-- Every backup row for a database, artifacts or not. Deletion uses it to decide
-- whether a tombstone has anything to parent.
SELECT count(*) FROM database_backups WHERE database_id = $1;
