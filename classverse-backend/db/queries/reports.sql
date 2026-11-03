-- name: CreateUserReport :one
-- Report a user without a message. The room (optional) must belong to the
-- organization; reporter and target must both be members of it. No row when
-- any check fails, so the caller answers 404 without revealing which.
INSERT INTO reports (org_id, room_id, reporter_id, target_user_id, reason, created_at)
SELECT r.org_id, r.id, sqlc.arg(reporter_id), sqlc.arg(target_id), sqlc.arg(reason), sqlc.arg(now)
FROM rooms r
JOIN memberships rep ON rep.org_id = r.org_id AND rep.user_id = sqlc.arg(reporter_id)
JOIN memberships tgt ON tgt.org_id = r.org_id AND tgt.user_id = sqlc.arg(target_id)
WHERE r.id = sqlc.arg(room_id) AND r.archived_at IS NULL
  AND sqlc.arg(target_id)::uuid <> sqlc.arg(reporter_id)::uuid
RETURNING id;

-- name: CreateMessageReport :one
-- Report a stored message. Organization, room and target come from the
-- message itself (never from the client); the reporter must be a member of
-- the message's organization.
INSERT INTO reports (org_id, room_id, reporter_id, target_user_id, message_id, reason, created_at)
SELECT r.org_id, r.id, sqlc.arg(reporter_id), cm.sender_id, cm.id, sqlc.arg(reason), sqlc.arg(now)
FROM chat_messages cm
JOIN room_sessions s ON s.id = cm.session_id
JOIN rooms r ON r.id = s.room_id
JOIN memberships rep ON rep.org_id = r.org_id AND rep.user_id = sqlc.arg(reporter_id)
WHERE cm.id = sqlc.arg(message_id) AND cm.sender_id <> sqlc.arg(reporter_id)
RETURNING id;

-- name: GetReport :one
-- Report with display data; scoped to the organization by the caller.
SELECT rp.id, rp.org_id, rp.room_id, rm.name AS room_name,
       rp.reporter_id, ur.display_name AS reporter_name,
       rp.target_user_id, ut.display_name AS target_name,
       rp.message_id, cm.body AS message_text,
       rp.reason, rp.status, rp.reviewed_by, rp.reviewed_at, rp.created_at
FROM reports rp
JOIN users ur ON ur.id = rp.reporter_id
JOIN users ut ON ut.id = rp.target_user_id
LEFT JOIN rooms rm ON rm.id = rp.room_id
LEFT JOIN chat_messages cm ON cm.id = rp.message_id
WHERE rp.id = sqlc.arg(id);

-- name: ListReports :many
SELECT rp.id, rp.org_id, rp.room_id, rm.name AS room_name,
       rp.reporter_id, ur.display_name AS reporter_name,
       rp.target_user_id, ut.display_name AS target_name,
       rp.message_id, cm.body AS message_text,
       rp.reason, rp.status, rp.reviewed_by, rp.reviewed_at, rp.created_at
FROM reports rp
JOIN users ur ON ur.id = rp.reporter_id
JOIN users ut ON ut.id = rp.target_user_id
LEFT JOIN rooms rm ON rm.id = rp.room_id
LEFT JOIN chat_messages cm ON cm.id = rp.message_id
WHERE rp.org_id = sqlc.arg(org_id)
  AND (sqlc.narg(status)::report_status IS NULL OR rp.status = sqlc.narg(status)::report_status)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (rp.created_at, rp.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY rp.created_at DESC, rp.id DESC
LIMIT sqlc.arg(row_limit);

-- name: ReviewReport :execrows
UPDATE reports
SET status = sqlc.arg(status)::report_status, reviewed_by = sqlc.arg(reviewer_id), reviewed_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND org_id = sqlc.arg(org_id);

-- name: ListModerationActions :many
-- Audit log of one organization (admin only, checked by the caller).
SELECT ma.id, ma.room_id, r.name AS room_name,
       ma.actor_id, ua.display_name AS actor_name,
       ma.target_id, ut.display_name AS target_name,
       ma.action::text AS action, ma.created_at
FROM moderation_actions ma
JOIN rooms r ON r.id = ma.room_id
JOIN users ua ON ua.id = ma.actor_id
LEFT JOIN users ut ON ut.id = ma.target_id
WHERE r.org_id = sqlc.arg(org_id)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (ma.created_at, ma.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY ma.created_at DESC, ma.id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetReportOrg :one
SELECT org_id FROM reports WHERE id = sqlc.arg(id);
