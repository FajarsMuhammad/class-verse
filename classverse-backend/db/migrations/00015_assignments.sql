-- +goose Up
-- Issued tasks retain their own recipients, ordered content and progress, independent of the open catalog.
CREATE TABLE assignments (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES organizations(id),
    class_id uuid NOT NULL,
    module_id uuid NOT NULL,
    title text NOT NULL,
    class_name text NOT NULL,
    year_label text NOT NULL,
    building_key text NOT NULL,
    level text NOT NULL,
    due_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    cancelled_at timestamptz,
    UNIQUE(org_id,id),
    FOREIGN KEY(org_id,class_id) REFERENCES classes(org_id,id),
    FOREIGN KEY(org_id,module_id) REFERENCES modules(org_id,id)
);
CREATE INDEX assignments_org_idx ON assignments(org_id,created_at DESC,id);
CREATE TABLE assignment_recipients (
    org_id uuid NOT NULL,
    assignment_id uuid NOT NULL,
    user_id uuid NOT NULL,
    PRIMARY KEY(org_id,assignment_id,user_id),
    FOREIGN KEY(org_id,assignment_id) REFERENCES assignments(org_id,id),
    FOREIGN KEY(org_id,user_id) REFERENCES memberships(org_id,user_id) ON DELETE CASCADE
);
CREATE INDEX assignment_recipients_user_idx ON assignment_recipients(org_id,user_id,assignment_id);
CREATE TABLE assignment_steps (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    assignment_id uuid NOT NULL,
    position integer NOT NULL CHECK(position >= 0),
    kind text NOT NULL CHECK(kind IN ('materi','berkas','video','kuis')),
    title text NOT NULL,
    body text NOT NULL,
    file_id uuid,
    video_id text,
    quiz_id uuid,
    minutes integer NOT NULL CHECK(minutes BETWEEN 1 AND 240),
    UNIQUE(org_id,assignment_id,id),
    UNIQUE(assignment_id,position),
    FOREIGN KEY(org_id,assignment_id) REFERENCES assignments(org_id,id),
    FOREIGN KEY(org_id,file_id) REFERENCES files(org_id,id),
    FOREIGN KEY(org_id,quiz_id) REFERENCES quizzes(org_id,id),
    CHECK ((kind='materi' AND file_id IS NULL AND video_id IS NULL AND quiz_id IS NULL)
        OR (kind='berkas' AND file_id IS NOT NULL AND video_id IS NULL AND quiz_id IS NULL)
        OR (kind='video' AND file_id IS NULL AND video_id IS NOT NULL AND quiz_id IS NULL)
        OR (kind='kuis' AND file_id IS NULL AND video_id IS NULL AND quiz_id IS NOT NULL))
);
CREATE INDEX assignment_steps_file_idx ON assignment_steps(org_id,file_id) WHERE file_id IS NOT NULL;
CREATE TABLE assignment_progress (
    org_id uuid NOT NULL,
    assignment_id uuid NOT NULL,
    user_id uuid NOT NULL,
    step_id uuid NOT NULL,
    completed_at timestamptz NOT NULL,
    PRIMARY KEY(org_id,assignment_id,user_id,step_id),
    FOREIGN KEY(org_id,assignment_id,user_id) REFERENCES assignment_recipients(org_id,assignment_id,user_id) ON DELETE CASCADE,
    FOREIGN KEY(org_id,assignment_id,step_id) REFERENCES assignment_steps(org_id,assignment_id,id)
);

ALTER TABLE quiz_attempts ADD COLUMN assignment_id uuid, ADD COLUMN assignment_step_id uuid,
 ADD CONSTRAINT quiz_attempts_assignment_context CHECK ((assignment_id IS NULL) = (assignment_step_id IS NULL)),
 ADD CONSTRAINT quiz_attempts_assignment_step_fk FOREIGN KEY(org_id,assignment_id,assignment_step_id) REFERENCES assignment_steps(org_id,assignment_id,id),
 ADD CONSTRAINT quiz_attempts_assignment_recipient_fk FOREIGN KEY(org_id,assignment_id,user_id) REFERENCES assignment_recipients(org_id,assignment_id,user_id) ON DELETE CASCADE;
DROP INDEX quiz_attempts_open_idx;
CREATE UNIQUE INDEX quiz_attempts_open_idx ON quiz_attempts(quiz_id,user_id,assignment_step_id) NULLS NOT DISTINCT WHERE finished_at IS NULL;

-- +goose Down
-- Contextual attempts cannot be safely relabelled as catalog work during rollback.
DELETE FROM quiz_attempts WHERE assignment_id IS NOT NULL;
DROP INDEX quiz_attempts_open_idx;
ALTER TABLE quiz_attempts DROP COLUMN assignment_id, DROP COLUMN assignment_step_id;
CREATE UNIQUE INDEX quiz_attempts_open_idx ON quiz_attempts(quiz_id,user_id) WHERE finished_at IS NULL;
DROP TABLE assignment_progress;
DROP TABLE assignment_steps;
DROP TABLE assignment_recipients;
DROP TABLE assignments;
