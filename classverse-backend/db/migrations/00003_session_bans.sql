-- +goose Up
-- Session-scoped bans survive actor cleanup and process restarts. Revocation
-- is explicit; an ended session is never consulted for admission.
CREATE TABLE session_bans (
    session_id uuid NOT NULL REFERENCES room_sessions(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (session_id, user_id)
);

-- Audit for revoking a kick (teacher lets the participant back in).
-- The value cannot be removed on Down; an unused enum value is harmless.
ALTER TYPE mod_action ADD VALUE IF NOT EXISTS 'allow';

-- +goose Down
DROP TABLE session_bans;
