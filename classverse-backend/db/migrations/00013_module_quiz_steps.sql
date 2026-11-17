-- +goose Up
-- Q1: a module step can point at a quiz. Passing the quiz completes the step (no manual completion).
ALTER TABLE module_steps ADD COLUMN quiz_id uuid;
ALTER TABLE module_steps ADD CONSTRAINT module_steps_quiz_fkey FOREIGN KEY (org_id, quiz_id) REFERENCES quizzes(org_id, id);
ALTER TABLE module_steps DROP CONSTRAINT module_steps_kind_check;
ALTER TABLE module_steps ADD CONSTRAINT module_steps_kind_check CHECK (kind IN ('materi', 'berkas', 'video', 'kuis'));
ALTER TABLE module_steps ADD CONSTRAINT module_steps_quiz_check CHECK ((kind = 'kuis') = (quiz_id IS NOT NULL));
CREATE INDEX module_steps_quiz_idx ON module_steps (org_id, quiz_id) WHERE archived_at IS NULL;

-- +goose Down
DROP INDEX module_steps_quiz_idx;
-- Quiz steps cannot exist under the old kind rule; archive-only data is dropped with the column.
DELETE FROM module_progress p USING module_steps s WHERE p.org_id = s.org_id AND p.step_id = s.id AND s.kind = 'kuis';
DELETE FROM module_steps WHERE kind = 'kuis';
ALTER TABLE module_steps DROP CONSTRAINT module_steps_quiz_check;
ALTER TABLE module_steps DROP CONSTRAINT module_steps_kind_check;
ALTER TABLE module_steps ADD CONSTRAINT module_steps_kind_check CHECK (kind IN ('materi', 'berkas', 'video'));
ALTER TABLE module_steps DROP CONSTRAINT module_steps_quiz_fkey;
ALTER TABLE module_steps DROP COLUMN quiz_id;
