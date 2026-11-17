-- +goose Up
-- C1: classes (rombel) per school and academic year. At most one active class per student per school and year.
CREATE TABLE academic_years (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES organizations(id),
    label text NOT NULL CHECK (label ~ '^[0-9]{4}/[0-9]{4}$'
        AND substr(label, 6, 4)::integer = substr(label, 1, 4)::integer + 1),
    active boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    UNIQUE (org_id, id),
    UNIQUE (org_id, label)
);
-- The admin-chosen current year; reports may still select older years.
CREATE UNIQUE INDEX academic_years_active_idx ON academic_years (org_id) WHERE active;

CREATE TABLE classes (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    academic_year_id uuid NOT NULL,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    archived_at timestamptz,
    UNIQUE (org_id, id),
    UNIQUE (org_id, academic_year_id, id),
    FOREIGN KEY (org_id, academic_year_id) REFERENCES academic_years(org_id, id)
);
CREATE UNIQUE INDEX classes_name_idx ON classes (org_id, academic_year_id, lower(name)) WHERE archived_at IS NULL;

-- History is kept: moving closes the active row (ended_at) and opens a new one in one transaction.
CREATE TABLE class_enrollments (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    academic_year_id uuid NOT NULL,
    class_id uuid NOT NULL,
    user_id uuid NOT NULL,
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id),
    CHECK (ended_at IS NULL OR ended_at >= started_at),
    -- The class must belong to the same school and year as the enrollment.
    FOREIGN KEY (org_id, academic_year_id, class_id) REFERENCES classes(org_id, academic_year_id, id),
    FOREIGN KEY (org_id, user_id) REFERENCES memberships(org_id, user_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX class_enrollments_active_idx ON class_enrollments (org_id, academic_year_id, user_id) WHERE ended_at IS NULL;
CREATE INDEX class_enrollments_class_idx ON class_enrollments (class_id) WHERE ended_at IS NULL;

-- +goose Down
DROP TABLE class_enrollments;
DROP TABLE classes;
DROP TABLE academic_years;
