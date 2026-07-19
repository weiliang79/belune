-- name: CreateNotificationChannel :one
INSERT INTO notification_channels (
    name, type, config_encrypted, events, enabled, created_by
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateNotificationChannel :one
UPDATE notification_channels
SET name             = $2,
    events           = $3,
    enabled          = $4,
    -- Keep the existing config when no new one is supplied (NULL); the type is
    -- immutable because the config shape is provider-specific.
    config_encrypted = COALESCE($5, config_encrypted),
    updated_at       = NOW()
WHERE id = $1
RETURNING *;

-- name: SetNotificationChannelEnabled :one
UPDATE notification_channels
SET enabled    = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetNotificationChannel :one
SELECT * FROM notification_channels WHERE id = $1;

-- name: ListNotificationChannels :many
SELECT id, name, type, events, enabled, last_sent_at, last_error, last_event_type, config_encrypted, created_by, created_at, updated_at
FROM notification_channels
ORDER BY name;

-- name: ListEnabledChannelsForEvent :many
SELECT * FROM notification_channels
WHERE enabled AND sqlc.arg(event_type)::text = ANY(events)
ORDER BY name;

-- name: DeleteNotificationChannel :exec
DELETE FROM notification_channels WHERE id = $1;

-- name: MarkNotificationChannelSent :exec
UPDATE notification_channels
SET last_sent_at    = NOW(),
    last_error      = NULL,
    last_event_type = $2
WHERE id = $1;

-- name: MarkNotificationChannelError :exec
UPDATE notification_channels
SET last_error      = $2,
    last_event_type = $3
WHERE id = $1;
