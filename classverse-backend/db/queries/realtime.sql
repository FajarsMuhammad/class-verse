-- name: GetRoomAdmission :one
-- The room plus the caller's membership and profile; no row when the room
-- does not exist, is archived, or the user is not a member of its organization.
SELECT r.id AS room_id, r.org_id, r.map_key, r.capacity,
       m.role, u.display_name, u.avatar_key
FROM rooms r
JOIN memberships m ON m.org_id = r.org_id AND m.user_id = sqlc.arg(user_id)
JOIN users u ON u.id = m.user_id
WHERE r.id = sqlc.arg(room_id) AND r.archived_at IS NULL;
