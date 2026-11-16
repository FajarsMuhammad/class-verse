-- +goose Up
-- Q1: self-paced quizzes per campus building and level. Draft: published_at NULL; archived, never deleted.
CREATE TABLE quizzes (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES organizations(id),
    building_key text NOT NULL CHECK (char_length(building_key) BETWEEN 1 AND 40),
    level text NOT NULL CHECK (level IN ('SD', 'SMP', 'SMA', 'Umum')),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    pass_score smallint NOT NULL DEFAULT 60 CHECK (pass_score BETWEEN 1 AND 100),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    published_at timestamptz,
    archived_at timestamptz,
    UNIQUE (org_id, id)
);
CREATE INDEX quizzes_org_building_idx ON quizzes (org_id, building_key, level) WHERE archived_at IS NULL;

-- Exactly four options and one answer. Questions may be deleted: attempts keep their own snapshot.
CREATE TABLE quiz_questions (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    quiz_id uuid NOT NULL,
    position integer NOT NULL CHECK (position >= 0),
    prompt text NOT NULL CHECK (char_length(prompt) BETWEEN 1 AND 500),
    options text[] NOT NULL CHECK (array_length(options, 1) = 4 AND array_position(options, NULL) IS NULL),
    answer smallint NOT NULL CHECK (answer BETWEEN 0 AND 3),
    explanation text NOT NULL DEFAULT '' CHECK (char_length(explanation) <= 500),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    FOREIGN KEY (org_id, quiz_id) REFERENCES quizzes(org_id, id)
);
CREATE INDEX quiz_questions_quiz_idx ON quiz_questions (quiz_id, position);

-- One row per try. `questions` snapshots prompt/options/answer/explanation at start so later edits never
-- change a result. answers[i] = -1 until answered. Score is set once, when the attempt is finished.
CREATE TABLE quiz_attempts (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    quiz_id uuid NOT NULL,
    user_id uuid NOT NULL,
    questions jsonb NOT NULL,
    answers smallint[] NOT NULL,
    pass_score smallint NOT NULL CHECK (pass_score BETWEEN 1 AND 100),
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    correct smallint CHECK (correct >= 0),
    score smallint CHECK (score BETWEEN 0 AND 100),
    FOREIGN KEY (org_id, quiz_id) REFERENCES quizzes(org_id, id),
    FOREIGN KEY (org_id, user_id) REFERENCES memberships(org_id, user_id) ON DELETE CASCADE,
    CHECK (jsonb_typeof(questions) = 'array' AND jsonb_array_length(questions) = cardinality(answers)),
    CHECK ((finished_at IS NULL) = (score IS NULL) AND (score IS NULL) = (correct IS NULL))
);
-- At most one unfinished attempt per member and quiz; starting again resumes it.
CREATE UNIQUE INDEX quiz_attempts_open_idx ON quiz_attempts (quiz_id, user_id) WHERE finished_at IS NULL;
CREATE INDEX quiz_attempts_user_idx ON quiz_attempts (org_id, user_id, quiz_id);

-- +goose Down
DROP TABLE quiz_attempts;
DROP TABLE quiz_questions;
DROP TABLE quizzes;
