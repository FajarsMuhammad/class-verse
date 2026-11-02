-- name: GetRoomAdmission :one
-- The room plus the caller's membership and profile; no row when the room
-- does not exist, is archived, or the user is not a member of its organization.
SELECT r.id AS room_id, r.org_id, r.map_key, r.capacity, r.name AS room_name,
       r.building_key, r.scheduled_at, r.all_chat_enabled,
       m.role, u.display_name, u.avatar_key,
       EXISTS (
         SELECT 1 FROM session_bans b
         JOIN room_sessions s ON s.id = b.session_id AND s.ended_at IS NULL
         WHERE s.room_id = r.id AND b.user_id = m.user_id
       ) AS banned
FROM rooms r
JOIN memberships m ON m.org_id = r.org_id AND m.user_id = sqlc.arg(user_id)
JOIN users u ON u.id = m.user_id
WHERE r.id = sqlc.arg(room_id) AND r.archived_at IS NULL;

-- name: GetCampusRoomID :one
SELECT id FROM rooms WHERE org_id = sqlc.arg(org_id) AND map_key = 'campus';

-- name: InsertChatMessage :one
-- Stores a filtered chat message in the open session of a room; no row when
-- the room has no open session.
INSERT INTO chat_messages (session_id, sender_id, scope, zone_key, body, created_at)
SELECT s.id, sqlc.arg(sender_id), sqlc.arg(scope), sqlc.narg(zone_key), sqlc.arg(body), sqlc.arg(now)
FROM room_sessions s
WHERE s.room_id = sqlc.arg(room_id) AND s.ended_at IS NULL
RETURNING id;

-- name: PurgeChatMessages :execrows
-- Retention: chat content older than the cutoff is deleted (reports keep a
-- null message_id through ON DELETE SET NULL).
DELETE FROM chat_messages WHERE created_at < sqlc.arg(cutoff);
