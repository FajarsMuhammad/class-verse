-- name: ListRecoverableRooms :many
-- Startup-only system query: enumerate open sessions across organizations.
SELECT r.id, r.org_id, r.name, r.building_key, r.map_key, r.capacity, r.all_chat_enabled
FROM rooms r JOIN room_sessions s ON s.room_id = r.id
WHERE s.ended_at IS NULL AND r.archived_at IS NULL AND r.map_key <> 'campus';

-- name: EndInterruptedAttendance :exec
-- Startup-only system cleanup: sockets from the previous process no longer exist.
UPDATE session_attendances SET left_at = GREATEST(joined_at, sqlc.arg(now)::timestamptz)
WHERE left_at IS NULL;
