-- name: CreateNotification :one
INSERT INTO notifications (user_id, type, title, body, link)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListNotifications :many
SELECT *
FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUnreadNotifications :one
SELECT count(*)
FROM notifications
WHERE user_id = $1 AND read = FALSE;

-- name: MarkNotificationRead :one
UPDATE notifications
SET read = TRUE
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET read = TRUE
WHERE user_id = $1 AND read = FALSE;

-- name: GetDeploymentNotifyInfo :one
SELECT u.id AS user_id, a.project_id, a.id AS application_id,
       a.name AS app_name, p.name AS project_name
FROM deployments d
JOIN applications a ON a.id = d.application_id
JOIN projects p ON p.id = a.project_id
JOIN users u ON u.id = p.user_id
WHERE d.id = $1;
