-- +goose Up
-- Files remain pending until the separate content-validation stage accepts them.
CREATE TABLE file_uploads (
    org_id uuid NOT NULL,
    file_id uuid PRIMARY KEY,
    expected_sha256 bytea NOT NULL CHECK (octet_length(expected_sha256) = 32),
    multipart_id text NOT NULL,
    state text NOT NULL DEFAULT 'uploading' CHECK (state IN ('uploading', 'assembled', 'canceled')),
    expires_at timestamptz NOT NULL,
    FOREIGN KEY (org_id, file_id) REFERENCES files(org_id, id),
    UNIQUE (org_id, file_id)
);
CREATE INDEX file_uploads_expiry_idx ON file_uploads(expires_at) WHERE state <> 'canceled';

-- PDF stores a page, audio stores whole seconds. Rows go away with their file.
CREATE TABLE reader_positions (
    org_id uuid NOT NULL,
    file_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id),
    position integer NOT NULL CHECK (position BETWEEN 0 AND 1000000),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (file_id, user_id),
    FOREIGN KEY (org_id, file_id) REFERENCES files(org_id, id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE reader_positions;
DROP TABLE file_uploads;
