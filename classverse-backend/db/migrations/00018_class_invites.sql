-- +goose Up
ALTER TABLE invites ADD COLUMN class_id uuid;
ALTER TABLE invites ADD CONSTRAINT invites_class_fk FOREIGN KEY (org_id, class_id) REFERENCES classes (org_id, id);
ALTER TABLE invites ADD CONSTRAINT invites_class_role_check CHECK (class_id IS NULL OR role = 'participant');

-- +goose Down
ALTER TABLE invites DROP CONSTRAINT invites_class_role_check;
ALTER TABLE invites DROP CONSTRAINT invites_class_fk;
ALTER TABLE invites DROP COLUMN class_id;
