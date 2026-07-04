-- name: CreateApplicationVolumeBackupConfig :one
INSERT INTO application_volume_backup_configs (
    application_volume_id, destination_id, prefix, schedule, keep_latest, enabled, quiesce
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateApplicationVolumeBackupConfig :one
UPDATE application_volume_backup_configs
SET destination_id = $2,
    prefix         = $3,
    schedule       = $4,
    keep_latest    = $5,
    enabled        = $6,
    quiesce        = $7,
    updated_at     = NOW()
WHERE id = $1
RETURNING *;

-- name: GetApplicationVolumeBackupConfig :one
SELECT * FROM application_volume_backup_configs WHERE id = $1;

-- name: ListApplicationVolumeBackupConfigs :many
SELECT * FROM application_volume_backup_configs
WHERE application_volume_id = $1
ORDER BY created_at;

-- name: ListEnabledApplicationVolumeBackupConfigs :many
SELECT * FROM application_volume_backup_configs
WHERE enabled AND schedule <> ''
ORDER BY created_at;

-- name: ListApplicationVolumeBackupConfigsForApp :many
SELECT c.*, v.name AS volume_name, v.mount_path AS volume_mount_path
FROM application_volume_backup_configs c
JOIN application_volumes v ON v.id = c.application_volume_id
WHERE v.application_id = $1
ORDER BY v.name, c.created_at;

-- name: DeleteApplicationVolumeBackupConfig :exec
DELETE FROM application_volume_backup_configs WHERE id = $1;

-- name: SetApplicationVolumeBackupConfigLastRun :exec
UPDATE application_volume_backup_configs
SET last_run_at = $2, updated_at = NOW()
WHERE id = $1;

-- name: InsertApplicationVolumeBackup :one
INSERT INTO application_volume_backups (application_volume_id, backup_config_id)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateApplicationVolumeBackup :exec
UPDATE application_volume_backups
SET finished_at = $2,
    status      = $3,
    local_path  = $4,
    remote_key  = $5,
    size_bytes  = $6,
    error       = $7,
    log         = $8
WHERE id = $1;

-- name: GetApplicationVolumeBackup :one
SELECT * FROM application_volume_backups WHERE id = $1;

-- name: ListApplicationVolumeBackups :many
SELECT * FROM application_volume_backups
WHERE application_volume_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListApplicationVolumeBackupsByConfig :many
SELECT * FROM application_volume_backups
WHERE backup_config_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: DeleteApplicationVolumeBackup :exec
DELETE FROM application_volume_backups WHERE id = $1;

-- name: InsertApplicationVolumeRestore :one
INSERT INTO application_volume_restores (application_volume_id, backup_id)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateApplicationVolumeRestore :exec
UPDATE application_volume_restores
SET finished_at = $2,
    status      = $3,
    error       = $4,
    log         = $5
WHERE id = $1;

-- name: ListApplicationVolumeRestores :many
SELECT * FROM application_volume_restores
WHERE application_volume_id = $1
ORDER BY started_at DESC
LIMIT $2;
