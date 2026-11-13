-- +goose Up
-- M2: one row per completed step per member. Steps are archived, never deleted, so rows stay valid.
ALTER TABLE module_steps ADD CONSTRAINT module_steps_org_id_id_key UNIQUE (org_id, id);

CREATE TABLE module_progress (
    org_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id),
    step_id uuid NOT NULL,
    completed_at timestamptz NOT NULL,
    PRIMARY KEY (org_id, user_id, step_id),
    FOREIGN KEY (org_id, step_id) REFERENCES module_steps(org_id, id),
    FOREIGN KEY (org_id, user_id) REFERENCES memberships(org_id, user_id) ON DELETE CASCADE
);
-- "Lanjutkan belajar": the member's most recent activity.
CREATE INDEX module_progress_recent_idx ON module_progress (org_id, user_id, completed_at DESC);

-- +goose Down
DROP TABLE module_progress;
ALTER TABLE module_steps DROP CONSTRAINT module_steps_org_id_id_key;
