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
SELECT sqlc.embed(s), COALESCE(p.position, 0)::integer AS reader_position, COALESCE(f.kind, '')::text AS file_kind,
  (d.step_id IS NOT NULL)::boolean AS done
FROM module_steps s
LEFT JOIN files f ON f.org_id = s.org_id AND f.id = s.file_id
LEFT JOIN reader_positions p ON p.file_id = s.file_id AND p.user_id = sqlc.arg(user_id)
LEFT JOIN module_progress d ON d.org_id = s.org_id AND d.step_id = s.id AND d.user_id = sqlc.arg(user_id)
WHERE s.org_id = sqlc.arg(org_id) AND s.module_id = ANY(sqlc.arg(module_ids)::uuid[]) AND s.archived_at IS NULL
ORDER BY s.module_id, s.position, s.id;

-- name: NextStepPosition :one
SELECT COALESCE(max(position) + 1, 0)::integer FROM module_steps WHERE module_id = $1 AND archived_at IS NULL;

-- name: CreateStep :one
INSERT INTO module_steps (id, org_id, module_id, position, kind, title, body, file_id, video_id, quiz_id, minutes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)
RETURNING *;

-- name: UpdateStep :one
UPDATE module_steps SET kind = $4, title = $5, body = $6, file_id = $7, video_id = $8, quiz_id = $9, minutes = $10, updated_at = $11
WHERE org_id = $1 AND module_id = $2 AND id = $3 AND archived_at IS NULL
RETURNING *;

-- name: ArchiveStep :execrows
UPDATE module_steps SET archived_at = $4, updated_at = $4
WHERE org_id = $1 AND module_id = $2 AND id = $3 AND archived_at IS NULL;

-- name: ActiveStepIDs :many
SELECT id FROM module_steps WHERE org_id = $1 AND module_id = $2 AND archived_at IS NULL ORDER BY position, id;

-- name: SetStepPosition :exec
UPDATE module_steps SET position = $3 WHERE module_id = $1 AND id = $2;

-- name: ReadableStep :one
-- A participant can complete only an active step of a published, non-archived module.
SELECT s.kind FROM module_steps s JOIN modules m ON m.org_id = s.org_id AND m.id = s.module_id
WHERE s.org_id = $1 AND s.module_id = $2 AND s.id = $3 AND s.archived_at IS NULL AND m.archived_at IS NULL
  AND (sqlc.arg(staff)::boolean OR m.published_at IS NOT NULL);

-- name: CompleteStep :exec
-- Idempotent: completing again keeps the first completion time.
INSERT INTO module_progress (org_id, user_id, step_id, completed_at) VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: UncompleteStep :exec
DELETE FROM module_progress WHERE org_id = $1 AND user_id = $2 AND step_id = $3;

-- name: ResumeModule :one
-- "Lanjutkan belajar": the published module the member touched last that still has an unfinished active step.
SELECT m.id, m.building_key, m.title,
  (SELECT count(*) FROM module_steps x WHERE x.module_id = m.id AND x.archived_at IS NULL)::integer AS total,
  (SELECT count(*) FROM module_steps x JOIN module_progress y ON y.org_id = x.org_id AND y.step_id = x.id
     AND y.user_id = sqlc.arg(user_id) WHERE x.module_id = m.id AND x.archived_at IS NULL)::integer AS done,
  (SELECT x.title FROM module_steps x WHERE x.module_id = m.id AND x.archived_at IS NULL
     AND NOT EXISTS (SELECT 1 FROM module_progress y WHERE y.org_id = x.org_id AND y.step_id = x.id AND y.user_id = sqlc.arg(user_id))
     ORDER BY x.position, x.id LIMIT 1)::text AS next_title,
  m.level
FROM modules m
JOIN module_steps s ON s.org_id = m.org_id AND s.module_id = m.id
JOIN module_progress p ON p.org_id = s.org_id AND p.step_id = s.id AND p.user_id = sqlc.arg(user_id)
WHERE m.org_id = sqlc.arg(org_id) AND m.archived_at IS NULL AND m.published_at IS NOT NULL
  AND EXISTS (SELECT 1 FROM module_steps x WHERE x.module_id = m.id AND x.archived_at IS NULL
    AND NOT EXISTS (SELECT 1 FROM module_progress y WHERE y.org_id = x.org_id AND y.step_id = x.id AND y.user_id = sqlc.arg(user_id)))
GROUP BY m.id
ORDER BY max(p.completed_at) DESC, m.id
LIMIT 1;

-- name: GetLinkableQuiz :one
-- A quiz step may point only at a published, non-archived quiz of the same school.
SELECT id FROM quizzes WHERE org_id = $1 AND id = $2 AND archived_at IS NULL AND published_at IS NOT NULL;
