-- name: ListAcademicYears :many
SELECT y.*, count(*) OVER()::integer AS total_count,
  (SELECT count(*) FROM classes c WHERE c.academic_year_id = y.id AND c.archived_at IS NULL)::integer AS class_count,
  (SELECT count(*) FROM class_enrollments e WHERE e.academic_year_id = y.id AND e.ended_at IS NULL)::integer AS student_count
FROM academic_years y WHERE y.org_id = $1 ORDER BY y.active DESC, y.label DESC, y.id LIMIT sqlc.narg(page_limit)::integer OFFSET sqlc.arg(page_offset)::integer;

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
-- Active classes of a year with student count, wali kelas and guru mapel count. Classes the caller teaches first.
SELECT c.id, c.academic_year_id, c.name, count(*) OVER()::integer AS total_count,
  (SELECT count(*) FROM class_enrollments e WHERE e.class_id = c.id AND e.ended_at IS NULL)::integer AS students,
  (SELECT count(*) FROM class_teachers t WHERE t.class_id = c.id AND t.role = 'subject' AND t.ended_at IS NULL)::integer AS subject_teachers,
  h.user_id AS homeroom_id, hu.display_name AS homeroom_name, mine.role AS my_role
FROM classes c
LEFT JOIN class_teachers h ON h.class_id = c.id AND h.role = 'homeroom' AND h.ended_at IS NULL
LEFT JOIN users hu ON hu.id = h.user_id
LEFT JOIN class_teachers mine ON mine.class_id = c.id AND mine.user_id = sqlc.arg(user_id) AND mine.ended_at IS NULL
WHERE c.org_id = sqlc.arg(org_id) AND c.academic_year_id = sqlc.arg(academic_year_id) AND c.archived_at IS NULL
  AND (sqlc.narg(class_id)::uuid IS NULL OR c.id = sqlc.narg(class_id))
  AND strpos(lower(c.name), lower(sqlc.arg(search)::text)) > 0
ORDER BY mine.role IS NULL, lower(c.name), c.id LIMIT sqlc.narg(page_limit)::integer OFFSET sqlc.arg(page_offset)::integer;

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
-- One page of the school's participants with their active class in the year (NULL: no class yet).
-- Filters: class_id (that class), unplaced (no class), q (case-insensitive name substring).
SELECT u.id AS user_id, u.display_name, u.avatar_key, e.class_id,
  (SELECT count(*) FROM assignment_progress p JOIN assignments a ON a.org_id = p.org_id AND a.id = p.assignment_id
    JOIN classes ac ON ac.org_id = a.org_id AND ac.id = a.class_id
    WHERE p.org_id = m.org_id AND p.user_id = m.user_id AND a.cancelled_at IS NULL AND ac.academic_year_id = sqlc.arg(academic_year_id))::integer AS steps_done,
  (SELECT count(*) FROM assignment_recipients r JOIN assignments a ON a.org_id = r.org_id AND a.id = r.assignment_id
    JOIN classes ac ON ac.org_id = a.org_id AND ac.id = a.class_id
    JOIN assignment_steps st ON st.assignment_id = a.id
    WHERE r.org_id = m.org_id AND r.user_id = m.user_id AND a.cancelled_at IS NULL AND ac.academic_year_id = sqlc.arg(academic_year_id))::integer AS steps_total
FROM memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN class_enrollments e ON e.org_id = m.org_id AND e.user_id = m.user_id
  AND e.academic_year_id = sqlc.arg(academic_year_id) AND e.ended_at IS NULL
WHERE m.org_id = sqlc.arg(org_id) AND m.role = 'participant'
  AND (sqlc.narg(class_id)::uuid IS NULL OR e.class_id = sqlc.narg(class_id))
  AND (NOT sqlc.arg(unplaced)::boolean OR e.class_id IS NULL)
  AND strpos(lower(u.display_name), lower(sqlc.arg(q)::text)) > 0
