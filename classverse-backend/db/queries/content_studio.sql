-- name: BumpModuleDraft :one
UPDATE modules SET draft_version=draft_version+1 WHERE org_id=$1 AND id=$2 AND draft_version=$3 AND archived_at IS NULL RETURNING draft_version;

-- name: BumpQuizDraft :one
UPDATE quizzes SET draft_version=draft_version+1 WHERE org_id=$1 AND id=$2 AND draft_version=$3 AND archived_at IS NULL RETURNING draft_version;

-- name: InsertQuizRevision :one
INSERT INTO quiz_revisions(id,org_id,quiz_id,revision,draft_version,building_key,level,title,description,pass_score,questions,published_at)
SELECT gen_random_uuid(),z.org_id,z.id,COALESCE((SELECT max(r.revision)+1 FROM quiz_revisions r WHERE r.quiz_id=z.id),1),z.draft_version,
 z.building_key,z.level,z.title,z.description,z.pass_score,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',q.id,'prompt',q.prompt,'options',q.options,'answer',q.answer,'explanation',q.explanation) ORDER BY q.position,q.id) FROM quiz_questions q WHERE q.quiz_id=z.id),'[]'::jsonb),sqlc.arg(published_at)
FROM quizzes z WHERE z.org_id=$1 AND z.id=$2
ON CONFLICT(quiz_id,draft_version) DO NOTHING RETURNING *;

-- name: PublishedQuizRevision :one
SELECT r.* FROM quiz_revisions r JOIN quizzes z ON z.org_id=r.org_id AND z.id=r.quiz_id
WHERE r.org_id=$1 AND r.quiz_id=$2 AND r.id=z.published_revision_id;

-- name: PublishQuizPointer :one
UPDATE quizzes SET published_revision_id=$3,published_at=$4 WHERE org_id=$1 AND id=$2 RETURNING *;

-- name: InsertModuleRevision :one
INSERT INTO module_revisions(id,org_id,module_id,revision,draft_version,building_key,level,title,description,published_at)
SELECT gen_random_uuid(),m.org_id,m.id,COALESCE((SELECT max(r.revision)+1 FROM module_revisions r WHERE r.module_id=m.id),1),m.draft_version,
 m.building_key,m.level,m.title,m.description,sqlc.arg(published_at)
FROM modules m WHERE m.org_id=$1 AND m.id=$2
ON CONFLICT(module_id,draft_version) DO NOTHING RETURNING *;

-- name: SnapshotModuleSteps :execrows
INSERT INTO module_revision_steps(org_id,module_id,revision_id,id,position,kind,title,body,rich_content,file_id,video_id,quiz_id,quiz_revision_id,minutes)
SELECT s.org_id,s.module_id,sqlc.arg(revision_id),s.id,s.position,s.kind,s.title,s.body,s.rich_content,s.file_id,s.video_id,s.quiz_id,z.published_revision_id,s.minutes
FROM module_steps s LEFT JOIN quizzes z ON z.org_id=s.org_id AND z.id=s.quiz_id
WHERE s.org_id=sqlc.arg(org_id) AND s.module_id=sqlc.arg(module_id) AND s.archived_at IS NULL;

-- name: PublishModulePointer :one
UPDATE modules SET published_revision_id=$3,published_at=$4 WHERE org_id=$1 AND id=$2 RETURNING *;

-- name: ModuleRevisionHistory :many
SELECT id,revision,draft_version,published_at FROM module_revisions WHERE org_id=$1 AND module_id=$2 ORDER BY revision DESC;

-- name: QuizRevisionHistory :many
SELECT id,revision,draft_version,published_at FROM quiz_revisions WHERE org_id=$1 AND quiz_id=$2 ORDER BY revision DESC;

-- name: PinModuleRevision :exec
INSERT INTO module_revision_pins(org_id,module_id,user_id,revision_id)
SELECT m.org_id,m.id,sqlc.arg(user_id),m.published_revision_id FROM modules m
WHERE m.org_id=sqlc.arg(org_id) AND m.id=sqlc.arg(module_id) AND m.archived_at IS NULL AND m.published_at IS NOT NULL AND m.published_revision_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- name: ReadModuleRevision :one
SELECT r.* FROM modules m JOIN module_revisions r ON r.org_id=m.org_id AND r.module_id=m.id
LEFT JOIN module_revision_pins p ON p.org_id=m.org_id AND p.module_id=m.id AND p.user_id=sqlc.arg(user_id)
WHERE m.org_id=sqlc.arg(org_id) AND m.id=sqlc.arg(module_id) AND m.archived_at IS NULL AND m.published_at IS NOT NULL
AND r.id=COALESCE(p.revision_id,m.published_revision_id);

