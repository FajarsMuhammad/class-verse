-- +goose Up
CREATE TABLE file_storage (
    org_id uuid PRIMARY KEY REFERENCES organizations(id),
    quota_bytes bigint NOT NULL DEFAULT 5000000000 CHECK (quota_bytes > 0),
    used_bytes bigint NOT NULL DEFAULT 0 CHECK (used_bytes >= 0)
);

CREATE TABLE files (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES file_storage(org_id),
    uploaded_by uuid NOT NULL REFERENCES users(id),
    kind text NOT NULL CHECK (kind IN ('pdf', 'audio')),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    content_type text CHECK (content_type IN ('application/pdf', 'audio/mpeg', 'audio/mp4')),
    size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 1 AND 50000000),
    sha256 bytea,
    object_key text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'deleting', 'deleted')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    deleted_at timestamptz,
    UNIQUE (org_id, id),
    CHECK (sha256 IS NULL OR octet_length(sha256) = 32),
    CHECK (status <> 'ready' OR (sha256 IS NOT NULL AND content_type IS NOT NULL)),
    CHECK ((status = 'deleted') = (deleted_at IS NOT NULL))
);
CREATE INDEX files_org_active_idx ON files (org_id, created_at, id) WHERE status <> 'deleted';
CREATE INDEX files_cleanup_idx ON files (updated_at, id) WHERE status IN ('pending', 'deleting');

-- +goose Down
DROP TABLE files;
DROP TABLE file_storage;
