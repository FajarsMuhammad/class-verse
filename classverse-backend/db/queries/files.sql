-- name: EnsureFileStorage :exec
INSERT INTO file_storage (org_id) VALUES ($1) ON CONFLICT (org_id) DO NOTHING;

-- name: ReserveFileBytes :one
UPDATE file_storage SET used_bytes = used_bytes + sqlc.arg(size_bytes)::bigint
WHERE org_id = sqlc.arg(org_id) AND sqlc.arg(size_bytes)::bigint > 0
AND sqlc.arg(size_bytes)::bigint <= quota_bytes - used_bytes
RETURNING *;

-- name: GetFileUsage :one
SELECT org_id, quota_bytes, used_bytes FROM file_storage WHERE org_id = $1;

-- name: CreatePendingFile :one
INSERT INTO files (id, org_id, uploaded_by, kind, name, size_bytes, object_key, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8) RETURNING *;

-- name: LockFile :one
SELECT * FROM files WHERE org_id = $1 AND id = $2 FOR UPDATE;

-- name: CompleteFile :exec
UPDATE files SET status = 'ready', sha256 = $3, content_type = $4, updated_at = $5
WHERE org_id = $1 AND id = $2 AND status = 'pending';

-- name: MarkFileDeleting :exec
UPDATE files SET status = 'deleting', updated_at = $3
WHERE org_id = $1 AND id = $2 AND status IN ('pending', 'ready');

-- name: ConfirmFileDeleted :exec
UPDATE files SET status = 'deleted', deleted_at = $3, updated_at = $3
WHERE org_id = $1 AND id = $2 AND status = 'deleting';

-- name: ReleaseFileBytes :execrows
UPDATE file_storage SET used_bytes = used_bytes - sqlc.arg(size_bytes)::bigint
WHERE org_id = sqlc.arg(org_id) AND used_bytes >= sqlc.arg(size_bytes)::bigint;

-- name: ListReadyFiles :many
SELECT f.id, f.kind, f.name, f.size_bytes, f.content_type, f.uploaded_by, f.created_at,
  COALESCE(p.position, 0)::integer AS position
FROM files f LEFT JOIN reader_positions p ON p.file_id = f.id AND p.user_id = sqlc.arg(user_id)
WHERE f.org_id = sqlc.arg(org_id) AND f.status = 'ready'
ORDER BY f.created_at DESC, f.id LIMIT sqlc.narg(page_limit)::integer OFFSET sqlc.arg(page_offset)::integer;

-- name: GetReadyFile :one
SELECT * FROM files WHERE org_id = $1 AND id = $2 AND status = 'ready';

-- name: SaveReaderPosition :exec
INSERT INTO reader_positions (org_id, file_id, user_id, position, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (file_id, user_id) DO UPDATE SET position = EXCLUDED.position, updated_at = EXCLUDED.updated_at;
