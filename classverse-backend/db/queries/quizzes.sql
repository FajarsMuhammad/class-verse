-- name: CreateQuiz :one
INSERT INTO quizzes (id, org_id, building_key, level, title, description, pass_score, created_by, created_at, updated_at, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10)
RETURNING *;

-- name: LockQuiz :one
SELECT * FROM quizzes WHERE org_id = $1 AND id = $2 AND archived_at IS NULL FOR UPDATE;

-- name: UpdateQuiz :one
UPDATE quizzes SET building_key = $3, level = $4, title = $5, description = $6, pass_score = $7, published_at = $8, updated_at = $9
WHERE org_id = $1 AND id = $2 AND archived_at IS NULL
RETURNING *;

-- name: ArchiveQuiz :execrows
UPDATE quizzes SET archived_at = $3, updated_at = $3 WHERE org_id = $1 AND id = $2 AND archived_at IS NULL;

-- name: ListQuizzes :many
-- Staff may include drafts; archived quizzes never show. best_score is the caller's highest finished score.
SELECT sqlc.embed(z),
  (SELECT count(*) FROM quiz_questions q WHERE q.quiz_id = z.id)::integer AS question_count,
  -- -1 when the caller has no finished attempt yet.
  COALESCE((SELECT max(a.score) FROM quiz_attempts a WHERE a.quiz_id = z.id AND a.user_id = sqlc.arg(user_id)), -1)::smallint AS best_score,
  (SELECT count(*) FROM quiz_attempts a WHERE a.quiz_id = z.id AND a.user_id = sqlc.arg(user_id) AND a.finished_at IS NOT NULL)::integer AS attempts
FROM quizzes z
WHERE z.org_id = sqlc.arg(org_id) AND z.archived_at IS NULL
  AND (sqlc.arg(include_drafts)::boolean OR z.published_at IS NOT NULL)
  AND (sqlc.narg(building_key)::text IS NULL OR z.building_key = sqlc.narg(building_key))
  AND (sqlc.narg(level)::text IS NULL OR z.level = sqlc.narg(level))
ORDER BY z.created_at, z.id
LIMIT 100;

-- name: ListQuizQuestions :many
SELECT * FROM quiz_questions WHERE quiz_id = $1 ORDER BY position, id;

-- name: NextQuestionPosition :one
SELECT COALESCE(max(position) + 1, 0)::integer FROM quiz_questions WHERE quiz_id = $1;

-- name: CreateQuestion :one
INSERT INTO quiz_questions (id, org_id, quiz_id, position, prompt, options, answer, explanation, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING *;

-- name: UpdateQuestion :one
UPDATE quiz_questions SET prompt = $4, options = $5, answer = $6, explanation = $7, updated_at = $8
WHERE org_id = $1 AND quiz_id = $2 AND id = $3
RETURNING *;

-- name: DeleteQuestion :execrows
DELETE FROM quiz_questions WHERE org_id = $1 AND quiz_id = $2 AND id = $3;

-- name: QuestionIDs :many
SELECT id FROM quiz_questions WHERE org_id = $1 AND quiz_id = $2 ORDER BY position, id;

-- name: SetQuestionPosition :exec
UPDATE quiz_questions SET position = $3 WHERE quiz_id = $1 AND id = $2;

-- name: OpenAttempt :one
SELECT * FROM quiz_attempts WHERE quiz_id = $1 AND user_id = $2 AND finished_at IS NULL
 AND assignment_step_id IS NOT DISTINCT FROM sqlc.narg(assignment_step_id)::uuid;

-- name: CreateAttempt :one
INSERT INTO quiz_attempts (id, org_id, quiz_id, user_id, questions, answers, pass_score, started_at, assignment_id, assignment_step_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, sqlc.narg(assignment_id)::uuid, sqlc.narg(assignment_step_id)::uuid)
RETURNING *;

-- name: LockAttempt :one
-- Own attempt only; other users and schools see not found.
SELECT * FROM quiz_attempts WHERE org_id = $1 AND quiz_id = $2 AND id = $3 AND user_id = $4 FOR UPDATE;

-- name: SetAnswers :exec
UPDATE quiz_attempts SET answers = $2 WHERE id = $1 AND finished_at IS NULL;

-- name: FinishAttempt :one
UPDATE quiz_attempts SET finished_at = $2, correct = $3, score = $4 WHERE id = $1 AND finished_at IS NULL
RETURNING *;

-- name: CompleteQuizSteps :exec
-- Passing a quiz completes every active step pointing at it in published modules. Idempotent.
INSERT INTO module_progress (org_id, user_id, step_id, completed_at)
SELECT s.org_id, sqlc.arg(user_id), s.id, sqlc.arg(completed_at)
FROM module_steps s JOIN modules m ON m.org_id = s.org_id AND m.id = s.module_id
WHERE s.org_id = sqlc.arg(org_id) AND s.quiz_id = sqlc.arg(quiz_id) AND s.archived_at IS NULL
  AND m.archived_at IS NULL AND m.published_at IS NOT NULL
ON CONFLICT DO NOTHING;
