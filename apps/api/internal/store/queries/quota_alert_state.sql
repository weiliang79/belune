-- name: GetQuotaAlertState :one
SELECT * FROM quota_alert_state WHERE scope = $1 AND scope_id = $2;

-- name: UpsertQuotaAlertState :exec
INSERT INTO quota_alert_state (scope, scope_id, last_alerted_at, last_alerted_percent)
VALUES ($1, $2, $3, $4)
ON CONFLICT (scope, scope_id) DO UPDATE
SET last_alerted_at      = EXCLUDED.last_alerted_at,
    last_alerted_percent = EXCLUDED.last_alerted_percent;
