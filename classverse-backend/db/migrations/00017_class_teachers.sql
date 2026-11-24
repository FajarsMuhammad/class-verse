-- +goose Up
-- C2: teachers of a class. homeroom = wali kelas (at most one active per class), subject = guru mapel (many).
-- History is kept: removing or replacing closes the row (ended_at). Role in the school is checked by the service.
CREATE TYPE class_teacher_role AS ENUM ('homeroom', 'subject');

CREATE TABLE class_teachers (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    class_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role class_teacher_role NOT NULL,
    subject text CHECK (subject IS NULL OR char_length(subject) BETWEEN 1 AND 40),
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id),
    CHECK (ended_at IS NULL OR ended_at >= started_at),
    CHECK (role = 'subject' OR subject IS NULL),
    FOREIGN KEY (org_id, class_id) REFERENCES classes(org_id, id),
    FOREIGN KEY (org_id, user_id) REFERENCES memberships(org_id, user_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX class_teachers_homeroom_idx ON class_teachers (class_id) WHERE role = 'homeroom' AND ended_at IS NULL;
CREATE UNIQUE INDEX class_teachers_person_idx ON class_teachers (class_id, user_id) WHERE ended_at IS NULL;
CREATE INDEX class_teachers_user_idx ON class_teachers (org_id, user_id) WHERE ended_at IS NULL;

-- +goose Down
DROP TABLE class_teachers;
DROP TYPE class_teacher_role;
