-- +goose Up
-- M1: learning paths per campus building and level. Draft: published_at NULL.
-- Steps are never deleted once created; archived_at hides them so M2 progress keeps its references.
CREATE TABLE modules (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES organizations(id),
    building_key text NOT NULL CHECK (char_length(building_key) BETWEEN 1 AND 40),
    level text NOT NULL CHECK (level IN ('SD', 'SMP', 'SMA', 'Umum')),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    published_at timestamptz,
    archived_at timestamptz,
    UNIQUE (org_id, id)
);
CREATE INDEX modules_org_building_idx ON modules (org_id, building_key, level) WHERE archived_at IS NULL;

CREATE TABLE module_steps (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    module_id uuid NOT NULL,
    position integer NOT NULL CHECK (position >= 0),
    kind text NOT NULL CHECK (kind IN ('materi', 'berkas', 'video')),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    body text NOT NULL DEFAULT '' CHECK (char_length(body) <= 5000),
    file_id uuid,
    -- YouTube video ID only; the client builds the youtube-nocookie embed URL.
    video_id text CHECK (video_id ~ '^[A-Za-z0-9_-]{11}$'),
    minutes integer NOT NULL CHECK (minutes BETWEEN 1 AND 240),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    archived_at timestamptz,
    FOREIGN KEY (org_id, module_id) REFERENCES modules(org_id, id),
    FOREIGN KEY (org_id, file_id) REFERENCES files(org_id, id),
    CHECK ((kind = 'berkas') = (file_id IS NOT NULL)),
    CHECK ((kind = 'video') = (video_id IS NOT NULL)),
    CHECK (kind <> 'materi' OR char_length(body) >= 1)
);
CREATE INDEX module_steps_module_idx ON module_steps (module_id, position) WHERE archived_at IS NULL;
CREATE INDEX module_steps_file_idx ON module_steps (org_id, file_id) WHERE archived_at IS NULL;

-- +goose Down
DROP TABLE module_steps;
DROP TABLE modules;
