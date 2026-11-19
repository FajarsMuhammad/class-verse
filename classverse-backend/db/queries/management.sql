-- name: ManagementSummary :one
SELECT
 (SELECT count(*) FROM modules m WHERE m.org_id = sqlc.arg(org_id) AND archived_at IS NULL AND published_at IS NOT NULL)::integer AS published_modules,
 (SELECT count(*) FROM quizzes q WHERE q.org_id = sqlc.arg(org_id) AND archived_at IS NULL AND published_at IS NOT NULL)::integer AS published_quizzes,
 (SELECT count(*) FROM assignments a WHERE a.org_id = sqlc.arg(org_id) AND cancelled_at IS NULL)::integer AS active_assignments,
 (SELECT count(*) FROM modules m WHERE m.org_id = sqlc.arg(org_id) AND archived_at IS NULL AND published_at IS NULL)::integer AS draft_modules,
 (SELECT count(*) FROM quizzes q WHERE q.org_id = sqlc.arg(org_id) AND q.archived_at IS NULL AND NOT EXISTS (SELECT 1 FROM quiz_questions x WHERE x.quiz_id = q.id))::integer AS empty_quizzes,
 (SELECT count(*) FROM memberships m WHERE m.org_id = sqlc.arg(org_id) AND m.role = 'participant')::integer AS students,
 (SELECT count(*) FROM classes c JOIN academic_years y ON y.id = c.academic_year_id WHERE c.org_id = sqlc.arg(org_id) AND c.archived_at IS NULL AND y.active AND NOT EXISTS (SELECT 1 FROM class_teachers t WHERE t.class_id = c.id AND t.role = 'homeroom' AND t.ended_at IS NULL))::integer AS missing_homerooms,
 (SELECT count(*) FROM classes c JOIN academic_years y ON y.id = c.academic_year_id WHERE c.org_id = sqlc.arg(org_id) AND c.archived_at IS NULL AND y.active AND NOT EXISTS (SELECT 1 FROM class_enrollments e WHERE e.class_id = c.id AND e.ended_at IS NULL))::integer AS empty_classes;
