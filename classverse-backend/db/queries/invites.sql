-- name: CreateInvite :one
INSERT INTO invites (org_id, code, role, max_uses, created_by, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListActiveInvites :many
-- Keyset pagination on (created_at, id) descending. Every filter includes org_id.
SELECT * FROM invites
WHERE org_id = sqlc.arg(org_id)
  AND revoked_at IS NULL
  AND expires_at > sqlc.arg(now)
  AND (max_uses IS NULL OR used_count < max_uses)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetInviteInOrg :one
SELECT * FROM invites
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: RevokeInvite :exec
UPDATE invites
SET revoked_at = coalesce(revoked_at, sqlc.arg(now))
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: GetInvitePreview :one
SELECT i.id, i.org_id, i.role, i.max_uses, i.used_count, i.expires_at, i.revoked_at, o.name AS org_name
FROM invites i
JOIN organizations o ON o.id = i.org_id
WHERE i.code = $1;

-- name: GetInviteByCodeForUpdate :one
SELECT * FROM invites
WHERE code = $1
FOR UPDATE;

-- name: IncrementInviteUse :exec
UPDATE invites
SET used_count = used_count + 1
WHERE id = $1;
