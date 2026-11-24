-- name: GetGuestRoom :one
-- A room that can host guests: not campus or staff-only, not archived.
SELECT r.id, r.org_id, r.name, r.map_key
FROM rooms r
WHERE r.id = sqlc.arg(room_id) AND r.archived_at IS NULL
  AND r.map_key NOT IN ('campus', 'staff') AND r.building_key IS DISTINCT FROM 'ruang-guru';

-- name: CreateGuestLink :one
INSERT INTO guest_links (id, org_id, room_id, code, max_guests, expires_at, created_by, created_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(room_id), sqlc.arg(code), sqlc.arg(max_guests),
        sqlc.arg(expires_at), sqlc.arg(created_by), sqlc.arg(created_at))
RETURNING *;

-- name: ListGuestLinks :many
SELECT * FROM guest_links
WHERE room_id = sqlc.arg(room_id) AND revoked_at IS NULL AND expires_at > sqlc.arg(now)
ORDER BY created_at DESC
LIMIT 20;

-- name: RevokeGuestLink :many
-- Revokes the link and returns its approved guests so live sessions can be ended.
WITH revoked AS (
  UPDATE guest_links gl SET revoked_at = sqlc.arg(now)
  WHERE gl.id = sqlc.arg(link_id) AND gl.room_id = sqlc.arg(room_id) AND gl.revoked_at IS NULL
  RETURNING gl.id AS link_id
)
SELECT g.user_id FROM guest_requests g JOIN revoked r ON r.link_id = g.link_id
WHERE g.status = 'approved';

-- name: GuestLinkExists :one
SELECT EXISTS (SELECT 1 FROM guest_links WHERE id = sqlc.arg(id) AND room_id = sqlc.arg(room_id));

-- name: PreviewGuestLink :one
SELECT l.expires_at, l.revoked_at, l.used_count, l.max_guests, r.name AS room_name, o.name AS org_name
FROM guest_links l
JOIN rooms r ON r.id = l.room_id AND r.archived_at IS NULL
JOIN organizations o ON o.id = l.org_id
WHERE l.code = sqlc.arg(code);

-- name: UseGuestLink :one
-- Claims one guest slot; no row when the link is missing, revoked, expired, or full.
UPDATE guest_links l SET used_count = used_count + 1
FROM rooms r
WHERE l.code = sqlc.arg(code) AND r.id = l.room_id AND r.archived_at IS NULL
  AND l.revoked_at IS NULL AND l.expires_at > sqlc.arg(now) AND l.used_count < l.max_guests
RETURNING l.id, l.room_id, l.expires_at;

-- name: CreateGuestUser :one
INSERT INTO users (display_name, is_guest)
VALUES (sqlc.arg(display_name), true)
RETURNING id;

-- name: CreateGuestRequest :exec
INSERT INTO guest_requests (id, link_id, user_id, request_key, created_at)
VALUES (sqlc.arg(id), sqlc.arg(link_id), sqlc.arg(user_id), sqlc.arg(request_key), sqlc.arg(created_at));

-- name: GuestStatus :one
-- The guest's own request; access ends with the link (revoked or expired).
SELECT g.status, u.display_name, l.expires_at,
       EXISTS (SELECT 1 FROM session_bans b JOIN room_sessions s ON s.id = b.session_id
               WHERE s.room_id = r.id AND s.ended_at IS NULL AND b.user_id = g.user_id) AS banned, r.id AS room_id, r.name AS room_name, r.map_key, o.name AS org_name,
       (l.revoked_at IS NULL AND l.expires_at > sqlc.arg(now) AND r.archived_at IS NULL)::boolean AS active
FROM guest_requests g
JOIN users u ON u.id = g.user_id
JOIN guest_links l ON l.id = g.link_id
JOIN rooms r ON r.id = l.room_id
JOIN organizations o ON o.id = l.org_id
WHERE g.user_id = sqlc.arg(user_id);

-- name: ListGuestRequests :many
-- Pending requests of active links of a room, oldest first.
SELECT g.id, g.user_id, u.display_name, g.created_at
FROM guest_requests g
JOIN guest_links l ON l.id = g.link_id
JOIN users u ON u.id = g.user_id
WHERE l.room_id = sqlc.arg(room_id) AND g.status = 'pending'
  AND l.revoked_at IS NULL AND l.expires_at > sqlc.arg(now)
ORDER BY g.created_at, g.id
LIMIT 50;

-- name: DecideGuestRequest :one
-- Decides a pending request of an active link of the room; no row otherwise.
UPDATE guest_requests g SET status = sqlc.arg(status), decided_by = sqlc.arg(decided_by), decided_at = sqlc.arg(now)
FROM guest_links l, users u
WHERE g.id = sqlc.arg(id) AND g.status = 'pending' AND l.id = g.link_id AND l.room_id = sqlc.arg(room_id)
  AND l.revoked_at IS NULL AND l.expires_at > sqlc.arg(now) AND u.id = g.user_id
RETURNING g.id, g.user_id, u.display_name, g.status, g.created_at;

-- name: GetGuestAdmission :one
-- Admission of an approved guest to the one room of their link; same shape as
-- GetRoomAdmission. No row for any other room or an inactive link.
SELECT r.id AS room_id, r.org_id, r.map_key, r.capacity, r.name AS room_name,
       r.building_key, r.scheduled_at, r.all_chat_enabled,
       u.display_name, u.avatar_key, l.expires_at,
       EXISTS (
         SELECT 1 FROM session_bans b
         JOIN room_sessions s ON s.id = b.session_id AND s.ended_at IS NULL
         WHERE s.room_id = r.id AND b.user_id = u.id
       ) AS banned
FROM guest_requests g
JOIN guest_links l ON l.id = g.link_id
JOIN rooms r ON r.id = l.room_id
JOIN users u ON u.id = g.user_id AND u.is_guest
WHERE g.user_id = sqlc.arg(user_id) AND g.status = 'approved' AND r.id = sqlc.arg(room_id)
  AND r.archived_at IS NULL AND l.revoked_at IS NULL AND l.expires_at > sqlc.arg(now)
  AND r.map_key NOT IN ('campus', 'staff') AND r.building_key IS DISTINCT FROM 'ruang-guru';

-- name: FindGuestRequest :one
SELECT g.user_id FROM guest_requests g JOIN guest_links l ON l.id = g.link_id
WHERE g.request_key = sqlc.arg(request_key) AND l.code = sqlc.arg(code);

-- name: LockGuestLink :one
SELECT l.id FROM guest_links l
WHERE l.room_id = sqlc.arg(room_id) AND l.id = sqlc.arg(link_id)
FOR UPDATE;

-- name: LockGuestRequestLink :one
SELECT l.id FROM guest_links l JOIN guest_requests g ON g.link_id = l.id
WHERE l.room_id = sqlc.arg(room_id) AND g.id = sqlc.arg(request_id)
FOR UPDATE OF l;

-- name: RecordGuestDecision :exec
INSERT INTO guest_decisions (room_id, link_id, actor_id, target_id, action, created_at)
VALUES (sqlc.arg(room_id), sqlc.arg(link_id), sqlc.arg(actor_id), sqlc.narg(target_id), sqlc.arg(action), sqlc.arg(created_at));

-- name: LockGuestRequestKey :exec
SELECT pg_advisory_xact_lock(hashtextextended(encode(sqlc.arg(request_key)::bytea, 'hex'), 0));