-- name: ListReadRevisions :many
-- Read-only: learner pin when present, else current published revision. Listing never pins.
SELECT r.* FROM modules m LEFT JOIN module_revision_pins p ON p.org_id=m.org_id AND p.module_id=m.id AND p.user_id=sqlc.arg(user_id)
JOIN module_revisions r ON r.org_id=m.org_id AND r.module_id=m.id AND r.id=COALESCE(p.revision_id,m.published_revision_id)
WHERE m.org_id=sqlc.arg(org_id) AND m.id=ANY(sqlc.arg(module_ids)::uuid[]) AND m.archived_at IS NULL AND m.published_at IS NOT NULL;

-- name: ListRevisionSteps :many
SELECT s.*, COALESCE(f.kind,'')::text AS file_kind,COALESCE(r.position,0)::integer AS reader_position,
 EXISTS(SELECT 1 FROM module_progress p WHERE p.org_id=s.org_id AND p.user_id=sqlc.arg(user_id) AND p.step_id=s.id) AS done
FROM module_revision_steps s
LEFT JOIN files f ON f.org_id=s.org_id AND f.id=s.file_id
LEFT JOIN reader_positions r ON r.org_id=s.org_id AND r.user_id=sqlc.arg(user_id) AND r.file_id=s.file_id
WHERE s.org_id=sqlc.arg(org_id) AND s.revision_id=ANY(sqlc.arg(revision_ids)::uuid[]) ORDER BY s.position,s.id;

-- name: ModuleQuizRevision :one
SELECT r.* FROM module_revision_steps s JOIN quiz_revisions r ON r.org_id=s.org_id AND r.quiz_id=s.quiz_id AND r.id=s.quiz_revision_id
JOIN module_revision_pins p ON p.org_id=s.org_id AND p.module_id=s.module_id AND p.revision_id=s.revision_id AND p.user_id=sqlc.arg(user_id)
JOIN modules m ON m.org_id=p.org_id AND m.id=p.module_id AND m.published_at IS NOT NULL AND m.archived_at IS NULL
WHERE s.org_id=sqlc.arg(org_id) AND s.module_id=sqlc.arg(module_id) AND s.id=sqlc.arg(step_id) AND s.quiz_id=sqlc.arg(quiz_id);

-- name: GetQuizRevision :one
SELECT * FROM quiz_revisions WHERE org_id=$1 AND quiz_id=$2 AND id=$3;

-- name: SetStepRichContent :exec
UPDATE module_steps SET rich_content=$4 WHERE org_id=$1 AND module_id=$2 AND id=$3;

-- name: LockDraftFileReferences :many
SELECT f.id FROM files f WHERE f.org_id=sqlc.arg(org_id)
 AND f.id IN(SELECT s.file_id FROM module_steps s WHERE s.org_id=f.org_id AND s.module_id=sqlc.arg(module_id) AND s.archived_at IS NULL)
 ORDER BY f.id FOR UPDATE;

-- name: SnapshotFileReadable :one
SELECT EXISTS(SELECT 1 FROM module_revision_steps s JOIN modules m ON m.org_id=s.org_id AND m.id=s.module_id
LEFT JOIN module_revision_pins p ON p.org_id=m.org_id AND p.module_id=m.id AND p.user_id=sqlc.arg(user_id)
WHERE s.org_id=sqlc.arg(org_id) AND s.file_id=sqlc.arg(file_id) AND m.archived_at IS NULL AND m.published_at IS NOT NULL
AND s.revision_id=COALESCE(p.revision_id,m.published_revision_id))::boolean;

-- name: LockAttemptQuiz :one
SELECT * FROM quizzes WHERE org_id=$1 AND id=$2 FOR UPDATE;

-- name: ModuleRevisionForDraft :one
SELECT * FROM module_revisions WHERE org_id=$1 AND module_id=$2 AND draft_version=$3;

-- name: QuizRevisionForDraft :one
SELECT * FROM quiz_revisions WHERE org_id=$1 AND quiz_id=$2 AND draft_version=$3;

-- name: LockPublishedStepQuizzes :many
SELECT z.id FROM quizzes z WHERE z.org_id=sqlc.arg(org_id)
AND z.id IN(SELECT s.quiz_id FROM module_steps s WHERE s.org_id=z.org_id AND s.module_id=sqlc.arg(module_id) AND s.archived_at IS NULL)
ORDER BY z.id FOR SHARE;
