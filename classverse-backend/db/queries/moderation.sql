-- name: KickFromOpenSession :execrows
-- Bans a participant from the room's open session and writes the audit row in
-- one statement, so neither can exist without the other. Zero rows when the
-- room has no open session, the actor is not staff, or the target is not a
-- participant of the same organization or a guest of this room (G1).
WITH target AS (
  SELECT s.id AS session_id, r.id AS room_id
  FROM room_sessions s
  JOIN rooms r ON r.id = s.room_id AND r.archived_at IS NULL
  JOIN memberships actor ON actor.org_id = r.org_id AND actor.user_id = sqlc.arg(actor_id)
  WHERE s.room_id = sqlc.arg(room_id) AND s.ended_at IS NULL
    AND actor.role IN ('teacher', 'admin')
    AND actor.user_id <> sqlc.arg(target_id)::uuid
    AND (EXISTS (SELECT 1 FROM memberships t
                 WHERE t.org_id = r.org_id AND t.user_id = sqlc.arg(target_id)::uuid AND t.role = 'participant')
      OR EXISTS (SELECT 1 FROM guest_requests g JOIN guest_links l ON l.id = g.link_id
                 WHERE g.user_id = sqlc.arg(target_id)::uuid AND l.room_id = r.id))
), banned AS (
  INSERT INTO session_bans (session_id, user_id, created_at)
  SELECT session_id, sqlc.arg(target_id)::uuid, sqlc.arg(now) FROM target
  ON CONFLICT (session_id, user_id) DO UPDATE SET created_at = session_bans.created_at
  RETURNING session_id
)
INSERT INTO moderation_actions (room_id, actor_id, target_id, action, created_at)
SELECT t.room_id, sqlc.arg(actor_id), sqlc.arg(target_id)::uuid, 'kick', sqlc.arg(now)
FROM target t JOIN banned b ON b.session_id = t.session_id;

-- name: AllowInOpenSession :execrows
-- Revokes a ban of the open session and audits it in one statement.
WITH target AS (
  SELECT s.id AS session_id, r.id AS room_id
  FROM room_sessions s
  JOIN rooms r ON r.id = s.room_id AND r.archived_at IS NULL
  JOIN memberships actor ON actor.org_id = r.org_id AND actor.user_id = sqlc.arg(actor_id)
  WHERE s.room_id = sqlc.arg(room_id) AND s.ended_at IS NULL
    AND actor.role IN ('teacher', 'admin')
), removed AS (
  DELETE FROM session_bans b USING target t
  WHERE b.session_id = t.session_id AND b.user_id = sqlc.arg(target_id)::uuid
  RETURNING b.session_id
)
INSERT INTO moderation_actions (room_id, actor_id, target_id, action, created_at)
SELECT t.room_id, sqlc.arg(actor_id), sqlc.arg(target_id)::uuid, 'allow', sqlc.arg(now)
FROM target t JOIN removed r ON r.session_id = t.session_id;

-- name: ListOpenSessionBans :many
SELECT u.id, u.display_name
FROM session_bans b
JOIN room_sessions s ON s.id = b.session_id AND s.ended_at IS NULL
JOIN users u ON u.id = b.user_id
WHERE s.room_id = sqlc.arg(room_id)
ORDER BY b.created_at, u.id;

-- name: RecordRoomAction :execrows
-- Audits mute/unmute/lock/unlock. Zero rows unless the room's session is open,
-- the actor is teacher/admin of its organization and, for mute/unmute, the
-- target is a participant of the same organization or a guest of this room.
INSERT INTO moderation_actions (room_id, actor_id, target_id, action, created_at)
SELECT r.id, sqlc.arg(actor_id), sqlc.narg(target_id)::uuid, sqlc.arg(action)::mod_action, sqlc.arg(now)
FROM rooms r
JOIN room_sessions s ON s.room_id = r.id AND s.ended_at IS NULL
JOIN memberships actor ON actor.org_id = r.org_id AND actor.user_id = sqlc.arg(actor_id)
WHERE r.id = sqlc.arg(room_id) AND r.archived_at IS NULL
  AND actor.role IN ('teacher', 'admin')
  AND (
    (sqlc.arg(action)::mod_action IN ('lock', 'unlock') AND sqlc.narg(target_id)::uuid IS NULL)
    OR (sqlc.arg(action)::mod_action IN ('mute', 'unmute') AND (EXISTS (
      SELECT 1 FROM memberships t
      WHERE t.org_id = r.org_id AND t.user_id = sqlc.narg(target_id)::uuid AND t.role = 'participant')
      OR EXISTS (SELECT 1 FROM guest_requests g JOIN guest_links l ON l.id = g.link_id
                 WHERE g.user_id = sqlc.narg(target_id)::uuid AND l.room_id = r.id)))
  );
