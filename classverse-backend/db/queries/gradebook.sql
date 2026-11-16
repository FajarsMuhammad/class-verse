-- name: GradebookStudents :many
SELECT u.id,u.display_name,e.class_id,c.name AS class_name
FROM memberships m JOIN users u ON u.id=m.user_id
LEFT JOIN class_enrollments e ON e.org_id=m.org_id AND e.user_id=m.user_id AND e.academic_year_id=sqlc.narg(year_id)::uuid AND e.ended_at IS NULL
LEFT JOIN classes c ON c.org_id=e.org_id AND c.id=e.class_id
WHERE m.org_id=sqlc.arg(org_id) AND m.role='participant'
 AND (sqlc.narg(class_id)::uuid IS NULL OR e.class_id=sqlc.narg(class_id)::uuid)
ORDER BY lower(u.display_name),u.id;

-- name: GradebookModules :many
SELECT m.id,m.title,m.building_key,m.level,(m.archived_at IS NOT NULL)::boolean AS archived,
 (SELECT count(*)::integer FROM module_steps s WHERE s.org_id=m.org_id AND s.module_id=m.id AND s.archived_at IS NULL) AS total,
 (SELECT count(*)::integer FROM module_steps s JOIN module_progress p ON p.org_id=s.org_id AND p.step_id=s.id AND p.user_id=sqlc.arg(user_id)
 WHERE s.org_id=m.org_id AND s.module_id=m.id AND s.archived_at IS NULL) AS done
FROM modules m WHERE m.org_id=sqlc.arg(org_id)
 AND (sqlc.arg(building_key)::text='' OR m.building_key=sqlc.arg(building_key))
 AND (sqlc.narg(module_id)::uuid IS NULL OR m.id=sqlc.narg(module_id)::uuid)
 AND ((m.published_at IS NOT NULL AND m.archived_at IS NULL) OR EXISTS(SELECT 1 FROM module_steps s JOIN module_progress p ON p.org_id=s.org_id AND p.step_id=s.id
  WHERE s.org_id=m.org_id AND s.module_id=m.id AND p.user_id=sqlc.arg(user_id)))
ORDER BY m.building_key,m.title,m.id;

-- name: GradebookQuizzes :many
SELECT z.id,z.title,z.building_key,(z.archived_at IS NOT NULL)::boolean AS archived,
 (SELECT count(*)::integer FROM quiz_attempts a WHERE a.org_id=z.org_id AND a.quiz_id=z.id AND a.user_id=sqlc.arg(user_id) AND a.finished_at IS NOT NULL) AS attempts,
 COALESCE((SELECT max(a.score) FROM quiz_attempts a WHERE a.org_id=z.org_id AND a.quiz_id=z.id AND a.user_id=sqlc.arg(user_id) AND a.finished_at IS NOT NULL),-1)::integer AS best,
 COALESCE((SELECT a.score FROM quiz_attempts a WHERE a.org_id=z.org_id AND a.quiz_id=z.id AND a.user_id=sqlc.arg(user_id) AND a.finished_at IS NOT NULL ORDER BY a.finished_at DESC,a.id DESC LIMIT 1),-1)::integer AS last
FROM quizzes z WHERE z.org_id=sqlc.arg(org_id)
 AND (sqlc.arg(building_key)::text='' OR z.building_key=sqlc.arg(building_key))
 AND (sqlc.narg(quiz_id)::uuid IS NULL OR z.id=sqlc.narg(quiz_id)::uuid)
 AND ((z.published_at IS NOT NULL AND z.archived_at IS NULL) OR EXISTS(SELECT 1 FROM quiz_attempts a WHERE a.org_id=z.org_id AND a.quiz_id=z.id AND a.user_id=sqlc.arg(user_id) AND a.finished_at IS NOT NULL))
ORDER BY z.building_key,z.title,z.id;

-- name: GradebookAssignments :many
SELECT a.id,a.title,a.class_name,a.year_label,a.building_key,(a.cancelled_at IS NOT NULL)::boolean AS cancelled,a.due_at,
 (SELECT count(*)::integer FROM assignment_steps s WHERE s.org_id=a.org_id AND s.assignment_id=a.id) AS total,
 (SELECT count(*)::integer FROM assignment_progress p WHERE p.org_id=a.org_id AND p.assignment_id=a.id AND p.user_id=r.user_id) AS done
FROM assignments a JOIN assignment_recipients r ON r.org_id=a.org_id AND r.assignment_id=a.id AND r.user_id=sqlc.arg(user_id)
WHERE a.org_id=sqlc.arg(org_id)
 AND (sqlc.arg(building_key)::text='' OR a.building_key=sqlc.arg(building_key))
 AND (sqlc.narg(module_id)::uuid IS NULL OR a.module_id=sqlc.narg(module_id)::uuid)
ORDER BY a.created_at DESC,a.id;

-- name: GradebookContentFilter :one
SELECT (sqlc.narg(module_id)::uuid IS NULL OR EXISTS(SELECT 1 FROM modules m WHERE m.org_id=sqlc.arg(org_id) AND m.id=sqlc.narg(module_id)::uuid))
 AND (sqlc.narg(quiz_id)::uuid IS NULL OR EXISTS(SELECT 1 FROM quizzes z WHERE z.org_id=sqlc.arg(org_id) AND z.id=sqlc.narg(quiz_id)::uuid))::boolean AS valid;
