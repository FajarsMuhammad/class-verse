-- name: CreateModule :one
INSERT INTO modules (id, org_id, building_key, level, title, description, created_by, created_at, updated_at, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)
RETURNING *;

-- name: LockModule :one
SELECT * FROM modules WHERE org_id = $1 AND id = $2 AND archived_at IS NULL FOR UPDATE;

-- name: UpdateModule :one
UPDATE modules SET building_key = $3, level = $4, title = $5, description = $6, published_at = $7, updated_at = $8
WHERE org_id = $1 AND id = $2 AND archived_at IS NULL
RETURNING *;

-- name: ArchiveModule :execrows
UPDATE modules SET archived_at = $3, updated_at = $3 WHERE org_id = $1 AND id = $2 AND archived_at IS NULL;

-- name: ListModules :many
-- Staff may include drafts; archived modules never show.
SELECT * FROM modules
WHERE org_id = sqlc.arg(org_id) AND archived_at IS NULL
  AND (sqlc.arg(include_drafts)::boolean OR published_at IS NOT NULL)
  AND (sqlc.narg(building_key)::text IS NULL OR building_key = sqlc.narg(building_key))
  AND (sqlc.narg(level)::text IS NULL OR level = sqlc.narg(level))
ORDER BY created_at, id
LIMIT 100;

-- name: ListModuleSteps :many
-- Active steps of the given modules, in path order, with the caller's reader position for file steps.
SELECT sqlc.embed(s), COALESCE(p.position, 0)::integer AS reader_position, COALESCE(f.kind, '')::text AS file_kind
FROM module_steps s
LEFT JOIN files f ON f.org_id = s.org_id AND f.id = s.file_id
LEFT JOIN reader_positions p ON p.file_id = s.file_id AND p.user_id = sqlc.arg(user_id)
WHERE s.org_id = sqlc.arg(org_id) AND s.module_id = ANY(sqlc.arg(module_ids)::uuid[]) AND s.archived_at IS NULL
ORDER BY s.module_id, s.position, s.id;

-- name: NextStepPosition :one
SELECT COALESCE(max(position) + 1, 0)::integer FROM module_steps WHERE module_id = $1 AND archived_at IS NULL;

-- name: CreateStep :one
INSERT INTO module_steps (id, org_id, module_id, position, kind, title, body, file_id, video_id, minutes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
RETURNING *;

-- name: UpdateStep :one
UPDATE module_steps SET kind = $4, title = $5, body = $6, file_id = $7, video_id = $8, minutes = $9, updated_at = $10
WHERE org_id = $1 AND module_id = $2 AND id = $3 AND archived_at IS NULL
RETURNING *;

-- name: ArchiveStep :execrows
UPDATE module_steps SET archived_at = $4, updated_at = $4
WHERE org_id = $1 AND module_id = $2 AND id = $3 AND archived_at IS NULL;

-- name: ActiveStepIDs :many
SELECT id FROM module_steps WHERE org_id = $1 AND module_id = $2 AND archived_at IS NULL ORDER BY position, id;

-- name: SetStepPosition :exec
UPDATE module_steps SET position = $3 WHERE module_id = $1 AND id = $2;
