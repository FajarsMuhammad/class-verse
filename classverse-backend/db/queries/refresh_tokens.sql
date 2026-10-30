-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetRefreshTokenForUpdate :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1
FOR UPDATE;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1;

-- name: MarkRefreshTokenUsed :exec
UPDATE refresh_tokens
SET used_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: RevokeRefreshFamily :exec
UPDATE refresh_tokens
SET revoked_at = sqlc.arg(now)
WHERE family_id = sqlc.arg(family_id) AND revoked_at IS NULL;
