-- +goose Up
-- Slice 5: one open session per room, plus lookups for listing rooms by building.
CREATE UNIQUE INDEX room_sessions_open_uq ON room_sessions (room_id) WHERE ended_at IS NULL;
CREATE INDEX rooms_org_building_idx ON rooms (org_id, building_key, created_at DESC, id DESC) WHERE archived_at IS NULL;

-- +goose Down
DROP INDEX rooms_org_building_idx;
DROP INDEX room_sessions_open_uq;
