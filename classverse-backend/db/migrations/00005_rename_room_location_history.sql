-- +goose Up
-- Domain names remain stable across release phases; preserve rollback data in place.
ALTER TABLE mvp2_room_locations RENAME TO room_location_history;
ALTER TABLE room_location_history RENAME CONSTRAINT mvp2_room_locations_pkey TO room_location_history_pkey;
ALTER TABLE room_location_history RENAME CONSTRAINT mvp2_room_locations_room_id_fkey TO room_location_history_room_id_fkey;

-- +goose Down
-- Restore the name expected by migration 00004 before its rollback runs.
ALTER TABLE room_location_history RENAME CONSTRAINT room_location_history_room_id_fkey TO mvp2_room_locations_room_id_fkey;
ALTER TABLE room_location_history RENAME CONSTRAINT room_location_history_pkey TO mvp2_room_locations_pkey;
ALTER TABLE room_location_history RENAME TO mvp2_room_locations;
