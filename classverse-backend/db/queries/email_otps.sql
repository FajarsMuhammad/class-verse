-- name: GetLatestOTP :one
SELECT * FROM email_otps
WHERE user_id = $1
ORDER BY expires_at DESC
LIMIT 1;

-- name: GetActiveOTPForUpdate :one
SELECT * FROM email_otps
WHERE user_id = $1 AND consumed_at IS NULL
ORDER BY expires_at DESC
LIMIT 1
FOR UPDATE;

-- name: ConsumeActiveOTPs :exec
UPDATE email_otps
SET consumed_at = sqlc.arg(now)
WHERE user_id = sqlc.arg(user_id) AND consumed_at IS NULL;

-- name: CreateOTP :exec
INSERT INTO email_otps (user_id, code_hash, expires_at)
VALUES ($1, $2, $3);

-- name: IncrementOTPAttempts :one
UPDATE email_otps
SET attempts = attempts + 1
WHERE id = $1
RETURNING attempts;

-- name: ConsumeOTP :exec
UPDATE email_otps
SET consumed_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
