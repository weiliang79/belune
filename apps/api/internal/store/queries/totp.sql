-- name: SetUserTOTPSecret :exec
-- Stores a freshly generated secret WITHOUT enabling the factor, and clears any
-- previous replay state so a re-enrollment starts clean.
UPDATE users
SET totp_secret_encrypted = $2,
    totp_enabled_at       = NULL,
    totp_last_step        = NULL
WHERE id = $1;

-- name: EnableUserTOTP :exec
-- Enables only when a secret is present: the factor cannot be turned on for an
-- account with nothing to verify against.
UPDATE users
SET totp_enabled_at = NOW(),
    totp_last_step  = $2
WHERE id = $1 AND totp_secret_encrypted IS NOT NULL;

-- name: SetUserTOTPLastStep :exec
-- Records the accepted step. Guarded so a slower concurrent request cannot move
-- the marker backwards and re-open a window that has already been spent.
UPDATE users
SET totp_last_step = $2
WHERE id = $1 AND (totp_last_step IS NULL OR totp_last_step < $2);

-- name: DisableUserTOTP :exec
UPDATE users
SET totp_secret_encrypted = NULL,
    totp_enabled_at       = NULL,
    totp_last_step        = NULL
WHERE id = $1;

-- name: InsertRecoveryCode :exec
INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, $2);

-- name: DeleteRecoveryCodes :exec
DELETE FROM user_recovery_codes WHERE user_id = $1;

-- name: CountUnusedRecoveryCodes :one
SELECT count(*) FROM user_recovery_codes WHERE user_id = $1 AND used_at IS NULL;

-- name: ConsumeRecoveryCode :one
-- Spending a code is the UPDATE itself, not a read followed by a write: two
-- requests racing with the same code both find it unused, and only the one that
-- wins this WHERE gets a row back.
UPDATE user_recovery_codes
SET used_at = NOW()
WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
RETURNING id;
