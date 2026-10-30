-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name, birth_date)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE lower(email) = lower(sqlc.arg(email));

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByGoogleSub :one
SELECT * FROM users
WHERE google_sub = $1;

-- name: UpdatePendingUser :exec
-- Re-registration of an email that was never verified replaces the pending data.
UPDATE users
SET password_hash = sqlc.arg(password_hash),
    display_name  = sqlc.arg(display_name),
    birth_date    = sqlc.arg(birth_date),
    updated_at    = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND email_verified_at IS NULL;

-- name: MarkEmailVerified :exec
UPDATE users
SET email_verified_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: LinkGoogleAccount :one
-- An unverified account loses its password: whoever registered it first never
-- proved ownership of the email, Google just did.
UPDATE users
SET google_sub        = sqlc.arg(google_sub),
    password_hash     = CASE WHEN email_verified_at IS NULL THEN NULL ELSE password_hash END,
    email_verified_at = coalesce(email_verified_at, sqlc.arg(now)),
    updated_at        = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CreateGoogleUser :one
INSERT INTO users (email, email_verified_at, google_sub, display_name)
VALUES (sqlc.arg(email), sqlc.arg(now), sqlc.arg(google_sub), sqlc.arg(display_name))
RETURNING *;

-- name: UpdateProfile :one
UPDATE users
SET display_name = coalesce(sqlc.narg(display_name), display_name),
    birth_date   = coalesce(sqlc.narg(birth_date), birth_date),
    updated_at   = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CreateTestUser :one
-- Test helper: a verified user with an optional birth date.
INSERT INTO users (email, email_verified_at, display_name, birth_date)
VALUES (sqlc.arg(email), now(), sqlc.arg(display_name), sqlc.narg(birth_date))
RETURNING *;
