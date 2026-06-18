-- name: GetAlertPreferences :one
SELECT * FROM alert_preferences WHERE user_id = $1;

-- name: UpsertAlertPreferences :one
INSERT INTO alert_preferences (user_id, deploy_failures, deploy_success, build_failures, quota_threshold, quota_threshold_percent)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id) DO UPDATE
SET deploy_failures         = EXCLUDED.deploy_failures,
    deploy_success          = EXCLUDED.deploy_success,
    build_failures          = EXCLUDED.build_failures,
    quota_threshold         = EXCLUDED.quota_threshold,
    quota_threshold_percent = EXCLUDED.quota_threshold_percent
RETURNING *;
