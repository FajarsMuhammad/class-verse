-- name: CreateRoom :one
INSERT INTO rooms (org_id, name, map_key, building_key, capacity, scheduled_at, created_by, created_at)
VALUES (sqlc.arg(org_id), sqlc.arg(name), sqlc.arg(map_key), sqlc.arg(building_key), 40,
        sqlc.narg(scheduled_at), sqlc.arg(created_by), sqlc.arg(now))
RETURNING *;

-- name: ListRooms :many
-- Non-campus, non-archived rooms of one organization, newest first. Staff-only
-- buildings are filtered by the caller's role.
SELECT * FROM rooms
WHERE org_id = sqlc.arg(org_id)
  AND archived_at IS NULL
  AND map_key <> 'campus'
  AND (sqlc.narg(building_key)::text IS NULL OR building_key = sqlc.narg(building_key))
  AND (sqlc.arg(include_staff)::boolean OR building_key <> 'ruang-guru')
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetRoomForMember :one
SELECT r.*, m.role AS member_role
FROM rooms r
JOIN memberships m ON m.org_id = r.org_id AND m.user_id = sqlc.arg(user_id)
WHERE r.id = sqlc.arg(id) AND r.archived_at IS NULL;

-- name: UpdateRoom :one
UPDATE rooms
SET name = coalesce(sqlc.narg(name), name),
    scheduled_at = CASE WHEN sqlc.arg(set_schedule)::boolean THEN sqlc.narg(scheduled_at) ELSE scheduled_at END
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id) AND archived_at IS NULL AND map_key <> 'campus'
RETURNING *;

-- name: ArchiveRoom :one
-- Only rooms without an open session can be archived; live occupancy is
-- checked in the hub before this runs.
UPDATE rooms r
SET archived_at = sqlc.arg(now)
WHERE r.id = sqlc.arg(id) AND r.org_id = sqlc.arg(org_id) AND r.archived_at IS NULL AND r.map_key <> 'campus'
  AND NOT EXISTS (SELECT 1 FROM room_sessions s WHERE s.room_id = r.id AND s.ended_at IS NULL)
RETURNING *;

-- name: OpenRoomSession :one
-- Reuses the open session if one exists (unique partial index).
INSERT INTO room_sessions (room_id, started_at)
VALUES (sqlc.arg(room_id), sqlc.arg(now))
ON CONFLICT (room_id) WHERE ended_at IS NULL DO UPDATE SET room_id = EXCLUDED.room_id
RETURNING id;

-- name: CloseRoomSession :exec
-- Ends the open session of a room and any attendance still open in it.
WITH closed AS (
  UPDATE room_sessions SET ended_at = sqlc.arg(now)
  WHERE room_id = sqlc.arg(room_id) AND ended_at IS NULL RETURNING id
)
UPDATE session_attendances SET left_at = sqlc.arg(now)
WHERE session_id IN (SELECT id FROM closed) AND left_at IS NULL;

-- name: StartAttendance :exec
INSERT INTO session_attendances (session_id, user_id, joined_at)
VALUES (sqlc.arg(session_id), sqlc.arg(user_id), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: EndAttendance :exec
UPDATE session_attendances SET left_at = sqlc.arg(now)
WHERE session_id = sqlc.arg(session_id) AND user_id = sqlc.arg(user_id) AND left_at IS NULL;

-- name: CloseOpenSessions :exec
-- Startup cleanup: sessions left open by a crash are ended.
WITH closed AS (
  UPDATE room_sessions SET ended_at = sqlc.arg(now) WHERE ended_at IS NULL RETURNING id
)
UPDATE session_attendances SET left_at = sqlc.arg(now)
WHERE session_id IN (SELECT id FROM closed) AND left_at IS NULL;
