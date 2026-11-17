-- +goose Up
ALTER TABLE room_sessions ADD COLUMN pin_module_id uuid REFERENCES modules(id),
 ADD COLUMN pin_step_id uuid REFERENCES module_steps(id),
 ADD CONSTRAINT room_sessions_pin_pair CHECK(pin_step_id IS NULL OR pin_module_id IS NOT NULL);
-- +goose Down
ALTER TABLE room_sessions DROP COLUMN pin_step_id, DROP COLUMN pin_module_id;
