-- name: AssignmentQuizContext :one
SELECT s.* FROM assignment_steps s JOIN assignments a ON a.org_id=s.org_id AND a.id=s.assignment_id
JOIN assignment_recipients r ON r.org_id=a.org_id AND r.assignment_id=a.id AND r.user_id=sqlc.arg(user_id)
WHERE s.org_id=sqlc.arg(org_id) AND s.id=sqlc.arg(step_id) AND s.kind='kuis' AND a.cancelled_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM assignment_steps prev WHERE prev.assignment_id=s.assignment_id AND prev.position<s.position
  AND NOT EXISTS(SELECT 1 FROM assignment_progress p WHERE p.assignment_id=prev.assignment_id AND p.step_id=prev.id AND p.user_id=r.user_id));

-- name: AssignmentFileReadable :one
SELECT EXISTS(SELECT 1 FROM assignment_steps s JOIN assignments a ON a.org_id=s.org_id AND a.id=s.assignment_id
JOIN assignment_recipients r ON r.org_id=a.org_id AND r.assignment_id=a.id AND r.user_id=sqlc.arg(user_id)
WHERE s.org_id=sqlc.arg(org_id) AND s.file_id=sqlc.arg(file_id) AND a.cancelled_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM assignment_steps prev WHERE prev.assignment_id=s.assignment_id AND prev.position<s.position
  AND NOT EXISTS(SELECT 1 FROM assignment_progress p WHERE p.assignment_id=prev.assignment_id AND p.step_id=prev.id AND p.user_id=r.user_id)))::boolean;

-- name: LockAssignmentFileReferences :many
-- File deletion locks files first; issuance takes matching locks before adding durable references.
SELECT f.id FROM files f WHERE f.org_id=sqlc.arg(org_id)
 AND f.id IN (SELECT s.file_id FROM module_revision_steps s JOIN modules m ON m.org_id=s.org_id AND m.id=s.module_id AND m.published_revision_id=s.revision_id WHERE s.org_id=f.org_id AND s.module_id=sqlc.arg(module_id))
 ORDER BY f.id FOR UPDATE;

-- name: LockAssignmentRequest :exec
SELECT pg_advisory_xact_lock(hashtextextended('assignment_request:' || sqlc.arg(org_id)::uuid::text || ':' || sqlc.arg(request_id)::uuid::text,0));
