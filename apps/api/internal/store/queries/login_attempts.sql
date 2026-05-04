-- name: GetLoginAttempt :one
SELECT * FROM login_attempts WHERE email = $1;

-- name: IncrementFailedLogin :one
-- $2 (window_start) is computed by the caller as NOW() - lockout window.
-- If the existing first_failed_at is before window_start (or null), the
-- counter resets to 1 and the window restarts. Otherwise it increments.
INSERT INTO login_attempts (email, failed_count, first_failed_at)
VALUES ($1, 1, NOW())
ON CONFLICT (email) DO UPDATE SET
    failed_count = CASE
        WHEN login_attempts.first_failed_at IS NULL
          OR login_attempts.first_failed_at < $2
        THEN 1
        ELSE login_attempts.failed_count + 1
    END,
    first_failed_at = CASE
        WHEN login_attempts.first_failed_at IS NULL
          OR login_attempts.first_failed_at < $2
        THEN NOW()
        ELSE login_attempts.first_failed_at
    END
RETURNING *;

-- name: LockAccount :exec
UPDATE login_attempts SET locked_until = $2 WHERE email = $1;

-- name: ResetLoginAttempts :exec
DELETE FROM login_attempts WHERE email = $1;
