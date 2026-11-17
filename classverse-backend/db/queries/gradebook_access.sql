-- name: GradebookClassExists :one
SELECT EXISTS(SELECT 1 FROM classes WHERE org_id=$1 AND academic_year_id=$2 AND id=$3)::boolean;

-- name: GradebookStudentName :one
SELECT u.display_name FROM memberships m JOIN users u ON u.id=m.user_id
WHERE m.org_id=$1 AND m.user_id=sqlc.arg(target_id) AND (m.role='participant' OR m.user_id=sqlc.arg(actor_id));

-- name: GradebookStudentCount :one
SELECT count(*)::integer FROM memberships m WHERE m.org_id=sqlc.arg(org_id) AND m.role='participant'
 AND (sqlc.narg(class_id)::uuid IS NULL OR EXISTS(SELECT 1 FROM class_enrollments e WHERE e.org_id=m.org_id AND e.user_id=m.user_id
 AND e.academic_year_id=sqlc.narg(year_id)::uuid AND e.class_id=sqlc.narg(class_id)::uuid AND e.ended_at IS NULL));
