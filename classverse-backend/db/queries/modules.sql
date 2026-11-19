-- name: CreateModule :one
INSERT INTO modules (id, org_id, building_key, level, title, description, created_by, created_at, updated_at, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)
RETURNING *;

-- name: LockModule :one
SELECT * FROM modules WHERE org_id = $1 AND id = $2 AND archived_at IS NULL FOR UPDATE;

-- name: UpdateModule :one
UPDATE modules SET building_key = $3, level = $4, title = $5, description = $6, updated_at = $7
WHERE org_id = $1 AND id = $2 AND archived_at IS NULL
RETURNING *;

-- name: ArchiveModule :execrows
UPDATE modules SET archived_at = $3, updated_at = $3 WHERE org_id = $1 AND id = $2 AND archived_at IS NULL;

-- name: ListModules :many
-- Archived rows are a separate staff-only list, authorized by the service.
SELECT sqlc.embed(m), count(*) OVER() AS total_count FROM modules m
LEFT JOIN module_revision_pins pin ON pin.org_id=m.org_id AND pin.module_id=m.id AND pin.user_id=sqlc.arg(user_id)
LEFT JOIN module_revisions r ON r.id=COALESCE(pin.revision_id,m.published_revision_id)
WHERE m.org_id = sqlc.arg(org_id) AND (m.archived_at IS NOT NULL) = sqlc.arg(archived)::boolean
  AND (sqlc.arg(include_drafts)::boolean OR m.published_at IS NOT NULL)
  AND (sqlc.narg(building_key)::text IS NULL OR CASE WHEN sqlc.arg(include_drafts)::boolean THEN m.building_key ELSE r.building_key END = sqlc.narg(building_key))
  AND (sqlc.narg(level)::text IS NULL OR CASE WHEN sqlc.arg(include_drafts)::boolean THEN m.level ELSE r.level END = sqlc.narg(level))
  AND (sqlc.narg(module_id)::uuid IS NULL OR m.id = sqlc.narg(module_id))
  AND (sqlc.arg(search)::text = '' OR position(lower(sqlc.arg(search)) in lower(CASE WHEN sqlc.arg(include_drafts)::boolean THEN m.title ELSE r.title END)) > 0)
  AND (sqlc.arg(status)::text = '' OR (m.published_at IS NOT NULL) = (sqlc.arg(status) = 'published'))
ORDER BY m.created_at, m.id
LIMIT sqlc.arg(page_limit)::integer OFFSET sqlc.arg(list_offset)::integer;

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
SELECT s.kind FROM module_revision_steps s JOIN modules m ON m.org_id=s.org_id AND m.id=s.module_id
JOIN module_revision_pins p ON p.org_id=s.org_id AND p.module_id=s.module_id AND p.revision_id=s.revision_id AND p.user_id=sqlc.arg(user_id)
WHERE s.org_id=sqlc.arg(org_id) AND s.module_id=sqlc.arg(module_id) AND s.id=sqlc.arg(id)
 AND m.archived_at IS NULL AND m.published_at IS NOT NULL;

-- name: CompleteStep :exec
-- Idempotent: completing again keeps the first completion time.
INSERT INTO module_progress (org_id, user_id, step_id, completed_at) VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: UncompleteStep :exec
DELETE FROM module_progress WHERE org_id = $1 AND user_id = $2 AND step_id = $3;

-- name: ResumeModule :one
SELECT m.id,r.building_key,r.title,r.level,
 (SELECT count(*)::integer FROM module_revision_steps x WHERE x.revision_id=r.id) AS total,
 (SELECT count(*)::integer FROM module_revision_steps x JOIN module_progress y ON y.org_id=x.org_id AND y.step_id=x.id AND y.user_id=sqlc.arg(user_id) WHERE x.revision_id=r.id) AS done,
 (SELECT x.title FROM module_revision_steps x WHERE x.revision_id=r.id AND NOT EXISTS(SELECT 1 FROM module_progress y WHERE y.org_id=x.org_id AND y.step_id=x.id AND y.user_id=sqlc.arg(user_id)) ORDER BY x.position,x.id LIMIT 1)::text AS next_title
FROM modules m JOIN module_revision_pins pin ON pin.org_id=m.org_id AND pin.module_id=m.id AND pin.user_id=sqlc.arg(user_id)
JOIN module_revisions r ON r.org_id=pin.org_id AND r.module_id=pin.module_id AND r.id=pin.revision_id
JOIN module_revision_steps s ON s.revision_id=r.id
JOIN module_progress p ON p.org_id=s.org_id AND p.step_id=s.id AND p.user_id=sqlc.arg(user_id)
WHERE m.org_id=sqlc.arg(org_id) AND m.archived_at IS NULL AND m.published_at IS NOT NULL
AND EXISTS(SELECT 1 FROM module_revision_steps x WHERE x.revision_id=r.id AND NOT EXISTS(SELECT 1 FROM module_progress y WHERE y.org_id=x.org_id AND y.step_id=x.id AND y.user_id=sqlc.arg(user_id)))
GROUP BY m.id,r.id ORDER BY max(p.completed_at) DESC,m.id LIMIT 1;

-- name: GetLinkableQuiz :one
-- A quiz step may point only at a published, non-archived quiz of the same school.
SELECT id FROM quizzes WHERE org_id = $1 AND id = $2 AND archived_at IS NULL AND published_at IS NOT NULL AND published_revision_id IS NOT NULL;
