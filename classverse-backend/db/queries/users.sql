-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name, birth_date)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE lower(email) = lower(sqlc.arg(email));
