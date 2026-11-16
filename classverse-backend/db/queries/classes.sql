-- name: ListAcademicYears :many
SELECT * FROM academic_years WHERE org_id = $1 ORDER BY label DESC;

-- name: CountAcademicYears :one
SELECT count(*)::integer FROM academic_years WHERE org_id = $1;

-- name: CreateAcademicYear :one
INSERT INTO academic_years (id, org_id, label, active, created_at) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAcademicYear :one
SELECT * FROM academic_years WHERE org_id = $1 AND id = $2;

-- name: ClearActiveYear :exec
UPDATE academic_years SET active = false WHERE org_id = $1 AND active;

-- name: SetActiveYear :execrows
UPDATE academic_years SET active = true WHERE org_id = $1 AND id = $2;

-- name: LockOrgYears :exec
-- Serializes year creation/activation per school (first year becomes active exactly once).
SELECT pg_advisory_xact_lock(hashtextextended('academic_years:' || sqlc.arg(org_id)::uuid::text, 0));

-- name: ListClasses :many
-- Active classes of a year with their active student count.
SELECT c.*, (SELECT count(*) FROM class_enrollments e WHERE e.class_id = c.id AND e.ended_at IS NULL)::integer AS students
FROM classes c
WHERE c.org_id = $1 AND c.academic_year_id = $2 AND c.archived_at IS NULL
ORDER BY lower(c.name), c.id;

-- name: CreateClass :one
INSERT INTO classes (id, org_id, academic_year_id, name, created_by, created_at) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: LockClass :one
SELECT * FROM classes WHERE org_id = $1 AND id = $2 AND archived_at IS NULL FOR UPDATE;

-- name: RenameClass :one
UPDATE classes SET name = $3 WHERE classes.org_id = $1 AND classes.id = $2 AND classes.archived_at IS NULL
RETURNING classes.*, (SELECT count(*) FROM class_enrollments e WHERE e.class_id = classes.id AND e.ended_at IS NULL)::integer AS students;

-- name: CountActiveEnrollments :one
SELECT count(*)::integer FROM class_enrollments WHERE class_id = $1 AND ended_at IS NULL;

-- name: ArchiveClass :exec
UPDATE classes SET archived_at = $3 WHERE org_id = $1 AND id = $2;

-- name: ListRoster :many
-- Every participant of the school with their active class in the given year (NULL: not placed yet).
SELECT u.id AS user_id, u.display_name, u.avatar_key, e.class_id
FROM memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN class_enrollments e ON e.org_id = m.org_id AND e.user_id = m.user_id
  AND e.academic_year_id = sqlc.arg(academic_year_id) AND e.ended_at IS NULL
WHERE m.org_id = sqlc.arg(org_id) AND m.role = 'participant'
ORDER BY lower(u.display_name), u.id;

-- name: GetMemberRoleForUpdate :one
-- Locks the membership row so concurrent placements of one student are serialized.
SELECT role FROM memberships WHERE org_id = $1 AND user_id = $2 FOR UPDATE;

-- name: ActiveEnrollment :one
SELECT * FROM class_enrollments WHERE org_id = $1 AND academic_year_id = $2 AND user_id = $3 AND ended_at IS NULL;

-- name: EndEnrollment :exec
UPDATE class_enrollments SET ended_at = $2 WHERE id = $1 AND ended_at IS NULL;

-- name: CreateEnrollment :one
INSERT INTO class_enrollments (id, org_id, academic_year_id, class_id, user_id, started_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: EnrollmentHistory :many
-- A student's placements in one school, newest first (all years), with class names.
SELECT e.*, c.name AS class_name, y.label AS year_label
FROM class_enrollments e
JOIN classes c ON c.id = e.class_id
JOIN academic_years y ON y.id = e.academic_year_id
WHERE e.org_id = $1 AND e.user_id = $2
ORDER BY e.started_at DESC, e.id;
