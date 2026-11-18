-- name: GetAssignment :one
SELECT * FROM assignments WHERE org_id=$1 AND id=$2;

-- name: LockAssignment :one
SELECT * FROM assignments WHERE org_id=$1 AND id=$2 FOR UPDATE;

-- name: CreateAssignment :one
INSERT INTO assignments(id,org_id,class_id,module_id,title,class_name,year_label,building_key,level,due_at,created_by,created_at)
SELECT sqlc.arg(id), c.org_id, c.id, m.id, rev.title, c.name, y.label, rev.building_key,rev.level,
 sqlc.narg(due_at)::timestamptz,sqlc.arg(created_by),sqlc.arg(created_at)
FROM classes c JOIN academic_years y ON y.org_id=c.org_id AND y.id=c.academic_year_id
JOIN modules m ON m.org_id=c.org_id AND m.id=sqlc.arg(module_id)
JOIN module_revisions rev ON rev.org_id=m.org_id AND rev.module_id=m.id AND rev.id=m.published_revision_id
WHERE c.org_id=sqlc.arg(org_id) AND c.id=sqlc.arg(class_id) AND c.archived_at IS NULL
 AND m.archived_at IS NULL AND m.published_at IS NOT NULL
RETURNING *;

-- name: SnapshotAssignmentSteps :execrows
INSERT INTO assignment_steps(id,org_id,assignment_id,position,kind,title,body,file_id,video_id,quiz_id,minutes,rich_content,quiz_revision_id)
SELECT gen_random_uuid(),s.org_id,sqlc.arg(assignment_id),row_number() OVER(ORDER BY s.position,s.id)-1,
 s.kind,s.title,s.body,s.file_id,s.video_id,s.quiz_id,s.minutes,s.rich_content,s.quiz_revision_id
FROM module_revision_steps s JOIN modules m ON m.org_id=s.org_id AND m.id=s.module_id AND m.published_revision_id=s.revision_id
WHERE s.org_id=sqlc.arg(org_id) AND s.module_id=sqlc.arg(module_id);

-- name: SnapshotAssignmentRecipients :execrows
INSERT INTO assignment_recipients(org_id,assignment_id,user_id)
SELECT e.org_id,sqlc.arg(assignment_id),e.user_id FROM class_enrollments e
JOIN memberships m ON m.org_id=e.org_id AND m.user_id=e.user_id AND m.role='participant'
WHERE e.org_id=sqlc.arg(org_id) AND e.class_id=sqlc.arg(class_id) AND e.ended_at IS NULL;

-- name: ListAssignments :many
SELECT a.*,
 (SELECT count(*)::integer FROM assignment_steps s WHERE s.assignment_id=a.id) AS total,
 (SELECT count(*)::integer FROM assignment_progress p WHERE p.assignment_id=a.id AND p.user_id=sqlc.arg(user_id)) AS done
FROM assignments a WHERE a.org_id=sqlc.arg(org_id)
 AND (sqlc.arg(staff)::boolean OR EXISTS(SELECT 1 FROM assignment_recipients r WHERE r.org_id=a.org_id AND r.assignment_id=a.id AND r.user_id=sqlc.arg(user_id)))
ORDER BY a.created_at DESC,a.id;

-- name: OwnAssignmentRecipient :one
SELECT * FROM assignment_recipients WHERE org_id=$1 AND assignment_id=$2 AND user_id=$3;

-- name: AssignmentSteps :many
SELECT s.*, f.kind AS file_kind,
 EXISTS(SELECT 1 FROM assignment_progress p WHERE p.org_id=s.org_id AND p.assignment_id=s.assignment_id AND p.step_id=s.id AND p.user_id=sqlc.arg(user_id)) AS done,
 NOT EXISTS(SELECT 1 FROM assignment_steps prev WHERE prev.assignment_id=s.assignment_id AND prev.position<s.position
   AND NOT EXISTS(SELECT 1 FROM assignment_progress p WHERE p.assignment_id=prev.assignment_id AND p.step_id=prev.id AND p.user_id=sqlc.arg(user_id))) AS unlocked
FROM assignment_steps s LEFT JOIN files f ON f.org_id=s.org_id AND f.id=s.file_id WHERE s.org_id=sqlc.arg(org_id) AND s.assignment_id=sqlc.arg(assignment_id)
ORDER BY s.position;

-- name: CompleteAssignmentStep :exec
INSERT INTO assignment_progress(org_id,assignment_id,user_id,step_id,completed_at) VALUES($1,$2,$3,$4,$5)
ON CONFLICT DO NOTHING;

-- name: CancelAssignment :exec
UPDATE assignments SET cancelled_at=COALESCE(cancelled_at,$3) WHERE org_id=$1 AND id=$2;

-- name: AssignmentRoster :many
SELECT r.user_id,u.display_name,
 (SELECT count(*)::integer FROM assignment_progress p WHERE p.assignment_id=r.assignment_id AND p.user_id=r.user_id) AS done,
 (SELECT count(*)::integer FROM assignment_steps s WHERE s.assignment_id=r.assignment_id) AS total
FROM assignment_recipients r JOIN users u ON u.id=r.user_id
WHERE r.org_id=$1 AND r.assignment_id=$2 ORDER BY lower(u.display_name),r.user_id;

-- name: LockAssignmentRoster :exec
-- All placement changes and issuance lock one school/year before membership/class/module rows.
SELECT pg_advisory_xact_lock(hashtextextended('class_roster:' || sqlc.arg(org_id)::uuid::text || ':' || sqlc.arg(year_id)::uuid::text,0));

-- name: GetAssignmentClass :one
SELECT * FROM classes WHERE org_id=$1 AND id=$2 AND archived_at IS NULL;
