-- name: GradebookRoster :many
WITH students AS (
 SELECT u.id,u.display_name,c.name AS class_name
 FROM memberships m JOIN users u ON u.id=m.user_id
 LEFT JOIN class_enrollments e ON e.org_id=m.org_id AND e.user_id=m.user_id AND e.academic_year_id=sqlc.narg(year_id)::uuid AND e.ended_at IS NULL
 LEFT JOIN classes c ON c.org_id=e.org_id AND c.id=e.class_id
 WHERE m.org_id=sqlc.arg(org_id) AND m.role='participant'
  AND (sqlc.narg(class_id)::uuid IS NULL OR e.class_id=sqlc.narg(class_id)::uuid)
), page AS (
 SELECT *,count(*) OVER()::integer AS total_students FROM students ORDER BY lower(display_name),id
 LIMIT sqlc.arg(page_limit)::integer OFFSET sqlc.arg(page_offset)::integer
), modules AS (
 SELECT m.id,m.published_revision_id FROM modules m
 WHERE m.org_id=sqlc.arg(org_id) AND m.published_at IS NOT NULL AND m.archived_at IS NULL
  AND (sqlc.arg(building_key)::text='' OR m.building_key=sqlc.arg(building_key))
  AND (sqlc.narg(module_id)::uuid IS NULL OR m.id=sqlc.narg(module_id)::uuid)

)
SELECT p.id,p.display_name,p.class_name,p.total_students,
 (SELECT count(*)::integer FROM modules) AS modules_total,
 (SELECT count(*)::integer FROM modules m
 LEFT JOIN module_revision_pins pin ON pin.org_id=sqlc.arg(org_id) AND pin.module_id=m.id AND pin.user_id=p.id
 WHERE EXISTS(SELECT 1 FROM module_revision_steps s WHERE s.revision_id=COALESCE(pin.revision_id,m.published_revision_id))
 AND NOT EXISTS(SELECT 1 FROM module_revision_steps s WHERE s.revision_id=COALESCE(pin.revision_id,m.published_revision_id)
 AND NOT EXISTS(SELECT 1 FROM module_progress mp WHERE mp.org_id=sqlc.arg(org_id) AND mp.user_id=p.id AND mp.step_id=s.id))) AS modules_done,
 (SELECT count(*)::integer FROM quizzes z WHERE z.org_id=sqlc.arg(org_id) AND z.published_at IS NOT NULL AND z.archived_at IS NULL
  AND (sqlc.arg(building_key)::text='' OR z.building_key=sqlc.arg(building_key))
  AND (sqlc.narg(quiz_id)::uuid IS NULL OR z.id=sqlc.narg(quiz_id)::uuid)) AS quizzes_total,
 (SELECT count(DISTINCT a.quiz_id)::integer FROM quiz_attempts a JOIN quizzes z ON z.org_id=a.org_id AND z.id=a.quiz_id
  WHERE a.org_id=sqlc.arg(org_id) AND a.user_id=p.id AND a.finished_at IS NOT NULL
   AND (sqlc.arg(building_key)::text='' OR z.building_key=sqlc.arg(building_key))
   AND (sqlc.narg(quiz_id)::uuid IS NULL OR z.id=sqlc.narg(quiz_id)::uuid)) AS quizzes_attempted,
 COALESCE((SELECT avg(best)::float8 FROM (
  SELECT max(a.score) AS best FROM quiz_attempts a JOIN quizzes z ON z.org_id=a.org_id AND z.id=a.quiz_id
  WHERE a.org_id=sqlc.arg(org_id) AND a.user_id=p.id AND a.finished_at IS NOT NULL
   AND (sqlc.arg(building_key)::text='' OR z.building_key=sqlc.arg(building_key))
   AND (sqlc.narg(quiz_id)::uuid IS NULL OR z.id=sqlc.narg(quiz_id)::uuid)
  GROUP BY a.quiz_id) scores),-1)::float8 AS average_best
FROM page p ORDER BY lower(p.display_name),p.id;