ORDER BY lower(u.display_name), u.id
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountRoster :one
SELECT count(*)::integer
FROM memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN class_enrollments e ON e.org_id = m.org_id AND e.user_id = m.user_id
  AND e.academic_year_id = sqlc.arg(academic_year_id) AND e.ended_at IS NULL
WHERE m.org_id = sqlc.arg(org_id) AND m.role = 'participant'
  AND (sqlc.narg(class_id)::uuid IS NULL OR e.class_id = sqlc.narg(class_id))
  AND (NOT sqlc.arg(unplaced)::boolean OR e.class_id IS NULL)
  AND strpos(lower(u.display_name), lower(sqlc.arg(q)::text)) > 0;

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

-- name: ListStaff :many
-- Teachers and admins of the school: candidates for wali kelas and guru mapel.
SELECT u.id AS user_id, u.display_name, m.role, count(*) OVER()::integer AS total_count
FROM memberships m JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1 AND m.role IN ('teacher', 'admin')
ORDER BY lower(u.display_name), u.id LIMIT sqlc.narg(page_limit)::integer OFFSET sqlc.arg(page_offset)::integer;

-- name: ListClassTeachers :many
SELECT t.user_id, u.display_name, t.role, t.subject
FROM class_teachers t JOIN users u ON u.id = t.user_id
WHERE t.org_id = $1 AND t.class_id = $2 AND t.ended_at IS NULL
ORDER BY t.role, lower(u.display_name), u.id;

-- name: ClassTeacherRole :one
-- The caller's active role in a class (no row: not a teacher of it).
SELECT role FROM class_teachers WHERE class_id = $1 AND user_id = $2 AND ended_at IS NULL;

-- name: EndHomeroom :exec
UPDATE class_teachers SET ended_at = $2 WHERE class_id = $1 AND role = 'homeroom' AND ended_at IS NULL;

-- name: EndClassTeacher :execrows
UPDATE class_teachers SET ended_at = $3 WHERE class_id = $1 AND user_id = $2 AND ended_at IS NULL;

-- name: EndClassTeachers :exec
UPDATE class_teachers SET ended_at = $2 WHERE class_id = $1 AND ended_at IS NULL;

-- name: CreateClassTeacher :exec
INSERT INTO class_teachers (id, org_id, class_id, user_id, role, subject, started_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListYearClasses :many
-- Active classes of a year (source of copy/promotion).
SELECT * FROM classes WHERE org_id = $1 AND academic_year_id = $2 AND archived_at IS NULL ORDER BY lower(name), id;

-- name: CreateClassIfFree :one
-- Copy target: skips (no row) when an active class of that year already has the name.
INSERT INTO classes (id, org_id, academic_year_id, name, created_by, created_at) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (org_id, academic_year_id, lower(name)) WHERE archived_at IS NULL DO NOTHING
RETURNING id;

-- name: CopyClassTeachers :exec
-- Copies active teachers who are still staff of the school.
INSERT INTO class_teachers (id, org_id, class_id, user_id, role, subject, started_at, created_by)
SELECT gen_random_uuid(), t.org_id, sqlc.arg(to_class_id), t.user_id, t.role, t.subject, sqlc.arg(started_at), sqlc.arg(created_by)
FROM class_teachers t
JOIN memberships m ON m.org_id = t.org_id AND m.user_id = t.user_id AND m.role IN ('teacher', 'admin')
WHERE t.class_id = sqlc.arg(from_class_id) AND t.ended_at IS NULL;

-- name: ClassStudents :many
-- Active students of a class with names (promotion source).
SELECT e.user_id, u.display_name FROM class_enrollments e JOIN users u ON u.id = e.user_id
WHERE e.class_id = $1 AND e.ended_at IS NULL ORDER BY lower(u.display_name), u.id;

-- name: GetClass :one
SELECT * FROM classes WHERE org_id = $1 AND id = $2 AND archived_at IS NULL;
